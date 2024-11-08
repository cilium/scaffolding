#!/usr/bin/env bash
# Run netperf against different encryption modes of Cilium,
# including IPSec w/ RPS.
set -xe

. ../common.sh
init

if ! [ -d "$ARTIFACTS/FlameGraph" ]
then
  git clone https://github.com/brendangregg/FlameGraph $ARTIFACTS/FlameGraph
fi

if ! [ -f "$ARTIFACTS/netperf.sh" ]
then
  cp $SCRIPT/netperf.sh $ARTIFACTS/netperf.sh
fi

duration="180"
proto="tcp"

# Set this to "yes" to enabling profiling via perf.
# May be unstable with long durations, so use with caution
# during long tests.
PROFILE="no"

run_test() {
  my_enc=$1
  my_typ=$2
  my_cores=$3
  my_duration=$4

  ctx="$proto-$my_enc-$my_typ-$my_cores"
  echo $ctx > $ARTIFACTS/last_ctx

  # Setup netperf server and client pods.
  kustomize build . | kubectl apply -f -
  wait_ready

  # This time is added on each artifact file.
  time="$(date +%s)"

  if [ "${PROFILE}" == "yes" ]; then
    # Start profiling each node.
    # Hacky way to tag if the node contains the server or client pod, will be:
    # node/<node name>/(client|server)
    clientNode="$(kubectl get node -l role.scaffolding/pod2pod-client=true -o name)/client"
    serverNode="$(kubectl get node -l role.scaffolding/pod2pod-server=true -o name)/server"
    for noderole in $clientNode $serverNode
    do
        node="$(echo $noderole |  cut -d '/' -f 2)"
        role="$(echo $noderole | cut -d '/' -f 3)"
        log="$ARTIFACTS/$ctx-$node-$time.log"

        touch $log  # fix race condition between tee and tail below
        ARTIFACTS=$ARTIFACTS $SCRIPT/profile_node.sh -d $node "$ctx-$role" $my_duration 2>&1 | tee $log &
        echo $!  # in case something fails, for cleanup
    done

    # The script profile-node.sh runs on each node will log 'Starting profile' before
    # running perf to profile nodes. Wait for these logs to show before moving on, to
    # ensure that profiles are actually running when the netperf tests start.
    grep -m2 'Starting profile' <(tail -n0 -f $ARTIFACTS/*-$time.log)
    echo "profiles started"
  fi

  ip="$(kubectl get pod -l app=pod2pod-server -ojsonpath='{.items[0].status.podIP}')"
  kubectl exec -it deploy/pod2pod-client -- /bin/bash /netperf-script/netperf.sh $ip "${my_duration}s" $my_cores $my_typ $proto

  wait  # for profiles to be done

  # Grab the results
  $SCRIPT/retry.sh 5 kubectl exec -it deploy/pod2pod-client -- cat /tmp/$my_typ-$proto.csv > $ARTIFACTS/$ctx.csv

  # Delete the netperf server and client for a reset.
  kustomize build . | kubectl delete -f -
  wait_ready

  while kubectl get pods | grep pod2pod
  do
    sleep 5
    echo "waiting for pod2pod pods to be deleted"
  done

  # Reset conntrack tables on each node.
  for node in $(kubectl get node -l '!node-role.kubernetes.io/control-plane' -o name)
  do
    node="$(echo $node | cut -d '/' -f 2)"
    for table in conntrack expect
    do
      $ARTIFACTS/toolkit ron \
      --node $node \
      --tolerate-all \
      --nsenter \
      --nsenter-opts="-t 1 -m" \
      -- conntrack -F $table
    done
  done
}

for enc in none wireguard ipsec ipsec-rps
do
  ./install-cilium.sh $enc

  # Perform a seed test so tunnels can be established if needed.
  echo "Running seed test"
  run_test seed-$enc stream 4 3

  for typ in stream rr crr
  do
    for cores in 1 2 4 8 12 16
    do
      run_test $enc $typ $cores $duration
    done  # cores
  done  # typ
done  # enc
