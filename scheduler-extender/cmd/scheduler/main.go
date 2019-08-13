package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path/filepath"

	"github.com/caicloud/kube-extended-resource/scheduler-extender/pkg/router"
	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig, masterURL string
	port                  int
	mode                  bool
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Paths to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server.")
	flag.IntVar(&port, "port", 9098, "Scheduler extender listen on port. Default 9098.")
	flag.BoolVar(&mode, "mode", true, "Setting gin running mode. True is release and false is dev.")
}

func main() {
	flag.Parse()

	clientset := createClientset()
	handler := router.Init(clientset, mode)

	s := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}

	glog.V(2).Infof("Server listening on %d", port)
	if err := s.ListenAndServe(); err != nil {
		glog.Fatalf("Start scheduler extender: %+v", err)
	}
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

func createClientset() *kubernetes.Clientset {
	c, err := GetConfig()
	if err != nil {
		glog.Fatalf("unable to get kubeconfig: %+v", err)
		return nil
	}
	return kubernetes.NewForConfigOrDie(c)
}
