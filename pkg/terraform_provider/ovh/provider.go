package ovh

import (
	"context"
	"fmt"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform_provider"
)

// TerraformProviderOVH implements tfprovider.TerraformProvider
// for OVHcloud. Not yet implemented.
type TerraformProviderOVH struct{}

func (p *TerraformProviderOVH) Name() string { return "ovh" }

func (p *TerraformProviderOVH) Validate() error {
	return fmt.Errorf("ovh: not yet implemented")
}

func (p *TerraformProviderOVH) RenderHCL(spec tfprovider.HCLSpec) error {
	return fmt.Errorf("ovh: not yet implemented")
}

func (p *TerraformProviderOVH) Init(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("ovh: not yet implemented")
}

func (p *TerraformProviderOVH) InitOrApply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("ovh: not yet implemented")
}

func (p *TerraformProviderOVH) Apply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("ovh: not yet implemented")
}

func (p *TerraformProviderOVH) Destroy(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("ovh: not yet implemented")
}

func (p *TerraformProviderOVH) Output(ctx context.Context, spec tfprovider.PlanetSpec) (*tfprovider.ProvisionOutput, error) {
	return nil, fmt.Errorf("ovh: not yet implemented")
}

func (p *TerraformProviderOVH) Status(ctx context.Context, spec tfprovider.PlanetSpec) (tfprovider.LifecycleStatus, error) {
	return tfprovider.LifecycleUnknown, fmt.Errorf("ovh: not yet implemented")
}

func (p *TerraformProviderOVH) StateBackend(spec tfprovider.PlanetSpec) (string, error) {
	return "", fmt.Errorf("ovh: not yet implemented")
}

func (p *TerraformProviderOVH) LoadState(spec tfprovider.PlanetSpec) (*tfprovider.ProviderState, error) {
	return nil, fmt.Errorf("ovh: not yet implemented")
}

func (p *TerraformProviderOVH) SaveState(spec tfprovider.PlanetSpec, state *tfprovider.ProviderState) error {
	return fmt.Errorf("ovh: not yet implemented")
}

func (p *TerraformProviderOVH) UnloadState(spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("ovh: not yet implemented")
}
