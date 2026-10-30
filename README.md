# Temporal Cloud Operator

The Temporal Cloud Operator (TCO) is a Kubernetes [Operator][k8s-operator] for
the lifecycle management of Temporal Cloud resources. It lets you define
Temporal Cloud resources declaratively as Kubernetes [Custom Resources][cr] and
keep them in version control, alongside the applications that use them. This
makes creating, changing and retiring those resources automated, auditable and
easy to review.

[cr]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[k8s-operator]: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/

## How It Works

You describe each Temporal Cloud resource as a custom resource and apply it with
`kubectl`. The operator watches these custom resources and creates, updates or
deletes the matching resources in Temporal Cloud. It keeps reconciling them, so
changes made outside Kubernetes are reverted. For an example, see [Create your
first resource](#create-your-first-resource).

If you are familiar with [Crossplane][crossplane] or [AWS Controllers for
Kubernetes][ack] (ACK), this is the same concept applied to Temporal Cloud.

[crossplane]: https://www.crossplane.io/
[ack]: https://aws-controllers-k8s.github.io/docs/intro/

## Getting Started

### Prerequisites

- Kubernetes cluster
- Temporal Cloud API key

### Installation

```bash
export TCO_VERSION=v0.1.0
export TEMPORAL_CLOUD_API_KEY=<your API key>
export TCO_REPO=https://github.com/temporalio/cloud-operator

# 1. Install the operator
kubectl apply -k "$TCO_REPO//deploy/kustomize/default?ref=$TCO_VERSION"

# 2. Store the API key. The operator uses it to connect to Temporal Cloud.
kubectl create secret generic temporal-cloud-api-key \
  -n temporal-cloud-operator --from-literal=apikey="$TEMPORAL_CLOUD_API_KEY"

# 3. Wait for the operator (needs the `temporal-cloud-api-key` Secret to start)
kubectl -n temporal-cloud-operator rollout status deployment/temporal-cloud-operator --timeout=180s
```

### Create your first resource

```bash
# Create a Temporal Cloud Namespace and wait for it to become Active
kubectl apply -f - <<EOF
apiVersion: cloud.temporal.io/v1alpha1
kind: Namespace
metadata:
  name: my-first-namespace
spec:
  replicas:
    - region: aws-us-west-2
  retentionDays: 30
  apiKeyAuth:
    enabled: true
EOF

kubectl wait namespaces.cloud.temporal.io/my-first-namespace \
  --for=jsonpath='{.status.state}'=Active --timeout=10m
```

To see the Namespace's details, including its Temporal Cloud ID, run:

```console
$ kubectl describe namespaces.cloud.temporal.io my-first-namespace
Name:         my-first-namespace
API Version:  cloud.temporal.io/v1alpha1
Kind:         Namespace
...
Spec:
  API Key Auth:
    Enabled:  true
  Replicas:
    Region:        aws-us-west-2
  Retention Days:  30
Status:
  ...
  Id:     my-first-namespace.<account-id>
  State:  Active
```

### Next steps

* For more details on managing the lifecycle of Temporal Cloud resources,
  including deletion, see [Managing Temporal Cloud resources](docs/resources.md).
* For additional installation details, including upgrades and uninstalling,
  see [Installing the Temporal Cloud Operator](docs/install.md).

## Documentation

| Document | Description |
|---|---|
| [Installing the Temporal Cloud Operator](docs/install.md) | Installation details, upgrades and uninstalling. |
| [Managing Temporal Cloud resources](docs/resources.md) | Create, change and delete Temporal Cloud resources. |
| [Development and testing](docs/development.md) | How contributors build and run the operator locally, set up the kind test cluster and Temporal Cloud test account, and run unit and end-to-end tests. |

## Contributing

We welcome all contributions!

* For code contributions, please start by [opening an issue](https://github.com/temporalio/cloud-operator/issues/new) to discuss your idea.
* For bug reports and feature requests, [file an issue](https://github.com/temporalio/cloud-operator/issues).
* For feedback, join the #temporal-workers channel on [Temporal Slack](https://t.mp/slack).

## Development and testing

Want to try the operator locally? Check out [Development and testing](docs/development.md) for the development setup.

## License

This project is licensed under the [Apache License 2.0](LICENSE).
