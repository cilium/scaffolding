package cmd

import (
	"runtime"

	"github.com/spf13/cobra"
)

var (
	Platform string
	Arch     string
	Dest     string
	Keep     bool
)

func addDownloadFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&Platform, "platform", "p", runtime.GOOS, "explicitly set platform")
	cmd.PersistentFlags().StringVarP(&Arch, "arch", "a", runtime.GOARCH, "explicitly set architecture")
	cmd.PersistentFlags().StringVarP(&Dest, "dest", "d", ".", "download directory")
	cmd.PersistentFlags().BoolVar(&Keep, "keep", false, "keep all download assets, such as checksums and tarballs")
}

func init() {
	rootCmd.AddCommand(lazyGetCmd)
}

var lazyGetCmd = &cobra.Command{
	Use:   "lazyget",
	Short: "get a thing so you don't have to",
	Long:  "",
}
