#!/usr/bin/env bash
set -eo pipefail

echo "importing the following from $(basename $0):"

SCENARIO_DIR=$(pwd)
ROOT_DIR=$(realpath ../../)
TOOLKIT=$ROOT_DIR/toolkit
SCRIPT=$ROOT_DIR/scripts
IMAGE=$ROOT_DIR/images
KUSTOMIZE=$ROOT_DIR/kustomize
ARTIFACTS=$SCENARIO_DIR/artifacts

echo "---  env vars ---"
cat <<EOF
SCENARIO_DIR=$SCENARIO_DIR
ROOT_DIR=$ROOT_DIR
TOOLKIT=$TOOLKIT
SCRIPT=$SCRIPT
IMAGE=$IMAGE
KUSTOMIZE=$KUSTOMIZE
ARTIFACTS=$ARTIFACTS
EOF

echo "--- functions ---"
echo "build_toolkit"
echo "wait_ready"
echo "breakpoint"

# build toolkit bin
build_toolkit() {
    cd $TOOLKIT
    go build -o $ARTIFACTS/toolkitbin .
    cd $SCENARIO_DIR
}

# wait for cluster to be ready to go
wait_ready () {
    $SCRIPT/retry.sh 5 $SCRIPT/k8s-api-readyz.sh
    $SCRIPT/retry.sh 5 $ARTIFACTS/toolkitbin verify k8s-ready
}


# wait for user input
breakpoint () {
    read  -n 1 -p "press key to continue..."
    echo
}

mkdir -p $ARTIFACTS
set -x
