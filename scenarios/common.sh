#!/usr/bin/env bash
set -eo pipefail

# init function for main scripts in scenarios, does the following:
# - set environment variables for locations to different directories
#   in the repo
# - create artifacts directory
# - ensure toolkit is in artifacts directory
init() {
    SCENARIO_DIR=$(pwd)
    ROOT_DIR=$(realpath ../../)
    TOOLKIT=$ROOT_DIR/toolkit
    SCRIPT=$ROOT_DIR/scripts
    IMAGE=$ROOT_DIR/images
    KUSTOMIZE=$ROOT_DIR/kustomize
    ARTIFACTS=$SCENARIO_DIR/artifacts

    mkdir -p $ARTIFACTS
    if ! test -f "$ARTIFACTS/toolkit"
    then
        build_toolkit
    fi
}

# print what is imported from this script
init_print() {
    cat <<EOF
importing the following from common.sh
--- env vars ---
SCENARIO_DIR=$SCENARIO_DIR
ROOT_DIR=$ROOT_DIR
TOOLKIT=$TOOLKIT
SCRIPT=$SCRIPT
IMAGE=$IMAGE
KUSTOMIZE=$KUSTOMIZE
ARTIFACTS=$ARTIFACTS
--- functions ---
build_toolkit
wait_ready
breakpoint
EOF
}


# build toolkit bin
build_toolkit() {
    cd $TOOLKIT
    go build -o $ARTIFACTS/toolkit .
    cd $SCENARIO_DIR
}

# wait for cluster to be ready to go
wait_ready() {
    $SCRIPT/retry.sh 5 $SCRIPT/k8s_api_readyz.sh
    $SCRIPT/retry.sh 5 $ARTIFACTS/toolkit verify k8s-ready
}


# wait for user input
breakpoint() {
    read  -n 1 -p "press key to continue..."
    echo
}
