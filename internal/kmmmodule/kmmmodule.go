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
	_ "embed"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	intelv1alpha1 "github.com/abyrne55/intel-gpu-operator-poc/api/v1alpha1"
	"github.com/abyrne55/intel-gpu-operator-poc/internal/constants"
)

const (
	kubeletDevicePluginsVolumeName = "kubelet-device-plugins"
	kubeletDevicePluginsPath       = "/var/lib/kubelet/device-plugins"
	nodeVarLibFirmwarePath         = "/var/lib/firmware"
	gpuDriverModuleName            = "xe"
	imageFirmwarePath = "placeholder"
)

var (
	//go:embed dockerfiles/driversDockerfile.txt
	buildDockerfile string
)

//go:generate mockgen -source=kmmmodule.go -package=kmmmodule -destination=mock_kmmmodule.go KMMModuleAPI
type KMMModuleAPI interface {
	SetBuildConfigMapAsDesired(buildCM *v1.ConfigMap, devConfig *intelv1alpha1.DeviceConfig) error
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

func (km *kmmModule) SetBuildConfigMapAsDesired(buildCM *v1.ConfigMap, devConfig *intelv1alpha1.DeviceConfig) error {
	if buildCM.Data == nil {
		buildCM.Data = make(map[string]string)
	}

	buildCM.Data["dockerfile"] = buildDockerfile
	return controllerutil.SetControllerReference(devConfig, buildCM, km.scheme)
}

func (km *kmmModule) SetKMMModuleAsDesired(mod *kmmv1beta1.Module, devConfig *intelv1alpha1.DeviceConfig) error {
	err := setKMMModuleLoader(mod, devConfig)
	if err != nil {
		return fmt.Errorf("failed to set KMM Module: %v", err)
	}
	setKMMDevicePlugin(mod, devConfig)
	return controllerutil.SetControllerReference(devConfig, mod, km.scheme)
}

func setKMMModuleLoader(mod *kmmv1beta1.Module, devConfig *intelv1alpha1.DeviceConfig) error {
        driversImage := devConfig.Spec.Driver.Image + "-$KERNEL_VERSION"

	mod.Spec.ModuleLoader = &kmmv1beta1.ModuleLoaderSpec{
		Container: kmmv1beta1.ModuleLoaderContainerSpec{
			Modprobe: kmmv1beta1.ModprobeSpec{
				ModuleName:   gpuDriverModuleName,
				FirmwarePath: imageFirmwarePath,
			},
			KernelMappings: []kmmv1beta1.KernelMapping{
				{
					Regexp:               "^.+$",
					ContainerImage:       driversImage,
					InTreeModulesToRemove: []string{gpuDriverModuleName,},
                                        
				},
			},
			ImagePullPolicy: v1.PullAlways,
                        Version:         devConfig.Spec.Driver.Version,
		},
	}
	mod.Spec.ModuleLoader.ServiceAccountName = "intel-gpu-operator-kmm-module-loader"
	mod.Spec.ImageRepoSecret = devConfig.Spec.ImageRepoSecret
	mod.Spec.Selector = getNodeSelector(devConfig)
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

func setKMMDevicePlugin(mod *kmmv1beta1.Module, devConfig *intelv1alpha1.DeviceConfig) {
	devicePluginImage := devConfig.Spec.DRA.Image
	hostPathDirectory := v1.HostPathDirectory
	mod.Spec.DevicePlugin = &kmmv1beta1.DevicePluginSpec{
		ServiceAccountName: "intel-gpu-operator-kmm-device-plugin",
		Container: kmmv1beta1.DevicePluginContainerSpec{
			Image: devicePluginImage,
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "sys",
					MountPath: "/sys",
				},
			},
		},
		Volumes: []v1.Volume{
			{
				Name: "sys",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/sys",
						Type: &hostPathDirectory,
					},
				},
			},
		},
	}
}

func getNodeSelector(devConfig *intelv1alpha1.DeviceConfig) map[string]string {
	if devConfig.Spec.Selector != nil {
		return devConfig.Spec.Selector
	}

	ns := make(map[string]string, 0)
	ns[fmt.Sprintf("feature.node.kubernetes.io/pci-%s.present", intelv1alpha1.PCIVendorID)] = "true"
	return ns
}
