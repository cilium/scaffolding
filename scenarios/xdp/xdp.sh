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

# get cilium cli for install/status
if ! [ -f "$ARTIFACTS/cilium" ]
then
    cd $ARTIFACTS
    $SCRIPT/get_ciliumcli.sh
    cd $SCENARIO_DIR
fi

# -- variables --
# anything in all caps is used in a global scope
CILIUM_VERSION=1.11.6  # target cilium version to install
CONTROL_PLANE_NODE="minikube"  # where k8s api server is running
MONITORING_NODE="minikube"  # where monitoring tools will be deployed
SERVER_NODE="minikube-m02"  # where netperf server is deployed
LB_NODE="minikube-m03"  # where our metallb loadbalancer will be pinned
ALL_NODES=("minikube" "minikube-m02" "minikube-m03")  # list of nodes to work with
SSH="minikube ssh -n"  # how to ssh into nodes
# these three are set at runtime
API_SERVER_IP=""  # ip of api server
API_SERVER_PORT=""  # port of api server
CLUSTER_CIDR=""  # cluster cidr of the cluster

# -- functions --
# create a new three node minikube cluster using kvm
# sets queue length for each interface to 4 by editing each node's
# xml and doing a reboot
# see https://marc.info/?l=xdp-newbies&m=157431975621734&w=2
new_minikube() {
    # assume 2 cpus per node
    # if that changes, adjust the queues below to be twice the number of cpus
    # per node
    minikube start --driver=kvm2 --nodes=3 --cni=false --network-plugin=cni
    for node in ${ALL_NODES[@]}
    do
        virsh --connect qemu:///system dumpxml $node > $ARTIFACTS/$node.xml
        sed -i 's/<interface type='"'"'network'"'"'>/<interface type='"'"'network'"'"'>\n<driver name='"'"'vhost'"'"' queues='"'"'4'"'"'\/>/g' artifacts/$node.xml
        virsh --connect qemu:///system define $ARTIFACTS/$node.xml
        virsh --connect qemu:///system shutdown $node
    done
    # wait until all nodes are down
    $SCRIPT/retry.sh 4 '[ ! "$(virsh --connect qemu:///system list | grep -i minikube)" ]'
    minikube start
}

# set correct date for each node and diasble rp_filter accross the board
node_prep() {
    now=$(date -u)
    for node in ${ALL_NODES[@]}
    do
        $SSH $node "sudo date --set=\"$now\""
        $SSH $node "sudo /bin/bash -c 'echo '"'"'net.ipv4.conf.*.rp_filter = 0'"'"' > /etc/sysctl.d/99-override_cilium_rp_filter.conf && systemctl restart systemd-sysctl'"
    done
}

# label nodes with correct scaffolding roles to
# satisfy node selectors from kustomize templates
label_nodes() {
    kubectl label nodes --all role.scaffolding/monitored=true
    kubectl label node $MONITORING_NODE role.scaffolding/monitoring=true
    kubectl label node $SERVER_NODE role.scaffolding/pod2pod-server=true
    kubectl label node $LB_NODE role.scaffolding/lb=true
}

# minikube installs kube-proxy by default, this function removes it
# see https://docs.cilium.io/en/v1.9/gettingstarted/kubeproxy-free/
delete_kube_proxy() {
    set +e
    kubectl -n kube-system get ds kube-proxy -o yaml > $ARTIFACTS/kube-proxy-ds.yaml
    kubectl -n kube-system delete ds kube-proxy
    kubectl -n kube-system get cm kube-proxy -o yaml > $ARTIFACTS/kube-proxy-cm.yaml
    kubectl -n kube-system delete cm kube-proxy 
    set -e

    for node in ${ALL_NODES[@]}
    do
        $SSH $node "sudo /bin/bash -c 'iptables-save | grep -v KUBE | iptables-restore'"
    done
}

# ensure that cilium is installed with KubeProxyReplacement set to strict
# verifies by using cilium status on a node
# see https://docs.cilium.io/en/v1.12/gettingstarted/kubeproxy-free/#validate-the-setup
ensure_strict_kpr() {
    kubectl -n kube-system exec ds/cilium -- cilium status | grep KubeProxyReplacement | grep Strict
}

# basic install of cilium, no kube-proxy replacement (kpr), dsr, xdp
install_cilium_no_kpr() {
    $ARTIFACTS/cilium install \
        --version=$CILIUM_VERSION
}

# install cilium with kube-proxy replacement (kpr), no dsr or xdp
# deletes kube-proxy if it exists in the cluster
install_cilium_kpr_no_xdp() {
    delete_kube_proxy
    wait_ready
    $ARTIFACTS/cilium install \
        --version=$CILIUM_VERSION \
        --helm-set endpointRoutes.enabled=true \
        --helm-set kubeProxyReplacement=strict \
        --helm-set k8sServiceHost=${API_SERVER_IP} \
        --helm-set k8sServicePort=${API_SERVER_PORT}
    ensure_strict_kpr
}

# install cilium with kub-proxy replacement (kpr), dsr and xdp acceleration
# deletes kube-proxy if it exists in the cluster
install_cilium_kpr_xdp() {
    delete_kube_proxy
    wait_ready
    $ARTIFACTS/cilium install \
        --version=$CILIUM_VERSION \
        --helm-set ipv4NativeRoutingCIDR=${CLUSTER_CIDR} \
        --helm-set endpointRoutes.enabled=true \
        --helm-set kubeProxyReplacement=strict \
        --helm-set k8sServiceHost=${API_SERVER_IP} \
        --helm-set k8sServicePort=${API_SERVER_PORT} \
        --helm-set tunnel=disabled \
        --helm-set autoDirectNodeRoutes=true \
        --helm-set loadBalancer.mode=dsr \
        --helm-set loadBalancer.acceleration=native
    ensure_strict_kpr
}

# wait for cilium to be ready and then run a connectivity test
wait_cilium_ready() {
    wait_ready
    $ARTIFACTS/cilium status --wait --wait-duration=1m


    if [ "$SKIP_CT" != "skip-ct" ]
    then   
        $ARTIFACTS/cilium connectivity test
        kubectl delete ns cilium-test
    fi
}

# install metallb l2 load balancer
# this probably won't apply to all environments, so adjust if needed
install_metallb() {
    kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.4/config/manifests/metallb-native.yaml
    wait_ready
    kubectl apply -f $SCENARIO_DIR/metallb_l2.yaml
}

# for an l2 lb, this verifies that the mac address of the pod2pod-server ip
# matches the mac address of the internal ip for LB_NODE
# this verifies that traffic to pod2pod-server will only pass through LB_NODE
verify_lb_pinned_l2() {
    lb_ip=$($SCRIPT/get_node_internal_ip.sh $LB_NODE)
    svc_ip=$(kubectl get svc pod2pod-server -ojsonpath='{.status.loadBalancer.ingress[0].ip}')
    # use ping to force host to lookup mac addrs
    for ip in $lb_ip $svc_ip
    do
        ping -c1 -W1 $ip || true
    done

    arp_info=$(ip neigh)
    lb_mac=$(echo "$arp_info" | grep "$lb_ip" | awk '{print $5}')
    svc_mac=$(echo "$arp_info" | grep "$svc_ip" | awk '{print $5}')

    if [ "$lb_mac" != "$svc_mac" ]
    then
        echo "cannot continue, as mac addr for pod2pod-server is not pinned to $LB_NODE"
        exit 1
    fi
}

# Setup minikube env
# idempodent, so we can have multiple runs of this script
# without having to recreate the cluster
if ! [ "$(kubectl config current-context)" == "minikube" ]
then
    new_minikube
    wait_ready
    node_prep
    wait_ready
    label_nodes
else
    wait_ready
    set +e
    label_nodes
    set -e
fi

_api_server_url=$($SCRIPT/get_apiserver_url.sh)
API_SERVER_IP=$(echo "$_api_server_url" | cut -d':' -f1)
API_SERVER_PORT=$(echo "$_api_server_url" | cut -d':' -f2)
CLUSTER_CIDR=$($SCRIPT/get_cluster_cidr.sh)

# Install cilium if needed
if ! $ARTIFACTS/cilium status --wait --wait-duration="1s"
then
    if [ "$1" == "nkpr" ]
    then
        install_cilium_no_kpr
    elif [ "$1" == "kpr" ]
    then
        install_cilium_kpr_no_xdp
    elif [ "$1" == "xdp" ]
    then
        install_cilium_kpr_xdp
    else
        echo "expected one of: nkpr, kpr, xdp"
        exit 1
    fi
fi
wait_cilium_ready

install_metallb
wait_ready

# Deploy netperf server, monitoring tools
kustomize build . > $ARTIFACTS/manifest.yaml

# Why create || replace?
# See https://github.com/prometheus-community/helm-charts/issues/1500
# the || true is there when we have an existing pvc for grafana
kubectl create -f $ARTIFACTS/manifest.yaml || kubectl replace -f $ARTIFACTS/manifest.yaml || true

# Wait for everything to show ready
wait_ready

# Insure our load balancer is setup correctly
verify_lb_pinned_l2

# Wait for user input to run netperf test
# For instance, during this time you can pull up grafana
# kubectl port-forward svc/grafana -n monitoring 3000:3000
# admin:admin
breakpoint
$SCENARIO_DIR/netperf.sh | tee -a $ARTIFACTS/netperf_$(date +%s)_"$1".txt
