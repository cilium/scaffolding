// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package cmd

import (
	"github.com/cilium/scaffolding/egw-scale-utils/pkg"

	"github.com/spf13/cobra"
)

var (
	externalTargetCfg = &pkg.ExternalTargetConfig{}

	externalTargetCmd = &cobra.Command{
		Use: "external-target",
		Run: func(cmd *cobra.Command, args []string) {
			if err := pkg.RunExternalTarget(externalTargetCfg); err != nil {
				panic(err)
			}
		},
	}
)

func init() {
	externalTargetCmd.PersistentFlags().StringVar(
		&externalTargetCfg.AllowedCIDRString, "allowed-cidr", "", "Only respond to clients from the given CIDR",
	)
	externalTargetCmd.PersistentFlags().IntVar(
		&externalTargetCfg.ListenPort, "listen-port", 1337, "Port to listen for incoming connections on",
	)

	externalTargetCmd.PersistentFlags().BoolVar(
		&externalTargetCfg.KeepOpen, "keep-open", false, "Keep incoming connections open until the client closes them",
	)

	rootCmd.AddCommand(externalTargetCmd)
}
