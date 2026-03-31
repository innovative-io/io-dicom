.PHONY: help test test-tags deps-from-source build-native transfer-syntax-matrix contract-check

NATIVE_CODEC_TAGS := libjpeg charls openjpeg libjxl openjph ffmpeg st2110

help:
	@echo "Available targets:"
	@echo "  make test               # Run go test ./..."
	@echo "  make test-tags          # Run all tagged codec backend tests"
	@echo "  make deps-from-source   # Download and compile codec dependencies from source"
	@echo "  make build-native       # Build source deps, compile with native codec tags, run contract checks"
	@echo "  make transfer-syntax-matrix # Regenerate transfer syntax support docs"
	@echo "  make contract-check     # Run transfer syntax/doc/test contract checks"

test:
	go test ./...

test-tags:
	./tools/test_codec_tags.sh

deps-from-source:
	./tools/build_codec_deps_from_source.sh

build-native:
	PREFIX="$$PWD/.local/codec-deps" WORK_DIR="$$PWD/.build/codec-deps" JOBS="8" ./tools/build_codec_deps_from_source.sh
	. "$$PWD/.local/codec-deps/env.sh" && go build -tags '$(NATIVE_CODEC_TAGS)' ./... && make contract-check

transfer-syntax-matrix:
	go run ./tools/generate_transfer_syntax_support_matrix.go

contract-check:
	make transfer-syntax-matrix
	go test ./dictionary/transfersyntax ./media
	./tools/test_codec_tags.sh
	go test ./...
