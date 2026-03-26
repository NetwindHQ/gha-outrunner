package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/actions/scaleset"
	"github.com/actions/scaleset/listener"
	"github.com/google/uuid"
	outrunner "github.com/psubocz/gha-outrunner"
	"github.com/spf13/cobra"
)

var cfg struct {
	URL        string
	Name       string
	Token      string
	MaxRunners int
	Image      string
}

var rootCmd = &cobra.Command{
	Use:   "outrunner",
	Short: "Ephemeral GitHub Actions runners — no Kubernetes required",
	Long: `outrunner provisions ephemeral Docker containers (or VMs) for each
GitHub Actions job. It uses the scaleset API to register as an autoscaling
runner group, then creates and destroys runner environments on demand.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer cancel()
		return run(ctx)
	},
}

func init() {
	f := rootCmd.Flags()
	f.StringVar(&cfg.URL, "url", "", "Repository or org URL (e.g. https://github.com/owner/repo)")
	f.StringVar(&cfg.Name, "name", "outrunner", "Scale set name (used as runs-on label)")
	f.StringVar(&cfg.Token, "token", "", "GitHub PAT (fine-grained, Administration read/write)")
	f.IntVar(&cfg.MaxRunners, "max-runners", 2, "Maximum concurrent runners")
	f.StringVar(&cfg.Image, "image", "ghcr.io/actions/actions-runner:latest", "Docker image for runners")

	rootCmd.MarkFlagRequired("url")
	rootCmd.MarkFlagRequired("token")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create scaleset client
	client, err := scaleset.NewClientWithPersonalAccessToken(scaleset.NewClientWithPersonalAccessTokenConfig{
		GitHubConfigURL:     cfg.URL,
		PersonalAccessToken: cfg.Token,
	})
	if err != nil {
		return fmt.Errorf("create scaleset client: %w", err)
	}

	// Register scale set
	logger.Info("Creating scale set", slog.String("name", cfg.Name))
	scaleSet, err := client.CreateRunnerScaleSet(ctx, &scaleset.RunnerScaleSet{
		Name:          cfg.Name,
		RunnerGroupID: 1,
		Labels: []scaleset.Label{
			{Name: cfg.Name, Type: "User"},
		},
		RunnerSetting: scaleset.RunnerSetting{
			DisableUpdate: true,
		},
	})
	if err != nil {
		return fmt.Errorf("create scale set: %w", err)
	}
	logger.Info("Scale set created", slog.Int("id", scaleSet.ID))

	defer func() {
		logger.Info("Deleting scale set")
		if err := client.DeleteRunnerScaleSet(context.WithoutCancel(ctx), scaleSet.ID); err != nil {
			logger.Error("Failed to delete scale set", slog.String("error", err.Error()))
		}
	}()

	// Create provisioner
	prov, err := outrunner.NewDockerProvisioner(
		logger.WithGroup("docker"),
		outrunner.DockerConfig{Image: cfg.Image},
	)
	if err != nil {
		return fmt.Errorf("create docker provisioner: %w", err)
	}
	defer prov.Close()

	// Create message session
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = uuid.NewString()
	}

	sessionClient, err := client.MessageSessionClient(ctx, scaleSet.ID, hostname)
	if err != nil {
		return fmt.Errorf("create message session: %w", err)
	}
	defer sessionClient.Close(context.Background())

	// Create listener
	l, err := listener.New(sessionClient, listener.Config{
		ScaleSetID: scaleSet.ID,
		MaxRunners: cfg.MaxRunners,
		Logger:     logger.WithGroup("listener"),
	})
	if err != nil {
		return fmt.Errorf("create listener: %w", err)
	}

	// Create scaler
	scaler := outrunner.NewScaler(
		logger.WithGroup("scaler"),
		client, scaleSet.ID, cfg.MaxRunners, prov,
	)

	logger.Info("Listening for jobs",
		slog.String("runsOn", cfg.Name),
		slog.Int("maxRunners", cfg.MaxRunners),
		slog.String("image", cfg.Image),
	)

	err = l.Run(ctx, scaler)

	// Graceful shutdown
	scaler.Shutdown(context.Background())

	if !errors.Is(err, context.Canceled) {
		return fmt.Errorf("listener: %w", err)
	}

	logger.Info("Shut down cleanly")
	return nil
}
