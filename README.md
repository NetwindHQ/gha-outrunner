# gha-outrunner

[![CI](https://github.com/NetwindHQ/gha-outrunner/actions/workflows/ci.yml/badge.svg)](https://github.com/NetwindHQ/gha-outrunner/actions/workflows/ci.yml)
[![Release](https://github.com/NetwindHQ/gha-outrunner/actions/workflows/release.yml/badge.svg)](https://github.com/NetwindHQ/gha-outrunner/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/NetwindHQ/gha-outrunner)](https://goreportcard.com/report/github.com/NetwindHQ/gha-outrunner)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Ephemeral GitHub Actions runners, no Kubernetes required.

![How gha-outrunner works](docs/info.png)

outrunner provisions fresh Docker containers or VMs for each GitHub Actions job, then destroys them when the job completes. It uses GitHub's [scaleset API](https://github.com/actions/scaleset) to register as an autoscaling runner group.

## Why outrunner?

GitHub's [Actions Runner Controller (ARC)](https://github.com/actions/actions-runner-controller) requires Kubernetes. If you're running on bare metal or a simple VPS, you shouldn't need a cluster just to get ephemeral runners. outrunner gives you the same isolation guarantees with Docker, libvirt, or Tart. No additional orchestrator needed.

Read more about the [motivation and design](docs/explanation/why-outrunner.md), [architecture](docs/explanation/architecture.md), and [security model](docs/explanation/security.md).

## Provisioners

| Provisioner | Host OS | Runner OS | How it works |
|-------------|---------|-----------|--------------|
| Docker | Linux, macOS | Linux | Container per job. Fastest startup. |
| libvirt | Linux | Windows, Linux | KVM VM from qcow2 golden image with CoW overlays. QEMU Guest Agent for command execution. |
| Tart | macOS (Apple Silicon) | macOS, Linux (ARM64) | VM clone per job. Tart guest agent for command execution. |

See the [provisioner reference](docs/reference/provisioners.md) for lifecycle details and [runner image requirements](docs/reference/image-requirements.md) for what each backend expects.

## Get Started

Pick a tutorial for your platform and backend -each one covers installation, configuration, and running your first job end to end:

- **Docker on Linux** -[the fastest path to a working runner](docs/tutorial/docker-linux.md)
- **Docker on macOS** -[works with Colima, Docker Desktop, or Podman](docs/tutorial/docker-macos.md)
- **Windows VMs on Linux** -[KVM/QEMU via libvirt](docs/tutorial/libvirt-windows.md)
- **macOS VMs on Apple Silicon** -[using Tart](docs/tutorial/tart-macos-runner.md)
- **Linux ARM64 VMs on Apple Silicon** -[using Tart](docs/tutorial/tart-linux-runner.md)

All packages and binaries are on the [Releases](https://github.com/NetwindHQ/gha-outrunner/releases) page.

## Going Further

### Deploy to production

- [Deploy as a systemd service](docs/howto/systemd-service.md) (Linux)
- [Deploy as a launchd service](docs/howto/launchd-service.md) (macOS)
- [Set up for an organization](docs/howto/organization-setup.md)
- [Run multiple backends together](docs/howto/mixed-backends.md)

### Customize runner images

- [Build a custom Docker runner image](docs/howto/custom-docker-image.md)
- [Build a custom Windows VM image](docs/howto/custom-windows-image.md)
- [Build a custom Tart macOS image](docs/howto/custom-tart-macos-image.md)
- [Build a custom Tart Linux image](docs/howto/custom-tart-linux-image.md)
- [Update runner images without downtime](docs/howto/update-runner-images.md)

### Reference

- [CLI reference](docs/reference/cli.md)
- [Configuration reference](docs/reference/configuration.md)

## Used by

- [delo.so](https://delo.so) -Desktop, offline-first CAD for makers. Uses outrunner for CI, build and test pipelines.

Using outrunner? [Open a PR](https://github.com/NetwindHQ/gha-outrunner/edit/main/README.md) to add your project.
