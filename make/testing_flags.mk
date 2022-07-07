# The functionality in this makefile is largely copied from lnd's 
# make/testing_flags.mk, full credit to LL devs.
COVER_PKG = $$(go list -deps -tags="$(DEV_TAGS)" ./... | grep '$(PKG)' | grep -v offersrpc)

GOLIST := go list -deps $(PKG)/... | grep '$(PKG)'
GOLISTCOVER := $(shell go list -deps -f '{{.ImportPath}}' ./... | grep '$(PKG)' | sed -e 's/^$(ESCPKG)/./')

# If specific package is being unit tested, construct the full name of the
# subpackage.
ifneq ($(pkg),)
UNITPKG := $(PKG)/$(pkg)
UNIT_TARGETED = yes
COVER_PKG = $(PKG)/$(pkg)
endif

# If a specific unit test case is being target, construct test.run filter.
ifneq ($(case),)
TEST_FLAGS += -test.run=$(case)
UNIT_TARGETED = yes
endif

# Define the log tags that will be applied only when running unit tests. If none
# are provided, we default to "nolog" which will be silent.
ifneq ($(log),)
LOG_TAGS := ${log}
else
LOG_TAGS := nolog
endif

# UNIT_TARGTED is undefined iff a specific package and/or unit test case is
# not being targeted.
UNIT_TARGETED ?= no

# If a specific package/test case was requested, run the unit test for the
# targeted case. Otherwise, default to running all tests.
ifeq ($(UNIT_TARGETED), yes)
UNIT := $(GOTEST) -tags="$(LOG_TAGS)" $(TEST_FLAGS) $(UNITPKG)
endif

ifeq ($(UNIT_TARGETED), no)
UNIT := $(GOLIST) | $(XARGS) env $(GOTEST) -tags="$(DEV_TAGS) $(LOG_TAGS)" $(TEST_FLAGS)
endif

