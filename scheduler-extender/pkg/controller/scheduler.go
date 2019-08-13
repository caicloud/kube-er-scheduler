package controller

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/caicloud/kube-extended-resource/scheduler-extender/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api/v1"
)

const (
	// PodNameAnnotation specify the erc bind the name of pod.
	PodNameAnnotation = "extendedresourceclaim/podName"
	// ExtendedResourceClaimNamespaceAnnotation specify the er bind the erc namespace
	ExtendedResourceClaimNamespaceAnnotation = "extendedresourceclaim/namespace"
)

type Controller interface {
	Filter(c *gin.Context)

	Bind(c *gin.Context)
}

type schedulerController struct {
	Clientset *kubernetes.Clientset
}

func NewSchedulerController(clientset *kubernetes.Clientset) Controller {
	return &schedulerController{
		Clientset: clientset,
	}
}

func (sc *schedulerController) Filter(c *gin.Context) {
	var extenderArgs schedulerapi.ExtenderArgs
	if err := c.ShouldBindJSON(&extenderArgs); err != nil {
		c.JSON(http.StatusBadRequest, &schedulerapi.ExtenderFilterResult{Error: err.Error()})
		return
	}

	if extenderArgs.Nodes == nil {
		c.JSON(http.StatusOK, &schedulerapi.ExtenderFilterResult{})
		return
	}

	pod := extenderArgs.Pod
	nodes := extenderArgs.Nodes.Items
	canScheduler := make([]v1.Node, 0)
	canNotSchedule := make(map[string]string)

	// default all node scheduling failed
	defaultNotSchedule := defaultFailedNodes(nodes)

	extendedResourceClaims, err := sc.ListExtendedResourceClaims(pod)
	if err != nil {
		glog.Warningf("Cannot list ExtendedResourceClaim: %+v", err)
		c.JSON(http.StatusOK, &schedulerapi.ExtenderFilterResult{
			Nodes: &v1.NodeList{
				Items: canScheduler,
			},
			FailedNodes: defaultNotSchedule,
			Error:       err.Error(),
		})
		return
	}

	if len(extendedResourceClaims) == 0 {
		glog.V(4).Infof("Pod have no extendedResourceClaims field.")
		c.JSON(http.StatusOK, &schedulerapi.ExtenderFilterResult{
			Nodes: &v1.NodeList{
				Items: nodes,
			},
			FailedNodes: canNotSchedule,
			Error:       "",
		})
		return
	}

	for _, claim := range extendedResourceClaims {
		if claim.Status.Phase == v1alpha1.ExtendedResourceClaimBound {
			c.JSON(http.StatusOK, &schedulerapi.ExtenderFilterResult{
				Nodes: &v1.NodeList{
					Items: nodes,
				},
				FailedNodes: canNotSchedule,
				Error:       fmt.Sprintf("ExtendedResourceClaim %s is bound", claim.Name),
			})
			return
		}
	}

	var extendedResourceNames = make([]string, 0)
	for _, erc := range extendedResourceClaims {
		extendedResourceNames = append(extendedResourceNames, erc.Spec.ExtendedResourceNames...)
	}

	glog.V(3).Infof("Pod need ExtendedResource: %+v", extendedResourceNames)

	var filterResultLock sync.Mutex
	checkNode := func(i int) {
		node := nodes[i]
		nodeName := node.Name
		extendedResourceAllocatable := node.Status.ExtendedResourceAllocatable

		if len(extendedResourceAllocatable) < len(extendedResourceNames) {
			msg := "extended resources that can be allocated on this node are less than pod needs"
			glog.Error(msg)
			filterResultLock.Lock()
			canNotSchedule[nodeName] = msg
			filterResultLock.Unlock()
			return
		}

		if sub, ok := utils.SliceInSlice(extendedResourceNames, extendedResourceAllocatable); !ok {
			msg := fmt.Sprintf("there are no such [%s] extended resource", strings.Join(sub, " "))
			glog.Errorf(msg)
			filterResultLock.Lock()
			canNotSchedule[nodeName] = msg
			filterResultLock.Unlock()
			return
		}

		extendedResources, err := sc.ListExtendedResource(extendedResourceAllocatable)
		if err != nil {
			filterResultLock.Lock()
			glog.Warningf("Cannot list ExtendedResource: %v", err)
			canNotSchedule[nodeName] = err.Error()
			filterResultLock.Unlock()
			return
		}

		// filter out the ExtendedResource specified in ExtendedResourceClaim and ExtendedResource status is not available
		if len(extendedResourceNames) > 0 {
			scheduled := false
			extendedResourcesCopy := make([]*v1alpha1.ExtendedResource, 0)
		loop:
			for index, extendedResource := range extendedResources {
				for _, name := range extendedResourceNames {
					if name != extendedResource.Name {
						extendedResourcesCopy = append(extendedResourcesCopy, extendedResources[index])
						continue
					}
					if extendedResource.Status.Phase != v1alpha1.ExtendedResourceAvailable {
						scheduled = true
						break loop
					}
				}
			}

			if scheduled {
				msg := "there are unavailable extended resources in extended resource claim"
				glog.Error(msg)
				filterResultLock.Lock()
				canNotSchedule[nodeName] = msg
				filterResultLock.Unlock()
				return
			}
			extendedResources = extendedResourcesCopy
		}

		glog.V(4).Infof("extendedresource: %+v", len(extendedResources))

		for _, extendedResourceClaim := range extendedResourceClaims {
			extendedResourceNames := extendedResourceClaim.Spec.ExtendedResourceNames
			extendedResourceNum := extendedResourceClaim.Spec.ExtendedResourceNum
			requirements := extendedResourceClaim.Spec.MetadataRequirements

			if extendedResourceNum == 0 {
				extendedResourceClaim.Spec.ExtendedResourceNum = int64(len(extendedResourceNames))
				extendedResourceClaim.Status.Phase = v1alpha1.ExtendedResourceClaimPending
				extendedResourceClaim.Status.Reason = "extended resource have been satisfied and waiting to bound"
				continue
			}

			glog.Infof("extendedResourceClaim name: %s", extendedResourceClaim.Name)

			for index, extendedResource := range extendedResources {

				glog.V(4).Infof("resource: %+v", extendedResource)

				if extendedResourceClaim.Spec.RawResourceName != extendedResource.Spec.RawResourceName {
					continue
				}

				prop := extendedResource.Spec.Properties
				if utils.Match(requirements, prop) {
					if int64(len(extendedResourceNames)) == extendedResourceNum {
						break
					}
					extendedResourceNames = append(extendedResourceNames, extendedResource.Name)
					extendedResources = append(extendedResources[:index], extendedResources[index+1:]...)
				}
			}

			if int64(len(extendedResourceNames)) < extendedResourceNum {
				msg := fmt.Sprintf("extended resource that can be allocated are not satisfy [%s] needs", extendedResourceClaim.Name)
				glog.Errorf(msg)
				filterResultLock.Lock()
				canNotSchedule[nodeName] = msg
				filterResultLock.Unlock()
				return
			}
			extendedResourceClaim.Spec.ExtendedResourceNames = extendedResourceNames
			extendedResourceClaim.Status.Phase = v1alpha1.ExtendedResourceClaimPending
			extendedResourceClaim.Status.Reason = "extended resource have been satisfied and waiting to bound"
		}

		flag := false
		for _, erc := range extendedResourceClaims {
			if erc.Status.Phase != v1alpha1.ExtendedResourceClaimPending {
				flag = true
				break
			}
		}

		if !flag {
			filterResultLock.Lock()
			canScheduler = append(canScheduler, node)
			filterResultLock.Unlock()
		} else {
			glog.Error("node can allocate extended resource are not satisfy pod needs")
			filterResultLock.Lock()
			canNotSchedule[nodeName] = "node can allocate extended resource are not satisfy pod needs"
			filterResultLock.Unlock()
			return
		}
	}

	workqueue.Parallelize(16, len(nodes), checkNode)

	// pod can be scheduled, so erc need to update, erc is pending
	if len(canScheduler) > 0 {
		for _, erc := range extendedResourceClaims {
			sc.UpdateExtendedResourceClaim(pod.Namespace, erc)
		}
	}

	c.JSON(http.StatusOK, &schedulerapi.ExtenderFilterResult{
		Nodes: &v1.NodeList{
			Items: canScheduler,
		},
		FailedNodes: canNotSchedule,
		Error:       "",
	})
	return
}

func (sc *schedulerController) Bind(c *gin.Context) {
	var extenderBindingArgs schedulerapi.ExtenderBindingArgs
	if err := c.ShouldBindJSON(&extenderBindingArgs); err != nil {
		glog.Errorf("Bind ExtenderBindingArgs: %+v", err)
		c.JSON(http.StatusBadRequest, &schedulerapi.ExtenderBindingResult{Error: err.Error()})
		return
	}

	glog.V(3).Infof("Bind handler: %+v", extenderBindingArgs)

	podName := extenderBindingArgs.PodName
	podNamespace := extenderBindingArgs.PodNamespace

	pod, err := sc.GetPod(podNamespace, podName)
	if err != nil {
		glog.Errorf("Get pod: %+v", err)
		c.JSON(http.StatusOK, &schedulerapi.ExtenderBindingResult{Error: err.Error()})
		return
	}

	extendedResourceClaims, err := sc.ListExtendedResourceClaims(*pod)
	if err != nil {
		glog.Errorf("Cannot list ExtendedResourceClaim: %+v", err)
		c.JSON(http.StatusOK, &schedulerapi.ExtenderBindingResult{Error: err.Error()})
		return
	}

	for _, erc := range extendedResourceClaims {
		if len(erc.Annotations) == 0 {
			erc.Annotations = make(map[string]string)
		}
		erc.Annotations[PodNameAnnotation] = podName

		erc.Status.Phase = v1alpha1.ExtendedResourceClaimBound
		erc.Status.Reason = "ExtendedResourceClaim has already bound to ExtendedResource"

		_ = sc.UpdateExtendedResourceClaim(podNamespace, erc)

		extendedResources, _ := sc.ListExtendedResource(erc.Spec.ExtendedResourceNames)

		for _, er := range extendedResources {
			if len(er.Annotations) == 0 {
				er.Annotations = make(map[string]string)
			}
			er.Annotations[ExtendedResourceClaimNamespaceAnnotation] = erc.Namespace
			er.Spec.ExtendedResourceClaimName = erc.Name
			er.Status.Phase = v1alpha1.ExtendedResourceBound
			_ = sc.UpdateExtendedResource(er)
		}
	}

	err = sc.BindNode(podNamespace, &v1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: podNamespace,
			Name:      podName,
			UID:       extenderBindingArgs.PodUID,
		},
		Target: v1.ObjectReference{
			Kind: "Node",
			Name: extenderBindingArgs.Node,
		},
	})

	if err != nil {
		glog.Warningf("Bind node: %+v", err)
		c.JSON(http.StatusOK, &schedulerapi.ExtenderBindingResult{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, &schedulerapi.ExtenderBindingResult{})
	return
}

func (sc *schedulerController) ListExtendedResourceClaims(pod v1.Pod) ([]*v1alpha1.ExtendedResourceClaim, error) {
	extendedResourceClaimNames := make([]string, 0)
	for _, container := range pod.Spec.Containers {
		if len(container.ExtendedResourceClaims) != 0 {
			extendedResourceClaimNames = append(extendedResourceClaimNames, container.ExtendedResourceClaims...)
		}
	}
	extendedResourceClaims := make([]*v1alpha1.ExtendedResourceClaim, 0)
	for _, ercName := range extendedResourceClaimNames {
		erc, err := sc.Clientset.ExtensionsV1alpha1().ExtendedResourceClaims(pod.Namespace).Get(ercName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		extendedResourceClaims = append(extendedResourceClaims, erc)
	}
	return extendedResourceClaims, nil
}

func (sc *schedulerController) ListExtendedResource(names []string) ([]*v1alpha1.ExtendedResource, error) {
	extendedResources := make([]*v1alpha1.ExtendedResource, 0)
	for _, name := range names {
		extendedResource, err := sc.Clientset.ExtensionsV1alpha1().ExtendedResources().Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		extendedResources = append(extendedResources, extendedResource)
	}
	return extendedResources, nil
}

func (sc *schedulerController) UpdateExtendedResource(extendedResource *v1alpha1.ExtendedResource) error {
	_, err := sc.Clientset.ExtensionsV1alpha1().ExtendedResources().Update(extendedResource)
	return err
}

func (sc *schedulerController) UpdateExtendedResourceClaim(namespace string, claim *v1alpha1.ExtendedResourceClaim) error {
	_, err := sc.Clientset.ExtensionsV1alpha1().ExtendedResourceClaims(namespace).Update(claim)
	return err
}

func (sc *schedulerController) GetPod(namespace, name string) (*v1.Pod, error) {
	return sc.Clientset.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
}

func (sc *schedulerController) GetNode(name string) (*v1.Node, error) {
	return sc.Clientset.CoreV1().Nodes().Get(name, metav1.GetOptions{})
}

func (sc *schedulerController) BindNode(namespace string, binding *v1.Binding) error {
	return sc.Clientset.CoreV1().Pods(namespace).Bind(binding)
}

// default set all node is fail
func defaultFailedNodes(nodes []v1.Node) map[string]string {
	canNotSchedule := make(map[string]string)
	for _, node := range nodes {
		canNotSchedule[node.ObjectMeta.Name] = ""
	}
	return canNotSchedule
}
