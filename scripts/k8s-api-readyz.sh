#!/usr/bin/env bash
# ---
# k8s-api-readyz.sh
# get ip and ca of k8s api server from kubeconfig, then contact
# `/readyz?verbose`
#
# uses a temporary file descriptor, (<(echo ca)) to pass the 
# ca into curl, meaning no `--insecure` is passed into curl
set -eo pipefail

VERBOSE=""
if [ "${1}" == '-d' ]
then
    set -x
    VERBOSE="--verbose"
fi

CURRENT_CONTEXT=$(kubectl config current-context)
CONFIG=$(kubectl config view --raw -o json)
CLUSTER=$(echo "$CONFIG" | jq '.clusters[] | select(.name=="'$CURRENT_CONTEXT'")')
CLUSTER_NAME=$(echo "$CLUSTER" | jq -r '.name')
CLUSTER_URL=$(echo "$CLUSTER" | jq -r '.cluster.server')
CA=$(echo "$CLUSTER" | jq -r '.cluster."certificate-authority-data"' | base64 -d)

echo "${CLUSTER_NAME} (${CLUSTER_URL})"
curl $VERBOSE $CLUSTER_URL'/readyz?verbose' --cacert <(echo "$CA")