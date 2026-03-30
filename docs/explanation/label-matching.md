# How Label Matching Works

## Overview

GitHub handles all label matching between workflows and runners. outrunner registers each runner's labels on its own scale set, and GitHub routes jobs to the correct scale set based on `runs-on` labels.

## Scale Set Registration

On startup, outrunner creates one scale set per runner defined in the config. Each scale set is registered with the labels declared in that runner's `labels` array:

```yaml
runners:
  linux-docker:
    labels: [self-hosted, linux, x64]    # registered on the "linux-docker" scale set
    docker:
      image: runner:latest
  windows-vm:
    labels: [self-hosted, windows, x64]  # registered on the "windows-vm" scale set
    libvirt:
      path: /images/win.qcow2
```

This creates two independent scale sets, each with its own labels.

## How `runs-on` Maps to Scale Sets

When a workflow specifies `runs-on`, GitHub matches those labels against all registered scale sets:

```yaml
jobs:
  build-linux:
    runs-on: [self-hosted, linux, x64]   # matches "linux-docker" scale set

  build-windows:
    runs-on: [self-hosted, windows, x64] # matches "windows-vm" scale set
```

GitHub finds the scale set whose labels satisfy the `runs-on` requirement and routes the job to it. outrunner receives the job on the matching runner's listener and provisions the correct backend automatically.

## No Internal Label Routing

In the previous architecture, outrunner had a single scale set with all labels and performed internal label routing via a MultiProvisioner. This is no longer the case. Each runner has its own scale set, and GitHub handles all routing. outrunner simply provisions whatever the listener for that runner receives.

## Per-Runner Concurrency

Since each runner has its own scale set, concurrency limits are per-runner. You can set `max_runners` independently:

```yaml
runners:
  linux:
    labels: [self-hosted, linux]
    max_runners: 8          # Docker is fast and lightweight
    docker:
      image: runner:latest

  macos:
    labels: [self-hosted, macos]
    max_runners: 2          # VMs are heavier
    tart:
      image: macos-runner
```

If `max_runners` is not set on a runner, it defaults to the `--max-runners` CLI flag value.

## Multiple Labels per Runner

Each runner can declare multiple labels. This is useful for matching workflows that use compound `runs-on` expressions:

```yaml
runners:
  linux-gpu:
    labels: [self-hosted, linux, gpu]
    docker:
      image: runner-with-cuda:latest
```

```yaml
jobs:
  train:
    runs-on: [self-hosted, linux, gpu]   # matches "linux-gpu" scale set
```

## Tips

- Use descriptive scale set names (the config map key). They appear in GitHub's runner UI and in log output.
- Include `self-hosted` in your labels if your workflows use it, since GitHub requires all `runs-on` labels to match.
- If you change a runner's labels in the config, the stale scale set may need to be deleted first. outrunner reuses existing scale sets by name without checking whether labels still match.
