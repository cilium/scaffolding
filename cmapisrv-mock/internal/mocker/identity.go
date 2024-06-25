// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"path"
	"strconv"

	"github.com/sirupsen/logrus"

	cmtypes "github.com/cilium/cilium/pkg/clustermesh/types"
	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/kvstore"
	"github.com/cilium/cilium/pkg/kvstore/store"
)

// github.com/cilium/cilium/pkg/identity/cache.IdentitiesPath
var IdentitiesPath = path.Join(kvstore.BaseKeyPrefix, "state", "identities", "v1")

type identities struct {
	syncer[*store.KVPair]

	cluster cmtypes.ClusterInfo
	cache   cache[*store.KVPair]
	encoder func([]byte) string
}

func newIdentities(log logrus.FieldLogger, cp cparams) *identities {
	prefix := kvstore.StateToCachePrefix(IdentitiesPath)
	ss := cp.factory.NewSyncStore(cp.cluster.Name, cp.backend,
		path.Join(prefix, cp.cluster.Name, "id"),
		store.WSSWithSyncedKeyOverride(prefix))

	ids := &identities{
		cluster: cp.cluster,
		cache:   newCache[*store.KVPair](),
		encoder: cp.backend.Encode,
	}

	ids.syncer = newSyncer(log, "identities", ss, ids.next)
	return ids
}

func (ids *identities) RandomIdentity() identity.NumericIdentity {
	id := ids.cache.Get()
	parsed, _ := strconv.ParseUint(id.Key, 10, 32)
	return identity.NumericIdentity(parsed)
}

func (ids *identities) next(synced bool) (obj *store.KVPair, delete bool) {
	if synced && rnd.ShouldRemove() && !ids.cache.AlmostEmpty() {
		return ids.cache.Remove(), true
	}

	for {
		identity := ids.new(identity.InvalidIdentity)
		if ids.cache.Add(identity) {
			return identity, false
		}
	}
}

func (ids *identities) new(id identity.NumericIdentity) *store.KVPair {
	if id == identity.InvalidIdentity {
		id = rnd.Identity(ids.cluster.ID)
	}

	var lbls []byte
	for _, lb := range rnd.IdentityLabels(ids.cluster.Name).Sort() {
		lbls = append(lbls, lb.FormatForKVStore()...)
	}

	return store.NewKVPair(strconv.FormatUint(uint64(id), 10), ids.encoder(lbls))
}
