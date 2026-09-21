#!/bin/sh
# Local/CI only. Separate volumes keep Social signing keys out of consumers.
set -eu
umask 077
if [ -f /ca/ready ]; then
  for file in /ca/ca.crt /signing/current.pem /signing/next.pem /file-signing/current.pem /file-signing/next.pem /search-signing/current.pem /search-signing/next.pem /social/tls.crt /social/tls.key /user/tls.crt /user/tls.key /space/tls.crt /space/tls.key /file/tls.crt /file/tls.key /search/tls.crt /search/tls.key; do
    test -s "$file" || { echo "Incomplete principal volume: $file" >&2; exit 1; }
  done
  exit 0
fi
# Never overwrite a partial bootstrap: stop the isolated Compose project and
# recreate only its principal volumes after inspecting the failed initializer.
for directory in /ca /signing /file-signing /search-signing /social /user /space /file /search; do
  test -z "$(ls -A "$directory")" || { echo "Partial principal bootstrap in $directory" >&2; exit 1; }
done
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out /signing/current.pem 2>/dev/null
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out /signing/next.pem 2>/dev/null
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out /file-signing/current.pem 2>/dev/null
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out /file-signing/next.pem 2>/dev/null
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out /search-signing/current.pem 2>/dev/null
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out /search-signing/next.pem 2>/dev/null
openssl req -x509 -newkey rsa:2048 -nodes -keyout /tmp/ca.key -out /ca/ca.crt -days 30 -subj /CN=Voice-Compose-Principal-CA -addext basicConstraints=critical,CA:TRUE -addext keyUsage=critical,keyCertSign,cRLSign 2>/dev/null
for service in social user space file search; do
  openssl req -new -newkey rsa:2048 -nodes -keyout "/$service/tls.key" -out "/tmp/$service.csr" -subj "/CN=$service" 2>/dev/null
  printf 'basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nsubjectAltName=DNS:%s\nextendedKeyUsage=serverAuth\n' "$service" > "/tmp/$service.ext"
  openssl x509 -req -in "/tmp/$service.csr" -CA /ca/ca.crt -CAkey /tmp/ca.key -set_serial "0x$(openssl rand -hex 16)" -days 30 -extfile "/tmp/$service.ext" -out "/$service/tls.crt" 2>/dev/null
done
# Service images run as UID 65532; private files stay 0600 and service-scoped.
# No CA private key is stored in a mounted volume.
rm -f /tmp/ca.key
chown -R 65532:65532 /signing /file-signing /search-signing /social /user /space /file /search
chmod 700 /signing /file-signing /search-signing /social /user /space /file /search
chmod 644 /ca/ca.crt
touch /ca/ready
