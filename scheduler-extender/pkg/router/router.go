package router

import (
	"net/http"

	"github.com/caicloud/kube-extended-resource/scheduler-extender/pkg/controller"
	"github.com/gin-gonic/gin"
	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
)

func Init(clientset *kubernetes.Clientset, mode bool) http.Handler {
	if mode {
		gin.SetMode(gin.ReleaseMode)
		glog.V(3).Info("Running release mode")
	} else {
		gin.SetMode(gin.DebugMode)
		glog.V(3).Info("Running debug mode")
	}

	glog.V(3).Info("Init router.")

	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	schedulerController := controller.NewSchedulerController(clientset)

	v1 := router.Group("/api/v1")
	{
		v1.POST("/scheduler/filter", schedulerController.Filter)
		v1.POST("/scheduler/bind", schedulerController.Bind)
	}

	return router
}
