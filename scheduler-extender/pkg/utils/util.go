package utils

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// Match return true if LabelSelector match properties
func Match(requirement metav1.LabelSelector, properties map[string]string) bool {
	return MapInMap(requirement.MatchLabels, properties) ||
		LabelMatchesLabelSelectorExpressions(requirement.MatchExpressions, properties)
}

// whether properties contain all labels, if contain all then return true, or false
func MapInMap(labels, properties map[string]string) bool {
	if len(labels) == 0 {
		return true
	}

	flag := false
	for k, v := range labels {
		if _, ok := properties[k]; !ok {
			flag = true
			break
		}
		if vv, ok := properties[k]; ok && vv != v {
			flag = true
			break
		}
	}
	return !flag
}

// target whether contain all sub slice, if not, return exclusive value and false
func SliceInSlice(sub, target []string) ([]string, bool) {
	re := make([]string, 0)
	targetStr := strings.Join(target, " ")
	for _, ele := range sub {
		if !strings.Contains(targetStr, ele) {
			re = append(re, ele)
		}
	}
	if len(re) > 0 {
		return re, false
	}
	return nil, true
}

func LabelMatchesLabelSelectorExpressions(matchExpressions []metav1.LabelSelectorRequirement, mLabels map[string]string) bool {
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
