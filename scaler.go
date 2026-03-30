package outrunner

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/actions/scaleset"
	"github.com/actions/scaleset/listener"
	"github.com/google/uuid"
)

// Scaler implements listener.Scaler by provisioning real runner environments.
type Scaler struct {
	logger      *slog.Logger
	client      *scaleset.Client
	scaleSetID  int
	maxRunners  int
	namePrefix  string
	runner      *RunnerConfig
	provisioner Provisioner

	mu      sync.Mutex
	runners map[string]struct{}
}

var _ listener.Scaler = (*Scaler)(nil)

func NewScaler(logger *slog.Logger, client *scaleset.Client, scaleSetID, maxRunners int, namePrefix string, runner *RunnerConfig, prov Provisioner) *Scaler {
	return &Scaler{
		logger:      logger,
		client:      client,
		scaleSetID:  scaleSetID,
		maxRunners:  maxRunners,
		namePrefix:  namePrefix,
		runner:      runner,
		provisioner: prov,
		runners:     make(map[string]struct{}),
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

		req := &RunnerRequest{
			Name:      name,
			JITConfig: jit.EncodedJITConfig,
			Runner:    s.runner,
		}

		s.logger.Info("Starting runner", slog.String("name", name))
		if err := s.provisioner.Start(ctx, req); err != nil {
			s.logger.Error("Failed to start runner",
				slog.String("name", name),
				slog.String("error", err.Error()),
			)
			continue
		}
		s.runners[name] = struct{}{}
	}

	return len(s.runners), nil
}

func (s *Scaler) HandleJobStarted(ctx context.Context, jobInfo *scaleset.JobStarted) error {
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

	name := jobInfo.RunnerName

	s.mu.Lock()
	_, exists := s.runners[name]
	delete(s.runners, name)
	s.mu.Unlock()

	if exists {
		s.logger.Info("Stopping runner", slog.String("name", name))
		if err := s.provisioner.Stop(ctx, name); err != nil {
			s.logger.Error("Failed to stop runner",
				slog.String("name", name),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// Shutdown stops all running runners. Call during graceful shutdown.
func (s *Scaler) Shutdown(ctx context.Context) {
	s.mu.Lock()
	names := make([]string, 0, len(s.runners))
	for name := range s.runners {
		names = append(names, name)
	}
	s.mu.Unlock()

	for _, name := range names {
		s.logger.Info("Shutting down runner", slog.String("name", name))
		if err := s.provisioner.Stop(ctx, name); err != nil {
			s.logger.Error("Failed to stop runner during shutdown",
				slog.String("name", name),
				slog.String("error", err.Error()),
			)
		}
	}
}
