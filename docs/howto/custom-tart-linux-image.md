# How to Build a Custom Tart Linux Image

Build an ARM64 Linux VM image with your tools pre-installed for use with the Tart provisioner on Apple Silicon.

## Prerequisites

- Apple Silicon Mac with Tart installed (`brew install cirruslabs/cli/tart`)

## 1. Start From a Base Image

Cirrus Labs provides ARM64 Ubuntu images with the Actions runner and guest agent pre-installed:

```bash
tart clone ghcr.io/cirruslabs/ubuntu-runner-arm64:latest my-linux-runner
```

If you want a bare Ubuntu image (no runner pre-installed):

```bash
tart clone ghcr.io/cirruslabs/ubuntu:latest my-linux-runner
```

## 2. Boot and Customize

```bash
tart run my-linux-runner
```

Log in (default credentials for Cirrus Labs images: `admin` / `admin`).

### Install Your Tools

```bash
sudo apt-get update
sudo apt-get install -y \
    build-essential \
    cmake \
    python3 python3-pip \
    nodejs npm \
    docker.io
```

### Install the Actions Runner (if not pre-installed)

If you started from a bare Ubuntu image:

```bash
mkdir -p ~/actions-runner && cd ~/actions-runner
curl -sL https://github.com/actions/runner/releases/download/v2.322.0/actions-runner-linux-arm64-2.322.0.tar.gz | tar xz
```

### Verify the Guest Agent

From the host (in a separate terminal):

```bash
tart exec my-linux-runner echo "agent works"
```

## 3. Shut Down

```bash
sudo shutdown -h now
```

## 4. Use in Config

```yaml
runners:
  linux-arm64:
    labels: [self-hosted, linux, arm64]
    tart:
      image: my-linux-runner
      runner_cmd: /home/admin/actions-runner/run.sh
      cpus: 4
      memory: 4096
```

## Why Tart for Linux

Tart runs Linux VMs natively on Apple Silicon using Apple's Virtualization.framework. Compared to Docker via Colima:

- **Native ARM64:** No emulation layer. Linux runs on bare virtual hardware.
- **Full kernel:** Real Linux kernel, not shared with the host. Useful for workflows that need kernel modules, Docker-in-Docker, or specific kernel features.
- **Isolation:** VM-level isolation, stronger than container isolation.

The tradeoff is startup time (VM boot vs. container start) and resource usage (dedicated memory allocation vs. shared).

## Publishing to a Registry

```bash
tart push my-linux-runner ghcr.io/your-org/linux-runner-arm64:latest
```

```yaml
runners:
  linux-arm64:
    labels: [self-hosted, linux, arm64]
    tart:
      image: ghcr.io/your-org/linux-runner-arm64:latest
```

## Tips

- **Runner path:** For Cirrus Labs `ubuntu-runner-arm64`, the runner is at `/home/admin/actions-runner/run.sh`.
- **Image size:** Linux images are 3-8 GB, much smaller than macOS images.
- **Docker-in-Docker:** Install Docker inside the Linux VM if your workflows need to build containers. This works because it's a full VM with its own kernel.
