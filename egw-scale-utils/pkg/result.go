// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package pkg

import (
	"log/slog"
)

type Result struct {
	ClientID          string  `json:"client-id"`
	MasqueradeDelay   float64 `json:"masquerade-delay"`
	NumFailedRequests int     `json:"num-failed-requests"`
}

func (r Result) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("client-id", r.ClientID),
		slog.Float64("masquerade-delay", r.MasqueradeDelay),
		slog.Int("num-failed-requests", r.NumFailedRequests),
	)
}
