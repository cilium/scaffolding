#!/usr/bin/env bash
set -xeo pipefail

. ../common.sh
init

pushd $ARTIFACTS

if ! [ -d ./perf-tests ]; then
  git clone --depth=1 --no-checkout https://github.com/kubernetes/perf-tests

  cd perf-tests

  git sparse-checkout init --cone
  git checkout master
  git sparse-checkout set clusterloader2
fi

popd

