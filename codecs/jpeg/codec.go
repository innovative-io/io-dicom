package jpeg

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"sort"
	"sync"
)

var CGOEnabled = nativeBackendEnabled

// Backend defines pluggable implementations for JPEG 12/16-bit profiles.
// Baseline 8-bit continues to use the built-in pure-Go implementation.
type Backend interface {
	Name() string
	Decode12(encoded []byte, output []byte) error
	Encode12(raw []byte, width uint16, height uint16, samples uint16, mode int) ([]byte, error)
	Decode16(encoded []byte, output []byte) error
	Encode16(raw []byte, width uint16, height uint16, samples uint16, mode int) ([]byte, error)
}

type passthroughBackend struct{}

func (passthroughBackend) Name() string {
	return "passthrough"
}

func (passthroughBackend) Decode12(encoded []byte, output []byte) error {
	if len(encoded) > len(output) {
		return errors.New("ERROR, Decode12, JPEG failed")
	}
	copy(output[:len(encoded)], encoded)
	return nil
}

func (passthroughBackend) Encode12(raw []byte, _ uint16, _ uint16, _ uint16, _ int) ([]byte, error) {
	encoded := make([]byte, len(raw))
	copy(encoded, raw)
	return encoded, nil
}

func (passthroughBackend) Decode16(encoded []byte, output []byte) error {
	if len(encoded) > len(output) {
		return errors.New("ERROR, Decode16, JPEG failed")
	}
	copy(output[:len(encoded)], encoded)
	return nil
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

func activeBackend() Backend {
	backendMu.RLock()
	defer backendMu.RUnlock()
	return currentBackend
}

// DIJG8decode decodes baseline 8-bit JPEG into raw pixel bytes.
func DIJG8decode(jpegData []byte, jpegSize uint32, outputData []byte, outputSize uint32) error {
	if len(jpegData) == 0 || len(outputData) == 0 {
		return errors.New("ERROR, Decode8, JPEG failed")
	}

	img, err := jpeg.Decode(bytes.NewReader(jpegData[:jpegSize]))
	if err != nil {
		return err
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if w*h == int(outputSize) {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				g := color.GrayModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.Gray)
				outputData[y*w+x] = g.Y
			}
		}
		return nil
	}

	if w*h*3 > int(outputSize) {
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
	if jpegSize > uint32(len(jpegData)) {
		return errors.New("ERROR, Decode12, JPEG failed")
	}
	if outputSize > uint32(len(outputData)) {
		return errors.New("ERROR, Decode12, JPEG failed")
	}
	return activeBackend().Decode12(jpegData[:jpegSize], outputData[:outputSize])
}

func EIJG12encode(rawData []uint8, width uint16, height uint16, samples uint16, outData *[]byte, outSize *int, mode int) error {
	if outData == nil || outSize == nil {
		return errors.New("ERROR, Encode12, JPEG failed")
	}
	encoded, err := activeBackend().Encode12(rawData, width, height, samples, mode)
	if err != nil {
		return err
	}
	*outData = append((*outData)[:0], encoded...)
	*outSize = len(encoded)
	return nil
}

func DIJG16decode(jpegData []byte, jpegSize uint32, outputData []byte, outputSize uint32) error {
	if jpegSize > uint32(len(jpegData)) {
		return errors.New("ERROR, Decode16, JPEG failed")
	}
	if outputSize > uint32(len(outputData)) {
		return errors.New("ERROR, Decode16, JPEG failed")
	}
	return activeBackend().Decode16(jpegData[:jpegSize], outputData[:outputSize])
}

func EIJG16encode(rawData []uint8, width uint16, height uint16, samples uint16, outData *[]byte, outSize *int, mode int) error {
	if outData == nil || outSize == nil {
		return errors.New("ERROR, Encode16, JPEG failed")
	}
	encoded, err := activeBackend().Encode16(rawData, width, height, samples, mode)
	if err != nil {
		return err
	}
	*outData = append((*outData)[:0], encoded...)
	*outSize = len(encoded)
	return nil
}
