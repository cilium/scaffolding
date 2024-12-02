#!/usr/bin/env bash
# Install Cilium with the given encryption mode.
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

encryptionType="$1"  # encryption type

encryptionEnabled="false"
case $encryptionType in
  none)
    ;;
  ipsec | ipsec-rps)
    if ! which xxd
    then
      echo "xxd is not installed, failing"
      exit 1
    fi
    encryptionEnabled="true"
    kubectl create -n kube-system secret generic cilium-ipsec-keys \
        --from-literal=keys="3 rfc4106(gcm(aes)) $(echo $(dd if=/dev/urandom count=20 bs=1 2> /dev/null | xxd -p -c 64)) 128" || true
    ;;
  wireguard)
    encryptionEnabled="true"
    ;;
esac

extraArgs=""
if [ "$encryptionType" == "ipsec-rps" ]
then
  extraArgs="--set extraArgs[0]=--enable-ipsec-acceleration=true"
  encryptionType="ipsec"
fi

if helm list -n kube-system | grep -i cilium
then
  $ARTIFACTS/cilium uninstall
fi

export CILIUM_CLI_MODE="helm"
$ARTIFACTS/cilium install \
  --version $CILIUM_VERSION \
  --set image.override=quay.io/cilium/cilium-ci:$CILIUM_SHA-unstripped \
  --set operator.image.override=quay.io/cilium/operator-generic-ci:$CILIUM_SHA-unstripped \
  --set l7Proxy=false \
  --set routingMode=native \
  --set ipv4NativeRoutingCIDR="192.168.0.0/16" \
  --set ipam.mode=cluster-pool \
  --set ipam.operator.clusterPoolIPv4PodCIDRList[0]="192.168.0.0/16" \
  --set ipam.operator.clusterPoolIPv4MaskSize="24" \
  --set autoDirectNodeRoutes=true \
  --set enableEndpointRoutes=true \
  --set hubble.enabled=false \
  --set encryption.enabled=$encryptionEnabled \
  --set encryption.type=$encryptionType \
  --set encryption.wireguard.persistentKeepalive="20s" \
  $extraArgs

kubectl rollout status --watch -n kube-system deploy/cilium-operator
kubectl rollout status --watch -n kube-system ds/cilium
$ARTIFACTS/cilium status --wait

wait_ready
