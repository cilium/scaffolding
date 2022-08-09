#!/usr/bin/env bash
# ---
# get_node_internal_ip.sh
# ---
# get a node's internal ip address
set -eo pipefail

VERBOSE=""
if [ "${1}" == '-d' ]
then
    set -x
    VERBOSE="--verbose"
fi

kubectl get node $1 -ojson | \
    jq -r '.status.addresses[] | select(.type=="InternalIP") | .address'
