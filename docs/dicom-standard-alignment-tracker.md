# DICOM Standard Alignment Tracker

This tracker defines how `io-dicom` is aligned to DICOM standard parts section by section.

Goal: make conformance work explicit, test-backed, and incrementally auditable.

## How To Use This Tracker

For each part and section:

1. Identify normative requirements ("shall", "shall not", required fields/behaviors).
2. Map each requirement to implementation and tests.
3. Classify status:
   - `aligned`: requirement implemented and covered by tests.
   - `partial`: some requirement paths implemented or tested.
   - `not-started`: no verified mapping yet.
   - `out-of-scope`: intentionally not implemented for this library.
4. Record gaps as concrete tasks with acceptance tests.

## Part-Level Baseline

| DICOM Part | Focus | Status | Evidence |
| --- | --- | --- | --- |
| PS3.1 | Introduction and overview | partial | Referenced in architecture docs; no explicit requirement checklist yet |
| PS3.2 | Conformance statement framework | partial | Conformance notes in `README.md`; no formal io-dicom statement document yet |
| PS3.3 | Information Object Definitions (IODs) | partial | Dictionary and object parsing exist; no IOD-level validation matrix yet |
| PS3.4 | Service Class specifications | partial | C-STORE/C-FIND/C-GET/C-MOVE service support exists; not fully mapped per service class section |
| PS3.5 | Data structures and encoding | partial | Transfer syntax matrix and media encode/decode tests exist |
| PS3.6 | Data dictionary | partial | Tag dictionary packages exist; no section-level audit checklist yet |
| PS3.7 | Message exchange (DIMSE) | partial | `dimse/` commands with section citations and tests |
| PS3.8 | Network communication support | partial | `network/` PDU and UL behavior with section citations and tests |
| PS3.10 | Media storage/file format | partial | DICOM object read/write paths and media tests |
| PS3.18 | Web services | partial | `wado/` package exists; section-level traceability pending |
| Other parts (PS3.11+ / PS3.12+ / PS3.14+ / PS3.15+ / PS3.16+ / PS3.17+ / PS3.19+ / PS3.20+ / PS3.21+ / PS3.22) | Profiles, security, display, content mapping, examples, and implementation guides | not-started | No formal traceability entries yet |

## Section-By-Section Work Queue

Order is chosen to maximize clinical interoperability impact.

1. PS3.7 DIMSE command requirements and status code behavior.
2. PS3.8 association state machine and PDU behavior.
3. PS3.5 transfer syntax and encoding edge cases.
4. PS3.3/PS3.4 IOD + service class contract verification.
5. PS3.10 file meta information and media interchange constraints.
6. PS3.18 DICOMweb conformance traceability.

## Phase 1: PS3.7 Detailed Checklist (Start Here)

Working matrix: `docs/ps3.7-dimse-requirements-matrix.md`

### Scope

- C-ECHO
- C-STORE
- C-FIND
- C-GET
- C-MOVE
- N-service response helpers

### Requirements To Verify

| Requirement Area | Status | Current Evidence | Gap To Close |
| --- | --- | --- | --- |
| Required command fields are validated on read/write | partial | `dimse/c_store.go`, `dimse/c_find.go`, `dimse/c_get.go`, `dimse/c_move.go` and tests | Add explicit negative tests for each missing required field across all remaining command types |
| Command group length correctness on write | partial | Command length comments and tests in DIMSE package | Add table-driven assertions for every command writer |
| Response status semantics (Success/Pending/Warning/Failure) | partial | Existing command handlers and integration behavior plus status matrix tests for C-FIND/C-GET/C-MOVE, including malformed `CommandDataSetType` rejection and pending C-FIND dataset enforcement | Add section-specific status constraints and disallowed combination tests by service |
| Sub-operation counter semantics for retrieve services | partial | C-MOVE response handling checks | Align C-GET/C-MOVE counters with complete section requirements and test all permutations |
| C-CANCEL behavior during active operations | partial | Cancel parsing/logging exists | Implement and test mid-operation cancel state transitions where applicable |

### Exit Criteria

- A PS3.7 requirement matrix exists with requirement IDs, implementation links, and test links.
- All mapped requirements have deterministic unit/integration coverage.
- Any intentionally unsupported semantics are documented with rationale.

## Definition Of Done For Any Part

- All normative requirements in scoped sections are mapped.
- Each requirement has one of: `aligned`, `partial`, `out-of-scope`.
- `aligned` items are backed by automated tests.
- Public documentation reflects behavior and known limitations.

## Phase 2: PS3.8 Detailed Checklist

Working matrix: `docs/ps3.8-network-requirements-matrix.md`

State-transition matrix: `docs/ps3.8-ul-state-transition-matrix.md`

### Scope

- Association negotiation (A-ASSOCIATE-RQ, A-ASSOCIATE-AC, A-ASSOCIATE-RJ)
- Association release (A-RELEASE-RQ, A-RELEASE-RP)
- Association abort (A-ABORT)
- PDU sequencing and state-machine transitions
- Transport-level connection lifecycle

### Requirements To Verify

| Requirement Area | Status | Current Evidence | Gap To Close |
| --- | --- | --- | --- |
| Association state machine transitions (all valid paths) | aligned | `network/pdu_service_ulmachine_test.go` table-driven tests covering ASSOCIATED→ReleaseRQ/ReleaseRP/Abort, invalid PDU type rejection | Implement outbound Connect tests (AWAITING-AC state validation, peer rejection); add SCP acceptance path test |
| Rejection closes transport immediately | aligned | `network/pdu_service.go` rejection path with `closeConn()`; integration test `TestNextPDUAssociationRejectClosesConnection` | All covered |
| Release handshake semantics (RQ→RP sequence with timeout) | aligned | `network/pdu_service.go::Close()` implements release logic; tests cover success and fallback-abort | Add negative tests for invalid release sequences |
| Abort at any state | aligned | `network/pdu_service.go::NextPDU()` abort case returns sentinel; test `TestNextPDU_ReleaseAndAbortSequences` | All covered |
| PDU field constraints (reserved bytes, source values) | aligned | `network/a_abort_rq.go`, `network/a_association_rj.go` with default values; tests validate per Table 9-21, Table 9-26 | All covered |
| Presentation context negotiation | partial | `network/pdu_service.go` transfer syntax preference logic | Add explicit test matrix for presentation context result codes and forced-acceptance semantics |

### Exit Criteria

- A PS3.8 state-transition matrix exists with all 11 major transitions mapped.
- UL state machine tested with 7+ table-driven test cases (some checking multiple states).
- Outbound Connect path includes peer rejection scenario.
- SCP acceptance path has explicit test.
- All tests pass; no regressions.

## Phase 3: PS3.5 and Media Encoding (Future)

Planned scope: Transfer syntax, encoding edge cases, pixel data flow.
