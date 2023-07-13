#!/usr/bin/env bash
# Create a two node gke cluster for testing.
# Nodes are labeled with pod2pod client and server roles
# for scheduling, and each node will have its conntrack table
# size increased.
set -xeo pipefail
. ./env.sh

. ../common.sh
init

gcloud container clusters create \
  $CLUSTER_NAME \
  --labels "usage=$DEV_REASON,owner=$OWNER_NAME" \
  --num-nodes 2 \
  --machine-type e2-custom-2-4096 \
  --disk-type pd-standard \
  --disk-size 20GB \
  --node-taints "node.cilium.io/agent-not-ready=true:NoExecute"


label=server
for node in $(kubectl get node -o name)
do
  kubectl label --overwrite $node role.scaffolding/pod2pod-$label=true
  label=client
  # CRR tests fail with drops due to limited conntrack size
  $ARTIFACTS/toolkit ron -- sysctl -w net.netfilter.nf_conntrack_max=262144
done

