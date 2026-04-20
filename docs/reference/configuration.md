# Configuration Reference

outrunner uses a YAML configuration file. Default location: `/etc/outrunner/config.yml` (override with `--config`).

## Schema

```yaml
url: <string>                        # Global repository or org URL.
token_file: <string>                 # Global path to a file containing the GitHub token.

runners:
  <scale-set-name>:                  # The key becomes the GitHub scale set name.
    url: <string>                    # Optional per-runner URL override.
    token_file: <string>             # Optional per-runner token file override.
    labels: [<string>, ...]          # Labels registered on this scale set.
    max_runners: <int>               # Optional. Defaults to --max-runners flag.
    docker:                          # Use Docker backend.
      image: <string>                # Docker image name or tag.
      runner_cmd: <string>           # Default: ./run.sh
      mounts:                        # Optional bind mounts.
        - source: <string>           # Host path.
          target: <string>           # Container path.
          read_only: <bool>          # Default: false
    libvirt:                         # Use libvirt/KVM backend.
      path: <string>                 # Path to the base qcow2 disk image.
      runner_cmd: <string>           # Default: /actions-runner/run.sh
      socket: <string>               # Default: /var/run/libvirt/libvirt-sock
      cpus: <int>                    # Default: 4
      memory: <int>                  # Default: 8192 (MiB)
      mount:                         # Optional virtiofs host directory share.
        source: <string>             # Host path. Tag is derived from the basename.
    tart:                            # Use Tart backend.
      image: <string>                # OCI image URL or local VM name.
      runner_cmd: <string>           # Default: /actions-runner/run.sh
      cpus: <int>                    # Default: 4
      memory: <int>                  # Default: 8192 (MiB)
      mounts:                        # Optional shared directories (--dir).
        - name: <string>             # Directory name inside the guest.
          source: <string>           # Host path.
          read_only: <bool>          # Default: false
```

## Top-Level Fields

### `url`

Global repository or organization URL. Applies to all runners that don't set their own `url`. Can also be set via the `--url` CLI flag (which takes precedence over this value).

Every runner must have a URL — either from this global field, the `--url` flag, or a per-runner `url` override.

```yaml
url: https://github.com/myorg/myrepo
```

### `token_file`

Global path to a file containing the GitHub PAT. The file should contain just the token, with optional trailing whitespace/newline. Applies to all runners that don't set their own `token_file`.

```yaml
token_file: /etc/outrunner/token
```

Token resolution precedence:
1. Per-runner `token_file` (if set on the runner)
2. `--token` CLI flag
3. `GITHUB_TOKEN` environment variable
4. `$CREDENTIALS_DIRECTORY/github-token` (systemd-creds)
5. Global `token_file` from config

See the [Linux setup guides](../setup/linux-deb.md) for details on each method.

## Runner Fields

### `runners.<name>`

**Required.** The map key becomes the name of the GitHub scale set. It is also used as a prefix for runner names and orphan cleanup. Each runner must specify exactly one of `docker`, `libvirt`, or `tart`.

### `runners.<name>.url`

**Optional.** Per-runner URL override. When set, this runner registers its scale set against this repository or organization instead of the global `url`. This enables a single outrunner instance to serve multiple repositories.

```yaml
runners:
  repo-b-linux:
    url: https://github.com/myorg/repo-b
    labels: [self-hosted, linux]
    docker:
      image: runner:latest
```

### `runners.<name>.token_file`

**Optional.** Per-runner token file override. When set, this runner authenticates with the token from this file instead of the global token resolution chain. Useful when different repositories require different PATs.

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
| `mounts` | list | `[]` | Bind mounts to attach to the container. See below. |

The container runs `<runner_cmd> --jitconfig <config>` as its command. The image must have the GitHub Actions runner installed at the working directory. See [Image Requirements](image-requirements.md).

**`mounts` entries:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `source` | string | (required) | Absolute path on the host. |
| `target` | string | (required) | Absolute path inside the container. |
| `read_only` | bool | `false` | Mount read-only. |

### `runners.<name>.libvirt`

Provisions a KVM/QEMU virtual machine per job using a copy-on-write overlay.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | (required) | Absolute path to the base qcow2 disk image. This image is never modified; each job gets a CoW overlay. |
| `runner_cmd` | string | `/actions-runner/run.sh` | Command to execute inside the VM via the QEMU Guest Agent. For Windows: `C:\actions-runner\run.cmd` |
| `socket` | string | `/var/run/libvirt/libvirt-sock` | Path to the libvirtd Unix socket. |
| `cpus` | int | `4` | Number of vCPUs allocated to the VM. |
| `memory` | int | `8192` | Memory in MiB allocated to the VM. |
| `mount` | object | (none) | Optional virtiofs host directory share. See below. |

**`mount` fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `source` | string | (required) | Absolute path on the host to share into the VM. The virtiofs tag is derived from the directory basename. |

The share is exposed via virtiofs. On Windows guests, `VirtioFsSvc` mounts it automatically as a drive letter (Z: by default, counting down). Requires WinFsp (`choco install -y winfsp`) and `VirtioFsSvc` set to auto-start (`Set-Service -Name VirtioFsSvc -StartupType Automatic`). `virtiofsd` must be installed on the host (`apt install virtiofsd`).

### `runners.<name>.tart`

Provisions a Tart virtual machine per job by cloning from a base image.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `image` | string | (required) | Tart image source. Can be an OCI registry reference (`ghcr.io/cirruslabs/macos-sequoia-base:latest`) or a local VM name. |
| `runner_cmd` | string | `/actions-runner/run.sh` | Command to execute inside the VM via `tart exec`. |
| `cpus` | int | `4` | Number of vCPUs allocated to the VM. |
| `memory` | int | `8192` | Memory in MiB allocated to the VM. |
| `mounts` | list | `[]` | Shared directories passed to `tart run --dir`. See below. |

**`mounts` entries:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | (required) | Directory name as it appears inside the guest. On macOS guests this is a subdirectory of `/Volumes/My Shared Files/`; on Linux guests mount the share manually with `mount -t virtiofs com.apple.virtio-fs.automount <mountpoint>`. |
| `source` | string | (required) | Absolute path on the host. |
| `read_only` | bool | `false` | Mount read-only. |

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

Mixed backends with shared cache:

```yaml
url: https://github.com/myorg
token_file: /etc/outrunner/token

runners:
  linux:
    labels: [self-hosted, linux]
    max_runners: 4
    docker:
      image: outrunner-runner:latest
      mounts:
        - source: /var/cache/vcpkg
          target: /opt/vcpkg-cache

  windows:
    labels: [self-hosted, windows]
    max_runners: 1
    libvirt:
      path: /var/lib/libvirt/images/ci-runners/windows-builder.qcow2
      runner_cmd: 'C:\actions-runner\run.cmd'
      cpus: 4
      memory: 8192
      mount:
        source: /var/cache/vcpkg

  macos:
    labels: [self-hosted, macos]
    max_runners: 1
    tart:
      image: ghcr.io/cirruslabs/macos-sequoia-base:latest
      runner_cmd: /Users/admin/actions-runner/run.sh
      cpus: 4
      memory: 8192
      mounts:
        - name: vcpkg
          source: /var/cache/vcpkg
```

Multi-repo (runners targeting different repositories):

```yaml
url: https://github.com/myorg/main-repo
token_file: /etc/outrunner/token

runners:
  main-linux:
    labels: [self-hosted, linux]
    docker:
      image: runner:latest

  # This runner targets a different repo, with its own token.
  infra-linux:
    url: https://github.com/myorg/infra-repo
    token_file: /etc/outrunner/token-infra
    labels: [self-hosted, linux]
    docker:
      image: runner:latest
```

Multi-repo without a global URL (every runner specifies its own):

```yaml
token_file: /etc/outrunner/token

runners:
  repo-a:
    url: https://github.com/myorg/repo-a
    labels: [self-hosted, linux]
    docker:
      image: runner:latest

  repo-b:
    url: https://github.com/myorg/repo-b
    labels: [self-hosted, linux]
    docker:
      image: runner:latest
```
