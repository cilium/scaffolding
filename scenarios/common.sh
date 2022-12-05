#!/usr/bin/env bash
set -eo pipefail

# init function for main scripts in scenarios, does the following:
# - set environment variables for locations to different directories
#   in the repo (these can be overriden by setting them beforehand)
# - create artifacts directory
# - ensure toolkit is in artifacts directory
init() {
    root_dir_default=$(realpath ../../)
    ROOT_DIR=${ROOT_DIR:-$root_dir_default}
    toolkit_default=${ROOT_DIR}/toolkit
    TOOLKIT=${TOOLKIT:-$toolkit_default}
    script_default=${ROOT_DIR}/scripts
    SCRIPT=${SCRIPT:-$script_default}
    image_default=${ROOT_DIR}/images
    IMAGE=${IMAGE:-$image_default}
    kustomize_default=${ROOT_DIR}/kustomize
    KUSTOMIZE=${KUSTOMIZE:-$kustomize_default}
    
    scenario_dir_default=$(pwd)
    SCENARIO_DIR=${SCENARIO_DIR:-$scenario_dir_default}
    artifacts_default=${SCENARIO_DIR}/artifacts
    ARTIFACTS=${ARTIFACTS:-$artifacts_default}

    mkdir -p $ARTIFACTS
    if ! test -f "$ARTIFACTS/toolkit"
    then
        build_toolkit
    fi
}

# print what is imported from this script and what has been
# overridden by the user
init_print() {
    overridden="(overridden) "
    cat <<EOF
importing the following from common.sh
--- env vars ---
$([[ $SCENARIO_DIR == $scenario_dir_default ]] || echo $overridden)SCENARIO_DIR=$SCENARIO_DIR
$([[ $ROOT_DIR == $root_dir_default ]] || echo $overridden) ROOT_DIR=$ROOT_DIR
$([[ $TOOLKIT == $toolkit_default ]] || echo $overridden)TOOLKIT=$TOOLKIT
$([[ $SCRIPT == $script_default ]] || echo $overridden)SCRIPT=$SCRIPT
$([[ $IMAGE == $image_default ]] || echo $overridden)IMAGE=$IMAGE
$([[ $KUSTOMIZE == $kustomize_default ]] || echo $overridden)KUSTOMIZE=$KUSTOMIZE
$([[ $ARTIFACTS == $artifacts_default ]] || echo $overridden)ARTIFACTS=$ARTIFACTS
--- functions ---
build_toolkit
wait_ready
breakpoint
add_env_var_or_die
wait_cilium_ready
EOF
}

# reset the variables set by this script, including those that may have
# been overridden by the user
reset_vars() {
    for var in ROOT_DIR TOOLKIT SCRIPT IMAGE KUSTOMIZE CENARIO_DIR ARTIFACTS
    do
        unset $var
    done
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

# wait for cilium to be ready and then run a connectivity test
# namespace cilium-test is deleted afterwards on success
# SKIP_CT: set to "skip-ct" to skip the connectivity test
wait_cilium_ready() {
    wait_ready
    $ARTIFACTS/cilium status --wait --wait-duration=1m

    if [ "$SKIP_CT" != "skip-ct" ]
    then
        $ARTIFACTS/cilium connectivity test
        kubectl delete ns cilium-test
    fi
}


# wait for user input
breakpoint() {
    read  -n 1 -p "press key to continue..."
    echo
}

# check env variable is set
env_var_or_die() {
    if [[ -z "${!1}" ]]
    then
        echo "$1 must be set to continue"
        exit 1
    else
        echo "$1: ${!1}"
    fi
}
