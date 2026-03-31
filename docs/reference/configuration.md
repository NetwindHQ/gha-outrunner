# Configuration Reference

outrunner uses a YAML configuration file. Default location: `/etc/outrunner/config.yml` (override with `--config`).

## Schema

```yaml
url: <string>                        # Repository or org URL.
token_file: <string>                 # Path to a file containing the GitHub token.

runners:
  <scale-set-name>:                  # The key becomes the GitHub scale set name.
    labels: [<string>, ...]          # Labels registered on this scale set.
    max_runners: <int>               # Optional. Defaults to --max-runners flag.
    docker:                          # Use Docker backend.
      image: <string>                # Docker image name or tag.
      runner_cmd: <string>           # Default: ./run.sh
    libvirt:                         # Use libvirt/KVM backend.
      path: <string>                 # Path to the base qcow2 disk image.
      runner_cmd: <string>           # Default: /actions-runner/run.sh
      socket: <string>               # Default: /var/run/libvirt/libvirt-sock
      cpus: <int>                    # Default: 4
      memory: <int>                  # Default: 8192 (MiB)
    tart:                            # Use Tart backend.
      image: <string>                # OCI image URL or local VM name.
      runner_cmd: <string>           # Default: /actions-runner/run.sh
      cpus: <int>                    # Default: 4
      memory: <int>                  # Default: 8192 (MiB)
```

## Top-Level Fields

### `url`

Repository or organization URL. Can also be set via the `--url` CLI flag (which takes precedence).

```yaml
url: https://github.com/myorg/myrepo
```

### `token_file`

Path to a file containing the GitHub PAT. The file should contain just the token, with optional trailing whitespace/newline.

```yaml
token_file: /etc/outrunner/token
```

Token resolution precedence:
1. `--token` CLI flag
2. `GITHUB_TOKEN` environment variable
3. `$CREDENTIALS_DIRECTORY/github-token` (systemd-creds)
4. `token_file` from config

See the [Linux setup guides](../setup/linux-deb.md) for details on each method.

## Runner Fields

### `runners.<name>`

**Required.** The map key becomes the name of the GitHub scale set. It is also used as a prefix for runner names and orphan cleanup. Each runner must specify exactly one of `docker`, `libvirt`, or `tart`.

### `runners.<name>.labels`

**Required.** An array of labels registered on this runner's scale set. GitHub uses these labels to route jobs.

```yaml
# In config:
runners:
  linux-docker:
    labels: [self-hosted, linux, x64]
    docker:
      image: runner:latest

# In workflow:
jobs:
  build:
    runs-on: [self-hosted, linux, x64]
```

### `runners.<name>.max_runners`

**Optional.** Maximum number of concurrent runners for this scale set. If not specified, defaults to the `--max-runners` CLI flag value (default: 2).

### `runners.<name>.docker`

Provisions a Docker container per job.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `image` | string | (required) | Docker image reference. Can be a local tag (`runner:latest`) or a registry reference (`ghcr.io/org/runner:v1`). |
| `runner_cmd` | string | `./run.sh` | Command to start the runner inside the container. |

The container runs `<runner_cmd> --jitconfig <config>` as its command. The image must have the GitHub Actions runner installed at the working directory. See [Image Requirements](image-requirements.md).

### `runners.<name>.libvirt`

Provisions a KVM/QEMU virtual machine per job using a copy-on-write overlay.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | (required) | Absolute path to the base qcow2 disk image. This image is never modified; each job gets a CoW overlay. |
| `runner_cmd` | string | `/actions-runner/run.sh` | Command to execute inside the VM via the QEMU Guest Agent. For Windows: `C:\actions-runner\run.cmd` |
| `socket` | string | `/var/run/libvirt/libvirt-sock` | Path to the libvirtd Unix socket. |
| `cpus` | int | `4` | Number of vCPUs allocated to the VM. |
| `memory` | int | `8192` | Memory in MiB allocated to the VM. |

### `runners.<name>.tart`

Provisions a Tart virtual machine per job by cloning from a base image.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `image` | string | (required) | Tart image source. Can be an OCI registry reference (`ghcr.io/cirruslabs/macos-sequoia-base:latest`) or a local VM name. |
| `runner_cmd` | string | `/actions-runner/run.sh` | Command to execute inside the VM via `tart exec`. |
| `cpus` | int | `4` | Number of vCPUs allocated to the VM. |
| `memory` | int | `8192` | Memory in MiB allocated to the VM. |

## Examples

Minimal (Docker, single runner):

```yaml
url: https://github.com/myorg/myrepo
token_file: /etc/outrunner/token

runners:
  linux:
    labels: [self-hosted, linux]
    docker:
      image: outrunner-runner:latest
```

Mixed backends:

```yaml
url: https://github.com/myorg
token_file: /etc/outrunner/token

runners:
  linux:
    labels: [self-hosted, linux]
    max_runners: 4
    docker:
      image: outrunner-runner:latest

  windows:
    labels: [self-hosted, windows]
    max_runners: 1
    libvirt:
      path: /var/lib/libvirt/images/windows-builder.qcow2
      runner_cmd: 'C:\actions-runner\run.cmd'
      cpus: 4
      memory: 8192

  macos:
    labels: [self-hosted, macos]
    max_runners: 1
    tart:
      image: ghcr.io/cirruslabs/macos-sequoia-base:latest
      runner_cmd: /actions-runner/run.sh
      cpus: 4
      memory: 8192
```
