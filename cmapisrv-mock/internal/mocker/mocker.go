// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"context"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/cilium/cilium/clustermesh-apiserver/health"
	"github.com/cilium/cilium/clustermesh-apiserver/syncstate"
	"github.com/cilium/cilium/pkg/hive/cell"
	"github.com/cilium/cilium/pkg/hive/job"
	"github.com/cilium/cilium/pkg/kvstore"
	"github.com/cilium/cilium/pkg/kvstore/store"
	"github.com/cilium/cilium/pkg/promise"
)

type mocker struct {
	cfg config

	log logrus.FieldLogger

	backend promise.Promise[kvstore.BackendOperations]
	factory store.Factory

	syncState syncstate.SyncState
}

func newMocker(in struct {
	cell.In

	Lifecycle   cell.Lifecycle
	Logger      logrus.FieldLogger
	JobRegistry job.Registry
	Scope       cell.Scope

	Config    config
	Backend   promise.Promise[kvstore.BackendOperations]
	Factory   store.Factory
	SyncState syncstate.SyncState
}) *mocker {
	mk := &mocker{
		cfg:       in.Config,
		log:       in.Logger,
		backend:   in.Backend,
		factory:   in.Factory,
		syncState: in.SyncState,
	}

	group := in.JobRegistry.NewGroup(
		in.Scope,
		job.WithLogger(in.Logger),
	)

	group.Add(job.OneShot("mocker", mk.Run))

	in.Lifecycle.Append(group)
	return mk
}

func (mk *mocker) Run(ctx context.Context, _ cell.HealthReporter) error {
	backend, err := mk.backend.Await(ctx)
	if err != nil {
		return err
	}

	cls := newClusters(mk.log, mk.cfg, mk.factory, backend)
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
					mk.log.WithError(err).Error("Failed to respond to /readyz request")
				}
			},
		},
	}
}
