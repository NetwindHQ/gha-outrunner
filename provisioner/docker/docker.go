package docker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	outrunner "github.com/NetwindHQ/gha-outrunner"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

// Provisioner creates ephemeral Docker containers as GitHub Actions runners.
type Provisioner struct {
	logger *slog.Logger
	client *client.Client
}

func New(logger *slog.Logger) (*Provisioner, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}

	// If DOCKER_HOST isn't set, ask the docker CLI for the active context's endpoint.
	if os.Getenv("DOCKER_HOST") == "" {
		if host := dockerHostFromContext(); host != "" {
			logger.Info("Auto-detected Docker host", slog.String("host", host))
			opts = append(opts, client.WithHost(host))
		}
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}

	return &Provisioner{
		logger: logger,
		client: cli,
	}, nil
}

// dockerHostFromContext asks the docker CLI for the active context's endpoint.
// Works with Colima, Docker Desktop, Podman, etc.
func dockerHostFromContext() string {
	out, err := exec.Command("docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}").Output()
	if err != nil {
		return ""
	}
	host := strings.TrimSpace(string(out))
	if host == "" || host == "<no value>" {
		return ""
	}
	// Verify the socket actually exists
	if strings.HasPrefix(host, "unix://") {
		if _, err := os.Stat(strings.TrimPrefix(host, "unix://")); err != nil {
			return ""
		}
	}
	return host
}

func (d *Provisioner) Start(ctx context.Context, req *outrunner.RunnerRequest) error {
	if req.Runner == nil || req.Runner.Docker == nil {
		return fmt.Errorf("no docker config for runner %s", req.Name)
	}
	dcfg := req.Runner.Docker
	img := dcfg.Image

	runnerCmd := dcfg.RunnerCmd
	if runnerCmd == "" {
		runnerCmd = "./run.sh"
	}

	// Pull image only if not available locally
	_, err := d.client.ImageInspect(ctx, img)
	if err != nil {
		d.logger.Debug("Pulling image", slog.String("image", img))
		reader, pullErr := d.client.ImagePull(ctx, img, image.PullOptions{})
		if pullErr != nil {
			return fmt.Errorf("pull image: %w", pullErr)
		}
		if _, err := io.Copy(io.Discard, reader); err != nil {
			_ = reader.Close()
			return fmt.Errorf("drain image pull: %w", err)
		}
		_ = reader.Close()
	}

	var mounts []mount.Mount
	for _, m := range dcfg.Mounts {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.Source,
			Target:   m.Target,
			ReadOnly: m.ReadOnly,
		})
	}

	resp, err := d.client.ContainerCreate(ctx,
		&container.Config{
			Image: img,
			Cmd:   []string{runnerCmd, "--jitconfig", req.JITConfig},
			Labels: map[string]string{
				"outrunner":      "true",
				"outrunner.name": req.Name,
			},
		},
		&container.HostConfig{
			AutoRemove: true,
			Mounts:     mounts,
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
		slog.String("image", img),
		slog.String("id", resp.ID[:12]),
	)
	return nil
}

func (d *Provisioner) Stop(ctx context.Context, name string) error {
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

func (d *Provisioner) Close() error {
	return d.client.Close()
}
