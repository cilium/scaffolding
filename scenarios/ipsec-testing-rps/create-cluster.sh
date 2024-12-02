#!/usr/bin/env bash
# Create a three node equinix cluster for testing.
# Nodes are labeled with pod2pod client and server roles
# for scheduling, and each node will have its conntrack table
# size increased.
set -xeo pipefail
. ./env.sh

. ../common.sh
init

# Perform some prechecks and env setup.
if ! which terraform
then
  echo "need terraform"
  exit 1
fi

if [ ! -d $ARTIFACTS/kubespray ]
then
  git clone https://github.com/kubernetes-sigs/kubespray $ARTIFACTS/kubespray
fi

if [ ! -f $ARTIFACTS/id_rsa ]
then
  ssh-keygen -f $ARTIFACTS/id_rsa -N ""
fi

if [ ! -d $ARTIFACTS/venv ]
then
  python3 -m venv $ARTIFACTS/venv
  . $ARTIFACTS/venv/bin/activate
  pip3 install -r $ARTIFACTS/kubespray/requirements.txt
elif ! which ansible
then
  . $ARTIFACTS/venv/bin/activate
fi

if ! which ansible
then
  echo "need ansible"
  exit 1
fi

# Setup the inventory.
cd $ARTIFACTS/kubespray
mkdir -p inventory/$CLUSTER_NAME
cp -LRp contrib/terraform/equinix/sample-inventory/* inventory/$CLUSTER_NAME/
cp $SCENARIO_DIR/ports.tf contrib/terraform/equinix

# Create the equinix hosts.
cd inventory/$CLUSTER_NAME
cp $SCENARIO_DIR/terraform.py ./hosts
cp $SCENARIO_DIR/cluster.tfvars .
cp $SCENARIO_DIR/k8s-cluster.yml ./group_vars/k8s_cluster/
terraform -chdir=../../contrib/terraform/equinix init -var-file=$(pwd)/cluster.tfvars
terraform -chdir=../../contrib/terraform/equinix apply -var-file=$(pwd)/cluster.tfvars --auto-approve || true
# This normally fails the first time due to issues with attaching the vlan.
# For some reason, just rerunning fixes it (have you tried turning it on and off again?)
terraform -chdir=../../contrib/terraform/equinix apply -var-file=$(pwd)/cluster.tfvars --auto-approve

# Deploy the cluster
cd $ARTIFACTS/kubespray
export ANSIBLE_HOST_KEY_CHECKING=False
ansible_common_args="-i inventory/$CLUSTER_NAME/hosts --private-key $ARTIFACTS/id_rsa"
# Set up internal IPs
i="100"
for node in '*master-1' '*node-1' '*node-2'
do
  ip="192.169.0.${i}/24"
  ansible $ansible_common_args -m ansible.builtin.shell $node -a 'ip link set down enp1s0f1np1 && ip link set enp1s0f1np1 nomaster && if ! ip a | grep '$ip'; then ip addr add '$ip' dev enp1s0f1np1; fi && ip link set dev enp1s0f1np1 up'
  i=$((i + 1))
done
ansible-playbook $ansible_common_args ./cluster.yml
# This normally fails the first time due to being unable to restart etcd.
# For some reason, just rerunning the playbook fixes it (have you tried turning it on and off again?)
ansible-playbook $ansible_common_args ./cluster.yml
# Fix permission issues in /opt/cni/bin
# See https://github.com/cilium/cilium/issues/23838
ansible $ansible_common_args all -m ansible.builtin.shell -a 'chmod 775 -R /opt/cni/bin'
ansible $ansible_common_args all -m ansible.builtin.shell -a 'ls -la /opt/cni'

# Setup the kubeconfig
cd $SCENARIO_DIR
cp $ARTIFACTS/kubespray/inventory/$CLUSTER_NAME/artifacts/admin.conf $ARTIFACTS/admin.conf
export KUBECONFIG=$ARTIFACTS/admin.conf
ip="$(cat $ARTIFACTS/kubespray/contrib/terraform/equinix/terraform.tfstate | jq -r '.outputs.k8s_masters.value[0]')"
sed -i "s/192.169.0.100/$ip/g" $KUBECONFIG

# Setup the worker nodes.
label=server
for node in $(kubectl get nodes -l '!node-role.kubernetes.io/control-plane' -o name)
do
  # Ron doesn't like node/name format, so just isolate the node name.
  node="$(echo $node | cut -d '/' -f 2)"
  kubectl label --overwrite node $node role.scaffolding/pod2pod-$label=true
  label=client
  kubectl label --overwrite node $node role.scaffolding/monitored=true
  # CRR tests fail with drops due to limited conntrack size.
  # Using ron here verifies that we can schedule and execute pods.
  $ARTIFACTS/toolkit ron --node $node --tolerate-all -- sysctl -w net.netfilter.nf_conntrack_max=262144
done

# Install Cilium so we can install longhorn
./install-cilium.sh none

# Install CSI plugin
if ! kubectl get ns longhorn-system
then
  helm install longhorn longhorn/longhorn --namespace longhorn-system --create-namespace --version 1.7.2
fi

# Install node-exporter
kubectl label --overwrite $(kubectl get nodes -l 'node-role.kubernetes.io/control-plane' -o name) role.scaffolding/monitoring=true
kubectl create -k $KUSTOMIZE/prometheus || kubectl replace -k $KUSTOMIZE/prometheus
kubectl apply -k $KUSTOMIZE/grafana
kubectl create -f $KUSTOMIZE/grafana/dashboards/node-exporter-dashboard.yaml || kubectl replace -f $KUSTOMIZE/grafana/dashboards/node-exporter-dashboard.yaml
kubectl apply -k $KUSTOMIZE/node-exporter

wait_ready

