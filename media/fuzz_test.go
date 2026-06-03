package media

import (
	"os"
	"strings"
	"testing"
)

// FuzzNewDCMObjFromBytes exercises the full DICOM parse path (file meta + dataset)
// against arbitrary bytes. The only contract under fuzzing is that parsing
// untrusted input must never panic or hang — a returned error is an acceptable
// outcome. Run with: go test -run=^$ -fuzz=FuzzNewDCMObjFromBytes ./media
func FuzzNewDCMObjFromBytes(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("DICM"))

	// 128-byte preamble + "DICM" magic with no following meta — a common
	// truncation the parser must reject cleanly.
	preamble := make([]byte, 132)
	copy(preamble[128:], "DICM")
	f.Add(preamble)

	// Regression (found by this fuzzer): a file-meta TransferSyntaxUID
	// (0002,0010) carrying an empty value drove GetTransferSyntaxFromUID("")
	// into a negative slice bound. Kept as an explicit seed because the repo
	// gitignores testdata/, so Go's testdata/fuzz crasher corpus is not tracked.
	f.Add([]byte(strings.Repeat("0", 128) + "DICM\x02\x00\x10\x0000\x00\x00"))

	// Seed from a real DICOM file when present so the fuzzer starts from valid
	// structure and mutates outward.
	if data, err := os.ReadFile("../testdata/test.dcm"); err == nil {
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		obj, err := NewDCMObjFromBytes(data)
		if err != nil || obj == nil {
			return
		}
		// Exercise read accessors on whatever was parsed to surface latent
		// panics in tag/index handling.
		n := obj.TagCount()
		for i := 0; i < n; i++ {
			tag := obj.GetTagAt(i)
			if tag == nil {
				continue
			}
			_ = tag.GetString()
		}
	})
}
