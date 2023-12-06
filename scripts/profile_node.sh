#!/usr/bin/env bash
# ---
# profile_node.sh [-d] node desc duration
# ---
# profile the given kubernetes node's userspace and kernelspace
# stacks using perf. The first argument is the name of the node
# to profile, the second argument is a short description to use
# as a reference, and the third argument is the number of seconds
# the profile should be run for.
#
# Results from the profile will be placed into the artifacts
# directory, with a name formatted as:
# '<unix timestamp>-<node name>-<description>`.
#
# After the profile is finished, a flamegraph will automatically
# be generated from the profile.
#
# If `profile.sh` is found in the artifacts directory, it will
# be called inside the pod to collect the node's profile. Otherwise,
# a script will be placed there.
set -eo pipefail

if [ "${1}" == '-d' ]
then
  set -x
  shift 1
fi

if [ -z "$ARTIFACTS" ]
then
  echo "Need \$ARTIFACTS variable to be set, failing"
  exit 1
fi

if ! [ -d $ARTIFACTS/FlameGraph ]
then
  git clone https://github.com/brendangregg/FlameGraph $ARTIFACTS/FlameGraph
fi

if ! [ -f $ARTIFACTS/profile.sh ]
then
  cat <<EOF > $ARTIFACTS/profile.sh
  #!/bin/sh
  echo "Starting profile..."
  echo "Duration: \$1 seconds"

  time="\$(date +%s)"
  nsenter -t 1 -S 0 -G 0 \
    -m -u -i -n -p \
    /bin/sh -c " \
    cd /tmp && \
    mkdir -p \$time && \
    cd \$time && \
    perf record -F 99 -a -g -o perf.data -- sleep \$1 && \
    perf script > script.data"

  cp /host/tmp/\$time/script.data /store/script.data

  echo "Done"
EOF
fi

node="$1"
# Replace any . characters with - characters.
# This is done specfically for any semvars included.
desc="$(echo $2 | sed 's/\./-/g')"
name="$node-$desc"
duration="$3"

output_dir="$ARTIFACTS/profiles/$(date +%s)-$name"
mkdir -p $output_dir

toolkit=$ARTIFACTS/toolkit

$toolkit ron \
  --node $1 \
  --nsenter \
  --pod-image alpine \
  --pod-name "$desc" \
  --pvc \
  --pvc-name "$desc" \
  --auto-copy \
  --auto-copy-dest "$output_dir/profile.tar.gz" \
  --mount $ARTIFACTS/profile.sh \
  --configmap-name "$desc" \
  --host-mounts /tmp \
  --cleanup-all \
  /bin/sh /configs/profile.sh $duration

tar -xzv --strip-components=1 -C $output_dir -f "$output_dir/profile.tar.gz"
cat $output_dir/script.data | perl -w $ARTIFACTS/FlameGraph/stackcollapse-perf.pl > $output_dir/collapsed
cat $output_dir/collapsed | perl -w $ARTIFACTS/FlameGraph/flamegraph.pl > $output_dir/graph.svg

