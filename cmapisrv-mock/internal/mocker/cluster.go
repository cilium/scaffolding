// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/cilium/cilium/clustermesh-apiserver/syncstate"
	cmtypes "github.com/cilium/cilium/pkg/clustermesh/types"
	cmutils "github.com/cilium/cilium/pkg/clustermesh/utils"
	"github.com/cilium/cilium/pkg/defaults"
	"github.com/cilium/cilium/pkg/kvstore"
	"github.com/cilium/cilium/pkg/kvstore/store"
)

type clusters struct {
	cfg config
	cls []cluster
}

func newClusters(log logrus.FieldLogger, cfg config, factory store.Factory, backend kvstore.BackendOperations) clusters {
	cls := clusters{cfg: cfg}

	for i := uint(0); i < cfg.Clusters; i++ {
		id := cfg.FirstClusterID + i
		name := fmt.Sprintf("cluster-%03d", id)
		cls.cls = append(cls.cls, newCluster(
			log.WithField("cluster", name),
			cmtypes.ClusterInfo{ID: uint32(id), Name: name},
			factory, backend, cfg.EnableIPv6))
	}

	return cls
}

func (cls clusters) Run(ctx context.Context, ss syncstate.SyncState) {
	var wg sync.WaitGroup
	wg.Add(len(cls.cls))

	for _, cl := range cls.cls {
		synced := ss.WaitForResource()
		go func(cl cluster) {
			cl.Run(ctx, cls.cfg, synced)
			wg.Done()
		}(cl)
	}

	ss.Stop()
	wg.Wait()
}

type cluster struct {
	log     logrus.FieldLogger
	backend kvstore.BackendOperations

	cinfo      cmtypes.ClusterInfo
	nodes      *nodes
	identities *identities
	endpoints  *endpoints
	services   *services
}

func newCluster(log logrus.FieldLogger, cinfo cmtypes.ClusterInfo, factory store.Factory,
	backend kvstore.BackendOperations, enableIPv6 bool) cluster {

	log.Info("Creating cluster")
	cl := cluster{
		log:     log,
		backend: backend,
		cinfo:   cinfo,

		nodes:      newNodes(log, cinfo, factory, backend, enableIPv6),
		identities: newIdentities(log, cinfo, factory, backend),
		services:   newServices(log, cinfo, factory, backend, enableIPv6),
	}

	cl.endpoints = newEndpoints(log, cinfo, factory, backend, enableIPv6, cl.nodes, cl.identities)
	return cl
}

func (cl *cluster) Run(ctx context.Context, cfg config, synced func(context.Context)) {
	var wg sync.WaitGroup

	cl.log.Info("Starting cluster")
	cl.writeClusterConfig(ctx)

	wg.Add(1)
	go func() {
		cl.nodes.Run(ctx, cfg.Nodes, rate.Limit(cfg.NodesQPS))
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		cl.identities.Run(ctx, cfg.Identities, rate.Limit(cfg.IdentitiesQPS))
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		cl.services.Run(ctx, cfg.Services, rate.Limit(cfg.ServicesQPS))
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		if cl.nodes.WaitForSync(ctx) != nil || cl.identities.WaitForSync(ctx) != nil {
			return
		}

		cl.endpoints.Run(ctx, cfg.Endpoints, rate.Limit(cfg.EndpointsQPS))
	}()

	if cl.nodes.WaitForSync(ctx) != nil || cl.identities.WaitForSync(ctx) != nil ||
		cl.endpoints.WaitForSync(ctx) != nil || cl.services.WaitForSync(ctx) != nil {
		return
	}

	synced(ctx)

	<-ctx.Done()
	wg.Wait()
}

func (cl *cluster) writeClusterConfig(ctx context.Context) {
	config := cmtypes.CiliumClusterConfig{
		ID: cl.cinfo.ID,
		Capabilities: cmtypes.CiliumClusterConfigCapabilities{
			SyncedCanaries:       true,
			MaxConnectedClusters: defaults.MaxConnectedClusters,
			// Use the KVStoreMesh API to allow simulating multiple clusters
			// using a single etcd instance.
			Cached: true,
		},
	}

	if err := cmutils.SetClusterConfig(ctx, cl.cinfo.Name, &config, cl.backend); err != nil {
		cl.log.WithError(err).Fatal("Failed to write ClusterConfig")
	}
	cl.log.Info("Written ClusterConfig")
}