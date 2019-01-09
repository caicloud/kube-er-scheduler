package main

import (
	"flag"
	"math/rand"
	"time"

	"github.com/caicloud/kube-extended-resource/extended-resource-controller/pkg/controller"
	"github.com/caicloud/kube-extended-resource/extended-resource-controller/pkg/util"
	"github.com/golang/glog"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	flag.Parse()

	kubeClient := util.CreateClientset()

	c := controller.NewExtendedResourceController(kubeClient)

	glog.V(2).Infof("Extended Resource ExtendedResourceController Run.")

	c.Run()
}
