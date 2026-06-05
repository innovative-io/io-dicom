package jpeg2000

// No native (cgo) JPEG 2000 backend: the decoder and lossless encoder are pure
// Go. registerNativeBackends is a no-op kept so codec.go's init can call it
// uniformly; nativeBackendEnabled stays false.

const nativeBackendEnabled = false

func registerNativeBackends() {}
