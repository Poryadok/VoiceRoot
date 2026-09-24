#!/usr/bin/env bash
# Read-only inspection of staging's NATS PVC prerequisites.
set -euo pipefail

namespace="${VOICE_K8S_NAMESPACE:-voice-staging}"
storage_class="${VOICE_NATS_STORAGE_CLASS:-}"
if [ -z "$storage_class" ]; then
  echo 'ERROR: set VOICE_NATS_STORAGE_CLASS to the class approved for staging.' >&2
  echo 'Read-only class listing:' >&2
  kubectl get storageclass -o custom-columns=NAME:.metadata.name,PROVISIONER:.provisioner,DEFAULT:.metadata.annotations.storageclass\.kubernetes\.io/is-default-class,BINDING:.volumeBindingMode
  exit 2
fi

kubectl get storageclass "$storage_class" -o custom-columns=NAME:.metadata.name,PROVISIONER:.provisioner,DEFAULT:.metadata.annotations.storageclass\.kubernetes\.io/is-default-class,BINDING:.volumeBindingMode,EXPAND:.allowVolumeExpansion
kubectl auth can-i create persistentvolumeclaims -n "$namespace"
kubectl get csistoragecapacities.storage.k8s.io -A -o custom-columns=NAMESPACE:.metadata.namespace,CLASS:.storageClassName,CAPACITY:.capacity,NODE:.nodeTopology.matchLabels.kubernetes\.io/hostname 2>/dev/null || {
  echo 'CSIStorageCapacity API unavailable; provisioner capacity cannot be established by this read-only check.'
}
echo 'Capacity still requires an operator-confirmed provisioning check; no PVC or workload was created.'
