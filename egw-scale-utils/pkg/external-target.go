// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package pkg

import (
	"log/slog"
	"net"
	"strconv"
)

type ExternalTargetConfig struct {
	AllowedCIDRString string
	ListenPort        int
	KeepOpen          bool
}

type lkey struct{ ip, op string }

var limiter = NewLogLimiter[lkey]()

func readUntilError(conn net.Conn) error {
	var (
		buffer = make([]byte, 10)
		err    error
	)

	for ; err == nil; _, err = conn.Read(buffer) {
		// Keep blocking on reading until an error occurs.
	}

	return err
}

func handleConnection(conn net.Conn, allowedCIDR *net.IPNet, keepOpen bool, logger *slog.Logger) {
	defer conn.Close()

	var remoteIP net.IP

	switch addr := conn.RemoteAddr().(type) {
	case *net.TCPAddr:
		remoteIP = addr.IP
	default:
		logger.Warn("Received non-TCP connection", "remote-addr", conn.RemoteAddr().String())

		return
	}

	if remoteIP == nil {
		logger.Warn("Unable to parse remote Addr from client", "remote-addr", conn.RemoteAddr().String())

		return
	}

	if !allowedCIDR.Contains(remoteIP) {
		logger.Debug("Received connection from IP outside allowed cidr", "remote-ip", remoteIP.String())

		return
	}

	_, err := conn.Write([]byte("pong\n"))
	if err != nil {
		logger.Error("Unexpected error while writing data back to client", "remote-ip", remoteIP.String(), "err", err)
	}

	if cnt, can := limiter.CanLog(lkey{remoteIP.String(), "open"}); can {
		logger.Info("Responded to IP in allowed cidr", "ip", remoteIP.String(), "cnt", cnt)
	}

	if keepOpen {
		// Wait until the client closes the connection before closing our side.
		err := readUntilError(conn)

		if cnt, can := limiter.CanLog(lkey{remoteIP.String(), "close"}); can {
			logger.Info("Read returned, closing connection", "ip", remoteIP.String(), "cnt", cnt, "err", err)
		}
	}
}

func RunExternalTarget(cfg *ExternalTargetConfig) error {
	if cfg.AllowedCIDRString == "" {
		return NewEmptyConfigValueError("--allowed-cidr")
	}

	_, allowedCIDR, err := net.ParseCIDR(cfg.AllowedCIDRString)
	if err != nil {
		return err
	}

	logger := NewLogger("external-target")
	logger.Info("Parsed allowed-cidr", "allowed-cidr", allowedCIDR)

	listenAddr := net.JoinHostPort(
		"0.0.0.0", strconv.FormatInt(int64(cfg.ListenPort), 10),
	)

	listener, err := net.Listen("tcp4", listenAddr)
	if err != nil {
		return err
	}

	logger.Info("Listening for new connections", "listen-addr", listenAddr, "keep-open", cfg.KeepOpen)

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("Unexpected error while accepting client connection", "err", err)

			continue
		}

		go handleConnection(conn, allowedCIDR, cfg.KeepOpen, logger)
	}
}
