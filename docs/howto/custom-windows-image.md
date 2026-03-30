# How to Build a Custom Windows VM Image

Build a Windows qcow2 golden image with your tools pre-installed for use with the libvirt provisioner.

## Prerequisites

- Linux server with KVM and libvirt
- Windows ISO (download from [microsoft.com](https://www.microsoft.com/en-us/software-download/))
- [Virtio-win ISO](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/)

## 1. Create and Install

```bash
qemu-img create -f qcow2 /var/lib/libvirt/images/ci-runners/windows-builder.qcow2 60G

virt-install \
  --name win-setup \
  --ram 8192 \
  --vcpus 4 \
  --os-variant win11 \
  --disk path=/var/lib/libvirt/images/ci-runners/windows-builder.qcow2,bus=scsi \
  --cdrom /path/to/windows.iso \
  --disk path=/path/to/virtio-win.iso,device=cdrom \
  --network network=default,model=virtio \
  --controller type=scsi,model=virtio-scsi \
  --graphics vnc,listen=0.0.0.0 \
  --boot firmware=efi
```

Connect via VNC to complete the installation. During setup, load the virtio-scsi driver from the virtio-win ISO so Windows can see the disk.

## 2. Install Drivers and Guest Agent

After Windows is installed, from the virtio-win ISO:

1. Run `virtio-win-gt-x64.msi` to install all virtio drivers.
2. Run `guest-agent\qemu-ga-x86_64.msi` to install the QEMU Guest Agent.
3. Verify the "QEMU Guest Agent" Windows service is running and set to **Automatic**.

## 3. Install the GitHub Actions Runner

In PowerShell:

```powershell
mkdir C:\actions-runner; cd C:\actions-runner
Invoke-WebRequest -Uri https://github.com/actions/runner/releases/download/v2.322.0/actions-runner-win-x64-2.322.0.zip -OutFile runner.zip
Expand-Archive runner.zip -DestinationPath .
Remove-Item runner.zip
```

Verify: `C:\actions-runner\run.cmd` should exist.

## 4. Install Your Tools

Install whatever your CI workflows need. Common examples:

```powershell
# Chocolatey
Set-ExecutionPolicy Bypass -Scope Process -Force
[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

# Build tools
choco install -y git cmake visualstudio2022buildtools
choco install -y visualstudio2022-workload-vctools

# Language runtimes
choco install -y python3 nodejs-lts golang
```

## 5. Clean Up and Sysprep (Optional)

For a clean image:

```powershell
# Clear temp files
Remove-Item -Recurse -Force $env:TEMP\*
Remove-Item -Recurse -Force C:\Windows\Temp\*

# Clear Windows Update cache
Stop-Service wuauserv
Remove-Item -Recurse -Force C:\Windows\SoftwareDistribution\Download\*
Start-Service wuauserv

# Defrag and compact (reduces qcow2 size)
Optimize-Volume -DriveLetter C -Defrag
```

Sysprep is optional. outrunner uses copy-on-write overlays, so the original image state doesn't affect job isolation.

## 6. Shut Down and Finalize

```powershell
shutdown /s /t 0
```

From the host:

```bash
virsh undefine win-setup

# Optionally compress the image
qemu-img convert -O qcow2 -c \
  /var/lib/libvirt/images/ci-runners/windows-builder.qcow2 \
  /var/lib/libvirt/images/ci-runners/windows-builder-compressed.qcow2
mv /var/lib/libvirt/images/ci-runners/windows-builder-compressed.qcow2 \
   /var/lib/libvirt/images/ci-runners/windows-builder.qcow2
```

## 7. Use in Config

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

## Tips

- **Disk size:** 60 GB is usually enough for Windows + tools. Adjust based on your needs. The qcow2 format only uses disk space for written data.
- **Disable Windows Update** in the golden image to avoid update noise during CI jobs.
- **Test the guest agent** before finalizing: `virsh qemu-agent-command win-setup '{"execute":"guest-ping"}'`
- **Keep a backup** of the golden image before making changes.
