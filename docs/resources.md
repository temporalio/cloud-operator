# Managing Temporal Cloud resources

This guide covers managing the lifecycle of Temporal Cloud resources. It assumes
a cluster admin has [installed the operator](install.md) and connected it to Temporal
Cloud.

* [How it works](#how-it-works)
* [Namespace](#namespace)
  * [Create a Namespace](#create-a-namespace)
  * [Modify a Namespace](#modify-a-namespace)
  * [Delete a Namespace](#delete-a-namespace)
    * [Delete the Temporal Cloud Namespace](#delete-the-temporal-cloud-namespace)
    * [Keep the Temporal Cloud Namespace](#keep-the-temporal-cloud-namespace)
    * [Delete a Namespace with delete protection](#delete-a-namespace-with-delete-protection)

## How it works

Each Temporal Cloud resource is described by a Kubernetes custom resource in the
`cloud.temporal.io` API group. When you apply one, the operator creates the
resource in Temporal Cloud. When you change it, the operator updates the
resource to match. The operator manages every resource with the API key
your cluster admin set up.

| Kind | Temporal Cloud resource | Availability |
|---|---|---|
| `Namespace` | [Namespace](https://docs.temporal.io/cloud/namespaces) | [Private Preview](https://github.com/temporalio/cloud-operator/milestone/1) |
| `Project`, `ServiceAccount`, `ApiKey` | [Project](https://docs.temporal.io/cloud/projects), [Service Account](https://docs.temporal.io/cloud/service-accounts), [API key](https://docs.temporal.io/cloud/api-keys) | [Public Preview](https://github.com/temporalio/cloud-operator/milestone/2) |

The resource's `spec` is the source of truth. If a user changes a field directly
in Temporal Cloud, for example in the Temporal Cloud UI, the operator
changes it back on its next check, every 30 seconds by default.

The resource's `status` shows its state in Temporal Cloud and surfaces any
errors or differences from the spec. To see why something failed, run `kubectl
describe` on the resource and read its conditions.

## Namespace

### Create a Namespace

Describe the Temporal Cloud Namespace in a `Namespace` manifest, such as
[`examples/namespace.yaml`](../examples/namespace.yaml):

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

Apply it to the Kubernetes Namespace that holds your Temporal Cloud resources:

```bash
$ kubectl apply -n cloud-resources -f examples/namespace.yaml
namespace.cloud.temporal.io/payments-prod created
```

The Temporal Cloud Namespace takes its name from `metadata.name`, which must be
a valid Temporal Cloud Namespace name. To list every `spec` field with the
Temporal Cloud field it maps to, run:

```bash
$ kubectl explain namespaces.spec --api-version=cloud.temporal.io/v1alpha1
```

The Kubernetes API server checks the structure of the manifest when you apply
it:

```bash
$ kubectl apply -n cloud-resources -f examples/namespace.yaml  # with both onDemand and provisioned capacity
The Namespace "payments-prod" is invalid: spec.capacity: Invalid value: exactly one of onDemand or provisioned must be set
```

The API server checks that:

* `replicas` and `retentionDays` are set, and `replicas` is not empty.
* Enum values are valid, replica regions are unique, and exactly one capacity
  mode is set.
* Updates do not add, change or remove `project`, and do not remove search
  attributes or change their type.

Temporal Cloud reports all other errors, such as invalid tags or a retention
period out of range. They appear in the resource's conditions.

**Note**: For production Namespaces, turn on Temporal Cloud's [deletion
protection](https://docs.temporal.io/cloud/namespaces#delete-protection) so the Namespace can't be
deleted by accident:

```yaml
spec:
  lifecycle:
    enableDeleteProtection: true
```

### Modify a Namespace

Edit the manifest and apply it again:

```bash
$ kubectl apply -n cloud-resources -f examples/namespace.yaml
namespace.cloud.temporal.io/payments-prod configured
```

The operator updates the Temporal Cloud Namespace to match. You can't change a
Namespace's name or `project`, or remove a search attribute. If you change
`metadata.name`, Kubernetes treats it as a new resource. The operator creates a
new, empty Temporal Cloud Namespace, and the old one stays until you delete its
resource.

### Delete a Namespace

What happens to the Temporal Cloud Namespace when you delete the resource
depends on the resource's deletion policy and on Temporal Cloud's delete
protection:

| Resource | Result of `kubectl delete` |
|---|---|
| `cloud.temporal.io/deletion-policy: delete` (default, also used when the annotation is not set) | The operator deletes the Temporal Cloud Namespace, then Kubernetes removes the resource. |
| `cloud.temporal.io/deletion-policy: retain` | Kubernetes removes the resource. The Temporal Cloud Namespace is kept. |
| `spec.lifecycle.enableDeleteProtection: true` | The operator keeps the resource and the Temporal Cloud Namespace until you turn off delete protection. See [Delete a Namespace with delete protection](#delete-a-namespace-with-delete-protection). |

#### Delete the Temporal Cloud Namespace

With the default `delete` policy, deleting the resource also deletes the
Temporal Cloud Namespace. A resource without the
`cloud.temporal.io/deletion-policy` annotation uses this policy:

```bash
$ kubectl delete -n cloud-resources namespaces.cloud.temporal.io payments-prod
namespace.cloud.temporal.io "payments-prod" deleted
```

The resource stays in Kubernetes, with `status.state` set to `Deleting`, until
Temporal Cloud has finished deleting the Namespace. `kubectl delete` waits
until then.

#### Keep the Temporal Cloud Namespace

To remove the resource but keep the Temporal Cloud Namespace, set the
`cloud.temporal.io/deletion-policy` annotation to `retain`, as in
[`examples/namespace-retain.yaml`](../examples/namespace-retain.yaml):

```yaml
metadata:
  name: payments-retained
  annotations:
    cloud.temporal.io/deletion-policy: retain
```

When you delete the resource, Kubernetes removes it right away and the
Temporal Cloud Namespace is left as is.

#### Delete a Namespace with delete protection

[`examples/namespace-delete-protection.yaml`](../examples/namespace-delete-protection.yaml)
turns on Temporal Cloud's delete protection:

```yaml
spec:
  lifecycle:
    enableDeleteProtection: true
```

With delete protection on, deleting the Namespace is a two-step process:

1. Turn off delete protection. Set `spec.lifecycle.enableDeleteProtection` to
   `false` and apply the manifest:

   ```bash
   $ kubectl apply -n cloud-resources -f examples/namespace-delete-protection.yaml
   namespace.cloud.temporal.io/payments-protected configured
   ```

2. Delete the resource:

   ```bash
   $ kubectl delete -n cloud-resources namespaces.cloud.temporal.io payments-protected
   namespace.cloud.temporal.io "payments-protected" deleted
   ```

**Note**: If you delete the resource before turning off the delete protection, the
operator doesn't delete the Temporal Cloud Namespace. It sets a `Stalled`
condition (reason `ResourceDeleteFailed`) explaining that delete protection is
on, and checks again every 30 seconds, by default.


```bash
$ kubectl describe -n cloud-resources namespaces.cloud.temporal.io payments-protected
Name:         payments-protected
...
Metadata:
  Deletion Timestamp:  2026-10-30T01:07:42Z
  Finalizers:
    namespace.cloud.temporal.io/finalizer
...
Spec:
  Lifecycle:
    Enable Delete Protection:  true
...
Status:
  Conditions:
    Message:  tcloud namespace payments-protected.ewkya has deletion protection enabled; set spec.lifecycle.enableDeleteProtection to false to delete it, or set the cloud.temporal.io/deletion-policy annotation to "retain" to keep it
    Reason:   ResourceDeleteFailed
    Status:   True
    Type:     Stalled
  State:      Active
```

To let the delete go through, set `spec.lifecycle.enableDeleteProtection` to
`false`. The operator turns off delete protection in Temporal Cloud, then
deletes the Temporal Cloud Namespace and removes the resource:

```bash
$ kubectl patch -n cloud-resources namespaces.cloud.temporal.io payments-protected \
    --type=merge -p '{"spec":{"lifecycle":{"enableDeleteProtection":false}}}'
namespace.cloud.temporal.io/payments-protected patched
```


