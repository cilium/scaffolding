#!/usr/bin/env bash
set -xeo pipefail

fill_template() {
    echo $EGW_EXTERNAL_TARGET_CIDR
    cat $1 | envsubst | tee > $(echo $1 | sed 's/\.tmpl//')
}

get_node_internal_ip() {
    kubectl get node -l $1 -ojsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}'
}

if [ "$1" != "baseline" ]; then
    external_target_ip=$(get_node_internal_ip "role.scaffolding/egw-node=true")
    export EGW_ALLOWED_CIDR="${external_target_ip}/32"
else
    export EGW_ALLOWED_CIDR="0.0.0.0/0"
fi

egw_node_ip=$(get_node_internal_ip "cilium.io/no-schedule=true") 
export EGW_EXTERNAL_TARGET_CIDR="${egw_node_ip}/32"
export EGW_EXTERNAL_TARGET_ADDR="${egw_node_ip}"

for template in ./manifests/*.tmpl.yaml; do
    fill_template $template
done

if [ "$1" != "baseline" ]; then
    kubectl apply -f ./manifests/egw-policy.yaml
elif  kubectl get ciliumegressgatewaypolicies.cilium.io egw-scale-test-route-external; then
    kubectl delete  kubectl get ciliumegressgatewaypolicies.cilium.io/egw-scale-test-route-external 
fi
