#!/usr/bin/env bash
ip=$(kubectl get svc pod2pod-server -ojsonpath='{.status.loadBalancer.ingress[0].ip}')

if [ -z "$ip" ]
then
    echo "Unable to continue, pod2pod-server has no lb ip"
    exit 1
fi

date
# -H: target ip
# -t: test type
# -l: length of test
# -j: keep additional testtats
netperf \
    -H $ip \
    -t TCP_STREAM \
    -l 60s \
    -j \
    -- \
    -P 12866 \
    -k THROUGHPUT,THROUGHPUT_UNITS,THROUGHPUT_CONFID,PROTOCOL,ELAPSED_TIME,LOCAL_SEND_CALLS,LOCAL_BYTES_PER_SEND,LOCAL_RECV_CALLS,LOCAL_BYTES_PER_RECV,REMOTE_SEND_CALLS,REMOTE_BYTES_PER_SEND,REMOTE_RECV_CALLS,REMOTE_BYTES_PER_RECV,REMOTEL_RELEASE,REMOTEL_VERSION,REMOTEL_MACHINE,COMMAND_LINE,LOCAL_TRANSPORT_RETRANS,REMOTE_TRANSPORT_RETRANS,TRANSACTION_RATE,P50_LATENCY,P90_LATENCY,RT_LATENCY