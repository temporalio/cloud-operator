This repository contains the Temporal Cloud Operator: a Kubernetes
[Operator][k8s-operator] that manages the lifecycle of one or more [Temporal
Cloud][tcloud] resources.

[k8s-operator]: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
[tcloud]: https://saas-api.tmprl.cloud/docs/httpapi.html#description/introduction

# How It Works

After installing the Temporal Cloud Operator (TCO), a user can create, modify
or delete Temporal Cloud resources using the `kubectl` CLI.

For example, to create a new Temporal Cloud Namespace called
`my-org-namespace`, you might do:

```bash
$ kubectl apply -f - <<EOF
apiVersion: cloud.temporal.io/v1alpha1
kind: Namespace
metadata:
  name: my-org-namespace
  namespace: temporal-cloud  # NOTE: this is the Kubernetes Namespace, not the Temporal Cloud Namespace.
spec:
  replicas:
    - region: aws-us-west-2
  retentionDays: 30
  apiKeyAuth:
    enabled: true
EOF
```

Temporal Cloud Operator watches the Kubernetes API for [Custom Resources][cr]
(CRs) that represent Temporal Cloud resources and performs all the necessary
actions to managed the lifecycle of those Temporal Cloud resources associated
with the Custom Resource.

If you are familiar with projects like [Crossplane][crossplane] or [AWS
Controllers for Kubernetes][ack] (ACK), this is the same concept applied to
Temporal Cloud resources.

[cr]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[crossplane]: https://www.crossplane.io/
[ack]: https://aws-controllers-k8s.github.io/docs/intro/

# Installing the Operator

## Kustomize

The recommended way to install Temporal Cloud Operator is to use Kustomize.

The following will install the latest Temporal Cloud Operator into your
Kubernetes Cluster in the `temporal-cloud-operator` Kubernetes Namespace:

```bash
$ export TCO_TAG=main
$ kubectl apply -n temporal-cloud-operator -k "https://github.com/temporalio/cloud-operator/deploy/kustomize?ref=$TCO_TAG"
```

## Helm

# Upgrading the Operator

# Using the Operator

## Create a new Temporal Cloud Namespace

Describe the Temporal Cloud Namespace in a `Namespace` manifest, such as
[`examples/namespace.yaml`](examples/namespace.yaml):

```yaml
apiVersion: cloud.temporal.io/v1alpha1
kind: Namespace
metadata:
  name: payments-prod
spec:
  tags:
    environment: production
    team: payments
  replicas:
    - region: aws-us-west-2
  retentionDays: 30
  apiKeyAuth:
    enabled: true
  capacity:
    onDemand: {}
  description: Payment workflows for production.
```

Apply it to the Kubernetes Namespace that holds your Temporal Cloud
resources:

```bash
$ kubectl apply -n temporal-cloud -f examples/namespace.yaml
namespace.cloud.temporal.io/payments-prod created
```

The Temporal Cloud Namespace takes its name from `metadata.name`, which must
be a valid Temporal Cloud Namespace name. To list every `spec` field with the
Temporal Cloud field it maps to, run:

```bash
$ kubectl explain namespaces.spec --api-version=cloud.temporal.io/v1alpha1
```

The Kubernetes API server checks the structure of the manifest when you apply
it:

```bash
$ kubectl apply -n temporal-cloud -f examples/namespace.yaml  # with both onDemand and provisioned capacity
The Namespace "payments-prod" is invalid: spec.capacity: Invalid value: exactly one of onDemand or provisioned must be set
```

The API server checks that:

* `replicas` and `retentionDays` are set, and `replicas` is not empty.
* Enum values are valid, replica regions are unique, and exactly one capacity
  mode is set.
* Updates do not change `project`, and do not remove search attributes or
  change their type.

Temporal Cloud reports all other errors, such as invalid tags or a retention
period out of range.

## Modify a Temporal Cloud Namespace

## Delete a Temporal Cloud Namespace

# Contributing

## Developing

### Generating code

There are several Makefile targets to assist you in generating code.

```
make gen-all
```

Will generate all Go code and Kubernetes deployment manifests.

## Testing

This repository does *not* use Ginkgo or the upstream Kubernetes
controller-runtime `envtest` package for testing the binaries and configuration
artifacts for Temporal Cloud Operator.

### Test environment

Several Makefile targets automate common tasks for setting up, tearing down,
and using a [KinD][kind] cluster for local testing of Temporal Resource
Operator.

```
make test-cluster-create
```

Will create the KinD cluster for local testing.

```
make test-cluster-delete
```

Will tear down the KinD cluster for local testing.

```
make test-cluster-load-image
```

Will build the Temporal Cloud Operator container image with your local
changes and load the container image into the KinD cluster's Docker registry.

```
make test-cluster-reset
```

Can be used to delete any existing test cluster environment, create a new one,
build the Temporal Cloud Operator container image and load it into the KinD
cluster's Docker registry.

```
make test-deploy
```

Will install the Temporal Cloud Operator running your local code along with
all of the CRDs, RBAC resources and related manifests in the `deploy/kustomize`
directory.

```
make test-undeploy
```

Will remove all CRDs, RBAC roles and Temporal Cloud Operator deployment
resources from the test cluster environment.

[kind]: https://kind.sigs.k8s.io/

### Temporal Cloud test account

Tests that manage Temporal Cloud resources run against a real Temporal Cloud
test account. They authenticate with an API key using a dedicated service
account.

As a best practice, use a service account key when possible, to contain the
blast radius. Creating a service account needs the Owner or Admin role. With
the Developer role, your only option is a personal API key for local runs.

#### 1. Check your role

```
make temporal-cloud-cli
bin/temporal-cloud login
bin/temporal-cloud whoami -o json | jq -r .user.spec.access.accountAccess.role
mkdir -p ~/.config/temporalio
export TCO_SA_NAME=cloud-operator-e2e
```

#### 2a. Owner or Admin

If the account doesn't have this service account yet, create it. The Developer
role lets it create namespaces and makes it Admin only on the namespaces it
creates:

```
bin/temporal-cloud service-account create --name "$TCO_SA_NAME" \
  --account-role developer \
  --description "e2e tests for github.com/temporalio/cloud-operator"
```

Then create your key and save it:

```
TCO_SA_ID=$(bin/temporal-cloud service-account list -o json |
  jq -er --arg name "$TCO_SA_NAME" '.ServiceAccounts[] | select(.spec.name == $name) | .id')

(umask 077; bin/temporal-cloud apikey create-for-service-account \
  --service-account-id "$TCO_SA_ID" \
  --display-name "$TCO_SA_NAME-$USER" --expiry-duration 30d \
  -o json | jq -er .token > ~/.config/temporalio/cloud-operator-e2e-api-key)
```

#### 2b. Developer

```
(umask 077; bin/temporal-cloud apikey create-for-me \
  --display-name "$TCO_SA_NAME-$USER" --expiry-duration 30d \
  -o json | jq -er .token > ~/.config/temporalio/cloud-operator-e2e-api-key)
```

The key has your own permissions, so pre-flight warns that it isn't a service
account key. If personal keys are turned off for the account, ask an Admin for
a service account key.

#### 3. Check it works

```
make test-cloud-preflight
```

Pre-flight checks the key and its access, and warns about extra rights or
expiry within 7 days. It never creates anything.

- **Projects:** if the account has Projects, give the service account access to
  one project and set `TEST_CLOUD_PROJECT_ID`. Otherwise leave it unset.
- **`TEMPORAL_CLOUD_API_KEY`:** set this to pass the key another way, such as
  from CI or a password manager. It overrides the file.

### Unit testing

For unit testing, we use simple, [table-driven][table-driven-testing] tests and
the following test libraries:

* `github.com/stretchr/testify/`
* `github.com/stretchr/testify/mock`

Unit tests should validate the smallest testable unit of work possible. If your
unit tests cross lots of function/method boundaries, you might not be writing a
unit test but instead building a larger functional or integration test.

To run unit tests:

```
make test-unit
```

[table-driven-testing]: https://dave.cheney.net/2019/05/07/prefer-table-driven-tests 

### Functional and end-to-end testing

For functional and integration testing, we use a declarative testing framework
called [`gdt`][gdt] which allows test authors to focus on clearly defining the
*assertions* that their tests should verify instead of cluttering tests with
lots of setup and implementation details.

`gdt` test cases are YAML files in the `test/` directory.

To run all functional end-to-end tests:

```
make test-e2e
```

To run just the functional tests that verify configuration artifacts are
installed properly:

```
make test-e2e-install
```

To run functional tests that verify that the Temporal Cloud Operator properly
manages the lifecycle of Temporal Cloud resources:

```
make test-e2e-resource
```

To run functional tests that verify that the Namespace CRD accepts valid
manifests, including `examples/namespace.yaml`, and rejects invalid ones at
`kubectl apply`:

```
make test-e2e-namespace-validation
```

#### Tests against Temporal Cloud

These need the [Temporal Cloud test account](#temporal-cloud-test-account). To check that the harness can
create, check and delete a namespace in Cloud, without the operator:

```
make test-cloud-harness
```

To write a Cloud test, follow `test/e2e/harness/cloud-basic.yaml`.

An illustrative example of a test that goes through the operator looks like
the following. It applies a Namespace resource, checks it in Kubernetes,
checks the result in Temporal Cloud with `e2e-cloud`, and cleans up.

Note: this example can run once [#8](https://github.com/temporalio/cloud-operator/issues/8) (create and update namespaces) and [#9](https://github.com/temporalio/cloud-operator/issues/9) (delete
namespaces) land.

```yaml
name: namespace-lifecycle
description: Applying a Namespace creates it in Temporal Cloud and deleting it removes it.

depends:
  - name: e2e-cloud
  - name: kubectl

defaults:
  kube:
    namespace: $TEST_USER_NAMESPACE

tests:
 - name: preflight
   exec: e2e-cloud preflight
   require:
     exit-code: 0

 # Apply the resource. The operator creates the Cloud namespace from its name.
 - name: create-namespace
   kube.create: manifests/lifecycle.yaml

 # The operator reports the resource as ready in K8s.
 - name: namespace-ready
   kube.get: namespaces.cloud.temporal.io/tro-e2e-${TEST_CLOUD_RUN_ID}-lifecycle
   timeout: 10m
   retry:
     interval: 15s
   assert:
     conditions:
       ready: true

 # The namespace really exist in Temporal Cloud and is active.
 - name: cloud-namespace-active
   exec: e2e-cloud namespace-exists --active tco-e2e-${TEST_CLOUD_RUN_ID}-lifecycle
   timeout: 2m
   retry:
     interval: 15s

 # The Temporal Cloud namespace has the settings the manifest asked for.
 # The expected file lists only the fields to compare, as Cloud names them.
 # (namespace-matches is planned, not yet in e2e-cloud.)
 - name: cloud-namespace-matches-manifest
   exec: e2e-cloud namespace-matches tco-e2e-${TEST_CLOUD_RUN_ID}-lifecycle expected/lifecycle-cloud.yaml

 - name: delete-namespace
   kube.delete: namespaces.cloud.temporal.io/tco-e2e-${TEST_CLOUD_RUN_ID}-lifecycle

 - name: cloud-namespace-absent-after-delete
   exec: e2e-cloud namespace-does-not-exist tco-e2e-${TEST_CLOUD_RUN_ID}-lifecycle
   timeout: 10m
   retry:
     interval: 15s

 # Cleanup everything from this run in Temporal Cloud.
 - name: cleanup
   exec: e2e-cloud cleanup-run ${TEST_CLOUD_RUN_ID}
   timeout: 10m
```

`manifests/lifecycle.yaml`, next to the test file. gdt substitutes environment
variables in manifest files too:

```yaml
apiVersion: cloud.temporal.io/v1alpha1
kind: Namespace
metadata:
  name: tco-e2e-${TEST_CLOUD_RUN_ID}-lifecycle
spec:
  description: manager=cloud-operator run-id=${TEST_CLOUD_RUN_ID} owner=${USER}
  replicas:
    - region: aws-us-west-2
  retentionDays: 1
  apiKeyAuth:
    enabled: true
```

Tests for other resources follow the same steps, with their own manifest and
`e2e-cloud` checks.

Cleanup only deletes your own run's namespaces, so several people can share
the test account.

If a run is interrupted, the next pre-flight lists what it left, with the run
ID and owner. Delete your own with:

```
TEMPORAL_CLOUD_CLI=bin/temporal-cloud scripts/e2e/e2e-cloud cleanup-run <run ID>
```

Each test namespace's description in Temporal Cloud records the manager, run
ID and owner, e.g. `manager=cloud-operator run-id=hs6or-a1b2 owner=tomba`.
The owner is `$USER`; set `TEST_CLOUD_OWNER` to override it, for example in CI.

[gdt]: https://github.com/gdt-dev/gdt
