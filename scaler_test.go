package outrunner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/actions/scaleset"
)

// mockClient implements ScaleSetClient for testing.
type mockClient struct {
	mu          sync.Mutex
	nextID      int
	removeCount atomic.Int32
	jitErr      error // if set, GenerateJitRunnerConfig returns this error
}

func newMockClient() *mockClient {
	return &mockClient{nextID: 1}
}

func (m *mockClient) GenerateJitRunnerConfig(_ context.Context, setting *scaleset.RunnerScaleSetJitRunnerSetting, _ int) (*scaleset.RunnerScaleSetJitRunnerConfig, error) {
	if m.jitErr != nil {
		return nil, m.jitErr
	}

	m.mu.Lock()
	id := m.nextID
	m.nextID++
	m.mu.Unlock()

	return &scaleset.RunnerScaleSetJitRunnerConfig{
		Runner: &scaleset.RunnerReference{
			ID:   id,
			Name: setting.Name,
		},
		EncodedJITConfig: "test-jit-config",
	}, nil
}

func (m *mockClient) RemoveRunner(_ context.Context, _ int64) error {
	m.removeCount.Add(1)
	return nil
}

// mockProvisioner implements Provisioner for testing.
type mockProvisioner struct {
	mu       sync.Mutex
	started  []string
	stopped  []string
	startErr error
	startCh  chan struct{} // if set, Start blocks until closed
}

func newMockProvisioner() *mockProvisioner {
	return &mockProvisioner{}
}

func (m *mockProvisioner) Start(ctx context.Context, req *RunnerRequest) error {
	if m.startCh != nil {
		select {
		case <-m.startCh:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if m.startErr != nil {
		return m.startErr
	}
	m.mu.Lock()
	m.started = append(m.started, req.Name)
	m.mu.Unlock()
	return nil
}

func (m *mockProvisioner) Stop(_ context.Context, name string) error {
	m.mu.Lock()
	m.stopped = append(m.stopped, name)
	m.mu.Unlock()
	return nil
}

func (m *mockProvisioner) Close() error { return nil }

func (m *mockProvisioner) stoppedNames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.stopped...)
}

func newTestScaler(client ScaleSetClient, prov Provisioner) *Scaler {
	return NewScaler(
		noopLogger(),
		client, 1, 10, "test",
		&RunnerConfig{Docker: &DockerImage{Image: "test:latest"}},
		prov,
	)
}

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNonBlockingProvisioning(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	prov.startCh = make(chan struct{}) // Start blocks until closed
	s := newTestScaler(client, prov)

	// HandleDesiredRunnerCount should return immediately
	done := make(chan struct{})
	go func() {
		count, err := s.HandleDesiredRunnerCount(context.Background(), 1)
		if err != nil {
			t.Errorf("HandleDesiredRunnerCount: %v", err)
		}
		if count != 1 {
			t.Errorf("expected count 1, got %d", count)
		}
		close(done)
	}()

	select {
	case <-done:
		// returned before Start completed
	case <-time.After(2 * time.Second):
		t.Fatal("HandleDesiredRunnerCount blocked on Start()")
	}

	// Start is still blocked, runner is in Provisioning phase
	runners := s.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(runners))
	}
	if runners[0].Phase != RunnerProvisioning {
		t.Errorf("expected Provisioning, got %s", runners[0].Phase)
	}

	// Unblock Start
	close(prov.startCh)
	time.Sleep(50 * time.Millisecond) // let goroutine proceed

	runners = s.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(runners))
	}
	if runners[0].Phase != RunnerIdle {
		t.Errorf("expected Idle, got %s", runners[0].Phase)
	}

	s.Shutdown(context.Background())
}

func TestHappyPath(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	s := newTestScaler(client, prov)

	// Request a runner
	count, err := s.HandleDesiredRunnerCount(context.Background(), 1)
	if err != nil {
		t.Fatalf("HandleDesiredRunnerCount: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	// Wait for provisioning
	time.Sleep(50 * time.Millisecond)

	runners := s.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(runners))
	}
	name := runners[0].Name

	// Job started
	_ = s.HandleJobStarted(context.Background(), &scaleset.JobStarted{
		RunnerName: name,
	})
	runners = s.Runners()
	if runners[0].Phase != RunnerRunning {
		t.Errorf("expected Running, got %s", runners[0].Phase)
	}

	// Job completed
	_ = s.HandleJobCompleted(context.Background(), &scaleset.JobCompleted{
		RunnerName: name,
		Result:     "succeeded",
	})

	// Wait for goroutine to finish cleanup
	time.Sleep(100 * time.Millisecond)

	runners = s.Runners()
	if len(runners) != 0 {
		t.Errorf("expected 0 runners, got %d", len(runners))
	}

	// Verify Stop was called
	stopped := prov.stoppedNames()
	if len(stopped) != 1 || stopped[0] != name {
		t.Errorf("expected Stop(%s), got %v", name, stopped)
	}

	// Verify RemoveRunner was called
	if client.removeCount.Load() != 1 {
		t.Errorf("expected 1 RemoveRunner call, got %d", client.removeCount.Load())
	}

	s.Shutdown(context.Background())
}

func TestProvisioningFailure(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	prov.startErr = context.DeadlineExceeded
	s := newTestScaler(client, prov)

	count, err := s.HandleDesiredRunnerCount(context.Background(), 1)
	if err != nil {
		t.Fatalf("HandleDesiredRunnerCount: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1 (provisioning), got %d", count)
	}

	// Wait for goroutine to detect failure and clean up
	time.Sleep(100 * time.Millisecond)

	runners := s.Runners()
	if len(runners) != 0 {
		t.Errorf("expected 0 runners after failure, got %d", len(runners))
	}

	// RemoveRunner should be called (deregistration)
	if client.removeCount.Load() != 1 {
		t.Errorf("expected 1 RemoveRunner call, got %d", client.removeCount.Load())
	}

	// Stop should NOT be called (Start failed)
	stopped := prov.stoppedNames()
	if len(stopped) != 0 {
		t.Errorf("expected no Stop calls, got %v", stopped)
	}

	s.Shutdown(context.Background())
}

func TestShutdownDuringProvisioning(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	prov.startCh = make(chan struct{}) // Start blocks
	s := newTestScaler(client, prov)

	_, _ = s.HandleDesiredRunnerCount(context.Background(), 1)

	// Runner is provisioning
	time.Sleep(50 * time.Millisecond)
	runners := s.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(runners))
	}

	// Shutdown cancels lifecycle context, which cancels Start
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Shutdown(ctx)

	// Runner should be cleaned up
	runners = s.Runners()
	if len(runners) != 0 {
		t.Errorf("expected 0 runners after shutdown, got %d", len(runners))
	}

	// RemoveRunner should be called
	if client.removeCount.Load() != 1 {
		t.Errorf("expected 1 RemoveRunner call, got %d", client.removeCount.Load())
	}
}

func TestCountAccuracy(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	s := newTestScaler(client, prov)

	// Request 3 runners
	count, err := s.HandleDesiredRunnerCount(context.Background(), 3)
	if err != nil {
		t.Fatalf("HandleDesiredRunnerCount: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count 3, got %d", count)
	}

	// Requesting 3 again should not create more (already at 3)
	count, err = s.HandleDesiredRunnerCount(context.Background(), 3)
	if err != nil {
		t.Fatalf("HandleDesiredRunnerCount: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count 3, got %d", count)
	}

	s.Shutdown(context.Background())
}

func TestShutdownWaitsForGoroutines(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	prov.startCh = make(chan struct{}) // Start blocks
	s := newTestScaler(client, prov)

	_, _ = s.HandleDesiredRunnerCount(context.Background(), 2)
	time.Sleep(50 * time.Millisecond)

	// Shutdown should block until goroutines finish
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Shutdown(ctx)
	elapsed := time.Since(start)

	// Should complete quickly (Start is cancelled by lifecycle context)
	if elapsed > 3*time.Second {
		t.Errorf("Shutdown took too long: %v", elapsed)
	}

	runners := s.Runners()
	if len(runners) != 0 {
		t.Errorf("expected 0 runners, got %d", len(runners))
	}
}

func TestConcurrentRunners(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	s := newTestScaler(client, prov)

	// Start 3 runners
	_, _ = s.HandleDesiredRunnerCount(context.Background(), 3)
	time.Sleep(50 * time.Millisecond)

	runners := s.Runners()
	if len(runners) != 3 {
		t.Fatalf("expected 3 runners, got %d", len(runners))
	}

	// Complete them in reverse order
	for i := len(runners) - 1; i >= 0; i-- {
		_ = s.HandleJobCompleted(context.Background(), &scaleset.JobCompleted{
			RunnerName: runners[i].Name,
			Result:     "succeeded",
		})
	}

	time.Sleep(200 * time.Millisecond)

	remaining := s.Runners()
	if len(remaining) != 0 {
		t.Errorf("expected 0 runners, got %d", len(remaining))
	}

	stopped := prov.stoppedNames()
	if len(stopped) != 3 {
		t.Errorf("expected 3 Stop calls, got %d", len(stopped))
	}

	s.Shutdown(context.Background())
}

func TestMaxRunnersClamping(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	s := NewScaler(noopLogger(), client, 1, 2, "test",
		&RunnerConfig{Docker: &DockerImage{Image: "test:latest"}}, prov)

	// Request more than max
	count, err := s.HandleDesiredRunnerCount(context.Background(), 10)
	if err != nil {
		t.Fatalf("HandleDesiredRunnerCount: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count clamped to 2, got %d", count)
	}

	s.Shutdown(context.Background())
}

func TestDesiredCountZero(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	s := newTestScaler(client, prov)

	count, err := s.HandleDesiredRunnerCount(context.Background(), 0)
	if err != nil {
		t.Fatalf("HandleDesiredRunnerCount: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}

	runners := s.Runners()
	if len(runners) != 0 {
		t.Errorf("expected 0 runners, got %d", len(runners))
	}

	s.Shutdown(context.Background())
}

func TestJitConfigError(t *testing.T) {
	client := newMockClient()
	client.jitErr = errors.New("GitHub API unavailable")
	prov := newMockProvisioner()
	s := newTestScaler(client, prov)

	count, err := s.HandleDesiredRunnerCount(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error from GenerateJitRunnerConfig")
	}
	if count != 0 {
		t.Errorf("expected count 0 on JIT error, got %d", count)
	}

	runners := s.Runners()
	if len(runners) != 0 {
		t.Errorf("expected 0 runners on JIT error, got %d", len(runners))
	}

	s.Shutdown(context.Background())
}

func TestJobCompletedUnknownRunner(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	s := newTestScaler(client, prov)

	err := s.HandleJobCompleted(context.Background(), &scaleset.JobCompleted{
		RunnerName: "nonexistent-runner",
		Result:     "succeeded",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	s.Shutdown(context.Background())
}

func TestJobStartedUnknownRunner(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	s := newTestScaler(client, prov)

	err := s.HandleJobStarted(context.Background(), &scaleset.JobStarted{
		RunnerName: "nonexistent-runner",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	s.Shutdown(context.Background())
}

func TestJobCompletedDuringProvisioning(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	prov.startCh = make(chan struct{})
	s := newTestScaler(client, prov)

	_, _ = s.HandleDesiredRunnerCount(context.Background(), 1)
	time.Sleep(50 * time.Millisecond)

	runners := s.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(runners))
	}
	name := runners[0].Name

	// Job completes while still provisioning
	_ = s.HandleJobCompleted(context.Background(), &scaleset.JobCompleted{
		RunnerName: name,
		Result:     "succeeded",
	})

	// Unblock Start
	close(prov.startCh)
	time.Sleep(100 * time.Millisecond)

	// Runner should be cleaned up
	runners = s.Runners()
	if len(runners) != 0 {
		t.Errorf("expected 0 runners, got %d", len(runners))
	}

	// Stop should still be called
	stopped := prov.stoppedNames()
	if len(stopped) != 1 {
		t.Errorf("expected 1 Stop call, got %d", len(stopped))
	}

	s.Shutdown(context.Background())
}

func TestJobStartedDuringProvisioning(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	prov.startCh = make(chan struct{})
	s := newTestScaler(client, prov)

	_, _ = s.HandleDesiredRunnerCount(context.Background(), 1)
	time.Sleep(50 * time.Millisecond)

	runners := s.Runners()
	name := runners[0].Name

	// Job starts while provisioning
	_ = s.HandleJobStarted(context.Background(), &scaleset.JobStarted{
		RunnerName: name,
	})

	// Phase should be Running even though Start hasn't returned
	runners = s.Runners()
	if runners[0].Phase != RunnerRunning {
		t.Errorf("expected Running, got %s", runners[0].Phase)
	}

	// Unblock Start
	close(prov.startCh)
	time.Sleep(50 * time.Millisecond)

	// Should stay Running (not reset to Idle)
	runners = s.Runners()
	if runners[0].Phase != RunnerRunning {
		t.Errorf("expected Running after Start, got %s", runners[0].Phase)
	}

	s.Shutdown(context.Background())
}

func TestShutdownWithNoRunners(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	s := newTestScaler(client, prov)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	s.Shutdown(ctx)
}

func TestRunnerNamePrefix(t *testing.T) {
	client := newMockClient()
	prov := newMockProvisioner()
	s := NewScaler(noopLogger(), client, 1, 10, "my-scale-set",
		&RunnerConfig{Docker: &DockerImage{Image: "test:latest"}}, prov)

	_, _ = s.HandleDesiredRunnerCount(context.Background(), 1)
	time.Sleep(50 * time.Millisecond)

	runners := s.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(runners))
	}

	name := runners[0].Name
	if len(name) < len("my-scale-set-") {
		t.Fatalf("runner name too short: %s", name)
	}
	prefix := name[:len("my-scale-set-")]
	if prefix != "my-scale-set-" {
		t.Errorf("expected prefix 'my-scale-set-', got %q", prefix)
	}

	s.Shutdown(context.Background())
}
