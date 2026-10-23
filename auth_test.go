package outrunner

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/actions/scaleset"
)

func writeTestAppKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "app.pem")
	data := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigGitHubAppValidation(t *testing.T) {
	keyPath := writeTestAppKey(t)
	tests := []struct {
		name    string
		app     string
		wantErr string
	}{
		{
			name: "valid top-level app",
			app:  "client_id: client-id\ninstallation_id: 123\nprivate_key_file: " + keyPath,
		},
		{
			name:    "missing client id",
			app:     "installation_id: 123\nprivate_key_file: " + keyPath,
			wantErr: "client_id is required",
		},
		{
			name:    "nonpositive installation id",
			app:     "client_id: client-id\ninstallation_id: 0\nprivate_key_file: " + keyPath,
			wantErr: "installation_id must be greater than zero",
		},
		{
			name:    "blank private key path",
			app:     "client_id: client-id\ninstallation_id: 123\nprivate_key_file: '   '",
			wantErr: "private_key_file is required",
		},
		{
			name:    "runner app and token file conflict",
			app:     "",
			wantErr: "app and token_file cannot both be set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appSection := ""
			if tt.name != "runner app and token file conflict" {
				appSection = "app:\n  " + strings.ReplaceAll(tt.app, "\n", "\n  ") + "\n"
			}
			contents := "url: https://github.com/org/repo\n" + appSection + "runners:\n  linux:\n    labels: [linux]\n    docker:\n      image: runner:latest\n"
			if tt.name == "runner app and token file conflict" {
				contents = "url: https://github.com/org/repo\nrunners:\n  linux:\n    token_file: /tmp/token\n    app:\n      client_id: client-id\n      installation_id: 123\n      private_key_file: " + keyPath + "\n    labels: [linux]\n    docker:\n      image: runner:latest\n"
			}
			path := filepath.Join(t.TempDir(), "config.yml")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfig(path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("LoadConfig error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if cfg.App == nil || cfg.App.ClientID != "client-id" || cfg.App.InstallationID != 123 || cfg.App.PrivateKeyFile != keyPath {
				t.Fatalf("loaded App config = %#v", cfg.App)
			}
		})
	}
}

func TestLoadConfigInvalidRunnerApp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	contents := "url: https://github.com/org/repo\nrunners:\n  linux:\n    labels: [linux]\n    app:\n      client_id: client-id\n      installation_id: -1\n      private_key_file: /tmp/key.pem\n    docker:\n      image: runner:latest\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "runner \"linux\" app: installation_id must be greater than zero") {
		t.Fatalf("LoadConfig error = %v, want runner App validation error", err)
	}
}

func TestLoadConfigInvalidAppPrivateKey(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(keyPath, []byte("not a PEM key"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yml")
	contents := "app:\n  client_id: client-id\n  installation_id: 123\n  private_key_file: " + keyPath + "\nrunners:\n  linux:\n    labels: [linux]\n    docker:\n      image: runner:latest\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig validates app fields without loading key: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", "ambient-pat")
	runner := cfg.Runners["linux"]
	_, err = ResolveRunnerAuth("flag-pat", cfg, &runner)
	if err == nil || !strings.Contains(err.Error(), "does not contain a valid RSA private key") {
		t.Fatalf("ResolveRunnerAuth error = %v, want invalid private key", err)
	}
	if strings.Contains(err.Error(), "not a PEM key") || strings.Contains(err.Error(), "ambient-pat") || strings.Contains(err.Error(), "flag-pat") {
		t.Fatalf("error disclosed key contents or PAT: %v", err)
	}
}

func TestResolveRunnerAuthPrivateKeyReadErrorDoesNotFallBack(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.pem")
	app := &AppConfig{ClientID: "client-id", InstallationID: 123, PrivateKeyFile: missingPath}
	t.Setenv("GITHUB_TOKEN", "ambient-pat")
	_, err := ResolveRunnerAuth("flag-pat", &Config{App: app}, &RunnerConfig{})
	if err == nil || !strings.Contains(err.Error(), "read app private key file") {
		t.Fatalf("ResolveRunnerAuth error = %v, want key read error", err)
	}
	if strings.Contains(err.Error(), "ambient-pat") || strings.Contains(err.Error(), "flag-pat") {
		t.Fatalf("key read error disclosed PAT: %v", err)
	}
}

func TestResolveRunnerAuthPrecedenceAndIdentity(t *testing.T) {
	globalKey := writeTestAppKey(t)
	runnerKey := writeTestAppKey(t)
	globalApp := &AppConfig{ClientID: "global-client", InstallationID: 7, PrivateKeyFile: globalKey}
	cfg := &Config{App: globalApp, TokenFile: filepath.Join(t.TempDir(), "not-read")}
	globalRunner := &RunnerConfig{}

	t.Setenv("GITHUB_TOKEN", "ambient-pat")
	globalAuth, err := ResolveRunnerAuth("flag-pat", cfg, globalRunner)
	if err != nil {
		t.Fatal(err)
	}
	if globalAuth.App == nil || globalAuth.App.ClientID != "global-client" || globalAuth.Token != "" {
		t.Fatalf("global App did not take precedence over PAT: %#v", globalAuth)
	}
	if globalAuth.Identity == "" || strings.Contains(globalAuth.Identity, "global-client") || strings.Contains(globalAuth.Identity, "PRIVATE KEY") {
		t.Fatalf("identity is missing or contains credential material: %q", globalAuth.Identity)
	}

	runnerApp := &AppConfig{ClientID: "runner-client", InstallationID: 9, PrivateKeyFile: runnerKey}
	runnerAuth, err := ResolveRunnerAuth("flag-pat", cfg, &RunnerConfig{App: runnerApp})
	if err != nil {
		t.Fatal(err)
	}
	if runnerAuth.App == nil || runnerAuth.App.ClientID != "runner-client" || runnerAuth.Identity == globalAuth.Identity {
		t.Fatalf("runner App did not override global App: %#v", runnerAuth)
	}

	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("runner-pat\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	patAuth, err := ResolveRunnerAuth("flag-pat", cfg, &RunnerConfig{TokenFile: tokenPath})
	if err != nil {
		t.Fatal(err)
	}
	if patAuth.App != nil || patAuth.Token != "runner-pat" || patAuth.Identity == globalAuth.Identity {
		t.Fatalf("runner token_file did not opt into PAT: %#v", patAuth)
	}

	ambientConfig := &Config{}
	ambient, err := ResolveRunnerAuth("flag-pat", ambientConfig, globalRunner)
	if err != nil {
		t.Fatal(err)
	}
	if ambient.Token != "flag-pat" || ambient.App != nil || ambient.Identity == patAuth.Identity {
		t.Fatalf("PAT resolution or identity incorrect: %#v", ambient)
	}

	// A runner-level App takes precedence over the configured global App.
	// The resolved identity is independent of key-file path and changes with its contents.
	copyKey := filepath.Join(t.TempDir(), "same-key.pem")
	keyData, err := os.ReadFile(globalKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyKey, keyData, 0o600); err != nil {
		t.Fatal(err)
	}
	sameAppAuth, err := ResolveRunnerAuth("", &Config{App: &AppConfig{ClientID: "global-client", InstallationID: 7, PrivateKeyFile: copyKey}}, globalRunner)
	if err != nil {
		t.Fatal(err)
	}
	if sameAppAuth.Identity != globalAuth.Identity {
		t.Fatalf("same effective App credentials have different identities: %q vs %q", sameAppAuth.Identity, globalAuth.Identity)
	}
}

func TestResolveRunnerAuthIdentityUsesCredentialContents(t *testing.T) {
	keyA := writeTestAppKey(t)
	keyB := writeTestAppKey(t)
	appA := &AppConfig{ClientID: "client-id", InstallationID: 42, PrivateKeyFile: keyA}
	appB := &AppConfig{ClientID: "client-id", InstallationID: 42, PrivateKeyFile: keyB}
	authA, err := ResolveRunnerAuth("", &Config{App: appA}, &RunnerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	authB, err := ResolveRunnerAuth("", &Config{App: appB}, &RunnerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if authA.Identity == authB.Identity {
		t.Fatal("different effective App keys received the same identity")
	}

	t.Setenv("GITHUB_TOKEN", "first-token")
	first, err := ResolveRunnerAuth("", &Config{}, &RunnerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_TOKEN", "second-token")
	second, err := ResolveRunnerAuth("", &Config{}, &RunnerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if first.Identity == second.Identity || strings.Contains(first.Identity, first.Token) {
		t.Fatal("PAT credential identity does not distinguish or protect token material")
	}
}

func TestResolveRunnerAuthRejectsAmbiguousAndMissingCredentials(t *testing.T) {
	app := &AppConfig{ClientID: "client", InstallationID: 1, PrivateKeyFile: writeTestAppKey(t)}
	_, err := ResolveRunnerAuth("", &Config{}, &RunnerConfig{App: app, TokenFile: "/tmp/token"})
	if err == nil || !strings.Contains(err.Error(), "app and token_file cannot both be set") {
		t.Fatalf("ambiguous auth error = %v", err)
	}

	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("CREDENTIALS_DIRECTORY", "")
	_, err = ResolveRunnerAuth("", &Config{}, &RunnerConfig{})
	if err == nil || !strings.Contains(err.Error(), "no token provided") {
		t.Fatalf("missing auth error = %v", err)
	}
}

func TestNewScaleSetClientRejectsEnterpriseAppScope(t *testing.T) {
	_, err := NewScaleSetClient("https://github.com/enterprises/example", ResolvedAuth{
		App: &scaleset.GitHubAppAuth{ClientID: "client", InstallationID: 1, PrivateKey: "never logged"},
	})
	if err == nil || !strings.Contains(err.Error(), "does not support enterprise-level runner scopes") {
		t.Fatalf("enterprise App error = %v", err)
	}
	if strings.Contains(err.Error(), "never logged") {
		t.Fatalf("error disclosed private key: %v", err)
	}
}
