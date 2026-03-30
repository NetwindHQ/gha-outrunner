# Runner Image Requirements

Every runner environment (container or VM) must have the GitHub Actions runner agent installed. The specific requirements depend on the provisioner backend.

## Common Requirements

All backends need:

- **GitHub Actions runner:** The [actions/runner](https://github.com/actions/runner) agent, extracted and ready to run.
- **Runner entrypoint:** The `run.sh` (Linux/macOS) or `run.cmd` (Windows) script must be executable at a known path.
- **Network access:** The runner needs outbound HTTPS to `github.com` and `*.actions.githubusercontent.com`.

The runner registers itself using a JIT (just-in-time) configuration token passed via `--jitconfig`. No pre-registration or PAT inside the image is needed.

## Docker

The container must have the runner at its working directory so that the `runner_cmd` (default: `./run.sh`) can be executed with `--jitconfig <config>`.

The simplest approach is to base your image on the official runner image:

```dockerfile
FROM ghcr.io/actions/actions-runner:latest

USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl git jq \
    && rm -rf /var/lib/apt/lists/*
USER runner
```

**Key points:**

- The official image includes the runner at `/home/runner` with `WORKDIR /home/runner`.
- Run as the `runner` user (UID 1001), not root.
- Pre-install any tools your workflows need. There's no caching between jobs since each container is ephemeral.
- The container is created with `AutoRemove: true`, so it's deleted after the runner process exits.

See the included `runner/Dockerfile` for a working example.

## Libvirt (KVM/QEMU)

The VM image must be a qcow2 disk with:

1. **Operating system** booting under UEFI (q35 machine type with EFI firmware).
2. **QEMU Guest Agent** installed and running at boot. This is how outrunner communicates with the VM (no SSH or WinRM needed).
3. **GitHub Actions runner** installed at the path specified by `runner_cmd` in the config.
4. **Virtio drivers** for disk (virtio-scsi) and network (virtio-net).

### Linux VMs

```bash
# Install guest agent (Debian/Ubuntu)
apt-get install -y qemu-guest-agent
systemctl enable qemu-guest-agent

# Install runner
mkdir -p /actions-runner && cd /actions-runner
curl -sL https://github.com/actions/runner/releases/download/v2.322.0/actions-runner-linux-x64-2.322.0.tar.gz | tar xz
```

### Windows VMs

- Install the [QEMU Guest Agent MSI](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/latest-virtio/) (included in virtio-win ISO).
- Install virtio storage and network drivers from the same ISO.
- The guest agent service (`QEMU Guest Agent`) must start automatically.
- Install the runner at `C:\actions-runner\`.
- Set `runner_cmd: 'C:\actions-runner\run.cmd'` in config.

Pre-built images with guest agent and virtio drivers:
- [rgl/windows-vagrant](https://github.com/rgl/windows-vagrant) boxes include everything needed.

## Tart

The VM image must have:

1. **Tart Guest Agent** running at boot. Pre-installed in all [Cirrus Labs](https://github.com/cirruslabs) images.
2. **GitHub Actions runner** installed at the path specified by `runner_cmd`.

### macOS VMs

Cirrus Labs provides ready-to-use macOS images with the guest agent pre-installed:

```bash
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest my-runner
```

To add the Actions runner to a base image:

```bash
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest runner-base
tart run runner-base
# Inside the VM:
mkdir -p ~/actions-runner && cd ~/actions-runner
curl -sL https://github.com/actions/runner/releases/download/v2.322.0/actions-runner-osx-arm64-2.322.0.tar.gz | tar xz
# Shut down the VM
```

### Linux VMs (ARM64)

Cirrus Labs also provides ARM64 Linux images:

```bash
tart clone ghcr.io/cirruslabs/ubuntu-runner-arm64:latest my-linux-runner
```

These come with both the guest agent and the Actions runner pre-installed.

## Summary

| Requirement | Docker | Libvirt | Tart |
|-------------|--------|---------|------|
| Actions runner | Yes | Yes | Yes |
| Guest agent | No | QEMU Guest Agent | Tart Guest Agent |
| Network drivers | N/A | Virtio | N/A |
| Disk drivers | N/A | Virtio-SCSI | N/A |
| Boot firmware | N/A | UEFI | N/A |
