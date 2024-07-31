#!/usr/bin/env bash
set -xeo pipefail

. ../common.sh
init

cluster_name="egw-scale-test"

if [ ! -f $ARTIFACTS/cilium ]
then
    pushd $ARTIFACTS
    $SCRIPT/get_ciliumcli.sh
    popd
fi

if ! kind get clusters | grep -i $cluster_name
then
    kind create cluster --config ./kind.yaml
    kubectl taint node -l cilium.io/no-schedule=true cilium.io/no-schedule=true:NoSchedule
fi

if ! helm list -n kube-system | grep -i cilium
then
    api_server_address=$($SCRIPT/get_node_internal_ip.sh "${cluster_name}-control-plane")
    pod_cidr=$($SCRIPT/get_cluster_cidr.sh)
    $ARTIFACTS/cilium install \
        --version v1.15.5 \
        --nodes-without-cilium=true \
        --set k8sServiceHost=${api_server_address} \
        --set ipv4NativeRoutingCIDR=${pod_cidr} \
        --set kubeProxyReplacement=true \
        --set l7Proxy=false \
        --set bpf.masquerade=true \
        --set egressGateway.enabled=true
fi

kubectl create ns monitoring || true

$SCRIPT/retry.sh 3 \
    $ARTIFACTS/toolkit verify k8s-ready --ignored-nodes "${cluster_name}-worker4"
$ARTIFACTS/cilium status --wait --wait-duration=1m

make -C $ROOT_DIR/egw-scale-utils docker-image
CLUSTER=$cluster_name make -C $ROOT_DIR/egw-scale-utils docker-image-load
