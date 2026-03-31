# How to Run Multiple Backends Together

A single outrunner instance can serve Docker containers, libvirt VMs, and Tart VMs simultaneously. Each runner in the config gets its own scale set, and GitHub routes jobs to the correct one based on labels.

## Configuration

Define all runners in one config file. Each runner declares its labels and backend:

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
      runner_cmd: /Users/admin/actions-runner/run.sh
      cpus: 4
      memory: 8192
```

outrunner initializes only the backends that are needed. If no runner uses Docker, the Docker client is never created.

Restart outrunner to pick up the new config.

## Workflow Usage

```yaml
jobs:
  build-linux:
    runs-on: [self-hosted, linux]
    steps:
      - run: echo "Docker container"

  build-windows:
    runs-on: [self-hosted, windows]
    steps:
      - run: echo "KVM virtual machine"

  build-macos:
    runs-on: [self-hosted, macos]
    steps:
      - run: echo "Tart virtual machine"
```

## Multiple Runners on the Same Backend

You can define multiple runners for the same backend with different labels:

```yaml
runners:
  linux-small:
    labels: [self-hosted, linux, small]
    docker:
      image: runner:latest

  linux-gpu:
    labels: [self-hosted, linux, gpu]
    docker:
      image: runner-with-cuda:latest

  windows-2022:
    labels: [self-hosted, windows, win2022]
    libvirt:
      path: /images/win2022.qcow2
      runner_cmd: 'C:\actions-runner\run.cmd'

  windows-2025:
    labels: [self-hosted, windows, win2025]
    libvirt:
      path: /images/win2025.qcow2
      runner_cmd: 'C:\actions-runner\run.cmd'
```

## Per-Runner Concurrency

Each runner can have its own `max_runners` limit. This is useful when backends have different resource costs:

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

## Platform Constraints

Not all backends work on all hosts:

| Backend | Linux host | macOS host |
|---------|-----------|------------|
| Docker  | Yes       | Yes (via Colima/Docker Desktop) |
| Libvirt | Yes       | No |
| Tart    | No        | Yes (Apple Silicon only) |

A single host can run Docker + libvirt (Linux) or Docker + Tart (macOS), but not all three.
