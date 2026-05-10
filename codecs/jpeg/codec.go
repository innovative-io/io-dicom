package jpeg

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"sort"
	"sync"
)

var CGOEnabled = nativeBackendEnabled

const maxCodecPayloadBytes = 512 << 20

var errBackendUnavailable = errors.New("jpeg 12/16-bit decode requires the libjpeg native backend (build with -tags libjpeg)")

var (
	errPayloadTooLarge    = errors.New("jpeg: payload exceeds maximum allowed size")
	errPayloadTruncated   = errors.New("jpeg: declared size exceeds slice length")
	errOutputTooSmall     = errors.New("jpeg: output buffer too small")
	errInvalidDimensions  = errors.New("jpeg: invalid image dimensions")
	errInputTooSmall      = errors.New("jpeg: input buffer too small")
	errUnsupportedSamples = errors.New("jpeg: unsupported number of samples per pixel")
	errNilOutputPointers  = errors.New("jpeg: nil output pointers")
)

var supportedTransferSyntaxUIDs = []string{
	"1.2.840.10008.1.2.4.50",
	"1.2.840.10008.1.2.4.51",
	"1.2.840.10008.1.2.4.57",
	"1.2.840.10008.1.2.4.70",
}

// SOFPrecision scans a JPEG payload and returns the sample precision (bits per
// SOFHeader holds the key fields from the first JPEG SOF (Start of Frame)
// segment. All fields are zero when the SOF cannot be found.
type SOFHeader struct {
	Precision uint8
	Height    uint16
	Width     uint16
}

// ReadSOFHeader scans a JPEG payload and returns the fields declared in the
// first SOF (Start of Frame) segment. Returns a zero-valued struct when the
// SOF cannot be found (e.g. the payload is too short or malformed).
// SOF markers: 0xC0–0xC3, 0xC5–0xC7, 0xC9–0xCB, 0xCD–0xCF (DHT 0xC4 and
// JPG 0xC8 are not SOF markers and are skipped).
func ReadSOFHeader(data []byte) SOFHeader {
	// Must start with SOI 0xFFD8
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return SOFHeader{}
	}
	i := 2
	for i+3 < len(data) {
		if data[i] != 0xFF {
			return SOFHeader{}
		}
		marker := data[i+1]
		i += 2
		// Markers with no length field: SOI, EOI, RST0-7, TEM
		if marker == 0xD8 || marker == 0xD9 || (marker >= 0xD0 && marker <= 0xD7) || marker == 0x01 {
			continue
		}
		if i+2 > len(data) {
			return SOFHeader{}
		}
		segLen := int(data[i])<<8 | int(data[i+1])
		if segLen < 2 || i+segLen > len(data) {
			return SOFHeader{}
		}
		// SOF markers (exclude DHT=0xC4 and JPG=0xC8)
		isSOF := (marker >= 0xC0 && marker <= 0xC3) ||
			(marker >= 0xC5 && marker <= 0xC7) ||
			(marker >= 0xC9 && marker <= 0xCB) ||
			(marker >= 0xCD && marker <= 0xCF)
		if isSOF {
			// SOF data layout: length(2) precision(1) height(2) width(2) …
			if i+7 <= len(data) {
				return SOFHeader{
					Precision: data[i+2],
					Height:    uint16(data[i+3])<<8 | uint16(data[i+4]),
					Width:     uint16(data[i+5])<<8 | uint16(data[i+6]),
				}
			}
			if i+3 <= len(data) {
				return SOFHeader{Precision: data[i+2]}
			}
			return SOFHeader{}
		}
		i += segLen
	}
	return SOFHeader{}
}

// SOFPrecision returns the data_precision (bits per sample) declared in the
// first SOF segment of a JPEG payload. Returns 0 if the precision cannot be
// determined (e.g. the payload is too short or malformed).
func SOFPrecision(data []byte) uint8 {
	return ReadSOFHeader(data).Precision
}

// nativeDecode8Backend is an optional interface that native backends may
// implement to handle 8-bit JPEG variants that the pure-Go decoder rejects
// (progressive scan, restart markers, extended sequential, etc.).
type nativeDecode8Backend interface {
	Decode8Context(ctx context.Context, encoded []byte, output []byte) error
}

// Backend defines pluggable implementations for JPEG 12/16-bit profiles.
// Baseline 8-bit continues to use the built-in pure-Go implementation.
type Backend interface {
	Name() string
	SupportedTransferSyntaxUIDs() []string
	Decode12(encoded []byte, output []byte) error
	Encode12(raw []byte, width uint16, height uint16, samples uint16, mode int) ([]byte, error)
	Decode16(encoded []byte, output []byte) error
	Encode16(raw []byte, width uint16, height uint16, samples uint16, mode int) ([]byte, error)
}

type contextBackend interface {
	Decode12Context(ctx context.Context, encoded []byte, output []byte) error
	Encode12Context(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, mode int) ([]byte, error)
	Decode16Context(ctx context.Context, encoded []byte, output []byte) error
	Encode16Context(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, mode int) ([]byte, error)
}

type readinessBackend interface {
	Ready() error
}

type passthroughBackend struct{}

func (passthroughBackend) Name() string {
	return "passthrough"
}

func (passthroughBackend) SupportedTransferSyntaxUIDs() []string {
	return SupportedTransferSyntaxUIDs()
}

func (passthroughBackend) Decode12(_ []byte, _ []byte) error {
	return errBackendUnavailable
}

func (passthroughBackend) Encode12(raw []byte, _ uint16, _ uint16, _ uint16, _ int) ([]byte, error) {
	encoded := make([]byte, len(raw))
	copy(encoded, raw)
	return encoded, nil
}

func (passthroughBackend) Decode16(_ []byte, _ []byte) error {
	return errBackendUnavailable
}

func (passthroughBackend) Encode16(raw []byte, _ uint16, _ uint16, _ uint16, _ int) ([]byte, error) {
	encoded := make([]byte, len(raw))
	copy(encoded, raw)
	return encoded, nil
}

var (
	backendMu        sync.RWMutex
	currentBackend   Backend = passthroughBackend{}
	currentName              = "passthrough"
	backendFactories         = map[string]func() Backend{
		"passthrough": func() Backend { return passthroughBackend{} },
	}
)

func init() {
	registerNativeBackends()
	selectDefaultBackend()
}

func selectDefaultBackend() {
	backendMu.Lock()
	defer backendMu.Unlock()

	preferred := preferredBackendNameLocked()
	if preferred == "passthrough" {
		slog.Warn("jpeg: no native backend available; 12/16-bit encode will pass raw bytes unchanged (build with -tags libjpeg)")
		return
	}
	factory := backendFactories[preferred]
	if factory == nil {
		return
	}
	backend := factory()
	if backend == nil {
		return
	}
	currentBackend = backend
	currentName = preferred
}

func preferredBackendNameLocked() string {
	nativeNames := make([]string, 0, len(backendFactories))
	for name := range backendFactories {
		if name == "passthrough" {
			continue
		}
		nativeNames = append(nativeNames, name)
	}
	if len(nativeNames) != 1 {
		return "passthrough"
	}
	return nativeNames[0]
}

// SetBackend overrides the active JPEG backend for 12/16-bit paths.
// Passing nil resets to default passthrough behavior.
func SetBackend(backend Backend) {
	backendMu.Lock()
	defer backendMu.Unlock()
	if backend == nil {
		currentBackend = passthroughBackend{}
		currentName = "passthrough"
		return
	}
	currentBackend = backend
	currentName = backend.Name()
}

// BackendName returns the current JPEG backend name for 12/16-bit paths.
func BackendName() string {
	backendMu.RLock()
	defer backendMu.RUnlock()
	return currentName
}

// RegisterBackend registers a named backend factory for runtime selection.
func RegisterBackend(name string, factory func() Backend) error {
	if name == "" {
		return errors.New("backend name is required")
	}
	if factory == nil {
		return errors.New("backend factory is required")
	}

	backendMu.Lock()
	defer backendMu.Unlock()
	if _, exists := backendFactories[name]; exists {
		return errors.New("backend already registered")
	}
	backendFactories[name] = factory
	return nil
}

// UseBackend switches to a previously registered backend by name.
func UseBackend(name string) error {
	backendMu.Lock()
	defer backendMu.Unlock()
	factory, exists := backendFactories[name]
	if !exists {
		return errors.New("backend not registered")
	}
	backend := factory()
	if backend == nil {
		return errors.New("backend factory returned nil")
	}
	currentBackend = backend
	currentName = name
	return nil
}

// AvailableBackends returns the list of registered backend names.
func AvailableBackends() []string {
	backendMu.RLock()
	defer backendMu.RUnlock()
	names := make([]string, 0, len(backendFactories))
	for name := range backendFactories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func ValidateBackend(name string) error {
	backendMu.RLock()
	if name == currentName {
		backend := currentBackend
		backendMu.RUnlock()
		if ready, ok := backend.(readinessBackend); ok {
			return ready.Ready()
		}
		return nil
	}
	factory, exists := backendFactories[name]
	backendMu.RUnlock()
	if !exists {
		return errors.New("backend not registered")
	}
	backend := factory()
	if backend == nil {
		return errors.New("backend factory returned nil")
	}
	if ready, ok := backend.(readinessBackend); ok {
		return ready.Ready()
	}
	return nil
}

func activeBackend() Backend {
	backendMu.RLock()
	defer backendMu.RUnlock()
	return currentBackend
}

func SupportedTransferSyntaxUIDs() []string {
	out := make([]string, len(supportedTransferSyntaxUIDs))
	copy(out, supportedTransferSyntaxUIDs)
	return out
}

func decode12WithContext(ctx context.Context, encoded []byte, output []byte) error {
	backend := activeBackend()
	if withContext, ok := backend.(contextBackend); ok {
		return withContext.Decode12Context(ctx, encoded, output)
	}
	return backend.Decode12(encoded, output)
}

func encode12WithContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, mode int) ([]byte, error) {
	backend := activeBackend()
	if withContext, ok := backend.(contextBackend); ok {
		return withContext.Encode12Context(ctx, raw, width, height, samples, mode)
	}
	return backend.Encode12(raw, width, height, samples, mode)
}

func decode16WithContext(ctx context.Context, encoded []byte, output []byte) error {
	backend := activeBackend()
	if withContext, ok := backend.(contextBackend); ok {
		return withContext.Decode16Context(ctx, encoded, output)
	}
	return backend.Decode16(encoded, output)
}

func encode16WithContext(ctx context.Context, raw []byte, width uint16, height uint16, samples uint16, mode int) ([]byte, error) {
	backend := activeBackend()
	if withContext, ok := backend.(contextBackend); ok {
		return withContext.Encode16Context(ctx, raw, width, height, samples, mode)
	}
	return backend.Encode16(raw, width, height, samples, mode)
}

func DIJG8decodeContext(ctx context.Context, jpegData []byte, jpegSize uint32, outputData []byte, outputSize uint32) error {
	if jpegSize > uint32(len(jpegData)) || outputSize > uint32(len(outputData)) {
		return errPayloadTruncated
	}
	if len(jpegData) == 0 || len(outputData) == 0 {
		return errInputTooSmall
	}
	if jpegSize > maxCodecPayloadBytes || outputSize > maxCodecPayloadBytes {
		return errPayloadTooLarge
	}

	jpegData = jpegData[:jpegSize]
	outputData = outputData[:outputSize]

	img, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		// Fall back to the native libjpeg backend if available. It supports
		// progressive, extended-sequential, restart-marker, and other JPEG
		// variants that the pure-Go decoder rejects.
		backend := activeBackend()
		if n8, ok := backend.(nativeDecode8Backend); ok {
			return n8.Decode8Context(ctx, jpegData, outputData)
		}
		return err
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if w*h == len(outputData) {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				g := color.GrayModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.Gray)
				outputData[y*w+x] = g.Y
			}
		}
		return nil
	}

	if w*h*3 > len(outputData) {
		return errOutputTooSmall
	}

	idx := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			outputData[idx] = byte(r >> 8)
			outputData[idx+1] = byte(g >> 8)
			outputData[idx+2] = byte(b >> 8)
			idx += 3
		}
	}
	return nil
}

// DIJG8decode decodes baseline 8-bit JPEG into raw pixel bytes.
func DIJG8decode(jpegData []byte, jpegSize uint32, outputData []byte, outputSize uint32) error {
	return DIJG8decodeContext(context.Background(), jpegData, jpegSize, outputData, outputSize)
}

// EIJG8encode encodes raw pixel bytes to baseline JPEG.
func EIJG8encode(rawData []byte, width uint16, height uint16, samples uint16, outData *[]byte, outSize *int, mode int) error {
	w, h := int(width), int(height)
	if w <= 0 || h <= 0 {
		return errInvalidDimensions
	}

	var img image.Image
	switch samples {
	case 1:
		expected := w * h
		if len(rawData) < expected {
			return errInputTooSmall
		}
		gray := image.NewGray(image.Rect(0, 0, w, h))
		copy(gray.Pix, rawData[:expected])
		img = gray
	case 3:
		expected := w * h * 3
		if len(rawData) < expected {
			return errInputTooSmall
		}
		rgba := image.NewRGBA(image.Rect(0, 0, w, h))
		src := rawData[:expected]
		si := 0
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				di := rgba.PixOffset(x, y)
				rgba.Pix[di] = src[si]
				rgba.Pix[di+1] = src[si+1]
				rgba.Pix[di+2] = src[si+2]
				rgba.Pix[di+3] = 0xFF
				si += 3
			}
		}
		img = rgba
	default:
		return errUnsupportedSamples
	}

	quality := 95
	if mode == 0 {
		quality = 90
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return err
	}
	*outData = buf.Bytes()
	*outSize = len(*outData)
	return nil
}

func DIJG12decode(jpegData []byte, jpegSize uint32, outputData []byte, outputSize uint32) error {
	return DIJG12decodeContext(context.Background(), jpegData, jpegSize, outputData, outputSize)
}

func DIJG12decodeContext(ctx context.Context, jpegData []byte, jpegSize uint32, outputData []byte, outputSize uint32) error {
	if jpegSize > uint32(len(jpegData)) {
		return errPayloadTruncated
	}
	if outputSize > uint32(len(outputData)) {
		return errOutputTooSmall
	}
	if jpegSize > maxCodecPayloadBytes || outputSize > maxCodecPayloadBytes {
		return errPayloadTooLarge
	}
	return decode12WithContext(ctx, jpegData[:jpegSize], outputData[:outputSize])
}

func EIJG12encode(rawData []uint8, width uint16, height uint16, samples uint16, outData *[]byte, outSize *int, mode int) error {
	return EIJG12encodeContext(context.Background(), rawData, width, height, samples, outData, outSize, mode)
}

func EIJG12encodeContext(ctx context.Context, rawData []uint8, width uint16, height uint16, samples uint16, outData *[]byte, outSize *int, mode int) error {
	if outData == nil || outSize == nil {
		return errNilOutputPointers
	}
	if len(rawData) > maxCodecPayloadBytes {
		return errPayloadTooLarge
	}
	encoded, err := encode12WithContext(ctx, rawData, width, height, samples, mode)
	if err != nil {
		return err
	}
	*outData = append((*outData)[:0], encoded...)
	*outSize = len(encoded)
	return nil
}

func DIJG16decode(jpegData []byte, jpegSize uint32, outputData []byte, outputSize uint32) error {
	return DIJG16decodeContext(context.Background(), jpegData, jpegSize, outputData, outputSize)
}

func DIJG16decodeContext(ctx context.Context, jpegData []byte, jpegSize uint32, outputData []byte, outputSize uint32) error {
	if jpegSize > uint32(len(jpegData)) {
		return errPayloadTruncated
	}
	if outputSize > uint32(len(outputData)) {
		return errOutputTooSmall
	}
	if jpegSize > maxCodecPayloadBytes || outputSize > maxCodecPayloadBytes {
		return errPayloadTooLarge
	}
	return decode16WithContext(ctx, jpegData[:jpegSize], outputData[:outputSize])
}

func EIJG16encode(rawData []uint8, width uint16, height uint16, samples uint16, outData *[]byte, outSize *int, mode int) error {
	return EIJG16encodeContext(context.Background(), rawData, width, height, samples, outData, outSize, mode)
}

func EIJG16encodeContext(ctx context.Context, rawData []uint8, width uint16, height uint16, samples uint16, outData *[]byte, outSize *int, mode int) error {
	if outData == nil || outSize == nil {
		return errNilOutputPointers
	}
	if len(rawData) > maxCodecPayloadBytes {
		return errPayloadTooLarge
	}
	encoded, err := encode16WithContext(ctx, rawData, width, height, samples, mode)
	if err != nil {
		return err
	}
	*outData = append((*outData)[:0], encoded...)
	*outSize = len(encoded)
	return nil
}
