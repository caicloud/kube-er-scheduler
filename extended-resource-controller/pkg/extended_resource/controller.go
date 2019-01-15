package extended_resource

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	extensions_lister "k8s.io/client-go/listers/extensions/v1alpha1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// PodNameAnnotation specify the erc bind the name of pod.
	PodNameAnnotation = "extendedresourceclaim/podName"
	// ExtendedResourceClaimNamespaceAnnotation specify the er bind the erc namespace
	ExtendedResourceClaimNamespaceAnnotation = "extendedresourceclaim/namespace"
)

// ExtendedResourceController defines how to unbind the erc and er.
type ExtendedResourceController struct {
	stopCh <-chan struct{}
	client clientset.Interface

	ercLister extensions_lister.ExtendedResourceClaimLister
	erLister  extensions_lister.ExtendedResourceLister

	ercInformerSynced cache.InformerSynced
	erInformerSynced  cache.InformerSynced

	claimQueue    *workqueue.Type
	resourceQueue *workqueue.Type
}

func NewExtendedResourceController(kubeClient clientset.Interface) *ExtendedResourceController {
	informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second*60)
	ercInformer := informerFactory.Extensions().V1alpha1().ExtendedResourceClaims()
	erInformer := informerFactory.Extensions().V1alpha1().ExtendedResources()

	claimQueue := workqueue.NewNamed("extended-resource-claim")
	resourceQueue := workqueue.NewNamed("extended-resource")

	c := &ExtendedResourceController{
		client:        kubeClient,
		stopCh:        make(<-chan struct{}),
		claimQueue:    claimQueue,
		resourceQueue: resourceQueue,
	}

	go informerFactory.Start(c.stopCh)

	ercInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueWork(c.claimQueue, obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			c.enqueueWork(c.claimQueue, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueWork(c.claimQueue, obj)
		},
	})

	erInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueWork(c.resourceQueue, obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.enqueueWork(c.resourceQueue, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueWork(c.resourceQueue, obj)
		},
	})

	c.ercLister = ercInformer.Lister()
	c.ercInformerSynced = ercInformer.Informer().HasSynced
	c.erLister = erInformer.Lister()
	c.erInformerSynced = erInformer.Informer().HasSynced
	return c
}

func (c *ExtendedResourceController) Run() {
	defer utilruntime.HandleCrash()
	defer c.claimQueue.ShutDown()
	defer c.resourceQueue.ShutDown()

	glog.Infof("Starting extended resource controller")
	defer glog.Infof("Shutting down extended resource controller")

	if !WaitForCacheSync("extended-resource", c.stopCh, c.ercInformerSynced, c.erInformerSynced) {
		return
	}

	go wait.Until(c.syncClaim, time.Second, c.stopCh)
	go wait.Until(c.syncResource, time.Second, c.stopCh)
	<-c.stopCh
}

func (c *ExtendedResourceController) syncClaim() {
	workFunc := func() bool {
		keyObj, quit := c.claimQueue.Get()
		if quit {
			return true
		}
		defer c.claimQueue.Done(keyObj)
		key := keyObj.(string)
		glog.V(5).Infof("ERCWorker[%s]", key)

		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			glog.V(4).Infof("error getting namespace & name of claim %q to get claim from informer: %v", key, err)
			return false
		}

		erc, err := c.ercLister.ExtendedResourceClaims(namespace).Get(name)
		if err != nil {
			return false
		}

		glog.V(2).Infof("Found ExtendedResourceClaim %s/%s", erc.Namespace, erc.Name)

		extendedResourceClaim := erc.DeepCopy()
		// update extendedresourceclaim status and unbind pod
		podName, ok := extendedResourceClaim.Annotations[PodNameAnnotation]
		if ok {
			pod, err := c.client.CoreV1().Pods(extendedResourceClaim.Namespace).Get(podName, metav1.GetOptions{})
			if errors.IsNotFound(err) || pod == nil {
				_ = c.updateExtendedResources(extendedResourceClaim.Spec.ExtendedResourceNames)
				err := c.updateClaim(extendedResourceClaim)
				if err != nil {
					glog.Errorf("Update ExtendedResourceClaim error: %+v", err)
					return false
				}
			}
			if err != nil {
				glog.Errorf("Found pod failed: %s/%s", extendedResourceClaim.Namespace, podName)
				return false
			}
		}

		if !ok && extendedResourceClaim.Status.Phase == v1alpha1.ExtendedResourceClaimBound {
			err := c.updateClaim(extendedResourceClaim)
			if err != nil {
				glog.Errorf("Update ExtendedResourceClaim error: %+v", err)
				return false
			}
		}

		exists := false
		resourceName := ""
		for _, name := range extendedResourceClaim.Spec.ExtendedResourceNames {
			er, err := c.erLister.Get(name)
			if errors.IsNotFound(err) || er == nil {
				exists = true
				resourceName = name
				break
			}

			if er.Status.Phase == v1alpha1.ExtendedResourcePending {
				exists = true
				resourceName = name
				break
			}
			// rebind between ExtendedResourceClaim and ExtendedResource
			if extendedResourceClaim.Status.Phase == v1alpha1.ExtendedResourceClaimBound &&
				er.Status.Phase == v1alpha1.ExtendedResourceAvailable {
				erCopy := er.DeepCopy()
				erCopy.Status.Phase = v1alpha1.ExtendedResourceBound
				erCopy.Spec.ExtendedResourceClaimName = extendedResourceClaim.Name
				_, err := c.client.ExtensionsV1alpha1().ExtendedResources().Update(erCopy)
				if err != nil {
					glog.Errorf("update ExtendedResource status: %+v", err)
				}
			}
		}

		// update extendedresourceclaim status to lost if extendedresource is not found in cluster
		if exists {
			extendedResourceClaim.Status.Phase = v1alpha1.ExtendedResourceClaimLost
			extendedResourceClaim.Status.Reason = fmt.Sprintf("ExtendedResource %s not found in cluster.", resourceName)
		}

		if !exists && extendedResourceClaim.Status.Phase == v1alpha1.ExtendedResourceClaimLost {
			extendedResourceClaim.Status.Phase = ""
			extendedResourceClaim.Status.Reason = ""
		}
		_, err = c.client.ExtensionsV1alpha1().ExtendedResourceClaims(extendedResourceClaim.Namespace).Update(extendedResourceClaim)
		if err != nil {
			glog.Errorf("Update ExtendedResourceClaim status: %+v", err)
			return false
		}

		return false
	}

	for {
		if quit := workFunc(); quit {
			glog.Infof("extended resource claim worker queue shutting down")
			return
		}
	}
}

func (c *ExtendedResourceController) syncResource() {
	workFunc := func() bool {
		keyObj, quit := c.resourceQueue.Get()
		if quit {
			return true
		}
		defer c.resourceQueue.Done(keyObj)
		key := keyObj.(string)
		glog.V(5).Infof("ERWorker[%s]", key)

		_, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			glog.V(4).Infof("error getting name of extended resource %q to get from informer: %v", key, err)
			return false
		}

		extendedResource, err := c.erLister.Get(name)
		if err != nil {
			return false
		}

		glog.V(2).Infof("Found ExtendedResource %s", extendedResource.Name)

		ercName := extendedResource.Spec.ExtendedResourceClaimName
		namespace, ok := extendedResource.Annotations[ExtendedResourceClaimNamespaceAnnotation]
		if ercName != "" && ok {
			erc, err := c.client.ExtensionsV1alpha1().ExtendedResourceClaims(namespace).Get(ercName, metav1.GetOptions{})
			if errors.IsNotFound(err) || erc == nil {
				err = c.updateER(extendedResource)
				glog.Warningf("Update ExtendedResource error: %+v", err)
				return false
			}
			if err != nil {
				glog.Warningf("Find ExtendedResourceClaim error: %+v", err)
				return false
			}
		}
		return false
	}

	for {
		if quit := workFunc(); quit {
			glog.Infof("extended resource worker queue shutting down")
			return
		}
	}
}

func (c *ExtendedResourceController) updateClaim(claim *v1alpha1.ExtendedResourceClaim) error {
	claim.Status.Phase = ""
	claim.Status.Message = ""
	claim.Status.Reason = ""
	claim.Spec.ExtendedResourceNames = make([]string, 0)
	delete(claim.Annotations, PodNameAnnotation)
	_, err := c.client.ExtensionsV1alpha1().ExtendedResourceClaims(claim.Namespace).Update(claim)
	if err != nil {
		return err
	}
	return nil
}

func (c *ExtendedResourceController) updateExtendedResources(extendedResourceNames []string) error {
	for _, name := range extendedResourceNames {
		extendedResource, err := c.erLister.Get(name)
		if err != nil {
			continue
		}

		err = c.updateER(extendedResource)
		if err != nil {
			continue
		}
	}
	return nil
}

func (c *ExtendedResourceController) updateER(extendedResource *v1alpha1.ExtendedResource) error {
	extendedResource.Spec.ExtendedResourceClaimName = ""
	extendedResource.Status.Phase = v1alpha1.ExtendedResourceAvailable
	extendedResource.Status.Reason = ""
	delete(extendedResource.Annotations, ExtendedResourceClaimNamespaceAnnotation)
	_, err := c.client.ExtensionsV1alpha1().ExtendedResources().Update(extendedResource)
	if err != nil {
		return err
	}
	return nil
}

// enqueueWork adds volume or claim to given work queue.
func (c *ExtendedResourceController) enqueueWork(queue workqueue.Interface, obj interface{}) {
	// Beware of "xxx deleted" events
	if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
		obj = unknown.Obj
	}
	objName, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("failed to get key from object: %v", err)
		return
	}
	glog.V(5).Infof("enqueued %q for sync", objName)
	queue.Add(objName)
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
