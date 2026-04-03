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
- `dimse/`: DIMSE command handlers (C-ECHO, C-FIND, C-GET, C-MOVE, C-STORE, N-service response helpers)
- `dictionary/`: DICOM tags, SOP classes, transfer syntaxes, coding schemes
- `codecs/jpeg/`: JPEG codec implementation and pure-Go fallback behavior
- `codecs/jpeg2000/`: JPEG2000 codec interface and pure-Go fallback behavior
- `transcoder/`: RLE and transfer pixel data transcoding helpers
- `database/`: sqlite-backed data access layer
- `wado/`: DICOMweb server and client (WADO-RS, STOW-RS, QIDO-RS)
- `utils/`, `uuids/`, `clients/`, `implementation/`: shared utilities and implementation metadata
- `samples/`: sample DICOM files used by tests and local validation

See `docs/project-structure.md` for package boundaries and maintenance conventions.

## DICOM Standards Alignment

Section-by-section standards alignment tracking lives in
`docs/dicom-standard-alignment-tracker.md`.

PS3.7 DIMSE section-level requirement mapping lives in
`docs/ps3.7-dimse-requirements-matrix.md`.

PS3.8 network/UL section-level requirement mapping lives in
`docs/ps3.8-network-requirements-matrix.md`.

PS3.8 UL association state-transition mapping lives in
`docs/ps3.8-ul-state-transition-matrix.md`.

Formal product-level conformance declaration lives in `CONFORMANCE.md`.

Transfer syntax implementation audit and deployment guidance lives in
`docs/transfer-syntax-coverage-audit.md`.

## DICOM Protocol Implementation

Implemented DIMSE protocol support (DICOM PS3.7):

- **C-ECHO** - Verification
- **C-STORE** - Storage
- **C-FIND** - Query
- **C-GET** - Query/Retrieve GET
- **C-MOVE** - Query/Retrieve MOVE (see [C-MOVE Implementation Guide](docs/dicom-cmove-implementation.md) for details)
- **N-service command response helpers** - N-EVENT-REPORT, N-GET, N-SET, N-ACTION, N-CREATE, N-DELETE response encoding helpers are available in `dimse/`

Notes:
- C-CANCEL command reception is parsed in SCP, tracked by message ID, and exposed through `OnCCancelRequest`.
- C-FIND, C-GET, and C-MOVE now use context-cancellable streaming handlers. SCP can emit pending responses while the handler is running and applies true in-flight cancel preemption by canceling handler context when a matching C-CANCEL arrives.
- Breaking API change: `OnCFindRequest`, `OnCGetRequest`, and `OnCMoveRequest` now require context-aware handler signatures with emit callbacks and structured final results.
- C-GET/C-MOVE progress counters are validated for deterministic monotonic progression (remaining non-increasing; completed/failed/warnings non-decreasing). Non-monotonic progress is terminated with processing-failure status.
- If a query/retrieve handler does not exit within the cancel grace window after matching C-CANCEL, SCP sends A-ABORT and closes the association.
- C-FIND status semantics are enforced: pending responses must include an identifier dataset, and final responses must not include one.
- Query/Retrieve operation-specific status validation is enforced for C-FIND/C-GET/C-MOVE response parsing and writing (including service-specific disallowed combinations such as C-STORE-only warning codes and C-MOVE-only refusal codes).
- Command writes select the negotiated presentation context by SOP class instead of relying on a single default context, so multiple accepted query/retrieve models can coexist on one association.
- Core DICOM status descriptions are available via `network/dicomstatus.Description(status)` with range-aware fallback for unmapped Axxx/Bxxx/Cxxx codes.
- C-STORE service-specific status-code class validation is enforced for response parsing and writing.
- C-STORE response field validation is enforced (`CommandDataSetType=0x0101` and non-zero `MessageIDBeingRespondedTo`).
- C-ECHO response field validation is enforced (`CommandDataSetType=0x0101` and non-zero `MessageIDBeingRespondedTo`).
- DICOM file writes now validate required output prerequisites before emitting file meta information: transfer syntax, SOP Class UID, and SOP Instance UID must all be present.
- For detailed C-MOVE usage patterns, error handling, and compliance information, see the [C-MOVE Implementation Guide](docs/dicom-cmove-implementation.md).

## DICOM Network Conformance Notes

- AE titles are encoded as fixed 16-byte, space-padded fields. Internal spaces are preserved while only trailing padding is trimmed when read back.
- Association presentation context negotiation now prefers `Explicit VR Little Endian`, then `Implicit VR Little Endian`, then `Explicit VR Big Endian` when multiple offered transfer syntaxes are known.
- Association presentation context negotiation accepts only transfer syntaxes in the supported transfer syntax contract (`dictionary/transfersyntax.SupportedTransferSyntax`).
- Association accept handling selects a default presentation context from accepted contexts using the same preference order and falls back to any accepted context when needed.
- Presentation context rejection reasons are now explicit: result code `3` for unsupported abstract syntax and `4` for unsupported transfer syntaxes.
- Rejecting an incoming A-ASSOCIATE-RQ closes the transport connection immediately after sending A-ASSOCIATE-RJ, conforming to DICOM PS 3.8 §9.3.4.
- A-ASSOCIATE-RJ reason text decoding is aligned to PS3.8 source/reason tables (UL service-user, ACSE provider, presentation provider), with deterministic `No reason given` fallback for unknown combinations.
- TLS 1.2+ is enforced for both the SCP listener (`NewSCPWithTLS`) and SCU outbound connections (`Destination.IsTLS` + `Destination.TLSConfig`). Pure-Go builds with no `crypto/tls` overhead remain the default when `IsTLS` is false.
- For TLS SCP associations, `network.AssociationRequest` now carries the negotiated peer certificate chain (`GetPeerCertificates`) so applications can attribute inbound clients by mTLS certificate identity.

## Query/Retrieve Identifier Validation

- SCP validates `QueryRetrieveLevel` for C-FIND, C-GET, and C-MOVE requests.
- Accepted levels are `PATIENT`, `STUDY`, `SERIES`, `IMAGE`, and `FRAME`.
- Invalid levels return `0xA900` (`FailureIdentifierDoesNotMatchSOPClass`) before handler execution.

## Breaking Changes

Latest architecture cleanup includes package path renames.

## No-CGO Codec Support

- Pure-Go supported:
  - JPEG Baseline 8-bit encode/decode (`codecs/jpeg`)
  - JPEG 12-bit / 16-bit passthrough encode helpers (`codecs/jpeg`)
  - RLE Lossless encode/decode (`transcoder`)
  - JPEG-LS passthrough encode helpers (`codecs/jpegls`)
  - JPEG XL passthrough encode helpers (`codecs/jpegxl`)
  - JPEG 2000 / HTJ2K passthrough encode helpers (`codecs/jpeg2000`)
  - SMPTE ST 2110 passthrough encode helpers (`codecs/smpte2110`)
  - JPIP HTJ2K passthrough encode helpers (`codecs/jpip`)
  - MPEG-2 / MPEG-4 AVC / HEVC passthrough encode helpers (`codecs/mpeg`)

- Decode behavior without the matching native backend now fails explicitly instead of returning compressed payload bytes as if they were decoded pixels. Build with the matching tag (`libjpeg`, `openjpeg`, `charls`, `libjxl`, `openjph`, `ffmpeg`, `st2110`) for production decode/transcode paths.

## Codec-Backed Transfer Syntaxes

The library's `SupportedTransferSyntax` set only includes non-retired transfer syntaxes
that are wired to implemented codec/transcode paths.

See `docs/transfer-syntax-support-matrix.md` for the current support contract,
including which syntaxes are native, shared-family, or intentionally unsupported.
The support contract is enforced by conformance tests in both
`dictionary/transfersyntax` and `media`, and representative behavioral
roundtrip tests in `media` validate dataset and pixel-data routing through
the supported syntax families.

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
- Builds that register exactly one native backend for a codec family now select it automatically during package initialization.
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
- The root `codecs` package also exposes `AvailableTransferSyntaxUIDs` and
  `ResolveBackendForUID` to inspect transfer syntax routing without duplicating
  per-family UID switches.
- The root `codecs` package also exposes `NativeDefaults`, `UseNativeDefaults`,
  `ValidateBackends`, and `ValidateCurrentBackends` so applications can fail fast
  during startup when required native tooling is unavailable.
- A real JPEG-LS backend can be registered without changing call sites in `media/dicom_object.go`.

Applications that need request-scoped cancellation for transcode operations can call
`ChangeTransferSyntaxContext` to propagate `context.Context` through native codec execution.

### Tagged Backend Validation

- Run all build-tagged codec backend tests locally:

```bash
./tools/test_codec_tags.sh
```

- Run only the media native-backend representative roundtrip check for a specific tag:

```bash
go test -tags openjpeg ./media -run TestRepresentativePixelTransferSyntaxRoundTripsWithNativeBackends
```

- Equivalent Make targets:

```bash
make deps-from-source
make build-native
make test-tags
make transfer-syntax-matrix
make contract-check
```

- `make build-native` runs source dependency build into a workspace-local prefix,
  compiles with all native codec tags enabled, and executes `make contract-check`.

- `make contract-check` runs transfer syntax doc generation, targeted conformance/media tests,
  tagged codec backend tests, and the full untagged suite in one command.

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
- CI caches both `$GITHUB_WORKSPACE/.local/codec-deps` and
  `$GITHUB_WORKSPACE/.build/codec-deps` keyed from `tools/build_codec_deps_from_source.sh`
  to avoid unnecessary source rebuilds when dependency definitions do not change.
- Source dependency builds explicitly disable CMake tests (including GTest lookup)
  to keep CI deterministic and avoid test-only third-party requirements.
- The source-build script also bootstraps libjxl's `third_party` tree via
  `deps.sh`, because GitHub release archives do not include those fetched
  dependencies by default.
- The libjxl source-build step also patches vendored `sjpeg` CMake metadata and
  sets an explicit policy minimum so it continues to configure under newer
  CMake releases.

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

### Send C-Get Request
```golang
request := utils.DefaultCMoveRequest(studyUID) // same Q/R identifier structure

scu := services.NewSCU(destination)
status, err := scu.GetSCU(request, 0)
if err != nil {
  log.Fatalln(err)
}
log.Printf("C-GET final status: 0x%04X", status)
```

### Start a TLS-enabled SCP Server

```golang
import "crypto/tls"

cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
if err != nil {
  log.Fatal(err)
}
tlsCfg := &tls.Config{
  Certificates: []tls.Certificate{cert},
  MinVersion:   tls.VersionTLS12,
}

scp := services.NewSCPWithTLS(port, tlsCfg)
// Register handlers exactly as with NewSCP …
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
if err := scp.Start(ctx); err != nil {
  log.Fatal(err)
}
```

### Connect an SCU over TLS

Set `IsTLS: true` and supply a `*tls.Config` in the `Destination`. The SCU
calls `ConnectTLS` automatically when the flag is set.

```golang
pool, _ := x509.SystemCertPool()
tlsCfg := &tls.Config{
  RootCAs:    pool,
  ServerName: "dicom.example.com",
  MinVersion: tls.VersionTLS12,
}

destination := &network.Destination{
  HostName:  "dicom.example.com",
  CalledAE:  "REMOTE_AE",
  CallingAE: "MY_AE",
  Port:      1043,
  IsTLS:     true,
  TLSConfig: tlsCfg,
}

scu := services.NewSCU(destination)
if err := scu.EchoSCU(30); err != nil {
  log.Fatal(err)
}
```

### Start SCP Server
```golang
scp := services.NewSCP(*port)

scp.OnAssociationRequest(func(request network.AssociationRequest) bool {
  called := request.GetCalledAE()
  return *calledAE == called
})

scp.OnCFindRequest(func(ctx context.Context, request network.AssociationRequest, queryLevel string, query media.DICOMObject, emit func(media.DICOMObject)) (services.CFindResult, error) {
  query.DumpTags()
  for i := 0; i < 10; i++ {
    emit(utils.GenerateCFindRequest())
  }
  return services.CFindResult{Status: dicomstatus.Success}, nil
})

scp.OnCGetRequest(func(ctx context.Context, request network.AssociationRequest, getLevel string, query media.DICOMObject, emit func(services.CGetProgress)) (services.CGetResult, error) {
  emit(services.CGetProgress{Remaining: 1, Completed: 0, Failed: 0, Warnings: 0})
  return services.CGetResult{Status: dicomstatus.Success, Remaining: 0, Completed: 1, Failed: 0, Warnings: 0}, nil
})

scp.OnCMoveRequest(func(ctx context.Context, request network.AssociationRequest, moveDestAE string, moveLevel string, query media.DICOMObject, emit func(services.CMoveProgress)) (services.CMoveResult, error) {
  query.DumpTags()
  return services.CMoveResult{Status: dicomstatus.Success}, nil
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

ctx, cancel := context.WithCancel(context.Background())
defer cancel()
if err := scp.Start(ctx); err != nil {
  log.Fatal(err)
}
```

### Start a DICOMweb (WADO-RS / STOW-RS / QIDO-RS) Server

```golang
// Implement wado.Store with your storage backend.
type myStore struct{}

func (s *myStore) RetrieveStudy(ctx context.Context, studyUID string) ([]media.DICOMObject, error) { ... }
func (s *myStore) RetrieveSeries(ctx context.Context, studyUID, seriesUID string) ([]media.DICOMObject, error) { ... }
func (s *myStore) RetrieveInstance(ctx context.Context, studyUID, seriesUID, sopUID string) (media.DICOMObject, error) { ... }
func (s *myStore) StoreInstances(ctx context.Context, objs []media.DICOMObject) error { ... }
func (s *myStore) SearchStudies(ctx context.Context, q url.Values) ([]media.DICOMObject, error) { ... }
func (s *myStore) SearchSeries(ctx context.Context, studyUID string, q url.Values) ([]media.DICOMObject, error) { ... }
func (s *myStore) SearchInstances(ctx context.Context, studyUID, seriesUID string, q url.Values) ([]media.DICOMObject, error) { ... }

srv := wado.NewServer(wado.ServerParams{
  Port:  8080,
  Store: &myStore{},
})

ctx, cancel := context.WithCancel(context.Background())
defer cancel()
if err := srv.Start(ctx); err != nil {
  log.Fatal(err)
}
```

**Routes registered:**

| Method | Path | Service |
|--------|------|---------|
| `GET` | `/wado/rs/studies/{studyUID}` | WADO-RS retrieve study |
| `GET` | `/wado/rs/studies/{studyUID}/series/{seriesUID}` | WADO-RS retrieve series |
| `GET` | `/wado/rs/studies/{studyUID}/series/{seriesUID}/instances/{sopInstanceUID}` | WADO-RS retrieve instance |
| `GET` | `/wado/rs/studies/{studyUID}/metadata` | WADO-RS study metadata (JSON) |
| `GET` | `/wado/rs/studies/{studyUID}/series/{seriesUID}/metadata` | WADO-RS series metadata (JSON) |
| `GET` | `/wado/rs/studies/{studyUID}/series/{seriesUID}/instances/{sopInstanceUID}/metadata` | WADO-RS instance metadata (JSON) |
| `GET` | `/wado/rs/studies/{studyUID}/series/{seriesUID}/instances/{sopInstanceUID}/frames/{frames}` | WADO-RS frame retrieval |
| `POST` | `/stow/rs/studies` | STOW-RS store instances |
| `POST` | `/stow/rs/studies/{studyUID}` | STOW-RS store into study |
| `GET` | `/qido/rs/studies` | QIDO-RS search studies |
| `GET` | `/qido/rs/studies/{studyUID}/series` | QIDO-RS search series |
| `GET` | `/qido/rs/studies/{studyUID}/series/{seriesUID}/instances` | QIDO-RS search instances |

### Use the DICOMweb Client

```golang
client := wado.NewClient(wado.ClientParams{
  BaseURL: "https://dicom.example.com",
  Timeout: 30 * time.Second,
})

// WADO-RS: retrieve all instances in a study
objects, err := client.RetrieveStudy(ctx, studyUID)

// WADO-RS: retrieve a single instance
obj, err := client.RetrieveInstance(ctx, studyUID, seriesUID, sopInstanceUID)

// WADO-RS: retrieve instance metadata as DICOMweb JSON
meta, err := client.RetrieveMetadata(ctx, studyUID, seriesUID, sopInstanceUID)

// STOW-RS: send instances to the server
err = client.StoreInstances(ctx, studyUID, []media.DICOMObject{obj})

// QIDO-RS: search for studies
results, err := client.SearchStudies(ctx, url.Values{"PatientName": []string{"SMITH"}})
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

## Refresh Transfer Syntax Support Docs

The transfer syntax support docs are generated from
`dictionary/transfersyntax.ConformanceMatrix`.

This regenerates:

- `docs/transfer-syntax-support-matrix.md`
- `docs/transfer-syntax-behavioral-summary.md`

Regenerate it with:

```bash
make transfer-syntax-matrix
go test ./...
```