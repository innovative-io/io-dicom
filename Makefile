.PHONY: help test test-tags deps-from-source transfer-syntax-matrix contract-check

help:
	@echo "Available targets:"
	@echo "  make test               # Run go test ./..."
	@echo "  make test-tags          # Run all tagged codec backend tests"
	@echo "  make deps-from-source   # Download and compile codec dependencies from source"
	@echo "  make transfer-syntax-matrix # Regenerate transfer syntax support docs"
	@echo "  make contract-check     # Run transfer syntax/doc/test contract checks"

test:
	go test ./...

test-tags:
	./tools/test_codec_tags.sh

deps-from-source:
	./tools/build_codec_deps_from_source.sh

transfer-syntax-matrix:
	go run ./tools/generate_transfer_syntax_support_matrix.go

contract-check:
	make transfer-syntax-matrix
	go test ./dictionary/transfersyntax ./media
	./tools/test_codec_tags.sh
	go test ./...
