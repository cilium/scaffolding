// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"net"
	"path"

	"github.com/sirupsen/logrus"

	cmtypes "github.com/cilium/cilium/pkg/clustermesh/types"
	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/kvstore"
	"github.com/cilium/cilium/pkg/kvstore/store"
)

// github.com/cilium/cilium/pkg/ipcache.IPIdentitiesPath
var IPIdentitiesPath = path.Join(kvstore.BaseKeyPrefix, "state", "ip", "v1")

type endpoints struct {
	syncer[*identity.IPIdentityPair]

	cluster cmtypes.ClusterInfo
	cache   cache[*identity.IPIdentityPair]

	podIPGetter    func() net.IP
	nodeIPGetter   func() net.IP
	identityGetter func() identity.NumericIdentity
}

func newEndpoints(
	log logrus.FieldLogger, cluster cmtypes.ClusterInfo,
	factory store.Factory, backend store.SyncStoreBackend,
	enableIPv6 bool, nodes *nodes, identities *identities) *endpoints {

	prefix := kvstore.StateToCachePrefix(IPIdentitiesPath)
	ss := factory.NewSyncStore(cluster.Name, backend,
		path.Join(prefix, cluster.Name),
		store.WSSWithSyncedKeyOverride(prefix))

	eps := &endpoints{
		cluster:        cluster,
		cache:          newCache[*identity.IPIdentityPair](),
		podIPGetter:    rnd.PodIP4,
		nodeIPGetter:   nodes.RandomHostIP,
		identityGetter: identities.RandomIdentity,
	}

	if enableIPv6 {
		eps.podIPGetter = rnd.PodIP
	}

	eps.syncer = newSyncer(log, "ips", ss, eps.next)
	return eps
}

func (eps *endpoints) next(synced bool) (obj *identity.IPIdentityPair, delete bool) {
	if synced && rnd.ShouldUpdateUnlikely() && !eps.cache.AlmostEmpty() {
		endpoint := eps.cache.Get()
		endpoint.ID = eps.identityGetter()
		eps.cache.Upsert(endpoint)
		return endpoint, false
	}

	if synced && rnd.ShouldRemove() && !eps.cache.AlmostEmpty() {
		return eps.cache.Remove(), true
	}

	for {
		endpoint := eps.new()
		if eps.cache.Add(endpoint) {
			return endpoint, false
		}
	}
}

func (eps *endpoints) new() *identity.IPIdentityPair {
	return &identity.IPIdentityPair{
		IP:           eps.podIPGetter(),
		HostIP:       eps.nodeIPGetter(),
		ID:           eps.identityGetter(),
		K8sPodName:   rnd.Name(),
		K8sNamespace: rnd.Namespace(),
	}
}
