// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/cilium/hive/cell"
	"github.com/cilium/hive/job"

	"github.com/cilium/cilium/clustermesh-apiserver/health"
	"github.com/cilium/cilium/clustermesh-apiserver/syncstate"
	"github.com/cilium/cilium/pkg/kvstore"
	"github.com/cilium/cilium/pkg/kvstore/store"
	"github.com/cilium/cilium/pkg/logging/logfields"
)

type mocker struct {
	cfg config

	log *slog.Logger

	backend kvstore.Client
	factory store.Factory
	rnd     *random

	syncState syncstate.SyncState
}

func newMocker(in struct {
	cell.In

	Lifecycle cell.Lifecycle
	Logger    *slog.Logger
	JobGroup  job.Group

	Config    config
	Backend   kvstore.Client
	Factory   store.Factory
	Random    *random
	SyncState syncstate.SyncState
}) *mocker {
	mk := &mocker{
		cfg:       in.Config,
		log:       in.Logger,
		backend:   in.Backend,
		factory:   in.Factory,
		rnd:       in.Random,
		syncState: in.SyncState,
	}

	in.JobGroup.Add(job.OneShot("mocker", mk.Run))
	return mk
}

func (mk *mocker) Run(ctx context.Context, _ cell.Health) error {
	// The etcdinit container initializes the RBAC so that the remote user can
	// only access the information of the specific target cluster, while the
	// local one can access the data cached via KVStoreMesh. However, in this
	// scale test, the mocker leverages the KVStoreMesh API to mock multiple
	// clusters at once. Hence, let's tune the user permissions so that the
	// real KVStoreMesh container can then retrieve the mocked data.
	mk.backend.UserEnforcePresence(ctx, "remote", []string{"local", "remote"})

	cls := newClusters(mk.log, mk.cfg, mk.factory, mk.backend, mk.rnd)
	cls.Run(ctx, mk.syncState)
	return nil
}

func (mk *mocker) HealthEndpoints() []health.EndpointFunc {
	return []health.EndpointFunc{
		{
			Path: "/readyz",
			HandlerFunc: func(w http.ResponseWriter, r *http.Request) {
				statusCode := http.StatusInternalServerError
				reply := "NotReady"

				if mk.syncState.Complete() {
					statusCode = http.StatusOK
					reply = "Ready"
				}

				w.WriteHeader(statusCode)
				if _, err := w.Write([]byte(reply)); err != nil {
					mk.log.Error("Failed to respond to /readyz request", logfields.Error, err)
				}
			},
		},
	}
}
