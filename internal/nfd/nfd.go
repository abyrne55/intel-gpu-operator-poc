package nfd

import (
	"context"

	intelv1alpha1 "github.com/abyrne55/intel-gpu-operator-poc/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

const (
	nfrName    = "intel-gpu-devices"
	managedBy  = "intel-gpu-operator"
	GPULabel   = "intel.feature.node.kubernetes.io/gpu"
	pciVendor  = "8086"
	pciClass03 = "0300"
	pciClass03x = "0380"
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

func (n *nfdRule) EnsureNodeFeatureRule(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) (controllerutil.OperationResult, error) {
	nfr := &nfdv1alpha1.NodeFeatureRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nfrName,
			Namespace: devConfig.Namespace,
		},
	}

	return controllerutil.CreateOrPatch(ctx, n.client, nfr, func() error {
		nfr.Labels = map[string]string{
			"app.kubernetes.io/managed-by": managedBy,
		}
		nfr.Spec = buildNodeFeatureRuleSpec()
		return nil
	})
}

func buildNodeFeatureRuleSpec() nfdv1alpha1.NodeFeatureRuleSpec {
	return nfdv1alpha1.NodeFeatureRuleSpec{
		Rules: []nfdv1alpha1.Rule{
			{
				Name: "intel.gpu",
				Labels: map[string]string{
					GPULabel: "true",
				},
				MatchFeatures: nfdv1alpha1.FeatureMatcher{
					{
						Feature: "pci.device",
						MatchExpressions: &nfdv1alpha1.MatchExpressionSet{
							"vendor": {Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{pciVendor}},
							"class":  {Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{pciClass03, pciClass03x}},
						},
					},
				},
				MatchAny: []nfdv1alpha1.MatchAnyElem{
					{MatchFeatures: nfdv1alpha1.FeatureMatcher{{
						Feature:          "kernel.loadedmodule",
						MatchExpressions: &nfdv1alpha1.MatchExpressionSet{"i915": {Op: nfdv1alpha1.MatchExists}},
					}}},
					{MatchFeatures: nfdv1alpha1.FeatureMatcher{{
						Feature:          "kernel.enabledmodule",
						MatchExpressions: &nfdv1alpha1.MatchExpressionSet{"i915": {Op: nfdv1alpha1.MatchExists}},
					}}},
					{MatchFeatures: nfdv1alpha1.FeatureMatcher{{
						Feature:          "kernel.loadedmodule",
						MatchExpressions: &nfdv1alpha1.MatchExpressionSet{"xe": {Op: nfdv1alpha1.MatchExists}},
					}}},
					{MatchFeatures: nfdv1alpha1.FeatureMatcher{{
						Feature:          "kernel.enabledmodule",
						MatchExpressions: &nfdv1alpha1.MatchExpressionSet{"xe": {Op: nfdv1alpha1.MatchExists}},
					}}},
				},
			},
		},
	}
}
