# Transfer Syntax Support Matrix

This document is generated from `dictionary/transfersyntax.ConformanceMatrix`.
Do not edit it manually; regenerate it from source instead.

It distinguishes between:

- fully supported uncompressed/native syntax handling,
- supported syntaxes that are implemented through a shared codec family path,
- intentionally unsupported or retired syntaxes.

## Supported

| UID | Name | Status | Behavioral (passthrough/native) | Notes |
| --- | --- | --- | --- | --- |
| 1.2.840.10008.1.2 | Implicit VR Little Endian | supported | - | Native uncompressed parsing/writing |
| 1.2.840.10008.1.2.1 | Explicit VR Little Endian | supported | - | Native uncompressed parsing/writing |
| 1.2.840.10008.1.2.1.98 | Encapsulated Uncompressed Explicit VR Little Endian | supported | exact / - | Encapsulation path in media layer |
| 1.2.840.10008.1.2.1.99 | Deflated Explicit VR Little Endian | supported | - | Dataset deflate/inflate path |
| 1.2.840.10008.1.2.4.50 | JPEG Baseline (Process 1) | supported | delta-3 / - | JPEG family path |
| 1.2.840.10008.1.2.4.51 | JPEG Extended (Process 2 and 4) | supported | exact / exact | JPEG family path |
| 1.2.840.10008.1.2.4.57 | JPEG Lossless, Non-Hierarchical (Process 14) | supported | - | JPEG family path |
| 1.2.840.10008.1.2.4.70 | JPEG Lossless, Non-Hierarchical, First-Order Prediction (Process 14 [Selection Value 1]) | supported | delta-3 / - | JPEG family path |
| 1.2.840.10008.1.2.4.80 | JPEG-LS Lossless Image Compression | supported | exact / exact | JPEG-LS backend path |
| 1.2.840.10008.1.2.4.81 | JPEG-LS Lossy (Near-Lossless) Image Compression | supported | - | JPEG-LS backend path |
| 1.2.840.10008.1.2.4.90 | JPEG 2000 Image Compression (Lossless Only) | supported | exact / exact | JPEG 2000 family path |
| 1.2.840.10008.1.2.4.91 | JPEG 2000 Image Compression | supported | - | JPEG 2000 family path |
| 1.2.840.10008.1.2.4.92 | JPEG 2000 Part 2 Multi-component Image Compression (Lossless Only) | supported-via-shared-family | - | Routed through JPEG 2000 codec path |
| 1.2.840.10008.1.2.4.93 | JPEG 2000 Part 2 Multi-component Image Compression | supported-via-shared-family | - | Routed through JPEG 2000 codec path |
| 1.2.840.10008.1.2.4.100 | MPEG2 Main Profile / Main Level | supported-via-shared-family | exact / non-empty | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.100.1 | Fragmentable MPEG2 Main Profile / Main Level | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.101 | MPEG2 Main Profile / High Level | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.101.1 | Fragmentable MPEG2 Main Profile / High Level | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.102 | MPEG-4 AVC/H.264 High Profile / Level 4.1 | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.102.1 | Fragmentable MPEG-4 AVC/H.264 High Profile / Level 4.1 | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.103 | MPEG-4 AVC/H.264 BD-compatible High Profile / Level 4.1 | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.103.1 | Fragmentable MPEG-4 AVC/H.264 BD-compatible High Profile / Level 4.1 | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.104 | MPEG-4 AVC/H.264 High Profile / Level 4.2 For 2D Video | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.104.1 | Fragmentable MPEG-4 AVC/H.264 High Profile / Level 4.2 For 2D Video | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.105 | MPEG-4 AVC/H.264 High Profile / Level 4.2 For 3D Video | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.105.1 | Fragmentable MPEG-4 AVC/H.264 High Profile / Level 4.2 For 3D Video | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.106 | MPEG-4 AVC/H.264 Stereo High Profile / Level 4.2 | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.106.1 | Fragmentable MPEG-4 AVC/H.264 Stereo High Profile / Level 4.2 | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.107 | HEVC/H.265 Main Profile / Level 5.1 | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.108 | HEVC/H.265 Main 10 Profile / Level 5.1 | supported-via-shared-family | - | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.110 | JPEG XL Lossless | supported | exact / exact | JPEG XL family path |
| 1.2.840.10008.1.2.4.111 | JPEG XL JPEG Recompression | supported-via-shared-family | - | Routed through JPEG XL family path |
| 1.2.840.10008.1.2.4.112 | JPEG XL | supported | exact / - | JPEG XL family path |
| 1.2.840.10008.1.2.4.201 | High-Throughput JPEG 2000 Image Compression (Lossless Only) | supported-via-shared-family | - | Routed through JPEG 2000 family path |
| 1.2.840.10008.1.2.4.202 | High-Throughput JPEG 2000 with RPCL Options Image Compression (Lossless Only) | supported-via-shared-family | - | Routed through JPEG 2000 family path |
| 1.2.840.10008.1.2.4.203 | High-Throughput JPEG 2000 Image Compression | supported-via-shared-family | - | Routed through JPEG 2000 family path |
| 1.2.840.10008.1.2.4.204 | JPIP HTJ2K Referenced | supported-via-shared-family | exact / exact | Routed through JPIP family path |
| 1.2.840.10008.1.2.4.205 | JPIP HTJ2K Referenced Deflate | supported-via-shared-family | - | Routed through JPIP family path |
| 1.2.840.10008.1.2.5 | RLE Lossless | supported | exact / - | Dedicated RLE transcoder implementation |
| 1.2.840.10008.1.2.7.1 | SMPTE ST 2110-20 Uncompressed Progressive Active Video | supported-via-shared-family | exact / non-empty | Routed through SMPTE 2110 family path |
| 1.2.840.10008.1.2.7.2 | SMPTE ST 2110-20 Uncompressed Interlaced Active Video | supported-via-shared-family | - | Routed through SMPTE 2110 family path |
| 1.2.840.10008.1.2.7.3 | SMPTE ST 2110-30 PCM Digital Audio | supported-via-shared-family | - | Routed through SMPTE 2110 family path |
| 1.2.840.10008.1.2.8.1 | Deflated Image Frame Compression | supported | exact / - | Dedicated frame deflate/inflate path |

## Intentionally Unsupported / Retired

| UID | Name | Reason |
| --- | --- | --- |
| 1.2.840.10008.1.2.2 | Explicit VR Big Endian | Retired; not in supported transfer syntax set |
| 1.2.840.10008.1.2.4.52 | JPEG Extended (Process 3 and 5) | Retired JPEG process variant not implemented |
| 1.2.840.10008.1.2.4.53 | JPEG Spectral Selection, Non-Hierarchical (Process 6 and 8) | Retired JPEG process variant not implemented |
| 1.2.840.10008.1.2.4.54 | JPEG Spectral Selection, Non-Hierarchical (Process 7 and 9) | Retired JPEG process variant not implemented |
| 1.2.840.10008.1.2.4.55 | JPEG Full Progression, Non-Hierarchical (Process 10 and 12) | Retired JPEG process variant not implemented |
| 1.2.840.10008.1.2.4.56 | JPEG Full Progression, Non-Hierarchical (Process 11 and 13) | Retired JPEG process variant not implemented |
| 1.2.840.10008.1.2.4.58 | JPEG Lossless, Non-Hierarchical (Process 15) | Retired JPEG process variant not implemented |
| 1.2.840.10008.1.2.4.59 | JPEG Extended, Hierarchical (Process 16 and 18) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.60 | JPEG Extended, Hierarchical (Process 17 and 19) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.61 | JPEG Spectral Selection, Hierarchical (Process 20 and 22) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.62 | JPEG Spectral Selection, Hierarchical (Process 21 and 23) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.63 | JPEG Full Progression, Hierarchical (Process 24 and 26) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.64 | JPEG Full Progression, Hierarchical (Process 25 and 27) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.65 | JPEG Lossless, Hierarchical (Process 28) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.66 | JPEG Lossless, Hierarchical (Process 29) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.94 | JPIP Referenced | Retired JPIP syntax excluded by supported set |
| 1.2.840.10008.1.2.4.95 | JPIP Referenced Deflate | Retired JPIP syntax excluded by supported set |
| 1.2.840.10008.1.2.6.1 | RFC 2557 MIME encapsulation | Retired / not implemented |
| 1.2.840.10008.1.2.6.2 | XML Encoding | Retired / not implemented |
| 1.2.840.10008.1.20 | Papyrus 3 Implicit VR Little Endian | Legacy Papyrus transfer syntax not implemented |

## Notes

- `supported-via-shared-family` means the syntax is accepted and transcoded, but it is handled by a broader codec implementation rather than a syntax-unique engine.
- The canonical support gate is `SupportedTransferSyntax` in `dictionary/transfersyntax/uids.go`.
- The canonical conformance inventory is `ConformanceMatrix` in `dictionary/transfersyntax/conformance_matrix.go`.
- Behavioral quality uses `exact`, `delta-3`, and `non-empty` to encode representative expectations for passthrough and native backend paths.
- The actual encode/decode switch logic lives in `media/dicom_object.go`.
- The support contract is guarded by tests in both `dictionary/transfersyntax` and `media` so supported syntaxes must stay aligned with media-layer handling.
- Representative behavioral roundtrip tests in `media/dicom_object_test.go` verify that native dataset syntaxes and representative pixel-data syntax families can roundtrip through the media layer with deterministic passthrough-backed codec selection.
