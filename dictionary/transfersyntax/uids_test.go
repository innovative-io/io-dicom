package transfersyntax

import (
	"reflect"
	"testing"
)

func TestGetTransferSyntaxFromName(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want *TransferSyntax
	}{
		{
			name: "Should get ILE transfer syntax",
			args: args{name: "ImplicitVRLittleEndian"},
			want: &TransferSyntax{
				UID:         "1.2.840.10008.1.2",
				Name:        "ImplicitVRLittleEndian",
				Description: "Implicit VR Little Endian",
				Type:        "Transfer Syntax",
			},
		},
		{
			name: "Should get nil from invalid transfer syntax name",
			args: args{name: "DERP"},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetTransferSyntaxFromName(tt.args.name); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetTransferSyntaxFromName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTransferSyntaxFromUID(t *testing.T) {
	type args struct {
		uid string
	}
	tests := []struct {
		name string
		args args
		want *TransferSyntax
	}{
		{
			name: "Should get ILE transfer syntax",
			args: args{uid: "1.2.840.10008.1.2"},
			want: &TransferSyntax{
				UID:         "1.2.840.10008.1.2",
				Name:        "ImplicitVRLittleEndian",
				Description: "Implicit VR Little Endian",
				Type:        "Transfer Syntax",
			},
		},
		{
			name: "Should get nil from invalid transfer syntax UID",
			args: args{uid: "1.2.840.10008.1.2.00000"},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetTransferSyntaxFromUID(tt.args.uid); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetTransferSyntaxFromUID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSupportedTransferSyntax(t *testing.T) {
	tests := []struct {
		name string
		uid  string
		want bool
	}{
		{
			name: "supports JPEG 2000 lossless multi-component",
			uid:  JPEG2000MCLossless.UID,
			want: true,
		},
		{
			name: "supports JPEG 2000 multi-component",
			uid:  JPEG2000MC.UID,
			want: true,
		},
		{
			name: "supports HTJ2K lossless",
			uid:  HTJ2KLossless.UID,
			want: true,
		},
		{
			name: "supports HTJ2K lossless RPCL",
			uid:  HTJ2KLosslessRPCL.UID,
			want: true,
		},
		{
			name: "supports HTJ2K",
			uid:  HTJ2K.UID,
			want: true,
		},
		{
			name: "supports JPEG-LS lossless",
			uid:  JPEGLSLossless.UID,
			want: true,
		},
		{
			name: "supports JPEG-LS near-lossless",
			uid:  JPEGLSNearLossless.UID,
			want: true,
		},
		{
			name: "supports JPEG XL lossless",
			uid:  JPEGXLLossless.UID,
			want: true,
		},
		{
			name: "supports JPEG XL JPEG recompression",
			uid:  JPEGXLJPEGRecompression.UID,
			want: true,
		},
		{
			name: "supports JPEG XL",
			uid:  JPEGXL.UID,
			want: true,
		},
		{
			name: "supports MPEG2 MPML",
			uid:  MPEG2MPML.UID,
			want: true,
		},
		{
			name: "supports MPEG2 MPML fragmentable",
			uid:  MPEG2MPMLF.UID,
			want: true,
		},
		{
			name: "supports MPEG2 MPHL",
			uid:  MPEG2MPHL.UID,
			want: true,
		},
		{
			name: "supports MPEG2 MPHL fragmentable",
			uid:  MPEG2MPHLF.UID,
			want: true,
		},
		{
			name: "supports MPEG4 HP 4.1",
			uid:  MPEG4HP41.UID,
			want: true,
		},
		{
			name: "supports MPEG4 HP 4.1 fragmentable",
			uid:  MPEG4HP41F.UID,
			want: true,
		},
		{
			name: "supports MPEG4 HP 4.1 BD",
			uid:  MPEG4HP41BD.UID,
			want: true,
		},
		{
			name: "supports MPEG4 HP 4.1 BD fragmentable",
			uid:  MPEG4HP41BDF.UID,
			want: true,
		},
		{
			name: "supports MPEG4 HP 4.2 2D",
			uid:  MPEG4HP422D.UID,
			want: true,
		},
		{
			name: "supports MPEG4 HP 4.2 2D fragmentable",
			uid:  MPEG4HP422DF.UID,
			want: true,
		},
		{
			name: "supports MPEG4 HP 4.2 3D",
			uid:  MPEG4HP423D.UID,
			want: true,
		},
		{
			name: "supports MPEG4 HP 4.2 3D fragmentable",
			uid:  MPEG4HP423DF.UID,
			want: true,
		},
		{
			name: "supports MPEG4 HP 4.2 stereo",
			uid:  MPEG4HP42STEREO.UID,
			want: true,
		},
		{
			name: "supports MPEG4 HP 4.2 stereo fragmentable",
			uid:  MPEG4HP42STEREOF.UID,
			want: true,
		},
		{
			name: "supports HEVC main profile",
			uid:  HEVCMP51.UID,
			want: true,
		},
		{
			name: "supports HEVC main 10 profile",
			uid:  HEVCM10P51.UID,
			want: true,
		},
		{
			name: "supports SMPTE ST 2110-20 uncompressed progressive active video",
			uid:  SMPTEST211020UncompressedProgressiveActiveVideo.UID,
			want: true,
		},
		{
			name: "supports SMPTE ST 2110-20 uncompressed interlaced active video",
			uid:  SMPTEST211020UncompressedInterlacedActiveVideo.UID,
			want: true,
		},
		{
			name: "supports SMPTE ST 2110-30 PCM digital audio",
			uid:  SMPTEST211030PCMDigitalAudio.UID,
			want: true,
		},
		{
			name: "supports JPIP HTJ2K referenced",
			uid:  JPIPHTJ2KReferenced.UID,
			want: true,
		},
		{
			name: "supports JPIP HTJ2K referenced deflate",
			uid:  JPIPHTJ2KReferencedDeflate.UID,
			want: true,
		},
		{
			name: "does not support retired JPIP referenced",
			uid:  JPIPReferenced.UID,
			want: false,
		},
		{
			name: "does not support retired JPIP referenced deflate",
			uid:  JPIPReferencedDeflate.UID,
			want: false,
		},
		{
			name: "does not support retired explicit VR big endian",
			uid:  ExplicitVRBigEndian.UID,
			want: false,
		},
		{
			name: "does not support retired JPEG extended hierarchical",
			uid:  JPEGExtendedHierarchical1618.UID,
			want: false,
		},
		{
			name: "does not support retired XML encoding",
			uid:  XMLEncoding.UID,
			want: false,
		},
		{
			name: "does not support retired RFC2557 MIME encapsulation",
			uid:  RFC2557MIMEEncapsulation.UID,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SupportedTransferSyntax(tt.uid); got != tt.want {
				t.Errorf("SupportedTransferSyntax() = %v, want %v", got, tt.want)
			}
		})
	}
}
