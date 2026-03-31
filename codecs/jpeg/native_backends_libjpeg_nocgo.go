//go:build libjpeg && !cgo

package jpeg

const nativeBackendEnabled = false

func registerNativeBackends() {
}
