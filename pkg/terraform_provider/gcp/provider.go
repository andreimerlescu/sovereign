package gcp

import (
	"context"
	"fmt"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform_provider"
)

// TerraformProviderGCP implements tfprovider.TerraformProvider
// for Google Cloud Platform. Not yet implemented.
type TerraformProviderGCP struct{}

func (p *TerraformProviderGCP) Name() string { return "gcp" }

func (p *TerraformProviderGCP) Validate() error {
	return fmt.Errorf("gcp: not yet implemented")
}

func (p *TerraformProviderGCP) RenderHCL(spec tfprovider.HCLSpec) error {
	return fmt.Errorf("gcp: not yet implemented")
}

func (p *TerraformProviderGCP) Init(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("gcp: not yet implemented")
}

func (p *TerraformProviderGCP) InitOrApply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("gcp: not yet implemented")
}

func (p *TerraformProviderGCP) Apply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("gcp: not yet implemented")
}

func (p *TerraformProviderGCP) Destroy(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("gcp: not yet implemented")
}

func (p *TerraformProviderGCP) Output(ctx context.Context, spec tfprovider.PlanetSpec) (*tfprovider.ProvisionOutput, error) {
	return nil, fmt.Errorf("gcp: not yet implemented")
}

func (p *TerraformProviderGCP) Status(ctx context.Context, spec tfprovider.PlanetSpec) (tfprovider.LifecycleStatus, error) {
	return tfprovider.LifecycleUnknown, fmt.Errorf("gcp: not yet implemented")
}

func (p *TerraformProviderGCP) StateBackend(spec tfprovider.PlanetSpec) (string, error) {
	return "", fmt.Errorf("gcp: not yet implemented")
}

func (p *TerraformProviderGCP) LoadState(spec tfprovider.PlanetSpec) (*tfprovider.ProviderState, error) {
	return nil, fmt.Errorf("gcp: not yet implemented")
}

func (p *TerraformProviderGCP) SaveState(spec tfprovider.PlanetSpec, state *tfprovider.ProviderState) error {
	return fmt.Errorf("gcp: not yet implemented")
}

func (p *TerraformProviderGCP) UnloadState(spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("gcp: not yet implemented")
}
