# CLAUDE.md

## Project: Sovereign

Sovereign is a distributed load testing coordinator. It provisions cloud instances (planets) across multiple regions, deploys lemmings (a load testing binary) on each, collects results, aggregates them into a unified report, and destroys the infrastructure when done.

---

## Architecture Overview

    sovereign (main package)
        |
        |-- pkg/terraform_provider/         interface + shared types
        |-- pkg/terraform_provider/aws/     AWS implementation (only complete provider)
        |-- pkg/terraform_provider/do/      stub
        |-- pkg/terraform_provider/gcp/     stub
        |-- pkg/terraform_provider/oci/     stub
        |-- pkg/terraform_provider/ovh/     stub

Sovereign calls terraform directly via exec. No topobuilder dependency. No Docker. No load balancer. No DNS.

---

## File Map

    main.go              CLI flags, boot summary, wires cfg -> NewSovereign -> Run
    sovereign.go         SovereignConfig, Planet, Sovereign struct, NewSovereign, Run lifecycle, newProvider factory
    terrain.go           provisionTerrain, provisionPlanet, destroyTerrain, destroyPlanet, resume logic, planetSpec, hclSpec
    lemmings.go          runLemmings, collectResults, buildLemmingsCommand, copyPlanFile, SSH/SCP calls
    ledger.go            Ledger, LedgerEvent, LedgerEventType, NewLedger, LoadLedger, Append
    state.go             saveState, loadState, cleanState, resolveStateFile, planetState
    report.go            aggregateReport, readPlanetReport, renderUnifiedMarkdown, renderUnifiedHTML, deliverReport, writeReport
    cost.go              estimateCost, fetchHourlyRate, dollarToMicrocents, microcentsToDisplay
    config.go            SovereignConfigFile, loadConfigFile, mergeConfigFile, validateConfig, exampleConfigFile
    template.go          DELETED -- replaced by pkg/terraform_provider/aws/hcl.go

    Test files mirror their source files 1:1.
    main_test.go holds all shared test helpers, mock servers, assertion helpers.
    terrain_test.go defines MockTerraformProvider.

---

## Key Design Decisions

### No Circular Dependencies

The provider packages never import the main sovereign package. The handoff between sovereign
and providers uses two types defined in pkg/terraform_provider:

    PlanetSpec    -- what a provider needs to operate (workdir, region, zone, planet ID)
    HCLSpec       -- what a provider needs to render HCL (all infrastructure parameters)

Sovereign constructs these from *Planet before calling any provider method. Providers never
see *Planet.

### Provider Interface

Every cloud provider implements TerraformProvider from pkg/terraform_provider/provider.go:

    Name() string
    Validate() error
    RenderHCL(spec HCLSpec) error
    Init(ctx, spec PlanetSpec) error
    InitOrApply(ctx, spec PlanetSpec) error
    Apply(ctx, spec PlanetSpec) error
    Destroy(ctx, spec PlanetSpec) error
    Output(ctx, spec PlanetSpec) (*ProvisionOutput, error)
    Status(ctx, spec PlanetSpec) (LifecycleStatus, error)
    StateBackend(spec PlanetSpec) (string, error)
    LoadState(spec PlanetSpec) (*ProviderState, error)
    SaveState(spec PlanetSpec, state *ProviderState) error
    UnloadState(spec PlanetSpec) error

Each provider owns its own HCL templates. Sovereign owns the values.

### Stateless Providers

TerraformProviderAWS holds only credentials. All per-planet data flows through PlanetSpec
and HCLSpec arguments. Safe for concurrent planet provisioning.

### Credentials -- ENV Only

AWS credentials are passed to terraform subprocess via environment variables only. They are
never written into HCL files. This means credential values never appear in backup files.

    AWS_ACCESS_KEY_ID       -> set in subprocess env
    AWS_SECRET_ACCESS_KEY   -> set in subprocess env

Required sovereign environment variables:

    AWS_ACCESS_KEY
    AWS_ACCESS_SECRET_KEY
    SOVEREIGN_SUBNET_ID
    SOVEREIGN_SECURITY_GROUP

### Rocky Linux Defaults

Sovereign defaults to Rocky Linux, not Ubuntu. SurgePays used Ubuntu -- that was a
customer-specific choice baked into topobuilder. Sovereign's defaults:

    RemoteUser:  rocky
    AMIOS:       Rocky-9-EC2-Base-9.*-x86_64*
    AMIOwners:   792107900819

The AMIOwners value is the Rocky Linux Foundation's AWS account ID, not "amazon". Wrong
owner ID means terraform finds nothing or an impostor image.

Every SSH and SCP call uses planet.RemoteUser, never a hardcoded string. Planet.RemoteUser
is populated in initPlanets from cfg.RemoteUser.

### Append-Only Ledger

The ledger is the audit log for a sovereign run. It is separate from the state file. One
ledger per run, all planets write to it under a mutex. Every Append flushes to disk
immediately.

    ~/.sovereignd/runs/<2006-01-02T15-04-05>/ledger.yml

The ledger is the source of truth for resume logic. On restart:

    PlanetCompleted(id)    -- skip entirely, already has collect complete event
    PlanetProvisioned(id)  -- skip provision, load from ProviderState and continue
    LastEventForPlanet(id) -- determine where in the lifecycle it stopped

### ProviderState vs SovereignState

Two separate state mechanisms:

    SovereignState   -- ~/.sovereignd/state.yml
                    -- planet slice, config, run metadata

    ProviderState    -- ~/.sovereignd/planets/planet-N/provider_state.yml
                    -- per-planet terraform lifecycle status

ProviderState carries LifecycleStatus which is the terraform-specific state machine:

    uninitialized -> initialized -> applying -> applied -> destroying -> destroyed
                                             -> interrupted (resume with InitOrApply)
                                             -> failed (start over)

### Microcent Arithmetic

All monetary values are int64 microcents. Float64 never stored.

    1 dollar  = 100,000 microcents
    1 cent    = 1,000 microcents

Display functions: microcentsToDisplay (2 decimal places), microcentsToDetailedDisplay
(5 decimal places). printCostEstimate uses these -- never fmt.Sprintf with %f on int64.

### First Planet Alone

The first planet always provisions alone before remaining planets provision in concurrent
batches of 3. This mirrors topobuilder's own internal zone sequencing requirement.
provisionTerrain enforces this. destroyTerrain is fully concurrent with no semaphore.

---

## Runtime Directory Layout

    ~/.sovereignd/
    |-- state.yml
    |-- runs/
    |   `-- 2026-04-22T14-30-00/
    |       |-- ledger.yml
    |       `-- summary.yml
    `-- planets/
        `-- planet-N/
            |-- provider_state.yml
            |-- terraform/
            |   |-- provider.tf
            |   |-- resources_compute.tf
            |   |-- resources_network.tf
            |   |-- data.tf
            |   |-- output.tf
            |   |-- .terraform/
            |   |-- .terraform.lock.hcl
            |   `-- terraform.tfstate
            |-- backups/
            |   `-- 2026-04-22T14-30-00/
            |       |-- provider.tf
            |       |-- resources_compute.tf
            |       `-- ...
            |-- lemmings.plan.yml
            `-- reports/
                `-- lemmings/

---

## SovereignConfig Fields

Core:

    Hit              string          target URL
    Planets          int             number of planets
    Terrain          int64           terrain groups per planet
    Pack             int64           lemmings per terrain
    Until            time.Duration   how long each lemming lives
    Ramp             time.Duration   ramp up duration
    Plan             string          path to lemmings plan YAML (optional)
    SaveTo           []string        report delivery destinations

Infrastructure:

    Provider         string          aws | digitalocean | gcp | oci | ovh
    Regions          []string        round-robin assigned to planets
    InstanceType     string          REQUIRED, no default (cost-sensitive)
    ProvisionTimeout time.Duration
    TerminateTimeout time.Duration
    Yes              bool            skip confirmation prompt
    DryRun           bool            estimate cost only

Remote instance:

    RemoteUser       string          default: rocky

AMI:

    AMIOS            string          default: Rocky-9-EC2-Base-9.*-x86_64*
    AMIOwners        string          default: 792107900819

Go and lemmings:

    GoVersion        string          default: 1.25.0
    LemmingsPackage  string          default: github.com/andreimerlescu/lemmings@latest

EBS volumes:

    RootVolumeSize   int             default: 30
    RootVolumeType   string          default: gp2 (valid: gp2, gp3, io2)
    TempVolumeSize   int             default: 10
    TempVolumeType   string          default: gp2

---

## Validation Rules

InstanceType must not be empty -- no safe default since it determines cost.
RemoteUser must not be empty.
AMIOS must not be empty.
AMIOwners must not be empty.
GoVersion must not be empty.
LemmingsPackage must not be empty.
RootVolumeSize and TempVolumeSize must be greater than zero.
RootVolumeType and TempVolumeType must be one of: gp2, gp3, io2.
Plan path must exist on disk if specified.
SaveTo must contain no empty strings.

---

## Test Conventions

All shared helpers live in main_test.go:

    testConfig()                          returns minimal valid SovereignConfig with all fields
    testSovereign(t)                      returns Sovereign with ledger initialized
    testSovereignWithPlanets(t, ps)       returns Sovereign with specific planet slice
    testSovereignWithMockProvider(t, m)   returns Sovereign with MockTerraformProvider
    testPlanet(id)                        PlanetStatusPending
    testProvisionedPlanet(id)             PlanetStatusRunning with IP and instance ID
    testDonePlanet(t, id)                 PlanetStatusDone with temp ReportPath
    testLostPlanet(id)                    PlanetStatusLost
    benchmarkSovereign(b)                 Sovereign with ledger for benchmark use
    setTestEnv(t)                         sets AWS_ACCESS_KEY, AWS_ACCESS_SECRET_KEY,
                                          SOVEREIGN_SUBNET_ID, SOVEREIGN_SECURITY_GROUP
    writeMockLemmingsReport(t, dir, id)
    newMockPricingServer(t)
    newMockPricingServerEmpty(t)
    newMockPricingServerError(t)
    requireNoError, requireError, requireEqual
    requireNotNil, requireTrue, requireFalse

MockTerraformProvider lives in terrain_test.go. It implements TerraformProvider with
configurable error injection and call recording fields:

    renderErr, initErr, applyErr, destroyErr, outputErr, stateErr
    outputResult, stateResult, statusResult
    initCalls, applyCalls, destroyCalls, outputCalls
    savedStates, renderedSpecs

containsStr helper lives in lemmings_test.go. Used by lemmings_test.go and terrain_test.go.

testUnifiedReport helper lives in report_test.go. Accepts nil *testing.T for benchmark use.

testSovereignWithMockProvider lives in terrain_test.go, not main_test.go.

---

## What Is Not Yet Implemented

Commander interface (MockCommander, RealCommander) defined in main_test.go is not wired
into production code. SSH and SCP calls in lemmings.go use exec.CommandContext directly.
Lemmings SSH/SCP paths are integration-testable only until Commander is wired in.

Provider stubs -- do, gcp, oci, ovh -- return "not yet implemented" for every method.
They satisfy the interface and are registered in newProvider but do no real work.

summary.yml under runs/<runID>/ is referenced in the directory layout but not yet written
by any code. It is reserved for post-run summary output.

---

## Common Mistakes to Avoid

Do not hardcode "ubuntu@" anywhere. Always use planet.RemoteUser.

Do not hardcode "/home/ubuntu/" anywhere. Use fmt.Sprintf("/home/%s/", planet.RemoteUser).

Do not use float64 for currency. All monetary values are int64 microcents.

Do not write AWS credentials into HCL files. Pass via subprocess env() method only.

Do not add loadState or saveState to sovereign.go. state.go owns these methods.

Do not add aggregateReport to lemmings.go. report.go owns aggregateReport.

Do not reference topobuilder anywhere. It has been replaced entirely by direct terraform
invocation through the TerraformProvider interface.

Do not add implementation logic to pkg/terraform_provider/provider.go. It is a types and
interface definition file only. Implementation belongs in the provider-specific packages.

Do not use GitLab environment variables. The GitLab Terraform backend has been removed.
Sovereign uses local terraform state only.

Do not import the main sovereign package from any provider package. The boundary is
enforced by PlanetSpec and HCLSpec as the only handoff types.

---

## Behavioral Guidelines (CLAUDE.md standard)

Think before coding. State assumptions explicitly before implementing anything. If multiple
interpretations exist, present them rather than picking silently. If something is unclear,
stop and ask.

Simplicity first. Minimum code that solves the problem. No features beyond what was asked.
No abstractions for single-use code. No speculative flexibility.

Surgical changes. Touch only what the request requires. Do not improve adjacent code. Match
existing style. If a change creates orphaned imports or variables, remove only those orphans
created by the change.

Goal-driven execution. For multi-step tasks state a brief plan with verifiable steps before
writing code. Strong success criteria allow independent verification.

When asked to produce a complete file, produce the entire file. Do not abbreviate with
comments like "rest of file unchanged". The file must be complete and correct as written.

When asked to make a surgical change, touch only the specified lines. State exactly what
changed and why. Do not reformat, rename, or reorganize anything else in the file.​​​​​​​​​​​​​​​​
