package toolkit

import (
	"fmt"
	"os"

	"path/filepath"

	log "github.com/sirupsen/logrus"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// FindKubeconfig attempts to resolve the location of the kubeconfig, returning its path.
// The following places will be uses, in order from first to last:
// 1. KUBECONFIG - environment variable
// 2. ./kubeconfig - kubeconfig in current working directory
// 3. ~/.kube/config - user's default kubeconfig
func FindKubeconfig(logger *log.Logger) string {
	logFoundKube := func(kubeconfig string, loc string) {
		logger.WithFields(
			log.Fields{
				"kubeconfig": kubeconfig,
			},
		).Info(fmt.Sprintf("found kubeconfig in %s", loc))
	}
	kubeEnv := os.Getenv("KUBECONFIG")
	if kubeEnv != "" && PathExists(kubeEnv) {
		logFoundKube(kubeEnv, "KUBECONFIG")
		return kubeEnv
	}

	cwd, err := os.Getwd()
	kubeCwd := filepath.Join(cwd, "kubeconfig")
	if err != nil && PathExists(kubeCwd) {
		logFoundKube(kubeCwd, "cwd")
		return kubeCwd
	}

	home := homedir.HomeDir()
	kubeHome := filepath.Join(home, ".kube", "config")
	if home != "" && PathExists(kubeHome) {
		logFoundKube(kubeHome, "user home")
		return kubeHome
	}
	return ""
}

// NewK8sClientSetOrDie attempts to use the given kubeconfig path to create a new k8s clientset object.
// Upon failure, `ExitWithError` is called, which terminates execution.
func NewK8sClientSetOrDie(logger *log.Logger, kubeconfigPath string) k8s.Interface {
	logAttempt := func(prefix string, kubeconfig string) {
		logger.Info(fmt.Sprintf("%s kubeconfig %s", prefix, kubeconfig))
	}

	orDie := func(kubeconfig string) k8s.Interface {
		logAttempt("trying", kubeconfig)

		if !PathExists(kubeconfig) {
			ExitWithError(
				logger,
				fmt.Errorf("kubeconfig not found: %s", kubeconfig),
			)
			return nil
		}

		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			ExitWithError(logger, err)
			return nil
		}

		clientset, err := k8s.NewForConfig(config)
		if err != nil {
			ExitWithError(logger, err)
			return nil
		}

		logAttempt("success with", kubeconfig)
		return clientset
	}

	if kubeconfigPath == "" {
		return orDie(FindKubeconfig(logger))
	}
	return orDie(kubeconfigPath)
}
