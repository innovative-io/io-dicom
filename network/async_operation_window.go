package network

import (
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network/internal/pdutype"
)

// AsyncOperationWindow - AsyncOperationWindow
type AsyncOperationWindow interface {
	GetMaxNumberOperationsInvoked() uint16
	GetMaxNumberOperationsPerformed() uint16
	Size() uint16
	Read(buf *media.DICOMBuffer) (err error)
	ReadDynamic(buf *media.DICOMBuffer) (err error)
}

type asyncOperationWindow struct {
	ItemType                     byte //0x53
	Reserved1                    byte
	Length                       uint16
	MaxNumberOperationsInvoked   uint16
	MaxNumberOperationsPerformed uint16
}

// NewAsyncOperationWindow - NewAsyncOperationWindow
func NewAsyncOperationWindow() AsyncOperationWindow {
	return &asyncOperationWindow{
		ItemType: pdutype.AsyncOperationsWindowItem,
	}
}

func (async *asyncOperationWindow) GetMaxNumberOperationsInvoked() uint16 {
	return async.MaxNumberOperationsInvoked
}

func (async *asyncOperationWindow) GetMaxNumberOperationsPerformed() uint16 {
	return async.MaxNumberOperationsPerformed
}

func (async *asyncOperationWindow) Size() uint16 {
	return async.Length + 4
}

func (async *asyncOperationWindow) Read(buf *media.DICOMBuffer) (err error) {
	if async.ItemType, err = buf.GetByte(); err != nil {
		return err
	}
	return async.ReadDynamic(buf)
}

func (async *asyncOperationWindow) ReadDynamic(buf *media.DICOMBuffer) (err error) {
	if async.Reserved1, err = buf.GetByte(); err != nil {
		return err
	}
	if async.Length, err = buf.ReadUint16(true); err != nil {
		return err
	}
	if async.MaxNumberOperationsInvoked, err = buf.ReadUint16(true); err != nil {
		return err
	}
	if async.MaxNumberOperationsPerformed, err = buf.ReadUint16(true); err != nil {
		return err
	}
	return
}
