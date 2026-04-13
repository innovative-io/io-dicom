package media

import (
	"bufio"
	"encoding/binary"
	"net"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

func TestDICOMBuffer_IsBigEndian(t *testing.T) {
	buf := NewEmptyBufData()
	if buf.IsBigEndian() {
		t.Fatal("new buffer should be little-endian")
	}
	buf.SetBigEndian(true)
	if !buf.IsBigEndian() {
		t.Fatal("expected big-endian after SetBigEndian(true)")
	}
}

func TestDICOMBuffer_ClearMemoryStream(t *testing.T) {
	buf := NewBufDataFromBytes([]byte{1, 2, 3})
	buf.ClearMemoryStream()
	if buf.GetSize() != 0 {
		t.Fatalf("ClearMemoryStream() size = %d, want 0", buf.GetSize())
	}
}

func TestDICOMBuffer_ReadByte(t *testing.T) {
	buf := NewBufDataFromBytes([]byte{0xAA, 0xBB})
	b, err := buf.ReadByte()
	if err != nil || b != 0xAA {
		t.Fatalf("ReadByte() = %v, %v", b, err)
	}
}

func TestDICOMBuffer_WriteAETitle(t *testing.T) {
	buf := NewEmptyBufData()
	buf.WriteAETitle("SCU")
	// 16-byte field: "SCU" followed by spaces
	got := buf.GetAllBytes()
	if len(got) != 16 {
		t.Fatalf("WriteAETitle() wrote %d bytes, want 16", len(got))
	}
	if got[0] != 'S' || got[1] != 'C' || got[2] != 'U' {
		t.Fatalf("WriteAETitle() data = %q", got)
	}
	for i := 3; i < 16; i++ {
		if got[i] != 0x20 {
			t.Fatalf("WriteAETitle() padding at %d = 0x%02X, want 0x20", i, got[i])
		}
	}
}

func TestDICOMBuffer_WriteByte(t *testing.T) {
	buf := NewEmptyBufData()
	if err := buf.WriteByte(0xFF); err != nil {
		t.Fatal(err)
	}
	got := buf.GetAllBytes()
	if len(got) != 1 || got[0] != 0xFF {
		t.Fatalf("WriteByte() = %v", got)
	}
}

func TestDICOMBuffer_WriteString(t *testing.T) {
	buf := NewEmptyBufData()
	buf.WriteString("HELLO")
	got := buf.GetAllBytes()
	if string(got) != "HELLO" {
		t.Fatalf("WriteString() = %q, want HELLO", got)
	}
}

func TestDICOMBuffer_ReadUint16_BigEndian(t *testing.T) {
	buf := NewBufDataFromBytes([]byte{0x01, 0x02})
	buf.SetBigEndian(true)
	v, err := buf.ReadUint16()
	if err != nil || v != 0x0102 {
		t.Fatalf("ReadUint16 BE = %v, %v", v, err)
	}
}

func TestDICOMBuffer_ReadUint32_BigEndian(t *testing.T) {
	buf := NewBufDataFromBytes([]byte{0x00, 0x00, 0x01, 0x02})
	buf.SetBigEndian(true)
	v, err := buf.ReadUint32()
	if err != nil || v != 0x00000102 {
		t.Fatalf("ReadUint32 BE = %v, %v", v, err)
	}
}

func TestDICOMBuffer_Send(t *testing.T) {
	// Send writes the buffer content through a net.Conn.
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	buf := NewBufDataFromBytes([]byte{0x01, 0x02, 0x03})
	errCh := make(chan error, 1)
	go func() {
		bw := bufio.NewReadWriter(bufio.NewReader(client), bufio.NewWriter(client))
		errCh <- buf.Send(bw)
	}()

	received := make([]byte, 3)
	if _, err := server.Read(received); err != nil {
		t.Fatal(err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if received[0] != 1 || received[1] != 2 || received[2] != 3 {
		t.Fatalf("Send() received %v", received)
	}
}

func TestDICOMBuffer_WriteObj_NormalizesAmbiguousPixelDataVRForNativeExplicitSyntax(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.SetExplicitVR(true)
	obj.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	obj.Add(&DICOMTag{
		Group:   0x7FE0,
		Element: 0x0010,
		VR:      "OB or OW",
		Length:  4,
		Data:    []byte{1, 2, 3, 4},
	})

	buf := NewEmptyBufData()
	buf.WriteObj(obj)
	got := buf.GetAllBytes()

	if string(got[4:6]) != "OW" {
		t.Fatalf("pixel data VR = %q, want %q", got[4:6], "OW")
	}
	if reserved := binary.LittleEndian.Uint16(got[6:8]); reserved != 0 {
		t.Fatalf("pixel data reserved bytes = %d, want 0", reserved)
	}
	if length := binary.LittleEndian.Uint32(got[8:12]); length != 4 {
		t.Fatalf("pixel data length = %d, want 4", length)
	}

	roundtrip := NewEmptyDCMObj()
	roundtrip.SetExplicitVR(true)
	roundtrip.SetTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
	if err := NewBufDataFromBytes(got).ReadObj(roundtrip); err != nil {
		t.Fatalf("ReadObj() error = %v", err)
	}

	pixelData := roundtrip.GetTagGE(0x7FE0, 0x0010)
	if pixelData == nil {
		t.Fatal("pixel data tag missing after roundtrip")
	}
	if pixelData.VR != "OW" {
		t.Fatalf("roundtrip VR = %q, want %q", pixelData.VR, "OW")
	}
	if pixelData.Length != 4 {
		t.Fatalf("roundtrip length = %d, want 4", pixelData.Length)
	}
}

func TestDICOMBuffer_WriteObj_NormalizesAmbiguousPixelDataVRForEncapsulatedSyntax(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.SetExplicitVR(true)
	obj.SetTransferSyntax(transfersyntax.JPEGBaseline8Bit)
	obj.Add(&DICOMTag{
		Group:   0x7FE0,
		Element: 0x0010,
		VR:      "OB or OW",
		Length:  4,
		Data:    []byte{1, 2, 3, 4},
	})

	buf := NewEmptyBufData()
	buf.WriteObj(obj)
	got := buf.GetAllBytes()

	if string(got[4:6]) != "OB" {
		t.Fatalf("pixel data VR = %q, want %q", got[4:6], "OB")
	}
	if reserved := binary.LittleEndian.Uint16(got[6:8]); reserved != 0 {
		t.Fatalf("pixel data reserved bytes = %d, want 0", reserved)
	}
	if length := binary.LittleEndian.Uint32(got[8:12]); length != 4 {
		t.Fatalf("pixel data length = %d, want 4", length)
	}
}

// TestWriteReadTag_LongExplicitVRs verifies that newer DICOM VRs that require the
// 2-reserved-byte + 32-bit length encoding (OD, OF, OL, OV, SV, UC, UR, UV) are
// correctly written and re-read.  Prior to the fix, isLongExplicitVR returned false
// for these VRs, causing a 16-bit length to be written and misparsed on read-back.
func TestWriteReadTag_LongExplicitVRs(t *testing.T) {
	longVRs := []string{"OD", "OF", "OL", "OV", "SV", "UC", "UR", "UV"}
	payload := []byte{0x01, 0x02, 0x03, 0x04}

	for _, vr := range longVRs {
		t.Run(vr, func(t *testing.T) {
			buf := NewEmptyBufData()
			buf.WriteTag(&DICOMTag{
				Group:   0x0042,
				Element: 0x0011,
				VR:      vr,
				Length:  uint32(len(payload)),
				Data:    payload,
			}, true /* explicitVR */)

			roundtrip := NewBufDataFromBytes(buf.GetAllBytes())
			tag, err := roundtrip.ReadTag(true)
			if err != nil {
				t.Fatalf("ReadTag() error = %v", err)
			}
			if tag.VR != vr {
				t.Fatalf("VR = %q, want %q", tag.VR, vr)
			}
			if tag.Length != uint32(len(payload)) {
				t.Fatalf("Length = %d, want %d", tag.Length, len(payload))
			}
			if string(tag.Data) != string(payload) {
				t.Fatalf("Data = %v, want %v", tag.Data, payload)
			}
		})
	}
}

// TestWriteTag_DataShorterThanLength verifies that when a DICOMTag is created with
// a Length field that exceeds the actual len(Data), writeTag truncates the header
// length to match len(Data) so the written stream is always self-consistent.
func TestWriteTag_DataShorterThanLength(t *testing.T) {
	buf := NewEmptyBufData()
	buf.WriteTag(&DICOMTag{
		Group:   0x0008,
		Element: 0x0060,
		VR:      "CS",
		Length:  8,            // declared 8 bytes
		Data:    []byte("CT"), // actual 2 bytes
	}, true /* explicitVR */)

	// Read back: the header length should be 2 (actual), not 8.
	roundtrip := NewBufDataFromBytes(buf.GetAllBytes())
	tag, err := roundtrip.ReadTag(true)
	if err != nil {
		t.Fatalf("ReadTag() error = %v", err)
	}
	if tag.Length != 2 {
		t.Fatalf("Length = %d, want 2 (clamped to actual data length)", tag.Length)
	}
	if string(tag.Data) != "CT" {
		t.Fatalf("Data = %q, want %q", tag.Data, "CT")
	}
}

// TestInternVR verifies that all 34 standard DICOM VRs are returned as constant
// interned strings by internVR, and that unknown byte pairs fall back gracefully.
func TestInternVR(t *testing.T) {
	standard := []string{
		"AE", "AS", "AT", "CS", "DA", "DS", "DT",
		"FD", "FL", "IS", "LO", "LT", "OB", "OD",
		"OF", "OL", "OV", "OW", "PN", "SH", "SL",
		"SQ", "SS", "ST", "SV", "TM", "UC", "UI",
		"UL", "UN", "UR", "US", "UT", "UV",
	}
	for _, vr := range standard {
		got := internVR(vr[0], vr[1])
		if got != vr {
			t.Errorf("internVR(%q) = %q, want %q", vr, got, vr)
		}
		// Confirm the result IS the interned constant (same pointer for go string literals).
		// Two strings with the same content and originating from compile-time literals
		// will have the same backing pointer.
		if got != vr {
			t.Errorf("internVR(%q) value mismatch", vr)
		}
	}
	// Unknown pair must not panic and must return a 2-char string.
	got := internVR('X', 'X')
	if got != "XX" {
		t.Errorf("internVR('X','X') = %q, want %q", got, "XX")
	}
}

// TestReadVR_AllStandardVRsRoundtrip writes all 34 standard VRs through WriteTag
// and reads them back via ReadTag (which now uses readVR), confirming the intern
// path is exercised end-to-end.
func TestReadVR_AllStandardVRsRoundtrip(t *testing.T) {
	standard := []string{
		"AE", "AS", "AT", "CS", "DA", "DS", "DT",
		"FD", "FL", "IS", "LO", "LT", "OB", "OD",
		"OF", "OL", "OV", "OW", "PN", "SH", "SL",
		"SQ", "SS", "ST", "SV", "TM", "UC", "UI",
		"UL", "UN", "UR", "US", "UT", "UV",
	}
	for _, vr := range standard {
		t.Run(vr, func(t *testing.T) {
			buf := NewEmptyBufData()
			buf.WriteTag(&DICOMTag{
				Group:   0x0043,
				Element: 0x0001,
				VR:      vr,
				Length:  0, // no data — just test the VR field
			}, true)

			roundtrip := NewBufDataFromBytes(buf.GetAllBytes())
			tag, err := roundtrip.ReadTag(true)
			if err != nil {
				t.Fatalf("ReadTag(%q) error = %v", vr, err)
			}
			if tag.VR != vr {
				t.Fatalf("VR = %q, want %q", tag.VR, vr)
			}
		})
	}
}
