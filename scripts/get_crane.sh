#!/usr/bin/env bash
# ---
# get_crane.sh
# ---
# Download crane binary from github.com/google/go-containerregistry
set -eo pipefail

VERBOSE=""
if [ "${1}" == '-d' ]
then
    set -x
    shift 1
fi

# Platform name needs to be 'titled'
# See https://stackoverflow.com/questions/42925485/making-a-script-that-transforms-sentences-to-title-case
tc() { set ${*,,} ; echo ${*^} ; }

CRANE_VERSION=$(curl -s https://api.github.com/repos/google/go-containerregistry/releases/latest | jq -r '.tag_name')

GOOS=$(tc $(go env GOOS))
GOARCH=$(go env GOARCH)

# These folks use x86_64 instead of amd64
if [ "${GOARCH}" == "amd64" ]
then
    GOARCH="x86_64"
fi

TARBALL="go-containerregistry_${GOOS}_${GOARCH}.tar.gz"

curl -L --remote-name-all https://github.com/google/go-containerregistry/releases/download/${CRANE_VERSION}/{${TARBALL},checksums.txt}
sha256sum --ignore-missing -c checksums.txt
tar -xzvf $TARBALL crane

rm $TARBALL checksums.txt