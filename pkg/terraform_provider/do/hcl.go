package do

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform_provider"
)

// hclFile represents a single rendered HCL file to be written to disk.
type hclFile struct {
	name    string
	content string
}

// renderHCL renders all HCL files required for a sovereign DigitalOcean
// planet and writes them to spec.WorkDir/terraform/. After writing it
// records a backup in spec.WorkDir/backups/<timestamp>/.
func renderHCL(spec tfprovider.HCLSpec) error {
	terraformDir := filepath.Join(spec.WorkDir, "terraform")
	if err := os.MkdirAll(terraformDir, 0700); err != nil {
		return fmt.Errorf("do: create terraform dir: %w", err)
	}

	files, err := renderAll(spec)
	if err != nil {
		return err
	}

	for _, f := range files {
		path := filepath.Join(terraformDir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0600); err != nil {
			return fmt.Errorf("do: write %s: %w", f.name, err)
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
			return nil, fmt.Errorf("do: render %s: %w", t.name, err)
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
		return fmt.Errorf("do: create backup dir: %w", err)
	}
	for _, f := range files {
		src := filepath.Join(terraformDir, f.name)
		dst := filepath.Join(backupDir, f.name)
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("do: backup read %s: %w", f.name, err)
		}
		if err := os.WriteFile(dst, data, 0600); err != nil {
			return fmt.Errorf("do: backup write %s: %w", f.name, err)
		}
	}
	return nil
}

// -- HCL Templates -----------------------------------------------------------
//
// DigitalOcean uses the digitalocean/digitalocean terraform provider.
// No secondary provider is needed.
//
// Credentials are not rendered into these templates. The DIGITALOCEAN_TOKEN
// environment variable is read automatically by the provider from the
// subprocess environment set in env().
//
// DigitalOcean Droplets use image slugs directly rather than AMI lookups.
// The AMIOS field in HCLSpec carries the Droplet image slug.
// For Rocky Linux 9 this is: rockylinux-9-x64
//
// InstanceType carries the Droplet size slug.
// Recommended for lemmings: s-2vcpu-4gb or c-2

const providerTF = `terraform {
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.0"
    }
  }
}

provider "digitalocean" {}
`

const dataTF = `# DigitalOcean uses image slugs directly in the Droplet resource.
# No data source lookup is required for images.
# The image slug is passed directly from HCLSpec.AMIOS.
# For Rocky Linux 9: rockylinux-9-x64
`

const resourcesNetworkTF = `resource "digitalocean_ssh_key" "planet" {
  name       = "sovereign-planet-{{ .PlanetID }}-{{ .Slug }}"
  public_key = file("{{ .SSHKeyPath }}.pub")
}

resource "digitalocean_firewall" "planet" {
  name = "sovereign-planet-{{ .PlanetID }}-{{ .Slug }}"

  droplet_ids = [digitalocean_droplet.planet.id]

  inbound_rule {
    protocol         = "tcp"
    port_range       = "22"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  inbound_rule {
    protocol         = "tcp"
    port_range       = "18888"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  inbound_rule {
    protocol         = "tcp"
    port_range       = "18889"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "tcp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "udp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "icmp"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
}
`

const resourcesComputeTF = `resource "digitalocean_droplet" "planet" {
  name               = "sovereign-planet-{{ .PlanetID }}"
  region             = "{{ .Region }}"
  size               = "{{ .InstanceType }}"
  image              = "{{ .AMIOS }}"
  ssh_keys           = [digitalocean_ssh_key.planet.fingerprint]
  private_networking = true
  monitoring         = false

  user_data = <<-EOF
    #!/bin/bash
    dnf install -y git curl tar
    curl -fsSL https://raw.githubusercontent.com/andreimerlescu/igo/main/install.sh | bash
    export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
    go install {{ .LemmingsPackage }}
  EOF

  tags = [
    "sovereign",
    "planet-{{ .PlanetID }}",
    "{{ .Slug }}",
  ]
}
`

const outputTF = `output "instance_id" {
  value = digitalocean_droplet.planet.id
}

output "public_ipv4" {
  value = digitalocean_droplet.planet.ipv4_address
}

output "private_ipv4" {
  value = digitalocean_droplet.planet.ipv4_address_private
}
`
