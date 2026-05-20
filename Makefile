ROOT              := $(realpath $(dir $(realpath $(firstword $(MAKEFILE_LIST)))))

# some systems requires opt-in for buildx
DOCKER_BUILDKIT   := 1
export DOCKER_BUILDKIT

ifdef CI
  BOLD  :=
  CYAN  :=
  RESET :=
else
  BOLD  := \033[1m
  CYAN  := \033[36m
  RESET := \033[0m
endif

BANNER = @printf "$(BOLD)$(CYAN)[target: $@]$(RESET)\n"

# Allocate a TTY in dev (for ctrl+c) but not in CI
MK_DOCKER_RUN_OPTS_TTY := $(if $(CI),,-it)
export MK_DOCKER_RUN_OPTS_TTY

# Detect a portable SHA256 tool, fallback to an echo mechanism if missing
MK_SHA256 := $(shell \
    if command -v sha256sum >/dev/null 2>&1; then \
        echo "sha256sum"; \
    elif command -v shasum >/dev/null 2>&1; then \
        echo "shasum -a 256"; \
    else \
        echo "echo unknown"; \
    fi)

# Safely detect a unique system identifier into a variable
MK_SYSTEM_ID := $(strip $(shell \
    if [ -s /etc/machine-id ]; then \
        cat /etc/machine-id 2>/dev/null; \
    elif command -v hostname >/dev/null 2>&1; then \
        hostname 2>/dev/null; \
    else \
        echo -n "unknown"; \
    fi))

# User might have several repos in a host. Distinguish each by using the abs path of the repo
MK_REPO_ID               := $(shell printf '%s' "$(ROOT)$(MK_SYSTEM_ID)" | $(MK_SHA256) | cut -c1-8)
MK_DOCKER_PROGRESS       ?= plain

MK_VALIDATE_CACHE_IMAGE  := harvester-loadbalancer-image-builder-validate-cache:$(MK_REPO_ID)
MK_TEST_CACHE_IMAGE      := harvester-loadbalancer-image-builder-test-cache:$(MK_REPO_ID)

# Legacy dapper env variables
CODECOV_TOKEN             ?=
REPO                      ?=
PUSH                      ?=

export MK_DOCKER_PROGRESS MK_REPO_ID MK_ADDONS_IMAGE MK_ISO_BUILDER_IMAGE
export CODECOV_TOKEN

MK_HOST_ARCH := $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
export MK_HOST_ARCH

DOCKER_BUILD = docker build \
	--progress=$(MK_DOCKER_PROGRESS) \
	--build-arg MK_REPO_ID \
	--build-arg MK_HOST_ARCH \
	-f $(ROOT)/Dockerfile $(ROOT)

.PHONY: build ci default generate-manifest package release test validate arm gen-version-env gen-version-env-debug clean-all


# ---- Directories ----
$(ROOT)/bin:
	@mkdir -p $@


# ---- Pre-generate version env for container builds (no .git needed inside Docker) ----
# Also handles git worktree checkouts where .git is a pointer file to an external directory.
gen-version-env:
	$(BANNER)
	@bash $(ROOT)/scripts/version > /dev/null


# ---- Generate and show the version env for debugging ----
gen-version-env-debug:
	$(BANNER)
	@bash $(ROOT)/scripts/version debug


# ---- Compile harvester binaries ----
build: gen-version-env | $(ROOT)/bin
	$(BANNER)
	$(DOCKER_BUILD) --target build-output --output type=local,dest=.


# ---- Validate ----
validate: gen-version-env
	$(BANNER)
	$(DOCKER_BUILD) --target validate -t $(MK_VALIDATE_CACHE_IMAGE)


# ---- Test ----
test: gen-version-env
	$(BANNER)
	$(DOCKER_BUILD) --target test -t $(MK_TEST_CACHE_IMAGE)


# ---- Package harvester image ----
package: build
	$(BANNER)
	$(ROOT)/scripts/package


# ---- Generate CRD manifests ----
generate-manifest: gen-version-env
	$(BANNER)
	$(DOCKER_BUILD) --target generate-manifest-output --output type=local,dest=$(ROOT)/crds


clean-all:
	$(BANNER)
	@docker rmi -f $(MK_VALIDATE_CACHE_IMAGE) $(MK_TEST_CACHE_IMAGE) || true


.DEFAULT_GOAL := default

ci: build package validate test

default: build package

arm: ci

release: ci
