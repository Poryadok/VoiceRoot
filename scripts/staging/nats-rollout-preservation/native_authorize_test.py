"""Focused fake typed-API consumer for the native authorization entry."""
import copy
import hashlib
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import native_authorize
from controller import Blocked


OP = "a" * 12
UID = "11111111-1111-4111-8111-111111111111"
PV_UID = "22222222-2222-4222-8222-222222222222"
TARGET = "new bootstrap script"
OLD = "old bootstrap script"


class TypedAPI:
    """Only the methods consumed by native authorization are exposed."""

    def __init__(self):
        self.calls = []

    def get_configmap(self, namespace, name):
        self.calls.append(("get-configmap", namespace, name))
        return {"metadata": {"name": name, "namespace": namespace, "uid": UID,
                "resourceVersion": "7"}, "data": {"bootstrap.sh": OLD}}

    def cas_nats_bootstrap_configmap(self, namespace, name, uid, rv, sha, operation, script):
        self.calls.append(("cas-configmap", name, uid, rv, sha, operation))
        return {"metadata": {"name": name, "namespace": namespace, "uid": uid,
                "resourceVersion": "8"}, "data": {"bootstrap.sh": script}}

    def make_native_migration_view(self, request):
        self.calls.append(("migration-view", request["operation"]))
        return self


class Stage:
    def __init__(self):
        self.events = []
        self.expected = {"namespace_uid": "ns", "source_claim": "source",
                         "source_claim_uid": UID, "source_pv_uid": PV_UID,
                         "source_node_uid": "node-uid", "source_node_name": "node"}
        self.marker = {"metadata": {"uid": "marker", "resourceVersion": "9"},
                       "data": {"generation": "r20261018", "previousGeneration": "r20261017",
                                "dataPVC": "source"}}
        self.final_claim = {"metadata": {"name": "source", "uid": UID}}
        self.final_pv = {"metadata": {"name": "source-pv", "uid": PV_UID}}

    def verify_final_storage(self):
        self.events.append("source-fence")


class PostRuntime:
    def __init__(self, events):
        self.events = events

    def archive_selected_store(self, path):
        self.events.append("post-archive")
        return {"mechanism": "fake", "archive_sha256": "c" * 64,
                "file_count": 0, "bytes": 0, "dirs": [], "files": []}

    def capture_census(self, path, manifest, label, *, validate):
        self.events.append("post-census")
        validate()
        return {"schema": "voice-nats-census-v1", "account": "A" + "B" * 55,
                "streams": [], "consumers": []}


class NativeAuthorizeTests(unittest.TestCase):
    def test_native_authorize_runs_typed_sequence_and_writes_receipt(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            (base / "rollout-before.tar").write_bytes(b"archive")
            (base / "rollout-before-manifest.json").write_bytes(b"manifest")
            events = []
            stage = Stage()
            api = TypedAPI()
            script_sha = hashlib.sha256(OLD.encode()).hexdigest()
            state = {
                "operation": OP, "phase": "AWAITING_OFF_NODE", "status": "WAITING",
                "fence_status": "VERIFIED", "events": [], "target": {"mode": "images-only"},
                "native_contract": {"request": {"operation": OP}, "binding": {"uid": UID},
                    "plan": {"contract": "native"}, "plan_envelope": {
                        "path": "native-contract-inputs.json",
                        "sha256": hashlib.sha256(b"envelope").hexdigest(), "bytes": 8},
                    "runner_image": "runner@sha256:" + "e" * 64},
                "migrations": {"plan": "captured"},
                "contract": {"scripts": [{"part": p, "configMapUID": UID,
                    "configMapResourceVersion": "7", "sha256": script_sha} for p in
                    ("realtime", "analytics-chat", "search")]},
                "nats_target_scripts": {p: {"script": TARGET,
                    "sha256": hashlib.sha256(TARGET.encode()).hexdigest()} for p in
                    ("realtime", "analytics-chat", "search")},
                "native_source_binding": {"binding": {"source_claim": "source",
                    "source_uid": UID, "source_pv_name": "source-pv", "source_pv_uid": PV_UID,
                    "source_node_name": "node"}, "proof": {"node_uid": "node-uid"}},
                "cut": {"manifest": {"archive_sha256": "a" * 64},
                        "manifest_sha256": "b" * 64, "census_sha256": "d" * 64,
                        "census": {"schema": "voice-nats-census-v1", "account": "A" + "B" * 55,
                                   "streams": [], "consumers": []}},
                "context": {"expected": stage.expected, "snapshots": {},
                    "original_snapshots": {}, "old_images": {}, "marker": stage.marker,
                    "service": {}, "final_claim": stage.final_claim, "final_pv": stage.final_pv},
            }
            post = PostRuntime(events)
            (base / "native-contract-inputs.json").write_bytes(b"envelope")
            (base / "native-contract-inputs.json").chmod(0o600)
            resolved = state["native_source_binding"]
            def contract_job(*args, **kwargs):
                events.append("contract-job")
                return {"job_uid": UID}

            def migration_request(*args, **kwargs):
                events.append("migration-build")
                return {"schema": "fake", "operation": OP}

            def migration_apply(*args, **kwargs):
                events.append("migration-apply")
                return {"status": "completed"}

            with patch("kube_json_api.resolve_source_binding", return_value=resolved), \
                 patch("native_authorize.file_sha", return_value="b" * 64), \
                 patch("native_authorize.verify_archive", return_value=True), \
                 patch("transaction.reconstruct", return_value=stage), \
                 patch("transaction.run_native_contract_job", side_effect=contract_job), \
                 patch("native_migrations_runtime.build_request", side_effect=migration_request), \
                 patch("native_migrations_runtime.apply", side_effect=migration_apply), \
                 patch("transaction.native_preservation_runtime", return_value=post), \
                 patch("transaction.context", return_value=state["context"]), \
                 patch("transaction.save"):
                receipt = native_authorize.authorize(
                    api, base, state, "a" * 64, "b" * 64, state["contract"],
                    native_config={"phase_image": "image@sha256:" + "e" * 64})
            self.assertEqual(events, ["contract-job", "migration-build", "migration-apply",
                                      "post-archive", "post-census"])
            self.assertEqual([call[0] for call in api.calls],
                             ["migration-view", "get-configmap", "cas-configmap",
                              "get-configmap", "cas-configmap", "get-configmap", "cas-configmap"])
            self.assertEqual(receipt["operation"], OP)
            self.assertEqual(receipt["backup"]["archive_sha256"], "a" * 64)
            self.assertEqual(state["phase"], "PAUSED_APPLY")
            self.assertNotIn("get", api.calls)

    def test_unsupported_target_is_rejected_before_reconstruct_or_api(self):
        state = {"phase": "AWAITING_OFF_NODE", "status": "WAITING",
                 "fence_status": "VERIFIED", "target": {"mode": "full"}}
        with patch("transaction.reconstruct") as reconstruct:
            with self.assertRaisesRegex(Blocked, "target_mode_unsupported"):
                native_authorize.authorize(object(), Path("."), state, "a" * 64,
                    "b" * 64, {}, native_config={"phase_image": "x"})
        reconstruct.assert_not_called()

    def test_missing_contract_authority_is_rejected_before_reconstruct(self):
        state = {"phase": "AWAITING_OFF_NODE", "status": "WAITING",
                 "fence_status": "VERIFIED", "target": {"mode": "images-only"},
                 "migrations": {"plan": "captured"}}
        with patch("transaction.reconstruct") as reconstruct:
            with self.assertRaisesRegex(Blocked, "contract_authority_missing"):
                native_authorize.authorize(object(), Path("."), state, "a" * 64,
                    "b" * 64, {}, native_config={"phase_image": "x"})
        reconstruct.assert_not_called()

    def test_native_api_and_config_must_be_supplied_as_a_pair(self):
        import bridge_root
        with self.assertRaisesRegex(Blocked, "native_authority_pair_invalid"):
            bridge_root.Actions(Path("."), native_api=object())
        with self.assertRaisesRegex(Blocked, "native_authority_pair_invalid"):
            bridge_root.Actions(Path("."), native_config={"phase_image": "x"})


if __name__ == "__main__":
    unittest.main()
