package cmd

import (
	"github.com/spf13/cobra"
)

var (
	Platform string
	Arch     string
	Dest     string
	Keep     bool
)

func init() {
	rootCmd.AddCommand(lazyGetCmd)
}

var lazyGetCmd = &cobra.Command{
	Use:   "lazyget",
	Short: "get a thing so you don't have to",
	Long:  "",
}
