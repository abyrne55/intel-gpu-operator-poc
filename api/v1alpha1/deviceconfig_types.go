/*
Copyright 2022.

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PCIVendorID = "8086"
)

type DriverSpec struct {
	// +kubebuilder:default=true
	UseInTreeDriver bool `json:"useInTreeDriver"`

	// +optional
	Image string `json:"image,omitempty"`

	// +optional
	Version string `json:"version,omitempty"`
}

type DRASpec struct {
	// +kubebuilder:default="ghcr.io/intel/intel-resource-drivers-for-kubernetes/intel-gpu-resource-driver:v0.9.0"
	Image string `json:"image"`
}

type XPUManagerSpec struct {
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// +kubebuilder:default="ghcr.io/intel/xpumanager/xpumd:v2.0.0"
	Image string `json:"image"`
}

type DeviceConfigSpec struct {
	// +optional
	Driver DriverSpec `json:"driver,omitempty"`

	// +optional
	DRA DRASpec `json:"dra,omitempty"`

	// +optional
	XPUManager XPUManagerSpec `json:"xpuManager,omitempty"`

	// +optional
	ImageRepoSecret *v1.LocalObjectReference `json:"imageRepoSecret,omitempty"`

	// +optional
	Selector map[string]string `json:"selector,omitempty"`
}

type DeploymentStatus struct {
	NodesMatchingSelectorNumber int32 `json:"nodesMatchingSelectorNumber,omitempty"`
	DesiredNumber               int32 `json:"desiredNumber,omitempty"`
	AvailableNumber             int32 `json:"availableNumber,omitempty"`
}

type DeviceConfigStatus struct {
	DevicePlugin DeploymentStatus `json:"devicePlugin,omitempty"`
	Drivers      DeploymentStatus `json:"driver"`
	DRA          DeploymentStatus `json:"dra,omitempty"`
	XPUManager   DeploymentStatus `json:"xpuManager,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Namespaced,shortName=inteldc
//+kubebuilder:subresource:status

// DeviceConfig describes how to enable intel GPU device
// +operator-sdk:csv:customresourcedefinitions:displayName="DeviceConfig"
type DeviceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeviceConfigSpec   `json:"spec,omitempty"`
	Status DeviceConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DeviceConfigList contains a list of DeviceConfigs
type DeviceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeviceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeviceConfig{}, &DeviceConfigList{})
}
