# gha-outrunner

Ephemeral GitHub Actions runners — no Kubernetes required.

![How gha-outrunner works](docs/info.png)

outrunner provisions fresh Docker containers or VMs for each GitHub Actions job, then destroys them when the job completes. It uses GitHub's [scaleset API](https://github.com/actions/scaleset) to register as an autoscaling runner group.

**Why?** GitHub's [Actions Runner Controller (ARC)](https://github.com/actions/actions-runner-controller) requires Kubernetes. If you're running on bare metal or a simple VPS, you shouldn't need a cluster just to get ephemeral runners. outrunner gives you the same isolation guarantees with Docker or libvirt.

## Status

Early development. Docker provisioner tested end-to-end. Libvirt provisioner compiles, needs testing.

## Quick Start

Create a config file:

```yaml
# outrunner.yml
images:
  - label: linux
    docker:
      image: ghcr.io/actions/actions-runner:latest

  - label: windows
    libvirt:
      path: /var/lib/libvirt/images/ci-runners/windows-builder.qcow2
      runner_cmd: 'C:\actions-runner\run.cmd'
      cpus: 4
      memory: 8192
```

Build and run:

```bash
go build -o outrunner ./cmd/outrunner

# Build the runner image (for Docker)
docker build -t outrunner-runner runner/

./outrunner \
  --url https://github.com/your/repo \
  --token ghp_xxx \
  --config outrunner.yml \
  --max-runners 2
```

Then in your workflow:

```yaml
jobs:
  build-linux:
    runs-on: linux
    steps:
      - run: echo "Running in an ephemeral container!"

  build-windows:
    runs-on: windows
    steps:
      - run: echo "Running in an ephemeral VM!"
```

## Architecture

```
GitHub ──(scaleset API)──► outrunner ──► Docker containers
                              │      └──► libvirt/QEMU VMs
                              │
                    polls for job demand
                    matches job labels → image config
                    provisions runner (docker or libvirt)
                    tears down after job
```

Each image in the config declares a label and a backend. When a job comes in, outrunner matches the job's `runs-on` labels against image labels and routes to the right provisioner.

## Provisioners

| Provisioner | Backend | How it works |
|-------------|---------|--------------|
| Docker | Containers | Creates a container per job, runs the GitHub Actions runner inside it |
| libvirt | KVM/QEMU VMs | Boots a VM from a qcow2 golden image (copy-on-write overlay), uses QEMU Guest Agent for command execution — no SSH/WinRM needed |

### QEMU Guest Agent

The libvirt provisioner uses the [QEMU Guest Agent](https://wiki.qemu.org/Features/GuestAgent) instead of SSH or WinRM. The agent communicates over virtio-serial (no network required) and supports `guest-exec` — essentially `docker exec` for VMs. The golden image must have `qemu-ga` installed (included by default in [rgl/windows-vagrant](https://github.com/rgl/windows-vagrant) images).

## Provisioner Roadmap

| Provisioner | Status | Use case |
|-------------|--------|----------|
| Docker | Working | Linux jobs, fastest startup |
| libvirt/QEMU | Compiles, needs testing | Windows/Linux VMs, full OS isolation |
| Tart | Future | macOS on Apple Silicon |

## License

MIT
