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

package controllers

import (
	"context"
	"fmt"

	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	intelv1alpha1 "github.com/abyrne55/intel-gpu-operator-poc/api/v1alpha1"
	"github.com/abyrne55/intel-gpu-operator-poc/internal/filter"
	"github.com/abyrne55/intel-gpu-operator-poc/internal/kmmmodule"
	"github.com/abyrne55/intel-gpu-operator-poc/internal/nfd"
	"github.com/abyrne55/intel-gpu-operator-poc/internal/upgrade"
	"github.com/abyrne55/intel-gpu-operator-poc/internal/xpumanager"
)

const (
	DeviceConfigReconcilerName = "DriverAndPluginReconciler"
	deviceConfigFinalizer      = "intel.node.kubernetes.io/deviceconfig-finalizer"
)

type DeviceConfigReconciler struct {
	helper deviceConfigReconcilerHelperAPI
	filter *filter.Filter
}

func NewDeviceConfigReconciler(
	client client.Client,
	kmmHandler kmmmodule.KMMModuleAPI,
	upgradeHandler upgrade.UpgradeAPI,
	nfdHandler nfd.NFDRuleAPI,
	xpuHandler xpumanager.XPUManagerAPI,
	filter *filter.Filter,
	scheme *runtime.Scheme,
) *DeviceConfigReconciler {
	helper := newDeviceConfigReconcilerHelper(client, kmmHandler, upgradeHandler, nfdHandler, xpuHandler, scheme)
	return &DeviceConfigReconciler{
		helper: helper,
		filter: filter,
	}
}

func (r *DeviceConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&intelv1alpha1.DeviceConfig{}).
		Owns(&kmmv1beta1.Module{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.filter.FindDeviceConfigForNodeChange),
			builder.WithPredicates(r.filter.GetNodePredicate()),
		).
		Named(DeviceConfigReconcilerName).
		Complete(
			reconcile.AsReconciler[*intelv1alpha1.DeviceConfig](mgr.GetClient(), r),
		)
}

//+kubebuilder:rbac:groups=intel.com,resources=deviceconfigs,verbs=get;list;watch;create;patch;update
//+kubebuilder:rbac:groups=kmm.sigs.x-k8s.io,resources=modules,verbs=get;list;watch;create;patch;update;delete
//+kubebuilder:rbac:groups=intel.com,resources=deviceconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=kmm.sigs.x-k8s.io,resources=modules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=create;delete;get;list;patch;watch;create
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=create;delete;get;list;patch;watch
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=create;delete;get;list;patch;watch
//+kubebuilder:rbac:groups=core,resources=services,verbs=create;delete;get;list;patch;watch
//+kubebuilder:rbac:groups=nfd.k8s-sigs.io,resources=nodefeaturerules,verbs=create;delete;get;list;patch;watch

func (r *DeviceConfigReconciler) Reconcile(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) (ctrl.Result, error) {
	res := ctrl.Result{}
	logger := log.FromContext(ctx).WithValues("DeviceConfig namespace", devConfig.Namespace, "DeviceConfig name", devConfig.Name)

	if devConfig.GetDeletionTimestamp() != nil {
		err := r.helper.finalizeDeviceConfig(ctx, devConfig)
		if err != nil {
			return res, fmt.Errorf("failed to finalize DeviceConfig: %v", err)
		}
		return res, nil
	}

	if err := r.helper.setFinalizer(ctx, devConfig); err != nil {
		return res, fmt.Errorf("failed to set finalizer for DeviceConfig: %v", err)
	}

	logger.Info("reconciling NFD NodeFeatureRule")
	if err := r.helper.handleNFDRule(ctx, devConfig); err != nil {
		return res, fmt.Errorf("failed to handle NFD rule for DeviceConfig: %v", err)
	}

	logger.Info("reconciling KMM Module")
	if err := r.helper.handleKMMModule(ctx, devConfig); err != nil {
		return res, fmt.Errorf("failed to handle KMM module for DeviceConfig: %v", err)
	}

	logger.Info("reconciling XPU Manager")
	if err := r.helper.handleXPUManager(ctx, devConfig); err != nil {
		return res, fmt.Errorf("failed to handle XPU Manager for DeviceConfig: %v", err)
	}

	logger.Info("reconciling rolling upgrade")
	if err := r.helper.handleModuleVersionUpgrade(ctx, devConfig); err != nil {
		return res, fmt.Errorf("failed to handle KMM module version upgrade for DeviceConfig: %v", err)
	}

	return res, nil
}

//go:generate mockgen -source=device_config_reconciler.go -package=controllers -destination=mock_device_config_reconciler.go deviceConfigReconcilerHelperAPI
type deviceConfigReconcilerHelperAPI interface {
	finalizeDeviceConfig(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) error
	setFinalizer(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) error
	handleKMMModule(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) error
	handleModuleVersionUpgrade(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) error
	handleNFDRule(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) error
	handleXPUManager(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) error
}

type deviceConfigReconcilerHelper struct {
	client         client.Client
	kmmHandler     kmmmodule.KMMModuleAPI
	upgradeHandler upgrade.UpgradeAPI
	nfdHandler     nfd.NFDRuleAPI
	xpuHandler     xpumanager.XPUManagerAPI
	scheme         *runtime.Scheme
}

func newDeviceConfigReconcilerHelper(
	client client.Client,
	kmmHandler kmmmodule.KMMModuleAPI,
	upgradeHandler upgrade.UpgradeAPI,
	nfdHandler nfd.NFDRuleAPI,
	xpuHandler xpumanager.XPUManagerAPI,
	scheme *runtime.Scheme,
) deviceConfigReconcilerHelperAPI {
	return &deviceConfigReconcilerHelper{
		client:         client,
		kmmHandler:     kmmHandler,
		upgradeHandler: upgradeHandler,
		nfdHandler:     nfdHandler,
		xpuHandler:     xpuHandler,
		scheme:         scheme,
	}
}

func (dcrh *deviceConfigReconcilerHelper) setFinalizer(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) error {
	if controllerutil.ContainsFinalizer(devConfig, deviceConfigFinalizer) {
		return nil
	}

	devConfigCopy := devConfig.DeepCopy()
	controllerutil.AddFinalizer(devConfig, deviceConfigFinalizer)
	return dcrh.client.Patch(ctx, devConfig, client.MergeFrom(devConfigCopy))
}

func (dcrh *deviceConfigReconcilerHelper) finalizeDeviceConfig(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) error {
	logger := log.FromContext(ctx)

	mod := kmmv1beta1.Module{}
	namespacedName := types.NamespacedName{
		Namespace: devConfig.Namespace,
		Name:      devConfig.Name,
	}
	err := dcrh.client.Get(ctx, namespacedName, &mod)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Info("module already deleted, removing finalizer", "module", namespacedName)
			devConfigCopy := devConfig.DeepCopy()
			controllerutil.RemoveFinalizer(devConfig, deviceConfigFinalizer)
			return dcrh.client.Patch(ctx, devConfig, client.MergeFrom(devConfigCopy))
		}
		return fmt.Errorf("failed to get the requested Module %s: %v", namespacedName, err)
	}
	logger.Info("deleting KMM Module", "module", namespacedName)
	return dcrh.client.Delete(ctx, &mod)
}

func (dcrh *deviceConfigReconcilerHelper) handleKMMModule(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) error {
	kmmMod := &kmmv1beta1.Module{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: devConfig.Namespace,
			Name:      devConfig.Name,
		},
	}
	logger := log.FromContext(ctx)
	opRes, err := controllerutil.CreateOrPatch(ctx, dcrh.client, kmmMod, func() error {
		return dcrh.kmmHandler.SetKMMModuleAsDesired(kmmMod, devConfig)
	})
	if err == nil {
		logger.Info("Reconciled KMM Module", "name", kmmMod.Name, "result", opRes)
	}
	return err
}

func (dcrh *deviceConfigReconcilerHelper) handleNFDRule(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) error {
	logger := log.FromContext(ctx)
	opRes, err := dcrh.nfdHandler.EnsureNodeFeatureRule(ctx, devConfig)
	if err == nil {
		logger.Info("Reconciled NFD NodeFeatureRule", "result", opRes)
	}
	return err
}

func (dcrh *deviceConfigReconcilerHelper) handleXPUManager(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) error {
	logger := log.FromContext(ctx)
	opRes, err := dcrh.xpuHandler.EnsureXPUManagerDaemonSet(ctx, devConfig)
	if err == nil {
		logger.Info("Reconciled XPU Manager", "result", opRes)
	}
	return err
}

func (dcrh *deviceConfigReconcilerHelper) handleModuleVersionUpgrade(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) error {
	if devConfig.Spec.Driver.Version == "" {
		return nil
	}
	logger := log.FromContext(ctx)
	targetedNodes, err := dcrh.upgradeHandler.GetTargetedNodes(ctx, devConfig)
	if err != nil {
		return fmt.Errorf("failed to get nodes targeted by the DeviceConfig %s/%s: %v", devConfig.Namespace, devConfig.Name, err)
	}

	logger.Info("targeted nodes for rolling upgrade", "num nodes", len(targetedNodes))

	node := dcrh.upgradeHandler.GetUpgradedNode(ctx, devConfig, targetedNodes)
	err = dcrh.upgradeHandler.UncordonUpgradedNode(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to finalize upgraded nodes for DeviceConfig %s/%s: %v", devConfig.Namespace, devConfig.Name, err)
	}

	node = dcrh.upgradeHandler.GetNodeForUpgrade(ctx, devConfig, targetedNodes)
	err = dcrh.upgradeHandler.CordonNodeForUpgrade(ctx, devConfig, node)
	if err != nil {
		return fmt.Errorf("failed to cordon node %s for DeviceConfig %s/%s: %v", node.Name, devConfig.Namespace, devConfig.Name, err)
	}

	return dcrh.upgradeHandler.KickoffUpgrade(ctx, devConfig, node)
}
