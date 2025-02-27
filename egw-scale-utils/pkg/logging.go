// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package pkg

import (
	"log/slog"
	"os"
	"sync"
)

func NewLogger(component string) *slog.Logger {
	handler := slog.NewTextHandler(
		os.Stderr,
		&slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		},
	)

	return slog.New(handler).With("component", component)
}

type LogLimiter[K comparable] struct {
	mu  sync.Mutex
	cnt map[K]uint64
}

func NewLogLimiter[K comparable]() *LogLimiter[K] {
	return &LogLimiter[K]{cnt: make(map[K]uint64)}
}

func (ll *LogLimiter[K]) CanLog(key K) (cnt uint64, can bool) {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	cnt = ll.cnt[key] + 1
	ll.cnt[key] = cnt

	return cnt, cnt <= 3 || cnt == 10 || cnt == 100 || cnt%1000 == 0
}
