package media

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
)

// TestWriteToMatchesWriteToBytes is the correctness contract for the streaming
// writer: WriteTo must produce a byte-for-byte identical result to the buffering
// WriteToBytes path, for every transfer syntax, including the deflated syntax
// that WriteTo deliberately falls back to buffering for.
func TestWriteToMatchesWriteToBytes(t *testing.T) {
	samples := []string{"../testdata/test2.dcm", "../testdata/jpeg8.dcm"}
	syntaxes := []*transfersyntax.TransferSyntax{
		nil, // keep the file's own transfer syntax
		transfersyntax.ExplicitVRLittleEndian,
		transfersyntax.ImplicitVRLittleEndian,
		transfersyntax.ExplicitVRBigEndian,
		transfersyntax.DeflatedExplicitVRLittleEndian,
	}

	for _, sample := range samples {
		if _, err := os.Stat(sample); err != nil {
			t.Skipf("sample fixture unavailable (%s): %v", sample, err)
		}
		for _, ts := range syntaxes {
			name := filepath.Base(sample) + "/original"
			if ts != nil {
				name = filepath.Base(sample) + "/" + ts.Name
			}
			t.Run(name, func(t *testing.T) {
				obj, err := NewDCMObjFromFile(sample)
				if err != nil {
					t.Fatalf("NewDCMObjFromFile: %v", err)
				}
				if ts != nil {
					obj.SetTransferSyntax(ts)
				}

				want := obj.WriteToBytes()
				if len(want) == 0 {
					t.Skip("object not serialisable under this transfer syntax")
				}

				var got bytes.Buffer
				n, err := obj.WriteTo(&got)
				if err != nil {
					t.Fatalf("WriteTo: %v", err)
				}
				if n != int64(got.Len()) {
					t.Fatalf("WriteTo reported %d bytes but wrote %d", n, got.Len())
				}
				if !bytes.Equal(want, got.Bytes()) {
					t.Fatalf("streamed output differs from buffered: len want=%d got=%d, first diff at %d",
						len(want), got.Len(), firstDiff(want, got.Bytes()))
				}
			})
		}
	}
}

// TestWriteToFileMatchesWriteToBytes covers the streaming WriteToFile path.
func TestWriteToFileMatchesWriteToBytes(t *testing.T) {
	const sample = "../testdata/test2.dcm"
	if _, err := os.Stat(sample); err != nil {
		t.Skipf("sample fixture unavailable: %v", err)
	}
	obj, err := NewDCMObjFromFile(sample)
	if err != nil {
		t.Fatalf("NewDCMObjFromFile: %v", err)
	}
	want := obj.WriteToBytes()

	out := filepath.Join(t.TempDir(), "streamed.dcm")
	if err := obj.WriteToFile(out); err != nil {
		t.Fatalf("WriteToFile: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("WriteToFile output differs from WriteToBytes: len want=%d got=%d, first diff at %d",
			len(want), len(got), firstDiff(want, got))
	}
	// The streamed file must still round-trip back into an object.
	if _, err := NewDCMObjFromFile(out); err != nil {
		t.Fatalf("streamed file does not parse back: %v", err)
	}
}

// BenchmarkWriteBuffered vs BenchmarkWriteStreamed contrast the two paths on the
// same object. The streamed path should allocate a small constant amount
// regardless of object size, while the buffered path allocates the whole object.
func BenchmarkWriteBuffered(b *testing.B) {
	obj := benchWriteObj(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if data := obj.WriteToBytes(); len(data) == 0 {
			b.Fatal("empty payload")
		}
	}
}

func BenchmarkWriteStreamed(b *testing.B) {
	obj := benchWriteObj(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := obj.WriteTo(discardWriter{}); err != nil {
			b.Fatal(err)
		}
	}
}

// discardWriter avoids io.Discard's ReadFrom fast path so both benchmarks
// measure the same work.
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func benchWriteObj(b *testing.B) DICOMObject {
	b.Helper()
	obj, err := NewDCMObjFromFile("../testdata/test2.dcm")
	if err != nil {
		b.Skipf("sample fixture unavailable: %v", err)
	}
	return obj
}

func firstDiff(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
