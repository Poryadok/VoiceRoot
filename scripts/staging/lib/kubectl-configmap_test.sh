#!/usr/bin/env bash
# Regression tests for Kubernetes ConfigMap generation across kubectl dry-run APIs.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
HELPER="${ROOT}/scripts/staging/lib/kubectl-configmap.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

write_kubectl_mock() {
  local mode="$1"
  cat >"${TMP}/kubectl" <<EOF
#!/usr/bin/env bash
set -euo pipefail
printf '%s\\n' "\$*" >>"${TMP}/calls"
if [ "\$1" = create ] && [ "\$2" = configmap ] && [ "\$3" = --help ]; then
  if [ "${mode}" != legacy ]; then
    echo '  --dry-run=client: generate a manifest locally'
  else
    echo '  --dry-run: if true, only print the object'
  fi
  exit 0
fi
if [ "\$1" = create ] && [ "\$2" = configmap ]; then
  if [ "${mode}" = empty ]; then
    exit 0
  fi
  printf '%s\\n' 'apiVersion: v1' 'kind: ConfigMap' 'metadata:' '  name: test-config' >"${TMP}/generated.yaml"
  cat "${TMP}/generated.yaml"
  exit 0
fi
if [ "\$1" = apply ] && [ "\$2" = -f ] && [ -f "\$3" ]; then
  cat "\$3" >"${TMP}/applied.yaml"
  exit 0
fi
fail "unexpected kubectl invocation: \$*"
EOF
  chmod +x "${TMP}/kubectl"
}

run_case() {
  local mode="$1"
  : >"${TMP}/calls"
  rm -f "${TMP}/generated.yaml" "${TMP}/applied.yaml"
  write_kubectl_mock "${mode}"
  PATH="${TMP}:$PATH" bash -c '
    source "$1"
    kubectl_apply_configmap test-config test-ns --from-file=payload=fixture.sql
  ' _ "${HELPER}"
  cmp "${TMP}/generated.yaml" "${TMP}/applied.yaml" >/dev/null \
    || fail "${mode}: generated ConfigMap was not applied"
}

run_case legacy
grep -Fxq 'create configmap test-config --namespace=test-ns --from-file=payload=fixture.sql --dry-run -o yaml' "${TMP}/calls" \
  || fail 'legacy kubectl must use boolean --dry-run'
if grep -Fq -- '--dry-run=client' "${TMP}/calls"; then
  fail 'legacy kubectl must not receive --dry-run=client'
fi

run_case modern
grep -Fxq 'create configmap test-config --namespace=test-ns --from-file=payload=fixture.sql --dry-run=client -o yaml' "${TMP}/calls" \
  || fail 'modern kubectl must use --dry-run=client'
if grep -Fxq 'create configmap test-config --namespace=test-ns --from-file=payload=fixture.sql --dry-run -o yaml' "${TMP}/calls"; then
  fail 'modern kubectl must not fall back to boolean --dry-run'
fi

: >"${TMP}/calls"
write_kubectl_mock empty
if PATH="${TMP}:$PATH" bash -c '
  source "$1"
  kubectl_apply_configmap test-config test-ns --from-file=payload=fixture.sql
' _ "${HELPER}"; then
  fail 'empty generated manifests must fail before kubectl apply'
fi
if grep -Fq 'apply -f' "${TMP}/calls"; then
  fail 'empty generated manifests must never reach kubectl apply'
fi

echo 'kubectl ConfigMap compatibility tests passed.'
