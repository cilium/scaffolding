#!/usr/bin/env bash
set -eo pipefail

remote="$1"
typ="$2"
tmp="/tmp"
duration="180s"
cores=4
common_selectors="ELAPSED_TIME"

# collapse will combine results from multiple netperf
# runs into one CSV file.
collapse() {
  cat $tmp/0 > $tmp/$1.csv

  for (( i=1; i<$cores; i++ ))
  do
    # Use tail -n +3 to skip header in the file.
    tail -n +3 $tmp/$i >> $tmp/$1.csv
  done

  cat $tmp/$1.csv
}

stream() {
  echo "Starting TCP Stream..."

  for (( i=0; i<$cores; i++ ))
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

  for (( i=0; i<$cores; i++ ))
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

  for (( i=0; i<$cores; i++ ))
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
