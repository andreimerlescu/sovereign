package aws

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

// renderHCL renders all HCL files required for a sovereign AWS planet
// and writes them to spec.WorkDir/terraform/. After writing, it records
// a backup of the rendered files in spec.WorkDir/backups/<timestamp>/.
func renderHCL(spec tfprovider.HCLSpec) error {
	terraformDir := filepath.Join(spec.WorkDir, "terraform")
	if err := os.MkdirAll(terraformDir, 0700); err != nil {
		return fmt.Errorf("aws: create terraform dir: %w", err)
	}

	files, err := renderAll(spec)
	if err != nil {
		return err
	}

	for _, f := range files {
		path := filepath.Join(terraformDir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0600); err != nil {
			return fmt.Errorf("aws: write %s: %w", f.name, err)
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
			return nil, fmt.Errorf("aws: render %s: %w", t.name, err)
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
// directory inside spec.WorkDir/backups/. Called after every successful
// write so the exact terraform that provisioned each instance is preserved.
func backupHCL(spec tfprovider.HCLSpec, terraformDir string, files []hclFile) error {
	ts := time.Now().UTC().Format("2006-01-02T15-04-05")
	backupDir := filepath.Join(spec.WorkDir, "backups", ts)
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return fmt.Errorf("aws: create backup dir: %w", err)
	}
	for _, f := range files {
		src := filepath.Join(terraformDir, f.name)
		dst := filepath.Join(backupDir, f.name)
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("aws: backup read %s: %w", f.name, err)
		}
		if err := os.WriteFile(dst, data, 0600); err != nil {
			return fmt.Errorf("aws: backup write %s: %w", f.name, err)
		}
	}
	return nil
}

// -- HCL Templates -----------------------------------------------------------
//
// All values are rendered directly from HCLSpec. No terraform variables
// are used -- sovereign generates a complete, self-contained terraform
// directory per planet with no external variable files required.

const providerTF = `terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region     = "{{ .Region }}"
  access_key = "{{ .AccessKey }}"
  secret_key = "{{ .SecretKey }}"
}
`

const dataTF = `data "aws_ami" "planet" {
  most_recent = true
  owners      = ["{{ .AMIOwners }}"]

  filter {
    name   = "name"
    values = ["{{ .AMIOS }}"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}
`

const resourcesNetworkTF = `resource "aws_key_pair" "planet" {
  key_name   = "sovereign-planet-{{ .PlanetID }}-{{ .Slug }}"
  public_key = file("{{ .SSHKeyPath }}.pub")
}
`

const resourcesComputeTF = `resource "aws_instance" "planet" {
  ami                         = data.aws_ami.planet.id
  instance_type               = "{{ .InstanceType }}"
  subnet_id                   = "{{ .SubnetID }}"
  key_name                    = aws_key_pair.planet.key_name
  associate_public_ip_address = true

  vpc_security_group_ids = [
    "{{ .SecurityGroupID }}",
  ]

  root_block_device {
    volume_size = {{ .RootVolumeSize }}
    volume_type = "{{ .RootVolumeType }}"
  }

  ebs_block_device {
    device_name = "/dev/sdb"
    volume_size = {{ .TempVolumeSize }}
    volume_type = "{{ .TempVolumeType }}"
  }

  user_data = <<-EOF
    #!/bin/bash
    dnf install -y git curl tar
    curl -fsSL https://raw.githubusercontent.com/andreimerlescu/igo/main/install.sh | bash
    export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
    go install {{ .LemmingsPackage }}
  EOF

  tags = {
    Name     = "sovereign-planet-{{ .PlanetID }}"
    Slug     = "{{ .Slug }}"
    ManagedBy = "sovereign"
  }
}
`

const outputTF = `output "instance_id" {
  value = aws_instance.planet.id
}

output "public_ipv4" {
  value = aws_instance.planet.public_ip
}

output "private_ipv4" {
  value = aws_instance.planet.private_ip
}
`
