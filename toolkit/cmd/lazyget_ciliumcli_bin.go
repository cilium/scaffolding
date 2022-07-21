package cmd

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/cilium/scaffolding/toolkit/toolkit"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	ListOnly bool
	Platform string
	Arch     string
	Dest     string
)

func init() {
	lazyGetCmd.AddCommand(lazyGetCiliumCliBin)
	lazyGetCiliumCliBin.PersistentFlags().BoolVarP(&ListOnly, "list", "l", false, "list available versions")
	lazyGetCiliumCliBin.PersistentFlags().StringVarP(&Platform, "platform", "p", runtime.GOOS, "explicitly set platform")
	lazyGetCiliumCliBin.PersistentFlags().StringVarP(&Arch, "arch", "a", runtime.GOARCH, "explicitly set architecture")
	lazyGetCiliumCliBin.PersistentFlags().StringVarP(&Dest, "dest", "d", ".", "download directory")
}

var lazyGetCiliumCliBin = &cobra.Command{
	Use:   "cilium-cli",
	Short: "download version of cilium-cli binary",
	Args:  cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		releaseList, err := toolkit.ListCiliumCliVersions(CmdCtx)
		if err != nil {
			toolkit.ExitWithError(Logger, err)
		}
		if ListOnly {
			Logger.Debug("listing cilium versions")
			for _, release := range releaseList {
				fmt.Println(release.GetTagName())
			}
			return
		}

		if len(args) != 1 {
			toolkit.ExitWithError(Logger, errors.New("need cilium-cli version to download"))
		}

		targetVersion := args[0]

		for _, release := range releaseList {
			logFields := log.Fields{
				"tag": release.GetTagName(),
			}
			if targetVersion == release.GetTagName() {
				Logger.WithFields(logFields).Debug("found release")
				err := toolkit.DownloadCiliumCliBin(
					Logger, release, Platform, Arch, Dest,
				)
				if err != nil {
					toolkit.ExitWithError(Logger, err)
				}
				break
			}
			Logger.WithFields(logFields).Debug("not a match")
		}
	},
}
