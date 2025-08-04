// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cilium/cilium/clustermesh-apiserver/etcdinit"
	"github.com/cilium/cilium/pkg/hive"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"
	"github.com/cilium/cilium/pkg/metrics"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/scaffolding/cmapisrv-mock/internal/mocker"
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
	rootCmd := &cobra.Command{
		Use:   "mocker",
		Short: "Run ClusterMesh mocker",
		Run: func(cmd *cobra.Command, args []string) {
			if err := h.Run(logging.DefaultSlogLogger); err != nil {
				logging.DefaultSlogLogger.Error(err.Error())
				os.Exit(-1)
			}
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			metrics.Namespace = "mocker"
			option.Config.SetupLogging(h.Viper(), "mocker")

			logger := logging.DefaultSlogLogger.With(logfields.LogSubsys, "mocker")
			option.LogRegisteredSlogOptions(h.Viper(), logger)
		},
	}

	h.RegisterFlags(rootCmd.Flags())
	rootCmd.AddCommand(h.Command())
	return rootCmd
}
