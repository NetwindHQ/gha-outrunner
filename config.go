package outrunner

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the outrunner configuration file format.
type Config struct {
	Images []ImageConfig `yaml:"images"`
}

// ImageConfig defines a runner environment and the label it satisfies.
// Exactly one of Docker, Libvirt, or Tart must be set.
type ImageConfig struct {
	Label   string         `yaml:"label"`
	Docker  *DockerImage   `yaml:"docker,omitempty"`
	Libvirt *LibvirtImage  `yaml:"libvirt,omitempty"`
	Tart    *TartImage     `yaml:"tart,omitempty"`
}

// DockerImage configures a Docker-based runner.
type DockerImage struct {
	Image string `yaml:"image"`
}

// LibvirtImage configures a libvirt/QEMU-based runner.
type LibvirtImage struct {
	Path      string `yaml:"path"`
	RunnerCmd string `yaml:"runner_cmd"`
	CPUs      int    `yaml:"cpus"`
	MemoryMB  int    `yaml:"memory"`
}

// TartImage configures a Tart-based runner (macOS/Linux on Apple Silicon).
type TartImage struct {
	Image     string `yaml:"image"`      // OCI image or local VM name
	RunnerCmd string `yaml:"runner_cmd"`
	CPUs      int    `yaml:"cpus"`
	MemoryMB  int    `yaml:"memory"`
}

// ProviderType returns which provisioner backend this image uses.
func (img *ImageConfig) ProviderType() string {
	switch {
	case img.Docker != nil:
		return "docker"
	case img.Libvirt != nil:
		return "libvirt"
	case img.Tart != nil:
		return "tart"
	default:
		return ""
	}
}

// LoadConfig reads and parses a config file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	for i := range cfg.Images {
		img := &cfg.Images[i]

		if img.Label == "" {
			return nil, fmt.Errorf("image %d: label is required", i)
		}
		if img.ProviderType() == "" {
			return nil, fmt.Errorf("image %q: must specify docker, libvirt, or tart", img.Label)
		}

		// Apply defaults for libvirt images
		if img.Libvirt != nil {
			if img.Libvirt.CPUs == 0 {
				img.Libvirt.CPUs = 4
			}
			if img.Libvirt.MemoryMB == 0 {
				img.Libvirt.MemoryMB = 8192
			}
		}

		// Apply defaults for tart images
		if img.Tart != nil {
			if img.Tart.CPUs == 0 {
				img.Tart.CPUs = 4
			}
			if img.Tart.MemoryMB == 0 {
				img.Tart.MemoryMB = 8192
			}
		}
	}

	return &cfg, nil
}

// MatchImage finds the image for a job based on its labels.
// If labels are empty (scaleset API doesn't expose them yet — see #20),
// falls back to the first image.
func (c *Config) MatchImage(jobLabels []string) (*ImageConfig, error) {
	if len(c.Images) == 0 {
		return nil, fmt.Errorf("no images configured")
	}

	if len(jobLabels) == 0 {
		return &c.Images[0], nil
	}

	jobSet := make(map[string]bool, len(jobLabels))
	for _, l := range jobLabels {
		jobSet[l] = true
	}

	for i := range c.Images {
		if jobSet[c.Images[i].Label] {
			return &c.Images[i], nil
		}
	}

	return nil, fmt.Errorf("no image matches labels %v", jobLabels)
}

// AllLabels returns all unique image labels (for scale set registration).
func (c *Config) AllLabels() []string {
	var labels []string
	for _, img := range c.Images {
		labels = append(labels, img.Label)
	}
	return labels
}

// NeedsDocker returns true if any image uses the Docker backend.
func (c *Config) NeedsDocker() bool {
	for _, img := range c.Images {
		if img.Docker != nil {
			return true
		}
	}
	return false
}

// NeedsLibvirt returns true if any image uses the libvirt backend.
func (c *Config) NeedsLibvirt() bool {
	for _, img := range c.Images {
		if img.Libvirt != nil {
			return true
		}
	}
	return false
}

// NeedsTart returns true if any image uses the Tart backend.
func (c *Config) NeedsTart() bool {
	for _, img := range c.Images {
		if img.Tart != nil {
			return true
		}
	}
	return false
}
