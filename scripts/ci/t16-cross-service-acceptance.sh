#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" != "prepare" ]]; then
  echo "usage: $0 prepare" >&2
  exit 2
fi

t16_dir="$PWD/tmp/t16-cross-service"
tls_dir="$t16_dir/tls"
mkdir -p "$tls_dir" "$t16_dir/auth-principal"

openssl req -x509 -newkey rsa:2048 -nodes -days 2 \
  -keyout "$tls_dir/ca.key" -out "$tls_dir/ca.crt" \
  -subj "/CN=Voice T16 disposable service CA" \
  -addext "basicConstraints=critical,CA:TRUE" \
  -addext "keyUsage=critical,keyCertSign,cRLSign" >/dev/null 2>&1

make_leaf() {
  local name="$1" common_name="$2" sans="$3" extended_usage="$4"
  local key="$tls_dir/$name.key" csr="$tls_dir/$name.csr" ext="$tls_dir/$name.ext"
  openssl req -new -newkey rsa:2048 -nodes -keyout "$key" -out "$csr" \
    -subj "/CN=$common_name" >/dev/null 2>&1
  printf 'basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=%s\nsubjectAltName=%s\n' \
    "$extended_usage" "$sans" > "$ext"
  openssl x509 -req -in "$csr" -CA "$tls_dir/ca.crt" -CAkey "$tls_dir/ca.key" \
    -CAcreateserial -days 2 -extfile "$ext" -out "$tls_dir/$name.crt" >/dev/null 2>&1
  rm "$csr" "$ext"
}

make_leaf auth-server auth 'DNS:auth,DNS:localhost,IP:127.0.0.1' serverAuth
make_leaf gis-server gameintegration 'DNS:gameintegration,DNS:localhost,IP:127.0.0.1' serverAuth
make_leaf gameintegration-client gameintegration 'URI:spiffe://voice/service/gameintegration' clientAuth
make_leaf messaging-client messaging 'URI:spiffe://voice/service/messaging' clientAuth

# Tomcat runs with JSSE in the hosted Temurin image. Build its truststore from
# only this disposable fixture CA; caCertificateFile alone is OpenSSL-specific.
T16_AUTH_CLIENT_TRUSTSTORE_PASSWORD="$(openssl rand -hex 16)"
T16_GAME_INTEGRATION_DB_PASSWORD="$(openssl rand -hex 32)"
T16_AUTH_WORKLOAD_KEY_B64="$(openssl rand -base64 32)"
T16_MESSAGING_WORKLOAD_KEY_B64="$(openssl rand -base64 32)"
# Register generated environment secrets with Actions before exporting them.
printf '::add-mask::%s\n' \
  "$T16_AUTH_CLIENT_TRUSTSTORE_PASSWORD" \
  "$T16_GAME_INTEGRATION_DB_PASSWORD" \
  "$T16_AUTH_WORKLOAD_KEY_B64" \
  "$T16_MESSAGING_WORKLOAD_KEY_B64"
keytool -importcert -noprompt -storetype PKCS12 -alias t16-client-ca \
  -file "$tls_dir/ca.crt" -keystore "$tls_dir/auth-client-ca.p12" \
  -storepass "$T16_AUTH_CLIENT_TRUSTSTORE_PASSWORD" >/dev/null 2>&1

cp src/backend/auth/src/test/resources/jwt-test-private.pem "$t16_dir/auth-principal/current.pem"
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 \
  -out "$t16_dir/auth-principal/next.pem" >/dev/null 2>&1
# Synthetic Auth/GIS keys are mounted read-only into non-root service
# containers (both run as uid/gid 65532). Keep private files owner-only and
# readable by those containers; the Messaging client key stays runner-owned
# because the real Go gRPC test process uses it directly.
chmod 400 \
  "$tls_dir/auth-server.key" \
  "$tls_dir/gis-server.key" \
  "$tls_dir/gameintegration-client.key" \
  "$t16_dir/auth-principal"/*.pem
chmod 600 "$tls_dir/messaging-client.key"
chmod 440 "$tls_dir/auth-client-ca.p12"
rm "$tls_dir/ca.key" "$tls_dir/ca.srl"

cat > "$t16_dir/compose.env" <<EOF
T16_FIXTURE_DIR=$t16_dir
T16_AUTH_WORKLOAD_KEY_B64=$T16_AUTH_WORKLOAD_KEY_B64
T16_MESSAGING_WORKLOAD_KEY_B64=$T16_MESSAGING_WORKLOAD_KEY_B64
T16_AUTH_CLIENT_TRUSTSTORE_PASSWORD=$T16_AUTH_CLIENT_TRUSTSTORE_PASSWORD
GAME_INTEGRATION_DB_PASSWORD=$T16_GAME_INTEGRATION_DB_PASSWORD
EOF
T16_GAME_PUBLIC_JWK="$(python3 - "$t16_dir/auth-principal/current.pem" <<'PY'
import base64
import json
import sys
from cryptography.hazmat.primitives import serialization

with open(sys.argv[1], "rb") as source:
    key = serialization.load_pem_private_key(source.read(), password=None)
numbers = key.public_key().public_numbers()

def encode(value):
    size = (value.bit_length() + 7) // 8
    return base64.urlsafe_b64encode(value.to_bytes(size, "big")).decode().rstrip("=")

print(json.dumps({"kty": "RSA", "n": encode(numbers.n), "e": encode(numbers.e)}, separators=(",", ":")))
PY
)"
printf 'T16_GAME_PUBLIC_JWK=%s\n' "$T16_GAME_PUBLIC_JWK" >> "$t16_dir/compose.env"
chmod 600 "$t16_dir/compose.env"
if [[ -n "${GITHUB_ENV:-}" ]]; then
  sed '/^T16_FIXTURE_DIR=/d' "$t16_dir/compose.env" >> "$GITHUB_ENV"
  printf 'T16_FIXTURE_DIR=%s\n' "$t16_dir" >> "$GITHUB_ENV"
fi

# Complete ownership changes after deriving the public JWK from the private
# key so the runner process does not need to read it afterward.
sudo chown 65532:65532 \
  "$tls_dir/auth-server.key" \
  "$tls_dir/gis-server.key" \
  "$tls_dir/gameintegration-client.key" \
  "$tls_dir/auth-client-ca.p12" \
  "$t16_dir/auth-principal"/*.pem
