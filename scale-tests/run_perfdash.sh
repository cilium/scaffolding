#!/usr/bin/env bash
# ---
# run.sh: Run perfdash locally

set -xeo pipefail

export SERVER_ADDRESS="0.0.0.0:8080"
export BUILDS="100"
export BUCKET="cilium-scale-results"
export GITHUB_CONFIG="https://api.github.com/repos/cilium/scaffolding/contents/scale-tests/jobs"

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
		--githubConfigDir=$GITHUB_CONFIG
}

main () {
    update_perf_tests
    build_perfdash
    run_perfdash
}

main
