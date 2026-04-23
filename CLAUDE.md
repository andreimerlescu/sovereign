# CLAUDE.md

## Project: Sovereign

Sovereign is a distributed load testing coordinator. It provisions
cloud instances (planets) across multiple regions, deploys lemmings
on each, collects results, aggregates them into a unified report,
and destroys the infrastructure when done.

---

## Architecture Overview

    sovereign (main package)
        |
        |-- pkg/events/                  sovereign event bus
        |-- pkg/ledger/                  append-only audit log
        |-- pkg/pricing/                 Pricer interface + VantagePricer + microcents
        |-- pkg/terraform_exeggutor/     shared terraform subprocess execution
        |-- pkg/terraform_provider/      TerraformProvider interface + shared types
        |-- pkg/terraform_provider/aws/  AWS implementation (complete)
        |-- pkg/terraform_provider/do/   DigitalOcean implementation (complete)
        |-- pkg/terraform_provider/ovh/  OVH implementation (complete)
        |-- pkg/terraform_provider/oci/  stub -- enterprise on-demand
        |-- pkg/terraform_provider/gcp/  stub -- enterprise on-demand

Sovereign calls terraform directly via exec. No Docker. No load balancer. No DNS.

---

## File Map

Main package:

    main.go          CLI flags, boot summary, wires cfg -> NewSovereign -> Run
    sovereign.go     SovereignConfig, Planet, Sovereign struct, NewSovereign,
                     Run lifecycle, newProvider factory, initPlanets,
                     defaultRegions, printCostEstimate, printFinalSummary
    terrain.go       provisionTerrain, provisionPlanet, destroyTerrain,
                     destroyPlanet, resume logic, planetSpec, hclSpec,
                     markLost, saveProviderState, planetWorkDir
    lemmings.go      runLemmings, collectResults, buildLemmingsCommand,
                     copyPlanFile, planetSSHKeyPath, planetReportDir,
                     completedPlanets
    cost.go          estimateCost (calls pkg/pricing), totalLemmings,
                     CostEstimate, PlanetEstimate
    ledger.go        DELETED -- replaced by pkg/ledger
    state.go         saveState, loadState, cleanState, resolveStateFile,
                     planetState, lostPlanets, pendingPlanets,
                     allPlanetsTerminal
    report.go        aggregateReport, readPlanetReport, renderUnifiedMarkdown,
                     renderUnifiedHTML, deliverReport, writeReport,
                     percentile, domainFromURL, formatBytes
    config.go        SovereignConfigFile, loadConfigFile, mergeConfigFile,
                     validateConfig, exampleConfigFile
    template.go      DELETED -- replaced by pkg/terraform_provider/*/hcl.go

Test files:

    main_test.go     all shared test helpers, assertion helpers,
                     mock pricing JSON server, MockCommander
    cost_test.go     estimateCost tests, totalLemmings tests
    config_test.go   loadConfigFile, mergeConfigFile, validateConfig tests
    state_test.go    saveState, loadState, cleanState, resolveStateFile tests
    ledger_test.go   DELETED -- replaced by pkg/ledger/ledger_test.go
    terrain_test.go  MockTerraformProvider, provisionPlanet, destroyPlanet,
                     planetSpec, hclSpec, markLost, saveProviderState tests
    lemmings_test.go buildLemmingsCommand, completedPlanets,
                     planetReportDir, planetSSHKeyPath tests
    report_test.go   renderUnifiedMarkdown, renderUnifiedHTML,
                     readPlanetReport, writeReport, percentile,
                     domainFromURL, formatBytes tests
    sovereign_test.go initPlanets, newProvider, lostPlanets,
                     pendingPlanets, allPlanetsTerminal tests

Package files:

    pkg/events/events.go          EventBus, Event, Subscriber, Filter, Tee
    pkg/events/kinds.go           all sovereign EventKind constants
    pkg/events/events_test.go

    pkg/ledger/ledger.go          Ledger, Entry, NewLedger, LoadLedger,
                                  Append, AsSubscriber, LastEntryForPlanet,
                                  PlanetCompleted, PlanetProvisioned
    pkg/ledger/ledger_test.go

    pkg/pricing/pricing.go        Pricer interface, PriceQuery, PriceResult,
                                  EstimateConfig, Estimate, EstimateCost,
                                  FormatQuery
    pkg/pricing/microcents.go     DollarToMicrocents, MicrocentsToDisplay,
                                  MicrocentsToDetailedDisplay,
                                  MicroCentsPerDollar, MicroCentsPerCent
    pkg/pricing/vantage.go        VantagePricer, NewVantagePricer,
                                  NewVantagePricerWithURL
    pkg/pricing/pricing_test.go

    pkg/terraform_exeggutor/exeggutor.go   Exeggutor, New, Init, Apply,
                                           Destroy, InitOrApply, run,
                                           Validate, StateFileExists,
                                           DotTerraformExists
    pkg/terraform_exeggutor/output.go      Result, ParseOutput
    pkg/terraform_exeggutor/status.go      ParseStatus, showState
    pkg/terraform_exeggutor/exeggutor_test.go

    pkg/terraform_provider/provider.go     TerraformProvider interface,
                                           PlanetSpec, HCLSpec,
                                           ProvisionOutput, LifecycleStatus,
                                           ProviderState, HCLBackup

    pkg/terraform_provider/aws/auth.go     Credentials, Resolve
    pkg/terraform_provider/aws/hcl.go      renderHCL, HCL templates
    pkg/terraform_provider/aws/provider.go TerraformProviderAWS

    pkg/terraform_provider/do/auth.go      Credentials, Resolve
    pkg/terraform_provider/do/hcl.go       renderHCL, HCL templates
    pkg/terraform_provider/do/provider.go  TerraformProviderDigitalOcean

    pkg/terraform_provider/ovh/auth.go     Credentials, Resolve
    pkg/terraform_provider/ovh/hcl.go      renderHCL, HCL templates
    pkg/terraform_provider/ovh/provider.go TerraformProviderOVH

    pkg/terraform_provider/oci/provider.go TerraformProviderOCI (stub)
    pkg/terraform_provider/gcp/provider.go TerraformProviderGCP (stub)

---

## Key Design Decisions

### Event Bus as Nervous System

Sovereign has its own event bus in pkg/events inspired by lemmings'
event bus but with a different contract. Sovereign's bus is
lifecycle-scoped, planet-aware, and permits the ledger subscriber
to flush to disk on every event because sovereign events are
infrequent.

The ledger is wired as the first bus subscriber in NewSovereign:

    s.bus = events.NewEventBus()
    s.bus.Subscribe(s.ledger.AsSubscriber())

terrain.go and lemmings.go emit to the bus. They never call the
ledger directly. The bus fans out to all subscribers.

### No Circular Dependencies

Package import rules enforced strictly:

    pkg/events              -- stdlib only
    pkg/ledger              -- pkg/events + yaml.v3 + stdlib
    pkg/pricing             -- stdlib + net/http only
    pkg/terraform_exeggutor -- pkg/terraform_provider (LifecycleStatus
                               only) + stdlib + os/exec
    pkg/terraform_provider  -- stdlib + yaml.v3 only
    pkg/terraform_provider/* -- pkg/terraform_provider +
                                pkg/terraform_exeggutor + yaml.v3 +
                                stdlib
    sovereign main          -- all of the above

No provider package ever imports sovereign main. No events or ledger
package imports anything from sovereign main.

### Provider Interface and Handoff Types

Providers never see *Planet. The handoff between sovereign and
providers uses two types defined in pkg/terraform_provider:

    PlanetSpec    -- what a provider needs to operate
                     (PlanetID, WorkDir, Region, Zone)
    HCLSpec       -- what a provider needs to render HCL
                     (all infrastructure parameters)

Sovereign constructs these in terrain.go via planetSpec() and
hclSpec() before calling any provider method.

### Stateless Provider Structs

Provider structs hold only credentials resolved at New() time.
Per-planet data flows through PlanetSpec and HCLSpec arguments.
Safe for concurrent planet provisioning without additional locking.

### terraform_exeggutor Pattern

Every provider delegates lifecycle methods to an Exeggutor:

    func (p *TerraformProviderAWS) Apply(
        ctx context.Context,
        spec tfprovider.PlanetSpec,
    ) error {
        return exeggutor.New(
            p.Name(), spec.PlanetID, spec.WorkDir, p.env(),
        ).Apply(ctx)
    }

Exeggutor is constructed per operation and discarded after use.
It owns Init, Apply, Destroy, InitOrApply, ParseOutput, ParseStatus,
StateFileExists, DotTerraformExists.

Resource type strings per provider:

    aws:          "aws_instance"
    digitalocean: "digitalocean_droplet"
    ovh:          "openstack_compute_instance_v2"

### Credentials via Environment Only

No provider writes credentials into HCL files. Credentials are
passed to terraform subprocess calls via the env() helper method.
They never appear in rendered HCL, backup directories, or terraform
plan output.

AWS subprocess environment:

    AWS_ACCESS_KEY_ID        mapped from AWS_ACCESS_KEY
    AWS_SECRET_ACCESS_KEY    mapped from AWS_ACCESS_SECRET_KEY

Required sovereign environment variables:

    AWS_ACCESS_KEY
    AWS_ACCESS_SECRET_KEY
    SOVEREIGN_SUBNET_ID
    SOVEREIGN_SECURITY_GROUP

### Rocky Linux Defaults

Sovereign defaults to Rocky Linux, not Ubuntu. Every SSH and SCP
call uses planet.RemoteUser, never a hardcoded string. Planet.RemoteUser
is populated in initPlanets from cfg.RemoteUser.

    RemoteUser:  rocky
    AMIOS:       Rocky-9-EC2-Base-9.*-x86_64*
    AMIOwners:   792107900819 (Rocky Linux Foundation AWS account ID)

### Ledger as Crash Recovery

Two separate state mechanisms:

    SovereignState   -- ~/.sovereignd/state.yml
                       planet slice, config, run metadata

    ProviderState    -- ~/.sovereignd/planets/planet-N/provider_state.yml
                       per-planet terraform lifecycle status

    Ledger           -- ~/.sovereignd/runs/<timestamp>/ledger.yml
                       append-only audit log, source of truth for resume

On restart sovereign reads the ledger first:

    PlanetCompleted(id)    -- KindCollectComplete exists -- skip planet
    PlanetProvisioned(id)  -- KindProvisionComplete exists -- skip
                              provisioning, resume from lemmings
    LastEntryForPlanet(id) -- determine exact interruption point

### Microcent Arithmetic

All monetary values are int64 microcents. Float64 never stored.
All arithmetic and display functions live in pkg/pricing/microcents.go.

    MicroCentsPerDollar int64 = 100,000
    MicroCentsPerCent   int64 = 1,000

printCostEstimate in sovereign.go uses:

    pricing.MicrocentsToDetailedDisplay(p.HourlyRate)
    pricing.MicrocentsToDisplay(estimate.TotalCost)

### First Planet Alone

provisionTerrain always provisions planet 0 alone before remaining
planets provision in concurrent batches of 3. destroyTerrain is
fully concurrent with no semaphore.

---

## Runtime Directory Layout

    ~/.sovereignd/
    |-- state.yml
    |-- runs/
    |   `-- 2026-04-22T14-30-00/
    |       `-- ledger.yml
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
            |       `-- *.tf
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

    Provider         string          aws | digitalocean | ovh
                                     (oci | gcp are stubs)
    Regions          []string        round-robin assigned to planets
    InstanceType     string          REQUIRED -- no default
    ProvisionTimeout time.Duration
    TerminateTimeout time.Duration
    Yes              bool            skip confirmation prompt
    DryRun           bool            estimate cost only

Remote instance:

    RemoteUser       string          default: rocky

AMI / image:

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

InstanceType must not be empty.
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

## Default Regions Per Provider

aws:

    us-east-1, us-west-1, eu-west-1, ap-southeast-1, sa-east-1

digitalocean:

    nyc1, sfo3, ams3, sgp1, lon1

ovh:

    GRA11, SBG5, DE1, UK1, SGP1

---

## Test Conventions

All shared helpers live in main_test.go:

    testConfig()
        returns minimal valid SovereignConfig with all fields set
        to defaults including new fields added this session

    testSovereign(t)
        constructs Sovereign with:
        - events.NewEventBus() wired to ledger.AsSubscriber()
        - ledger.NewLedger() in t.TempDir()
        - t.Cleanup(bus.Close) registered
        - planets initialized via initPlanets()

    testSovereignWithPlanets(t, planets)
        testSovereign with specific planet slice

    testSovereignWithMockProvider(t, mock)
        defined in terrain_test.go not main_test.go

    testPlanet(id)              PlanetStatusPending
    testProvisionedPlanet(id)   PlanetStatusRunning with IP and instance ID
    testDonePlanet(t, id)       PlanetStatusDone with temp ReportPath
    testLostPlanet(id)          PlanetStatusLost

    benchmarkSovereign(b)
        mirrors testSovereign for benchmark use with testing.B

    setTestEnv(t)
        sets AWS_ACCESS_KEY, AWS_ACCESS_SECRET_KEY,
        SOVEREIGN_SUBNET_ID, SOVEREIGN_SECURITY_GROUP

    newMockPricingJSONServer(t, hourlyRateDollars)
        httptest.Server returning JSON pricing response
        constructs JSON as format string -- no dependency on
        unexported vantageResponse type in pkg/pricing

    newMockPricingJSONServerError(t)
        httptest.Server returning HTTP 500

    writeMockLemmingsReport(t, dir, planetID)
    testStateFile(t)
    requireNoError, requireError, requireEqual
    requireNotNil, requireTrue, requireFalse

MockTerraformProvider lives in terrain_test.go with configurable
error injection and call recording.

containsStr helper lives in lemmings_test.go.

testUnifiedReport helper lives in report_test.go, accepts nil *testing.T
for benchmark use.

testSovereignWithMockProvider lives in terrain_test.go.

testPricingEstimateForSovereign lives in cost_test.go.

---

## Fuzz Targets and Their Packages

    FuzzMergeConfigFile       -- . (main package)
    FuzzDomainFromURL         -- . (main package, report.go)
    FuzzFormatBytes           -- . (main package, report.go)
    FuzzDollarToMicrocents    -- ./pkg/pricing/...
    FuzzMicrocentsToDisplay   -- ./pkg/pricing/...

Run all fuzz targets:

    make test-fuzz

Run a specific package:

    go test -fuzz=FuzzDollarToMicrocents -fuzztime=30s ./pkg/pricing/...

---

## What Is Not Yet Implemented

Commander interface (MockCommander, RealCommander) defined in
main_test.go is not yet wired into production SSH/SCP calls in
lemmings.go. Those calls use exec.CommandContext directly. Wiring
Commander would make lemmings SSH/SCP paths unit-testable.

OCI and GCP providers are stubs that return "not yet implemented"
from every method. They are enterprise on-demand implementations.

summary.yml under runs/<runID>/ is referenced in the directory
layout but not yet written by any code.

---

## Common Mistakes to Avoid

Do not hardcode "ubuntu@" anywhere. Always use planet.RemoteUser.

Do not hardcode "/home/ubuntu/" anywhere. Use:
    fmt.Sprintf("/home/%s/", planet.RemoteUser)

Do not use float64 for currency. All monetary values are int64
microcents. Use pricing.MicrocentsToDisplay or
pricing.MicrocentsToDetailedDisplay for output.

Do not write AWS credentials into HCL files. Pass via the env()
method of the provider struct to the terraform subprocess only.

Do not add loadState or saveState to sovereign.go. state.go owns
these methods.

Do not add aggregateReport to lemmings.go. report.go owns it.

Do not add implementation logic to pkg/terraform_provider/provider.go.
It is a types and interface definition file only.

Do not call s.ledger.Append directly from terrain.go or lemmings.go.
Emit to s.bus instead. The ledger is a bus subscriber.

Do not import pkg/events or pkg/ledger from any provider package.
The provider packages know nothing about the event bus or the ledger.

Do not use GitLab environment variables. The GitLab Terraform backend
has been removed. Sovereign uses local terraform state only.

Do not import sovereign main from any pkg/* package. The boundary is
enforced by PlanetSpec and HCLSpec as the only handoff types.

Do not construct a Ledger in tests using the old main package NewLedger.
Use ledger.NewLedger from pkg/ledger. Do not construct an EventBus
in tests without calling t.Cleanup(bus.Close).

---

## Behavioral Guidelines

Think before coding. State assumptions explicitly before implementing.
If multiple interpretations exist present them rather than picking
silently. If something is unclear stop and ask.

Simplicity first. Minimum code that solves the problem. No features
beyond what was asked. No abstractions for single-use code. No
speculative flexibility.

Surgical changes are acceptable but the full file is preferred.
Git handles the diff. When asked to produce a complete file produce
the entire file with no abbreviation.

Goal-driven execution. For multi-step tasks state a brief plan with
verifiable steps before writing code.

When a package is added update CLAUDE.md, the Makefile, and the
relevant README files in the same session.

When a file is deleted note it explicitly. Do not leave ghost
references in other files.

Every new pkg/* package needs three things before it is considered
done: the implementation files, a test file with full coverage and
benchmarks, and a README.md in the same format as the others.​​​​​​​​​​​​​​​​
