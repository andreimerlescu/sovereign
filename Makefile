BINARY     := sovereign
MODULE     := github.com/andreimerlescu/sovereign
GO         := go
GOFLAGS    := -race
FUZZTIME   := 12s
COVEROUT   := coverage.out
INSTALLDIR := $(HOME)/bin

# Detect the current platform so `make build` builds the right target.
GOOS   := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

.DEFAULT_GOAL := all

# -- Build -------------------------------------------------------------------

.PHONY: all
all: lint test build

# build dispatches to the detected platform target.
.PHONY: build
build: build-$(GOOS)-$(GOARCH)

.PHONY: build-all
build-all: \
	build-darwin-arm64 \
	build-darwin-amd64 \
	build-linux-arm64 \
	build-linux-amd64 \
	build-windows-amd64

.PHONY: build-darwin-arm64
build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(GO) build -o bin/$(BINARY)-darwin-arm64 .
	@echo "built bin/$(BINARY)-darwin-arm64"

.PHONY: build-darwin-amd64
build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GO) build -o bin/$(BINARY)-darwin-amd64 .
	@echo "built bin/$(BINARY)-darwin-amd64"

.PHONY: build-linux-arm64
build-linux-arm64:
	GOOS=linux GOARCH=arm64 $(GO) build -o bin/$(BINARY)-linux-arm64 .
	@echo "built bin/$(BINARY)-linux-arm64"

.PHONY: build-linux-amd64
build-linux-amd64:
	GOOS=linux GOARCH=amd64 $(GO) build -o bin/$(BINARY)-linux-amd64 .
	@echo "built bin/$(BINARY)-linux-amd64"

.PHONY: build-windows-amd64
build-windows-amd64:
	GOOS=windows GOARCH=amd64 $(GO) build -o bin/$(BINARY)-windows-amd64.exe .
	@echo "built bin/$(BINARY)-windows-amd64.exe"

# -- Install / Uninstall -----------------------------------------------------

# install builds the platform binary first, then copies it into ~/bin.
# ~/bin is created if it does not exist.
.PHONY: install
install: build
	@mkdir -p $(INSTALLDIR)
	cp bin/$(BINARY)-$(GOOS)-$(GOARCH) $(INSTALLDIR)/$(BINARY)
	chmod +x $(INSTALLDIR)/$(BINARY)
	@echo "installed $(INSTALLDIR)/$(BINARY)"

.PHONY: uninstall
uninstall:
	@if [ -f $(INSTALLDIR)/$(BINARY) ]; then \
		rm $(INSTALLDIR)/$(BINARY); \
		echo "removed $(INSTALLDIR)/$(BINARY)"; \
	else \
		echo "$(INSTALLDIR)/$(BINARY) not found -- nothing to remove"; \
	fi

# -- Test --------------------------------------------------------------------

.PHONY: test
test:
	$(GO) test $(GOFLAGS) ./...

.PHONY: test-fuzz
test-fuzz:
	$(GO) test $(GOFLAGS) -fuzz=FuzzMergeConfigFile    -fuzztime=$(FUZZTIME) .
	$(GO) test $(GOFLAGS) -fuzz=FuzzDomainFromURL       -fuzztime=$(FUZZTIME) .
	$(GO) test $(GOFLAGS) -fuzz=FuzzFormatBytes         -fuzztime=$(FUZZTIME) .
	$(GO) test $(GOFLAGS) -fuzz=FuzzDollarToMicrocents  -fuzztime=$(FUZZTIME) ./pkg/pricing/...
	$(GO) test $(GOFLAGS) -fuzz=FuzzMicrocentsToDisplay -fuzztime=$(FUZZTIME) ./pkg/pricing/...

.PHONY: test-bench
test-bench:
	$(GO) test -bench=. -benchmem -run='^$$' ./...

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
	rm -f $(COVEROUT)
	rm -f bin/$(BINARY)-darwin-arm64
	rm -f bin/$(BINARY)-darwin-amd64
	rm -f bin/$(BINARY)-linux-arm64
	rm -f bin/$(BINARY)-linux-amd64
	rm -f bin/$(BINARY)-windows-amd64.exe

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
		--instance t3.medium \
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
		--instance t3.medium \
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

# -- Ledger ------------------------------------------------------------------

.PHONY: show-ledger
show-ledger:
	@find ~/.sovereignd/runs -name "ledger.yml" | sort | tail -1 | xargs cat 2>/dev/null \
		|| echo "no ledger found"

# -- Packages ----------------------------------------------------------------

.PHONY: test-events
test-events:
	$(GO) test $(GOFLAGS) ./pkg/events/...

.PHONY: test-ledger
test-ledger:
	$(GO) test $(GOFLAGS) ./pkg/ledger/...

.PHONY: test-pricing
test-pricing:
	$(GO) test $(GOFLAGS) ./pkg/pricing/...

.PHONY: test-exeggutor
test-exeggutor:
	$(GO) test $(GOFLAGS) ./pkg/terraform/exeggutor/...

.PHONY: test-providers
test-providers:
	$(GO) test $(GOFLAGS) ./pkg/terraform/provider/...

.PHONY: test-commander
test-commander:
	$(GO) test $(GOFLAGS) ./pkg/commander/...

.PHONY: test-deliverer
test-deliverer:
	$(GO) test $(GOFLAGS) ./pkg/deliverer/...

.PHONY: test-packages
test-packages: test-events test-ledger test-pricing test-exeggutor test-providers test-commander test-deliverer

# -- Help --------------------------------------------------------------------

.PHONY: help
help:
	@echo ""
	@echo "  sovereign -- available targets"
	@echo ""
	@echo "  Build"
	@echo "    make all                   lint + test + build (current platform)"
	@echo "    make build                 compile for current platform ($(GOOS)/$(GOARCH))"
	@echo "    make build-all             compile for all supported platforms"
	@echo "    make build-darwin-arm64    compile for macOS Apple Silicon"
	@echo "    make build-darwin-amd64    compile for macOS Intel"
	@echo "    make build-linux-arm64     compile for Linux ARM64"
	@echo "    make build-linux-amd64     compile for Linux x86-64"
	@echo "    make build-windows-amd64   compile for Windows x86-64"
	@echo ""
	@echo "  Install"
	@echo "    make install               build current platform and copy to ~/bin/sovereign"
	@echo "    make uninstall             remove ~/bin/sovereign"
	@echo ""
	@echo "  Test"
	@echo "    make test                  unit tests with race detector (all packages)"
	@echo "    make test-fuzz             fuzz all fuzz targets for FUZZTIME each"
	@echo "    make test-bench            benchmark tests (1s per benchmark, Go default)"
	@echo "    make test-all              unit + fuzz + bench"
	@echo "    make test-cover            unit tests with HTML coverage report"
	@echo "    make test-packages         test all pkg/* packages individually"
	@echo "    make test-commander        test pkg/commander only"
	@echo "    make test-events           test pkg/events only"
	@echo "    make test-ledger           test pkg/ledger only"
	@echo "    make test-pricing          test pkg/pricing only"
	@echo "    make test-exeggutor        test pkg/terraform/exeggutor only"
	@echo "    make test-providers        test pkg/terraform/provider/... only"
	@echo "    make test-deliverer        test pkg/deliverer only"
	@echo ""
	@echo "  Lint"
	@echo "    make lint                  go vet + gofmt"
	@echo "    make vet                   go vet only"
	@echo "    make fmt                   gofmt -w (writes files)"
	@echo "    make fmt-check             gofmt check only (exits 1 if files need formatting)"
	@echo ""
	@echo "  Maintenance"
	@echo "    make clean                 remove built binaries and coverage output"
	@echo "    make tidy                  go mod tidy + go mod verify"
	@echo ""
	@echo "  State"
	@echo "    make clean-state           remove .sovereignd state directory"
	@echo "    make show-state            print current sovereign state file"
	@echo ""
	@echo "  Ledger"
	@echo "    make show-ledger           print most recent run ledger"
	@echo ""
	@echo "  Run"
	@echo "    make dry-run               estimate cost without provisioning"
	@echo "    make run-local             run against localhost with minimal config"
	@echo ""
	@echo "  Overrides"
	@echo "    FUZZTIME=30s make test-fuzz"
	@echo "    INSTALLDIR=/usr/local/bin make install"
	@echo ""