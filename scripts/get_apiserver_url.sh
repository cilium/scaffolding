#!/usr/bin/env bash
# ---
# get_apiserver_url.sh
# ---
# get the apiserver's cluster ip and port, formatted as ip:port
# this is accomplished by looking for a pod with "kube-apiserver" in
# its name, and using its spec to find the ip/port
set -eo pipefail

if [ "${1}" == '-d' ]
then
    set -x
    shift 1
fi

api_server_pod=$(kubectl get pods -n kube-system | grep -i kube-apiserver | cut -d' ' -f1)
if [ -z "$api_server_pod" ]
then
    echo "unable to determine api server pod"
    exit 1
fi

ip=$(kubectl get pod -n kube-system $api_server_pod -o jsonpath='{.status.hostIP}')
if [ -z "$ip" ]
then
    echo "unable to determine api server ip"
    exit 1
fi

# some api server pods are hostNetwork, if not all, so just pull the port from
# the readinessProbe probe
port=$(kubectl get pod -n kube-system $api_server_pod -o jsonpath='{.spec.containers[0].readinessProbe.httpGet.port}')
if [ -z "$port" ]
then
    echo "unable to determine api server port"
    exit 1
fi

echo $ip:$port