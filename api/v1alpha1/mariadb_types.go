/*
Copyright 2021.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MariaDBSpec defines the desired state of MariaDB
type MariaDBSpec struct {

	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Maximum=4
	Replicas *int32 `json:"replicas"`

	// Database additional user details (base64 encoded)
	// +kubebuilder:validation:Required
	Username string `json:"username"`

	// Database additional user password (base64 encoded)
	// +kubebuilder:validation:Required
	Password string `json:"password"`

	// New Database name
	// +kubebuilder:validation:Required
	Database string `json:"database"`

	// Root user password (if set will be used, else root secret will be created)
	// +optional
	Rootpwd string `json:"rootpwd"`

	// Image name with version
	// +optional
	Image string `json:"image"`

	// Image version (latest is 10.6, so let's have it as latest)
	// +optional
	// +kubebuilder:default="10.6"
	ImageVersion string `json:"imageVersion"`

	// Database storage Path
	// +kubebuilder:validation:Required
	DataStoragePath string `json:"dataStoragePath"`

	// Database storage Size (Ex. 1Gi, 100Mi)
	// +optional
	DataStorageSize string `json:"dataStorageSize"`

	// Port number exposed for Database service
	// +optional
	// +kubebuilder:default=3306

	Port int32 `json:"port"`
}

type StatusPhase string

const (
	RunningStatusPhase      StatusPhase = "RUNNING"
	BootstrapingStatusPhase StatusPhase = "BOOTSTRAP"
	ErrorStatusPhase        StatusPhase = "ERROR"
)

// MariaDBStatus defines the observed state of MariaDB
type MariaDBStatus struct {
	CurrentReplicas *int32      `json:"currentReplicas,omitempty"` // If it's nil, it is unset, we'll use a default. If it is 0 than it is set to 0
	DesiredReplicas int32       `json:"desiredReplicas"`           // 0 is the same as unset (no value) and default will be applied even if user applies 0.
	LastMessage     string      `json:"lastMessage"`
	DbState         StatusPhase `json:"dbState"`

	// +optional
	// +kubebuilder:default="NOT STARTED"

	ShowState string `json:"showState"`
	SecretSet int32  `json:"secretSet"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:priority=0,name=MariaDB State,type=string,JSONPath=".status.showState",description="State of the MariaDB instance",format=""
// +kubebuilder:printcolumn:priority=0,name=Port,type=string,JSONPath=".spec.port",description="Port of the MariaDB instance",format=""
// +kubebuilder:printcolumn:priority=1,name=Image,type=string,JSONPath=".spec.image",description="Image of the MariaDB instance",format=""
// +kubebuilder:printcolumn:priority=0,name=Age, type=date,JSONPath=".metadata.creationTimestamp"

// MariaDB is the Schema for the mariadbs API
type MariaDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MariaDBSpec   `json:"spec,omitempty"`
	Status MariaDBStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MariaDBList contains a list of MariaDB
type MariaDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MariaDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MariaDB{}, &MariaDBList{})
}
