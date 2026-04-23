# terraform_provider

> The cloud provider abstraction layer for sovereign.
> Each provider generates Terraform HCL, manages the terraform
> lifecycle, and persists state independently of sovereign's
> internal domain model.

---

## What This Package Is

This package defines the contract every cloud provider must fulfill
to participate in sovereign's provisioning lifecycle. It contains
no implementation logic. It contains only the interface, the shared
types that cross the boundary between sovereign and a provider, and
the state types that track terraform lifecycle progress.

Sovereign never calls a cloud provider API directly. It calls
terraform. This package is the abstraction that makes sovereign
indifferent to which cloud is on the other side.

---

## Package Layout

    pkg/terraform_provider/
    |-- provider.go          TerraformProvider interface, PlanetSpec, HCLSpec,
    |                        ProvisionOutput, LifecycleStatus, ProviderState, HCLBackup
    |
    |-- aws/
    |   |-- provider.go      TerraformProviderAWS -- full implementation
    |   |-- hcl.go           AWS HCL templates and file rendering
    |   `-- auth.go          AWS credential resolution from environment
    |
    |-- do/
    |   `-- provider.go      TerraformProviderDigitalOcean -- stub, not yet implemented
    |
    |-- gcp/
    |   `-- provider.go      TerraformProviderGCP -- stub, not yet implemented
    |
    |-- oci/
    |   `-- provider.go      TerraformProviderOCI -- stub, not yet implemented
    |
    `-- ovh/
        `-- provider.go      TerraformProviderOVH -- stub, not yet implemented

---

## The Interface

Every provider implements TerraformProvider:

    type TerraformProvider interface {

        // Identity
        Name() string
        Validate() error

        // HCL -- provider owns templates, sovereign provides specs
        RenderHCL(spec HCLSpec) error

        // Lifecycle
        Init(ctx context.Context, spec PlanetSpec) error
        InitOrApply(ctx context.Context, spec PlanetSpec) error
        Apply(ctx context.Context, spec PlanetSpec) error
        Destroy(ctx context.Context, spec PlanetSpec) error
        Output(ctx context.Context, spec PlanetSpec) (*ProvisionOutput, error)
        Status(ctx context.Context, spec PlanetSpec) (LifecycleStatus, error)

        // State
        StateBackend(spec PlanetSpec) (string, error)
        LoadState(spec PlanetSpec) (*ProviderState, error)
        SaveState(spec PlanetSpec, state *ProviderState) error
        UnloadState(spec PlanetSpec) error
    }

---

## The Boundary Types

These are the only types that cross the boundary between sovereign
and a provider. Providers never see sovereign's internal *Planet type.
Sovereign constructs these from *Planet before calling any provider
method.

### PlanetSpec

What a provider needs to locate and operate on a planet's terraform
working directory. Passed to all lifecycle methods.

    type PlanetSpec struct {
        PlanetID int
        WorkDir  string
        Region   string
        Zone     string
    }

### HCLSpec

What a provider needs to render Terraform HCL for a planet. Passed
to RenderHCL only. Contains every parameter any provider could need
across any HCL file. Providers ignore fields they do not use.

    type HCLSpec struct {
        // Identity
        PlanetID int
        Slug     string

        // Compute
        Region       string
        Zone         string
        InstanceType string
        RemoteUser   string

        // AMI / Image selection
        AMIOS     string
        AMIOwners string

        // Network
        SubnetID        string
        SecurityGroupID string

        // Storage
        RootVolumeSize int
        RootVolumeType string
        TempVolumeSize int
        TempVolumeType string

        // SSH
        SSHKeyPath string

        // Lemmings runtime
        GoVersion       string
        LemmingsPackage string
        Hit             string
        Terrain         int64
        Pack            int64
        Until           string
        Ramp            string
        PlanRemotePath  string
        SaveTo          string

        // Terraform
        WorkDir       string
        StateFilePath string
    }

### ProvisionOutput

What every provider returns after a successful apply. The only data
that flows back from the provider to sovereign after provisioning.

    type ProvisionOutput struct {
        InstanceID  string
        PublicIPv4  string
        PrivateIPv4 string
        Region      string
        Zone        string
        ProviderRef string // provider-specific resource identifier
    }

---

## Lifecycle Status

LifecycleStatus tracks the terraform state machine for a single planet.
Persisted in ProviderState so sovereign knows exactly where to resume
after any interruption.

    LifecycleUnknown        -- status cannot be determined
    LifecycleUninitialized  -- terraform init has not run
    LifecycleInitialized    -- terraform init succeeded
    LifecycleApplying       -- terraform apply is in progress
    LifecycleApplied        -- terraform apply succeeded, instance is running
    LifecycleDestroying     -- terraform destroy is in progress
    LifecycleDestroyed      -- terraform destroy succeeded
    LifecycleFailed         -- an operation failed, start over
    LifecycleInterrupted    -- interrupted mid-apply, resume with InitOrApply

Resume routing in sovereign:

    Uninitialized or Failed      -- provision from scratch
    Initialized                  -- skip init, run apply
    Applying or Interrupted      -- run InitOrApply
    Applied                      -- hydrate planet from state, skip provisioning
    Destroying or Destroyed      -- mark planet lost, nothing to do

---

## Provider State

ProviderState is written to disk after every lifecycle transition.
It lives at:

    ~/.sovereignd/planets/planet-N/provider_state.yml

Fields:

    PlanetID       int
    Provider       string
    Region         string
    Zone           string
    Status         LifecycleStatus
    InstanceID     string
    PublicIPv4     string
    PrivateIPv4    string
    TerraformDir   string
    StateFile      string
    LastTransition time.Time
    InitializedAt  time.Time
    AppliedAt      time.Time
    DestroyedAt    time.Time
    InterruptedAt  time.Time
    HCLBackups     []HCLBackup

HCLBackup records a timestamped snapshot of the rendered HCL files
taken after every successful apply:

    type HCLBackup struct {
        Timestamp time.Time
        Directory string
        Files     []string
    }

Backups live at:

    ~/.sovereignd/planets/planet-N/backups/<timestamp>/

---

## AWS Implementation

The AWS provider is the only fully implemented provider. It lives in
pkg/terraform_provider/aws/ and consists of three files.

### auth.go

Resolves AWS credentials from environment variables at construction
time. Called once by New(). Credentials are stored on the struct and
never read from the environment again.

Required environment variables:

    AWS_ACCESS_KEY
    AWS_ACCESS_SECRET_KEY

Credentials are passed to terraform subprocess calls via environment
variables only. They are never written into HCL files and never appear
in backup files on disk.

### hcl.go

Owns all AWS HCL templates. Renders five files per planet:

    provider.tf              AWS provider block with region
    data.tf                  AMI lookup data source
    resources_network.tf     Key pair resource
    resources_compute.tf     EC2 instance resource with user data bootstrap
    output.tf                instance_id, public_ip, private_ip outputs

All values are rendered directly from HCLSpec. No terraform variables
are used. Each rendered planet directory is self-contained with no
external variable files required.

The user data bootstrap in resources_compute.tf installs Go via igo
and then installs lemmings:

    #!/bin/bash
    dnf install -y git curl tar
    curl -fsSL https://raw.githubusercontent.com/andreimerlescu/igo/main/install.sh | bash
    export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
    go install <LemmingsPackage>

After every successful write, all five files are copied to a
timestamped backup directory. backupHCL records the backup path in
ProviderState.HCLBackups.

### provider.go

Implements TerraformProvider for AWS. The struct is stateless with
respect to individual planets. All per-planet data flows through
PlanetSpec and HCLSpec arguments. Credentials are the only thing
stored on the struct.

Lifecycle method behavior:

    Name()         returns "aws"

    Validate()     checks credentials are resolved and terraform
                   binary is available in PATH

    RenderHCL()    delegates to hcl.go renderHCL()

    Init()         runs terraform init -no-color in WorkDir/terraform/

    InitOrApply()  checks for .terraform/ directory existence --
                   if present skips init and runs apply directly,
                   if absent runs init then apply

    Apply()        runs terraform apply -auto-approve -no-color

    Destroy()      runs terraform destroy -auto-approve -no-color

    Output()       runs terraform output -json and parses
                   instance_id, public_ipv4, private_ipv4

    Status()       checks for terraform.tfstate existence --
                   if absent returns LifecycleUninitialized --
                   if present runs terraform show -json and checks
                   for aws_instance resource with populated id

    StateBackend() returns WorkDir/terraform/terraform.tfstate

    LoadState()    reads WorkDir/provider_state.yml into ProviderState
                   returns nil, nil if file does not exist

    SaveState()    marshals ProviderState to YAML and writes to
                   WorkDir/provider_state.yml, sets LastTransition

    UnloadState()  no-op -- struct is stateless, nothing to clear

The env() helper passes credentials to every terraform subprocess:

    PATH
    HOME
    AWS_ACCESS_KEY_ID
    AWS_SECRET_ACCESS_KEY

---

## Stub Providers

The following providers satisfy the TerraformProvider interface but
return "not yet implemented" for every method. They are registered in
sovereign's newProvider factory so the --provider flag accepts their
names and returns a clear error at Validate() time rather than an
unknown provider error.

    do   -- TerraformProviderDigitalOcean -- package do
    gcp  -- TerraformProviderGCP          -- package gcp
    oci  -- TerraformProviderOCI          -- package oci
    ovh  -- TerraformProviderOVH          -- package ovh

Implementing a new provider requires:

    1. Create pkg/terraform_provider/<name>/provider.go
    2. Implement all TerraformProvider methods
    3. Create pkg/terraform_provider/<name>/hcl.go with HCL templates
       for the five standard files
    4. Create pkg/terraform_provider/<name>/auth.go for credential
       resolution from environment variables
    5. Register the provider in sovereign.go newProvider() factory

---

## Design Constraints

### No Circular Dependencies

Provider packages import only:

    pkg/terraform_provider    -- for interface and shared types
    standard library
    gopkg.in/yaml.v3          -- for state file marshaling

Provider packages never import the sovereign main package. The
boundary is enforced by PlanetSpec and HCLSpec as the only handoff
types. If you find yourself wanting to import something from main,
the field belongs in HCLSpec or PlanetSpec instead.

### No Implementation Logic in provider.go

pkg/terraform_provider/provider.go is a types and interface definition
file only. Helpers, utilities, and shared logic that multiple providers
need belong in a dedicated file within this package, not in provider.go
and not in the provider-specific packages.

### Stateless Provider Structs

Provider structs hold only credentials and static configuration resolved
at construction time. Per-planet state is never stored on the struct.
All per-planet data arrives via PlanetSpec or HCLSpec arguments and
leaves via ProvisionOutput or ProviderState. This makes providers safe
for concurrent planet provisioning without any additional locking.

### Local Terraform State Only

Sovereign uses local terraform state per planet. There is no remote
backend configured. The state file lives at:

    WorkDir/terraform/terraform.tfstate

StateBackend() returns this path. If a remote backend is added in the
future it belongs in the provider-specific HCL templates, not in the
interface or shared types.

### Credentials via Environment Only

No provider writes credentials into HCL files. Credentials are passed
to terraform subprocess calls via the env() helper method. This ensures
credentials never appear in rendered HCL files, backup directories,
or terraform plan output.

---

## Terraform Files Generated Per Planet

Every provider is expected to generate these five files in
WorkDir/terraform/ when RenderHCL is called. Files are written
atomically -- all are rendered in memory before any are written to
disk so a template error does not produce a partially written
terraform directory.

    provider.tf              cloud provider configuration and authentication
    data.tf                  data sources (AMI lookup, image lookup, etc.)
    resources_network.tf     network-adjacent resources (key pairs, firewall rules)
    resources_compute.tf     the compute instance resource and user data bootstrap
    output.tf                instance_id, public_ipv4, private_ipv4 outputs

The output names instance_id, public_ipv4, and private_ipv4 are
required by the interface contract. Output() parses these names from
terraform output -json. A provider that uses different output names
will cause Output() to return a missing outputs error.

---

## Import Paths

    import tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform_provider"
    import tfaws     "github.com/andreimerlescu/sovereign/pkg/terraform_provider/aws"
    import tfdo      "github.com/andreimerlescu/sovereign/pkg/terraform_provider/do"
    import tfgcp     "github.com/andreimerlescu/sovereign/pkg/terraform_provider/gcp"
    import tfoci     "github.com/andreimerlescu/sovereign/pkg/terraform_provider/oci"
    import tfovh     "github.com/andreimerlescu/sovereign/pkg/terraform_provider/ovh"​​​​​​​​​​​​​​​​
