# Cilium Cluster Mesh API Server Mock (cmapisrv-mock)

A component which mocks the behavior of the Cilium Cluster Mesh API Server. It
supports mocking a user-configurable number of clusters, each composed of a given
number of nodes, endpoints, identities and services, initialized at start-up time.
Subsequently, at run-time, it additionally generates insertion, update (where
appropriate) and deletion churn, depending on the configured QPS values.

## How to deploy cmapisrv-mock

cmapisrv-mock can be deployed leveraging the provided helm chart, which exposes
the main configuration options through the values file:

```bash
helm upgrade --install cmapisrv-mock -n kube-system ./deploy/cmapisrv-mock \
    --set image.repository=...
```

The helm chart assumes that Cilium has already been deployed in the same cluster,
and that it exists a Secret named `cilium-ca` containing the Cilium's CA (
automatically created by Cilium when either the clustermesh-apiserver or
Hubble Relay are enabled).

## How to configure Cilium to connect to cmapisrv-mock

Cilium agents can be configured to connect to the clusters mocked by cmapisrv-mock
through a configuration resembling the following (unrelated settings are omitted
for the sake of brevity):

```yaml
cluster:
  name: real-cluster
  id: 255

clustermesh:
  # We enable the real clustermesh-apiserver to force the creation of the TLS certificates.
  useAPIServer: true

  config:
    enabled: true
    domain: mesh.cilium.io
    clusters:
    - name: cluster-001
      address: cmapisrv-mock.kube-system.svc
      port: 2379
    - name: cluster-002
      address: cmapisrv-mock.kube-system.svc
      port: 2379
    ... # Depending on the number of mocked clusters
```
