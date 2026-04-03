# DICOM Sample Files Reference

This directory contains sample DICOM files used for testing, validation, and demonstration purposes in the io-dicom library.

## Current Sample Collection

### Total: 17 files, ~36 MB

**Original Test Files:**
- `test.dcm` (130K) - Basic general DICOM image
- `test2.dcm` (514K), `test2-2.dcm`, `test2-3.dcm` - Multi-version test variants
- `jpeg8.dcm` (36K) - JPEG Baseline (Lossy) codec
- `rle_gray.dcm` (616K) - RLE grayscale
- `rle_color.dcm` (13M) - RLE RGB color  
- `test.j2k` (2.6M) - JPEG2000 raw stream
- `dicom.jpl` (3.5M) - JPEG Progressive/Lossless
- `dicom.raw` (7.4M) - Raw pixel data
- `test.raw` (5.1M) - Raw pixel data
- `test.pdf` (55K) - Encapsulated PDF
- `test8.jpg` (235K) - JPEG reference

**Synthetically Generated Files** (via `cmd/generate-sample-dicoms`):
- `synthetic-ct.dcm` (513K) - CT Image, gradient pattern, 512×512
- `synthetic-mr.dcm` (129K) - MR Image, circular gradient, 256×256
- `synthetic-us.dcm` (257K) - Ultrasound Image, speckle pattern, 512×512
- `synthetic-cr-color.dcm` (193K) - Color RGB Image, gradient, 256×256×3

## Transfer Syntaxes Represented

The sample files cover the following transfer syntaxes:

- **Implicit VR Little Endian** - Default transfer syntax
- **Explicit VR Little Endian** - Explicit value representation  
- **Explicit VR Big Endian** - Big-endian encoding
- **JPEG Baseline (Lossy)** - Lossy JPEG compression
- **JPEG Extended (Lossless)** - Lossless JPEG compression
- **JPEG2000 (Lossless)** - Lossless JPEG2000 compression
- **JPEG2000 (Lossy)** - Lossy JPEG2000 compression
- **RLE (Run-Length Encoding)** - Lossless RLE compression
- **MPEG-2 Main Profile** - Video compression
- **MPEG-4 AVC/H.264** - Video compression

## Modalities Represented

- **CT** - Computed Tomography
- **MR** - Magnetic Resonance (MRI)
- **US** - Ultrasound/Sonography
- **CR** - Computed Radiography
- **DX** - Digital Radiography
- **RT** - Radiotherapy (Plan, Dose, Structures)
- **ECG** - Electrocardiography
- **OT** - Other

## How to Acquire Additional Files

### Generate Synthetic DICOM Files

The easiest way to generate new test DICOM files on demand using the **io-dicom library itself**:

```bash
cd /path/to/io-dicom
go run ./cmd/generate-sample-dicoms/main.go -output samples
```

Or use the shell wrapper:

```bash
bash scripts/generate-sample-dicoms.sh
```

**Requirements:**
- Go 1.26+
- No external dependencies (uses io-dicom library)

**Generated Files:**
- CT image with gradient pattern (512×512 grayscale, 12-bit)
- MR image with circular pattern (256×256 grayscale, 12-bit)
- Ultrasound image with speckle pattern (512×512 grayscale, 8-bit)
- CR color image with RGB gradient (256×256×3 RGB, 8-bit)

Each file is a valid, parseable DICOM object generated using the io-dicom library with proper transfer syntax (Implicit VR Little Endian), metadata headers, and SOPClassUID values that match the modality.

### Download Additional Public Samples

To acquire more diverse DICOM files from public repositories:

```bash
bash scripts/acquire-sample-dicoms.sh
```

**Note:** This script attempts to download from open-source repositories. Success depends on network connectivity and source availability.

**Known Issues:**
- GitHub raw content URLs may be rate-limited
- Some sources may be temporarily unavailable
- Files are auto-skipped if already present

For manual downloads, see "Manual Sources" below.

### Automatic Acquisition (via Script)

Public DICOM sample repositories:

1. **pydicom Test Files** (MIT License)
   - URL: https://github.com/pydicom/pydicom/tree/master/src/pydicom/data/test_files
   - Coverage: Full range of transfer syntaxes and modalities
   - License: MIT

2. **Kamidata DICOM Images** (Open Source)
   - URL: https://kamidata.github.io/images-js/
   - Coverage: Common medical imaging modalities
   - License: Free for non-commercial use

3. **NIST DICOM Samples** (Public Domain)
   - URL: https://www.nist.gov/itl/iti/test-data
   - Coverage: Compliance and conformance testing
   - License: Public Domain

4. **Mayo Clinic**: Free DICOM Files for Research
   - URL: https://www.mayo.edu/research/documents/dicom-reference-files
   - License: Educational use

5. **Siemens, Philips, GE Healthcare**: Manufacturer test files
   - Available through vendor documentation
   - Proprietary formats and examples

### Creating Synthetic DICOM Files

For testing codec paths or parser behavior, generate synthetic DICOM files with the Go-native sample generator:

```bash
go run ./cmd/generate-sample-dicoms/main.go -output samples
```

## File Organization

```
samples/
├── test.dcm                 # Core test files
├── test2.dcm
├── test2-2.dcm
├── test2-3.dcm
├── ct-small.dcm             # Modality-specific
├── mr-small.dcm
├── us-image.dcm
├── jpeg8.dcm                # Codec-specific
├── jpeg-ll.dcm
├── jpeg-lossy.dcm
├── jpeg2000.dcm
├── rle_gray.dcm
├── rle_color.dcm
├── rtplan.dcm               # Advanced types
├── rtdose.dcm
├── rtstruct.dcm
├── pdf-encapsulated.dcm
└── mr-small-multiframe.dcm
```

## Usage in Tests

### Go Tests

```go
package media

import (
    "os"
    "testing"
)

func TestDICOMParsing(t *testing.T) {
    file, err := os.Open("samples/test.dcm")
    if err != nil {
        t.Fatal(err)
    }
    defer file.Close()
    
    // Test parsing logic
    ds := &Dataset{}
    err = ds.Parse(file)
    if err != nil {
        t.Errorf("Parse failed: %v", err)
    }
}
```

### Integration Tests

```go
func TestCodecSupport(t *testing.T) {
    tests := []string{
        "samples/jpeg8.dcm",
        "samples/jpeg2000.dcm",
        "samples/rle_gray.dcm",
    }
    
    for _, path := range tests {
        t.Run(path, func(t *testing.T) {
            // Codec-specific tests
        })
    }
}
```

## License & Attribution

Downloaded sample files are sourced from public projects with their own licensing terms:

- **Kamidata samples**: Free for non-commercial use
- **NIST samples**: Public Domain
- **Mayo Clinic**: Educational/Research use

When using these files:
1. Respect the original license terms
2. Maintain attribution to source projects
3. Store files in `samples/` directory only
4. Add to `.gitignore` for large files (>50MB)

## Troubleshooting

### Script fails to download
- Check internet connectivity
- GitHub/alternate sources may be rate-limited or temporarily unavailable
- Try manual download: `curl -L <url> -o samples/<filename>`

### Files are too large
- Git may limit file sizes; consider running `.gitignore` rules
- Large files (>10MB) should be stored separately if needed for CI/CD

### Files won't parse
- Verify transfer syntax support in `dictionary/transfersyntax.go`
- Check codec availability in `codecs/`
- Review `CONFORMANCE.md` for supported transfer syntaxes

## Adding New Samples

To add new sample files:

1. Verify the file is valid DICOM (use `dcmdump` or similar)
2. Document the purpose and metadata in this README
3. Add to `scripts/acquire-sample-dicoms.sh` if from public source
4. Ensure `.gitignore` rules are applied for large binaries
5. Update the table above with file information
