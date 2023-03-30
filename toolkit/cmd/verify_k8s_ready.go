package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/cilium/scaffolding/toolkit/toolkit"
	"github.com/cilium/scaffolding/toolkit/toolkit/k8s"
)

func init() {
	verifyCmd.AddCommand(verifyK8sReadyCmd)
	// add wait option here

}

var verifyK8sReadyCmd = &cobra.Command{
	Use:   "k8s-ready",
	Short: "verify k8s cluster is ready to go",
	Run: func(_ *cobra.Command, _ []string) {
		khelp := k8s.NewHelperOrDie(Logger, Kubeconfig)

		nodesReady, err := khelp.VerifyResourcesAreReady(CmdCtx, *k8s.GVRNode)
		if err != nil {
			toolkit.ExitWithError(Logger, err)
		}
		if !nodesReady {
			os.Exit(1)
		}

		podsReady, err := khelp.VerifyResourcesAreReady(CmdCtx, *k8s.GVRPod)
		if err != nil {
			toolkit.ExitWithError(Logger, err)
		}
		if !podsReady {
			os.Exit(1)
		}

		deploymentsReady, err := khelp.VerifyResourcesAreReady(
			CmdCtx, *k8s.GVRDeployment,
		)
		if err != nil {
			toolkit.ExitWithError(Logger, err)
		}
		if !deploymentsReady {
			os.Exit(1)
		}
	},
}
