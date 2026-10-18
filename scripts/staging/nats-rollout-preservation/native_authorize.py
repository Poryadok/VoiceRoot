"""Closed native authorization boundary.

This module is intentionally the only authorization caller for the rootless
runner.  It accepts the already captured operation and a typed Kubernetes API;
there is no ambient Kube client, Docker runtime, shell, or fallback path here.
"""
from __future__ import annotations

import copy
import datetime as dt
import hashlib
import os
import stat
from pathlib import Path

from controller import Blocked, file_sha
from native_store import verify_archive


NAMESPACE = "voice-staging"


def _validate_supported_target(state):
    """Reject unsupported authorities before reading custody or touching an API."""
    target = state.get("target")
    if not isinstance(target, dict) or target.get("mode") != "images-only":
        raise Blocked("rollout_native_target_mode_unsupported")
    changed = target.get("changed_services")
    if changed is not None and (not isinstance(changed, list) or not changed
            or any(not isinstance(value, str) or not value for value in changed)
            or changed != sorted(set(changed))):
        raise Blocked("rollout_native_target_components_invalid")
    if any(state.get(key) is not None for key in
           ("gis_first_install", "gis_transition", "renderer_authority",
            "provisional", "repair_parent", "expired_recovery_authority")):
        raise Blocked("rollout_native_target_component_unsupported")
    nonnats = state.get("nonnats")
    if isinstance(nonnats, dict) and nonnats.get("actions"):
        raise Blocked("rollout_native_nonnats_actions_unsupported")
    space = state.get("space_preflight")
    if isinstance(space, dict) and isinstance(space.get("requirement"), dict):
        if space["requirement"].get("backup_required") is True:
            raise Blocked("rollout_native_space_backup_unsupported")
    services = state.get("service_actor_services")
    authority = state.get("service_actor_authority")
    if services is not None:
        if (not isinstance(services, list) or services != sorted(set(services))
                or not all(isinstance(value, str) and value for value in services)
                or not isinstance(authority, dict)
                or authority.get("schema") != "voice-nats-service-actor-authority-v1"
                or authority.get("roles") != sorted(authority.get("roles", []))):
            raise Blocked("rollout_native_actor_authority_invalid")
        if changed is not None and services != changed:
            raise Blocked("rollout_native_actor_services_changed")
    if (state.get("nats_contract") is not None
            or state.get("nats_target_scripts") is not None
            or state.get("migrations")):
        if not isinstance(state.get("nats_target_scripts"), dict):
            raise Blocked("rollout_native_nats_authority_invalid")
        if state.get("native_contract") is None:
            raise Blocked("rollout_native_contract_authority_missing")


def _read_contract_envelope(base, descriptor):
    if (not isinstance(descriptor, dict)
            or set(descriptor) != {"path", "sha256", "bytes"}
            or descriptor.get("path") != "native-contract-inputs.json"
            or not isinstance(descriptor.get("sha256"), str)
            or len(descriptor["sha256"]) != 64
            or type(descriptor.get("bytes")) is not int
            or descriptor["bytes"] <= 0):
        raise Blocked("rollout_native_contract_envelope_invalid")
    path = Path(base) / descriptor["path"]
    try:
        info = path.lstat()
        if (not stat.S_ISREG(info.st_mode) or stat.S_ISLNK(info.st_mode)
                or info.st_uid != os.geteuid() or info.st_nlink != 1
                or stat.S_IMODE(info.st_mode) != 0o600
                or info.st_size != descriptor["bytes"]):
            raise Blocked("rollout_native_contract_envelope_invalid")
        raw = path.open("rb", opener=lambda p, flags: os.open(
            p, flags | os.O_NOFOLLOW)).read()
    except (OSError, ValueError):
        raise Blocked("rollout_native_contract_envelope_invalid") from None
    if (len(raw) != descriptor["bytes"]
            or hashlib.sha256(raw).hexdigest() != descriptor["sha256"]):
        raise Blocked("rollout_native_contract_envelope_changed")
    return raw


def _deadline(state, authorization_deadline):
    if authorization_deadline is None:
        return dt.datetime.now(dt.timezone.utc) + dt.timedelta(hours=4)
    now = dt.datetime.now(dt.timezone.utc)
    if (not isinstance(authorization_deadline, dt.datetime)
            or authorization_deadline.tzinfo is None
            or not now < authorization_deadline <= now + dt.timedelta(seconds=600)):
        raise Blocked("preserved_upload_authorization_expired")
    expires = state.get("nats_contract_expires_at")
    if expires is not None and authorization_deadline > dt.datetime.fromisoformat(expires):
        raise Blocked("preserved_upload_authorization_expired")
    return authorization_deadline


def _source_fence(api, state, stage):
    import kube_json_api

    expected = state.get("native_source_binding")
    if not isinstance(expected, dict):
        raise Blocked("rollout_native_source_binding_invalid")
    try:
        resolved = kube_json_api.resolve_source_binding(
            api, operation=state["operation"], expected=expected)
    except Exception:
        raise Blocked("rollout_native_source_binding_changed") from None
    stage.verify_final_storage()
    source = resolved["binding"]
    if (stage.expected.get("source_claim") != source["source_claim"]
            or stage.expected.get("source_claim_uid") != source["source_uid"]
            or stage.expected.get("source_pv_uid") != source["source_pv_uid"]
            or stage.expected.get("source_node_uid") != resolved["proof"]["node_uid"]):
        raise Blocked("rollout_native_source_binding_mismatch")
    return resolved


def _cold_cut(base, state, archive_sha, manifest_sha):
    cut = state.get("cut")
    if not isinstance(cut, dict) or not isinstance(cut.get("manifest"), dict):
        raise Blocked("rollout_native_cold_cut_missing")
    manifest = cut["manifest"]
    if archive_sha != manifest.get("archive_sha256") or manifest_sha != cut.get("manifest_sha256"):
        raise Blocked("rollout_offnode_copy_mismatch")
    archive = Path(base) / "rollout-before.tar"
    manifest_file = Path(base) / "rollout-before-manifest.json"
    if (file_sha(manifest_file) != cut["manifest_sha256"]
            or verify_archive(archive, manifest) is not True):
        raise Blocked("rollout_offnode_copy_mismatch")
    return cut


def _native_contract(base, state):
    row = state.get("native_contract")
    if row is None:
        return None
    if (not isinstance(row, dict)
            or set(row) != {"request", "binding", "plan", "plan_envelope", "runner_image"}):
        raise Blocked("rollout_native_contract_authority_invalid")
    envelope = _read_contract_envelope(base, row["plan_envelope"])
    result = copy.deepcopy(row)
    result["plan_envelope"] = envelope
    return result


def _run_contract(api, base, state, journal, revalidate):
    row = _native_contract(base, state)
    if row is None:
        return None
    import transaction

    return transaction.run_native_contract_job(
        api, row["request"], row["binding"], runner_image=row["runner_image"],
        plan=row["plan"], plan_envelope=row["plan_envelope"],
        journal_events=state.get("events", ()), journal_event=journal,
        revalidate=revalidate)


def _run_migrations(api, state, journal, revalidate):
    plan = state.get("migrations")
    if plan is None:
        return None
    import native_migrations_runtime

    request = native_migrations_runtime.build_request(
        plan, state["target"]["mode"], state["operation"],
        authority_secret=state.get("native_migration_authority_secret"))
    view_factory = getattr(api, "make_native_migration_view", None)
    if not callable(view_factory):
        raise Blocked("native_migrations_typed_view_missing")
    view = view_factory(request)
    receipt = native_migrations_runtime.apply(
        view, request, journal_event=journal, revalidate=revalidate,
        journal_events=state.get("events", ()))
    return {"request_sha256": hashlib.sha256(
        native_migrations_runtime._canonical(request)).hexdigest(), "receipt": receipt}


def _post_archive_census(api, base, state, stage, native_config, cut, revalidate):
    """Capture an independent after cut before public ConfigMap CAS."""
    import transaction

    revalidate()
    runtime = transaction.native_preservation_runtime(
        base, state, stage, api, native_config, cut, "authorize")
    after = Path(base) / "rollout-after.tar"
    if after.exists() or after.is_symlink():
        raise Blocked("rollout_native_post_archive_reused")
    manifest = runtime.archive_selected_store(after)
    census = runtime.capture_census(after, manifest, "rollout-after", validate=revalidate)
    if not isinstance(census, dict):
        raise Blocked("rollout_native_post_census_invalid")
    plan = state.get("nats_contract")
    if plan is not None:
        import nats_contract_plan
        try:
            proof = nats_contract_plan.verify_census(plan, cut["census"], census)
        except Exception:
            raise Blocked("rollout_native_post_census_contract_mismatch") from None
    else:
        proof = {"before_sha256": cut["census_sha256"],
                 "after_sha256": hashlib.sha256(
                     __import__("json").dumps(census, sort_keys=True,
                     separators=(",", ":")).encode()).hexdigest()}
    return {"archive": after.name, "archive_sha256": manifest["archive_sha256"],
            "manifest": manifest, "census": census, "proof": proof}


def _cas_bootstrap(api, state, journal, revalidate):
    targets = state.get("nats_target_scripts")
    if targets is None:
        return []
    contract = copy.deepcopy(state.get("contract"))
    if not isinstance(contract, dict) or not isinstance(contract.get("scripts"), list):
        raise Blocked("nats_contract_script_authority_missing")
    by_part = {row.get("part"): row for row in contract["scripts"] if isinstance(row, dict)}
    results = []
    for part in ("realtime", "analytics-chat", "search"):
        target = targets.get(part) if isinstance(targets, dict) else None
        expected = by_part.get(part)
        if not isinstance(target, dict) or not isinstance(expected, dict):
            raise Blocked("nats_contract_script_authority_missing")
        name = "voice-nats-" + part + "-bootstrap"
        current = api.get_configmap(NAMESPACE, name)
        metadata, data = current.get("metadata", {}), current.get("data", {})
        actual = data.get("bootstrap.sh")
        if (metadata.get("uid") != expected.get("configMapUID")
                or metadata.get("resourceVersion") != expected.get("configMapResourceVersion")
                or not isinstance(actual, str)
                or hashlib.sha256(actual.encode()).hexdigest() != expected.get("sha256")):
            raise Blocked("nats_contract_cm_drift")
        if actual != target.get("script"):
            revalidate()
            journal({"kind": "nats_contract_cm_intent", "part": part,
                     "uid": metadata["uid"], "resourceVersion": metadata["resourceVersion"],
                     "target_sha256": target.get("sha256")})
            updated = api.cas_nats_bootstrap_configmap(
                NAMESPACE, name, metadata["uid"], metadata["resourceVersion"],
                expected["sha256"], state["operation"], target["script"])
            revalidate()
        else:
            updated = current
        if (updated.get("data", {}).get("bootstrap.sh") != target.get("script")
                or updated.get("metadata", {}).get("uid") != metadata.get("uid")):
            raise Blocked("nats_contract_cm_target_changed")
        journal({"kind": "nats_contract_cm_applied", "part": part,
                 "uid": updated["metadata"]["uid"],
                 "resourceVersion": updated["metadata"].get("resourceVersion"),
                 "target_sha256": target["sha256"]})
        expected.update({"configMapResourceVersion": updated["metadata"].get("resourceVersion"),
                         "sha256": target["sha256"],
                         "bytes": len(target["script"].encode())})
        results.append({"part": part, "uid": updated["metadata"]["uid"],
                        "resourceVersion": updated["metadata"].get("resourceVersion"),
                        "sha256": target["sha256"]})
    state["contract"] = contract
    return results


def authorize(api, base, state, off_node_archive_sha, off_node_manifest_sha,
              contract, *, native_config, authorization_deadline=None):
    """Authorize a fenced native operation with one typed API call graph."""
    _validate_supported_target(state)
    if state.get("phase") != "AWAITING_OFF_NODE" or state.get("status") != "WAITING":
        raise Blocked("rollout_offnode_phase_invalid")
    if state.get("fence_status") != "VERIFIED":
        raise Blocked("rollout_native_fence_invalid")
    if contract != state.get("contract"):
        raise Blocked("rollout_native_contract_changed")
    expires = _deadline(state, authorization_deadline)
    import transaction

    stage = None
    journal = transaction.checkpoint_journal(base, state, lambda: stage)
    stage = transaction.reconstruct(None, state, journal, native_api=api)
    _source_fence(api, state, stage)
    cut = _cold_cut(base, state, off_node_archive_sha, off_node_manifest_sha)

    def revalidate():
        _source_fence(api, state, stage)

    contract_job = _run_contract(api, base, state, journal, revalidate)
    migrations = _run_migrations(api, state, journal, revalidate)
    post = _post_archive_census(api, base, state, stage, native_config, cut, revalidate)
    cas = _cas_bootstrap(api, state, journal, revalidate)
    revalidate()

    expected = stage.expected
    marker = stage.marker["metadata"]
    claim = stage.final_claim["metadata"]
    pv = stage.final_pv["metadata"]
    receipt = {
        "schema": "nats-rollout-preservation-v1",
        "operation": state["operation"],
        "namespace_uid": expected["namespace_uid"],
        "marker": {"uid": marker["uid"], "resourceVersion": marker["resourceVersion"],
                   **{key: stage.marker["data"][key]
                      for key in ("generation", "previousGeneration", "dataPVC")}},
        "pvc": {"name": claim["name"], "uid": claim["uid"],
                "pv_name": pv["name"]},
        "pv": {"uid": pv["uid"], "node": expected.get("source_node_name")},
        "target": state["target"],
        "backup": {"archive_sha256": cut["manifest"]["archive_sha256"],
                    "manifest_sha256": cut["manifest_sha256"],
                    "census_sha256": cut["census_sha256"], "off_node_verified": True},
        "expires_at": expires.isoformat(),
        "native": {"contract_job": contract_job, "migrations": migrations,
                    "post_archive": {key: post[key] for key in
                                      ("archive", "archive_sha256", "proof")},
                    "configmap_cas": cas},
    }
    transaction.save(Path(base) / "apply-authorization.json", receipt)
    state["authorization"] = copy.deepcopy(receipt)
    state["native_authorize"] = copy.deepcopy(receipt["native"])
    state["context"] = transaction.context(stage)
    state["phase"] = "PAUSED_APPLY"
    state["status"] = "WAITING"
    transaction.save(Path(base) / "checkpoint.json", state)
    return receipt
