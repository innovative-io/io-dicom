package services

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/innovative-io/io-dicom/dictionary/sopclass"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/dimse"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
)

// SCU - interface to a scu
type SCU interface {
	EchoSCU(timeout int) error
	FindSCU(Query media.DICOMObject, timeout int) (int, uint16, error)
	MoveSCU(destAET string, Query media.DICOMObject, timeout int) (uint16, error)
	StoreSCU(FileName string, timeout int) error
	SetOnCFindResult(f func(result media.DICOMObject))
	SetOnCMoveResult(f func(result media.DICOMObject))
	openAssociation(pdu network.PDUService, abstractSyntax string, transferSyntaxes []string, timeout int) error
	writeStoreRQ(pdu network.PDUService, DDO media.DICOMObject, SOPClassUID string) (uint16, error)
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
	if err := d.openAssociation(pdu, sopclass.Verification.UID, []string{}, timeout); err != nil {
		return err
	}
	defer pdu.Close()
	if err := dimse.CEchoWriteRQ(pdu); err != nil {
		return err
	}
	if err := dimse.CEchoReadRSP(pdu); err != nil {
		return err
	}
	return nil
}

func (d *scu) FindSCU(Query media.DICOMObject, timeout int) (int, uint16, error) {
	results := 0
	status := dicomstatus.Warning
	SOPClassUID := sopclass.StudyRootQueryRetrieveInformationModelFind

	pdu := network.NewPDUService()
	if err := d.openAssociation(pdu, SOPClassUID.UID, []string{}, timeout); err != nil {
		return results, status, err
	}
	defer pdu.Close()
	if err := dimse.CFindWriteRQ(pdu, Query); err != nil {
		return results, status, err
	}
	for status != dicomstatus.Success {
		ddo, s, err := dimse.CFindReadRSP(pdu)
		status = s
		if err != nil {
			return results, status, err
		}
		if (status == dicomstatus.Pending) || (status == dicomstatus.PendingWithWarnings) {
			results++
			if d.onCFindResult != nil {
				d.onCFindResult(ddo)
			} else {
				slog.Warn("No onCFindResult event found")
			}
		}
	}

	return results, status, nil
}

func (d *scu) MoveSCU(destAET string, Query media.DICOMObject, timeout int) (uint16, error) {
	var pending int
	status := dicomstatus.Pending
	SOPClassUID := sopclass.StudyRootQueryRetrieveInformationModelMove

	pdu := network.NewPDUService()
	if err := d.openAssociation(pdu, SOPClassUID.UID, []string{}, timeout); err != nil {
		return dicomstatus.FailureUnableToProcess, err
	}
	defer pdu.Close()
	if err := dimse.CMoveWriteRQ(pdu, Query, destAET); err != nil {
		return dicomstatus.FailureUnableToProcess, err
	}

	for status == dicomstatus.Pending {
		ddo, s, err := dimse.CMoveReadRSP(pdu, &pending)
		status = s
		if err != nil {
			return dicomstatus.FailureUnableToProcess, err
		}
		if d.onCMoveResult != nil {
			d.onCMoveResult(ddo)
		} else {
			slog.Warn("No onCMoveResult event found")
		}
	}
	return status, nil
}

func (d *scu) StoreSCU(FileName string, timeout int) error {
	DDO, err := media.NewDCMObjFromFile(FileName)
	if err != nil {
		return err
	}

	SOPClassUID := DDO.GetString(tags.SOPClassUID)
	if len(SOPClassUID) > 0 {
		pdu := network.NewPDUService()
		err := d.openAssociation(pdu, SOPClassUID, []string{DDO.GetTransferSyntax().UID}, timeout)
		if err != nil {
			return err
		}
		defer pdu.Close()
		r, err := d.writeStoreRQ(pdu, DDO, SOPClassUID)
		if err != nil {
			return err
		}
		if r != dicomstatus.Success {
			return errors.New("serviceuser::StoreSCU, dimse.CStoreReadRSP failed")
		}
		c, err := dimse.CStoreReadRSP(pdu)
		if err != nil {
			return err
		}
		if c != dicomstatus.Success {
			return fmt.Errorf("serviceuser::StoreSCU, dimse.CStoreReadRSP failed - %d", c)
		}
		return nil
	}
	return errors.New("serviceuser::StoreSCU, OpenAssociation failed, RAET: " + d.destination.CalledAE)
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
	PresContext := network.NewPresentationContext()
	PresContext.SetAbstractSyntax(abstractSyntax)
	for _, ts := range transferSyntaxes {
		PresContext.AddTransferSyntax(ts)
	}
	PresContext.AddTransferSyntax(transfersyntax.ExplicitVRLittleEndian.UID)
	pdu.AddPresContexts(PresContext)

	return pdu.Connect(d.destination.HostName, strconv.Itoa(d.destination.Port))
}

func (d *scu) writeStoreRQ(pdu network.PDUService, DDO media.DICOMObject, SOPClassUID string) (uint16, error) {
	status := dicomstatus.FailureUnableToProcess

	PCID := pdu.GetPresentationContextID()
	if PCID == 0 {
		return dicomstatus.FailureUnableToProcess, errors.New("serviceuser::WriteStoreRQ, PCID==0")
	}
	TrnSyntOUT := pdu.GetTransferSyntax(PCID)

	if TrnSyntOUT == nil {
		return dicomstatus.FailureUnableToProcess, errors.New("serviceuser::WriteStoreRQ, TrnSyntOut is empty")
	}

	if TrnSyntOUT.UID == DDO.GetTransferSyntax().UID {
		if err := dimse.CStoreWriteRQ(pdu, DDO); err != nil {
			return status, err
		}
		return dicomstatus.Success, nil
	}

	DDO.SetTransferSyntax(TrnSyntOUT)
	DDO.SetExplicitVR(true)
	DDO.SetBigEndian(false)
	if TrnSyntOUT.UID == transfersyntax.ImplicitVRLittleEndian.UID {
		DDO.SetExplicitVR(false)
	}
	if TrnSyntOUT.UID == transfersyntax.ExplicitVRBigEndian.UID {
		DDO.SetBigEndian(true)
	}
	err := dimse.CStoreWriteRQ(pdu, DDO)
	if err != nil {
		return dicomstatus.FailureUnableToProcess, err
	}
	return dicomstatus.Success, nil
}
