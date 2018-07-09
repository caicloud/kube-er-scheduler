package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"k8s.io/client-go/util/workqueue"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1alpha1"
	"k8s.io/client-go/kubernetes"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api/v1"
)

// Predicates implemented filter functions.
// The filter list is expected to be a subset of the supplied list.
func Predicates(clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		body := io.TeeReader(r.Body, &buf)
		glog.V(2).Infof("body: %s", buf.String())

		var extenderArgs schedulerapi.ExtenderArgs
		var extenderFilterResult *schedulerapi.ExtenderFilterResult

		if err := json.NewDecoder(body).Decode(&extenderArgs); err != nil {
			extenderFilterResult = &schedulerapi.ExtenderFilterResult{
				Nodes:       nil,
				FailedNodes: nil,
				Error:       err.Error(),
			}
		} else {
			extendedResourceScheduler := &ExtendedResourceScheduler{
				Clientset: clientset,
			}
			extenderFilterResult = filter(extenderArgs, extendedResourceScheduler)
		}

		w.Header().Set("Content-Type", "application/json")
		if resultBody, err := json.Marshal(extenderFilterResult); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write(resultBody)
		}
	}
}

func filter(extenderArgs schedulerapi.ExtenderArgs, extendedResourceScheduler *ExtendedResourceScheduler) *schedulerapi.ExtenderFilterResult {
	pod := extenderArgs.Pod
	nodes := extenderArgs.Nodes.Items

	canSchedule := make([]v1.Node, 0)
	canNotSchedule := make(map[string]string)
	// default all node scheduling failed
	defaultNotSchedule := defaultFailedNodes(nodes)

	result := &schedulerapi.ExtenderFilterResult{
		Nodes: &v1.NodeList{
			Items: canSchedule,
		},
		FailedNodes: defaultNotSchedule,
		Error:       "",
	}

	extendedResourceClaims, err := findExtendedResourceClaims(pod, extendedResourceScheduler)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	// if pod not declare extended resource claim, return all node can be scheduled
	if len(extendedResourceClaims) == 0 {
		result.Nodes.Items = nodes
		result.FailedNodes = canNotSchedule
		return result
	}

	// calculate how much extendedResource are needed for pod
	// TODO: Check whether the user's declared rawResourceName is the same as the declared rawResourceName of extended resource
	var extendedResourceNames = make([]string, 0)
	for _, erc := range extendedResourceClaims {
		extendedResourceNames = append(extendedResourceNames, erc.Spec.ExtendedResourceNames...)
	}

	glog.V(2).Info("start to filter node")

	var filterResultLock sync.Mutex
	checkNode := func(i int) {
		node := nodes[i]
		nodeName := node.Name
		extendedResourceAllocatable := node.Status.ExtendedResourceAllocatable
		if len(extendedResourceAllocatable) < len(extendedResourceNames) {
			filterResultLock.Lock()
			canNotSchedule[nodeName] = "extended resources that can be allocated on this node are less than pod needs"
			filterResultLock.Unlock()
			return
		}

		if ss, b := sliceInSlice(extendedResourceNames, extendedResourceAllocatable); !b {
			filterResultLock.Lock()
			canNotSchedule[nodeName] = fmt.Sprintf("there are no such [%s] extended resource", strings.Join(ss, " "))
			filterResultLock.Unlock()
			return
		}

		extendedResources, err := extendedResourceScheduler.FindExtendedResourceList(extendedResourceAllocatable)
		if err != nil {
			filterResultLock.Lock()
			glog.Errorf("find extended resource error: %v", err)
			canNotSchedule[nodeName] = err.Error()
			filterResultLock.Unlock()
			return
		}

		// filter out the er specified in erc and er status is not available
		scheduled := false
		extendedResourcePendings := make([]*v1alpha1.ExtendedResource, 0)
		if len(extendedResourceNames) > 0 {
		loop:
			for i := 0; i < len(extendedResources); i++ {
				for _, name := range extendedResourceNames {
					if name != extendedResources[i].Name {
						continue
					}
					if extendedResources[i].Status.Phase != v1alpha1.ExtendedResourceAvailable {
						scheduled = true
						break loop
					}
					extendedResources[i].Status.Phase = v1alpha1.ExtendedResourcePending
					extendedResourcePendings = append(extendedResourcePendings, extendedResources[i])
					extendedResources = append(extendedResources[:i], extendedResources[i+1:]...)
				}
			}
			if scheduled {
				filterResultLock.Lock()
				canNotSchedule[nodeName] = "there are unavailable extended resources in extended resource claim"
				filterResultLock.Unlock()
				return
			}
		}

		for _, erc := range extendedResourceClaims {
			extendedResourceNames := erc.Spec.ExtendedResourceNames
			extendedResourceNum := erc.Spec.ExtendedResourceNum
			requirements := erc.Spec.MetadataRequirements

			for i := 0; i < len(extendedResources); i++ {
				er := extendedResources[i]
				prop := er.Spec.Properties
				if int64(len(extendedResourceNames)) < extendedResourceNum &&
					erc.Spec.RawResourceName == er.Spec.RawResourceName &&
					(mapInMap(requirements.MatchLabels, prop) ||
						labelMatchesLabelSelectorExpressions(requirements.MatchExpressions, prop)) {
					extendedResourceNames = append(extendedResourceNames, er.Name)
					er.Spec.ExtendedResourceClaimName = erc.Name
					er.Status.Phase = v1alpha1.ExtendedResourcePending
					extendedResourcePendings = append(extendedResourcePendings, er)
					extendedResources = append(extendedResources[:i], extendedResources[i+1:]...)
				}
			}
			if extendedResourceNum != 0 && int64(len(extendedResourceNames)) < extendedResourceNum {
				filterResultLock.Lock()
				canNotSchedule[nodeName] = fmt.Sprintf("extended resource that can be allocated are not satisfy [%s] needs", erc.Name)
				filterResultLock.Unlock()
				return
			}
			erc.Spec.ExtendedResourceNames = extendedResourceNames
			erc.Status.Phase = v1alpha1.ExtendedResourceClaimPending
			erc.Status.Reason = "extended resource have been satisfied and waiting to bound"
		}

		for _, erc := range extendedResourceClaims {
			if erc.Status.Phase != v1alpha1.ExtendedResourceClaimPending {
				scheduled = true
				break
			}
		}
		if !scheduled {
			for _, er := range extendedResourcePendings {
				extendedResourceScheduler.UpdateExtendedResource(er)
			}
			filterResultLock.Lock()
			canSchedule = append(canSchedule, node)
			filterResultLock.Unlock()
		} else {
			filterResultLock.Lock()
			canNotSchedule[nodeName] = "node can allocate extended resource are not satisfy pod needs"
			filterResultLock.Unlock()
			return
		}
	}

	workqueue.Parallelize(16, len(nodes), checkNode)

	// pod can be scheduled, so erc need to update, erc is pending
	if len(canSchedule) > 0 {
		for _, erc := range extendedResourceClaims {
			erc.Status.Phase = v1alpha1.ExtendedResourceClaimBound
			erc.Status.Reason = "ExtendedResourceClaim is already bound to ExtendedResource"
			extendedResourceScheduler.UpdateExtendedResourceClaim(pod.Namespace, erc)
		}
	}
	result.FailedNodes = canNotSchedule
	result.Nodes.Items = canSchedule
	return result
}

func findExtendedResourceClaims(pod v1.Pod, extendedResourceScheduler *ExtendedResourceScheduler) ([]*v1alpha1.ExtendedResourceClaim, error) {
	extendedResourceClaimNames := make([]string, 0)
	for _, container := range pod.Spec.Containers {
		if len(container.ExtendedResourceClaims) != 0 {
			extendedResourceClaimNames = append(extendedResourceClaimNames, container.ExtendedResourceClaims...)
		}
	}
	if len(extendedResourceClaimNames) == 0 {
		return nil, errors.New("extendedresourceclaims not set")
	}
	extendedResourceClaims := make([]*v1alpha1.ExtendedResourceClaim, 0)
	for _, ercName := range extendedResourceClaimNames {
		erc, err := extendedResourceScheduler.FindExtendedResourceClaim(pod.Namespace, ercName)
		if err != nil {
			return nil, err
		}
		extendedResourceClaims = append(extendedResourceClaims, erc)
	}
	return extendedResourceClaims, nil
}

// default set all node is fail
func defaultFailedNodes(nodes []v1.Node) map[string]string {
	canNotSchedule := make(map[string]string)
	for _, node := range nodes {
		canNotSchedule[node.ObjectMeta.Name] = ""
	}
	return canNotSchedule
}
