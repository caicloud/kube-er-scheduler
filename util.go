package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

// ExtendedResourceScheduler is a set of methods that can find extendedresource and extendedresourceclaim
type ExtendedResourceScheduler struct {
	Clientset *kubernetes.Clientset
}

// FindExtendedResourceClaim find extendedresourceclaim by namespace and ercname
func (e *ExtendedResourceScheduler) FindExtendedResourceClaim(namespace, ercName string) (*v1alpha1.ExtendedResourceClaim, error) {
	erc, err := e.Clientset.ExtensionsV1alpha1().ExtendedResourceClaims(namespace).Get(ercName, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("not found extendedresourceclaim: %v", err)
		return nil, err
	}
	if len(erc.Spec.ExtendedResourceNames) == 0 && erc.Spec.ExtendedResourceNum == 0 {
		glog.Errorf("ExtendedResourceNames and ExtendedResourceNum are empty")
		return nil, errors.New("ExtendedResourceNames and ExtendedResourceNum are empty")
	}
	return erc, nil
}

// UpdateExtendedResourceClaim update extendedresourceclaim by namespace and erc
func (e *ExtendedResourceScheduler) UpdateExtendedResourceClaim(namespace string, erc *v1alpha1.ExtendedResourceClaim) error {
	_, err := e.Clientset.ExtensionsV1alpha1().ExtendedResourceClaims(namespace).Update(erc)
	return err
}

// FindExtendedResourceList get a set of ExtendedResource
func (e *ExtendedResourceScheduler) FindExtendedResourceList(erNames []string) ([]*v1alpha1.ExtendedResource, error) {
	extendedResources := make([]*v1alpha1.ExtendedResource, 0)
	for _, name := range erNames {
		extendedResource, err := e.FindExtendedResource(name)
		if err != nil {
			glog.Errorf("find extendedresource by ernames: %v", err)
			return nil, err
		}
		extendedResources = append(extendedResources, extendedResource)
	}
	return extendedResources, nil
}

// FindExtendedResource find extendedresource by ername
func (e *ExtendedResourceScheduler) FindExtendedResource(erName string) (*v1alpha1.ExtendedResource, error) {
	er, err := e.Clientset.ExtensionsV1alpha1().ExtendedResources().Get(erName, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("not found extendedresource by ername: %v", err)
		return nil, err
	}
	return er, nil
}

// UpdateExtendedResource update extendedresource
func (e *ExtendedResourceScheduler) UpdateExtendedResource(er *v1alpha1.ExtendedResource) error {
	_, err := e.Clientset.ExtensionsV1alpha1().ExtendedResources().Update(er)
	return err
}

// FindNode is get node
func (e *ExtendedResourceScheduler) findNode(name string, options metav1.GetOptions) (*v1.Node, error) {
	node, err := e.Clientset.CoreV1().Nodes().Get(name, options)
	if err != nil {
		glog.Errorf("find node failed: %v", err)
		return nil, err
	}
	return node, nil
}

// UpdateNodeStatus is used to update node status object
func (e *ExtendedResourceScheduler) updateNodeStatus(node *v1.Node) error {
	_, err := e.Clientset.CoreV1().Nodes().UpdateStatus(node)
	if err != nil {
		glog.Errorf("update node failed: %v", err)
		return err
	}
	return nil
}

// whether properties contain all labels, if contain all then return true, or return false
func mapInMap(labels, properties map[string]string) bool {
	if len(labels) == 0 {
		return true
	}
	for k, v := range labels {
		if vv, ok := properties[k]; ok && v == vv {
			return true
		}
		return false
	}
	return false
}

// target whether contain all s slice, if not, return exclusive value and false
func sliceInSlice(s, target []string) ([]string, bool) {
	re := make([]string, 0)
	targetStr := strings.Join(target, " ")
	for _, ele := range s {
		if !strings.Contains(targetStr, ele) {
			re = append(re, ele)
		}
	}
	if len(re) > 0 {
		return re, false
	}
	return nil, true
}

func labelMatchesLabelSelectorExpressions(matchExpressions []metav1.LabelSelectorRequirement, mLabels map[string]string) bool {
	if len(matchExpressions) == 0 {
		return true
	}
	labelSelector, err := labelSelectorRequirementsAsSelector(matchExpressions)
	if err != nil {
		glog.V(3).Infof("Failed to parse MatchExpressions: %+v, regarding as not match.", matchExpressions)
		return false
	}
	if labelSelector.Matches(labels.Set(mLabels)) {
		return true
	}
	return false
}

func labelSelectorRequirementsAsSelector(lsr []metav1.LabelSelectorRequirement) (labels.Selector, error) {
	if len(lsr) == 0 {
		return labels.Nothing(), nil
	}
	selector := labels.NewSelector()
	for _, expr := range lsr {
		var op selection.Operator
		switch expr.Operator {
		case metav1.LabelSelectorOpIn:
			op = selection.In
		case metav1.LabelSelectorOpNotIn:
			op = selection.NotIn
		case metav1.LabelSelectorOpExists:
			op = selection.Exists
		case metav1.LabelSelectorOpDoesNotExist:
			op = selection.DoesNotExist
		default:
			return nil, fmt.Errorf("%q is not a valid label selector operator", expr.Operator)
		}
		r, err := labels.NewRequirement(expr.Key, op, expr.Values)
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*r)
	}
	return selector, nil
}
