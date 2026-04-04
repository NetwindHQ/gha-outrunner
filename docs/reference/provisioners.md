# Provisioner Reference

Each provisioner implements the same lifecycle: **create environment → start runner → destroy after job**. They differ in how they create and destroy environments.

## Docker

**Host OS:** Linux, macOS (via Colima/Docker Desktop)

Creates a container per job. Fastest startup time.

### Lifecycle

1. **Image check:** Inspects the image locally. Pulls from registry only if not found.
2. **Create container:** Named using the scale set name as prefix (e.g., `linux-a1b2c3d4`). Labels: `outrunner=true`, `outrunner.name=<name>`. Configured with `AutoRemove: true`.
3. **Start container:** Runs the configured `runner_cmd` (default: `./run.sh`) with `--jitconfig <config>`.
4. **Stop:** Sends stop signal. Container may already be gone due to AutoRemove.

### Docker Host Detection

If `DOCKER_HOST` is not set, outrunner runs `docker context inspect` to find the active context's endpoint. This works with:

- Docker Desktop (macOS, Linux)
- Colima (`unix:///.../.colima/default/docker.sock`)
- Podman with Docker-compatible socket
- Any custom Docker context

The detected socket path is verified to exist before use.

### Bind Mounts

The `mounts` config field attaches host directories into each container as bind mounts. This is useful for a persistent build cache that survives across ephemeral containers without being downloaded each run.

```yaml
docker:
  image: outrunner-runner:latest
  mounts:
    - source: /var/cache/vcpkg
      target: /opt/vcpkg-cache
```

### Requirements

- Docker Engine or compatible runtime
- Docker CLI (for context detection)
- Runner image with GitHub Actions runner installed (see [Image Requirements](image-requirements.md))

### Cleanup

Containers are created with `AutoRemove: true`, so they clean up after the runner process exits. If outrunner is killed, orphaned containers with the `outrunner` label may remain. Remove them with `docker ps -a --filter label=outrunner`.

---

## Libvirt

**Host OS:** Linux (KVM/QEMU)

Creates a KVM virtual machine per job using copy-on-write disk overlays. Uses the QEMU Guest Agent for command execution, no SSH or WinRM needed.

### Lifecycle

1. **Create overlay:** Runs `qemu-img create` to make a CoW qcow2 overlay backed by the golden image. The base image is never modified.
2. **Create domain:** Defines and starts a transient KVM domain with the overlay disk, virtio networking, and a virtio-serial channel for the guest agent.
3. **Wait for guest agent:** Polls `guest-ping` via the QEMU Guest Agent channel until the VM responds (timeout: 3 minutes).
4. **Start runner:** Executes the runner command inside the VM via `guest-exec`.
5. **Stop:** Destroys the domain (force power-off) and deletes the overlay file.

### Domain Configuration

VMs are created with:

- **CPU:** Host passthrough (`host-passthrough` mode)
- **Machine:** q35 with EFI firmware
- **Disk:** SCSI via virtio-scsi controller, qcow2 with writeback cache
- **Network:** virtio NIC on the configured libvirt network (default: `default`)
- **Guest agent:** virtio-serial channel (`org.qemu.guest_agent.0`)
- **Console:** Serial + console (for debugging via `virsh console`)

### virtiofs Mount

The `mount` config field shares a host directory into the VM via virtiofs. This is the recommended approach for a persistent build cache — the host directory is accessible at near-native speed without any network round-trip.

```yaml
libvirt:
  path: /images/windows-builder.qcow2
  mount:
    source: /var/cache/vcpkg
```

The virtiofs tag is derived from the source directory basename. On Windows guests, `VirtioFsSvc` mounts it automatically as a drive letter once the VM boots. Requires:

- `virtiofsd` installed on the host (`apt install virtiofsd`)
- WinFsp installed in the guest image (for Windows guests)
- `memfd`-backed shared memory (added to the domain XML automatically when `mount` is set)

### Connection

Connects to libvirtd via the Unix socket at `/var/run/libvirt/libvirt-sock`. The user running outrunner must have access to this socket (typically via the `libvirt` group).

### Overlay Directory

Ephemeral overlay files are created in the system temp directory by default. Configure `LibvirtConfig.OverlayDir` to use a different location (e.g., a fast SSD).

### Requirements

- libvirtd running with KVM support
- `qemu-img` command available
- QEMU Guest Agent installed and enabled in the base image
- Base qcow2 image with GitHub Actions runner installed

### Cleanup

On startup, outrunner lists all domains and destroys any whose name starts with the runner's scale set name prefix. Corresponding overlay files are also deleted.

---

## Tart

**Host OS:** macOS (Apple Silicon)

Creates a Tart virtual machine per job by cloning from a base image. Uses `tart exec` (Tart's guest agent) for command execution.

### Lifecycle

1. **Clone:** `tart clone <image> <name>`. Creates an independent copy of the base image.
2. **Set resources:** `tart set <name> --cpu <n> --memory <n>`. Configures vCPU and memory.
3. **Run:** `tart run --no-graphics <name>`. Starts the VM in a background goroutine (blocking command).
4. **Wait for guest agent:** Polls `tart exec <name> echo ok` until it succeeds (timeout: 3 minutes).
5. **Start runner:** `tart exec <name> <runner_cmd> --jitconfig <config>`. Runs in a background goroutine.
6. **Stop:** Cancels the run context, runs `tart stop <name>`, then `tart delete <name>`.

### Image Sources

The `image` field can be:

- **OCI registry reference:** `ghcr.io/cirruslabs/macos-sequoia-base:latest` (pulled on first use, cached locally by Tart)
- **Local VM name:** A VM already present in `~/.tart/vms/`

### Requirements

- [Tart](https://github.com/cirruslabs/tart) installed (`brew install cirruslabs/cli/tart`)
- Apple Silicon Mac (M1 or later)
- Base VM image with:
  - [Tart Guest Agent](https://github.com/cirruslabs/tart-guest-agent) installed and running
  - GitHub Actions runner installed
- Sufficient disk space for VM clones

### Shared Directories

The `mounts` config field passes `--dir` flags to `tart run`, sharing host directories into the VM via virtiofs. This is the recommended approach for a persistent build cache.

```yaml
tart:
  image: ghcr.io/cirruslabs/macos-sequoia-base:latest
  mounts:
    - name: vcpkg
      source: /var/cache/vcpkg
```

How the share appears inside the guest depends on the guest OS:

- **macOS guests:** Automatically mounted at `/Volumes/My Shared Files/<name>`. No setup needed.
- **Linux guests:** Mount manually: `mount -t virtiofs com.apple.virtio-fs.automount /mnt/shared`. The `<name>` becomes a subdirectory under the mount point.

### Cleanup

On startup, outrunner runs `tart list --quiet` and deletes any VMs whose name starts with the runner's scale set name prefix (via `tart stop` + `tart delete`).
