# CLAUDE.md

# CLAUDE.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

## Project: Sovereign

Sovereign is a distributed load testing coordinator. It provisions
cloud instances (planets) across multiple regions, deploys lemmings
on each, collects results, aggregates them into a unified report,
and destroys the infrastructure when done.

Sovereign calls terraform directly via exec. No Docker. No load balancer. No DNS.

## Key Design Decisions

### Config File Merge Strategy

mergeConfigFile uses zero-value detection, not default-equality
detection. A config file field is merged into cfg only when the
corresponding cfg field is its zero value (empty string, 0, zero
duration). This means any value explicitly set by a flag -- even one
that happens to equal a compile-time default -- is never overwritten
by the config file.

The previous approach compared against named defaults (cfg.RemoteUser
== defaultRemoteUser). That was incorrect because it made it
impossible to explicitly set a field to a value that matched a
default via a flag. Zero-value detection is the correct strategy.

Tests that exercise mergeConfigFile must use a zero-value SovereignConfig
(cfg := SovereignConfig{}) to simulate "no flags set". They must not
use a cfg pre-populated with defaults if the intent is to verify that
all fields are merged from the file.

### Event Bus as Nervous System

Sovereign has its own event bus in pkg/events. The ledger is wired
as the first bus subscriber in NewSovereign:

    s.bus = events.NewEventBus()
    s.bus.Subscribe(s.ledger.AsSubscriber())

terrain.go and lemmings.go emit to the bus. They never call the
ledger directly. The bus fans out to all subscribers.

### Provider Interface and Handoff Types

Providers never see *Planet. The handoff between sovereign and
providers uses two types defined in pkg/terraform_provider:

    PlanetSpec    -- what a provider needs to operate
    HCLSpec       -- what a provider needs to render HCL

### Stateless Provider Structs

Provider structs hold only credentials resolved at New() time.
Per-planet data flows through PlanetSpec and HCLSpec arguments.
Safe for concurrent planet provisioning without additional locking.

### MockTerraformProvider is Concurrent

MockTerraformProvider in terrain_test.go contains a sync.Mutex.
All methods that read or write the mock's mutable fields (initCalls,
applyCalls, destroyCalls, outputCalls, savedStates, renderedSpecs,
initErr, applyErr, etc.) must hold the mutex. Tests that read mock
counters after concurrent operations must also lock the mutex before
reading.

Forgetting to lock the mock is a data race. The race detector will
catch it. provisionTerrain and destroyTerrain both operate
concurrently and both call mock methods.

### Credentials via Environment Only

No provider writes credentials into HCL files. Credentials are
passed to terraform subprocess calls via the env() helper method.

AWS subprocess environment:

    AWS_ACCESS_KEY_ID        mapped from AWS_ACCESS_KEY
    AWS_SECRET_ACCESS_KEY    mapped from AWS_ACCESS_SECRET_KEY

Required sovereign environment variables for AWS:

    AWS_ACCESS_KEY
    AWS_ACCESS_SECRET_KEY
    SOVEREIGN_SUBNET_ID
    SOVEREIGN_SECURITY_GROUP

### Rocky Linux Defaults

Sovereign defaults to Rocky Linux, not Ubuntu. Every SSH and SCP
call uses planet.RemoteUser, never a hardcoded string.

    RemoteUser:  rocky
    AMIOS:       Rocky-9-EC2-Base-9.*-x86_64*
    AMIOwners:   792107900819

### Ledger as Crash Recovery

Two separate state mechanisms:

    SovereignState   -- ~/.sovereignd/state.yml
    ProviderState    -- ~/.sovereignd/planets/planet-N/provider_state.yml
    Ledger           -- ~/.sovereignd/runs/<timestamp>/ledger.yml

On restart sovereign reads the ledger first via PlanetCompleted,
PlanetProvisioned, and LastEntryForPlanet.

### Microcent Arithmetic

All monetary values are int64 microcents. Float64 never stored.

    MicroCentsPerDollar int64 = 100,000
    MicroCentsPerCent   int64 = 1,000

### First Planet Alone

provisionTerrain always provisions planet 0 alone before remaining
planets provision in concurrent batches of 3. destroyTerrain is
fully concurrent with no semaphore.

### Duration Fields in YAML Reports

lemmingsReportFile uses time.Duration fields. The YAML marshaling
for time.Duration requires string values like "30s" or "1ms" -- not
bare integer nanoseconds. writeMockLemmingsReport in main_test.go
must write duration values as Go duration strings.

Bare integers like 30000000000 will cause yaml.Unmarshal to fail
with "cannot unmarshal !!int into time.Duration".

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

## What Is Not Yet Implemented

Commander interface (MockCommander, RealCommander) defined in
main_test.go is not yet wired into production SSH/SCP calls in
lemmings.go. Those calls use exec.CommandContext directly.

summary.yml under runs/<runID>/ is referenced in the directory
layout but not yet written by any code.

---

## Common Mistakes to Avoid

Do not add gcp or oci provider packages. They have been permanently
removed. If tests for those providers fail, fix the tests to expect
an error from newProvider -- do not add code.

Do not hardcode "ubuntu@" anywhere. Always use planet.RemoteUser.

Do not hardcode "/home/ubuntu/" anywhere. Use:
fmt.Sprintf("/home/%s/", planet.RemoteUser)

Do not use float64 for currency. All monetary values are int64
microcents.

Do not write AWS credentials into HCL files.

Do not add loadState or saveState to sovereign.go. state.go owns
these methods.

Do not add aggregateReport to lemmings.go. report.go owns it.

Do not add implementation logic to pkg/terraform_provider/provider.go.
It is a types and interface definition file only.

Do not call s.ledger.Append directly from terrain.go or lemmings.go.
Emit to s.bus instead.

Do not import pkg/events or pkg/ledger from any provider package.

Do not use GitLab environment variables.

Do not import sovereign main from any pkg/* package.

Do not construct a Ledger in tests using anything other than
ledger.NewLedger from pkg/ledger.

Do not write duration values as bare integers in YAML test fixtures.
Use Go duration strings: "30s", "5m", "1ms", "400ms".

Do not compare cfg fields against named defaults in mergeConfigFile.
Use zero-value detection (empty string, 0, zero duration) only.

---

## Behavioral Guidelines

Think before coding. State assumptions explicitly before implementing.
If multiple interpretations exist present them rather than picking
silently. If something is unclear stop and ask.

Simplicity first. Minimum code that solves the problem. No features
beyond what was asked.

Surgical changes are acceptable but the full file is preferred.

Goal-driven execution. For multi-step tasks state a brief plan with
verifiable steps before writing code.

When a package is added update CLAUDE.md, the Makefile, and the
relevant README files in the same session.

When a file is deleted note it explicitly. Do not leave ghost
references in other files.

Every new pkg/* package needs three things before it is considered
done: the implementation files, a test file with full coverage and
benchmarks, and a README.md in the same format as the others.
