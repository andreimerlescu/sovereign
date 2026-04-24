package aws

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	exeggutor "github.com/andreimerlescu/sovereign/pkg/terraform/exeggutor"
	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform/provider"
	"gopkg.in/yaml.v3"
)

// resourceType is the terraform resource type that ParseStatus searches
// for to determine whether an AWS EC2 instance has been provisioned.
const resourceType = "aws_instance"

// TerraformProviderAWS implements tfprovider.TerraformProvider for AWS.
// The struct is stateless with respect to individual planets. All
// per-planet data flows through PlanetSpec and HCLSpec arguments.
// Credentials are resolved once at construction and stored here.
type TerraformProviderAWS struct {
	creds Credentials
}

// New constructs a TerraformProviderAWS with resolved credentials.
// Returns an error if any required environment variable is missing.
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
// binary is available in PATH.
func (p *TerraformProviderAWS) Validate() error {
	if p.creds.AccessKey == "" || p.creds.AccessSecretKey == "" {
		return fmt.Errorf("aws: credentials not resolved -- call New()")
	}
	return exeggutor.Validate()
}

// RenderHCL renders all HCL files for the planet described by spec
// and writes them to spec.WorkDir/terraform/.
func (p *TerraformProviderAWS) RenderHCL(spec tfprovider.HCLSpec) error {
	return renderHCL(spec)
}

// Init runs terraform init in the planet's terraform directory.
func (p *TerraformProviderAWS) Init(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return exeggutor.New(p.Name(), spec.PlanetID, spec.WorkDir, p.env()).Init(ctx)
}

// InitOrApply runs terraform init if not already initialized then apply.
func (p *TerraformProviderAWS) InitOrApply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return exeggutor.New(p.Name(), spec.PlanetID, spec.WorkDir, p.env()).InitOrApply(ctx)
}

// Apply runs terraform apply in the planet's terraform directory.
func (p *TerraformProviderAWS) Apply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return exeggutor.New(p.Name(), spec.PlanetID, spec.WorkDir, p.env()).Apply(ctx)
}

// Destroy runs terraform destroy in the planet's terraform directory.
func (p *TerraformProviderAWS) Destroy(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return exeggutor.New(p.Name(), spec.PlanetID, spec.WorkDir, p.env()).Destroy(ctx)
}

// Output runs terraform output -json and parses instance_id,
// public_ipv4, private_ipv4 into a ProvisionOutput.
func (p *TerraformProviderAWS) Output(ctx context.Context, spec tfprovider.PlanetSpec) (*tfprovider.ProvisionOutput, error) {
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
func (p *TerraformProviderAWS) Status(ctx context.Context, spec tfprovider.PlanetSpec) (tfprovider.LifecycleStatus, error) {
	return exeggutor.New(p.Name(), spec.PlanetID, spec.WorkDir, p.env()).ParseStatus(ctx, resourceType)
}

// StateBackend returns the local terraform state file path for this planet.
func (p *TerraformProviderAWS) StateBackend(spec tfprovider.PlanetSpec) (string, error) {
	return filepath.Join(spec.WorkDir, "terraform", "terraform.tfstate"), nil
}

// LoadState reads the provider state file from the planet's work directory.
// Returns nil, nil if no state file exists.
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
// transition.
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

// UnloadState is a no-op. TerraformProviderAWS is stateless.
func (p *TerraformProviderAWS) UnloadState(_ tfprovider.PlanetSpec) error {
	return nil
}

// -- private helpers ---------------------------------------------------------

// env returns the environment for terraform subprocess calls.
// Credentials are passed as environment variables so they never
// appear in HCL files or backup directories.
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
