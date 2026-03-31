# Install on macOS

This guide sets up outrunner with Docker. You'll need Docker running via Colima, Docker Desktop, or Podman. For VM-based backends (Tart), see the [backend guides](#next-steps) after completing setup.

## 1. Install outrunner

```bash
brew tap NetwindHQ/tap
brew install outrunner
```

## 2. Create a GitHub PAT

Go to [github.com/settings/tokens?type=beta](https://github.com/settings/tokens?type=beta) and create a fine-grained token:

- **Token name:** outrunner
- **Resource owner:** Your user or organization
- **Repository access:** Select the repository you want to use
- **Permissions:** Administration -> Read and write

## 3. Set up the token

```bash
echo -n "ghp_YOUR_TOKEN" > $(brew --prefix)/etc/outrunner/token
chmod 600 $(brew --prefix)/etc/outrunner/token
```

## 4. Edit the config

Edit `$(brew --prefix)/etc/outrunner/config.yml` - set the `url` to your repository or organization and uncomment the `runners` section:

```yaml
url: https://github.com/your-org/your-repo
token_file: /opt/homebrew/etc/outrunner/token

runners:
  linux:
    labels: [self-hosted, linux]
    docker:
      image: ghcr.io/actions/actions-runner:latest
```

See the [configuration reference](../reference/configuration.md) for all options.

## 5. Start the service

```bash
brew services start outrunner
tail -f $(brew --prefix)/var/log/outrunner.log
```

You should see outrunner connect and start listening for jobs.

## 6. Run a test workflow

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
      - run: uname -a
```

Push this file, then go to GitHub -> Actions -> "Test Outrunner" -> "Run workflow".

Check the log:

```bash
tail -f $(brew --prefix)/var/log/outrunner.log
```

## Next steps

outrunner is now running with Docker. For other backends:

- [macOS VMs via Tart](../tutorial/tart-macos.md)
- [Linux ARM64 VMs via Tart](../tutorial/tart-linux.md)
