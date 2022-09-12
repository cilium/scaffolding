package k8s

import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	k8sDynamic "k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/cilium/scaffolding/toolkit/toolkit"
)

// FindKubeconfig attempts to resolve the location of the kubeconfig, returning its path.
// The following places will be uses, in order from first to last:
// 1. KUBECONFIG - environment variable
// 2. ./kubeconfig - kubeconfig in current working directory
// 3. ~/.kube/config - user's default kubeconfig
func FindKubeconfig(logger *log.Logger) string {
	logFoundKube := func(kubeconfig string, loc string) {
		logger.WithField("kubeconfig", kubeconfig).Info(fmt.Sprintf("found kubeconfig in %s", loc))
	}
	kubeEnv := os.Getenv("KUBECONFIG")
	if kubeEnv != "" && toolkit.PathExists(kubeEnv) {
		logFoundKube(kubeEnv, "KUBECONFIG")
		return kubeEnv
	}

	cwd, err := os.Getwd()
	kubeCwd := filepath.Join(cwd, "kubeconfig")
	if err != nil && toolkit.PathExists(kubeCwd) {
		logFoundKube(kubeCwd, "cwd")
		return kubeCwd
	}

	home := homedir.HomeDir()
	kubeHome := filepath.Join(home, ".kube", "config")
	if home != "" && toolkit.PathExists(kubeHome) {
		logFoundKube(kubeHome, "user home")
		return kubeHome
	}
	return ""
}

func NewK8sConfigOrDie(logger *log.Logger, kubeconfigPath string) *rest.Config {
	logAttempt := func(prefix string, kubeconfig string) {
		logger.Info(fmt.Sprintf("%s kubeconfig %s", prefix, kubeconfig))
	}

	orDie := func(kubeconfig string) *rest.Config {
		logAttempt("trying", kubeconfig)

		if !toolkit.PathExists(kubeconfig) {
			toolkit.ExitWithError(logger, fmt.Errorf("kubeconfig not found: %s", kubeconfig))
			return nil
		}

		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			toolkit.ExitWithError(logger, err)
			return nil
		}

		return config
	}

	if kubeconfigPath == "" {
		return orDie(FindKubeconfig(logger))
	}
	return orDie(kubeconfigPath)
}

// NewK8sClientSetOrDie attempts to use the given kubeconfig path to create a new k8s clientset object.
// Upon failure, `ExitWithError` is called, which terminates execution.
func NewK8sClientSetOrDie(logger *log.Logger, config *rest.Config) k8s.Interface {
	logger.Debug("creating k8s clientset")
	clientset, err := k8s.NewForConfig(config)
	if err != nil {
		toolkit.ExitWithError(logger, err)
		return nil
	}
	logger.Debug("created k8s clientset")
	return clientset
}

func NewDynamicK8sClientSetOrDie(logger *log.Logger, config *rest.Config) k8sDynamic.Interface {
	logger.Debug("creating dynamic k8s clientset")
	clientset, err := k8sDynamic.NewForConfig(config)
	if err != nil {
		toolkit.ExitWithError(logger, err)
		return nil
	}
	logger.Debug("created dynamic k8s clientset")
	return clientset
}
