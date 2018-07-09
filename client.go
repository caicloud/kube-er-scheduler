package main

import (
	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// CreateClientset is create a kubernetes client
func CreateClientset(master, kubeConfig *string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags(*master, *kubeConfig)
	if err != nil {
		glog.Errorf("unable to build config: %v", err)
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Errorf("unable to create clientset from config: %v", err)
		return nil, err
	}
	return clientset, nil
}
