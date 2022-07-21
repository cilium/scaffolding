package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(provisionCmd)
}

var provisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "create a thing",
}
