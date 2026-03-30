package outrunner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the outrunner configuration file format.
type Config struct {
	URL       string                  `yaml:"url"`
	TokenFile string                  `yaml:"token_file"`
	Runners   map[string]RunnerConfig `yaml:"runners"`
}

// RunnerConfig defines a runner environment and the scale set it registers.
// The map key in Config.Runners is used as the scale set name.
// Exactly one of Docker, Libvirt, or Tart must be set.
type RunnerConfig struct {
	Labels     []string      `yaml:"labels"`
	MaxRunners int           `yaml:"max_runners,omitempty"`
	Docker     *DockerImage  `yaml:"docker,omitempty"`
	Libvirt    *LibvirtImage `yaml:"libvirt,omitempty"`
	Tart       *TartImage    `yaml:"tart,omitempty"`
}

// DockerImage configures a Docker-based runner.
type DockerImage struct {
	Image     string `yaml:"image"`
	RunnerCmd string `yaml:"runner_cmd"`
}

// LibvirtImage configures a libvirt/QEMU-based runner.
type LibvirtImage struct {
	Path      string `yaml:"path"`
	RunnerCmd string `yaml:"runner_cmd"`
	Socket    string `yaml:"socket"`
	CPUs      int    `yaml:"cpus"`
	MemoryMB  int    `yaml:"memory"`
}

// TartImage configures a Tart-based runner (macOS/Linux on Apple Silicon).
type TartImage struct {
	Image     string `yaml:"image"` // OCI image or local VM name
	RunnerCmd string `yaml:"runner_cmd"`
	CPUs      int    `yaml:"cpus"`
	MemoryMB  int    `yaml:"memory"`
}

// ProviderType returns which provisioner backend this runner uses.
func (r *RunnerConfig) ProviderType() string {
	switch {
	case r.Docker != nil:
		return "docker"
	case r.Libvirt != nil:
		return "libvirt"
	case r.Tart != nil:
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

	if len(cfg.Runners) == 0 {
		return nil, fmt.Errorf("no runners configured")
	}

	for name, runner := range cfg.Runners {
		if len(runner.Labels) == 0 {
			return nil, fmt.Errorf("runner %q: labels are required", name)
		}
		if runner.ProviderType() == "" {
			return nil, fmt.Errorf("runner %q: must specify docker, libvirt, or tart", name)
		}

		// Apply defaults for libvirt runners
		if runner.Libvirt != nil {
			if runner.Libvirt.CPUs == 0 {
				runner.Libvirt.CPUs = 4
			}
			if runner.Libvirt.MemoryMB == 0 {
				runner.Libvirt.MemoryMB = 8192
			}
		}

		// Apply defaults for tart runners
		if runner.Tart != nil {
			if runner.Tart.CPUs == 0 {
				runner.Tart.CPUs = 4
			}
			if runner.Tart.MemoryMB == 0 {
				runner.Tart.MemoryMB = 8192
			}
		}

		cfg.Runners[name] = runner
	}

	return &cfg, nil
}

// ResolveToken determines the GitHub token using the following precedence:
//  1. flagToken (--token CLI flag)
//  2. GITHUB_TOKEN environment variable
//  3. $CREDENTIALS_DIRECTORY/github-token (systemd-creds)
//  4. token_file from config
func ResolveToken(flagToken string, cfg *Config) (string, error) {
	// 1. CLI flag
	if flagToken != "" {
		return flagToken, nil
	}

	// 2. Environment variable
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}

	// 3. systemd credentials directory
	if credDir := os.Getenv("CREDENTIALS_DIRECTORY"); credDir != "" {
		path := filepath.Join(credDir, "github-token")
		if data, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(data)), nil
		}
	}

	// 4. token_file from config
	if cfg.TokenFile != "" {
		data, err := os.ReadFile(cfg.TokenFile)
		if err != nil {
			return "", fmt.Errorf("read token_file %q: %w", cfg.TokenFile, err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	return "", fmt.Errorf("no token provided (use --token, GITHUB_TOKEN env var, systemd-creds, or token_file in config)")
}

// ResolveURL determines the GitHub URL using the following precedence:
//  1. flagURL (--url CLI flag)
//  2. url from config
func ResolveURL(flagURL string, cfg *Config) (string, error) {
	if flagURL != "" {
		return flagURL, nil
	}
	if cfg.URL != "" {
		return cfg.URL, nil
	}
	return "", fmt.Errorf("no URL provided (use --url flag or url in config)")
}
