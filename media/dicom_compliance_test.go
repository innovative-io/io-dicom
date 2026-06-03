// Package media_test: DICOM standard compliance tests.
//
// Each test names the specific section of the DICOM standard it verifies so
// that a failing test immediately points to the relevant specification text.
// All tests are self-contained and produce DICOM data entirely in memory —
// no sample files are required.
package media_test

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
)

// minimalObj returns a valid DICOMObject that passes ValidateFileWrite.
func minimalObj(ts *transfersyntax.TransferSyntax) media.DICOMObject {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(ts)
	obj.SetExplicitVR(ts.UID != transfersyntax.ImplicitVRLittleEndian.UID)
	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.9000")
	return obj
}

// --- PS3.10 §10.1 File format structure ---

// TestPS310_FilePreambleAndMagic verifies that WriteToBytes produces 128 zero
// preamble bytes followed immediately by the 4-byte "DICM" prefix.
func TestPS310_FilePreambleAndMagic(t *testing.T) {
	data := minimalObj(transfersyntax.ExplicitVRLittleEndian).WriteToBytes()
	if len(data) < 132 {
		t.Fatalf("output too short (%d bytes), want >= 132", len(data))
	}
	for i, b := range data[:128] {
		if b != 0x00 {
			t.Fatalf("preamble byte %d = 0x%02X, want 0x00", i, b)
		}
	}
	if string(data[128:132]) != "DICM" {
		t.Fatalf("magic at offset 128 = %q, want DICM", data[128:132])
	}
}

// TestPS310_FileMetaGroupLengthIsAccurate verifies that (0002,0000) Group
// Length equals the byte count of all subsequent group-0002 tags.
// Per PS3.10 §10.1 the value must be the number of bytes from the byte
// following (0002,0000) to the last byte of the File Meta Information.
func TestPS310_FileMetaGroupLengthIsAccurate(t *testing.T) {
	obj := minimalObj(transfersyntax.ExplicitVRLittleEndian)
	data := obj.WriteToBytes()

	// (0002,0000) UL tag starts at offset 132.
	// Header layout: group(2) + element(2) + VR"UL"(2) + length16(2) = 8 bytes header, then 4 bytes data.
	// Total tag size: 12 bytes. Group length data is at bytes 140–143.
	if len(data) < 144 {
		t.Fatalf("output too short for group-length check (%d bytes)", len(data))
	}
	groupLen := binary.LittleEndian.Uint32(data[140:144])

	// groupLen is the count of bytes from offset 144 to the end of group 0002.
	// Re-parse to find where group 0002 ends.
	parsed, err := media.NewDCMObjFromBytes(data)
	if err != nil {
		t.Fatalf("NewDCMObjFromBytes: %v", err)
	}
	ts := parsed.GetTransferSyntax()
	if ts == nil {
		t.Fatal("transfer syntax not found in re-parsed file")
	}

	// The declared group length must equal actual meta size: count bytes
	// from offset 144 until we hit the first non-0002 tag.
	pos := 144
	for pos < len(data) {
		if pos+4 > len(data) {
			break
		}
		group := binary.LittleEndian.Uint16(data[pos : pos+2])
		if group != 0x0002 {
			break
		}
		// Determine tag size: short or long explicit VR.
		vr := string(data[pos+4 : pos+6])
		var tagSize int
		switch vr {
		case "OB", "OD", "OF", "OL", "OV", "OW", "SQ", "SV", "UC", "UN", "UR", "UT", "UV":
			// long VR: 8-byte header + 4-byte length field
			if pos+12 > len(data) {
				break
			}
			payloadLen := int(binary.LittleEndian.Uint32(data[pos+8 : pos+12]))
			tagSize = 12 + payloadLen
		default:
			// short VR: 6-byte header + 2-byte length field
			if pos+8 > len(data) {
				break
			}
			payloadLen := int(binary.LittleEndian.Uint16(data[pos+6 : pos+8]))
			tagSize = 8 + payloadLen
		}
		pos += tagSize
	}
	actual := uint32(pos - 144)
	if groupLen != actual {
		t.Fatalf("(0002,0000) Group Length = %d, actual meta byte count = %d", groupLen, actual)
	}
}

// TestPS310_FileMetaIsAlwaysLittleEndianForBigEndianTS verifies that the File
// Meta Information is written in little-endian byte order even when the dataset
// transfer syntax is ExplicitVRBigEndian (PS3.10 §10.1).
// A big-endian group number 0x0002 would be [0x00, 0x02]; little-endian is [0x02, 0x00].
func TestPS310_FileMetaIsAlwaysLittleEndianForBigEndianTS(t *testing.T) {
	obj := minimalObj(transfersyntax.ExplicitVRBigEndian)
	obj.SetBigEndian(true)
	data := obj.WriteToBytes()
	if len(data) < 136 {
		t.Fatalf("output too short (%d bytes)", len(data))
	}
	// First meta tag at offset 132: group must be 0x0002 in little-endian → [0x02, 0x00].
	if data[132] != 0x02 || data[133] != 0x00 {
		t.Fatalf("group bytes at offset 132 = [%#x, %#x], want [0x02, 0x00] (little-endian); "+
			"meta header is being written in big-endian", data[132], data[133])
	}
	// VR length field for UL (short VR) at offset 138 must be 0x0004 in little-endian → [0x04, 0x00].
	if data[138] != 0x04 || data[139] != 0x00 {
		t.Fatalf("UL length bytes at offset 138 = [%#x, %#x], want [0x04, 0x00] (little-endian)",
			data[138], data[139])
	}
}

// TestPS310_TransferSyntaxUIDPresentInMeta verifies that (0002,0010) is
// present in the encoded file and holds the correct UID string.
func TestPS310_TransferSyntaxUIDPresentInMeta(t *testing.T) {
	obj := minimalObj(transfersyntax.ExplicitVRLittleEndian)
	data := obj.WriteToBytes()

	parsed, err := media.NewDCMObjFromBytes(data)
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	if ts := parsed.GetTransferSyntax(); ts == nil || ts.UID != transfersyntax.ExplicitVRLittleEndian.UID {
		t.Fatalf("TransferSyntaxUID in meta = %v, want %s",
			ts, transfersyntax.ExplicitVRLittleEndian.UID)
	}
}

// --- PS3.5 §6.2 Value Representation encoding rules ---

// TestPS35_UIVRPaddedWithNullByte verifies that a UI value of odd byte length
// is padded with a single trailing 0x00 (not 0x20) to achieve even length.
// PS3.5 §6.2 Table 6.2-1: "UI: Padded with a single trailing NULL (00H)."
func TestPS35_UIVRPaddedWithNullByte(t *testing.T) {
	// "1.2.840.10008.1.2.1" is 19 bytes (odd) — must become 20 bytes with 0x00 padding.
	const uid = "1.2.840.10008.1.2.1" // 19 chars
	if len(uid)%2 != 1 {
		t.Fatal("test precondition: uid must have odd byte length")
	}
	obj := media.NewEmptyDCMObj()
	obj.Write(tags.TransferSyntaxUID, uid)

	tag := obj.GetTag(tags.TransferSyntaxUID)
	if tag == nil {
		t.Fatal("TransferSyntaxUID tag not found")
	}
	if len(tag.Data)%2 != 0 {
		t.Fatalf("UI tag data length = %d (odd), want even", len(tag.Data))
	}
	if tag.Data[len(tag.Data)-1] != 0x00 {
		t.Fatalf("UI tag padding byte = 0x%02X, want 0x00 (NULL)", tag.Data[len(tag.Data)-1])
	}
	// The logical value read back must equal the unpadded UID.
	if got := obj.GetString(tags.TransferSyntaxUID); got != uid {
		t.Fatalf("GetString(UITag) = %q, want %q", got, uid)
	}
}

// TestPS35_StringVRPaddedWithSpace verifies that a non-UI string VR value of
// odd byte length is padded with a trailing 0x20 (SPACE).
// PS3.5 §6.2 Table 6.2-1: "LO, PN, SH, CS … Padded with trailing spaces."
func TestPS35_StringVRPaddedWithSpace(t *testing.T) {
	// "CT" is 2 bytes (even) — no padding. "MR" is 2. Use "DX" for even, "CT" for odd:
	// Actually use a 5-character modality string to get odd length → "GSPS" (4, even).
	// "XRF" = 3 bytes (odd).
	const modality = "XRF" // 3 bytes, odd
	obj := media.NewEmptyDCMObj()
	obj.Write(tags.Modality, modality) // CS VR

	tag := obj.GetTag(tags.Modality)
	if tag == nil {
		t.Fatal("Modality tag not found")
	}
	if len(tag.Data)%2 != 0 {
		t.Fatalf("CS tag data length = %d (odd), want even", len(tag.Data))
	}
	if tag.Data[len(tag.Data)-1] != 0x20 {
		t.Fatalf("CS tag padding byte = 0x%02X, want 0x20 (SPACE)", tag.Data[len(tag.Data)-1])
	}
	if got := obj.GetString(tags.Modality); got != modality {
		t.Fatalf("GetString(Modality) = %q, want %q", got, modality)
	}
}

// TestPS35_GetStringStripsTrailingPadding verifies that GetString removes
// trailing NULL bytes and trailing SPACE characters introduced by VR padding.
func TestPS35_GetStringStripsTrailingPadding(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"trailing spaces", []byte{'D', 'O', 'E', ' ', ' '}, "DOE"},
		{"trailing null", []byte{'1', '.', '2', '.', '3', 0x00}, "1.2.3"},
		{"mixed null then spaces", []byte{'C', 'T', 0x00, ' '}, "CT"},
		{"all spaces", []byte{' ', ' ', ' ', ' '}, ""},
		{"empty", []byte{}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tag := &media.DICOMTag{
				Group: 0x0010, Element: 0x0010,
				VR: "LO", Length: uint32(len(tc.data)), Data: tc.data,
			}
			if got := tag.GetString(); got != tc.want {
				t.Fatalf("GetString() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPS35_AllTagValuesAreEvenLength verifies that every tag value written via
// Write() has an even byte length, satisfying the PS3.5 §7.1.3 even-length rule.
func TestPS35_AllTagValuesAreEvenLength(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.Write(tags.PatientName, "DOE^A")            // 5 bytes → padded to 6
	obj.Write(tags.PatientID, "1")                  // 1 byte → padded to 2
	obj.Write(tags.StudyDate, "20240101")            // 8 bytes, already even
	obj.Write(tags.Modality, "XRF")                 // 3 bytes → padded to 4
	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7") // 26 bytes, even
	obj.Write(tags.BitsAllocated, uint16(16))        // 2 bytes, always even
	obj.Write(tags.BitsStored, uint16(16))
	obj.Write(tags.PixelRepresentation, uint16(0))

	for i := 0; i < obj.TagCount(); i++ {
		tag := obj.GetTagAt(i)
		if tag.Length == 0xFFFFFFFF {
			continue // undefined-length containers have no data bytes
		}
		if len(tag.Data)%2 != 0 {
			t.Errorf("tag (%04X,%04X) VR=%s data length = %d (odd)",
				tag.Group, tag.Element, tag.VR, len(tag.Data))
		}
	}
}

// --- PS3.5 §7.2 Implicit VR encoding ---

// TestPS35_ImplicitVRRoundtrip writes an ImplicitVRLittleEndian file entirely
// in memory, re-parses it, and verifies that the dictionary VR is restored for
// each tag (since the VR is not stored in the stream for implicit files).
func TestPS35_ImplicitVRRoundtrip(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ImplicitVRLittleEndian)
	obj.SetExplicitVR(false)
	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.9001")
	obj.Write(tags.PatientName, "DOE^JOHN")
	obj.Write(tags.StudyDate, "20240101")
	obj.Write(tags.BitsAllocated, uint16(16))

	data := obj.WriteToBytes()
	if len(data) == 0 {
		t.Fatal("WriteToBytes returned empty slice")
	}

	parsed, err := media.NewDCMObjFromBytes(data)
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	if parsed.GetTransferSyntax().UID != transfersyntax.ImplicitVRLittleEndian.UID {
		t.Fatalf("re-parsed TS = %s", parsed.GetTransferSyntax().UID)
	}

	// String values must survive the round-trip.
	if got := parsed.GetString(tags.PatientName); got != "DOE^JOHN" {
		t.Fatalf("PatientName = %q, want DOE^JOHN", got)
	}
	if got := parsed.GetString(tags.StudyDate); got != "20240101" {
		t.Fatalf("StudyDate = %q, want 20240101", got)
	}
	if got := parsed.GetUint16(tags.BitsAllocated); got != 16 {
		t.Fatalf("BitsAllocated = %d, want 16", got)
	}

	// The dictionary VR must be populated after an implicit-VR parse.
	if pt := parsed.GetTag(tags.PatientName); pt == nil || pt.VR != "PN" {
		if pt == nil {
			t.Fatal("PatientName tag absent after roundtrip")
		}
		t.Fatalf("PatientName VR = %q, want PN (dictionary lookup)", pt.VR)
	}
	if pt := parsed.GetTag(tags.BitsAllocated); pt == nil || pt.VR != "US" {
		if pt == nil {
			t.Fatal("BitsAllocated tag absent after roundtrip")
		}
		t.Fatalf("BitsAllocated VR = %q, want US (dictionary lookup)", pt.VR)
	}
}

// --- PS3.5 §7.5 Nested sequences ---

// TestPS35_SequenceExplicitVRFlagPreserved verifies that ReadSeq returns a
// DICOMObject whose IsExplicitVR() matches the flag passed to ReadSeq.
// A returned object with the wrong flag would decode any nested SQ tags using
// the wrong encoding, silently producing corrupt data on deeply-nested files.
func TestPS35_SequenceExplicitVRFlagPreserved(t *testing.T) {
	// Build a sequence item with some content.
	item := media.NewEmptyDCMObj()
	item.SetExplicitVR(true)
	item.Write(tags.PatientID, "SEQ-TEST")

	seqTag := &media.DICOMTag{}
	seqTag.WriteSeq(0x0040, 0xA730, item)

	// ReadSeq(true) → returned object must have IsExplicitVR() == true.
	readExplicit := seqTag.ReadSeq(true)
	if !readExplicit.IsExplicitVR() {
		t.Error("ReadSeq(true): returned object has IsExplicitVR()=false; nested SQ decoding will be wrong")
	}

	// ReadSeq(false) → returned object must have IsExplicitVR() == false.
	readImplicit := seqTag.ReadSeq(false)
	if readImplicit.IsExplicitVR() {
		t.Error("ReadSeq(false): returned object has IsExplicitVR()=true")
	}
}

// TestPS35_NestedSequenceContentReadable verifies that data written into a
// sequence item with WriteSeq can be recovered via ReadSeq.
func TestPS35_NestedSequenceContentReadable(t *testing.T) {
	item := media.NewEmptyDCMObj()
	item.SetExplicitVR(true)
	item.Write(tags.PatientName, "NESTED^PATIENT")
	item.Write(tags.BitsAllocated, uint16(8))

	seqTag := &media.DICOMTag{}
	seqTag.WriteSeq(0x0040, 0xA730, item)

	readBack := seqTag.ReadSeq(true)
	if got := readBack.GetString(tags.PatientName); got != "NESTED^PATIENT" {
		t.Fatalf("nested PatientName = %q, want NESTED^PATIENT", got)
	}
	if got := readBack.GetUint16(tags.BitsAllocated); got != 8 {
		t.Fatalf("nested BitsAllocated = %d, want 8", got)
	}
}

// --- Write API correctness ---

// TestWrite_RepeatedWriteSameTagOverwrites verifies that calling Write for the
// same group+element twice updates the tag value in place rather than appending
// a second copy. Duplicate tags in a DICOM data set are non-conformant and
// cause inconsistent results in other parsers.
func TestWrite_RepeatedWriteSameTagOverwrites(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.Write(tags.PatientName, "DOE^JOHN")
	obj.Write(tags.PatientName, "DOE^JANE")

	if got := obj.GetString(tags.PatientName); got != "DOE^JANE" {
		t.Fatalf("after second Write, PatientName = %q, want DOE^JANE", got)
	}

	// Exactly one PatientName entry must exist in the flat tag list.
	count := 0
	for i := 0; i < obj.TagCount(); i++ {
		tag := obj.GetTagAt(i)
		if tag.Group == tags.PatientName.Group && tag.Element == tags.PatientName.Element {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("found %d PatientName tags after two Writes, want 1 (Write must overwrite, not append)", count)
	}
}

// TestWrite_TypedOverloadsAllDeduplicate verifies that the typed Write helpers
// (WriteUint16, WriteString, WriteFloat32, WriteFloat64) all share the same
// deduplication behaviour as Write.
func TestWrite_TypedOverloadsAllDeduplicate(t *testing.T) {
	obj := media.NewEmptyDCMObj()

	obj.WriteUint16(tags.BitsAllocated, 8)
	obj.WriteUint16(tags.BitsAllocated, 16)
	if got := obj.GetUint16(tags.BitsAllocated); got != 16 {
		t.Fatalf("WriteUint16 overwrite: got %d, want 16", got)
	}
	if countTag(obj, tags.BitsAllocated.Group, tags.BitsAllocated.Element) != 1 {
		t.Fatal("WriteUint16: duplicate tag entries found")
	}

	obj.WriteString(tags.PatientID, "OLD")
	obj.WriteString(tags.PatientID, "NEW")
	if got := obj.GetString(tags.PatientID); got != "NEW" {
		t.Fatalf("WriteString overwrite: got %q, want NEW", got)
	}
	if countTag(obj, tags.PatientID.Group, tags.PatientID.Element) != 1 {
		t.Fatal("WriteString: duplicate tag entries found")
	}

	obj.WriteFloat32(tags.SelectorFLValue, 1.5)
	obj.WriteFloat32(tags.SelectorFLValue, 3.0)
	if got := obj.GetFloat32(tags.SelectorFLValue); got != 3.0 {
		t.Fatalf("WriteFloat32 overwrite: got %v, want 3.0", got)
	}

	obj.WriteFloat64(tags.SelectorFDValue, 1.5)
	obj.WriteFloat64(tags.SelectorFDValue, 9.9)
	if got := obj.GetFloat64(tags.SelectorFDValue); got != 9.9 {
		t.Fatalf("WriteFloat64 overwrite: got %v, want 9.9", got)
	}
}

func countTag(obj media.DICOMObject, group, element uint16) int {
	n := 0
	for i := 0; i < obj.TagCount(); i++ {
		t := obj.GetTagAt(i)
		if t.Group == group && t.Element == element {
			n++
		}
	}
	return n
}

// --- ValidateFileWrite rules ---

// TestValidateFileWrite_SOPClassUIDMismatch verifies that a file where
// (0002,0002) MediaStorageSOPClassUID differs from (0008,0016) SOPClassUID is
// rejected before encoding.
func TestValidateFileWrite_SOPClassUIDMismatch(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.Write(tags.MediaStorageSOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.4") // MR — different
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.9002")
	obj.Write(tags.MediaStorageSOPInstanceUID, "1.2.826.0.1.3680043.10.90.9002")

	if err := media.ValidateFileWrite(obj); err == nil {
		t.Fatal("ValidateFileWrite: expected error for SOPClassUID mismatch, got nil")
	}
}

// TestValidateFileWrite_SOPInstanceUIDMismatch verifies that a file where
// (0002,0003) MediaStorageSOPInstanceUID differs from (0008,0018) SOPInstanceUID
// is rejected before encoding.
func TestValidateFileWrite_SOPInstanceUIDMismatch(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.MediaStorageSOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.9003")
	obj.Write(tags.MediaStorageSOPInstanceUID, "1.2.826.0.1.3680043.10.90.DIFFERENT")

	if err := media.ValidateFileWrite(obj); err == nil {
		t.Fatal("ValidateFileWrite: expected error for SOPInstanceUID mismatch, got nil")
	}
}

// TestValidateFileWrite_MatchingUIDs verifies that a file where both UID pairs
// match passes ValidateFileWrite without error.
func TestValidateFileWrite_MatchingUIDs(t *testing.T) {
	const class = "1.2.840.10008.5.1.4.1.1.7"
	const instance = "1.2.826.0.1.3680043.10.90.9004"
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.Write(tags.SOPClassUID, class)
	obj.Write(tags.MediaStorageSOPClassUID, class)
	obj.Write(tags.SOPInstanceUID, instance)
	obj.Write(tags.MediaStorageSOPInstanceUID, instance)

	if err := media.ValidateFileWrite(obj); err != nil {
		t.Fatalf("ValidateFileWrite: unexpected error for matching UIDs: %v", err)
	}
}

// --- ParseIndexFromBytes error cases ---

// TestParseIndex_ErrNotDICOM verifies that ParseIndexFromBytes returns
// ErrNotDICOM when the input does not contain the "DICM" magic bytes.
func TestParseIndex_ErrNotDICOM(t *testing.T) {
	notDICOM := make([]byte, 200)
	copy(notDICOM, []byte("NOT A DICOM FILE"))
	_, err := media.ParseIndexFromBytes(notDICOM)
	if !errors.Is(err, media.ErrNotDICOM) {
		t.Fatalf("ParseIndexFromBytes: got %v, want ErrNotDICOM", err)
	}
}

// TestParseIndex_ErrFileTooSmall verifies that ParseIndexFromBytes returns
// ErrFileTooSmall when given fewer than 132 bytes (128-byte preamble + 4-byte magic).
func TestParseIndex_ErrFileTooSmall(t *testing.T) {
	_, err := media.ParseIndexFromBytes([]byte("too short"))
	if !errors.Is(err, media.ErrFileTooSmall) {
		t.Fatalf("ParseIndexFromBytes: got %v, want ErrFileTooSmall", err)
	}
}

// TestParseIndex_ExtractsMetaFields verifies that ParseIndexFromBytes correctly
// extracts the key patient/study/series fields from a fully encoded DICOM file.
func TestParseIndex_ExtractsMetaFields(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.9005")
	obj.Write(tags.PatientName, "DOE^COMPLIANCE")
	obj.Write(tags.PatientID, "C-00001")
	obj.Write(tags.StudyDate, "20240601")
	obj.Write(tags.Modality, "CT")
	obj.Write(tags.StudyInstanceUID, "1.2.826.0.1.3680043.10.91.9005")
	obj.Write(tags.SeriesInstanceUID, "1.2.826.0.1.3680043.10.92.9005")

	data := obj.WriteToBytes()
	rec, err := media.ParseIndexFromBytes(data)
	if err != nil {
		t.Fatalf("ParseIndexFromBytes: %v", err)
	}
	if rec.PatientName != "DOE^COMPLIANCE" {
		t.Errorf("PatientName = %q, want DOE^COMPLIANCE", rec.PatientName)
	}
	if rec.PatientID != "C-00001" {
		t.Errorf("PatientID = %q, want C-00001", rec.PatientID)
	}
	if rec.StudyDate != "20240601" {
		t.Errorf("StudyDate = %q, want 20240601", rec.StudyDate)
	}
	if rec.SOPClassUID != "1.2.840.10008.5.1.4.1.1.7" {
		t.Errorf("SOPClassUID = %q", rec.SOPClassUID)
	}
}

// TestPS310_RoundtripPreservesAllFields verifies a full write→parse cycle for
// the common clinical tags, confirming that encoding and decoding are inverse
// operations for all VR types used in practice.
func TestPS310_RoundtripPreservesAllFields(t *testing.T) {
	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.SetExplicitVR(true)
	obj.Write(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.7")
	obj.Write(tags.SOPInstanceUID, "1.2.826.0.1.3680043.10.90.9006")
	obj.Write(tags.PatientName, "ROUND^TRIP")
	obj.Write(tags.PatientID, "RT-12345")
	obj.Write(tags.StudyDate, "20240315")
	obj.Write(tags.StudyTime, "143000")
	obj.Write(tags.Modality, "MR")
	obj.Write(tags.BitsAllocated, uint16(12))
	obj.Write(tags.BitsStored, uint16(12))
	obj.Write(tags.PixelRepresentation, uint16(0))
	obj.Write(tags.Rows, uint16(256))
	obj.Write(tags.Columns, uint16(256))
	obj.WriteFloat64(tags.SelectorFDValue, 3.14159)
	obj.WriteFloat32(tags.SelectorFLValue, float32(2.5))

	data := obj.WriteToBytes()
	parsed, err := media.NewDCMObjFromBytes(data)
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"PatientName", parsed.GetString(tags.PatientName), "ROUND^TRIP"},
		{"PatientID", parsed.GetString(tags.PatientID), "RT-12345"},
		{"StudyDate", parsed.GetString(tags.StudyDate), "20240315"},
		{"StudyTime", parsed.GetString(tags.StudyTime), "143000"},
		{"Modality", parsed.GetString(tags.Modality), "MR"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if got := parsed.GetUint16(tags.BitsAllocated); got != 12 {
		t.Errorf("BitsAllocated = %d, want 12", got)
	}
	if got := parsed.GetUint16(tags.Rows); got != 256 {
		t.Errorf("Rows = %d, want 256", got)
	}
	if got := parsed.GetFloat64(tags.SelectorFDValue); got != 3.14159 {
		t.Errorf("FD tag = %v, want 3.14159", got)
	}
	if got := parsed.GetFloat32(tags.SelectorFLValue); got != float32(2.5) {
		t.Errorf("FL tag = %v, want 2.5", got)
	}
}
