package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/dimse"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomcommand"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
	"github.com/innovative-io/io-dicom/network/priority"
)

// SCU - interface to a scu
type SCU interface {
	EchoSCU(ctx context.Context, timeout int) error
	FindSCU(ctx context.Context, Query media.DICOMObject, timeout int) (int, uint16, error)
	WorklistSCU(ctx context.Context, Query media.DICOMObject, timeout int) (int, uint16, error)
	MoveSCU(ctx context.Context, destAET string, Query media.DICOMObject, timeout int) (uint16, error)
	GetSCU(ctx context.Context, Query media.DICOMObject, timeout int) (uint16, error)
	StoreSCU(ctx context.Context, FileName string, timeout int) error
	SetOnCFindResult(f func(result media.DICOMObject))
	SetOnCMoveResult(f func(result media.DICOMObject))
	// SetOnCGetStore registers a callback invoked for each C-STORE sub-operation
	// received during a GetSCU call. Return the DICOM status to send back to the
	// SCP (use dicomstatus.Success to accept). If not set, all C-STOREs are
	// accepted with Success and the data is discarded.
	SetOnCGetStore(f func(data media.DICOMObject) uint16)
	SetOnRawPDU(f func(event network.RawPDUEvent))
	// SetPriority sets the DICOM priority (High/Medium/Low) used for all
	// outgoing C-FIND, C-GET, C-MOVE, and C-STORE requests.
	// Use the constants from network/priority: priority.High, priority.Medium, priority.Low.
	// The default is priority.Medium.
	SetPriority(pri uint16)
	// GetNegotiatedContexts returns the presentation contexts accepted by the
	// remote SCP after the most recent successful association. Returns nil if no
	// association has been made yet.
	GetNegotiatedContexts() []network.PresentationContextAccept
}

type scu struct {
	destination        *network.Destination
	priority           uint16
	onCFindResult      func(result media.DICOMObject)
	onCMoveResult      func(result media.DICOMObject)
	onCGetStore        func(data media.DICOMObject) uint16
	onRawPDU           func(event network.RawPDUEvent)
	negotiatedContexts []network.PresentationContextAccept
}

type associationPresentationContext struct {
	abstractSyntax   string
	transferSyntaxes []string
}

// NewSCU - Creates an interface to scu
func NewSCU(destination *network.Destination) SCU {
	return &scu{
		destination: destination,
		priority:    priority.Medium,
	}
}

func (d *scu) EchoSCU(ctx context.Context, timeout int) error {
	pdu := network.NewPDUService()
	defer pdu.Close()
	if err := d.openAssociation(ctx, pdu, sopclass.Verification.UID, []string{}, timeout); err != nil {
		return err
	}
	if err := dimse.CEchoWriteRQ(pdu); err != nil {
		return err
	}
	return dimse.CEchoReadRSP(pdu)
}

func (d *scu) FindSCU(ctx context.Context, Query media.DICOMObject, timeout int) (int, uint16, error) {
	return d.cfindSCU(ctx, sopclass.StudyRootQueryRetrieveInformationModelFind.UID, Query, timeout)
}

// WorklistSCU sends a Modality Worklist C-FIND (SOP 1.2.840.10008.5.1.4.31)
// and returns the match count, final status, and any error.
func (d *scu) WorklistSCU(ctx context.Context, Query media.DICOMObject, timeout int) (int, uint16, error) {
	return d.cfindSCU(ctx, sopclass.ModalityWorklistInformationModelFind.UID, Query, timeout)
}

// cfindSCU opens an association using abstractSyntax, sends a C-FIND request,
// and drains all pending matches before returning the total count, final
// status, and any transport or protocol error.
func (d *scu) cfindSCU(ctx context.Context, abstractSyntax string, Query media.DICOMObject, timeout int) (int, uint16, error) {
	results := 0
	status := dicomstatus.FailureProcessingFailure

	pdu := network.NewPDUService()
	defer pdu.Close()
	if err := d.openAssociation(ctx, pdu, abstractSyntax, []string{}, timeout); err != nil {
		return results, status, err
	}

	if err := dimse.CFindWriteRQ(pdu, Query, d.priority); err != nil {
		return results, status, err
	}

	for {
		ddo, s, err := dimse.CFindReadRSP(pdu)
		status = s
		if err != nil {
			return results, status, err
		}
		if status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings {
			results++
			if d.onCFindResult != nil {
				d.onCFindResult(ddo)
			}
			continue
		}
		// Success, Failure, Cancel, or Warning — loop is done.
		break
	}

	return results, status, nil
}

func (d *scu) MoveSCU(ctx context.Context, destAET string, Query media.DICOMObject, timeout int) (uint16, error) {
	var pending int

	pdu := network.NewPDUService()
	defer pdu.Close()
	if err := d.openAssociation(ctx, pdu, sopclass.StudyRootQueryRetrieveInformationModelMove.UID, []string{}, timeout); err != nil {
		return dicomstatus.FailureProcessingFailure, err
	}

	if err := dimse.CMoveWriteRQ(pdu, Query, destAET, d.priority); err != nil {
		return dicomstatus.FailureProcessingFailure, err
	}

	for {
		ddo, status, err := dimse.CMoveReadRSP(pdu, &pending)
		if err != nil {
			return dicomstatus.FailureProcessingFailure, err
		}
		if status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings {
			if d.onCMoveResult != nil {
				d.onCMoveResult(ddo)
			}
			continue
		}
		return status, nil
	}
}

func (d *scu) GetSCU(ctx context.Context, Query media.DICOMObject, timeout int) (uint16, error) {
	pdu := network.NewPDUService()
	defer pdu.Close()
	contexts := []associationPresentationContext{{
		abstractSyntax: sopclass.StudyRootQueryRetrieveInformationModelGet.UID,
	}}
	storageTransferSyntaxes := transfersyntax.GetSupportedTransferSyntaxUIDs()
	for _, storageClass := range sopclass.GetStorageSOPClasses() {
		contexts = append(contexts, associationPresentationContext{
			abstractSyntax:   storageClass.UID,
			transferSyntaxes: storageTransferSyntaxes,
		})
	}
	if err := d.openAssociationWithContexts(ctx, pdu, contexts, timeout); err != nil {
		return dicomstatus.FailureProcessingFailure, err
	}

	if err := dimse.CGetWriteRQ(pdu, Query, d.priority); err != nil {
		return dicomstatus.FailureProcessingFailure, err
	}

	// The C-GET SCP sends C-STORE-RQ sub-operations back over the same
	// association before sending the final C-GET-RSP. Handle both message types.
	for {
		dco, err := pdu.NextPDU()
		if err != nil {
			return dicomstatus.FailureProcessingFailure, err
		}
		switch dco.GetUint16(tags.CommandField) {
		case dicomcommand.CStoreRequest:
			// Read the image data object sent by the SCP.
			ddo, err := pdu.NextPDU()
			if err != nil {
				return dicomstatus.FailureProcessingFailure, fmt.Errorf("GetSCU: read C-STORE dataset: %w", err)
			}
			storeStatus := uint16(dicomstatus.Success)
			if d.onCGetStore != nil {
				storeStatus = d.onCGetStore(ddo)
			}
			if err := dimse.CStoreWriteRSP(pdu, dco, storeStatus); err != nil {
				return dicomstatus.FailureProcessingFailure, fmt.Errorf("GetSCU: write C-STORE-RSP: %w", err)
			}
		case dicomcommand.CGetResponse:
			status := dco.GetUint16(tags.Status)
			if status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings {
				continue
			}
			return status, nil
		default:
			return dicomstatus.FailureProcessingFailure, fmt.Errorf("GetSCU: unexpected command 0x%04X", dco.GetUint16(tags.CommandField))
		}
	}
}

func (d *scu) StoreSCU(ctx context.Context, FileName string, timeout int) error {
	DDO, err := media.NewDCMObjFromFile(FileName)
	if err != nil {
		return err
	}

	SOPClassUID := DDO.GetString(tags.SOPClassUID)
	if SOPClassUID == "" {
		return errors.New("scu: StoreSCU: missing SOPClassUID in DICOM file")
	}

	pdu := network.NewPDUService()
	defer pdu.Close()
	if err := d.openAssociation(ctx, pdu, SOPClassUID, []string{DDO.GetTransferSyntax().UID}, timeout); err != nil {
		return err
	}

	if err := d.writeStoreRQ(ctx, pdu, DDO); err != nil {
		return err
	}

	status, err := dimse.CStoreReadRSP(pdu)
	if err != nil {
		return err
	}
	if status != dicomstatus.Success {
		return fmt.Errorf("scu: StoreSCU: C-Store failed with status 0x%04X (%s)", status, dicomstatus.Description(status))
	}
	return nil
}

func (d *scu) SetOnCFindResult(f func(result media.DICOMObject)) {
	d.onCFindResult = f
}

func (d *scu) SetOnCMoveResult(f func(result media.DICOMObject)) {
	d.onCMoveResult = f
}

func (d *scu) SetOnCGetStore(f func(data media.DICOMObject) uint16) {
	d.onCGetStore = f
}

func (d *scu) SetOnRawPDU(f func(event network.RawPDUEvent)) {
	d.onRawPDU = f
}

func (d *scu) SetPriority(pri uint16) {
	d.priority = pri
}

func (d *scu) openAssociation(ctx context.Context, pdu network.PDUService, abstractSyntax string, transferSyntaxes []string, timeout int) error {
	return d.openAssociationWithContexts(ctx, pdu, []associationPresentationContext{{
		abstractSyntax:   abstractSyntax,
		transferSyntaxes: transferSyntaxes,
	}}, timeout)
}

func (d *scu) GetNegotiatedContexts() []network.PresentationContextAccept {
	return d.negotiatedContexts
}

func (d *scu) openAssociationWithContexts(ctx context.Context, pdu network.PDUService, contexts []associationPresentationContext, timeout int) error {
	pdu.SetCallingAE(d.destination.CallingAE)
	pdu.SetCalledAE(d.destination.CalledAE)
	pdu.SetTimeout(timeout)
	pdu.SetOnRawPDU(d.onRawPDU)

	network.Resetuniq()
	for _, contextSpec := range contexts {
		presContext := network.NewPresentationContext()
		presContext.SetAbstractSyntax(contextSpec.abstractSyntax)
		hasEVLE := false
		for _, ts := range contextSpec.transferSyntaxes {
			presContext.AddTransferSyntax(ts)
			if ts == transfersyntax.ExplicitVRLittleEndian.UID {
				hasEVLE = true
			}
		}
		// Always offer EVLE as a universal fallback so the remote SCP can accept
		// even if it does not support the native transfer syntax.
		if !hasEVLE {
			presContext.AddTransferSyntax(transfersyntax.ExplicitVRLittleEndian.UID)
		}
		pdu.AddPresContexts(presContext)
	}

	var err error
	if d.destination.IsTLS {
		err = pdu.ConnectTLS(ctx, d.destination.HostName, strconv.Itoa(d.destination.Port), d.destination.TLSConfig)
	} else {
		err = pdu.Connect(ctx, d.destination.HostName, strconv.Itoa(d.destination.Port))
	}
	if err == nil {
		d.negotiatedContexts = pdu.GetAcceptedPresentationContexts()
	}
	return err
}

func (d *scu) writeStoreRQ(ctx context.Context, pdu network.PDUService, DDO media.DICOMObject) error {
	PCID := pdu.GetPresentationContextID()
	if PCID == 0 {
		return errors.New("scu: writeStoreRQ: no accepted presentation context")
	}

	trnSyntOut := pdu.GetTransferSyntax(PCID)
	if trnSyntOut == nil {
		return errors.New("scu: writeStoreRQ: no negotiated transfer syntax for presentation context")
	}

	// Transcode to the negotiated transfer syntax if it differs from the file's.
	// ChangeTransferSyntaxContext is a no-op when both UIDs are the same, so this
	// is always safe to call. For compressed → uncompressed it decodes pixel data;
	// for VR-only differences (e.g. EVLE → ImplicitVRLE) it re-encodes in place.
	if err := DDO.ChangeTransferSyntaxContext(ctx, trnSyntOut); err != nil {
		return fmt.Errorf("scu: writeStoreRQ: transcode to %s: %w", trnSyntOut.Name, err)
	}

	return dimse.CStoreWriteRQ(pdu, DDO, d.priority)
}
