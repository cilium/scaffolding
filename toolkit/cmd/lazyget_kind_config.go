package cmd

import (
	"fmt"

	"github.com/cilium/scaffolding/toolkit/toolkit"
	"github.com/spf13/cobra"
)

var (
	Name            string
	NumWorkers      int
	NumControlPlane int
	WithCNI         bool
	KindImage       string
)

func init() {
	lazyGetCmd.AddCommand(lazyGetKindImageConfigCmd)
	lazyGetKindImageConfigCmd.PersistentFlags().StringVarP(&Name, "name", "n", "kind", "name of the kind cluster")
	lazyGetKindImageConfigCmd.PersistentFlags().IntVarP(
		&NumWorkers, "num-workers", "w", 0, "number of workers to create",
	)
	lazyGetKindImageConfigCmd.PersistentFlags().IntVarP(&NumControlPlane, "num-control", "c", 1, "number of control plane nodes to create")
	lazyGetKindImageConfigCmd.PersistentFlags().BoolVar(&WithCNI, "with-cni", false, "have kind use its default cni")
	lazyGetKindImageConfigCmd.PersistentFlags().StringVarP(
		&KindImage, "image", "i", toolkit.KindNodeImageLatest, "container image for kind nodes",
	)
}

var lazyGetKindImageConfigCmd = &cobra.Command{
	Use:   "kind-config",
	Short: "create a simple kind config",
	Run: func(_ *cobra.Command, _ []string) {
		result, err := toolkit.NewKindConfig(Logger, Name, NumWorkers, NumControlPlane, WithCNI, KindImage)
		if err != nil {
			toolkit.ExitWithError(Logger, err)
		}
		Logger.Debug(result)
		fmt.Print(result)
	},
}
