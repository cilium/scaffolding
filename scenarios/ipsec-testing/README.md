# Datapath Testing with IPSec

This scenario is meant to perform throughput and latency tests with Netperf for
IPSec configured cluseters.

* Two node GKE cluster
* TCP stream, RR and CRR tests
* Pod-to-pod across nodes

Breakdown of each file:

* env.sh: Environment variables holding cluster information. Be sure to modify this file before
  kicking off a test!
* create-cluster.sh: GKE cluster creation and setup.
* kustomization.yaml: Deployments for netperf client and server.
* tagshas.csv: Cilium version information. This should be sorted so newer
  versions of Cilium are at the botton of the file, while older versions
  are at the top.
* install-cilium.sh: Script to install or upgrade cilium using the cilum-cli.
* run.sh: Run the main test.

Usage looks like this:

1. Edit env.sh
2. Exec create-cluster.sh
3. Exec run.sh

The script run.sh will run each netperf test on each Cilium version found in
tagshas.csv. Rather than uninstalling and reinstalling Cilium, upgrades will
be performed, which is why it is important that tagshas.csv has its rows sorted
in ascending order by Cilium version.

Results will be stored in the local directory artifacts/. Netperf results are 
stored in CSV files, with each line representing results from one running instance.
Filenames are adjusted to include the timestamp of when the test started and
which test was performed.

Profiles are stored in artifacts/profiles and are named similarly to the netperf results,
however the name of the node the profile was taken on will be included
in the filename as well.

