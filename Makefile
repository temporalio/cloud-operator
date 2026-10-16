# Top-level Makefile containing make targets that automate build, release,
# deployment and local testing tasks.

KUBECTL ?= kubectl
SCRIPTS_DIR ?= $(shell pwd)/scripts

BIN_DIR ?= $(shell pwd)/bin
$(BIN_DIR):
	@mkdir -p "$(BIN_DIR)"

GIT_VERSION ?= $(shell git describe --tags --always --dirty || echo "unknown")
GIT_COMMIT ?= $(shell git rev-parse HEAD)
VERSION_PKG ?= github.com/temporalio/resource-operator/pkg/version

include .mk/build.mk
include .mk/go.mk
include .mk/gen.mk
include .mk/test.mk
include .mk/e2e-cloud.mk
include .mk/install.mk
include .mk/help.mk
