# This file is used to enable hybrid unbonded mode
# networking.
resource "equinix_metal_port" "k8s_node_eth1" {
  count = var.number_of_k8s_nodes
  port_id = [for p in equinix_metal_device.k8s_node[count.index].ports: p.id if p.name == "eth1"][0]
  bonded = false
}

resource "equinix_metal_port" "k8s_master_eth1" {
  count = var.number_of_k8s_masters
  port_id = [for p in equinix_metal_device.k8s_master[count.index].ports: p.id if p.name == "eth1"][0]
  bonded = false
}

resource "equinix_metal_vlan" "cluster_vlan" {
  metro = var.metro
  project_id = var.equinix_metal_project_id
}

resource "equinix_metal_port_vlan_attachment" "k8s_node_vlan_attachement" {
  count = var.number_of_k8s_nodes
  device_id = equinix_metal_device.k8s_node[count.index].id
  port_name = "eth1"
  vlan_vnid = equinix_metal_vlan.cluster_vlan.vxlan
}


resource "equinix_metal_port_vlan_attachment" "k8s_master_vlan_attachement" {
  count = var.number_of_k8s_masters
  device_id = equinix_metal_device.k8s_master[count.index].id
  port_name = "eth1"
  vlan_vnid = equinix_metal_vlan.cluster_vlan.vxlan
}
