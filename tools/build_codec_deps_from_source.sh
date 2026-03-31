#!/usr/bin/env bash
set -euo pipefail

# Builds codec dependencies from source for the current target system.
# Output is installed under PREFIX (default: $HOME/.local/codec-deps).

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="${WORK_DIR:-$ROOT_DIR/.build/codec-deps}"
PREFIX="${PREFIX:-$HOME/.local/codec-deps}"
LIB_DIR="$PREFIX/lib"
JOBS="${JOBS:-$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)}"

CHARLS_VERSION="${CHARLS_VERSION:-2.4.3}"
OPENJPEG_VERSION="${OPENJPEG_VERSION:-2.5.3}"
JPEGXL_VERSION="${JPEGXL_VERSION:-0.10.3}"
FFMPEG_VERSION="${FFMPEG_VERSION:-7.1}"
LIBJPEG_TURBO_VERSION="${LIBJPEG_TURBO_VERSION:-3.0.3}"

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

build_charls() {
  local src="$WORK_DIR/charls-src"
  local build="$WORK_DIR/charls-build"
  local tarball="$WORK_DIR/charls-${CHARLS_VERSION}.tar.gz"

  echo "[codec-deps] building CharLS ${CHARLS_VERSION}"
  fetch_tarball "https://github.com/team-charls/charls/archive/refs/tags/${CHARLS_VERSION}.tar.gz" "$tarball"
  extract_tarball "$tarball" "$src"

  cmake -S "$src" -B "$build" \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX="$PREFIX" \
    -DCMAKE_INSTALL_LIBDIR=lib \
    -DBUILD_SHARED_LIBS=ON \
    -DBUILD_TESTING=OFF
  cmake --build "$build" --parallel "$JOBS"
  cmake --install "$build"
}

build_openjpeg() {
  local src="$WORK_DIR/openjpeg-src"
  local build="$WORK_DIR/openjpeg-build"
  local tarball="$WORK_DIR/openjpeg-${OPENJPEG_VERSION}.tar.gz"

  echo "[codec-deps] building OpenJPEG ${OPENJPEG_VERSION}"
  fetch_tarball "https://github.com/uclouvain/openjpeg/archive/refs/tags/v${OPENJPEG_VERSION}.tar.gz" "$tarball"
  extract_tarball "$tarball" "$src"

  cmake -S "$src" -B "$build" \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX="$PREFIX" \
    -DCMAKE_INSTALL_LIBDIR=lib \
    -DBUILD_SHARED_LIBS=ON \
    -DBUILD_TESTING=OFF \
    -DBUILD_CODEC=ON
  cmake --build "$build" --parallel "$JOBS"
  cmake --install "$build"
}

build_jpegxl() {
  local src="$WORK_DIR/jpegxl-src"
  local build="$WORK_DIR/jpegxl-build"
  local tarball="$WORK_DIR/jpegxl-${JPEGXL_VERSION}.tar.gz"

  echo "[codec-deps] building libjxl ${JPEGXL_VERSION}"
  fetch_tarball "https://github.com/libjxl/libjxl/archive/refs/tags/v${JPEGXL_VERSION}.tar.gz" "$tarball"
  extract_tarball "$tarball" "$src"

  cmake -S "$src" -B "$build" \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX="$PREFIX" \
    -DCMAKE_INSTALL_LIBDIR=lib \
    -DBUILD_SHARED_LIBS=ON \
    -DBUILD_TESTING=OFF \
    -DCMAKE_DISABLE_FIND_PACKAGE_GTest=ON \
    -DJPEGXL_ENABLE_TESTS=OFF \
    -DJPEGXL_ENABLE_TOOLS=ON \
    -DJPEGXL_ENABLE_DEVTOOLS=OFF \
    -DJPEGXL_ENABLE_BENCHMARK=OFF \
    -DJPEGXL_ENABLE_EXAMPLES=OFF \
    -DJPEGXL_ENABLE_JNI=OFF \
    -DJPEGXL_FORCE_SYSTEM_BROTLI=ON \
    -DJPEGXL_FORCE_SYSTEM_HWY=ON
  cmake --build "$build" --parallel "$JOBS"
  cmake --install "$build"
}

build_libjpeg_turbo() {
  local src="$WORK_DIR/libjpeg-turbo-src"
  local build="$WORK_DIR/libjpeg-turbo-build"
  local tarball="$WORK_DIR/libjpeg-turbo-${LIBJPEG_TURBO_VERSION}.tar.gz"

  echo "[codec-deps] building libjpeg-turbo ${LIBJPEG_TURBO_VERSION}"
  fetch_tarball "https://github.com/libjpeg-turbo/libjpeg-turbo/archive/refs/tags/${LIBJPEG_TURBO_VERSION}.tar.gz" "$tarball"
  extract_tarball "$tarball" "$src"

  cmake -S "$src" -B "$build" \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX="$PREFIX" \
    -DCMAKE_INSTALL_LIBDIR=lib \
    -DENABLE_SHARED=ON \
    -DENABLE_STATIC=OFF \
    -DBUILD_TESTING=OFF
  cmake --build "$build" --parallel "$JOBS"
  cmake --install "$build"
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
    --enable-shared
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

  build_charls
  build_openjpeg
  build_jpegxl
  build_libjpeg_turbo
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
