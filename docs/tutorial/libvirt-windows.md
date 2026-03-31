# Windows VMs via libvirt/KVM

This guide assumes outrunner is already installed and running. If not, start with one of the [setup guides](../setup/).

## Prerequisites

- Linux server with KVM support (`grep -c vmx /proc/cpuinfo` or `grep -c svm /proc/cpuinfo` should return > 0)
- A Windows ISO or pre-built qcow2 image

## 1. Install libvirt and QEMU

On Ubuntu/Debian:

```bash
sudo apt-get install -y qemu-kvm libvirt-daemon-system virtinst qemu-utils
sudo systemctl enable --now libvirtd
```

On Fedora:

```bash
sudo dnf install -y qemu-kvm libvirt virt-install qemu-img
sudo systemctl enable --now libvirtd
```

Add the outrunner user to the libvirt group:

```bash
sudo usermod -aG libvirt outrunner
```

## 2. Prepare a Windows base image

You need a qcow2 image with Windows, virtio drivers, the QEMU Guest Agent, and the GitHub Actions runner installed.

### Option A: Use a pre-built image

The [rgl/windows-vagrant](https://github.com/rgl/windows-vagrant) project provides Windows images with virtio drivers and guest agent pre-installed.

### Option B: Build from scratch

Download the [virtio-win ISO](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/).

Create a blank disk and install Windows:

```bash
sudo mkdir -p /var/lib/libvirt/images/ci-runners

qemu-img create -f qcow2 /var/lib/libvirt/images/ci-runners/windows-builder.qcow2 60G

virt-install \
  --name windows-setup \
  --ram 8192 \
  --vcpus 4 \
  --os-variant win11 \
  --disk path=/var/lib/libvirt/images/ci-runners/windows-builder.qcow2,bus=scsi \
  --cdrom /path/to/windows.iso \
  --disk path=/path/to/virtio-win.iso,device=cdrom \
  --network network=default,model=virtio \
  --controller type=scsi,model=virtio-scsi \
  --graphics vnc,listen=0.0.0.0
```

During Windows setup:

1. Load the virtio-scsi driver from the ISO (`vioscsi\w11\amd64`) to see the disk.
2. After installation, install all virtio drivers from the ISO.
3. Install the QEMU Guest Agent (`guest-agent\qemu-ga-x86_64.msi`). Verify the service is running and set to Automatic.

Then install the runner:

```powershell
mkdir C:\actions-runner; cd C:\actions-runner
Invoke-WebRequest -Uri https://github.com/actions/runner/releases/download/v2.322.0/actions-runner-win-x64-2.322.0.zip -OutFile runner.zip
Expand-Archive runner.zip -DestinationPath .
Remove-Item runner.zip
```

Shut down and clean up:

```bash
virsh shutdown windows-setup
virsh undefine windows-setup
```

The qcow2 file is now your golden image.

See [Build a custom Windows VM image](../howto/custom-windows-image.md) for more details.

## 3. Configure outrunner

Update your config to use the libvirt backend:

```yaml
runners:
  windows:
    labels: [self-hosted, windows]
    max_runners: 1
    libvirt:
      path: /var/lib/libvirt/images/ci-runners/windows-builder.qcow2
      runner_cmd: 'C:\actions-runner\run.cmd'
      cpus: 4
      memory: 8192
```

Restart outrunner to pick up the new config:

```bash
sudo systemctl restart outrunner
```

## 4. Test it

Create `.github/workflows/test-windows.yml` in your repository:

```yaml
name: Test Windows

on:
  workflow_dispatch:

jobs:
  hello:
    runs-on: [self-hosted, windows]
    steps:
      - run: echo "Hello from a Windows VM!"
      - run: systeminfo | findstr /B /C:"OS Name" /C:"OS Version"
```

Push and trigger from GitHub -> Actions.

The guest agent wait may take 30-90 seconds while Windows boots.

## How it works

1. outrunner creates a copy-on-write qcow2 overlay backed by the golden image.
2. Boots a transient KVM domain with the overlay disk.
3. Waits for the QEMU Guest Agent to respond.
4. Executes the runner via `guest-exec`.
5. After the job, destroys the domain and deletes the overlay.

The golden image is never modified.

## Next steps

- [Build a custom Windows VM image](../howto/custom-windows-image.md)
- [Run multiple backends together](../howto/mixed-backends.md)
- [Configuration reference](../reference/configuration.md)
