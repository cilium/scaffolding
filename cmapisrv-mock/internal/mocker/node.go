// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"net"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/cilium/cilium/pkg/cidr"
	cmtypes "github.com/cilium/cilium/pkg/clustermesh/types"
	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/kvstore"
	"github.com/cilium/cilium/pkg/kvstore/store"
	"github.com/cilium/cilium/pkg/node/addressing"
	nodeStore "github.com/cilium/cilium/pkg/node/store"
	nodeTypes "github.com/cilium/cilium/pkg/node/types"
)

type nodes struct {
	syncer[*nodeTypes.Node]

	cluster    cmtypes.ClusterInfo
	cache      cache[*nodeTypes.Node]
	enableIPv6 bool
}

func newNodes(log logrus.FieldLogger, cp cparams) *nodes {
	prefix := kvstore.StateToCachePrefix(nodeStore.NodeStorePrefix)
	ss := cp.factory.NewSyncStore(cp.cluster.Name, cp.backend,
		path.Join(prefix, cp.cluster.Name),
		store.WSSWithSyncedKeyOverride(prefix))

	ns := &nodes{
		cluster:    cp.cluster,
		cache:      newCache[*nodeTypes.Node](),
		enableIPv6: cp.enableIPv6,
	}

	ns.syncer = newSyncer(log, "nodes", ss, ns.next)
	return ns
}

func (ns *nodes) RandomHostIP() net.IP {
	no := ns.cache.Get()
	return no.GetNodeInternalIPv4()
}

func (ns *nodes) next(synced bool) (obj *nodeTypes.Node, delete bool) {
	if synced && rnd.ShouldRemove() && !ns.cache.AlmostEmpty() {
		return ns.cache.Remove(), true
	}

	for {
		node := ns.new()
		if ns.cache.Add(node) {
			return node, false
		}
	}
}

func (ns *nodes) new() *nodeTypes.Node {
	name := rnd.Name()

	no := &nodeTypes.Node{
		Name:      name,
		Cluster:   ns.cluster.Name,
		ClusterID: ns.cluster.ID,
		Labels: map[string]string{
			"kubernetes.io/hostname": name,
			"kubernetes.io/arch":     "amd64",
			"kubernetes.io/os":       "linux",
		},
		IPAddresses: []nodeTypes.Address{
			{Type: addressing.NodeInternalIP, IP: rnd.NodeIP4()},
			{Type: addressing.NodeCiliumInternalIP, IP: rnd.PodIP4()},
		},
		IPv4AllocCIDR: &cidr.CIDR{IPNet: rnd.CIDR4()},
		IPv4HealthIP:  rnd.PodIP4(),
		IPv4IngressIP: rnd.PodIP4(),
		NodeIdentity:  uint32(identity.ReservedIdentityRemoteNode),
	}

	if ns.enableIPv6 {
		no.IPAddresses = append(no.IPAddresses, nodeTypes.Address{Type: addressing.NodeInternalIP, IP: rnd.NodeIP6()})
		no.IPAddresses = append(no.IPAddresses, nodeTypes.Address{Type: addressing.NodeCiliumInternalIP, IP: rnd.PodIP6()})
		no.IPv6AllocCIDR = &cidr.CIDR{IPNet: rnd.CIDR6()}
		no.IPv6HealthIP = rnd.PodIP6()
		no.IPv6IngressIP = rnd.PodIP6()
	}

	return no
}
