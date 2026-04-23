package terraform_exeggutor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Exeggutor executes terraform subcommands for a single planet's
// working directory. It is constructed per operation and discarded
// after use -- it carries no per-run state between calls.
//
// All credentials and environment variables are passed at construction
// time via the Env field. Exeggutor never reads from os.Environ directly.
//
// The Name field is used as the error prefix in all returned errors so
// callers can identify which provider produced the error:
//
//	aws: terraform init planet 3: exit status 1
//	do:  terraform apply planet 1: exit status 1
type Exeggutor struct {
	// Name is the provider name used as the error message prefix.
	// Examples: aws, digitalocean, ovh
	Name string

	// PlanetID is the planet number used in error messages.
	PlanetID int

	// WorkDir is the planet's working directory. Terraform commands
	// are executed in WorkDir/terraform/.
	WorkDir string

	// Env is the complete environment for terraform subprocess calls.
	// Constructed by the provider's env() method. Must include PATH
	// and HOME in addition to provider-specific credential variables.
	Env []string
}

// New constructs an Exeggutor for a planet. name is the provider
// identifier used in error messages. env is the complete subprocess
// environment from the provider's env() method.
func New(name string, planetID int, workDir string, env []string) *Exeggutor {
	return &Exeggutor{
		Name:     name,
		PlanetID: planetID,
		WorkDir:  workDir,
		Env:      env,
	}
}

// terraformDir returns the terraform working directory for this planet.
func (e *Exeggutor) terraformDir() string {
	return filepath.Join(e.WorkDir, "terraform")
}

// Init runs terraform init -no-color in the planet's terraform directory.
func (e *Exeggutor) Init(ctx context.Context) error {
	return e.run(ctx, "init", "-no-color")
}

// Apply runs terraform apply -auto-approve -no-color.
func (e *Exeggutor) Apply(ctx context.Context) error {
	return e.run(ctx, "apply", "-auto-approve", "-no-color")
}

// Destroy runs terraform destroy -auto-approve -no-color.
func (e *Exeggutor) Destroy(ctx context.Context) error {
	return e.run(ctx, "destroy", "-auto-approve", "-no-color")
}

// InitOrApply runs terraform init if the .terraform directory does not
// exist, then runs apply. Safe to call after an interrupted apply --
// if init already ran it is skipped and apply runs directly.
func (e *Exeggutor) InitOrApply(ctx context.Context) error {
	dotTerraform := filepath.Join(e.terraformDir(), ".terraform")
	if _, err := os.Stat(dotTerraform); os.IsNotExist(err) {
		if err := e.Init(ctx); err != nil {
			return err
		}
	}
	return e.Apply(ctx)
}

// run executes a terraform subcommand in the planet's terraform
// directory. Stdout and stderr are forwarded to the sovereign process
// so the operator sees terraform output in real time.
func (e *Exeggutor) run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Dir = e.terraformDir()
	cmd.Env = e.Env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: terraform %s planet %d: %w",
			e.Name, args[0], e.PlanetID, err)
	}
	return nil
}

// Validate checks that the terraform binary is available in PATH.
// Called by provider.Validate() before any provisioning begins.
func Validate() error {
	if _, err := exec.LookPath("terraform"); err != nil {
		return fmt.Errorf("terraform binary not found in PATH")
	}
	return nil
}
