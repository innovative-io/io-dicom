.PHONY: help test test-tags deps-from-source build-native install install-native transfer-syntax-matrix contract-check

NATIVE_CODEC_TAGS := libjpeg charls openjpeg libjxl openjph ffmpeg st2110

help:
	@echo "Available targets:"
	@echo "  make test               # Run go test ./..."
	@echo "  make test-tags          # Run all tagged codec backend tests"
	@echo "  make deps-from-source   # Download and compile codec dependencies from source"
	@echo "  make build-native       # Build source deps, compile with native codec tags, run contract checks"
	@echo "  make install            # Install io-dicom CLI (pure Go) to GOPATH/bin"
	@echo "  make install-native     # Install io-dicom CLI with native codecs to GOPATH/bin"
	@echo "  make transfer-syntax-matrix # Regenerate transfer syntax support docs"
	@echo "  make contract-check     # Run transfer syntax/doc/test contract checks"

test:
	go test ./...

test-tags:
	./scripts/test_codec_tags.sh

deps-from-source:
	./scripts/build_codec_deps_from_source.sh

build-native:
	PREFIX="$$PWD/.local/codec-deps" WORK_DIR="$$PWD/.build/codec-deps" JOBS="8" ./scripts/build_codec_deps_from_source.sh
	. "$$PWD/.local/codec-deps/env.sh" && go build -tags '$(NATIVE_CODEC_TAGS)' ./... && make contract-check

install:
	CGO_ENABLED=0 go install ./cmd/io-dicom/

install-native:
	@if [ ! -f "$$PWD/.local/codec-deps/env.sh" ]; then \
		echo "Codec deps not found — run 'make deps-from-source' first"; exit 1; \
	fi
	. "$$PWD/.local/codec-deps/env.sh" && go install -tags '$(NATIVE_CODEC_TAGS)' ./cmd/io-dicom/
	find "$$PWD/.local/codec-deps/lib" -name "*.dylib" -exec codesign --force --sign - {} \;
	codesign --force --sign - $$(go env GOPATH)/bin/io-dicom

transfer-syntax-matrix:
	go run ./cmd/generate-transfer-syntax-support-matrix/

contract-check:
	make transfer-syntax-matrix
	go test ./dictionary/transfersyntax ./media
	./scripts/test_codec_tags.sh
	go test ./...
