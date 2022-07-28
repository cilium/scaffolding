package cmd

import (
	"github.com/cilium/scaffolding/toolkit/toolkit"
	"github.com/spf13/cobra"
)

func init() {
	lazyGetCmd.AddCommand(lazyGetCraneBin)
	addDownloadFlags(lazyGetCraneBin)
}

var lazyGetCraneBin = &cobra.Command{
	Use:   "crane",
	Short: "download latest version of crane binary",
	Run: func(_ *cobra.Command, args []string) {
		err := toolkit.DownloadCraneBin(
			CmdCtx, Logger, Platform, Arch, Dest, Keep,
		)
		if err != nil {
			toolkit.ExitWithError(Logger, err)
		}
	},
}
