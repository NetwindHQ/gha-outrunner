# Install from source

## 1. Install outrunner

Requires Go 1.26+.

```bash
go install github.com/NetwindHQ/gha-outrunner/cmd/outrunner@latest
```

This installs the `outrunner` binary to your `$GOPATH/bin`. No systemd service or default config is created.

## 2. Choose an authentication method

### GitHub App (recommended)

Create a GitHub App for the repository or organization. For **repository-scoped** runners (`url: https://github.com/owner/repo`), grant **Repository permissions → Administration: Read and write**; Metadata is read-only. For **organization-scoped** runners (`url: https://github.com/org`), grant **Organization permissions → Self-hosted runners: Read and write**; Repository Administration is **not required**. Install the App on the target organization/repositories. These permissions follow GitHub's [runner authentication guidance](https://docs.github.com/en/actions/how-tos/manage-runners/use-actions-runner-controller/authenticate-to-the-api); the API scope, not the VM manager, determines the grant.

This polling service does not need webhooks; disable webhook delivery in the App settings. The SDK's internal Actions-service admin token is a short-lived runner-management credential, not a requirement for the GitHub App's Repository Administration permission.

Copy the App Client ID (distinct from the App ID) and the installation ID (also distinct from the App ID). Download the App's private key, store it in a protected file readable only by the outrunner account, and remove other copies. For example:

```bash
sudo install -o outrunner -g outrunner -m 0600 github-app-private-key.pem /etc/outrunner/github-app.pem
```

Configure the App:

```yaml
url: https://github.com/your-org/your-repo
app:
  client_id: Iv1.your-client-id
  installation_id: 12345678
  private_key_file: /etc/outrunner/github-app.pem

runners:
  linux:
    labels: [self-hosted, linux]
    docker:
      image: ghcr.io/actions/actions-runner:latest
```

### Personal Access Token

For PAT authentication, create a fine-grained token at [github.com/settings/tokens?type=beta](https://github.com/settings/tokens?type=beta):

- **Token name:** outrunner
- **Resource owner:** Your user or organization
- **Repository access:** Select the repository you want to use
- **Permissions:** Administration -> Read and write

## 3. Write a config file

Create `outrunner.yml` for the method you chose. For App authentication, use the configuration above; for PAT authentication, retain the configuration below and supply the token with `--token`:

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

For App authentication, run `outrunner --config outrunner.yml` without `--token`.

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
