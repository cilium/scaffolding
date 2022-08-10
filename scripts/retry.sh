#!/usr/bin/env bash
# ---
# retry.sh
# ---
# retry the given command until success, must pass the number
# of seconds to sleep inbetween each attempt as the first argument
# uses 'eval' to run a user's arguments
set -eo pipefail

if [ "${1}" == '-d' ]
then
    set -x
    shift 1
fi

SLEEP=${1}
shift 1

attempts=1
while ! eval "${@}"
do
    echo "failed attempt ${attempts}"
    echo "sleeping until next attempt..."
    for ((i=0; i < $SLEEP; i++))
    do
        echo -n "."
        sleep 1
    done
    echo
    attempts=$(($attempts+1))
done
echo "success after ${attempts} attempt(s)"
