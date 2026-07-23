# Sovereign

> The command center for distributed lemmings load testing.
> Sovereign provisions terrain, deploys agents, collects results,
> and renders unified reports across geographic origins.

[![Go](https://github.com/andreimerlescu/sovereign/actions/workflows/sovereign.yml/badge.svg)](https://github.com/andreimerlescu/sovereign/actions/workflows/sovereign.yml)
[![GoDoc](https://pkg.go.dev/badge/github.com/andreimerlescu/sovereign.svg)](https://pkg.go.dev/github.com/andreimerlescu/sovereign)
[![Go Version](https://img.shields.io/badge/go-1.26.2-blue)](https://go.dev/doc/go1.26)
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

If provisioning fails after some planets are already running, sovereign
destroys the running planets before returning the error. No instances
are left orphaned by a partial provisioning failure.

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

SSH and SCP calls use StrictHostKeyChecking=no and ConnectTimeout=30
by default. Additional OpenSSH options can be supplied via --ssh-option
or the SSHOptions config file field and are appended to every SSH and
SCP invocation, allowing operators to override host key checking or
supply a known-hosts file without modifying sovereign's source.

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

## Installation

### From source

Clone the repository and use make to build and install:

    git clone https://github.com/andreimerlescu/sovereign.git
    cd sovereign
    make install

This compiles a binary for the current platform and copies it to
~/bin/sovereign. The ~/bin directory is created if it does not exist.
The binary is made executable automatically.

To install to a different directory:

    INSTALLDIR=/usr/local/bin make install

To remove the installed binary:

    make uninstall

### Via go install

    go install github.com/andreimerlescu/sovereign@latest

This places the binary in $GOPATH/bin or $HOME/go/bin depending on
your Go configuration. It does not use the Makefile.

### Ensure terraform is in PATH

    terraform -version

Terraform must be available for provisioning to work. The binary is
not bundled with sovereign.

---

## Quick Start

Set required environment variables for AWS:

    export AWS_ACCESS_KEY_ID=...
    export AWS_SECRET_ACCESS_KEY=...
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
    --save-to, -s          Report delivery destinations (repeatable)
    --save-to-key, -sk     SSH private key path for ssh:// save-to destinations

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

### SSH Options

    --ssh-option           Extra OpenSSH -o option appended to every SSH and
                           SCP call sovereign makes (repeatable). The built-in
                           defaults StrictHostKeyChecking=no and
                           ConnectTimeout=30 are always present before these.
                           Example:
                             --ssh-option UserKnownHostsFile=/path/to/known_hosts
                             --ssh-option StrictHostKeyChecking=yes

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

### FTP Credentials

    --ftp-username         Username for ftp:// save-to destinations
                           (default: anonymous)
    --ftp-password         Password for ftp:// save-to destinations

### Config File

    --config, -c           Path to a sovereign YAML config file

---

## Report Delivery

Sovereign delivers the unified report as both markdown and HTML to
every destination listed in --save-to. Multiple destinations are
supported simultaneously. Each destination is parsed by its URL
scheme. A delivery failure to one destination is logged as a warning
and does not block delivery to the others.

### Local filesystem

    --save-to .
    --save-to /tmp/reports
    --save-to file:///tmp/reports

Writes sovereign.<date>.<domain>.md and .html into the directory.
The directory is created if it does not exist.

### Amazon S3

    --save-to s3://my-bucket/reports/2026

Uploads via aws s3 cp with appropriate Content-Type headers.
Sovereign never manages AWS S3 credentials -- the aws CLI reads them
from the subprocess environment, ~/.aws/credentials, or instance
metadata as it normally would.

Uploads to:

    s3://my-bucket/reports/2026/sovereign.<date>.<domain>.md
    s3://my-bucket/reports/2026/sovereign.<date>.<domain>.html

### Remote host via SCP

    --save-to ssh://rocky@myserver.com:/reports
    --save-to-key /home/rocky/.ssh/my-key.pem

Uploads via scp. The --save-to-key flag provides the SSH private key.
If --save-to-key is omitted the SSH agent or default key discovery
is used. StrictHostKeyChecking is disabled.

Uploads to:

    rocky@myserver.com:/reports/sovereign.<date>.<domain>.md
    rocky@myserver.com:/reports/sovereign.<date>.<domain>.html

### FTP server

    --save-to ftp://ftpserver.example.com/reports
    --ftp-username myuser
    --ftp-password mypassword

Uploads via FTP using passive mode (RFC 959). No external dependency
is required -- sovereign implements the FTP transfer directly using
the standard library. If --ftp-username is omitted the login is
attempted as anonymous.

The port defaults to 21 when not specified. An explicit port can be
included in the URL:

    --save-to ftp://ftpserver.example.com:2121/reports

Uploads to:

    ftp://ftpserver.example.com/reports/sovereign.<date>.<domain>.md
    ftp://ftpserver.example.com/reports/sovereign.<date>.<domain>.html

Credentials are never logged. The FTP connection is opened per file
and closed immediately after the transfer completes.

### Multiple destinations

    sovereign \
        --save-to . \
        --save-to s3://my-bucket/reports \
        --save-to ssh://rocky@myserver.com:/reports \
        --save-to ftp://ftpserver.example.com/reports \
        --save-to-key ~/.ssh/my-key.pem \
        --ftp-username ftpuser \
        --ftp-password secret

All destinations receive the report. A delivery failure to one
destination is logged as a warning and does not block delivery
to the others.

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

    # SSHOptions are extra -o flags appended to every SSH and SCP call.
    # The built-in defaults (StrictHostKeyChecking=no, ConnectTimeout=30)
    # are always present before these options.
    SSHOptions:
      - "UserKnownHostsFile=/path/to/known_hosts"

    SaveTo:
      - .
      - s3://my-bucket/sovereign-reports
      - ssh://rocky@myserver.com:/reports
      - ftp://ftpserver.example.com/reports

    SaveToKey: /home/rocky/.ssh/my-key.pem

    FTPUsername: ftpuser
    FTPPassword: ""

---

## Environment Variables

Required for AWS:

    AWS_ACCESS_KEY_ID           AWS access key ID
    AWS_SECRET_ACCESS_KEY       AWS secret access key
    SOVEREIGN_SUBNET_ID         Subnet ID for planet instances
    SOVEREIGN_SECURITY_GROUP    Security group ID for planet instances

These variable names match the standard AWS CLI and SDK convention.
Any tooling that already has AWS credentials configured for the CLI
will work with sovereign without additional setup.

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
    aws          S3 report delivery (only required when using s3:// save-to)

FTP delivery has no external binary dependency. It is implemented
directly using the Go standard library net package.

Lemmings is installed on each planet at provisioning time via:

    go install github.com/andreimerlescu/lemmings@latest

Cost estimation uses the public instances.vantage.sh API. No API key
is required. A pricing failure for a planet is non-fatal.

---

## Makefile

Sovereign ships a Makefile that covers the full development and
release workflow. Run `make help` to see all targets at any time.

### Build targets

`make build` detects the current platform automatically using
`go env GOOS` and `go env GOARCH` and compiles the appropriate
binary into the bin/ directory. On an Apple Silicon Mac this produces
bin/sovereign-darwin-arm64. On a Linux x86-64 server it produces
bin/sovereign-linux-amd64.

`make build-all` compiles for all five supported targets in sequence.
All binaries land in bin/ with their platform suffix so they do not
overwrite each other.

    make build                   compile for current platform
    make build-all               compile for all platforms
    make build-darwin-arm64      macOS Apple Silicon
    make build-darwin-amd64      macOS Intel
    make build-linux-arm64       Linux ARM64
    make build-linux-amd64       Linux x86-64
    make build-windows-amd64     Windows x86-64 (.exe)

### Install and uninstall

`make install` builds the current platform binary first, then copies
it to ~/bin/sovereign and makes it executable. The ~/bin directory is
created if it does not exist.

    make install                 build and install to ~/bin/sovereign
    make uninstall               remove ~/bin/sovereign

To install to a system-wide location override INSTALLDIR:

    INSTALLDIR=/usr/local/bin make install

### Test targets

    make test                    unit tests with race detector (all packages)
    make test-fuzz               fuzz all five targets for FUZZTIME each
    make test-bench              benchmark tests across all packages
    make test-all                unit + fuzz + bench in sequence
    make test-cover              unit tests with HTML coverage report
    make test-packages           test all pkg/* packages individually
    make test-commander          test pkg/commander only
    make test-events             test pkg/events only
    make test-ledger             test pkg/ledger only
    make test-pricing            test pkg/pricing only
    make test-exeggutor          test pkg/terraform/exeggutor only
    make test-providers          test pkg/terraform/provider/... only
    make test-deliverer          test pkg/deliverer only

The default fuzz duration is 12 seconds per target. Override it:

    FUZZTIME=60s make test-fuzz

### Lint targets

    make lint                    go vet + gofmt (writes files)
    make vet                     go vet only
    make fmt                     gofmt -w (writes files in place)
    make fmt-check               gofmt check only, exits 1 if any file needs formatting

`make all` runs lint, test, and build for the current platform in
that order. A failing lint step does not proceed to test, and a
failing test does not proceed to build.

### Maintenance targets

    make clean                   remove all built binaries and coverage output
    make tidy                    go mod tidy + go mod verify
    make clean-state             remove the .sovereignd state directory
    make show-state              print the current sovereign state file
    make show-ledger             print the most recent run ledger

### Run targets

    make dry-run                 estimate cost against localhost without provisioning
    make run-local               run a minimal test against localhost

---

## Fuzz Testing

Sovereign ships five fuzz targets covering the most likely surfaces
for unexpected input. All targets are run for 12 seconds each as part
of `make test-fuzz`. The results below were recorded on an Apple M3
Ultra and represent a healthy baseline. No crashes or unexpected
panics were found across any target at any corpus size.

    FuzzMergeConfigFile

        Exercises duration string parsing -- Until, Ramp,
        ProvisionTimeout, TerminateTimeout -- with arbitrary input.
        After 12 seconds and roughly 39,000 executions the fuzzer
        found no crashes. The baseline corpus covers valid durations,
        empty strings, zero durations, and multi-day durations. The
        small execution count reflects that the duration parser has
        little branching beyond parse-or-error, so the fuzzer
        exhausts interesting paths quickly.

    FuzzDomainFromURL

        Exercises the URL-to-filename conversion used to name report
        files. After 12 seconds and roughly 20,000 executions the
        fuzzer found no crashes or empty-string returns for inputs
        that contained a parseable host. The baseline corpus covers
        http, https, localhost with port, and paths with slashes.

    FuzzFormatBytes

        Exercises the human-readable byte formatter across the full
        int64 range. After 12 seconds and roughly 120,000 executions
        the fuzzer found no crashes and confirmed that every non-
        negative input produces a non-empty string. The high
        throughput reflects the simplicity of the function -- a
        switch on magnitude with no allocation-heavy paths.

    FuzzDollarToMicrocents (pkg/pricing)

        Exercises float64-to-int64 currency conversion. After 12
        seconds and roughly 200,000 executions the fuzzer confirmed
        that no non-negative input produces a negative microcent
        value. Float64 arithmetic near zero and near MaxFloat64 were
        both covered without panic.

    FuzzMicrocentsToDisplay (pkg/pricing)

        Exercises the microcent display formatter. After 12 seconds
        and roughly 193,000 executions the fuzzer confirmed that every
        non-negative input produces a string beginning with $ and that
        no input panics. The execution rate of roughly 16,000 per
        second reflects the formatter's single fmt.Sprintf call.

---

## Benchmarks

The results below were recorded on an Apple M3 Ultra running macOS.
All benchmarks use -benchmem so allocation counts are included.
Numbers are representative of the M3 Ultra architecture and will
differ on other hardware, but the relative ratios between operations
are meaningful across machines.

### Core sovereign (github.com/andreimerlescu/sovereign)

    ValidateConfig           164 ns/op     144 B/op    1 alloc/op

        Config validation is dominated by a single url.Parse call and
        a handful of string comparisons. One allocation for the parsed
        URL. Fast enough to run on every startup without concern.

    MergeConfigFile           70 ns/op       0 B/op    0 allocs/op

        Zero allocations. The merge loop is a series of string and
        int zero-value comparisons with no heap activity unless a
        duration needs parsing, which is excluded from the baseline
        corpus here. This is as fast as it can be.

    EstimateCost           92,490 ns/op   9,651 B/op  114 allocs/op

        Dominated by the HTTP round trip to the mock pricing server in
        the test. In production this is a real network call and will be
        measured in tens of milliseconds, not microseconds. The
        allocation count of 114 is the HTTP client machinery and JSON
        decoder -- nothing sovereign controls directly.

    TotalLemmings              0.27 ns/op     0 B/op    0 allocs/op

        Three integer multiplications. Essentially free.

    BuildLemmingsCommand     266 ns/op     152 B/op    6 allocs/op

        Six allocations for the fmt.Sprintf calls that assemble the
        remote command string. This runs once per planet per run.
        The cost is negligible.

    RenderUnifiedMarkdown   12,140 ns/op   4,502 B/op   94 allocs/op

        text/template execution across the unified report structure.
        94 allocations reflect template reflection overhead on the
        data structs. Runs once per sovereign run, not on a hot path.

    RenderUnifiedHTML       24,249 ns/op  14,973 B/op  269 allocs/op

        html/template is roughly twice as expensive as text/template
        for the same data because it performs contextual escaping on
        every value. The extra cost is the correct trade -- user-
        controlled strings in the report (instance IDs, region names,
        target URLs) are autoescaped rather than written raw. Runs
        once per sovereign run.

    SaveState            4,750,144 ns/op  85,441 B/op  299 allocs/op

        Dominated by YAML marshal and a synchronous disk write with
        fsync. At 4.75 milliseconds this is intentionally slow --
        the state file is the crash recovery mechanism and every write
        must reach stable storage before sovereign continues. Runs
        after provisioning and not again unless sovereign is extended.

    LoadState               77,753 ns/op  32,134 B/op  509 allocs/op

        A file read plus YAML unmarshal. The 509 allocations are the
        YAML decoder building the planet slice. Runs once at startup
        on resume. Not on a hot path.

    ProvisionPlanet      11,644,677 ns/op  28 MB/op  72,348 allocs/op

        In tests this calls the mock provider which does ledger appends
        and bus dispatches for every lifecycle event. The 28 MB and
        72,000 allocations are dominated by the ledger flushing the
        entire YAML file to disk on each append -- there are roughly
        six ledger events per successful provision. In production the
        real cost is terraform init and apply, which take minutes and
        dwarf everything measured here.

    DestroyPlanet         9,499,963 ns/op  28 MB/op  62,771 allocs/op

        Same ledger flush pattern as ProvisionPlanet but with fewer
        lifecycle events (two instead of six). The allocation ratio
        is consistent with the event count.

    HCLSpec                  744 ns/op     320 B/op   12 allocs/op

        Assembles the HCL spec struct from planet and config fields.
        Twelve allocations for the string formatting in the struct
        fields. Runs once per planet per provision.

    PlanetSpec               168 ns/op      56 B/op    2 allocs/op

        Two allocations for the work directory path string. Called
        at every lifecycle transition. Negligible.

### Commander (pkg/commander)

    MockCommander.Run (unregistered)   153 ns/op   648 B/op   3 allocs/op
    MockCommander.Run (registered)     168 ns/op   598 B/op   4 allocs/op

        The mock commander is used in every test that exercises
        SSH or SCP calls. Three to four allocations cover the call
        record append and the empty reader construction. The
        registered case is marginally more expensive because it
        also allocates the response readers.

    RealCommander.Run (true)         3,228,458 ns/op  82 KB/op  115 allocs/op

        A real subprocess fork-exec. 3.2 milliseconds on an M3 Ultra
        for the simplest possible command (true). Every SSH and SCP
        call in a real sovereign run is at least this expensive before
        any network latency is considered.

### Deliverer (pkg/deliverer)

    LocalDeliverer.Deliver    205,620 ns/op   1,280 B/op   14 allocs/op

        Two os.WriteFile calls with MkdirAll. 200 microseconds is
        dominated by filesystem metadata operations. Fast enough to
        deliver dozens of destinations without measurable delay.

    S3Deliverer.Deliver       432,311 ns/op   4,411 B/op   46 allocs/op
    SSHDeliverer.Deliver      434,387 ns/op   4,591 B/op   56 allocs/op

        Both run against mock commanders in tests so the measured cost
        is temp file creation, commander dispatch, and cleanup. In
        production S3 uploads are network-bound and SCP transfers
        involve an SSH handshake -- both will be orders of magnitude
        slower than the benchmark suggests.

    New (local)    43 ns/op   48 B/op   2 allocs/op
    New (S3)       64 ns/op  112 B/op   3 allocs/op
    New (SSH)      55 ns/op  112 B/op   2 allocs/op

        Deliverer construction parses a URL string and allocates the
        struct. All three are fast enough that constructing a new
        deliverer per save-to destination per run costs nothing
        observable.

### Events (pkg/events)

    Emit (1 subscriber)       39 ns/op    0 B/op   0 allocs/op
    Emit (10 subscribers)     70 ns/op    0 B/op   0 allocs/op
    Emit (concurrent)        192 ns/op    0 B/op   0 allocs/op
    Filter                     6 ns/op    0 B/op   0 allocs/op

        Zero allocations across all event bus operations. The bus
        holds subscribers in a slice and iterates it under a read
        lock. Emit at 39 nanoseconds per dispatch with one subscriber
        scales to 70 nanoseconds with ten, showing that the per-
        subscriber cost is roughly 3 nanoseconds -- dominated by the
        nil check and the function call overhead, not by any
        allocation. The concurrent case at 192 nanoseconds reflects
        the RWMutex contention from 28 parallel goroutines on the
        benchmark machine. Filter at 6 nanoseconds is a map lookup
        with no allocation -- the allowed-kinds map is built once at
        registration time and read-only thereafter.

### Ledger (pkg/ledger)

    Append              9,169,334 ns/op  33 MB/op  67,783 allocs/op
    LastEntryForPlanet         40 ns/op     144 B/op      1 alloc/op

        Append takes 9.2 milliseconds because it marshals the entire
        entry slice to YAML and calls os.WriteFile with fsync on every
        call. This is intentional -- the ledger is the crash recovery
        mechanism and every event must reach stable storage before the
        lifecycle continues. The allocation count of 67,783 is the
        YAML encoder serializing the growing slice. In a typical five-
        planet run with fewer than one hundred total events the
        cumulative ledger cost is under one second.

        LastEntryForPlanet at 40 nanoseconds scans the entry slice in
        reverse and returns a pointer copy on the first match. One
        allocation for the returned Entry copy. Called at resume time,
        not on the hot path.

### Pricing (pkg/pricing)

    DollarToMicrocents      0.27 ns/op    0 B/op   0 allocs/op
    MicrocentsToDisplay       73 ns/op    5 B/op   1 alloc/op
    FetchHourlyRate       89,785 ns/op  9,477 B/op  109 allocs/op
    EstimateCost         451,228 ns/op  47,869 B/op  547 allocs/op

        DollarToMicrocents is a single float64 multiply and int64
        cast -- essentially free. MicrocentsToDisplay allocates one
        string via fmt.Sprintf. Both are called during report rendering
        and cost nothing observable at run scale.

        FetchHourlyRate at 90 microseconds is a mock HTTP round trip.
        In production this is a real HTTPS request to
        instances.vantage.sh and will be measured in tens of
        milliseconds. The 109 allocations are the HTTP client and JSON
        decoder. EstimateCost multiplies this by the planet count --
        a five-planet run makes five sequential pricing calls.

### Terraform provider (pkg/terraform/provider/aws)

    RenderAll       18,679 ns/op  29,297 B/op  294 allocs/op
    RenderHCL    1,370,250 ns/op  41,794 B/op  377 allocs/op
    SaveState       85,140 ns/op  18,575 B/op   90 allocs/op
    LoadState       83,492 ns/op  13,297 B/op  139 allocs/op

        RenderAll at 19 microseconds executes five Go templates to
        produce the HCL strings in memory. The 294 allocations are
        the template engine reflecting over the HCLSpec struct.

        RenderHCL at 1.4 milliseconds adds two os.MkdirAll calls,
        five os.WriteFile calls, and the backup copy loop on top of
        RenderAll. The difference between 19 microseconds and 1.4
        milliseconds is entirely filesystem metadata.

        SaveState and LoadState are symmetric at roughly 84
        microseconds each. Both are a YAML marshal or unmarshal plus
        a single file write or read. These run at every lifecycle
        transition -- uninitialized, initialized, applying, applied,
        destroying, destroyed -- so a complete planet lifecycle
        triggers six SaveState calls totaling roughly 500 microseconds
        of provider state I/O, well below any meaningful threshold.

---

## The Name

A lemming is a sovereign's agent. It is born with a predetermined
destiny. It follows its plan. It reports back. It has no free will
of its own. The sovereign is the engineer -- the one who decides
the plan, commands the terrain, and reads the report.

Sovereign is the application that makes the engineer's intent real
across the globe.
