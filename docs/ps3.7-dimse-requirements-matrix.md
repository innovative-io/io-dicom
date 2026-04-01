# PS3.7 DIMSE Requirements Matrix

This is the first detailed section-by-section conformance matrix for `io-dicom`.

Scope: message exchange semantics for implemented DIMSE services.

## Requirement Mapping

| Requirement ID | PS3.7 Reference | Requirement Summary | Implementation Evidence | Test Evidence | Status |
| --- | --- | --- | --- | --- | --- |
| PS3.7-C.1-CECHO-RQ-RSP | C.1 | C-ECHO request/response command field behavior | `dimse/c_echo.go` (`CEchoWriteRQ`, `CEchoReadRSP`, `CEchoWriteRSP`) with required SOP Class, request object, and MessageID validation in writers | `dimse/dimse_test.go` C-ECHO block including missing SOP Class, missing MessageID, and nil request negative tests | partial |
| PS3.7-C.3-CSTORE-RQ-REQUIRED | C.3.1 | C-STORE-RQ must include required command fields including Affected SOP Instance UID | `dimse/c_store.go` required field guard in `CStoreWriteRQ` | `dimse/dimse_test.go` assertions for required UID presence and missing UID error | aligned |
| PS3.7-C.3-CSTORE-RSP-STATUS | C.3 | C-STORE-RSP status propagation | `dimse/c_store.go` (`CStoreReadRSP`, `CStoreWriteRSP`) with required request field validation | `dimse/dimse_test.go` C-STORE block including missing required field and nil request checks | partial |
| PS3.7-C.4.1-CFIND-RQ-RSP | C.4.1 | C-FIND request/response command and dataset wiring | `dimse/c_find.go` (`CFindWriteRQ`, `CFindReadRSP`, `CFindWriteRSP`) with required SOP Class, request object, MessageID validation, valid `CommandDataSetType` enforcement, and pending-dataset consistency checks | `dimse/dimse_test.go` C-FIND block including missing SOP Class, missing MessageID, nil request, invalid `CommandDataSetType`, pending-without-dataset errors, and status-matrix tests | partial |
| PS3.7-C.4.2-CMOVE-RQ-FIELDS | C.4.2.1, C.4.2.1.1 | C-MOVE-RQ required command fields and validation | `dimse/c_move.go` (`CMoveReadRQ`, `validateCMoveRequest`, `CMoveWriteRQ`) | `dimse/dimse_test.go` C-MOVE-RQ parsing/writing blocks | partial |
| PS3.7-C.4.2-CMOVE-RSP-COUNTERS | C.4.2.1.9 | C-MOVE-RSP includes all four sub-operation counters in every response | `dimse/c_move.go` (`CMoveReadRSP`, `GetCMoveResponseStats`, `CMoveWriteRSP`) with strict `CommandDataSetType` validation on read | `dimse/dimse_test.go` C-MOVE response stats block plus status-and-pending matrix, status propagation matrix, and invalid `CommandDataSetType` error tests | partial |
| PS3.7-C.4.3-CGET-RSP-COUNTERS | C.4.3 | C-GET-RSP counter behavior and status handling | `dimse/c_get.go` (`CGetWriteRQ`, `CGetReadRSP`, `CGetWriteRSP`) with required SOP Class and MessageID validation in writers and strict `CommandDataSetType` validation on read | `dimse/dimse_test.go` C-GET block including missing SOP Class/missing MessageID negative tests, status-and-pending matrix tests, status propagation matrix tests, and invalid `CommandDataSetType` error tests | partial |
| PS3.7-C.5-NSERVICE-RSP | C.5 | N-service response command writer behavior | `dimse/n_service.go` (`NWriteRSP`) | `dimse/dimse_test.go` N-SERVICE Generic Response block | partial |

## Gaps Identified In This Pass

1. No complete requirement-by-requirement negative test matrix for missing required fields across all command types (C-STORE, C-FIND, C-GET, and C-ECHO baselines now in place).
2. Status-code matrix tests now cover C-FIND/C-GET/C-MOVE transitions; remaining work is to encode section-specific status constraints per service class tables (including disallowed combinations).
3. C-CANCEL semantics are currently limited by synchronous per-association request processing; advanced cancel semantics need explicit conformance position and tests.

## Next Actions

1. Expand this matrix into requirement-level rows for each mandatory command attribute per service.
2. Add table-driven negative tests for each mandatory field omission and invalid command field values.
3. Add a status-code conformance suite for C-STORE/C-FIND/C-GET/C-MOVE covering pending, success, warning, and failure transitions.
