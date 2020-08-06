package network

import (
	"bufio"
	"errors"
	"log"
	"net"
	"time"

	"git.onebytedata.com/OneByteDataPlatform/go-dicom/media"
)

// PDUService - struct for PDUService
type PDUService interface {
	InterogateAAssociateAC() bool
	InterogateAAssociateRQ(rw *bufio.ReadWriter) error
	ParseDCMIntoRaw(DCO media.DcmObj) bool
	Write(DCO media.DcmObj, SOPClass string, ItemType byte) error
	GetTransferSyntaxUID(pcid byte) string
	ParseRawVRIntoDCM(DCO media.DcmObj) bool
	Read(DCO media.DcmObj) error
	SetTimeout(timeout int)
	Connect(IP string, Port string) error
	Close()
	Multiplex(rw *bufio.ReadWriter) error
	GetAAssociationRQ() AAssociationRQ
	GetACCalledAE() string
	SetACCalledAE(calledAE string)
	GetACCallingAE() string
	SetACCallingAE(callingAE string)
	GetRQCalledAE() string
	SetRQCalledAE(calledAE string)
	GetRQCallingAE() string
	SetRQCallingAE(callingAE string)
	AddPresContexts(presentationContext PresentationContext)
	GetPresentationContextID() byte
	SetOnAssociationRequest(f func(request AAssociationRQ) bool)
}

type pduService struct {
	AcceptedPresentationContexts []PresentationContextAccept
	rw                           *bufio.ReadWriter
	AssocRQ                      AAssociationRQ
	AssocAC                      AAssociationAC
	AssocRJ                      AAssociationRJ
	ReleaseRQ                    AReleaseRQ
	ReleaseRP                    AReleaseRP
	AbortRQ                      AAbortRQ
	Pdata                        PDataTF
	Timeout                      int
	OnAssociationRequest         func(request AAssociationRQ) bool
}

// NewPDUService - creates a pointer to PDUService
func NewPDUService() PDUService {
	return &pduService{
		AssocRQ:   NewAAssociationRQ(),
		AssocAC:   NewAAssociationAC(),
		AssocRJ:   NewAAssociationRJ(),
		ReleaseRQ: NewAReleaseRQ(),
		ReleaseRP: NewAReleaseRP(),
		AbortRQ:   NewAAbortRQ(),
	}
}

func (pdu *pduService) InterogateAAssociateAC() bool {
	var PresentationContextID byte
	TS := ""

	for _, presContextAccept := range pdu.AssocAC.GetPresContextAccepts() {
		if presContextAccept.GetResult() == 0 {
			pdu.AcceptedPresentationContexts = append(pdu.AcceptedPresentationContexts, presContextAccept)
			if len(TS) == 0 {
				if presContextAccept.GetTrnSyntax().UIDName == "1.2.840.10008.1.2.1" {
					TS = presContextAccept.GetTrnSyntax().UIDName
					PresentationContextID = presContextAccept.GetPresentationContextID()
				}
			}
			if len(TS) == 0 {
				if presContextAccept.GetTrnSyntax().UIDName == "1.2.840.10008.1.2" {
					TS = presContextAccept.GetTrnSyntax().UIDName
					PresentationContextID = presContextAccept.GetPresentationContextID()
				}
			}
		}
	}
	if (len(TS) > 0) && (len(pdu.AcceptedPresentationContexts) > 0) {
		pdu.Pdata.PresentationContextID = PresentationContextID
		return true
	}
	return false
}

func (pdu *pduService) InterogateAAssociateRQ(rw *bufio.ReadWriter) error {
	if pdu.OnAssociationRequest == nil || !pdu.OnAssociationRequest(pdu.AssocRQ) {
		pdu.AssocRJ.Set(1, 7)
		return pdu.AssocRJ.Write(rw)
	}

	pdu.AssocAC.SetCalledAE(pdu.AssocRQ.GetCalledAE())
	pdu.AssocAC.SetCallingAE(pdu.AssocRQ.GetCallingAE())

	pdu.AssocAC.SetAppContext(pdu.AssocRQ.GetAppContext())

	pdu.AssocAC.SetUserInformation(pdu.AssocRQ.GetUserInformation())

	for _, PresContext := range pdu.AssocRQ.GetPresContexts() {
		log.Printf("INFO, ASSOC-RQ: \tPresentation Context %s\n", PresContext.GetAbstractSyntax().UIDName)
		for _, TransferSyn := range PresContext.GetTransferSyntaxes() {
			log.Printf("INFO, ASSOC-RQ: \t\tTransfer Synxtax %s\n", TransferSyn.UIDName)
		}

		PresContextAccept := NewPresentationContextAccept()
		PresContextAccept.SetResult(4)
		PresContextAccept.SetTransferSyntax("")
		PresContextAccept.SetAbstractSyntax(PresContext.GetAbstractSyntax().UIDName)
		TS := ""
		for _, TrnSyntax := range PresContext.GetTransferSyntaxes() {
			if TrnSyntax.UIDName == "1.2.840.10008.1.2.1" {
				TS = TrnSyntax.UIDName
			}
		}
		if TS == "" {
			for _, TrnSyntax := range PresContext.GetTransferSyntaxes() {
				if TrnSyntax.UIDName == "1.2.840.10008.1.2" {
					TS = TrnSyntax.UIDName
				}
			}
		}
		if len(TS) > 0 {
			PresContextAccept.SetResult(0)
			PresContextAccept.SetTransferSyntax(TS)
			PresContextAccept.SetPresentationContextID(PresContext.GetPresentationContextID())
			pdu.AcceptedPresentationContexts = append(pdu.AcceptedPresentationContexts, PresContextAccept)
		}
		pdu.AssocAC.AddPresContextAccept(PresContextAccept)
	}

	if len(pdu.AcceptedPresentationContexts) > 0 {
		MaxSubLength := NewMaximumSubLength()
		UserInfo := NewUserInformation()

		MaxSubLength.SetMaximumLength(16384)
		UserInfo.SetImpClassUID("1.2.826.0.1.3680043.10.90.999")
		UserInfo.SetImpVersionName("One-Byte-Data")
		UserInfo.SetMaxSubLength(MaxSubLength)
		pdu.AssocAC.SetUserInformation(UserInfo)
		return pdu.AssocAC.Write(rw)
	}

	log.Println("ERROR, pduservice::InterogateAAssociateRQ, No valid AcceptedPresentationContexts")
	return pdu.AssocRJ.Write(rw)
}

func (pdu *pduService) ParseDCMIntoRaw(DCO media.DcmObj) bool {
	pdu.Pdata.Buffer.WriteObj(DCO)
	return true
}

func (pdu *pduService) Write(DCO media.DcmObj, SOPClass string, ItemType byte) error {
	if pdu.Pdata.Buffer != nil {
		pdu.Pdata.Buffer.ClearMemoryStream()
	} else {
		pdu.Pdata.Buffer = media.NewEmptyBufData()
	}

	if pdu.Pdata.PresentationContextID == 0 {
		return errors.New("ERROR, pduservice::Write, PresentationContextID==0")
	}
	if ItemType == 0x01 {
		if pdu.ParseDCMIntoRaw(DCO) == false {
			return errors.New("ERROR, pduservice::Write, ParseDCMIntoRaw failed")
		}
	} else {
		if pdu.ParseDCMIntoRaw(DCO) == false {
			return errors.New("ERROR, pduservice::Write, ParseDCMIntoRaw failed")
		}
	}

	pdu.Pdata.MsgHeader = ItemType
	if pdu.AssocAC.GetUserInformation().GetMaxSubLength().GetMaximumLength() > 16384 {
		pdu.AssocAC.SetMaxSubLength(16384)
	}

	pdu.Pdata.BlockSize = pdu.AssocAC.GetMaxSubLength()

	log.Printf("INFO, PDU-Service: %s --> %s", SOPClass, pdu.GetACCallingAE())

	return pdu.Pdata.Write(pdu.rw)
}

func (pdu *pduService) GetTransferSyntaxUID(pcid byte) string {
	for _, pca := range pdu.AcceptedPresentationContexts {
		if pca.GetPresentationContextID() == pcid {
			return pca.GetTrnSyntax().UIDName
		}
	}
	return ""
}

func (pdu *pduService) ParseRawVRIntoDCM(DCO media.DcmObj) bool {
	TrnSyntax := pdu.GetTransferSyntaxUID(pdu.Pdata.PresentationContextID)
	if len(TrnSyntax) == 0 {
		log.Println("ERROR, pduservice::ParseRawVRIntoDCM, TrnSyntax len is 0")
		return false
	}
	DCO.SetTransferSyntax(TrnSyntax)
	if TrnSyntax == "1.2.840.10008.1.2.1" {
		DCO.SetExplicitVR(true)
	}
	if TrnSyntax == "1.2.840.10008.1.2.2" {
		DCO.SetBigEndian(true)
	}
	pdu.Pdata.Buffer.SetPosition(0)
	return pdu.Pdata.Buffer.ReadObj(DCO)
}

func (pdu *pduService) Read(DCO media.DcmObj) error {
	if pdu.Pdata.Buffer != nil {
		pdu.Pdata.Buffer.ClearMemoryStream()
	} else {
		pdu.Pdata.Buffer = media.NewEmptyBufData()
	}

	pdu.Pdata.MsgStatus = 0
	if pdu.Pdata.Length != 0 {
		pdu.Pdata.ReadDynamic(pdu.rw)
		if pdu.Pdata.MsgStatus > 0 {
			if !pdu.ParseRawVRIntoDCM(DCO) {
				pdu.AbortRQ.Write(pdu.rw)
				return errors.New("ERROR, pduservice::Read, ParseRawVRIntoDCM failed")
			}
			return nil
		}
	}

	for {
		ItemType, err := ReadByte(pdu.rw)
		if err != nil {
			return err
		}

		switch ItemType {
		case 0x01: // A-Associate-RQ, should not get here
			log.Printf("INFO, ASSOC-RQ: %s --> %s\n", pdu.AssocRQ.GetCallingAE(), pdu.AssocRQ.GetCalledAE())
			pdu.AssocRQ.Read(pdu.rw)
			pdu.AbortRQ.Write(pdu.rw)
			return errors.New("ERROR, pduservice::Read, A-Associate-RQ")
		case 0x02: // A-Associate-AC, should not get here
			log.Printf("INFO, ASSOC-AC: %s <-- %s\n", pdu.AssocAC.GetCallingAE(), pdu.AssocAC.GetCalledAE())
			pdu.AssocAC.Read(pdu.rw)
			pdu.AbortRQ.Write(pdu.rw)
			return errors.New("ERROR, pduservice::Read, A-Associate-AC")
		case 0x03: // A-Associate-RJ, should not get here
			log.Printf("INFO, ASSOC-RJ: %s <-- %s\n", pdu.AssocRQ.GetCallingAE(), pdu.AssocRQ.GetCalledAE())
			pdu.AbortRQ.Write(pdu.rw)
			return errors.New("ERROR, pduservice::Read, A-Associate-RJ")
		case 0x04: // P-Data-TF
			err := pdu.Pdata.ReadDynamic(pdu.rw)
			if err != nil {
				return err
			}
			if pdu.Pdata.MsgStatus > 0 {
				if !pdu.ParseRawVRIntoDCM(DCO) {
					pdu.AbortRQ.Write(pdu.rw)
					return errors.New("ERROR, pduservice::Read, ParseRawVRIntoDCM failed")
				}
				return nil
			}
			break
		case 0x05: // A-Release-RQ
			log.Printf("INFO, ASSOC-R-RQ: %s --> %s\n", pdu.AssocRQ.GetCallingAE(), pdu.AssocRQ.GetCalledAE())
			pdu.ReleaseRQ.ReadDynamic(pdu.rw)
			pdu.ReleaseRP.Write(pdu.rw)
			return errors.New("ERROR, pduservice::Read, A-Release-RQ")
		case 0x06: // A-Release-RP
			log.Printf("INFO, ASSOC-R-RP: %s <-- %s\n", pdu.AssocRQ.GetCallingAE(), pdu.AssocRQ.GetCalledAE())
			return errors.New("ERROR, pduservice::Read, A-Release-RP")
		case 0x07: //A-Abort-RQ
			log.Printf("INFO, ASSOC-ABORT-RQ: %s --> %s\n", pdu.AssocRQ.GetCallingAE(), pdu.AssocRQ.GetCalledAE())
			return errors.New("ERROR, pduservice::Read, A-Abort-RQ")
		default:
			pdu.AbortRQ.Write(pdu.rw)
			return errors.New("ERROR, pduservice::Read, unknown ItemType")
		}
	}
}

func (pdu *pduService) SetTimeout(timeout int) {
	pdu.Timeout = timeout
}

func (pdu *pduService) Connect(IP string, Port string) error {
	conn, err := net.Dial("tcp", IP+":"+Port)
	if err != nil {
		return errors.New("ERROR, pduservice::Connect, " + err.Error())
	}

	conn.SetDeadline(time.Now().Add(time.Duration(int32(pdu.Timeout)) * time.Second))

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	pdu.rw = rw

	pdu.AssocRQ.SetMaxSubLength(16384)
	pdu.AssocRQ.SetImpClassUID("1.2.826.0.1.3680043.10.90.999")
	pdu.AssocRQ.SetImpVersionName("One-Byte-Data")

	err = pdu.AssocRQ.Write(pdu.rw)
	if err != nil {
		return err
	}

	ItemType, err := ReadByte(pdu.rw)
	if err != nil {
		return err
	}

	switch ItemType {
	case 0x02:
		pdu.AssocAC.ReadDynamic(pdu.rw)
		if !pdu.InterogateAAssociateAC() {
			return errors.New("ERROR, pduservice::Connect, InterogateAAssociateAC failed")
		}
		return nil
	case 0x03:
		// Error, Assoc. Rejected.
		pdu.AssocRJ.ReadDynamic(pdu.rw)
		return errors.New("ERROR, pduservice::Connect, Assoc. Rejected")
	default:
		// Error, Corrupt Transmission
		return errors.New("ERROR, pduservice::Connect, Corrupt Transmision")
	}
}

func (pdu *pduService) Close() {
	pdu.ReleaseRQ.Write(pdu.rw)
	pdu.ReleaseRP.Read(pdu.rw)
}

func (pdu *pduService) Multiplex(rw *bufio.ReadWriter) error {
	pdu.rw = rw
	err := pdu.AssocRQ.Read(pdu.rw)
	if err != nil {
		return err
	}

	err = pdu.InterogateAAssociateRQ(pdu.rw)
	if err != nil {
		return err
	}
	return nil
}

func (pdu *pduService) GetAAssociationRQ() AAssociationRQ {
	return pdu.AssocRQ
}

func (pdu *pduService) GetACCalledAE() string {
	return pdu.AssocAC.GetCalledAE()
}

func (pdu *pduService) SetACCalledAE(calledAE string) {
	pdu.AssocAC.SetCalledAE(calledAE)
}

func (pdu *pduService) GetACCallingAE() string {
	return pdu.AssocAC.GetCallingAE()
}

func (pdu *pduService) SetACCallingAE(callingAE string) {
	pdu.AssocAC.SetCallingAE(callingAE)
}

func (pdu *pduService) GetRQCalledAE() string {
	return pdu.AssocRQ.GetCalledAE()
}

func (pdu *pduService) SetRQCalledAE(calledAE string) {
	pdu.AssocRQ.SetCalledAE(calledAE)
}

func (pdu *pduService) GetRQCallingAE() string {
	return pdu.AssocRQ.GetCallingAE()
}

func (pdu *pduService) SetRQCallingAE(callingAE string) {
	pdu.AssocRQ.SetCallingAE(callingAE)
}

func (pdu *pduService) AddPresContexts(presentationContext PresentationContext) {
	pdu.AssocRQ.AddPresContexts(presentationContext)
}

func (pdu *pduService) GetPresentationContextID() byte {
	return pdu.Pdata.PresentationContextID
}

func (pdu *pduService) SetOnAssociationRequest(f func(request AAssociationRQ) bool) {
	pdu.OnAssociationRequest = f
}
