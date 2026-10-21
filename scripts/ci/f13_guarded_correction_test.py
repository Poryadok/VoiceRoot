"""Real Git collector regressions for source-pinned guarded corrections."""
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts/ci/voice-db-runtime-provisioning-contract_test.sh"
VERIFIER = ROOT / "scripts/ci/f13_guarded_correction.py"
RECORD_PATH = "docs/testing/guarded-correction-voice013-v1.json"
DOWN = "src/backend/migrations/voice_db/000013_a3_roster_outbox.down.sql"
UP = "src/backend/migrations/voice_db/000013_a3_roster_outbox.up.sql"


def git(repo, *args):
    return subprocess.check_output(["git", "-C", str(repo), *args], text=True).strip()


def functions(path):
    selected = {
        "f13_collect_develop_contributions", "f13_path_is_guarded",
        "f13_evaluate_guarded_owner_qualifiers", "f13_guarded_owner_transition_allowed",
        "f13_resolve_source_base", "f13_collect_failure",
        "f13_emit_guarded_failure_diagnostic", "f13_tree_entry_identity", "f13_guarded_entry_matches_final_target",
        "game_checkpoint_contribution_allowed", "game_checkpoint_aggregate_path_allowed",
        "game_checkpoint_aggregate_binding_valid", "game_checkpoint_path_authorized_in_delta",
        "be255_checkpoint_path_authorized_in_delta", "game_checkpoint_domain_path",
        "f13_changed_content", "forbidden_activation_content", "voice_r22_runtime_changed",
        "f13_develop_aggregate_enabled", "f13_guarded_correction_qualified",
    }
    return "\n".join(m.group(0) for m in re.finditer(
        r"(?ms)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{\n.*?^\}\n", path.read_text())
        if m.group(1) in selected)


class History:
    def __init__(self, folder, retained_activation=False, extra_correction=False, recorded_extra=False):
        self.repo = folder / "repo"
        self.repo.mkdir()
        git(self.repo, "init", "-q", "-b", "develop")
        git(self.repo, "config", "user.name", "F13 fixture")
        git(self.repo, "config", "user.email", "fixture@example.invalid")
        self.path = "src/backend/voice/pb/fixture.pb.go" if retained_activation else DOWN
        self.write(self.path, "baseline\n")
        self.write(UP, "unchanged prerequisite\n")
        self.base = self.commit("base")
        git(self.repo, "checkout", "-qb", "source")
        self.write(self.path, "NewLifecycleWorker()\nunsafe\n" if retained_activation else "unsafe\n")
        if retained_activation:
            self.write("src/backend/voice/internal/roomlifecycle/fixture.go", "runtime input\n")
        self.source = self.commit("source")
        git(self.repo, "checkout", "-qb", "donor")
        self.write(self.path, "NewLifecycleWorker()\nsafe\n" if retained_activation else "safe\n")
        if recorded_extra:
            self.write("z-new.txt", "ordinary independent history\n")
        self.donor = self.commit("reviewed donor")
        git(self.repo, "checkout", "-q", "develop")
        git(self.repo, "merge", "--no-ff", "-qm", "import source", "source")
        self.owner = git(self.repo, "rev-parse", "HEAD")
        self.write(self.path, git(self.repo, "show", f"{self.donor}:{self.path}") + "\n")
        if extra_correction:
            self.write("unrecorded.txt", "extra delta must not qualify\n")
        if recorded_extra:
            self.write("z-new.txt", "ordinary independent history\n")
        self.correction = self.commit("direct correction")
        self.record = {
            "version": "GUARDED-CORRECTION-V1", "prior_owner": self.owner,
            "prior_source": self.source, "correction": self.correction,
            "content_base": self.base, "prior_runtime_trigger": "true",
            "donor": self.donor, "review": "https://example.invalid/review",
            "corrections": [{"path": self.path, "before": self.entry(self.owner, self.path),
                             "after": self.entry(self.correction, self.path)}],
            "prerequisites": [{"path": UP, "entry": self.entry(self.owner, UP)}],
        }
        if recorded_extra:
            self.record["corrections"].append({"path": "z-new.txt", "before": None,
                                                "after": self.entry(self.correction, "z-new.txt")})
        self.pin_record()

    def write(self, name, content):
        p = self.repo / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(content)

    def commit(self, message):
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-qm", message)
        return git(self.repo, "rev-parse", "HEAD")

    def entry(self, revision, path):
        row = git(self.repo, "ls-tree", revision, "--", path)
        if not row:
            return None
        mode, kind, oid = row.split("\t")[0].split()
        return {"mode": mode, "type": kind, "oid": oid}

    def pin_record(self):
        data = json.dumps(self.record, indent=2) + "\n"
        self.write(RECORD_PATH, data)
        self.record_commit = self.commit("immutable authority record")
        # Compile-time constants in a private fixture copy, never environment
        # authority and never a write to the real production verifier/record.
        if VERIFIER.exists():
            code = VERIFIER.read_text()
            code = re.sub(r'^PINNED_RECORD_COMMIT = ".*"$',
                          'PINNED_RECORD_COMMIT = "' + self.record_commit + '"', code, flags=re.M)
            code = re.sub(r'^PINNED_RECORD_SHA256 = ".*"$',
                          'PINNED_RECORD_SHA256 = "' + hashlib.sha256(data.encode()).hexdigest() + '"', code, flags=re.M)
            self.write("scripts/ci/f13_guarded_correction.py", code)
        self.write("fixture-pin.txt", self.record_commit + "\n")
        self.target = self.commit("later source verifier pin")

    def collect(self, event="push", ref="refs/heads/develop", downstream=False, target=None):
        temp = self.repo.parent / ("run-" + str(len(list(self.repo.parent.glob("run-*")))))
        temp.mkdir()
        (temp / "game-checkpoint-tree").touch()
        (temp / "be255-checkpoint-tree").touch()
        program = "set -euo pipefail\n" + functions(SCRIPT)
        program += "\n" + functions(ROOT / "scripts/ci/voice-r22-runtime-scope.sh")
        program += f"\nROOT={shlex.quote(str(self.repo))}\nTMP_DIR={shlex.quote(str(temp))}\n"
        program += "game_checkpoint_aggregate_active=false\ngame_checkpoint_snapshot_base=''\ngame_checkpoint_snapshot_head=''\ngame_checkpoint_snapshot_record=''\ngame_checkpoint_aggregate_head=''\n"
        program += f"GITHUB_EVENT_NAME={shlex.quote(event)}\nGITHUB_REF={shlex.quote(ref)}\n"
        program += f"f13_collect_develop_contributions \"$ROOT\" {self.base} {target or self.target} \"$TMP_DIR/map\" \"$TMP_DIR/category\"\n"
        if downstream:
            program += f'f13_changed_content "$ROOT" "$TMP_DIR/map" {shlex.quote(self.path)} >"$TMP_DIR/content"\n'
            program += 'forbidden_activation_content "$TMP_DIR/content"\nprintf "DOWNSTREAM_FORBIDDEN_RETAINED\\n"\n'
        harness = temp / "collector.sh"
        harness.write_text(program)
        r = subprocess.run(["bash", str(harness)], capture_output=True, text=True)
        rows = [line.split("\t") for line in (temp / "map").read_text().splitlines()] if (temp / "map").exists() else []
        return r, rows, temp


class CollectorTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.before = {p: hashlib.sha256(p.read_bytes()).hexdigest() for p in (SCRIPT, VERIFIER) if p.exists()}

    def tearDown(self):
        for path, digest in self.before.items():
            self.assertEqual(digest, hashlib.sha256(path.read_bytes()).hexdigest(), "fixture changed production source")

    def history(self, **kwargs):
        folder = Path(self.temp.name) / str(len(list(Path(self.temp.name).iterdir())))
        folder.mkdir()
        return History(folder, **kwargs)

    def assert_rejected(self, history, **kwargs):
        r, _, temp = history.collect(**kwargs)
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn("duplicate_guarded_owner", r.stderr)
        self.assertFalse((temp / "f13-guarded-corrections.jsonl").exists())

    def test_qualified_direct_correction_keeps_exact_real_owner_row(self):
        h = self.history()
        r, rows, temp = h.collect()
        self.assertEqual(r.returncode, 0, r.stderr)
        selected = [row for row in rows if row[0] == h.path]
        self.assertEqual(len(selected), 1)
        self.assertEqual(selected[0][2:7], [h.correction, h.owner, h.owner, h.correction, h.base])
        self.assertEqual(selected[0][1], "true")
        ledger = (temp / "f13-guarded-corrections.jsonl").read_text().splitlines()
        self.assertEqual(len(ledger), 1)
        self.assertEqual(json.loads(ledger[0])["correction"], h.correction)

    def test_qualification_retains_prior_runtime_and_forbidden_content(self):
        h = self.history(retained_activation=True)
        r, rows, _ = h.collect(downstream=True)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("DOWNSTREAM_FORBIDDEN_RETAINED", r.stdout)
        selected = [row for row in rows if row[0] == h.path]
        self.assertEqual(selected[0][1], "true")
        self.assertEqual(selected[0][6], h.base)

    def test_missing_record_pin_and_off_lineage_record_fail_closed(self):
        for case in ("missing", "off-lineage"):
            with self.subTest(case=case):
                h = self.history()
                if case == "missing":
                    p = h.repo / "scripts/ci/f13_guarded_correction.py"
                    p.write_text(p.read_text().replace(h.record_commit, "0" * 40))
                    self.assert_rejected(h)
                else:
                    self.assert_rejected(h, target=h.correction)

    def test_wrong_record_bindings_and_incomplete_entries_fail_closed(self):
        cases = ("owner", "source", "donor", "before", "after", "mode", "type", "duplicate", "empty", "unknown", "prerequisite")
        for case in cases:
            with self.subTest(case=case):
                h = self.history()
                if case in ("owner", "source", "donor"):
                    key = {"owner": "prior_owner", "source": "prior_source", "donor": "donor"}[case]
                    h.record[key] = h.base
                elif case in ("before", "after"):
                    h.record["corrections"][0][case] = None
                elif case in ("mode", "type"):
                    h.record["corrections"][0]["after"][case] = "120000" if case == "mode" else "commit"
                elif case == "duplicate":
                    h.record["corrections"].append(dict(h.record["corrections"][0]))
                elif case == "empty":
                    h.record["corrections"] = []
                elif case == "unknown":
                    h.record["authority_override"] = True
                else:
                    h.record["prerequisites"][0]["entry"]["oid"] = "0" * 40
                h.pin_record()
                self.assert_rejected(h)

    def test_complete_delta_rejects_unrecorded_guarded_or_unguarded_changes(self):
        h = self.history(extra_correction=True)
        self.assert_rejected(h)

    def test_complete_mixed_set_preserves_new_path_own_history(self):
        h = self.history(recorded_extra=True)
        r, rows, _ = h.collect()
        self.assertEqual(r.returncode, 0, r.stderr)
        new = next(row for row in rows if row[0] == "z-new.txt")
        old = next(row for row in rows if row[0] == h.path)
        self.assertEqual(new[6], h.owner)
        self.assertEqual(old[6], h.base)
        h.record["corrections"].reverse()
        h.pin_record()
        self.assert_rejected(h)

    def test_working_tree_record_is_not_authority_and_wrong_pin_rejects(self):
        h = self.history()
        h.write(RECORD_PATH, '{"caller": "invented authority"}\n')
        r, _, _ = h.collect()
        self.assertEqual(r.returncode, 0, r.stderr)
        p = h.repo / "scripts/ci/f13_guarded_correction.py"
        p.write_text(re.sub(r'^PINNED_RECORD_SHA256 = ".*"$',
                           'PINNED_RECORD_SHA256 = "' + "0" * 64 + '"', p.read_text(), flags=re.M))
        self.assert_rejected(h)

    def test_final_target_bytes_mode_type_and_prerequisite_must_be_retained(self):
        for case in ("bytes", "mode", "symlink", "up", "restore-old"):
            with self.subTest(case=case):
                h = self.history()
                p = h.repo / h.path
                if case == "bytes":
                    h.write(h.path, "unapproved later bytes\n")
                elif case == "mode":
                    p.chmod(0o755)
                elif case == "symlink":
                    p.unlink()
                    p.symlink_to("unapproved-target")
                elif case == "up":
                    h.write(UP, "changed prerequisite\n")
                else:
                    h.write(h.path, "unsafe\n")
                h.target = h.commit("unapproved guarded drift")
                self.assert_rejected(h)

    def test_wrong_event_does_not_select_new_authority(self):
        for event, ref in (("workflow_dispatch", "refs/heads/develop"), ("pull_request", "refs/pull/1/merge"), ("push", "refs/heads/master"), ("push", "refs/heads/other")):
            with self.subTest(event=event, ref=ref):
                self.assert_rejected(self.history(), event=event, ref=ref)

    def test_real_prior_map_rejects_missing_duplicate_or_forged_rows(self):
        h = self.history()
        r, _, temp = h.collect(target=h.owner)
        self.assertEqual(r.returncode, 0, r.stderr)
        original = (temp / "map").read_text()
        selected = next(line for line in original.splitlines() if line.split("\t")[0] == h.path)
        args = ["python3", str(h.repo / "scripts/ci/f13_guarded_correction.py"),
                "--repo", str(h.repo), "--base", h.base, "--target", h.target,
                "--path", h.path, "--correction", h.correction,
                "--owner-map", str(temp / "map"), "--event", "push", "--ref", "refs/heads/develop"]
        self.assertEqual(subprocess.run(args, capture_output=True).returncode, 0)
        for case in ("missing", "duplicate", "owner", "source", "base", "content", "late-content", "false-trigger"):
            with self.subTest(case=case):
                fields = selected.split("\t")
                if case == "missing":
                    value = ""
                elif case == "duplicate":
                    value = original + selected + "\n"
                else:
                    index = {"owner": 2, "source": 5, "base": 4, "content": 6, "late-content": 6, "false-trigger": 1}[case]
                    fields[index] = h.owner if case == "late-content" else "false" if case == "false-trigger" else "0" * 40
                    value = "\t".join(fields) + "\n"
                (temp / "map").write_text(value)
                self.assertEqual(subprocess.run(args, capture_output=True).returncode, 1)
        (temp / "map").write_text(original)
        self.assertNotEqual(subprocess.run(args + ["--record-commit", h.record_commit], capture_output=True).returncode, 0)


if __name__ == "__main__":
    unittest.main()
