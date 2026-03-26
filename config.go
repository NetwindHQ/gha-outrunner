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

// ImageConfig defines a VM image and the labels it satisfies.
type ImageConfig struct {
	Labels    []string `yaml:"labels"`
	Path      string   `yaml:"path"`
	RunnerCmd string   `yaml:"runner_cmd"`
	CPUs      int      `yaml:"cpus"`
	MemoryMB  int      `yaml:"memory"`
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
		if cfg.Images[i].CPUs == 0 {
			cfg.Images[i].CPUs = 4
		}
		if cfg.Images[i].MemoryMB == 0 {
			cfg.Images[i].MemoryMB = 8192
		}
	}

	return &cfg, nil
}

// MatchImage finds the best matching image for the given job labels.
// The image whose labels are the largest subset of the job labels wins.
// Returns an error if no image matches.
func (c *Config) MatchImage(jobLabels []string) (*ImageConfig, error) {
	jobSet := make(map[string]bool, len(jobLabels))
	for _, l := range jobLabels {
		jobSet[l] = true
	}

	var best *ImageConfig
	bestCount := -1

	for i := range c.Images {
		img := &c.Images[i]

		// All image labels must be present in the job labels
		matched := 0
		for _, l := range img.Labels {
			if !jobSet[l] {
				matched = -1
				break
			}
			matched++
		}

		if matched > bestCount {
			bestCount = matched
			best = img
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no image matches labels %v", jobLabels)
	}

	return best, nil
}
