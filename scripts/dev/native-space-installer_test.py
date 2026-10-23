"""Space migration adapters must not bypass the owning Native installer."""
import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch
import os
import json
import copy
from types import SimpleNamespace

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("voice_dev_migrations", Path(__file__).with_name("migrations.py"))
migrations = importlib.util.module_from_spec(spec)
spec.loader.exec_module(migrations)


class NativeSpaceAdapters(unittest.TestCase):
    def test_dev_space_never_renders_generic_up(self):
        with patch.dict(os.environ, {"SPACE_NATIVE_INSTALLER_IMAGE": "example.invalid/installer@sha256:" + "a" * 64}):
            job = migrations.job("space", "postgres:18-alpine")
        container = job["spec"]["template"]["spec"]["containers"][0]
        self.assertNotIn("exec migrate ", " ".join(container.get("command", [])))
        self.assertIn("native-migrate", " ".join(container.get("command", [])))
        self.assertEqual(0, job["spec"]["backoffLimit"], "lost installer custody cannot auto-resume")

    def test_non_space_job_retains_existing_driver(self):
        container = migrations.job("chat", "postgres:18-alpine")["spec"]["template"]["spec"]["containers"][0]
        self.assertIn("exec migrate ", " ".join(container["command"]))

    def test_compose_space_branches_before_generic_state_repair(self):
        source = (ROOT / "docker/postgres/compose-migrate-dbs.sh").read_text(encoding="utf8")
        body = source.split("migrate_db() {", 1)[1]
        self.assertIn("native-migrate", body.split("has_sm=", 1)[0])

    def test_space_missing_digest_fails_before_job_render(self):
        with patch.dict(os.environ, {"SPACE_NATIVE_INSTALLER_IMAGE": ""}):
            with self.assertRaises(ValueError):
                migrations.job("space", "postgres:18-alpine")

    def test_sql_only_completed_space_job_cannot_skip_live_verifier(self):
        calls = []
        old = {"metadata": {"name": "voice-migrate-space-db"}, "status": {"succeeded": 1}}
        def get(kind):
            return [old] if kind == "jobs" else []
        def run(*args, **kwargs):
            calls.append(args)
            return SimpleNamespace(stdout=b"")
        with patch.dict(os.environ, {"SPACE_NATIVE_INSTALLER_IMAGE": "example.invalid/installer@sha256:" + "a" * 64}):
            with self.assertRaisesRegex(ValueError, "completed source/image-bound"):
                migrations.apply_space(ROOT, "postgres:18-alpine", lambda _: self.fail("unknown SQL Job cannot be adopted"), run, get)
        self.assertFalse(any("delete" in row for row in calls))


    def test_completed_evidence_requires_actual_successor_chain(self):
        image = "example.invalid/installer@sha256:" + "a" * 64
        expected = migrations.space_input_digest(ROOT, image)
        job = {"metadata": {"uid": "owned-job", "annotations": {"voice.io/native-input-hash": expected}},
               "status": {"succeeded": 1}, "spec": {"template": {"spec": {"containers": [{"name": "migrate", "image": image}]}}}}
        pod = {"metadata": {"ownerReferences": [{"uid": "owned-job"}]},
               "spec": {"containers": [{"name": "migrate", "image": image}]},
               "status": {"containerStatuses": [{"name": "migrate", "imageID": image}]}}
        receipt = {"created": True, "source_sha256": migrations.space_source_digest(ROOT),
                   "creation": {"database_name": "space_db"}, "trusted44": {"version": 44},
                   "transition45": {"Changed": True}, "transition50": {"Changed": True},
                   "trusted50": {"version": 50}, "transition51": {"profile": "native_catalog_rowjson_51"},
                   "final": {"version": 51}}
        for kind in ("valid51", "old50", "missing transition51", "wrong profile"):
            with self.subTest(kind=kind):
                candidate = copy.deepcopy(receipt)
                if kind == "old50":
                    candidate["final"]["version"] = 50
                elif kind == "missing transition51":
                    del candidate["transition51"]
                elif kind == "wrong profile":
                    candidate["transition51"]["profile"] = "native_catalog_50"
                log = json.dumps({"event": "complete", "receipt": candidate})
                if kind == "valid51":
                    evidence = migrations.space_evidence(ROOT, "voice-dev", image, job, [pod], log)
                    self.assertEqual(log, evidence["data"]["prior.jsonl"])
                    self.assertEqual(expected, evidence["metadata"]["annotations"]["voice.io/native-input-hash"])
                else:
                    with self.assertRaisesRegex(ValueError, "complete linked installer output"):
                        migrations.space_evidence(ROOT, "voice-dev", image, job, [pod], log)


if __name__ == "__main__":
    unittest.main()
