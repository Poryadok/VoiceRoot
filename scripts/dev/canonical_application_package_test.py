import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import canonical_application_package as p


class CanonicalPackageTests(unittest.TestCase):
    def setUp(self):
        self.sha = "a" * 40
        self.run = {"id": 12, "run_attempt": 1, "head_sha": self.sha, "head_branch": "master",
                    "event": "push", "path": ".github/workflows/ci.yml", "status": "completed", "conclusion": "success",
                    "repository": {"full_name": "Poryadok/VoiceRoot"}, "head_repository": {"full_name": "Poryadok/VoiceRoot"}}
        self.jobs = []
        for name, step in p.required_jobs().items():
            self.jobs.append({"id": len(self.jobs) + 1, "name": name, "run_id": 12, "run_attempt": 1,
                              "head_sha": self.sha, "status": "completed", "conclusion": "success",
                              "steps": [{"name": step, "conclusion": "success"}]})
        self.jobs.append({"id": 99, "name": "staging-images-promote", "run_id": 12, "run_attempt": 1,
                          "head_sha": self.sha, "status": "completed", "conclusion": "skipped", "steps": []})
        self.jobs.append({"id": 100, "name": "deploy-staging", "run_id": 12, "run_attempt": 1,
                          "head_sha": self.sha, "status": "completed", "conclusion": "skipped", "steps": []})

    def test_exact_master_push_and_all_builds(self):
        p.validate_run(self.run, self.sha, 12, 1)
        p.validate_jobs(self.jobs, self.sha, 12, 1)
        for key, value in (("event", "pull_request"), ("head_branch", "feature"), ("conclusion", "failure"), ("run_attempt", 2)):
            bad = dict(self.run, **{key: value})
            with self.assertRaises(ValueError): p.validate_run(bad, self.sha, 12, 1)
        for rows in (self.jobs[:-3] + self.jobs[-2:], self.jobs + [self.jobs[0]],
                     [dict(self.jobs[0], conclusion="failure")] + self.jobs[1:],
                     self.jobs[:-2] + [dict(self.jobs[-2], conclusion="success"), self.jobs[-1]],
                     self.jobs[:-1] + [dict(self.jobs[-1], conclusion="success")]):
            with self.assertRaises(ValueError): p.validate_jobs(rows, self.sha, 12, 1)

    def test_request_strict_and_changed(self):
        raw = json.dumps({"schema": "voice-full-application-build-request-v1", "cycle": 1, "request_id": "1" * 32}).encode()
        p.validate_request(raw, None)
        with self.assertRaises(ValueError): p.validate_request(raw, raw)
        second = json.dumps({"schema": "voice-full-application-build-request-v1", "cycle": 2, "request_id": "2" * 32}).encode()
        p.validate_request(second, raw)
        with self.assertRaises(ValueError): p.validate_request(second, None)
        for invalid in (b'{}', b'{"schema":"voice-full-application-build-request-v1","cycle":1,"cycle":2,"request_id":"11111111111111111111111111111111"}',
                        b'{"schema":"voice-full-application-build-request-v1","cycle":3,"request_id":"11111111111111111111111111111111"}'):
            with self.assertRaises(ValueError): p.validate_request(invalid, None)

    def test_receipt_inventory_and_image_binding(self):
        images = {name: p.REGISTRY + "/" + name + "@sha256:" + "1" * 64 for name in p.IMAGE_NAMES}
        p.validate_images(images)
        for bad in (dict(list(images.items())[:-1]), dict(images, web="evil@sha256:" + "1" * 64),
                    dict(images, web=p.REGISTRY + "/web:latest")):
            with self.assertRaises(ValueError): p.validate_images(bad)

    def test_receipt_is_exclusive_and_complete(self):
        with tempfile.TemporaryDirectory() as folder:
            output = Path(folder) / "receipt.json"
            p.write_receipt(output, {"source": self.sha})
            self.assertEqual(json.loads(output.read_bytes()), {"source": self.sha})
            with self.assertRaises(FileExistsError): p.write_receipt(output, {"source": "other"})
            self.assertEqual(json.loads(output.read_bytes()), {"source": self.sha})
            self.assertEqual(list(Path(folder).iterdir()), [output])

    def test_capture_never_emits_receipt_if_revalidation_fails(self):
        raw = json.dumps({"schema": "voice-full-application-build-request-v1", "cycle": 1, "request_id": "1" * 32}).encode()
        images = {name: p.REGISTRY + "/" + name + "@sha256:" + "1" * 64 for name in p.IMAGE_NAMES}
        def api(url, headers, deadline):
            if "/actions/runs/" in url: return self.run
            if "/git/commits/" in url: return {"sha": self.sha, "parents": [{"sha": "b" * 40}]}
            raise AssertionError(url)
        def source_capture(token, run_id, sha, folder):
            marker = folder / p.REQUEST
            marker.parent.mkdir(parents=True)
            marker.write_bytes(raw)
            return {"run_attempt": 1, "source_sha": sha, "images": images, "lock_artifact_id": 7, "lock_sha256": "2" * 64}
        with tempfile.TemporaryDirectory() as folder, \
             mock.patch.object(p.common, "command", return_value=("c" * 40).encode()), \
             mock.patch.object(p.common, "source_snapshot", return_value={"source": self.sha}), \
             mock.patch.object(p.source, "_get", side_effect=api), \
             mock.patch.object(p.source, "_pages", return_value=self.jobs), \
             mock.patch.object(p, "tree_blob", return_value=None), \
             mock.patch.object(p.source, "capture_equivalent_master_source", side_effect=source_capture), \
             mock.patch.object(p.source, "_head", side_effect=ValueError("stale")):
            output = Path(folder) / "package.json"
            with self.assertRaisesRegex(ValueError, "stale"): p.capture("token", self.sha, 12, 1, output)
            self.assertFalse(output.exists())

    def test_equivalent_later_master_keeps_exact_source_images(self):
        raw = json.dumps({"schema": "voice-full-application-build-request-v1", "cycle": 1, "request_id": "1" * 32}).encode()
        images = {name: p.REGISTRY + "/" + name + "@sha256:" + "1" * 64 for name in p.IMAGE_NAMES}
        def api(url, headers, deadline):
            if "/actions/runs/" in url: return self.run
            if "/git/commits/" in url: return {"sha": self.sha, "parents": [{"sha": "b" * 40}]}
            raise AssertionError(url)
        def source_capture(token, run_id, sha, folder):
            marker = folder / p.REQUEST
            marker.parent.mkdir(parents=True)
            marker.write_bytes(raw)
            return {"run_attempt": 1, "source_sha": sha, "images": images, "lock_artifact_id": 7,
                    "lock_sha256": "2" * 64, "source_equivalence": {"current_master_sha_after": "d" * 40}}
        with tempfile.TemporaryDirectory() as folder, \
             mock.patch.object(p.common, "command", return_value=("c" * 40).encode()), \
             mock.patch.object(p.common, "source_snapshot", return_value={"source": "c" * 40}), \
             mock.patch.object(p.source, "_get", side_effect=api), \
             mock.patch.object(p.source, "_pages", return_value=self.jobs), \
             mock.patch.object(p, "tree_blob", return_value=None), \
             mock.patch.object(p.source, "capture_equivalent_master_source", side_effect=source_capture), \
             mock.patch.object(p.source, "_head", return_value={}), \
             mock.patch.object(p.source, "_equivalent_master_source") as equivalent, \
             mock.patch.object(p.source, "_ci", return_value=(self.run, {})), \
             mock.patch.object(p.source, "_resolve_image", side_effect=lambda name, sha, deadline: images[name]):
            output = Path(folder) / "package.json"
            receipt = p.capture("token", self.sha, 12, 1, output)
            self.assertEqual(receipt["source_sha"], self.sha)
            self.assertEqual(receipt["images"], images)
            self.assertEqual(receipt["source_equivalence"]["current_master_sha_after"], "d" * 40)
            equivalent.assert_called_once()
            # A changed mutable SHA tag after capture cannot authorize a receipt.
            with mock.patch.object(p.source, "_resolve_image", return_value=p.REGISTRY + "/wrong@sha256:" + "9" * 64):
                rejected = Path(folder) / "rejected.json"
                with self.assertRaisesRegex(ValueError, "tag_digest_changed"):
                    p.capture("token", self.sha, 12, 1, rejected)
                self.assertFalse(rejected.exists())


if __name__ == "__main__": unittest.main()
