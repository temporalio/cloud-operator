# Managing Temporal Cloud resources

This guide covers managing the lifecycle of Temporal Cloud resources. It assumes
a cluster admin has [installed the operator](install.md) and connected it to Temporal
Cloud.

* [How it works](#how-it-works)
* [Namespace](#namespace)
  * [Create a Namespace](#create-a-namespace)
  * [Modify a Namespace](#modify-a-namespace)
  * [Delete a Namespace](#delete-a-namespace)

## How it works

Each Temporal Cloud resource is described by a Kubernetes custom resource in the
`cloud.temporal.io` API group. When you apply one, the operator creates the
resource in Temporal Cloud. When you change it, the operator updates the
resource to match. The operator manages every resource through the connection
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
$ kubectl apply -n temporal-cloud -f examples/namespace.yaml
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
$ kubectl apply -n temporal-cloud -f examples/namespace.yaml  # with both onDemand and provisioned capacity
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
$ kubectl apply -n temporal-cloud -f examples/namespace.yaml
namespace.cloud.temporal.io/payments-prod configured
```

The operator updates the Temporal Cloud Namespace to match. You can't change a
Namespace's name or `project`, or remove a search attribute. If you change
`metadata.name`, Kubernetes treats it as a new resource. The operator creates a
new, empty Temporal Cloud Namespace, and the old one stays until you delete its
resource.

### Delete a Namespace

```bash
$ kubectl delete -n temporal-cloud namespaces.cloud.temporal.io payments-prod
```

When you delete the resource, the operator also deletes the Temporal Cloud
Namespace. The resource stays in Kubernetes until Temporal Cloud has finished
deleting it.

To keep the Temporal Cloud Namespace, set the `cloud.temporal.io/deletion-policy`
annotation to `retain` before you delete the resource:

```bash
$ kubectl annotate -n temporal-cloud namespaces.cloud.temporal.io payments-prod \
    cloud.temporal.io/deletion-policy=retain
```

Temporal Cloud won't delete a Namespace that has deletion protection turned on.
To delete it, set `spec.lifecycle.enableDeleteProtection` to `false` and apply
the manifest first.
