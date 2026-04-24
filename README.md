# Sovereign

> The command center for distributed lemmings load testing.
> Sovereign provisions terrain, deploys agents, collects results,
> and renders unified reports across geographic origins.

[![Go](https://github.com/andreimerlescu/sovereign/actions/workflows/sovereign.yml/badge.svg)](https://github.com/andreimerlescu/sovereign/actions/workflows/sovereign.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/andreimerlescu/sovereign)](https://goreportcard.com/report/github.com/andreimerlescu/sovereign)
[![GoDoc](https://pkg.go.dev/badge/github.com/andreimerlescu/sovereign.svg)](https://pkg.go.dev/github.com/andreimerlescu/sovereign)
[![Go Version](https://img.shields.io/badge/go-1.25.0-blue)](https://go.dev/doc/go1.25)
[![Release](https://img.shields.io/github/v/release/andreimerlescu/sovereign)](https://github.com/andreimerlescu/sovereign/releases)

---

## What Sovereign Is

Sovereign is the engineer's command center. It sits above lemmings
and coordinates distributed load tests that no single machine can
perform alone.

A single lemmings instance tells you how your application performs
under load from one origin. Sovereign tells you how your application
performs under load from every origin simultaneously -- multiple
regions, multiple cloud providers, hundreds of thousands of lemmings
running concurrently across the globe -- with a single command.

Sovereign provisions cloud infrastructure directly via Terraform. It 
installs lemmings on each planet, runs the load test, collects results, 
and destroys the infrastructure when done.

---

## How It Works

    sovereign
        |
        |-- terraform (creates terrain per planet)
        |       |
        |       |-- planet 1 -- lemmings (us-east-1)
        |       |-- planet 2 -- lemmings (us-west-1)
        |       |-- planet 3 -- lemmings (eu-west-1)
        |       |-- planet 4 -- lemmings (ap-southeast-1)
        |       |-- planet 5 -- lemmings (sa-east-1)
        |
        |-- waits for all planets to complete
        |
        |-- collects LifeLogs via SCP
        |
        |-- aggregates into unified report
        |
        |-- terraform destroy (per planet, concurrent)

lemmings never knows sovereign exists.
sovereign knows everything.

---

## The Lifecycle

### 1. Cost Estimation

Before provisioning any infrastructure, sovereign queries
instances.vantage.sh for current on-demand pricing for each planet's
region and instance type. The estimate is printed and confirmation is
requested before any infrastructure is created. The --yes flag skips
confirmation for automated pipelines. The --dry-run flag exits after
the estimate without provisioning.

### 2. Terrain Provisioning

Sovereign generates Terraform HCL for each planet and calls
terraform init and terraform apply. The first planet provisions
alone. Remaining planets provision in concurrent batches of 3.
A planet that fails to provision is marked lost and skipped in
subsequent phases. Lost planets are noted in the final report.
The run continues with surviving planets.

Each planet gets its own working directory:

    ~/.sovereignd/planets/planet-N/terraform/

Terraform state is local per planet. After every successful apply
the HCL files are backed up to a timestamped directory so the exact
terraform that provisioned each instance is never lost.

### 3. Lemmings Deployment

Sovereign SSHes into each planet and runs lemmings with the
configured flags. If a plan file is provided it is SCPed to the
planet before lemmings starts. Lemmings runs block until completion
or the deadline (ramp + until + 5 minutes overhead) is exceeded.

### 4. Result Collection

When each planet completes its lemmings run, sovereign SCPs the
report files back to the planet's local reports directory. Collection
runs concurrently across all non-lost planets. A collection failure
marks the planet lost but does not block collection from others.

### 5. Report Aggregation

Sovereign reads the lemmings YAML report from each completed planet,
merges all results into a unified report, and delivers it to all
configured save-to destinations as both markdown and HTML.

The unified report includes total visits across all planets and all
terrains, per-planet breakdown showing geographic performance
differences, per-path breakdown aggregated globally, timing
percentiles at global and per-planet level, waiting room metrics
aggregated across all origins, lost planet summary with instance IDs
for manual intervention, and projected cost based on instance hours
consumed.

### 6. Terrain Destruction

All planets are destroyed concurrently via terraform destroy.
Sovereign waits for confirmation that all instances have been
terminated before exiting. If any planet fails to terminate within
the timeout, sovereign logs the instance IDs that require manual
intervention and exits with a non-zero status.

---

## State and Resume

Sovereign writes two kinds of state to survive crashes and restarts.

The sovereign state file records the planet slice with instance IDs:

    ~/.sovereignd/state.yml

The provider state file records the terraform lifecycle status per planet:

    ~/.sovereignd/planets/planet-N/provider_state.yml

The ledger is an append-only audit log of every lifecycle event:

    ~/.sovereignd/runs/<timestamp>/ledger.yml

On restart, sovereign reads the ledger to determine what each planet
has already accomplished and routes accordingly. A planet with a
collect complete event is skipped entirely. A planet with a provision
complete event skips provisioning and resumes from lemmings execution.
A planet with an interrupted apply resumes with terraform apply. A
planet with no usable state provisions from scratch.

---

## Cloud Providers

Sovereign uses a TerraformProvider interface. Each cloud provider
generates its own Terraform HCL and implements the same lifecycle
contract. The provider is selected via the --provider flag.

Currently implemented providers:

    aws            fully implemented
    digitalocean   fully implemented
    ovh            fully implemented

Providers that have been removed and are not available:

    gcp            removed -- enterprise on-demand only
    oci            removed -- enterprise on-demand only

Passing --provider gcp or --provider oci will return an error at
startup. Do not add stub packages for these providers without a
full implementation.

---

## Quick Start

Install sovereign:

    go install github.com/andreimerlescu/sovereign@latest

Ensure terraform is in PATH:

    terraform -version

Set required environment variables for AWS:

    export AWS_ACCESS_KEY=...
    export AWS_ACCESS_SECRET_KEY=...
    export SOVEREIGN_SUBNET_ID=subnet-...
    export SOVEREIGN_SECURITY_GROUP=sg-...

Estimate cost without provisioning:

    sovereign \
        --hit https://myapp.com \
        --planets 5 \
        --terrain 50 \
        --pack 50 \
        --until 30s \
        --ramp 5m \
        --instance c6a.large \
        --dry-run

Run a distributed load test across five planets:

    sovereign \
        --hit https://myapp.com \
        --planets 5 \
        --terrain 50 \
        --pack 50 \
        --until 30s \
        --ramp 5m \
        --instance c6a.large \
        --save-to .

Run with a plan file across three planets:

    sovereign \
        --hit https://myapp.com \
        --planets 3 \
        --terrain 20 \
        --pack 20 \
        --until 30s \
        --plan lemmings.plan.yml \
        --instance c6a.large \
        --save-to /tmp/sovereign-reports

---

## All Flags

### Core

    --hit, -h              Target URL for lemmings to load test
    --planets, -p          Number of cloud instances to provision
    --terrain, -t          Number of terrain groups per planet
    --pack, -pk            Number of lemmings per terrain per planet
    --until, -u            How long each lemming lives
    --ramp, -r             Duration to bring all terrains online per planet
    --plan, -pl            Path to a lemmings plan YAML file
    --save-to, -s          Report delivery destinations

### Infrastructure

    --provider, -pv        Cloud provider: aws, digitalocean, ovh
    --region, -rg          Regions to deploy into (repeatable)
    --instance, -i         Instance type per planet -- REQUIRED
    --yes, -y              Skip cost confirmation prompt
    --dry-run, -dr         Estimate cost and exit without provisioning
    --provision-timeout    Maximum time to provision one planet (default 5m)
    --terminate-timeout    Maximum time to terminate one planet (default 5m)

### Remote Instance

    --remote-user, -ru     SSH user on provisioned instances (default: rocky)

### AMI

    --ami-os, -ao          AMI OS search pattern (default: Rocky-9-EC2-Base-9.*-x86_64*)
    --ami-owners, -aw      AMI owners account ID (default: 792107900819)

### Go and Lemmings

    --go-version, -gv      Go version to install via igo (default: 1.25.0)
    --lemmings-package     Lemmings package path for go install

### EBS Volumes

    --root-volume-size     Root EBS volume size in GB (default: 30)
    --root-volume-type     Root EBS volume type: gp2, gp3, io2 (default: gp2)
    --temp-volume-size     Temp EBS volume size in GB (default: 10)
    --temp-volume-type     Temp EBS volume type: gp2, gp3, io2 (default: gp2)

### Config File

    --config, -c           Path to a sovereign YAML config file

---

## The Sovereign Configuration File

For repeatable runs, sovereign accepts a YAML configuration file
via --config. All flags can be expressed in the config file. Flags
always override config file values when both are present. The merge
strategy uses zero-value detection: a config field is only merged
from the file if the corresponding struct field is its zero value
(empty string, zero int, zero duration). This means any flag
explicitly set by the operator -- even one that equals a default --
is preserved and never overwritten.

    sovereign --config sovereign.yml

Example sovereign.yml:

    Hit: https://myapp.com
    Planets: 5
    Terrain: 50
    Pack: 50
    Until: 30s
    Ramp: 5m
    Plan: lemmings.plan.yml

    Provider: aws
    Regions:
      - us-east-1
      - us-west-1
      - eu-west-1
      - ap-southeast-1
      - sa-east-1

    InstanceType: c6a.large

    RemoteUser: rocky
    AMIOS: "Rocky-9-EC2-Base-9.*-x86_64*"
    AMIOwners: "792107900819"

    GoVersion: "1.25.0"
    LemmingsPackage: "github.com/andreimerlescu/lemmings@latest"

    RootVolumeSize: 30
    RootVolumeType: gp2
    TempVolumeSize: 10
    TempVolumeType: gp2

    ProvisionTimeout: 5m
    TerminateTimeout: 5m

    SaveTo:
      - .
      - s3://my-bucket/sovereign-reports

---

## Environment Variables

Required for AWS:

    AWS_ACCESS_KEY              AWS access key ID
    AWS_ACCESS_SECRET_KEY       AWS secret access key
    SOVEREIGN_SUBNET_ID         Subnet ID for planet instances
    SOVEREIGN_SECURITY_GROUP    Security group ID for planet instances

Required for DigitalOcean:

    DIGITALOCEAN_TOKEN          DigitalOcean personal access token

Required for OVH:

    OVH_ENDPOINT                API endpoint: ovh-eu, ovh-us, ovh-ca
    OVH_APPLICATION_KEY         OVH application key
    OVH_APPLICATION_SECRET      OVH application secret
    OVH_CONSUMER_KEY            OVH consumer key
    OS_AUTH_URL                 OpenStack identity endpoint
    OS_TENANT_ID                OpenStack project ID
    OS_TENANT_NAME              OpenStack project name
    OS_USERNAME                 OpenStack username
    OS_PASSWORD                 OpenStack password
    OS_REGION_NAME              OpenStack region name

Credentials are never written to disk or into Terraform HCL files.
They are passed to the terraform subprocess via environment variables.

---

## Dependencies

Runtime dependencies that must be present in PATH:

    terraform    infrastructure provisioning
    ssh          lemmings execution on remote planets
    scp          plan file delivery and result collection

Lemmings is installed on each planet at provisioning time via:

    go install github.com/andreimerlescu/lemmings@latest

Cost estimation uses the public instances.vantage.sh API. No API key
is required. A pricing failure for a planet is non-fatal.

---

## Makefile

    make all             lint + test + build
    make build           compile the sovereign binary
    make install         go install to GOPATH/bin
    make test            unit tests with race detector
    make test-fuzz       fuzz all fuzz targets
    make test-bench      benchmark tests
    make test-all        unit + fuzz + bench
    make test-cover      unit tests with HTML coverage report
    make lint            go vet + gofmt
    make clean           remove binary and coverage output
    make tidy            go mod tidy + go mod verify
    make dry-run         estimate cost without provisioning
    make run-local       run against localhost with minimal config
    make clean-state     remove .sovereignd state directory
    make show-state      print current sovereign state file
    make show-ledger     print most recent run ledger

---

## The Name

A lemming is a sovereign's agent. It is born with a predetermined
destiny. It follows its plan. It reports back. It has no free will
of its own. The sovereign is the engineer -- the one who decides
the plan, commands the terrain, and reads the report.

Sovereign is the application that makes the engineer's intent real
across the globe.
