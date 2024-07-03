// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package mocker

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/cilium/cilium/pkg/kvstore/store"
)

type nextFn[T store.Key] func(synced bool) (obj T, delete bool)

type syncer[T store.Key] struct {
	log   logrus.FieldLogger
	store store.SyncStore
	next  nextFn[T]
	init  chan struct{}
}

func newSyncer[T store.Key](log logrus.FieldLogger, typ string, store store.SyncStore, next nextFn[T]) syncer[T] {
	return syncer[T]{
		log:   log.WithFields(logrus.Fields{"type": typ}),
		store: store,
		next:  next,
		init:  make(chan struct{}),
	}
}

func (s syncer[T]) Run(ctx context.Context, target uint, qps rate.Limit, allSynced <-chan struct{}) {
	s.log.Info("Starting synchronization")
	do := func(obj T, delete bool) {
		if delete {
			s.log.WithField("key", obj.GetKeyName()).Debug("Deleting key")
			if err := s.store.DeleteKey(ctx, obj); err != nil {
				s.log.WithError(err).Fatal("Failed to delete key")
			}
			return
		}

		s.log.WithField("key", obj.GetKeyName()).Debug("Upserting key")
		if err := s.store.UpsertKey(ctx, obj); err != nil {
			s.log.WithError(err).Fatal("Failed to upsert key")
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
