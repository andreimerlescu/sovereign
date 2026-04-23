package ovh

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform_provider"
	exeggutor "github.com/andreimerlescu/sovereign/pkg/terraform_exeggutor"
)

// resourceType is the terraform resource type that ParseStatus searches
// for to determine whether an OVH OpenStack compute instance has been
// provisioned.
const resourceType = "openstack_compute_instance_v2"

// TerraformProviderOVH implements tfprovider.TerraformProvider for
// OVHcloud Public Cloud using the OpenStack compute API.
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
	return exeggutor.Validate()
}

// RenderHCL renders all HCL files for the planet described by spec
// and writes them to spec.WorkDir/terraform/.
func (p *TerraformProviderOVH) RenderHCL(spec tfprovider.HCLSpec) error {
	return renderHCL(spec)
}

// Init runs terraform init in the planet's terraform directory.
func (p *TerraformProviderOVH) Init(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return exeggutor.New(p.Name(), spec.PlanetID, spec.WorkDir, p.env()).Init(ctx)
}

// InitOrApply runs terraform init if not already initialized then apply.
func (p *TerraformProviderOVH) InitOrApply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return exeggutor.New(p.Name(), spec.PlanetID, spec.WorkDir, p.env()).InitOrApply(ctx)
}

// Apply runs terraform apply in the planet's terraform directory.
func (p *TerraformProviderOVH) Apply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return exeggutor.New(p.Name(), spec.PlanetID, spec.WorkDir, p.env()).Apply(ctx)
}

// Destroy runs terraform destroy in the planet's terraform directory.
func (p *TerraformProviderOVH) Destroy(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return exeggutor.New(p.Name(), spec.PlanetID, spec.WorkDir, p.env()).Destroy(ctx)
}

// Output runs terraform output -json and parses instance_id,
// public_ipv4, private_ipv4 into a ProvisionOutput.
func (p *TerraformProviderOVH) Output(ctx context.Context, spec tfprovider.PlanetSpec) (*tfprovider.ProvisionOutput, error) {
	exe := exeggutor.New(p.Name(), spec.PlanetID, spec.WorkDir, p.env())
	result, err := exe.ParseOutput(ctx)
	if err != nil {
		return nil, err
	}
	return &tfprovider.ProvisionOutput{
		InstanceID:  result.InstanceID,
		PublicIPv4:  result.PublicIPv4,
		PrivateIPv4: result.PrivateIPv4,
		Region:      spec.Region,
		Zone:        spec.Zone,
		ProviderRef: result.InstanceID,
	}, nil
}

// Status checks the current terraform lifecycle status for the planet.
func (p *TerraformProviderOVH) Status(ctx context.Context, spec tfprovider.PlanetSpec) (tfprovider.LifecycleStatus, error) {
	return exeggutor.New(p.Name(), spec.PlanetID, spec.WorkDir, p.env()).ParseStatus(ctx, resourceType)
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

// UnloadState is a no-op. TerraformProviderOVH is stateless.
func (p *TerraformProviderOVH) UnloadState(_ tfprovider.PlanetSpec) error {
	return nil
}

// -- private helpers ---------------------------------------------------------

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
