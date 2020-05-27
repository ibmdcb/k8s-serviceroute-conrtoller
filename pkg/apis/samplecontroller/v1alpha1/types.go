/*
Copyright 2017 The Kubernetes Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CapsRoute is a specification for a CapsRoute resource
type CapsRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CapsRouteSpec   `json:"spec"`
	Status CapsRouteStatus `json:"status"`
}

// CapsRouteSpec is the spec for a CapsRoute resource
type CapsRouteSpec struct {
	ServiceName string `json:"serviceName"`
	RouteName   string `json:"routeName"`
}

// CapsRouteStatus is the status for a CapsRoute resource
type CapsRouteStatus struct {
	FullRouteName string `json:"fullRouteName"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CapsRouteList is a list of CapsRoute resources
type CapsRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CapsRoute `json:"items"`
}
