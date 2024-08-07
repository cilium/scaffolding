// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/cilium/cilium/clustermesh-apiserver/etcdinit"
	"github.com/cilium/cilium/pkg/hive"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"
	"github.com/cilium/cilium/pkg/metrics"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/scaffolding/cmapisrv-mock/internal/mocker"
)

var (
	log = logging.DefaultLogger.WithField(logfields.LogSubsys, "mocker")
)

func main() {
	cmd := &cobra.Command{
		Use:   "cmapisrv-mock",
		Short: "Run the ClusterMesh apiserver mock",
	}

	cmd.AddCommand(
		etcdinit.NewCmd(),
		newMockerCmd(hive.New(mocker.Cell)),
	)

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func newMockerCmd(h *hive.Hive) *cobra.Command {
	var debug bool

	rootCmd := &cobra.Command{
		Use:   "mocker",
		Short: "Run ClusterMesh mocker",
		Run: func(cmd *cobra.Command, args []string) {
			if err := h.Run(slog.Default()); err != nil {
				log.Fatal(err)
			}
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			metrics.Namespace = "mocker"

			if debug {
				log.Logger.SetLevel(logrus.DebugLevel)
			}

			option.LogRegisteredOptions(h.Viper(), log)
		},
	}

	h.RegisterFlags(rootCmd.Flags())
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debugging logs")
	rootCmd.AddCommand(h.Command())
	return rootCmd
}
