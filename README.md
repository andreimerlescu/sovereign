# Sovereign

> The command center for distributed lemmings load testing.
> Sovereign schedules terrain, deploys agents, collects results,
> and renders unified reports across geographic origins.

---

## What Sovereign Is

Sovereign is the engineer's command center. It sits above both
lemmings and topobuilder and coordinates them to execute
distributed load tests that no single machine can perform alone.

A single lemmings instance tells you how your application performs
under load from one origin. Sovereign tells you how your application
performs under load from every origin simultaneously -- multiple
regions, multiple cloud providers, hundreds of thousands of lemmings
running concurrently across the globe -- with a single command.

Sovereign does not run lemmings. It does not build infrastructure.
It commands the tools that do. It is the sovereign. lemmings are
its agents. topobuilder is its terrain builder. The engineer is
the one who decides when and where the lemmings run.

---

## The Three Applications

Understanding sovereign requires understanding where it sits in
relation to the other two applications.

lemmings is the agent. It runs on a single machine, follows a
plan, produces a LifeLog, and reports back. It knows nothing
about planets, infrastructure, or other lemmings instances running
elsewhere in the world.

topobuilder is the terrain builder. It spins up cloud instances
across geographic regions, installs software on them, and tears
them down when instructed. It knows nothing about lemmings
specifically until the lemmings fork adds ProvisionerLemmings.

sovereign is the command center. It knows about both. It is the
only application in the system that understands the full picture --
how many planets to create, what lemmings configuration each planet
receives, how to wait for all planets to finish, how to collect and
merge LifeLogs from distributed sources, when to trigger terrain
destruction, and what the unified final report looks like across
all planets.

---

## The Relationship

    sovereign
        |
        |-- topobuilder (creates terrain)
        |       |
        |       |-- planet 1 -- lemmings (us-east-1)
        |       |-- planet 2 -- lemmings (us-west-1)
        |       |-- planet 3 -- lemmings (eu-west-1)
        |       |-- planet 4 -- lemmings (ap-southeast-1)
        |       |-- planet 5 -- lemmings (sa-east-1)
        |
        |-- waits for all planets to report
        |
        |-- collects LifeLogs via SCP
        |
        |-- aggregates into unified report
        |
        |-- triggers topobuilder destroy

lemmings never knows sovereign exists.
topobuilder never knows lemmings exists until the fork.
sovereign knows everything.

---

## What Sovereign Does

### 1. Render the terrain configuration

Sovereign reads its own YAML configuration and renders a
topobuilder-compatible YAML file for the requested number of
planets. It knows which cloud providers to target, which regions
to use, what instance types to provision, and what SSH keys to use.

### 2. Schedule terrain creation

Sovereign calls topobuilder with the rendered configuration.
topobuilder provisions the instances, installs Go, installs
lemmings via ProvisionerLemmings, and copies the lemmings plan
file to each planet.

### 3. Distribute lemmings configurations

Each planet receives its own lemmings configuration tailored to
its terrain -- the target URL, the pack size, the terrain count,
the until duration, the ramp, and the plan file. Sovereign manages
the per-planet configuration generation so that the aggregate load
across all planets matches the engineer's intent.

### 4. Wait for completion

Sovereign monitors each planet. The default planet timeout is
five minutes to provision and five minutes to terminate. If a
planet fails to provision within the timeout, sovereign marks
it as lost, logs the failure, and continues with the remaining
planets. A lost planet is noted in the final report. Manual
intervention is flagged when termination cannot complete within
the timeout.

### 5. Collect LifeLogs

When each planet completes its lemmings run, sovereign collects
the LifeLog and report files via SCP. Collection happens per
planet as each one completes rather than waiting for all planets
to finish -- a slow planet does not delay collection from fast
ones.

### 6. Aggregate results

Sovereign merges the LifeLogs from all planets into a unified
ledger. The unified report shows:

    -- Total visits across all planets and all terrains
    -- Per-planet breakdown showing geographic performance differences
    -- Per-path breakdown aggregated globally
    -- Timing percentiles at the global, per-planet, and per-path level
    -- Waiting room metrics aggregated across all origins
    -- Lost planet summary with failure reasons
    -- Projected cost of the run based on instance hours consumed

### 7. Destroy terrain

Once all planets have reported and all LifeLogs have been collected,
sovereign triggers topobuilder destroy. Destroy runs per-planet
concurrently. Sovereign waits for confirmation that all instances
have been terminated before exiting. If any planet fails to
terminate within the timeout, sovereign logs the instance IDs
that require manual intervention and exits with a non-zero status.

---

## Remote Terrain Constraint

Sovereign enforces the remote terrain constraint before calling
topobuilder. Only the web lemmings engine is permitted on remote
planets. Database, NoSQL, and CDN lemmings require direct network
access to internal infrastructure that randomly provisioned cloud
instances will not have.

If a sovereign configuration specifies a non-web engine alongside
a planet count greater than zero, sovereign exits with a clear
error before any infrastructure is created:

    sovereign: engine "sql" does not support remote terrain
    sovereign: remove -planets or switch to the web engine

---

## Cost Estimation

Before provisioning any infrastructure, sovereign calculates the
projected cost of the run using the instances.vantage.sh API to
look up current spot and on-demand pricing for the requested
instance types in each region.

The estimate includes:

    -- Per-planet instance cost based on instance type and region
    -- Estimated run duration based on ramp + until + collection overhead
    -- Total projected cost across all planets
    -- Spot vs on-demand pricing comparison where available

Sovereign prints the cost estimate and waits for confirmation
before provisioning. The --yes flag skips confirmation for
automated pipelines.

---

## State Persistence

Sovereign writes state to a hidden directory to survive crashes
and restarts without duplicating cloud resources.

On startup, sovereign checks for an existing state file in this
order:

    ./.sovereignd/state.yml
    ~/.sovereignd/state.yml

If a state file exists and contains active planet information,
sovereign loads that state and resumes from where it left off
rather than provisioning new instances. This means a crash during
a long run does not result in orphaned infrastructure -- sovereign
reconnects to the existing planets and continues collection.

---

## Quick Start

    go install github.com/andreimerlescu/sovereign@latest

Run a distributed load test across five planets:

    sovereign \
        --hit https://myapp.com \
        --planets 5 \
        --terrain 50 \
        --pack 50 \
        --until 30s \
        --ramp 5m \
        --provider digitalocean \
        --save-to .

Run with a plan file across three planets:

    sovereign \
        --hit https://myapp.com \
        --planets 3 \
        --terrain 20 \
        --pack 20 \
        --until 30s \
        --plan lemmings.plan.yml \
        --provider aws \
        --save-to s3://my-bucket/sovereign-reports

Dry run -- estimate cost without provisioning:

    sovereign \
        --hit https://myapp.com \
        --planets 5 \
        --terrain 50 \
        --pack 50 \
        --until 30s \
        --dry-run

---

## All Flags

### Core

    --hit          Target URL for lemmings to load test
    --planets      Number of cloud instances to provision
    --terrain      Number of terrain groups per planet
    --pack         Number of lemmings per terrain per planet
    --until        How long each lemming lives
    --ramp         Duration to bring all terrains online per planet
    --plan         Path to a lemmings plan YAML file
    --max-plan-items  Maximum plan items (default 999)

### Infrastructure

    --provider     Cloud provider: digitalocean, aws, gcp, azure
    --region       Comma-separated list of regions to deploy into
    --instance     Instance type per planet (default: provider minimum)
    --yes          Skip cost confirmation prompt
    --dry-run      Estimate cost and exit without provisioning

### Timeouts

    --provision-timeout   Maximum time to provision one planet (default 5m)
    --terminate-timeout   Maximum time to terminate one planet (default 5m)

### Report Delivery

    --save-to      Comma-separated delivery destinations matching
                   lemmings save-to format -- local path, s3://, mailto:

---

## The Sovereign Configuration File

For repeatable runs, sovereign accepts a YAML configuration file
via --config. All flags can be expressed in the config file.
Flags override config file values when both are present.

    sovereign --config sovereign.yml

Example sovereign.yml:

    Hit: https://myapp.com
    Planets: 5
    Terrain: 50
    Pack: 50
    Until: 30s
    Ramp: 5m
    Plan: lemmings.plan.yml

    Provider: digitalocean
    Regions:
      - nyc3
      - sfo3
      - ams3
      - sgp1
      - lon1

    ProvisionTimeout: 5m
    TerminateTimeout: 5m

    SaveTo:
      - .
      - s3://my-bucket/sovereign-reports

---

## Dependencies

    lemmings      -- the agent that runs on each planet
    topobuilder   -- the terrain builder that provisions planets
    instances.vantage.sh -- cloud pricing API for cost estimation

---

## The Name

A lemming is a sovereign's agent. It is born with a predetermined
destiny. It follows its plan. It reports back. It has no free will
of its own. The sovereign is the engineer -- the one who decides
the plan, commands the terrain, and reads the report.

Sovereign is the application that makes the engineer's intent real
across the globe.
