package k8s

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	k8sDynamic "k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/exec"

	"github.com/cilium/scaffolding/toolkit/toolkit"
)

// Helper has useful methods for doing things in kubernetes.
// The goal with Helper is to reduce boilerplate code of tasks required by commands in toolkit
type Helper struct {
	Ctx              context.Context
	DynamicClientset k8sDynamic.Interface
	Clientset        k8s.Interface
	Kubeconfig       string
	Config           *rest.Config
	Logger           *log.Logger
}

// RetryOpts defines variables controlling how tasks should be retried.
// Currently only supports a simple "retry x times with y delay in-between each attempt"
type RetryOpts struct {
	MaxAttempts int
	Delay       time.Duration
}

// NewHelperOrDie creates a new Helper instance from the given kubeconfig and logger.
// If something goes wrong while creating it, then the entire program will exit.
func NewHelperOrDie(ctx context.Context, logger *log.Logger, kubeconfig string) *Helper {
	khelp := Helper{
		Ctx:        ctx,
		Kubeconfig: kubeconfig,
		Logger:     logger,
	}

	khelp.Config = NewK8sConfigOrDie(logger, kubeconfig)
	khelp.Config.GroupVersion = &schema.GroupVersion{Version: "v1"}
	khelp.Config.APIPath = "/api"
	khelp.Config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	khelp.Clientset = NewK8sClientSetOrDie(logger, khelp.Config)
	khelp.DynamicClientset = NewDynamicK8sClientSetOrDie(logger, khelp.Config)

	return &khelp
}

// getResourceLoggerFromGivens constructs a logger with the given parameters added as fields.
func (k *Helper) getResourceLoggerFromGivens(resourceType string, namespace string, name string) *log.Entry {
	resLogger := k.Logger.WithFields(log.Fields{
		"resource": resourceType,
	})
	if name != "" {
		resLogger = resLogger.WithField("name", name)
	}
	if namespace != "" {
		resLogger = resLogger.WithField("namespace", namespace)
	}

	return resLogger
}

// getResourceLoggerFromRes calls getResourceLoggerFromGivens, pulling parameters from the given unstructured resource.
func (k *Helper) getResourceLoggerFromRes(gvr schema.GroupVersionResource, res *unstructured.Unstructured) *log.Entry {
	name := res.GetName()
	ns := res.GetNamespace()
	resourceType := gvr.String()
	return k.getResourceLoggerFromGivens(resourceType, ns, name)
}

// ApplyResource is a wrapper around a dynamic apply, logging tasks and errors along the way.
// waitReady can be given to wait for the given structure to be ready. This uses WaitOnWatchedResource and
// CheckUnstructuredForReadyState.
func (k *Helper) ApplyResource(
	gvr schema.GroupVersionResource, res *unstructured.Unstructured, waitReady bool, ro *RetryOpts,
) (*unstructured.Unstructured, error) {
	name := res.GetName()
	ns := res.GetNamespace()

	resLogger := k.getResourceLoggerFromRes(gvr, res)
	resInterface := k.DynamicClientset.Resource(gvr).Namespace(res.GetNamespace())

	applyRes := func() (*unstructured.Unstructured, error) {
		resLogger.Info("applying resource")
		resLogger.WithField("res-raw", res).Debug("applying unstructured struct")
		returnedRes, err := resInterface.Apply(
			k.Ctx, name, res,
			metav1.ApplyOptions{
				FieldManager: "cilium/scaffolding/toolkit",
			},
		)
		if err != nil {
			resLogger.WithError(err).Error("unable to apply")
			return nil, err
		}
		return returnedRes, nil
	}

	doWaitReady := func() (*unstructured.Unstructured, error) {
		resLogger.Info("waiting for resource to be ready")

		var returnedRes unstructured.Unstructured

		_, err := k.WaitOnWatchedResource(
			k.Ctx, gvr, ns, NewListOptionsFromName(name),
			func(event watch.Event) (bool, error) {
				resLogger.WithField("event", event).WithField("obj", event.Object).Debug(
					"got event while waiting for res to be ready",
				)
				if event.Type == watch.Modified || event.Type == watch.Added {
					res := event.Object.(*unstructured.Unstructured)
					resLogger.WithField("res", res).Debug("pulled resource from event")
					ready, err := CheckUnstructuredForReadyState(k.Logger, res)
					if err != nil {
						return false, err
					}
					if ready {
						returnedRes = *res
						return true, nil
					}
				}
				return false, nil
			},
		)
		return &returnedRes, err
	}

	returnedRes, err := applyRes()
	if err != nil {
		return nil, err
	}

	if waitReady {
		return doWaitReady()
	}
	return returnedRes, nil
}

// DeleteResourceAndWaitGone is a wrapper around a dynamic Delete, only returning when a Deleted event is observed.
func (k *Helper) DeleteResourceAndWaitGone(
	gvr schema.GroupVersionResource, name string, ns string, ro *RetryOpts,
) error {
	delLogger := k.getResourceLoggerFromGivens(gvr.Resource, ns, name)
	resInterface := k.DynamicClientset.Resource(gvr).Namespace(ns)

	doDelete := func() error {
		delLogger.Warn("marking for deletion")
		err := resInterface.Delete(k.Ctx, name, metav1.DeleteOptions{})
		if err != nil {
			delLogger.WithError(err).Error("unable to delete resource")
			return err
		}
		return nil
	}

	checkDone := make(chan bool)
	checkCtx, cancelCheck := context.WithCancel(k.Ctx)
	defer cancelCheck()
	doCheck := func() error {
		delLogger.Info("waiting for resource to be gone")
		_, err := k.WaitOnWatchedResource(
			checkCtx, gvr, ns, NewListOptionsFromName(name), func(event watch.Event) (bool, error) {
				if event.Type != watch.Deleted {
					return false, nil
				}
				return true, nil
			},
		)
		checkDone <- true
		return err
	}

	go doCheck()
	err := doDelete()
	if err != nil {
		cancelCheck()
		return err
	}
	<-checkDone
	return nil
}

// LogPodLogs attempts to stream logs from the containers in the given pod.
// There are two possible exit options: (1) Wait for all streams to hit an EOF with wg.Done, or (2) call the
// returned cancel function.
// Containers can be ignored from the log streams by passing them as variadic arguments. This is useful for pause
// or containers which have no logs, as they may not ever hit an EOF. This could cause the wait group to hang
// forever.
// This method needs a lot of love in the future. A great nice-to-have is being able to watch the given pod and
// create log streams for containers as they become available. This would require some work to differentiate between
// initContainers and normal containers
func (k *Helper) LogPodLogs(
	ctx context.Context, ns string, podName string, wg *sync.WaitGroup, ro *RetryOpts,
	ignoredContainers ...string,
) (context.CancelFunc, error) {
	podLogger := k.getResourceLoggerFromGivens("pods", ns, podName)
	podLogger.Debug("setting up log streams for containers")

	logCtx, cancelFunc := context.WithCancel(ctx)

	pods := k.Clientset.CoreV1().Pods(ns)
	pod, err := pods.Get(k.Ctx, podName, metav1.GetOptions{})
	if err != nil {
		podLogger.WithError(err).Error("unable to get pod for log streams")
		return cancelFunc, err
	}
	for _, container := range pod.Spec.Containers {
		containerName := container.Name
		if toolkit.SliceContains(ignoredContainers, containerName) {
			continue
		}
		containerLogger := podLogger.WithField("container", containerName)
		containerLogger.Info("opening stream for logs")
		stream, err := pods.GetLogs(
			podName,
			&v1.PodLogOptions{
				Container:  containerName,
				Follow:     true,
				Timestamps: true,
			},
		).Stream(k.Ctx)
		if err != nil {
			podLogger.WithError(err).Error("unable to get logs from container")
			continue
		}

		wg.Add(1)
		go func() {
			containerLogger.Debug("starting to read logs from stream")
			defer stream.Close()
			reader := bufio.NewScanner(stream)
			exit := false
			for reader.Scan() {
				if exit {
					break
				}
				select {
				case <-logCtx.Done():
					containerLogger.WithError(logCtx.Err()).Debug("container log stream context cancelled")
					exit = true
				default:
					line := reader.Text()
					containerLogger.Info(fmt.Sprintf("[%s]: %s", containerName, line))
				}
			}
			containerLogger.Info("closing log stream")
			if err := reader.Err(); err != nil {
				containerLogger.WithError(err).Error("got error while reading log stream")
			}
			wg.Done()
		}()
	}

	return cancelFunc, nil
}

// WaitOnWatchedResource is a wrapper around a dynamic watch, passing events to a callback.
// The given callback function should have two returns: a boolean describing if the loop should terminate, and
// an error describing if an error occurred during the loop. If an error does occur, the loop will terminate.
func (k *Helper) WaitOnWatchedResource(
	ctx context.Context, gvr schema.GroupVersionResource, ns string, listOptions metav1.ListOptions,
	callback func(watch.Event) (bool, error),
) (bool, error) {
	resLogger := k.getResourceLoggerFromGivens(gvr.Resource, ns, "")
	resInterface := k.DynamicClientset.Resource(gvr).Namespace(ns)
	resLogger.Debug("watching resource")

	resLogger.Debug("creating watcher")
	watcher, err := resInterface.Watch(k.Ctx, listOptions)
	if err != nil {
		resLogger.WithError(err).Error("unable to create watcher")
		return false, err
	}
	defer watcher.Stop()

	for {
		select {
		case event := <-watcher.ResultChan():
			resLogger.WithField("event", event).WithField("obj", event.Object).Debug("got event while watching res")
			done, err := callback(event)
			if err != nil {
				return false, err
			}
			if !done {
				resLogger.Debug("not done watching, still waiting")
			} else {
				resLogger.Debug("done watching")
				return true, nil
			}
		case <-ctx.Done():
			resLogger.WithError(ctx.Err()).Debug("watch context cancelled")
			return false, ctx.Err()
		}
	}
}

// VerifyResourceIsReady runs a dynamic get on the given resource with name and ns, then passes it to
// CheckUnstructuredForReadyState.
func (k *Helper) VerifyResourceIsReady(gvr schema.GroupVersionResource, name string, ns string) (bool, error) {
	resInterface := k.DynamicClientset.Resource(gvr).Namespace(ns)
	resource, err := resInterface.Get(k.Ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	ready, err := CheckUnstructuredForReadyState(k.Logger, resource)
	if err != nil {
		return false, err
	}
	if !ready {
		return false, nil
	}
	return true, nil
}

// VerifyResourcesAreReady will list all resources describe by the given GVR and pass them to
// CheckUnstructuredForReadyState.
func (k *Helper) VerifyResourcesAreReady(gvr schema.GroupVersionResource) (bool, error) {
	resourceName := gvr.Resource
	k.Logger.Info(fmt.Sprintf("verifying %s in ready state", resourceName))

	resources, err := k.DynamicClientset.Resource(gvr).List(k.Ctx, metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	items := resources.Items
	if len(items) == 0 {
		k.Logger.Warn(fmt.Sprintf(
			"it seems the cluster has zero %s, so, they are all ready?",
			resourceName,
		))
		return true, nil
	}

	resourcesAreReady := true
	for _, resource := range items {
		ready, err := CheckUnstructuredForReadyState(k.Logger, &resource)
		if err != nil {
			return false, err
		}
		if !ready {
			resourcesAreReady = false
		}
	}

	if resourcesAreReady {
		k.Logger.Info(fmt.Sprintf("💚 all %s are ready!", resourceName))
	} else {
		k.Logger.Error(fmt.Sprintf("🤔 looks like not all %s are ready", resourceName))
	}

	return resourcesAreReady, nil
}

// PodExec runs a command within a pod within the given container.
// dst is a buffer where output from the command will be placed.
func (k *Helper) PodExec(
	podName string, containerName string, namespace string, cmd []string, dst *bytes.Buffer,
) error {
	options := &exec.ExecOptions{}

	errOut := bytes.NewBuffer([]byte{})
	writer := bytes.Buffer{}

	options.StreamOptions = exec.StreamOptions{
		IOStreams: genericclioptions.IOStreams{
			In:     nil,
			Out:    &writer,
			ErrOut: errOut,
		},
		Namespace: namespace,
		PodName:   podName,
	}
	options.Executor = &exec.DefaultRemoteExecutor{}
	options.Namespace = namespace
	options.PodName = podName
	options.ContainerName = containerName
	options.Config = k.Config
	options.PodClient = k.Clientset.CoreV1()
	options.Command = cmd

	logger := k.Logger.WithFields(log.Fields{
		"pod":       podName,
		"container": containerName,
		"namespace": namespace,
		"cmd":       strings.Join(cmd, " "),
	})
	logger.Info("running something in a container")
	err := options.Run()
	if err != nil {
		logger.WithError(err).
			WithField("out", writer.String()).
			WithField("err-out", errOut.String()).
			Error(
				"error occurred while running something in a container",
			)
		return err
	}

	n, err := io.Copy(dst, &writer)
	if err != nil {
		logger.WithError(err).Error("error occurred while copying bytes from io writer to dst buffer")
		return err
	}
	logger.WithField("num-bytes", n).Debug("copied bytes from io writer to dst buffer")

	return nil
}

// WatchAndLogEvents is a wrapper around a dynamic watch call, logging events as they come.
// Fields to pull from each event can be specified as variadic arguments.
func (k *Helper) WatchAndLogEvents(watchOpts metav1.ListOptions, eventFields ...string) (func(), error) {
	eventLogger := k.Logger.WithField("opts", watchOpts)
	eventLogger.Debug("starting event watcher")
	watchInterface, err := k.DynamicClientset.Resource(*GVREvents).Watch(k.Ctx, watchOpts)
	if err != nil {
		k.Logger.WithError(err).Error("unable to create event watcher interface")
		return nil, err
	}

	watchContext, stopFunc := context.WithCancel(k.Ctx)
	go func() {
		defer watchInterface.Stop()
		for {
			select {
			case watchEvent := <-watchInterface.ResultChan():
				eventLogger.WithField("watch-event", watchEvent).Debug("got watch event")
				event := watchEvent.Object.(*unstructured.Unstructured)
				fields := log.Fields{}
				level := log.InfoLevel
				for k, v := range event.Object {
					if k == "message" {
						continue
					}
					if k == "type" && v.(string) != "Normal" {
						level = log.WarnLevel
					}
					if toolkit.SliceContains(eventFields, k) {
						fields[k] = v
					}
				}
				msg, ok := event.Object["message"].(string)
				if !ok {
					msg = "new event"
				}
				k.Logger.WithFields(fields).Log(level, msg)
			case <-watchContext.Done():
				eventLogger.Debug("stopping event watcher")
				return
			}
		}
	}()

	return stopFunc, nil
}
