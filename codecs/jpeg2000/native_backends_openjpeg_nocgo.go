//go:build openjpeg && !cgo

package jpeg2000

const nativeBackendEnabled = false

func registerNativeBackends() {
}
