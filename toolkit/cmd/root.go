package cmd

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	Logger     = log.StandardLogger()
	CmdCtx     = context.Background()
	Kubeconfig string
	Verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "toolkit",
	Short: "collection of tools to assist in running performance benchmarks",
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		if Verbose {
			Logger.SetLevel(log.DebugLevel)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// k8s stuff
	rootCmd.PersistentFlags().StringVarP(
		&Kubeconfig, "kubeconfig", "k", "",
		`path to kubeconfig for k8s-related commands
if not given will try the following (in order):
KUBECONFIG, ./kubeconfig, ~/.kube/config`,
	)
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "show debug logs")
}
