#!/usr/bin/env bash
set -euo pipefail

# Builds codec dependencies from source for the current target system.
# Output is installed under PREFIX (default: $HOME/.local/codec-deps).

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="${WORK_DIR:-$ROOT_DIR/.build/codec-deps}"
PREFIX="${PREFIX:-$HOME/.local/codec-deps}"
LIB_DIR="$PREFIX/lib"
JOBS="${JOBS:-$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)}"

JPEGXL_VERSION="${JPEGXL_VERSION:-0.10.3}"
OPENJPH_VERSION="${OPENJPH_VERSION:-0.26.3}"
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

build_jpegxl() {
  local src="$WORK_DIR/jpegxl-src"
  local build="$WORK_DIR/jpegxl-build"
  local tarball="$WORK_DIR/jpegxl-${JPEGXL_VERSION}.tar.gz"

  echo "[codec-deps] building libjxl ${JPEGXL_VERSION}"
  fetch_tarball "https://github.com/libjxl/libjxl/archive/refs/tags/v${JPEGXL_VERSION}.tar.gz" "$tarball"
  extract_tarball "$tarball" "$src"

  # GitHub source archives do not contain libjxl's vendored third_party tree.
  # Bootstrap it before CMake configure so tagged CI builds stay self-contained.
  (cd "$src" && bash ./deps.sh)

  # Vendored sjpeg still declares a CMake minimum that CMake 4 rejects.
  perl -0pi -e 's/cmake_minimum_required\(VERSION 2\.8\.7\)/cmake_minimum_required(VERSION 3.5)/' \
    "$src/third_party/sjpeg/CMakeLists.txt"

  cmake -S "$src" -B "$build" \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_POLICY_VERSION_MINIMUM=3.5 \
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

  # libjxl is built against system (Homebrew) highway and brotli.
  # Copy those transitive dependencies into the prefix so they can be bundled
  # into the app and are present on machines other than the build host.
  copy_jxl_system_deps
}

# Detect libhwy and libbrotli{common,dec,enc} by inspecting the installed libjxl
# via otool, then copy any that resolve outside the prefix into $LIB_DIR.
copy_jxl_system_deps() {
  local libjxl="$LIB_DIR/libjxl.dylib"
  [[ -f "$libjxl" ]] || return 0

  local dep resolved name
  while IFS= read -r dep; do
    [[ -z "$dep" ]] && continue
    [[ "$dep" == /System/* || "$dep" == /usr/lib/* ]] && continue
    [[ "$dep" == @* ]] && continue
    name="$(basename "$dep")"
    # Only the dylibs we care about: highway and brotli
    case "$name" in
      libhwy.*.dylib|libbrotli*.dylib) ;;
      *) continue ;;
    esac
    if [[ -f "$dep" && ! -f "$LIB_DIR/$name" ]]; then
      echo "[codec-deps] bundling jxl dep: $name"
      cp -f "$dep" "$LIB_DIR/$name"
      # Fix install name so it resolves via @rpath in the app bundle
      install_name_tool -id "@rpath/$name" "$LIB_DIR/$name" 2>/dev/null || true
    fi
  done < <(otool -L "$libjxl" | tail -n +2 | awk '{print $1}')
}

build_openjph() {
  local src="$WORK_DIR/openjph-src"
  local build="$WORK_DIR/openjph-build"
  local tarball="$WORK_DIR/openjph-${OPENJPH_VERSION}.tar.gz"

  echo "[codec-deps] building OpenJPH ${OPENJPH_VERSION}"
  fetch_tarball "https://github.com/aous72/OpenJPH/archive/refs/tags/${OPENJPH_VERSION}.tar.gz" "$tarball"
  extract_tarball "$tarball" "$src"

  cmake -S "$src" -B "$build" \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_POLICY_VERSION_MINIMUM=3.5 \
    -DCMAKE_INSTALL_PREFIX="$PREFIX" \
    -DCMAKE_INSTALL_LIBDIR=lib \
    -DBUILD_SHARED_LIBS=ON \
    -DOJPH_BUILD_TESTS=OFF \
    -DOJPH_ENABLE_TIFF_SUPPORT=OFF
  cmake --build "$build" --parallel "$JOBS"
  cmake --install "$build"

  # OpenJPH does not ship a pkg-config file; write one so CGO can find it.
  mkdir -p "$PREFIX/lib/pkgconfig"
  cat > "$PREFIX/lib/pkgconfig/openjph.pc" <<EOF
prefix=$PREFIX
exec_prefix=\${prefix}
libdir=\${exec_prefix}/lib
includedir=\${prefix}/include

Name: openjph
Description: OpenJPH open source JPEG 2000 High Throughput (HTJ2K) implementation
Version: ${OPENJPH_VERSION}
Libs: -L\${libdir} -lopenjph
Cflags: -I\${includedir}
EOF
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

  build_jpegxl
  build_openjph
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
