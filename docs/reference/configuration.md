# Configuration Reference

outrunner uses a YAML configuration file to define runners. Each entry in the `runners` map defines a named runner with its labels and provisioner backend. Each runner gets its own GitHub scale set.

## Schema

```yaml
runners:
  <scale-set-name>:            # Required. The key becomes the GitHub scale set name.
    labels: [<string>, ...]    # Required. Labels registered on this scale set.
    max_runners: <int>         # Optional. Max concurrent runners for this scale set. Defaults to --max-runners flag.
    docker:                    # Use Docker backend.
      image: <string>          # Docker image name or tag.
      runner_cmd: <string>     # Command to start the runner. Default: ./run.sh
    libvirt:                   # Use libvirt/KVM backend.
      path: <string>           # Path to the base qcow2 disk image.
      runner_cmd: <string>     # Command to start the runner. Default: /actions-runner/run.sh
      cpus: <int>              # vCPU count. Default: 4
      memory: <int>            # Memory in MiB. Default: 8192
    tart:                      # Use Tart backend.
      image: <string>          # OCI image URL or local VM name.
      runner_cmd: <string>     # Command to start the runner. Default: /actions-runner/run.sh
      cpus: <int>              # vCPU count. Default: 4
      memory: <int>            # Memory in MiB. Default: 8192
```

## Rules

- Each runner must have a unique key (used as the scale set name).
- Each runner must have `labels` (an array of one or more strings).
- Each runner must specify exactly one of `docker`, `libvirt`, or `tart`.
- Multiple runners can use the same backend with different labels.
- GitHub handles label matching. When a workflow uses `runs-on`, GitHub routes the job to the scale set whose labels match.

## Fields

### `runners.<name>`

**Required.** The map key becomes the name of the GitHub scale set. It is also used as a prefix for runner names and orphan cleanup.

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

**Optional.** Maximum number of concurrent runners for this scale set. If not specified, defaults to the `--max-runners` CLI flag value.

```yaml
runners:
  linux-docker:
    labels: [self-hosted, linux]
    max_runners: 8
    docker:
      image: runner:latest
  macos-tart:
    labels: [self-hosted, macos]
    max_runners: 2
    tart:
      image: macos-runner
```

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

Docker only:

```yaml
runners:
  linux:
    labels: [self-hosted, linux]
    docker:
      image: outrunner-runner:latest
```

Mixed backends:

```yaml
runners:
  linux:
    labels: [self-hosted, linux]
    docker:
      image: outrunner-runner:latest

  windows:
    labels: [self-hosted, windows]
    libvirt:
      path: /var/lib/libvirt/images/windows-builder.qcow2
      runner_cmd: 'C:\actions-runner\run.cmd'
      cpus: 4
      memory: 8192

  macos:
    labels: [self-hosted, macos]
    tart:
      image: ghcr.io/cirruslabs/macos-sequoia-base:latest
      runner_cmd: /actions-runner/run.sh
      cpus: 4
      memory: 8192
```

Multiple runners on the same backend:

```yaml
runners:
  linux-small:
    labels: [self-hosted, linux, small]
    docker:
      image: runner:latest

  linux-large:
    labels: [self-hosted, linux, large]
    docker:
      image: runner-with-tools:latest
```

Per-runner concurrency limits:

```yaml
runners:
  linux:
    labels: [self-hosted, linux]
    max_runners: 8
    docker:
      image: runner:latest

  macos:
    labels: [self-hosted, macos]
    max_runners: 2
    tart:
      image: macos-runner
```

## Label Matching

GitHub handles all label matching. Each runner in the config gets its own scale set with its declared labels. When a workflow uses `runs-on`, GitHub routes the job to the scale set whose labels match. outrunner does not perform any label routing internally.
