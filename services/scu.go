package services

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/dimse"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
	"github.com/innovative-io/io-dicom/network/priority"
)

// SCU - interface to a scu
type SCU interface {
	EchoSCU(timeout int) error
	FindSCU(Query media.DICOMObject, timeout int) (int, uint16, error)
	WorklistSCU(Query media.DICOMObject, timeout int) (int, uint16, error)
	MoveSCU(destAET string, Query media.DICOMObject, timeout int) (uint16, error)
	GetSCU(Query media.DICOMObject, timeout int) (uint16, error)
	StoreSCU(FileName string, timeout int) error
	SetOnCFindResult(f func(result media.DICOMObject))
	SetOnCMoveResult(f func(result media.DICOMObject))
}

type scu struct {
	destination   *network.Destination
	onCFindResult func(result media.DICOMObject)
	onCMoveResult func(result media.DICOMObject)
}

// NewSCU - Creates an interface to scu
func NewSCU(destination *network.Destination) SCU {
	return &scu{
		destination: destination,
	}
}

func (d *scu) EchoSCU(timeout int) error {
	pdu := network.NewPDUService()
	defer pdu.Close()
	if err := d.openAssociation(pdu, sopclass.Verification.UID, []string{}, timeout); err != nil {
		return err
	}
	if err := dimse.CEchoWriteRQ(pdu); err != nil {
		return err
	}
	return dimse.CEchoReadRSP(pdu)
}

func (d *scu) FindSCU(Query media.DICOMObject, timeout int) (int, uint16, error) {
	results := 0
	status := dicomstatus.FailureProcessingFailure

	pdu := network.NewPDUService()
	defer pdu.Close()
	if err := d.openAssociation(pdu, sopclass.StudyRootQueryRetrieveInformationModelFind.UID, []string{}, timeout); err != nil {
		return results, status, err
	}

	if err := dimse.CFindWriteRQ(pdu, Query); err != nil {
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

// WorklistSCU sends a Modality Worklist C-FIND (SOP 1.2.840.10008.5.1.4.31)
// and returns the match count, final status, and any error.
func (d *scu) WorklistSCU(Query media.DICOMObject, timeout int) (int, uint16, error) {
	results := 0
	status := dicomstatus.FailureProcessingFailure

	pdu := network.NewPDUService()
	defer pdu.Close()
	if err := d.openAssociation(pdu, sopclass.ModalityWorklistInformationModelFind.UID, []string{}, timeout); err != nil {
		return results, status, err
	}

	if err := dimse.CFindWriteRQ(pdu, Query); err != nil {
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
		break
	}

	return results, status, nil
}

func (d *scu) MoveSCU(destAET string, Query media.DICOMObject, timeout int) (uint16, error) {
	var pending int

	pdu := network.NewPDUService()
	defer pdu.Close()
	if err := d.openAssociation(pdu, sopclass.StudyRootQueryRetrieveInformationModelMove.UID, []string{}, timeout); err != nil {
		return dicomstatus.FailureProcessingFailure, err
	}

	if err := dimse.CMoveWriteRQ(pdu, Query, destAET, priority.Medium); err != nil {
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

func (d *scu) GetSCU(Query media.DICOMObject, timeout int) (uint16, error) {
	var pending int

	pdu := network.NewPDUService()
	defer pdu.Close()
	if err := d.openAssociation(pdu, sopclass.StudyRootQueryRetrieveInformationModelGet.UID, []string{}, timeout); err != nil {
		return dicomstatus.FailureProcessingFailure, err
	}

	if err := dimse.CGetWriteRQ(pdu, Query); err != nil {
		return dicomstatus.FailureProcessingFailure, err
	}

	for {
		_, status, err := dimse.CGetReadRSP(pdu, &pending)
		if err != nil {
			return dicomstatus.FailureProcessingFailure, err
		}
		if status == dicomstatus.Pending || status == dicomstatus.PendingWithWarnings {
			continue
		}
		return status, nil
	}
}

func (d *scu) StoreSCU(FileName string, timeout int) error {
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
	if err := d.openAssociation(pdu, SOPClassUID, []string{DDO.GetTransferSyntax().UID}, timeout); err != nil {
		return err
	}

	if err := d.writeStoreRQ(pdu, DDO); err != nil {
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

func (d *scu) openAssociation(pdu network.PDUService, abstractSyntax string, transferSyntaxes []string, timeout int) error {
	pdu.SetCallingAE(d.destination.CallingAE)
	pdu.SetCalledAE(d.destination.CalledAE)
	pdu.SetTimeout(timeout)

	network.Resetuniq()
	presContext := network.NewPresentationContext()
	presContext.SetAbstractSyntax(abstractSyntax)
	for _, ts := range transferSyntaxes {
		presContext.AddTransferSyntax(ts)
	}
	presContext.AddTransferSyntax(transfersyntax.ExplicitVRLittleEndian.UID)
	pdu.AddPresContexts(presContext)

	if d.destination.IsTLS {
		return pdu.ConnectTLS(d.destination.HostName, strconv.Itoa(d.destination.Port), d.destination.TLSConfig)
	}
	return pdu.Connect(d.destination.HostName, strconv.Itoa(d.destination.Port))
}

func (d *scu) writeStoreRQ(pdu network.PDUService, DDO media.DICOMObject) error {
	PCID := pdu.GetPresentationContextID()
	if PCID == 0 {
		return errors.New("scu: writeStoreRQ: no accepted presentation context")
	}

	trnSyntOut := pdu.GetTransferSyntax(PCID)
	if trnSyntOut == nil {
		return errors.New("scu: writeStoreRQ: no negotiated transfer syntax for presentation context")
	}

	// Only re-encode if the negotiated syntax differs from the file's syntax.
	if trnSyntOut.UID != DDO.GetTransferSyntax().UID {
		DDO.SetTransferSyntax(trnSyntOut)
		switch trnSyntOut.UID {
		case transfersyntax.ImplicitVRLittleEndian.UID:
			DDO.SetExplicitVR(false)
			DDO.SetBigEndian(false)
		case transfersyntax.ExplicitVRBigEndian.UID:
			DDO.SetExplicitVR(true)
			DDO.SetBigEndian(true)
		default:
			DDO.SetExplicitVR(true)
			DDO.SetBigEndian(false)
		}
	}

	return dimse.CStoreWriteRQ(pdu, DDO)
}
