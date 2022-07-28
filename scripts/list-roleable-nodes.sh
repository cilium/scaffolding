#!/usr/bin/env bash
# ---
# list-schedulable-unlabeled-nodes.sh
# list all nodes that meet the following two conditions:
# 1. do not have a taint with a NoSchedule effect
# 2. do not have the scaffolding/role label
# nodes will be printed to stdout separated by newlines
# based on https://stackoverflow.com/questions/41348531/how-to-identify-schedulable-nodes-in-kubernetes
VERBOSE=""
if [ "${1}" == '-d' ]
then
    set -x
    VERBOSE="--verbose"
    shift 1
fi

GO_TEMPLATE='{{/* scehdulable.gotmpl */}}
{{- range .items }}
  {{- if (index .metadata.labels "scaffolding/role") }}
    {{ continue }}
  {{- end }}
  {{- $taints:="" }}
  {{- range .spec.taints }}
    {{- if eq .effect "NoSchedule" }}
      {{- $taints = print $taints .key "," }}
    {{- end }}
  {{- end }}
  {{- if not $taints }}
    {{- .metadata.name}}{{ "\n" }}
  {{- end }}
{{- end }}'

kubectl get nodes -o go-template-file=<(echo $GO_TEMPLATE) | head -n -1
