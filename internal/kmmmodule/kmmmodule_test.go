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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
        intelv1alpha1 "github.com/abyrne55/intel-gpu-operator-poc/api/v1alpha1"
	"github.com/abyrne55/intel-gpu-operator-poc/internal/constants"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

var _ = Describe("setKMMModuleLoader", func() {
	It("KMM module creation - default input values", func() {
		mod := kmmv1beta1.Module{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "moduleName",
				Namespace: "moduleNamespace",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Module",
				APIVersion: "kmm.sigs.x-k8s.io/v1beta1",
			},
		}
		input := intelv1alpha1.DeviceConfig{
			Spec: intelv1alpha1.DeviceConfigSpec{
				Driver: intelv1alpha1.DriverSpec{Image: "some image:tag"},
			},
		}

		expectedYAMLFile, err := os.ReadFile("testdata/module_loader_test.yaml")
		Expect(err).To(BeNil())
		expectedMod := kmmv1beta1.Module{}
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		err = yaml.Unmarshal(expectedJSON, &expectedMod)
		Expect(err).To(BeNil())
		fmt.Printf("<%s>\n", expectedMod.Name)
		fmt.Printf("<%s>\n", expectedMod.Spec.ModuleLoader.Container.Modprobe.ModuleName)
		Expect(len(expectedMod.Spec.ModuleLoader.Container.KernelMappings)).To(Equal(1))

		expectedMod.Spec.ModuleLoader.Container.KernelMappings[0].ContainerImage = "some image:tag-$KERNEL_VERSION"
		
		expectedMod.Spec.ModuleLoader.Container.KernelMappings[0].Build = nil
                
		expectedMod.Spec.Selector = map[string]string{"feature.node.kubernetes.io/pci-8086.present": "true"}
		expectedMod.Spec.Tolerations[0].Key = constants.UpgradeTaintTolerationKey

		err = setKMMModuleLoader(&mod, &input)

		Expect(err).To(BeNil())
		Expect(mod).To(Equal(expectedMod))
	})

	It("KMM module creation - user input values", func() {
		mod := kmmv1beta1.Module{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "moduleName",
				Namespace: "moduleNamespace",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Module",
				APIVersion: "kmm.sigs.x-k8s.io/v1beta1",
			},
		}
		input := intelv1alpha1.DeviceConfig{
			Spec: intelv1alpha1.DeviceConfigSpec{
				Driver:          intelv1alpha1.DriverSpec{Image: "some driver image", Version: "some driver version"},
				Selector:        map[string]string{"some label": "some label value"},
				ImageRepoSecret: &v1.LocalObjectReference{Name: "image repo secret name"},
			},
		}

		expectedYAMLFile, err := os.ReadFile("testdata/module_loader_test.yaml")
		Expect(err).To(BeNil())
		expectedMod := kmmv1beta1.Module{}
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		err = yaml.Unmarshal(expectedJSON, &expectedMod)
		Expect(err).To(BeNil())
		fmt.Printf("<%s>\n", expectedMod.Name)
		fmt.Printf("<%s>\n", expectedMod.Spec.ModuleLoader.Container.Modprobe.ModuleName)
		Expect(len(expectedMod.Spec.ModuleLoader.Container.KernelMappings)).To(Equal(1))

		expectedMod.Spec.ModuleLoader.Container.Version = input.Spec.Driver.Version
		expectedMod.Spec.ModuleLoader.Container.KernelMappings[0].ContainerImage = input.Spec.Driver.Image + "-$KERNEL_VERSION"
                
                expectedMod.Spec.ModuleLoader.Container.KernelMappings[0].Build = nil
                
		expectedMod.Spec.Selector = map[string]string{"some label": "some label value"}
		expectedMod.Spec.ImageRepoSecret = &v1.LocalObjectReference{Name: "image repo secret name"}
                expectedMod.Spec.Tolerations[0].Key = constants.UpgradeTaintTolerationKey

		err = setKMMModuleLoader(&mod, &input)

		Expect(err).To(BeNil())
		Expect(mod).To(Equal(expectedMod))
	})
})

var _ = Describe("setKMMDevicePlugin", func() {
	It("KMM module creation - default input values", func() {
		mod := kmmv1beta1.Module{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "moduleName",
				Namespace: "moduleNamespace",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Module",
				APIVersion: "kmm.sigs.x-k8s.io/v1beta1",
			},
		}

		input := intelv1alpha1.DeviceConfig{}

		expectedYAMLFile, err := os.ReadFile("testdata/device_plugin_test.yaml")
		Expect(err).To(BeNil())
		expectedMod := kmmv1beta1.Module{}
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		err = yaml.Unmarshal(expectedJSON, &expectedMod)
		Expect(err).To(BeNil())

		setKMMDevicePlugin(&mod, &input)

		Expect(mod).To(Equal(expectedMod))
	})

	It("KMM module creation - user input values", func() {
		mod := kmmv1beta1.Module{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "moduleName",
				Namespace: "moduleNamespace",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Module",
				APIVersion: "kmm.sigs.x-k8s.io/v1beta1",
			},
		}

		input := intelv1alpha1.DeviceConfig{
			Spec: intelv1alpha1.DeviceConfigSpec{
				DRA: intelv1alpha1.DRASpec{Image: "some device plugin image"},
			},
		}

		expectedYAMLFile, err := os.ReadFile("testdata/device_plugin_test.yaml")
		Expect(err).To(BeNil())
		expectedMod := kmmv1beta1.Module{}
		expectedJSON, err := yaml.YAMLToJSON(expectedYAMLFile)
		Expect(err).To(BeNil())
		err = yaml.Unmarshal(expectedJSON, &expectedMod)
		Expect(err).To(BeNil())

		expectedMod.Spec.DevicePlugin.Container.Image = "some device plugin image"

		setKMMDevicePlugin(&mod, &input)

		Expect(mod).To(Equal(expectedMod))
	})
})
