# Transfer Syntax Behavioral Summary

This document is generated from `dictionary/transfersyntax.ConformanceMatrix`.
Do not edit it manually; regenerate it from source instead.

Behavioral quality labels summarize representative media-layer roundtrip expectations:

- `exact`: pixel payload should roundtrip exactly.
- `delta-3`: roundtrip values may differ but stay within absolute per-byte delta <= 3.
- `non-empty`: exact byte match is not required, but decoded payload must be present.

## Passthrough Behavioral Quality

### Exact

| UID | Name | Status | Notes |
| --- | --- | --- | --- |
| 1.2.840.10008.1.2.1.98 | Encapsulated Uncompressed Explicit VR Little Endian | supported | Encapsulation path in media layer |
| 1.2.840.10008.1.2.4.51 | JPEG Extended (Process 2 and 4) | supported | JPEG family path |
| 1.2.840.10008.1.2.4.80 | JPEG-LS Lossless Image Compression | supported | JPEG-LS backend path |
| 1.2.840.10008.1.2.4.90 | JPEG 2000 Image Compression (Lossless Only) | supported | JPEG 2000 family path |
| 1.2.840.10008.1.2.4.100 | MPEG2 Main Profile / Main Level | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.110 | JPEG XL Lossless | supported | JPEG XL family path |
| 1.2.840.10008.1.2.4.112 | JPEG XL | supported | JPEG XL family path |
| 1.2.840.10008.1.2.4.204 | JPIP HTJ2K Referenced | supported-via-shared-family | Routed through JPIP family path |
| 1.2.840.10008.1.2.5 | RLE Lossless | supported | Dedicated RLE transcoder implementation |
| 1.2.840.10008.1.2.7.1 | SMPTE ST 2110-20 Uncompressed Progressive Active Video | supported-via-shared-family | Routed through SMPTE 2110 family path |
| 1.2.840.10008.1.2.8.1 | Deflated Image Frame Compression | supported | Dedicated frame deflate/inflate path |

### Delta-3

| UID | Name | Status | Notes |
| --- | --- | --- | --- |
| 1.2.840.10008.1.2.4.50 | JPEG Baseline (Process 1) | supported | JPEG family path |
| 1.2.840.10008.1.2.4.70 | JPEG Lossless, Non-Hierarchical, First-Order Prediction (Process 14 [Selection Value 1]) | supported | JPEG family path |

### Non-Empty

| UID | Name | Status | Notes |
| --- | --- | --- | --- |
| - | - | - | no transfer syntaxes currently tagged with this behavioral quality |

## Native Behavioral Quality

### Exact

| UID | Name | Status | Notes |
| --- | --- | --- | --- |
| 1.2.840.10008.1.2.4.51 | JPEG Extended (Process 2 and 4) | supported | JPEG family path |
| 1.2.840.10008.1.2.4.80 | JPEG-LS Lossless Image Compression | supported | JPEG-LS backend path |
| 1.2.840.10008.1.2.4.90 | JPEG 2000 Image Compression (Lossless Only) | supported | JPEG 2000 family path |
| 1.2.840.10008.1.2.4.110 | JPEG XL Lossless | supported | JPEG XL family path |
| 1.2.840.10008.1.2.4.204 | JPIP HTJ2K Referenced | supported-via-shared-family | Routed through JPIP family path |

### Delta-3

| UID | Name | Status | Notes |
| --- | --- | --- | --- |
| - | - | - | no transfer syntaxes currently tagged with this behavioral quality |

### Non-Empty

| UID | Name | Status | Notes |
| --- | --- | --- | --- |
| 1.2.840.10008.1.2.4.100 | MPEG2 Main Profile / Main Level | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.7.1 | SMPTE ST 2110-20 Uncompressed Progressive Active Video | supported-via-shared-family | Routed through SMPTE 2110 family path |

## Notes

- Behavioral expectations are intentionally attached only to representative syntax families.
- Canonical expectations are maintained in `dictionary/transfersyntax/conformance_matrix.go`.
- Representative roundtrip assertions live in media tests and are exercised by both package tests and tagged backend suites.
