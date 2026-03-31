package codecs

import (
	"fmt"

	jpegcodec "github.com/innovative-io/io-dicom/codecs/jpeg"
	jpeg2000codec "github.com/innovative-io/io-dicom/codecs/jpeg2000"
	jpeglscodec "github.com/innovative-io/io-dicom/codecs/jpegls"
	jpegxlcodec "github.com/innovative-io/io-dicom/codecs/jpegxl"
	jpipcodec "github.com/innovative-io/io-dicom/codecs/jpip"
	mpegcodec "github.com/innovative-io/io-dicom/codecs/mpeg"
	smptecodec "github.com/innovative-io/io-dicom/codecs/smpte2110"
)

// BackendConfig defines backend names by codec family.
// Empty values leave the current backend unchanged.
type BackendConfig struct {
	JPEG      string
	JPEGLS    string
	JPEG2000  string
	JPEGXL    string
	MPEG      string
	JPIP      string
	SMPTE2110 string
}

// UseBackends applies backend selections across codec families.
func UseBackends(cfg BackendConfig) error {
	if cfg.JPEG != "" {
		if err := jpegcodec.UseBackend(cfg.JPEG); err != nil {
			return fmt.Errorf("jpeg backend: %w", err)
		}
	}
	if cfg.JPEGLS != "" {
		if err := jpeglscodec.UseBackend(cfg.JPEGLS); err != nil {
			return fmt.Errorf("jpegls backend: %w", err)
		}
	}
	if cfg.JPEG2000 != "" {
		if err := jpeg2000codec.UseBackend(cfg.JPEG2000); err != nil {
			return fmt.Errorf("jpeg2000 backend: %w", err)
		}
	}
	if cfg.JPEGXL != "" {
		if err := jpegxlcodec.UseBackend(cfg.JPEGXL); err != nil {
			return fmt.Errorf("jpegxl backend: %w", err)
		}
	}
	if cfg.MPEG != "" {
		if err := mpegcodec.UseBackend(cfg.MPEG); err != nil {
			return fmt.Errorf("mpeg backend: %w", err)
		}
	}
	if cfg.JPIP != "" {
		if err := jpipcodec.UseBackend(cfg.JPIP); err != nil {
			return fmt.Errorf("jpip backend: %w", err)
		}
	}
	if cfg.SMPTE2110 != "" {
		if err := smptecodec.UseBackend(cfg.SMPTE2110); err != nil {
			return fmt.Errorf("smpte2110 backend: %w", err)
		}
	}
	return nil
}

// AvailableBackends returns currently registered backend names for each codec family.
func AvailableBackends() map[string][]string {
	return map[string][]string{
		"jpeg":      jpegcodec.AvailableBackends(),
		"jpegls":    jpeglscodec.AvailableBackends(),
		"jpeg2000":  jpeg2000codec.AvailableBackends(),
		"jpegxl":    jpegxlcodec.AvailableBackends(),
		"mpeg":      mpegcodec.AvailableBackends(),
		"jpip":      jpipcodec.AvailableBackends(),
		"smpte2110": smptecodec.AvailableBackends(),
	}
}
