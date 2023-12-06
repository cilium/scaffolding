#!/usr/bin/env bash
# ---
# netperf.sh [-d] remote duration p (stream,rr,crr)
# ---
# run a netperf stream, rr or crr test, outputting results into
# a CSV file. Each row in the CSV file is a set of results from
# one netperf instance.
#
# The first argument is the remote target IP of the netserver
# (passed to -H), second argument is the duration of the test
# (passed to -l), the third argument is the number of parallel
# netperf instances to run at once, and the forth argument is the
# type of test to run (one of stream, rr or crr).
set -eo pipefail

if [ "${1}" == '-d' ]
then
  set -x
  shift 1
fi

remote="$1"
duration="$2"
p="$3"
typ="$4"

tmp="/tmp"
common_selectors="ELAPSED_TIME"

# collapse will combine results from multiple netperf
# runs into one CSV file.
collapse() {
  cat $tmp/0 > $tmp/$1.csv

  for (( i=1; i<$p; i++ ))
  do
    # Use tail -n +3 to skip header in the file.
    tail -n +3 $tmp/$i >> $tmp/$1.csv
  done

  cat $tmp/$1.csv
}

stream() {
  echo "Starting TCP Stream..."

  for (( i=0; i<$p; i++ ))
  do
    netperf -H $remote \
      -t tcp_stream \
      -l $duration \
      -- \
      -o THROUGHPUT,THROUGHPUT_UNITS,$common_selectors \
    > $tmp/$i &
  done

  wait

  collapse stream
}

rr() {
  echo "Starting TCP RR..."

  for (( i=0; i<$p; i++ ))
  do
    netperf -H $remote \
      -t tcp_rr \
      -l $duration \
      -- \
      -o P50_LATENCY,P90_LATENCY,P99_LATENCY,RT_LATENCY,REQUEST_SIZE,RESPONSE_SIZE,$common_selectors \
    > $tmp/$i &
  done

  wait

  collapse rr
}

crr() {
  echo "Starting TCP CRR..."

  for (( i=0; i<$p; i++ ))
  do
    netperf -H $remote \
      -t tcp_crr \
      -l $duration \
      -- \
      -o P50_LATENCY,P90_LATENCY,P99_LATENCY,RT_LATENCY,REQUEST_SIZE,RESPONSE_SIZE,$common_selectors \
    > $tmp/$i &
  done

  wait

  collapse crr
}

case $typ in
  stream)
    stream
    ;;
  rr)
    rr
    ;;
  crr)
    crr
    ;;
esac
