# PS3.8 Network Requirements Matrix

This matrix tracks DICOM PS3.8 UL/network conformance for `io-dicom`.

Scope: association lifecycle, release/abort behavior, and UL PDU field constraints.

## Requirement Mapping

| Requirement ID | PS3.8 Reference | Requirement Summary | Implementation Evidence | Test Evidence | Status |
| --- | --- | --- | --- | --- | --- |
| PS3.8-9.3.4-REJECT-CLOSE | 9.3.4 | Rejecting AE sends A-ASSOCIATE-RJ and closes transport | `network/pdu_service.go` rejection path writes RJ and closes transport on `ErrAssociationRejected` | `network/pdu_service_ulmachine_test.go` `TestInterogateAAssociateRQRejectsAndWritesRJ` and `TestNextPDUAssociationRejectClosesConnection` | aligned |
| PS3.8-T9.21-RJ-FIELDS | Table 9-21 | A-ASSOCIATE-RJ Source/Reason coding follows defined values | `network/a_association_rj.go` `Set` and writer layout | `network/pdu_service_ulmachine_test.go` validates RJ payload bytes on reject path | partial |
| PS3.8-T9.26-ABORT-RESERVED | Table 9-26 | A-ABORT reserved bytes and source values are valid | `network/a_abort_rq.go` sets `Reserved3` and source defaults | `network/pdu_service_ulmachine_test.go` `TestAbortRequestReservedByteIsZero`, `TestAbortRequestSourceIsValid` | aligned |
| PS3.8-RELEASE-HANDSHAKE | UL release procedure | Close path performs release request/response handshake and fallback abort on peer failure | `network/pdu_service.go` `Close` logic for release then abort fallback | `network/pdu_service_ulmachine_test.go` `TestClosePerformsReleaseHandshake`, `TestCloseAbortsWhenPeerDoesNotRespond` | partial |
| PS3.8-RELEASE-ABORT-SENTINELS | UL event handling | NextPDU surfaces release/abort as sentinel errors | `network/pdu_service.go` `NextPDU` release/abort cases | `network/pdu_service_ulmachine_test.go` `TestNextPDUReleaseRequestReturnsSentinel`, `TestNextPDUAbortReturnsSentinel` | aligned |

## Gaps Identified In This Pass

1. No section-level matrix yet for association negotiation constraints beyond current transfer syntax preference behavior.
2. **UPDATED**: UL state-transition matrix now created and table-driven tests implemented (see [ps3.8-ul-state-transition-matrix.md](ps3.8-ul-state-transition-matrix.md)).

## Next Actions

1. ✅ ~~Add table-driven UL state transition tests (association, data transfer, release, abort).~~ **COMPLETED** — See [ps3.8-ul-state-transition-matrix.md](ps3.8-ul-state-transition-matrix.md) with 11 transitions covered, 8 aligned.
2. Add outbound Connect tests (AWAITING-AC state validation, peer rejection handling).
3. Expand this matrix with requirement IDs tied to all implemented association negotiation paths (presentation context result codes, transfer syntax selection).
4. (Future) Create PS3.8 association negotiation constraints matrix.
