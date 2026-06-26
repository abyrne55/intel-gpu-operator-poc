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

package kmmmodule

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	intelv1alpha1 "github.com/abyrne55/intel-gpu-operator-poc/api/v1alpha1"
	"github.com/abyrne55/intel-gpu-operator-poc/internal/constants"
	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
)

const (
	gpuDriverModuleName = "xe"
	draDriverName       = "gpu.intel.com"
)

//go:generate mockgen -source=kmmmodule.go -package=kmmmodule -destination=mock_kmmmodule.go KMMModuleAPI
type KMMModuleAPI interface {
	SetKMMModuleAsDesired(mod *kmmv1beta1.Module, devConfig *intelv1alpha1.DeviceConfig) error
}

type kmmModule struct {
	client client.Client
	scheme *runtime.Scheme
}

func NewKMMModule(client client.Client, scheme *runtime.Scheme) KMMModuleAPI {
	return &kmmModule{
		client: client,
		scheme: scheme,
	}
}

func (km *kmmModule) SetKMMModuleAsDesired(mod *kmmv1beta1.Module, devConfig *intelv1alpha1.DeviceConfig) error {
	mod.Spec.Selector = getNodeSelector(devConfig)
	mod.Spec.ImageRepoSecret = devConfig.Spec.ImageRepoSecret

	if !devConfig.Spec.Driver.UseInTreeDriver {
		if err := setKMMModuleLoader(mod, devConfig); err != nil {
			return fmt.Errorf("failed to set KMM ModuleLoader: %v", err)
		}
	} else {
		mod.Spec.ModuleLoader = nil
	}

	setKMMDRA(mod, devConfig)
	return controllerutil.SetControllerReference(devConfig, mod, km.scheme)
}

func setKMMModuleLoader(mod *kmmv1beta1.Module, devConfig *intelv1alpha1.DeviceConfig) error {
	driversImage := devConfig.Spec.Driver.Image
	if driversImage == "" {
		return fmt.Errorf("spec.driver.image is required when useInTreeDriver is false")
	}

	mod.Spec.ModuleLoader = &kmmv1beta1.ModuleLoaderSpec{
		Container: kmmv1beta1.ModuleLoaderContainerSpec{
			Modprobe: kmmv1beta1.ModprobeSpec{
				ModuleName: gpuDriverModuleName,
			},
			KernelMappings: []kmmv1beta1.KernelMapping{
				{
					Regexp:               "^.+$",
					ContainerImage:       driversImage,
					InTreeModulesToRemove: []string{gpuDriverModuleName},
				},
			},
			ImagePullPolicy: v1.PullAlways,
			Version:         devConfig.Spec.Driver.Version,
		},
		ServiceAccountName: "intel-gpu-operator-kmm-module-loader",
	}

	mod.Spec.Tolerations = []v1.Toleration{
		{
			Key:      constants.UpgradeTaintTolerationKey,
			Value:    "true",
			Operator: v1.TolerationOpEqual,
			Effect:   v1.TaintEffectNoExecute,
		},
	}
	return nil
}

func setKMMDRA(mod *kmmv1beta1.Module, devConfig *intelv1alpha1.DeviceConfig) {
	draImage := devConfig.Spec.DRA.Image
	celExpression := fmt.Sprintf("device.driver == %q", draDriverName)

	mod.Spec.DevicePlugin = nil
	mod.Spec.DRA = &kmmv1beta1.DRASpec{
		DriverName:         draDriverName,
		ServiceAccountName: "intel-gpu-resource-driver-service-account",
		Container: kmmv1beta1.CommonContainerSpec{
			Image:           draImage,
			ImagePullPolicy: v1.PullIfNotPresent,
			Command:         []string{"/kubelet-gpu-plugin"},
			Env: []v1.EnvVar{
				{
					Name: "NODE_NAME",
					ValueFrom: &v1.EnvVarSource{
						FieldRef: &v1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
					},
				},
				{
					Name: "POD_NAMESPACE",
					ValueFrom: &v1.EnvVarSource{
						FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
					},
				},
				{
					Name:  "SYSFS_ROOT",
					Value: "/sysfs",
				},
			},
			VolumeMounts: []v1.VolumeMount{
				{Name: "plugins-registry", MountPath: "/var/lib/kubelet/plugins_registry"},
				{Name: "plugins", MountPath: "/var/lib/kubelet/plugins"},
				{Name: "cdi", MountPath: "/etc/cdi"},
				{Name: "varruncdi", MountPath: "/var/run/cdi"},
				{Name: "xpumdrundir", MountPath: "/run/xpumd"},
				{Name: "sysfs", MountPath: "/sysfs"},
			},
		},
		Volumes: draVolumes(),
		DeviceClasses: []kmmv1beta1.DeviceClassSpec{
			{
				Name: "gpu.intel.com",
				Selectors: []resourcev1.DeviceSelector{
					{CEL: &resourcev1.CELDeviceSelector{Expression: celExpression}},
				},
			},
			{
				Name: "gpu-vfio.intel.com",
				Selectors: []resourcev1.DeviceSelector{
					{CEL: &resourcev1.CELDeviceSelector{Expression: celExpression}},
				},
			},
		},
	}
}

func draVolumes() []v1.Volume {
	directoryOrCreate := v1.HostPathDirectoryOrCreate
	return []v1.Volume{
		{Name: "plugins-registry", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins_registry"}}},
		{Name: "plugins", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins"}}},
		{Name: "cdi", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/etc/cdi"}}},
		{Name: "varruncdi", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/var/run/cdi"}}},
		{Name: "sysfs", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/sys"}}},
		{Name: "xpumdrundir", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/run/xpumd", Type: &directoryOrCreate}}},
	}
}

func getNodeSelector(devConfig *intelv1alpha1.DeviceConfig) map[string]string {
	if devConfig.Spec.Selector != nil {
		return devConfig.Spec.Selector
	}

	return map[string]string{
		fmt.Sprintf("feature.node.kubernetes.io/pci-%s.present", intelv1alpha1.PCIVendorID): "true",
	}
}
