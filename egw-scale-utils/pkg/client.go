// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package pkg

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type ClientConfig struct {
	ExternalTargetAddr string
	Interval           time.Duration
	TestTimeout        time.Duration
}

var (
	leakedRequestsCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "egw_scale_test_leaked_requests_total",
		Help: "The total number of leaked requests a client made when trying to access the external target",
	})

	masqueradeDelayCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "egw_scale_test_masquerade_delay_seconds_total",
		Help: "The number of seconds between a client pod starting and hitting the external target",
	})

	testFailureCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "egw_scale_test_failed_tests_total",
		Help: "Incremented when a client Pod is unable to connect to the external target after a preconfigured timeout",
	})
)

func getHttpReadinessProbeHandler(testHasFinished *atomic.Bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if testHasFinished.Load() {
			w.WriteHeader(200)
		} else {
			w.WriteHeader(500)
		}
	}
}

func connectToExternalTarget(
	cfg *ClientConfig,
	testHasFinished *atomic.Bool,
	logger *slog.Logger,
) error {
	defer func() {
		testHasFinished.Store(true)
	}()

	var endTime time.Time
	startTime := time.Now()
	nextAt := startTime

	timeout := time.After(cfg.TestTimeout)
	for {
		select {
		case <-time.After(time.Until(nextAt)):
		case <-timeout:
			logger.Error("Hit timeout, abandoning test", "timeout", cfg.TestTimeout.String())
			testFailureCounter.Inc()

			return nil
		}

		nextAt = nextAt.Add(cfg.Interval)

		conn, err := (&net.Dialer{
			Deadline: nextAt,
		}).Dial("tcp4", cfg.ExternalTargetAddr)
		if err != nil {
			logger.Warn("Failed to dial target", "err", err)

			leakedRequestsCounter.Add(1)
			continue
		}

		endTime = time.Now()

		if err = conn.SetDeadline(nextAt); err != nil {
			logger.Warn("Failed to set deadline on connection", "err", err)
			conn.Close()

			leakedRequestsCounter.Add(1)
			continue
		}

		reply, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			logger.Warn("Failed reading from connection", "err", err)
			conn.Close()

			leakedRequestsCounter.Add(1)
			continue
		}

		if reply != "pong\n" {
			logger.Debug("Received incorrect reply", "reply", reply)
			conn.Close()

			leakedRequestsCounter.Add(1)
			continue
		}

		logger.Info("Successfully connected to external target")
		conn.Close()

		break
	}

	delay := endTime.Sub(startTime)

	masqueradeDelayCounter.Add(delay.Seconds())

	return nil
}

func RunClient(cfg *ClientConfig) error {
	logger := NewLogger("client").With("external-target", cfg.ExternalTargetAddr)
	logger.Info("Starting")

	testHasFinished := &atomic.Bool{}
	testHasFinished.Store(false)

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/readyz", getHttpReadinessProbeHandler(testHasFinished))

	errors := make(chan error)

	go func() {
		if err := connectToExternalTarget(cfg, testHasFinished, logger); err != nil {
			errors <- err
		}
	}()

	go func() {
		errors <- http.ListenAndServe("0.0.0.0:2112", nil)
	}()

	return <-errors
}
