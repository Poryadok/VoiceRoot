import copy
import hashlib
import json
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
                              "3355912", "61061cc7accc3e0eccc1475a0c77deed5e6424f288f83d7a494ac95228c19e3c"),
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
            {"name": "match-squad-provider", "port": 9094, "protocol": "TCP",
             "targetPort": "match-squad-provider"},
            {"name": "match-squad-member", "port": 9095, "protocol": "TCP",
             "targetPort": "match-squad-member"},
        ])
        operations = overlay.object_patch(base, desired)
        self.assertEqual(operations[0], {"op": "test", "path": "/metadata/uid",
                                         "value": "4f99e5d7-15ed-4ce9-bbc4-cd999fa91475"})
        self.assertEqual([op["value"]["port"] for op in operations[2:]], [9094, 9095])

    def test_api_patch_request_is_single_guarded_json_patch(self):
        target = overlay.TARGETS[1]
        base = captured_service("voice-chat", "service-uid", "17", [])
        desired = copy.deepcopy(base)
        desired["spec"]["ports"].append({"name": "match-squad-principal", "port": 9092,
                                           "targetPort": "match-squad-principal"})
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

    def test_actual_voice_deployment_fixture_preserves_media_trust_and_existing_sidecar(self):
        base = sanitized_live_api_fixture()[("Deployment", "voice-voice")]
        desired = copy.deepcopy(base)
        pod_spec = desired["spec"]["template"]["spec"]
        container = pod_spec["containers"][0]
        container["env"].append({"name": "VOICE_MATCH_SQUAD_PRINCIPAL_CLIENT_CA_FILE",
                                 "value": "/etc/voice/match-squad/ca/ca.crt"})
        container["env"].append({"name": "VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_CLIENT_CA_FILE",
                                 "value": "/etc/voice/media/ca/ca.crt"})
        container["ports"].append({"name": "match-squad-provider", "containerPort": 9094})
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
        desired["spec"]["ports"] = [{"name": "match-squad-principal", "port": 9092,
                                       "targetPort": "match-squad-principal"}]
        operations = overlay.object_patch(base, desired)
        self.assertEqual(operations[:2], [
            {"op": "test", "path": "/metadata/uid", "value": "service-uid"},
            {"op": "test", "path": "/metadata/resourceVersion", "value": "17"},
        ])
        self.assertEqual(operations[2:], [{
            "op": "add", "path": "/spec/ports/-",
            "value": {"name": "match-squad-principal", "port": 9092,
                      "targetPort": "match-squad-principal"},
        }])

    def test_deployment_plan_appends_only_reviewed_list_items(self):
        base = captured_deployment("voice-voice", "voice", "deploy-uid", "21")
        desired = copy.deepcopy(base)
        container = desired["spec"]["template"]["spec"]["containers"][0]
        container["env"].append({"name": "VOICE_MATCH_SQUAD_PRINCIPAL_GRPC_LISTEN",
                                 "value": ":9094"})
        container["ports"].append({"name": "match-squad-provider", "containerPort": 9094})
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
            overlay.build_object_plan(Renderer(), target, base, patch_file)

    def test_plan_binds_source_patch_and_render_hashes(self):
        self.assertEqual(len(overlay.sha256_bytes(b"reviewed")), 64)
        self.assertEqual(overlay.sha256_bytes(b"reviewed"),
                         hashlib.sha256(b"reviewed").hexdigest())
        with self.assertRaises(overlay.OverlayError):
            overlay.validate_source_sha("0" * 40, "1" * 40)

    def test_api_readback_hash_normalizes_only_documented_defaults(self):
        target = overlay.TARGETS[1]
        local = captured_service("voice-chat", "service-uid", "17", [
            {"name": "match-squad-principal", "port": 9092,
             "targetPort": "match-squad-principal"},
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

    def fake_client(self, fail_at=None):
        class Client:
            def __init__(self, fail_at):
                self.current = sanitized_live_api_fixture()
                self.fail_at = fail_at
                for target in overlay.TARGETS:
                    if target.kind == "Deployment":
                        container = next(c for c in self.current[(target.kind, target.name)]
                                         ["spec"]["template"]["spec"]["containers"]
                                         if c["name"] == overlay.CONTAINERS[target.name])
                        container["image"] = overlay.QUALIFIED_IMAGES[target.name]
                self.rendered = {}
                self.applied = []
                self.attempts = []
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
                             "voice-chat": ("match-squad-principal", 9092),
                             "voice-voice": ("match-squad-provider", 9094)}
                    name, port = ports[target.name]
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

        return Client(fail_at)

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


if __name__ == "__main__":
    unittest.main()
