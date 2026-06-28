#!/usr/bin/env bash
# Start compose Postgres+Redis, run the Auth image, verify HTTP /health, JWKS REST, gRPC GetJWKS.
# Usage: auth-container-smoke.sh [REPO_ROOT]
# Env: AUTH_IMAGE (default voice-auth:ci), AUTH_HTTP_PORT (18080), AUTH_GRPC_PORT (19090),
#      JWT_KEY_PATH (override path to PKCS#8 PEM)
set -euo pipefail

ROOT="${1:-$(pwd)}"
AUTH_IMAGE="${AUTH_IMAGE:-voice-auth:ci}"
HTTP_PORT="${AUTH_HTTP_PORT:-18080}"
GRPC_PORT="${AUTH_GRPC_PORT:-19090}"
CONTAINER_NAME="${AUTH_SMOKE_CONTAINER:-auth-smoke}"
JWT_KEY="${JWT_KEY_PATH:-$ROOT/src/backend/auth/src/test/resources/jwt-test-private.pem}"
# Docker Desktop on Windows (Git Bash / MSYS): bind mount source must be //drive/path, not /d/path.
JWT_DOCKER_VOL="$JWT_KEY"
if [[ "$JWT_DOCKER_VOL" =~ ^/([a-zA-Z])/(.*)$ ]]; then
  case "$(uname -s)" in
    CYGWIN* | MINGW* | MSYS*)
      JWT_DOCKER_VOL="//${BASH_REMATCH[1]}/${BASH_REMATCH[2]}"
      ;;
  esac
fi

cleanup() {
  set +e
  docker rm -f "$CONTAINER_NAME" 2>/dev/null
  if [[ -d "$ROOT" ]]; then
    (cd "$ROOT" && docker compose down)
  fi
}

install_grpcurl() {
  if command -v grpcurl >/dev/null 2>&1; then
    return 0
  fi
  local ver="1.9.2"
  local url tmp installdir suffix archive_fmt=tgz
  tmp="$(mktemp)"
  installdir="$(mktemp -d)"
  unames="$(uname -s)"
  unm="$(uname -m)"
  suffix=""
  case "${unames}-${unm}" in
  Linux-x86_64) suffix="linux_x86_64" ;;
  Linux-aarch64 | Linux-arm64) suffix="linux_arm64" ;;
  Darwin-x86_64) suffix="osx_x86_64" ;;
  Darwin-arm64) suffix="osx_arm64" ;;
  esac
  if [[ -z "$suffix" ]] && [[ "$unames" == MINGW*_NT-* || "$unames" == MSYS* ]] && [[ "$unm" == x86_64 ]]; then
    suffix="windows_x86_64"
    archive_fmt="zip"
  fi
  if [[ -z "$suffix" ]]; then
    echo "unsupported platform: ${unames} ${unm}" >&2
    return 1
  fi
  local curl_opts=(-fsSL)
  case "${unames}" in
  MINGW*_NT-* | MSYS*) curl_opts+=(--ssl-no-revoke) ;;
  esac
  if [[ "$archive_fmt" == zip ]]; then
    url="https://github.com/fullstorydev/grpcurl/releases/download/v${ver}/grpcurl_${ver}_${suffix}.zip"
    curl "${curl_opts[@]}" "$url" -o "$tmp"
    unzip -qo "$tmp" -d "$installdir"
    mv "${installdir}/grpcurl.exe" "${installdir}/grpcurl"
  else
    url="https://github.com/fullstorydev/grpcurl/releases/download/v${ver}/grpcurl_${ver}_${suffix}.tar.gz"
    curl "${curl_opts[@]}" "$url" -o "$tmp"
    tar -xzf "$tmp" -C "$installdir" grpcurl
  fi
  rm -f "$tmp"
  chmod +x "${installdir}/grpcurl"
  export PATH="$installdir:$PATH"
}

trap cleanup EXIT

cd "$ROOT"

if [[ ! -f "$JWT_KEY" ]]; then
  echo "JWT key not found: $JWT_KEY" >&2
  exit 1
fi

if [[ ! -f "$ROOT/protos/voice/auth/v1/auth.proto" ]]; then
  echo "Proto not found under $ROOT/protos" >&2
  exit 1
fi

echo "Starting postgres and redis..."
docker compose up -d postgres redis

echo "Waiting for postgres..."
for _ in $(seq 1 60); do
  if docker compose exec -T postgres pg_isready -U voice -d voice >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
if ! docker compose exec -T postgres pg_isready -U voice -d voice >/dev/null 2>&1; then
  echo "Postgres not ready" >&2
  docker compose ps
  docker compose logs postgres --tail 80 >&2 || true
  exit 1
fi

echo "Waiting for postgres init (chat_db)..."
for _ in $(seq 1 60); do
  if docker compose exec -T postgres psql -U voice -d voice -tAc \
    "SELECT 1 FROM pg_database WHERE datname='chat_db'" 2>/dev/null | grep -q 1; then
    break
  fi
  sleep 2
done
if ! docker compose exec -T postgres psql -U voice -d voice -tAc \
  "SELECT 1 FROM pg_database WHERE datname='chat_db'" 2>/dev/null | grep -q 1; then
  echo "chat_db not ready" >&2
  docker compose ps
  docker compose logs postgres --tail 80 >&2 || true
  exit 1
fi

echo "Waiting for redis..."
for _ in $(seq 1 30); do
  if docker compose exec -T redis redis-cli ping 2>/dev/null | grep -q PONG; then
    break
  fi
  sleep 1
done
if ! docker compose exec -T redis redis-cli ping 2>/dev/null | grep -q PONG; then
  echo "Redis not ready" >&2
  docker compose ps
  docker compose logs redis --tail 40 >&2 || true
  exit 1
fi

if ! docker compose exec -T postgres psql -U voice -d user_db -tAc "SELECT to_regclass('public.profiles')" | grep -q profiles; then
  echo "Applying user_db schema for smoke..."
  cat "$ROOT/docker/postgres/user_db_init.sql.snippet" | docker compose exec -T postgres psql -U voice -d user_db -v ON_ERROR_STOP=1 -f -
fi

POSTGRES_CID=$(docker compose ps -q postgres)
NETWORK=$(docker inspect "$POSTGRES_CID" --format '{{range $k, $v := .NetworkSettings.Networks}}{{$k}}{{"\n"}}{{end}}' | head -1)
if [[ -z "$NETWORK" ]]; then
  echo "Could not detect compose network from postgres container" >&2
  exit 1
fi
echo "Using Docker network: $NETWORK"

docker rm -f "$CONTAINER_NAME" 2>/dev/null || true

echo "Starting auth container ($AUTH_IMAGE)..."
docker run -d --name "$CONTAINER_NAME" --network "$NETWORK" \
  -e SPRING_DATASOURCE_URL=jdbc:postgresql://postgres:5432/auth_db \
  -e SPRING_DATASOURCE_USERNAME=voice \
  -e SPRING_DATASOURCE_PASSWORD=voice \
  -e AUTH_USER_DB_JDBC_URL=jdbc:postgresql://postgres:5432/user_db \
  -e AUTH_USER_DB_USERNAME=voice \
  -e AUTH_USER_DB_PASSWORD=voice \
  -e SPRING_DATA_REDIS_HOST=redis \
  -e SPRING_DATA_REDIS_PORT=6379 \
  -e AUTH_JWT_PRIVATE_KEY_LOCATION=file:/run/jwt.pem \
  -v "$JWT_DOCKER_VOL:/run/jwt.pem:ro" \
  -p "${HTTP_PORT}:8080" \
  -p "${GRPC_PORT}:9090" \
  "$AUTH_IMAGE"

echo "Waiting for /health..."
ok=0
for _ in $(seq 1 180); do
  if curl -sf "http://127.0.0.1:${HTTP_PORT}/health" >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 1
done
if [[ "$ok" -ne 1 ]]; then
  echo "Auth /health did not become ready" >&2
  docker logs "$CONTAINER_NAME" >&2 || true
  exit 1
fi

curl -sf "http://127.0.0.1:${HTTP_PORT}/health" | tee /tmp/auth-health.json >/dev/null
if command -v jq >/dev/null 2>&1; then
  jq -e '.service == "auth" and .status == "ok"' /tmp/auth-health.json >/dev/null
else
  grep -q '"service"' /tmp/auth-health.json && grep -q '"status"' /tmp/auth-health.json \
    && grep -q '"ok"' /tmp/auth-health.json && grep -q '"auth"' /tmp/auth-health.json
fi
echo "Health OK"

curl -sf "http://127.0.0.1:${HTTP_PORT}/api/v1/auth/.well-known/jwks.json" | tee /tmp/auth-jwks.json >/dev/null
if command -v jq >/dev/null 2>&1; then
  jq -e '.keys != null and (.keys | length > 0)' /tmp/auth-jwks.json >/dev/null
else
  grep -q '"keys"' /tmp/auth-jwks.json
fi
echo "JWKS REST OK"

install_grpcurl

GRPC_OUT="$(mktemp)"
if ! grpcurl -import-path "$ROOT/protos" -proto voice/auth/v1/auth.proto -plaintext \
  -d '{}' "127.0.0.1:${GRPC_PORT}" voice.auth.v1.AuthService/GetJWKS >"$GRPC_OUT" 2>&1; then
  cat "$GRPC_OUT" >&2
  exit 1
fi
cat "$GRPC_OUT"
if ! grep -qE 'keysJson|keys_json' "$GRPC_OUT"; then
  echo "Unexpected gRPC response (missing keys JSON)" >&2
  exit 1
fi
echo "gRPC GetJWKS OK"
rm -f "$GRPC_OUT"

echo "Auth container smoke passed."
