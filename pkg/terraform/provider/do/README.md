# do

> Terraform provider implementation for DigitalOcean.
> Provisions Droplets running Rocky Linux 9 for sovereign planets.

---

## What This Package Is

This package implements the TerraformProvider interface for DigitalOcean
using the digitalocean/digitalocean terraform provider. DigitalOcean
offers inexpensive globally distributed compute with a simple single-token
authentication model making it well suited for sovereign's load testing
use case.

---

## Files

    auth.go       credential resolution from environment variables
    hcl.go        HCL templates and file rendering
    provider.go   TerraformProviderDigitalOcean struct and all interface methods

---

## Authentication

DigitalOcean uses a single personal access token. Resolved once at
construction time via New(). Stored on the struct. Passed to every
terraform subprocess call via the env() helper. Never written into
HCL files.

Required environment variables:

    DIGITALOCEAN_TOKEN    DigitalOcean personal access token

The terraform provider reads DIGITALOCEAN_TOKEN from the subprocess
environment automatically when the provider block contains no
token attribute.

---

## HCL Files Generated

Five files are written to WorkDir/terraform/ when RenderHCL is called.
All files are rendered in memory before any are written to disk.

    provider.tf

        DigitalOcean provider block. No credentials in the file.
        Terraform reads DIGITALOCEAN_TOKEN from the subprocess
        environment automatically.

        terraform required_providers block pins digitalocean/digitalocean ~> 2.0.

    data.tf

        Comment-only file. DigitalOcean does not require a data source
        for image lookup. The image slug is passed directly into the
        Droplet resource from HCLSpec.AMIOS. The file is retained for
        structural consistency with the five-file contract.

    resources_network.tf

        Three resources:

        digitalocean_ssh_key uploads the public key from SSHKeyPath.pub
        to the DigitalOcean account.

        digitalocean_firewall creates a firewall attached to the Droplet
        with these rules:

            inbound  tcp 22       SSH access from any source
            inbound  tcp 18888    StatusAgent HTTP from any source
            inbound  tcp 18889    StatusAgent HTTPS from any source
            outbound tcp 1-65535  all outbound TCP
            outbound udp 1-65535  all outbound UDP
            outbound icmp         all outbound ICMP

    resources_compute.tf

        digitalocean_droplet resource. References the SSH key from
        resources_network.tf. Sets private_networking to true so
        private_ipv4 output is populated. Sets monitoring to false.

        Image is set directly from HCLSpec.AMIOS as a slug string.
        For Rocky Linux 9: rockylinux-9-x64

        Size is set from HCLSpec.InstanceType as a size slug.
        For example: s-2vcpu-4gb or c-2

        User data bootstrap installs Go via igo and then installs
        lemmings via go install:

            #!/bin/bash
            dnf install -y git curl tar
            curl -fsSL https://raw.githubusercontent.com/andreimerlescu/igo/main/install.sh | bash
            export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
            go install <LemmingsPackage>

    output.tf

        Three required outputs that the Output() method parses:

            instance_id     digitalocean_droplet.planet.id
            public_ipv4     digitalocean_droplet.planet.ipv4_address
            private_ipv4    digitalocean_droplet.planet.ipv4_address_private

---

## Lifecycle Methods

    New()

        Resolves credentials via auth.Resolve(). Returns an error if
        DIGITALOCEAN_TOKEN is missing. Constructs and returns
        TerraformProviderDigitalOcean.

    Name()

        Returns "digitalocean".

    Validate()

        Checks that the token is non-empty and that the terraform
        binary is available in PATH via exec.LookPath.

    RenderHCL(spec HCLSpec)

        Delegates to hcl.go renderHCL(). Renders all five HCL files
        into spec.WorkDir/terraform/. Backs up rendered files to
        spec.WorkDir/backups/<timestamp>/ after every successful write.

    Init(ctx, spec)

        Runs terraform init -no-color in spec.WorkDir/terraform/.

    InitOrApply(ctx, spec)

        Checks for .terraform/ directory. If absent runs Init then
        Apply. If present skips Init and runs Apply directly.

    Apply(ctx, spec)

        Runs terraform apply -auto-approve -no-color.

    Destroy(ctx, spec)

        Runs terraform destroy -auto-approve -no-color.

    Output(ctx, spec)

        Runs terraform output -json -no-color. Parses instance_id,
        public_ipv4, private_ipv4. Returns an error listing any
        missing output names.

    Status(ctx, spec)

        Checks for terraform.tfstate existence. If absent returns
        LifecycleUninitialized. If present runs terraform show -json
        and searches for a digitalocean_droplet resource with a
        non-empty id. Returns LifecycleApplied if found,
        LifecycleUninitialized if not.

    StateBackend(spec)

        Returns spec.WorkDir/terraform/terraform.tfstate.

    LoadState(spec)

        Reads spec.WorkDir/provider_state.yml into ProviderState.
        Returns nil, nil if the file does not exist.

    SaveState(spec, state)

        Sets state.LastTransition to time.Now().UTC(). Marshals
        ProviderState to YAML. Writes to spec.WorkDir/provider_state.yml.

    UnloadState(spec)

        No-op. TerraformProviderDigitalOcean is stateless.

---

## Regions

DigitalOcean regions use short slugs, not AWS-style strings. When
using the digitalocean provider the operator must set --region to
DigitalOcean slugs. The defaultDORegions in sovereign.go are used
when no regions are specified:

    nyc1    New York City 1
    sfo3    San Francisco 3
    ams3    Amsterdam 3
    sgp1    Singapore 1
    lon1    London 1

Other available regions:

    fra1    Frankfurt 1
    tor1    Toronto 1
    blr1    Bangalore 1
    syd1    Sydney 1

---

## Recommended Size Slugs

DigitalOcean uses size slugs instead of instance type names:

    s-2vcpu-4gb    2 vCPU  4GB RAM   shared CPU, cost-effective
    s-4vcpu-8gb    4 vCPU  8GB RAM   shared CPU, higher terrain count
    c-2            2 vCPU  4GB RAM   dedicated CPU, stable for longer runs
    c-4            4 vCPU  8GB RAM   dedicated CPU, high pack count

---

## Default Image Configuration

The AMIOS field carries the Droplet image slug directly. No data
source lookup is performed. Default for Rocky Linux 9:

    AMIOS: rockylinux-9-x64

To use Ubuntu instead:

    AMIOS:      ubuntu-22-04-x64
    RemoteUser: ubuntu

---

## Import Path

    import tfdo "github.com/andreimerlescu/sovereign/pkg/terraform_provider/do"

---

## Usage in sovereign

    provider, err := tfdo.New()
    if err != nil {
        // missing DIGITALOCEAN_TOKEN
    }

The newProvider factory in sovereign.go handles this automatically
when --provider digitalocean is set.
