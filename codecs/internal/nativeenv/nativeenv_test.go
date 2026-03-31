package nativeenv

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func resetNativeEnvTestState() {
	once = sync.Once{}
	resolvedBin = ""
	resolvedLib = ""
	resolvedPkg = ""
	commandEnv = nil
}

func TestLookPathRejectsNonExecutableInCodecBin(t *testing.T) {
	prefix := t.TempDir()
	binDir := filepath.Join(prefix, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	toolName := "io-dicom-nonexec-test-tool"
	toolPath := filepath.Join(binDir, toolName)
	if err := os.WriteFile(toolPath, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	t.Setenv("CODEC_DEPS_PREFIX", prefix)
	resetNativeEnvTestState()

	if _, err := LookPath(toolName); err == nil {
		t.Fatal("expected LookPath to reject non-executable codec bin file")
	}
}

func TestLookPathAcceptsExecutableInCodecBin(t *testing.T) {
	prefix := t.TempDir()
	binDir := filepath.Join(prefix, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	toolName := "io-dicom-exec-test-tool"
	toolPath := filepath.Join(binDir, toolName)
	if err := os.WriteFile(toolPath, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	t.Setenv("CODEC_DEPS_PREFIX", prefix)
	resetNativeEnvTestState()

	resolved, err := LookPath(toolName)
	if err != nil {
		t.Fatalf("LookPath failed: %v", err)
	}
	if resolved != toolPath {
		t.Fatalf("unexpected resolved path: got %q want %q", resolved, toolPath)
	}
}
