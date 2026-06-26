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

package test

import (
	intelv1alpha1 "github.com/abyrne55/intel-gpu-operator-poc/api/v1alpha1"
	hubv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api-hub/v1beta1"
	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

func TestScheme() (*runtime.Scheme, error) {
	s := runtime.NewScheme()

	funcs := []func(s *runtime.Scheme) error{
		scheme.AddToScheme,
		intelv1alpha1.AddToScheme,
		kmmv1beta1.AddToScheme,
		hubv1beta1.AddToScheme,
		clusterv1.Install,
		workv1.Install,
		nfdv1alpha1.AddToScheme,
	}

	for _, f := range funcs {
		if err := f(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}
