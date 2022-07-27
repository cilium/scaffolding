package cmd

import (
	"runtime"

	"github.com/spf13/cobra"
)

var (
	Platform string
	Arch     string
	Dest     string
)

func addDownloadFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&Platform, "platform", "p", runtime.GOOS, "explicitly set platform")
	cmd.PersistentFlags().StringVarP(&Arch, "arch", "a", runtime.GOARCH, "explicitly set architecture")
	cmd.PersistentFlags().StringVarP(&Dest, "dest", "d", ".", "download directory")
}

func init() {
	rootCmd.AddCommand(lazyGetCmd)
}

var lazyGetCmd = &cobra.Command{
	Use:   "lazyget",
	Short: "get a thing so you don't have to",
	Long:  "",
}
