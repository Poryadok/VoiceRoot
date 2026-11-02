#!/usr/bin/env bash
# Run inside an owned disposable postgres:16-bookworm container with this repo
# mounted at /repo, with python3 installed:
# bash /repo/scripts/staging/apply-migrate-jobs-restored-db_test.sh
# PostgreSQL must be initialized with POSTGRES_PASSWORD=fixture. No real kubectl.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
definitions="$(awk '/^if ! kubectl get secret/{exit} !/^ROOT=/ {print}' "$ROOT/scripts/staging/apply-migrate-jobs.sh")"
eval "$definitions"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir "$work/sql"
printf 'SELECT 1;\n' > "$work/sql/000001_fixture.up.sql"
printf 'SELECT 2;\n' > "$work/sql/000002_fixture.up.sql"
hash=$(migration_content_hash "$work/sql")
calls="$work/calls"
fixture_target=restored_db
fixture_query=sslmode=disable
fixture_suffix=
unavailable=false
secret_missing=false
retained=true
kubectl() {
  printf '%s\n' "$1" >> "$calls"
  case "$1 ${2:-}" in
    'get configmap') printf '%s' "$hash" ;;
    'get job') [[ $retained == true ]] || return 1; [[ "$*" != *status.succeeded* ]] || printf 1 ;;
    'get secret')
      [[ $secret_missing == false ]] || return 1
      printf '%s' "postgres://postgres:fixture@127.0.0.1:5432/$fixture_target?$fixture_query$fixture_suffix" | base64 -w0 ;;
    'exec '*)
      [[ $unavailable == false ]] || return 1
      while [[ "$1" != -- ]]; do shift; done
      shift
      "$@" ;;
    'apply '*) cat >/dev/null ;;
    'annotate '*|'wait '*) ;;
    *) echo 'unexpected kubectl invocation' >&2; return 1 ;;
  esac
}
kubectl_apply_configmap() { echo configmap.apply >> "$calls"; }
cat > "$work/template" <<'EOF'
args: ["-path=/migrations", "-database=$(DATABASE_URL)", "up"]
secretKeyRef: {name: voice-app-secrets, key: RESTORED_DATABASE_URL}
EOF
sql() { psql -X -U postgres -d restored_db -v ON_ERROR_STOP=1 -qc "$1"; }
psql -X -U postgres -v ON_ERROR_STOP=1 -qc 'CREATE DATABASE restored_db'
sql 'CREATE TABLE schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(1,false); CREATE TABLE restored_payload(value text); INSERT INTO restored_payload VALUES($$preserved$$)'
pg_dump -U postgres restored_db > "$work/older.sql"
sql 'UPDATE schema_migrations SET version=2'

check_retained() {
  local label="$1" expected="$2" result output
  : > "$calls"
  set +e
  output=$(apply_migrate restored_db "$work/sql" "$work/template" retained-job retained-configmap 2>&1)
  result=$?
  set -e
  if [[ $expected == skip ]]; then
    [[ $result == 0 ]] || { echo "FAIL: $label rejected current clean target: $output"; exit 1; }
    grep -Fq skipping <<< "$output"
  else
    [[ $result != 0 ]] && grep -Fq 'HOLD:' <<< "$output" || {
      echo "FAIL: $label accepted retained Job without valid actual DB state"; exit 1;
    }
  fi
  if [[ ${3:-probe} == no-probe ]]; then
    ! grep -Fxq exec "$calls"
  elif [[ $secret_missing == false ]]; then
    grep -Fxq exec "$calls"
  fi
  ! grep -Eq '^(delete|apply|annotate|wait|configmap.apply)$' "$calls"
  ! grep -Fq 'fixture@' <<< "$output"
  echo "PASS: $label"
}

sql 'DROP SCHEMA public CASCADE; CREATE SCHEMA public'
psql -X -U postgres -d restored_db -v ON_ERROR_STOP=1 -qf "$work/older.sql"
check_retained retained-job-older-restored-db hold
[[ $(psql -X -U postgres -d restored_db -Atqc 'SELECT value FROM restored_payload') == preserved ]]
sql 'UPDATE schema_migrations SET version=2'
check_retained current-clean skip
fixture_query='sslmode=disable&dbname=other_db'
check_retained unsupported-query-target-selector hold no-probe
fixture_query='sslmode=disable&password=fixture'
check_retained unsupported-query-credential-selector hold no-probe
fixture_query=sslmode=disable
fixture_suffix=$'\n'
check_retained trailing-newline-secret hold no-probe
fixture_suffix=
sql 'UPDATE schema_migrations SET version=2,dirty=true'
check_retained dirty hold
sql 'UPDATE schema_migrations SET version=3,dirty=false'
check_retained ahead-of-source hold
sql 'UPDATE schema_migrations SET version=2'
unavailable=true
check_retained unavailable-target hold
unavailable=false
psql -X -U postgres -v ON_ERROR_STOP=1 -qc 'CREATE DATABASE other_db'
psql -X -U postgres -d other_db -v ON_ERROR_STOP=1 -qc 'CREATE TABLE schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(2,false)'
fixture_target=other_db
check_retained wrong-database-with-current-clean-ledger hold
fixture_target=restored_db
sql 'INSERT INTO schema_migrations VALUES(2,false)'
check_retained ambiguous-ledger hold
sql 'DROP TABLE schema_migrations'
check_retained missing-ledger hold
secret_missing=true
check_retained missing-target-secret hold
secret_missing=false
retained=false
: > "$calls"
apply_migrate restored_db "$work/sql" "$work/template" retained-job retained-configmap >/dev/null
grep -Fxq apply "$calls"
grep -Fxq wait "$calls"
! grep -Fxq exec "$calls"
echo 'PASS: fresh migration Job still applies and waits'
