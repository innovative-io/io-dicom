# io-dicom

Innovative IO DICOM Golang Library

## Compatibility

- Go 1.26+
- No `cgo` required. The repository contains no C source code.
- Pure-Go SQLite driver (`modernc.org/sqlite`).

## Project Structure

- `cmd/`: executable entrypoints
  - `cmd/io-dicom/`: main CLI client/server utility
  - `cmd/compare/`: DICOM metadata comparison utility
  - `cmd/utilities/`: helper binary for utility workflows
- `media/`: DICOM object model, parsing, encoding, and pixel pipeline orchestration
- `network/`: DICOM network protocol data units and association primitives
- `services/`: SCU/SCP high-level service APIs
- `dimse/`: DIMSE command handlers (C-ECHO, C-FIND, C-MOVE, C-STORE)
- `dictionary/`: DICOM tags, SOP classes, transfer syntaxes, coding schemes
- `codecs/jpeg/`: JPEG codec implementation and pure-Go fallback behavior
- `codecs/jpeg2000/`: JPEG2000 codec interface and pure-Go fallback behavior
- `transcoder/`: RLE and transfer pixel data transcoding helpers
- `database/`: sqlite-backed data access layer
- `utils/`, `uuids/`, `clients/`, `implementation/`: shared utilities and implementation metadata
- `samples/`: sample DICOM files used by tests and local validation

See `docs/project-structure.md` for package boundaries and maintenance conventions.

## Breaking Changes

Latest architecture cleanup includes package path renames.

See `docs/migration-vNext.md` for full upgrade steps and bulk rewrite examples.

## No-CGO Codec Support

- Pure-Go supported:
  - JPEG Baseline 8-bit encode/decode (`codecs/jpeg`)
  - JPEG 12-bit / 16-bit passthrough encode/decode helpers (`codecs/jpeg`)
  - RLE Lossless encode/decode (`transcoder`)
  - JPEG-LS passthrough encode/decode (`codecs/jpegls`)
  - JPEG XL passthrough encode/decode (`codecs/jpegxl`)
  - JPEG 2000 / HTJ2K passthrough encode/decode (`codecs/jpeg2000`)
  - SMPTE ST 2110 passthrough encode/decode (`codecs/smpte2110`)
  - JPIP HTJ2K passthrough encode/decode (`codecs/jpip`)
  - MPEG-2 / MPEG-4 AVC / HEVC passthrough encode/decode (`codecs/mpeg`)

## Codec-Backed Transfer Syntaxes

The library's `SupportedTransferSyntax` set only includes non-retired transfer syntaxes
that are wired to implemented codec/transcode paths.

See `docs/transfer-syntax-support-matrix.md` for the current support contract,
including which syntaxes are native, shared-family, or intentionally unsupported.
The support contract is enforced by conformance tests in both
`dictionary/transfersyntax` and `media`.

- JPEG family:
  - `1.2.840.10008.1.2.4.50` (JPEG Baseline)
  - `1.2.840.10008.1.2.4.51` (JPEG Extended 12-bit)
  - `1.2.840.10008.1.2.4.57` (JPEG Lossless, Process 14)
  - `1.2.840.10008.1.2.4.70` (JPEG Lossless, SV1)
- JPEG 2000 family:
  - `1.2.840.10008.1.2.4.90` (JPEG 2000 Lossless)
  - `1.2.840.10008.1.2.4.91` (JPEG 2000)
  - `1.2.840.10008.1.2.4.92` (JPEG 2000 Part 2 Multi-component Lossless)
  - `1.2.840.10008.1.2.4.93` (JPEG 2000 Part 2 Multi-component)
  - `1.2.840.10008.1.2.4.201` (HTJ2K Lossless)
  - `1.2.840.10008.1.2.4.202` (HTJ2K Lossless RPCL)
  - `1.2.840.10008.1.2.4.203` (HTJ2K)
  - `1.2.840.10008.1.2.4.204` (JPIP HTJ2K Referenced)
  - `1.2.840.10008.1.2.4.205` (JPIP HTJ2K Referenced Deflate)
- JPEG-LS family:
  - `1.2.840.10008.1.2.4.80` (JPEG-LS Lossless)
  - `1.2.840.10008.1.2.4.81` (JPEG-LS Near-Lossless)
- JPEG XL family:
  - `1.2.840.10008.1.2.4.110` (JPEG XL Lossless)
  - `1.2.840.10008.1.2.4.111` (JPEG XL JPEG Recompression)
  - `1.2.840.10008.1.2.4.112` (JPEG XL)
- MPEG/HEVC family:
  - `1.2.840.10008.1.2.4.100` (MPEG2 Main Profile / Main Level)
  - `1.2.840.10008.1.2.4.100.1` (Fragmentable MPEG2 Main Profile / Main Level)
  - `1.2.840.10008.1.2.4.101` (MPEG2 Main Profile / High Level)
  - `1.2.840.10008.1.2.4.101.1` (Fragmentable MPEG2 Main Profile / High Level)
  - `1.2.840.10008.1.2.4.102` (MPEG-4 AVC/H.264 High Profile / Level 4.1)
  - `1.2.840.10008.1.2.4.102.1` (Fragmentable MPEG-4 AVC/H.264 High Profile / Level 4.1)
  - `1.2.840.10008.1.2.4.103` (MPEG-4 AVC/H.264 BD-compatible High Profile / Level 4.1)
  - `1.2.840.10008.1.2.4.103.1` (Fragmentable MPEG-4 AVC/H.264 BD-compatible High Profile / Level 4.1)
  - `1.2.840.10008.1.2.4.104` (MPEG-4 AVC/H.264 High Profile / Level 4.2 for 2D Video)
  - `1.2.840.10008.1.2.4.104.1` (Fragmentable MPEG-4 AVC/H.264 High Profile / Level 4.2 for 2D Video)
  - `1.2.840.10008.1.2.4.105` (MPEG-4 AVC/H.264 High Profile / Level 4.2 for 3D Video)
  - `1.2.840.10008.1.2.4.105.1` (Fragmentable MPEG-4 AVC/H.264 High Profile / Level 4.2 for 3D Video)
  - `1.2.840.10008.1.2.4.106` (MPEG-4 AVC/H.264 Stereo High Profile / Level 4.2)
  - `1.2.840.10008.1.2.4.106.1` (Fragmentable MPEG-4 AVC/H.264 Stereo High Profile / Level 4.2)
  - `1.2.840.10008.1.2.4.107` (HEVC/H.265 Main Profile / Level 5.1)
  - `1.2.840.10008.1.2.4.108` (HEVC/H.265 Main 10 Profile / Level 5.1)
- SMPTE ST 2110 family:
  - `1.2.840.10008.1.2.7.1` (SMPTE ST 2110-20 Uncompressed Progressive Active Video)
  - `1.2.840.10008.1.2.7.2` (SMPTE ST 2110-20 Uncompressed Interlaced Active Video)
  - `1.2.840.10008.1.2.7.3` (SMPTE ST 2110-30 PCM Digital Audio)
- Other transcode paths:
  - RLE Lossless, Deflated Explicit VR Little Endian,
    Deflated Image Frame Compression,
    Encapsulated Uncompressed Explicit VR Little Endian,
    Implicit/Explicit VR Little Endian.

## Codec Package Layout

- Canonical codec packages:
  - `codecs/jpeg`
  - `codecs/jpeg2000`
  - `codecs/jpegls`
  - `codecs/jpegxl`
  - `codecs/jpip`
  - `codecs/mpeg`
  - `codecs/smpte2110`

## Codec Backend Integration

- `codecs/jpeg` now supports pluggable backends for 12/16-bit paths via `SetBackend`.
- `codecs/jpegls` now supports pluggable backends via `SetBackend`.
- `codecs/jpeg2000` now supports pluggable backends via `SetBackend`.
- `codecs/jpegxl` now supports pluggable backends via `SetBackend`.
- `codecs/mpeg` now supports pluggable backends via `SetBackend`.
- `codecs/jpip` now supports pluggable backends via `SetBackend`.
- `codecs/smpte2110` now supports pluggable backends via `SetBackend`.
- Default behavior remains pure-Go passthrough for no-cgo environments.
- `codecs/jpeg` also supports named backend registration and selection for
  12/16-bit paths via `RegisterBackend`, `UseBackend`, and `AvailableBackends`.
- `codecs/jpeg` now includes a build-tagged `libjpeg` backend registration path
  for 12/16-bit profiles:
  - default builds keep pure-Go passthrough (`CGOEnabled == false`),
  - `-tags libjpeg` with cgo enables `CGOEnabled == true` and registers backend name `libjpeg`.
  - the `libjpeg` backend now uses `cjpeg`/`djpeg` lossless paths for 12/16-bit
    encode/decode behavior when tools and precision support are available in `PATH`,
    with passthrough-compatible fallback semantics otherwise.
  - prerequisites for tagged builds:
    - libjpeg command-line tools must be installed (`cjpeg`, `djpeg`).
- `codecs/jpegls` also supports named backend registration and selection via
  `RegisterBackend`, `UseBackend`, and `AvailableBackends`.
- `codecs/jpegls` now includes a build-tagged `charls` backend registration path:
  - default builds keep pure-Go passthrough (`CGOEnabled == false`),
  - `-tags charls` with cgo enables `CGOEnabled == true` and registers backend name `charls`.
  - the `charls` backend now bridges to the CharLS C API for encode/decode when
    CharLS is available in the build environment.
  - prerequisites for tagged builds:
    - `pkg-config` must be available,
    - CharLS development package must be installed and discoverable via `PKG_CONFIG_PATH` (providing `charls.pc`).
- `codecs/jpeg2000` also supports named backend registration and selection via
  `RegisterBackend`, `UseBackend`, and `AvailableBackends`.
- `codecs/jpeg2000` now includes a build-tagged `openjpeg` backend registration path:
  - default builds keep pure-Go passthrough (`CGOEnabled == false`),
  - `-tags openjpeg` with cgo enables `CGOEnabled == true` and registers backend name `openjpeg`.
  - the `openjpeg` backend now uses OpenJPEG command-line tools (`opj_compress` and
    `opj_decompress`) for encode/decode when available in `PATH`.
  - prerequisites for tagged builds:
    - OpenJPEG CLI tools must be installed (`opj_compress`, `opj_decompress`).
- `codecs/jpegxl` also supports named backend registration and selection via
  `RegisterBackend`, `UseBackend`, and `AvailableBackends`.
- `codecs/jpegxl` now includes a build-tagged `libjxl` backend registration path:
  - default builds keep pure-Go passthrough (`CGOEnabled == false`),
  - `-tags libjxl` with cgo enables `CGOEnabled == true` and registers backend name `libjxl`.
  - the `libjxl` backend now uses libjxl command-line tools (`cjxl` and `djxl`)
    for encode/decode when available in `PATH`.
  - prerequisites for tagged builds:
    - libjxl CLI tools must be installed (`cjxl`, `djxl`).
- `codecs/mpeg` also supports named backend registration and selection via
  `RegisterBackend`, `UseBackend`, and `AvailableBackends`.
- `codecs/mpeg` now includes a build-tagged `ffmpeg` backend registration path:
  - default builds keep pure-Go passthrough (`CGOEnabled == false`),
  - `-tags ffmpeg` with cgo enables `CGOEnabled == true` and registers backend name `ffmpeg`.
  - the `ffmpeg` backend now uses FFmpeg command-line tools (`ffmpeg` and
    `ffprobe`) for encode/decode when available in `PATH`.
  - prerequisites for tagged builds:
    - FFmpeg CLI tools must be installed (`ffmpeg`, `ffprobe`).
- `codecs/jpip` also supports named backend registration and selection via
  `RegisterBackend`, `UseBackend`, and `AvailableBackends`.
- `codecs/jpip` now includes a build-tagged `openjph` backend registration path:
  - default builds keep pure-Go passthrough (`CGOEnabled == false`),
  - `-tags openjph` with cgo enables `CGOEnabled == true` and registers backend name `openjph`.
  - the `openjph` backend now uses OpenJPH tools (`ojph_compress`,
    `ojph_decompress`) when available, and falls back to compatible OpenJPEG tools
    (`opj_compress`, `opj_decompress`) for encode/decode.
  - prerequisites for tagged builds:
    - OpenJPH CLI tools recommended (`ojph_compress`, `ojph_decompress`), or
    - OpenJPEG CLI tools (`opj_compress`, `opj_decompress`).
- `codecs/smpte2110` also supports named backend registration and selection via
  `RegisterBackend`, `UseBackend`, and `AvailableBackends`.
- `codecs/smpte2110` now includes a build-tagged `st2110` backend registration path:
  - default builds keep pure-Go passthrough (`CGOEnabled == false`),
  - `-tags st2110` with cgo enables `CGOEnabled == true` and registers backend name `st2110`.
  - the `st2110` backend now uses FFmpeg command-line tools (`ffmpeg` and
    `ffprobe`) to encode/decode frame payloads when available in `PATH`.
  - prerequisites for tagged builds:
    - FFmpeg CLI tools must be installed (`ffmpeg`, `ffprobe`).
- The root `codecs` package exposes a central manager with `UseBackends` and
  `AvailableBackends` to configure all codec families from one call.
- A real JPEG-LS backend can be registered without changing call sites in `media/dicom_object.go`.

### Tagged Backend Validation

- Run all build-tagged codec backend tests locally:

```bash
./tools/test_codec_tags.sh
```

- Equivalent Make targets:

```bash
make deps-from-source
make test-tags
```

- Build codec dependencies from source for the current target system (installs to
  `$HOME/.local/codec-deps` by default):

```bash
./tools/build_codec_deps_from_source.sh
```

- This produces a single shared-library directory at `$PREFIX/lib` and writes
  `$PREFIX/env.sh` with `PKG_CONFIG_PATH`, `CGO_CFLAGS`, `CGO_LDFLAGS`, and
  runtime library path exports so cgo loads libraries from that location.

- All cgo-tagged codec backends now auto-initialize native dependency discovery
  from `CODEC_DEPS_PREFIX` (or `$HOME/.local/codec-deps` as fallback), so one
  shared source-built prefix can be used across all codec families.

```bash
source "$HOME/.local/codec-deps/env.sh"
```

- For custom install location / parallelism:

```bash
PREFIX=$PWD/.local/codec-deps JOBS=8 ./tools/build_codec_deps_from_source.sh
```

- CI workflow is provided at `.github/workflows/codec-tagged-tests.yml` and
  now builds codec dependencies from source before running untagged and tagged
  codec suites on each push/PR.

## Install

```bash
go get -u github.com/innovative-io/io-dicom
```

## Performance Benchmarks

- DICOM load/read benchmarks (file and bytes):

```bash
go test ./media -run '^$' -bench 'BenchmarkNewDCMObjFrom(File|Bytes)_' -benchmem
```

- DICOM write/serialize benchmarks:

```bash
go test ./media -run '^$' -bench 'BenchmarkWriteToBytes_' -benchmem
```

## Usage

### Load DICOM File

```golang
obj, err := media.NewDCMObjFromFile(fileName)
if err != nil {
  log.Panicln(err)
}
obj.DumpTags()
```

### Send C-Echo Request
```golang
scu := services.NewSCU(destination)
err := scu.EchoSCU(0)
if err != nil {
  log.Fatalln(err)
}
log.Println("CEcho was successful")
```

### Send C-Find Request
```golang
request := utils.DefaultCFindRequest()
scu := services.NewSCU(destination)
scu.SetOnCFindResult(func(result media.DICOMObject) {
  log.Printf("Found study %s\n", result.GetString(tags.StudyInstanceUID))
  result.DumpTags()
})

count, status, err := scu.FindSCU(request, 0)
if err != nil {
  log.Fatalln(err)
}

log.Printf("Found %d results, final status: 0x%04X", count, status)
```

### Send C-Store Request
```golang
scu := services.NewSCU(destination)
err := scu.StoreSCU(fileName, 0)
if err != nil {
  log.Fatalln(err)
}
```

### Change Transfer Syntax
```golang
err := obj.ChangeTransferSyntax(transfersyntax.ExplicitVRLittleEndian)
if err != nil {
  log.Fatalln(err)
}
```

### Send C-Move Request
```golang
request := utils.DefaultCMoveRequest(studyUID)

scu := services.NewSCU(destination)
_, err := scu.MoveSCU(destinationAE, request, 0)
if err != nil {
  log.Fatalln(err)
}
```

### Start SCP Server
```golang
scp := services.NewSCP(*port)

scp.OnAssociationRequest(func(request network.AssociationRequest) bool {
  called := request.GetCalledAE()
  return *calledAE == called
})

scp.OnCFindRequest(func(request network.AssociationRequest, queryLevel string, query media.DICOMObject) ([]media.DICOMObject, uint16) {
  query.DumpTags()
  results := make([]media.DICOMObject, 0)
  for i := 0; i < 10; i++ {
    results = append(results, utils.GenerateCFindRequest())
  }
  return results, dicomstatus.Success
})

scp.OnCMoveRequest(func(request network.AssociationRequest, moveLevel string, query media.DICOMObject) uint16 {
  query.DumpTags()
  return dicomstatus.Success
})

scp.OnCStoreRequest(func(request network.AssociationRequest, data media.DICOMObject) uint16 {
  log.Printf("INFO, C-Store recieved %s", data.GetString(tags.SOPInstanceUID))
  directory := filepath.Join(*datastore, data.GetString(tags.PatientID), data.GetString(tags.StudyInstanceUID), data.GetString(tags.SeriesInstanceUID))
  os.MkdirAll(directory, 0755)

  path := filepath.Join(directory, data.GetString(tags.SOPInstanceUID)+".dcm")

  err := data.WriteToFile(path)
  if err != nil {
    log.Printf("ERROR: There was an error saving %s : %s", path, err.Error())
  }
  return dicomstatus.Success
})

err := scp.Start()
if err != nil {
  log.Fatal(err)
}
```

## Test

```bash
go test ./...
```

## Refresh DICOM Dictionaries

The transfer syntax UID list and DICOM tag dictionary are generated from the latest
upstream pydicom DICOM data tables.

Regenerate both files with:

```bash
/usr/bin/python3 tools/update_dictionaries.py
gofmt -w dictionary/tags/dicom_tags.go dictionary/transfersyntax/transfer_syntaxes.go
go test ./...
```