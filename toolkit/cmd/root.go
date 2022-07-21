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
	Long: `for k8s related commands, if a kubeconfig is not given the following locations will be tried (in order):
	
	1. KUBECONFIG env var
	2. ./kubeconfig
	3. ~/.kube/config
	`,
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
	rootCmd.PersistentFlags().StringVarP(&Kubeconfig, "kubeconfig", "k", "", "path to kubeconfig for k8s-related commands")
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "show debug logs")
}
