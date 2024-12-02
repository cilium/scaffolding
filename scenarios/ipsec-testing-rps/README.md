# Datapath Testing with IPSec

This scenario is meant to perform throughput and latency tests with Netperf for
clusters with different encryption modes.

* Three node Equinix Metal cluster
* TCP stream, RR and CRR tests
* Pod-to-pod across nodes

Breakdown of each file:

* env.sh: Environment variables holding cluster information. Be sure to modify this file before
  kicking off a test!
* create-cluster.sh: Cluster creation and setup.
* kustomization.yaml: Deployments for netperf client and server.
* cluster.tfvars, ports.tf: Used by terraform to setup Equinix metal nodes.
* k8s-cluster.yml, terraform.py: Used by ansible to deploy Kubernetes.
* requirements.txt: Python dependencies needed for kubespray.
* notebook/requirements.txt: Dependencies needed to analyze results.
* notebook/flake.nix: Contains a python devshell with the appropriate library
                      paths for Nix users. Enter into the devshell and create a
                      python virtual environment with
                      `python3 -m venv ./venv && . ./venv/bin/activate && pip install requirements.txt`.
                      This can also be used to install the python dependencies
                      that kubespray needs.
* notebook/results.ipynb: Jupyter notebook to create graphs of the results.
                          This contains results from a test executed on November of 2024.
* install-cilium.sh: Script to install cilium using the cilium-cli.
* run.sh: Run the main test.

Usage looks like this:

1. Edit env.sh
2. Set METAL_AUTH_TOKEN to an Equinix Metal API key
3. Exec create-cluster.sh
4. Exec run.sh
5. Wait four to five hours
6. View results with notebook/results.ipynb

The script run.sh will run each netperf test on each of Cilium's encryption modes.

Results will be stored in the local directory artifacts/. Netperf results are
stored in CSV files, with each line representing results from one running instance.

Profiling can be enabled by editing run.sh. See the comments for details.
