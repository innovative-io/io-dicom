package media

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

// ErrNotDICOM is returned when a file does not contain a valid DICOM preamble.
var ErrNotDICOM = errors.New("media: not a DICOM file (DICM preamble missing)")

// ErrFileTooSmall is returned when the file or payload is smaller than the
// minimum required DICOM size (132 bytes: 128-byte preamble + 4-byte magic).
var ErrFileTooSmall = errors.New("media: file too small to be a valid DICOM file")

// IndexRecord holds the flat metadata extracted from a DICOM file.
// It contains exactly the fields needed to populate patient/study/series/instance
// database records, with no pixel data ever loaded.
type IndexRecord struct {
	FilePath                string
	TransferSyntaxUID       string
	MediaStorageSOPClassUID string

	PatientName      string
	PatientID        string
	PatientBirthDate string
	PatientSex       string

	AccessionNumber  string
	Modality         string
	StudyDate        string
	StudyTime        string
	StudyDescription string
	StudyInstanceUID string

	SeriesDate        string
	SeriesTime        string
	SeriesDescription string
	SeriesInstanceUID string
	SeriesNumber      string

	SOPClassUID    string
	SOPInstanceUID string
	InstanceNumber string
}

// indexReadLimit is the maximum bytes read from a file for metadata extraction.
// 128 KiB covers all DICOM metadata for any real-world modality with significant margin.
const indexReadLimit = 128 * 1024

var indexBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, indexReadLimit)
		return &b
	},
}

// ParseIndexFromFile reads only the first indexReadLimit bytes of a DICOM file,
// parses all patient/study/series/instance metadata tags (groups 0002–0020),
// and returns a populated IndexRecord. Pixel data is never loaded or decoded.
//
// Returns ErrNotDICOM if the file does not begin with a valid DICOM preamble.
func ParseIndexFromFile(path string) (*IndexRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bufPtr := indexBufPool.Get().(*[]byte)
	buf := *bufPtr
	defer indexBufPool.Put(bufPtr)

	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("media: ParseIndexFromFile %q: %w", path, err)
	}
	if n < 132 {
		return nil, fmt.Errorf("media: ParseIndexFromFile %q: %w (%d bytes)", path, ErrFileTooSmall, n)
	}

	rec := &IndexRecord{FilePath: path}
	if err := extractIndexTags(buf[:n], rec); err != nil {
		return nil, fmt.Errorf("media: ParseIndexFromFile %q: %w", path, err)
	}
	return rec, nil
}

// ParseIndexFromBytes extracts index metadata from an in-memory DICOM payload.
// Only the first indexReadLimit bytes are examined; pixel data is never decoded.
//
// Returns ErrNotDICOM if data does not begin with a valid DICOM preamble.
func ParseIndexFromBytes(data []byte) (*IndexRecord, error) {
	if len(data) < 132 {
		return nil, fmt.Errorf("media: ParseIndexFromBytes: %w (%d bytes)", ErrFileTooSmall, len(data))
	}
	limit := len(data)
	if limit > indexReadLimit {
		limit = indexReadLimit
	}
	rec := &IndexRecord{}
	if err := extractIndexTags(data[:limit], rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// extractIndexTags parses DICOM metadata from data and populates rec.
//
// Phase 1 reads group 0002 with the fixed LE/explicit encoding required by PS3.10 §10.1.
// Phase 2 reads the dataset with the encoding declared by the TransferSyntaxUID.
// This two-phase approach mirrors parseDICOMBuffer: group 0002 is ALWAYS little-endian,
// so we must not set BigEndian until after all group-0002 bytes are consumed.
//
// Parsing stops after tag (0020,0013) InstanceNumber — pixel data at (7FE0,0010)
// is never reached or decoded.
//
// Undefined-length SQ nesting is tracked with sqDepth so that values inside nested
// sequences are not mistakenly extracted as top-level fields.
// Defined-length SQ tags have their payload consumed by ReadTag (as raw Data bytes)
// and require no depth tracking.
func extractIndexTags(data []byte, rec *IndexRecord) error {
	if len(data) < 132 || string(data[128:132]) != "DICM" {
		return ErrNotDICOM
	}

	buf := &DICOMBuffer{
		data:     data,
		position: 132, // skip 128-byte preamble + "DICM" magic
		size:     len(data),
	}

	// Phase 1: Read group 0002 tags (always LE explicit VR, per PS3.10 §10.1).
	// We record prePos before each read and rewind when we see the first
	// non-group-0002 tag, so that tag is re-read in phase 2 with correct flags.
	// BigEndian is NOT set inside this loop — only after phase 1 completes.
	var (
		pendingBigEndian  bool
		implicitVRDataset bool
	)
	for buf.position < buf.size {
		prePos := buf.position
		tag, err := buf.ReadTag(true) // group 0002 is always explicit VR
		if err != nil || tag == nil {
			break
		}
		if tag.Group != 0x0002 {
			// Rewind: this first dataset tag must be read again in phase 2.
			buf.position = prePos
			break
		}
		switch tag.Element {
		case 0x0002:
			rec.MediaStorageSOPClassUID = tag.GetString()
		case 0x0010:
			uid := tag.GetString()
			rec.TransferSyntaxUID = uid
			switch uid {
			case transfersyntax.ImplicitVRLittleEndian.UID:
				implicitVRDataset = true
			case transfersyntax.ExplicitVRBigEndian.UID:
				pendingBigEndian = true
			}
		}
	}

	// Apply dataset encoding flags after all group-0002 bytes have been consumed.
	explicitVR := !implicitVRDataset
	if pendingBigEndian {
		buf.SetBigEndian(true)
	}

	// Phase 2: Read dataset tags with correct endianness until (0020,0013).
	sqDepth := 0
	for buf.position < buf.size {
		tag, err := buf.ReadTag(explicitVR)
		if err != nil {
			// Truncation at read-window boundary — treat as end of metadata.
			break
		}
		if tag == nil {
			break
		}

		// Handle item/sequence delimiter tags; manage undefined-length SQ depth.
		if tag.Group == 0xFFFE {
			if tag.Element == 0xE0DD && sqDepth > 0 {
				sqDepth--
			}
			continue
		}

		// Undefined-length SQ: subsequent flat tags belong to nested items until
		// the matching (FFFE,E0DD) sequence delimiter.
		if tag.VR == "SQ" && tag.Length == 0xFFFFFFFF {
			sqDepth++
			continue
		}

		if tag.Group > 0x0020 {
			break
		}
		if tag.Group == 0x0020 && tag.Element > 0x0013 {
			break
		}

		if sqDepth > 0 {
			continue
		}

		switch tag.Group {
		case 0x0008:
			switch tag.Element {
			case 0x0016:
				rec.SOPClassUID = tag.GetString()
			case 0x0018:
				rec.SOPInstanceUID = tag.GetString()
			case 0x0020:
				rec.StudyDate = tag.GetString()
			case 0x0021:
				rec.SeriesDate = tag.GetString()
			case 0x0030:
				rec.StudyTime = tag.GetString()
			case 0x0031:
				rec.SeriesTime = tag.GetString()
			case 0x0050:
				rec.AccessionNumber = tag.GetString()
			case 0x0060:
				rec.Modality = tag.GetString()
			case 0x1030:
				rec.StudyDescription = tag.GetString()
			case 0x103E:
				rec.SeriesDescription = tag.GetString()
			}
		case 0x0010:
			switch tag.Element {
			case 0x0010:
				rec.PatientName = tag.GetString()
			case 0x0020:
				rec.PatientID = tag.GetString()
			case 0x0030:
				rec.PatientBirthDate = tag.GetString()
			case 0x0040:
				rec.PatientSex = tag.GetString()
			}
		case 0x0020:
			switch tag.Element {
			case 0x000D:
				rec.StudyInstanceUID = tag.GetString()
			case 0x000E:
				rec.SeriesInstanceUID = tag.GetString()
			case 0x0011:
				rec.SeriesNumber = tag.GetString()
			case 0x0013:
				rec.InstanceNumber = tag.GetString()
			}
		}
	}

	return nil
}
