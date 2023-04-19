package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/cilium/scaffolding/toolkit/toolkit"
	"github.com/cilium/scaffolding/toolkit/toolkit/k8s"
)

type RonOptions struct {
	NSEnter                  bool
	NSEnterBinary            string
	NSEnterOpts              string
	PodName                  string
	PodImage                 string
	PinNode                  string
	PVC                      bool
	PVCName                  string
	PVCSize                  string
	PVCAccessMode            string
	ManagePVC                bool
	NSName                   string
	AutoCopy                 bool
	AutoCopyTarball          string
	CleanupAll               bool
	CleanupAndExit           bool
	CleanupPVC               bool
	CleanupPod               bool
	CleanupConfigMaps        bool
	RetryMaxAttempts         int
	RetryAttemptDelaySeconds int
	ConfigMapMounts          []string
	ConfigMapName            string
	ShowEvents               bool
}

var (
	RonOpts RonOptions
)

func init() {
	rootCmd.AddCommand(ronCmd)
	ronCmd.PersistentFlags().BoolVar(&RonOpts.NSEnter, "nsenter", false, "use nsenter as primary entrypoint")
	ronCmd.PersistentFlags().StringVar(
		&RonOpts.NSEnterBinary, "nsenter-binary", "nsenter", "path to or name of nsenter binary",
	)
	ronCmd.PersistentFlags().StringVar(
		&RonOpts.NSEnterOpts, "nsenter-opts", "-t 1 -u -i -n -p", "nsenter binary options",
	)
	ronCmd.PersistentFlags().StringVar(&RonOpts.PodName, "pod-name", "ron", "name of pod to create")
	ronCmd.PersistentFlags().StringVar(
		&RonOpts.PodImage, "pod-image", "alpine", "name of image to use in exec container",
	)
	ronCmd.PersistentFlags().StringVar(&RonOpts.PinNode, "node", "", "name of node to pin pod to")
	ronCmd.PersistentFlags().BoolVar(&RonOpts.PVC, "pvc", false, "mount pvc into exec pod for data transfer")
	ronCmd.PersistentFlags().StringVar(&RonOpts.PVCName, "pvc-name", "ron-pvc", "name of pvc to create")
	ronCmd.PersistentFlags().StringVar(&RonOpts.PVCSize, "pvc-size", "1Gi", "size of pvc to create")
	ronCmd.PersistentFlags().StringVar(
		&RonOpts.PVCAccessMode, "pvc-access-mode", "ReadWriteOnce", "access mode for pvc",
	)
	ronCmd.PersistentFlags().BoolVar(
		&RonOpts.ManagePVC, "manage-pvc", true, "assume control over pvc with given name, do not touch otherwise",
	)
	ronCmd.PersistentFlags().StringVar(&RonOpts.NSName, "ns", "default", "namespace for all created resources")
	ronCmd.PersistentFlags().BoolVar(
		&RonOpts.AutoCopy, "auto-copy", true, "copy all files present in pvc to path set by --auto-copy-dest",
	)
	ronCmd.PersistentFlags().StringVar(
		&RonOpts.AutoCopyTarball, "auto-copy-dest", "./ron.tar.gz",
		"destination tarball for all files copied out of exec pod's pvc",
	)
	ronCmd.PersistentFlags().BoolVar(
		&RonOpts.CleanupAll, "cleanup-all", true, "delete all created resources before exit",
	)
	ronCmd.PersistentFlags().BoolVar(&RonOpts.CleanupPod, "cleanup-pod", false, "delete exec pod before exit")
	ronCmd.PersistentFlags().BoolVar(&RonOpts.CleanupPVC, "cleanup-pvc", false, "delete storage pvc before exit")
	ronCmd.PersistentFlags().BoolVar(
		&RonOpts.CleanupAndExit, "cleanup-and-exit", false, "just cleanup resources that would be created and exit",
	)
	ronCmd.PersistentFlags().IntVar(
		&RonOpts.RetryMaxAttempts, "retry-attempts", 15, "number of times to retry an operation if it fails",
	)
	ronCmd.PersistentFlags().IntVar(
		&RonOpts.RetryAttemptDelaySeconds, "retry-delay", 1, "number of seconds to wait before retrying something",
	)
	ronCmd.PersistentFlags().StringSliceVarP(
		&RonOpts.ConfigMapMounts, "mount", "m", nil, "files to mount into exec container, such as scripts or data files",
	)
	ronCmd.PersistentFlags().StringVar(
		&RonOpts.ConfigMapName, "configmap-name", "roncm",
		"name of config map to create for mounting files into exec container",
	)
	ronCmd.PersistentFlags().BoolVar(
		&RonOpts.ShowEvents, "show-events", false, "show kubernetes events (WARN lots of output!)",
	)
}

func Ron() {}

var ronCmd = &cobra.Command{
	Use:   "ron",
	Short: "Run On Node",
	Run: func(_ *cobra.Command, args []string) {
		exitIfError := func(args ...any) {
			if len(args) == 0 {
				return
			}
			lastArg := args[len(args)-1]
			if lastArg != nil {
				err, ok := lastArg.(error)
				if !ok {
					return
				}
				toolkit.ExitWithError(Logger, err.(error))
			}
		}

		Logger.WithField("opts", RonOpts).Debug("using the following args")
		khelp := k8s.NewHelperOrDie(Logger, Kubeconfig)
		retryOpts := &k8s.RetryOpts{
			MaxAttempts: RonOpts.RetryMaxAttempts,
			Delay:       time.Duration(RonOpts.RetryAttemptDelaySeconds) * time.Second,
		}

		// create our resources
		ns := k8s.NewUnstructuredNS(RonOpts.NSName)
		pvc := k8s.WithScaffoldingLabel(k8s.NewUnstructuredPVC(
			RonOpts.PVCName, RonOpts.NSName, RonOpts.PVCAccessMode, RonOpts.PVCSize,
		))
		podCmd := make([]string, 0)
		if RonOpts.NSEnter {
			podCmd = append([]string{RonOpts.NSEnterBinary}, strings.Split(RonOpts.NSEnterOpts, " ")...)
		}
		pod := k8s.WithScaffoldingLabel(k8s.NewUnstructuredPod(
			RonOpts.PodName, RonOpts.NSName, RonOpts.PodImage, podCmd, args,
			&k8s.UnstructuredPodOpts{
				PinnedNode:         RonOpts.PinNode,
				MountPVC:           RonOpts.PVC,
				PVCName:            RonOpts.PVCName,
				MountConfigMap:     len(RonOpts.ConfigMapMounts) > 0,
				ConfigMapName:      RonOpts.ConfigMapName,
				HostNS:             true,
				WithSleepContainer: RonOpts.PVC,
			},
		))
		var cm *unstructured.Unstructured
		if len(RonOpts.ConfigMapMounts) > 0 {
			Logger.Info("constructing configmap for mounting files")
			cmData := map[string]string{}
			for _, cmf := range RonOpts.ConfigMapMounts {
				fileLogger := Logger.WithField("file", cmf)
				fileLogger.Info("reading file to mount")
				absPath, err := filepath.Abs(cmf)
				exitIfError(err)
				fileLogger.WithField("path", absPath).Debug("abs path for file")
				content, err := os.ReadFile(absPath)
				exitIfError(err)
				fileLogger.WithField("content", string(content)).Debug("got content for file")
				cmData[filepath.Base(absPath)] = string(content)
			}
			Logger.WithField("data", cmData).Debug("created data map for mounted files")
			cm = k8s.WithScaffoldingLabel(k8s.NewUnstructuredConfigMap(
				RonOpts.ConfigMapName, RonOpts.NSName, cmData,
			))
		}

		stopLogs := func() {} // will be set in the below branch in function "startPodLogs", called after cleanup
		var logwg sync.WaitGroup
		if !RonOpts.CleanupAndExit {
			var err error
			// Kick off our event watcher
			stopEventWatcher := func() {}
			if RonOpts.ShowEvents {
				Logger.Info("watching events")
				stopEventWatcher, err = khelp.WatchAndLogEvents(
					CmdCtx, v1.ListOptions{}, "reason", "firstTimestamp", "type",
				)
				exitIfError(err)
			}

			// Create NS
			Logger.WithField("namespace", RonOpts.NSName).Info("ensuring namespace exists and is ready")
			exitIfError(khelp.ApplyResource(CmdCtx, *k8s.GVRNamespace, ns, true, retryOpts))

			// Create pvc
			if RonOpts.PVC && RonOpts.ManagePVC {
				Logger.WithField("pvc", RonOpts.PVCName).Info("ensuring pvc is ready to go")
				exitIfError(khelp.ApplyResource(CmdCtx, *k8s.GVRPersistentVolumeClaim, pvc, true, retryOpts))
			}

			// Create configmap
			if cm != nil {
				Logger.WithField("configmap", RonOpts.ConfigMapName).Info("ensuring configMap is up to date")
				exitIfError(khelp.ApplyResource(CmdCtx, *k8s.GVRConfigMap, cm, true, retryOpts))
			}

			// Create pod
			podLogger := Logger.WithField("pod", RonOpts.PodName).WithField("namespace", RonOpts.NSName)
			podLogger.Info("setting up exec pod")
			exitIfError(khelp.ApplyResource(CmdCtx, *k8s.GVRPod, pod, false, retryOpts))

			// Need to watch the Ron Pod for changes as it starts up, so create a callback
			// to handle this

			// kick of streaming logs from the ron pod
			startWatchingPodLogs := func() error {
				Logger.Info("watching pod logs")
				stopLogs, err = khelp.LogPodLogs( // this stopLogs func is called during cleanup, see start of branch
					CmdCtx, RonOpts.NSName, RonOpts.PodName, &logwg, retryOpts, "sleep",
				)
				if err != nil {
					return err // unable to create infra for log streams
				}
				return nil
			}
			// check if the given pod has a main container with completed status
			checkMainContainerIsDone := func(res *unstructured.Unstructured) (bool, error) {
				containerStatuses, containerStatusesFound, err := k8s.GetNestedSliceStringInterfaceMap(
					res, "status", "containerStatuses",
				)
				if err != nil {
					return false, err // unable to get containerStatuses, quit and debug
				}
				if !containerStatusesFound {
					// not sure what could lead us here,
					// maybe there is a time in-between when a pod is ready and when statuses are published
					return false, fmt.Errorf("unable to find containerStatuses in pod %s", RonOpts.PodName)
				}
				for _, c := range containerStatuses {
					// find the main container
					podLogger.WithField("container", c).Debug("got container")
					if c["name"].(string) == "main" {
						podLogger.Debug("found main container")
						completed, err := k8s.DoesContainerStatusShowCompleted(c)
						podLogger.WithFields(log.Fields{
							"completed": completed,
							"err":       err,
						}).Debug("checked for terminated")
						if err != nil {
							return false, err // something bad happened trying to find status.reason
						}
						if completed {
							podLogger.Info("found main container and it is completed, exiting")
							return true, nil // only "good" return, everything is done
						}
						podLogger.WithField("container", "main").Debug("main container is not done")
						return false, nil // continue waiting for main container to finish
					}
				}
				// we should return inside the for loop, otherwise something is wrong
				return false, fmt.Errorf("could not find container 'main' in exec pod")
			}
			// put it all together in our callback
			logsStarted := false
			handleEventOnRonPod := func(event watch.Event) (bool, error) {
				podLogger.WithField("type", event.Type).Info("observed event on exec pod")
				podLogger.WithField("object", event.Object).Debug("event object observed")

				// exit if pod deleted by user
				if event.Type == watch.Deleted {
					return true, fmt.Errorf("ron pod was deleted, cannot continue") // done, pod isn't available
				}
				// something happened, check on the pod
				// 1. check if pod is ready
				// 1a. if pod is not ready, return and wait for next event
				// 1b. if pod is ready and we haven't started streaming logs, stream logs
				// 2. check if pod's main container has terminated with reason 'Completed'
				// 2a. if that's the case, we're all done, return
				// 2b. if that's not the case, return and wait for next event
				if event.Type == watch.Modified || event.Type == watch.Added {
					res := event.Object.(*unstructured.Unstructured)
					// is the pod actually ready?
					ready, err := k8s.CheckUnstructuredForReadyState(Logger, res)
					if err != nil {
						return false, err // unable to get pod, need a retry
					}
					if !ready {
						podLogger.Info(
							"pod itself is not ready, either there's an error or it just hasn't started yet",
						)
						return false, nil // pod isn't ready, wait for next event
					}
					if ready {
						podLogger.Info("pod showing ready state")
						if !logsStarted {
							err := startWatchingPodLogs()
							if err != nil {
								return false, err // unable to start watching logs
							}
							logsStarted = true
						}
						podLogger.Info("checking if exec pod is finished")
						// look for completed main container
						return checkMainContainerIsDone(res)
					}
				}
				return false, nil // got a watch event that isn't modified or deleted, continue
			}

			// this becomes our main loop, waiting for pod to start and complete
			_, err = khelp.WaitOnWatchedResource(
				CmdCtx, *k8s.GVRPod, RonOpts.NSName, k8s.NewFieldSelectorFromName(RonOpts.PodName), "", nil, handleEventOnRonPod,
			)
			exitIfError(err)

			// copy files out
			if RonOpts.PVC && RonOpts.AutoCopy {
				podLogger.Info("copying /store")
				tarballBuffer := bytes.Buffer{}
				err = khelp.PodExec(
					RonOpts.PodName, "sleep", RonOpts.NSName, []string{"tar", "-zc", "/store"}, &tarballBuffer,
				)
				exitIfError(err)
				tarballAbsolutePath, err := filepath.Abs(RonOpts.AutoCopyTarball)
				exitIfError(err)
				podLogger.WithField("dst", tarballAbsolutePath).Debug("writing buffer to tarball")
				exitIfError(os.WriteFile(tarballAbsolutePath, tarballBuffer.Bytes(), 0755))
			}
			stopEventWatcher()
		}

		if RonOpts.CleanupAll {
			if RonOpts.NSName == "default" {
				// Remove things one by one since can't just purge the ns
				RonOpts.CleanupPVC = true
				RonOpts.CleanupPod = true
				RonOpts.CleanupConfigMaps = true
			} else {
				err := khelp.DeleteResourceAndWaitGone(
					CmdCtx, *k8s.GVRNamespace, ns.GetName(), ns.GetNamespace(), retryOpts,
				)
				if err != nil {
					Logger.WithError(err).WithField("namespace", RonOpts.NSName).Warning("unable to delete namespace")
				}
				return // don't need to continue to clean up things individually, since we purged the ns
			}
		}
		if RonOpts.CleanupPod {
			Logger.Info("waiting for log goroutines to hit EOF on their streams")
			logwg.Wait() // wait for logs to be done first
			err := khelp.DeleteResourceAndWaitGone(CmdCtx, *k8s.GVRPod, pod.GetName(), pod.GetNamespace(), retryOpts)
			if err != nil {
				Logger.WithError(err).WithField("pod", RonOpts.PodName).Warning("unable to delete pod")
			}
		} else {
			// k8s.Helper.LogPodLogs has built-in methods for stopping streams when the target pod is deleted
			// but it returns a cancel function we need to call in case RonOpts.CleanupPod is false
			Logger.Debug("calling stop logs context cancel function")
			stopLogs()
		}
		if RonOpts.PVC && RonOpts.CleanupPVC && RonOpts.ManagePVC {
			err := khelp.DeleteResourceAndWaitGone(
				CmdCtx, *k8s.GVRPersistentVolumeClaim, pvc.GetName(), pvc.GetNamespace(), retryOpts,
			)
			if err != nil {
				Logger.WithError(err).WithField("pvc", RonOpts.PVCName).Warning("unable to delete PVC")
			}
		}
		if len(RonOpts.ConfigMapMounts) > 0 && RonOpts.CleanupConfigMaps {
			err := khelp.DeleteResourceAndWaitGone(
				CmdCtx, *k8s.GVRConfigMap, cm.GetName(), cm.GetNamespace(), retryOpts,
			)
			if err != nil {
				Logger.WithError(err).WithField("configmap", RonOpts.ConfigMapName).Warning(
					"unable to delete configmap",
				)
			}
		}

	},
}
