package nativeenv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	once        sync.Once
	resolvedBin string
	resolvedLib string
	resolvedPkg string
	commandEnv  []string
)

// Init prepares environment variables so cgo-linked native dependencies built
// under a single prefix can be discovered at build/runtime.
func Init() {
	once.Do(func() {
		prefix := discoverPrefix()
		if prefix == "" {
			commandEnv = os.Environ()
			return
		}

		libDir := filepath.Join(prefix, "lib")
		binDir := filepath.Join(prefix, "bin")
		pkgConfigDir := filepath.Join(libDir, "pkgconfig")
		resolvedBin = binDir
		if dirExists(libDir) {
			resolvedLib = libDir
		}
		if dirExists(pkgConfigDir) {
			resolvedPkg = pkgConfigDir
		}
		commandEnv = mergedEnv(os.Environ())
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

func mergedEnv(base []string) []string {
	merged := append([]string(nil), base...)
	merged = setOrPrepend(merged, "PATH", resolvedBin, dirExists(resolvedBin))
	merged = setOrPrepend(merged, "PKG_CONFIG_PATH", resolvedPkg, dirExists(resolvedPkg))
	merged = setOrPrepend(merged, "LD_LIBRARY_PATH", resolvedLib, dirExists(resolvedLib))
	merged = setOrPrepend(merged, "DYLD_LIBRARY_PATH", resolvedLib, dirExists(resolvedLib))
	return merged
}

func setOrPrepend(env []string, key string, value string, enabled bool) []string {
	if !enabled || value == "" {
		return env
	}
	prefix := key + "="
	for idx, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			continue
		}
		current := strings.TrimPrefix(entry, prefix)
		for _, part := range strings.Split(current, ":") {
			if part == value {
				return env
			}
		}
		env[idx] = prefix + value + ":" + current
		return env
	}
	return append(env, prefix+value)
}

func Environ() []string {
	Init()
	return append([]string(nil), commandEnv...)
}

func LookPath(file string) (string, error) {
	Init()
	if resolvedBin != "" {
		candidate := filepath.Join(resolvedBin, file)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
			return candidate, nil
		}
	}
	return exec.LookPath(file)
}

func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = Environ()
	return cmd
}
