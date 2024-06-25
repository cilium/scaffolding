// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"path"

	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"

	cmtypes "github.com/cilium/cilium/pkg/clustermesh/types"
	"github.com/cilium/cilium/pkg/kvstore"
	"github.com/cilium/cilium/pkg/kvstore/store"
	"github.com/cilium/cilium/pkg/loadbalancer"
	serviceStore "github.com/cilium/cilium/pkg/service/store"
)

type services struct {
	syncer[*serviceStore.ClusterService]

	cluster    cmtypes.ClusterInfo
	cache      cache[*serviceStore.ClusterService]
	rnd        *random
	enableIPv6 bool
}

func newServices(log logrus.FieldLogger, cp cparams) *services {
	prefix := kvstore.StateToCachePrefix(serviceStore.ServiceStorePrefix)
	ss := cp.factory.NewSyncStore(cp.cluster.Name, cp.backend,
		path.Join(prefix, cp.cluster.Name),
		store.WSSWithSyncedKeyOverride(prefix))

	svc := &services{
		cluster:    cp.cluster,
		cache:      newCache[*serviceStore.ClusterService](),
		rnd:        cp.rnd,
		enableIPv6: cp.enableIPv6,
	}

	svc.syncer = newSyncer(log, "services", ss, svc.next)
	return svc
}

func (svc *services) next(synced bool) (obj *serviceStore.ClusterService, delete bool) {
	if synced && svc.rnd.ShouldUpdateLikely() && !svc.cache.AlmostEmpty() {
		service := svc.cache.Get(svc.rnd)
		service.Backends = svc.updated(service.Backends)
		svc.cache.Upsert(service)
		return service, false
	}

	if synced && svc.rnd.ShouldRemove() && !svc.cache.AlmostEmpty() {
		return svc.cache.Remove(svc.rnd), true
	}

	for {
		service := svc.new()
		if svc.cache.Add(service) {
			return service, false
		}
	}
}

func (svc *services) new() *serviceStore.ClusterService {
	lbls := svc.rnd.ServiceLabels()
	return &serviceStore.ClusterService{
		Cluster:         svc.cluster.Name,
		ClusterID:       svc.cluster.ID,
		Namespace:       svc.rnd.Namespace(),
		Labels:          lbls,
		Selector:        lbls,
		Name:            svc.rnd.Name(),
		Frontends:       svc.frontends(),
		Backends:        svc.backends(),
		Shared:          true,
		IncludeExternal: true,
	}
}

func (svc *services) frontends() map[string]serviceStore.PortConfiguration {
	fe := make(map[string]serviceStore.PortConfiguration)
	ports := serviceStore.PortConfiguration{
		"foo": loadbalancer.NewL4Addr(loadbalancer.TCP, 80),
		"bar": loadbalancer.NewL4Addr(loadbalancer.TCP, 90),
	}

	fe[svc.rnd.ServiceIP4().String()] = ports
	if svc.enableIPv6 {
		fe[svc.rnd.ServiceIP6().String()] = ports
	}

	return fe
}

func (svc *services) backends() map[string]serviceStore.PortConfiguration {
	n := svc.rnd.ServiceBackends()
	if svc.enableIPv6 {
		n *= 2
	}

	be := make(map[string]serviceStore.PortConfiguration, n)
	ports := serviceStore.PortConfiguration{
		"foo": loadbalancer.NewL4Addr(loadbalancer.TCP, 8080),
		"bar": loadbalancer.NewL4Addr(loadbalancer.TCP, 9090),
	}

	for len(be) < n {
		be[svc.rnd.PodIP4().String()] = ports
		if svc.enableIPv6 {
			be[svc.rnd.PodIP6().String()] = ports
		}
	}

	return be
}

func (svc *services) updated(be map[string]serviceStore.PortConfiguration) map[string]serviceStore.PortConfiguration {
	if svc.rnd.ShouldRemove() && len(be) > 0 {
		key := maps.Keys(be)[svc.rnd.Index(len(be))]
		delete(be, key)
		return be
	}

	ports := serviceStore.PortConfiguration{
		"foo": loadbalancer.NewL4Addr(loadbalancer.TCP, 8080),
		"bar": loadbalancer.NewL4Addr(loadbalancer.TCP, 9090),
	}

	be[svc.rnd.PodIP4().String()] = ports
	if svc.enableIPv6 {
		be[svc.rnd.PodIP6().String()] = ports
	}

	return be
}
