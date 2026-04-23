package ovh

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

// TerraformProviderOVH implements tfprovider.TerraformProvider for
// OVH Public Cloud using the OpenStack compute API.
//
// OVH exposes OpenStack-compatible APIs for its public cloud compute
// resources. Terraform uses two providers together -- ovh/ovh for
// OVH-specific resources and openstack for compute instances.
//
// The struct is stateless with respect to individual planets. All
// per-planet data flows through PlanetSpec and HCLSpec arguments.
// Credentials are resolved once at construction and stored here.
type TerraformProviderOVH struct {
	creds Credentials
}

// New constructs a TerraformProviderOVH with resolved credentials.
// Returns an error if any required environment variable is missing.
func New() (*TerraformProviderOVH, error) {
	creds, err := Resolve()
	if err != nil {
		return nil, err
	}
	return &TerraformProviderOVH{creds: creds}, nil
}

// Name returns the provider identifier used in logs and state files.
func (p *TerraformProviderOVH) Name() string {
	return "ovh"
}

// Validate checks that credentials are present and the terraform
// binary is available in PATH.
func (p *TerraformProviderOVH) Validate() error {
	if p.creds.ApplicationKey == "" {
		return fmt.Errorf("ovh: credentials not resolved -- call New()")
	}
	if _, err := exec.LookPath("terraform"); err != nil {
		return fmt.Errorf("ovh: terraform binary not found in PATH")
	}
	return nil
}

// RenderHCL renders all HCL files for the planet described by spec
// and writes them to spec.WorkDir/terraform/.
func (p *TerraformProviderOVH) RenderHCL(spec tfprovider.HCLSpec) error {
	return renderHCL(spec)
}

// Init runs terraform init in the planet's terraform directory.
func (p *TerraformProviderOVH) Init(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return p.tf(ctx, spec, "init", "-no-color")
}

// InitOrApply runs terraform init if the .terraform directory does not
// exist, then runs apply. Safe to call on an already-initialized planet.
func (p *TerraformProviderOVH) InitOrApply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	dotTerraform := filepath.Join(spec.WorkDir, "terraform", ".terraform")
	if _, err := os.Stat(dotTerraform); os.IsNotExist(err) {
		if err := p.Init(ctx, spec); err != nil {
			return err
		}
	}
	return p.Apply(ctx, spec)
}

// Apply runs terraform apply in the planet's terraform directory.
func (p *TerraformProviderOVH) Apply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return p.tf(ctx, spec, "apply", "-auto-approve", "-no-color")
}

// Destroy runs terraform destroy in the planet's terraform directory.
func (p *TerraformProviderOVH) Destroy(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return p.tf(ctx, spec, "destroy", "-auto-approve", "-no-color")
}

// Output runs terraform output -json and parses the result into
// a ProvisionOutput. Returns an error if any expected output is missing.
func (p *TerraformProviderOVH) Output(ctx context.Context, spec tfprovider.PlanetSpec) (*tfprovider.ProvisionOutput, error) {
	terraformDir := filepath.Join(spec.WorkDir, "terraform")
	cmd := exec.CommandContext(ctx, "terraform", "output", "-json", "-no-color")
	cmd.Dir = terraformDir
	cmd.Env = p.env()

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ovh: terraform output planet %d: %w",
			spec.PlanetID, err)
	}

	var raw map[string]struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("ovh: parse terraform output planet %d: %w",
			spec.PlanetID, err)
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
		return nil, fmt.Errorf("ovh: missing terraform outputs for planet %d: %v",
			spec.PlanetID, missing)
	}

	return result, nil
}

// Status runs terraform show -json and reports the current lifecycle
// status of the planet's managed instance resource.
func (p *TerraformProviderOVH) Status(ctx context.Context, spec tfprovider.PlanetSpec) (tfprovider.LifecycleStatus, error) {
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
		return tfprovider.LifecycleFailed,
			fmt.Errorf("ovh: terraform show planet %d: %w", spec.PlanetID, err)
	}

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
		return tfprovider.LifecycleFailed,
			fmt.Errorf("ovh: parse terraform show planet %d: %w", spec.PlanetID, err)
	}

	for _, r := range state.Values.RootModule.Resources {
		if r.Type == "openstack_compute_instance_v2" && r.Values.ID != "" {
			return tfprovider.LifecycleApplied, nil
		}
	}

	return tfprovider.LifecycleUninitialized, nil
}

// StateBackend returns the local terraform state file path for this planet.
func (p *TerraformProviderOVH) StateBackend(spec tfprovider.PlanetSpec) (string, error) {
	return filepath.Join(spec.WorkDir, "terraform", "terraform.tfstate"), nil
}

// LoadState reads the provider state file from the planet's work directory.
// Returns nil, nil if no state file exists.
func (p *TerraformProviderOVH) LoadState(spec tfprovider.PlanetSpec) (*tfprovider.ProviderState, error) {
	path := p.stateFilePath(spec)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ovh: read state planet %d: %w", spec.PlanetID, err)
	}
	var state tfprovider.ProviderState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("ovh: parse state planet %d: %w", spec.PlanetID, err)
	}
	return &state, nil
}

// SaveState writes the provider state to disk after every lifecycle
// transition.
func (p *TerraformProviderOVH) SaveState(spec tfprovider.PlanetSpec, state *tfprovider.ProviderState) error {
	state.LastTransition = time.Now().UTC()
	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("ovh: marshal state planet %d: %w", spec.PlanetID, err)
	}
	path := p.stateFilePath(spec)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("ovh: create state dir planet %d: %w", spec.PlanetID, err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("ovh: write state planet %d: %w", spec.PlanetID, err)
	}
	return nil
}

// UnloadState is a no-op. TerraformProviderOVH is stateless -- there
// is no in-memory per-planet state to clear.
func (p *TerraformProviderOVH) UnloadState(_ tfprovider.PlanetSpec) error {
	return nil
}

// -- private helpers ---------------------------------------------------------

// tf runs a terraform subcommand in the planet's terraform directory.
func (p *TerraformProviderOVH) tf(ctx context.Context, spec tfprovider.PlanetSpec, args ...string) error {
	terraformDir := filepath.Join(spec.WorkDir, "terraform")
	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Dir = terraformDir
	cmd.Env = p.env()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ovh: terraform %s planet %d: %w",
			args[0], spec.PlanetID, err)
	}
	return nil
}

// env returns the environment for terraform subprocess calls.
// All credentials are passed as environment variables so they
// never appear in HCL files or backup directories.
func (p *TerraformProviderOVH) env() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),

		// OVH API credentials
		"OVH_ENDPOINT=" + p.creds.Endpoint,
		"OVH_APPLICATION_KEY=" + p.creds.ApplicationKey,
		"OVH_APPLICATION_SECRET=" + p.creds.ApplicationSecret,
		"OVH_CONSUMER_KEY=" + p.creds.ConsumerKey,

		// OpenStack credentials
		"OS_AUTH_URL=" + p.creds.AuthURL,
		"OS_TENANT_ID=" + p.creds.TenantID,
		"OS_TENANT_NAME=" + p.creds.TenantName,
		"OS_USERNAME=" + p.creds.Username,
		"OS_PASSWORD=" + p.creds.Password,
		"OS_REGION_NAME=" + p.creds.RegionName,
	}
}

// stateFilePath returns the path to the provider_state.yml file
// for a planet.
func (p *TerraformProviderOVH) stateFilePath(spec tfprovider.PlanetSpec) string {
	return filepath.Join(spec.WorkDir, "provider_state.yml")
}
