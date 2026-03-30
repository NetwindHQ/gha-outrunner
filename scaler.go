package outrunner

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/actions/scaleset"
	"github.com/actions/scaleset/listener"
	"github.com/google/uuid"
)

// ScaleSetClient is the subset of scaleset.Client that Scaler uses.
// Extracted as an interface for testability.
type ScaleSetClient interface {
	GenerateJitRunnerConfig(ctx context.Context, setting *scaleset.RunnerScaleSetJitRunnerSetting, scaleSetID int) (*scaleset.RunnerScaleSetJitRunnerConfig, error)
	RemoveRunner(ctx context.Context, runnerID int64) error
}

// Scaler implements listener.Scaler by provisioning real runner environments.
// Each runner gets its own goroutine that manages the full lifecycle:
// provisioning, waiting for job completion, stopping, and deregistration.
type Scaler struct {
	logger      *slog.Logger
	client      ScaleSetClient
	scaleSetID  int
	maxRunners  int
	namePrefix  string
	runner      *RunnerConfig
	provisioner Provisioner

	mu      sync.Mutex
	runners map[string]*RunnerState

	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	wg              sync.WaitGroup
}

var _ listener.Scaler = (*Scaler)(nil)

func NewScaler(logger *slog.Logger, client ScaleSetClient, scaleSetID, maxRunners int, namePrefix string, runner *RunnerConfig, prov Provisioner) *Scaler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scaler{
		logger:          logger,
		client:          client,
		scaleSetID:      scaleSetID,
		maxRunners:      maxRunners,
		namePrefix:      namePrefix,
		runner:          runner,
		provisioner:     prov,
		runners:         make(map[string]*RunnerState),
		lifecycleCtx:    ctx,
		lifecycleCancel: cancel,
	}
}

func (s *Scaler) HandleDesiredRunnerCount(ctx context.Context, count int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	target := min(s.maxRunners, count)
	current := len(s.runners)

	s.logger.Debug("Desired runner count",
		slog.Int("requested", count),
		slog.Int("target", target),
		slog.Int("current", current),
	)

	for range target - current {
		name := fmt.Sprintf("%s-%s", s.namePrefix, uuid.NewString()[:8])

		jit, err := s.client.GenerateJitRunnerConfig(ctx,
			&scaleset.RunnerScaleSetJitRunnerSetting{Name: name},
			s.scaleSetID,
		)
		if err != nil {
			return len(s.runners), fmt.Errorf("generate JIT config: %w", err)
		}

		state := &RunnerState{
			Name:      name,
			RunnerID:  jit.Runner.ID,
			Phase:     RunnerProvisioning,
			CreatedAt: time.Now(),
			done:      make(chan struct{}),
		}
		s.runners[name] = state

		req := &RunnerRequest{
			Name:      name,
			JITConfig: jit.EncodedJITConfig,
			Runner:    s.runner,
		}

		s.logger.Info("Spawning runner",
			slog.String("name", name),
			slog.Int("runnerID", state.RunnerID),
		)

		s.wg.Add(1)
		go s.runRunnerLifecycle(state, req)
	}

	return len(s.runners), nil
}

func (s *Scaler) HandleJobStarted(ctx context.Context, jobInfo *scaleset.JobStarted) error {
	s.mu.Lock()
	state, exists := s.runners[jobInfo.RunnerName]
	if exists {
		state.Phase = RunnerRunning
	}
	s.mu.Unlock()

	s.logger.Info("Job started",
		slog.String("runnerName", jobInfo.RunnerName),
		slog.Int64("requestId", jobInfo.RunnerRequestID),
	)
	return nil
}

func (s *Scaler) HandleJobCompleted(ctx context.Context, jobInfo *scaleset.JobCompleted) error {
	s.logger.Info("Job completed",
		slog.String("runnerName", jobInfo.RunnerName),
		slog.String("result", jobInfo.Result),
	)

	s.mu.Lock()
	state, exists := s.runners[jobInfo.RunnerName]
	s.mu.Unlock()

	if exists {
		state.SignalDone()
	}

	return nil
}

// Shutdown cancels all runner goroutines and waits for them to finish.
func (s *Scaler) Shutdown(ctx context.Context) {
	s.lifecycleCancel()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("All runners shut down")
	case <-ctx.Done():
		s.logger.Warn("Shutdown timed out, some runners may not be fully cleaned up")
	}
}

// RunnerSnapshot is a point-in-time copy of a runner's state, safe to read
// without holding the scaler's mutex.
type RunnerSnapshot struct {
	Name      string
	RunnerID  int
	Phase     RunnerPhase
	CreatedAt time.Time
	StartedAt time.Time
}

// Runners returns a snapshot of current runner states.
func (s *Scaler) Runners() []RunnerSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshots := make([]RunnerSnapshot, 0, len(s.runners))
	for _, state := range s.runners {
		snapshots = append(snapshots, RunnerSnapshot{
			Name:      state.Name,
			RunnerID:  state.RunnerID,
			Phase:     state.Phase,
			CreatedAt: state.CreatedAt,
			StartedAt: state.StartedAt,
		})
	}
	return snapshots
}

// runRunnerLifecycle manages the full lifecycle of a single runner.
// Runs in its own goroutine. Handles provisioning, waiting, stopping,
// deregistration, and map cleanup.
func (s *Scaler) runRunnerLifecycle(state *RunnerState, req *RunnerRequest) {
	defer s.wg.Done()
	defer s.removeRunner(state.Name)

	name := state.Name

	// 1. Provision
	if err := s.provisioner.Start(s.lifecycleCtx, req); err != nil {
		s.logger.Error("Failed to start runner",
			slog.String("name", name),
			slog.String("error", err.Error()),
		)
		s.deregisterRunner(name, state.RunnerID)
		return
	}

	s.mu.Lock()
	if state.Phase == RunnerRunning {
		// HandleJobStarted arrived during provisioning
		s.logger.Info("Runner provisioned (job already assigned)", slog.String("name", name))
	} else {
		state.Phase = RunnerIdle
		s.logger.Info("Runner provisioned", slog.String("name", name))
	}
	state.StartedAt = time.Now()
	s.mu.Unlock()

	// 2. Wait for completion signal or shutdown
	select {
	case <-state.done:
		s.logger.Debug("Runner signaled done", slog.String("name", name))
	case <-s.lifecycleCtx.Done():
		s.logger.Debug("Runner shutdown requested", slog.String("name", name))
	}

	// 3. Stop
	s.mu.Lock()
	state.Phase = RunnerStopping
	s.mu.Unlock()

	s.logger.Info("Stopping runner", slog.String("name", name))
	if err := s.provisioner.Stop(context.Background(), name); err != nil {
		s.logger.Error("Failed to stop runner",
			slog.String("name", name),
			slog.String("error", err.Error()),
		)
	}

	// 4. Deregister from GitHub
	s.deregisterRunner(name, state.RunnerID)
}

func (s *Scaler) removeRunner(name string) {
	s.mu.Lock()
	delete(s.runners, name)
	s.mu.Unlock()
}

func (s *Scaler) deregisterRunner(name string, runnerID int) {
	if runnerID == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.client.RemoveRunner(ctx, int64(runnerID)); err != nil {
		s.logger.Debug("Failed to deregister runner",
			slog.String("name", name),
			slog.Int("runnerID", runnerID),
			slog.String("error", err.Error()),
		)
	}
}
