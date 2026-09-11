#!/usr/bin/env bash
# Prove compose-db-init upgrades a PostgreSQL volume that predates voice_db,
# without changing data already stored in another service database.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
readonly label_key='voice.r22.existing-volume'
readonly run_id="$(date +%s)-${RANDOM}${RANDOM}"
readonly project="voice-r22-f14-${run_id}"
readonly volume="${project}_voice_pgdata"
readonly bootstrap="${project}-bootstrap"
readonly sentinel_hex='006f6c642d766f6c756d652d73656e74696e656cff'
compose_started=false
bootstrap_created=false
volume_created=false

compose() {
  COMPOSE_PROJECT_NAME="${project}" POSTGRES_PORT=0 \
    docker compose -f "${ROOT}/docker-compose.yml" "$@"
}

volume_is_owned() {
  [[ "$(docker volume inspect --format "{{index .Labels \"${label_key}\"}}" "${volume}" 2>/dev/null)" == "${run_id}" ]] &&
    [[ "$(docker volume inspect --format '{{index .Labels "com.docker.compose.project"}}' "${volume}" 2>/dev/null)" == "${project}" ]] &&
    [[ "$(docker volume inspect --format '{{index .Labels "com.docker.compose.volume"}}' "${volume}" 2>/dev/null)" == 'voice_pgdata' ]]
}

project_containers_are_owned() {
  local ids id
  ids="$(docker ps -aq --filter "label=com.docker.compose.project=${project}" 2>/dev/null || true)"
  while IFS= read -r id; do
    [[ -z "${id}" ]] && continue
    [[ "$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project"}}' "${id}" 2>/dev/null)" == "${project}" ]] || return 1
  done <<<"${ids}"
}

cleanup() {
  local status=$?
  set +e
  if [[ "${bootstrap_created}" == true ]] &&
    [[ "$(docker inspect --format "{{index .Config.Labels \"${label_key}\"}}" "${bootstrap}" 2>/dev/null)" == "${run_id}" ]]; then
    docker rm -f "${bootstrap}" >/dev/null
  fi
  if [[ "${compose_started}" == true ]] && volume_is_owned && project_containers_are_owned; then
    compose down -v --remove-orphans >/dev/null
    volume_created=false
  fi
  if [[ "${volume_created}" == true ]] && volume_is_owned; then
    docker volume rm "${volume}" >/dev/null
  fi
  return "${status}"
}
trap cleanup EXIT

fail() {
  echo "compose-existing-volume-voice-db: $*" >&2
  compose ps >&2 || true
  compose logs postgres compose-db-init --tail 100 >&2 || true
  exit 1
}

docker_exec() {
  MSYS_NO_PATHCONV=1 docker exec "$@"
}

wait_for_postgres() {
  local container="$1"
  local deadline=$((SECONDS + 90))
  until docker_exec "${container}" pg_isready -h 127.0.0.1 -p 5432 -U voice -d voice >/dev/null 2>&1; do
    ((SECONDS < deadline)) || fail "timed out waiting for PostgreSQL in ${container}"
    sleep 1
  done
}

query() {
  local database="$1"
  local sql="$2"
  compose exec -T postgres psql -v ON_ERROR_STOP=1 -Atq -U voice -d "${database}" -c "${sql}"
}

docker image inspect postgres:16-alpine >/dev/null || fail 'postgres:16-alpine is required locally'

docker volume create \
  --label "${label_key}=${run_id}" \
  --label "com.docker.compose.project=${project}" \
  --label 'com.docker.compose.volume=voice_pgdata' \
  "${volume}" >/dev/null
volume_created=true
volume_is_owned || fail 'refusing to use an existing volume without the exact test labels'

docker run -d --name "${bootstrap}" \
  --label "${label_key}=${run_id}" \
  -e POSTGRES_USER=voice \
  -e POSTGRES_PASSWORD=voice \
  -e POSTGRES_DB=voice \
  -v "${volume}:/var/lib/postgresql/data" \
  postgres:16-alpine >/dev/null
bootstrap_created=true
wait_for_postgres "${bootstrap}"

docker_exec "${bootstrap}" psql -v ON_ERROR_STOP=1 -U voice -d voice -c 'CREATE DATABASE chat_db' >/dev/null
docker_exec "${bootstrap}" psql -v ON_ERROR_STOP=1 -U voice -d chat_db -c \
  "CREATE TABLE r22_existing_volume_sentinel (id integer PRIMARY KEY, payload bytea NOT NULL); INSERT INTO r22_existing_volume_sentinel VALUES (1, decode('${sentinel_hex}', 'hex'));" >/dev/null
docker rm -f "${bootstrap}" >/dev/null
bootstrap_created=false

compose_started=true
compose up -d postgres
postgres_id="$(compose ps -q postgres)"
[[ -n "${postgres_id}" ]] || fail 'Compose PostgreSQL container was not created'
wait_for_postgres "${postgres_id}"

first_output="$(compose --profile app run --rm compose-db-init 2>&1)" || {
  printf '%s\n' "${first_output}" >&2
  fail 'first compose-db-init run failed'
}

[[ "$(query voice "SELECT count(*) FROM pg_database WHERE datname = 'voice_db'")" == '1' ]] || fail 'voice_db was not created on the existing volume'
schema_state="$(query voice_db 'SELECT version::text || chr(58) || dirty::text FROM schema_migrations LIMIT 1')"
[[ "${schema_state}" == '1:false' ]] || fail "expected clean Voice migration version 1, got ${schema_state}"

tables="$(query voice_db "SELECT string_agg(tablename, ',' ORDER BY tablename) FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'schema_migrations'")"
expected_tables='voice_event_outbox,voice_lifecycle_effects,voice_lifecycle_operations,voice_media_epoch_denials,voice_room_instances,voice_room_memberships'
[[ "${tables}" == "${expected_tables}" ]] || fail "unexpected Voice tables: ${tables}"

sentinel_before="$(query chat_db 'SELECT encode(payload, '\''hex'\'') FROM r22_existing_volume_sentinel WHERE id = 1')"
[[ "${sentinel_before}" == "${sentinel_hex}" ]] || fail 'older database sentinel changed during first init'

second_output="$(compose --profile app run --rm compose-db-init 2>&1)" || {
  printf '%s\n' "${second_output}" >&2
  fail 'second compose-db-init run failed'
}
schema_state_second="$(query voice_db 'SELECT version::text || chr(58) || dirty::text FROM schema_migrations LIMIT 1')"
sentinel_after="$(query chat_db 'SELECT encode(payload, '\''hex'\'') FROM r22_existing_volume_sentinel WHERE id = 1')"
[[ "${schema_state_second}" == '1:false' ]] || fail "second init changed Voice migration state to ${schema_state_second}"
[[ "${sentinel_after}" == "${sentinel_before}" ]] || fail 'second init changed the older database sentinel'

if ! grep -Eiq '(no change|already.*current|up to date)' <<<"${second_output}"; then
  fail 'second compose-db-init run did not report an idempotent migration result'
fi

query voice_db 'UPDATE schema_migrations SET dirty = true WHERE version = 1' >/dev/null
dirty_before="$(query voice_db 'SELECT version::text || chr(58) || dirty::text FROM schema_migrations LIMIT 1')"
[[ "${dirty_before}" == '1:true' ]] || fail "failed to prepare dirty Voice migration state: ${dirty_before}"
set +e
dirty_output="$(compose --profile app run --rm compose-db-init 2>&1)"
dirty_status=$?
set -e
dirty_after="$(query voice_db 'SELECT version::text || chr(58) || dirty::text FROM schema_migrations LIMIT 1')"
dirty_contract_failed=false
if [[ "${dirty_status}" -eq 0 ]]; then
  echo 'dirty voice_db must fail closed instead of running migrate force' >&2
  dirty_contract_failed=true
fi
if [[ "${dirty_after}" != "${dirty_before}" ]]; then
  echo "dirty Voice migration state mutated: ${dirty_before} -> ${dirty_after}" >&2
  dirty_contract_failed=true
fi
if ! grep -Eiq '(dirty.*voice_db|voice_db.*dirty)' <<<"${dirty_output}"; then
  echo 'dirty Voice migration failure must identify voice_db' >&2
  dirty_contract_failed=true
fi
[[ "${dirty_contract_failed}" == false ]] || fail 'dirty Voice migration fail-closed contract failed'

echo 'compose-existing-volume-voice-db: PASS'
