//go:build libjxl || ffmpeg || st2110

package media_test

import (
	"strings"
	"testing"

	"github.com/innovative-io/io-dicom/codecs"
	"github.com/innovative-io/io-dicom/dictionary/transfersyntax"
	"github.com/innovative-io/io-dicom/transcoder"
)

func TestRepresentativePixelTransferSyntaxRoundTripsWithNativeBackends(t *testing.T) {
	available := codecs.AvailableBackends()

	ranCount := 0
	for _, tc := range representativePixelRoundTripCases() {
		if tc.nativeFamily == "" || tc.nativeBackend == "" {
			continue
		}
		if !containsString(available[tc.nativeFamily], tc.nativeBackend) {
			continue
		}
		ranCount++

		t.Run(tc.name, func(t *testing.T) {
			cfg := codecs.BackendConfig{
				JPEG:      "passthrough",
				JPEGLS:    "passthrough",
				JPEG2000:  "passthrough",
				JPEGXL:    "passthrough",
				MPEG:      "passthrough",
				JPIP:      "passthrough",
				SMPTE2110: "passthrough",
			}
			switch tc.nativeFamily {
			case "jpeg":
				cfg.JPEG = tc.nativeBackend
			case "jpegls":
				cfg.JPEGLS = tc.nativeBackend
			case "jpeg2000":
				cfg.JPEG2000 = tc.nativeBackend
			case "jpegxl":
				cfg.JPEGXL = tc.nativeBackend
			case "mpeg":
				cfg.MPEG = tc.nativeBackend
			case "jpip":
				cfg.JPIP = tc.nativeBackend
			case "smpte2110":
				cfg.SMPTE2110 = tc.nativeBackend
			default:
				t.Fatalf("unsupported codec family %q", tc.nativeFamily)
			}

			forceCodecBackends(t, cfg)
			obj := tc.newObj()

			if err := transcoder.ChangeTransferSyntax(obj, tc.ts); err != nil {
				if tc.nativeToolingHint != "" && strings.Contains(err.Error(), tc.nativeToolingHint) {
					t.Skipf("native tooling unavailable for %s backend: %v", tc.nativeBackend, err)
				}
				t.Fatalf("ChangeTransferSyntax to %s failed: %v", tc.ts.Name, err)
			}
			if err := transcoder.ChangeTransferSyntax(obj, transfersyntax.ExplicitVRLittleEndian); err != nil {
				t.Fatalf("ChangeTransferSyntax back to ExplicitVRLittleEndian failed: %v", err)
			}

			assertRoundTripByMode(t, obj, tc.want, tc.nativeMode)
		})
	}

	if ranCount == 0 {
		t.Skip("no tagged native backends registered for representative roundtrip cases; run with codec build tags and cgo")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
