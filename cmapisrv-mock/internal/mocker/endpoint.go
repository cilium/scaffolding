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
	rnd     *random

	podIPGetter    func() net.IP
	nodeIPGetter   func() net.IP
	identityGetter func() identity.NumericIdentity
	encKeyGetter   func() uint8
}

func newEndpoints(
	log logrus.FieldLogger, cp cparams,
	nodes *nodes, identities *identities) *endpoints {

	prefix := kvstore.StateToCachePrefix(IPIdentitiesPath)
	ss := cp.factory.NewSyncStore(cp.cluster.Name, cp.backend,
		path.Join(prefix, cp.cluster.Name),
		store.WSSWithSyncedKeyOverride(prefix))

	eps := &endpoints{
		cluster:        cp.cluster,
		cache:          newCache[*identity.IPIdentityPair](),
		rnd:            cp.rnd,
		podIPGetter:    cp.rnd.PodIP4,
		nodeIPGetter:   nodes.RandomHostIP,
		identityGetter: identities.RandomIdentity,
		encKeyGetter:   cp.encryption.toKey,
	}

	if cp.enableIPv6 {
		eps.podIPGetter = cp.rnd.PodIP
	}

	eps.syncer = newSyncer(log, "ips", ss, eps.next)
	return eps
}

func (eps *endpoints) next(synced bool) (obj *identity.IPIdentityPair, delete bool) {
	if synced && eps.rnd.ShouldUpdateUnlikely() && !eps.cache.AlmostEmpty() {
		endpoint := eps.cache.Get(eps.rnd)
		endpoint.ID = eps.identityGetter()
		eps.cache.Upsert(endpoint)
		return endpoint, false
	}

	if synced && eps.rnd.ShouldRemove() && !eps.cache.AlmostEmpty() {
		return eps.cache.Remove(eps.rnd), true
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
		Key:          eps.encKeyGetter(),
		K8sPodName:   eps.rnd.Name(),
		K8sNamespace: eps.rnd.Namespace(),
	}
}
