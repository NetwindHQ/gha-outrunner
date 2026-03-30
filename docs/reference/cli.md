# CLI Reference

## Install

```bash
# macOS
brew tap NetwindHQ/tap
brew install outrunner

# Ubuntu / Debian
curl -LO https://github.com/NetwindHQ/gha-outrunner/releases/latest/download/outrunner_amd64.deb
sudo dpkg -i outrunner_amd64.deb

# From source
go install github.com/NetwindHQ/gha-outrunner/cmd/outrunner@latest
```

## Usage

```
outrunner [flags]
```

outrunner registers a GitHub Actions [scale set](https://github.com/actions/scaleset) on a repository or organization, then listens for jobs and provisions ephemeral runner environments for each one.

## Flags

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--url` | string | | Yes | Repository or organization URL. Examples: `https://github.com/owner/repo`, `https://github.com/org` |
| `--token` | string | | Yes | GitHub Personal Access Token (fine-grained). Needs **Administration** read/write permission on the target repository or organization. |
| `--config` | string | | Yes | Path to the YAML configuration file. See [Configuration Reference](configuration.md). |
| `--max-runners` | int | `2` | No | Default maximum number of concurrent runners per scale set. Individual runners can override this via `max_runners` in the config file. |
| `-h`, `--help` | | | No | Show help. |

## Examples

Single-repo, Docker-only:

```bash
./outrunner \
  --url https://github.com/myorg/myrepo \
  --token ghp_xxx \
  --config outrunner.yml
```

Organization-wide, higher default concurrency:

```bash
./outrunner \
  --url https://github.com/myorg \
  --token ghp_xxx \
  --config outrunner.yml \
  --max-runners 8
```

## Behavior

- On startup, outrunner creates one scale set per runner defined in the config file. Each scale set is named after the runner's key in the `runners` map. If a scale set with that name already exists, it is reused.
- Scale sets are kept across restarts. They are not deleted on shutdown.
- On shutdown (Ctrl+C / SIGINT), outrunner stops all running environments and deregisters their runners from GitHub.
- If outrunner is force-killed (SIGKILL), running environments may be left behind. On next startup, each provisioner cleans up orphaned environments whose names start with the runner's key.

## GitHub PAT Requirements

Create a **fine-grained** Personal Access Token at [github.com/settings/tokens](https://github.com/settings/tokens?type=beta):

- **Resource owner:** The organization or user that owns the repository.
- **Repository access:** Select the target repository (or all repositories for org-wide use).
- **Permissions:** Administration → Read and write.

Classic tokens also work but fine-grained tokens are recommended for least-privilege.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Clean shutdown (Ctrl+C) |
| 1 | Error (invalid config, authentication failure, listener error) |
