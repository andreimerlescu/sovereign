# aws

> Terraform provider implementation for Amazon Web Services.
> Provisions EC2 instances running Rocky Linux 9 for sovereign planets.

---

## What This Package Is

This package implements the TerraformProvider interface for AWS using
the hashicorp/aws terraform provider. It is the primary and most
complete provider implementation in sovereign. All other provider
implementations follow the patterns established here.

---

## Files

    auth.go       credential resolution from environment variables
    hcl.go        HCL templates and file rendering
    provider.go   TerraformProviderAWS struct and all interface methods

---

## Authentication

Credentials are resolved once at construction time via New(). They
are stored on the struct and never read from the environment again.
They are passed to every terraform subprocess call via the env()
helper and never written into HCL files.

Required environment variables:

    AWS_ACCESS_KEY              AWS access key ID
    AWS_ACCESS_SECRET_KEY       AWS secret access key

Sovereign-specific environment variables also required at runtime
but not part of provider authentication:

    SOVEREIGN_SUBNET_ID         Subnet ID for planet EC2 instances
    SOVEREIGN_SECURITY_GROUP    Security group ID for planet EC2 instances

The terraform subprocess receives credentials as:

    AWS_ACCESS_KEY_ID           mapped from AWS_ACCESS_KEY
    AWS_SECRET_ACCESS_KEY       mapped from AWS_ACCESS_SECRET_KEY

---

## HCL Files Generated

Five files are written to WorkDir/terraform/ when RenderHCL is called.
All files are rendered in memory before any are written to disk so a
template error never produces a partially written terraform directory.

    provider.tf

        AWS provider block with region. No credentials in the file.
        Terraform reads AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
        from the subprocess environment automatically.

        terraform required_providers block pins hashicorp/aws ~> 5.0.

    data.tf

        AMI lookup data source. Searches for the most recent image
        matching AMIOS in the AMIOwners account. Filters on x86_64
        architecture and hvm virtualization type.

        For Rocky Linux 9 the defaults are:

            AMIOS:     Rocky-9-EC2-Base-9.*-x86_64*
            AMIOwners: 792107900819

        AMIOwners 792107900819 is the Rocky Linux Foundation AWS
        account ID. Using "amazon" as the owner will not find Rocky
        Linux images.

    resources_network.tf

        aws_key_pair resource. Uploads the public key from SSHKeyPath.pub
        to AWS. The key pair name includes the planet ID and slug for
        uniqueness across concurrent sovereign runs.

    resources_compute.tf

        aws_instance resource. References the AMI from data.tf and the
        key pair from resources_network.tf. Configures root and temp
        EBS volumes. Associates the configured subnet and security group.
        Sets associate_public_ip_address to true.

        User data bootstrap installs Go via igo and then installs
        lemmings via go install:

            #!/bin/bash
            dnf install -y git curl tar
            curl -fsSL https://raw.githubusercontent.com/andreimerlescu/igo/main/install.sh | bash
            export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
            go install <LemmingsPackage>

    output.tf

        Three required outputs that the Output() method parses:

            instance_id     aws_instance.planet.id
            public_ipv4     aws_instance.planet.public_ip
            private_ipv4    aws_instance.planet.private_ip

---

## Lifecycle Methods

    New()

        Resolves credentials via auth.Resolve(). Returns an error if
        any required environment variable is missing. Constructs and
        returns TerraformProviderAWS.

    Name()

        Returns "aws".

    Validate()

        Checks that credentials are non-empty and that the terraform
        binary is available in PATH via exec.LookPath.

    RenderHCL(spec HCLSpec)

        Delegates to hcl.go renderHCL(). Renders all five HCL files
        into spec.WorkDir/terraform/. Creates the directory if absent.
        Backs up rendered files to spec.WorkDir/backups/<timestamp>/
        after every successful write.

    Init(ctx, spec)

        Runs terraform init -no-color in spec.WorkDir/terraform/.

    InitOrApply(ctx, spec)

        Checks for .terraform/ directory in spec.WorkDir/terraform/.
        If absent runs Init then Apply. If present skips Init and
        runs Apply directly. Safe to call after an interrupted apply.

    Apply(ctx, spec)

        Runs terraform apply -auto-approve -no-color.

    Destroy(ctx, spec)

        Runs terraform destroy -auto-approve -no-color.

    Output(ctx, spec)

        Runs terraform output -json -no-color. Parses instance_id,
        public_ipv4, private_ipv4 from the JSON response. Returns
        an error listing any missing output names.

    Status(ctx, spec)

        Checks for terraform.tfstate existence. If absent returns
        LifecycleUninitialized. If present runs terraform show -json
        and searches for an aws_instance resource with a non-empty id.
        Returns LifecycleApplied if found, LifecycleUninitialized if not.

    StateBackend(spec)

        Returns spec.WorkDir/terraform/terraform.tfstate.

    LoadState(spec)

        Reads spec.WorkDir/provider_state.yml into ProviderState.
        Returns nil, nil if the file does not exist.

    SaveState(spec, state)

        Sets state.LastTransition to time.Now().UTC(). Marshals
        ProviderState to YAML. Writes to spec.WorkDir/provider_state.yml.
        Creates the directory if absent.

    UnloadState(spec)

        No-op. TerraformProviderAWS is stateless. Nothing to clear.

---

## Backup Behavior

After every successful RenderHCL call, all five rendered HCL files
are copied to a timestamped backup directory:

    spec.WorkDir/backups/2026-04-22T14-30-05/

This means the exact terraform that provisioned each instance is
preserved even if the live terraform directory is modified or cleaned.
Backup directories accumulate across runs and must be cleaned manually
or by sovereign's cleanup phase.

---

## State File Location

    spec.WorkDir/provider_state.yml

Which resolves to:

    ~/.sovereignd/planets/planet-N/provider_state.yml

---

## Default AMI Configuration

The defaults in sovereign's main package target Rocky Linux 9:

    AMIOS:     Rocky-9-EC2-Base-9.*-x86_64*
    AMIOwners: 792107900819

To use Ubuntu instead set:

    AMIOS:     ubuntu/images/hvm-ssd/ubuntu-*-amd64*
    AMIOwners: amazon
    RemoteUser: ubuntu

To use Amazon Linux set:

    AMIOS:     amzn2-ami-hvm-*-x86_64-gp2
    AMIOwners: amazon
    RemoteUser: ec2-user

---

## Default Regions

When no regions are specified and provider is aws:

    us-east-1
    us-west-1
    eu-west-1
    ap-southeast-1
    sa-east-1

---

## Recommended Instance Types

    t3.medium      2 vCPU  4GB RAM   general purpose, cost-effective
    c6a.large      2 vCPU  4GB RAM   compute optimized, stable long-term
    c6a.xlarge     4 vCPU  8GB RAM   high terrain count runs

---

## Import Path

    import tfaws "github.com/andreimerlescu/sovereign/pkg/terraform_provider/aws"

---

## Usage in sovereign

    provider, err := tfaws.New()
    if err != nil {
        // missing credentials
    }
    if err := provider.Validate(); err != nil {
        // terraform not in PATH
    }

The newProvider factory in sovereign.go handles this automatically
when --provider aws is set or defaulted.
