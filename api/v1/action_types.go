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

	// identify the action is from same history
	HistoryID string `json:"historyID"`

	// setup the initContainers of worker that will be created when executing a Action.
	// +optional
	InitContainers []Container `json:"initContainers"`

	// setup the containers of worker that will be created when executing a Action.
	Containers []Container `json:"containers"`

	// If mark TrigerRun to true, when detected by controller, it will run action and resum this value to false to ensure run once
	TrigerRun bool `json:"trigerRun"`
	// If mark Stop to true, when detected by controller, it will stop action and resum this value to false to ensure run once
	TrigerStop bool `json:"trigerStop"`

	// Limit numbers of old history worker
	WorkerHistoryLimit *int32 `json:"WorkerHistoryLimit"`
}

type Container struct {
	Image string `json:"image"`
	// +optional
	Command []string `json:"command"`
	// +optional
	Args []string `json:"args"`
	// +optional
	Env map[string]string `json:"Env"`
	// +optional
	VolumeMounts map[string]string `json:"volumeMounts"`
}

// ActionStatus defines the observed state of Action
type ActionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Pointers to currently running worker.
	// +optional
	Worker *corev1.ObjectReference `json:"worker,omitempty"`

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
