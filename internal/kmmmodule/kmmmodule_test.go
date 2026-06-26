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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	intelv1alpha1 "github.com/abyrne55/intel-gpu-operator-poc/api/v1alpha1"
	"github.com/abyrne55/intel-gpu-operator-poc/internal/constants"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("setKMMModuleLoader", func() {
	It("should set ModuleLoader for OOT mode", func() {
		mod := kmmv1beta1.Module{}
		devConfig := &intelv1alpha1.DeviceConfig{
			Spec: intelv1alpha1.DeviceConfigSpec{
				Driver: intelv1alpha1.DriverSpec{
					Image:   "my-registry/xe-driver:v1.0",
					Version: "v1.0",
				},
			},
		}

		err := setKMMModuleLoader(&mod, devConfig)
		Expect(err).NotTo(HaveOccurred())

		Expect(mod.Spec.ModuleLoader).NotTo(BeNil())
		Expect(mod.Spec.ModuleLoader.Container.Modprobe.ModuleName).To(Equal("xe"))
		Expect(mod.Spec.ModuleLoader.Container.KernelMappings).To(HaveLen(1))
		Expect(mod.Spec.ModuleLoader.Container.KernelMappings[0].ContainerImage).To(Equal("my-registry/xe-driver:v1.0"))
		Expect(mod.Spec.ModuleLoader.Container.KernelMappings[0].InTreeModulesToRemove).To(Equal([]string{"xe"}))
		Expect(mod.Spec.ModuleLoader.Container.Version).To(Equal("v1.0"))
		Expect(mod.Spec.ModuleLoader.ServiceAccountName).To(Equal("intel-gpu-operator-kmm-module-loader"))
		Expect(mod.Spec.Tolerations).To(HaveLen(1))
		Expect(mod.Spec.Tolerations[0].Key).To(Equal(constants.UpgradeTaintTolerationKey))
	})

	It("should return error when driver image is empty", func() {
		mod := kmmv1beta1.Module{}
		devConfig := &intelv1alpha1.DeviceConfig{
			Spec: intelv1alpha1.DeviceConfigSpec{
				Driver: intelv1alpha1.DriverSpec{Version: "v1.0"},
			},
		}

		err := setKMMModuleLoader(&mod, devConfig)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.driver.image is required"))
	})
})

var _ = Describe("setKMMDRA", func() {
	It("should configure DRA with correct driver name and DeviceClasses", func() {
		mod := kmmv1beta1.Module{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-module",
				Namespace: "test-ns",
			},
		}
		devConfig := &intelv1alpha1.DeviceConfig{
			Spec: intelv1alpha1.DeviceConfigSpec{
				DRA: intelv1alpha1.DRASpec{
					Image: "ghcr.io/intel/intel-resource-drivers-for-kubernetes/intel-gpu-resource-driver:v0.9.0",
				},
			},
		}

		setKMMDRA(&mod, devConfig)

		Expect(mod.Spec.DRA).NotTo(BeNil())
		Expect(mod.Spec.DevicePlugin).To(BeNil())
		Expect(mod.Spec.DRA.DriverName).To(Equal("gpu.intel.com"))
		Expect(mod.Spec.DRA.Container.Image).To(Equal(devConfig.Spec.DRA.Image))
		Expect(mod.Spec.DRA.Container.Command).To(Equal([]string{"/kubelet-gpu-plugin"}))
		Expect(mod.Spec.DRA.Container.ImagePullPolicy).To(Equal(v1.PullIfNotPresent))
	})

	It("should set correct environment variables", func() {
		mod := kmmv1beta1.Module{}
		devConfig := &intelv1alpha1.DeviceConfig{
			Spec: intelv1alpha1.DeviceConfigSpec{
				DRA: intelv1alpha1.DRASpec{Image: "test:latest"},
			},
		}

		setKMMDRA(&mod, devConfig)

		env := mod.Spec.DRA.Container.Env
		Expect(env).To(HaveLen(3))

		nodeNameEnv := env[0]
		Expect(nodeNameEnv.Name).To(Equal("NODE_NAME"))
		Expect(nodeNameEnv.ValueFrom.FieldRef.FieldPath).To(Equal("spec.nodeName"))

		podNSEnv := env[1]
		Expect(podNSEnv.Name).To(Equal("POD_NAMESPACE"))
		Expect(podNSEnv.ValueFrom.FieldRef.FieldPath).To(Equal("metadata.namespace"))

		sysfsEnv := env[2]
		Expect(sysfsEnv.Name).To(Equal("SYSFS_ROOT"))
		Expect(sysfsEnv.Value).To(Equal("/sysfs"))
	})

	It("should create two DeviceClasses with correct CEL selectors", func() {
		mod := kmmv1beta1.Module{}
		devConfig := &intelv1alpha1.DeviceConfig{
			Spec: intelv1alpha1.DeviceConfigSpec{
				DRA: intelv1alpha1.DRASpec{Image: "test:latest"},
			},
		}

		setKMMDRA(&mod, devConfig)

		Expect(mod.Spec.DRA.DeviceClasses).To(HaveLen(2))

		gpuClass := mod.Spec.DRA.DeviceClasses[0]
		Expect(gpuClass.Name).To(Equal("gpu.intel.com"))
		Expect(gpuClass.Selectors).To(HaveLen(1))
		Expect(gpuClass.Selectors[0].CEL.Expression).To(Equal(`device.driver == "gpu.intel.com"`))

		vfioClass := mod.Spec.DRA.DeviceClasses[1]
		Expect(vfioClass.Name).To(Equal("gpu-vfio.intel.com"))
		Expect(vfioClass.Selectors).To(HaveLen(1))
		Expect(vfioClass.Selectors[0].CEL.Expression).To(Equal(`device.driver == "gpu.intel.com"`))
	})

	It("should configure 6 volume mounts and 6 volumes", func() {
		mod := kmmv1beta1.Module{}
		devConfig := &intelv1alpha1.DeviceConfig{
			Spec: intelv1alpha1.DeviceConfigSpec{
				DRA: intelv1alpha1.DRASpec{Image: "test:latest"},
			},
		}

		setKMMDRA(&mod, devConfig)

		Expect(mod.Spec.DRA.Container.VolumeMounts).To(HaveLen(6))
		Expect(mod.Spec.DRA.Volumes).To(HaveLen(6))
	})
})

var _ = Describe("getNodeSelector", func() {
	It("should return custom selector when set", func() {
		devConfig := &intelv1alpha1.DeviceConfig{
			Spec: intelv1alpha1.DeviceConfigSpec{
				Selector: map[string]string{"custom-label": "value"},
			},
		}

		sel := getNodeSelector(devConfig)
		Expect(sel).To(Equal(map[string]string{"custom-label": "value"}))
	})

	It("should return PCI vendor selector as default", func() {
		devConfig := &intelv1alpha1.DeviceConfig{}

		sel := getNodeSelector(devConfig)
		Expect(sel).To(HaveKey("feature.node.kubernetes.io/pci-8086.present"))
	})
})
