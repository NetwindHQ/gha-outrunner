package outrunner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	content := `runners:
  linux:
    labels: [self-hosted, linux]
    docker:
      image: runner:latest
  windows:
    labels: [self-hosted, windows]
    libvirt:
      path: /tmp/win.qcow2
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if len(cfg.Runners) != 2 {
		t.Fatalf("expected 2 runners, got %d", len(cfg.Runners))
	}

	linux, ok := cfg.Runners["linux"]
	if !ok {
		t.Fatal("expected linux runner")
	}
	if linux.Docker == nil {
		t.Error("expected docker config")
	}
	if len(linux.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(linux.Labels))
	}

	windows, ok := cfg.Runners["windows"]
	if !ok {
		t.Fatal("expected windows runner")
	}
	if windows.Libvirt == nil {
		t.Error("expected libvirt config")
	}
	// Check defaults applied
	if windows.Libvirt.CPUs != 4 {
		t.Errorf("expected default CPUs 4, got %d", windows.Libvirt.CPUs)
	}
	if windows.Libvirt.MemoryMB != 8192 {
		t.Errorf("expected default memory 8192, got %d", windows.Libvirt.MemoryMB)
	}
}

func TestLoadConfigMissingLabels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	content := `runners:
  linux:
    docker:
      image: runner:latest
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing labels")
	}
}

func TestLoadConfigMissingProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	content := `runners:
  linux:
    labels: [linux]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing provider")
	}
}

func TestLoadConfigNoRunners(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	content := `runners: {}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for empty runners")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	content := `runners:
  tart-runner:
    labels: [macos]
    tart:
      image: base:latest
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	runner := cfg.Runners["tart-runner"]
	if runner.Tart.CPUs != 4 {
		t.Errorf("expected default CPUs 4, got %d", runner.Tart.CPUs)
	}
	if runner.Tart.MemoryMB != 8192 {
		t.Errorf("expected default memory 8192, got %d", runner.Tart.MemoryMB)
	}
}

func TestProviderType(t *testing.T) {
	tests := []struct {
		name   string
		runner RunnerConfig
		want   string
	}{
		{"docker", RunnerConfig{Docker: &DockerImage{Image: "x"}}, "docker"},
		{"libvirt", RunnerConfig{Libvirt: &LibvirtImage{Path: "x"}}, "libvirt"},
		{"tart", RunnerConfig{Tart: &TartImage{Image: "x"}}, "tart"},
		{"empty", RunnerConfig{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.runner.ProviderType()
			if got != tt.want {
				t.Errorf("ProviderType() = %q, want %q", got, tt.want)
			}
		})
	}
}
