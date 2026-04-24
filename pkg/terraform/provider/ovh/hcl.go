package ovh

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform/provider"
)

// hclFile represents a single rendered HCL file to be written to disk.
type hclFile struct {
	name    string
	content string
}

// renderHCL renders all HCL files required for a sovereign OVH planet
// and writes them to spec.WorkDir/terraform/. After writing, it records
// a backup of the rendered files in spec.WorkDir/backups/<timestamp>/.
func renderHCL(spec tfprovider.HCLSpec) error {
	terraformDir := filepath.Join(spec.WorkDir, "terraform")
	if err := os.MkdirAll(terraformDir, 0700); err != nil {
		return fmt.Errorf("ovh: create terraform dir: %w", err)
	}

	files, err := renderAll(spec)
	if err != nil {
		return err
	}

	for _, f := range files {
		path := filepath.Join(terraformDir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0600); err != nil {
			return fmt.Errorf("ovh: write %s: %w", f.name, err)
		}
	}

	return backupHCL(spec, terraformDir, files)
}

// renderAll renders each HCL file from its template and returns the
// results. All files are rendered before any are written so a template
// error does not produce a partially written terraform directory.
func renderAll(spec tfprovider.HCLSpec) ([]hclFile, error) {
	templates := []struct {
		name string
		tmpl string
	}{
		{"provider.tf", providerTF},
		{"data.tf", dataTF},
		{"resources_network.tf", resourcesNetworkTF},
		{"resources_compute.tf", resourcesComputeTF},
		{"output.tf", outputTF},
	}

	files := make([]hclFile, 0, len(templates))
	for _, t := range templates {
		content, err := render(t.tmpl, spec)
		if err != nil {
			return nil, fmt.Errorf("ovh: render %s: %w", t.name, err)
		}
		files = append(files, hclFile{name: t.name, content: content})
	}
	return files, nil
}

// render executes a single HCL template with the provided spec.
func render(tmpl string, spec tfprovider.HCLSpec) (string, error) {
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, spec); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// backupHCL copies the rendered HCL files to a timestamped backup
// directory inside spec.WorkDir/backups/.
func backupHCL(spec tfprovider.HCLSpec, terraformDir string, files []hclFile) error {
	ts := time.Now().UTC().Format("2006-01-02T15-04-05")
	backupDir := filepath.Join(spec.WorkDir, "backups", ts)
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return fmt.Errorf("ovh: create backup dir: %w", err)
	}
	for _, f := range files {
		src := filepath.Join(terraformDir, f.name)
		dst := filepath.Join(backupDir, f.name)
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("ovh: backup read %s: %w", f.name, err)
		}
		if err := os.WriteFile(dst, data, 0600); err != nil {
			return fmt.Errorf("ovh: backup write %s: %w", f.name, err)
		}
	}
	return nil
}

// -- HCL Templates -----------------------------------------------------------
//
// Credentials are never rendered into HCL files. OVH and OpenStack
// credentials are passed to the terraform subprocess via environment
// variables in env(). Both the ovh/ovh and openstack terraform providers
// read their respective credentials from the subprocess environment
// automatically when no credential attributes are present in the
// provider blocks.

const providerTF = `terraform {
  required_providers {
    ovh = {
      source  = "ovh/ovh"
      version = "~> 0.40"
    }
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "~> 1.54"
    }
  }
}

provider "ovh" {}

provider "openstack" {
  region = "{{ .Region }}"
}
`

const dataTF = `data "openstack_images_image_v2" "planet" {
  name        = "{{ .AMIOS }}"
  most_recent = true

  properties = {
    os_distro = "rocky"
  }
}

data "openstack_compute_flavor_v2" "planet" {
  name = "{{ .InstanceType }}"
}
`

const resourcesNetworkTF = `resource "openstack_compute_keypair_v2" "planet" {
  name       = "sovereign-planet-{{ .PlanetID }}-{{ .Slug }}"
  public_key = file("{{ .SSHKeyPath }}.pub")
}

resource "openstack_networking_secgroup_v2" "planet" {
  name        = "sovereign-planet-{{ .PlanetID }}-{{ .Slug }}"
  description = "sovereign planet {{ .PlanetID }} security group"
}

resource "openstack_networking_secgroup_rule_v2" "ssh" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 22
  port_range_max    = 22
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.planet.id
}

resource "openstack_networking_secgroup_rule_v2" "status_agent_unsecure" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 18888
  port_range_max    = 18888
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.planet.id
}

resource "openstack_networking_secgroup_rule_v2" "status_agent_secure" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 18889
  port_range_max    = 18889
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.planet.id
}

resource "openstack_networking_secgroup_rule_v2" "egress" {
  direction         = "egress"
  ethertype         = "IPv4"
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.planet.id
}
`

const resourcesComputeTF = `resource "openstack_compute_instance_v2" "planet" {
  name            = "sovereign-planet-{{ .PlanetID }}"
  image_id        = data.openstack_images_image_v2.planet.id
  flavor_id       = data.openstack_compute_flavor_v2.planet.id
  key_pair        = openstack_compute_keypair_v2.planet.name
  security_groups = [openstack_networking_secgroup_v2.planet.name]

  network {
    name = "Ext-Net"
  }

  user_data = <<-EOF
    #!/bin/bash
    dnf install -y git curl tar
    curl -fsSL https://raw.githubusercontent.com/andreimerlescu/igo/main/install.sh | bash
    export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
    go install {{ .LemmingsPackage }}
  EOF

  metadata = {
    Name      = "sovereign-planet-{{ .PlanetID }}"
    Slug      = "{{ .Slug }}"
    ManagedBy = "sovereign"
  }
}
`

const outputTF = `output "instance_id" {
  value = openstack_compute_instance_v2.planet.id
}

output "public_ipv4" {
  value = openstack_compute_instance_v2.planet.access_ip_v4
}

output "private_ipv4" {
  value = openstack_compute_instance_v2.planet.access_ip_v4
}
`
