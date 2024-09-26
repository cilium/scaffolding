#!/usr/bin/env bash
# ---
# run.sh: Run perfdash locally

set -xeo pipefail

export SERVER_ADDRESS="${SERVER_ADDRESS:-0.0.0.0:8080}"
export BUILDS="${BUILDS:-10}"
export BUCKET="${BUCKET:-cilium-scale-results}"
export CONFIG_PATH="${CONFIG_PATH:-../../jobs/tests.yaml}"
export STORAGE_URL="https://console.cloud.google.com/storage/browser"

update_perf_tests () {
    if ! [ -d ./perf-tests ]
    then
      git clone https://github.com/kubernetes/perf-tests ./perf-tests
    fi
    cd ./perf-tests
    git pull
}

build_perfdash () {
    cd ./perfdash
    make perfdash
}

run_perfdash () {
    ./perfdash \
        --www \
        --address=$SERVER_ADDRESS \
        --builds=$BUILDS \
        --force-builds \
        --logsBucket=$BUCKET \
        --configPath=$CONFIG_PATH \
        --storageURL=$STORAGE_URL
}

main () {
    update_perf_tests
    build_perfdash
    run_perfdash
}

main
