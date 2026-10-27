# Temporal Cloud Operator

The Temporal Cloud Operator (TCO) is a Kubernetes [Operator][k8s-operator] for
the lifecycle management of Temporal Cloud resources. It lets you define
Temporal Cloud resources declaratively as Kubernetes [Custom Resources][cr] and
keep them in version control, alongside the applications that use them. This
makes creating, changing and retiring those resources automated, auditable and
easy to review. The operator continuously reconciles the state of Temporal Cloud
resources with the desired state specified in the custom resources, reverting
any out-of-band changes.

[cr]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[k8s-operator]: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/

## How It Works

After installing the Temporal Cloud Operator, a user can create, modify or
delete Temporal Cloud resources using the `kubectl` CLI.

For example, to create a new Temporal Cloud Namespace called `my-org-namespace`,
you might do:

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

Temporal Cloud Operator watches the Kubernetes API for CRs that represent
Temporal Cloud resources and performs all the necessary actions to manage the
lifecycle of those Temporal Cloud resources associated with the Custom Resource.

If you are familiar with projects like [Crossplane][crossplane] or [AWS
Controllers for Kubernetes][ack] (ACK), this is the same concept applied to
Temporal Cloud resources.

[crossplane]: https://www.crossplane.io/
[ack]: https://aws-controllers-k8s.github.io/docs/intro/

## Getting Started

### Prerequisites

- Kubernetes cluster
- Temporal Cloud API key

### Quickstart

```bash
export TCO_VERSION=v0.1.0
export TEMPORAL_CLOUD_API_KEY=<your API key>
TCO_REPO=https://github.com/temporalio/cloud-operator

# 1. Install the operator
kubectl apply -k "$TCO_REPO//deploy/kustomize/default?ref=$TCO_VERSION"
kubectl wait --for=condition=Established --timeout=60s crd/clusterconnections.cloud.temporal.io

# 2. Store the API key
kubectl create secret generic temporal-cloud-api-key \
  -n temporal-cloud-operator --from-literal=apikey="$TEMPORAL_CLOUD_API_KEY"

# 3. Connect the operator to Temporal Cloud
kubectl apply -k "$TCO_REPO//deploy/kustomize/overlays/default-connection?ref=$TCO_VERSION"

# 4. Check the connection
kubectl -n temporal-cloud-operator rollout status deployment/temporal-cloud-operator
kubectl get clusterconnections
```

`kubectl get clusterconnections` should show the `default` connection with
`READY` set to `True`.

### Next steps

* To create your first cloud resource, see [Managing Temporal Cloud
  resources](docs/resources.md).
* For additional installation details, including connections, upgrades and
  uninstalling, see [Installing the Temporal Cloud Operator](docs/install.md).

## Documentation

| Document | Description |
|---|---|
| [Installing the Temporal Cloud Operator](docs/install.md) | Installation details, connections, upgrades and uninstalling. |
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
