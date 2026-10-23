# ==== Installation targets ====

##@ Installation

.PHONY: install
install: $(KUSTOMIZE) ## Install temporal-cloud-operator artifacts to the K8s cluster specified in ~/.kube/config.
	cd deploy/kustomize/cloud-operator && "$(KUSTOMIZE)" edit set image cloud-operator="$(BUILD_CONTAINER_IMAGE):$(BUILD_GIT_VERSION)"
	"$(KUSTOMIZE)" build deploy/kustomize/default | "$(KUBECTL)" apply -f -

.PHONY: uninstall
uninstall: $(KUSTOMIZE) ## Uninstall temporal-cloud-operator artifacts from the K8s cluster specified in ~/.kube/config.
	"$(KUSTOMIZE)" build deploy/kustomize/default | "$(KUBECTL)" delete --ignore-not-found -f -
