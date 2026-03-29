package outrunner

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TartProvisioner creates ephemeral Tart VMs as GitHub Actions runners.
// Uses `tart exec` via the guest agent for command execution.
type TartProvisioner struct {
	logger  *slog.Logger
	mu      sync.Mutex
	running map[string]context.CancelFunc
}

func NewTartProvisioner(logger *slog.Logger) *TartProvisioner {
	return &TartProvisioner{
		logger:  logger,
		running: make(map[string]context.CancelFunc),
	}
}

func (t *TartProvisioner) Start(ctx context.Context, req *RunnerRequest) error {
	if req.Image == nil || req.Image.Tart == nil {
		return fmt.Errorf("no tart image config for runner %s", req.Name)
	}
	img := req.Image.Tart

	// 1. Clone from base image
	t.logger.Debug("Cloning VM", slog.String("image", img.Image), slog.String("name", req.Name))
	if out, err := exec.CommandContext(ctx, "tart", "clone", img.Image, req.Name).CombinedOutput(); err != nil {
		return fmt.Errorf("tart clone: %w: %s", err, out)
	}

	// 2. Set resources
	if out, err := exec.CommandContext(ctx, "tart", "set", req.Name,
		"--cpu", strconv.Itoa(img.CPUs),
		"--memory", strconv.Itoa(img.MemoryMB),
	).CombinedOutput(); err != nil {
		t.deleteVM(req.Name)
		return fmt.Errorf("tart set: %w: %s", err, out)
	}

	// 3. Run in background (tart run is blocking)
	runCtx, cancel := context.WithCancel(ctx)
	t.mu.Lock()
	t.running[req.Name] = cancel
	t.mu.Unlock()

	go func() {
		t.logger.Debug("Starting VM", slog.String("name", req.Name))
		if out, err := exec.CommandContext(runCtx, "tart", "run", "--no-graphics", req.Name).CombinedOutput(); err != nil {
			if runCtx.Err() == nil {
				// Unexpected exit, not a cancellation
				t.logger.Error("VM exited unexpectedly",
					slog.String("name", req.Name),
					slog.String("error", err.Error()),
					slog.String("output", string(out)),
				)
			}
		}
	}()

	// 4. Wait for guest agent
	t.logger.Debug("Waiting for guest agent", slog.String("name", req.Name))
	if err := t.waitForAgent(ctx, req.Name, 3*time.Minute); err != nil {
		t.Stop(ctx, req.Name)
		return fmt.Errorf("guest agent not ready: %w", err)
	}

	// 5. Start runner via tart exec
	runnerCmd := img.RunnerCmd
	if runnerCmd == "" {
		runnerCmd = "/actions-runner/run.sh"
	}

	t.logger.Info("Starting runner in VM",
		slog.String("name", req.Name),
		slog.String("cmd", runnerCmd),
	)

	go func() {
		out, err := exec.CommandContext(runCtx, "tart", "exec", req.Name,
			runnerCmd, "--jitconfig", req.JITConfig,
		).CombinedOutput()
		if err != nil && runCtx.Err() == nil {
			t.logger.Error("Runner exited with error",
				slog.String("name", req.Name),
				slog.String("error", err.Error()),
				slog.String("output", string(out)),
			)
		}
	}()

	t.logger.Info("Runner started in VM", slog.String("name", req.Name))
	return nil
}

func (t *TartProvisioner) Stop(ctx context.Context, name string) error {
	t.logger.Debug("Stopping VM", slog.String("name", name))

	t.mu.Lock()
	if cancel, ok := t.running[name]; ok {
		cancel()
		delete(t.running, name)
	}
	t.mu.Unlock()

	// Stop the VM (idempotent)
	exec.CommandContext(ctx, "tart", "stop", name).Run()
	// Delete the clone
	t.deleteVM(name)

	return nil
}

func (t *TartProvisioner) Close() error {
	t.mu.Lock()
	for name, cancel := range t.running {
		cancel()
		delete(t.running, name)
	}
	t.mu.Unlock()
	return nil
}

// Cleanup removes orphaned VMs from previous runs.
func (t *TartProvisioner) Cleanup(prefix string) {
	out, err := exec.Command("tart", "list", "--quiet").Output()
	if err != nil {
		t.logger.Error("Failed to list VMs for cleanup", slog.String("error", err.Error()))
		return
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || !strings.HasPrefix(name, prefix) {
			continue
		}
		t.logger.Info("Cleaning up orphaned VM", slog.String("name", name))
		exec.Command("tart", "stop", name).Run()
		exec.Command("tart", "delete", name).Run()
	}
}

func (t *TartProvisioner) deleteVM(name string) {
	if out, err := exec.Command("tart", "delete", name).CombinedOutput(); err != nil {
		t.logger.Debug("VM delete error (may already be gone)",
			slog.String("name", name),
			slog.String("error", err.Error()),
			slog.String("output", string(out)),
		)
	}
}

func (t *TartProvisioner) waitForAgent(ctx context.Context, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout after %v", timeout)
		}

		err := exec.CommandContext(ctx, "tart", "exec", name, "echo", "ok").Run()
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
