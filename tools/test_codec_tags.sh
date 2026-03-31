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
run charls ./media -run TestRepresentativePixelTransferSyntaxRoundTripsWithNativeBackends
run openjpeg ./codecs/jpeg2000
run openjpeg ./media -run TestRepresentativePixelTransferSyntaxRoundTripsWithNativeBackends
run libjxl ./codecs/jpegxl
run libjxl ./media -run TestRepresentativePixelTransferSyntaxRoundTripsWithNativeBackends
run ffmpeg ./codecs/mpeg
run ffmpeg ./media -run TestRepresentativePixelTransferSyntaxRoundTripsWithNativeBackends
run openjph ./codecs/jpip
run openjph ./media -run TestRepresentativePixelTransferSyntaxRoundTripsWithNativeBackends
run st2110 ./codecs/smpte2110
run st2110 ./media -run TestRepresentativePixelTransferSyntaxRoundTripsWithNativeBackends
run libjpeg ./codecs/jpeg
run libjpeg ./media -run TestRepresentativePixelTransferSyntaxRoundTripsWithNativeBackends

echo "All tagged codec backend tests passed."
