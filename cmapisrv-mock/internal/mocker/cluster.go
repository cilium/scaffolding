// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"golang.org/x/time/rate"

	"github.com/cilium/cilium/clustermesh-apiserver/syncstate"
	cmtypes "github.com/cilium/cilium/pkg/clustermesh/types"
	cmutils "github.com/cilium/cilium/pkg/clustermesh/utils"
	"github.com/cilium/cilium/pkg/defaults"
	"github.com/cilium/cilium/pkg/kvstore"
	"github.com/cilium/cilium/pkg/kvstore/store"
	"github.com/cilium/cilium/pkg/logging/logfields"
)

type clusters struct {
	cfg config
	cls []cluster
}

func newClusters(log *slog.Logger, cfg config, factory store.Factory, backend kvstore.BackendOperations, rnd *random) clusters {
	cls := clusters{cfg: cfg}

	for i := uint(0); i < cfg.Clusters; i++ {
		id := cfg.FirstClusterID + i
		name := fmt.Sprintf("cluster-%03d", id)
		cls.cls = append(cls.cls, newCluster(
			log.With("cluster", name),
			cparams{
				cluster:         cmtypes.ClusterInfo{ID: uint32(id), Name: name},
				factory:         factory,
				backend:         backend,
				rnd:             rnd,
				enableIPv6:      cfg.EnableIPv6,
				encryption:      cfg.Encryption,
				nodeAnnotations: cfg.NodeAnnotations,
			}))
	}

	return cls
}

func (cls clusters) Run(ctx context.Context, ss syncstate.SyncState) {
	var wg sync.WaitGroup
	wg.Add(len(cls.cls))

	for _, cl := range cls.cls {
		synced := ss.WaitForResource()
		go func(cl cluster) {
			cl.Run(ctx, cls.cfg, synced, ss.WaitChannel())
			wg.Done()
		}(cl)
	}

	ss.Stop()
	wg.Wait()
}

type cluster struct {
	log     *slog.Logger
	backend kvstore.BackendOperations

	cinfo      cmtypes.ClusterInfo
	nodes      *nodes
	identities *identities
	endpoints  *endpoints
	services   *services
}

type cparams struct {
	cluster         cmtypes.ClusterInfo
	factory         store.Factory
	backend         kvstore.BackendOperations
	rnd             *random
	enableIPv6      bool
	encryption      encryptionMode
	nodeAnnotations map[string]string
}

func newCluster(log *slog.Logger, cp cparams) cluster {
	log.Info("Creating cluster")
	cl := cluster{
		log:     log,
		backend: cp.backend,
		cinfo:   cp.cluster,

		nodes:      newNodes(log, cp),
		identities: newIdentities(log, cp),
		services:   newServices(log, cp),
	}

	cl.endpoints = newEndpoints(log, cp, cl.nodes, cl.identities)
	return cl
}

func (cl *cluster) Run(ctx context.Context, cfg config, synced func(context.Context), allSynced <-chan struct{}) {
	var wg sync.WaitGroup

	cl.log.Info("Starting cluster")
	cl.writeClusterConfig(ctx)

	wg.Add(1)
	go func() {
		cl.nodes.Run(ctx, cfg.Nodes, rate.Limit(cfg.NodesQPS), allSynced)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		cl.identities.Run(ctx, cfg.Identities, rate.Limit(cfg.IdentitiesQPS), allSynced)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		cl.services.Run(ctx, cfg.Services, rate.Limit(cfg.ServicesQPS), allSynced)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		if cl.nodes.WaitForSync(ctx) != nil || cl.identities.WaitForSync(ctx) != nil {
			return
		}

		cl.endpoints.Run(ctx, cfg.Endpoints, rate.Limit(cfg.EndpointsQPS), allSynced)
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

	if err := cmutils.SetClusterConfig(ctx, cl.cinfo.Name, config, cl.backend); err != nil {
		cl.log.Error("Failed to write ClusterConfig", logfields.Error, err)
		os.Exit(-1)
	}
	cl.log.Info("Written ClusterConfig")
}
