package media

import (
	"context"
	"errors"
	"testing"

	jpeg2000codec "github.com/innovative-io/io-dicom/codecs/jpeg2000"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

type mediaContextJPEG2000Backend struct{}

func (mediaContextJPEG2000Backend) Name() string {
	return "media-context-jpeg2000"
}

func (mediaContextJPEG2000Backend) SupportedTransferSyntaxUIDs() []string {
	return []string{transfersyntax.JPEG2000Lossless.UID}
}

func (mediaContextJPEG2000Backend) Decode([]byte, []byte) error {
	return errors.New("Decode called without context")
}

func (mediaContextJPEG2000Backend) Encode([]byte, uint16, uint16, uint16, uint16, int) ([]byte, error) {
	return nil, errors.New("unexpected Encode call")
}

func (mediaContextJPEG2000Backend) DecodeContext(ctx context.Context, _ []byte, _ []byte) error {
	return ctx.Err()
}

func (mediaContextJPEG2000Backend) EncodeContext(context.Context, []byte, uint16, uint16, uint16, uint16, int) ([]byte, error) {
	return nil, errors.New("unexpected EncodeContext call")
}

func TestChangeTransferSyntaxContextPropagatesCodecContext(t *testing.T) {
	if err := jpeg2000codec.RegisterBackend("media-context-jpeg2000", func() jpeg2000codec.Backend { return mediaContextJPEG2000Backend{} }); err != nil && err.Error() != "backend already registered" {
		t.Fatalf("RegisterBackend failed: %v", err)
	}
	jpeg2000codec.SetBackend(nil)
	t.Cleanup(func() { jpeg2000codec.SetBackend(nil) })
	if err := jpeg2000codec.UseBackend("media-context-jpeg2000"); err != nil {
		t.Fatalf("UseBackend failed: %v", err)
	}

	obj := &dicomObject{
		Tags:           make([]*DICOMTag, 0),
		TransferSyntax: transfersyntax.JPEG2000Lossless,
		ExplicitVR:     true,
		BigEndian:      false,
	}
	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.Rows, 1)
	obj.Write(tags.Columns, 1)
	obj.Write(tags.BitsAllocated, 8)
	obj.Write(tags.BitsStored, 8)
	obj.Write(tags.PixelRepresentation, 0)
	obj.Add(&DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB", BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 0, VR: "DL", BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 1, VR: "DL", Data: []byte{0x01}, BigEndian: false})
	obj.Add(&DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL", BigEndian: false})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := obj.ChangeTransferSyntaxContext(ctx, transfersyntax.ExplicitVRLittleEndian)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
