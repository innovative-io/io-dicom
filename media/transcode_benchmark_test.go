package media

// Transcoding benchmarks: each iteration is NewDCMObjFromBytes + ChangeTransferSyntax (fresh parse).
//
// Run without unit tests:
//
//	go test ./media -run '^$' -bench 'BenchmarkTranscode_' -benchmem -benchtime=1s
//
import (
	"os"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

// benchmarkTranscode measures parse + ChangeTransferSyntax per iteration (fresh object each time),
// which matches batch “read file → convert transfer syntax” workloads.
func benchmarkTranscode(b *testing.B, relPath string, outTS *transfersyntax.TransferSyntax) {
	b.Helper()
	muteParserLogsForBenchmarks()
	InitDict()

	data, err := os.ReadFile(relPath)
	if err != nil {
		b.Skipf("benchmark sample %s: %v", relPath, err)
	}

	obj, err := NewDCMObjFromBytes(data)
	if err != nil {
		b.Fatalf("NewDCMObjFromBytes(%s): %v", relPath, err)
	}
	if err := obj.ChangeTransferSyntax(outTS); err != nil {
		b.Skipf("transcode %s → %s not available in this build: %v", relPath, outTS.Name, err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		o, err := NewDCMObjFromBytes(data)
		if err != nil {
			b.Fatalf("NewDCMObjFromBytes: %v", err)
		}
		if err := o.ChangeTransferSyntax(outTS); err != nil {
			b.Fatalf("ChangeTransferSyntax → %s: %v", outTS.Name, err)
		}
	}
}

// benchmarkTranscodeRoundTrip runs two ChangeTransferSyntax calls per iteration on a fresh parse.
func benchmarkTranscodeRoundTrip(b *testing.B, relPath string, midTS, backTS *transfersyntax.TransferSyntax) {
	b.Helper()
	muteParserLogsForBenchmarks()
	InitDict()

	data, err := os.ReadFile(relPath)
	if err != nil {
		b.Skipf("benchmark sample %s: %v", relPath, err)
	}

	obj, err := NewDCMObjFromBytes(data)
	if err != nil {
		b.Fatalf("NewDCMObjFromBytes(%s): %v", relPath, err)
	}
	if err := obj.ChangeTransferSyntax(midTS); err != nil {
		b.Skipf("transcode → %s: %v", midTS.Name, err)
	}
	if err := obj.ChangeTransferSyntax(backTS); err != nil {
		b.Skipf("transcode back → %s: %v", backTS.Name, err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		o, err := NewDCMObjFromBytes(data)
		if err != nil {
			b.Fatalf("NewDCMObjFromBytes: %v", err)
		}
		if err := o.ChangeTransferSyntax(midTS); err != nil {
			b.Fatalf("ChangeTransferSyntax → %s: %v", midTS.Name, err)
		}
		if err := o.ChangeTransferSyntax(backTS); err != nil {
			b.Fatalf("ChangeTransferSyntax → %s: %v", backTS.Name, err)
		}
	}
}

// --- test2.dcm (uncompressed explicit VR LE baseline) ---

func BenchmarkTranscode_Test2_to_ImplicitVRLittleEndian(b *testing.B) {
	benchmarkTranscode(b, "../samples/test2.dcm", transfersyntax.ImplicitVRLittleEndian)
}

func BenchmarkTranscode_Test2_to_EncapsulatedUncompressedExplicitVRLittleEndian(b *testing.B) {
	benchmarkTranscode(b, "../samples/test2.dcm", transfersyntax.EncapsulatedUncompressedExplicitVRLittleEndian)
}

func BenchmarkTranscode_Test2_to_RLELossless(b *testing.B) {
	benchmarkTranscode(b, "../samples/test2.dcm", transfersyntax.RLELossless)
}

func BenchmarkTranscode_Test2_to_JPEGBaseline8Bit(b *testing.B) {
	benchmarkTranscode(b, "../samples/test2.dcm", transfersyntax.JPEGBaseline8Bit)
}

func BenchmarkTranscode_Test2_to_JPEGLosslessSV1(b *testing.B) {
	benchmarkTranscode(b, "../samples/test2.dcm", transfersyntax.JPEGLosslessSV1)
}

func BenchmarkTranscode_Test2_to_JPEG2000(b *testing.B) {
	benchmarkTranscode(b, "../samples/test2.dcm", transfersyntax.JPEG2000)
}

func BenchmarkTranscode_Test2_to_DeflatedImageFrameCompression(b *testing.B) {
	benchmarkTranscode(b, "../samples/test2.dcm", transfersyntax.DeflatedImageFrameCompression)
}

func BenchmarkTranscode_Test2_roundtrip_ExplicitVR_via_RLE(b *testing.B) {
	benchmarkTranscodeRoundTrip(b, "../samples/test2.dcm", transfersyntax.RLELossless, transfersyntax.ExplicitVRLittleEndian)
}

// --- jpeg8.dcm (JPEG baseline compressed) → uncompressed ---
// Skips when JPEG decode needs a native backend (e.g. 12-bit path without -tags libjpeg).

func BenchmarkTranscode_JPEG8_to_ExplicitVRLittleEndian(b *testing.B) {
	benchmarkTranscode(b, "../samples/jpeg8.dcm", transfersyntax.ExplicitVRLittleEndian)
}

// --- rle_gray.dcm (RLE lossless) → uncompressed ---

func BenchmarkTranscode_RLEGray_to_ExplicitVRLittleEndian(b *testing.B) {
	benchmarkTranscode(b, "../samples/rle_gray.dcm", transfersyntax.ExplicitVRLittleEndian)
}
