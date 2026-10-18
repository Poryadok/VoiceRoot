#!/usr/bin/env python3
"""Plan or apply the reviewed MatchSquad trust overlay to the existing DEV objects.

The default command is read-only and writes a local, source-bound plan. Applying
requires that plan and its SHA-256. Each Kubernetes object is changed by one
JSON Patch request guarded by UID and resourceVersion tests. The sequence stops
on the first drift or uncertain result and never retries or rolls back.
"""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

NS = "voice-dev"
EXPECTED_NAMESPACE_UID = "15950fc8-0392-4053-9156-e95e78b596f9"
OWNERSHIP = {"voice.environment": "dev",
             "app.kubernetes.io/managed-by": "voice-dev"}
class _Target:
    __slots__ = ("kind", "name", "patch_file")

    def __init__(self, kind, name, patch_file):
        self.kind = kind
        self.name = name
        self.patch_file = patch_file


CONTAINERS = {
    "voice-matchmaking": "matchmaking",
    "voice-chat": "chat",
    "voice-voice": "voice",
    "voice-gateway": "gateway",
}
QUALIFIED_IMAGES = {
    "voice-gateway": "ghcr.io/poryadok/voiceroot/gateway@sha256:a6b9a6c6e8f28bc7632c961fec94b6906d6621f5ae45f2f4668a8441d8b82716",
    "voice-chat": "ghcr.io/poryadok/voiceroot/chat@sha256:69f759c98b7407a0b958e3f1c13ae845f1a78cb9a5ed424ebca30c1768a80a27",
    "voice-matchmaking": "ghcr.io/poryadok/voiceroot/matchmaking@sha256:7f10b25a34ddc6f301219c63a945e1aa8f89b3b3337b568f35a6d42790394a3e",
    "voice-voice": "ghcr.io/poryadok/voiceroot/voice@sha256:bd7eb5d1afa8ece622f02670978a07f022e114075065e09dc56160cd4726dc66",
}
EXPECTED_OBJECT_UIDS = {
    ("Service", "voice-matchmaking"): "4ef51303-f3ef-4ca1-8dd2-9ff280b7ec16",
    ("Service", "voice-chat"): "aaf56c0d-c1af-4221-adde-fc6c9664e30c",
    ("Service", "voice-voice"): "4f99e5d7-15ed-4ce9-bbc4-cd999fa91475",
    ("Deployment", "voice-matchmaking"): "f16ecb5d-3ad0-48ad-903b-91bff733d7b9",
    ("Deployment", "voice-chat"): "eeb8f50d-409a-44e3-93bb-e6ed042db811",
    ("Deployment", "voice-voice"): "ef11fadf-9857-42e1-a324-31c9a4978138",
    ("Deployment", "voice-gateway"): "56a7f8f9-de86-4bde-9052-8ffdfe9fcd7f",
}
MATCHMAKING_FS_GROUP_RECOVERY_TEMPLATE_SHA256 = (
    "39a62305c3fdb01824b47bce9dce6d7533c5010c91673cbfbfbf876d97995339")
TARGETS = (
    # Services are published before any new listener is rolled out.
    _Target("Service", "voice-matchmaking", "voice-matchmaking.service.patch.yaml"),
    _Target("Service", "voice-chat", "voice-chat.service.patch.yaml"),
    _Target("Service", "voice-voice", "voice-voice.service.patch.yaml"),
    # The publisher must start before its JWKS consumers.
    _Target("Deployment", "voice-matchmaking", "voice-matchmaking.runtime.patch.yaml"),
    _Target("Deployment", "voice-chat", "voice-chat.runtime.patch.yaml"),
    _Target("Deployment", "voice-voice", "voice-voice.runtime.patch.yaml"),
    _Target("Deployment", "voice-gateway", "voice-gateway.patch.yaml"),
)
RECOVERY_TARGET = _Target(
    "Deployment", "voice-matchmaking", "matchmaking-fsgroup-recovery.patch.yaml")
RECOVERY_MODE = "matchmaking-fsgroup-only"


class OverlayError(RuntimeError):
    pass


def sha256_bytes(value):
    return hashlib.sha256(value).hexdigest()


def canonical_bytes(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"),
                      ensure_ascii=False).encode("utf-8")


def object_hash(value):
    return sha256_bytes(canonical_bytes(value))


def rendered_hash(target, obj):
    if target.kind == "Deployment":
        projection = obj.get("spec", {}).get("template")
        if projection is None:
            raise OverlayError("rendered target projection missing")
    elif target.kind == "Service":
        projection = obj.get("spec", {}).get("ports")
        if projection is None:
            raise OverlayError("rendered target projection missing")
    else:
        raise OverlayError("unsupported rendered object kind")
    return object_hash(projection)


def observed_render_hash(target, obj):
    """Normalize only API defaults absent from kubectl's local strategic render."""
    if target.kind == "Deployment":
        projection = copy.deepcopy(obj.get("spec", {}).get("template"))
        if projection is None:
            raise OverlayError("rendered target projection missing")
        pod_spec = projection.get("spec", {})
        for container in pod_spec.get("containers", []):
            for port in container.get("ports", []):
                port.setdefault("protocol", "TCP")
        for volume in pod_spec.get("volumes", []):
            source = volume.get("configMap") or volume.get("secret")
            if source is not None:
                source.setdefault("defaultMode", 420)
    elif target.kind == "Service":
        projection = copy.deepcopy(obj.get("spec", {}).get("ports"))
        if projection is not None:
            for port in projection:
                port.setdefault("protocol", "TCP")
    else:
        raise OverlayError("unsupported rendered object kind")
    if projection is None:
        raise OverlayError("rendered target projection missing")
    return object_hash(projection)


def validate_source_sha(actual, expected):
    if (not isinstance(actual, str) or len(actual) != 40 or
            any(char not in "0123456789abcdef" for char in actual) or actual != expected):
        raise OverlayError("source checkout does not match requested commit")


def validate_namespace(namespace, expected_uid=None):
    metadata = namespace.get("metadata", {})
    if metadata.get("name") != NS or any(metadata.get("labels", {}).get(key) != value
                                         for key, value in OWNERSHIP.items()):
        raise OverlayError("DEV namespace ownership check failed")
    uid = metadata.get("uid")
    expected_uid = expected_uid or EXPECTED_NAMESPACE_UID
    if not uid or uid != expected_uid:
        raise OverlayError("DEV namespace UID changed or missing")
    return uid


def validate_target(target, obj):
    meta = obj.get("metadata", {})
    labels = meta.get("labels", {})
    if (obj.get("kind") != target.kind or meta.get("namespace") != NS or
            meta.get("name") != target.name or not meta.get("uid") or
            not meta.get("resourceVersion") or
            labels.get("app.kubernetes.io/managed-by") != "voice-dev" or
            ("voice.environment" in labels and labels["voice.environment"] != "dev")):
        raise OverlayError("target identity or DEV ownership mismatch: " + target.name)
    if target.kind == "Deployment":
        containers = obj.get("spec", {}).get("template", {}).get("spec", {}).get("containers", [])
        names = [container.get("name") for container in containers]
        if names.count(CONTAINERS[target.name]) != 1:
            raise OverlayError("exact application container missing: " + target.name)
    return meta["uid"], meta["resourceVersion"]


def _pointer(value):
    return str(value).replace("~", "~0").replace("/", "~1")


def _list_identity(item, field):
    if field in ("env", "ports", "volumeMounts", "volumes"):
        key = "containerPort" if field == "ports" and "containerPort" in item else "name"
        return item.get(key)
    if field == "servicePorts":
        return item.get("port")
    raise OverlayError("unapproved list field")


def _allowed_list_path(path):
    if path in ("/spec/ports", "/spec/ports/-", "/spec/template/spec/volumes",
                "/spec/template/spec/volumes/-"):
        return True
    parts = path.split("/")
    return (len(parts) in (7, 8) and parts[:5] == ["", "spec", "template", "spec", "containers"]
            and parts[5].isdigit() and parts[6] in ("env", "ports", "volumeMounts")
            and (len(parts) == 7 or parts[7] == "-"))


def _append_or_merge(old_list, new_list, path, identity_field, parent_exists=True,
                     allowed_removals=()):
    if not _allowed_list_path(path):
        raise OverlayError("unapproved list path")
    if not isinstance(old_list, list) or not isinstance(new_list, list):
        raise OverlayError("overlay list shape is invalid")
    if not parent_exists and old_list:
        raise OverlayError("missing list parent has existing items")
    old_by_id = {}
    for item in old_list:
        identity = _list_identity(item, identity_field)
        if identity is None or identity in old_by_id:
            raise OverlayError("base list identity is ambiguous")
        old_by_id[identity] = item
    new_by_id = {}
    for item in new_list:
        identity = _list_identity(item, identity_field)
        if identity is None or identity in new_by_id:
            raise OverlayError("rendered list identity is ambiguous")
        new_by_id[identity] = item
    removed = old_by_id.keys() - new_by_id.keys()
    if not removed <= set(allowed_removals):
        raise OverlayError("overlay would remove a list item")
    old_order = [_list_identity(item, identity_field) for item in old_list]
    rendered_old_order = [_list_identity(item, identity_field) for item in new_list
                          if _list_identity(item, identity_field) in old_by_id]
    if rendered_old_order != [identity for identity in old_order if identity not in removed]:
        raise OverlayError("overlay changed existing list order")
    operations = []
    for index in reversed(range(len(old_list))):
        identity = _list_identity(old_list[index], identity_field)
        if identity in removed:
            operations.append({"op": "remove", "path": path + "/" + str(index)})
    for index, old_item in enumerate(old_list):
        identity = _list_identity(old_item, identity_field)
        if identity in removed:
            continue
        if new_by_id[identity] != old_item:
            raise OverlayError("overlay would modify an existing list item")
    additions = []
    for new_item in new_list:
        identity = _list_identity(new_item, identity_field)
        if identity not in old_by_id:
            additions.append(new_item)
    if additions and not parent_exists:
        operations.append({"op": "add", "path": path,
                           "value": copy.deepcopy(new_list)})
    else:
        for new_item in additions:
            operations.append({"op": "add", "path": path + "/-", "value": new_item})
    return operations


def _apply_append_operations(base, operations):
    """Predict the API result of the exact list-add JSON Patch we submit."""
    result = copy.deepcopy(base)
    for operation in operations:
        path = operation.get("path", "")
        if operation.get("op") != "add" or not _allowed_list_path(path):
            raise OverlayError("generated overlay operation is not an approved list add")
        append = path.endswith("/-")
        encoded_parts = path.split("/")[1:]
        if append:
            encoded_parts = encoded_parts[:-1]
        parts = [part.replace("~1", "/").replace("~0", "~") for part in encoded_parts]
        parent = result
        try:
            for part in parts[:-1] if not append else parts:
                parent = parent[int(part)] if isinstance(parent, list) else parent[part]
            if append:
                if not isinstance(parent, list) or not isinstance(operation.get("value"), dict):
                    raise OverlayError("generated append target is not a list")
                parent.append(copy.deepcopy(operation["value"]))
            else:
                key = parts[-1]
                if not isinstance(parent, dict) or key in parent or not isinstance(operation.get("value"), list):
                    raise OverlayError("generated list parent add is not absent and list-shaped")
                parent[key] = copy.deepcopy(operation["value"])
        except (KeyError, IndexError, TypeError, ValueError) as error:
            raise OverlayError("generated list-add parent is absent or invalid") from error
    return result


def _apply_overlay_operations(base, operations, target):
    """Apply generated additions plus the two exact reviewed scalar/list changes."""
    result = copy.deepcopy(base)
    for operation in operations:
        if operation.get("op") == "add" and operation.get("path") in (
                "/spec/template/spec/securityContext",
                "/spec/template/spec/securityContext/fsGroup"):
            if (target.kind != "Deployment" or target.name != "voice-matchmaking" or
                    operation.get("value") != ({"fsGroup": 65532}
                    if operation["path"] == "/spec/template/spec/securityContext" else 65532)):
                raise OverlayError("generated fsGroup addition is outside the fixed Matchmaking setting")
            pod_spec = result.get("spec", {}).get("template", {}).get("spec", {})
            if operation["path"] == "/spec/template/spec/securityContext":
                if "securityContext" in pod_spec:
                    raise OverlayError("generated fsGroup parent already exists")
                pod_spec["securityContext"] = {"fsGroup": 65532}
            else:
                security_context = pod_spec.get("securityContext")
                if not isinstance(security_context, dict) or "fsGroup" in security_context:
                    raise OverlayError("generated fsGroup target is not absent")
                security_context["fsGroup"] = 65532
            continue
        if operation.get("op") != "remove":
            result = _apply_append_operations(result, [operation])
            continue
        if target.kind != "Deployment" or target.name != "voice-matchmaking":
            raise OverlayError("generated overlay removal is outside the fixed Matchmaking env")
        containers = result.get("spec", {}).get("template", {}).get("spec", {}).get(
            "containers", [])
        matches = [index for index, container in enumerate(containers)
                   if container.get("name") == CONTAINERS[target.name]]
        if len(matches) != 1:
            raise OverlayError("generated overlay removal lacks the exact Matchmaking container")
        prefix = "/spec/template/spec/containers/" + str(matches[0]) + "/env/"
        path = operation.get("path", "")
        suffix = path[len(prefix):] if path.startswith(prefix) else ""
        if not suffix.isdecimal():
            raise OverlayError("generated overlay removal is outside the fixed Matchmaking env")
        env = containers[matches[0]].get("env", [])
        index = int(suffix)
        if (index >= len(env) or
                env[index].get("name") != "S2S_JWKS_URLS_JSON"):
            raise OverlayError("generated overlay removal does not target the reviewed env")
        del env[index]
    return result


def object_patch(base, desired):
    """Validate a rendered overlay delta and return one atomic UID/RV JSON Patch."""
    if base.get("kind") != desired.get("kind") or base.get("apiVersion") != desired.get("apiVersion"):
        raise OverlayError("rendered target identity changed")
    base_uid, base_rv = validate_target(_Target(base["kind"], base["metadata"]["name"], ""), base)
    desired_uid, desired_rv = validate_target(
        _Target(desired["kind"], desired["metadata"]["name"], ""), desired)
    if (base_uid, base_rv) != (desired_uid, desired_rv):
        raise OverlayError("render operation changed target identity")

    desired_projection = copy.deepcopy(desired)
    delta = []
    if base["kind"] == "Service":
        if base["metadata"]["name"] not in ("voice-matchmaking", "voice-chat", "voice-voice"):
            raise OverlayError("Service outside fixed target set")
        delta.extend(_append_or_merge(base["spec"].get("ports", []),
                                      desired["spec"].get("ports", []),
                                      "/spec/ports", "servicePorts",
                                      parent_exists="ports" in base["spec"]))
    elif base["kind"] == "Deployment":
        name = base["metadata"]["name"]
        if name not in CONTAINERS:
            raise OverlayError("Deployment outside fixed target set")
        base_containers = base["spec"]["template"]["spec"]["containers"]
        desired_containers = desired["spec"]["template"]["spec"]["containers"]
        if [container.get("name") for container in base_containers] != [
                container.get("name") for container in desired_containers]:
            raise OverlayError("overlay changed container identities or order")
        main_index = [container.get("name") for container in base_containers].index(CONTAINERS[name])
        for index, fields in enumerate(("env", "ports", "volumeMounts")):
            old_container = base_containers[main_index]
            new_container = desired_containers[main_index]
            old_items = old_container.get(fields, [])
            new_items = new_container.get(fields, [])
            allowed_removals = ("S2S_JWKS_URLS_JSON",) if (
                name == "voice-matchmaking" and fields == "env") else ()
            delta.extend(_append_or_merge(
                old_items, new_items,
                "/spec/template/spec/containers/" + str(main_index) + "/" + fields,
                fields, parent_exists=fields in old_container,
                allowed_removals=allowed_removals))
        old_volumes = base["spec"]["template"]["spec"].get("volumes", [])
        new_volumes = desired["spec"]["template"]["spec"].get("volumes", [])
        delta.extend(_append_or_merge(old_volumes, new_volumes,
                                      "/spec/template/spec/volumes", "volumes",
                                      parent_exists="volumes" in base["spec"]["template"]["spec"]))
        old_security_context = base["spec"]["template"]["spec"].get("securityContext")
        new_security_context = desired["spec"]["template"]["spec"].get("securityContext")
        if old_security_context != new_security_context:
            if name != "voice-matchmaking":
                raise OverlayError("securityContext change is outside the fixed Matchmaking fsGroup")
            if ((old_security_context is not None and not isinstance(old_security_context, dict)) or
                    not isinstance(new_security_context, dict)):
                raise OverlayError("Matchmaking securityContext shape is invalid")
            if (old_security_context is not None and "fsGroup" in old_security_context and
                    old_security_context["fsGroup"] != 65532):
                raise OverlayError("Matchmaking fsGroup replacement is not permitted")
            expected_security_context = copy.deepcopy(old_security_context or {})
            expected_security_context["fsGroup"] = 65532
            if new_security_context != expected_security_context:
                raise OverlayError("Matchmaking securityContext changed outside fsGroup 65532")
            if old_security_context is None:
                delta.append({"op": "add", "path": "/spec/template/spec/securityContext",
                              "value": {"fsGroup": 65532}})
            elif "fsGroup" not in old_security_context:
                delta.append({"op": "add", "path": "/spec/template/spec/securityContext/fsGroup",
                              "value": 65532})
    else:
        raise OverlayError("object kind outside fixed target set")

    target = _Target(base["kind"], base["metadata"]["name"], "")
    projected = _apply_overlay_operations(base, delta, target)
    if base["kind"] == "Service":
        desired_projection["spec"]["ports"] = projected["spec"]["ports"]
    else:
        desired_spec = desired_projection["spec"]["template"]["spec"]
        projected_spec = projected["spec"]["template"]["spec"]
        desired_container = desired_spec["containers"][main_index]
        projected_container = projected_spec["containers"][main_index]
        for fields in ("env", "ports", "volumeMounts"):
            if fields in desired_container or fields in projected_container:
                desired_container[fields] = projected_container.get(fields, [])
        if "volumes" in desired_spec or "volumes" in projected_spec:
            desired_spec["volumes"] = projected_spec.get("volumes", [])
    if projected != desired_projection:
        raise OverlayError("overlay changed an unapproved object path")
    # The reducer above admits list additions only at the reviewed paths and
    # one removal only for Matchmaking's exact optional S2S env identity.
    return ([{"op": "test", "path": "/metadata/uid", "value": base_uid},
             {"op": "test", "path": "/metadata/resourceVersion", "value": base_rv}]
            + delta)


def strategic_render(client, target, base, patch_path):
    return client.render(target, base, patch_path)


def _has_reviewed_matchmaking_listener_opt_in(target, base):
    """Recognize only the Matchmaking setting this overlay removes."""
    if target.kind != "Deployment" or target.name != "voice-matchmaking":
        return False
    containers = base.get("spec", {}).get("template", {}).get("spec", {}).get(
        "containers", [])
    matches = [container for container in containers
               if container.get("name") == CONTAINERS[target.name]]
    if len(matches) != 1:
        return False
    env = matches[0].get("env", [])
    return sum(item.get("name") == "S2S_JWKS_URLS_JSON" for item in env) == 1


def _is_exact_matchmaking_listener_removal(target, base, operations):
    if not _has_reviewed_matchmaking_listener_opt_in(target, base):
        return False
    containers = base["spec"]["template"]["spec"]["containers"]
    container_index = next(index for index, item in enumerate(containers)
                           if item.get("name") == CONTAINERS[target.name])
    env = containers[container_index].get("env", [])
    env_index = next(index for index, item in enumerate(env)
                     if item.get("name") == "S2S_JWKS_URLS_JSON")
    return operations[2:] == [{
        "op": "remove",
        "path": "/spec/template/spec/containers/{}/env/{}".format(
            container_index, env_index),
    }]


def _is_exact_matchmaking_fs_group_recovery(target, base, operations):
    """Allow only the already-deployed Matchmaking template's missing fsGroup."""
    if target.kind != "Deployment" or target.name != "voice-matchmaking":
        return False
    template = base.get("spec", {}).get("template")
    if not isinstance(template, dict) or sha256_bytes(canonical_bytes(template)) != (
            MATCHMAKING_FS_GROUP_RECOVERY_TEMPLATE_SHA256):
        return False
    if _has_reviewed_matchmaking_listener_opt_in(target, base):
        return False
    return operations[2:] == [{
        "op": "add", "path": "/spec/template/spec/securityContext/fsGroup",
        "value": 65532,
    }]


def _validate_matchmaking_fs_group_recovery_base(target, base, status):
    if target != RECOVERY_TARGET:
        return False
    replicas = base.get("spec", {}).get("replicas", 1)
    generation = base.get("metadata", {}).get("generation", 0)
    template = base.get("spec", {}).get("template")
    if (not isinstance(template, dict) or sha256_bytes(canonical_bytes(template)) !=
            MATCHMAKING_FS_GROUP_RECOVERY_TEMPLATE_SHA256):
        raise OverlayError("fsGroup recovery template does not match the pinned projection")
    if _has_reviewed_matchmaking_listener_opt_in(target, base):
        raise OverlayError("fsGroup recovery requires the S2S entry to be absent")
    containers = base.get("spec", {}).get("template", {}).get("spec", {}).get(
        "containers", [])
    matches = [container for container in containers
               if container.get("name") == CONTAINERS[target.name]]
    if len(matches) != 1 or matches[0].get("image") != QUALIFIED_IMAGES[target.name]:
        raise OverlayError("fsGroup recovery image projection is not source-qualified")
    if (replicas <= 0 or status.get("observedGeneration", 0) < generation or
            status.get("updatedReplicas", 0) < replicas):
        raise OverlayError("fsGroup recovery requires the current generation and updated replicas")
    if (status.get("readyReplicas", 0) >= replicas and
            status.get("availableReplicas", 0) >= replicas):
        raise OverlayError("fsGroup recovery is only for an unready Matchmaking Deployment")
    return True


def build_object_plan(client, target, base, patch_path):
    validate_target(target, base)
    if base["metadata"]["uid"] != EXPECTED_OBJECT_UIDS[(target.kind, target.name)]:
        raise OverlayError("target object UID is outside the bound DEV activation")
    operations = None
    if target.kind == "Deployment":
        status = base.get("status", {})
        replicas = base.get("spec", {}).get("replicas", 1)
        generation_current = status.get("observedGeneration", 0) >= base["metadata"].get("generation", 0)
        updated = status.get("updatedReplicas", 0) >= replicas
        ready = (status.get("readyReplicas", 0) >= replicas and
                 status.get("availableReplicas", 0) >= replicas)
        fs_group_recovery = _validate_matchmaking_fs_group_recovery_base(
            target, base, status)
        if not (generation_current and updated and ready):
            possible_recovery = (
                target.name == "voice-matchmaking" and replicas > 0 and
                generation_current and updated and
                _has_reviewed_matchmaking_listener_opt_in(target, base))
            if not fs_group_recovery and not possible_recovery:
                raise OverlayError("Deployment is not ordinarily ready: " + target.name)
            desired = strategic_render(client, target, base, patch_path)
            operations = object_patch(base, desired)
            if fs_group_recovery:
                if not _is_exact_matchmaking_fs_group_recovery(target, base, operations):
                    raise OverlayError("fsGroup recovery projection is not exact")
            elif not _is_exact_matchmaking_listener_removal(target, base, operations):
                raise OverlayError("unready Matchmaking recovery projection is not exact")
        containers = base["spec"]["template"]["spec"]["containers"]
        app_container = next(container for container in containers
                             if container.get("name") == CONTAINERS[target.name])
        if app_container.get("image") != QUALIFIED_IMAGES[target.name]:
            raise OverlayError("source-qualified application image not active: " + target.name)
    patch_bytes = patch_path.read_bytes()
    if operations is None:
        desired = strategic_render(client, target, base, patch_path)
        operations = object_patch(base, desired)
    already_satisfied = len(operations) == 2
    return {
        "kind": target.kind, "name": target.name,
        "uid": base["metadata"]["uid"], "resourceVersion": base["metadata"]["resourceVersion"],
        "baseObjectSha256": object_hash(base),
        "patchFile": target.patch_file,
        "patchSha256": sha256_bytes(patch_bytes),
        "desiredRenderSha256": rendered_hash(
            target, _apply_overlay_operations(base, operations[2:], target)),
        "desiredObservedSha256": observed_render_hash(
            target, _apply_overlay_operations(base, operations[2:], target)),
        "alreadySatisfied": already_satisfied,
        "operations": operations,
    }


class KubectlClient:
    def __init__(self, namespace=NS):
        self.namespace = namespace

    def call(self, *args, body=None, input_bytes=None):
        command = ["kubectl", "--namespace", self.namespace, *args]
        payload = input_bytes
        if body is not None:
            payload = json.dumps(body, separators=(",", ":")).encode("utf-8")
        try:
            result = subprocess.run(command, input=payload, capture_output=True,
                                    timeout=90, check=False)
        except (OSError, subprocess.TimeoutExpired) as exc:
            raise OverlayError("kubectl transport failed") from exc
        if result.returncode:
            # kubectl's private stderr may contain full object data. Never echo it.
            raise OverlayError("kubectl command failed stage=" + args[0] +
                               " exit=" + str(result.returncode))
        try:
            return json.loads(result.stdout) if result.stdout.strip() else {}
        except (json.JSONDecodeError, UnicodeDecodeError) as exc:
            raise OverlayError("kubectl returned invalid JSON") from exc

    def get_namespace(self):
        return self.call("get", "namespace", NS, "-o", "json")

    def get(self, target):
        return self.call("get", target.kind.lower(), target.name, "-o", "json")

    def render(self, target, base, patch_path):
        with tempfile.TemporaryDirectory(prefix="match-squad-overlay-") as temp_dir:
            base_path = Path(temp_dir) / "base.json"
            base_path.write_bytes(canonical_bytes(base))
            command = ["kubectl", "--namespace", NS, "patch", "--local", "-f",
                       str(base_path), "--type=strategic", "--patch-file", str(patch_path),
                       "-o", "json"]
            try:
                result = subprocess.run(command, capture_output=True, timeout=30, check=False)
            except (OSError, subprocess.TimeoutExpired) as exc:
                raise OverlayError("local strategic render unavailable") from exc
            if result.returncode:
                raise OverlayError("local strategic render failed: " + target.patch_file)
            try:
                return json.loads(result.stdout)
            except (json.JSONDecodeError, UnicodeDecodeError) as exc:
                raise OverlayError("local strategic render returned invalid JSON") from exc

    def apply(self, plan):
        result = self.call("patch", plan["kind"].lower(), plan["name"],
                           "--type=json", "--patch=" + json.dumps(plan["operations"],
                                                                      separators=(",", ":")),
                           "-o", "json")
        target = _Target(plan["kind"], plan["name"], plan["patchFile"])
        if (result.get("metadata", {}).get("uid") != plan["uid"] or
                result.get("metadata", {}).get("resourceVersion") in
                (None, plan["resourceVersion"]) or
                observed_render_hash(target, result) != plan["desiredObservedSha256"]):
            error = OverlayError("patched object did not match the frozen desired hash")
            error.persisted = (result.get("metadata", {}).get("uid") == plan["uid"] and
                               result.get("metadata", {}).get("resourceVersion") not in
                               (None, plan["resourceVersion"]))
            raise error
        return result

    def server_dry_run(self, plan):
        """Ask the API server to validate the exact guarded patch without writing."""
        result = self.call("patch", plan["kind"].lower(), plan["name"],
                           "--type=json", "--patch=" + json.dumps(plan["operations"],
                                                                      separators=(",", ":")),
                           "--dry-run=server", "-o", "json")
        target = _Target(plan["kind"], plan["name"], plan["patchFile"])
        if (result.get("metadata", {}).get("uid") != plan["uid"] or
                observed_render_hash(target, result) != plan["desiredObservedSha256"]):
            raise OverlayError("server dry-run result did not match frozen desired hash")
        return result

    def wait_deployment(self, plan):
        self.call("rollout", "status", "deployment/" + plan["name"], "--timeout=600s")
        target = _Target("Deployment", plan["name"], plan["patchFile"])
        current = self.get(target)
        status = current.get("status", {})
        replicas = current.get("spec", {}).get("replicas", 1)
        if (current.get("metadata", {}).get("uid") != plan["uid"] or
                observed_render_hash(target, current) != plan["desiredObservedSha256"] or
                status.get("observedGeneration", 0) < current.get("metadata", {}).get("generation", 0) or
                status.get("updatedReplicas", 0) < replicas or
                status.get("readyReplicas", 0) < replicas or
                status.get("availableReplicas", 0) < replicas):
            raise OverlayError("Deployment rollout did not reach the frozen ready state")

    def verify_service_endpoints(self, name, expected_ports):
        slices = self.call("get", "endpointslices",
                           "-l", "kubernetes.io/service-name=" + name, "-o", "json")
        ready_ports = set()
        ready_addresses = 0
        for item in slices.get("items", []):
            ports = {entry.get("name"): entry.get("port") for entry in item.get("ports", [])}
            has_ready_endpoint = any(endpoint.get("conditions", {}).get("ready") is True
                                     and endpoint.get("addresses")
                                     for endpoint in item.get("endpoints", []))
            if has_ready_endpoint:
                ready_addresses += 1
                ready_ports.update((port_name, port) for port_name, port in ports.items())
        if ready_addresses == 0 or not set(expected_ports.items()) <= ready_ports:
            raise OverlayError("Service endpoints are not ready for " + name)


def _source_identity(root):
    def git(*args):
        try:
            return subprocess.check_output(["git", "-C", str(root), *args],
                                           stderr=subprocess.DEVNULL).decode().strip()
        except (OSError, subprocess.CalledProcessError, UnicodeDecodeError) as exc:
            raise OverlayError("source identity unavailable") from exc
    head = git("rev-parse", "HEAD")
    if git("status", "--porcelain"):
        raise OverlayError("source checkout is dirty")
    return head


def _write_plan(path, plan):
    data = canonical_bytes(plan) + b"\n"
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL
    fd = os.open(path, flags, 0o600)
    with os.fdopen(fd, "wb") as stream:
        stream.write(data)
        stream.flush()
        os.fsync(stream.fileno())
    return sha256_bytes(data)


def _read_plan(path, expected_sha):
    data = path.read_bytes()
    if sha256_bytes(data) != expected_sha:
        raise OverlayError("plan digest mismatch")
    try:
        plan = json.loads(data)
    except (json.JSONDecodeError, UnicodeDecodeError) as exc:
        raise OverlayError("plan file is invalid") from exc
    return plan


def _selected_targets(recovery_only):
    return (RECOVERY_TARGET,) if recovery_only else TARGETS


def _target_patch_path(overlay_dir, target, recovery_only):
    if recovery_only:
        if target is not RECOVERY_TARGET:
            raise OverlayError("recovery target is not fixed")
        return overlay_dir / RECOVERY_TARGET.patch_file
    return overlay_dir / target.patch_file


def execute(root, source_sha, plan_path, plan_sha=None, apply=False, client=None,
            recovery_only=False):
    root = Path(root).resolve()
    plan_path = plan_path.resolve()
    if plan_path.is_relative_to(root):
        raise OverlayError("plan must be stored outside the source checkout")
    validate_source_sha(_source_identity(root), source_sha)
    overlay_dir = root / "deploy/dev/match-squad-trust-overlay"
    client = client or KubectlClient()
    namespace_uid = validate_namespace(client.get_namespace())
    selected_targets = _selected_targets(recovery_only)

    if not apply:
        if plan_path.exists():
            raise OverlayError("plan path already exists; choose a new path")
        object_plans = []
        for target in selected_targets:
            patch_path = _target_patch_path(overlay_dir, target, recovery_only)
            base = client.get(target)
            object_plans.append(build_object_plan(client, target, base, patch_path))
        plan = {"format": 1, "sourceSha": source_sha, "namespace": NS,
                "namespaceUid": namespace_uid, "targets": object_plans}
        if recovery_only:
            plan["recoveryMode"] = RECOVERY_MODE
        digest = _write_plan(plan_path, plan)
        for item in object_plans:
            print("MATCH_SQUAD_OVERLAY_PLANNED kind=" + item["kind"] +
                  " name=" + item["name"] + " uid=" + item["uid"] +
                  " rv=" + item["resourceVersion"] +
                  " render=" + item["desiredRenderSha256"])
        print("MATCH_SQUAD_OVERLAY_PLAN_READY count=" + str(len(object_plans)) +
              " sha256=" + digest)
        return

    if not plan_sha:
        raise OverlayError("apply requires --plan-sha256")
    plan = _read_plan(plan_path, plan_sha)
    if (plan.get("format") != 1 or plan.get("sourceSha") != source_sha or
            plan.get("namespace") != NS or plan.get("namespaceUid") != namespace_uid or
            (plan.get("recoveryMode") != RECOVERY_MODE if recovery_only else
             "recoveryMode" in plan) or
            len(plan.get("targets", [])) != len(selected_targets)):
        raise OverlayError("plan source or namespace binding changed")
    fresh_plans = []
    for target, frozen in zip(selected_targets, plan["targets"]):
        if (frozen.get("kind"), frozen.get("name"), frozen.get("patchFile")) != (
                target.kind, target.name, target.patch_file):
            raise OverlayError("plan target order changed")
        patch_path = _target_patch_path(overlay_dir, target, recovery_only)
        base = client.get(target)
        fresh = build_object_plan(client, target, base, patch_path)
        if fresh != frozen:
            raise OverlayError("target drift or overlay source changed: " + target.name)
        fresh_plans.append(fresh)

    # Validate every effective patch against live admission/schema before the
    # first write. The eventual request repeats UID/RV tests to close races.
    for target, plan in zip(selected_targets, fresh_plans):
        if plan["alreadySatisfied"]:
            continue
        try:
            client.server_dry_run(plan)
        except Exception as exc:
            print("MATCH_SQUAD_OVERLAY_PREFLIGHT_REJECTED kind=" + target.kind +
                  " name=" + target.name + " stage=server-dry-run writes=0",
                  file=sys.stderr)
            raise OverlayError("server dry-run rejected target: " + target.name) from exc
        print("MATCH_SQUAD_OVERLAY_PREFLIGHT_OK kind=" + target.kind +
              " name=" + target.name)

    # The request itself repeats UID/RV tests to close the race on each object.
    completed = 0
    applied = 0
    for target, plan in zip(selected_targets, fresh_plans):
        try:
            if plan["alreadySatisfied"]:
                current = client.get(target)
                if (current.get("metadata", {}).get("uid") != plan["uid"] or
                        current.get("metadata", {}).get("resourceVersion") != plan["resourceVersion"] or
                        object_hash(current) != plan["baseObjectSha256"] or
                        observed_render_hash(target, current) != plan["desiredObservedSha256"]):
                    raise OverlayError("already-satisfied target changed before skip")
            else:
                client.apply(plan)
                applied += 1
                if target.kind == "Deployment":
                    client.wait_deployment(plan)
        except Exception as exc:
            persisted = applied + int(bool(getattr(exc, "persisted", False)))
            print("MATCH_SQUAD_OVERLAY_STOPPED kind=" + target.kind + " name=" +
                  target.name + " completed_prefix=" + str(completed) +
                  " applied_prefix=" + str(persisted) +
                  " result=uncertain", file=sys.stderr)
            raise OverlayError("overlay stopped at first failed target") from exc
        completed += 1
        print("MATCH_SQUAD_OVERLAY_OBJECT_READY kind=" + target.kind + " name=" + target.name)
    if recovery_only:
        print("MATCH_SQUAD_OVERLAY_RECOVERY_COMPLETE count=1")
        return
    for service, ports in (
            ("voice-matchmaking", {"principal-jwks": 8443}),
            ("voice-chat", {"match-principal": 9092}),
            ("voice-voice", {"match-provider": 9094, "match-member": 9095})):
        try:
            client.verify_service_endpoints(service, ports)
        except Exception as exc:
            print("MATCH_SQUAD_OVERLAY_ENDPOINTS_NOT_READY service=" + service +
                  " completed_prefix=7 applied_prefix=" + str(applied) +
                  " result=uncertain", file=sys.stderr)
            raise OverlayError("overlay services are patched but endpoints are not ready") from exc
    print("MATCH_SQUAD_OVERLAY_APPLY_COMPLETE count=7")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--plan-file", required=True)
    parser.add_argument("--plan-sha256")
    parser.add_argument("--apply", action="store_true")
    parser.add_argument("--recover-matchmaking-fsgroup", action="store_true",
                        help="plan/apply only the fixed source-pinned fsGroup recovery")
    args = parser.parse_args(argv)
    try:
        execute(args.root, args.source_sha, Path(args.plan_file), args.plan_sha256,
                apply=args.apply, recovery_only=args.recover_matchmaking_fsgroup)
    except (OSError, ValueError, KeyError, TypeError, OverlayError):
        print("MatchSquad overlay stopped; inspect the bound plan and exact target state",
              file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
