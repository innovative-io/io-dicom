//go:build !libjpeg

package jpeg

const nativeBackendEnabled = false

func registerNativeBackends() {
}
