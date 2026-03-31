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

func TestResolveBackendForUID(t *testing.T) {
	family, ok := ResolveBackendForUID("1.2.840.10008.1.2.4.90")
	if !ok || family != "jpeg2000" {
		t.Fatalf("unexpected resolver result: family=%q ok=%v", family, ok)
	}

	if family, ok := ResolveBackendForUID("1.2.3"); ok || family != "" {
		t.Fatalf("expected unresolved UID, got family=%q ok=%v", family, ok)
	}
}

func TestAvailableTransferSyntaxUIDs(t *testing.T) {
	available := AvailableTransferSyntaxUIDs()
	if len(available["jpeg"]) == 0 {
		t.Fatal("expected jpeg transfer syntaxes")
	}
	if len(available["jpip"]) == 0 {
		t.Fatal("expected jpip transfer syntaxes")
	}
}

func TestNativeDefaultsMatchesRegisteredBackends(t *testing.T) {
	defaults := NativeDefaults()
	available := AvailableBackends()

	assertDefault := func(family string, got string) {
		t.Helper()
		nativeCount := 0
		lastNative := ""
		for _, name := range available[family] {
			if name == "passthrough" {
				continue
			}
			nativeCount++
			lastNative = name
		}
		expected := ""
		if nativeCount == 1 {
			expected = lastNative
		}
		if got != expected {
			t.Fatalf("unexpected native default for %s: got %q want %q", family, got, expected)
		}
	}

	assertDefault("jpeg", defaults.JPEG)
	assertDefault("jpegls", defaults.JPEGLS)
	assertDefault("jpeg2000", defaults.JPEG2000)
	assertDefault("jpegxl", defaults.JPEGXL)
	assertDefault("mpeg", defaults.MPEG)
	assertDefault("jpip", defaults.JPIP)
	assertDefault("smpte2110", defaults.SMPTE2110)
}

func TestUseNativeDefaultsAndValidateCurrentBackends(t *testing.T) {
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

	if err := UseNativeDefaults(); err != nil {
		t.Fatalf("UseNativeDefaults failed: %v", err)
	}
	if err := ValidateCurrentBackends(); err != nil {
		t.Fatalf("ValidateCurrentBackends failed: %v", err)
	}

	defaults := NativeDefaults()
	assertBackend := func(family string, got string, expected string) {
		t.Helper()
		if expected == "" {
			expected = "passthrough"
		}
		if got != expected {
			t.Fatalf("unexpected backend for %s: got %q want %q", family, got, expected)
		}
	}

	assertBackend("jpeg", jpegcodec.BackendName(), defaults.JPEG)
	assertBackend("jpegls", jpeglscodec.BackendName(), defaults.JPEGLS)
	assertBackend("jpeg2000", jpeg2000codec.BackendName(), defaults.JPEG2000)
	assertBackend("jpegxl", jpegxlcodec.BackendName(), defaults.JPEGXL)
	assertBackend("mpeg", mpegcodec.BackendName(), defaults.MPEG)
	assertBackend("jpip", jpipcodec.BackendName(), defaults.JPIP)
	assertBackend("smpte2110", smptecodec.BackendName(), defaults.SMPTE2110)
}
