package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cilium/scaffolding/toolkit/toolkit"
	"github.com/spf13/cobra"
)

func GetAvailableVersionsStr() string {
	var err error
	var versionStr string
	versionsStrBuilder := strings.Builder{}
	versionsStrBuilder.WriteString("available versions are: (pick one of the left size <---)\n")

	for _, k8s := range AvailableVersionsSlice {
		versionStr = fmt.Sprintf("%s: %s\n", k8s, K8sVToKindImage[k8s])
		_, err = versionsStrBuilder.WriteString(versionStr)
		if err != nil {
			toolkit.ExitWithError(Logger, err)
		}
	}
	return versionsStrBuilder.String()
}

var (
	K8sVToKindImage = map[string]string{
		"latest": toolkit.KindNodeImageLatest,
		"24":     toolkit.KindNodeImageV24,
		"23":     toolkit.KindNodeImageV23,
		"22":     toolkit.KindNodeImageV22,
		"21":     toolkit.KindNodeImageV21,
		"20":     toolkit.KindNodeImageV20,
		"19":     toolkit.KindNodeImageV19,
		"18":     toolkit.KindNodeImageV18,
	}
	AvailableVersionsSlice = func() []string {
		result := []string{}
		for k8s := range K8sVToKindImage {
			result = append(result, k8s)
		}
		sort.Strings(result)
		return result
	}()
	AvailableVersionsStr = GetAvailableVersionsStr()
)

func init() {
	lazyGetCmd.AddCommand(lazyGetKindImageCmd)
}

var lazyGetKindImageCmd = &cobra.Command{
	Use:       "kind-image [k8s-version]",
	Short:     "get path to kind image based on kubernetes version",
	Long:      AvailableVersionsStr,
	Args:      cobra.ExactValidArgs(1),
	ValidArgs: AvailableVersionsSlice,
	Run: func(_ *cobra.Command, args []string) {
		fmt.Print(K8sVToKindImage[args[0]])
	},
}
