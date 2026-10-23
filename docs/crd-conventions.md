# Custom resource conventions

This document describes the proposed conventions for the `cloud.temporal.io` custom resources (CR) and how they map to the Temporal Cloud Ops API.

## Resource shape

### Spec structure

Put the fields of a Cloud create request's `spec` directly under the CR's `spec`. For example, `CreateNamespaceRequest.spec.apiKeyAuth` becomes `Namespace.spec.apiKeyAuth`, rather than `Namespace.spec.spec.apiKeyAuth`. This makes manifests easier to read and write. In the current Cloud create requests reviewed (as of October 2026), root request fields do not collide with fields inside the request's `spec`.

Place root request fields that describe desired state, such as the Cloud Namespace's `projectId` and `tags`, in the CR's `spec`.

Document upstream field mappings in Go field comments. These should appear in the generated CRD schema and `kubectl explain`.

### Name mapping

`metadata.name` identifies the Kubernetes CR and cannot be changed. It is used as a Cloud resource name only when the resource specific mapping below says so.

| Cloud naming field | CR mapping | Examples |
| --- | --- | --- |
| Immutable, user-chosen `spec.name` | Map CR `metadata.name` to Cloud `spec.name`; omit CR `spec.name`. | Namespace |
| Mutable `spec.name` | Map to CR `spec.name`. | Service Account, Nexus Endpoint, Custom Role. |
| Mutable `spec.displayName` | Map to CR `spec.name`. | Project, API Key, User Group. |
| No Cloud name field | CR `metadata.name` is a local name and is not sent as a Cloud name. | Connectivity Rule, if it gets a CRD. |

Changing CR `spec.name` updates the Cloud resource without renaming the CR. Both mutable Cloud naming fields map to CR `spec.name` for a consistent CR interface.

### Field naming

For naming consistency, drop an upstream `Spec` suffix when the field already sits under the CR's `spec` and the suffix adds no meaning. For example, Cloud `NamespaceSpec.capacitySpec` becomes CR `spec.capacity`. Write `ID` rather than `Id` in CR field names. e.g., use `projectID` instead of `projectId`, and `connectivityRuleIDs` instead of `connectivityRuleIds`.

### Deprecated fields

Omit deprecated Cloud fields from new CRDs when a supported replacement exists. For Namespace, use `replicas.region` instead of `regions`, and `searchAttributes` instead of `customSearchAttributes`. 

### Admission validation

Validate fields through the CRD schema so the API server rejects invalid manifests when they are applied. Use OpenAPI schema rules for types, required fields, ranges, patterns, and enum values. Use CEL rules for checks that span fields or compare against the previous value. For example, a CEL rule on `spec.capacity` can require exactly one of `onDemand` and `provisioned` when `capacity` is set, and a CEL rule on an API Key can prevent `spec.ownerID` from changing after creation.

### Status structure

A CR's `status` has three parts: Cloud reference fields that link the CR to its Cloud resource, progress fields that report reconciliation, and the observed state, which is the rest of the Cloud API's get response excluding `spec`.

### Reconciliation

For the fields a CR manages, its `spec` is the source of truth. When the controller finds that one of those fields was changed outside the operator, for example in the Cloud UI, it restores the CR's value and records the drift in `conditions`. Errors that only the Cloud API can detect, such as an unknown region, are also reported in `conditions`.

## Example Namespace CR

This manifest illustrates the proposed CR shape. The `status` field names `cloudID` and `cloudResourceVersion` are placeholders for the Cloud reference fields, whose names are not yet decided. All values are illustrative.

```yaml
apiVersion: cloud.temporal.io/v1alpha1
kind: Namespace
metadata:
  name: payments-prod
  namespace: temporal-cloud
  labels:
    team: payments
spec:
  projectID: project-123
  tags:
    environment: production
  replicas:
    - region: aws-us-west-2
  retentionDays: 30
  apiKeyAuth:
    enabled: true
  capacity:
    onDemand: {}
status:
  # Cloud reference
  cloudID: payments-prod.a1b2c
  cloudResourceVersion: "4"
  # Progress
  observedGeneration: 1
  conditions:
    - type: Reconciling
      status: "False"
      reason: ReconcileSucceeded
      observedGeneration: 1
      lastTransitionTime: "2026-10-08T10:05:00Z"
  # Observed state
  state: Active
  activeRegion: aws-us-west-2
  endpoints:
    grpcAddress: payments-prod.a1b2c.tmprl.cloud:7233
  replicas:
    - id: replica-1
      region: aws-us-west-2
      isPrimary: true
      state: Active
  limits:
    actionsPerSecondLimit: 400
  createdTime: "2026-10-08T10:00:00Z"
  lastModifiedTime: "2026-10-08T10:05:00Z"
```

## References

- [Temporal Cloud Ops API reference](https://saas-api.tmprl.cloud/docs/httpapi.html)
- [Cloud API protobuf source](https://github.com/temporalio/cloud-api/tree/main/temporal/api/cloud)
