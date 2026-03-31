//go:build libjxl && !cgo

package jpegxl

const nativeBackendEnabled = false

func registerNativeBackends() {
}
