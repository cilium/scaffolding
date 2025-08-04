// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"golang.org/x/time/rate"

	"github.com/cilium/cilium/pkg/kvstore/store"
	"github.com/cilium/cilium/pkg/logging/logfields"
)

type nextFn[T store.Key] func(synced bool) (obj T, delete bool)

type syncer[T store.Key] struct {
	log   *slog.Logger
	store store.SyncStore
	next  nextFn[T]
	init  chan struct{}
}

func newSyncer[T store.Key](log *slog.Logger, typ string, store store.SyncStore, next nextFn[T]) syncer[T] {
	return syncer[T]{
		log:   log.With("type", typ),
		store: store,
		next:  next,
		init:  make(chan struct{}),
	}
}

func (s syncer[T]) Run(ctx context.Context, target uint, qps rate.Limit, allSynced <-chan struct{}) {
	s.log.Info("Starting synchronization")
	do := func(obj T, delete bool) {
		if delete {
			s.log.Debug("Deleting key", "key", obj.GetKeyName())
			if err := s.store.DeleteKey(ctx, obj); err != nil {
				s.log.Error("Failed to delete key", logfields.Error, err)
				os.Exit(-1)
			}
			return
		}

		s.log.Debug("Upserting key", "key", obj.GetKeyName())
		if err := s.store.UpsertKey(ctx, obj); err != nil {
			s.log.Error("Failed to upsert key", logfields.Error, err)
			os.Exit(-1)
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		s.store.Run(ctx)
		wg.Done()
	}()

	for i := uint(0); i < target; i++ {
		do(s.next(false))
	}

	s.store.Synced(ctx, func(context.Context) {
		s.log.Info("Initial synchronization completed")
		close(s.init)
	})

	select {
	case <-ctx.Done():
		return
	case <-allSynced:
		// Wait for synchronization completion in all mocked clusters and for
		// all resources before starting the churn phase, to avoid unnecessarily
		// consuming rate limiter slots before turning ready.
	}

	rl := rate.NewLimiter(qps, 1)
	for {
		if err := rl.Wait(ctx); err != nil {
			wg.Wait()
			s.log.Info("Ending synchronization")
			return
		}

		if target != 0 && qps > 0 {
			do(s.next(true))
		}
	}
}

func (s syncer[T]) WaitForSync(ctx context.Context) error {
	select {
	case <-s.init:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
