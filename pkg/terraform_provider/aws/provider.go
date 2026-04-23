package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform_provider"
)

// TerraformProviderAWS implements tfprovider.TerraformProvider for AWS.
// The struct is stateless with respect to individual planets -- all
// per-planet data flows through PlanetSpec and HCLSpec arguments.
// Credentials are resolved once at construction and stored here.
type TerraformProviderAWS struct {
	creds Credentials
}

// New constructs a TerraformProviderAWS with resolved credentials.
// Returns an error if required environment variables are missing.
func New() (*TerraformProviderAWS, error) {
	creds, err := Resolve()
	if err != nil {
		return nil, err
	}
	return &TerraformProviderAWS{creds: creds}, nil
}

// Name returns the provider identifier used in logs and state files.
func (p *TerraformProviderAWS) Name() string {
	return "aws"
}

// Validate checks that credentials are present and the terraform
// binary is available in PATH. Called once before any provisioning.
func (p *TerraformProviderAWS) Validate() error {
	if p.creds.AccessKey == "" || p.creds.AccessSecretKey == "" {
		return fmt.Errorf("aws: credentials not resolved -- call New()")
	}
	if _, err := exec.LookPath("terraform"); err != nil {
		return fmt.Errorf("aws: terraform binary not found in PATH")
	}
	return nil
}

// RenderHCL renders all HCL files for the planet described by spec
// and writes them to spec.WorkDir/terraform/. Credentials are injected
// into the spec before rendering so the provider block is complete.
func (p *TerraformProviderAWS) RenderHCL(spec tfprovider.HCLSpec) error {
	// inject credentials into the spec before passing to the renderer
	// HCLSpec carries AccessKey/SecretKey for the provider.tf template
	spec.AccessKey = p.creds.AccessKey
	spec.SecretKey = p.creds.AccessSecretKey
	return renderHCL(spec)
}

// Init runs terraform init in the planet's terraform directory.
func (p *TerraformProviderAWS) Init(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return p.tf(ctx, spec, "init", "-no-color")
}

// InitOrApply runs terraform init if the .terraform directory does not
// exist, then runs apply. Safe to call on an already-initialized planet.
func (p *TerraformProviderAWS) InitOrApply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	terraformDir := filepath.Join(spec.WorkDir, "terraform")
	dotTerraform := filepath.Join(terraformDir, ".terraform")
	if _, err := os.Stat(dotTerraform); os.IsNotExist(err) {
		if err := p.Init(ctx, spec); err != nil {
			return err
		}
	}
	return p.Apply(ctx, spec)
}

// Apply runs terraform apply in the planet's terraform directory.
func (p *TerraformProviderAWS) Apply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return p.tf(ctx, spec, "apply", "-auto-approve", "-no-color")
}

// Destroy runs terraform destroy in the planet's terraform directory.
func (p *TerraformProviderAWS) Destroy(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return p.tf(ctx, spec, "destroy", "-auto-approve", "-no-color")
}

// Output runs terraform output -json and parses the result into
// a ProvisionOutput. Returns an error if any expected output is missing.
func (p *TerraformProviderAWS) Output(ctx context.Context, spec tfprovider.PlanetSpec) (*tfprovider.ProvisionOutput, error) {
	terraformDir := filepath.Join(spec.WorkDir, "terraform")
	cmd := exec.CommandContext(ctx, "terraform", "output", "-json", "-no-color")
	cmd.Dir = terraformDir
	cmd.Env = p.env()

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("aws: terraform output planet %d: %w", spec.PlanetID, err)
	}

	// terraform output -json returns a map of output name ->
	// { "value": <value>, "type": <type> }
	var raw map[string]struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("aws: parse terraform output planet %d: %w", spec.PlanetID, err)
	}

	result := &tfprovider.ProvisionOutput{
		Region: spec.Region,
		Zone:   spec.Zone,
	}

	var missing []string
	if v, ok := raw["instance_id"]; ok {
		result.InstanceID = v.Value
		result.ProviderRef = v.Value
	} else {
		missing = append(missing, "instance_id")
	}
	if v, ok := raw["public_ipv4"]; ok {
		result.PublicIPv4 = v.Value
	} else {
		missing = append(missing, "public_ipv4")
	}
	if v, ok := raw["private_ipv4"]; ok {
		result.PrivateIPv4 = v.Value
	} else {
		missing = append(missing, "private_ipv4")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("aws: missing terraform outputs for planet %d: %v",
			spec.PlanetID, missing)
	}

	return result, nil
}

// Status runs terraform show -json and reports the current lifecycle
// status of the planet's managed instance resource.
func (p *TerraformProviderAWS) Status(ctx context.Context, spec tfprovider.PlanetSpec) (tfprovider.LifecycleStatus, error) {
	stateFile := filepath.Join(spec.WorkDir, "terraform", "terraform.tfstate")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		return tfprovider.LifecycleUninitialized, nil
	}

	terraformDir := filepath.Join(spec.WorkDir, "terraform")
	cmd := exec.CommandContext(ctx, "terraform", "show", "-json", "-no-color")
	cmd.Dir = terraformDir
	cmd.Env = p.env()

	out, err := cmd.Output()
	if err != nil {
		return tfprovider.LifecycleFailed, fmt.Errorf("aws: terraform show planet %d: %w",
			spec.PlanetID, err)
	}

	// look for the managed instance resource in the state
	var state struct {
		Values struct {
			RootModule struct {
				Resources []struct {
					Type   string `json:"type"`
					Values struct {
						ID string `json:"id"`
					} `json:"values"`
				} `json:"resources"`
			} `json:"root_module"`
		} `json:"values"`
	}

	if err := json.Unmarshal(out, &state); err != nil {
		return tfprovider.LifecycleFailed, fmt.Errorf("aws: parse terraform show planet %d: %w",
			spec.PlanetID, err)
	}

	for _, r := range state.Values.RootModule.Resources {
		if r.Type == "aws_instance" && r.Values.ID != "" {
			return tfprovider.LifecycleApplied, nil
		}
	}

	return tfprovider.LifecycleUninitialized, nil
}

// StateBackend returns the local terraform state file path for this planet.
func (p *TerraformProviderAWS) StateBackend(spec tfprovider.PlanetSpec) (string, error) {
	return filepath.Join(spec.WorkDir, "terraform", "terraform.tfstate"), nil
}

// LoadState reads the provider state file from the planet's work directory.
// Returns nil, nil if no state file exists -- not an error.
func (p *TerraformProviderAWS) LoadState(spec tfprovider.PlanetSpec) (*tfprovider.ProviderState, error) {
	path := p.stateFilePath(spec)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("aws: read state planet %d: %w", spec.PlanetID, err)
	}
	var state tfprovider.ProviderState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("aws: parse state planet %d: %w", spec.PlanetID, err)
	}
	return &state, nil
}

// SaveState writes the provider state to disk after every lifecycle
// transition. The state directory is created if it does not exist.
func (p *TerraformProviderAWS) SaveState(spec tfprovider.PlanetSpec, state *tfprovider.ProviderState) error {
	state.LastTransition = time.Now().UTC()
	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("aws: marshal state planet %d: %w", spec.PlanetID, err)
	}
	path := p.stateFilePath(spec)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("aws: create state dir planet %d: %w", spec.PlanetID, err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("aws: write state planet %d: %w", spec.PlanetID, err)
	}
	return nil
}

// UnloadState removes the in-memory reference only.
// The file on disk is not deleted -- it persists until the planet
// is destroyed and sovereign's cleanup phase removes it.
// Since TerraformProviderAWS is stateless, this is a no-op --
// there is no in-memory state to clear.
func (p *TerraformProviderAWS) UnloadState(_ tfprovider.PlanetSpec) error {
	return nil
}

// -- private helpers ---------------------------------------------------------

// tf runs a terraform subcommand in the planet's terraform directory.
func (p *TerraformProviderAWS) tf(ctx context.Context, spec tfprovider.PlanetSpec, args ...string) error {
	terraformDir := filepath.Join(spec.WorkDir, "terraform")
	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Dir = terraformDir
	cmd.Env = p.env()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aws: terraform %s planet %d: %w", args[0], spec.PlanetID, err)
	}
	return nil
}

// env returns the environment for terraform subprocess calls.
// Credentials are passed as environment variables rather than
// written into the HCL so they do not appear in terraform logs.
func (p *TerraformProviderAWS) env() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"AWS_ACCESS_KEY_ID=" + p.creds.AccessKey,
		"AWS_SECRET_ACCESS_KEY=" + p.creds.AccessSecretKey,
	}
}

// stateFilePath returns the path to the provider_state.yml file
// for a planet.
func (p *TerraformProviderAWS) stateFilePath(spec tfprovider.PlanetSpec) string {
	return filepath.Join(spec.WorkDir, "provider_state.yml")
}
