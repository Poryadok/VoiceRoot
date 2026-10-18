"""Focused fake typed-API consumer for the native authorization entry."""
import copy
import hashlib
import json
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import native_authorize
import transaction
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


def _contract_fixture(base):
    (base / "inputs").mkdir()
    (base / "inputs/bootstrap.creds").write_bytes(b"credentials")
    (base / "server.conf").write_bytes(b"server configuration")
    (base / "inputs/bootstrap.creds").chmod(0o600)
    (base / "server.conf").chmod(0o600)
    source_uid = "33333333-3333-4333-8333-333333333333"
    source_pv_uid = "44444444-4444-4444-8444-444444444444"
    verify_uid = "55555555-5555-4555-8555-555555555555"
    verify_pv_uid = "66666666-6666-4666-8666-666666666666"
    job_uid = "77777777-7777-4777-8777-777777777777"
    pod_uid = "88888888-8888-4888-8888-888888888888"
    attempt = "abcdef12"
    nonce = "b" * 32
    job_name = "voice-nats-contract-" + OP + "-" + attempt
    return {
        "operation": OP, "native_source_binding": {
            "binding": {"source_claim": "voice-nats-jsdata-d20261018a",
                "source_uid": source_uid, "source_pv_name": "source-pv",
                "source_pv_uid": source_pv_uid, "source_node_name": "pmdebook"},
            "proof": {"node_hostname": "pmdebook"}},
        "nats_contract": {"schema": "fixture", "actions": []},
        "cut": {"manifest": {"archive_sha256": "a" * 64, "bytes": 128}},
        "events": [
            {"phase": "census", "state": "PVC_BOUND", "operation": OP,
                "nonce": nonce, "verification_claim": "voice-nats-verify-" + OP + "-" + attempt,
                "verification_uid": verify_uid, "verification_pv_name": "verify-pv",
                "verification_pv_uid": verify_pv_uid, "verification_node_name": "pmdebook",
                "verification_hostname": "pmdebook", "session_attempt": attempt},
            {"phase": "census", "state": "JOB_CREATE_INTENT", "operation": OP,
                "nonce": nonce, "job_name": job_name, "template_sha256": "c" * 64,
                "verification_uid": verify_uid, "session_attempt": attempt},
            {"phase": "census", "state": "JOB_CREATED", "operation": OP,
                "nonce": nonce, "job_name": job_name, "job_uid": job_uid,
                "template_sha256": "c" * 64, "verification_uid": verify_uid,
                "session_attempt": attempt},
            {"phase": "census", "state": "INPUTS_COMPLETE", "operation": OP,
                "nonce": nonce, "pod_uid": pod_uid, "job_uid": job_uid,
                "session_attempt": attempt, "input_bundle_sha256": "b" * 64,
                "input_bundle_bytes": 256},
            {"phase": "census", "state": "CENSUS_EXECUTION_COMPLETE", "operation": OP,
                "nonce": nonce, "pod_uid": pod_uid, "job_uid": job_uid,
                "session_attempt": attempt, "archive_sha256": "a" * 64,
                "archive_bytes": 128}],
    }


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

    def test_prepare_produces_contract_from_census_and_private_inputs(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            state = _contract_fixture(base)
            transaction._produce_native_contract_authority(base, state, {
                "runner_image": "runner@sha256:" + "e" * 64})
            row = native_authorize._native_contract(base, state)
            envelope = json.loads(row["plan_envelope"].decode())
            self.assertEqual(envelope["source_pvc_uid"], "33333333-3333-4333-8333-333333333333")
            self.assertEqual(envelope["verification_pvc_uid"], "55555555-5555-4555-8555-555555555555")
            self.assertEqual(row["binding"]["contract_attempt"], envelope["contract_attempt"])

    def test_prepare_rejects_mixed_census_session_before_contract_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            state = _contract_fixture(base)
            state["events"][3]["session_attempt"] = "deadbeef"
            with patch("transaction.run_native_contract_job") as contract_job:
                with self.assertRaisesRegex(Blocked, "census_binding_invalid"):
                    transaction._produce_native_contract_authority(base, state, {
                        "runner_image": "runner@sha256:" + "e" * 64})
            contract_job.assert_not_called()
            self.assertFalse((base / "native-contract-inputs.json").exists())

    def test_prepare_rejects_archive_mismatch_before_contract_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            state = _contract_fixture(base)
            state["events"][4]["archive_sha256"] = "f" * 64
            with patch("transaction.run_native_contract_job") as contract_job:
                with self.assertRaisesRegex(Blocked, "census_binding_invalid"):
                    transaction._produce_native_contract_authority(base, state, {
                        "runner_image": "runner@sha256:" + "e" * 64})
            contract_job.assert_not_called()
            self.assertFalse((base / "native-contract-inputs.json").exists())


if __name__ == "__main__":
    unittest.main()
