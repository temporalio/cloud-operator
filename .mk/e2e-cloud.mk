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

RUN_ID ?= $(USER)
CLOUD_GDT = PATH="$(BIN_DIR):$$PATH" TEMPORAL_API_KEY="$$TEMPORAL_CLOUD_API_KEY" \
	TEST_KUBE_CONTEXT="$(TEST_KUBE_CONTEXT)" RUN_ID="$(RUN_ID)" \
	$(GDT) $(or $(TEST_GDT_ARGS),-d) run

##@ Test (Temporal Cloud)

.PHONY: temporal-cloud-cli
temporal-cloud-cli: $(TEMPORAL_CLOUD_CLI) ## Install the pinned Temporal Cloud CLI into bin/.

.PHONY: test-cloud-preflight
test-cloud-preflight: $(TEMPORAL_CLOUD_CLI) ## Check the Temporal Cloud e2e API key and access.
	@$(E2E_CLOUD_ENV) e2e-cloud preflight

.PHONY: test-cloud
test-cloud: test-cloud-api-key $(KIND) $(GDT) $(TEMPORAL_CLOUD_CLI) test-cluster-reset test-deploy ## Run the operator tests in test/cloud against Temporal Cloud.
	@echo "run ID: $(RUN_ID)"
	@$(CLOUD_GDT) "$(abspath test/cloud)" || { \
		"$(KUBECTL)" --context "$(TEST_KUBE_CONTEXT)" -n "$(TEST_OPERATOR_NAMESPACE)" \
			logs deployment/temporal-cloud-operator; \
		exit 1; }

.PHONY: test-cloud-api-key
test-cloud-api-key:
	@[ -n "$${TEMPORAL_CLOUD_API_KEY:-}" ] || { echo "TEMPORAL_CLOUD_API_KEY is not set" >&2; exit 1; }
