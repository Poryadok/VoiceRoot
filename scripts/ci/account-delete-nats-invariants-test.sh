#!/usr/bin/env bash
# Static contract for the disabled account-delete lifecycle activation surface.
set -euo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"

for env in staging prod; do
  infra="deploy/${env}/infra.yaml"
  services="deploy/${env}/services.yaml"
  secrets="deploy/${env}/secret.example.yaml"
  grep -A12 'name: voice-nats' "$infra" | grep -q 'type: ClusterIP'
  grep -q 'USER_ACCOUNT_DELETE_CONSUMER_ENABLED, value: "false"' "$services"
  ! grep -q 'voice-nats-account-delete' "$infra"
  ! grep -q 'AUTH_NATS_CREDS_FILE' "$services"
  ! grep -q 'USER_ACCOUNT_DELETE_NATS_CREDS_FILE' "$services"
done

grep -q 'USER_ACCOUNT_DELETE_NATS_URL' src/backend/user/account_deletion_config.go
! grep -q 'getenv("NATS_URL")' src/backend/user/account_deletion_config.go
grep -q 'options.credentialPath(credentialsFile)' src/backend/auth/src/main/java/voice/backend/auth/events/NatsAuthEventPublisher.java
grep -q 'voice-migrate-user-db' deploy/templates/migrate-user-db-job.yaml
grep -q 'USER_DATABASE_URL' deploy/templates/migrate-user-db-job.yaml
grep -q 'apply_migrate user_db' scripts/staging/apply-migrate-jobs.sh
grep -q 'user.account_deleted' deploy/templates/network-policy-nats-account-delete.yaml
grep -q 'port: 4222' deploy/templates/network-policy-nats-account-delete.yaml
grep -q 'port: 8222' deploy/templates/network-policy-nats-account-delete.yaml

echo 'account-delete NATS deployment invariants passed.'
