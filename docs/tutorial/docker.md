# Docker Runners

This guide assumes outrunner is already installed and running. If not, start with one of the [setup guides](../setup/).

## Runner image

You need a Docker image with the GitHub Actions runner agent. You can use the official image or build your own.

### Option A: Official image

```bash
docker pull ghcr.io/actions/actions-runner:latest
```

### Option B: Build a custom image

Clone the repository and build:

```bash
git clone https://github.com/NetwindHQ/gha-outrunner.git
cd gha-outrunner
docker build -t outrunner-runner runner/
```

See [Build a custom Docker runner image](../howto/custom-docker-image.md) for more details.

## Configuration

In your outrunner config, add a runner with the `docker` backend:

```yaml
runners:
  linux:
    labels: [self-hosted, linux]
    docker:
      image: ghcr.io/actions/actions-runner:latest
```

Or with a custom image:

```yaml
runners:
  linux:
    labels: [self-hosted, linux]
    docker:
      image: outrunner-runner:latest
      runner_cmd: ./run.sh
```

`runner_cmd` defaults to `./run.sh` and can be omitted for most images.

## Docker socket

outrunner auto-detects the Docker socket. On Linux it uses `/var/run/docker.sock`. On macOS it detects Colima, Docker Desktop, or Podman sockets automatically.

To override, set the `DOCKER_HOST` environment variable.

## How it works

1. outrunner pulls the image if not available locally.
2. Creates a container with the JIT registration token.
3. The runner inside the container registers with GitHub, picks up the job, and runs it.
4. On job completion, outrunner stops the container. It auto-removes itself (`AutoRemove: true`).

## Notes

- Docker containers share the host kernel. For stronger isolation, use [libvirt](libvirt-windows.md) or [Tart](tart-macos.md).
- On macOS, Docker runs inside a Linux VM (Colima, Docker Desktop). Performance is slightly lower but rarely noticeable for CI.
- On Apple Silicon, `docker build` produces ARM64 images by default.

## Next steps

- [Build a custom Docker runner image](../howto/custom-docker-image.md)
- [Run multiple backends together](../howto/mixed-backends.md)
- [Configuration reference](../reference/configuration.md)
