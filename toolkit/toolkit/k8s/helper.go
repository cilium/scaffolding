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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	k8sDynamic "k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	watchTools "k8s.io/client-go/tools/watch"
	"k8s.io/kubectl/pkg/cmd/exec"

	"github.com/cilium/scaffolding/toolkit/toolkit"
)

// Helper has useful methods for doing things in kubernetes.
// The goal with Helper is to reduce boilerplate code of tasks required by commands in toolkit
type Helper struct {
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
func NewHelperOrDie(logger *log.Logger, kubeconfig string) *Helper {
	khelp := Helper{
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
	ctx context.Context, gvr schema.GroupVersionResource, res *unstructured.Unstructured, waitReady bool, ro *RetryOpts,
) (*unstructured.Unstructured, error) {
	name := res.GetName()
	ns := res.GetNamespace()

	resLogger := k.getResourceLoggerFromRes(gvr, res)
	resInterface := k.DynamicClientset.Resource(gvr).Namespace(res.GetNamespace())

	applyRes := func() (*unstructured.Unstructured, error) {
		resLogger.Info("applying resource")
		resLogger.WithField("res-raw", res).Debug("applying unstructured struct")
		returnedRes, err := resInterface.Apply(
			ctx, name, res,
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
			ctx, gvr, ns, NewFieldSelectorFromName(name), "", nil,
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

// DeleteResourceAndWaitGone is a wrapper around a dynamic Delete, only returning when it is confirmed that the given
// resource is gone.
func (k *Helper) DeleteResourceAndWaitGone(
	ctx context.Context, gvr schema.GroupVersionResource, name string, ns string, ro *RetryOpts,
) error {
	delLogger := k.getResourceLoggerFromGivens(gvr.Resource, ns, name)
	delLogger.Debug("deleting resource and ensuring it is gone")

	resInterface := k.DynamicClientset.Resource(gvr).Namespace(ns)

	// Check if the resource even exists before trying to delete it.
	delLogger.Debug("checking if resource exists")
	_, err := resInterface.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			delLogger.Debug("resource does not exist, considering it 'deleted'")

			return nil
		}

		delLogger.WithError(err).Warn("error occurred while checking if resource exists")

		return err
	}

	// Setup go-routine to listen for a delete event on the existing resource.

	// Cancel the watch after this function returns.
	checkCtx, cancelCheck := context.WithCancel(ctx)
	defer cancelCheck()

	// Only attempt to delete the resource after the watch has completed a cache sync, to prevent a situation
	// where the resource is delete before the watch is able to observe the deletion event.
	cacheSyncDone := make(chan bool)
	// Holds returned error, or nil, from the check go-routine.
	checkDone := make(chan error, 1)

	go func() {
		delLogger.Info("waiting for resource to be gone")
		_, err := k.WaitOnWatchedResource(
			checkCtx, gvr, ns, NewFieldSelectorFromName(name), "",
			func() {
				close(cacheSyncDone)
			},
			func(event watch.Event) (bool, error) {
				if event.Type != watch.Deleted {
					return false, nil
				}
				delLogger.Info("resource is gone")

				return true, nil
			},
		)

		if err != nil {
			delLogger.WithError(err).Warn("error occurred while waiting for resource to be gone")
		}

		checkDone <- err
		return
	}()

	// Wait for the cache sync or context cancel.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-cacheSyncDone:
	}

	delLogger.Warn("marking for deletion")

	err = resInterface.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		// If we get this error, then we know the resource is gone and can return.
		if errors.IsNotFound(err) {
			delLogger.Info("resource is gone")

			return nil
		}

		delLogger.WithError(err).Error("unable to delete resource")
		return err
	}

	// Wait for the check to be done or context cancel.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-checkDone:
		return err
	}
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
	pod, err := pods.Get(ctx, podName, metav1.GetOptions{})
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
		).Stream(ctx)
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

// WaitOnWatchedResource is a wrapper around client-go/tools/watch.UntilWithSync function, providing the plumbing needed
// to watch resources, without having to handle creating a lister watcher.
func (k *Helper) WaitOnWatchedResource(
	ctx context.Context, gvr schema.GroupVersionResource, ns string, fieldSelector string, labelSelector string,
	cacheSyncCallback func(), conditions ...watchTools.ConditionFunc,
) (*watch.Event, error) {
	kind, ok := ResourceToKind[gvr.Resource]
	if !ok {
		return nil, fmt.Errorf("unknown kind for resource %s", gvr.Resource)
	}

	objType := &unstructured.Unstructured{}
	objType.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   gvr.Group,
			Version: gvr.Version,
			Kind:    kind,
		},
	)

	resInterface := k.DynamicClientset.Resource(gvr).Namespace(ns)

	optionsModifier := func(options *metav1.ListOptions) {
		options.FieldSelector = fieldSelector
		options.LabelSelector = labelSelector
	}

	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			optionsModifier(&options)
			return resInterface.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			optionsModifier(&options)
			return resInterface.Watch(ctx, options)
		},
	}

	logger := k.getResourceLoggerFromGivens(gvr.Resource, ns, "").WithFields(log.Fields{
		"kind":           kind,
		"field-selector": fieldSelector,
		"label-selector": labelSelector,
	})

	logger.Debug("watching resource")

	return watchTools.UntilWithSync(
		ctx, lw, objType,
		func(store cache.Store) (bool, error) {
			if cacheSyncCallback != nil {
				cacheSyncCallback()
			}
			return false, nil
		},
		conditions...,
	)
}

// VerifyResourceIsReady runs a dynamic get on the given resource with name and ns, then passes it to
// CheckUnstructuredForReadyState.
func (k *Helper) VerifyResourceIsReady(
	ctx context.Context, gvr schema.GroupVersionResource, name string, ns string,
) (bool, error) {
	resInterface := k.DynamicClientset.Resource(gvr).Namespace(ns)
	resource, err := resInterface.Get(ctx, name, metav1.GetOptions{})
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
func (k *Helper) VerifyResourcesAreReady(ctx context.Context, gvr schema.GroupVersionResource) (bool, error) {
	resourceName := gvr.Resource
	k.Logger.Info(fmt.Sprintf("verifying %s in ready state", resourceName))

	resources, err := k.DynamicClientset.Resource(gvr).List(ctx, metav1.ListOptions{})
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
func (k *Helper) WatchAndLogEvents(
	ctx context.Context, watchOpts metav1.ListOptions, eventFields ...string,
) (func(), error) {
	eventLogger := k.Logger.WithField("opts", watchOpts)
	eventLogger.Debug("starting event watcher")
	watchInterface, err := k.DynamicClientset.Resource(*GVREvents).Watch(ctx, watchOpts)
	if err != nil {
		k.Logger.WithError(err).Error("unable to create event watcher interface")
		return nil, err
	}

	watchContext, stopFunc := context.WithCancel(ctx)
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
