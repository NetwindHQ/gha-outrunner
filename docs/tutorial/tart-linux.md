# Linux ARM64 VMs via Tart

This guide assumes outrunner is already installed and running. If not, start with the [macOS setup guide](../setup/macos.md).

## Prerequisites

- Apple Silicon Mac (M1 or later)
- [Tart](https://github.com/cirruslabs/tart) installed: `brew install cirruslabs/cli/tart`

## 1. Pull a Linux runner image

Cirrus Labs provides ARM64 Ubuntu images with the GitHub Actions runner and guest agent pre-installed:

```bash
tart clone ghcr.io/cirruslabs/ubuntu-runner-arm64:latest ubuntu-runner
```

This is about 5 GB. Verify:

```bash
tart list
```

You should see `ubuntu-runner` in the list.

## 2. Configure outrunner

Update your config to use the Tart backend:

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

Restart outrunner:

```bash
brew services restart outrunner
```

## 3. Test it

Create `.github/workflows/test-linux-arm64.yml` in your repository:

```yaml
name: Test Linux ARM64

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

Push and trigger from GitHub -> Actions. The `uname -a` step will show `aarch64`.

## How it works

1. outrunner clones the base VM image to create an independent copy.
2. Sets CPU and memory, then boots the VM headlessly.
3. Waits for the Tart guest agent to respond.
4. Launches the runner via `tart exec`.
5. After the job, stops and deletes the clone.

The base image is never modified.

## Next steps

- [Build a custom Tart Linux image](../howto/custom-tart-linux-image.md)
- [Tart macOS runners](tart-macos.md)
- [Run multiple backends together](../howto/mixed-backends.md)
- [Configuration reference](../reference/configuration.md)
