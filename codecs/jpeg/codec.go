package jpeg

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"sort"
	"sync"
)

var CGOEnabled = nativeBackendEnabled

const maxCodecPayloadBytes = 512 << 20

var errBackendUnavailable = errors.New("jpeg 12/16-bit decode requires the libjpeg native backend (build with -tags libjpeg)")

var supportedTransferSyntaxUIDs = []string{
	"1.2.840.10008.1.2.4.50",
	"1.2.840.10008.1.2.4.51",
	"1.2.840.10008.1.2.4.57",
	"1.2.840.10008.1.2.4.70",
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

func (passthroughBackend) Decode12(encoded []byte, output []byte) error {
	_ = encoded
	_ = output
	return errBackendUnavailable
}

func (passthroughBackend) Encode12(raw []byte, _ uint16, _ uint16, _ uint16, _ int) ([]byte, error) {
	encoded := make([]byte, len(raw))
	copy(encoded, raw)
	return encoded, nil
}

func (passthroughBackend) Decode16(encoded []byte, output []byte) error {
	_ = encoded
	_ = output
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

func DIJG8decodeContext(_ context.Context, jpegData []byte, jpegSize uint32, outputData []byte, outputSize uint32) error {
	if jpegSize > uint32(len(jpegData)) || outputSize > uint32(len(outputData)) {
		return errors.New("ERROR, Decode8, JPEG failed")
	}
	if len(jpegData) == 0 || len(outputData) == 0 {
		return errors.New("ERROR, Decode8, JPEG failed")
	}
	if jpegSize > maxCodecPayloadBytes || outputSize > maxCodecPayloadBytes {
		return errors.New("ERROR, Decode8, JPEG failed")
	}

	jpegData = jpegData[:jpegSize]
	outputData = outputData[:outputSize]

	img, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
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
		return errors.New("ERROR, Decode8, JPEG failed")
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
		return errors.New("ERROR, Encode8, JPEG failed")
	}

	var img image.Image
	switch samples {
	case 1:
		expected := w * h
		if len(rawData) < expected {
			return errors.New("ERROR, Encode8, JPEG failed")
		}
		gray := image.NewGray(image.Rect(0, 0, w, h))
		copy(gray.Pix, rawData[:expected])
		img = gray
	case 3:
		expected := w * h * 3
		if len(rawData) < expected {
			return errors.New("ERROR, Encode8, JPEG failed")
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
		return errors.New("ERROR, Encode8, JPEG failed")
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
		return errors.New("ERROR, Decode12, JPEG failed")
	}
	if outputSize > uint32(len(outputData)) {
		return errors.New("ERROR, Decode12, JPEG failed")
	}
	if jpegSize > maxCodecPayloadBytes || outputSize > maxCodecPayloadBytes {
		return errors.New("ERROR, Decode12, JPEG failed")
	}
	return decode12WithContext(ctx, jpegData[:jpegSize], outputData[:outputSize])
}

func EIJG12encode(rawData []uint8, width uint16, height uint16, samples uint16, outData *[]byte, outSize *int, mode int) error {
	return EIJG12encodeContext(context.Background(), rawData, width, height, samples, outData, outSize, mode)
}

func EIJG12encodeContext(ctx context.Context, rawData []uint8, width uint16, height uint16, samples uint16, outData *[]byte, outSize *int, mode int) error {
	if outData == nil || outSize == nil {
		return errors.New("ERROR, Encode12, JPEG failed")
	}
	if len(rawData) > maxCodecPayloadBytes {
		return errors.New("ERROR, Encode12, JPEG failed")
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
		return errors.New("ERROR, Decode16, JPEG failed")
	}
	if outputSize > uint32(len(outputData)) {
		return errors.New("ERROR, Decode16, JPEG failed")
	}
	if jpegSize > maxCodecPayloadBytes || outputSize > maxCodecPayloadBytes {
		return errors.New("ERROR, Decode16, JPEG failed")
	}
	return decode16WithContext(ctx, jpegData[:jpegSize], outputData[:outputSize])
}

func EIJG16encode(rawData []uint8, width uint16, height uint16, samples uint16, outData *[]byte, outSize *int, mode int) error {
	return EIJG16encodeContext(context.Background(), rawData, width, height, samples, outData, outSize, mode)
}

func EIJG16encodeContext(ctx context.Context, rawData []uint8, width uint16, height uint16, samples uint16, outData *[]byte, outSize *int, mode int) error {
	if outData == nil || outSize == nil {
		return errors.New("ERROR, Encode16, JPEG failed")
	}
	if len(rawData) > maxCodecPayloadBytes {
		return errors.New("ERROR, Encode16, JPEG failed")
	}
	encoded, err := encode16WithContext(ctx, rawData, width, height, samples, mode)
	if err != nil {
		return err
	}
	*outData = append((*outData)[:0], encoded...)
	*outSize = len(encoded)
	return nil
}
