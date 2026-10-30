# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0/).

## [0.1.0](https://github.com/temporalio/cloud-operator/releases/tag/v0.1.0) - 2026-10-30

### Features

- **crd:** show ID, state and region in kubectl get namespaces ([`ccb261f`](https://github.com/temporalio/cloud-operator/commit/ccb261fdcdb53d829a9c1173cf2cdadcead3d8b7))
- add secret and setup default cloud client ([`6ae9cf9`](https://github.com/temporalio/cloud-operator/commit/6ae9cf9efbc316d6799b89c2697218530049067c))
- **crd:** add shared Cloud resource types ([`7ab78a1`](https://github.com/temporalio/cloud-operator/commit/7ab78a199caa7a94a8b8716be19d102f65b411c5))
- **crd:** define Namespace spec and validation ([`9ca6e1b`](https://github.com/temporalio/cloud-operator/commit/9ca6e1b0a866a2cd78d61d982a70d800e28d5c5b))

### Bug Fixes

- ignore empty cloud blocks for fields unset in the namespace CR ([`e10c6ab`](https://github.com/temporalio/cloud-operator/commit/e10c6ab36ed398f56cb34631f68bc788bbaf9834))

### Refactoring

- **crd:** rename Namespace ProjectID field to Project ([`624b767`](https://github.com/temporalio/cloud-operator/commit/624b7679b1152215f3c69bcbf08df44cd032ce12))

### Documentation

- use placeholder account ID in example output ([`5be9c8b`](https://github.com/temporalio/cloud-operator/commit/5be9c8ba5a7e293bbca7505708831b1ecfb20002))
- address doc review comments ([`52f662b`](https://github.com/temporalio/cloud-operator/commit/52f662b6b73d47a987fb3d8576432815b3999b88))
- refactor README and install guide ([`6fd6cf5`](https://github.com/temporalio/cloud-operator/commit/6fd6cf5afa81f44644ff48b41e8a5d745ec0b052))
- **crd:** add proposal for CRD conventions ([`6b98125`](https://github.com/temporalio/cloud-operator/commit/6b981253fd3ad01c6c4e20c2a866345b75dd9f4c))

### Tests

- **e2e:** add Temporal Cloud e2e harness ([`4e164c7`](https://github.com/temporalio/cloud-operator/commit/4e164c7a8ebe1f9d98aaaa3463598c2009067597))
- **e2e:** add Temporal Cloud pre-flight check for e2e tests ([`8bb29e7`](https://github.com/temporalio/cloud-operator/commit/8bb29e77c717c5554c27038cb21f08f3a7501253))
- **e2e:** replace patched manifests with static fixtures ([`01a7021`](https://github.com/temporalio/cloud-operator/commit/01a70218c03cfdbef028a1705980ec6a14707cfa))

### Other Changes

- Publish images to Docker Hub on release ([`e46947d`](https://github.com/temporalio/cloud-operator/commit/e46947de192491cf518ddd10f6cd43ca38823eb2))
- updated docs for namespace lifecycle protection ([`adcc594`](https://github.com/temporalio/cloud-operator/commit/adcc5949c427c1cab95df954554e4a06b1b52285))
- updated install guide and a minor fix in main ([`fface5d`](https://github.com/temporalio/cloud-operator/commit/fface5d538771c91cb6119ef9c2bd971d8e94170))
- Turn off deletion protection in Temporal Cloud before deleting ([`02c02d3`](https://github.com/temporalio/cloud-operator/commit/02c02d3078573b7b733cabba6fe45a25eb8c7afd))
- Block delete when deletion protection is enabled ([`48bf442`](https://github.com/temporalio/cloud-operator/commit/48bf442a52c674d401f26428648d56af9084de82))
- Surface invalid deletion policy as Stalled and requeue; accept any case ([`94d2b66`](https://github.com/temporalio/cloud-operator/commit/94d2b66dc7a23f4ecc4fb212083ca5c8e1e590b3))
- gofmt fake server namespace.go ([`d10f12f`](https://github.com/temporalio/cloud-operator/commit/d10f12f66c694992e829c43cc9aff6c9f350deb9))
- Address review: use kube-temporal v0.6.0, deletion policy type, delete tests ([`e1109dc`](https://github.com/temporalio/cloud-operator/commit/e1109dc04b35c914c4352be3bc4324f04fdb6fa3))
- Add deletion policy field and decision logic ([`b86360d`](https://github.com/temporalio/cloud-operator/commit/b86360d3fabfe2b1b5e1b436904aab7315ec3bc1))
- Add separate GetNamespace assertion helper ([`537121a`](https://github.com/temporalio/cloud-operator/commit/537121a8933133f174635701b5dcc397c965203b))
- introduce a fake Temporal Cloud server ([`aa26079`](https://github.com/temporalio/cloud-operator/commit/aa260794b655e83717397b3b24d2ac43383598c4))
- Add implementation and helpers for ResourceSetLatest and ResourceSync ([`6c40758`](https://github.com/temporalio/cloud-operator/commit/6c4075859d595bf344b1754102871e3a6f3adf57))
- rename resource-operator to cloud-operator ([`1af3dd7`](https://github.com/temporalio/cloud-operator/commit/1af3dd77204cf11bfc3f66232399b73580fbeaa0))
- integrate kube-temporal base reconciler library ([`e03d12a`](https://github.com/temporalio/cloud-operator/commit/e03d12a14ce6c8585277bb4af0dcd51b5c1f414e))
- Temporal Cloud client/connection handling ([`af0e3d2`](https://github.com/temporalio/cloud-operator/commit/af0e3d23dac914b5c6a8ec577956b7eab0e22c0b))
- use kube-temporal@v0.2.0 with base controller ([`9316351`](https://github.com/temporalio/cloud-operator/commit/93163517feccd5f45fc526419a81b016143f302d))
- add api/cloud and get all tests succeeding ([`d1a4692`](https://github.com/temporalio/cloud-operator/commit/d1a4692dc1652a5a7dd1ae95114c0b257b3d36e8))
- update gdt test cases and kustomization files ([`032bd0c`](https://github.com/temporalio/cloud-operator/commit/032bd0c729547c120ef78c6c22247e18c065dccc))
- update Dockerfile for required go builder image ([`b24492a`](https://github.com/temporalio/cloud-operator/commit/b24492a3d7673cc13abb3d475a28d49eeb91666e))
- Bump google.golang.org/grpc from 1.82.1 to 1.83.2 ([`21851da`](https://github.com/temporalio/cloud-operator/commit/21851da240ea42311dac9ef8ebc2be06fb040bb5))
- Bump go.opentelemetry.io/otel/sdk from 1.44.0 to 1.45.0 ([`3da038c`](https://github.com/temporalio/cloud-operator/commit/3da038c7f4df69371d991b16bce5408cab13f8ca))
- Uplift temporalio/kube-temporal to v0.1.0 ([`5c99376`](https://github.com/temporalio/cloud-operator/commit/5c99376993c4b994d7aff670432214c5887687e0))
- add kustomize artifacts and update Makefiles ([`50d3e32`](https://github.com/temporalio/cloud-operator/commit/50d3e32206a2d0bedd07278b1202bbebc98bb982))
