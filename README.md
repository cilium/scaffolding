# scaffolding
Ansible Automation to bring up a Performance SUT

## Setup
Ansible is a bit weird with `--tags` it asumes `all` if no tag is passed. 

We will be using `--tags` for each cloud provider. Be sure to pass which provider you are interested in.

## Providers
- GKE (WIP)

### GKE
#### Setup
Login to the Console, choose the project you want to build clusters. Create a service account (under IAM), create a key (json), save it locally here as `sa.json`

#### Create cluster
`ansible-playbook platform.yml --tags gke -e "create=true" -e "num_nodes=2"`
This will create a 2 node cluster in GKE.

#### Cleanup
*Note* Currently, this will cleanup all clusters, use at your own risk.
`ansible-playbook platform.yml --tags gke -e "destroy=true"`
