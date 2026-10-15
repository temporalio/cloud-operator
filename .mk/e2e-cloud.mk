# ==== Temporal Cloud e2e test targets ====

TEMPORAL_CLOUD_CLI ?= $(BIN_DIR)/temporal-cloud
TEMPORAL_CLOUD_CLI_VERSION ?= v0.2.0
$(TEMPORAL_CLOUD_CLI): | $(BIN_DIR)
	@echo -n "installing temporal-cloud@$(TEMPORAL_CLOUD_CLI_VERSION) ... "
	@$(call go-install-tool,$(TEMPORAL_CLOUD_CLI),github.com/temporalio/cloud-cli/cmd/temporal-cloud,$(TEMPORAL_CLOUD_CLI_VERSION))
	@echo "ok."

# Environment for the Cloud e2e helpers: the pinned CLI, and the helpers on
# PATH so gdt exec steps can call them by name.
E2E_CLOUD_ENV = TEMPORAL_CLOUD_CLI="$(TEMPORAL_CLOUD_CLI)" PATH="$(SCRIPTS_DIR)/e2e:$$PATH"

##@ Test (Temporal Cloud)

.PHONY: temporal-cloud-cli
temporal-cloud-cli: $(TEMPORAL_CLOUD_CLI) ## Install the pinned Temporal Cloud CLI into bin/.

.PHONY: test-cloud-preflight
test-cloud-preflight: $(TEMPORAL_CLOUD_CLI) ## Check the Temporal Cloud e2e API key and access.
	@$(E2E_CLOUD_ENV) tro-e2e-cloud preflight
