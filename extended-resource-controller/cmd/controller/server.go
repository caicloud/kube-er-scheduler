package main

import (
	"github.com/caicloud/kube-extended-resource/extended-resource-controller/pkg/extended_resource"
	"github.com/caicloud/kube-extended-resource/extended-resource-controller/pkg/node_failure"
	"github.com/caicloud/kube-extended-resource/extended-resource-controller/pkg/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
)

func NewServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "extended_resource_controller",
		Long: `The controller synchronize the ExtendedResourceClaim and ExtendedResource.`,
		Run: func(cmd *cobra.Command, args []string) {
			Run(wait.NeverStop)
		},
	}

	return cmd
}

func Run(stopCh <-chan struct{}) {
	go runExtendedResourceController()
	go runNodeWatcher()
	<-stopCh
}

func runExtendedResourceController() {
	kubeClient := util.CreateClientset()
	glog.V(2).Infof("Extended Resource ExtendedResourceController Run.")
	extended_resource.NewExtendedResourceController(kubeClient).Run(wait.NeverStop)
}

func runNodeWatcher() {
	kubeClient := util.CreateClientset()
	glog.V(2).Infof("NodeWatcher Run.")
	node_failure.NewNodeWatcher(kubeClient).Run(wait.NeverStop)
}
