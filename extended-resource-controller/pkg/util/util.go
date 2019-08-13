package util

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// DefaultInformerResyncPeriod is the resync period of informer
	DefaultInformerResyncPeriod = 5 * time.Second

	// DefaultMonitorResyncPeriod is the resync period
	DefaultResyncPeriod = 30 * time.Second

	// UpdateERRetryCount is the retry count of ER updating
	UpdateERRetryCount = 5

	// UpdateERInterval is the interval of ER updating
	UpdateERInterval = 5 * time.Millisecond

	// DefaultNodeNotReadyTimeDuration is the default time interval we need to consider node broken if it keeps NotReady
	DefaultNodeNotReadyTimeDuration = 120 * time.Second
)

var (
	kubeconfig, masterURL string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Paths to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server.")
}

// GetConfig creates a *rest.Config for talking to a Kubernetes apiserver.
func GetConfig() (*rest.Config, error) {
	// If a flag is specified with the config location, use that
	if len(kubeconfig) > 0 {
		return clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	}
	// If an env variable is specified with the config locaiton, use that
	if len(os.Getenv("KUBECONFIG")) > 0 {
		return clientcmd.BuildConfigFromFlags(masterURL, os.Getenv("KUBECONFIG"))
	}
	// If no explicit location, try the in-cluster config
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	// If no in-cluster config, try the default location in the user's home directory
	if usr, err := user.Current(); err == nil {
		if c, err := clientcmd.BuildConfigFromFlags(
			"", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not locate a kubeconfig")
}

func CreateClientset() kubernetes.Interface {
	c, err := GetConfig()
	if err != nil {
		glog.Fatalf("unable to get kubeconfig: %+v", err)
		return nil
	}
	return kubernetes.NewForConfigOrDie(c)
}

func WaitForCacheSync(controllerName string, stopCh <-chan struct{}, cacheSyncs ...cache.InformerSynced) bool {
	glog.Infof("Waiting for caches to sync for %s controller", controllerName)

	if !cache.WaitForCacheSync(stopCh, cacheSyncs...) {
		utilruntime.HandleError(fmt.Errorf("unable to sync caches for %s controller", controllerName))
		return false
	}

	glog.Infof("Caches are synced for %s controller", controllerName)
	return true
}
