# How to Build a Custom Tart macOS Image

Build a macOS VM image with your tools pre-installed for use with the Tart provisioner.

## Prerequisites

- Apple Silicon Mac with Tart installed (`brew install cirruslabs/cli/tart`)

## 1. Start From a Base Image

Clone a Cirrus Labs base image:

```bash
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest my-runner-base
```

For Xcode workflows, use the Xcode image instead:

```bash
tart clone ghcr.io/cirruslabs/macos-sequoia-xcode:latest my-runner-base
```

## 2. Boot and Customize

```bash
tart run my-runner-base
```

A macOS desktop appears. Open Terminal and install your tools.

### Install the Actions Runner

```bash
mkdir -p ~/actions-runner && cd ~/actions-runner
curl -sL https://github.com/actions/runner/releases/download/v2.322.0/actions-runner-osx-arm64-2.322.0.tar.gz | tar xz
```

### Install Homebrew Packages

```bash
brew install cmake ninja python node
```

### Install Xcode Command Line Tools

```bash
xcode-select --install
```

### Install Other Tools

```bash
# Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y

# Go
brew install go
```

## 3. Verify the Guest Agent

The Tart Guest Agent should already be running (pre-installed in Cirrus Labs images). Verify from the host (in a separate terminal):

```bash
tart exec my-runner-base echo "agent works"
```

If this fails, install the agent inside the VM:

```bash
brew install cirruslabs/cli/tart-guest-agent
brew services start tart-guest-agent
```

## 4. Shut Down

From the VM's Apple menu → Shut Down, or in Terminal:

```bash
sudo shutdown -h now
```

## 5. Use in Config

```yaml
runners:
  macos:
    labels: [self-hosted, macos]
    tart:
      image: my-runner-base
      runner_cmd: /Users/admin/actions-runner/run.sh
      cpus: 4
      memory: 8192
```

## Updating the Image

To add or update tools later:

```bash
tart run my-runner-base
# Make changes in the VM
# Shut down
```

Changes persist to the `my-runner-base` image. Running outrunner jobs won't affect it because they use clones.

## Publishing to a Registry

Share your image via an OCI registry:

```bash
tart push my-runner-base ghcr.io/your-org/macos-runner:latest
```

Then reference it in config by the registry URL:

```yaml
runners:
  macos:
    labels: [self-hosted, macos]
    tart:
      image: ghcr.io/your-org/macos-runner:latest
```

## Tips

- **Cirrus Labs images default user:** `admin` (password: `admin`).
- **Runner path:** Typically `/Users/admin/actions-runner/run.sh`.
- **Image size:** macOS images are 15-25 GB. Ensure enough disk for concurrent clones (`--max-runners` x image size).
- **Xcode versions:** Use specific Xcode images from Cirrus Labs for pinned Xcode versions rather than installing Xcode yourself.
