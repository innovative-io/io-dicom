package network

import (
	"bufio"
	"fmt"

	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network/internal/pdutype"
)

// PresentationDataTransfer - PresentationDataTransfer
type PresentationDataTransfer struct {
	ItemType              byte
	Reserved1             byte
	Length                uint32
	Buffer                *media.DICOMBuffer
	BlockSize             uint32
	MsgStatus             uint32
	Endian                uint32
	pdv                   PresentationDataValue
	PresentationContextID byte
	MsgHeader             byte
}

// ReadDynamic - ReadDynamic
func (pd *PresentationDataTransfer) ReadDynamic(buf *media.DICOMBuffer) (err error) {
	if pd.Length == 0 {
		if pd.Reserved1, err = buf.GetByte(); err != nil {
			return
		}
		if pd.Length, err = buf.ReadUint32(true); err != nil {
			return
		}
	}

	count := pd.Length

	pd.MsgStatus = 0

	for count > 0 {
		if pd.pdv.Length, err = buf.ReadUint32(true); err != nil {
			return err
		}
		if pd.pdv.PresentationContextID, err = buf.GetByte(); err != nil {
			return err
		}
		if pd.pdv.MsgHeader, err = buf.GetByte(); err != nil {
			return err
		}

		// ReadSlice returns a zero-copy view into buf's backing array —
		// buf is freshly allocated per PDU and never reused, so the slice
		// is stable for the lifetime of this call.
		pdvPayload, err := buf.ReadSlice(int(pd.pdv.Length - 2))
		if err != nil {
			return err
		}
		pd.Buffer.Write(pdvPayload, int(pd.pdv.Length-2))
		count = count - pd.pdv.Length - 4
		pd.Length = pd.Length - pd.pdv.Length - 4

		if pd.pdv.MsgHeader&PDVLastFragment > 0 {
			pd.MsgStatus = 1
			pd.PresentationContextID = pd.pdv.PresentationContextID
			return nil
		}
	}

	if pd.pdv.MsgHeader&PDVLastFragment > 0 {
		pd.MsgStatus = 1
	}

	pd.PresentationContextID = pd.pdv.PresentationContextID
	return nil
}
func (pd *PresentationDataTransfer) Write(rw *bufio.ReadWriter) error {
	TotalSize := uint32(pd.Buffer.GetSize())
	pd.Buffer.SetPosition(0)
	if pd.BlockSize == 0 {
		pd.BlockSize = 4096
	}

	SentSize := uint32(0)
	TLength := pd.Length

	// Edge case: empty dataset must still emit one terminating PDV so the remote
	// peer sees the last-fragment bit and unblocks from its NextPDU read.
	if TotalSize == 0 {
		pd.MsgHeader = pd.MsgHeader | PDVLastFragment
		pd.pdv.PresentationContextID = pd.PresentationContextID
		pd.pdv.MsgHeader = pd.MsgHeader
		pd.pdv.Length = 2 // 2 header bytes (PCID + MsgHeader), no payload
		pd.Length = pd.pdv.Length + 4
		pd.ItemType = pdutype.PDUDataTransfer
		pd.Reserved1 = 0
		buf := media.NewDICOMBuffer()
		buf.SetBigEndian(true)
		buf.WriteByte(pd.ItemType)
		buf.WriteByte(pd.Reserved1)
		buf.WriteUint32(pd.Length)
		buf.WriteUint32(pd.pdv.Length)
		buf.WriteByte(pd.pdv.PresentationContextID)
		buf.WriteByte(pd.MsgHeader)
		if err := buf.Send(rw); err != nil {
			return fmt.Errorf("pdata::Write: %w", err)
		}
		rw.Flush()
		pd.Length = TLength
		return nil
	}

	for SentSize < TotalSize {
		if (TotalSize - SentSize) < pd.BlockSize {
			pd.BlockSize = TotalSize - SentSize
		}
		if (pd.BlockSize + SentSize) == TotalSize {
			pd.MsgHeader = pd.MsgHeader | PDVLastFragment
		} else {
			pd.MsgHeader = pd.MsgHeader & PDVTypeMask
		}

		pd.pdv.PresentationContextID = pd.PresentationContextID
		pd.pdv.MsgHeader = pd.MsgHeader
		pd.pdv.Length = pd.BlockSize + 2
		pd.Length = pd.pdv.Length + 4
		pd.ItemType = pdutype.PDUDataTransfer
		pd.Reserved1 = 0
		buf := media.NewDICOMBuffer()

		buf.SetBigEndian(true)
		buf.WriteByte(pd.ItemType)
		buf.WriteByte(pd.Reserved1)
		buf.WriteUint32(pd.Length)
		buf.WriteUint32(pd.pdv.Length)
		buf.WriteByte(pd.pdv.PresentationContextID)
		buf.WriteByte(pd.MsgHeader)

		if err := buf.Send(rw); err != nil {
			return fmt.Errorf("pdata::Write: %w", err)
		}

		buff, err := pd.Buffer.Read(int(pd.BlockSize))
		if err != nil {
			return fmt.Errorf("pdata::Write: %w", err)
		}

		n, err := rw.Write(buff)
		if err != nil {
			return fmt.Errorf("pdata::Write: %w", err)
		}

		rw.Flush()

		if n != int(pd.BlockSize) {
			return fmt.Errorf("pdata::Write: wrote %d bytes, expected %d", n, pd.BlockSize)
		}

		SentSize += pd.BlockSize
	}
	pd.Length = TLength
	return nil
}
