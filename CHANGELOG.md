# Changelog

## [1.1.2] - 2026-04-08

### Fixes

- Capture tart stderr on VM/runner failure — previously only logged `exit status 1` with no detail. Stderr is now streamed to a temp file and the last 1KB shown inline in the error log. Log files are cleaned up on success.

## [1.1.1] - 2026-04-06

Minor bugfix release.

### Fixes

- Disable `PrivateTmp` in systemd service — libvirt provisioner creates qcow2 overlays in `/tmp` that QEMU needs to access outside the namespace
- Clarify Windows virtiofs mount requirements in docs: WinFsp install, `VirtioFsSvc` auto-start, default drive letter (Z:)

## [1.1.0] - 2026-04-04

### Features

- Docker provisioner: bind mounts (`mounts` field with `source`, `target`, `read_only` per entry)
- Tart provisioner: shared directories via `tart run --dir` (`mounts` field with `name`, `source`, `read_only` per entry)
- Libvirt provisioner: virtiofs host directory share (`mount` field with `source`; tag derived from basename; `memoryBacking`/`memfd` added to domain XML automatically)

Primary use case: persistent build cache shared across ephemeral runners without network round-trips.

## [1.0.0] - 2026-03-31

First stable release.

### Features

- Ephemeral GitHub Actions runners via Docker, libvirt/KVM, and Tart backends
- Automatic scale set registration via GitHub's scaleset API
- One goroutine per runner with full lifecycle management (provisioning → idle → running → stopping)
- Orphan cleanup on startup to remove leftover containers/VMs from previous runs
- YAML configuration with per-runner labels, resource limits, and backend selection
- Token resolution with multiple sources (CLI flag, env var, systemd-creds, token file)
- Human-readable log format
- `--version` flag with build-time version injection

### Packaging

- Signed deb and rpm packages via GoReleaser
- Homebrew formula (`brew install NetwindHQ/tap/outrunner`)
- apt repository at `pkg.netwind.pl` with automatic setup on deb install
- rpm repository at `pkg.netwind.pl` for `dnf config-manager addrepo`
- systemd service unit with security hardening
- Default config template with quick-setup instructions

### Documentation

- Tutorials for all five backend/platform combinations
- How-to guides for production deployment, custom images, and organization setup
- CLI, configuration, and provisioner reference
- Architecture, security model, and release pipeline docs
