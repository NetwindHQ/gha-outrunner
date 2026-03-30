# How to Update Runner Images Without Downtime

When you need to update tools, patches, or the Actions runner itself in your runner images, you can do it without stopping outrunner.

## Docker

Docker images are resolved by tag at container creation time. To update:

1. Build the new image with the same tag:

```bash
docker build -t outrunner-runner:latest runner/
```

2. That's it. The next job will use the new image. Currently running jobs continue with the old image until they complete.

For more control, use versioned tags:

```bash
docker build -t outrunner-runner:v2 runner/
```

Update the config and restart outrunner:

```yaml
runners:
  linux:
    labels: [self-hosted, linux]
    docker:
      image: outrunner-runner:v2
```

## Libvirt

The base qcow2 image is read-only during normal operation (outrunner uses CoW overlays). To update:

1. Stop outrunner (wait for running jobs to complete, or let them finish).
2. Boot the golden image directly:

```bash
virt-install --name update-vm --import \
  --disk path=/var/lib/libvirt/images/windows-builder.qcow2 \
  --ram 8192 --vcpus 4 --graphics vnc
```

3. Make changes inside the VM (install updates, new tools, etc.).
4. Shut down the VM and undefine it:

```bash
virsh shutdown update-vm
virsh undefine update-vm
```

5. Restart outrunner.

Alternatively, keep the old image and create a new one side-by-side:

```bash
cp windows-builder.qcow2 windows-builder-v2.qcow2
# Boot and update windows-builder-v2.qcow2
```

Update the config to point to the new path, then restart outrunner.

## Tart

### Local Images

1. Boot and update the base image:

```bash
tart run my-runner-base
# Install updates inside the VM
# Shut down
```

2. Running jobs are unaffected (they use clones of the old image state). New jobs will clone from the updated image.

Note: If outrunner is running, avoid modifying the base image while clones are being created. Stop outrunner first, or use the registry approach below.

### Registry Images

For images pulled from a registry (e.g., `ghcr.io/your-org/runner:latest`):

1. Push the updated image to the registry.
2. On the outrunner host, pull the new version:

```bash
tart pull ghcr.io/your-org/runner:latest
```

3. New clones will use the updated image.

## Queue Draining

If you need to stop a runner from accepting new jobs while letting in-flight jobs finish (e.g., before a host reboot or major image update), set `max_runners: 0` in the config and restart outrunner:

```yaml
runners:
  linux:
    labels: [self-hosted, linux]
    max_runners: 0      # stop accepting new jobs
    docker:
      image: runner:latest
```

outrunner will keep its scale set registered but won't provision new runners. Once all running jobs complete, the host is idle and safe to maintain. Set `max_runners` back to the desired value and restart when ready.

## Updating the Actions Runner Agent

GitHub occasionally releases new versions of the runner agent. Since outrunner sets `DisableUpdate: true` on the scale set, the runner won't auto-update. To update:

1. Check the latest version at [github.com/actions/runner/releases](https://github.com/actions/runner/releases).
2. Update the runner in your image (rebuild Docker image, boot and update VM).
3. Deploy the updated image as described above.
