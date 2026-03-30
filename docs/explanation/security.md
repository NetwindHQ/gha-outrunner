# Security Model

## Authentication

### GitHub PAT

outrunner requires a fine-grained Personal Access Token with **Administration read/write** permission. This token is used to:

1. Register and delete scale sets on the repository or organization.
2. Generate JIT runner configuration tokens for each job.

The PAT is held in outrunner's process memory. It is never written to disk, passed to containers, or injected into VMs. Protect it as you would any credential with write access to your repository settings.

### JIT Configuration Tokens

Each runner receives a JIT (just-in-time) configuration token via `--jitconfig`. This token:

- Is generated per-runner at provisioning time.
- Allows a single runner registration.
- Is passed as a command-line argument to `run.sh` inside the container/VM.
- Expires after use. It cannot be reused to register another runner.

The JIT token is visible inside the runner environment (in the process arguments). This is acceptable because the environment is ephemeral and single-use.

## Isolation

### Docker

Containers provide process and filesystem isolation via Linux namespaces and cgroups. This is the same level of isolation as any Docker container:

- Containers run as a non-root user (`runner`, UID 1001 in the official image).
- Each container has its own filesystem, network namespace, and PID namespace.
- Containers are created with `AutoRemove: true`, deleted when the runner process exits.

Docker isolation is **not** equivalent to VM isolation. A malicious workflow could potentially escape the container if the Docker daemon or kernel has vulnerabilities. For untrusted workloads, use the libvirt or Tart provisioners.

### Libvirt (KVM)

KVM virtual machines provide hardware-level isolation:

- Each VM has its own kernel, memory space, and virtual hardware.
- Disk overlays are copy-on-write. The base image is never modified.
- VMs are transient (created via `DomainCreateXML`, not defined). They leave no persistent configuration.
- On job completion, the VM is force-destroyed and the overlay is deleted.

This is the strongest isolation model outrunner offers and is appropriate for untrusted workloads.

### Tart

Tart VMs use Apple's Virtualization.framework, which provides hardware-level isolation:

- Each VM is a clone of the base image, fully independent.
- On job completion, the clone is stopped and deleted.

Isolation guarantees are similar to libvirt, backed by Apple Silicon's hardware virtualization support.

## Network

### Outbound

Runner environments need outbound HTTPS access to:

- `github.com` (API calls and repository access)
- `*.actions.githubusercontent.com` (runner service, logs, and artifacts)
- Any registries or services your workflows use.

### Inbound

No inbound connections are needed. The scaleset API uses outbound long-polling, and guest agents communicate over host-local channels (not the network).

### Between Runners

By default, Docker containers share the host's Docker network and can reach each other. Libvirt VMs share the libvirt network. If you need network isolation between concurrent runners, configure separate Docker networks or libvirt networks.

## Cleanup and Orphans

If outrunner is killed without graceful shutdown:

- **Docker:** Containers with `AutoRemove: true` will self-destruct when their runner process exits naturally (job timeout or GitHub cancellation). Otherwise, orphaned containers persist until manually removed.
- **Libvirt:** VMs and overlay files persist. On next startup, outrunner cleans up domains and overlays matching each runner's scale set name prefix.
- **Tart:** VM clones persist. On next startup, outrunner cleans up VMs matching each runner's scale set name prefix.

The scale set registration also persists on GitHub's side. outrunner reuses it on next startup via get-or-create logic.

## Recommendations

1. **Use a dedicated PAT** with minimal scope: one repository, Administration read/write only.
2. **Run outrunner as a dedicated system user**, not root. For Docker, add this user to the `docker` group. For libvirt, add to the `libvirt` group.
3. **Use VM backends for untrusted workloads** (public repos, third-party PRs). Docker isolation is sufficient for trusted internal workloads.
4. **Rotate the PAT periodically** and store it in a secrets manager. outrunner reads it once at startup, so rotation requires a restart.
5. **Don't bake secrets into runner images.** Use GitHub Actions secrets, OIDC, or environment-specific credential providers instead.
