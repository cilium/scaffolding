#!/usr/bin/env bash
# Run netperf against different versions of Cilium.
# Assumes cluster has just been created an Cilium is not
# installed yet.
set -xe

. ../common.sh
init

duration="180"
cores="2"

initial_version=""
versions=$(cat "./tagshas.csv")
for version in $versions
do
  # Skip the header row
  if [ "${version}" == "tag,sha" ]
  then
    continue
  fi

  tag="$(echo $version | cut -d "," -f 1)"
  sha="$(echo $version | cut -d "," -f 2)"

  # Install Cilium with the found version

  echo "$tag,$sha"

  ./install-cilium.sh $tag $sha $initial_version

  # If this is the first time Cilium is installed, save the initial version
  # so an upgrade can be performed next.
  if [[ "$initial_version" == "" ]]
  then
    initial_version="$tag"
  fi

  for typ in stream rr crr
  do
    # Setup netperf server and client pods.
    kustomize build . | kubectl apply -f -
    wait_ready

    # This time is added on each artifact file.
    time="$(date +%s)"

    # Start profiling each node.
    # Hacky way to tag if the node contains the server or client pod, will be:
    # node/<node name>/(client|server)
    clientNode="$(kubectl get node -l role.scaffolding/pod2pod-client=true -o name)/client"
    serverNode="$(kubectl get node -l role.scaffolding/pod2pod-server=true -o name)/server"
    for noderole in $clientNode $serverNode
    do
      node="$(echo $noderole |  cut -d '/' -f 2)"
      role="$(echo $noderole | cut -d '/' -f 3)"
      log="$ARTIFACTS/$node-$typ-$tag-$time.log"

      touch $log  # fix race condition between tee and tail below
      $ARTIFACTS/profile-node.sh $node "$typ-$tag-$role" $duration 2>&1 | tee $log &
      echo $!  # in case something fails, for cleanup
    done

    # The script profile-node.sh runs on each node will log 'Starting profile' before
    # running bpftool to profile nodes. Wait for these logs to show before moving on, to
    # ensure that profiles are actually running when the netperf tests start.
    grep -m2 'Starting profile' <(tail -n0 -f $ARTIFACTS/*-$time.log)
    echo "profiles started"

    ip="$(kubectl get pod -l app=pod2pod-server -ojsonpath='{.items[0].status.podIP}')"
    kubectl exec -it deploy/pod2pod-client -- /bin/bash /netperf-script/netperf.sh $ip "${duration}s" $cores $typ

    wait  # for profiles to be done

    # Grab the results
    $SCRIPT/retry.sh 5 kubectl exec -it deploy/pod2pod-client -- cat /tmp/$typ.csv > $ARTIFACTS/$typ-$tag-$time.csv

    # Delete the netperf server and client for a reset.
    # This probably isn't needed.
    kustomize build . | kubectl delete -f -
    wait_ready

    while kubectl get pods | grep pod2pod
    do
      sleep 5
      echo "waiting for pod2pod pods to be deleted"
    done
  done
done
