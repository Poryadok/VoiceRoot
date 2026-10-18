import copy
import contextlib
import hashlib
import io
import json
import re
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).parent))
import match_squad_overlay as overlay


def captured_service(name, uid, rv, ports):
    return {
        "apiVersion": "v1", "kind": "Service",
        "metadata": {"name": name, "namespace": "voice-dev", "uid": uid,
                     "resourceVersion": rv,
                     "labels": {"voice.environment": "dev",
                                "app.kubernetes.io/managed-by": "voice-dev"}},
        "spec": {"ports": copy.deepcopy(ports), "selector": {"app": name}},
    }


def captured_deployment(name, container_name, uid, rv):
    return {
        "apiVersion": "apps/v1", "kind": "Deployment",
        "metadata": {"name": name, "namespace": "voice-dev", "uid": uid,
                     "resourceVersion": rv,
                     "labels": {"voice.environment": "dev",
                                "app.kubernetes.io/managed-by": "voice-dev"}},
        "spec": {"replicas": 1, "selector": {"matchLabels": {"app": name}},
                 "template": {"metadata": {"labels": {"app": name}},
                              "spec": {"containers": [{"name": container_name,
                                                        "image": "ghcr.io/poryadok/voiceroot/"
                                                                + name.removeprefix("voice-")
                                                                + "@sha256:" + "a" * 64,
                                                        "env": [{"name": "EXISTING_CONFIG",
                                                                 "valueFrom": {"configMapKeyRef": {
                                                                     "name": "voice-app-config",
                                                                     "key": "REDIS_ADDR"}}}],
                                                        "ports": [], "volumeMounts": []}],
                                        "volumes": []}}},
    }


def sanitized_live_api_fixture():
    """Relevant projection of the captured DEV API objects; env values redacted."""
    service_rows = {
        "voice-matchmaking": ("4ef51303-f3ef-4ca1-8dd2-9ff280b7ec16", "3353864",
                              [("http", 8080, 8080), ("grpc", 9090, 9090)]),
        "voice-chat": ("aaf56c0d-c1af-4221-adde-fc6c9664e30c", "3353832",
                       [("http", 8080, 8080), ("grpc", 9090, 9090)]),
        "voice-voice": ("4f99e5d7-15ed-4ce9-bbc4-cd999fa91475", "3353928",
                        [("http", 8080, 8080), ("grpc", 9090, 9090),
                         ("principal-jwks", 8443, 8443), ("user-principal", 9093, 9093)]),
    }
    objects = {}
    for name, (uid, rv, ports) in service_rows.items():
        objects[("Service", name)] = {
            "apiVersion": "v1", "kind": "Service",
            "metadata": {"name": name, "namespace": "voice-dev", "uid": uid,
                         "resourceVersion": rv,
                         # The live Services have managed-by, but no environment label.
                         "labels": {"app.kubernetes.io/managed-by": "voice-dev"}},
            "spec": {"clusterIP": {"voice-matchmaking": "10.43.104.157",
                                    "voice-chat": "10.43.27.38",
                                    "voice-voice": "10.43.22.93"}[name],
                     "ports": [{"name": port_name, "port": port, "protocol": "TCP",
                                "targetPort": target} for port_name, port, target in ports],
                     "selector": {"app": name}, "type": "ClusterIP"},
        }
    deployment_rows = {
        "voice-matchmaking": ("matchmaking", "f16ecb5d-3ad0-48ad-903b-91bff733d7b9",
                              "3355912", "7f10b25a34ddc6f301219c63a945e1aa8f89b3b3337b568f35a6d42790394a3e"),
        "voice-chat": ("chat", "eeb8f50d-409a-44e3-93bb-e6ed042db811",
                       "3355858", "d3133792b42a82e69f67dee4b37f5007974be80fb154719ebffb5752b1032d94"),
        "voice-voice": ("voice", "ef11fadf-9857-42e1-a324-31c9a4978138",
                        "3378060", "0ee1dac672a36f9c194cb3bf06e1090d49508830def20fe0d275d7158052956c"),
        "voice-gateway": ("gateway", "56a7f8f9-de86-4bde-9052-8ffdfe9fcd7f",
                          "3355815", "fde78ce8042183720f96fc769e79f3f6d67b3328906c23aecc650effd5db2699"),
    }
    for name, (container_name, uid, rv, image_digest) in deployment_rows.items():
        obj = captured_deployment(name, container_name, uid, rv)
        generation = 5 if name == "voice-voice" else 1
        obj["metadata"]["generation"] = generation
        obj["status"] = {"observedGeneration": generation, "replicas": 1,
                         "updatedReplicas": 1, "readyReplicas": 1,
                         "availableReplicas": 1}
        obj["spec"]["template"]["spec"]["containers"][0]["image"] = (
            "ghcr.io/poryadok/voiceroot/" + name.removeprefix("voice-") +
            "@sha256:" + image_digest)
        obj["metadata"]["labels"] = {"app": name, "app.kubernetes.io/managed-by": "voice-dev",
                                      "voice.environment": "dev"}
        obj["spec"]["template"]["metadata"]["labels"] = dict(obj["metadata"]["labels"])
        obj["spec"]["template"]["spec"]["automountServiceAccountToken"] = False
        obj["spec"]["template"]["spec"]["securityContext"] = (
            {"fsGroup": 65532} if name in ("voice-voice", "voice-gateway") else {})
        if name == "voice-voice":
            obj["spec"]["template"]["spec"]["containers"][0]["env"].append(
                {"name": "VOICE_USER_PRINCIPAL_CLIENT_CA_FILE", "value": "<redacted>"})
        if name in ("voice-matchmaking", "voice-chat", "voice-voice"):
            obj["spec"]["template"]["spec"]["containers"].append(
                {"name": "nats-leaf", "image": "docker.io/library/nats@sha256:" + "c" * 64,
                 "args": ["-c", "/etc/nats/leaf.conf"],
                 "env": [{"name": "NATS_CREDS", "value": "<redacted>"}],
                 "volumeMounts": [{"mountPath": "/etc/nats/leaf.conf", "name": "nats-leaf-config",
                                   "readOnly": True, "subPath": "leaf.conf"}]})
        objects[("Deployment", name)] = obj
    return objects


def matchmaking_secret_mount_snapshot():
    """Secret-free read-only projection of the current DEV Matchmaking pod template."""
    deployment = captured_deployment(
        "voice-matchmaking", "matchmaking",
        overlay.EXPECTED_OBJECT_UIDS[("Deployment", "voice-matchmaking")], "3392422")
    pod_spec = deployment["spec"]["template"]["spec"]
    pod_spec["securityContext"] = {}
    pod_spec["volumes"] = [
        {"name": "nats-service-creds", "secret": {
            "secretName": "voice-nats-service-credentials", "defaultMode": 256,
            "items": [{"key": "matchmaking.creds", "path": "matchmaking.creds"}]}},
        {"name": "nats-hub-tls", "secret": {
            "secretName": "voice-nats-hub-tls", "defaultMode": 420,
            "items": [{"key": "ca.crt", "path": "ca.crt"}]}},
        {"name": "match-squad-client", "secret": {
            "secretName": "voice-matchmaking-client-tls", "defaultMode": 288,
            "items": [{"key": "tls.crt", "path": "tls.crt"},
                      {"key": "tls.key", "path": "tls.key"}]}},
        {"name": "matchmaking-principal-signing", "secret": {
            "secretName": "voice-matchmaking-principal-signing", "defaultMode": 288,
            "items": [{"key": "current.pem", "path": "current.pem"},
                      {"key": "next.pem", "path": "next.pem"}]}},
        {"name": "matchmaking-principal-jwks-tls", "secret": {
            "secretName": "voice-matchmaking-principal-jwks-tls", "defaultMode": 288,
            "items": [{"key": "tls.crt", "path": "tls.crt"},
                      {"key": "tls.key", "path": "tls.key"}]}},
    ]
    pod_spec["containers"][0]["volumeMounts"] = [
        {"name": "match-squad-client", "mountPath": "/etc/voice/match-squad/client",
         "readOnly": True},
        {"name": "matchmaking-principal-signing", "mountPath": "/etc/voice/principal/keys",
         "readOnly": True},
        {"name": "matchmaking-principal-jwks-tls", "mountPath": "/etc/voice/principal-jwks/tls",
         "readOnly": True},
    ]
    pod_spec["containers"].append({
        "name": "nats-leaf", "image": "docker.io/library/nats@sha256:" + "c" * 64,
        "volumeMounts": [
            {"name": "nats-service-creds", "mountPath": "/var/run/nats/creds/matchmaking.creds",
             "readOnly": True},
            {"name": "nats-hub-tls", "mountPath": "/etc/nats/tls/ca.crt",
             "readOnly": True},
        ],
    })
    return deployment


class MatchSquadOverlayApplicatorTests(unittest.TestCase):
    def test_ci_filter_and_required_job_reach_the_focused_contract_suite(self):
        root = Path(__file__).resolve().parents[2]
        filters = (root / ".github/ci/path-filters.yml").read_text(encoding="utf-8")
        workflow = (root / ".github/workflows/ci.yml").read_text(encoding="utf-8")
        gate = (root / ".github/ci/verify-required-jobs.sh").read_text(encoding="utf-8")
        self.assertIn("match_squad_overlay:", filters)
        for path in (
                "scripts/dev/match_squad_overlay.py",
                "scripts/dev/match_squad_overlay_test.py",
                "scripts/dev/match_squad_runtime_overlay.py",
                "scripts/dev/match_squad_runtime_overlay_test.py",
                "deploy/dev/match-squad-trust-overlay/**"):
            self.assertIn("- " + path, filters)
        self.assertIn("match_squad_overlay: ${{ steps.filter.outputs.match_squad_overlay }}", workflow)
        self.assertIn("needs.changes.outputs.match_squad_overlay == 'true'", workflow)
        self.assertIn("- match-squad-overlay", workflow)
        self.assertIn("JOB_MATCH_SQUAD_OVERLAY: ${{ needs.match-squad-overlay.result }}", workflow)
        self.assertIn('check_if "${RUN_MATCH_SQUAD_OVERLAY:-false}" match-squad-overlay', gate)

    def test_chat_voice_service_and_container_port_names_match_kubernetes_limit(self):
        root = Path(__file__).resolve().parents[2] / "deploy/dev/match-squad-trust-overlay"
        expected = {
            "voice-chat": [("match-principal", 9092)],
            "voice-voice": [("match-provider", 9094), ("match-member", 9095)],
        }
        for service, ports in expected.items():
            with self.subTest(service=service):
                service_text = (root / (service + ".service.patch.yaml")).read_text(encoding="utf-8")
                runtime_text = (root / (service + ".runtime.patch.yaml")).read_text(encoding="utf-8")
                # Include targetPort in the comparison to catch mismatched names.
                service_matches = re.findall(
                    r"- name: ([a-z0-9-]+)\s+port: (\d+)\s+targetPort: ([a-z0-9-]+)",
                    service_text)
                self.assertEqual([(name, int(port), target) for name, port, target in service_matches],
                                 [(name, port, name) for name, port in ports])
                runtime_matches = [(name, int(number)) for name, number in re.findall(
                    r"- name: ([a-z0-9-]+)\s+containerPort: (\d+)", runtime_text)]
                self.assertEqual(runtime_matches, ports)
                for name, _ in ports:
                    self.assertLessEqual(len(name), 15)

    def test_actual_sanitized_api_fixture_identities_and_unrelated_fields(self):
        captured = sanitized_live_api_fixture()
        self.assertEqual(len(captured), 7)
        for target in overlay.TARGETS:
            with self.subTest(target=target.name, kind=target.kind):
                obj = captured[(target.kind, target.name)]
                overlay.validate_target(target, obj)
                self.assertNotIn("<redacted>", json.dumps(obj["metadata"]))
                if target.kind == "Service":
                    self.assertIn("clusterIP", obj["spec"])
                elif target.name in ("voice-matchmaking", "voice-chat"):
                    self.assertEqual(obj["spec"]["template"]["spec"]["containers"][1]["name"],
                                     "nats-leaf")

    def test_actual_service_fixture_cas_addition_preserves_existing_ports(self):
        base = sanitized_live_api_fixture()[("Service", "voice-voice")]
        desired = copy.deepcopy(base)
        desired["spec"]["ports"].extend([
            {"name": "match-provider", "port": 9094, "protocol": "TCP",
             "targetPort": "match-provider"},
            {"name": "match-member", "port": 9095, "protocol": "TCP",
             "targetPort": "match-member"},
        ])
        operations = overlay.object_patch(base, desired)
        self.assertEqual(operations[0], {"op": "test", "path": "/metadata/uid",
                                         "value": "4f99e5d7-15ed-4ce9-bbc4-cd999fa91475"})
        self.assertEqual([op["value"]["port"] for op in operations[2:]], [9094, 9095])

    def test_service_plan_hash_matches_json_patch_append_order(self):
        target = overlay.TARGETS[0]
        base = sanitized_live_api_fixture()[(target.kind, target.name)]
        desired = copy.deepcopy(base)
        desired["spec"]["ports"].insert(0, {
            "name": "principal-jwks", "port": 8443, "protocol": "TCP",
            "targetPort": "principal-jwks"})
        client = mock.Mock()
        client.render.return_value = desired
        patch_path = Path(__file__).resolve().parents[2] / "deploy/dev/match-squad-trust-overlay" / target.patch_file
        plan = overlay.build_object_plan(client, target, base, patch_path)
        expected = copy.deepcopy(base)
        expected["spec"]["ports"].append(desired["spec"]["ports"][0])
        self.assertEqual(plan["desiredObservedSha256"], overlay.observed_render_hash(target, expected))

    def test_unready_matchmaking_can_plan_only_the_reviewed_optional_listener_removal(self):
        target = overlay.TARGETS[3]
        patch_path = (Path(__file__).resolve().parents[2] /
                      "deploy/dev/match-squad-trust-overlay" / target.patch_file)
        base = sanitized_live_api_fixture()[(target.kind, target.name)]
        base = copy.deepcopy(base)
        base["metadata"]["generation"] = 3
        base["status"].update({"observedGeneration": 3, "updatedReplicas": 1,
                               "readyReplicas": 0, "availableReplicas": 0})
        container = next(item for item in base["spec"]["template"]["spec"]["containers"]
                         if item["name"] == overlay.CONTAINERS[target.name])
        container["image"] = overlay.QUALIFIED_IMAGES[target.name]
        container["env"].append({"name": "S2S_JWKS_URLS_JSON", "value": "<redacted>"})
        desired = copy.deepcopy(base)
        desired_container = desired["spec"]["template"]["spec"]["containers"][0]
        desired_container["env"] = [item for item in desired_container["env"]
                                     if item["name"] != "S2S_JWKS_URLS_JSON"]
        client = mock.Mock()
        client.render.return_value = desired

        plan = overlay.build_object_plan(client, target, base, patch_path)

        self.assertFalse(plan["alreadySatisfied"])
        self.assertEqual(plan["operations"][2:], [{
            "op": "remove",
            "path": "/spec/template/spec/containers/0/env/1",
        }])
        self.assertEqual(
            plan["desiredObservedSha256"],
            overlay.observed_render_hash(target, desired),
        )

        base_without_opt_in = copy.deepcopy(base)
        base_without_opt_in["spec"]["template"]["spec"]["containers"][0]["env"] = [
            item for item in base_without_opt_in["spec"]["template"]["spec"]["containers"][0]["env"]
            if item["name"] != "S2S_JWKS_URLS_JSON"]
        with self.assertRaisesRegex(overlay.OverlayError, "not ordinarily ready"):
            overlay.build_object_plan(client, target, base_without_opt_in, patch_path)

        chat = overlay.TARGETS[4]
        unready_chat = sanitized_live_api_fixture()[(chat.kind, chat.name)]
        unready_chat = copy.deepcopy(unready_chat)
        unready_chat["spec"]["template"]["spec"]["containers"][0]["image"] = (
            overlay.QUALIFIED_IMAGES[chat.name])
        unready_chat["status"].update({"readyReplicas": 0, "availableReplicas": 0})
        unready_chat["spec"]["template"]["spec"]["containers"][0]["env"].append(
            {"name": "S2S_JWKS_URLS_JSON", "value": "<redacted>"})
        with self.assertRaisesRegex(overlay.OverlayError, "not ordinarily ready"):
            overlay.build_object_plan(client, chat, unready_chat, patch_path)

        stale = copy.deepcopy(base)
        stale["metadata"]["generation"] = 4
        with self.assertRaisesRegex(overlay.OverlayError, "not ordinarily ready"):
            overlay.build_object_plan(client, target, stale, patch_path)

    def test_unready_matchmaking_can_plan_exact_fs_group_recovery_when_s2s_is_absent(self):
        target = overlay.RECOVERY_TARGET
        patch_path = (Path(__file__).resolve().parents[2] /
                      "deploy/dev/match-squad-trust-overlay" / target.patch_file)
        base = copy.deepcopy(sanitized_live_api_fixture()[(target.kind, target.name)])
        base["metadata"]["generation"] = 6
        base["metadata"]["resourceVersion"] = "3392422"
        base["status"].update({"observedGeneration": 6, "updatedReplicas": 1,
                               "readyReplicas": 0, "availableReplicas": 0})
        pod_spec = base["spec"]["template"]["spec"]
        pod_spec["securityContext"] = {}
        container = next(item for item in pod_spec["containers"]
                         if item["name"] == overlay.CONTAINERS[target.name])
        container["image"] = overlay.QUALIFIED_IMAGES[target.name]
        container["env"] = [item for item in container["env"]
                             if item["name"] != "S2S_JWKS_URLS_JSON"]
        client = overlay.KubectlClient()
        template_fingerprint = overlay.sha256_bytes(
            overlay.canonical_bytes(base["spec"]["template"]))

        with mock.patch.object(overlay, "MATCHMAKING_FS_GROUP_RECOVERY_TEMPLATE_SHA256",
                               template_fingerprint):
            plan = overlay.build_object_plan(client, target, base, patch_path)

        self.assertFalse(plan["alreadySatisfied"])
        self.assertEqual(plan["operations"][2:], [{
            "op": "add", "path": "/spec/template/spec/securityContext/fsGroup",
            "value": 65532,
        }])
        self.assertEqual(plan["patchFile"], "matchmaking-fsgroup-recovery.patch.yaml")
        self.assertEqual(plan["desiredObservedSha256"],
                         overlay.observed_render_hash(
                             target, overlay._apply_overlay_operations(
                                 base, plan["operations"][2:], target)))

        full_target = overlay.TARGETS[3]
        full_path = patch_path.parent / full_target.patch_file
        with mock.patch.object(overlay, "MATCHMAKING_FS_GROUP_RECOVERY_TEMPLATE_SHA256",
                               template_fingerprint):
            with self.assertRaisesRegex(overlay.OverlayError,
                                        "not ordinarily ready|recovery projection"):
                overlay.build_object_plan(client, full_target, base, full_path)

    def test_unready_matchmaking_fs_group_recovery_rejects_other_projection_changes(self):
        target = overlay.RECOVERY_TARGET
        patch_path = (Path(__file__).resolve().parents[2] /
                      "deploy/dev/match-squad-trust-overlay" / target.patch_file)
        base = copy.deepcopy(sanitized_live_api_fixture()[(target.kind, target.name)])
        base["metadata"]["generation"] = 6
        base["status"].update({"observedGeneration": 6, "updatedReplicas": 1,
                               "readyReplicas": 0, "availableReplicas": 0})
        base["spec"]["template"]["spec"]["securityContext"] = {}
        pod_spec = base["spec"]["template"]["spec"]
        pod_spec.setdefault("volumes", [])
        app = next(item for item in pod_spec["containers"]
                   if item["name"] == overlay.CONTAINERS[target.name])
        app["image"] = overlay.QUALIFIED_IMAGES[target.name]
        env = base["spec"]["template"]["spec"]["containers"][0]["env"]
        env[:] = [item for item in env if item["name"] != "S2S_JWKS_URLS_JSON"]
        template_fingerprint = overlay.sha256_bytes(
            overlay.canonical_bytes(base["spec"]["template"]))
        target_cases = []
        for label, mutate in (
                ("wrong image", lambda obj: obj["spec"]["template"]["spec"]["containers"][0].update(
                    {"image": "ghcr.io/poryadok/voiceroot/matchmaking@sha256:" + "f" * 64})),
                ("extra env", lambda obj: obj["spec"]["template"]["spec"]["containers"][0]["env"].append(
                    {"name": "UNEXPECTED", "value": "x"})),
                ("volume drift", lambda obj: obj["spec"]["template"]["spec"]["volumes"].append(
                    {"name": "unexpected", "emptyDir": {}})),
                ("container drift", lambda obj: obj["spec"]["template"]["spec"]["containers"].append(
                    {"name": "unexpected-sidecar", "image": "example.invalid/sidecar:v1"})),
                ("security drift", lambda obj: obj["spec"]["template"]["spec"]["securityContext"].update(
                    {"runAsUser": 0})),
        ):
            changed = copy.deepcopy(base)
            mutate(changed)
            target_cases.append((label, changed))

        for label, changed in target_cases:
            with self.subTest(label=label):
                client = overlay.KubectlClient()
                changed_fingerprint = overlay.sha256_bytes(
                    overlay.canonical_bytes(changed["spec"]["template"]))
                with mock.patch.object(
                        overlay, "MATCHMAKING_FS_GROUP_RECOVERY_TEMPLATE_SHA256",
                        template_fingerprint):
                    with self.assertRaisesRegex(overlay.OverlayError,
                                                "not ordinarily ready|recovery projection|pinned projection|S2S entry"):
                        overlay.build_object_plan(client, target, changed, patch_path)
                self.assertNotEqual(changed_fingerprint, template_fingerprint)

        with self.subTest(label="S2S opt-in still present"):
            s2s_base = copy.deepcopy(base)
            app = next(item for item in s2s_base["spec"]["template"]["spec"]["containers"]
                       if item["name"] == overlay.CONTAINERS[target.name])
            app.setdefault("env", []).append({"name": "S2S_JWKS_URLS_JSON", "value": "{}"})
            client = overlay.KubectlClient()
            with mock.patch.object(
                    overlay, "MATCHMAKING_FS_GROUP_RECOVERY_TEMPLATE_SHA256",
                    template_fingerprint):
                with self.assertRaisesRegex(overlay.OverlayError,
                                            "not ordinarily ready|recovery projection|pinned projection|S2S entry"):
                    overlay.build_object_plan(client, target, s2s_base, patch_path)

    def test_fs_group_recovery_rejects_ready_deployment_even_when_projection_is_pinned(self):
        target = overlay.RECOVERY_TARGET
        patch_path = (Path(__file__).resolve().parents[2] /
                      "deploy/dev/match-squad-trust-overlay" / target.patch_file)
        base = copy.deepcopy(sanitized_live_api_fixture()[(target.kind, target.name)])
        base["metadata"]["generation"] = 6
        base["status"].update({"observedGeneration": 6, "updatedReplicas": 1,
                               "readyReplicas": 1, "availableReplicas": 1})
        pod_spec = base["spec"]["template"]["spec"]
        pod_spec["securityContext"] = {}
        app = next(item for item in pod_spec["containers"]
                   if item["name"] == overlay.CONTAINERS[target.name])
        app["image"] = overlay.QUALIFIED_IMAGES[target.name]
        app["env"] = [item for item in app["env"]
                      if item["name"] != "S2S_JWKS_URLS_JSON"]
        fingerprint = overlay.sha256_bytes(overlay.canonical_bytes(base["spec"]["template"]))
        with mock.patch.object(overlay, "MATCHMAKING_FS_GROUP_RECOVERY_TEMPLATE_SHA256",
                               fingerprint):
            with self.assertRaisesRegex(overlay.OverlayError,
                                        "only for an unready Matchmaking Deployment"):
                overlay.build_object_plan(overlay.KubectlClient(), target, base, patch_path)

    def test_fs_group_recovery_rejects_ready_template_drift(self):
        target = overlay.RECOVERY_TARGET
        patch_path = (Path(__file__).resolve().parents[2] /
                      "deploy/dev/match-squad-trust-overlay" / target.patch_file)
        base = copy.deepcopy(sanitized_live_api_fixture()[(target.kind, target.name)])
        base["metadata"]["generation"] = 6
        base["status"].update({"observedGeneration": 6, "updatedReplicas": 1,
                               "readyReplicas": 1, "availableReplicas": 1})
        pod_spec = base["spec"]["template"]["spec"]
        pod_spec["securityContext"] = {}
        app = next(item for item in pod_spec["containers"]
                   if item["name"] == overlay.CONTAINERS[target.name])
        app["image"] = overlay.QUALIFIED_IMAGES[target.name]
        app["env"] = [item for item in app["env"]
                      if item["name"] != "S2S_JWKS_URLS_JSON"]
        base["spec"]["template"]["metadata"].setdefault("labels", {})["unexpected"] = "drift"
        with mock.patch.object(overlay, "MATCHMAKING_FS_GROUP_RECOVERY_TEMPLATE_SHA256",
                               "0" * 64):
            with self.assertRaisesRegex(overlay.OverlayError,
                                        "pinned projection"):
                overlay.build_object_plan(overlay.KubectlClient(), target, base, patch_path)

    def test_recovery_cli_plan_is_single_purpose_and_cannot_apply_as_normal_overlay(self):
        target = overlay.RECOVERY_TARGET
        base = copy.deepcopy(sanitized_live_api_fixture()[(target.kind, target.name)])
        base["metadata"]["generation"] = 6
        base["status"].update({"observedGeneration": 6, "updatedReplicas": 1,
                               "readyReplicas": 0, "availableReplicas": 0})
        pod_spec = base["spec"]["template"]["spec"]
        pod_spec["securityContext"] = {}
        app = next(item for item in pod_spec["containers"]
                   if item["name"] == overlay.CONTAINERS[target.name])
        app["image"] = overlay.QUALIFIED_IMAGES[target.name]
        app["env"] = [item for item in app["env"]
                      if item["name"] != "S2S_JWKS_URLS_JSON"]
        template_fingerprint = overlay.sha256_bytes(
            overlay.canonical_bytes(base["spec"]["template"]))

        class RecoveryClient:
            def __init__(self):
                self.renderer = overlay.KubectlClient()
                self.applied = []

            def get_namespace(self):
                return {"metadata": {"name": overlay.NS,
                                     "uid": overlay.EXPECTED_NAMESPACE_UID,
                                     "labels": overlay.OWNERSHIP}}

            def get(self, requested):
                if requested != target:
                    raise AssertionError("recovery mode requested an unexpected target")
                return copy.deepcopy(base)

            def render(self, requested, current, patch_path):
                return self.renderer.render(requested, current, patch_path)

        source_sha = "a" * 40
        root = Path(__file__).resolve().parents[2]
        client = RecoveryClient()
        with tempfile.TemporaryDirectory(prefix="overlay-fsgroup-recovery-") as tmp:
            plan_path = Path(tmp) / "recovery-plan.json"
            with mock.patch.object(overlay, "_source_identity", return_value=source_sha), \
                    mock.patch.object(overlay, "MATCHMAKING_FS_GROUP_RECOVERY_TEMPLATE_SHA256",
                                      template_fingerprint):
                overlay.execute(root, source_sha, plan_path, client=client,
                                recovery_only=True)
                plan = json.loads(plan_path.read_text(encoding="utf-8"))
                self.assertEqual(plan["recoveryMode"], overlay.RECOVERY_MODE)
                self.assertEqual(len(plan["targets"]), 1)
                self.assertEqual(plan["targets"][0]["patchFile"], target.patch_file)
                digest = hashlib.sha256(plan_path.read_bytes()).hexdigest()
                with self.assertRaisesRegex(overlay.OverlayError,
                                            "plan source or namespace binding changed"):
                    overlay.execute(root, source_sha, plan_path, digest,
                                    apply=True, client=client, recovery_only=False)
        self.assertEqual(client.applied, [])

    def test_existing_service_port_order_cannot_be_rewritten(self):
        base = captured_service("voice-chat", "service-uid", "17", [
            {"name": "http", "port": 8080, "targetPort": 8080},
            {"name": "grpc", "port": 9090, "targetPort": 9090}])
        desired = copy.deepcopy(base)
        desired["spec"]["ports"] = [desired["spec"]["ports"][1],
                                     desired["spec"]["ports"][0],
            {"name": "match-principal", "port": 9092,
             "targetPort": "match-principal"}]
        with self.assertRaisesRegex(overlay.OverlayError, "existing list order"):
            overlay.object_patch(base, desired)

    def test_deployment_patch_predictions_append_each_list_in_operation_order(self):
        base = sanitized_live_api_fixture()[("Deployment", "voice-voice")]
        desired = copy.deepcopy(base)
        pod_spec = desired["spec"]["template"]["spec"]
        pod_spec["containers"][0]["env"].append({"name": "NEW_ENV", "value": "x"})
        pod_spec["containers"][0]["ports"].extend([
            {"name": "new-a", "containerPort": 9094},
            {"name": "new-b", "containerPort": 9095}])
        pod_spec["containers"][0]["volumeMounts"].append(
            {"name": "new-volume", "mountPath": "/new", "readOnly": True})
        pod_spec["volumes"].append({"name": "new-volume", "configMap": {"name": "new-config"}})
        operations = overlay.object_patch(base, desired)
        self.assertEqual([op["path"] for op in operations[2:]], [
            "/spec/template/spec/containers/0/env/-",
            "/spec/template/spec/containers/0/ports/-",
            "/spec/template/spec/containers/0/ports/-",
            "/spec/template/spec/containers/0/volumeMounts/-",
            "/spec/template/spec/volumes/-"])
        self.assertEqual([op["value"]["name"] for op in operations[2:]],
                         ["NEW_ENV", "new-a", "new-b", "new-volume", "new-volume"])

    def test_matchmaking_fs_group_patch_preserves_secret_mode_and_mount_isolation(self):
        base = matchmaking_secret_mount_snapshot()
        pod_spec = base["spec"]["template"]["spec"]
        secret_volumes = {volume["secret"]["secretName"]: volume["secret"]
                          for volume in pod_spec["volumes"] if "secret" in volume}
        self.assertEqual(secret_volumes["voice-matchmaking-principal-signing"]["defaultMode"], 288)
        self.assertEqual(secret_volumes["voice-matchmaking-principal-signing"]["items"], [
            {"key": "current.pem", "path": "current.pem"},
            {"key": "next.pem", "path": "next.pem"},
        ])
        self.assertEqual(secret_volumes["voice-nats-service-credentials"]["defaultMode"], 256)
        self.assertEqual(secret_volumes["voice-nats-service-credentials"]["items"], [
            {"key": "matchmaking.creds", "path": "matchmaking.creds"},
        ])
        by_volume_name = {volume["name"]: volume for volume in pod_spec["volumes"]}
        mounted_secrets = {
            container["name"]: {
                by_volume_name[mount["name"]]["secret"]["secretName"]
                for mount in container.get("volumeMounts", [])
                if mount["name"] in by_volume_name and "secret" in by_volume_name[mount["name"]]
            }
            for container in pod_spec["containers"]
        }
        self.assertEqual(mounted_secrets, {
            "matchmaking": {
                "voice-matchmaking-client-tls",
                "voice-matchmaking-principal-signing",
                "voice-matchmaking-principal-jwks-tls",
            },
            "nats-leaf": {"voice-nats-service-credentials", "voice-nats-hub-tls"},
        })

        desired = copy.deepcopy(base)
        desired["spec"]["template"]["spec"]["securityContext"]["fsGroup"] = 65532
        operations = overlay.object_patch(base, desired)
        self.assertEqual(operations[2:], [{
            "op": "add", "path": "/spec/template/spec/securityContext/fsGroup", "value": 65532,
        }])
        projected = overlay._apply_overlay_operations(
            base, operations[2:], overlay._Target("Deployment", "voice-matchmaking", ""))
        self.assertEqual(projected, desired)

        existing_setting = copy.deepcopy(base)
        existing_setting["spec"]["template"]["spec"]["securityContext"] = {
            "runAsNonRoot": True,
        }
        desired_existing_setting = copy.deepcopy(existing_setting)
        desired_existing_setting["spec"]["template"]["spec"]["securityContext"]["fsGroup"] = 65532
        preserving_operations = overlay.object_patch(existing_setting, desired_existing_setting)
        self.assertEqual(preserving_operations[2:], [{
            "op": "add", "path": "/spec/template/spec/securityContext/fsGroup", "value": 65532,
        }])
        self.assertEqual(overlay._apply_overlay_operations(
            existing_setting, preserving_operations[2:],
            overlay._Target("Deployment", "voice-matchmaking", "")), desired_existing_setting)
        self.assertEqual(
            [volume["secret"] for volume in projected["spec"]["template"]["spec"]["volumes"]],
            [volume["secret"] for volume in pod_spec["volumes"]],
        )
        target = overlay.TARGETS[3]
        plan = {"kind": target.kind, "name": target.name, "patchFile": target.patch_file,
                "uid": base["metadata"]["uid"],
                "resourceVersion": base["metadata"]["resourceVersion"],
                "operations": operations,
                "desiredObservedSha256": overlay.observed_render_hash(target, desired)}
        client = overlay.KubectlClient()
        with mock.patch.object(client, "call", return_value=desired) as call:
            client.server_dry_run(plan)
        args = call.call_args.args
        self.assertEqual(args[:3], ("patch", "deployment", "voice-matchmaking"))
        self.assertIn("--dry-run=server", args)
        patch_arg = next(arg for arg in args if arg.startswith("--patch="))
        self.assertEqual(json.loads(patch_arg[len("--patch="):]), operations)

        absent_parent = copy.deepcopy(base)
        absent_parent["spec"]["template"]["spec"].pop("securityContext")
        desired_without_parent = copy.deepcopy(absent_parent)
        desired_without_parent["spec"]["template"]["spec"]["securityContext"] = {"fsGroup": 65532}
        parent_operations = overlay.object_patch(absent_parent, desired_without_parent)
        self.assertEqual(parent_operations[2:], [{
            "op": "add", "path": "/spec/template/spec/securityContext",
            "value": {"fsGroup": 65532},
        }])

        already_satisfied = copy.deepcopy(desired)
        self.assertEqual(overlay.object_patch(already_satisfied, copy.deepcopy(already_satisfied)), [
            {"op": "test", "path": "/metadata/uid", "value": base["metadata"]["uid"]},
            {"op": "test", "path": "/metadata/resourceVersion",
             "value": base["metadata"]["resourceVersion"]},
        ])

    def test_matchmaking_fs_group_patch_rejects_other_security_changes(self):
        base = matchmaking_secret_mount_snapshot()
        cases = (
            ("other fsGroup", {"fsGroup": 1000}),
            ("extra pod security setting", {"fsGroup": 65532, "runAsNonRoot": True}),
            ("other deployment", {"fsGroup": 65532}),
            ("replace existing fsGroup", {"fsGroup": 65532}),
        )
        for label, security_context in cases:
            with self.subTest(case=label):
                if label == "other deployment":
                    target = captured_deployment(
                        "voice-chat", "chat",
                        overlay.EXPECTED_OBJECT_UIDS[("Deployment", "voice-chat")], "17")
                    target["spec"]["template"]["spec"]["securityContext"] = {}
                else:
                    target = copy.deepcopy(base)
                if label == "replace existing fsGroup":
                    target["spec"]["template"]["spec"]["securityContext"]["fsGroup"] = 1000
                desired = copy.deepcopy(target)
                desired["spec"]["template"]["spec"]["securityContext"] = security_context
                with self.assertRaises(overlay.OverlayError):
                    overlay.object_patch(target, desired)

        unrelated = copy.deepcopy(base)
        desired = copy.deepcopy(base)
        desired["spec"]["template"]["spec"]["securityContext"]["fsGroup"] = 65532
        desired["spec"]["template"]["spec"]["automountServiceAccountToken"] = False
        with self.assertRaises(overlay.OverlayError):
            overlay.object_patch(unrelated, desired)

    def test_missing_list_parents_use_single_parent_add_operation(self):
        cases = (
            ("Service", "voice-chat", "/spec/ports", "servicePorts", "ports"),
            ("Deployment", "voice-chat", "/spec/template/spec/containers/0/env",
             "env", "env"),
            ("Deployment", "voice-chat", "/spec/template/spec/containers/0/ports",
             "ports", "ports"),
            ("Deployment", "voice-chat", "/spec/template/spec/containers/0/volumeMounts",
             "volumeMounts", "volumeMounts"),
            ("Deployment", "voice-chat", "/spec/template/spec/volumes", "volumes", "volumes"),
        )
        for kind, name, path, identity_field, object_field in cases:
            with self.subTest(path=path):
                if kind == "Service":
                    base = captured_service(name, "service-uid", "17", [])
                    desired = copy.deepcopy(base)
                    desired["spec"].pop(object_field)
                    desired["spec"][object_field] = [
                        {"name": "new", "port": 9092, "targetPort": "new"}]
                    base["spec"].pop(object_field)
                else:
                    base = captured_deployment(name, "chat", "deployment-uid", "17")
                    desired = copy.deepcopy(base)
                    if object_field == "volumes":
                        base_spec = base["spec"]["template"]["spec"]
                        desired_spec = desired["spec"]["template"]["spec"]
                    else:
                        base_spec = base["spec"]["template"]["spec"]["containers"][0]
                        desired_spec = desired["spec"]["template"]["spec"]["containers"][0]
                    base_spec.pop(object_field)
                    desired_spec.pop(object_field)
                    item = ({"name": "new", "value": "x"} if identity_field == "env" else
                            {"name": "new", "containerPort": 9092} if identity_field == "ports" else
                            {"name": "new", "mountPath": "/new"} if identity_field == "volumeMounts" else
                            {"name": "new", "configMap": {"name": "new-config"}})
                    desired_spec[object_field] = [item]
                operations = overlay.object_patch(base, desired)
                additions = [op for op in operations if op["op"] == "add"]
                self.assertEqual(additions, [{"op": "add", "path": path,
                                              "value": desired_spec[object_field]
                                              if kind == "Deployment" else desired["spec"][object_field]}])
                self.assertEqual(overlay._apply_append_operations(base, additions), desired)

    def test_matchmaking_space_principal_opt_in_is_removed_without_other_env_changes(self):
        base = captured_deployment(
            "voice-matchmaking", "matchmaking",
            overlay.EXPECTED_OBJECT_UIDS[("Deployment", "voice-matchmaking")], "31")
        env = base["spec"]["template"]["spec"]["containers"][0]["env"]
        env.append({"name": "S2S_JWKS_URLS_JSON", "value": "<redacted>"})
        env.append({"name": "MATCHMAKING_REDIS_ADDR", "value": "<redacted>"})
        desired = copy.deepcopy(base)
        desired_env = desired["spec"]["template"]["spec"]["containers"][0]["env"]
        desired_env[:] = [item for item in desired_env
                          if item["name"] != "S2S_JWKS_URLS_JSON"]

        operations = overlay.object_patch(base, desired)

        self.assertEqual(operations[2:], [{
            "op": "remove",
            "path": "/spec/template/spec/containers/0/env/1",
        }])
        projected = overlay._apply_overlay_operations(
            base, operations[2:], overlay._Target("Deployment", "voice-matchmaking", ""))
        self.assertEqual(projected, desired)
        self.assertEqual(
            [item for item in projected["spec"]["template"]["spec"]["containers"][0]["env"]
             if item["name"] == "MATCHMAKING_REDIS_ADDR"],
            [{"name": "MATCHMAKING_REDIS_ADDR", "value": "<redacted>"}],
        )

    def test_matchmaking_principal_removal_is_exact_target_and_identity_only(self):
        for target_name, env_name in (
                ("voice-matchmaking", "OTHER_CONFIG"),
                ("voice-chat", "S2S_JWKS_URLS_JSON")):
            with self.subTest(target=target_name, env=env_name):
                container = overlay.CONTAINERS[target_name]
                base = captured_deployment(
                    target_name, container,
                    overlay.EXPECTED_OBJECT_UIDS[("Deployment", target_name)], "31")
                base_env = base["spec"]["template"]["spec"]["containers"][0]["env"]
                base_env.append({"name": env_name, "value": "<redacted>"})
                desired = copy.deepcopy(base)
                desired["spec"]["template"]["spec"]["containers"][0]["env"] = [
                    item for item in desired["spec"]["template"]["spec"]["containers"][0]["env"]
                    if item["name"] != env_name]
                with self.assertRaisesRegex(overlay.OverlayError, "remove a list item"):
                    overlay.object_patch(base, desired)

    def test_matchmaking_patch_preserves_only_gateway_s2s_publisher(self):
        root = Path(__file__).resolve().parents[2]
        patch = (root / "deploy/dev/match-squad-trust-overlay/"
                 "voice-matchmaking.runtime.patch.yaml").read_text(encoding="utf-8")
        self.assertIn(
            'name: S2S_JWKS_URLS_JSON\n              value: \'{"gateway":"https://api-dev.comrade.click/.well-known/voice-principal-jwks.json"}\'',
            patch,
        )
        self.assertNotIn("$patch: delete", "\n".join(
            path.read_text(encoding="utf-8") for path in (
                root / "deploy/dev/match-squad-trust-overlay/voice-chat.runtime.patch.yaml",
                root / "deploy/dev/match-squad-trust-overlay/voice-voice.runtime.patch.yaml",
                root / "deploy/dev/match-squad-trust-overlay/voice-gateway.patch.yaml")))

    def test_present_empty_list_parent_keeps_append_operation(self):
        base = captured_service("voice-chat", "service-uid", "17", [])
        desired = copy.deepcopy(base)
        desired["spec"]["ports"].append(
            {"name": "principal", "port": 9092, "targetPort": "principal"})
        operations = overlay.object_patch(base, desired)
        addition = next(op for op in operations if op["op"] == "add")
        self.assertEqual(addition["path"], "/spec/ports/-")

    def test_invalid_list_parent_shapes_fail_closed(self):
        base = captured_service("voice-chat", "service-uid", "17", [])
        base["spec"]["ports"] = {"unexpected": "mapping"}
        desired = copy.deepcopy(base)
        desired["spec"]["ports"] = [
            {"name": "principal", "port": 9092, "targetPort": "principal"}]
        with self.assertRaisesRegex(overlay.OverlayError, "list shape"):
            overlay.object_patch(base, desired)

        with self.assertRaisesRegex(overlay.OverlayError, "not absent and list-shaped"):
            overlay._apply_append_operations(
                {"spec": []}, [{"op": "add", "path": "/spec/ports",
                                "value": [{"name": "principal", "port": 9092}]}])

    def test_api_postcondition_failure_marks_same_uid_new_rv_as_persisted(self):
        target = overlay.TARGETS[0]
        base = captured_service("voice-matchmaking", overlay.EXPECTED_OBJECT_UIDS[
            ("Service", "voice-matchmaking")], "17", [
                {"name": "http", "port": 8080, "targetPort": 8080}])
        desired = copy.deepcopy(base)
        desired["spec"]["ports"].append({"name": "principal-jwks", "port": 8443,
                                           "targetPort": "principal-jwks"})
        plan = {"kind": "Service", "name": target.name, "patchFile": target.patch_file,
                "uid": base["metadata"]["uid"], "resourceVersion": "17",
                "operations": overlay.object_patch(base, desired),
                "desiredObservedSha256": "f" * 64}
        actual = copy.deepcopy(base)
        actual["metadata"]["resourceVersion"] = "18"
        actual["spec"]["ports"].append(desired["spec"]["ports"][1])
        client = overlay.KubectlClient()
        with mock.patch.object(client, "call", return_value=actual):
            with self.assertRaisesRegex(overlay.OverlayError, "did not match") as raised:
                client.apply(plan)
        self.assertTrue(raised.exception.persisted)

    def test_api_patch_request_is_single_guarded_json_patch(self):
        target = overlay.TARGETS[1]
        base = captured_service("voice-chat", "service-uid", "17", [])
        desired = copy.deepcopy(base)
        desired["spec"]["ports"].append({"name": "match-principal", "port": 9092,
                                           "targetPort": "match-principal"})
        operations = overlay.object_patch(base, desired)
        expected = copy.deepcopy(desired)
        expected["metadata"]["resourceVersion"] = "18"
        expected["spec"]["ports"][0]["protocol"] = "TCP"
        plan = {"kind": "Service", "name": "voice-chat", "patchFile": target.patch_file,
                "uid": "service-uid", "resourceVersion": "17",
                "operations": operations,
                "desiredObservedSha256": overlay.observed_render_hash(target, desired)}
        client = overlay.KubectlClient()
        with mock.patch.object(client, "call", return_value=expected) as call:
            client.apply(plan)
        args = call.call_args.args
        self.assertEqual(args[:3], ("patch", "service", "voice-chat"))
        self.assertIn("--type=json", args)
        patch_arg = next(arg for arg in args if arg.startswith("--patch="))
        self.assertEqual(json.loads(patch_arg[len("--patch="):]), operations)
        self.assertEqual([item["op"] for item in operations[:2]], ["test", "test"])
        self.assertNotIn("remove", [item["op"] for item in operations])

        expected["metadata"]["uid"] = "recreated-service"
        with mock.patch.object(client, "call", return_value=expected):
            with self.assertRaisesRegex(overlay.OverlayError, "did not match"):
                client.apply(plan)

    def test_server_dry_run_uses_same_guarded_patch_and_checks_projection(self):
        target = overlay.TARGETS[1]
        base = captured_service("voice-chat", "service-uid", "17", [])
        desired = copy.deepcopy(base)
        desired["spec"]["ports"].append({"name": "match-principal", "port": 9092,
                                           "targetPort": "match-principal"})
        operations = overlay.object_patch(base, desired)
        api_result = copy.deepcopy(desired)
        api_result["spec"]["ports"][0]["protocol"] = "TCP"
        plan = {"kind": "Service", "name": target.name, "patchFile": target.patch_file,
                "uid": base["metadata"]["uid"], "resourceVersion": "17",
                "operations": operations,
                "desiredObservedSha256": overlay.observed_render_hash(target, desired)}
        client = overlay.KubectlClient()
        with mock.patch.object(client, "call", return_value=api_result) as call:
            client.server_dry_run(plan)
        args = call.call_args.args
        self.assertEqual(args[:3], ("patch", "service", "voice-chat"))
        self.assertIn("--type=json", args)
        self.assertIn("--dry-run=server", args)
        patch_arg = next(arg for arg in args if arg.startswith("--patch="))
        self.assertEqual(json.loads(patch_arg[len("--patch="):]), operations)

        api_result["spec"]["ports"][0]["targetPort"] = 9092
        with mock.patch.object(client, "call", return_value=api_result):
            with self.assertRaisesRegex(overlay.OverlayError, "server dry-run result"):
                client.server_dry_run(plan)

    def test_actual_voice_deployment_fixture_preserves_media_trust_and_existing_sidecar(self):
        base = sanitized_live_api_fixture()[("Deployment", "voice-voice")]
        desired = copy.deepcopy(base)
        pod_spec = desired["spec"]["template"]["spec"]
        container = pod_spec["containers"][0]
        container["env"].append({"name": "VOICE_MATCH_SQUAD_PRINCIPAL_CLIENT_CA_FILE",
                                 "value": "/etc/voice/match-squad/ca/ca.crt"})
        container["env"].append({"name": "VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_CLIENT_CA_FILE",
                                 "value": "/etc/voice/media/ca/ca.crt"})
        container["ports"].append({"name": "match-provider", "containerPort": 9094})
        container["volumeMounts"].append({"name": "match-squad-ca",
                                            "mountPath": "/etc/voice/match-squad/ca",
                                            "readOnly": True})
        pod_spec["volumes"].append({"name": "match-squad-ca",
                                    "configMap": {"name": "voice-match-squad-ca",
                                                  "items": [{"key": "ca.crt",
                                                             "path": "ca.crt"}]}})
        operations = overlay.object_patch(base, desired)
        self.assertTrue(any(isinstance(op.get("value"), dict) and
                            op["value"].get("name") ==
                            "VOICE_MATCH_SQUAD_PRINCIPAL_CLIENT_CA_FILE"
                            for op in operations))
        original = base["spec"]["template"]["spec"]["containers"][0]
        planned = desired["spec"]["template"]["spec"]["containers"][0]
        self.assertEqual(
            next(env for env in planned["env"]
                 if env["name"] == "VOICE_USER_PRINCIPAL_CLIENT_CA_FILE"),
            next(env for env in original["env"]
                 if env["name"] == "VOICE_USER_PRINCIPAL_CLIENT_CA_FILE"),
        )
        self.assertEqual(pod_spec["containers"][1],
                         base["spec"]["template"]["spec"]["containers"][1])

    def test_fixed_target_order_and_atomic_cas_append_plan(self):
        self.assertEqual(
            [(item.kind, item.name) for item in overlay.TARGETS],
            [("Service", "voice-matchmaking"), ("Service", "voice-chat"),
             ("Service", "voice-voice"), ("Deployment", "voice-matchmaking"),
             ("Deployment", "voice-chat"), ("Deployment", "voice-voice"),
             ("Deployment", "voice-gateway")],
        )
        base = captured_service("voice-chat", "service-uid", "17", [])
        desired = copy.deepcopy(base)
        desired["spec"]["ports"] = [{"name": "match-principal", "port": 9092,
                                       "targetPort": "match-principal"}]
        operations = overlay.object_patch(base, desired)
        self.assertEqual(operations[:2], [
            {"op": "test", "path": "/metadata/uid", "value": "service-uid"},
            {"op": "test", "path": "/metadata/resourceVersion", "value": "17"},
        ])
        self.assertEqual(operations[2:], [{
            "op": "add", "path": "/spec/ports/-",
            "value": {"name": "match-principal", "port": 9092,
                      "targetPort": "match-principal"},
        }])

    def test_deployment_plan_appends_only_reviewed_list_items(self):
        base = captured_deployment("voice-voice", "voice", "deploy-uid", "21")
        desired = copy.deepcopy(base)
        container = desired["spec"]["template"]["spec"]["containers"][0]
        container["env"].append({"name": "VOICE_MATCH_SQUAD_PRINCIPAL_GRPC_LISTEN",
                                 "value": ":9094"})
        container["ports"].append({"name": "match-provider", "containerPort": 9094})
        container["volumeMounts"].append({"name": "match-squad-ca",
                                            "mountPath": "/etc/voice/match-squad/ca",
                                            "readOnly": True})
        desired["spec"]["template"]["spec"]["volumes"].append({
            "name": "match-squad-ca", "configMap": {"name": "voice-match-squad-ca",
                                                        "items": [{"key": "ca.crt",
                                                                   "path": "ca.crt"}]}})
        operations = overlay.object_patch(base, desired)
        self.assertEqual([op["path"] for op in operations[2:]], [
            "/spec/template/spec/containers/0/env/-",
            "/spec/template/spec/containers/0/ports/-",
            "/spec/template/spec/containers/0/volumeMounts/-",
            "/spec/template/spec/volumes/-",
        ])
        self.assertEqual(base["spec"]["template"]["spec"]["containers"][0]["env"],
                         [{"name": "EXISTING_CONFIG", "valueFrom": {
                             "configMapKeyRef": {"name": "voice-app-config",
                                                  "key": "REDIS_ADDR"}}}])

    def test_rejects_mutation_replacement_extra_path_and_duplicate_list_identity(self):
        base = captured_deployment("voice-chat", "chat", "uid", "2")
        desired = copy.deepcopy(base)
        desired["spec"]["replicas"] = 2
        with self.assertRaises(overlay.OverlayError):
            overlay.object_patch(base, desired)
        desired = copy.deepcopy(base)
        desired["spec"]["template"]["metadata"]["annotations"] = {"unexpected": "yes"}
        with self.assertRaises(overlay.OverlayError):
            overlay.object_patch(base, desired)
        desired = copy.deepcopy(base)
        desired["spec"]["template"]["spec"]["containers"][0]["env"].append(
            {"name": "EXISTING_CONFIG", "value": "overridden"})
        with self.assertRaises(overlay.OverlayError):
            overlay.object_patch(base, desired)

    def test_existing_list_items_are_immutable_even_when_identity_is_unchanged(self):
        cases = []
        service = captured_service("voice-chat", "uid", "2", [
            {"name": "grpc", "port": 9090, "targetPort": 9090, "protocol": "TCP"}])
        changed = copy.deepcopy(service)
        changed["spec"]["ports"][0]["targetPort"] = 9091
        cases.append((service, changed))
        changed = copy.deepcopy(service)
        changed["spec"]["ports"][0]["protocol"] = "UDP"
        cases.append((service, changed))

        deployment = captured_deployment("voice-chat", "chat", "uid", "2")
        container = deployment["spec"]["template"]["spec"]["containers"][0]
        container["env"] = [{"name": "EXISTING_CONFIG", "value": "original"}]
        changed = copy.deepcopy(deployment)
        changed["spec"]["template"]["spec"]["containers"][0]["env"][0]["value"] = "changed"
        cases.append((deployment, changed))
        changed = copy.deepcopy(deployment)
        changed["spec"]["template"]["spec"]["containers"][0]["env"][0] = {
            "name": "EXISTING_CONFIG", "valueFrom": {"secretKeyRef": {"name": "other", "key": "value"}}}
        cases.append((deployment, changed))

        container["volumeMounts"] = [{"name": "existing", "mountPath": "/data", "readOnly": True}]
        changed = copy.deepcopy(deployment)
        changed["spec"]["template"]["spec"]["containers"][0]["volumeMounts"][0]["mountPath"] = "/other"
        cases.append((deployment, changed))
        changed = copy.deepcopy(deployment)
        changed["spec"]["template"]["spec"]["containers"][0]["volumeMounts"][0]["readOnly"] = False
        cases.append((deployment, changed))
        changed = copy.deepcopy(deployment)
        changed["spec"]["template"]["spec"]["containers"][0]["volumeMounts"][0]["subPath"] = "other"
        cases.append((deployment, changed))

        deployment["spec"]["template"]["spec"]["volumes"] = [{
            "name": "existing", "configMap": {"name": "existing-config"}}]
        changed = copy.deepcopy(deployment)
        changed["spec"]["template"]["spec"]["volumes"][0]["configMap"]["name"] = "other-config"
        cases.append((deployment, changed))
        changed = copy.deepcopy(deployment)
        changed["spec"]["template"]["spec"]["volumes"][0] = {
            "name": "existing", "secret": {"secretName": "other-secret"}}
        cases.append((deployment, changed))

        for base, desired in cases:
            with self.subTest(kind=base["kind"], name=base["metadata"]["name"], desired=desired):
                with self.assertRaisesRegex(overlay.OverlayError, "existing list item"):
                    overlay.object_patch(base, desired)

    def test_rejects_uid_resourceversion_and_namespace_drift(self):
        base = captured_service("voice-chat", "uid", "2", [])
        for field, value in (("uid", "other"), ("resourceVersion", "3")):
            changed = copy.deepcopy(base)
            changed["metadata"][field] = value
            with self.subTest(field=field), self.assertRaises(overlay.OverlayError):
                overlay.object_patch(base, changed)
        outside = copy.deepcopy(base)
        outside["metadata"]["namespace"] = "voice-staging"
        with self.assertRaises(overlay.OverlayError):
            overlay.validate_target(overlay.TARGETS[1], outside)

    def test_planner_rejects_recreated_target_and_unqualified_image(self):
        class Renderer:
            def render(self, target, base, patch_path):
                return copy.deepcopy(base)

        root = Path(__file__).resolve().parents[2]
        target = overlay.TARGETS[3]
        patch_file = root / "deploy/dev/match-squad-trust-overlay" / target.patch_file
        base = sanitized_live_api_fixture()[(target.kind, target.name)]
        with self.assertRaisesRegex(overlay.OverlayError, "object UID"):
            recreated = copy.deepcopy(base)
            recreated["metadata"]["uid"] = "replacement-object"
            overlay.build_object_plan(Renderer(), target, recreated, patch_file)
        with self.assertRaisesRegex(overlay.OverlayError, "image not active"):
            unqualified = copy.deepcopy(base)
            unqualified["spec"]["template"]["spec"]["containers"][0]["image"] = (
                "ghcr.io/poryadok/voiceroot/matchmaking@sha256:" + "0" * 64)
            overlay.build_object_plan(Renderer(), target, unqualified, patch_file)

    def test_plan_binds_source_patch_and_render_hashes(self):
        self.assertEqual(len(overlay.sha256_bytes(b"reviewed")), 64)
        self.assertEqual(overlay.sha256_bytes(b"reviewed"),
                         hashlib.sha256(b"reviewed").hexdigest())
        with self.assertRaises(overlay.OverlayError):
            overlay.validate_source_sha("0" * 40, "1" * 40)

    def test_api_readback_hash_normalizes_only_documented_defaults(self):
        target = overlay.TARGETS[1]
        local = captured_service("voice-chat", "service-uid", "17", [
            {"name": "match-principal", "port": 9092,
             "targetPort": "match-principal"},
        ])
        api = copy.deepcopy(local)
        api["spec"]["ports"][0]["protocol"] = "TCP"
        self.assertNotEqual(overlay.rendered_hash(target, local),
                            overlay.rendered_hash(target, api))
        self.assertEqual(overlay.observed_render_hash(target, local),
                         overlay.observed_render_hash(target, api))
        api["spec"]["ports"][0]["protocol"] = "UDP"
        self.assertNotEqual(overlay.observed_render_hash(target, local),
                            overlay.observed_render_hash(target, api))

        deployment_target = overlay.TARGETS[3]
        local_deployment = captured_deployment("voice-matchmaking", "matchmaking", "uid", "4")
        spec = local_deployment["spec"]["template"]["spec"]
        spec["containers"][0]["ports"] = [{"name": "principal-jwks", "containerPort": 8443}]
        spec["volumes"] = [{"name": "match-squad-ca",
                            "configMap": {"name": "voice-match-squad-ca"}}]
        api_deployment = copy.deepcopy(local_deployment)
        api_spec = api_deployment["spec"]["template"]["spec"]
        api_spec["containers"][0]["ports"][0]["protocol"] = "TCP"
        api_spec["volumes"][0]["configMap"]["defaultMode"] = 420
        self.assertNotEqual(overlay.rendered_hash(deployment_target, local_deployment),
                            overlay.rendered_hash(deployment_target, api_deployment))
        self.assertEqual(overlay.observed_render_hash(deployment_target, local_deployment),
                         overlay.observed_render_hash(deployment_target, api_deployment))
        api_spec["volumes"][0]["configMap"]["defaultMode"] = 256
        self.assertNotEqual(overlay.observed_render_hash(deployment_target, local_deployment),
                            overlay.observed_render_hash(deployment_target, api_deployment))

    def fake_client(self, fail_at=None, dry_run_fail_at=None):
        class Client:
            def __init__(self, fail_at, dry_run_fail_at):
                self.current = sanitized_live_api_fixture()
                self.fail_at = fail_at
                self.dry_run_fail_at = dry_run_fail_at
                for target in overlay.TARGETS:
                    if target.kind == "Deployment":
                        container = next(c for c in self.current[(target.kind, target.name)]
                                         ["spec"]["template"]["spec"]["containers"]
                                         if c["name"] == overlay.CONTAINERS[target.name])
                        container["image"] = overlay.QUALIFIED_IMAGES[target.name]
                self.rendered = {}
                self.applied = []
                self.attempts = []
                self.dry_runs = []
                self.rollback_calls = 0
                self.rollouts = []

            def get_namespace(self):
                return {"metadata": {"name": "voice-dev",
                                      "uid": overlay.EXPECTED_NAMESPACE_UID,
                                      "labels": overlay.OWNERSHIP}}

            def get(self, target):
                return copy.deepcopy(self.current[(target.kind, target.name)])

            def render(self, target, base, patch_path):
                desired = copy.deepcopy(base)
                if target.kind == "Service":
                    ports = {"voice-matchmaking": ("principal-jwks", 8443),
                             "voice-chat": ("match-principal", 9092),
                             "voice-voice": ("match-provider", 9094)}
                    name, port = ports[target.name]
                    if not any(item.get("name") == name for item in desired["spec"]["ports"]):
                        desired["spec"]["ports"].append({"name": name, "port": port,
                                                           "targetPort": name})
                else:
                    pod_spec = desired["spec"]["template"]["spec"]
                    main = next(c for c in pod_spec["containers"]
                                if c["name"] == overlay.CONTAINERS[target.name])
                    main.setdefault("env", []).append({"name": "TEST_" + target.name,
                                                        "value": "controlled"})
                    pod_spec.setdefault("volumes", []).append({
                        "name": "test-overlay", "configMap": {"name": "test-overlay"}})
                self.rendered[(target.kind, target.name)] = desired
                return desired

            def server_dry_run(self, plan):
                target = (plan["kind"], plan["name"])
                self.dry_runs.append(target)
                if target == self.dry_run_fail_at:
                    raise overlay.OverlayError("synthetic API schema rejection")
                result = copy.deepcopy(self.rendered[target])
                return result

            def apply(self, plan):
                target = (plan["kind"], plan["name"])
                self.attempts.append(target)
                if target == self.fail_at:
                    raise RuntimeError("injected failure")
                result = copy.deepcopy(self.rendered[target])
                result["metadata"]["resourceVersion"] = "after-" + str(len(self.applied) + 1)
                self.current[target] = result
                self.applied.append(target)
                return result

            def wait_deployment(self, plan):
                self.rollouts.append(plan["name"])

            def verify_service_endpoints(self, name, expected_ports):
                return None

        return Client(fail_at, dry_run_fail_at)

    def test_server_dry_run_rejection_stops_before_every_write(self):
        client = self.fake_client(dry_run_fail_at=("Service", "voice-chat"))
        source_sha = "a" * 40
        root = Path(__file__).resolve().parents[2]
        stderr = io.StringIO()
        with tempfile.TemporaryDirectory(prefix="overlay-dry-run-reject-") as tmp:
            plan_path = Path(tmp) / "plan.json"
            with mock.patch.object(overlay, "_source_identity", return_value=source_sha):
                overlay.execute(root, source_sha, plan_path, client=client)
                plan_sha = hashlib.sha256(plan_path.read_bytes()).hexdigest()
                with contextlib.redirect_stderr(stderr), self.assertRaisesRegex(
                        overlay.OverlayError, "server dry-run rejected target"):
                    overlay.execute(root, source_sha, plan_path, plan_sha,
                                    apply=True, client=client)
        self.assertEqual(client.dry_runs, [("Service", "voice-matchmaking"),
                                           ("Service", "voice-chat")])
        self.assertEqual(client.applied, [])
        self.assertIn("writes=0", stderr.getvalue())

    def test_server_dry_run_preflights_all_effective_targets_before_apply(self):
        client = self.fake_client()
        source_sha = "a" * 40
        root = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory(prefix="overlay-dry-run-pass-") as tmp:
            plan_path = Path(tmp) / "plan.json"
            with mock.patch.object(overlay, "_source_identity", return_value=source_sha):
                overlay.execute(root, source_sha, plan_path, client=client)
                plan_sha = hashlib.sha256(plan_path.read_bytes()).hexdigest()
                overlay.execute(root, source_sha, plan_path, plan_sha,
                                apply=True, client=client)
        self.assertEqual(client.dry_runs, [(item.kind, item.name) for item in overlay.TARGETS])
        self.assertEqual(client.applied, client.dry_runs)

    def test_readonly_plan_then_exact_apply_covers_full_order(self):
        client = self.fake_client()
        source_sha = "a" * 40
        root = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory(prefix="overlay-plan-") as tmp:
            plan_path = Path(tmp) / "plan.json"
            with mock.patch.object(overlay, "_source_identity", return_value=source_sha):
                overlay.execute(root, source_sha, plan_path, client=client)
                self.assertEqual(client.applied, [])
                plan_bytes = plan_path.read_bytes()
                plan_sha = hashlib.sha256(plan_bytes).hexdigest()
                overlay.execute(root, source_sha, plan_path, plan_sha,
                                apply=True, client=client)
            self.assertEqual(client.applied, [(item.kind, item.name) for item in overlay.TARGETS])
            self.assertEqual(client.attempts, client.applied)
            self.assertEqual(client.rollouts, ["voice-matchmaking", "voice-chat",
                                               "voice-voice", "voice-gateway"])
            self.assertEqual(plan_path.stat().st_mode & 0o777, 0o600)

    def test_partial_plan_skips_exactly_satisfied_first_service_without_patch(self):
        client = self.fake_client()
        first = client.current[("Service", "voice-matchmaking")]
        first["spec"]["ports"].append({"name": "principal-jwks", "port": 8443,
                                         "targetPort": "principal-jwks"})
        source_sha = "a" * 40
        root = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory(prefix="overlay-partial-resume-") as tmp:
            plan_path = Path(tmp) / "plan.json"
            with mock.patch.object(overlay, "_source_identity", return_value=source_sha):
                overlay.execute(root, source_sha, plan_path, client=client)
                plan = json.loads(plan_path.read_text(encoding="utf-8"))
                self.assertTrue(plan["targets"][0]["alreadySatisfied"])
                self.assertEqual(plan["targets"][0]["operations"], [
                    {"op": "test", "path": "/metadata/uid", "value": first["metadata"]["uid"]},
                    {"op": "test", "path": "/metadata/resourceVersion",
                     "value": first["metadata"]["resourceVersion"]}])
                plan_sha = hashlib.sha256(plan_path.read_bytes()).hexdigest()
                overlay.execute(root, source_sha, plan_path, plan_sha,
                                apply=True, client=client)
        self.assertNotIn(("Service", "voice-matchmaking"), client.attempts)
        self.assertEqual(client.applied, [(item.kind, item.name) for item in overlay.TARGETS[1:]])

    def test_partial_plan_rejects_drift_before_any_suffix_patch(self):
        client = self.fake_client()
        first = client.current[("Service", "voice-matchmaking")]
        first["spec"]["ports"].append({"name": "principal-jwks", "port": 8443,
                                         "targetPort": "principal-jwks"})
        source_sha = "a" * 40
        root = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory(prefix="overlay-partial-drift-") as tmp:
            plan_path = Path(tmp) / "plan.json"
            with mock.patch.object(overlay, "_source_identity", return_value=source_sha):
                overlay.execute(root, source_sha, plan_path, client=client)
                first["spec"]["selector"]["app"] = "unexpected"
                plan_sha = hashlib.sha256(plan_path.read_bytes()).hexdigest()
                with self.assertRaisesRegex(overlay.OverlayError, "target drift"):
                    overlay.execute(root, source_sha, plan_path, plan_sha,
                                    apply=True, client=client)
        self.assertEqual(client.attempts, [])

    def test_plan_digest_tampering_is_rejected(self):
        plan = {"format": 1, "sourceSha": "a" * 40}
        with tempfile.TemporaryDirectory(prefix="overlay-plan-") as tmp:
            path = Path(tmp) / "plan.json"
            data = overlay.canonical_bytes(plan) + b"\n"
            path.write_bytes(data + b" ")
            with self.assertRaises(overlay.OverlayError):
                overlay._read_plan(path, hashlib.sha256(data).hexdigest())

    def test_partial_prefix_stops_without_retry_or_rollback(self):
        client = self.fake_client(fail_at=("Deployment", "voice-chat"))
        source_sha = "a" * 40
        root = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory(prefix="overlay-partial-") as tmp:
            plan_path = Path(tmp) / "plan.json"
            with mock.patch.object(overlay, "_source_identity", return_value=source_sha):
                overlay.execute(root, source_sha, plan_path, client=client)
                plan_sha = hashlib.sha256(plan_path.read_bytes()).hexdigest()
                with self.assertRaisesRegex(overlay.OverlayError, "first failed target"):
                    overlay.execute(root, source_sha, plan_path, plan_sha,
                                    apply=True, client=client)
        self.assertEqual(client.attempts, [("Service", "voice-matchmaking"),
                                           ("Service", "voice-chat"),
                                           ("Service", "voice-voice"),
                                           ("Deployment", "voice-matchmaking"),
                                           ("Deployment", "voice-chat")])
        self.assertEqual(client.applied, client.attempts[:-1])
        self.assertEqual(client.rollouts, ["voice-matchmaking"])
        self.assertEqual(client.rollback_calls, 0)

    def test_stop_reports_a_persisted_unverified_target_separately(self):
        client = self.fake_client()
        source_sha = "a" * 40
        root = Path(__file__).resolve().parents[2]
        original_apply = client.apply

        def mismatch(plan):
            if (plan["kind"], plan["name"]) == ("Service", "voice-matchmaking"):
                error = overlay.OverlayError("synthetic postcondition mismatch")
                error.persisted = True
                raise error
            return original_apply(plan)

        client.apply = mismatch
        stderr = io.StringIO()
        with tempfile.TemporaryDirectory(prefix="overlay-partial-prefix-") as tmp:
            plan_path = Path(tmp) / "plan.json"
            with mock.patch.object(overlay, "_source_identity", return_value=source_sha):
                overlay.execute(root, source_sha, plan_path, client=client)
                plan_sha = hashlib.sha256(plan_path.read_bytes()).hexdigest()
                with contextlib.redirect_stderr(stderr):
                    with self.assertRaisesRegex(overlay.OverlayError, "first failed target"):
                        overlay.execute(root, source_sha, plan_path, plan_sha,
                                        apply=True, client=client)
        self.assertIn("completed_prefix=0 applied_prefix=1", stderr.getvalue())
        self.assertEqual(client.applied, [])


if __name__ == "__main__":
    unittest.main()
