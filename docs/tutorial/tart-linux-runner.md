# Tutorial: Tart Linux Runner on macOS

In this tutorial we will set up outrunner on an Apple Silicon Mac to run GitHub Actions jobs in ephemeral ARM64 Linux VMs via Tart. By the end, you'll trigger a workflow that runs inside a Linux VM, created on demand and destroyed after.

This is a great option for ARM64 Linux CI on Apple Silicon, no Docker or Colima needed.

## Prerequisites

- Apple Silicon Mac (M1 or later)
- [Homebrew](https://brew.sh) installed
- A GitHub repository you own

## 1. Install Tart

```bash
brew install cirruslabs/cli/tart
```

Verify it's installed:

```bash
tart --version
```

## 2. Pull a Linux Runner Image

Cirrus Labs provides ready-to-use ARM64 Ubuntu images with the GitHub Actions runner and guest agent pre-installed:

```bash
tart clone ghcr.io/cirruslabs/ubuntu-runner-arm64:latest ubuntu-runner
```

This downloads the OCI image and creates a local VM. It's about 5 GB, so the first pull takes a few minutes. Subsequent clones use the cached image.

Verify the image:

```bash
tart list
```

You should see `ubuntu-runner` in the list.

## 3. Build outrunner

```bash
git clone https://github.com/psubocz/gha-outrunner.git
cd gha-outrunner
go build -o outrunner ./cmd/outrunner
```

## 4. Create a GitHub PAT

Go to [github.com/settings/tokens?type=beta](https://github.com/settings/tokens?type=beta) and create a fine-grained token:

- **Token name:** outrunner
- **Resource owner:** Your user or organization
- **Repository access:** Select the repository you want to use
- **Permissions:** Administration → Read and write

Copy the token.

## 5. Write a Configuration File

Create `outrunner.yml`:

```yaml
runners:
  linux-arm64:
    labels: [self-hosted, linux, arm64]
    tart:
      image: ubuntu-runner
      runner_cmd: /home/admin/actions-runner/run.sh
      cpus: 4
      memory: 4096
```

Note: The `runner_cmd` path depends on the image. For Cirrus Labs images, the runner is at `/home/admin/actions-runner/run.sh`.

## 6. Start outrunner

```bash
./outrunner \
  --url https://github.com/YOUR_USER/YOUR_REPO \
  --token ghp_YOUR_TOKEN \
  --config outrunner.yml
```

You should see:

```
level=INFO msg="Loaded config" runners=1
level=INFO msg="Scale set created" id=5
level=INFO msg="Tart provisioner initialized"
level=INFO msg="Listening for jobs" scaleSet=linux-arm64 maxRunners=2
```

## 7. Create a Test Workflow

In your repository, create `.github/workflows/test-outrunner.yml`:

```yaml
name: Test Outrunner

on:
  workflow_dispatch:

jobs:
  hello:
    runs-on: [self-hosted, linux, arm64]
    steps:
      - run: echo "Hello from a Tart Linux VM!"
      - run: uname -a
      - run: cat /etc/os-release
```

Push this file and trigger it from GitHub → Actions → "Test Outrunner" → "Run workflow".

## 8. Watch It Work

In the outrunner terminal you'll see the VM lifecycle:

```
level=DEBUG msg="Cloning VM" tart.image=ubuntu-runner tart.name=linux-arm64-a1b2c3d4
level=DEBUG msg="Waiting for guest agent" tart.name=linux-arm64-a1b2c3d4
level=DEBUG msg="Starting VM" tart.name=linux-arm64-a1b2c3d4
level=INFO  msg="Starting runner in VM" tart.name=linux-arm64-a1b2c3d4
level=INFO  msg="Runner started in VM" tart.name=linux-arm64-a1b2c3d4
level=INFO  msg="Job started" scaler.runnerName=linux-arm64-a1b2c3d4
level=INFO  msg="Job completed" scaler.runnerName=linux-arm64-a1b2c3d4 scaler.result=succeeded
level=DEBUG msg="Stopping VM" tart.name=linux-arm64-a1b2c3d4
```

The workflow on GitHub should show a green checkmark. The `uname -a` step will show `aarch64`, confirming it ran on ARM64 Linux.

## 9. Clean Up

Press Ctrl+C to stop outrunner. Then remove the base image if you're done:

```bash
tart delete ubuntu-runner
```

## How It Works

1. outrunner clones the `ubuntu-runner` VM to create `linux-arm64-a1b2c3d4` (an independent copy).
2. It sets the CPU and memory allocation, then boots the VM headlessly.
3. It waits for the Tart guest agent to respond (polling `tart exec <name> echo ok`).
4. It launches the runner inside the VM via `tart exec`.
5. After the job completes, it stops and deletes the clone.

The base `ubuntu-runner` image is never modified. Each job gets a fresh clone.

## Next Steps

- [Tutorial: Tart macOS runner on macOS](tart-macos-runner.md)
- [How to build a custom Tart Linux image](../howto/custom-tart-linux-image.md)
- [How to run multiple backends together](../howto/mixed-backends.md)
