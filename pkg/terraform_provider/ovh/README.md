# ovh

> Terraform provider implementation for OVHcloud Public Cloud.
> Provisions OpenStack compute instances running Rocky Linux 9
> for sovereign planets at low cost across global regions.

---

## What This Package Is

This package implements the TerraformProvider interface for OVHcloud
using two terraform providers together -- ovh/ovh for OVH-specific
resources and terraform-provider-openstack/openstack for compute
instances. OVHcloud exposes OpenStack-compatible APIs for its public
cloud compute layer.

OVH offers some of the most inexpensive compute resources globally
making it particularly well suited for cost-sensitive distributed
load testing runs with sovereign.

---

## Files

    auth.go       credential resolution from environment variables
    hcl.go        HCL templates and file rendering
    provider.go   TerraformProviderOVH struct and all interface methods

---

## Authentication

OVH requires two sets of credentials resolved at construction time
via New(). Both sets are stored on the struct and passed to every
terraform subprocess call via the env() helper. Neither set is ever
written into HCL files.

OVH API credentials:

    OVH_ENDPOINT              API endpoint: ovh-eu, ovh-us, or ovh-ca
    OVH_APPLICATION_KEY       OVH application key
    OVH_APPLICATION_SECRET    OVH application secret
    OVH_CONSUMER_KEY          OVH consumer key

OpenStack credentials:

    OS_AUTH_URL               OpenStack identity endpoint
    OS_TENANT_ID              OpenStack project ID
    OS_TENANT_NAME            OpenStack project name
    OS_USERNAME               OpenStack username
    OS_PASSWORD               OpenStack password
    OS_REGION_NAME            OpenStack region name

Both the ovh/ovh and openstack terraform providers read their
respective credentials from the subprocess environment automatically
when no credential attributes are present in the provider blocks.

To obtain OVH API credentials log into the OVH control panel and
create an application at https://eu.api.ovh.com/createApp/. The
OpenStack credentials are available in the OVH control panel under
Public Cloud > Users and Roles as an OpenStack RC file. Source the
RC file to populate the OS_* environment variables.

---

## HCL Files Generated

Five files are written to WorkDir/terraform/ when RenderHCL is called.
All files are rendered in memory before any are written to disk.

    provider.tf

        Two provider blocks. Neither contains credentials.

        The ovh provider block is empty -- OVH_ENDPOINT and API
        credentials are read from the subprocess environment.

        The openstack provider block sets region from HCLSpec.Region.
        All OS_* credentials are read from the subprocess environment.

        terraform required_providers pins:
            ovh/ovh                              ~> 0.40
            terraform-provider-openstack/openstack ~> 1.54

    data.tf

        Two data sources:

        openstack_images_image_v2 searches for the most recent image
        matching HCLSpec.AMIOS with os_distro set to rocky. For Rocky
        Linux 9 the image name is Rocky Linux 9.

        openstack_compute_flavor_v2 looks up the flavor by name from
        HCLSpec.InstanceType. For example b2-7 for a 2 vCPU 7GB flavor.

    resources_network.tf

        Four resources:

        openstack_compute_keypair_v2 uploads the public key from
        SSHKeyPath.pub to the OVH project.

        openstack_networking_secgroup_v2 creates a security group
        named after the planet ID and slug.

        openstack_networking_secgroup_rule_v2 resources create these
        ingress rules on the security group:

            tcp 22      SSH from any source
            tcp 18888   StatusAgent HTTP from any source
            tcp 18889   StatusAgent HTTPS from any source

        One egress rule allows all outbound IPv4 traffic.

    resources_compute.tf

        openstack_compute_instance_v2 resource. References the image
        and flavor from data.tf, the keypair from resources_network.tf,
        and the security group from resources_network.tf.

        Network is set to Ext-Net which is OVH's public network that
        provides a direct public IPv4 address on the instance. No
        floating IP or router configuration is required.

        User data bootstrap installs Go via igo and then installs
        lemmings via go install:

            #!/bin/bash
            dnf install -y git curl tar
            curl -fsSL https://raw.githubusercontent.com/andreimerlescu/igo/main/install.sh | bash
            export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
            go install <LemmingsPackage>

    output.tf

        Three required outputs that the Output() method parses:

            instance_id     openstack_compute_instance_v2.planet.id
            public_ipv4     openstack_compute_instance_v2.planet.access_ip_v4
            private_ipv4    openstack_compute_instance_v2.planet.access_ip_v4

        Both public_ipv4 and private_ipv4 map to access_ip_v4 because
        OVH instances on Ext-Net receive a single public IP as their
        primary access address. There is no separate private IP in this
        topology.

---

## Lifecycle Methods

    New()

        Resolves both OVH and OpenStack credentials via auth.Resolve().
        Returns an error listing all missing environment variables.
        Constructs and returns TerraformProviderOVH.

    Name()

        Returns "ovh".

    Validate()

        Checks that ApplicationKey is non-empty and that the terraform
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
        and searches for an openstack_compute_instance_v2 resource
        with a non-empty id. Returns LifecycleApplied if found,
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

        No-op. TerraformProviderOVH is stateless.

---

## Regions

OVH public cloud regions use uppercase datacenter codes. When using
the ovh provider the operator must set --region to OVH region codes.
The defaultOVHRegions in sovereign.go are used when no regions are
specified:

    GRA11    Gravelines 11, France
    SBG5     Strasbourg 5, France
    DE1      Frankfurt, Germany
    UK1      London, United Kingdom
    SGP1     Singapore

Other available regions vary by OVH subsidiary:

    BHS5     Beauharnois, Canada
    WAW1     Warsaw, Poland
    SYD1     Sydney, Australia

The OS_REGION_NAME environment variable must match the region used
in the HCLSpec.Region field. Set both consistently.

---

## Recommended Flavor Names

OVH uses OpenStack flavor names. B-series flavors offer good
price-to-performance for lemmings workloads:

    b2-7       2 vCPU   7GB RAM    general purpose, cost-effective
    b2-15      4 vCPU  15GB RAM    higher terrain count runs
    c2-7       2 vCPU   7GB RAM    compute optimized
    c2-15      4 vCPU  15GB RAM    compute optimized, high pack count

---

## Default Image Configuration

AMIOS carries the OpenStack image name. The data source searches for
the most recent image matching this name with os_distro rocky.

Default for Rocky Linux 9:

    AMIOS: Rocky Linux 9

To use Ubuntu instead:

    AMIOS:      Ubuntu 22.04
    RemoteUser: ubuntu

Note that on OVH the image name must match exactly what is available
in the target region. Image availability varies by region.

---

## Cost Advantage

OVH public cloud compute is significantly less expensive than AWS or
DigitalOcean for equivalent vCPU and RAM. A b2-7 instance (2 vCPU,
7GB RAM) costs approximately EUR 0.025/hour making it the most
cost-effective option for large sovereign runs spanning many planets
and long durations.

---

## Import Path

    import tfovh "github.com/andreimerlescu/sovereign/pkg/terraform_provider/ovh"

---

## Usage in sovereign

    provider, err := tfovh.New()
    if err != nil {
        // missing OVH or OpenStack credentials
    }

The newProvider factory in sovereign.go handles this automatically
when --provider ovh is set.​​​​​​​​​​​​​​​​
