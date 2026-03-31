//go:build !openjpeg

package jpeg2000

const nativeBackendEnabled = false

func registerNativeBackends() {
}
