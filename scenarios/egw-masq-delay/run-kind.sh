#!/usr/bin/env bash
set -xeo pipefail

. ../common.sh
init

cluster_name="egw-scale-test"

# n is the number of client pods to create
n=$1
# qps is the number of client pods to create per second
qps=$2
# baseline represents if we are running a baseline test or not
# a baseline test will be triggered if this argument is non-empty
baseline=$3

# Cleanup previous run, if needed
kubectl delete cegp/egw-scale-test-route-external || true

$SCRIPT/retry.sh 3 \
  $ARTIFACTS/toolkit verify k8s-ready --ignored-nodes "${cluster_name}-worker4"

if ! [ -z "$baseline" ]; then
  baseline="baseline"
fi

test_dir=$ROOT_DIR/egw-scale-utils/test

# Run preflight steps
pushd $test_dir
EGW_IMAGE_TAG=latest ./preflight.sh $baseline
popd

# Run the test
pushd $ARTIFACTS/perf-tests/clusterloader2

export CL2_PROMETHEUS_PVC_ENABLED=false
export CL2_ENABLE_PVS=false
export CL2_PROMETHEUS_NODE_SELECTOR="role.scaffolding/monitoring: \"true\""
export CL2_PROMETHEUS_SCRAPE_APISERVER_ONLY=true
export CL2_NUM_EGW_CLIENTS=$n
export CL2_EGW_CLIENTS_QPS=$qps

go run ./cmd/clusterloader.go \
  --testconfig=$test_dir/config.yaml \
  --provider=kind \
  --kubeconfig=${HOME}/.kube/config \
  --v=2 \
  --enable-prometheus-server \
  --prometheus-apiserver-scrape-port=6443 \
  --tear-down-prometheus-server=false \
  --prometheus-additional-monitors-path=$test_dir/prom-extra-podmons \
  --testoverrides=$SCENARIO_DIR/overrides.yaml \
  --report-dir=$ARTIFACTS/$(date +%s)

