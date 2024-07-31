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
}

func handleConnection(conn net.Conn, allowedCIDR *net.IPNet, logger *slog.Logger) {
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

	logger.Info("Responded to IP in allowed cidr", "ip", remoteIP.String())
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

	logger.Info("Listening for new connections", "listen-addr", listenAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("Unexpected error while accepting client connection", "err", err)

			continue
		}

		go handleConnection(conn, allowedCIDR, logger)
	}
}
