# grafana

## Included Dashboards

* node-exporter [1860](https://grafana.com/grafana/dashboards/1860)
* Cilium v1.12 Operator Metrics [16612](https://grafana.com/grafana/dashboards/16612-cilium-operator/)
* Cilium v1.12 Agent Metrics [16611](https://grafana.com/grafana/dashboards/16611-cilium-metrics/)
* kube-state-metrics-v2 [13332](https://grafana.com/grafana/dashboards/13332-kube-state-metrics-v2/)

In these dashboards, old style graph panels have been upgraded to the newer time series panel type. Additionally, kube-state-metrics-v2 had its `cluster` variable removed to simplify setup (this can be revisited in the future as needed).
