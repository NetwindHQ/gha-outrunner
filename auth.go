package outrunner

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/actions/scaleset"
)

// AppConfig configures GitHub App authentication for one target. ClientID is
// the App's Client ID (the App ID is also accepted by GitHub), InstallationID
// is the installation ID, and PrivateKeyFile names the PEM private-key file.
type AppConfig struct {
	ClientID       string `yaml:"client_id"`
	InstallationID int64  `yaml:"installation_id"`
	PrivateKeyFile string `yaml:"private_key_file"`

	privateKey     string
	privateKeyErr  error
	privateKeyRead bool
	privateKeyMu   sync.Mutex
}

func validateAppConfig(app *AppConfig) error {
	if strings.TrimSpace(app.ClientID) == "" {
		return fmt.Errorf("client_id is required")
	}
	if app.InstallationID <= 0 {
		return fmt.Errorf("installation_id must be greater than zero")
	}
	if strings.TrimSpace(app.PrivateKeyFile) == "" {
		return fmt.Errorf("private_key_file is required")
	}
	return nil
}

func (app *AppConfig) loadPrivateKey() (string, error) {
	app.privateKeyMu.Lock()
	defer app.privateKeyMu.Unlock()
	if !app.privateKeyRead {
		data, err := os.ReadFile(app.PrivateKeyFile)
		if err != nil {
			app.privateKeyErr = fmt.Errorf("read app private key file: %w", err)
		} else if !validRSAPrivateKey(data) {
			app.privateKeyErr = fmt.Errorf("app private key file does not contain a valid RSA private key")
		} else {
			app.privateKey = string(data)
		}
		app.privateKeyRead = true
	}
	if app.privateKeyErr != nil {
		return "", app.privateKeyErr
	}
	return app.privateKey, nil
}

func validRSAPrivateKey(data []byte) bool {
	block, _ := pem.Decode(data)
	if block == nil {
		return false
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key != nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return false
	}
	_, ok := key.(*rsa.PrivateKey)
	return ok
}

// ResolvedAuth contains the effective credential for a runner. Identity is an
// opaque, non-secret cache key; it must not be logged or treated as a credential.
type ResolvedAuth struct {
	App      *scaleset.GitHubAppAuth
	Token    string
	Identity string
}

// ResolveRunnerAuth applies credential precedence for a runner. A runner-level
// token_file is an explicit PAT override; otherwise a runner App overrides the
// global App, and a global App takes precedence over ambient/global PAT sources.
func ResolveRunnerAuth(globalToken string, cfg *Config, runner *RunnerConfig) (ResolvedAuth, error) {
	if cfg == nil {
		return ResolvedAuth{}, fmt.Errorf("resolve runner authentication: config is required")
	}
	if runner == nil {
		return ResolvedAuth{}, fmt.Errorf("resolve runner authentication: runner is required")
	}
	if runner.App != nil && runner.TokenFile != "" {
		return ResolvedAuth{}, fmt.Errorf("app and token_file cannot both be set for a runner")
	}

	if runner.App != nil {
		return resolveAppAuth(runner.App)
	}
	if runner.TokenFile == "" && cfg.App != nil {
		return resolveAppAuth(cfg.App)
	}

	token, err := ResolveRunnerToken(globalToken, cfg, runner)
	if err != nil {
		return ResolvedAuth{}, err
	}
	return ResolvedAuth{
		Token:    token,
		Identity: credentialIdentity("pat", []byte(token)),
	}, nil
}

func resolveAppAuth(app *AppConfig) (ResolvedAuth, error) {
	if err := validateAppConfig(app); err != nil {
		return ResolvedAuth{}, fmt.Errorf("invalid GitHub App configuration: %w", err)
	}
	privateKey, err := app.loadPrivateKey()
	if err != nil {
		return ResolvedAuth{}, err
	}
	resolved := &scaleset.GitHubAppAuth{
		ClientID:       app.ClientID,
		InstallationID: app.InstallationID,
		PrivateKey:     privateKey,
	}
	identityMaterial := []byte(fmt.Sprintf("%s\x00%d\x00%s", app.ClientID, app.InstallationID, privateKey))
	return ResolvedAuth{
		App:      resolved,
		Identity: credentialIdentity("github-app", identityMaterial),
	}, nil
}

func credentialIdentity(kind string, material []byte) string {
	h := sha256.New()
	_, _ = h.Write([]byte(kind))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(material)
	return kind + ":" + hex.EncodeToString(h.Sum(nil))
}

// NewScaleSetClient constructs a scaleset client using the selected credential.
// An App failure is returned directly; it never falls back to a PAT.
func NewScaleSetClient(githubConfigURL string, auth ResolvedAuth, options ...scaleset.HTTPOption) (*scaleset.Client, error) {
	if auth.App != nil {
		if err := auth.App.Validate(); err != nil {
			return nil, fmt.Errorf("create GitHub App scaleset client: invalid credentials: %w", err)
		}
		if err := rejectEnterpriseAppURL(githubConfigURL); err != nil {
			return nil, err
		}
		return scaleset.NewClientWithGitHubApp(scaleset.ClientWithGitHubAppConfig{
			GitHubConfigURL: githubConfigURL,
			GitHubAppAuth:   *auth.App,
		}, options...)
	}
	if auth.Token == "" {
		return nil, fmt.Errorf("create scaleset client: no authentication credential configured")
	}
	return scaleset.NewClientWithPersonalAccessToken(scaleset.NewClientWithPersonalAccessTokenConfig{
		GitHubConfigURL:     githubConfigURL,
		PersonalAccessToken: auth.Token,
	}, options...)
}

func rejectEnterpriseAppURL(githubConfigURL string) error {
	u, err := url.Parse(githubConfigURL)
	if err != nil {
		return fmt.Errorf("GitHub App authentication: invalid GitHub URL")
	}
	path := strings.Trim(u.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 2 && strings.EqualFold(parts[0], "enterprises") {
		return fmt.Errorf("GitHub App authentication does not support enterprise-level runner scopes")
	}
	return nil
}
