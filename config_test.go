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

func TestLoadConfigCustomValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	content := `runners:
  beefy:
    labels: [linux]
    libvirt:
      path: /images/linux.qcow2
      cpus: 16
      memory: 32768
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	runner := cfg.Runners["beefy"]
	if runner.Libvirt.CPUs != 16 {
		t.Errorf("expected CPUs 16, got %d", runner.Libvirt.CPUs)
	}
	if runner.Libvirt.MemoryMB != 32768 {
		t.Errorf("expected memory 32768, got %d", runner.Libvirt.MemoryMB)
	}
}

func TestLoadConfigMaxRunners(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	content := `runners:
  linux:
    labels: [linux]
    max_runners: 8
    docker:
      image: runner:latest
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Runners["linux"].MaxRunners != 8 {
		t.Errorf("expected max_runners 8, got %d", cfg.Runners["linux"].MaxRunners)
	}
}

func TestLoadConfigRunnerCmd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	content := `runners:
  linux:
    labels: [linux]
    docker:
      image: runner:latest
      runner_cmd: /custom/run.sh
  windows:
    labels: [windows]
    libvirt:
      path: /images/win.qcow2
      runner_cmd: 'C:\runner\run.cmd'
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Runners["linux"].Docker.RunnerCmd != "/custom/run.sh" {
		t.Errorf("expected /custom/run.sh, got %s", cfg.Runners["linux"].Docker.RunnerCmd)
	}
	if cfg.Runners["windows"].Libvirt.RunnerCmd != `C:\runner\run.cmd` {
		t.Errorf("expected C:\\runner\\run.cmd, got %s", cfg.Runners["windows"].Libvirt.RunnerCmd)
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
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
