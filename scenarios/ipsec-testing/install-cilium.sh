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
encryptionType="$3"  # encryption type
monitor="$4"  # disable monitor events

encryptionEnabled="false"
case $encryptionType in
  none)
    ;;
  ipsec)
    if ! which xxd
    then
      echo "xxd is not installed, failing"
      exit 1
    fi
    encryptionEnabled="true"
    kubectl delete -n kube-system secret cilium-ipsec-keys || true
    kubectl create -n kube-system secret generic cilium-ipsec-keys \
        --from-literal=keys="3 rfc4106(gcm(aes)) $(echo $(dd if=/dev/urandom count=20 bs=1 2> /dev/null | xxd -p -c 64)) 128"
    ;;
  wireguard)
    encryptionEnabled="true"
    ;;
esac

monValues=""
if [ "${monitor}" == "nomon" ]
then
    monValues='--set bpf.monitorAggregation=maximum --set extraArgs[0]=--trace-payloadlen=0'
fi

action="install"
upgradeCompatibility="--set upgradeCompatibility=$version"
upgradeValues=""
if helm list -n kube-system | grep -i cilium
then
  action="upgrade"
  upgradeCompatibility=""
  helm get values -n kube-system cilium > $ARTIFACTS/upgrade-values.yaml
  upgradeValues="-f $ARTIFACTS/upgrade-values.yaml"
fi

export CILIUM_CLI_MODE="helm"

$ARTIFACTS/cilium $action \
  --version $version \
  $upgradeValues \
  $monValues \
  $upgradeCompatibility \
  --set image.override=quay.io/cilium/cilium-ci:$sha-unstripped \
  --set operator.image.override=quay.io/cilium/operator-generic-ci:$sha-unstripped \
  --set l7Proxy=false \
  --set tunnel=vxlan \
  --set encryption.enabled=$encryptionEnabled \
  --set encryption.type=$encryptionType

wait_ready

