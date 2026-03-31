# Migration Guide (vNext)

This guide covers breaking package-path changes introduced in the architecture cleanup completed in March 2026.

## What Changed

Two public package paths were renamed:

- github.com/innovative-io/io-dicom/dimsec -> github.com/innovative-io/io-dicom/dimse
- github.com/innovative-io/io-dicom/imp -> github.com/innovative-io/io-dicom/implementation

These are import-path breaking changes for downstream users.

## Impact

Any external code importing the old packages will fail to compile until imports and symbol qualifiers are updated.

## Upgrade Steps

1. Update imports in all Go files.
2. Update package qualifiers in call sites.
3. Run formatting and full tests.

## Import Rewrite Examples

Update imports:

```go
import (
    "github.com/innovative-io/io-dicom/dimse"
    "github.com/innovative-io/io-dicom/implementation"
)
```

Update qualifiers:

- dimsec.CStoreWriteRQ(...) -> dimse.CStoreWriteRQ(...)
- imp.GetImplementationClassUID() -> implementation.GetImplementationClassUID()

## Optional Bulk Rewrite (macOS/Linux)

Review before running in your repository:

```bash
find . -type f -name '*.go' -print0 | xargs -0 perl -0pi -e 's#"github.com/innovative-io/io-dicom/dimsec"#"github.com/innovative-io/io-dicom/dimse"#g; s#\bdimsec\.#dimse.#g; s#"github.com/innovative-io/io-dicom/imp"#"github.com/innovative-io/io-dicom/implementation"#g; s#\bimp\.#implementation.#g'
```

Then verify:

```bash
gofmt -w ./...
go test ./...
```

## Notes

- Legacy compatibility wrappers (`jpeglib`, `openjpeg`) were removed in this rewrite.
- Canonical codec code and tests live under `codecs/jpeg` and `codecs/jpeg2000`.
- If you maintain libraries on top of io-dicom, consider releasing a semver-major update that reflects these import-path changes.

## Public Type Renames

Several public interfaces/structs were renamed to improve clarity. Common examples:

- `media.DcmObj` -> `media.DICOMObject`
- `media.DcmTag` -> `media.DICOMTag`
- `media.DCMStudy` -> `media.DICOMStudy`
- `network.AAssociationRQ` -> `network.AssociationRequest`
- `network.AAssociationAC` -> `network.AssociationAccept`
- `network.AAssociationRJ` -> `network.AssociationReject`
- `network.AAbortRQ` -> `network.AbortRequest`
- `network.AReleaseRQ` -> `network.ReleaseRequest`
- `network.AReleaseRP` -> `network.ReleaseResponse`

## Codec Import Migration

Use canonical codec imports:

- `github.com/innovative-io/io-dicom/codecs/jpeg`
- `github.com/innovative-io/io-dicom/codecs/jpeg2000`
