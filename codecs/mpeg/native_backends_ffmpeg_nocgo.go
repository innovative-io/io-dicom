//go:build ffmpeg && !cgo

package mpeg

const nativeBackendEnabled = false

func registerNativeBackends() {
}
