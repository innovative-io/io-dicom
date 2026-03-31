# Project Structure

This document defines package responsibilities and the intended dependency direction.

## Top-Level Areas

- `cmd/`
  - CLI entrypoints only.
  - Should depend on service/domain packages, not the reverse.
- `media/`
  - DICOM object model and file/pixel processing logic.
  - Central package used by services and DIMSE handlers.
- `network/`
  - Protocol-level PDUs, association negotiation, and transport framing.
  - No CLI concerns.
- `services/`
  - High-level SCU/SCP workflows built on top of `network/` + `dimse/` + `media/`.
- `dimse/`
  - DIMSE command and response handling.
- `dictionary/`
  - Static DICOM dictionaries and UID/tag mappings.
- `codecs/jpeg/`, `codecs/jpeg2000/`, `transcoder/`
  - Canonical pixel transcoding and codec integration packages.
- `database/`
  - Storage layer and sqlite integration.
- `utils/`, `uuids/`, `clients/`, `implementation/`
  - Shared supporting modules.
- `samples/`
  - Test fixtures and manual validation input files.

## Dependency Guidelines

- Keep dependency flow one-way where possible:
  - `cmd/*` -> `services`, `media`, `utils`
  - `services` -> `network`, `dimse`, `media`, `dictionary`
  - `media` -> `dictionary`, `codecs/*`, `transcoder`
- Avoid introducing dependencies from low-level packages to CLI packages.
- Keep transport/protocol concerns (`network`, `dimse`) separate from persistence (`database`).

## Folder Hygiene

- Add a `README.md` for each binary folder under `cmd/`.
- Keep sample files in `samples/`; do not store generated runtime artifacts there.
- Prefer package-local tests (`*_test.go`) next to implementation files.

## Suggested Workflow For Future Reorganization

1. Move files only when package boundaries are clear.
2. Update imports incrementally and run `go test ./...` after each move batch.
3. Keep public API names stable unless a breaking change is explicitly planned.
