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
func (p *TerraformProv​​​​​​​​​​​​​​​​
