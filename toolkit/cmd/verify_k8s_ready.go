package cmd

import (
	"os"

	"github.com/cilium/scaffolding/toolkit/toolkit"
	"github.com/spf13/cobra"
)

func init() {
	verifyCmd.AddCommand(verifyK8sReadyCmd)
	// add wait option here

}

var verifyK8sReadyCmd = &cobra.Command{
	Use:   "k8s-ready",
	Short: "verify k8s cluster is ready to go",
	Run: func(_ *cobra.Command, _ []string) {
		clientset := toolkit.NewK8sClientSetOrDie(Logger, Kubeconfig)

		nodesReady, err := toolkit.VerifyNodesReady(CmdCtx, Logger, clientset)
		if err != nil {
			toolkit.ExitWithError(Logger, err)
		}
		if !nodesReady {
			os.Exit(1)
		}

		podsReady, err := toolkit.VerifyPodsReady(CmdCtx, Logger, clientset)
		if err != nil {
			toolkit.ExitWithError(Logger, err)
		}
		if !podsReady {
			os.Exit(1)
		}

		deploymentsReady, err := toolkit.VerifyDeploymentsReady(CmdCtx, Logger, clientset)
		if err != nil {
			toolkit.ExitWithError(Logger, err)
		}
		if !deploymentsReady {
			os.Exit(1)
		}
	},
}
