#!/usr/bin/env bash
set -euo pipefail

NS="${1:?namespace required}"
MINIO_MC_IMAGE="${2:?expected MinIO client image required}"
LEGACY_MINIO_MC_IMAGE='quay.io/minio/mc:RELEASE.2025-08-13T08-35-41Z@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727'

if [ "${NS}" != voice-staging ]; then
  echo 'ERROR: MinIO bucket Job replacement is restricted to voice-staging' >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo 'ERROR: jq is required to validate existing MinIO bucket Jobs' >&2
  exit 1
fi

for bucket in avatars files; do
  job="voice-minio-create-${bucket}-bucket"
  if ! existing="$(kubectl get job "${job}" -n "${NS}" -o json --ignore-not-found=true 2>/dev/null)"; then
    echo "ERROR: unable to inspect MinIO bucket Job ${job}; refusing manifest apply" >&2
    exit 1
  fi
  if [ -z "${existing}" ] || [ "${existing}" = null ]; then
    continue
  fi

  if ! printf '%s' "${existing}" | jq -e --arg name "${job}" --arg namespace "${NS}" --arg bucket "${bucket}" --arg image "${MINIO_MC_IMAGE}" --arg legacy "${LEGACY_MINIO_MC_IMAGE}" '
    .metadata.name == $name and .metadata.namespace == $namespace and
    .spec.backoffLimit == 10 and
    ((.spec | keys - ["backoffLimit", "template", "parallelism", "completions", "completionMode", "suspend", "selector", "manualSelector", "podReplacementPolicy"]) | length) == 0 and
    ((.spec.parallelism // 1) == 1) and ((.spec.completions // 1) == 1) and
    ((.spec.completionMode // "NonIndexed") == "NonIndexed") and ((.spec.suspend // false) == false) and
    ((.spec.manualSelector // false) == false) and ((.spec.podReplacementPolicy // "TerminatingOrFailed") == "TerminatingOrFailed") and
    (.metadata.uid as $uid | .spec.selector.matchLabels as $labels | .spec.template.metadata.labels as $templateLabels |
      ($labels | type == "object" and ((keys == ["controller-uid"]) or (keys == ["batch.kubernetes.io/controller-uid"]))) and
      ((.spec.selector | keys) == ["matchLabels"]) and
      (($labels["controller-uid"] // $labels["batch.kubernetes.io/controller-uid"]) == $uid) and
      ($templateLabels | type == "object" and keys == ["batch.kubernetes.io/controller-uid", "batch.kubernetes.io/job-name", "controller-uid", "job-name"]) and
      $templateLabels["controller-uid"] == $uid and
      $templateLabels["batch.kubernetes.io/controller-uid"] == $uid and
      $templateLabels["job-name"] == $name and
      $templateLabels["batch.kubernetes.io/job-name"] == $name and
      all($labels | to_entries[]; . as $entry | $templateLabels[$entry.key] == $entry.value) and
      ((.spec.template.metadata | keys - ["labels"]) | length) == 0
    ) and
    .spec.template.spec.restartPolicy == "OnFailure" and
    ((.spec.template.spec | keys - ["restartPolicy", "containers", "dnsPolicy", "schedulerName", "terminationGracePeriodSeconds", "enableServiceLinks", "securityContext"]) | length) == 0 and
    ((.spec.template.spec.dnsPolicy // "ClusterFirst") == "ClusterFirst") and
    ((.spec.template.spec.schedulerName // "default-scheduler") == "default-scheduler") and
    ((.spec.template.spec.terminationGracePeriodSeconds // 30) == 30) and
    ((.spec.template.spec.enableServiceLinks // true) == true) and
    ((.spec.template.spec.securityContext // {}) == {}) and
    ((.spec.template.spec.containers | length) == 1) and
    (.spec.template.spec.containers[0] as $container |
      $container.name == "mc" and
      ($container.image == $image or $container.image == $legacy) and
      $container.args == ["mb", "--ignore-existing", ("local/voice-staging-" + $bucket)] and
      (($container | keys - ["name", "image", "args", "env", "imagePullPolicy", "terminationMessagePath", "terminationMessagePolicy", "resources"]) | length) == 0 and
      (($container.imagePullPolicy // "IfNotPresent") == "IfNotPresent") and
      (($container.terminationMessagePath // "/dev/termination-log") == "/dev/termination-log") and
      (($container.terminationMessagePolicy // "File") == "File") and
      (($container.resources // {}) == {}) and
      ($container.env | length) == 3 and
      ([ $container.env[] | select(.name == "MINIO_ROOT_USER" and .valueFrom.secretKeyRef.name == "voice-minio-credentials" and .valueFrom.secretKeyRef.key == "MINIO_ROOT_USER" and ((.valueFrom.secretKeyRef.optional // false) == false)) ] | length) == 1 and
      ([ $container.env[] | select(.name == "MINIO_ROOT_PASSWORD" and .valueFrom.secretKeyRef.name == "voice-minio-credentials" and .valueFrom.secretKeyRef.key == "MINIO_ROOT_PASSWORD" and ((.valueFrom.secretKeyRef.optional // false) == false)) ] | length) == 1 and
      ([ $container.env[] | select(.name == "MINIO_ROOT_USER" and ((keys - ["name", "valueFrom"]) | length) == 0 and ((.valueFrom | keys - ["secretKeyRef"]) | length) == 0 and ((.valueFrom.secretKeyRef | keys - ["name", "key", "optional"]) | length) == 0) ] | length) == 1 and
      ([ $container.env[] | select(.name == "MINIO_ROOT_PASSWORD" and ((keys - ["name", "valueFrom"]) | length) == 0 and ((.valueFrom | keys - ["secretKeyRef"]) | length) == 0 and ((.valueFrom.secretKeyRef | keys - ["name", "key", "optional"]) | length) == 0) ] | length) == 1 and
      ([ $container.env[] | select(.name == "MC_HOST_local" and .value == "http://$(MINIO_ROOT_USER):$(MINIO_ROOT_PASSWORD)@voice-minio:9000" and ((keys - ["name", "value"]) | length) == 0) ] | length) == 1
    )
  ' >/dev/null 2>&1; then
    echo "ERROR: MinIO bucket Job ${job} has an unexpected spec; refusing manifest apply" >&2
    exit 1
  fi

  current_image="$(printf '%s' "${existing}" | jq -er '.spec.template.spec.containers[0].image')"
  uid="$(printf '%s' "${existing}" | jq -er '.metadata.uid')"
  if [ "${current_image}" = "${MINIO_MC_IMAGE}" ]; then
    continue
  fi
  if [ "${current_image}" != "${LEGACY_MINIO_MC_IMAGE}" ] || ! printf '%s' "${existing}" | jq -e '
    .status.succeeded == 1 and ((.status.active // 0) == 0) and ((.status.failed // 0) == 0) and
    any(.status.conditions[]?; .type == "Complete" and .status == "True")
  ' >/dev/null 2>&1; then
    echo "ERROR: MinIO bucket Job ${job} is not a completed known-image Job; refusing manifest apply" >&2
    exit 1
  fi

  delete_options="$(jq -cn --arg uid "${uid}" '{apiVersion:"meta.k8s.io/v1",kind:"DeleteOptions",preconditions:{uid:$uid}}')"
  printf '%s' "${delete_options}" | MSYS_NO_PATHCONV=1 kubectl delete --raw="/apis/batch/v1/namespaces/${NS}/jobs/${job}" -f - >/dev/null
  kubectl wait --for=delete "job/${job}" -n "${NS}" --timeout=60s >/dev/null
done
