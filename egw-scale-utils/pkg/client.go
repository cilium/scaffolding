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
	Stress             bool
	StressDelay        time.Duration
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

	testStressConnectionsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "egw_scale_test_stress_connections_total",
		Help: "The number of connections either successfully opened or unexpectedly closed towards the external target",
	}, []string{"operation"})

	testStressConnectionLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "egw_scale_test_stress_connection_latency_seconds",
		Help: "The time that it takes for a new connection to be successfully opened",
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

func stressExternalTarget(
	cfg *ClientConfig,
	testHasFinished *atomic.Bool,
	logger *slog.Logger,
) {
	const (
		dialTimeout = 5 * time.Second
		readTimeout = 5 * time.Second
		maxerrs     = 3
	)

	var (
		count  uint32
		errcnt uint32
		dialer = net.Dialer{
			Timeout: dialTimeout,
			KeepAliveConfig: net.KeepAliveConfig{
				Enable: true, Idle: 1 * time.Second,
				Interval: 1 * time.Second, Count: 15,
			},
		}
		limiter = NewLogLimiter[string]()
	)

	// Initialize the metric labels
	testStressConnectionsCounter.WithLabelValues("open")
	testStressConnectionsCounter.WithLabelValues("close")

	logger.Info("Waiting before starting the connections stress test", "delay", cfg.StressDelay)
	time.Sleep(cfg.StressDelay)

	for errcnt < maxerrs {
		start := time.Now()
		conn, err := dialer.Dial("tcp4", cfg.ExternalTargetAddr)
		elapsed := time.Since(start)

		if err != nil {
			errcnt++
			logger.Warn("Failed to dial target", "err", err, "cnt", count, "errcnt", errcnt, "elapsed", elapsed)
			continue
		}

		count++

		testStressConnectionsCounter.WithLabelValues("open").Inc()
		testStressConnectionLatency.Observe(elapsed.Seconds())

		if _, can := limiter.CanLog("open"); can {
			logger.Debug("Successfully dialed target", "cnt", count, "errcnt", errcnt)
		}

		if elapsed > 100*time.Millisecond {
			logger.Warn("Dialing took more than 100ms", "cnt", count, "errcnt", errcnt, "elapsed", elapsed)
		}

		go func(conn net.Conn) {
			// We never actually close the connections; they will be GCed when
			// the client is closed.
			err := readUntilError(conn)

			if cnt, can := limiter.CanLog("close"); can {
				logger.Error("Connection unexpectedly closed", "err", err, "cnt", cnt)
			}

			testStressConnectionsCounter.WithLabelValues("close").Inc()
			conn.Close()
		}(conn)
	}

	testHasFinished.Store(true)
	logger.Info("Test completed", "cnt", count, "errcnt", errcnt)
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

	if cfg.Stress {
		stressExternalTarget(cfg, testHasFinished, logger)
	}

	return nil
}

func RunClient(cfg *ClientConfig) error {
	logger := NewLogger("client").With("external-target", cfg.ExternalTargetAddr)
	logger.Info("Starting", "stress", cfg.Stress)

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
