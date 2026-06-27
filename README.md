# Intel GPU Operator PoC

A proof-of-concept Kubernetes/OpenShift operator for Intel GPU enablement, built on the [gpu-operator-templater](https://github.com/yevgeny-shnaidman/gpu-operator-templater) framework. Delegates driver lifecycle to [KMM](https://github.com/rh-ecosystem-edge/kernel-module-management) and GPU exposure to [DRA](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/).

## What it does

A single `DeviceConfig` CR triggers the full GPU enablement chain:

1. **NFD NodeFeatureRule** — labels nodes with Intel GPUs (`intel.feature.node.kubernetes.io/gpu`)
2. **KMM Module** — manages the DRA kubelet plugin DaemonSet, DeviceClasses, and optionally the OOT kernel driver
3. **XPU Manager** — DaemonSet for GPU telemetry (energy, frequency, utilization) via Prometheus metrics
4. **ServiceMonitor** — auto-created if prometheus-operator is installed

```
DeviceConfig CR
  |
  +-> NodeFeatureRule (NFD)
  +-> Module CR (KMM)
  |     +-> DRA DaemonSet        (created by KMM)
  |     +-> DeviceClasses         (created by KMM)
  |     +-> ResourceSlices        (published by DRA plugin)
  +-> XPU Manager DaemonSet
  +-> ServiceMonitor
```

## Prerequisites

- Kubernetes 1.35+ / OpenShift 4.22+ with DRA enabled (`resource.k8s.io/v1`)
- [NFD Operator](https://github.com/kubernetes-sigs/node-feature-discovery) installed
- [KMM](https://github.com/rh-ecosystem-edge/kernel-module-management) installed (requires [PR #1836](https://github.com/rh-ecosystem-edge/kernel-module-management/pull/1836) for in-tree driver + DRA mode)
- Intel GPU node with `xe` or `i915` driver loaded

## Quick start

```bash
# Build and push the operator image
make docker-build docker-push IMG=quay.io/abyrne_openshift/intel-gpu-operator-poc:latest

# Deploy to the cluster
make deploy IMG=quay.io/abyrne_openshift/intel-gpu-operator-poc:latest

# Create the DeviceConfig
kubectl apply -f - <<EOF
apiVersion: intel.com/v1alpha1
kind: DeviceConfig
metadata:
  name: gpu-config
  namespace: generated-system
spec:
  driver:
    useInTreeDriver: true
  dra:
    image: ghcr.io/intel/intel-resource-drivers-for-kubernetes/intel-gpu-resource-driver:v0.9.0
  xpuManager:
    enabled: true
EOF
```

### OpenShift-specific setup

The DRA and XPU Manager pods need privileged SCCs:

```bash
oc create sa intel-gpu-resource-driver-service-account -n generated-system
oc adm policy add-scc-to-user privileged -z default -n generated-system
oc adm policy add-scc-to-user privileged -z intel-gpu-resource-driver-service-account -n generated-system
```

The DRA service account also needs RBAC for ResourceSlice management:

```bash
kubectl apply -f config/samples/dra-rbac.yaml   # TODO: operator should reconcile this
```

## Driver modes

| Mode | `spec.driver.useInTreeDriver` | Description |
|------|------|-------------|
| **In-tree** (default) | `true` | Uses the kernel's built-in `xe` driver. No module loading. |
| **Out-of-tree** | `false` | KMM loads `xe` from `spec.driver.image`, replacing the in-tree module. Requires `spec.driver.version`. |

## Teardown

```bash
kubectl delete deviceconfig gpu-config -n generated-system
make undeploy
```

## Status

**Phase 1 complete** — core GPU operator validated end-to-end. See [#1](https://github.com/abyrne55/intel-gpu-operator-poc/issues/1) for phase 2 (firmware updates via fwupd).

Known limitations:
- Operator does not create the DRA ServiceAccount or RBAC (manual step on OpenShift)
- KMM's DRA NetworkPolicy blocks egress until [PR #1836](https://github.com/rh-ecosystem-edge/kernel-module-management/pull/1836) merges
- `DeviceConfig.status` is not populated
- Namespace is `generated-system` (templater default)

## Related

- [intel/gpu-base-operator](https://github.com/intel/gpu-base-operator) — Intel's official operator (different architecture, no KMM)
- [gpu-operator-templater](https://github.com/yevgeny-shnaidman/gpu-operator-templater) — framework this PoC is built on

