#!/usr/bin/env bash
# Remove generated A1 test identities from one local Docker Compose project.
# This is deliberately not a deployment or staging database operation.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT"

usage() {
  cat <<'EOF'
Usage:
  scripts/dev/reset-a1-test-accounts.sh --dry-run [--project NAME]
  VOICE_LOCAL_TEST_ACCOUNT_RESET=DELETE_GENERATED_TEST_ACCOUNTS \
    scripts/dev/reset-a1-test-accounts.sh --apply [--project NAME]

The script only addresses Auth- and User-owned identity rows in a local Docker
Compose PostgreSQL container. It leaves migrations and all other service data
untouched. Stop the local auth and user services before --apply, then restart
the Compose app stack so they rebuild caches from the empty identity stores.
EOF
}

mode=""
project="${VOICE_TEST_ACCOUNT_RESET_PROJECT:-voice}"
while (($#)); do
  case "$1" in
    --dry-run|--apply)
      [[ -z "$mode" ]] || { echo "choose exactly one of --dry-run or --apply" >&2; exit 2; }
      mode="$1" ;;
    --project)
      shift
      [[ $# -gt 0 ]] || { echo "--project requires a name" >&2; exit 2; }
      project="$1" ;;
    --help|-h) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

[[ -n "$mode" ]] || { usage >&2; exit 2; }
[[ "$project" =~ ^voice(-[a-z0-9][a-z0-9-]*)?$ ]] || {
  echo "refusing non-Voice local Compose project: $project" >&2; exit 2;
}
project_lower="${project,,}"
case "$project_lower" in *prod*|*stage*)
  echo "refusing staging or production-like Compose project: $project" >&2; exit 2 ;;
esac
case "${VOICE_DEPLOYMENT_ENV:-}" in production|prod|staging)
  echo "refusing deployment environment: ${VOICE_DEPLOYMENT_ENV}" >&2; exit 2 ;;
esac
[[ -z "${KUBERNETES_SERVICE_HOST:-}" ]] || { echo "refusing Kubernetes environment" >&2; exit 2; }

compose=(docker compose --project-name "$project" --project-directory "$ROOT" -f "$ROOT/docker-compose.yml")
postgres_id="$("${compose[@]}" ps -q postgres 2>/dev/null || true)"
[[ -n "$postgres_id" ]] || { echo "local Compose postgres is not running for project $project" >&2; exit 1; }
[[ "$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.project" }}' "$postgres_id")" == "$project" ]] || {
  echo "refusing postgres container outside requested Compose project" >&2; exit 1;
}
[[ "$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.service" }}' "$postgres_id")" == "postgres" ]] || {
  echo "refusing non-postgres Compose service" >&2; exit 1;
}
[[ "$(docker inspect -f '{{.Config.Image}}' "$postgres_id")" == "postgres:16-alpine" ]] || {
  echo "refusing unexpected postgres image" >&2; exit 1;
}

for service in auth user; do
  service_id="$("${compose[@]}" ps -q "$service" 2>/dev/null || true)"
  if [[ -n "$service_id" ]] && [[ "$(docker inspect -f '{{.State.Running}}' "$service_id")" == "true" ]]; then
    echo "refusing while $service is running; stop auth and user before reset" >&2
    exit 1
  fi
done

counts_sql="SELECT 'auth.accounts' AS table_name, count(*) FROM accounts
UNION ALL SELECT 'auth.refresh_tokens', count(*) FROM refresh_tokens
UNION ALL SELECT 'auth.otp_codes', count(*) FROM otp_codes
UNION ALL SELECT 'auth.backup_codes', count(*) FROM backup_codes
UNION ALL SELECT 'auth.linked_identities', count(*) FROM linked_identities
UNION ALL SELECT 'auth.e2e_key_backups', count(*) FROM e2e_key_backups
UNION ALL SELECT 'auth.guest_conversion_operations', count(*) FROM guest_conversion_operations
UNION ALL SELECT 'auth.account_deletion_operations', count(*) FROM account_deletion_operations;"
user_counts_sql="SELECT 'user.profiles' AS table_name, count(*) FROM profiles
UNION ALL SELECT 'user.onboarding_state', count(*) FROM onboarding_state
UNION ALL SELECT 'user.privacy_settings', count(*) FROM privacy_settings
UNION ALL SELECT 'user.organization_verification_requests', count(*) FROM organization_verification_requests;"

echo "==> target: local Docker Compose project $project ($postgres_id)"
echo "==> Auth-owned test identity rows"
"${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-voice}" -d auth_db -c "$counts_sql"
echo "==> User-owned test identity rows"
"${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-voice}" -d user_db -c "$user_counts_sql"

if [[ "$mode" == "--dry-run" ]]; then
  echo "dry run only; no rows were deleted"
  exit 0
fi
[[ "${VOICE_LOCAL_TEST_ACCOUNT_RESET:-}" == "DELETE_GENERATED_TEST_ACCOUNTS" ]] || {
  echo "--apply requires VOICE_LOCAL_TEST_ACCOUNT_RESET=DELETE_GENERATED_TEST_ACCOUNTS" >&2; exit 2;
}

"${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-voice}" -d auth_db <<'SQL'
BEGIN;
TRUNCATE TABLE account_deletion_operations, guest_conversion_operations,
  linked_identities, e2e_key_backups, backup_codes, otp_codes, refresh_tokens,
  accounts RESTART IDENTITY;
COMMIT;
SQL
"${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-voice}" -d user_db <<'SQL'
BEGIN;
TRUNCATE TABLE organization_verification_requests, privacy_settings,
  onboarding_state, profiles RESTART IDENTITY;
COMMIT;
SQL
echo "A1 generated test identities were removed from local Compose project $project."
