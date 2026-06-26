package xpumanager

import (
	"context"

	intelv1alpha1 "github.com/abyrne55/intel-gpu-operator-poc/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

//go:generate mockgen -source=xpumanager.go -package=xpumanager -destination=mock_xpumanager.go XPUManagerAPI
type XPUManagerAPI interface {
	EnsureXPUManagerDaemonSet(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) (controllerutil.OperationResult, error)
}

type xpuManager struct {
	client client.Client
	scheme *runtime.Scheme
}

func NewXPUManager(client client.Client, scheme *runtime.Scheme) XPUManagerAPI {
	return &xpuManager{client: client, scheme: scheme}
}

func (x *xpuManager) EnsureXPUManagerDaemonSet(_ context.Context, _ *intelv1alpha1.DeviceConfig) (controllerutil.OperationResult, error) {
	// TODO: implement in step 6
	return controllerutil.OperationResultNone, nil
}
