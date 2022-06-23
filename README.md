# scaffolding
Ansible Automation to bring up a Performance SUT for Cilium Performance testing

# Quickstart

## GKE

Pull the container image with all the necessary tools

`$ podman pull quay.io/jtaleric/scaffolding`

One can also build the container image from the project root, however please note that any files
(ie including service account credentials) within the project directory will be included into the
image

`$ podman build . -t scaffolding`

Kick it

`$ podman run -ti --name scaffold --hostname scaffolding --network host quay.io/jtaleric/scaffolding:latest /bin/bash`

You will need to retrieve a `Service Account`. To get this:
- Login to the GCloud Console
- Click the Project which you want to build clusters in.
- In the Menu choose "IAM & Admin" -> Service Account
- Click Create Service Account
- Once created, click the newly created Service Account
- On the top bar, click Keys
- Click Add Key and download the JSON

Store this file within this directory. Whatever you name it, update `group_vars/all` with the value.

```yaml
gke:
  region: "us-west2-a"
  project: "cilium-perf"
  auth_kind: "serviceaccount"
  sa_file: "my_sa.json"
  machine_type: "n1-standard-4"
  image_type: ubuntu_containerd
```
You will also need to update the `project` key here too to match you specific project.

If you want to swap kernels, you need to use the `ubuntu_containerd` image type.

We will assume the user is storing results in Elasticsearch, so update `es_url` with your ES Server information.

Once all the necessary changes are made, and the Service Account JSON is dropped in, kick a full pipeline run

`$ ansible-playbook platform-install-kernel-benchmark.yml --tags gke,prometheus,benchmark-operator -e "create=true" -e "num_nodes=2" -e "kernel=v5.17"`

Breaking down the `ansible-playbook` command :\
`--tags` -- `gke` Will tell Ansible what the Platform we are creating and testing is.\
`--tags` -- `prometheus, benchmark-operator` This will install the specific tools we want to have for our SUT \
`-e` -- Extra-var, which we will override the `create` variable to `true` so the automation will prepare us a cluster.\
`-e` -- Extra-var, which will set our `num_nodes` param. *required*\
`-e` -- Extra-var, which will set our `kernel` version to `5.17` this is only available under GKE today.

### Custom install params to pass to Cilium CLI
To modify the cilium install params

`$ ansible-playbook pipeline.yml --tags gke,prometheus,benchmark-operator,datapath -e "create=true" -e "num_nodes=2" -e "kernel=v5.18-rc6" -e "cilium_install_params='--helm-set bandwidthManager.enabled=true --helm-set bandwidthManager.bbr=false --helm-set kubeProxyReplacement=strict'" -vvv`

### Cleanup
`ansible-playbook platform.yml --tags gke -e "destroy=true" -e "archive_dir=/location/where/cluster/artifacts/are"`

## Providers
- GKE
- OCP

## Tools
- Prometheus (via Helm)
- Benchmark Operator
- Performance Dashboards (Grafana w/ customized dashboards)

## Known Nuances
- Stopping OpenShift mid-install can result in the `metadata.json` to be missing for a cleanup. To circumvent this, we can build a net-new `metadata.json` to clean up objects in the specified platform.

## Artifacts after run
```
 cilium*               # Binary which we used to install Cilium
 starttime             # When the Automation started
 cluster_name          # Name of the cluster in the event we have to manually clean up
 region                # Region we deployed in
 platform              # What Platform, GKE, OpenShift
 project               # Project we built the cluster in
 kubeconfig
  bmo/                 # Benchmark-Operator
 kernel                # Kernel which was installed
 *-uperf.yml           # Workload(s)
 *-uperf.log           # Console from workload
 datapath-uuid         # Benchmark UUID (to retrieve from ES)
 uperf.json            # For result collection (touchstone)
 *-result.out          # Result from performance run
 destroyed             # If the cluster has been destroyed
```

# CI

The goal of scaffolding's CI is to automate the performance testing of K8s CNIs within a variety of different conditions.

Each set of conditions is referred to a 'scenario', with each scenario represented as a set of variables contained within a [vars file](https://docs.ansible.com/ansible/latest/user_guide/playbooks_variables.html#vars-from-a-json-or-yaml-file) that modifies the behavior of one of the scaffolding Ansible playbooks.

The file [ci/matrix.jsonnet](ci/matrix.jsonnet) contains the current testing matrix as a list of JSON objects, with each JSON object representing a scenario.

CI is implemented through CircleCI, with [dynamically generated configurations](https://circleci.com/docs/2.0/dynamic-config/) created by [ci/circleci.py](ci/circleci.py).

There are two different configurations which can be triggered:

1. `pipeline.yml`: CircleCI configuration that will test each scenario within [ci/matrix.jsonnet](ci/matrix.jsonnet) by rendering [ci/templates/pipeline.yml.j2](ci/templates/pipeline.yml/j2). Performs some setup tasks and then runs each scenario in parallel. This must be triggered manually.
2. `build.yml`: Builds and updates the scaffolding container image on Quay. This is triggered on a push to main or when a tag is created.

## Triggering

Triggering a run on the CI can be done through the web UI, however a utility script also exists for triggering a run from the command line.

The script, [ci/util/trigger.sh](ci/util/trigger.sh), requires a [personal API token](https://circleci.com/docs/2.0/managing-api-tokens/#creating-a-personal-api-token), as all the script does is make an API call to the CircleCI API server.

Once you have a token, view the head of the script for usage details.

## Testing

CircleCI has a CLI which allows us to run jobs locally. This may not work for everything, as the CLI cannot replicate the execution environment perfectly, but it's a great starting point to find bugs and test out changes.

The process for doing this is essentially:

1. Render configuration templates for local use:

`python3 ci/circleci.py --local <output_dir>`

2. Process configuration into something that the CircleCI CLI can use (and validate):

`circleci config process <output_dir>/<config>.yml > processed.yml`
`circleci config validate processed.yml`

3. Run a job within the processed config

`circleci local execute -c processed.yml --job <job_name>`

Be sure to view the processed configuration that you are passing into `circleci local execute`, as job names may change depending on how the configuration is parameterized.

To test running pipelines in their entirety, you need to use a fork to set up the project within CircleCI's web UI. (See [create-project](https://circleci.com/docs/2.0/create-project) for an overview.)

The following configuration is expected:

- [Environment variables](https://circleci.com/docs/2.0/env-vars#setting-an-environment-variable-in-a-project)
  - `ES_URL`: Fully qualified URL to ElasticSearch where results will be sent to. Note that the benchmark playbook will hang if this is not set.
  - `QUAY_LOGIN_USER`: Username for authenticating to quay.io.
  - `QUAY_PASS`: Password for authenticating to quay.io.
  - `QUAY_USER`: User hosting the repository for the scaffolding image (ie `quay.io/$QUAY_USER/scaffolding`)
  - `GKE_SAFILE_B64`: Base64 encoded service account credential file for GKE. You can create this using: `cat sa.json | base64`

## Other CI Notes

Available cilium versions can be listed by using:

`cilium install --list-versions`

To help improve build times, buildkit's inline caching is utilized. To manually build and push the container image, be sure to enable it:

`docker build . -t scaffolding:latest --build-arg BUILDKIT_INLINE_CACHE=1`

Other utility scripts:

* [ci/util/artifact.sh](ci/util/artifact.sh): View and download artifacts for a job.
* [ci/util/ppmatrix.sh](ci/util/ppmatrix.sh): Pretty-print rendered [matrix.jsonnet](ci/matrix.jsonnet).
* [ci/util/cleanup.sh](ci/util/cleanup.sh): Cleanup GKE clusters created by the CI.
