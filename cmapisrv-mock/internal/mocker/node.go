// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"net"

	"github.com/sirupsen/logrus"

	"github.com/cilium/cilium/pkg/cidr"
	cmtypes "github.com/cilium/cilium/pkg/clustermesh/types"
	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/kvstore"
	"github.com/cilium/cilium/pkg/node/addressing"
	nodeStore "github.com/cilium/cilium/pkg/node/store"
	nodeTypes "github.com/cilium/cilium/pkg/node/types"
)

type nodes struct {
	syncer[*nodeTypes.Node]

	cluster     cmtypes.ClusterInfo
	cache       cache[*nodeTypes.Node]
	rnd         *random
	enableIPv6  bool
	encryption  encryptionMode
	annotations map[string]string
}

func newNodes(log logrus.FieldLogger, cp cparams) *nodes {
	prefix := kvstore.StateToCachePrefix(nodeStore.NodeStorePrefix)
	ss := cp.factory.NewSyncStore(cp.cluster.Name, cp.backend, prefix)

	ns := &nodes{
		cluster:     cp.cluster,
		cache:       newCache[*nodeTypes.Node](),
		rnd:         cp.rnd,
		enableIPv6:  cp.enableIPv6,
		encryption:  cp.encryption,
		annotations: cp.nodeAnnotations,
	}

	ns.syncer = newSyncer(log, "nodes", ss, ns.next)
	return ns
}

func (ns *nodes) RandomHostIP() net.IP {
	no := ns.cache.Get(ns.rnd)
	return no.GetNodeInternalIPv4()
}

func (ns *nodes) next(synced bool) (obj *nodeTypes.Node, delete bool) {
	if synced && ns.rnd.ShouldRemove() && !ns.cache.AlmostEmpty() {
		return ns.cache.Remove(ns.rnd), true
	}

	for {
		node := ns.new()
		if ns.cache.Add(node) {
			return node, false
		}
	}
}

func (ns *nodes) new() *nodeTypes.Node {
	name := ns.rnd.Name()

	no := &nodeTypes.Node{
		Name:      name,
		Cluster:   ns.cluster.Name,
		ClusterID: ns.cluster.ID,
		Labels: map[string]string{
			"kubernetes.io/hostname": name,
			"kubernetes.io/arch":     "amd64",
			"kubernetes.io/os":       "linux",
		},
		Annotations: ns.annotations,
		IPAddresses: []nodeTypes.Address{
			{Type: addressing.NodeInternalIP, IP: ns.rnd.NodeIP4()},
			{Type: addressing.NodeCiliumInternalIP, IP: ns.rnd.PodIP4()},
		},
		IPv4AllocCIDR: &cidr.CIDR{IPNet: ns.rnd.CIDR4()},
		IPv4HealthIP:  ns.rnd.PodIP4(),
		IPv4IngressIP: ns.rnd.PodIP4(),
		NodeIdentity:  uint32(identity.ReservedIdentityRemoteNode),
		EncryptionKey: ns.encryption.toKey(),
	}

	if ns.enableIPv6 {
		no.IPAddresses = append(no.IPAddresses, nodeTypes.Address{Type: addressing.NodeInternalIP, IP: ns.rnd.NodeIP6()})
		no.IPAddresses = append(no.IPAddresses, nodeTypes.Address{Type: addressing.NodeCiliumInternalIP, IP: ns.rnd.PodIP6()})
		no.IPv6AllocCIDR = &cidr.CIDR{IPNet: ns.rnd.CIDR6()}
		no.IPv6HealthIP = ns.rnd.PodIP6()
		no.IPv6IngressIP = ns.rnd.PodIP6()
	}

	if ns.encryption == encryptionModeWireGuard {
		key, err := ns.rnd.WireGuardPublicKey()
		if err != nil {
			ns.log.WithError(err).Fatal("Failed to generate WireGuard key")
		}

		no.WireguardPubKey = key
	}

	return no
}
