package codecs

import (
	"fmt"
	"sort"

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

// AvailableTransferSyntaxUIDs returns the supported transfer syntax UIDs by codec family.
func AvailableTransferSyntaxUIDs() map[string][]string {
	return map[string][]string{
		"jpeg":      jpegcodec.SupportedTransferSyntaxUIDs(),
		"jpegls":    jpeglscodec.SupportedTransferSyntaxUIDs(),
		"jpeg2000":  jpeg2000codec.SupportedTransferSyntaxUIDs(),
		"jpegxl":    jpegxlcodec.SupportedTransferSyntaxUIDs(),
		"mpeg":      mpegcodec.SupportedTransferSyntaxUIDs(),
		"jpip":      jpipcodec.SupportedTransferSyntaxUIDs(),
		"smpte2110": smptecodec.SupportedTransferSyntaxUIDs(),
	}
}

// NativeDefaults returns the single native backend, when one is registered for a family.
// Families with no native backend or multiple native choices are left unchanged.
func NativeDefaults() BackendConfig {
	return BackendConfig{
		JPEG:      nativeDefault(AvailableBackends()["jpeg"]),
		JPEGLS:    nativeDefault(AvailableBackends()["jpegls"]),
		JPEG2000:  nativeDefault(AvailableBackends()["jpeg2000"]),
		JPEGXL:    nativeDefault(AvailableBackends()["jpegxl"]),
		MPEG:      nativeDefault(AvailableBackends()["mpeg"]),
		JPIP:      nativeDefault(AvailableBackends()["jpip"]),
		SMPTE2110: nativeDefault(AvailableBackends()["smpte2110"]),
	}
}

func nativeDefault(names []string) string {
	native := ""
	for _, name := range names {
		if name == "passthrough" {
			continue
		}
		if native != "" {
			return ""
		}
		native = name
	}
	return native
}

// UseNativeDefaults switches each codec family to its single registered native backend, if present,
// and validates that the selected backends are ready for use.
func UseNativeDefaults() error {
	cfg := NativeDefaults()
	if err := UseBackends(cfg); err != nil {
		return err
	}
	return ValidateCurrentBackends()
}

// ValidateBackends probes the supplied backend names for readiness.
func ValidateBackends(cfg BackendConfig) error {
	if cfg.JPEG != "" {
		if err := jpegcodec.ValidateBackend(cfg.JPEG); err != nil {
			return fmt.Errorf("jpeg backend: %w", err)
		}
	}
	if cfg.JPEGLS != "" {
		if err := jpeglscodec.ValidateBackend(cfg.JPEGLS); err != nil {
			return fmt.Errorf("jpegls backend: %w", err)
		}
	}
	if cfg.JPEG2000 != "" {
		if err := jpeg2000codec.ValidateBackend(cfg.JPEG2000); err != nil {
			return fmt.Errorf("jpeg2000 backend: %w", err)
		}
	}
	if cfg.JPEGXL != "" {
		if err := jpegxlcodec.ValidateBackend(cfg.JPEGXL); err != nil {
			return fmt.Errorf("jpegxl backend: %w", err)
		}
	}
	if cfg.MPEG != "" {
		if err := mpegcodec.ValidateBackend(cfg.MPEG); err != nil {
			return fmt.Errorf("mpeg backend: %w", err)
		}
	}
	if cfg.JPIP != "" {
		if err := jpipcodec.ValidateBackend(cfg.JPIP); err != nil {
			return fmt.Errorf("jpip backend: %w", err)
		}
	}
	if cfg.SMPTE2110 != "" {
		if err := smptecodec.ValidateBackend(cfg.SMPTE2110); err != nil {
			return fmt.Errorf("smpte2110 backend: %w", err)
		}
	}
	return nil
}

// ValidateCurrentBackends probes the active backend for each codec family.
func ValidateCurrentBackends() error {
	return ValidateBackends(BackendConfig{
		JPEG:      jpegcodec.BackendName(),
		JPEGLS:    jpeglscodec.BackendName(),
		JPEG2000:  jpeg2000codec.BackendName(),
		JPEGXL:    jpegxlcodec.BackendName(),
		MPEG:      mpegcodec.BackendName(),
		JPIP:      jpipcodec.BackendName(),
		SMPTE2110: smptecodec.BackendName(),
	})
}

// ResolveBackendForUID returns the codec family that handles the transfer syntax UID.
func ResolveBackendForUID(uid string) (string, bool) {
	available := AvailableTransferSyntaxUIDs()
	families := make([]string, 0, len(available))
	for family := range available {
		families = append(families, family)
	}
	sort.Strings(families)
	for _, family := range families {
		for _, supportedUID := range available[family] {
			if supportedUID == uid {
				return family, true
			}
		}
	}
	return "", false
}
