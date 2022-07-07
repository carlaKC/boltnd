# The functionality in this makefile is largely copied from lnd/Makefile, 
# credit to LL devs.

PKG := github.com/carlakc/boltnd
ESCPKG := github.com\/carlakc\/boltnd

GOTEST := go test
GOACC_PKG := github.com/ory/go-acc

GO_BIN := ${GOPATH}/bin
GOACC_BIN := $(GO_BIN)/go-acc

DOCKER_TOOLS = docker run -v $$(pwd):/build carlakirkcohen/lnd-tools

# Linting uses a lot of memory, so keep it under control by limiting the number
# of workers if requested.
ifneq ($(workers),)
LINT_WORKERS = --concurrency=$(workers)
endif

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

lint: 
	@$(call print, "Linting source.")
	$(DOCKER_TOOLS) golangci-lint run -v $(LINT_WORKERS)

# =====
# TESTS
# =====

unit: 
	@$(call print, "Running unit tests.")
	$(UNIT)

unit-cover: $(GOACC_BIN)
	@$(call print, "Running unit coverage tests.")
	$(GOACC_BIN) $(COVER_PKG) -- -tags="$(LOG_TAGS)"

# ============
# DEPENDENCIES
# ============

$(GOACC_BIN):
	@$(call print, "Installing go-acc.")
	go install -trimpath -tags=tools $(GOACC_PKG)@latest

