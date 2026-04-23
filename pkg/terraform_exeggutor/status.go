package terraform_exeggutor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform_provider"
)

// showState is the subset of terraform show -json output that
// ParseStatus needs to determine lifecycle status.
type showState struct {
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

// ParseStatus runs terraform show -json in the planet's terraform
// directory and determines the current LifecycleStatus by searching
// for a resource of the given type with a non-empty id.
//
// resourceType is the terraform resource type to search for:
//
//	aws:          "aws_instance"
//	digitalocean: "digitalocean_droplet"
//	ovh:          "openstack_compute_instance_v2"
//
// Returns:
//
//	LifecycleUninitialized   state file does not exist
//	LifecycleApplied         resource found with non-empty id
//	LifecycleUninitialized   state file exists but resource not found
//	LifecycleFailed          terraform show failed or JSON unparseable
func (e *Exeggutor) ParseStatus(ctx context.Context, resourceType string) (tfprovider.LifecycleStatus, error) {
	stateFile := filepath.Join(e.WorkDir, "terraform", "terraform.tfstate")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		return tfprovider.LifecycleUninitialized, nil
	}

	cmd := exec.CommandContext(ctx, "terraform", "show", "-json", "-no-color")
	cmd.Dir = filepath.Join(e.WorkDir, "terraform")
	cmd.Env = e.Env

	out, err := cmd.Output()
	if err != nil {
		return tfprovider.LifecycleFailed,
			fmt.Errorf("%s: terraform show planet %d: %w",
				e.Name, e.PlanetID, err)
	}

	var state showState
	if err := json.Unmarshal(out, &state); err != nil {
		return tfprovider.LifecycleFailed,
			fmt.Errorf("%s: parse terraform show planet %d: %w",
				e.Name, e.PlanetID, err)
	}

	for _, r := range state.Values.RootModule.Resources {
		if r.Type == resourceType && r.Values.ID != "" {
			return tfprovider.LifecycleApplied, nil
		}
	}

	return tfprovider.LifecycleUninitialized, nil
}

// StateFileExists returns true if the terraform state file exists in
// the planet's terraform directory. Used by providers that need to
// check initialization state before calling ParseStatus.
func (e *Exeggutor) StateFileExists() bool {
	stateFile := filepath.Join(e.WorkDir, "terraform", "terraform.tfstate")
	_, err := os.Stat(stateFile)
	return err == nil
}

// DotTerraformExists returns true if the .terraform directory exists
// in the planet's terraform directory. Used by InitOrApply to determine
// whether terraform init has already run.
func (e *Exeggutor) DotTerraformExists() bool {
	dot := filepath.Join(e.WorkDir, "terraform", ".terraform")
	_, err := os.Stat(dot)
	return err == nil
}
