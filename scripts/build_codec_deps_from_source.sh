#!/usr/bin/env bash
set -euo pipefail

# Builds codec dependencies from source for the current target system.
# Output is installed under PREFIX (default: $HOME/.local/codec-deps).

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="${WORK_DIR:-$ROOT_DIR/.build/codec-deps}"
PREFIX="${PREFIX:-$HOME/.local/codec-deps}"
LIB_DIR="$PREFIX/lib"
JOBS="${JOBS:-$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)}"

FFMPEG_VERSION="${FFMPEG_VERSION:-7.1}"

echo "[codec-deps] target: $(uname -s)/$(uname -m)"
echo "[codec-deps] prefix: ${PREFIX}"
echo "[codec-deps] libdir: ${LIB_DIR}"
echo "[codec-deps] workdir: ${WORK_DIR}"

mkdir -p "$WORK_DIR" "$PREFIX" "$LIB_DIR"

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

fetch_tarball() {
  local url="$1"
  local out="$2"
  if [[ ! -f "$out" ]]; then
    curl -fsSL "$url" -o "$out"
  fi
}

extract_tarball() {
  local archive="$1"
  local dir="$2"
  rm -rf "$dir"
  mkdir -p "$dir"
  tar -xf "$archive" -C "$dir" --strip-components=1
}

build_ffmpeg() {
  local src="$WORK_DIR/ffmpeg-src"
  local tarball="$WORK_DIR/ffmpeg-${FFMPEG_VERSION}.tar.xz"

  echo "[codec-deps] building FFmpeg ${FFMPEG_VERSION}"
  fetch_tarball "https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz" "$tarball"
  extract_tarball "$tarball" "$src"

  pushd "$src" >/dev/null
  ./configure \
    --prefix="$PREFIX" \
    --libdir="$LIB_DIR" \
    --pkg-config-flags="--static" \
    --extra-cflags="-I$PREFIX/include" \
    --extra-ldflags="-L$LIB_DIR" \
    --extra-libs="-lpthread -lm" \
    --disable-doc \
    --disable-ffplay \
    --disable-static \
    --enable-shared \
    --disable-xlib \
    --disable-libxcb \
    --disable-libxcb-shm \
    --disable-libxcb-xfixes \
    --disable-libxcb-shape \
    --disable-indev=xcbgrab
  make -j"$JOBS"
  make install
  popd >/dev/null
}

normalize_lib_layout() {
  if [[ -d "$PREFIX/lib64" ]]; then
    find "$PREFIX/lib64" -mindepth 1 -maxdepth 1 -exec mv -f {} "$LIB_DIR"/ \;
    rmdir "$PREFIX/lib64" || true
  fi
}

write_env_file() {
  local env_file="$PREFIX/env.sh"
  cat > "$env_file" <<EOF
#!/usr/bin/env bash
# Source this file to use source-built codec dependencies.
export CODEC_DEPS_PREFIX="$PREFIX"
export PATH="$PREFIX/bin:\${PATH}"
export PKG_CONFIG_PATH="$LIB_DIR/pkgconfig:\${PKG_CONFIG_PATH:-}"
export CGO_CFLAGS="-I$PREFIX/include \${CGO_CFLAGS:-}"
export CGO_LDFLAGS="-L$LIB_DIR -Wl,-rpath,$LIB_DIR \${CGO_LDFLAGS:-}"
export LD_LIBRARY_PATH="$LIB_DIR:\${LD_LIBRARY_PATH:-}"
export DYLD_LIBRARY_PATH="$LIB_DIR:\${DYLD_LIBRARY_PATH:-}"
EOF
  chmod +x "$env_file"
}

main() {
  need_cmd curl
  need_cmd tar
  need_cmd cmake
  need_cmd make
  need_cmd pkg-config
  need_cmd gcc
  need_cmd g++

  build_ffmpeg
  normalize_lib_layout
  write_env_file

  cat <<EOF
[codec-deps] build complete
[codec-deps] export these for current shell:
  source "$PREFIX/env.sh"
EOF
}

main "$@"
