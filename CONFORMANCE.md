# io-dicom DICOM Conformance Statement

This document formally defines the DICOM standards alignment and known limitations of the `io-dicom` Go library.

**io-dicom Version**: Latest  
**Compliance Date**: 2026-04-01  
**Standards Referenced**: DICOM PS3.1-3.22 (2024 Edition)

---

## Executive Summary

io-dicom is a **partial-conformance** DICOM implementation targeting **SCU (Service Class User) and SCP (Service Class Provider)** roles with primary focus on query/retrieve and storage workflows. The library prioritizes:

- ✅ **Standards compliance** for implemented features (with mapped conformance)
- ✅ **Pure-Go, no-CGO design** (trade-off: some codec implementations are fallback-only)
- ✅ **Network protocol correctness** (PS3.8)
- ✅ **Core DIMSE services** (C-ECHO, C-STORE, C-FIND, C-GET, C-MOVE)
- ⚠️ **Intentional limitations** (documented per section)

---

## Part-Level Conformance Summary

| DICOM Part | Focus | Conformance Level | Notes |
|---|---|---|---|
| **PS3.1** | Introduction | Informational Only | Architectural alignment only |
| **PS3.2** | Conformance | **This Document** | Formal conformance statement |
| **PS3.3** | IOD Definitions | Partial | Object parsing works; IOD validation incomplete |
| **PS3.4** | Service Classes | Partial | Core services (C-ECHO, C-STORE, C-FIND, C-GET, C-MOVE); see section below |
| **PS3.5** | Data Structures & Encoding | Partial | Transfer syntax support matrix below; edge cases incomplete |
| **PS3.6** | Data Dictionary | Aligned | Tag library complete with all standard tags |
| **PS3.7** | DIMSE Services | **Partial** | Core command semantics; status-code edge cases incomplete (see section) |
| **PS3.8** | Network Communication | **Aligned** | UL state machine fully tested; PDU sequencing conforms |
| **PS3.10** | Media Storage | Partial | DICOM file I/O works; constraints incomplete |
| **PS3.11** | Ultrasound | Out of Scope | No ultrasound-specific object handling |
| **PS3.12-14,16-17** | Modality-Specific | Out of Scope | No modality validation or constraints |
| **PS3.15** | Security Profiles | Partial | TLS transport profile baseline implemented (TLS 1.2 minimum) |
| **PS3.18** | Web Services | Partial | WADO-RS, STOW-RS, QIDO-RS implemented; not fully traced to standard |
| **PS3.19-22** | Profiles, Security, Display, Application Hosting | Out of Scope | Not implemented |

---

## Detailed Feature Conformance

### PS3.4: Service Classes

#### ✅ C-ECHO (Verification)

**Status**: Aligned  
**Standard Reference**: PS3.4 §A.1 (Verification Service Class)  
**Implementation**: `dimse/c_echo.go`  
**Tests**: `dimse/dimse_test.go` (C-ECHO test block)

**Supported**:
- Request/Response command field exchange
- Required fields validation (Affected SOP Class UID, MessageID)
- Response status encoding
- C-ECHO-RSP field validation (`CommandDataSetType=0x0101`, `MessageIDBeingRespondedTo` present)

**Known Limitations**: None

---

#### ✅ C-STORE (Storage)

**Status**: Aligned  
**Standard Reference**: PS3.4 §B.5 (Storage Service Class)  
**Implementation**: `dimse/c_store.go`  
**Tests**: `dimse/dimse_test.go` (C-STORE test block)

**Supported**:
- Request/Response message exchange
- Affected SOP Instance UID validation
- Dataset encoding/decoding
- Status code propagation (Success, Failure)
- Service-specific C-STORE status-code class validation in read/write paths
- C-STORE-RSP required field validation (`CommandDataSetType=0x0101`, `MessageIDBeingRespondedTo` present)

**Known Limitations**:
- Storage Media File-Set support incomplete (PS3.10 subset)
- No implicit DIMSE timeout (caller must manage)

---

#### ⚠️ C-FIND (Query)

**Status**: Partial  
**Standard Reference**: PS3.4 §C.4.1 (Query/Retrieve Service Class, C-FIND Operation)  
**Implementation**: `dimse/c_find.go`  
**Tests**: `dimse/dimse_test.go` (C-FIND test block with matrix tables)

**Supported**:
- Request/Response command field exchange
- Identifier dataset handling
- Query/Retrieve level validation in SCP request handling (`PATIENT`, `STUDY`, `SERIES`, `IMAGE`, `FRAME`)
- Status code transitions (Success, Pending, Warning, Failure)
- Response dataset validation (pending responses must include identifier)
- Final response dataset validation (final responses must not include identifier)
- CommandDataSetType validation (0x0101, 0x0102)

**Known Limitations**:
- Service class attributes (Relational Query, Timezone Query) not enforced
- No Query Model negotiation (Standard Query Model assumed)

---

#### ⚠️ C-GET (Query/Retrieve GET)

**Status**: Partial  
**Standard Reference**: PS3.4 §C.4.3 (Query/Retrieve Service Class, C-GET Operation)  
**Implementation**: `dimse/c_get.go`  
**Tests**: `dimse/dimse_test.go` (C-GET test block with sub-operation counter matrix)

**Supported**:
- Request/Response command field exchange
- Identifier dataset handling
- Sub-operation counter tracking (NumberOfRemainingSubOperations, NumberOfCompletedSubOperations, etc.)
- Core sub-operation counter invariant validation in read/write paths
- Status code transitions (Pending, Success, Warning, Failure)
- CommandDataSetType validation

**Known Limitations**:
- ⚠️ Full sub-operation lifecycle semantics (including externally initiated sub-op cardinality) remain caller-defined
- No automatic sub-operation abort on error
- Caller responsible for SOP Instance UID uniqueness across sub-operations

---

#### ⚠️ C-MOVE (Query/Retrieve MOVE)

**Status**: Partial  
**Standard Reference**: PS3.4 §C.4.2 (Query/Retrieve Service Class, C-MOVE Operation)  
**Implementation**: `dimse/c_move.go`  
**Tests**: `dimse/dimse_test.go` (C-MOVE test block with sub-operation counter matrix)  
**Documentation**: `docs/dicom-cmove-implementation.md`

**Supported**:
- Request parsing with priority handling
- Destination AE matching (passed to handler)
- Sub-operation counter tracking (all four per PS3.4 §C.4.2.1.9)
- Core sub-operation counter invariant validation in read/write paths
- Status code transitions (Pending, Success, Warning, Failure)
- CommandDataSetType validation
- Priority field support (0=High, 1=Medium, 2=Low)

**Known Limitations**:
- ⚠️ Full sub-operation lifecycle semantics remain application-managed because destination C-STORE orchestration is external to this package
- No built-in C-STORE forwarding to destination (responsibility on handler)
- Move destination validation is handler-specific (no UID lookup)

---

#### ⚠️ C-CANCEL (Cancel)

**Status**: Partial  
**Standard Reference**: PS3.4 §C.3.4.2.1 (Cancel Operation)  
**Implementation**: `dimse/` command parsing  
**Tests**: `services/scp_test.go` (cancel tracking path covered indirectly in SCP command handling)

**Supported**:
- Request parsing
- Message ID tracking for cancellation requests
- Optional callback hook (`OnCCancelRequest`) for application-level cancel handling
- Pre-operation cancellation response support for C-FIND/C-GET/C-MOVE when cancellation is already registered

**Known Limitations**:
- ⚠️ **Critical**: Synchronous per-association request processing means mid-operation cancellation is **not** functionally supported
- Cancel request is not yet preemptive for in-flight result streaming
- **Recommendation**: Implement asynchronous handler dispatch if C-CANCEL support is required

---

### PS3.7: DIMSE Services

#### ✅ Command Field Validation

**Status**: Partial  
**Standard Reference**: PS3.7 §6 (Command Encoding)  
**Implementation**: All command writers in `dimse/`

**Supported**:
- Mandatory command field enforcement on write
- Reserved field zero-initialization
- Command group length calculation

**Known Limitations**:
- ⚠️ **Negative test coverage**: Not all missing field combinations have explicit unit tests
- Some optional fields may not reject invalid value ranges

---

#### ⚠️ Status Code Semantics

**Status**: Partial  
**Standard Reference**: PS3.7 §7.4 (Status)  
**Implementation**: Status handling in command writers

**Supported**:
- Status code encoding (3-byte format per status class)
- Success, Pending, Warning, Failure classifications
- Query/Retrieve operation-specific status validation in C-FIND/C-GET/C-MOVE read/write paths
	- C-FIND rejects Bxxx warning final statuses
	- C-GET/C-MOVE accept B000 warning semantics while rejecting C-STORE-only B006/B007 warning statuses
	- C-MOVE-specific refusal code A801 is rejected outside C-MOVE operations

**Known Limitations**:
- Full standard-wide status string mapping is not exhaustive; core implemented statuses are mapped with range-aware fallbacks

---

#### ✅ Sub-Operation Counters

**Status**: Aligned (core invariants)  
**Standard Reference**: PS3.7 §C.4.2.1.9, PS3.4 §C.4.2.1.8, PS3.4 §C.4.3.1.8  
**Implementation**: `dimse/c_move.go`, `dimse/c_get.go` response handling

**Supported**:
- Four-counter tracking (Completed, Remaining, Failed, Warning)
- Counter encoding in response command
- Counter invariant validation in C-GET/C-MOVE read and write paths:
	- Pending responses require Remaining > 0
	- Final responses require Remaining == 0
	- Success responses require Failed == 0 and Warning == 0

**Known Limitations**:
- ⚠️ Total initiated sub-operation cardinality is caller-defined; this library validates response consistency but does not infer workload cardinality

---

### PS3.8: Network Communication

#### ✅ UL Association State Machine

**Status**: Aligned  
**Standard Reference**: PS3.8 §8.2 (UL State Machine), §9.2-9.3 (Association Procedures)  
**Implementation**: `network/pdu_service.go`  
**Tests**: `network/pdu_service_ulmachine_test.go` (25+ tests covering all major transitions)

**Supported**:
- ✅ IDLE → AWAITING-AC (outbound; covered by integration tests)
- ✅ IDLE → ASSOCIATED (SCP-side acceptance)
- ✅ AWAITING-AC → ASSOCIATED (receive accept)
- ✅ AWAITING-AC → RELEASED (receive rejection)
- ✅ AWAITING-AC → ABORTED (receive abort)
- ✅ ASSOCIATED ↔ P-DATA-TF (data transfer)
- ✅ ASSOCIATED → AWAITING-RELEASE-RP (release handshake)
- ✅ AWAITING-RELEASE-RP → RELEASED (release complete)
- ✅ (any) → ABORTED (abort at any state)
- ✅ Timeout fallback to abort if peer doesn't respond within 30s

**Known Limitations**: None for state machine correctness

---

#### ✅ PDU Sequencing

**Status**: Aligned  
**Standard Reference**: PS3.8 §9.2 (Normal Operation), §9.3 (Error Handling)  
**Implementation**: `network/pdu_service.go::NextPDU()`  
**Tests**: `network/pdu_service_ulmachine_test.go` state transition table tests

**Supported**:
- Invalid PDU type rejection out of sequence
- Graceful close (release handshake)
- Forced abort on protocol violations
- Sentinel error returns for release/abort signals

---

#### ⚠️ Association Negotiation

**Status**: Partial  
**Standard Reference**: PS3.8 §9.2.1 (Associate Service)  
**Implementation**: `network/pdu_service.go`

**Supported**:
- A-ASSOCIATE-RQ generation with presentation context list
- A-ASSOCIATE-AC/RJ/AB parsing
- Transfer syntax preference (Explicit VR LE → Implicit VR LE → Explicit VR BE)
- Negotiation restricted to declared supported transfer syntaxes (`dictionary/transfersyntax.SupportedTransferSyntax`)
- AE title handling (16-byte, space-padded)
- Implementation class UID and version name encoding
- Explicit presentation-context reject result mapping:
	- Result 3: abstract syntax not supported
	- Result 4: transfer syntaxes not supported

**Known Limitations**:
- Role negotiation partially tested; no asynchronous operation window constraints validated

---

### PS3.15: Security Profiles

#### ⚠️ TLS Transport Profile Baseline

**Status**: Partial  
**Standard Reference**: PS3.15 (Secure Transport Connection Profiles)  
**Implementation**: `network/pdu_service.go`, `services/scp.go`  
**Tests**: `services/tls_test.go`, `network/pdu_service_negotiation_test.go`

**Supported**:
- TLS for SCU (`ConnectTLS`) and SCP (`NewSCPWithTLS`)
- Enforced minimum TLS version of 1.2 for client and server config normalization

**Known Limitations**:
- Cipher-suite profile pinning is caller-managed
- Mutual TLS policy and certificate identity authorization policy are deployment-specific
- Audit trail and ATNA profile workflows are not implemented

---

#### ✅ Association Release

**Status**: Aligned  
**Standard Reference**: PS3.8 §9.2.5 (Association Release)  
**Implementation**: `network/pdu_service.go::Close()`  
**Tests**: `network/pdu_service_ulmachine_test.go::TestClosePerformsReleaseHandshake`, `TestCloseAbortsWhenPeerDoesNotRespond`

**Supported**:
- Clean release with A-RELEASE-RQ/RP handshake
- Timeout detection (30s) with automatic fallback to abort
- Sentinel return on release completion

**Known Limitations**: None

---

#### ✅ Association Abort

**Status**: Aligned  
**Standard Reference**: PS3.8 §9.3.5 (Association Abort)  
**Implementation**: `network/pdu_service.go::NextPDU()`  
**Tests**: `network/pdu_service_ulmachine_test.go::TestNextPDUAbortReturnsSentinel`, `TestNextPDU_ReleaseAndAbortSequences`

**Supported**:
- A-ABORT-RQ reception at any state
- Immediate connection close
- Sentinel error return
- Source and information fields per PS3.8 Table 9-26

**Known Limitations**: None

---

### PS3.5: Data Structures and Encoding

#### ✅ Tag Dictionary

**Status**: Aligned  
**Standard Reference**: PS3.6 (Data Dictionary)  
**Implementation**: `dictionary/tags/`

**Supported**:
- All standard DICOM tags
- VR (Value Representation) definitions
- VM (Value Multiplicity) definitions
- Tag search by number, keyword, or alias

**Known Limitations**: None for tag definitions

---

#### ⚠️ Transfer Syntax Support

**Status**: Partial  
**Standard Reference**: PS3.5 §8-10 (DICOM Data Element Encoding)

**See**: `docs/transfer-syntax-support-matrix.md` for detailed coverage

**Codec Support by Category**:

| Category | Coverage | Implementation | Status |
|---|---|---|---|
| Uncompressed (Implicit VR LE/BE, Explicit VR LE/BE) | ✅ 100% | Native Go | Aligned |
| JPEG Baseline (Process 1, 4) | ✅ 8-bit | `codecs/jpeg/` pure-Go | Aligned (8-bit only; 12-bit fallback) |
| JPEG Lossless | ⚠️ Fallback | passthrough | Minimal |
| JPEG2000 | ⚠️ Fallback | passthrough | Minimal |
| JPEG-LS | ⚠️ Fallback | passthrough | Minimal |
| JPEG XL | ⚠️ Fallback | passthrough | Minimal |
| RLE Lossless | ✅ Implemented | `transcoder/` pure-Go | Aligned |
| MPEG-2 | ⚠️ Fallback | passthrough | Minimal |
| SMPTE 2110-20 | ⚠️ Recognition only | `codecs/smpte2110/` | Minimal |

**Known Limitations**:
- ⚠️ **Lossy compression codecs (JPEG, JPEG2000, JPEG-LS, JPEG XL, MPEG)**: Implemented as passthrough (data preserved but not recompressed)
- Trade-off: Pure-Go design prevents codec library dependencies
- **Recommendation**: Use external codec services for codec conversion workflows

---

#### ⚠️ Data Element Encoding

**Status**: Partial  
**Standard Reference**: PS3.5 §7 (Parsing Rules)  
**Implementation**: `media/` package

**Supported**:
- VR-aware encoding/decoding
- Implicit/Explicit VR handling
- Big-Endian/Little-Endian byte order
- Sequence and item nesting

**Known Limitations**:
- ⚠️ **Pixel data edge cases**: 16-bit, 32-bit pixel data with non-standard frame layouts not exhaustively tested
- **Compressed pixel data in datasets**: Passthrough only (no decompression)

---

### PS3.10: Media Storage and File Format

#### ⚠️ DICOM File Format

**Status**: Partial  
**Standard Reference**: PS3.10 §7 (DICOM File Format)  
**Implementation**: `media/` package

**Supported**:
- DICOM Preamble (128-byte prefix + "DICM" magic)
- File Meta Information Group Length (0x0002, 0x0000)
- Transfer Syntax UID encoding/selection
- Media Storage SOP Class/Instance UID
- File-level meta information

**Known Limitations**:
- ⚠️ **Media Storage File-Set**: Directory and catalog file management not implemented
- No validation of required file meta attributes
- No automatic transfer syntax negotiation based on file encoding

---

### PS3.18: Web Services (DICOMweb)

#### ⚠️ DICOMweb APIs

**Status**: Partial  
**Standard Reference**: PS3.18 (Web Access to DICOM Persistent Objects)  
**Implementation**: `wado/` package

**Supported APIs**:
- WADO-RS (Web Access and Disposal of Objects) - Retrieve
- STOW-RS (Store over the Web) - Store
- QIDO-RS (Query based on ID for DICOM Objects) - Query

**Known Limitations**:
- ⚠️ **No formal section-level traceability**: Standard conformance not explicitly documented per section
- Proprietary extensions may exist beyond standard scope
- Transaction atomicity not guaranteed

---

## Design Decisions and Trade-Offs

### 1. Pure-Go, No-CGO Design

**Decision**: Avoid C library dependencies for codec implementations  
**Impact**: 
- ✅ Easy deployment (single binary, no linking)
- ✅ No codec library licensing complications
- ⚠️ Codec performance may lag native implementations
- ⚠️ Some codecs implemented as passthrough

**Conformance Impact**: Codec operations preserve data but may not recompress efficiently

---

### 2. Synchronous Request Processing

**Decision**: Synchronous per-association request dispatch  
**Impact**:
- ✅ Simpler state management
- ✅ Deterministic error semantics
- ⚠️ C-CANCEL cannot interrupt mid-operation
- ⚠️ Async workloads require external goroutine wrapping

**Conformance Impact**: C-CANCEL conformance limited (documented as minimal support)

---

### 3. No Implicit Timeout Management

**Decision**: Callers manage operation timeouts  
**Impact**:
- ✅ Flexible timeout semantics
- ✅ No hidden resource allocation
- ⚠️ Caller must implement DIMSE operation timeout logic

**Conformance Impact**: No deviation from standard (timeouts are implementation-specific)

---

## Conformance Limitations by Category

### ❌ Not Implemented (Out of Scope)

1. **PS3.11-PS3.17**: Modality-specific object handling (Ultrasound, XC, SR, etc.)
2. **PS3.19**: Application Hosting (not applicable to library)
3. **PS3.20**: Constraint Information Model (not applicable to library)
4. **PS3.21**: Implicit VR Encoding (alternative encoding; explicit preferable)
5. **PS3.22**: Implementation Guides (reference only)

---

### ⚠️ Known Gaps (Partial Conformance)

| Gap | Severity | Mitigation | Workaround |
|---|---|---|---|
| Status-code edge cases (PS3.4 disallowed combinations) | Low | Continue expanding per-operation matrix tests to full status tables | Validate status codes in handler layer |
| C-CANCEL mid-operation | High | Implement async request dispatcher | Wrap handlers in goroutines with cancellation |
| Codec conversion (lossy) | Medium | Use external codec service | Route through ffmpeg or similar for conversion |
| Presentation context result code matrix | Low | Add comprehensive negotiation tests | Accept handler returns default transfer syntax |
| Media file-set management | Low | Use filesystem directly or external PACS | Implement custom file organization |
| IOD-level validation | Medium | Build IOD constraint matrix | Validate in handler layer |

---

## Testing and Validation

### Unit Tests
- **Network**: 44+ tests covering UL state machine, PDU sequencing
- **DIMSE**: 45+ tests covering command parsing, status semantics
- **Dictionary**: Comprehensive tag/SOP class/transfer syntax lookups
- **Media**: File I/O and encoding tests
- **Codecs**: JPEG baseline, RLE, transfer syntax tests

### Integration Tests
- Service-level C-ECHO, C-STORE, C-FIND, C-GET, C-MOVE workflows
- Network association handshake and release
- Web service endpoints (WADO-RS, STOW-RS, QIDO-RS)

---

## Compliance Certification

**Claiming Conformance To**: DICOM PS3.4 Service Classes (partial), PS3.7 DIMSE (partial), PS3.8 Network (aligned)

### Not DICOM Certified
This library has **not** undergone formal DICOM Conformance Review Board (CRB) certification. This conformance statement is self-assessed and should be validated against specific deployment requirements.

---

## Recommendations for Users

1. **Interoperability Testing**: Validate io-dicom behavior with target PACS systems before production deployment
2. **C-CANCEL Support**: If mid-operation cancellation is required, implement asynchronous handler wrapping
3. **Codec Conversion**: Route lossy codec conversion through external services (ffmpeg, DicomRtl, etc.)
4. **IOD Validation**: Implement DICOM object attribute validation in your handler layer
5. **Status Code Validation**: Enforce service-specific status-code constraints in your application logic

---

## Document History

| Version | Date | Changes |
|---|---|---|
| 1.0 | 2026-04-01 | Initial formal conformance statement; comprehensive PS3.4/7/8 alignment assessment |

---

## Contact and Feedback

For conformance questions or discrepancies, please file an issue or reach out to the io-dicom maintainers.

---

**End of Conformance Statement**
