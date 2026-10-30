# Installing the Temporal Cloud Operator

This guide is for cluster admins who install and manage the Temporal Cloud
Operator.

## Prerequisites

You need a Kubernetes cluster, `kubectl` with cluster admin access and a
Temporal Cloud [API key](https://docs.temporal.io/cloud/api-keys). As a 
best practice, use the API key of a [service account](https://docs.temporal.io/cloud/service-accounts)
with the Global Admin [role](https://docs.temporal.io/cloud/manage-access/roles-and-permissions)
(Account Owner also has the access the operator needs, but it's not
recommended).

## Install

The recommended way to install the Temporal Cloud Operator is with
[kustomize](https://kustomize.io/), and a Helm chart is planned for [Public
Preview](https://github.com/temporalio/cloud-operator/milestone/2).

### 1. Install the operator

Pick a release from the [releases
page](https://github.com/temporalio/cloud-operator/releases) and install it:

```bash
export TCO_VERSION=<release>
TCO_REPO=https://github.com/temporalio/cloud-operator

kubectl apply -k "$TCO_REPO//deploy/kustomize/default?ref=$TCO_VERSION"
```

This creates the `cloud.temporal.io` CustomResourceDefinitions and the
`temporal-cloud-operator` Kubernetes Namespace, with the operator's
ServiceAccount, RBAC rules and Deployment. The operator doesn't start until
the API key Secret from the next step exists.

### 2. Store the API key

The operator reads the Temporal Cloud API key from the `apikey` entry of the
`temporal-cloud-api-key` Secret when it starts:

```bash
export TEMPORAL_CLOUD_API_KEY=<your API key>

kubectl create secret generic temporal-cloud-api-key \
  -n temporal-cloud-operator --from-literal=apikey="$TEMPORAL_CLOUD_API_KEY"
```

In production, it's recommended to create this Secret from a secrets manager
instead, for example with [External Secrets](https://external-secrets.io/), or to
generate it from your own kustomization (see [Manage the install with your own
kustomization](#manage-the-install-with-your-own-kustomization)).

### 3. Wait for the operator

```bash
kubectl -n temporal-cloud-operator rollout status deployment/temporal-cloud-operator --timeout=180s
```

To check that the operator can reach Temporal Cloud, [create your first
resource](../README.md#create-your-first-resource).

### Manage the install with your own kustomization

To add your own patches, write a kustomization that uses the operator's
install. It can also generate the API key Secret with `secretGenerator`:

```yaml
# kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- https://github.com/temporalio/cloud-operator//deploy/kustomize/default?ref=<release>
secretGenerator:
- name: temporal-cloud-api-key
  namespace: temporal-cloud-operator
  files:
  - apikey=apikey.txt   # a local file holding the key and should be kept out of Git
  options:
    disableNameSuffixHash: true
```

Apply it with `kubectl apply -k .`.

## Upgrade

Apply the new release the same way it was installed:

```bash
export TCO_VERSION=<new release>

kubectl apply -k "$TCO_REPO//deploy/kustomize/default?ref=$TCO_VERSION"
```

Since the Secret isn't part of the install, the upgrade will leave it unchanged.

## Uninstall

1. Set every custom resource to retain its corresponding Temporal Cloud resource, so
   that the uninstall doesn't delete it:

   ```bash
   kubectl annotate namespaces.cloud.temporal.io --all --all-namespaces \
     cloud.temporal.io/deletion-policy=retain --overwrite
   ```

   Without this step, the uninstall will attempt to delete every Temporal Cloud
   resource the operator created.

2. Delete the resources from Kubernetes while the operator is still running, so
   it can remove their finalizers:

   ```bash
   kubectl delete namespaces.cloud.temporal.io --all --all-namespaces
   ```

3. Remove the operator, the CustomResourceDefinitions and the
   `temporal-cloud-operator` Kubernetes Namespace, including the API key Secret:

   ```bash
   kubectl delete -k "$TCO_REPO//deploy/kustomize/default?ref=$TCO_VERSION"
   ```