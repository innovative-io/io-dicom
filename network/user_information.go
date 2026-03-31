package network

import (
	"bufio"
	"errors"
	"strconv"

	"github.com/innovative-io/io-dicom/media"
)

// UserInformation - UserInformation
type UserInformation interface {
	GetItemType() byte
	SetItemType(t byte)
	GetAsyncOperationWindow() AsyncOperationWindow
	GetMaxSubLength() MaximumPDULength
	SetMaxSubLength(length MaximumPDULength)
	Size() uint16
	GetImplementationClass() UIDItem
	SetImplementationClassUID(name string)
	GetImplementationVersion() UIDItem
	SetImplementationVersionName(name string)
	Write(rw *bufio.ReadWriter) (err error)
	Read(ms media.MemoryStream) (err error)
	ReadDynamic(ms media.MemoryStream) (err error)
}

type userInformation struct {
	ItemType        byte //0x50
	Reserved1       byte
	Length          uint16
	UserInfoBaggage uint32
	MaxSubLength    MaximumPDULength
	AsyncOpWindow   AsyncOperationWindow
	SCPSCURole      RoleSelect
	ImpClass        uidItem
	ImpVersion      uidItem
}

// NewUserInformation - NewUserInformation
func NewUserInformation() UserInformation {
	return &userInformation{
		ItemType:      0x50,
		MaxSubLength:  NewMaximumPDULength(),
		AsyncOpWindow: NewAsyncOperationWindow(),
		SCPSCURole:    NewRoleSelect(),
		ImpClass: uidItem{
			itemType: 0x52,
		},
		ImpVersion: uidItem{
			itemType: 0x55,
		},
	}
}

func (ui *userInformation) GetItemType() byte {
	return ui.ItemType
}

func (ui *userInformation) SetItemType(t byte) {
	ui.ItemType = t
}

func (ui *userInformation) GetMaxSubLength() MaximumPDULength {
	return ui.MaxSubLength
}

func (ui *userInformation) GetAsyncOperationWindow() AsyncOperationWindow {
	return ui.AsyncOpWindow
}

func (ui *userInformation) SetMaxSubLength(length MaximumPDULength) {
	ui.MaxSubLength = length
}

func (ui *userInformation) Size() uint16 {
	ui.Length = ui.MaxSubLength.Size()
	ui.Length += ui.ImpClass.GetSize()
	ui.Length += ui.ImpVersion.GetSize()
	return ui.Length + 4
}

func (ui *userInformation) GetImplementationClass() UIDItem {
	return &ui.ImpClass
}

func (ui *userInformation) SetImplementationClassUID(name string) {
	ui.ImpClass.SetType(0x52)
	ui.ImpClass.SetReserved(0x00)
	ui.ImpClass.SetUID(name)
	ui.ImpClass.SetLength(uint16(len(name)))
}

func (ui *userInformation) GetImplementationVersion() UIDItem {
	return &ui.ImpVersion
}

func (ui *userInformation) SetImplementationVersionName(name string) {
	ui.ImpVersion.SetType(0x55)
	ui.ImpVersion.SetReserved(0x00)
	ui.ImpVersion.SetUID(name)
	ui.ImpVersion.SetLength(uint16(len(name)))
}

func (ui *userInformation) Write(rw *bufio.ReadWriter) (err error) {
	bd := media.NewEmptyBufData()

	bd.SetBigEndian(true)
	ui.Size()
	bd.WriteByte(ui.ItemType)
	bd.WriteByte(ui.Reserved1)
	bd.WriteUint16(ui.Length)

	if err = bd.Send(rw); err != nil {
		return err
	}

	ui.MaxSubLength.Write(rw)
	ui.ImpClass.Write(rw)
	ui.ImpVersion.Write(rw)

	return
}

func (ui *userInformation) Read(ms media.MemoryStream) (err error) {
	if ui.ItemType, err = ms.GetByte(); err != nil {
		return err
	}
	return ui.ReadDynamic(ms)
}

func (ui *userInformation) ReadDynamic(ms media.MemoryStream) (err error) {
	if ui.Reserved1, err = ms.GetByte(); err != nil {
		return err
	}
	if ui.Length, err = ms.GetUint16(); err != nil {
		return err
	}

	Count := int(ui.Length)
	for Count > 0 {
		TempByte, err := ms.GetByte()
		if err != nil {
			return err
		}

		switch TempByte {
		case 0x51:
			ui.MaxSubLength.ReadDynamic(ms)
			Count = Count - int(ui.MaxSubLength.Size())
		case 0x52:
			ui.ImpClass.ReadDynamic(ms)
			Count = Count - int(ui.ImpClass.GetSize())
		case 0x53:
			ui.AsyncOpWindow.ReadDynamic(ms)
			Count = Count - int(ui.AsyncOpWindow.Size())
		case 0x54:
			ui.SCPSCURole.ReadDynamic(ms)
			Count = Count - int(ui.SCPSCURole.Size())
			ui.UserInfoBaggage += uint32(ui.SCPSCURole.Size())
		case 0x55:
			ui.ImpVersion.ReadDynamic(ms)
			Count = Count - int(ui.ImpVersion.GetSize())
		default:
			ui.UserInfoBaggage = uint32(Count)
			Count = -1
			return errors.New("user::ReadDynamic, unknown TempByte: " + strconv.Itoa(int(TempByte)))
		}
	}

	if Count == 0 {
		return nil
	}

	return errors.New("user::ReadDynamic, Count is not zero")
}
