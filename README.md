# gha-outrunner

Ephemeral GitHub Actions runners — no Kubernetes required.

outrunner provisions fresh Docker containers (or VMs) for each GitHub Actions job, then destroys them when the job completes. It uses GitHub's [scaleset API](https://github.com/actions/scaleset) to register as an autoscaling runner group.

**Why?** GitHub's [Actions Runner Controller (ARC)](https://github.com/actions/actions-runner-controller) requires Kubernetes. If you're running on bare metal or a simple VPS, you shouldn't need a cluster just to get ephemeral runners. outrunner gives you the same isolation guarantees with Docker or libvirt.

## Status

Early development. Docker provisioner works. libvirt (VM) provisioner planned.

## Quick Start

```bash
# Build
go build -o outrunner ./cmd/outrunner

# Build the runner image
docker build -t outrunner-runner runner/

# Run (needs a fine-grained PAT with Administration read/write)
./outrunner \
  --url https://github.com/your/repo \
  --token ghp_xxx \
  --name outrunner \
  --image outrunner-runner \
  --max-runners 2
```

Then in your workflow:

```yaml
jobs:
  build:
    runs-on: outrunner  # matches --name
    steps:
      - uses: actions/checkout@v4
      - run: echo "Running in an ephemeral container!"
```

## Architecture

```
GitHub ──(scaleset API)──► outrunner ──(Docker/libvirt)──► ephemeral runner
                              │
                    polls for job demand
                    generates JIT configs
                    provisions runners
                    tears down after job
```

outrunner implements the `Provisioner` interface:

```go
type Provisioner interface {
    Start(ctx context.Context, req *RunnerRequest) error
    Stop(ctx context.Context, name string) error
    Close() error
}
```

Current provisioners:
- **Docker** — creates a container per job, auto-removes on completion
- **libvirt** — (planned) boots a VM from a qcow2 image per job

## Provisioners Roadmap

| Provisioner | Status | Use case |
|-------------|--------|----------|
| Docker | Working | Linux jobs, fastest startup |
| libvirt/QEMU | Planned | Windows/Linux VMs, full OS isolation |
| Tart | Future | macOS on Apple Silicon |

## License

MIT
