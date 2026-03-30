# How to Build a Custom Docker Runner Image

The included `runner/Dockerfile` provides a minimal runner image. Extend it to pre-install tools your workflows need.

## Start From the Base

```dockerfile
FROM ghcr.io/actions/actions-runner:latest

USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    git \
    jq \
    && rm -rf /var/lib/apt/lists/*
USER runner
```

## Add Your Tools

Add package installs before the final `USER runner` line:

```dockerfile
FROM ghcr.io/actions/actions-runner:latest

USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    git \
    jq \
    # Add your tools below
    build-essential \
    cmake \
    python3 \
    python3-pip \
    nodejs \
    npm \
    && rm -rf /var/lib/apt/lists/*
USER runner
```

## Install Tools That Need Custom Steps

For tools not in apt:

```dockerfile
FROM ghcr.io/actions/actions-runner:latest

USER root

# System packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl git jq build-essential \
    && rm -rf /var/lib/apt/lists/*

# Go
RUN curl -sL https://go.dev/dl/go1.22.0.linux-amd64.tar.gz | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

# Rust
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"

USER runner
```

## Build and Test

```bash
docker build -t my-runner runner/
docker run --rm my-runner go version   # verify tools are available
```

## Use in Config

```yaml
runners:
  linux:
    labels: [self-hosted, linux]
    docker:
      image: my-runner:latest
```

## Multiple Images for Different Workloads

Create separate Dockerfiles for different workloads:

```
runner/
├── Dockerfile              # base image
├── Dockerfile.go           # Go development
├── Dockerfile.node         # Node.js development
└── Dockerfile.heavy        # full toolchain
```

```bash
docker build -t runner-go -f runner/Dockerfile.go runner/
docker build -t runner-node -f runner/Dockerfile.node runner/
```

```yaml
runners:
  linux-go:
    labels: [self-hosted, linux, go]
    docker:
      image: runner-go:latest
  linux-node:
    labels: [self-hosted, linux, node]
    docker:
      image: runner-node:latest
```

## Tips

- **Keep images small.** Every MB is pulled (or takes up disk) for every job. Use `--no-install-recommends` and clean up apt caches.
- **Don't bake secrets into images.** Use GitHub Actions secrets or OIDC in your workflows instead.
- **Pin versions.** Use specific tags rather than `latest` for reproducibility.
- **Pre-install what's slow.** If a workflow step takes minutes to install something, bake it into the image.
