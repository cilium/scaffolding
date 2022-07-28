#!/usr/bin/env bash
# ---
# exec_with_registry.sh
# port-forward the 'registry' service to localhost on port
# 5000, execute the given command, and kill the port forward.
# can be used to facilitate pushing images to the registry.
set -eo pipefail

VERBOSE=""
if [ "${1}" == '-d' ]
then
    set -x
    shift 1
fi

RETRY=$(dirname -- "$0")/retry.sh
if ! stat $RETRY
then
    echo "unable to continue, cannot find retry script"
    echo "it should be in the same directory as this script"
    exit 1
fi

# first find the registry service
RESULT=$(kubectl get svc -A \
  --selector app.kubernetes.io/part-of=scaffolding \
  --selector app.kubernetes.io/name=registry \
  -o json)

NUM_RESULTS=$(echo $RESULT | jq -r '.items | length')
if [ "${NUM_RESULTS}" != "1" ]
then
    echo "unable to continue, could not find exactly one matching registry service"
    exit 1
fi

NAME=$(echo $RESULT | jq -r '.items[0].metadata.name')
NAMESPACE=$(echo $RESULT | jq -r '.items[0].metadata.namespace')

echo "forwarding svc/$NAME from ns $NAMESPACE"
kubectl port-forward svc/registry -n registry 5000:5000 &

PF_PID=$!
echo "port-forward started with pid $PF_PID"

echo "waiting for registry to be available"
$RETRY 1 curl http://localhost:5000

set +e  # allows user command to fail 'successfully', so we can cleanup
echo "executing given command"
eval "${@}"

echo "stopping port-forward"
kill $PF_PID