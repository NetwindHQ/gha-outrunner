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
	ConfigFile string
}

var rootCmd = &cobra.Command{
	Use:   "outrunner",
	Short: "Ephemeral GitHub Actions runners — no Kubernetes required",
	Long: `outrunner provisions ephemeral Docker containers and/or VMs for each
GitHub Actions job. It uses the scaleset API to register as an autoscaling
runner group, then creates and destroys runner environments on demand.

Configure images in a YAML config file. Each image declares labels it
satisfies and which backend (docker, libvirt) to use. Job labels are
matched against image labels to select the right environment.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer cancel()
		return run(ctx)
	},
}

func init() {
	f := rootCmd.Flags()
	f.StringVar(&cfg.URL, "url", "", "Repository or org URL (e.g. https://github.com/owner/repo)")
	f.StringVar(&cfg.Name, "name", "outrunner", "Scale set name")
	f.StringVar(&cfg.Token, "token", "", "GitHub PAT (fine-grained, Administration read/write)")
	f.IntVar(&cfg.MaxRunners, "max-runners", 2, "Maximum concurrent runners")
	f.StringVar(&cfg.ConfigFile, "config", "", "Config file path (YAML)")

	rootCmd.MarkFlagRequired("url")
	rootCmd.MarkFlagRequired("token")
	rootCmd.MarkFlagRequired("config")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Load config
	config, err := outrunner.LoadConfig(cfg.ConfigFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger.Info("Loaded config", slog.Int("images", len(config.Images)))

	// Create scaleset client
	client, err := scaleset.NewClientWithPersonalAccessToken(scaleset.NewClientWithPersonalAccessTokenConfig{
		GitHubConfigURL:     cfg.URL,
		PersonalAccessToken: cfg.Token,
	})
	if err != nil {
		return fmt.Errorf("create scaleset client: %w", err)
	}

	// Register all image labels on the scale set
	var labels []scaleset.Label
	for _, l := range config.AllLabels() {
		labels = append(labels, scaleset.Label{Name: l, Type: "User"})
	}

	// Get or create scale set
	logger.Info("Looking for scale set", slog.String("name", cfg.Name))
	scaleSet, err := client.GetRunnerScaleSet(ctx, 1, cfg.Name)
	if err != nil || scaleSet == nil {
		logger.Info("Scale set not found, creating", slog.String("name", cfg.Name))
		scaleSet, err = client.CreateRunnerScaleSet(ctx, &scaleset.RunnerScaleSet{
			Name:          cfg.Name,
			RunnerGroupID: 1,
			Labels:        labels,
			RunnerSetting: scaleset.RunnerSetting{
				DisableUpdate: true,
			},
		})
		if err != nil {
			return fmt.Errorf("create scale set: %w", err)
		}
		logger.Info("Scale set created", slog.Int("id", scaleSet.ID))
	} else {
		logger.Info("Using existing scale set", slog.Int("id", scaleSet.ID))
	}

	defer func() {
		logger.Info("Deleting scale set")
		if err := client.DeleteRunnerScaleSet(context.WithoutCancel(ctx), scaleSet.ID); err != nil {
			logger.Error("Failed to delete scale set", slog.String("error", err.Error()))
		}
	}()

	// Create multi-provisioner with backends based on config
	multi := outrunner.NewMultiProvisioner(logger.WithGroup("provisioner"), config)

	if config.NeedsDocker() {
		prov, err := outrunner.NewDockerProvisioner(logger.WithGroup("docker"))
		if err != nil {
			return fmt.Errorf("create docker provisioner: %w", err)
		}
		multi.Register("docker", prov)
		logger.Info("Docker provisioner initialized")
	}

	if config.NeedsLibvirt() {
		prov, err := outrunner.NewLibvirtProvisioner(
			logger.WithGroup("libvirt"),
			outrunner.LibvirtConfig{},
		)
		if err != nil {
			return fmt.Errorf("create libvirt provisioner: %w", err)
		}
		prov.Cleanup(cfg.Name + "-")
		multi.Register("libvirt", prov)
		logger.Info("Libvirt provisioner initialized")
	}

	if config.NeedsTart() {
		prov := outrunner.NewTartProvisioner(logger.WithGroup("tart"))
		prov.Cleanup(cfg.Name + "-")
		multi.Register("tart", prov)
		logger.Info("Tart provisioner initialized")
	}

	defer multi.Close()

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
		client, scaleSet.ID, cfg.MaxRunners, multi,
	)

	logger.Info("Listening for jobs",
		slog.String("scaleSet", cfg.Name),
		slog.Int("maxRunners", cfg.MaxRunners),
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
