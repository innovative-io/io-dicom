package pdutype

// AssociationRequest - Association request
const AssociationRequest = 0x01

// AssociationAccept - Association accept
const AssociationAccept = 0x02

// AssociationReject - Association reject
const AssociationReject = 0x03

// PDUDataTransfer - PDU Data
const PDUDataTransfer = 0x04

// AssociationReleaseRequest - Association release request
const AssociationReleaseRequest = 0x05

// AssociationReleaseResponse - Association release response
const AssociationReleaseResponse = 0x06

// AssociationAbortRequest - Association abort request
const AssociationAbortRequest = 0x07

// ApplicationContextItem is the item-type byte for an Application Context Name
// sub-item within an A-ASSOCIATE-RQ/AC PDU (PS3.8 Table 9-12).
const ApplicationContextItem byte = 0x10

// PresentationContextItem is the item-type byte for a Presentation Context
// sub-item in an A-ASSOCIATE-RQ PDU (PS3.8 Table 9-13).
const PresentationContextItem byte = 0x20

// PresentationContextAcceptItem is the item-type byte for a Presentation Context
// sub-item in an A-ASSOCIATE-AC PDU (PS3.8 Table 9-14).
const PresentationContextAcceptItem byte = 0x21

// AbstractSyntaxItem is the item-type byte for an Abstract Syntax sub-item
// within a Presentation Context (PS3.8 Table 9-14).
const AbstractSyntaxItem byte = 0x30

// TransferSyntaxItem is the item-type byte for a Transfer Syntax sub-item
// within a Presentation Context (PS3.8 Table 9-14).
const TransferSyntaxItem byte = 0x40

// UserInformationItem is the item-type byte for the User Information sub-item
// in an A-ASSOCIATE-RQ/AC PDU (PS3.8 Table 9-16).
const UserInformationItem byte = 0x50

// MaximumSubLengthItem is the item-type byte for the Maximum Length sub-item
// within User Information (PS3.8 Table D.1-1).
const MaximumSubLengthItem byte = 0x51

// ImplementationClassUIDItem is the item-type byte for the Implementation
// Class UID sub-item within User Information (PS3.7 Annex D.3.3.2).
const ImplementationClassUIDItem byte = 0x52

// AsyncOperationsWindowItem is the item-type byte for the Asynchronous
// Operations Window sub-item within User Information (PS3.7 Annex D.3.3.3).
const AsyncOperationsWindowItem byte = 0x53

// SCPSCURoleSelectionItem is the item-type byte for the SCP/SCU Role Selection
// sub-item within User Information (PS3.7 Annex D.3.3.4).
const SCPSCURoleSelectionItem byte = 0x54

// ImplementationVersionNameItem is the item-type byte for the Implementation
// Version Name sub-item within User Information (PS3.7 Annex D.3.3.2).
const ImplementationVersionNameItem byte = 0x55
