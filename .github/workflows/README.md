# GitHub Actions Workflows

---

## go.yml

Runs lint, test, and build in sequence on every push and pull request
to main. Jobs are sequential -- lint must pass before test runs, test
must pass before build runs. A failing lint job costs the fewest
minutes.

---

## Private Module Authentication

Sovereign is a private module at github.com/andreimerlescu/sovereign.
The Go toolchain must be told not to verify private modules through
the public sum database and must be given credentials to fetch them.

Three environment variables are set at the workflow level:

    GOPRIVATE:    github.com/andreimerlescu/*
    GONOSUMDB:    github.com/andreimerlescu/*
    GONOSUMCHECK: github.com/andreimerlescu/*

The wildcard covers sovereign itself and any other private module
under the same org -- for example lemmings if it becomes a private
module dependency in go.mod.

Each job runs a Configure private module access step that does two
things:

    git config --global url."https://<token>@github.com/".insteadOf \
        "https://github.com/"

This rewrites every github.com HTTPS URL to include the token inline
so go mod tidy, go get, and go build all authenticate transparently
without any change to go.mod or import paths.

    echo "machine github.com login andreimerlescu password <token>" \
        >> ~/.netrc

This provides the same credentials to git's credential helper as a
fallback. The chmod 600 is mandatory -- git refuses to read .netrc
if it is group or world readable and silently falls back to
unauthenticated requests which fail on private repos with a
misleading "not found" error rather than an authentication error.

---

## The GITHUB_TOKEN Gotcha

GitHub Actions automatically provides a built-in secret called
GITHUB_TOKEN in every workflow run. It is tempting to use it here
instead of creating a separate secret because it requires no setup
and is always available.

Do not use GITHUB_TOKEN for private module access in this workflow.

GITHUB_TOKEN is scoped to the repository the workflow is running in.
It can read and write github.com/andreimerlescu/sovereign because
that is the repo the workflow lives in. It cannot read any other
private repository -- including other private modules under
github.com/andreimerlescu/* that sovereign may depend on in go.mod.

When the Go toolchain runs go mod tidy or go build and encounters
an import of a private module that is not the current repository,
it issues a git clone against that module's repository. GITHUB_TOKEN
has no access to that repository. The clone fails.

The failure message is not helpful. Go does not say authentication
failed. It says the module cannot be found, which looks identical to
a typo in an import path or a module that does not exist. This makes
the root cause non-obvious and wastes time.

    go: github.com/andreimerlescu/lemmings@v0.1.0: reading
    github.com/andreimerlescu/lemmings/go.mod at revision v0.1.0:
    unknown revision v0.1.0

That error means git could not authenticate, not that the version
does not exist.

GIT_TOKEN must be a GitHub Personal Access Token with repo scope.
A PAT with repo scope can read every private repository the token
owner has access to under any organization they belong to. This
means one token covers sovereign and every other private module
under github.com/andreimerlescu/* without any per-repository
configuration.

---

## Required Secret

Add this secret to the repository before the workflow will pass:

    Name:   GIT_TOKEN
    Value:  a GitHub Personal Access Token with repo scope

Location:

    github.com/andreimerlescu/sovereign
        -> Settings
        -> Secrets and variables
        -> Actions
        -> New repository secret

The token owner must have read access to every private module that
appears in go.mod or go.sum. If a new private dependency is added
to go.mod and the CI starts failing with a module not found error
on that dependency, the first thing to check is whether the token
owner has read access to that repository.

---

## Concurrency

The workflow cancels in-progress runs on the same branch when a new
push arrives. On a busy PR branch this prevents redundant runs from
stacking up and consuming minutes for work that will be superseded
immediately.

The concurrency group key is:

    ${{ github.workflow }}-${{ github.ref }}

This means concurrent runs are isolated per branch. Two different
PRs can run their workflows simultaneously without cancelling each
other.

---

## Fuzz and Bench

Fuzz testing and benchmarks do not run in CI. They run locally:

    make test-fuzz
    make test-bench

Fuzz testing requires significant CPU time with no deterministic
completion point and is not appropriate as a PR gate. Benchmark
results are meaningless without a stable baseline environment --
GitHub-hosted runners have variable performance that makes benchmark
comparisons unreliable.​​​​​​​​​​​​​​​​
