# Command Binaries

This folder contains executable entrypoints for the project.

## Binaries

- `cmd/io-dicom/` — Main CLI for SCU/SCP operations and DICOM file tasks.
- `cmd/compare/` — Compares metadata and tag values between two DICOM files.
- `cmd/utilities/` — Developer tool that regenerates the DICOM dictionary source files from upstream.

---

## Build

### Build all binaries at once

From the repository root:

```bash
go build ./cmd/...
```

Binaries are placed in the current directory as `io-dicom`, `compare`, and `utilities`.

### Build a single binary

```bash
go build -o io-dicom   ./cmd/io-dicom
go build -o compare    ./cmd/compare
go build -o utilities  ./cmd/utilities
```

### Build with version string embedded

```bash
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")

go build -ldflags "-X main.version=${VERSION}" -o io-dicom  ./cmd/io-dicom
go build -ldflags "-X main.version=${VERSION}" -o compare   ./cmd/compare
```

---

## Install to `$GOPATH/bin` (or `~/go/bin`)

`go install` compiles and places the binary in your Go binary directory
(`$(go env GOPATH)/bin`, typically `~/go/bin`).
Make sure that directory is on your `$PATH`.

### Install all three tools

```bash
go install ./cmd/...
```

### Install a single tool

```bash
go install ./cmd/io-dicom
go install ./cmd/compare
go install ./cmd/utilities
```

After installation you can run them directly:

```bash
io-dicom --help
compare  --help
utilities
```

### Install with version string

```bash
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")

go install -ldflags "-X main.version=${VERSION}" ./cmd/io-dicom
go install -ldflags "-X main.version=${VERSION}" ./cmd/compare
```

---

## Cross-compile for a different OS/architecture

```bash
GOOS=linux  GOARCH=amd64 go build -o io-dicom-linux-amd64   ./cmd/io-dicom
GOOS=darwin GOARCH=arm64 go build -o io-dicom-darwin-arm64  ./cmd/io-dicom
GOOS=windows GOARCH=amd64 go build -o io-dicom-windows.exe  ./cmd/io-dicom
```

---

## Run without building

```bash
go run ./cmd/io-dicom  --help
go run ./cmd/compare   --help
go run ./cmd/utilities
```

### SCP health probe port

When running io-dicom in SCP mode, use the dedicated TCP health probe listener to keep health checks off the DICOM listener port.

```bash
go run ./cmd/io-dicom -scp -datastore ./data -calledae DICOM_SCP -port 1040 -healthport 18040
```

Set `-healthport 0` to disable the dedicated health listener.

---

## Verify `$PATH` contains the Go binary directory

```bash
echo $PATH | tr ':' '\n' | grep -q "$(go env GOPATH)/bin" \
  && echo "go/bin is in PATH" \
  || echo "Add $(go env GOPATH)/bin to your PATH"
```

Add the following line to your shell profile (`~/.zshrc`, `~/.bashrc`, etc.) if needed:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

---

## Building with native CGO codec backends

By default, all binaries are built pure-Go (no CGO, no native codec decoding).
To enable hardware-accelerated encode/decode, build with one or more codec tags.

### System prerequisites

The source-build script (`tools/build_codec_deps_from_source.sh`) compiles all
codec libraries from source. The following tools must be present on your system
before running it.

#### Required on all platforms

| Tool | Purpose |
|------|---------|
| `curl` | Download source archives |
| `tar` | Extract source archives |
| `cmake` ≥ 3.15 | Configure C/C++ builds (FFmpeg drive) |
| `make` | Compile FFmpeg and drive CMake builds |
| `gcc` + `g++` | C and C++ compiler (or clang equivalents) |
| `pkg-config` | Locate installed libraries for cgo |

#### Recommended for FFmpeg (`-tags ffmpeg`, `-tags st2110`)

FFmpeg's assembly-optimised paths use NASM. Without it the build still succeeds
but is slower at runtime:

| Tool | macOS | Ubuntu / Debian | Fedora / RHEL |
|------|-------|-----------------|---------------|
| `nasm` | `brew install nasm` | `apt install nasm` | `dnf install nasm` |

The repo source-build path also configures FFmpeg as a headless codec/runtime
dependency and explicitly disables X11/XCB display integration so downstream
macOS app bundles do not inherit those GUI-side dylib dependencies.

#### macOS — install all prerequisites at once

```bash
brew install cmake make pkg-config curl nasm
# Xcode command-line tools provide gcc/g++/clang/perl/bash:
xcode-select --install
```

#### Ubuntu / Debian — install all prerequisites at once

```bash
sudo apt update
sudo apt install -y \
  build-essential cmake pkg-config curl nasm
```

#### Fedora / RHEL — install all prerequisites at once

```bash
sudo dnf install -y \
  gcc gcc-c++ cmake make pkg-config curl nasm
```

#### Windows

The source-build script is a Bash script that uses POSIX tools (`bash`, `curl`,
`cmake`, `make`, `gcc`). Native Windows Command Prompt / PowerShell cannot run
it directly. Two supported approaches:

**Option A — WSL 2 (recommended)**

Install [WSL 2](https://learn.microsoft.com/en-us/windows/wsl/install) with
Ubuntu, then follow the Ubuntu/Debian instructions above inside the WSL shell.
The built binaries run natively on Linux inside WSL. To produce a native Windows
`.exe`, cross-compile from inside WSL:

```bash
# Inside WSL — cross-compile for Windows amd64
GOOS=windows GOARCH=amd64 \
  go build -tags 'ffmpeg st2110' \
  -o io-dicom.exe ./cmd/io-dicom
```

> Cross-compiling a CGO binary for Windows from Linux requires the
> MinGW-w64 cross-compiler. Install it first:
> ```bash
> sudo apt install gcc-mingw-w64-x86-64
> export CC=x86_64-w64-mingw32-gcc
> ```

**Option B — MSYS2 / MinGW-w64**

Install [MSYS2](https://www.msys2.org/), then open an **MSYS2 UCRT64** shell
and install the required toolchain:

```bash
pacman -Syu
pacman -S --needed \
  mingw-w64-ucrt-x86_64-gcc \
  mingw-w64-ucrt-x86_64-cmake \
  mingw-w64-ucrt-x86_64-pkg-config \
  mingw-w64-ucrt-x86_64-nasm \
  curl perl make
```

Then run the source-build script and `go build` commands from inside the
MSYS2 UCRT64 shell exactly as documented in the Linux steps above.

> **Note:** Pure-Go builds (`go build ./cmd/...` without any `-tags`) work on
> Windows with the standard Go toolchain and no additional tools required.

---

### Available build tags

| Tag | Codec | Required tools |
|-----|-------|----------------|
| `ffmpeg` | MPEG-2/4/HEVC via FFmpeg | `pkg-config`, FFmpeg dev packages (`libavcodec.pc`, `libavformat.pc`, `libavutil.pc`, `libswscale.pc`) |
| `st2110` | SMPTE ST 2110 via FFmpeg | `pkg-config`, FFmpeg dev packages (`libavcodec.pc`, `libavformat.pc`, `libavutil.pc`, `libswscale.pc`) |

### Step 1 — Build codec dependencies from source

The script downloads, compiles, and installs all codec libraries under
`$HOME/.local/codec-deps` (or a custom `PREFIX`):

```bash
./scripts/build_codec_deps_from_source.sh
```

Custom prefix or parallel jobs:

```bash
PREFIX=$PWD/.local/codec-deps JOBS=8 ./scripts/build_codec_deps_from_source.sh
```

The script writes an `env.sh` file to the prefix directory that exports the
`PKG_CONFIG_PATH`, `CGO_CFLAGS`, `CGO_LDFLAGS`, and runtime library path
variables needed for cgo builds.

### Step 2 — Source the environment

```bash
source "$HOME/.local/codec-deps/env.sh"
```

Or for a custom prefix:

```bash
source "$PWD/.local/codec-deps/env.sh"
```

### Step 3 — Build with codec tags

Build with all native codec tags (matches `make build-native`):

```bash
go build -tags 'ffmpeg st2110' \
  -o io-dicom ./cmd/io-dicom
```

Build with a subset of tags (e.g. MPEG/HEVC only):

```bash
go build -tags 'ffmpeg' -o io-dicom ./cmd/io-dicom
```

### Install with native codec tags

```bash
go install -tags 'ffmpeg st2110' \
  ./cmd/io-dicom
```

### Use the Makefile shortcut

The `build-native` target does all three steps automatically:

```bash
make build-native
```

This builds all codec deps from source (into `$PWD/.local/codec-deps`),
sources the environment, then compiles and runs the full contract-check suite.

### Runtime note

The native codec backends locate shared libraries via `CODEC_DEPS_PREFIX`
(defaulting to `$HOME/.local/codec-deps`). On macOS you may also need:

```bash
export DYLD_LIBRARY_PATH="$HOME/.local/codec-deps/lib:$DYLD_LIBRARY_PATH"
```

On Linux:

```bash
export LD_LIBRARY_PATH="$HOME/.local/codec-deps/lib:$LD_LIBRARY_PATH"
```

The `env.sh` sourced in Step 2 sets these automatically for the current shell session.

