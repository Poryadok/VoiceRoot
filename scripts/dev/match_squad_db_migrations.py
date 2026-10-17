#!/usr/bin/env python3
"""Source-bound, target-only DEV MatchSquad database migration runner.

This deliberately does not replace the broad local bootstrap migration path.
It exposes a narrow DEV operation for the five reviewed MatchSquad UP files.
"""

from __future__ import annotations

import argparse
import copy
import hashlib
import ipaddress
import json
import re
import subprocess
import sys
import time
from pathlib import Path

import migrations
from gis_dev_first_install import Blocked as KubeBlocked, KubeError, Kubectl


NAMESPACE = "voice-dev"
NAMESPACE_UID = "15950fc8-0392-4053-9156-e95e78b596f9"
SERVICES = ("matchmaking", "chat", "voice")
DATABASES = {"matchmaking": "matchmaking_db", "chat": "chat_db", "voice": "voice_db"}
BASELINES = {"matchmaking": 15, "chat": 22, "voice": 10}
TARGETS = {"matchmaking": 18, "chat": 23, "voice": 11}
MIGRATION_FILES = {
    "matchmaking": (
        "000016_match_deadline_squad_recovery.up.sql",
        "000017_match_squad_lifecycle.up.sql",
        "000018_match_squad_durable_recovery.up.sql",
    ),
    "chat": ("000023_match_squad_operations.up.sql",),
    "voice": ("000011_match_squad_operations.up.sql",),
}
MIGRATION_HASHES = {
    "000016_match_deadline_squad_recovery.up.sql": "1ecf553bfd4f93289b18b99215118120cfcac8e6b913d9a29fe7a4e58fbb1e69",
    "000017_match_squad_lifecycle.up.sql": "ff3b8b5bbe215e35bcf4de7ad7af21b0d0049b41661a7ae4281eefc088a857af",
    "000018_match_squad_durable_recovery.up.sql": "fc9234b277dd03f795a3b21a630802a5a25292c5df98a1b79c294f382bc59852",
    "000023_match_squad_operations.up.sql": "c3190cb9cbfe08d7bda4f2cdcf661c3e1555a7f7436cfa6f0f196f8db20b7dd2",
    "000011_match_squad_operations.up.sql": "6fa0574e17c4f38922b2f517eddc34d3a791716b2ebfa55243ec179fa9345301",
}
SOURCE_ANNOTATION = "voice.io/match-squad-migration-source"
NAMESPACE_UID_ANNOTATION = "voice.io/match-squad-namespace-uid"
INPUT_ANNOTATION = "voice.io/match-squad-migration-input"
DATABASE_ANNOTATION = "voice.io/match-squad-database"
MIGRATE_IMAGE = migrations.GAMEINTEGRATION_IMAGES["migrate"]
POSTGRES_IMAGE = migrations.GAMEINTEGRATION_IMAGES["postgres"]


class Blocked(RuntimeError):
    """The request is outside the fixed, guarded DEV migration contract."""


def _blocked(reason: str):
    raise Blocked(reason)


def plan_services():
    return SERVICES


def validate_source(actual: str, expected: str, *, clean: bool):
    if not re.fullmatch(r"[0-9a-f]{40}", actual or "") or actual != expected:
        _blocked("source_identity_mismatch")
    if not clean:
        _blocked("source_tree_dirty")
    return True


def validate_namespace(namespace):
    metadata = namespace.get("metadata", {}) if isinstance(namespace, dict) else {}
    if metadata.get("name") != NAMESPACE or metadata.get("uid") != NAMESPACE_UID:
        _blocked("namespace_identity_mismatch")
    return True


def database_for_service(service):
    try:
        return DATABASES[service]
    except (KeyError, TypeError):
        _blocked("database_out_of_scope")


def _migration_dir(root: Path, service: str) -> Path:
    db_dir = {"matchmaking": "matchmaking_db", "chat": "chat_db", "voice": "voice_db"}[service]
    return root / "src/backend/migrations" / db_dir


def _digest_data(data):
    packed = json.dumps(data, sort_keys=True, separators=(",", ":")).encode()
    return hashlib.sha256(packed).hexdigest()


def _validate_image_pins():
    for image in (MIGRATE_IMAGE, POSTGRES_IMAGE):
        if not isinstance(image, str) or not re.fullmatch(r"[^\s@]+(?:/[^\s@]+)*@sha256:[0-9a-f]{64}", image):
            _blocked("migration_image_digest_pin_invalid")


def build_plan(root, source_sha):
    _validate_image_pins()
    root = Path(root).resolve()
    if not re.fullmatch(r"[0-9a-f]{40}", source_sha or ""):
        _blocked("source_identity_invalid")
    plan = []
    for service in SERVICES:
        data = {}
        for name in MIGRATION_FILES[service]:
            path = _migration_dir(root, service) / name
            try:
                content = path.read_text(encoding="utf-8")
            except OSError:
                _blocked("migration_input_missing")
            if hashlib.sha256(content.encode()).hexdigest() != MIGRATION_HASHES[name]:
                _blocked("migration_input_changed")
            data[name] = content
        input_hash = _digest_data(data)
        suffix = source_sha[:10] + "-" + input_hash[:8]
        job_name = "voice-msq-migrate-" + service + "-" + suffix
        cm_name = job_name + "-sql"
        plan.append({
            "service": service,
            "database": database_for_service(service),
            "baseline_version": BASELINES[service],
            "expected_version": TARGETS[service],
            "new_up_migrations": list(data),
            "migration_data": data,
            "input_hash": input_hash,
            "source_sha": source_sha,
            "job_name": job_name,
            "configmap_name": cm_name,
        })
    return plan


def parse_ledger(service, output):
    database = database_for_service(service)
    lines = [line.strip() for line in output.splitlines() if line.strip()]
    if len(lines) != 1 or not re.fullmatch(r"[a-z0-9_]+\|[0-9]+\|[ft]", lines[0]):
        _blocked("migration_ledger_invalid")
    actual_database, version, dirty = lines[0].split("|")
    if actual_database != database:
        _blocked("migration_database_identity_mismatch")
    return int(version), dirty == "t"


def validate_ledger(item, output, completed=False):
    version, dirty = parse_ledger(item["service"], output)
    if dirty:
        _blocked("migration_ledger_dirty")
    expected = item["expected_version"] if completed else item["baseline_version"]
    if version != expected:
        _blocked("migration_ledger_version_mismatch")
    return version


def validate_bound_object(obj, item, kind):
    metadata = obj.get("metadata", {}) if isinstance(obj, dict) else {}
    annotations = metadata.get("annotations", {})
    expected_name = item["configmap_name"] if kind == "configmap" else item["job_name"]
    if (metadata.get("name") != expected_name or metadata.get("namespace") != NAMESPACE
            or annotations.get(SOURCE_ANNOTATION) != item["source_sha"]
            or annotations.get(NAMESPACE_UID_ANNOTATION) != NAMESPACE_UID
            or annotations.get(INPUT_ANNOTATION) != item["input_hash"]
            or annotations.get(DATABASE_ANNOTATION) != item["database"]):
        _blocked("existing_object_not_source_bound")
    if kind == "configmap" and (obj.get("data") != item["migration_data"]
            or obj.get("immutable") is not True):
        _blocked("existing_configmap_content_mismatch")
    if kind == "job":
        postgres_ip = item.get("postgres_ip")
        wanted = job_manifest(item, postgres_ip)["spec"]
        actual = copy.deepcopy(obj.get("spec", {}))
        selector = actual.pop("selector", None)
        uid = metadata.get("uid")
        if not _valid_job_selector(selector, item, uid):
            _blocked("existing_job_selector_mismatch")
        if not _matches_kubernetes_defaults(actual, wanted):
            _blocked("existing_job_spec_mismatch")
    if kind not in {"configmap", "job"}:
        _blocked("existing_object_kind_invalid")
    if not isinstance(metadata.get("uid"), str) or not metadata["uid"]:
        _blocked("existing_object_uid_missing")
    return True


_KUBE_DEFAULTS = {
    ("parallelism",): 1,
    ("completions",): 1,
    ("completionMode",): "NonIndexed",
    ("suspend",): False,
    ("manualSelector",): False,
    ("podReplacementPolicy",): "TerminatingOrFailed",
    ("template", "spec", "dnsPolicy"): "ClusterFirst",
    ("template", "spec", "schedulerName"): "default-scheduler",
    ("template", "spec", "terminationGracePeriodSeconds"): 30,
    ("template", "spec", "serviceAccountName"): "default",
    ("template", "spec", "enableServiceLinks"): True,
    ("template", "spec", "preemptionPolicy"): "PreemptLowerPriority",
    ("template", "spec", "containers", "*", "imagePullPolicy"): "IfNotPresent",
    ("template", "spec", "initContainers", "*", "imagePullPolicy"): "IfNotPresent",
    ("template", "spec", "containers", "*", "terminationMessagePath"): "/dev/termination-log",
    ("template", "spec", "initContainers", "*", "terminationMessagePath"): "/dev/termination-log",
    ("template", "spec", "containers", "*", "terminationMessagePolicy"): "File",
    ("template", "spec", "initContainers", "*", "terminationMessagePolicy"): "File",
    ("template", "spec", "containers", "*", "stdin"): False,
    ("template", "spec", "initContainers", "*", "stdin"): False,
    ("template", "spec", "containers", "*", "stdinOnce"): False,
    ("template", "spec", "initContainers", "*", "stdinOnce"): False,
    ("template", "spec", "containers", "*", "tty"): False,
    ("template", "spec", "initContainers", "*", "tty"): False,
    ("template", "spec", "volumes", "*", "configMap", "defaultMode"): 420,
    ("template", "spec", "volumes", "*", "configMap", "optional"): False,
}


def _valid_job_selector(selector, item, uid):
    if not isinstance(uid, str) or not uid or not isinstance(selector, dict):
        return False
    if set(selector) != {"matchLabels"} or not isinstance(selector["matchLabels"], dict):
        return False
    labels = selector["matchLabels"]
    return labels in (
        {"batch.kubernetes.io/controller-uid": uid},
        {"controller-uid": uid},
    )


def _matches_kubernetes_defaults(actual, expected, path=()):
    """Require exact authored fields/lists; tolerate only known API defaults."""
    if isinstance(expected, dict):
        if not isinstance(actual, dict):
            return False
        for key, value in expected.items():
            if key not in actual or not _matches_kubernetes_defaults(actual[key], value, path + (key,)):
                return False
        for key, value in actual.items():
            if key not in expected:
                if _KUBE_DEFAULTS.get(path + (key,), object()) != value:
                    return False
        return True
    if isinstance(expected, list):
        return isinstance(actual, list) and len(actual) == len(expected) and all(
            _matches_kubernetes_defaults(actual_item, expected_item, path + ("*",))
            for actual_item, expected_item in zip(actual, expected)
        )
    return actual == expected


def validate_terminal_job(job, item):
    validate_bound_object(job, item, "job")
    status = job.get("status", {})
    if status.get("active") or status.get("failed") or not status.get("succeeded"):
        _blocked("job_not_successfully_terminal")
    return True


def validate_create_readback(created, readback):
    before = created.get("metadata", {})
    after = readback.get("metadata", {})
    if not before.get("uid") or before.get("uid") != after.get("uid"):
        _blocked("create_readback_uid_changed")
    if not before.get("resourceVersion") or not after.get("resourceVersion"):
        _blocked("create_readback_resource_version_missing")
    return True


def _bound_metadata(name, item):
    return {"name": name, "namespace": NAMESPACE, "annotations": {
        SOURCE_ANNOTATION: item["source_sha"],
        NAMESPACE_UID_ANNOTATION: NAMESPACE_UID,
        INPUT_ANNOTATION: item["input_hash"],
        DATABASE_ANNOTATION: item["database"],
    }}


def configmap_manifest(item):
    return {"apiVersion": "v1", "kind": "ConfigMap",
            "metadata": _bound_metadata(item["configmap_name"], item),
            "immutable": True,
            "data": item["migration_data"]}


def job_manifest(item, postgres_ip):
    try:
        postgres_ip = str(ipaddress.ip_address(postgres_ip))
    except (TypeError, ValueError):
        _blocked("postgres_pod_ip_invalid")
    resources = {"requests": {"cpu": "20m", "memory": "64Mi"},
                 "limits": {"cpu": "500m", "memory": "512Mi"}}
    name = item["job_name"]
    db_url_key = item["service"].upper() + "_DATABASE_URL"
    database_url_file = "/run/migration/validated-database-url"
    target_check = """case \"$DATABASE_URL\" in postgres://voice:*) ;; *) echo migration_target_url_invalid >&2; exit 1 ;; esac
authority_path=${DATABASE_URL#postgres://}
userinfo=${authority_path%%@*}
target=${authority_path#*@}
test \"$target\" != \"$authority_path\" || { echo migration_target_url_invalid >&2; exit 1; }
case \"$userinfo\" in voice:*) password=${userinfo#voice:}; test -n \"$password\" || { echo migration_target_url_invalid >&2; exit 1; } ;; *) echo migration_target_url_invalid >&2; exit 1 ;; esac
case \"$target\" in \"voice-postgres:5432/${EXPECTED_DATABASE}?sslmode=disable\") ;; *) echo migration_target_url_invalid >&2; exit 1 ;; esac
sealed_url=\"postgres://${userinfo}@${EXPECTED_POSTGRES_IP}:5432/${EXPECTED_DATABASE}?sslmode=disable\"
unset PGHOST PGHOSTADDR PGPORT PGUSER PGDATABASE PGSERVICE PGSERVICEFILE PGPASSWORD
if observed=$(psql -X -qAt -v ON_ERROR_STOP=1 -d \"$sealed_url\" -c \"SELECT current_database() || '|' || current_user || '|' || host(inet_server_addr())\" 2>/dev/null); then :; else echo migration_target_probe_failed >&2; exit 1; fi
test \"$observed\" = \"$EXPECTED_DATABASE|voice|$EXPECTED_POSTGRES_IP\" || { echo migration_target_mismatch >&2; exit 1; }
umask 077
test ! -e \"/run/migration/validated-database-url\" || { echo migration_target_already_sealed >&2; exit 1; }
(set -C; printf '%s' \"$sealed_url\" > \"/run/migration/validated-database-url\") || { echo migration_target_seal_failed >&2; exit 1; }
chmod 600 \"/run/migration/validated-database-url\" || { echo migration_target_seal_failed >&2; exit 1; }
unset DATABASE_URL sealed_url userinfo authority_path target password observed"""
    migration_command = f"""database_url=$(cat {database_url_file}) || {{ echo migration_source_unavailable >&2; exit 1; }}
umask 077
if migrate -path=/migrations -database \"$database_url\" up >/run/migration/migrate.stdout 2>/run/migration/migrate.stderr; then exit 0; fi
echo migration_failed >&2
exit 1"""
    return {"apiVersion": "batch/v1", "kind": "Job",
            "metadata": _bound_metadata(name, item),
            "spec": {"backoffLimit": 0, "activeDeadlineSeconds": 600,
                "template": {"metadata": {"labels": {"app": name}}, "spec": {
                    "restartPolicy": "Never", "automountServiceAccountToken": False,
                    "initContainers": [{"name": "wait-postgres", "image": POSTGRES_IMAGE,
                        "command": ["/bin/sh", "-ec", "until pg_isready -h \"$EXPECTED_POSTGRES_IP\" -p 5432 -U voice -d \"$EXPECTED_DATABASE\" >/dev/null 2>&1; do sleep 2; done"],
                        "env": [{"name": "EXPECTED_DATABASE", "value": item["database"]},
                            {"name": "EXPECTED_POSTGRES_IP", "value": postgres_ip}],
                        "resources": resources}, {"name": "verify-database-target", "image": POSTGRES_IMAGE,
                        "command": ["/bin/sh", "-ec", target_check],
                        "env": [{"name": "DATABASE_URL", "valueFrom": {"secretKeyRef": {"name": "voice-app-secrets", "key": db_url_key}}},
                            {"name": "EXPECTED_DATABASE", "value": item["database"]},
                            {"name": "EXPECTED_POSTGRES_IP", "value": postgres_ip}],
                        "volumeMounts": [{"name": "migration-runtime", "mountPath": "/run/migration"}],
                        "resources": resources}],
                    "containers": [{"name": "migrate", "image": MIGRATE_IMAGE,
                        "command": ["/bin/sh", "-ec", migration_command],
                        "volumeMounts": [{"name": "migrations", "mountPath": "/migrations", "readOnly": True},
                            {"name": "migration-runtime", "mountPath": "/run/migration"}],
                        "resources": resources}],
                    "volumes": [{"name": "migrations", "configMap": {"name": item["configmap_name"]}},
                        {"name": "migration-runtime", "emptyDir": {"medium": "Memory", "sizeLimit": "1Mi"}}]}}}}


def _read_source_identity(root):
    try:
        sha = subprocess.run(["git", "rev-parse", "HEAD"], cwd=root, check=True,
            capture_output=True, text=True, timeout=10).stdout.strip()
        dirty = bool(subprocess.run(["git", "status", "--porcelain", "--untracked-files=all"],
            cwd=root, check=True, capture_output=True, text=True, timeout=10).stdout.strip())
    except Exception:
        _blocked("source_git_identity_unavailable")
    return sha, dirty


def _read_ledger(kube, item, postgres_pod):
    sql = ("SELECT current_database() || '|' || version::text || '|' || "
           "CASE WHEN dirty THEN 't' ELSE 'f' END "
           "FROM schema_migrations;")
    shell = "psql -X -qAt -v ON_ERROR_STOP=1 -U \"$POSTGRES_USER\" -d " + item["database"] + " -c '" + sql + "'"
    result = kube.run(["exec", postgres_pod, "-c", "postgres", "--", "/bin/sh", "-ec", shell], timeout=30)
    return parse_ledger(item["service"], result)


def _metadata_identity(obj, expected_name):
    metadata = obj.get("metadata", {}) if isinstance(obj, dict) else {}
    if (not isinstance(metadata, dict) or metadata.get("name") != expected_name
            or metadata.get("namespace") != NAMESPACE
            or not isinstance(metadata.get("uid"), str) or not metadata["uid"]
            or not isinstance(metadata.get("resourceVersion"), str) or not metadata["resourceVersion"]):
        _blocked("postgres_resource_identity_invalid")
    return {"uid": metadata["uid"], "resourceVersion": metadata["resourceVersion"]}


def _postgres_target_identity(kube):
    service = kube.get("service", "voice-postgres")
    service_identity = _metadata_identity(service, "voice-postgres")
    service_spec = service.get("spec", {})
    if not isinstance(service_spec, dict):
        _blocked("postgres_service_port_invalid")
    ports = service_spec.get("ports", [])
    if (not isinstance(service_spec, dict) or not isinstance(ports, list)
            or any(not isinstance(row, dict) for row in ports) or not any(
            row.get("port") == 5432 and row.get("protocol", "TCP") == "TCP"
            and row.get("targetPort", 5432) in (5432, "postgres") for row in ports)):
        _blocked("postgres_service_port_invalid")
    selector = service_spec.get("selector", {})
    if not isinstance(selector, dict) or not selector:
        _blocked("postgres_service_selector_mismatch")

    statefulset = kube.get("statefulset", "voice-postgres")
    statefulset_identity = _metadata_identity(statefulset, "voice-postgres")
    sts_meta = statefulset.get("metadata", {})
    sts_spec = statefulset.get("spec", {})
    sts_status = statefulset.get("status", {})
    if not all(isinstance(row, dict) for row in (sts_meta, sts_spec, sts_status)):
        _blocked("postgres_statefulset_not_currently_ready")
    generation = sts_meta.get("generation")
    if (type(generation) is not int or sts_status.get("observedGeneration") != generation
            or type(sts_spec.get("replicas")) is not int or sts_spec.get("replicas") != 1
            or type(sts_status.get("readyReplicas")) is not int or sts_status.get("readyReplicas") != 1):
        _blocked("postgres_statefulset_not_currently_ready")
    template = sts_spec.get("template", {})
    template_spec = template.get("spec", {}) if isinstance(template, dict) else None
    containers = template_spec.get("containers", []) if isinstance(template_spec, dict) else None
    if not isinstance(containers, list) or any(not isinstance(row, dict) for row in containers):
        _blocked("postgres_statefulset_image_invalid")
    db_containers = [row for row in containers if row.get("name") == "postgres"]
    if len(db_containers) != 1 or db_containers[0].get("image") != POSTGRES_IMAGE:
        _blocked("postgres_statefulset_image_invalid")

    response = kube.run(["get", "endpointslices", "-l", "kubernetes.io/service-name=voice-postgres", "-o", "json"])
    if not isinstance(response, dict) or not isinstance(response.get("items"), list):
        _blocked("postgres_ready_endpoint_missing")
    endpoints = []
    for row in response.get("items", []):
        if not isinstance(row, dict) or not isinstance(row.get("endpoints", []), list):
            _blocked("postgres_ready_endpoint_missing")
        slice_ports = row.get("ports", [])
        if (not isinstance(slice_ports, list) or len(slice_ports) != 1
                or not isinstance(slice_ports[0], dict)
                or slice_ports[0].get("port") != 5432
                or slice_ports[0].get("protocol", "TCP") != "TCP"
                or slice_ports[0].get("name") not in (None, "")):
            _blocked("postgres_ready_endpoint_port_invalid")
        for endpoint in row.get("endpoints", []):
            if not isinstance(endpoint, dict):
                _blocked("postgres_ready_endpoint_missing")
            target = endpoint.get("targetRef", {})
            conditions = endpoint.get("conditions", {})
            if not isinstance(conditions, dict) or not isinstance(target, dict):
                _blocked("postgres_ready_endpoint_missing")
            if conditions.get("ready") is True and target.get("kind") == "Pod":
                endpoints.append((target, endpoint.get("addresses")))
    if len(endpoints) != 1:
        _blocked("postgres_ready_endpoint_missing")
    target, endpoint_addresses = endpoints[0]
    pod_name = target.get("name")
    pod_uid = target.get("uid")
    if (pod_name != "voice-postgres-0" or not isinstance(pod_uid, str)
            or not re.fullmatch(r"[a-z0-9-]{1,63}", pod_name)):
        _blocked("postgres_ready_endpoint_invalid")
    pod = kube.get("pod", pod_name)
    pod_identity = _metadata_identity(pod, pod_name)
    if pod_identity["uid"] != pod_uid:
        _blocked("postgres_endpoint_pod_uid_mismatch")
    pod_meta = pod.get("metadata", {})
    if not isinstance(pod_meta, dict):
        _blocked("postgres_pod_owner_mismatch")
    owner_refs = pod_meta.get("ownerReferences", [])
    if not isinstance(owner_refs, list) or any(not isinstance(row, dict) for row in owner_refs):
        _blocked("postgres_pod_owner_mismatch")
    owner = next((row for row in owner_refs
        if row.get("kind") == "StatefulSet" and row.get("name") == "voice-postgres"), None)
    if not owner or owner.get("uid") != statefulset_identity["uid"]:
        _blocked("postgres_pod_owner_mismatch")
    labels = pod_meta.get("labels", {})
    if not isinstance(labels, dict) or any(labels.get(key) != value for key, value in selector.items()):
        _blocked("postgres_service_selector_mismatch")
    pod_spec = pod.get("spec", {})
    pod_containers = pod_spec.get("containers", [])
    pod_status = pod.get("status", {})
    statuses = pod_status.get("containerStatuses", []) if isinstance(pod_status, dict) else None
    if (not isinstance(pod_spec, dict) or not isinstance(pod_containers, list)
            or any(not isinstance(row, dict) for row in pod_containers)
            or not isinstance(statuses, list) or any(not isinstance(row, dict) for row in statuses)):
        _blocked("postgres_pod_not_ready")
    pod_db_containers = [row for row in pod_containers if row.get("name") == "postgres"]
    db_statuses = [row for row in statuses if row.get("name") == "postgres"]
    if (len(pod_db_containers) != 1 or pod_db_containers[0].get("image") != POSTGRES_IMAGE
            or len(db_statuses) != 1 or db_statuses[0].get("ready") is not True):
        _blocked("postgres_pod_not_ready")
    image_id = db_statuses[0].get("imageID", "")
    if not isinstance(image_id, str) or not re.search(r"sha256:[0-9a-f]{64}$", image_id):
        _blocked("postgres_running_image_identity_invalid")
    pod_ip = pod.get("status", {}).get("podIP")
    try:
        pod_ip = str(ipaddress.ip_address(pod_ip))
    except (TypeError, ValueError):
        _blocked("postgres_pod_ip_invalid")
    if (not isinstance(endpoint_addresses, list) or len(endpoint_addresses) != 1
            or endpoint_addresses[0] != pod_ip):
        _blocked("postgres_endpoint_address_mismatch")
    return {"pod": pod_name, "service": service_identity,
        "service_ports": copy.deepcopy(ports), "service_selector": copy.deepcopy(selector),
        "statefulset": statefulset_identity, "pod_identity": pod_identity,
        "service_spec_sha256": _digest_data(service_spec),
        "statefulset_spec_sha256": _digest_data(sts_spec),
        "pod_spec_sha256": _digest_data(pod_spec),
        "statefulset_generation": generation, "statefulset_replicas": sts_spec.get("replicas"),
        "statefulset_selector": copy.deepcopy(sts_spec.get("selector")),
        "statefulset_image": db_containers[0]["image"],
        "running_image_id": image_id, "pod_ip": pod_ip,
        "endpoint_addresses": copy.deepcopy(endpoint_addresses), "endpoint_port": 5432}


def _credential_secret_identity(kube):
    # Go-template emits key names and metadata only; Secret payload bytes never
    # enter this process or its output.
    template = ("go-template={{.metadata.name}}|{{.metadata.namespace}}|{{.metadata.uid}}|"
                "{{.metadata.resourceVersion}}|{{range $key, $_ := .data}}{{$key}} {{end}}")
    output = kube.run(["get", "secret", "voice-app-secrets", "-o", template], timeout=20)
    fields = output.split("|", 4) if isinstance(output, str) else []
    if len(fields) != 5 or fields[0] != "voice-app-secrets" or fields[1] != NAMESPACE:
        _blocked("migration_secret_metadata_invalid")
    keys = set(fields[4].split())
    required = {service.upper() + "_DATABASE_URL" for service in SERVICES}
    if (not fields[2] or not fields[3] or not required <= keys):
        _blocked("migration_secret_reference_missing")
    return {"name": fields[0], "namespace": fields[1], "uid": fields[2],
        "resourceVersion": fields[3], "keys": sorted(required)}


def _verify_live_targets(kube, prepared):
    validate_namespace(kube.get_namespace())
    current_postgres = _postgres_target_identity(kube)
    recorded_postgres = prepared.get("postgres_identity", {})
    stable_postgres_fields = ("pod", "service_ports", "service_selector",
        "statefulset_generation", "statefulset_replicas", "statefulset_selector",
        "service_spec_sha256", "statefulset_spec_sha256", "pod_spec_sha256",
        "statefulset_image", "running_image_id", "pod_ip", "endpoint_addresses", "endpoint_port")
    if (any(current_postgres.get(key) != recorded_postgres.get(key)
            for key in stable_postgres_fields)
            or any(current_postgres.get(key, {}).get("uid") != recorded_postgres.get(key, {}).get("uid")
                   for key in ("service", "statefulset", "pod_identity"))):
        _blocked("postgres_target_identity_changed")
    current_secret = _credential_secret_identity(kube)
    recorded_secret = prepared.get("credential_secret", {})
    if any(current_secret.get(key) != recorded_secret.get(key)
           for key in ("name", "namespace", "uid", "keys")):
        _blocked("migration_secret_identity_changed")


def preflight(kube, root, expected_source, *, plan=None):
    root = Path(root).resolve(strict=True)
    actual_source, dirty = _read_source_identity(root)
    validate_source(actual_source, expected_source, clean=not dirty)
    validate_namespace(kube.get_namespace())
    expected_plan = build_plan(root, actual_source)
    plan = expected_plan if plan is None else plan
    if plan != expected_plan:
        _blocked("plan_service_selection_invalid")
    postgres_identity = _postgres_target_identity(kube)
    secret_identity = _credential_secret_identity(kube)
    pod = postgres_identity["pod"]
    ledgers = {}
    for item in plan:
        version, dirty = _read_ledger(kube, item, pod)
        if dirty or version != item["baseline_version"]:
            _blocked("migration_ledger_preflight_failed")
        ledgers[item["service"]] = (version, dirty)
    return {"source_sha": actual_source, "namespace_uid": NAMESPACE_UID,
            "root": str(root),
            "postgres_pod": pod, "postgres_identity": postgres_identity,
            "credential_secret": secret_identity, "ledgers": ledgers, "plan": plan}


def _get_optional(kube, kind, name):
    try:
        return kube.get(kind, name)
    except Exception as error:
        if getattr(error, "status", None) == 404:
            return None
        raise


def execute(kube, prepared, *, timeout=600, poll_seconds=2):
    """Create-only sequential execution; ambiguous/failed jobs are never replayed."""
    root = Path(prepared.get("root", ""))
    if not root.is_dir():
        _blocked("prepared_source_root_invalid")
    current_source, dirty = _read_source_identity(root)
    validate_source(current_source, prepared.get("source_sha"), clean=not dirty)
    if prepared.get("plan") != build_plan(root, current_source):
        _blocked("prepared_source_plan_changed")
    _verify_live_targets(kube, prepared)
    if not isinstance(timeout, int) or not 1 <= timeout <= 600:
        _blocked("execution_timeout_invalid")
    for original_item in prepared["plan"]:
        item = {**original_item, "postgres_ip": prepared["postgres_identity"]["pod_ip"]}
        _verify_live_targets(kube, prepared)
        current, dirty = _read_ledger(kube, item, prepared["postgres_pod"])
        if dirty or current != item["baseline_version"]:
            _blocked("migration_ledger_changed_before_create")
        existing_cm = _get_optional(kube, "configmap", item["configmap_name"])
        if existing_cm is None:
            _verify_live_targets(kube, prepared)
            created = kube.run(["create", "-f", "/dev/stdin", "-o", "json"], body=configmap_manifest(item))
            readback = kube.get("configmap", item["configmap_name"])
            validate_create_readback(created, readback)
            validate_bound_object(readback, item, "configmap")
        else:
            validate_bound_object(existing_cm, item, "configmap")
        existing_job = _get_optional(kube, "job", item["job_name"])
        if existing_job is not None:
            if not existing_job.get("status", {}).get("succeeded"):
                _blocked("existing_job_will_not_be_replayed")
            validate_terminal_job(existing_job, item)
            if _read_ledger(kube, item, prepared["postgres_pod"]) != (item["expected_version"], False):
                _blocked("completed_job_ledger_mismatch")
            continue
        _verify_live_targets(kube, prepared)
        created = kube.run(["create", "-f", "/dev/stdin", "-o", "json"],
            body=job_manifest(item, item["postgres_ip"]))
        readback = kube.get("job", item["job_name"])
        validate_create_readback(created, readback)
        validate_bound_object(readback, item, "job")
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            current_job = kube.get("job", item["job_name"])
            validate_create_readback(created, current_job)
            if current_job.get("status", {}).get("failed"):
                _blocked("migration_job_failed_no_replay")
            if current_job.get("status", {}).get("succeeded"):
                validate_terminal_job(current_job, item)
                if _read_ledger(kube, item, prepared["postgres_pod"]) != (item["expected_version"], False):
                    _blocked("migration_job_ledger_readback_mismatch")
                break
            time.sleep(poll_seconds)
        else:
            _blocked("migration_job_timeout_no_replay")
    return True


def preflight_summary(prepared):
    """Expose only reviewed public identities and migration ledger metadata."""
    postgres = prepared["postgres_identity"]
    secret = prepared["credential_secret"]
    return {
        "mode": "preflight-only",
        "source_sha": prepared["source_sha"],
        "namespace": NAMESPACE,
        "namespace_uid": prepared["namespace_uid"],
        "images": {"migrate": MIGRATE_IMAGE, "postgres": POSTGRES_IMAGE},
        "postgres": {
            "pod": postgres["pod"],
            "service_uid": postgres["service"]["uid"],
            "service_resource_version": postgres["service"]["resourceVersion"],
            "statefulset_uid": postgres["statefulset"]["uid"],
            "statefulset_resource_version": postgres["statefulset"]["resourceVersion"],
            "pod_uid": postgres["pod_identity"]["uid"],
            "pod_resource_version": postgres["pod_identity"]["resourceVersion"],
            "pod_ip": postgres["pod_ip"],
            "endpoint_addresses": postgres["endpoint_addresses"],
            "endpoint_port": postgres["endpoint_port"],
            "image": postgres["statefulset_image"],
            "running_image_id": postgres["running_image_id"],
        },
        "secret": secret,
        "ledgers": {service: {"database": DATABASES[service],
            "version": prepared["ledgers"][service][0],
            "dirty": prepared["ledgers"][service][1]} for service in SERVICES},
        "migration_inputs": [{"service": item["service"], "database": item["database"],
            "input_sha256": item["input_hash"],
            "up_migrations": [{"name": name, "sha256": MIGRATION_HASHES[name]}
                for name in item["new_up_migrations"]]} for item in prepared["plan"]],
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--root", default=str(Path(__file__).resolve().parents[2]))
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--preflight-only", action="store_true",
        help="run read-only target identity and ledger checks; create no resources")
    mode.add_argument("--apply", action="store_true", help="create target-only DEV Jobs after full preflight")
    args = parser.parse_args(argv)
    try:
        root = Path(args.root).resolve(strict=True)
        plan = build_plan(root, args.source_sha)
        if args.preflight_only:
            prepared = preflight(Kubectl(), root, args.source_sha, plan=plan)
            print(json.dumps(preflight_summary(prepared), sort_keys=True, separators=(",", ":")))
            return 0
        if not args.apply:
            print(json.dumps({"namespace": NAMESPACE, "source_sha": args.source_sha,
                "services": [{"service": row["service"], "database": row["database"],
                    "baseline": row["baseline_version"], "target": row["expected_version"],
                    "migrations": row["new_up_migrations"], "job": row["job_name"]} for row in plan]},
                sort_keys=True, separators=(",", ":")))
            return 0
        kube = Kubectl()
        prepared = preflight(kube, root, args.source_sha, plan=plan)
        execute(kube, prepared)
        print("MATCH_SQUAD_DEV_MIGRATIONS_COMPLETE", flush=True)
        return 0
    except (Blocked, KubeBlocked) as error:
        print("BLOCKED=" + str(error), file=sys.stderr)
        return 2
    except KubeError:
        print("BLOCKED=kubernetes_api_error", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
