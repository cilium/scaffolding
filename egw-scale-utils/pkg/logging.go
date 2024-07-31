// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package pkg

import (
	"log/slog"
	"os"
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
