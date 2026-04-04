package outrunner

import (
	"os"
	"path/filepath"
	"strings"
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
	if !strings.Contains(err.Error(), "no runners configured") {
		t.Errorf("expected 'no runners configured' in error, got %q", err)
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

func TestLoadConfigMounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	content := `runners:
  docker-runner:
    labels: [linux]
    docker:
      image: runner:latest
      mounts:
        - source: /var/cache/vcpkg
          target: /opt/vcpkg-cache
        - source: /var/cache/cargo
          target: /opt/cargo-cache
          read_only: true
  tart-runner:
    labels: [macos]
    tart:
      image: base:latest
      mounts:
        - name: vcpkg
          source: /var/cache/vcpkg
        - name: cargo
          source: /var/cache/cargo
          read_only: true
  windows-runner:
    labels: [windows]
    libvirt:
      path: /images/win.qcow2
      mount:
        source: /var/cache/vcpkg
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Docker mounts
	docker := cfg.Runners["docker-runner"].Docker
	if len(docker.Mounts) != 2 {
		t.Fatalf("expected 2 docker mounts, got %d", len(docker.Mounts))
	}
	if docker.Mounts[0].Source != "/var/cache/vcpkg" || docker.Mounts[0].Target != "/opt/vcpkg-cache" || docker.Mounts[0].ReadOnly {
		t.Errorf("unexpected docker mount[0]: %+v", docker.Mounts[0])
	}
	if docker.Mounts[1].Source != "/var/cache/cargo" || docker.Mounts[1].Target != "/opt/cargo-cache" || !docker.Mounts[1].ReadOnly {
		t.Errorf("unexpected docker mount[1]: %+v", docker.Mounts[1])
	}

	// Tart mounts
	tart := cfg.Runners["tart-runner"].Tart
	if len(tart.Mounts) != 2 {
		t.Fatalf("expected 2 tart mounts, got %d", len(tart.Mounts))
	}
	if tart.Mounts[0].Name != "vcpkg" || tart.Mounts[0].Source != "/var/cache/vcpkg" || tart.Mounts[0].ReadOnly {
		t.Errorf("unexpected tart mount[0]: %+v", tart.Mounts[0])
	}
	if tart.Mounts[1].Name != "cargo" || tart.Mounts[1].Source != "/var/cache/cargo" || !tart.Mounts[1].ReadOnly {
		t.Errorf("unexpected tart mount[1]: %+v", tart.Mounts[1])
	}

	// Libvirt mount
	libvirt := cfg.Runners["windows-runner"].Libvirt
	if libvirt.Mount == nil {
		t.Fatal("expected libvirt mount")
	}
	if libvirt.Mount.Source != "/var/cache/vcpkg" {
		t.Errorf("unexpected libvirt mount source: %s", libvirt.Mount.Source)
	}
}

func TestLoadConfigNoMounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	content := `runners:
  linux:
    labels: [linux]
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

	if mounts := cfg.Runners["linux"].Docker.Mounts; len(mounts) != 0 {
		t.Errorf("expected no mounts, got %d", len(mounts))
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

func TestLoadConfigURLAndTokenFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	content := `url: https://github.com/myorg
token_file: /etc/outrunner/token
runners:
  linux:
    labels: [linux]
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

	if cfg.URL != "https://github.com/myorg" {
		t.Errorf("expected URL https://github.com/myorg, got %s", cfg.URL)
	}
	if cfg.TokenFile != "/etc/outrunner/token" {
		t.Errorf("expected token_file /etc/outrunner/token, got %s", cfg.TokenFile)
	}
}

func TestResolveToken(t *testing.T) {
	t.Run("flag takes precedence", func(t *testing.T) {
		token, err := ResolveToken("flag-token", &Config{TokenFile: "/nonexistent"})
		if err != nil {
			t.Fatal(err)
		}
		if token != "flag-token" {
			t.Errorf("expected flag-token, got %s", token)
		}
	})

	t.Run("env var", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "env-token")
		token, err := ResolveToken("", &Config{})
		if err != nil {
			t.Fatal(err)
		}
		if token != "env-token" {
			t.Errorf("expected env-token, got %s", token)
		}
	})

	t.Run("credentials directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "github-token"), []byte("  cred-token\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CREDENTIALS_DIRECTORY", dir)
		t.Setenv("GITHUB_TOKEN", "")
		token, err := ResolveToken("", &Config{})
		if err != nil {
			t.Fatal(err)
		}
		if token != "cred-token" {
			t.Errorf("expected cred-token, got %q", token)
		}
	})

	t.Run("token file", func(t *testing.T) {
		dir := t.TempDir()
		tokenPath := filepath.Join(dir, "token")
		if err := os.WriteFile(tokenPath, []byte("file-token\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("CREDENTIALS_DIRECTORY", "")
		token, err := ResolveToken("", &Config{TokenFile: tokenPath})
		if err != nil {
			t.Fatal(err)
		}
		if token != "file-token" {
			t.Errorf("expected file-token, got %q", token)
		}
	})

	t.Run("nothing configured", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("CREDENTIALS_DIRECTORY", "")
		_, err := ResolveToken("", &Config{})
		if err == nil {
			t.Fatal("expected error when no token source available")
		}
	})
}

func TestResolveURL(t *testing.T) {
	t.Run("flag takes precedence", func(t *testing.T) {
		url, err := ResolveURL("https://flag.com", &Config{URL: "https://config.com"})
		if err != nil {
			t.Fatal(err)
		}
		if url != "https://flag.com" {
			t.Errorf("expected https://flag.com, got %s", url)
		}
	})

	t.Run("config fallback", func(t *testing.T) {
		url, err := ResolveURL("", &Config{URL: "https://config.com"})
		if err != nil {
			t.Fatal(err)
		}
		if url != "https://config.com" {
			t.Errorf("expected https://config.com, got %s", url)
		}
	})

	t.Run("nothing configured", func(t *testing.T) {
		_, err := ResolveURL("", &Config{})
		if err == nil {
			t.Fatal("expected error when no URL source available")
		}
	})
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
