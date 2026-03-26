package outrunner

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// DockerProvisioner creates ephemeral Docker containers as GitHub Actions runners.
type DockerProvisioner struct {
	logger *slog.Logger
	client *client.Client
	image  string
}

// DockerConfig configures the Docker provisioner.
type DockerConfig struct {
	// Image is the Docker image to use for runners.
	// Must have the GitHub Actions runner pre-installed at /actions-runner.
	Image string
}

func NewDockerProvisioner(logger *slog.Logger, cfg DockerConfig) (*DockerProvisioner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}

	return &DockerProvisioner{
		logger: logger,
		client: cli,
		image:  cfg.Image,
	}, nil
}

func (d *DockerProvisioner) Start(ctx context.Context, req *RunnerRequest) error {
	// Pull image if not present
	d.logger.Debug("Pulling image", slog.String("image", d.image))
	reader, err := d.client.ImagePull(ctx, d.image, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	io.Copy(io.Discard, reader)
	reader.Close()

	resp, err := d.client.ContainerCreate(ctx,
		&container.Config{
			Image: d.image,
			Cmd:   []string{"./run.sh", "--jitconfig", req.JITConfig},
			Labels: map[string]string{
				"outrunner":      "true",
				"outrunner.name": req.Name,
			},
		},
		&container.HostConfig{
			AutoRemove: true,
		},
		nil, nil, req.Name,
	)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	d.logger.Info("Container started",
		slog.String("name", req.Name),
		slog.String("id", resp.ID[:12]),
	)
	return nil
}

func (d *DockerProvisioner) Stop(ctx context.Context, name string) error {
	d.logger.Debug("Stopping container", slog.String("name", name))
	err := d.client.ContainerStop(ctx, name, container.StopOptions{})
	if err != nil {
		// Container may already be gone (AutoRemove)
		d.logger.Debug("Container stop returned error (may already be removed)",
			slog.String("name", name),
			slog.String("error", err.Error()),
		)
	}
	return nil
}

func (d *DockerProvisioner) Close() error {
	return d.client.Close()
}
