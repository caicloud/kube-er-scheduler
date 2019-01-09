/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtendedResourceClaim is used by users to ask for extended resources
type ExtendedResourceClaim struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec is the desired state of the ExtendedResourceClaim.
	Spec ExtendedResourceClaimSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`

	// Status is the current state of the ExtendedResourceClaim.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status
	// +optional
	Status ExtendedResourceClaimStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtendedResourceClaimList is a collection of ExtendedResourceClaim.
type ExtendedResourceClaimList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is the list of ExtendedResourceClaim.
	Items []ExtendedResourceClaim `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ExtendedResourceClaimSpec describes the ExtendedResourceClaim the user wishes to exist.
type ExtendedResourceClaimSpec struct {
	// defines general resource property matching constraints.
	// e.g.: zone in { us-west1-b, us-west1-c }; type: k80
	// +optional
	MetadataRequirements metav1.LabelSelector `json:"metadataRequirements,omitempty" protobuf:"bytes,1,opt,name=metadataRequirements"`

	// ExtendedResourceNames are the names of ExtendedResources
	// +optional
	ExtendedResourceNames []string `json:"extendedResourceNames,omitempty" protobuf:"bytes,2,opt,name=extendedResourceNames"`

	// raw extended resource name, such as nvidia.com/gpu
	// used for batch resources request
	RawResourceName string `json:"rawResourceName" protobuf:"bytes,3,opt,name=rawResourceName"`

	// number of extended resource, for example: request 8 nvidia.com/gpu at one time
	// +optional
	ExtendedResourceNum int64 `json:"extendResourceNum,omitempty" protobuf:"varint,4,opt,name=extendResourceNum"`
}

// ExtendedResourceClaimStatus is the status of ExtendedResourceClaim
type ExtendedResourceClaimStatus struct {
	// Phase indicates if the Extended Resource Claim is Lost, bound or pending
	// +optional
	Phase ExtendedResourceClaimPhase `json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase"`
	// A human-readable message indicating details about why CRC is in this phase
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,2,opt,name=message"`
	// Brief string that describes any failure, used for CLI etc
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,3,opt,name=reason"`
}

type ExtendedResourceClaimPhase string

const (
	// used for ExtendedResourceClaim that lost its underlying ExtendedResource
	ExtendedResourceClaimLost ExtendedResourceClaimPhase = "Lost"

	// used for ExtendedResourceClaim that is bound to ExtendedResource
	ExtendedResourceClaimBound ExtendedResourceClaimPhase = "Bound"

	// used for ExtendedResourceClaim that is pending
	ExtendedResourceClaimPending ExtendedResourceClaimPhase = "Pending"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtendedResource represents a specific extended resource
type ExtendedResource struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec is the desired state of the ExtendedResource.
	Spec ExtendedResourceSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`

	// Status is the current state of the ExtendedResource.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status
	// +optional
	Status ExtendedResourceStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtendedResourceList is a collection of ExtendedResource.
type ExtendedResourceList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is the list of ExtendedResource.
	Items []ExtendedResource `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ExtendedResourceSpec describes the ExtendedResource the user wishes to exist.
type ExtendedResourceSpec struct {
	// NodeAffinity defines constraints that limit what nodes this resource can be accessed from.
	// This field influences the scheduling of pods that use this resource.
	// +optional
	NodeAffinity *ResourceNodeAffinity `json:"nodeAffinity,omitempty" protobuf:"bytes,1,opt,name=nodeAffinity"`

	// Raw resource name. E.g.: nvidia.com/gpu
	RawResourceName string `json:"rawResourceName" protobuf:"bytes,2,opt,name=rawResourceName"`

	// device id
	DeviceID string `json:"deviceID" protobuf:"bytes,3,opt,name=deviceID"`

	// gpuType: k80
	// zone: us-west1-b
	// Note Kubelet adds a special property corresponding to the above ResourceName field.
	// This will allow a single ResourceClass (e.g., “gpu”) to match multiple types of
	// resources (e.g., nvidia.com/gpu and amd.com/gpu) through general set selector.
	// +optional
	Properties map[string]string `json:"properties,omitempty" protobuf:"bytes,4,opt,name=properties"`

	// ExtendedResourceClaimName is the name of ExtendedResourceClaim that the ExtendedResource is bound to
	// +optional
	ExtendedResourceClaimName string `json:"extendedResourceClaimName" protobuf:"bytes,5,opt,name=extendedResourceClaimName"`
}

// ResourceNodeAffinity defines constraints that limit what nodes this extended resource can be accessed from.
type ResourceNodeAffinity struct {
	// Required specifies hard node constraints that must be met.
	Required *v1.NodeSelector `json:"required,omitempty" protobuf:"bytes,1,opt,name=required"`
}

// ExtendedResourceStatus is the status of ExtendedResource
type ExtendedResourceStatus struct {
	// Capacity is the capacity of this device
	// +optional
	Capacity resource.Quantity `json:"capacity,omitempty" protobuf:"bytes,1,opt,name=capacity"`
	// Allocatable is the resource of this device that can be available for scheduling
	// +optional
	Allocatable resource.Quantity `json:"allocatable,omitempty" protobuf:"bytes,2,opt,name=allocatable"`
	// Phase indicates if the extended Resource is available, bound or pending
	// +optional
	Phase ExtendedResourcePhase `json:"phase,omitempty" protobuf:"bytes,3,opt,name=phase"`
	// A human-readable message indicating details about why CR is in this phase
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,4,opt,name=message"`
	// Brief string that describes any failure, used for CLI etc
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`
}

type ExtendedResourcePhase string

const (
	// used for ExtendedResource that is already on a specific node and not used
	ExtendedResourceAvailable ExtendedResourcePhase = "Available"

	// used for ExtendedResource that is bound
	ExtendedResourceBound ExtendedResourcePhase = "Bound"

	// used for ExtendedResource that is not available now
	ExtendedResourcePending ExtendedResourcePhase = "Pending"
)
