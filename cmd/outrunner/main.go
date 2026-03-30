package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/actions/scaleset"
	"github.com/actions/scaleset/listener"
	"github.com/google/uuid"
	outrunner "github.com/psubocz/gha-outrunner"
	"github.com/psubocz/gha-outrunner/provisioner/docker"
	"github.com/psubocz/gha-outrunner/provisioner/libvirt"
	"github.com/psubocz/gha-outrunner/provisioner/tart"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var cfg struct {
	URL        string
	Token      string
	MaxRunners int
	ConfigFile string
}

var rootCmd = &cobra.Command{
	Use:   "outrunner",
	Short: "Ephemeral GitHub Actions runners, no Kubernetes required",
	Long: `outrunner provisions ephemeral Docker containers and/or VMs for each
GitHub Actions job. It uses the scaleset API to register as an autoscaling
runner group, then creates and destroys runner environments on demand.

Each runner definition in the config file gets its own scale set. GitHub
routes jobs to the correct scale set based on labels.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer cancel()
		return run(ctx)
	},
}

func init() {
	f := rootCmd.Flags()
	f.StringVar(&cfg.URL, "url", "", "Repository or org URL (e.g. https://github.com/owner/repo)")
	f.StringVar(&cfg.Token, "token", "", "GitHub PAT (fine-grained, Administration read/write)")
	f.IntVar(&cfg.MaxRunners, "max-runners", 2, "Default max concurrent runners per scale set")
	f.StringVar(&cfg.ConfigFile, "config", "", "Config file path (YAML)")

	_ = rootCmd.MarkFlagRequired("url")
	_ = rootCmd.MarkFlagRequired("token")
	_ = rootCmd.MarkFlagRequired("config")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	config, err := outrunner.LoadConfig(cfg.ConfigFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger.Info("Loaded config", slog.Int("runners", len(config.Runners)))

	client, err := scaleset.NewClientWithPersonalAccessToken(scaleset.NewClientWithPersonalAccessTokenConfig{
		GitHubConfigURL:     cfg.URL,
		PersonalAccessToken: cfg.Token,
	})
	if err != nil {
		return fmt.Errorf("create scaleset client: %w", err)
	}

	g, ctx := errgroup.WithContext(ctx)
	for name, runner := range config.Runners {
		maxRunners := runner.MaxRunners
		if maxRunners == 0 {
			maxRunners = cfg.MaxRunners
		}
		g.Go(func() error {
			return runWorker(ctx, logger, client, name, &runner, maxRunners)
		})
	}

	err = g.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	logger.Info("Shut down cleanly")
	return nil
}

func runWorker(ctx context.Context, logger *slog.Logger, client *scaleset.Client, name string, runner *outrunner.RunnerConfig, maxRunners int) error {
	logger = logger.With(slog.String("scaleSet", name))

	// Create provisioner
	prov, err := createProvisioner(logger, runner)
	if err != nil {
		return fmt.Errorf("runner %s: %w", name, err)
	}
	defer func() { _ = prov.Close() }()

	// Clean up orphans from previous runs
	cleanupOrphans(logger, prov, name)

	// Build labels
	var labels []scaleset.Label
	for _, l := range runner.Labels {
		labels = append(labels, scaleset.Label{Name: l, Type: "User"})
	}

	// Get or create scale set
	logger.Info("Looking for scale set")
	scaleSet, err := client.GetRunnerScaleSet(ctx, 1, name)
	if err != nil || scaleSet == nil {
		logger.Info("Creating scale set")
		scaleSet, err = client.CreateRunnerScaleSet(ctx, &scaleset.RunnerScaleSet{
			Name:          name,
			RunnerGroupID: 1,
			Labels:        labels,
			RunnerSetting: scaleset.RunnerSetting{
				DisableUpdate: true,
			},
		})
		if err != nil {
			return fmt.Errorf("runner %s: create scale set: %w", name, err)
		}
	}
	logger.Info("Scale set ready", slog.Int("id", scaleSet.ID))

	defer func() {
		logger.Info("Deleting scale set")
		if err := client.DeleteRunnerScaleSet(context.WithoutCancel(ctx), scaleSet.ID); err != nil {
			logger.Error("Failed to delete scale set", slog.String("error", err.Error()))
		}
	}()

	// Create message session
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = uuid.NewString()
	}

	sessionClient, err := client.MessageSessionClient(ctx, scaleSet.ID, hostname)
	if err != nil {
		return fmt.Errorf("runner %s: create message session: %w", name, err)
	}
	defer func() { _ = sessionClient.Close(context.Background()) }()

	// Create listener
	l, err := listener.New(sessionClient, listener.Config{
		ScaleSetID: scaleSet.ID,
		MaxRunners: maxRunners,
		Logger:     logger.WithGroup("listener"),
	})
	if err != nil {
		return fmt.Errorf("runner %s: create listener: %w", name, err)
	}

	// Create scaler
	scaler := outrunner.NewScaler(
		logger.WithGroup("scaler"),
		client, scaleSet.ID, maxRunners, name, runner, prov,
	)

	logger.Info("Listening for jobs", slog.Int("maxRunners", maxRunners))
	err = l.Run(ctx, scaler)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	scaler.Shutdown(shutdownCtx)

	if !errors.Is(err, context.Canceled) {
		return fmt.Errorf("runner %s: listener: %w", name, err)
	}
	return nil
}

func createProvisioner(logger *slog.Logger, runner *outrunner.RunnerConfig) (outrunner.Provisioner, error) {
	switch runner.ProviderType() {
	case "docker":
		return docker.New(logger.WithGroup("docker"))
	case "libvirt":
		return libvirt.New(logger.WithGroup("libvirt"), libvirt.Config{})
	case "tart":
		return tart.New(logger.WithGroup("tart")), nil
	default:
		return nil, fmt.Errorf("unknown provider type %q", runner.ProviderType())
	}
}

// cleanupOrphans removes leftover resources from previous runs.
func cleanupOrphans(logger *slog.Logger, prov outrunner.Provisioner, name string) {
	prefix := name + "-"
	type cleaner interface {
		Cleanup(prefix string)
	}
	if c, ok := prov.(cleaner); ok {
		c.Cleanup(prefix)
	}
}
