.PHONY: help test test-tags deps-from-source

help:
	@echo "Available targets:"
	@echo "  make test               # Run go test ./..."
	@echo "  make test-tags          # Run all tagged codec backend tests"
	@echo "  make deps-from-source   # Download and compile codec dependencies from source"

test:
	go test ./...

test-tags:
	./tools/test_codec_tags.sh

deps-from-source:
	./tools/build_codec_deps_from_source.sh
