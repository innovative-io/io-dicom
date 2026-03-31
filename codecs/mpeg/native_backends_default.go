//go:build !ffmpeg

package mpeg

const nativeBackendEnabled = false

func registerNativeBackends() {
}
