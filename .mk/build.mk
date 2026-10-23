# ==== Build targets ====

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

## Location to put temporary build artifacts
BUILD_DIR ?= $(shell pwd)/build
$(BUILD_DIR):
	@mkdir -p "$(BUILD_DIR)"

BUILD_DATE ?= $(shell date +%Y-%m-%dT%H:%M)
BUILD_GIT_VERSION ?= $(GIT_VERSION)
BUILD_GIT_COMMIT ?= $(GIT_COMMIT)
BUILD_VERSION_PKG ?= $(VERSION_PKG)

BUILD_CONTAINER_IMAGE ?= temporalio/cloud-operator

##@ Build binaries and container images

.PHONY: build-all
build-all: build build-image ## Build all binaries and container images.

.PHONY: build
build: build-cloud-operator ## Build all binaries.

.PHONY: build-cloud-operator
build-cloud-operator: | $(BUILD_DIR) ## Build cloud-operator binary.
	@echo -n "building cloud-operator binary ($(BUILD_GIT_VERSION)) ... "
	@go build -ldflags="-X $(BUILD_VERSION_PKG).GitVersion=$(BUILD_GIT_VERSION) -X $(BUILD_VERSION_PKG).GitCommit=$(BUILD_GIT_COMMIT) -X $(BUILD_VERSION_PKG).BuildDate=$(BUILD_DATE)" \
		-a -o $(BUILD_DIR)/cloud-operator cmd/cloud-operator/main.go
	@echo "ok."

.PHONY: build-image
build-image: build-cloud-operator-image ## Build all container images.

.PHONY: build-cloud-operator-image
build-cloud-operator-image: ## Build cloud-operator container image.
	@echo -n "building cloud-operator container image ($(BUILD_GIT_VERSION)) ... "
	@$(CONTAINER_TOOL) build --quiet \
	   -t "$(BUILD_CONTAINER_IMAGE):$(BUILD_GIT_VERSION)" \
	   -f Dockerfile \
	   --build-arg VERSION_PKG="$(BUILD_VERSION_PKG)" \
	   --build-arg GIT_VERSION="$(BUILD_GIT_VERSION)" \
	   --build-arg GIT_COMMIT="$(BUILD_GIT_COMMIT)" \
	   --build-arg BUILD_DATE="$(BUILD_DATE)" \
	   .
	@echo "ok."

.PHONY: build-clean
build-clean: ## Removes all temporary build artifacts.
	@rm -rf build/
