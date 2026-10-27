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
export TCO_VERSION=v0.1.0
TCO_REPO=https://github.com/temporalio/cloud-operator

kubectl apply -k "$TCO_REPO//deploy/kustomize/default?ref=$TCO_VERSION"
kubectl wait --for=condition=Established --timeout=60s \
  crd/clusterconnections.cloud.temporal.io \
  crd/namespaces.cloud.temporal.io
```

This creates the `cloud.temporal.io` CustomResourceDefinitions and the
`temporal-cloud-operator` Kubernetes Namespace, with the operator's
ServiceAccount, RBAC rules and Deployment. At this point the operator is
running, but it isn't connected to Temporal Cloud yet. The next two steps store
the API key in a Secret and connect the operator to Temporal Cloud.

### 2. Store the API key

Create a Secret to store the API key, so that the connection resource in the
next step can read from it.

```bash
export TEMPORAL_CLOUD_API_KEY=<your API key>

kubectl create secret generic temporal-cloud-api-key \
  -n temporal-cloud-operator --from-literal=apikey="$TEMPORAL_CLOUD_API_KEY"
```

In production, it's recommended to create this Secret from a secrets manager
instead, for example with [External Secrets](https://external-secrets.io/), or to
generate it from your own kustomization (see [Manage the install with your own
kustomization](#manage-the-install-with-your-own-kustomization)).

### 3. Connect the operator to Temporal Cloud

A connection tells the operator how to connect to the Temporal Cloud Ops API
and where to find the API key. The following creates a default connection
the operator uses for every resource. Support for multiple connections is
planned, to enable one cluster manage more than one Temporal Cloud account,
such as production and staging, or give each team its own service account
and API key.

```bash
kubectl apply -k "$TCO_REPO//deploy/kustomize/overlays/default-connection?ref=$TCO_VERSION"
```

The `default-connection` overlay applies the same install again, plus a
ClusterConnection named `default`:

```yaml
apiVersion: cloud.temporal.io/v1alpha1
kind: ClusterConnection
metadata:
  name: default
spec:
  hostPort: saas-api.tmprl.cloud:443
  credentials:
    apiKey:
      secretRef:
        name: temporal-cloud-api-key
        key: apikey
```

### 4. Check the connection

```bash
$ kubectl -n temporal-cloud-operator rollout status deployment/temporal-cloud-operator
$ kubectl get clusterconnections
NAME      ENDPOINT                   READY   AGE
default   saas-api.tmprl.cloud:443   True    10s
```

`READY` is `True` once the operator has read the API key.

### Manage the install with your own kustomization

To add your own patches, write a kustomization that uses the `default-connection`
overlay. It can also generate the API key Secret with `secretGenerator`:

```yaml
# kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- https://github.com/temporalio/cloud-operator//deploy/kustomize/overlays/default-connection?ref=v0.1.0
secretGenerator:
- name: temporal-cloud-api-key
  namespace: temporal-cloud-operator
  files:
  - apikey=apikey.txt   # a local file holding the key and should be kept out of Git
  options:
    disableNameSuffixHash: true
```

Apply it with `kubectl apply -k .` once the CustomResourceDefinitions exist.

## Upgrade

Apply the new release the same way it was installed:

```bash
export TCO_VERSION=<new release>

kubectl apply -k "$TCO_REPO//deploy/kustomize/default?ref=$TCO_VERSION"
kubectl apply -k "$TCO_REPO//deploy/kustomize/overlays/default-connection?ref=$TCO_VERSION"
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

3. Remove the operator, the default connection, the CustomResourceDefinitions
   and the `temporal-cloud-operator` Kubernetes Namespace, including the API key
   Secret:

   ```bash
   kubectl delete -k "$TCO_REPO//deploy/kustomize/overlays/default-connection?ref=$TCO_VERSION"
   ```