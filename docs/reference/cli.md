# CLI Reference

## Install

```bash
# macOS
brew tap NetwindHQ/tap
brew install outrunner

# Ubuntu / Debian (automatically adds apt repo for future updates)
curl -LO https://github.com/NetwindHQ/gha-outrunner/releases/latest/download/outrunner_amd64.deb
sudo dpkg -i outrunner_amd64.deb

# Fedora / RHEL
sudo dnf config-manager addrepo \
  --from-repofile=https://pkg.netwind.pl/NetwindHQ/gha-outrunner/outrunner.repo
sudo dnf install outrunner

# From source
go install github.com/NetwindHQ/gha-outrunner/cmd/outrunner@latest
```

The deb package automatically sets up the apt repository at `pkg.netwind.pl` so future updates arrive via `apt upgrade`. To skip this, install with `OUTRUNNER_NO_REPO=1 dpkg -i outrunner_*.deb`.

## Usage

```
outrunner [flags]
```

outrunner registers a GitHub Actions [scale set](https://github.com/actions/scaleset) on a repository or organization, then listens for jobs and provisions ephemeral runner environments for each one.

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `/etc/outrunner/config.yml` | Config file path. |
| `--url` | string | | Repository or org URL. Overrides `url` in config. |
| `--token` | string | | GitHub PAT. Overrides env var and config. |
| `--max-runners` | int | `2` | Default max concurrent runners per scale set. |
| `-h`, `--help` | | | Show help. |
| `-v`, `--version` | | | Print version. |

## Token Resolution

The GitHub token is resolved in this order:

1. `--token` CLI flag
2. `GITHUB_TOKEN` environment variable
3. `$CREDENTIALS_DIRECTORY/github-token` (systemd-creds)
4. `token_file` in config file

For production deployments, use systemd-creds (encrypted at rest) or an environment file. See the [setup guides](../setup/).

## Examples

As a service (config has url and token_file):

```bash
outrunner
```

With CLI overrides (for testing):

```bash
outrunner \
  --url https://github.com/myorg/myrepo \
  --token ghp_xxx \
  --config outrunner.yml
```

## Behavior

- On startup, outrunner creates one scale set per runner defined in the config file. Each scale set is named after the runner's key in the `runners` map. If a scale set with that name already exists, it reuses it (updating labels if they changed).
- Scale sets are kept across restarts. They are not deleted on shutdown.
- On shutdown (Ctrl+C / SIGINT), outrunner stops all running environments and deregisters their runners from GitHub.
- If outrunner is force-killed (SIGKILL), running environments may be left behind. On next startup, each provisioner cleans up orphaned environments whose names start with the runner's key.

## GitHub PAT Requirements

Create a **fine-grained** Personal Access Token at [github.com/settings/tokens](https://github.com/settings/tokens?type=beta):

- **Resource owner:** The organization or user that owns the repository.
- **Repository access:** Select the target repository (or all repositories for org-wide use).
- **Permissions:** Administration -> Read and write.

Classic tokens also work but fine-grained tokens are recommended for least-privilege.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Clean shutdown (Ctrl+C) |
| 1 | Error (invalid config, authentication failure, listener error) |
