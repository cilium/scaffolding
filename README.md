# Scaffolding

scaffolding's aim is to provide a framework for writing simple scripts to execute performance benchmarks, with a focus on keeping the process quick, flexible and simple.

The project is organized as follows:

* `./toolkit`: go package which automates simple tasks that would be too tedious or repetitive to implement scripting with other CLI tools.
* `./scale-tests`: collection of scripts for visualizing scale and performance data.
* `./scripts`: collection of bash scripts which implement commonly used/required functionality.
* `./kustomize`: collection of [kustomize](https://kustomize.io/) templates for applying commonly used manifests.
* `./scenarios`: implementation scripts for running benchmarks within different scenarios for some purpose.
* `./cmapisrv-mock`: a component which mocks the behavior of the Cilium Cluster Mesh API Server for scalability testing purposes.

## toolkit

```
collection of tools to assist in running performance benchmarks

Usage:
  toolkit [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  lazyget     get a thing so you don't have to
  ron         Run On Node
  verify      verify the state of things

Flags:
  -h, --help                help for toolkit
  -k, --kubeconfig string   path to kubeconfig for k8s-related commands
                            if not given will try the following (in order):
                            KUBECONFIG, ./kubeconfig, ~/.kube/config
  -v, --verbose             show debug logs

Use "toolkit [command] --help" for more information about a command.
```

Currently have the following subcommands:

* `lazyget`, used for:
  * creating kind configurations (`kind-config`)
  * getting kind images based on kubernetes version (`kind-image`)
* `ron` used for:
  * running commands on nodes in a kubernetes cluster through the use of pods, with support for: mounting local files, creating PVC for storing artifacts, auto-copying data out of PVC, prefixing commands with nsenter, and automatic cleanup.
* `verify`, used for:
  * verifying all pods and nodes have no failing conditions (`k8s-ready`)

For adding new subcommands, be sure to check out `util.go`, which has some commonly used utility functions ready to go.

## scripts

Most, if not all, of these scripts support passing `-d` as the first parameter, which asks the script to run a `set -x` for verbose output:

```bash
if [ "${1}" == '-d' ]
then
    set -x
    shift 1
fi
```

As a convention, the filenames of these scripts should be in snake case.

* **`add_grafana_dashboard.sh`**: Download a Grafana dashboard from grafana.com based on an ID, and create a ConfigMap with its contents. This ConfigMap is then used as a patch to live-update the dashboards ConfigMap used by `kustomize/grafana/dashboards.yaml`, in order to add a dashboard into a live grafana instance. If `-p` is passed to the script, then then live-updating behavior is suppressed, allowing for just a ConfigMap containing the dashboard to be created. This is suitable for adding dashboards into `kustomize/grafana/dashboards`.
* **`exec_with_registry.sh`**: Find a service with the labels `app.kubernetes.io/part-of=scaffolding` and `app.kubernetes.io/name=registry`, port-forward it to localhost on port 5000, execute a given command, then kill the port-forward. Useful for a `(crane|docker|podman) push`.
* **`get_apiserver_url.sh`**: Look for a pod with a prefix of `kube-apiserver` in its name and return it's IP and port in the format of `ip:port`. Not very v6 friendly.
* **`get_ciliumcli.sh`**: Download cilium-cli to current directory using instructions from the documentation.
* **`get_cluster_cidr.sh`**: Find the cluster cidr as passed to kubelets through the `--cluster-cidr` arg.
* **`get_crane.sh`**: Download [crane](https://github.com/google/go-containerregistry/blob/main/cmd/crane/doc/crane.md) to the current directory using instructions from their documentation.
* **`get_node_internal_ip.sh`**: Return the address for a node with the type `InternalIP`.
* **`k8s_api_readyz.sh`**: Grab the current context's API server IP and CA data and make a curl request to `/readyz?verbose=true` to check if the API server is up. If the CA data cannot be determined, then use `--insecure` with curl to still allow for a request to go out.
* **`retry.sh`**: Retry a given command, using a given delay in-between attempts. For example, `retry.sh 5 echo hi` will attempt to run `echo hi` every `5` seconds until success.

## kustomize

This collection of kustomize templates is meant to be easy to reference in a `kustomization.yaml` for your needs. As an example, within a scenario's directory add:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../../kustomize/prometheus
- ./../kustomize/grafana
```

into a `kustomization.yaml` and execute `kustomize build . | kubectl apply -f` and *boom*, you have prometheus and grafana. If you want to modify the deployment, just add patches. For instance, to upload `node_cpu_seconds_total` metrics to grafana cloud:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../../kustomize/prometheus
- ./../kustomize/grafana
patchesStrategicMerge:
- -|
    apiVersion: monitoring.coreos.com/v1
    kind: Prometheus
    metadata:
      name: prometheus
      labels:
        app: prometheus
    spec:
      remoteWrite:
      - url: <MY_PROM_PUSH_URL/>
        basicAuth:
          username:
            name: <MY_PROM_SECRET/>
            key: username
          password:
            name: <MY_PROM_SECRET/>
            key: password
        writeRelabelConfigs:
          - source_labels: 
              - "__name__"
            regex: "node_cpu_seconds_total"
            action: "keep"
```

Or to add a dashboard stored in `./my-cool-dashboard.json` to grafana:

```yaml
# my-cool-dashboard-cm.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-dashboards
data:
  my-cool-dashboard.json: |-
    <paste dashboard contents here/>
```

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../../kustomize/prometheus
- ./../kustomize/grafana
patches:
- ./my-cool-dashboard-cm.yaml
```

It's convention that each resource can be pinned to a node using NodeSelectors and `role.scaffolding/<role/>=true` labels, which is useful when we want to dedicate a node for a certain resource, such as netperf server. See below for specifics.

### prometheus

Deploys prometheus into the `monitoring` namespace onto any node labeled `role.scaffolding/monitoring=true`, accessible using the service named `prometheus`.
This is done through the use of [prometheus-operator](https://github.com/prometheus-operator/prometheus-operator).

Has a 'select-all' configured for ServiceMonitors.

### grafana

Deploys grafana onto a node with the `role.scaffolding/monitoring=true` label into the `monitoring` namespace, accessible using the service named `grafana`.

By default, will use the prometheus deployment above as a datasource.

A ConfigMap named `grafana-dashboards` will have its keys mounted as files inside the grafana container at `/var/lib/grafana/dashboards`, therefore to add a new dashboard, add its json to said ConfigMap with its filename as the key.
The script at `scripts/add_grafana_dashboard.sh` can be used to help facilitate this process.

Three dashboards are provided out-of-the-box in `kustomize/grafana/dashboards`, applied as patches in `kustomize/grafana/kustomization.yaml`:

* node-exporter: [grafana.com](https://grafana.com/grafana/dashboards/1860)
* Cilium v1.12 Operator Metrics [grafana.com](https://grafana.com/grafana/dashboards/16612-cilium-operator/)
* Cilium v1.12 Agent Metrics [grafana.com](https://grafana.com/grafana/dashboards/16611-cilium-metrics/)

A dashboard provider is used to accomplish this. See the [grafana docs](https://grafana.com/docs/grafana/latest/administration/provisioning/#dashboards) for more information.

### node-exporter

Deploys [node-exporter](https://github.com/prometheus/node_exporter) in the `monitoring` namespace onto any node labeled with `role.scaffolding/monitored=true`.
A ServiceMonitor is created to allow for collection from Prometheus.
A dashboard for the exported metrics is included within the grafana kustomization (see above).

`HostNetwork` is enabled to allow for network metrics on the node to be collected.

### monitoring-ns

Creates a namespace named 'monitoring'.
Kustomize doesn't auto-create namespaces, so something needs to create the 'monitoring' ns for resources like prometheus and grafana (see above).
By including this manifest, you won't have to manually create the namespace before a `kustomize build | kubectl apply -f -`.

### registry

Deploys an in-cluster registry in the namespace `registry`, available through the service named `registry`.
This means the DNS name `registry.registry.svc` can be used as the URL for pushed images.

[Crane](https://github.com/google/go-containerregistry/blob/main/cmd/crane/doc/crane.md) is a great way to interact with this registry and can be downloaded using `scripts/get_crane.sh`.
If you need to build a custom image and don't want to mess with pushing and downloading from a remote registry just to get it into your cluster, then this is the manifest for you!

### topologies

Sets up pod topologies for performance testing.
Right now we just have the one, **pod2pod**, and the intention here is to overwrite details of the deployment as needed within a `kustomization.yaml`.
This is definitely subject to change, as there is probably a better way to do this which doesn't involve a lot of boilerplate.

#### topologies/pod2pod/base

Creates two pods for network performance testing by using a Deployment with one replica and a NodeSelector:

* **`pod2pod-client`**: Selects nodes with the label `role.scaffolding/pod2pod-client=true`.
* **`pod2pod-server`**: Selects nodes with the label `role.scaffolding/pod2pod-server=true`.

Each of these deployments has a pod with a single container named `main`, using `k8s.gcr.io/pause:3.1` as its image.
To override the image for both deployments, you can use kustomize's `images` transformer:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../../kustomize/topologies/pod2pod/base
images:
- name: k8s.gcr.io/pause:3.1
  newName: <mycoolimage/>
```

If you just want the server or the client, you can use the `patchesStrategicMerge` transformer as follows:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../../kustomize/topologies/pod2pod/base
patchesStrategicMerge:
- |-
  $patch: delete
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: pod2pod-client
```

### topologies/pod2pod/overlays/pod

Uses `pod2pod/base`, but has a patch to ensure that each of the Deployments has one replica. Basically just an alias at this point.

### topologies/pod2pod/overlays/service

Creates an incomplete Service that selects the `pod2pod-server`.
You still need to fill in the service's spec with details about how you want it to function.
For instance, if I want to:

* Have `pod2pod-server` run `httpd` on port 80,
* Expose it as a LoadBalancer service on port 80
* Have `pod2pod-client` run an `alpine` container forever, for `kubectl exec`

I would write the following:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../../kustomize/topologies/pod2pod/overlays/service
patchesStrategicMerge:
- |-
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: pod2pod-client
  spec:
    template:
      spec:
        containers:
          - name: main
            image: alpine
            command: ["sleep", "infinity"]
- |-
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: pod2pod-server
  spec:
    template:
      spec:
        containers:
          - name: main
            image: httpd
            ports:
            - containerPort: 80
              name: http
              protocol: TCP
- |-
  apiVersion: v1
  kind: Service
  metadata:
    name: pod2pod-server
  spec:
    type: LoadBalancer
    ports:
      - protocol: TCP
        port: 80
        targetPort: 80
        name: http
```

## scenarios

Each sub-directory within the `scenarios` directory is meant to house resources for running any kind of performance test, using the resources within scaffolding.
The idea here is that each directory has:

* A main script for running the test(s),
* A `kustomization.yaml` file for deploying intra needed for the test,
* An artifacts directory where items produced from the test are kept,
* A `README.md` describing what is going on,
* And any other resources required.

`scenarios/common.sh` can be sourced within as a helper, containing common environment variables and functions:

Environment variables:

* **`SCENARIO_DIR`:** Absolute path to the directory of the current scenario (ie cwd when `common.sh` is sourced)
* **`ARTIFACTS`:** Absolute path to the scenario's artifacts directory
* **`ROOT_DIR`:** Absolute path to the root of scaffolding
* **`TOOLKIT`:** ... toolkit sub-directory ...
* **`SCRIPT`:** ... script sub-directory ...
* **`KUSTOMIZE`:** ... kustomize sub-directory ...

These environment variables can be overridden if needed by setting them prior to calling the `init` function below.

Functions:

* **`init()`:** Set the above environment variables, create `ARTIFACTS` directory, build `toolkit` if `ARTIFACTS/toolkit` does not exist.
* **`init_print()`:** Print the above imported environment variables and functions.
* **`reset_vars()`:** Reset the above imported environment variables.
* **`build_toolkit()`:** Build a binary for toolkit and save it into the artifacts directory.
* **`wait_ready()`:** Use `scripts/retry.sh` along with `scripts/k8s_api_readyz.sh` and the toolkit's `verify k8s-ready` command to wait until the k8s cluster is ready to go before proceeding. This is great to use after applying a built kustomize file or after provisioning a cluster.
* **`wait_cilium_ready()`:** Call `wait_ready`,  wait one minute for Cilium to show ready through `cilium status`, and then run a connectivity test. The connectivity test can be skipped by setting `SKIP_CT` to `skip-ct`.
* **`breakpoint()`:** Wait to continue until some data comes in from STDIN (ie from a user).
* **`env_var_or_die()`:** Check if the given variable is set, and if it isn't, exit with rc 1.

### xdp

Demonstrate the positive CPU impact of XDP native acceleration and DSR on a load-balancer.
Requires three nodes, one for a load balancer, one for a netperf server, one for grafana and prometheus.

Implemented within `minikube` for local development, but can easily be modified for other environments as needed.

Run `kubectl port-forward -n monitoring svc/grafana 3000:3000` to view the `node-exporter` dashboard, which can be used to monitor the CPU usage of the load balancer node.
