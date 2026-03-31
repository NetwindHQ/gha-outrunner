# macOS VMs via Tart

This guide assumes outrunner is already installed and running. If not, start with the [macOS setup guide](../setup/macos.md).

## Prerequisites

- Apple Silicon Mac (M1 or later)
- [Tart](https://github.com/cirruslabs/tart) installed: `brew install cirruslabs/cli/tart`

## 1. Pull a macOS base image

Cirrus Labs provides macOS images with the Tart guest agent pre-installed:

```bash
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest macos-runner
```

This is about 20 GB. Xcode images are also available:

```bash
tart clone ghcr.io/cirruslabs/macos-sequoia-xcode:latest macos-xcode-runner
```

## 2. Install the GitHub Actions runner in the image

The base images don't include the Actions runner. Start the VM:

```bash
tart run macos-runner
```

Open Terminal inside the VM and run:

```bash
mkdir -p ~/actions-runner && cd ~/actions-runner
curl -sL https://github.com/actions/runner/releases/download/v2.322.0/actions-runner-osx-arm64-2.322.0.tar.gz | tar xz
```

Shut down the VM from Apple menu -> Shut Down.

The changes are saved to the `macos-runner` image.

See [Build a custom Tart macOS image](../howto/custom-tart-macos-image.md) for more details.

## 3. Configure outrunner

Update your config to use the Tart backend:

```yaml
runners:
  macos:
    labels: [self-hosted, macos]
    tart:
      image: macos-runner
      runner_cmd: /Users/admin/actions-runner/run.sh
      cpus: 4
      memory: 8192
```

The default user in Cirrus Labs images is `admin`.

Restart outrunner:

```bash
brew services restart outrunner
```

## 4. Test it

Create `.github/workflows/test-macos.yml` in your repository:

```yaml
name: Test macOS

on:
  workflow_dispatch:

jobs:
  hello:
    runs-on: [self-hosted, macos]
    steps:
      - run: echo "Hello from a macOS VM!"
      - run: sw_vers
```

Push and trigger from GitHub -> Actions.

## How it works

1. outrunner clones the base VM image to create an independent copy.
2. Sets CPU and memory, then boots the VM headlessly.
3. Waits for the Tart guest agent to respond.
4. Launches the runner via `tart exec`.
5. After the job, stops and deletes the clone.

The base image is never modified.

## Performance notes

- macOS VMs are heavier than Docker containers. Set `max_runners` conservatively (1-2 per Mac).
- Allocate at least 4 GB memory, 8 GB recommended.
- Ensure sufficient disk space for concurrent clones.

## Next steps

- [Build a custom Tart macOS image](../howto/custom-tart-macos-image.md)
- [Run multiple backends together](../howto/mixed-backends.md)
- [Configuration reference](../reference/configuration.md)
