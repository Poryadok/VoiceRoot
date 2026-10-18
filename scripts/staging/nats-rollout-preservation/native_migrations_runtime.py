"""Typed, create-only adapter for the canonical database migration plan.

The root transaction owns authorization, SQL preflight, the writer fence, and
the durable journal.  This module only turns the already captured
``migrations.py`` plan into a fixed request and drives a typed API supplied by
that transaction.  It never reads a Secret, invokes a shell, or retries an
uncertain Kubernetes mutation.
"""
from __future__ import annotations

import copy
import hashlib
import json
import re
from typing import Any

import guard
import migrations
from controller import Blocked


SCHEMA = "voice-native-migrations-v1"
PHASE = "DATABASE_MIGRATIONS"
NAMESPACE = guard.NS
# Migration Jobs deliberately keep automountServiceAccountToken=false and do
# not select a privileged account.  The typed API caller is bound to the
# existing runner ServiceAccount used by kube_json_api; no caller supplied
# account is accepted.
SERVICE_ACCOUNT = "voice-nats-staging-runner"
MAX_RESOURCES = (len(migrations.DATABASES) + 1) * 2 + 1
UUID_RE = re.compile(r"[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}")


def _blocked(code: str) -> None:
    raise Blocked("native_migrations_" + code)


def _canonical(value: Any) -> bytes:
    try:
        return json.dumps(value, sort_keys=True, separators=(",", ":"),
                          ensure_ascii=False, allow_nan=False).encode("utf-8")
    except (TypeError, ValueError, UnicodeError):
        _blocked("json_invalid")


def _digest(value: Any) -> str:
    return hashlib.sha256(_canonical(value)).hexdigest()


def _operation(operation: Any) -> str:
    if not isinstance(operation, str) or not re.fullmatch(r"[a-f0-9]{12}", operation):
        _blocked("operation_invalid")
    return operation


def _uid(value: Any) -> str:
    if not isinstance(value, str) or not UUID_RE.fullmatch(value):
        _blocked("uid_invalid")
    return value


def _identity(obj: Any, *, kind: str, name: str) -> dict:
    if not isinstance(obj, dict) or obj.get("kind") != kind:
        _blocked("resource_kind_invalid")
    metadata = obj.get("metadata")
    if (not isinstance(metadata, dict) or metadata.get("namespace") != NAMESPACE
            or metadata.get("name") != name):
        _blocked("resource_identity_invalid")
    return metadata


def _secret_rows(plan: list[dict]) -> list[dict]:
    return [row for row in plan if row["database"] in migrations.LEGACY_DSN]


def _validate_legacy_secret(secret: Any, plan: list[dict], operation: str) -> dict | None:
    legacy = _secret_rows(plan)
    if not legacy:
        if secret is not None:
            _blocked("unexpected_authority_secret")
        return None
    if not isinstance(secret, dict) or set(secret) != {
            "apiVersion", "kind", "type", "immutable", "metadata", "data"}:
        _blocked("authority_secret_shape_invalid")
    if secret.get("apiVersion") != "v1" or secret.get("kind") != "Secret" \
            or secret.get("type") != "Opaque" or secret.get("immutable") is not True:
        _blocked("authority_secret_spec_invalid")
    metadata = _identity(secret, kind="Secret",
                         name="voice-rollout-" + operation + "-database-urls")
    if metadata.get("labels") != {"voice-rollout-operation": operation}:
        _blocked("authority_secret_labels_invalid")
    data = secret.get("data")
    wanted = {row["secret_key"] for row in legacy}
    if not isinstance(data, dict) or set(data) != wanted:
        _blocked("authority_secret_keys_invalid")
    # The runtime supplies this object after its source Secret/configmap
    # preflight.  Validate framing without ever decoding or logging values.
    for value in data.values():
        if (not isinstance(value, str) or not 1 <= len(value) <= 65536
                or not re.fullmatch(r"[A-Za-z0-9+/]+={0,2}", value)):
            _blocked("authority_secret_value_invalid")
    return copy.deepcopy(secret)


def _resource_key(resource: dict) -> str:
    return resource["kind"] + "/" + resource["metadata"]["name"]


def _validate_documents(resources: Any, plan: list[dict], operation: str) -> list[dict]:
    if (not isinstance(resources, list)
            or (plan and not 1 <= len(resources) <= MAX_RESOURCES)
            or (not plan and len(resources) != 0)):
        _blocked("resource_count_invalid")
    wanted = migrations.documents(plan, operation)
    if resources != wanted:
        _blocked("resource_projection_changed")
    seen: set[str] = set()
    for resource in resources:
        if not isinstance(resource, dict) or resource.get("metadata", {}).get("namespace") != NAMESPACE:
            _blocked("resource_namespace_invalid")
        key = _resource_key(resource)
        if key in seen:
            _blocked("resource_duplicate")
        seen.add(key)
        if resource["kind"] == "ConfigMap":
            if resource.get("immutable") is not True:
                _blocked("configmap_mutable")
        elif resource["kind"] == "Job":
            spec = resource.get("spec")
            template = spec.get("template") if isinstance(spec, dict) else None
            pod = template.get("spec") if isinstance(template, dict) else None
            if (spec.get("backoffLimit") != 0 or spec.get("activeDeadlineSeconds") != 300
                    or not isinstance(pod, dict) or pod.get("automountServiceAccountToken") is not False
                    or pod.get("securityContext") != {
                        "runAsNonRoot": True, "runAsUser": 65532, "runAsGroup": 65532}):
                _blocked("job_profile_invalid")
        else:
            _blocked("resource_kind_invalid")
    return copy.deepcopy(resources)


def build_request(plan: Any, mode: str, operation: Any, *, authority_secret: Any = None) -> dict:
    """Build an immutable, source-backed request for the runtime consumer.

    ``authority_secret`` is an already prepared Secret manifest from the
    caller's read-only preflight.  It is optional only when no legacy DSN row
    is present.  No Kubernetes object is read here.
    """
    operation = _operation(operation)
    try:
        checked = migrations.validate_plan(copy.deepcopy(plan), mode)
    except Exception:
        raise
    if not isinstance(checked, list):
        _blocked("plan_invalid")
    checked = copy.deepcopy(checked)
    authority = _validate_legacy_secret(authority_secret, checked, operation)
    resources = _validate_documents(migrations.documents(checked, operation), checked, operation)
    if authority is not None:
        resources.insert(0, authority)
    return {
        "schema": SCHEMA,
        "phase": PHASE,
        "operation": operation,
        "mode": mode,
        "namespace": NAMESPACE,
        "service_account": SERVICE_ACCOUNT,
        "plan_sha256": _digest(checked),
        "plan": checked,
        "resources": resources,
    }


def _validate_request(request: Any) -> dict:
    if not isinstance(request, dict) or set(request) != {
            "schema", "phase", "operation", "mode", "namespace", "service_account",
            "plan_sha256", "plan", "resources"}:
        _blocked("request_shape_invalid")
    if (request["schema"] != SCHEMA or request["phase"] != PHASE
            or request["namespace"] != NAMESPACE
            or request["service_account"] != SERVICE_ACCOUNT):
        _blocked("request_binding_invalid")
    operation = _operation(request["operation"])
    checked = migrations.validate_plan(copy.deepcopy(request["plan"]), request["mode"])
    if request["plan_sha256"] != _digest(checked):
        _blocked("plan_digest_invalid")
    if not isinstance(request["resources"], list) or any(
            not isinstance(resource, dict) for resource in request["resources"]):
        _blocked("resource_projection_changed")
    resources = _validate_documents(
        [resource for resource in request["resources"] if resource.get("kind") != "Secret"],
        checked, operation)
    authority = next((resource for resource in request["resources"]
                      if resource.get("kind") == "Secret"), None)
    _validate_legacy_secret(authority, checked, operation)
    if len(request["resources"]) != len(resources) + int(authority is not None):
        _blocked("resource_projection_changed")
    result = copy.deepcopy(request)
    result["plan"] = checked
    result["resources"] = ([copy.deepcopy(authority)] if authority is not None else []) + resources
    return result


def _expected_pod(job: dict) -> dict:
    return job["spec"]["template"]["spec"]


def _verify_created(observed: Any, wanted: dict, expected_uid: Any = None) -> dict:
    if not isinstance(observed, dict):
        _blocked("create_receipt_invalid")
    metadata = observed.get("metadata")
    if not isinstance(metadata, dict) or metadata.get("name") != wanted["metadata"]["name"] \
            or metadata.get("namespace") != NAMESPACE:
        _blocked("create_receipt_invalid")
    uid = _uid(metadata.get("uid"))
    if expected_uid is not None and uid != expected_uid:
        _blocked("resource_uid_changed")
    # Reconciliation is safe only when the returned object still projects to
    # the exact immutable manifest.  Ignore API-server identity fields, but
    # reject spec/data/label drift and unexpected caller-controlled fields.
    def projection(value: dict) -> dict:
        projected = copy.deepcopy(value)
        projected.pop("status", None)
        meta = projected.get("metadata", {})
        projected["metadata"] = {key: copy.deepcopy(meta[key]) for key in (
            "name", "namespace", "labels", "annotations") if key in meta}
        return projected
    if projection(observed) != projection(wanted):
        _blocked("resource_projection_changed")
    return {"name": metadata["name"], "namespace": NAMESPACE, "uid": uid,
            "kind": wanted["kind"]}


def _events_for(events: list, key: str, value: Any) -> list[dict]:
    return [event for event in events if isinstance(event, dict) and event.get(key) == value]


def _event_for(events: list, key: str, value: Any, event_kind: str) -> dict | None:
    rows = [event for event in _events_for(events, key, value)
            if event.get("kind") == event_kind]
    if len(rows) > 1:
        _blocked("journal_ambiguous")
    return rows[0] if rows else None


def apply(api: Any, request: dict, *, journal_event, revalidate,
          journal_events=()) -> dict:
    """Apply the request via the typed API and return a durable receipt.

    Required API methods are ``get_resource(kind, namespace, name)``,
    ``create_resource(kind, namespace, manifest)``, ``wait_job(namespace,
    name, uid, timeout_seconds)``, and ``get_job_pod(namespace, name, uid)``.
    Implementations must map these to direct API calls; shell/kubectl fallbacks
    are intentionally outside this ABI.
    """
    request = _validate_request(request)
    if not callable(journal_event) or not callable(revalidate):
        _blocked("callbacks_invalid")
    history = [copy.deepcopy(event) for event in journal_events]
    if not isinstance(journal_events, (list, tuple)):
        _blocked("journal_shape_invalid")

    def append(event: dict) -> None:
        journal_event(copy.deepcopy(event))
        history.append(copy.deepcopy(event))

    results: list[dict] = []
    jobs: list[dict] = []
    for resource in request["resources"]:
        kind = resource["kind"]
        name = resource["metadata"]["name"]
        key = _resource_key(resource)
        base = {"operation": request["operation"], "resource": key,
                "resource_kind": kind, "name": name, "namespace": NAMESPACE}
        intent = {"kind": "native_migration_resource_intent", **base,
                  "manifest_sha256": _digest(resource)}
        issued = {"kind": "native_migration_resource_issued", **base,
                  "manifest_sha256": _digest(resource)}
        created_kind = "native_migration_resource_created"
        prior_intent = _event_for(history, "resource", key, intent["kind"])
        prior_issued = _event_for(history, "resource", key, issued["kind"])
        prior_created = _event_for(history, "resource", key, created_kind)
        if prior_intent not in (None, intent) or prior_issued not in (None, issued):
            _blocked("journal_changed")
        if prior_created is not None and (set(prior_created) != set(base) | {
                "kind", "uid", "manifest_sha256"} or
                any(prior_created.get(k) != base[k] for k in base)
                or prior_created.get("manifest_sha256") != _digest(resource)):
            _blocked("journal_changed")

        revalidate()
        try:
            current = api.get_resource(kind, NAMESPACE, name)
        except Exception as error:
            if error.__class__.__name__ not in ("KubeNotFound", "NotFound"):
                _blocked("read_unknown")
            current = None
        if current is not None and prior_intent is None:
            _blocked("preexisting_resource")
        if prior_intent is None:
            append(intent)
        if current is None:
            if prior_issued is not None:
                _blocked("create_unknown")
            revalidate()
            append(issued)
            try:
                current = api.create_resource(kind, NAMESPACE, copy.deepcopy(resource))
            except Exception:
                # One read-only reconciliation is allowed.  Never POST again.
                try:
                    current = api.get_resource(kind, NAMESPACE, name)
                except Exception:
                    _blocked("create_unknown")
                if current is None:
                    _blocked("create_unknown")
        identity = _verify_created(current, resource,
                                   (prior_created or {}).get("uid"))
        created = {"kind": created_kind, **base, "uid": identity["uid"],
                   "manifest_sha256": _digest(resource)}
        if prior_created is not None and prior_created != created:
            _blocked("resource_uid_changed")
        if prior_created is None:
            append(created)
        results.append(identity)

        if kind != "Job":
            continue
        revalidate()
        try:
            completed = api.wait_job(NAMESPACE, name, identity["uid"], 300)
        except Exception:
            _blocked("job_wait_unknown")
        if (not isinstance(completed, dict)
                or completed.get("metadata", {}).get("uid") != identity["uid"]
                or completed.get("status", {}).get("succeeded") != 1):
            _blocked("job_not_complete")
        revalidate()
        try:
            pod = api.get_job_pod(NAMESPACE, name, identity["uid"])
        except Exception:
            _blocked("pod_read_unknown")
        if not isinstance(pod, dict) or not isinstance(pod.get("metadata"), dict):
            _blocked("pod_identity_invalid")
        pod_uid = _uid(pod["metadata"].get("uid"))
        if pod.get("spec") != _expected_pod(resource):
            _blocked("pod_profile_changed")
        profile_sha = _digest(_expected_pod(resource))
        job_result = {"name": name, "uid": identity["uid"], "pod_name": pod["metadata"].get("name"),
                      "pod_uid": pod_uid, "profile_sha256": profile_sha,
                      "configmap_name": name}
        jobs.append(job_result)
        append({"kind": "native_migration_job_complete", **base,
                "uid": identity["uid"], "pod_uid": pod_uid,
                "pod_name": job_result["pod_name"], "profile_sha256": profile_sha})

    return {"schema": SCHEMA, "phase": PHASE, "operation": request["operation"],
            "mode": request["mode"], "namespace": NAMESPACE,
            "service_account": SERVICE_ACCOUNT, "plan_sha256": request["plan_sha256"],
            "resources": results, "jobs": jobs}
