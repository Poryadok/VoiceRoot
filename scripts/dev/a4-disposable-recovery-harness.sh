#!/usr/bin/env bash
# Produces non-secret evidence for an A4 recovery drill. It never connects to a database.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: a4-disposable-recovery-harness.sh --dry-run --source <postgres-url> \
  --target <postgres-url> --artifact-dir <path> --config-validation <path> \
  --admission <path> --observability <path> --durable-store-manifest <path>

The target must be an isolated PostgreSQL authority. This command only records
the pre-drill evidence bundle; the separately approved operator drill performs
backup/restore against the supplied isolated target.
EOF
}

die() { printf 'ERROR: %s\n' "$*" >&2; exit 2; }

mode=''
source_url=''
target_url=''
artifact_dir=''
config_validation=''
admission=''
observability=''
durable_store_manifest=''

while (($#)); do
  case "$1" in
    --dry-run) mode='dry-run'; shift ;;
    --source) source_url="${2:-}"; shift 2 ;;
    --target) target_url="${2:-}"; shift 2 ;;
    --artifact-dir) artifact_dir="${2:-}"; shift 2 ;;
    --config-validation) config_validation="${2:-}"; shift 2 ;;
    --admission) admission="${2:-}"; shift 2 ;;
    --observability) observability="${2:-}"; shift 2 ;;
    --durable-store-manifest) durable_store_manifest="${2:-}"; shift 2 ;;
    --help|-h) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

[[ "${mode}" == 'dry-run' ]] || die 'only --dry-run is supported; this harness never executes backup or restore'
[[ "${source_url}" == postgres://* || "${source_url}" == postgresql://* ]] || die 'source must be a PostgreSQL URL'
[[ "${target_url}" == postgres://* || "${target_url}" == postgresql://* ]] || die 'target must be a PostgreSQL URL'
[[ -n "${artifact_dir}" ]] || die '--artifact-dir is required'

postgres_authority() {
  local without_scheme="${1#*://}"
  local host_path="${without_scheme##*@}"
  printf '%s\n' "${host_path%%/*}"
}

redacted_endpoint() {
  local without_scheme="${1#*://}"
  printf '%s\n' "${without_scheme##*@}"
}

source_authority="$(postgres_authority "${source_url}")"
target_authority="$(postgres_authority "${target_url}")"
[[ -n "${source_authority}" && -n "${target_authority}" ]] || die 'source and target must name PostgreSQL authorities'
[[ "${source_authority}" != "${target_authority}" ]] || die 'target authority must differ from source authority'

for evidence in "${config_validation}" "${admission}" "${observability}" "${durable_store_manifest}"; do
  [[ -s "${evidence}" ]] || die "required evidence is missing or empty: ${evidence}"
done

mkdir -p "${artifact_dir}"
copy_evidence() {
  local source="$1"
  local destination="${artifact_dir}/$(basename "${source}")"
  cp "${source}" "${destination}"
  printf '%s' "$(basename "${source}")"
}

config_name="$(copy_evidence "${config_validation}")"
admission_name="$(copy_evidence "${admission}")"
observability_name="$(copy_evidence "${observability}")"
durable_name="$(copy_evidence "${durable_store_manifest}")"
sha256() { sha256sum "$1" | awk '{print $1}'; }

cat >"${artifact_dir}/recovery-evidence.json" <<EOF
{
  "mode": "${mode}",
  "source": "$(redacted_endpoint "${source_url}")",
  "target": "$(redacted_endpoint "${target_url}")",
  "restore_boundary": "target authority differs from source authority; no source write is permitted",
  "evidence": [
    {"name": "${config_name}", "sha256": "$(sha256 "${artifact_dir}/${config_name}")"},
    {"name": "${admission_name}", "sha256": "$(sha256 "${artifact_dir}/${admission_name}")"},
    {"name": "${observability_name}", "sha256": "$(sha256 "${artifact_dir}/${observability_name}")"},
    {"name": "${durable_name}", "sha256": "$(sha256 "${artifact_dir}/${durable_name}")"}
  ]
}
EOF

printf 'A4 recovery evidence bundle: %s\n' "${artifact_dir}/recovery-evidence.json"
