#!/usr/bin/env bash
# Secret manifests must be private and removed on every exit path.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT
mkdir -p "${TMP}/bin" "${TMP}/manifests"
export TMPDIR="${TMP}/manifests"
cat >"${TMP}/bin/kubectl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
if [ "$1" = create ]; then
  if [ "$4" = --help ]; then echo '--dry-run=client'; exit 0; fi
  case "$SCENARIO" in
    failed) echo partial-manifest; exit 21 ;;
    empty) exit 0 ;;
  esac
  echo 'kind: Secret'
  exit 0
fi
if [ "$1" = apply ]; then
  [ -s "$3" ] || exit 41
  # POSIX permission bits are meaningful on CI Linux, not Windows ACL mounts.
  case "$(uname -s)" in
    MINGW*|MSYS*) ;;
    *) [ "$(stat -c '%a' "$3")" = 600 ] || exit 42 ;;
  esac
  [ "$SCENARIO" != apply-failure ] || exit 37
  exit 0
fi
exit 43
MOCK
chmod +x "${TMP}/bin/kubectl"
for scenario in success failed empty apply-failure; do
  status=0
  PATH="${TMP}/bin:$PATH" SCENARIO="$scenario" bash -c '
    source "$1"
    kubectl_apply_secret fixture test-ns --from-literal=value=fixture
  ' _ "${ROOT}/scripts/staging/lib/kubectl-secret.sh" || status=$?
  case "$scenario" in
    success) [ "$status" -eq 0 ] ;;
    apply-failure) [ "$status" -eq 37 ] ;;
    *) [ "$status" -ne 0 ] ;;
  esac
  [ -z "$(find "${TMPDIR}" -type f -print -quit)" ] || {
    echo "FAIL: $scenario left a Secret manifest" >&2
    exit 1
  }
done
echo 'Secret manifest privacy and cleanup tests passed.'
