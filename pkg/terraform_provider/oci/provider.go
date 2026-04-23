package oci

import (
	"context"
	"fmt"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform_provider"
)

// TerraformProviderOCI implements tfprovider.TerraformProvider
// for Oracle Cloud Infrastructure. Not yet implemented.
type TerraformProviderOCI struct{}

func (p *TerraformProviderOCI) Name() string { return "oci" }

func (p *TerraformProviderOCI) Validate() error {
	return fmt.Errorf("oci: not yet implemented")
}

func (p *TerraformProviderOCI) RenderHCL(spec tfprovider.HCLSpec) error {
	return fmt.Errorf("oci: not yet implemented")
}

func (p *TerraformProviderOCI) Init(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("oci: not yet implemented")
}

func (p *TerraformProviderOCI) InitOrApply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("oci: not yet implemented")
}

func (p *TerraformProviderOCI) Apply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("oci: not yet implemented")
}

func (p *TerraformProviderOCI) Destroy(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("oci: not yet implemented")
}

func (p *TerraformProviderOCI) Output(ctx context.Context, spec tfprovider.PlanetSpec) (*tfprovider.ProvisionOutput, error) {
	return nil, fmt.Errorf("oci: not yet implemented")
}

func (p *TerraformProviderOCI) Status(ctx context.Context, spec tfprovider.PlanetSpec) (tfprovider.LifecycleStatus, error) {
	return tfprovider.LifecycleUnknown, fmt.Errorf("oci: not yet implemented")
}

func (p *TerraformProviderOCI) StateBackend(spec tfprovider.PlanetSpec) (string, error) {
	return "", fmt.Errorf("oci: not yet implemented")
}

func (p *TerraformProviderOCI) LoadState(spec tfprovider.PlanetSpec) (*tfprovider.ProviderState, error) {
	return nil, fmt.Errorf("oci: not yet implemented")
}

func (p *TerraformProviderOCI) SaveState(spec tfprovider.PlanetSpec, state *tfprovider.ProviderState) error {
	return fmt.Errorf("oci: not yet implemented")
}

func (p *TerraformProviderOCI) UnloadState(spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("oci: not yet implemented")
}
