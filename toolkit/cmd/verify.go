package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(verifyCmd)
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "verify the state of things",
}
