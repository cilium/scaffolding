// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package cmd

import (
	"time"

	"github.com/cilium/scaffolding/egw-scale-utils/pkg"

	"github.com/spf13/cobra"
)

var (
	clientCfg = &pkg.ClientConfig{}

	clientCmd = &cobra.Command{
		Use: "client",
		Run: func(cmd *cobra.Command, args []string) {
			if err := pkg.RunClient(clientCfg); err != nil {
				panic(err)
			}
		},
	}
)

func init() {
	clientCmd.PersistentFlags().StringVar(
		&clientCfg.ExternalTargetAddr, "external-target-addr", "", "Address of external target to connect to. Needs to be of the format 'IP:Port'",
	)
	clientCmd.PersistentFlags().DurationVar(
		&clientCfg.Interval, "interval", 50*time.Millisecond, "The interval at which the client sends probes to the server.",
	)
	clientCmd.PersistentFlags().DurationVar(
		&clientCfg.TestTimeout, "test-timeout", time.Minute, "The duration the client has to connect to the external target before cancelling the test.",
	)
	clientCmd.PersistentFlags().BoolVar(
		&clientCfg.Stress, "stress", false, "Keep opening connections to the external target until repeated failures occur.",
	)
	clientCmd.PersistentFlags().DurationVar(
		&clientCfg.StressDelay, "stress-delay", 0, "Delay before starting the connections stress test, for metrics scraping purpose.",
	)

	rootCmd.AddCommand(clientCmd)
}
