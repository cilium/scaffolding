#!/usr/bin/env bash
# ---
# add_grafana_dashboard.sh [-d] [-p] name dashboard-id
# ---
# takes a dashboard id from grafana.com, downloads its json, and
# adds it into the configmap with the following labels:
# - app.kubernetes.io/name: grafana-dashboards
# - app.kubernetes.io/managed-by: scaffolding
# first argument is reference name of dashboard, second argument is id
# optional argument '-p' can be added to just create a patch file rather
# than patching the grafana configmap
set -eo pipefail

if [ "${1}" == '-d' ]
then
    set -x
    shift 1
fi

patch_only="no"
if [ "${1}" == '-p' ]
then
    shift 1
    patch_only="yes"
fi

fn=$1
fn_no_ext="${1%%.*}"
fn_patch="${fn_no_ext}.yaml"
fn_json="${fn_no_ext}.json"
id=$2

curl -L https://grafana.com/api/dashboards/$id/revisions/1/download \
    --output $fn_json

cat <<EOF > $fn_patch
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-dashboards
data:
  $fn_json: |-
EOF
cat $fn_json | sed 's/^/    /' >> $fn_patch

if [ "${patch_only}" == "yes" ]
then
    exit 0
fi

cm=$(kubectl get configmap -A \
    -l app.kubernetes.io/name=grafana-dashboards \
    -l app.kubernetes.io/managed-by=scaffolding \
    -o jsonpath='{.items[0].metadata.name},{.items[0].metadata.namespace}')
name=$(echo "${cm}" | cut -d',' -f 1)
ns=$(echo "${cm}" | cut -d',' -f 2)

kubectl patch cm $name -n $ns --patch-file $fn_patch
