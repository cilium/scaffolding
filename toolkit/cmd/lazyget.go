package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(lazyGetCmd)
}

var lazyGetCmd = &cobra.Command{
	Use:   "lazyget",
	Short: "get a thing so you don't have to",
	Long:  "",
}
