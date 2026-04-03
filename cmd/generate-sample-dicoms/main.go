package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
)

func init() {
	media.InitDict()
}

func main() {
	outputDir := flag.String("output", "samples", "Output directory for generated DICOM files")
	flag.Parse()

	// Ensure output directory exists
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	fmt.Printf("Generating synthetic DICOM files using io-dicom library...\n")
	fmt.Printf("Output directory: %s\n\n", *outputDir)

	generated := 0
	failed := 0

	// 1. Generate CT Image
	fmt.Print("  • Generating synthetic CT image... ")
	if err := generateCTImage(*outputDir); err != nil {
		fmt.Printf("✗ Error: %v\n", err)
		failed++
	} else {
		fmt.Println("✓")
		generated++
	}

	// 2. Generate MR Image
	fmt.Print("  • Generating synthetic MR image... ")
	if err := generateMRImage(*outputDir); err != nil {
		fmt.Printf("✗ Error: %v\n", err)
		failed++
	} else {
		fmt.Println("✓")
		generated++
	}

	// 3. Generate Ultrasound Image
	fmt.Print("  • Generating synthetic ultrasound image... ")
	if err := generateUltrasoundImage(*outputDir); err != nil {
		fmt.Printf("✗ Error: %v\n", err)
		failed++
	} else {
		fmt.Println("✓")
		generated++
	}

	// 4. Generate Color Image (CR)
	fmt.Print("  • Generating synthetic CR color image... ")
	if err := generateCRColorImage(*outputDir); err != nil {
		fmt.Printf("✗ Error: %v\n", err)
		failed++
	} else {
		fmt.Println("✓")
		generated++
	}

	fmt.Printf("\nGenerated %d synthetic DICOM files", generated)
	if failed > 0 {
		fmt.Printf(" (%d failed)", failed)
	}
	fmt.Println()
}

// generateCTImage creates a synthetic CT DICOM image
func generateCTImage(outDir string) error {
	ds := media.NewEmptyDCMObj()

	// Set transfer syntax (Implicit VR Little Endian)
	ts := transfersyntax.ImplicitVRLittleEndian
	ds.SetTransferSyntax(ts)

	// Patient Information
	ds.WriteString(tags.PatientName, "Test^CT")
	ds.WriteString(tags.PatientID, "CT001")

	// Study Information
	ds.WriteString(tags.StudyInstanceUID, "1.2.3.4.5")
	ds.WriteString(tags.SeriesInstanceUID, "1.2.3.4.5.1")
	ds.WriteString(tags.SOPInstanceUID, "1.2.3.4.5.1.1")
	ds.WriteString(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.2") // CT Image Storage

	// Dates and times
	now := time.Now()
	ds.WriteDate(tags.StudyDate, now)
	ds.WriteTime(tags.StudyTime, now)
	ds.WriteDate(tags.ContentDate, now)
	ds.WriteTime(tags.ContentTime, now)

	// Modality
	ds.WriteString(tags.Modality, "CT")

	// Image dimensions
	ds.WriteUint16(tags.Rows, 512)
	ds.WriteUint16(tags.Columns, 512)

	// Pixel representation
	ds.WriteUint16(tags.SamplesPerPixel, 1)
	ds.WriteString(tags.PhotometricInterpretation, "MONOCHROME2")
	ds.WriteUint16(tags.BitsAllocated, 16)
	ds.WriteUint16(tags.BitsStored, 12)
	ds.WriteUint16(tags.HighBit, 11)
	ds.WriteUint16(tags.PixelRepresentation, 0)

	// Create synthetic pixel data (gradient pattern)
	pixelData := make([]byte, 512*512*2)
	for i := 0; i < 512; i++ {
		for j := 0; j < 512; j++ {
			val := uint16((i + j) % 4096)
			offset := (i*512 + j) * 2
			binary.LittleEndian.PutUint16(pixelData[offset:], val)
		}
	}

	ds.WriteStringGE(tags.PixelData.Group, tags.PixelData.Element, "OW", string(pixelData))

	filename := fmt.Sprintf("%s/synthetic-ct.dcm", outDir)
	return ds.WriteToFile(filename)
}

// generateMRImage creates a synthetic MR DICOM image
func generateMRImage(outDir string) error {
	ds := media.NewEmptyDCMObj()

	// Set transfer syntax (Implicit VR Little Endian)
	ts := transfersyntax.ImplicitVRLittleEndian
	ds.SetTransferSyntax(ts)

	// Patient Information
	ds.WriteString(tags.PatientName, "Test^MR")
	ds.WriteString(tags.PatientID, "MR001")

	// Study Information
	ds.WriteString(tags.StudyInstanceUID, "1.2.3.4.5")
	ds.WriteString(tags.SeriesInstanceUID, "1.2.3.4.5.2")
	ds.WriteString(tags.SOPInstanceUID, "1.2.3.4.5.2.1")
	ds.WriteString(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.4") // MR Image Storage

	// Dates and times
	now := time.Now()
	ds.WriteDate(tags.StudyDate, now)
	ds.WriteTime(tags.StudyTime, now)
	ds.WriteDate(tags.ContentDate, now)
	ds.WriteTime(tags.ContentTime, now)

	// Modality
	ds.WriteString(tags.Modality, "MR")

	// MR-specific parameters
	ds.WriteString(tags.EchoTime, "0.1")
	ds.WriteString(tags.RepetitionTime, "0.5")
	ds.WriteString(tags.FlipAngle, "90")

	// Image dimensions
	ds.WriteUint16(tags.Rows, 256)
	ds.WriteUint16(tags.Columns, 256)

	// Pixel representation
	ds.WriteUint16(tags.SamplesPerPixel, 1)
	ds.WriteString(tags.PhotometricInterpretation, "MONOCHROME2")
	ds.WriteUint16(tags.BitsAllocated, 16)
	ds.WriteUint16(tags.BitsStored, 12)
	ds.WriteUint16(tags.HighBit, 11)
	ds.WriteUint16(tags.PixelRepresentation, 0)

	// Create synthetic pixel data (concentric circles pattern)
	pixelData := make([]byte, 256*256*2)
	for i := 0; i < 256; i++ {
		for j := 0; j < 256; j++ {
			di := float64(i - 128)
			dj := float64(j - 128)
			dist := di*di + dj*dj
			var val uint16
			if dist < 128*128 {
				val = uint16((1.0 - (dist / (128 * 128))) * 4096)
			}
			offset := (i*256 + j) * 2
			binary.LittleEndian.PutUint16(pixelData[offset:], val)
		}
	}

	ds.WriteStringGE(tags.PixelData.Group, tags.PixelData.Element, "OW", string(pixelData))

	filename := fmt.Sprintf("%s/synthetic-mr.dcm", outDir)
	return ds.WriteToFile(filename)
}

// generateUltrasoundImage creates a synthetic ultrasound DICOM image
func generateUltrasoundImage(outDir string) error {
	ds := media.NewEmptyDCMObj()

	// Set transfer syntax (Implicit VR Little Endian)
	ts := transfersyntax.ImplicitVRLittleEndian
	ds.SetTransferSyntax(ts)

	// Patient Information
	ds.WriteString(tags.PatientName, "Test^US")
	ds.WriteString(tags.PatientID, "US001")

	// Study Information
	ds.WriteString(tags.StudyInstanceUID, "1.2.3.4.5")
	ds.WriteString(tags.SeriesInstanceUID, "1.2.3.4.5.3")
	ds.WriteString(tags.SOPInstanceUID, "1.2.3.4.5.3.1")
	ds.WriteString(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.6.4") // Ultrasound Image Storage

	// Dates and times
	now := time.Now()
	ds.WriteDate(tags.StudyDate, now)
	ds.WriteTime(tags.StudyTime, now)
	ds.WriteDate(tags.ContentDate, now)
	ds.WriteTime(tags.ContentTime, now)

	// Modality
	ds.WriteString(tags.Modality, "US")

	// Image dimensions
	ds.WriteUint16(tags.Rows, 512)
	ds.WriteUint16(tags.Columns, 512)

	// Pixel representation (8-bit for US)
	ds.WriteUint16(tags.SamplesPerPixel, 1)
	ds.WriteString(tags.PhotometricInterpretation, "MONOCHROME2")
	ds.WriteUint16(tags.BitsAllocated, 8)
	ds.WriteUint16(tags.BitsStored, 8)
	ds.WriteUint16(tags.HighBit, 7)
	ds.WriteUint16(tags.PixelRepresentation, 0)

	// Create synthetic pixel data (speckle pattern)
	pixelData := make([]byte, 512*512)
	for i := 0; i < 512*512; i++ {
		// Pseudo-random speckle pattern
		pixelData[i] = byte((i * 7) % 256)
	}

	// Add some structure (horizontal lines)
	for i := 0; i < 512; i += 50 {
		for j := 0; j < 10 && i+j < 512; j++ {
			for k := 0; k < 512; k++ {
				val := pixelData[(i+j)*512+k]
				if val < 200 {
					pixelData[(i+j)*512+k] = 200
				}
			}
		}
	}

	ds.WriteStringGE(tags.PixelData.Group, tags.PixelData.Element, "OB", string(pixelData))

	filename := fmt.Sprintf("%s/synthetic-us.dcm", outDir)
	return ds.WriteToFile(filename)
}

// generateCRColorImage creates a synthetic CR color DICOM image
func generateCRColorImage(outDir string) error {
	ds := media.NewEmptyDCMObj()

	// Set transfer syntax (Implicit VR Little Endian)
	ts := transfersyntax.ImplicitVRLittleEndian
	ds.SetTransferSyntax(ts)

	// Patient Information
	ds.WriteString(tags.PatientName, "Test^CR")
	ds.WriteString(tags.PatientID, "CR001")

	// Study Information
	ds.WriteString(tags.StudyInstanceUID, "1.2.3.4.5")
	ds.WriteString(tags.SeriesInstanceUID, "1.2.3.4.5.4")
	ds.WriteString(tags.SOPInstanceUID, "1.2.3.4.5.4.1")
	ds.WriteString(tags.SOPClassUID, "1.2.840.10008.5.1.4.1.1.1.1") // CR Image Storage

	// Dates and times
	now := time.Now()
	ds.WriteDate(tags.StudyDate, now)
	ds.WriteTime(tags.StudyTime, now)
	ds.WriteDate(tags.ContentDate, now)
	ds.WriteTime(tags.ContentTime, now)

	// Modality
	ds.WriteString(tags.Modality, "CR")

	// Image dimensions
	ds.WriteUint16(tags.Rows, 256)
	ds.WriteUint16(tags.Columns, 256)

	// RGB Pixel representation
	ds.WriteUint16(tags.SamplesPerPixel, 3)
	ds.WriteString(tags.PhotometricInterpretation, "RGB")
	ds.WriteUint16(tags.BitsAllocated, 8)
	ds.WriteUint16(tags.BitsStored, 8)
	ds.WriteUint16(tags.HighBit, 7)
	ds.WriteUint16(tags.PixelRepresentation, 0)
	ds.WriteUint16(tags.PlanarConfiguration, 0) // Interleaved

	// Create synthetic RGB pixel data (color gradient)
	// Interleaved format: RGBRGBRGB...
	pixelData := make([]byte, 256*256*3)
	idx := 0
	for i := 0; i < 256; i++ {
		for j := 0; j < 256; j++ {
			pixelData[idx] = byte(i)     // Red
			pixelData[idx+1] = byte(128) // Green
			pixelData[idx+2] = byte(j)   // Blue
			idx += 3
		}
	}

	ds.WriteStringGE(tags.PixelData.Group, tags.PixelData.Element, "OB", string(pixelData))

	filename := fmt.Sprintf("%s/synthetic-cr-color.dcm", outDir)
	return ds.WriteToFile(filename)
}
