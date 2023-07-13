#!/usr/bin/env bash
# Profile the given node.
# First argumet is the name of the node to profile.
# Second argument is a description that will be used to set the
# output directory name.
set -eo pipefail

. ../common.sh
init

node="$1"
# Replace any . characters with - characters.
# This is done specfically for any semvars included.
desc="$(echo $2 | sed 's/\./-/g')"
name="$node-$desc"

output_dir="$ARTIFACTS/profiles/$(date +%s)-$name"
mkdir -p $output_dir

cwd="$(pwd)"

toolkit=$ARTIFACTS/toolkit

$toolkit ron \
  --node $1 \
  --nsenter \
  --pod-image quay.io/iovisor/bpftrace:latest \
  --pod-name "${name:0:62}" \
  --pvc \
  --pvc-name "${name:0:62}" \
  --auto-copy \
  --auto-copy-dest "$output_dir/profile.tar.gz" \
  --mount ./profile.bt \
  --mount ./profile.sh \
  --configmap-name "${name:0:62}" \
  --host-mounts /usr/src,/lib/modules,/sys/kernel/debug,/sys/kernel/btf \
  --cleanup-all \
  /bin/sh /configs/profile.sh

tar -xzv --strip-components=1 -C $output_dir -f "$output_dir/profile.tar.gz"

