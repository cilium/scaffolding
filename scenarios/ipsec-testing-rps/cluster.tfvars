# your Kubernetes cluster name here
cluster_name = # "changeme"

# Your Equinix Metal project ID. See https://metal.equinix.com/developers/docs/accounts/
equinix_metal_project_id = # "changeme"

# The public SSH key to be uploaded into authorized_keys in bare metal Equinix Metal nodes provisioned
# leave this value blank if the public key is already setup in the Equinix Metal project
# Terraform will complain if the public key is setup in Equinix Metal
public_key_path = "../../../../id_rsa.pub"

# Equinix interconnected bare metal across our global metros.
metro = "da"

# operating_system
operating_system = "ubuntu_24_04"

# masters
number_of_k8s_masters = 1

number_of_k8s_masters_no_etcd = 0

plan_k8s_masters = "m3.small.x86"

# nodes
number_of_k8s_nodes = 2

plan_k8s_nodes = "m3.small.x86"
