# Command Binaries

This folder contains executable entrypoints for the project.

## Binaries

- `cmd/io-dicom/`
  - Main CLI for SCU/SCP operations and DICOM file tasks.
- `cmd/compare/`
  - Compares metadata and values between two DICOM files.
- `cmd/utilities/`
  - Utility binary for helper workflows.

## Build

From repository root:

```bash
go build ./cmd/...
```

## Run

Examples:

```bash
go run ./cmd/io-dicom --help
go run ./cmd/compare --help
go run ./cmd/utilities --help
```
