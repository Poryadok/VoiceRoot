"""Finite source qualification fixtures use real Git objects and histories."""
import pathlib
import hashlib
import json
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import f13_native_p3_source as native


class NativeSourceTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.repo = pathlib.Path(self.tmp.name)
        self.git("init", "-q")
        self.git("config", "user.name", "Native fixture")
        self.git("config", "user.email", "native@example.invalid")
        self.write("src/backend/voice/main.go", "base\n")
        self.write("src/backend/voice/removed.go", "old disabled fixture\n")
        self.commit("base")
        self.base = self.git("rev-parse", "HEAD")
        self.write("build.cfg", "unchanged dependency\n")
        self.write("src/backend/voice/main.go", "dormant\n")
        (self.repo / "src/backend/voice/removed.go").unlink()
        self.commit("repaired dormant source")
        self.head = self.git("rev-parse", "HEAD")
        self.record_data = {
            "version": "NATIVE-P3-SOURCE-v1", "base": self.base, "source": self.head,
            "classification": "DORMANT-A3-RESTRICTIVE-P3-SOURCE",
            "consumers": ["DEVELOP-SOURCE-v1", "MASTER-SOURCE-v1"],
            "projection": [{"path": "src/backend/voice/main.go", "before": self.entry(self.base, "src/backend/voice/main.go"),
                            "after": self.entry(self.head, "src/backend/voice/main.go")},
                           {"path": "src/backend/voice/removed.go", "before": self.entry(self.base, "src/backend/voice/removed.go"),
                            "after": None}],
            "context": [{"path": "build.cfg", "entry": self.entry(self.head, "build.cfg")}],
            "context_only_changed_generated": [], "closure_review": "https://example.invalid/closure",
            "down_review": "https://example.invalid/down", "source_proposal_sha256": "1" * 64,
            "evidence": [{"name": "synthetic-fixture", "sha256": "2" * 64}],
            "excluded_authority": ["activation"],
            "evidence_matrix": {k: {"result": "PASS", "evidence": "synthetic " + k, "review": "fixture"}
                                for k in native.EVIDENCE_CASES},
        }
        self.pin()

    def entry(self, revision, name):
        return native.entry(str(self.repo), revision, name)

    def pin(self):
        raw = json.dumps(self.record_data, indent=2) + "\n"
        self.write(native.RECORD_PATH, raw)
        self.commit("immutable record")
        self.record = self.git("rev-parse", "HEAD")
        self.write(native.VERIFIER_PATH, pathlib.Path(native.__file__).read_text())
        self.write(native.SHELL_PATH, pathlib.Path(native.__file__).with_name(pathlib.Path(native.SHELL_PATH).name).read_text())
        self.commit("later verifier carrier")
        self.carrier = self.git("rev-parse", "HEAD")
        self.pins = {"PINNED_BASE": self.base, "PINNED_SOURCE": self.head,
                     "PINNED_RECORD_COMMIT": self.record,
                     "PINNED_RECORD_SHA256": hashlib.sha256(raw.encode()).hexdigest()}

    def qualify(self, source=None, target=None, event="push", ref="refs/heads/develop", before=None, base_ref="", event_sha=None):
        source = source or self.carrier
        target = target or source
        with patch.multiple(native, **self.pins):
            return native.qualify(self.repo, self.base, source, target, event, ref, event_sha or target,
                                  before or self.base, base_ref)

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.repo), *args], text=True).strip()

    def write(self, name, value):
        p = self.repo / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(value)

    def commit(self, name):
        self.git("add", ".")
        self.git("commit", "--allow-empty", "-qm", name)

    def test_complete_repaired_unit_qualifies_develop_source(self):
        result = self.qualify()
        self.assertEqual(result["source"], self.carrier)
        self.assertEqual(result["consumer"], "DEVELOP-SOURCE-v1")

    def test_actual_master_pr_and_push_have_distinct_source_consumer(self):
        target = self.git("commit-tree", self.carrier + "^{tree}", "-p", self.base, "-p", self.carrier, "-m", "release")
        for event, ref in (("pull_request", "refs/pull/875/merge"), ("push", "refs/heads/master")):
            with self.subTest(event=event):
                result = self.qualify(target=target, event=event, ref=ref, base_ref="master")
                self.assertEqual(result["consumer"], "MASTER-SOURCE-v1")

    def test_wrong_event_shape_and_before_do_not_acquire_authority(self):
        target = self.git("commit-tree", self.carrier + "^{tree}", "-p", self.base, "-p", self.carrier, "-m", "release")
        cases = [dict(event="workflow_dispatch"), dict(ref="refs/heads/other"), dict(event_sha=self.base),
                 dict(before=self.head), dict(event="push", ref="refs/heads/master"),
                 dict(target=target, event="pull_request", ref="refs/pull/875/head", base_ref="master"),
                 dict(target=target, event="push", ref="refs/heads/master", before=self.head)]
        for case in cases:
            with self.subTest(case=case), self.assertRaises(native.Rejected):
                self.qualify(**case)

    def test_master_source_and_result_entries_must_match(self):
        self.write("foreign.txt", "only in merge result")
        self.commit("wrong merge tree")
        target = self.git("commit-tree", "HEAD^{tree}", "-p", self.base, "-p", self.carrier, "-m", "bad release")
        with self.assertRaisesRegex(native.Rejected, "master_result"):
            self.qualify(target=target, event="push", ref="refs/heads/master")

    def test_uncovered_real_evidence_rejects_even_closed_source_manifest(self):
        self.record_data["evidence_matrix"]["B3"]["result"] = "OPEN"
        self.pin()
        with self.assertRaisesRegex(native.Rejected, "evidence_open_B3"):
            self.qualify()

    def test_missing_support_context_mixed_entries_and_record_shape_reject(self):
        cases = ("support", "context", "mode", "type", "oid", "before", "duplicate", "extra", "scope")
        for case in cases:
            with self.subTest(case=case):
                saved = json.loads(json.dumps(self.record_data))
                if case == "support":
                    self.record_data["projection"] = []
                elif case == "context":
                    self.record_data["context"][0]["entry"]["oid"] = "0" * 40
                elif case in ("mode", "type", "oid"):
                    self.record_data["projection"][0]["after"][case] = {"mode": "100755", "type": "tree", "oid": "0" * 40}[case]
                elif case == "before":
                    self.record_data["projection"][0]["before"] = None
                elif case == "duplicate":
                    self.record_data["projection"] *= 2
                elif case == "extra":
                    self.record_data["projection"].append({"path": "z-foreign", "before": None, "after": None})
                else:
                    self.record_data["classification"] = "ACTIVE"
                self.pin()
                with self.assertRaises(native.Rejected):
                    self.qualify()
                self.record_data = saved

    def test_post_h_source_context_and_record_drift_reject(self):
        for name in ("src/backend/voice/main.go", "build.cfg", native.RECORD_PATH):
            with self.subTest(name=name):
                self.git("checkout", "-q", self.carrier)
                self.write(name, "unapproved\n")
                self.commit("post H drift")
                with self.assertRaises(native.Rejected):
                    self.qualify(source=self.git("rev-parse", "HEAD"))

    def test_wrong_pin_missing_objects_and_self_record_reject(self):
        for key, value in (("PINNED_RECORD_SHA256", "0" * 64), ("PINNED_RECORD_COMMIT", "0" * 40),
                           ("PINNED_RECORD_COMMIT", self.head)):
            with self.subTest(key=key, value=value):
                saved = self.pins[key]
                self.pins[key] = value
                with self.assertRaises(native.Rejected):
                    self.qualify()
                self.pins[key] = saved

    def test_worktree_record_and_environment_cannot_select_authority(self):
        self.write(native.RECORD_PATH, '{"caller": "spoof"}')
        with patch.dict("os.environ", {"NATIVE_P3_RECORD": "0" * 40, "NATIVE_P3_SKIP": "1"}):
            self.assertEqual(self.qualify()["record"], self.record)


if __name__ == "__main__":
    unittest.main()
