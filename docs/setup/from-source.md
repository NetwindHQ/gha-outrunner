# Install from source

## 1. Install outrunner

Requires Go 1.26+.

```bash
go install github.com/NetwindHQ/gha-outrunner/cmd/outrunner@latest
```

This installs the `outrunner` binary to your `$GOPATH/bin`. No systemd service or default config is created.

## 2. Create a GitHub PAT

Go to [github.com/settings/tokens?type=beta](https://github.com/settings/tokens?type=beta) and create a fine-grained token:

- **Token name:** outrunner
- **Resource owner:** Your user or organization
- **Repository access:** Select the repository you want to use
- **Permissions:** Administration -> Read and write

## 3. Write a config file

Create `outrunner.yml`:

```yaml
url: https://github.com/your-org/your-repo

runners:
  linux:
    labels: [self-hosted, linux]
    docker:
      image: ghcr.io/actions/actions-runner:latest
```

See the [configuration reference](../reference/configuration.md) for all options.

## 4. Verify

```bash
outrunner --version
outrunner --token ghp_YOUR_TOKEN --config outrunner.yml
```

You should see outrunner connect and start listening for jobs. Press Ctrl+C to stop.

## 5. Run a test workflow

In your GitHub repository, create `.github/workflows/test-outrunner.yml`:

```yaml
name: Test Outrunner

on:
  workflow_dispatch:

jobs:
  hello:
    runs-on: [self-hosted, linux]
    steps:
      - run: echo "Hello from an ephemeral container!"
      - run: hostname
```

Push this file, then go to GitHub -> Actions -> "Test Outrunner" -> "Run workflow".

You should see outrunner spawn a runner, pick up the job, and clean up.

## Next steps

outrunner is now running with Docker. For other backends:

- [Windows VMs via libvirt/KVM](../tutorial/libvirt-windows.md)
- [macOS VMs via Tart](../tutorial/tart-macos.md)
- [Linux ARM64 VMs via Tart](../tutorial/tart-linux.md)
