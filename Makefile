# The functionality in this makefile is largely copied from lnd/Makefile, 
# credit to LL devs.

PKG := github.com/carlakc/boltnd
ESCPKG := github.com\/carlakc\/boltnd
LND_PKG := github.com/lightningnetwork/lnd
BTCD_PKG := github.com/btcsuite/btcd

XARGS := xargs -L 1

GOBUILD := GO111MODULE=on go build -v
GOTEST := go test
GOACC_PKG := github.com/ory/go-acc
LINT_PKG := github.com/golangci/golangci-lint/cmd/golangci-lint

GO_BIN := ${GOPATH}/bin
GOACC_BIN := $(GO_BIN)/go-acc
LINT_BIN := $(GO_BIN)/golangci-lint

LINT_VER = v1.46.2

# Linting uses a lot of memory, so keep it under control by limiting the number
# of workers if requested.
ifneq ($(workers),)
LINT_WORKERS = --concurrency=$(workers)
endif

LINT = $(LINT_BIN) run -v $(LINT_WORKERS)

GREEN := "\\033[0;32m"
NC := "\\033[0m"
define print
	echo $(GREEN)$1$(NC)
endef

include make/testing_flags.mk

# =========
# UTILITIES
# =========

rpc:
	@$(call print, "Compiling protos.")
	cd ./offersrpc; ./gen_protos_docker.sh

lint: $(LINT_BIN)
	@$(call print, "Linting source.")
	$(LINT)

# =====
# TESTS
# =====

unit: 
	@$(call print, "Running unit tests.")
	$(UNIT)

unit-cover: $(GOACC_BIN)
	@$(call print, "Running unit coverage tests.")
	$(GOACC_BIN) $(COVER_PKG) -- -tags="$(LOG_TAGS)"

itest: build-itest itest-only

itest-only:
	@$(call print, "Running integration tests.")
	rm -rf itest/logs; date
	$(GOTEST) ./itest -v -tags="itest" -btcdexec=./btcd-itest -logdir=logs

# ============
# DEPENDENCIES
# ============

$(GOACC_BIN):
	@$(call print, "Installing go-acc.")
	go install -trimpath -tags=tools $(GOACC_PKG)@latest

$(LINT_BIN):
	@$(call print, "Installing linter.")
	go install -trimpath -tags=tools $(LINT_PKG)@$(LINT_VER)

# ============
# INSTALLATION
# ============
build-itest:
	@$(call print, "Building itest btcd.")
	CGO_ENABLED=0 $(GOBUILD) -tags="rpctest" -o itest/btcd-itest $(BTCD_PKG)

	@$(call print, "Building itest lnd.")
	CGO_ENABLED=0 $(GOBUILD) -tags="rpctest" -o itest/lnd-itest $(LND_PKG)/cmd/lnd

