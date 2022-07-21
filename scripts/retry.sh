#!/usr/bin/env bash
set -eo pipefail

VERBOSE=""
if [ "${1}" == '-d' ]
then
    set -x
    VERBOSE="--verbose"
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
