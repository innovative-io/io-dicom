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

run ffmpeg ./codecs/mpeg
run ffmpeg ./media -run TestRepresentativePixelTransferSyntaxRoundTripsWithNativeBackends
# JPEG XL (libjxl) and jpip (HTJ2K, openjph) are pure-Go now, with no cgo
# backend; they are covered by the untagged suite and the ffmpeg/st2110 native
# matrix runs.
run st2110 ./codecs/smpte2110
run st2110 ./media -run TestRepresentativePixelTransferSyntaxRoundTripsWithNativeBackends

echo "All tagged codec backend tests passed."
