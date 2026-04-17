BINARY     := sovereign
MODULE     := github.com/andreimerlescu/sovereign
GO         := go
GOFLAGS    := -race
BENCHTIME  := 5s
FUZZTIME   := 10s
COVEROUT   := coverage.out

.DEFAULT_GOAL := all

# -- Build -------------------------------------------------------------------

.PHONY: all
all: lint test build

.PHONY: build
build:
	$(GO) build -o $(BINARY) .

.PHONY: install
install:
	$(GO) install .

# -- Test --------------------------------------------------------------------

.PHONY: test
test:
	$(GO) test $(GOFLAGS) ./...

.PHONY: test-fuzz
test-fuzz:
	$(GO) test $(GOFLAGS) -fuzz=FuzzRenderTopobuilderYAML -fuzztime=$(FUZZTIME) .
	$(GO) test $(GOFLAGS) -fuzz=FuzzMergeConfigFile       -fuzztime=$(FUZZTIME) .
	$(GO) test $(GOFLAGS) -fuzz=FuzzDomainFromURL         -fuzztime=$(FUZZTIME) .
	$(GO) test $(GOFLAGS) -fuzz=FuzzFormatBytes           -fuzztime=$(FUZZTIME) .

.PHONY: test-bench
test-bench:
	$(GO) test -bench=. -benchmem -benchtime=$(BENCHTIME) -run='^$$' ./...

.PHONY: test-all
test-all: test test-fuzz test-bench

.PHONY: test-cover
test-cover:
	$(GO) test $(GOFLAGS) -coverprofile=$(COVEROUT) ./...
	$(GO) tool cover -html=$(COVEROUT)

# -- Lint --------------------------------------------------------------------

.PHONY: lint
lint: vet fmt

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: fmt
fmt:
	gofmt -l -w .

.PHONY: fmt-check
fmt-check:
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "the following files need formatting:"; \
		gofmt -l .; \
		exit 1; \
	fi

# -- Clean -------------------------------------------------------------------

.PHONY: clean
clean:
	$(GO) clean ./...
	rm -f $(BINARY)
	rm -f $(COVEROUT)

# -- Tidy --------------------------------------------------------------------

.PHONY: tidy
tidy:
	$(GO) mod tidy
	$(GO) mod verify

# -- Run ---------------------------------------------------------------------

.PHONY: dry-run
dry-run:
	$(GO) run . \
		--hit http://localhost:8080/ \
		--planets 1 \
		--terrain 2 \
		--pack 2 \
		--until 10s \
		--ramp 5s \
		--dry-run

.PHONY: run-local
run-local:
	$(GO) run . \
		--hit http://localhost:8080/ \
		--planets 1 \
		--terrain 2 \
		--pack 2 \
		--until 10s \
		--ramp 5s \
		--yes \
		--save-to /tmp/sovereign-test

# -- State -------------------------------------------------------------------

.PHONY: clean-state
clean-state:
	rm -rf .sovereignd
	@echo "sovereign state cleared"

.PHONY: show-state
show-state:
	@if [ -f .sovereignd/state.yml ]; then \
		cat .sovereignd/state.yml; \
	elif [ -f ~/.sovereignd/state.yml ]; then \
		cat ~/.sovereignd/state.yml; \
	else \
		echo "no sovereign state file found"; \
	fi

# -- Help --------------------------------------------------------------------

.PHONY: help
help:
	@echo ""
	@echo "  sovereign -- available targets"
	@echo ""
	@echo "  Build"
	@echo "    make all           lint + test + build"
	@echo "    make build         compile the sovereign binary"
	@echo "    make install       go install to GOPATH/bin"
	@echo ""
	@echo "  Test"
	@echo "    make test          unit tests with race detector"
	@echo "    make test-fuzz     fuzz all fuzz targets for FUZZTIME each"
	@echo "    make test-bench    benchmark tests"
	@echo "    make test-all      unit + fuzz + bench"
	@echo "    make test-cover    unit tests with HTML coverage report"
	@echo ""
	@echo "  Lint"
	@echo "    make lint          go vet + gofmt"
	@echo "    make vet           go vet only"
	@echo "    make fmt           gofmt -w (writes files)"
	@echo "    make fmt-check     gofmt check only (exits 1 if files need formatting)"
	@echo ""
	@echo "  Maintenance"
	@echo "    make clean         remove binary and coverage output"
	@echo "    make tidy          go mod tidy + go mod verify"
	@echo ""
	@echo "  State"
	@echo "    make clean-state   remove .sovereignd state directory"
	@echo "    make show-state    print current sovereign state file"
	@echo ""
	@echo "  Run"
	@echo "    make dry-run       estimate cost without provisioning"
	@echo "    make run-local     run against localhost with minimal config"
	@echo ""
	@echo "  Overrides"
	@echo "    FUZZTIME=30s make test-fuzz"
	@echo "    BENCHTIME=10s make test-bench"
	@echo ""​​​​​​​​​​​​​​​​
