package codecs

import (
	"testing"

	jpegcodec "github.com/innovative-io/io-dicom/codecs/jpeg"
	jpeg2000codec "github.com/innovative-io/io-dicom/codecs/jpeg2000"
	jpeglscodec "github.com/innovative-io/io-dicom/codecs/jpegls"
	jpegxlcodec "github.com/innovative-io/io-dicom/codecs/jpegxl"
	jpipcodec "github.com/innovative-io/io-dicom/codecs/jpip"
	mpegcodec "github.com/innovative-io/io-dicom/codecs/mpeg"
	smptecodec "github.com/innovative-io/io-dicom/codecs/smpte2110"
)

func TestAvailableBackendsIncludesPassthrough(t *testing.T) {
	available := AvailableBackends()
	keys := []string{"jpeg", "jpegls", "jpeg2000", "jpegxl", "mpeg", "jpip", "smpte2110"}
	for _, key := range keys {
		values, ok := available[key]
		if !ok {
			t.Fatalf("missing key %s", key)
		}
		found := false
		for _, name := range values {
			if name == "passthrough" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("passthrough backend not found for %s", key)
		}
	}
}

func TestUseBackendsPassthrough(t *testing.T) {
	jpegcodec.SetBackend(nil)
	jpeglscodec.SetBackend(nil)
	jpeg2000codec.SetBackend(nil)
	jpegxlcodec.SetBackend(nil)
	mpegcodec.SetBackend(nil)
	jpipcodec.SetBackend(nil)
	smptecodec.SetBackend(nil)

	t.Cleanup(func() {
		jpegcodec.SetBackend(nil)
		jpeglscodec.SetBackend(nil)
		jpeg2000codec.SetBackend(nil)
		jpegxlcodec.SetBackend(nil)
		mpegcodec.SetBackend(nil)
		jpipcodec.SetBackend(nil)
		smptecodec.SetBackend(nil)
	})

	err := UseBackends(BackendConfig{
		JPEG:      "passthrough",
		JPEGLS:    "passthrough",
		JPEG2000:  "passthrough",
		JPEGXL:    "passthrough",
		MPEG:      "passthrough",
		JPIP:      "passthrough",
		SMPTE2110: "passthrough",
	})
	if err != nil {
		t.Fatalf("UseBackends failed: %v", err)
	}

	if jpegcodec.BackendName() != "passthrough" {
		t.Fatalf("jpeg backend mismatch: %s", jpegcodec.BackendName())
	}
	if jpeglscodec.BackendName() != "passthrough" {
		t.Fatalf("jpegls backend mismatch: %s", jpeglscodec.BackendName())
	}
	if jpeg2000codec.BackendName() != "passthrough" {
		t.Fatalf("jpeg2000 backend mismatch: %s", jpeg2000codec.BackendName())
	}
	if jpegxlcodec.BackendName() != "passthrough" {
		t.Fatalf("jpegxl backend mismatch: %s", jpegxlcodec.BackendName())
	}
	if mpegcodec.BackendName() != "passthrough" {
		t.Fatalf("mpeg backend mismatch: %s", mpegcodec.BackendName())
	}
	if jpipcodec.BackendName() != "passthrough" {
		t.Fatalf("jpip backend mismatch: %s", jpipcodec.BackendName())
	}
	if smptecodec.BackendName() != "passthrough" {
		t.Fatalf("smpte2110 backend mismatch: %s", smptecodec.BackendName())
	}
}

func TestUseBackendsUnknown(t *testing.T) {
	err := UseBackends(BackendConfig{JPEG2000: "does-not-exist"})
	if err == nil {
		t.Fatal("expected UseBackends to fail for unknown backend")
	}
}
