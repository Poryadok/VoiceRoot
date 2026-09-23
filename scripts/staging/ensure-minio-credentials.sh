#!/usr/bin/env bash
# Restore the separately managed staging MinIO Secret without rotating an existing one.
set -euo pipefail

NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
SECRET=voice-minio-credentials

if kubectl get secret "${SECRET}" -n "${NS}" >/dev/null 2>&1; then
  if [ "$(kubectl get secret "${SECRET}" -n "${NS}" \
    -o go-template='{{if and (index .data "MINIO_ROOT_USER") (index .data "MINIO_ROOT_PASSWORD")}}ok{{end}}')" != ok ]; then
    echo "ERROR: ${SECRET} exists but lacks required keys in ${NS}" >&2
    exit 1
  fi
  echo "MinIO credentials already present in ${NS}"
  exit 0
fi

: "${STAGING_MINIO_ROOT_USER:?Set the staging environment secret STAGING_MINIO_ROOT_USER}"
: "${STAGING_MINIO_ROOT_PASSWORD:?Set the staging environment secret STAGING_MINIO_ROOT_PASSWORD}"

user_b64="$(printf '%s' "${STAGING_MINIO_ROOT_USER}" | base64 | tr -d '\r\n')"
password_b64="$(printf '%s' "${STAGING_MINIO_ROOT_PASSWORD}" | base64 | tr -d '\r\n')"
printf '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"%s"},"type":"Opaque","data":{"MINIO_ROOT_USER":"%s","MINIO_ROOT_PASSWORD":"%s"}}' \
  "${SECRET}" "${user_b64}" "${password_b64}" | kubectl create -n "${NS}" -f -
