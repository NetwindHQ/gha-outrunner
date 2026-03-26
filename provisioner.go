package outrunner

import "context"

// RunnerRequest contains everything a provisioner needs to start a runner.
type RunnerRequest struct {
	// Name is a unique identifier for this runner instance.
	Name string

	// JITConfig is the base64-encoded JIT configuration from GitHub.
	// Pass to: ./run.sh --jitconfig <JITConfig>
	JITConfig string

	// Labels from the workflow's runs-on field (e.g., ["self-hosted", "linux", "docker"]).
	Labels []string
}

// Provisioner creates and destroys ephemeral runner environments.
// Each implementation handles a different backend (Docker, libvirt, etc.).
type Provisioner interface {
	// Start provisions a new runner environment and starts the GitHub Actions
	// runner process inside it. The runner should use the JIT config to
	// register itself with GitHub and pick up the assigned job.
	//
	// Start must return after the runner process has been launched.
	// It does not need to wait for the job to complete.
	Start(ctx context.Context, req *RunnerRequest) error

	// Stop tears down the runner environment. Called after the job completes
	// or if the runner needs to be forcefully removed.
	Stop(ctx context.Context, name string) error

	// Close releases any resources held by the provisioner (e.g., Docker client).
	Close() error
}
