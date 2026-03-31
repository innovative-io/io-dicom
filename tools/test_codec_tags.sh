#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

run() {
  local tag="$1"
  local pkg="$2"
  echo "==> go test -tags ${tag} ${pkg}"
  go test -tags "$tag" "$pkg"
}

run charls ./codecs/jpegls
run openjpeg ./codecs/jpeg2000
run libjxl ./codecs/jpegxl
run ffmpeg ./codecs/mpeg
run openjph ./codecs/jpip
run st2110 ./codecs/smpte2110
run libjpeg ./codecs/jpeg

echo "All tagged codec backend tests passed."
