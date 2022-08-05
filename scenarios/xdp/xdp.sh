#!/usr/bin/env bash
set -eo pipefail

# Env and common functions
. ../common.sh

# don't do a connectivity test for cilium 
SKIP_CT=""
if [ "${1}" == '-no-ct' ]
then
    SKIP_CT="skip-ct"
    shift 1
fi

# build the golang toolkit bin, which will be used to verify
# k8s pods and nodes are ready
build_toolkit

CILIUM_VERSION=1.11.6  # target cilium version to install

MONITORING_NODE="minikube"  # where monitoring tools will be deployed
SERVER_NODE="minikube-m03"  # where netperf server is deployed
SVC_NODE="minikube"  # where our netperf client will target
# label above nodes with correct scaffolding roles to
# satisfy node selectors from kustomize templates
label_nodes() {
    kubectl label node $MONITORING_NODE role.scaffolding/monitoring=true
    kubectl label node $SERVER_NODE role.scaffolding/pod2pod-server=true
    # we won't use this client, but it needs to go somewhere
    kubectl label node $SERVER_NODE role.scaffolding/pod2pod-client=true
    kubectl label node $SERVER_NODE role.scaffolding/monitored=true
}

SSH="minikube ssh -n"  # how to ssh into nodes
# minikube installs kube-proxy by default, this function removes it
# see https://docs.cilium.io/en/v1.9/gettingstarted/kubeproxy-free/
delete_kube_proxy() {
    set +e
    kubectl -n kube-system delete ds kube-proxy
    kubectl -n kube-system delete cm kube-proxy
    set -e

    for node in $MONITORING_NODE $CLIENT_NODE $SERVER_NODE
    do
        $SSH $node "sudo /bin/bash -c 'iptables-save | grep -v KUBE | iptables-restore'"
        $SSH $node "sudo /bin/bash -c 'echo '"'"'net.ipv4.conf.*.rp_filter = 0'"'"' > /etc/sysctl.d/99-override_cilium_rp_filter.conf && systemctl restart systemd-sysctl'"
    done
}

API_SERVER_IP=""
API_SERVER_PORT=""
# delete kube proxy, grap api server ip and port
prep_for_cilium() {
    delete_kube_proxy
    API_SERVER_IP="$(kubectl get node minikube -o jsonpath='{.status.addresses[0].address}')"
    API_SERVER_PORT="8443"
}

# install cilium using the cilium cli for a non-xdp deployment
install_cilium_no_xdp() {
    $ARTIFACTS/cilium install \
        --version=$CILIUM_VERSION \
        --helm-set endpointRoutes.enabled=true \
        --helm-set kubeProxyReplacement=strict \
        --helm-set k8sServiceHost=${API_SERVER_IP} \
        --helm-set k8sServicePort=${API_SERVER_PORT}
}

# install cilium using the cilium cli for a xdp deployment
# to be precise, enables dsr with xdp acceleration
install_cilium_xdp() {
    $ARTIFACTS/cilium install \
        --version=$CILIUM_VERSION \
        --helm-set endpointRoutes.enabled=true \
        --helm-set kubeProxyReplacement=strict \
        --helm-set k8sServiceHost=${API_SERVER_IP} \
        --helm-set k8sServicePort=${API_SERVER_PORT} \
        --helm-set tunnel=disabled \
        --helm-set autoDirectNodeRoutes=true \
        --helm-set loadBalancer.mode=dsr \
        --helm-set loadBalancer.acceleration=native
}

# wait for cilium to be ready, ensure KubeProxyReplacement is strict
# run a connectivity test
wait_cilium_ready() {
    wait_ready
    $ARTIFACTS/cilium status --wait --wait-duration=1m
    # Verify
    kubectl -n kube-system exec ds/cilium -- cilium status | grep KubeProxyReplacement | grep Strict

    if [ "$SKIP_CT" != "skip-ct" ]
    then   
        $ARTIFACTS/cilium connectivity test
        kubectl delete ns cilium-test
    fi
}

# run netperf against svc/pod2pod-server, which is a nodePort service
run_netperf() {
    ip=$(kubectl get svc pod2pod-server -ojsonpath='{.status.loadBalancer.ingress[0].ip}')
    # defined in the manifest, our nodeport target is 30000

    # -H: target ip
    # -t: test type
    # -l: length of test
    # -j: keep additional test stats
    netperf \
        -H $ip \
        -t TCP_STREAM \
        -l 60s \
        -j
}

# Get cilium cli for status
if ! [ -f "$ARTIFACTS/cilium" ]
then
    cd $ARTIFACTS
    $SCRIPT/get_ciliumcli.sh
    cd $SCENARIO_DIR
fi

# Setup minikube env
# idempodent, so we can have multiple runs of this script
# without having to recreate the cluster
if ! [ "$(kubectl config current-context)" == "minikube" ]
then
    minikube start --driver=kvm2 --nodes=3 --cni=false --network-plugin=cni
    wait_ready
    label_nodes
    prep_for_cilium
else
    wait_ready
    set +e
    label_nodes
    set -e
    prep_for_cilium
fi


# Install cilium if needed
if ! $ARTIFACTS/cilium status --wait --wait-duration="1s"
then
    if [ "$1" == "nxdp" ]
    then
        install_cilium_no_xdp
    else
        install_cilium_xdp
    fi
fi
wait_cilium_ready

# Deploy netperf server, monitoring tools
kustomize build . > $ARTIFACTS/manifest.yaml

# Why create || replace?
# See https://github.com/prometheus-community/helm-charts/issues/1500
kubectl create -f $ARTIFACTS/manifest.yaml || kubectl replace -f $ARTIFACTS/manifest.yaml

# Wait for everything to show ready
wait_ready

# Wait for user input to run netperf test
# For instance, during this time you can pull up grafana
# kubectl port-forward svc/grafana -n monitoring 3000:3000
# admin:admin
# If running on minikube, need to do a `minikube tunnel` call
breakpoint
run_netperf
