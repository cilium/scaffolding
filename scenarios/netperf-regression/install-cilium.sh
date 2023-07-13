#!/usr/bin/env bash
# Install Cilium with the given version, or perform
# an upgrade to the given version.
# See below for expected arguments.
set -xeo pipefail
. ./env.sh

. ../common.sh
init

if ! [ -f "$ARTIFACTS/cilium" ]
then
  pushd $ARTIFACTS
  $SCRIPT/get_ciliumcli.sh
  popd
fi

version="$1"  # target version to install
sha="$2"  # commit sha for the version
initial="$3"  # if cilium is already installed, first installed version on the cluster

action="install"
upgradeCompatibility="$version"
helmValues=""
if [[ "$initial" != "" ]]
then
  action="upgrade"
  upgradeCompatibility="$initial"
  # Save helm values, as platform-specific information is lost during upgrade.
  # See https://github.com/cilium/cilium-cli/issues/1820
  helm get values -n kube-system cilium > $ARTIFACTS/upgrade-values.yaml
  helmValues="-f $ARTIFACTS/upgrade-values.yaml"
fi

export CILIUM_CLI_MODE="helm"

# Use unstripped builds for profiling information.
$ARTIFACTS/cilium $action \
  --version $version \
  -f $ARTIFACTS/upgrade-values.yaml \
  --set upgradeCompatibility="$upgradeCompatibility" \
  --set image.override=quay.io/cilium/cilium-ci:$sha-unstripped \
  --set operator.image.override=quay.io/cilium/operator-generic-ci:$sha-unstripped

wait_ready

