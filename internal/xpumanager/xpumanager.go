package xpumanager

import (
	"context"
	_ "embed"
	"fmt"

	intelv1alpha1 "github.com/abyrne55/intel-gpu-operator-poc/api/v1alpha1"
	"github.com/abyrne55/intel-gpu-operator-poc/internal/nfd"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	daemonSetName  = "intel-xpumd"
	configMapName  = "intel-xpumd-config"
	serviceName    = "intel-xpumanager"
	appLabel       = "intel-xpumanager"
	appK8sName     = "xpumd"
	containerName  = "xpumd"
	httpPortName   = "http"
	httpPort       = 8080
)

//go:embed otel-config.yaml
var otelConfig string

//go:generate mockgen -source=xpumanager.go -package=xpumanager -destination=mock_xpumanager.go XPUManagerAPI
type XPUManagerAPI interface {
	EnsureXPUManagerDaemonSet(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) (controllerutil.OperationResult, error)
}

type xpuManager struct {
	client                  client.Client
	scheme                  *runtime.Scheme
	serviceMonitorAvailable bool
}

func NewXPUManager(client client.Client, scheme *runtime.Scheme, serviceMonitorAvailable bool) XPUManagerAPI {
	return &xpuManager{client: client, scheme: scheme, serviceMonitorAvailable: serviceMonitorAvailable}
}

func (x *xpuManager) EnsureXPUManagerDaemonSet(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) (controllerutil.OperationResult, error) {
	if !devConfig.Spec.XPUManager.Enabled {
		return controllerutil.OperationResultNone, nil
	}

	if _, err := x.ensureConfigMap(ctx, devConfig); err != nil {
		return controllerutil.OperationResultNone, fmt.Errorf("failed to ensure ConfigMap: %v", err)
	}

	if _, err := x.ensureService(ctx, devConfig); err != nil {
		return controllerutil.OperationResultNone, fmt.Errorf("failed to ensure Service: %v", err)
	}

	if x.serviceMonitorAvailable {
		if _, err := x.ensureServiceMonitor(ctx, devConfig); err != nil {
			return controllerutil.OperationResultNone, fmt.Errorf("failed to ensure ServiceMonitor: %v", err)
		}
	}

	return x.ensureDaemonSet(ctx, devConfig)
}

func (x *xpuManager) ensureConfigMap(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) (controllerutil.OperationResult, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: devConfig.Namespace,
		},
	}
	return controllerutil.CreateOrPatch(ctx, x.client, cm, func() error {
		cm.Data = map[string]string{
			"config.yaml": otelConfig,
		}
		return controllerutil.SetControllerReference(devConfig, cm, x.scheme)
	})
}

func (x *xpuManager) ensureService(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) (controllerutil.OperationResult, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: devConfig.Namespace,
		},
	}
	return controllerutil.CreateOrPatch(ctx, x.client, svc, func() error {
		svc.Labels = map[string]string{
			"app": appLabel,
		}
		svc.Spec.Selector = map[string]string{
			"app": appLabel,
		}
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       httpPortName,
				Port:       httpPort,
				TargetPort: intstr.FromString(httpPortName),
				Protocol:   corev1.ProtocolTCP,
			},
		}
		return controllerutil.SetControllerReference(devConfig, svc, x.scheme)
	})
}

func (x *xpuManager) ensureDaemonSet(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) (controllerutil.OperationResult, error) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonSetName,
			Namespace: devConfig.Namespace,
		},
	}
	return controllerutil.CreateOrPatch(ctx, x.client, ds, func() error {
		labels := map[string]string{
			"app.kubernetes.io/name": appK8sName,
			"app":                    appLabel,
		}
		ds.Labels = labels
		ds.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: labels,
		}

		automount := false
		runAsUser := int64(0)
		readOnly := true
		noEscalation := false
		directoryOrCreate := corev1.HostPathDirectoryOrCreate
		seccompRuntime := corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}

		ds.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				AutomountServiceAccountToken: &automount,
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &seccompRuntime,
				},
				NodeSelector: map[string]string{
					nfd.GPULabel: "true",
				},
				Containers: []corev1.Container{
					{
						Name:            containerName,
						Image:           devConfig.Spec.XPUManager.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Args:            []string{"--config=/etc/xpumd/config.yaml"},
						SecurityContext: &corev1.SecurityContext{
							RunAsUser:                &runAsUser,
							ReadOnlyRootFilesystem:   &readOnly,
							AllowPrivilegeEscalation: &noEscalation,
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"ALL"},
								Add:  []corev1.Capability{"SYS_ADMIN"},
							},
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          httpPortName,
								ContainerPort: httpPort,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "config", MountPath: "/etc/xpumd", ReadOnly: true},
							{Name: "rundir", MountPath: "/run/xpumd"},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
							},
						},
					},
					{
						Name: "rundir",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/run/xpumd",
								Type: &directoryOrCreate,
							},
						},
					},
				},
			},
		}

		return controllerutil.SetControllerReference(devConfig, ds, x.scheme)
	})
}

func (x *xpuManager) ensureServiceMonitor(ctx context.Context, devConfig *intelv1alpha1.DeviceConfig) (controllerutil.OperationResult, error) {
	sm := &unstructured.Unstructured{}
	sm.SetGroupVersionKind(serviceMonitorGVK())
	sm.SetName(serviceName)
	sm.SetNamespace(devConfig.Namespace)

	return controllerutil.CreateOrPatch(ctx, x.client, sm, func() error {
		sm.SetLabels(map[string]string{
			"app": appLabel,
		})
		if err := unstructured.SetNestedField(sm.Object, map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"app": appLabel,
			},
		}, "spec", "selector"); err != nil {
			return err
		}
		if err := unstructured.SetNestedField(sm.Object, map[string]interface{}{
			"matchNames": []interface{}{devConfig.Namespace},
		}, "spec", "namespaceSelector"); err != nil {
			return err
		}
		if err := unstructured.SetNestedSlice(sm.Object, []interface{}{
			map[string]interface{}{
				"port":     httpPortName,
				"path":     "/metrics",
				"interval": "5s",
			},
		}, "spec", "endpoints"); err != nil {
			return err
		}
		return controllerutil.SetControllerReference(devConfig, sm, x.scheme)
	})
}

func serviceMonitorGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "monitoring.coreos.com",
		Version: "v1",
		Kind:    "ServiceMonitor",
	}
}
