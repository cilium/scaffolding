# xdp

Demonstrate the positive CPU impact of XDP native acceleration and DSR on a load-balancer. Requires three nodes, one for a load balancer, one for a netperf server, one for grafana and prometheus.

Implemented within `minikube` for local development, but can easily be modified for other environments as needed.

`xdp.sh` will do the following:

* Provision a three node `minikube` cluster using the `kvm2` driver
  * Modifying the NIC definitions to support XDP.
  * Set the time of each node to match that of the host
  * Disable `rp_filter`
* Label nodes with `role.scaffolding` labels to pin each set of infrastructure onto specific nodes.
* Install cilium with certain features based on the given cli argument:
  * `nkpr`: no kube-proxy replacement
  * `kpr`: kube-proxy replacement
  * `xdp`: kube-proxy replacement with XDP native acceleration and DSR
* If cilium is installed with KPR, then kube-proxy will be purged from the cluster.
* Run a connectivity test after cilium shows ready status (can be disabled with `-no-ct`). The `cilium-test` namespace will be deleted after a successful test.
* Install [metallb](https://metallb.universe.tf/) for load balancing. This may not be needed depending on your environment (for instance, k3s has a built in load balancer that can be [pinned to a node using labels](https://rancher.com/docs/k3s/latest/en/networking/#excluding-the-service-lb-from-nodes))
* Configure metallb using `./metallb_l2.yaml`.
* Verify that the mac address of the ExternalIP for `svc/pod2pod-server` matches the IP of the load balancer node.
* Pause for user input.
* Use `./netperf.sh` to run a netperf client on the host machine, targeting the ExternalIP of `svc/pod2pod-server`. All output is saved into `./artifacts`.

To run all three tests, you can do something like the following:

```bash
# Run without kpr
./xdp.sh -no-ct nkpr
# Uninstall cilium
./artifacts/cilium uninstall
# Run with kpr
./xdp.sh -no-ct kpr
# Uninstall cilium
./artifacts/cilium uninstall
# Run with xdp and dsr
./xdp.sh  -no-ct xdp
# View results
kubectl port-forward -n monitoring svc/grafana 3000:3000
```