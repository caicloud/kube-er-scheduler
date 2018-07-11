package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api/v1"
)

// Bind delegates the action of binding a pod to a node.
func Bind(clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		body := io.TeeReader(r.Body, &buf)
		var extenderBindingArgs schedulerapi.ExtenderBindingArgs
		var extenderBindingResult *schedulerapi.ExtenderBindingResult
		if err := json.NewDecoder(body).Decode(&extenderBindingArgs); err != nil {
			extenderBindingResult = &schedulerapi.ExtenderBindingResult{
				Error: err.Error(),
			}
		} else {
			extendedResourceScheduler := &ExtendedResourceScheduler{
				Clientset: clientset,
			}
			extenderBindingResult = bind(extenderBindingArgs, extendedResourceScheduler)
		}

		w.Header().Set("Content-Type", "application/json")
		if resultBody, err := json.Marshal(extenderBindingResult); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write(resultBody)
		}
	}
}

func bind(extenderBindingArgs schedulerapi.ExtenderBindingArgs, extendedResourceScheduler *ExtendedResourceScheduler) *schedulerapi.ExtenderBindingResult {
	podName := extenderBindingArgs.PodName
	podNamespace := extenderBindingArgs.PodNamespace

	bindingResult := &schedulerapi.ExtenderBindingResult{}
	pod, err := extendedResourceScheduler.FindPod(podName, podNamespace)
	if err != nil {
		glog.Errorf("find pod error, podname: %s podnamespace: %s", podName, podNamespace)
		bindingResult.Error = err.Error()
		return bindingResult
	}

	extendedResourceClaims, err := extendedResourceScheduler.FindExtendedResourceClaimList(*pod)
	if err != nil {
		bindingResult.Error = err.Error()
		return bindingResult
	}

	// TODO: update extendedresource and extendedresourceclaim asynchronously
	for _, erc := range extendedResourceClaims {
		erc.Status.Phase = v1alpha1.ExtendedResourceClaimBound
		erc.Status.Reason = "ExtendedResourceClaim is already bound to ExtendedResource"
		err := extendedResourceScheduler.UpdateExtendedResourceClaim(pod.Namespace, erc)
		if err != nil {
			bindingResult.Error = err.Error()
			return bindingResult
		}
		extendedResources, err := extendedResourceScheduler.FindExtendedResourceList(erc.Spec.ExtendedResourceNames)
		if err != nil {
			bindingResult.Error = err.Error()
			return bindingResult
		}
		for _, er := range extendedResources {
			er.Spec.ExtendedResourceClaimName = erc.Name
			er.Status.Phase = v1alpha1.ExtendedResourceBound
			err := extendedResourceScheduler.UpdateExtendedResource(er)
			if err != nil {
				er.Status.Phase = v1alpha1.ExtendedResourceAvailable
				extendedResourceScheduler.UpdateExtendedResource(er)
				bindingResult.Error = err.Error()
				return bindingResult
			}
		}
	}

	b := &v1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: podNamespace,
			Name:      podName,
			UID:       extenderBindingArgs.PodUID,
		},
		Target: v1.ObjectReference{
			Kind: "Node",
			Name: extenderBindingArgs.Node,
		},
	}
	err = extendedResourceScheduler.Bind(podNamespace, b)
	if err != nil {
		bindingResult.Error = err.Error()
		return bindingResult
	}
	return bindingResult
}
