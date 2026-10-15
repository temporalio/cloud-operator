# ==== Test targets ====

TEST_GDT_ARGS ?= 
TEST_CLUSTER_NAME ?= tro-test
TEST_KUBE_CONTEXT ?= kind-$(TEST_CLUSTER_NAME)
TEST_OPERATOR_NAMESPACE ?= temporal-resource-operator
TEST_CONTAINER_IMAGE ?= temporal-resource-operator
TEST_BUILD_DIR ?= $(BUILD_DIR)/$(TEST_CLUSTER_NAME)
$(TEST_BUILD_DIR):
	@mkdir -p "$(TEST_BUILD_DIR)"

KIND ?= $(BIN_DIR)/kind
KIND_VERSION ?= v0.33.0
$(KIND): | $(BIN_DIR)
	@echo -n "installing kind@$(KIND_VERSION) ... "
	@$(call go-install-tool,$(KIND),sigs.k8s.io/kind,$(KIND_VERSION))
	@echo "ok."

GDT ?= $(BIN_DIR)/gdt
GDT_VERSION ?= main
$(GDT): | $(BIN_DIR)
	@echo -n "installing gdt@$(GDT_VERSION) ... "
	@$(call go-install-tool,$(GDT),github.com/gdt-dev/gdt/cmd/gdt,$(GDT_VERSION))
	@echo "ok."

##@ Test

.PHONY: test
test: test-unit ## Run all tests.

.PHONY: test-unit
test-unit: ## Run all unit tests.
	@go test -v ./...

.PHONY: test-cluster-delete
test-cluster-delete: $(KIND) ## Delete the testing cluster.
	@echo -n "deleting '${TEST_CLUSTER_NAME}' kind cluster ... "
	@$(KIND) delete cluster -q -n "${TEST_CLUSTER_NAME}"
	@echo "ok."

.PHONY: test-cluster-create
test-cluster-create: $(KIND) ## Create the testing cluster.
	@echo -n "creating '${TEST_CLUSTER_NAME}' kind cluster ... "
	@$(KIND) create cluster -q -n "${TEST_CLUSTER_NAME}"
	@echo "ok."
	@sleep 5

.PHONY: test-cluster-load-image
test-cluster-load-image: $(KIND) test-build-resource-operator-image ## Load all images into the test cluster.
	@echo -n "loading image '$(TEST_CONTAINER_IMAGE):$(BUILD_GIT_VERSION)' into '$(TEST_CLUSTER_NAME)' kind cluster ... "
	@$(KIND) load docker-image -q -n "$(TEST_CLUSTER_NAME)" "$(TEST_CONTAINER_IMAGE):$(BUILD_GIT_VERSION)"
	@echo "ok."

.PHONY: test-cluster-reset
test-cluster-reset: test-cluster-delete test-cluster-create test-cluster-load-image ## Reset the test cluster entirely.

.PHONY: test-build-resource-operator-image
test-build-resource-operator-image: ## Build resource-operator container image for use in test cluster.
	@echo -n "building resource-operator container image for test cluster ($(BUILD_GIT_VERSION)) ... "
	@$(CONTAINER_TOOL) build --quiet \
	   -t "$(TEST_CONTAINER_IMAGE):$(BUILD_GIT_VERSION)" \
	   -f Dockerfile \
	   --build-arg VERSION_PKG="$(BUILD_VERSION_PKG)" \
	   --build-arg GIT_VERSION="$(BUILD_GIT_VERSION)" \
	   --build-arg GIT_COMMIT="$(BUILD_GIT_COMMIT)" \
	   --build-arg BUILD_DATE="$(BUILD_DATE)" \
	   . >/dev/null 2>&1
	@echo "ok."

.PHONY: test-deploy
test-deploy: $(TEST_BUILD_DIR) $(KUSTOMIZE) test-clean-build ## Install temporal-resource-operator artifacts to the test cluster.
	@cp -r deploy $(TEST_BUILD_DIR)/deploy
	@cd $(TEST_BUILD_DIR)/deploy/kustomize/resource-operator && \
		"$(KUSTOMIZE)" edit set image $(BUILD_CONTAINER_IMAGE)="$(TEST_CONTAINER_IMAGE):$(BUILD_GIT_VERSION)" && \
		"$(KUSTOMIZE)" build $(TEST_BUILD_DIR)/deploy/kustomize/overlays/develop | \
		"$(KUBECTL)" --context $(TEST_KUBE_CONTEXT) apply -f -

.PHONY: test-undeploy
test-undeploy: $(TEST_BUILD_DIR) $(KUSTOMIZE) ## Uninstall temporal-resource-operator artifacts from the test cluster.
	@"$(KUSTOMIZE)" build $(TEST_BUILD_DIR)/deploy/kustomize/overlays/develop | \
		"$(KUBECTL)" --context $(TEST_KUBE_CONTEXT) delete --ignore-not-found -f -

.PHONY: test-resource-operator-logs
test-resource-operator-logs: ## Show logs from temporal-resource-operator pod.
	@POD_NAME=$(shell "$(KUBECTL)" --context "$(TEST_KUBE_CONTEXT)" get pods -n $(TEST_OPERATOR_NAMESPACE) -o jsonpath='{.items[*].metadata.name}'); \
		"$(KUBECTL)" --context $(TEST_KUBE_CONTEXT) -n $(TEST_OPERATOR_NAMESPACE) logs $$POD_NAME

.PHONY: test-clean-build
test-clean-build:
	@rm -rf $(TEST_BUILD_DIR)/deploy

.PHONY: test-clean
test-clean: test-clean-build test-cluster-delete ## Clean up the test environment.

.PHONY: test-e2e
test-e2e: test-e2e-install test-e2e-namespace-validation test-e2e-namespace ## Run all end to end tests.

.PHONY: test-e2e-install
test-e2e-install: $(GDT) ## Test that expected artifacts are all present in test cluster.
	@$(GDT) $(TEST_GDT_ARGS) run test/install/check.yaml

.PHONY: test-e2e-namespace
test-e2e-namespace: $(GDT) ## Test lifecycle management of Temporal Cloud Namespace by the temporal-resource-operator.
	@TEST_USER_NAMESPACE=tro-e2e-cluster $(GDT) $(TEST_GDT_ARGS) run test/e2e/namespace/happy-default.yaml

.PHONY: test-e2e-namespace-validation
test-e2e-namespace-validation: $(GDT) ## Test that the Namespace CRD accepts valid and rejects invalid Namespace manifests.
	@TEST_USER_NAMESPACE=tro-e2e-validation $(GDT) $(TEST_GDT_ARGS) run test/e2e/namespace/validation.yaml
