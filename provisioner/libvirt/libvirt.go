package libvirt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	outrunner "github.com/psubocz/gha-outrunner"

	golibvirt "github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
)

// Config configures the libvirt provisioner.
type Config struct {
	// OverlayDir is where ephemeral qcow2 overlay files are created.
	OverlayDir string

	// Network is the libvirt network name. Defaults to "default".
	Network string
}

// Provisioner creates ephemeral KVM/QEMU VMs as GitHub Actions runners.
// Uses QEMU Guest Agent for command execution (no SSH/WinRM needed).
type Provisioner struct {
	logger     *slog.Logger
	conn       *golibvirt.Libvirt
	overlayDir string
	network    string

	mu       sync.Mutex
	overlays map[string]string // runner name -> overlay path
}

func New(logger *slog.Logger, cfg Config) (*Provisioner, error) {
	if cfg.Network == "" {
		cfg.Network = "default"
	}

	// Connect to libvirtd via Unix socket
	sockPath := "/var/run/libvirt/libvirt-sock"
	l := golibvirt.NewWithDialer(dialers.NewLocal(dialers.WithSocket(sockPath)))
	if err := l.Connect(); err != nil {
		return nil, fmt.Errorf("libvirt connect to %s: %w", sockPath, err)
	}

	return &Provisioner{
		logger:     logger,
		conn:       l,
		overlayDir: cfg.OverlayDir,
		network:    cfg.Network,
		overlays:   make(map[string]string),
	}, nil
}

func (p *Provisioner) Start(ctx context.Context, req *outrunner.RunnerRequest) error {
	if req.Runner == nil || req.Runner.Libvirt == nil {
		return fmt.Errorf("no libvirt config for runner %s", req.Name)
	}
	img := req.Runner.Libvirt

	// Use configured overlay dir, or system temp
	overlayDir := p.overlayDir
	if overlayDir == "" {
		overlayDir = os.TempDir()
	}
	overlayPath := filepath.Join(overlayDir, req.Name+".qcow2")

	// 1. Create qcow2 overlay
	p.logger.Debug("Creating overlay", slog.String("base", img.Path), slog.String("overlay", overlayPath))
	out, err := exec.CommandContext(ctx, "qemu-img", "create",
		"-f", "qcow2", "-F", "qcow2", "-b", img.Path, overlayPath,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("create overlay: %w: %s", err, out)
	}

	// 2. Generate domain XML
	domainXML, err := renderDomainXML(req.Name, overlayPath, img, p.network)
	if err != nil {
		_ = os.Remove(overlayPath)
		return fmt.Errorf("render domain XML: %w", err)
	}

	// 3. Create transient domain (auto-undefines on destroy)
	p.logger.Debug("Creating domain", slog.String("name", req.Name))
	dom, err := p.conn.DomainCreateXML(domainXML, 0)
	if err != nil {
		_ = os.Remove(overlayPath)
		return fmt.Errorf("create domain: %w", err)
	}

	// 4. Wait for guest agent
	p.logger.Debug("Waiting for guest agent", slog.String("name", req.Name))
	if err := p.waitForAgent(ctx, dom, 3*time.Minute); err != nil {
		p.destroyDomain(req.Name)
		_ = os.Remove(overlayPath)
		return fmt.Errorf("guest agent not ready: %w", err)
	}

	// 5. Execute runner via guest-exec
	runnerCmd := img.RunnerCmd
	if runnerCmd == "" {
		runnerCmd = "/actions-runner/run.sh"
	}

	p.logger.Info("Starting runner in VM",
		slog.String("name", req.Name),
		slog.String("cmd", runnerCmd),
	)

	pid, err := p.guestExec(ctx, dom, runnerCmd, []string{"--jitconfig", req.JITConfig})
	if err != nil {
		p.destroyDomain(req.Name)
		_ = os.Remove(overlayPath)
		return fmt.Errorf("guest-exec: %w", err)
	}

	p.logger.Info("Runner started in VM",
		slog.String("name", req.Name),
		slog.Int("pid", pid),
	)

	p.mu.Lock()
	p.overlays[req.Name] = overlayPath
	p.mu.Unlock()

	return nil
}

func (p *Provisioner) Stop(ctx context.Context, name string) error {
	p.logger.Debug("Stopping VM", slog.String("name", name))
	p.destroyDomain(name)

	p.mu.Lock()
	overlayPath, ok := p.overlays[name]
	delete(p.overlays, name)
	p.mu.Unlock()

	if ok {
		_ = os.Remove(overlayPath)
	}

	return nil
}

func (p *Provisioner) Close() error {
	return p.conn.Disconnect()
}

// Cleanup destroys any leftover VMs and overlay files from previous runs.
func (p *Provisioner) Cleanup(prefix string) {
	domains, _, err := p.conn.ConnectListAllDomains(0, 0)
	if err != nil {
		p.logger.Error("Failed to list domains for cleanup", slog.String("error", err.Error()))
		return
	}

	for _, dom := range domains {
		name := dom.Name
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		p.logger.Info("Cleaning up orphaned VM", slog.String("name", name))
		_ = p.conn.DomainDestroy(dom)

		overlayPath := filepath.Join(p.overlayDir, name+".qcow2")
		_ = os.Remove(overlayPath)
	}
}

// destroyDomain force-stops a domain. Idempotent — ignores "not found" errors.
func (p *Provisioner) destroyDomain(name string) {
	dom, err := p.conn.DomainLookupByName(name)
	if err != nil {
		return // already gone
	}
	if err := p.conn.DomainDestroy(dom); err != nil {
		p.logger.Debug("Domain destroy error (may already be gone)",
			slog.String("name", name),
			slog.String("error", err.Error()),
		)
	}
}

// waitForAgent polls guest-ping until the QEMU guest agent responds.
func (p *Provisioner) waitForAgent(ctx context.Context, dom golibvirt.Domain, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ping := `{"execute":"guest-ping"}`

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout after %v", timeout)
		}

		_, err := p.conn.QEMUDomainAgentCommand(dom, ping, 5, 0)
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// guestExecResult is the response from guest-exec.
type guestExecResult struct {
	Return struct {
		PID int `json:"pid"`
	} `json:"return"`
}

// guestExec runs a command inside the VM via the QEMU guest agent.
// Returns the PID of the started process.
func (p *Provisioner) guestExec(ctx context.Context, dom golibvirt.Domain, path string, args []string) (int, error) {
	cmd := map[string]any{
		"execute": "guest-exec",
		"arguments": map[string]any{
			"path": path,
			"arg":  args,
		},
	}

	cmdJSON, err := json.Marshal(cmd)
	if err != nil {
		return 0, fmt.Errorf("marshal command: %w", err)
	}

	result, err := p.conn.QEMUDomainAgentCommand(dom, string(cmdJSON), 30, 0)
	if err != nil {
		return 0, fmt.Errorf("agent command: %w", err)
	}

	// OptString is []string — get the first element
	if len(result) == 0 {
		return 0, fmt.Errorf("empty response from guest agent")
	}

	var execResult guestExecResult
	if err := json.Unmarshal([]byte(result[0]), &execResult); err != nil {
		return 0, fmt.Errorf("parse response: %w (raw: %s)", err, result[0])
	}

	return execResult.Return.PID, nil
}

// domainXMLParams are the template parameters for domain XML generation.
type domainXMLParams struct {
	Name        string
	OverlayPath string
	CPUs        int
	MemoryMB    int
	Network     string
}

var domainTmpl = template.Must(template.New("domain").Parse(`<domain type='kvm'>
  <name>{{.Name}}</name>
  <memory unit='MiB'>{{.MemoryMB}}</memory>
  <vcpu>{{.CPUs}}</vcpu>
  <os firmware='efi'>
    <type arch='x86_64' machine='q35'>hvm</type>
    <boot dev='hd'/>
  </os>
  <features>
    <acpi/>
    <apic/>
  </features>
  <cpu mode='host-passthrough'/>
  <devices>
    <controller type='scsi' model='virtio-scsi'/>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2' cache='writeback'/>
      <source file='{{.OverlayPath}}'/>
      <target dev='sda' bus='scsi'/>
    </disk>
    <interface type='network'>
      <source network='{{.Network}}'/>
      <model type='virtio'/>
    </interface>
    <channel type='unix'>
      <target type='virtio' name='org.qemu.guest_agent.0'/>
    </channel>
    <serial type='pty'/>
    <console type='pty'/>
  </devices>
</domain>`))

func renderDomainXML(name, overlayPath string, img *outrunner.LibvirtImage, network string) (string, error) {
	params := domainXMLParams{
		Name:        name,
		OverlayPath: overlayPath,
		CPUs:        img.CPUs,
		MemoryMB:    img.MemoryMB,
		Network:     network,
	}

	var buf strings.Builder
	if err := domainTmpl.Execute(&buf, params); err != nil {
		return "", err
	}
	return buf.String(), nil
}
