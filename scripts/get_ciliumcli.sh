#!/usr/bin/env bash
# ---
# get_ciliumcli.sh
# ---
# Download latest binary release of the Cilium CLI
# Uses instructions from the README, but places binary in
# cwd rather than /usr/local/bin
set -eo pipefail

VERBOSE=""
if [ "${1}" == '-d' ]
then
    set -x
    VERBOSE="--verbose"
    shift 1
fi

CILIUM_CLI_VERSION=$(curl -s https://raw.githubusercontent.com/cilium/cilium-cli/master/stable.txt)
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)

curl $VERBOSE -L --remote-name-all https://github.com/cilium/cilium-cli/releases/download/${CILIUM_CLI_VERSION}/cilium-${GOOS}-${GOARCH}.tar.gz{,.sha256sum}
sha256sum --check cilium-${GOOS}-${GOARCH}.tar.gz.sha256sum
tar -xzvf cilium-${GOOS}-${GOARCH}.tar.gz

rm cilium-${GOOS}-${GOARCH}.tar.gz{,.sha256sum}