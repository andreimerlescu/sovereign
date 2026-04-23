package do

import (
	"context"
	"fmt"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform_provider"
)

// TerraformProviderDigitalOcean implements tfprovider.TerraformProvider
// for DigitalOcean. Not yet implemented.
type TerraformProviderDigitalOcean struct{}

func (p *TerraformProviderDigitalOcean) Name() string { return "digitalocean" }

func (p *TerraformProviderDigitalOcean) Validate() error {
	return fmt.Errorf("digitalocean: not yet implemented")
}

func (p *TerraformProviderDigitalOcean) RenderHCL(spec tfprovider.HCLSpec) error {
	return fmt.Errorf("digitalocean: not yet implemented")
}

func (p *TerraformProviderDigitalOcean) Init(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("digitalocean: not yet implemented")
}

func (p *TerraformProviderDigitalOcean) InitOrApply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("digitalocean: not yet implemented")
}

func (p *TerraformProviderDigitalOcean) Apply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("digitalocean: not yet implemented")
}

func (p *TerraformProviderDigitalOcean) Destroy(ctx context.Context, spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("digitalocean: not yet implemented")
}

func (p *TerraformProviderDigitalOcean) Output(ctx context.Context, spec tfprovider.PlanetSpec) (*tfprovider.ProvisionOutput, error) {
	return nil, fmt.Errorf("digitalocean: not yet implemented")
}

func (p *TerraformProviderDigitalOcean) Status(ctx context.Context, spec tfprovider.PlanetSpec) (tfprovider.LifecycleStatus, error) {
	return tfprovider.LifecycleUnknown, fmt.Errorf("digitalocean: not yet implemented")
}

func (p *TerraformProviderDigitalOcean) StateBackend(spec tfprovider.PlanetSpec) (string, error) {
	return "", fmt.Errorf("digitalocean: not yet implemented")
}

func (p *TerraformProviderDigitalOcean) LoadState(spec tfprovider.PlanetSpec) (*tfprovider.ProviderState, error) {
	return nil, fmt.Errorf("digitalocean: not yet implemented")
}

func (p *TerraformProviderDigitalOcean) SaveState(spec tfprovider.PlanetSpec, state *tfprovider.ProviderState) error {
	return fmt.Errorf("digitalocean: not yet implemented")
}

func (p *TerraformProviderDigitalOcean) UnloadState(spec tfprovider.PlanetSpec) error {
	return fmt.Errorf("digitalocean: not yet implemented")
}
