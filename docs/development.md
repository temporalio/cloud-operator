# Development and testing

This guide covers building, generating code and testing the Temporal Cloud
Operator.

* [Developing](#developing)
  * [Building](#building)
  * [Generating code](#generating-code)
* [Testing](#testing)
  * [Test environment](#test-environment)
  * [Temporal Cloud test account](#temporal-cloud-test-account)
  * [Unit testing](#unit-testing)
  * [Functional and end-to-end testing](#functional-and-end-to-end-testing)
    * [Tests against Temporal Cloud](#tests-against-temporal-cloud)

## Developing

To list every Makefile target, run `make help`.

### Building

`make build` builds the binary to `build/cloud-operator`, and `make build-image`
builds the container image.

### Generating code

After changing anything under `api/`, regenerate the code and commit the
generated files with your change:

```
make gen-all
```

`gen-all` runs three targets, which you can also run on their own:

| Target | Generates |
|---|---|
| `gen-kube-helpers` | Deepcopy and validation Go code for the API types |
| `gen-kube-register` | Scheme and group-version registration Go code |
| `gen-kube-manifests` | The CRDs in `deploy/kustomize/crd/bases` and the ClusterRole in `deploy/kustomize/rbac` |

## Testing

This repository does *not* use Ginkgo or the upstream Kubernetes
controller-runtime `envtest` package for testing the binaries and configuration
artifacts for Temporal Cloud Operator.

### Test environment

Several Makefile targets automate common tasks for setting up, tearing down, and
using a [KinD][kind] cluster for local testing of Temporal Cloud Operator.

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

Will build the Temporal Cloud Operator container image with your local changes
and load the container image into the KinD cluster's Docker registry.

```
make test-cluster-reset
```

Can be used to delete any existing test cluster environment, create a new one,
build the Temporal Cloud Operator container image and load it into the KinD
cluster's Docker registry.

```
make test-deploy
```

Will install the Temporal Cloud Operator running your local code along with all
of the CRDs, RBAC resources and related manifests in the `deploy/kustomize`
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
blast radius. Creating a service account needs the Owner or Admin role. With the
Developer role, your only option is a personal API key for local runs.

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

bin/temporal-cloud apikey create-for-service-account \
  --service-account-id "$TCO_SA_ID" \
  --display-name "$TCO_SA_NAME-$USER" --expiry-duration 30d \
  -o json | jq -er .token > ~/.config/temporalio/cloud-operator-e2e-api-key
```

#### 2b. Developer

```
bin/temporal-cloud apikey create-for-me \
  --display-name "$TCO_SA_NAME-$USER" --expiry-duration 30d \
  -o json | jq -er .token > ~/.config/temporalio/cloud-operator-e2e-api-key
```

#### 3. Check it works

The key has your own permissions, so pre-flight warns that it isn't a service
account key. If personal keys are turned off for the account, ask an Admin for a
service account key.

```
make test-cloud-preflight
```

Pre-flight checks the key and its access, and warns about extra rights or expiry
within 7 days. It never creates anything.

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

These need the [Temporal Cloud test account](#temporal-cloud-test-account). To
check that the harness can create, check and delete a namespace in Cloud,
without the operator:

```
make test-cloud-harness
```

To write a Cloud test, follow `test/e2e/harness/cloud-basic.yaml`.

An illustrative example of a test that goes through the operator looks like the
following. It applies a Namespace resource, checks it in Kubernetes, checks the
result in Temporal Cloud with `e2e-cloud`, and cleans up.

Note: this example can run once
[#8](https://github.com/temporalio/cloud-operator/issues/8) (create and update
namespaces) and [#9](https://github.com/temporalio/cloud-operator/issues/9)
(delete namespaces) land.

```yaml
name: namespace-lifecycle
description: Applying a Namespace creates it in Temporal Cloud and deleting it removes it.

depends:
  - name: e2e-cloud

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
   kube.get: namespaces.cloud.temporal.io/tco-e2e-${TEST_CLOUD_RUN_ID}-lifecycle
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

Cleanup only deletes your own run's namespaces, so several people can share the
test account.

If a run is interrupted, the next pre-flight lists what it left, with the run ID
and owner. Delete your own with:

```
TEMPORAL_CLOUD_CLI=bin/temporal-cloud scripts/e2e/e2e-cloud cleanup-run <run ID>
```

Each test namespace's description in Temporal Cloud records the manager, run ID
and owner, e.g. `manager=cloud-operator run-id=hs6or-a1b2 owner=tomba`. The
owner is `$USER`; set `TEST_CLOUD_OWNER` to override it, for example in CI.

[gdt]: https://github.com/gdt-dev/gdt
