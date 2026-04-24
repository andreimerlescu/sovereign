package exeggutor

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
)

// Result holds the parsed output from terraform output -json.
// The three fields -- InstanceID, PublicIPv4, PrivateIPv4 -- are
// required by the TerraformProvider interface contract. A missing
// output field causes ParseOutput to return an error listing the
// missing names.
//
// Result is intentionally independent of tfprovider.ProvisionOutput.
// The provider converts Result to ProvisionOutput after calling
// ParseOutput so that exeggutor does not depend on the provider
// interface package.
type Result struct {
	InstanceID  string
	PublicIPv4  string
	PrivateIPv4 string
}

// outputValue is the per-key structure returned by terraform output -json.
// Each key maps to an object with a value field and a type field.
// Only the value field is used.
type outputValue struct {
	Value interface{} `json:"value"`
}

// ParseOutput runs terraform output -json in the planet's terraform
// directory and parses the three required output values.
//
// Returns an error if:
//   - terraform output fails or times out
//   - the JSON cannot be decoded
//   - any of instance_id, public_ipv4, private_ipv4 are missing
func (e *Exeggutor) ParseOutput(ctx context.Context) (*Result, error) {
	cmd := exec.CommandContext(ctx, "terraform", "output", "-json", "-no-color")
	cmd.Dir = filepath.Join(e.WorkDir, "terraform")
	cmd.Env = e.Env

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: terraform output planet %d: %w",
			e.Name, e.PlanetID, err)
	}

	var raw map[string]outputValue
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("%s: parse terraform output planet %d: %w",
			e.Name, e.PlanetID, err)
	}

	result := &Result{}
	var missing []string

	if v, ok := raw["instance_id"]; ok {
		if s, ok := v.Value.(string); ok {
			result.InstanceID = s
		}
	} else {
		missing = append(missing, "instance_id")
	}

	if v, ok := raw["public_ipv4"]; ok {
		if s, ok := v.Value.(string); ok {
			result.PublicIPv4 = s
		}
	} else {
		missing = append(missing, "public_ipv4")
	}

	if v, ok := raw["private_ipv4"]; ok {
		if s, ok := v.Value.(string); ok {
			result.PrivateIPv4 = s
		}
	} else {
		missing = append(missing, "private_ipv4")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("%s: missing terraform outputs for planet %d: %v",
			e.Name, e.PlanetID, missing)
	}

	return result, nil
}
