#!/bin/sh
# Local/CI only. Separate volumes keep Social signing keys out of consumers.
set -eu
umask 077
signing_directories="/signing /file-signing /search-signing /space-signing /story-signing /messaging-signing"
services="social user space file search story messaging matchmaking"
if [ -f /ca/ready ]; then
  test -s /ca/ca.crt || { echo "Incomplete principal CA volume" >&2; exit 1; }
  for directory in $signing_directories; do
    for key in current next; do
      test -s "$directory/$key.pem" || { echo "Incomplete principal signer: $directory/$key.pem" >&2; exit 1; }
    done
  done
  for service in $services; do
    for extension in crt key; do
      test -s "/$service/tls.$extension" || { echo "Incomplete principal TLS identity: $service" >&2; exit 1; }
    done
  done
  exit 0
fi
# Never overwrite a partial bootstrap: stop the isolated Compose project and
# recreate only its principal volumes after inspecting the failed initializer.
for directory in /ca $signing_directories $services; do
  case "$directory" in /*) ;; *) directory="/$directory" ;; esac
  test -z "$(ls -A "$directory")" || { echo "Partial principal bootstrap in $directory" >&2; exit 1; }
done
for directory in $signing_directories; do
  for key in current next; do
    openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$directory/$key.pem" 2>/dev/null
  done
done
openssl req -x509 -newkey rsa:2048 -nodes -keyout /tmp/ca.key -out /ca/ca.crt -days 30 -subj /CN=Voice-Compose-Principal-CA -addext basicConstraints=critical,CA:TRUE -addext keyUsage=critical,keyCertSign,cRLSign 2>/dev/null
for service in $services; do
  openssl req -new -newkey rsa:2048 -nodes -keyout "/$service/tls.key" -out "/tmp/$service.csr" -subj "/CN=$service" 2>/dev/null
  printf 'basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nsubjectAltName=DNS:%s\nextendedKeyUsage=serverAuth\n' "$service" > "/tmp/$service.ext"
  openssl x509 -req -in "/tmp/$service.csr" -CA /ca/ca.crt -CAkey /tmp/ca.key -set_serial "0x$(openssl rand -hex 16)" -days 30 -extfile "/tmp/$service.ext" -out "/$service/tls.crt" 2>/dev/null
done
# Service images run as UID 65532; private files stay 0600 and service-scoped.
# No CA private key is stored in a mounted volume.
rm -f /tmp/ca.key
for directory in $signing_directories $services; do
  case "$directory" in /*) ;; *) directory="/$directory" ;; esac
  chown -R 65532:65532 "$directory"
  chmod 700 "$directory"
done
chmod 644 /ca/ca.crt
touch /ca/ready
