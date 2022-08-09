#!/usr/bin/env bash
# ---
# get_cluster_cidr.sh
# ---
# get the cluster cidr of the cluster
# quick one-liner, but hard to remeber, so, here ya go
# two notes:
# 1. to get the cluster info, look for the 'cluster-cidr' kubelet arg, which 
# will be formatted as '--cluster-cidr=10.244.0.0/16'
# 2. use process substitution for cluster-info to avoid 144 rc from grep
# see https://stackoverflow.com/questions/19120263/why-exit-code-141-with-grep-q
grep -m 1 -o -E -- '--cluster-cidr=[0-9./]+' <(kubectl cluster-info dump) | cut -d'=' -f2