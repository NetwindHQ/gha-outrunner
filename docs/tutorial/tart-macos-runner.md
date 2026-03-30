# Tutorial: Tart macOS Runner on macOS

In this tutorial we will set up outrunner on an Apple Silicon Mac to run GitHub Actions jobs in ephemeral macOS VMs via Tart. By the end, you'll trigger a workflow that runs inside a fresh macOS VM, created on demand and destroyed after.

## Prerequisites

- Apple Silicon Mac (M1 or later)
- [Homebrew](https://brew.sh) installed
- A GitHub repository you own

## 1. Install Tart

```bash
brew install cirruslabs/cli/tart
```

## 2. Pull a macOS Base Image

Cirrus Labs provides macOS images with the Tart guest agent pre-installed:

```bash
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest macos-runner
```

This downloads a macOS Sequoia base image (~20 GB). The first pull takes a while.

You can also use Xcode images if your workflows need Xcode:

```bash
tart clone ghcr.io/cirruslabs/macos-sequoia-xcode:latest macos-xcode-runner
```

## 3. Install the GitHub Actions Runner in the Image

The Cirrus Labs base images don't include the Actions runner, so we need to add it.

Start the VM:

```bash
tart run macos-runner
```

A macOS desktop will appear. Open Terminal inside the VM and run:

```bash
mkdir -p ~/actions-runner && cd ~/actions-runner
curl -sL https://github.com/actions/runner/releases/download/v2.322.0/actions-runner-osx-arm64-2.322.0.tar.gz | tar xz
```

Verify it extracted:

```bash
ls ~/actions-runner/run.sh
```

Shut down the VM from the Apple menu → Shut Down (or run `sudo shutdown -h now` in Terminal).

The changes are saved to the `macos-runner` image. This is now your base image with the runner installed.

## 4. Install outrunner

```bash
brew tap NetwindHQ/tap
brew install outrunner
```

## 5. Create a GitHub PAT

Go to [github.com/settings/tokens?type=beta](https://github.com/settings/tokens?type=beta) and create a fine-grained token:

- **Token name:** outrunner
- **Resource owner:** Your user or organization
- **Repository access:** Select the repository you want to use
- **Permissions:** Administration → Read and write

Copy the token.

## 6. Write a Configuration File

Create `outrunner.yml`:

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

Note: The default user in Cirrus Labs images is `admin`, so the runner path is `/Users/admin/actions-runner/run.sh`.

## 7. Start outrunner

```bash
./outrunner \
  --url https://github.com/YOUR_USER/YOUR_REPO \
  --token ghp_YOUR_TOKEN \
  --config outrunner.yml
```

You should see:

```
level=INFO msg="Loaded config" runners=1
level=INFO msg="Scale set created" id=6
level=INFO msg="Tart provisioner initialized"
level=INFO msg="Listening for jobs" scaleSet=macos maxRunners=2
```

## 8. Create a Test Workflow

In your repository, create `.github/workflows/test-outrunner.yml`:

```yaml
name: Test Outrunner

on:
  workflow_dispatch:

jobs:
  hello:
    runs-on: [self-hosted, macos]
    steps:
      - run: echo "Hello from a macOS VM!"
      - run: sw_vers
      - run: sysctl -n machdep.cpu.brand_string
```

Push this file and trigger it from GitHub → Actions → "Test Outrunner" → "Run workflow".

## 9. Watch It Work

In the outrunner terminal:

```
level=DEBUG msg="Cloning VM" tart.image=macos-runner tart.name=macos-a1b2c3d4
level=DEBUG msg="Waiting for guest agent" tart.name=macos-a1b2c3d4
level=INFO  msg="Starting runner in VM" tart.name=macos-a1b2c3d4
level=INFO  msg="Job completed" scaler.runnerName=macos-a1b2c3d4 scaler.result=succeeded
level=DEBUG msg="Stopping VM" tart.name=macos-a1b2c3d4
```

The `sw_vers` step will show macOS Sequoia, confirming it ran inside a real macOS VM.

## 10. Clean Up

Press Ctrl+C to stop outrunner. Remove the base image if you're done:

```bash
tart delete macos-runner
```

## Performance Notes

macOS VMs are heavier than Docker containers. Each clone takes a few seconds and uses several GB of disk space. For this reason:

- Set `--max-runners` conservatively (1-2 per Mac).
- Allocate enough memory. macOS needs at least 4 GB, 8 GB recommended.
- Ensure sufficient disk space for concurrent clones.

## Next Steps

- [How to build a custom Tart macOS image](../howto/custom-tart-macos-image.md)
- [How to deploy as a launchd service](../howto/launchd-service.md)
- [How to run multiple backends together](../howto/mixed-backends.md)
