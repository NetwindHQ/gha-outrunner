# Install on Fedora / RHEL / CentOS

This guide sets up outrunner with Docker. Make sure Docker is installed before continuing. For other backends, see the [backend guides](#next-steps) after completing setup.

## 1. Add the repository

```bash
sudo dnf config-manager addrepo \
  --from-repofile=https://pkg.netwind.pl/NetwindHQ/gha-outrunner/outrunner.repo
```

## 2. Install outrunner

```bash
sudo dnf install outrunner
```

dnf will ask you to import the GPG key on first install. Verify the fingerprint and accept.

This installs the binary, systemd service, and a default config at `/etc/outrunner/config.yml`.

Add the outrunner user to the docker group so it can access the Docker socket:

```bash
sudo usermod -aG docker outrunner
```

## 3. Create a GitHub PAT

Go to [github.com/settings/tokens?type=beta](https://github.com/settings/tokens?type=beta) and create a fine-grained token:

- **Token name:** outrunner
- **Resource owner:** Your user or organization
- **Repository access:** Select the repository you want to use
- **Permissions:** Administration -> Read and write

## 4. Set up the token

The simplest option is a token file:

```bash
echo -n "ghp_YOUR_TOKEN" | sudo tee /etc/outrunner/token
sudo chmod 600 /etc/outrunner/token
sudo chown outrunner:outrunner /etc/outrunner/token
```

Alternatively, use **systemd-creds** for encryption at rest (systemd v250+):

```bash
echo -n "ghp_YOUR_TOKEN" | sudo systemd-creds encrypt --name=github-token - /etc/outrunner/github-token.cred
sudo chown outrunner:outrunner /etc/outrunner/github-token.cred
sudo systemctl edit outrunner
```

Add to the override:

```ini
[Service]
LoadCredentialEncrypted=github-token:/etc/outrunner/github-token.cred
```

Or use an **environment file**:

```bash
echo 'GITHUB_TOKEN=ghp_YOUR_TOKEN' | sudo tee /etc/outrunner/env
sudo chmod 600 /etc/outrunner/env
sudo chown outrunner:outrunner /etc/outrunner/env
```

## 5. Edit the config

Edit `/etc/outrunner/config.yml` - set the `url` to your repository or organization and uncomment the `runners` section:

```yaml
url: https://github.com/your-org/your-repo
token_file: /etc/outrunner/token

runners:
  linux:
    labels: [self-hosted, linux]
    docker:
      image: ghcr.io/actions/actions-runner:latest
```

See the [configuration reference](../reference/configuration.md) for all options.

## 6. Start the service

```bash
sudo systemctl enable --now outrunner
sudo journalctl -u outrunner -f
```

You should see outrunner connect and start listening for jobs.

## 7. Run a test workflow

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

In the journal you should see the runner spawn, pick up the job, and clean up:

```
sudo journalctl -u outrunner -f
```

## Next steps

outrunner is now running with Docker. For other backends:

- [Windows VMs via libvirt/KVM](../tutorial/libvirt-windows.md)
- [macOS VMs via Tart](../tutorial/tart-macos.md)
- [Linux ARM64 VMs via Tart](../tutorial/tart-linux.md)
