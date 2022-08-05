# prometheus

must use `create` over `apply`, otherwise you'll run into this issue:

```text
$ kustomize build . | kubectl apply -f -
...
unable to recognize "STDIN": no matches for kind "Prometheus" in version "monitoring.coreos.com/v1"
Error from server (Invalid): error when creating "STDIN": CustomResourceDefinition.apiextensions.k8s.io "prometheuses.monitoring.coreos.com" is invalid: metadata.annotations: Too long: must have at most 262144 bytes
```

See [prometheus-community/helm-charts#1500](https://github.com/prometheus-community/helm-charts/issues/1500)
