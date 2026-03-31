package nativeenv

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var once sync.Once

// Init prepares environment variables so cgo-linked native dependencies built
// under a single prefix can be discovered at build/runtime.
func Init() {
	once.Do(func() {
		prefix := discoverPrefix()
		if prefix == "" {
			return
		}

		libDir := filepath.Join(prefix, "lib")
		if !dirExists(libDir) {
			return
		}

		binDir := filepath.Join(prefix, "bin")
		pkgConfigDir := filepath.Join(libDir, "pkgconfig")

		prependEnv("PATH", binDir)
		prependEnv("PKG_CONFIG_PATH", pkgConfigDir)
		prependEnv("LD_LIBRARY_PATH", libDir)
		prependEnv("DYLD_LIBRARY_PATH", libDir)
	})
}

func discoverPrefix() string {
	if v := os.Getenv("CODEC_DEPS_PREFIX"); v != "" && dirExists(v) {
		return v
	}

	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".local", "codec-deps")
		if dirExists(p) {
			return p
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		p := filepath.Join(cwd, ".local", "codec-deps")
		if dirExists(p) {
			return p
		}
	}

	return ""
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func prependEnv(key, value string) {
	if value == "" || !dirExists(value) {
		return
	}

	current := os.Getenv(key)
	if current == "" {
		_ = os.Setenv(key, value)
		return
	}

	for _, part := range strings.Split(current, ":") {
		if part == value {
			return
		}
	}

	_ = os.Setenv(key, value+":"+current)
}
