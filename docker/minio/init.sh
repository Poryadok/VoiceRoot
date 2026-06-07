#!/bin/sh
set -eu

MINIO_HOST="${MINIO_HOST:-minio}"
MINIO_PORT="${MINIO_PORT:-9000}"
MINIO_USER="${MINIO_ROOT_USER:-voice-minio}"
MINIO_PASSWORD="${MINIO_ROOT_PASSWORD:-voice-minio-dev}"
AVATARS_BUCKET="${AVATARS_BUCKET:-voice-dev-avatars}"
FILES_BUCKET="${FILES_BUCKET:-voice-dev-files}"

echo "minio-init: waiting for MinIO at ${MINIO_HOST}:${MINIO_PORT}..."
until mc alias set local "http://${MINIO_HOST}:${MINIO_PORT}" "${MINIO_USER}" "${MINIO_PASSWORD}" 2>/dev/null; do
  sleep 2
done

mc mb "local/${AVATARS_BUCKET}" --ignore-existing
mc mb "local/${FILES_BUCKET}" --ignore-existing

mc anonymous set download "local/${AVATARS_BUCKET}"

echo "minio-init: buckets ${AVATARS_BUCKET}, ${FILES_BUCKET} ready"
