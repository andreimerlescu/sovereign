# terraform_exeggutor

> Shared terraform subprocess execution for sovereign providers.
> One place for init, apply, destroy, output parsing, and status
> checking across every cloud provider implementation.

---

## What This Package Is

Every sovereign cloud provider runs the same four terraform operations
-- init, apply, destroy, and output parsing -- against a planet's
working directory. Before this package existed, those operations were
duplicated byte-for-byte in aws/provider.go, do/provider.go, and
ovh/provider.go.

terraform_exeggutor eliminates that duplication. Each provider
constructs an Exeggutor per operation, delegates to it, and discards
it. The provider retains only what is unique to it -- credentials,
HCL templates, and the resource type name passed to ParseStatus.

The name is intentional. An exeggutor executes. It does not plan,
validate, or decide. It runs terraform and returns the result.

---

## Files

    exeggutor.go        Exeggutor struct, New, Init, Apply, Destroy,
                        InitOrApply, run, Validate,
                        StateFileExists, DotTerraformExists
    output.go           Result, ParseOutput
    status.go           ParseStatus, showState
    exeggutor_test.go   full test and benchmark coverage

---

## Dependencies

    github.com/andreimerlescu/sovereign/pkg/terraform_provider
        -- LifecycleStatus type only, used in status.go
    standard library
    os/exec   -- terraform subprocess execution
    encoding/json   -- terraform output and show JSON parsing

---

## Exeggutor

Constructed per planet per operation. Discarded after use. Carries
no state between calls. Safe for concurrent planet provisioning
because each planet gets its own Exeggutor instance.

    type Exeggutor struct {
        Name     string    // provider name for error messages
        PlanetID int       // planet ID for error messages
        WorkDir  string    // planet working directory
        Env      []string  // complete subprocess environment
    }

Construction:

    exe := terraform_exeggutor.New(
        p.Name(),
        spec.PlanetID,
        spec.WorkDir,
        p.env(),
    )

---

## Lifecycle Methods

    Init(ctx)

        Runs terraform init -no-color in WorkDir/terraform/.
        Returns a prefixed error on failure:

            aws: terraform init planet 3: exit status 1

    Apply(ctx)

        Runs terraform apply -auto-approve -no-color.

    Destroy(ctx)

        Runs terraform destroy -auto-approve -no-color.

    InitOrApply(ctx)

        Checks for .terraform/ directory existence via
        DotTerraformExists(). If absent runs Init then Apply.
        If present skips Init and runs Apply directly.
        Safe to call after an interrupted apply.

All methods forward stdout and stderr to the sovereign process so
the operator sees terraform output in real time.

---

## ParseOutput

Runs terraform output -json -no-color and parses the three required
output values into a Result:

    result, err := exe.ParseOutput(ctx)
    if err != nil {
        // missing outputs or terraform failed
    }

    type Result struct {
        InstanceID  string
        PublicIPv4  string
        PrivateIPv4 string
    }

The three output names -- instance_id, public_ipv4, private_ipv4 --
are required by the TerraformProvider interface contract. Every
provider's output.tf must define these exact names. ParseOutput
returns an error listing any missing names.

---

## ParseStatus

Runs terraform show -json -no-color and searches for a resource of
the given type with a non-empty id field:

    status, err := exe.ParseStatus(ctx, "aws_instance")
    status, err := exe.ParseStatus(ctx, "digitalocean_droplet")
    status, err := exe.ParseStatus(ctx, "openstack_compute_instance_v2")

Returns:

    LifecycleUninitialized   state file does not exist
    LifecycleApplied         resource found with non-empty id
    LifecycleUninitialized   state file exists but resource absent
    LifecycleFailed          terraform show failed or unparseable

The resourceType argument is the only provider-specific value
ParseStatus needs. Each provider passes its own resource type string.

---

## Filesystem Helpers

    StateFileExists() bool

        Returns true if WorkDir/terraform/terraform.tfstate exists.
        Used by providers to check initialization state.

    DotTerraformExists() bool

        Returns true if WorkDir/terraform/.terraform exists.
        Used by InitOrApply to determine whether init has run.

---

## Validate

Package-level function. Checks that terraform is available in PATH:

    if err := terraform_exeggutor.Validate(); err != nil {
        // terraform not installed
    }

Called by each provider's Validate() method before provisioning begins.

---

## Environment

Exeggutor never reads from os.Environ directly. The complete
subprocess environment is passed at construction time from the
provider's env() method:

    func (p *TerraformProviderAWS) env() []string {
        return []string{
            "PATH=" + os.Getenv("PATH"),
            "HOME=" + os.Getenv("HOME"),
            "AWS_ACCESS_KEY_ID=" + p.creds.AccessKey,
            "AWS_SECRET_ACCESS_KEY=" + p.creds.AccessSecretKey,
        }
    }

This guarantees that credentials never leak into other subprocess
calls and that each provider controls exactly what its terraform
subprocess can see.

---

## How Providers Use Exeggutor

Before exeggutor, a provider's Apply method looked like this:

    func (p *TerraformProviderAWS) Apply(
        ctx context.Context,
        spec tfprovider.PlanetSpec,
    ) error {
        terraformDir := filepath.Join(spec.WorkDir, "terraform")
        cmd := exec.CommandContext(ctx, "terraform",
            "apply", "-auto-approve", "-no-color")
        cmd.Dir = terraformDir
        cmd.Env = p.env()
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr
        if err := cmd.Run(); err != nil {
            return fmt.Errorf("aws: terraform apply planet %d: %w",
                spec.PlanetID, err)
        }
        return nil
    }

After exeggutor, the same method is:

    func (p *TerraformProviderAWS) Apply(
        ctx context.Context,
        spec tfprovider.PlanetSpec,
    ) error {
        return terraform_exeggutor.New(
            p.Name(), spec.PlanetID, spec.WorkDir, p.env(),
        ).Apply(ctx)
    }

The same reduction applies to Init, Destroy, Output, and Status.
Each provider method becomes a one or two line delegation.

---

## Resource Type Strings

    aws:          "aws_instance"
    digitalocean: "digitalocean_droplet"
    ovh:          "openstack_compute_instance_v2"

When implementing a new provider define a package-level constant
for the resource type and pass it to ParseStatus:

    const resourceType = "google_compute_instance"

    func (p *TerraformProviderGCP) Status(...) {
        return exe.ParseStatus(ctx, resourceType)
    }

---

## Import Path

    import terraform_exeggutor "github.com/andreimerlescu/sovereign/pkg/terraform_exeggutor"

---

## Testing

    go test -race ./pkg/terraform_exeggutor/...
    go test -bench=. -benchmem ./pkg/terraform_exeggutor/...

Tests that require a real terraform binary are skipped automatically
when terraform is not in PATH. Unit tests cover structural behavior
using fake state files and direct struct manipulation without
spawning terraform processes.
