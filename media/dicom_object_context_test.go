package media_test

import (
	"context"
	"errors"
	"testing"

	jpeg2000codec "github.com/innovative-io/io-dicom/codecs/jpeg2000"
	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/transcoder"
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
	// Restore the pure-Go default by name on cleanup. SetBackend(nil) would call
	// SelectDefault, which picks the highest-priority registered backend — and the
	// mock above registers at the default priority 0, outranking gojpeg2000 (-1).
	// That would leak the mock into later tests in this binary, so name the real
	// backend explicitly instead.
	t.Cleanup(func() {
		if err := jpeg2000codec.UseBackend("gojpeg2000"); err != nil {
			t.Errorf("cleanup: restoring gojpeg2000 backend failed: %v", err)
		}
	})
	if err := jpeg2000codec.UseBackend("media-context-jpeg2000"); err != nil {
		t.Fatalf("UseBackend failed: %v", err)
	}

	obj := media.NewEmptyDCMObj()
	obj.SetTransferSyntax(transfersyntax.JPEG2000Lossless)
	obj.SetExplicitVR(true)
	obj.SetBigEndian(false)

	obj.Write(tags.PhotometricInterpretation, "MONOCHROME2")
	obj.Write(tags.Rows, 1)
	obj.Write(tags.Columns, 1)
	obj.Write(tags.BitsAllocated, 8)
	obj.Write(tags.BitsStored, 8)
	obj.Write(tags.PixelRepresentation, 0)
	obj.Add(&media.DICOMTag{Group: 0x7FE0, Element: 0x0010, Length: 0xFFFFFFFF, VR: "OB", BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 0, VR: "DL", BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE000, Length: 1, VR: "DL", Data: []byte{0x01}, BigEndian: false})
	obj.Add(&media.DICOMTag{Group: 0xFFFE, Element: 0xE0DD, Length: 0, VR: "DL", BigEndian: false})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := transcoder.ChangeTransferSyntaxContext(ctx, obj, transfersyntax.ExplicitVRLittleEndian)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
