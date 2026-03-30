# Tutorial: Windows VM Runner on Linux

In this tutorial we will set up outrunner on a Linux server to run GitHub Actions jobs in ephemeral Windows VMs via libvirt/KVM. By the end, you'll trigger a workflow that runs inside a Windows VM, created on demand and destroyed after.

This is the most complex setup because it requires a Windows base image with the right drivers and guest agent. The payoff is full Windows CI with hardware-level isolation and no SSH or WinRM configuration.

## Prerequisites

- A Linux server with KVM support (`grep -c vmx /proc/cpuinfo` or `grep -c svm /proc/cpuinfo` should return > 0)
- Root or sudo access
- A GitHub repository you own
- A Windows ISO or pre-built qcow2 image

## 1. Install libvirt and QEMU

On Ubuntu/Debian:

```bash
sudo apt-get update
sudo apt-get install -y qemu-kvm libvirt-daemon-system virtinst qemu-utils
sudo systemctl enable --now libvirtd
```

On Fedora:

```bash
sudo dnf install -y qemu-kvm libvirt virt-install qemu-img
sudo systemctl enable --now libvirtd
```

Add your user to the libvirt group:

```bash
sudo usermod -aG libvirt $USER
newgrp libvirt
```

Verify KVM works:

```bash
virsh list --all
```

## 2. Prepare a Windows Base Image

You need a qcow2 image with:
- Windows installed
- Virtio drivers (disk and network)
- QEMU Guest Agent
- GitHub Actions runner

### Option A: Use a Pre-built Image

The [rgl/windows-vagrant](https://github.com/rgl/windows-vagrant) project provides Windows images with virtio drivers and guest agent pre-installed. Convert from Vagrant box to qcow2 if needed.

### Option B: Build From Scratch

Download the [virtio-win ISO](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/). It contains drivers and the guest agent.

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
1. Load the virtio-scsi driver from the virtio-win ISO (`vioscsi\w11\amd64`) to see the disk.
2. After installation, install all virtio drivers from the ISO.
3. Install the QEMU Guest Agent from the ISO (`guest-agent\qemu-ga-x86_64.msi`).
4. Verify the "QEMU Guest Agent" service is running and set to Automatic.

Then install the runner:

```powershell
mkdir C:\actions-runner; cd C:\actions-runner
Invoke-WebRequest -Uri https://github.com/actions/runner/releases/download/v2.322.0/actions-runner-win-x64-2.322.0.zip -OutFile runner.zip
Expand-Archive runner.zip -DestinationPath .
Remove-Item runner.zip
```

Shut down the VM:

```bash
virsh shutdown windows-setup
virsh undefine windows-setup
```

The qcow2 file is now your golden image.

## 3. Build outrunner

```bash
git clone https://github.com/psubocz/gha-outrunner.git
cd gha-outrunner
go build -o outrunner ./cmd/outrunner
```

## 4. Create a GitHub PAT

Go to [github.com/settings/tokens?type=beta](https://github.com/settings/tokens?type=beta) and create a fine-grained token:

- **Token name:** outrunner
- **Resource owner:** Your user or organization
- **Repository access:** Select the repository you want to use
- **Permissions:** Administration → Read and write

## 5. Write a Configuration File

Create `outrunner.yml`:

```yaml
runners:
  windows:
    labels: [self-hosted, windows]
    libvirt:
      path: /var/lib/libvirt/images/ci-runners/windows-builder.qcow2
      runner_cmd: 'C:\actions-runner\run.cmd'
      cpus: 4
      memory: 8192
```

## 6. Start outrunner

```bash
./outrunner \
  --url https://github.com/YOUR_USER/YOUR_REPO \
  --token ghp_YOUR_TOKEN \
  --config outrunner.yml \
  --max-runners 1
```

Note: We set `--max-runners 1` because Windows VMs are resource-heavy.

You should see:

```
level=INFO msg="Loaded config" runners=1
level=INFO msg="Scale set created" id=7
level=INFO msg="Libvirt provisioner initialized"
level=INFO msg="Listening for jobs" scaleSet=windows maxRunners=1
```

## 7. Create a Test Workflow

In your repository, create `.github/workflows/test-outrunner.yml`:

```yaml
name: Test Outrunner

on:
  workflow_dispatch:

jobs:
  hello:
    runs-on: [self-hosted, windows]
    steps:
      - run: echo "Hello from a Windows VM!"
      - run: systeminfo | findstr /B /C:"OS Name" /C:"OS Version"
      - run: wmic cpu get name
```

Push and trigger from GitHub → Actions.

## 8. Watch It Work

In the outrunner terminal:

```
level=DEBUG msg="Creating overlay" libvirt.base=/var/lib/libvirt/images/ci-runners/windows-builder.qcow2
level=DEBUG msg="Creating domain" libvirt.name=windows-a1b2c3d4
level=DEBUG msg="Waiting for guest agent" libvirt.name=windows-a1b2c3d4
level=INFO  msg="Starting runner in VM" libvirt.name=windows-a1b2c3d4
level=INFO  msg="Runner started in VM" libvirt.name=windows-a1b2c3d4 libvirt.pid=1234
level=INFO  msg="Job completed" scaler.runnerName=windows-a1b2c3d4 scaler.result=succeeded
level=DEBUG msg="Stopping VM" libvirt.name=windows-a1b2c3d4
```

The guest agent wait may take 30-90 seconds while Windows boots. After that, the runner starts and picks up the job.

## 9. Clean Up

Press Ctrl+C. outrunner destroys the VM and deletes the overlay file. The golden image is untouched.

## What Happened

1. outrunner created a copy-on-write qcow2 overlay backed by the golden image.
2. It booted a transient KVM domain with the overlay disk.
3. It waited for the QEMU Guest Agent inside Windows to respond to pings.
4. It executed `C:\actions-runner\run.cmd --jitconfig <config>` via the guest agent.
5. The runner registered with GitHub, ran the job, and exited.
6. outrunner destroyed the domain and deleted the overlay.

The golden image was never modified. All writes went to the ephemeral overlay.

## Next Steps

- [How to build a custom Windows VM image](../howto/custom-windows-image.md)
- [How to deploy as a systemd service](../howto/systemd-service.md)
- [How to run multiple backends together](../howto/mixed-backends.md)
