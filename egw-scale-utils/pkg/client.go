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
	externalTargetAddr string,
	timeout time.Duration,
	testHasFinished *atomic.Bool,
	logger *slog.Logger,
) error {
	defer func() {
		testHasFinished.Store(true)
	}()

	d := net.Dialer{
		Timeout: time.Millisecond * 50,
	}

	var err error
	var conn net.Conn
	var endTime time.Time

	needIterationDelay := false
	startTime := time.Now()
	timeoutTimer := time.NewTimer(timeout)

	defer timeoutTimer.Stop()

	for {
		select {
		case <-timeoutTimer.C:
			logger.Error("Hit timeout, abandoning test", "timeout", timeout.String())
			testFailureCounter.Inc()

			return nil
		default:
		}

		if needIterationDelay {
			time.Sleep(50 * time.Millisecond)
		}

		conn, err = d.Dial("tcp4", externalTargetAddr)
		endTime = time.Now()

		if err != nil {
			return err
		}

		reply, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			conn.Close()

			return err
		}

		if reply != "pong\n" {
			logger.Debug("Received incorrect reply", "reply", reply)
			conn.Close()

			leakedRequestsCounter.Add(1)
			needIterationDelay = true

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

	testHasFinished := &atomic.Bool{}
	testHasFinished.Store(false)

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/readyz", getHttpReadinessProbeHandler(testHasFinished))

	rrors := make(chan error)

	go func() {
		if err := connectToExternalTarget(cfg.ExternalTargetAddr, cfg.TestTimeout, testHasFinished, logger); err != nil {
			rrors <- err
		}
	}()

	go func() {
		rrors <- http.ListenAndServe("0.0.0.0:2112", nil)
	}()

	return <-rrors
}
