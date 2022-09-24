package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(shlexCmd)
}

var shlexCmd = &cobra.Command{
	Use:   "shlex",
	Short: "split shell string into json array",
	Run: func(_ *cobra.Command, args []string) {
		fmt.Printf(
			"[\"%s\"]\n", strings.Join(args, "\", \""),
		)
	},
}
