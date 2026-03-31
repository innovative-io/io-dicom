//go:build !libjxl

package jpegxl

const nativeBackendEnabled = false

func registerNativeBackends() {
}
