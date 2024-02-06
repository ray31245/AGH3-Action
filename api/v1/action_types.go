/*
Copyright 2024.

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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ActionSpec defines the desired state of Action
type ActionSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Specifies the image of worker that will be created when executing a Action.
	Image string `json:"image"`
	// Specifies the args of worker that will be created when executing a Action.
	Args []string `json:"args"`

	// If mark Activation to true, when detected by controller, it will run action and resum this value to false to ensure run once
	Activation bool `json:"activation"`
	// If mark Stop to true, controller will not run action
	Stop bool `json:"stop"`

	// Limit numbers of old history worker
	WorkerHistoryLimit *int32 `json:"WorkerHistoryLimit"`
}

// ActionStatus defines the observed state of Action
type ActionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Pointers to currently running worker.
	// +optional
	Active corev1.ObjectReference `json:"active,omitempty"`

	// ActiveStatus is depended on worker's status
	ActiveStatus ActiveStatus `json:"activeStatus"`
}

// Only on of the following status may be recorded on ActiveStatus
type ActiveStatus string

const (
	// Action not run yet
	ActiveStatusPending ActiveStatus = "Pending"
	// Action is running
	ActiveStatusRuning ActiveStatus = "Runing"
	// Action is Successed
	ActiveStatusSuccessed ActiveStatus = "Successed"
	// Action is Fail
	ActiveStatusFail ActiveStatus = "Fail"
	// Action is Stoped
	ActiveStatusStop ActiveStatus = "Stop"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Action is the Schema for the actions API
// +kubebuilder:printcolumn:name="status",type=string,JSONPath=`.status.activeStatus`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Action struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActionSpec   `json:"spec,omitempty"`
	Status ActionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ActionList contains a list of Action
type ActionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Action `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Action{}, &ActionList{})
}
