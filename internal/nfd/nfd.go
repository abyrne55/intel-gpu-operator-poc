package nfd

import (
	"context"

	intelv1alpha1 "github.com/abyrne55/intel-gpu-operator-poc/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

//go:generate mockgen -source=nfd.go -package=nfd -destination=mock_nfd.go NFDRuleAPI
type NFDRuleAPI interface {
	EnsureNodeFeatureRule(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) (controllerutil.OperationResult, error)
}

type nfdRule struct {
	client client.Client
	scheme *runtime.Scheme
}

func NewNFDRule(client client.Client, scheme *runtime.Scheme) NFDRuleAPI {
	return &nfdRule{client: client, scheme: scheme}
}

func (n *nfdRule) EnsureNodeFeatureRule(_ context.Context, _ *intelv1alpha1.DeviceConfig) (controllerutil.OperationResult, error) {
	// TODO: implement in step 5
	return controllerutil.OperationResultNone, nil
}
