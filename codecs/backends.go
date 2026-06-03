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

// uidToFamily maps every static transfer syntax UID to its codec family name.
// Built once at init time; ResolveBackendForUID lookups are O(1).
var uidToFamily map[string]string

func init() {
	raw := map[string][]string{
		"jpeg":      jpegcodec.SupportedTransferSyntaxUIDs(),
		"jpegls":    jpeglscodec.SupportedTransferSyntaxUIDs(),
		"jpeg2000":  jpeg2000codec.SupportedTransferSyntaxUIDs(),
		"jpegxl":    jpegxlcodec.SupportedTransferSyntaxUIDs(),
		"mpeg":      mpegcodec.SupportedTransferSyntaxUIDs(),
		"jpip":      jpipcodec.SupportedTransferSyntaxUIDs(),
		"smpte2110": smptecodec.SupportedTransferSyntaxUIDs(),
	}
	m := make(map[string]string)
	for family, uids := range raw {
		for _, uid := range uids {
			m[uid] = family
		}
	}
	uidToFamily = m
}

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

// applyBackendConfig iterates over every codec family in cfg. When validateOnly
// is true it calls ValidateBackend; otherwise it calls UseBackend.
func applyBackendConfig(cfg BackendConfig, validateOnly bool) error {
	for _, b := range []struct {
		name     string
		value    string
		use      func(string) error
		validate func(string) error
	}{
		{"jpeg", cfg.JPEG, jpegcodec.UseBackend, jpegcodec.ValidateBackend},
		{"jpegls", cfg.JPEGLS, jpeglscodec.UseBackend, jpeglscodec.ValidateBackend},
		{"jpeg2000", cfg.JPEG2000, jpeg2000codec.UseBackend, jpeg2000codec.ValidateBackend},
		{"jpegxl", cfg.JPEGXL, jpegxlcodec.UseBackend, jpegxlcodec.ValidateBackend},
		{"mpeg", cfg.MPEG, mpegcodec.UseBackend, mpegcodec.ValidateBackend},
		{"jpip", cfg.JPIP, jpipcodec.UseBackend, jpipcodec.ValidateBackend},
		{"smpte2110", cfg.SMPTE2110, smptecodec.UseBackend, smptecodec.ValidateBackend},
	} {
		if b.value == "" {
			continue
		}
		fn := b.use
		if validateOnly {
			fn = b.validate
		}
		if err := fn(b.value); err != nil {
			return fmt.Errorf("%s backend: %w", b.name, err)
		}
	}
	return nil
}

// UseBackends applies backend selections across codec families.
func UseBackends(cfg BackendConfig) error {
	return applyBackendConfig(cfg, false)
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
	available := AvailableBackends()
	return BackendConfig{
		JPEG:      nativeDefault(available["jpeg"]),
		JPEGLS:    nativeDefault(available["jpegls"]),
		JPEG2000:  nativeDefault(available["jpeg2000"]),
		JPEGXL:    nativeDefault(available["jpegxl"]),
		MPEG:      nativeDefault(available["mpeg"]),
		JPIP:      nativeDefault(available["jpip"]),
		SMPTE2110: nativeDefault(available["smpte2110"]),
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
	return applyBackendConfig(cfg, true)
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
// Lookup is O(1) via a map built at package init time.
func ResolveBackendForUID(uid string) (string, bool) {
	family, ok := uidToFamily[uid]
	return family, ok
}
