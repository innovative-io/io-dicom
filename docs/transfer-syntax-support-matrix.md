# Transfer Syntax Support Matrix

This document captures the current transfer syntax support contract for io-dicom.
It distinguishes between:

- fully supported uncompressed/native syntax handling,
- supported syntaxes that are implemented through a shared codec family path,
- intentionally unsupported or retired syntaxes.

## Supported

| UID | Name | Status | Notes |
| --- | --- | --- | --- |
| 1.2.840.10008.1.2 | Implicit VR Little Endian | supported | Native uncompressed parsing/writing |
| 1.2.840.10008.1.2.1 | Explicit VR Little Endian | supported | Native uncompressed parsing/writing |
| 1.2.840.10008.1.2.1.98 | Encapsulated Uncompressed Explicit VR Little Endian | supported | Encapsulation path in media layer |
| 1.2.840.10008.1.2.1.99 | Deflated Explicit VR Little Endian | supported | Dataset deflate/inflate path |
| 1.2.840.10008.1.2.5 | RLE Lossless | supported | Dedicated RLE transcoder implementation |
| 1.2.840.10008.1.2.8.1 | Deflated Image Frame Compression | supported | Dedicated frame deflate/inflate path |
| 1.2.840.10008.1.2.4.50 | JPEG Baseline (Process 1) | supported | JPEG family path |
| 1.2.840.10008.1.2.4.51 | JPEG Extended (Process 2 and 4) | supported | JPEG family path |
| 1.2.840.10008.1.2.4.57 | JPEG Lossless (Process 14) | supported | JPEG family path |
| 1.2.840.10008.1.2.4.70 | JPEG Lossless SV1 | supported | JPEG family path |
| 1.2.840.10008.1.2.4.80 | JPEG-LS Lossless | supported | JPEG-LS backend path |
| 1.2.840.10008.1.2.4.81 | JPEG-LS Near-Lossless | supported | JPEG-LS backend path |
| 1.2.840.10008.1.2.4.90 | JPEG 2000 Lossless | supported | JPEG 2000 family path |
| 1.2.840.10008.1.2.4.91 | JPEG 2000 | supported | JPEG 2000 family path |
| 1.2.840.10008.1.2.4.92 | JPEG 2000 MC Lossless | supported-via-shared-family | Routed through JPEG 2000 codec path |
| 1.2.840.10008.1.2.4.93 | JPEG 2000 MC | supported-via-shared-family | Routed through JPEG 2000 codec path |
| 1.2.840.10008.1.2.4.100 | MPEG2 MP/ML | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.100.1 | MPEG2 MP/ML Fragmentable | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.101 | MPEG2 MP/HL | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.101.1 | MPEG2 MP/HL Fragmentable | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.102 | MPEG4 AVC/H.264 HP 4.1 | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.102.1 | MPEG4 AVC/H.264 HP 4.1 Fragmentable | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.103 | MPEG4 AVC/H.264 BD-compatible | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.103.1 | MPEG4 AVC/H.264 BD-compatible Fragmentable | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.104 | MPEG4 AVC/H.264 HP 4.2 2D | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.104.1 | MPEG4 AVC/H.264 HP 4.2 2D Fragmentable | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.105 | MPEG4 AVC/H.264 HP 4.2 3D | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.105.1 | MPEG4 AVC/H.264 HP 4.2 3D Fragmentable | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.106 | MPEG4 AVC/H.264 HP 4.2 Stereo | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.106.1 | MPEG4 AVC/H.264 HP 4.2 Stereo Fragmentable | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.107 | HEVC Main Profile | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.108 | HEVC Main 10 Profile | supported-via-shared-family | Routed through FFmpeg-backed MPEG path |
| 1.2.840.10008.1.2.4.110 | JPEG XL Lossless | supported | JPEG XL family path |
| 1.2.840.10008.1.2.4.111 | JPEG XL JPEG Recompression | supported-via-shared-family | Routed through JPEG XL family path |
| 1.2.840.10008.1.2.4.112 | JPEG XL | supported | JPEG XL family path |
| 1.2.840.10008.1.2.4.201 | HTJ2K Lossless | supported-via-shared-family | Routed through JPEG 2000 family path |
| 1.2.840.10008.1.2.4.202 | HTJ2K Lossless RPCL | supported-via-shared-family | Routed through JPEG 2000 family path |
| 1.2.840.10008.1.2.4.203 | HTJ2K | supported-via-shared-family | Routed through JPEG 2000 family path |
| 1.2.840.10008.1.2.4.204 | JPIP HTJ2K Referenced | supported-via-shared-family | Routed through JPIP family path |
| 1.2.840.10008.1.2.4.205 | JPIP HTJ2K Referenced Deflate | supported-via-shared-family | Routed through JPIP family path |
| 1.2.840.10008.1.2.7.1 | SMPTE ST 2110-20 Progressive | supported-via-shared-family | Routed through SMPTE 2110 family path |
| 1.2.840.10008.1.2.7.2 | SMPTE ST 2110-20 Interlaced | supported-via-shared-family | Routed through SMPTE 2110 family path |
| 1.2.840.10008.1.2.7.3 | SMPTE ST 2110-30 PCM Audio | supported-via-shared-family | Routed through SMPTE 2110 family path |

## Intentionally Unsupported / Retired

| UID | Name | Reason |
| --- | --- | --- |
| 1.2.840.10008.1.2.2 | Explicit VR Big Endian | Retired; not in supported transfer syntax set |
| 1.2.840.10008.1.2.4.52 | JPEG Extended (Process 3 and 5) | Retired JPEG process variant not implemented |
| 1.2.840.10008.1.2.4.53 | JPEG Spectral Selection (6 and 8) | Retired JPEG process variant not implemented |
| 1.2.840.10008.1.2.4.54 | JPEG Spectral Selection (7 and 9) | Retired JPEG process variant not implemented |
| 1.2.840.10008.1.2.4.55 | JPEG Full Progression (10 and 12) | Retired JPEG process variant not implemented |
| 1.2.840.10008.1.2.4.56 | JPEG Full Progression (11 and 13) | Retired JPEG process variant not implemented |
| 1.2.840.10008.1.2.4.59 | JPEG Extended Hierarchical (16 and 18) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.60 | JPEG Extended Hierarchical (17 and 19) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.61 | JPEG Spectral Selection Hierarchical (20 and 22) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.62 | JPEG Spectral Selection Hierarchical (21 and 23) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.63 | JPEG Full Progression Hierarchical (24 and 26) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.64 | JPEG Full Progression Hierarchical (25 and 27) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.65 | JPEG Lossless Hierarchical (28) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.66 | JPEG Lossless Hierarchical (29) | Retired hierarchical JPEG variant not implemented |
| 1.2.840.10008.1.2.4.94 | JPIP Referenced | Retired JPIP syntax excluded by supported set |
| 1.2.840.10008.1.2.4.95 | JPIP Referenced Deflate | Retired JPIP syntax excluded by supported set |
| 1.2.840.10008.1.2.6.1 | RFC2557 MIME Encapsulation | Retired / not implemented |
| 1.2.840.10008.1.2.6.2 | XML Encoding | Retired / not implemented |

## Notes

- “supported-via-shared-family” means the syntax is accepted and transcoded, but it is handled by a broader codec implementation rather than a syntax-unique engine.
- The canonical support gate is `SupportedTransferSyntax` in `dictionary/transfersyntax/uids.go`.
- The actual encode/decode switch logic lives in `media/dicom_object.go`.
- The support contract is guarded by tests in both `dictionary/transfersyntax` and `media` so supported syntaxes must stay aligned with media-layer handling.
