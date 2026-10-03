import importlib.util
import hashlib
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from contextlib import redirect_stdout
import types
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("locator", Path(__file__).with_name("locator.py"))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
UID = "11111111-1111-1111-1111-111111111111"


class LocatorTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(dir="/root")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def test_local_locator_preserves_private_contents_and_names(self):
        candidate = self.root / ("pvc-" + UID + "_voice-staging_voice-nats-jsdata")
        candidate.mkdir()
        payload = candidate / "secret-sentinel"
        payload.write_bytes(b"payload-private-sentinel")
        before = payload.stat()
        (self.root / "unrelated-private-sentinel").mkdir()
        report = m.scan(self.root, "LOCAL_PATH")
        self.assertEqual(len(report["candidates"]), 1)
        self.assertEqual(report["candidates"][0]["status"], "DIRECTORY_METADATA_ONLY")
        output = json.dumps(report)
        self.assertNotIn("sentinel", output)
        self.assertEqual(report["candidates"][0]["relative_name"], candidate.name)
        self.assertEqual(payload.stat(), before)

    def test_pod_exact_jsdata_shape_without_account_listing(self):
        js = self.root / UID / "volumes/kubernetes.io~empty-dir/jsdata"
        (js / "jetstream" / "private-account-sentinel").mkdir(parents=True)
        (self.root / "22222222-2222-2222-2222-222222222222").mkdir()
        report = m.scan(self.root, "POD_CANDIDATE")
        self.assertEqual(len(report["candidates"]), 1)
        self.assertTrue(report["candidates"][0]["jetstream_directory"])
        self.assertNotIn("private-account", json.dumps(report))
        self.assertEqual(report["candidates"][0]["pod_uid"], UID)

    def test_absence_never_claims_history_or_loss(self):
        report = m.scan(self.root / "missing", "POD_CANDIDATE")
        self.assertEqual(report["status"], "CANDIDATE_ROOT_ABSENT")
        self.assertNotIn("lost", json.dumps(report).lower())

    def test_symlink_parent_and_candidate_rejected_without_following(self):
        real = self.root / "private"
        real.mkdir()
        link = self.root / ("pvc-" + UID + "_voice-staging_voice-nats-jsdata")
        link.symlink_to(real)
        with self.assertRaises(m.Unsafe):
            m.scan(self.root, "LOCAL_PATH")
        with self.assertRaises(m.Unsafe):
            m.scan(link, "LOCAL_PATH")

    def test_fifo_rejected_without_opening_it(self):
        os.mkfifo(self.root / ("pvc-" + UID + "_voice-staging_voice-nats-jsdata"))
        with self.assertRaises(m.Unsafe):
            m.scan(self.root, "LOCAL_PATH")

    def test_fixed_entry_bound_even_for_unrelated_names(self):
        for name in ("unrelated1", "unrelated2", "unrelated3"):
            (self.root / name).mkdir()
        with patch.object(m, "MAX_ENTRIES", 2), self.assertRaises(m.Unsafe):
            m.scan(self.root, "LOCAL_PATH")

    def test_candidate_bound(self):
        for prefix in ("1", "2"):
            (self.root / ("pvc-" + prefix + UID[1:] + "_voice-staging_voice-nats-jsdata")).mkdir()
        with patch.object(m, "MAX_CANDIDATES", 1), self.assertRaises(m.Unsafe):
            m.scan(self.root, "LOCAL_PATH")

    def test_deadline_stops_scan(self):
        (self.root / "entry").mkdir()
        with self.assertRaises(m.Unsafe):
            m.scan(self.root, "LOCAL_PATH", deadline=0)

    def test_mount_candidate_veto_before_descent(self):
        candidate = self.root / ("pvc-" + UID + "_voice-staging_voice-nats-jsdata")
        candidate.mkdir()
        with self.assertRaises(m.Unsafe):
            m.scan(self.root, "LOCAL_PATH", mounts={str(candidate)})

    def test_intermediate_symlink_is_not_traversed(self):
        pod = self.root / UID
        pod.mkdir()
        other = self.root / "unrelated"
        other.mkdir()
        (pod / "volumes").symlink_to(other)
        with self.assertRaises(m.Unsafe):
            m.scan(self.root, "POD_CANDIDATE")

    def test_replaced_path_rejected_after_open(self):
        candidate = self.root / ("pvc-" + UID + "_voice-staging_voice-nats-jsdata")
        candidate.mkdir()
        original = m.opened
        def replace(*args, **kwargs):
            fd = original(*args, **kwargs)
            if args[2] == candidate.name:
                candidate.rename(self.root / "moved")
                candidate.mkdir()
            return fd
        with patch.object(m, "opened", side_effect=replace), self.assertRaises(m.Unsafe):
            m.scan(self.root, "LOCAL_PATH")

    def test_invalid_grammar_not_candidate(self):
        for name in ("../../escape", "pvc-INVALID_voice-staging_voice-nats", UID.upper() + "x"):
            if "/" not in name:
                (self.root / name).mkdir()
        self.assertEqual(m.scan(self.root, "LOCAL_PATH")["candidates"], [])

    def test_no_source_content_open_or_jetstream_account_enumeration(self):
        js = self.root / UID / "volumes/kubernetes.io~empty-dir/jsdata/jetstream"
        (js / "private-account").mkdir(parents=True)
        (js / "private-account/payload").write_bytes(b"never read")
        original_open, original_scan = os.open, os.scandir
        def directory_only(path, flags, *args, **kwargs):
            self.assertTrue(flags & os.O_DIRECTORY)
            return original_open(path, flags, *args, **kwargs)
        enumerated = []
        def root_only(fd):
            enumerated.append(os.fstat(fd).st_ino)
            return original_scan(fd)
        before = {p: p.stat() for p in (js, js / "private-account/payload")}
        with patch.object(os, "open", side_effect=directory_only), patch.object(os, "scandir", side_effect=root_only):
            m.scan(self.root, "POD_CANDIDATE")
        self.assertEqual(enumerated, [self.root.stat().st_ino])
        self.assertEqual({p: p.stat() for p in before}, before)

    def test_protected_report_and_main_provenance(self):
        private = self.root / "private"
        private.mkdir(mode=0o700)
        code = private / "locator.py"
        code.write_bytes(Path(m.__file__).read_bytes())
        code.chmod(0o600)
        output = self.root / "output"
        capture = io.StringIO()
        with patch.object(m, "__file__", str(code)), patch.object(m, "OUTPUT", output), \
             patch.object(m, "ROOTS", ((str(self.root / "absent"), "POD_CANDIDATE"),)), \
             patch.object(m.socket, "gethostname", return_value="pmdebook"), \
             patch.object(m.grp, "getgrnam", return_value=types.SimpleNamespace(gr_gid=1000)), \
             redirect_stdout(capture):
            m.main()
        target = Path(capture.getvalue().strip())
        report = json.loads(target.read_text())
        self.assertEqual(report["code_sha256"], hashlib.sha256(code.read_bytes()).hexdigest())
        self.assertFalse(report["preservation_pass"])
        self.assertFalse(report["historical_attribution_verified"])
        self.assertEqual(report["historical_completeness"], "UNKNOWN")
        self.assertEqual(report["source_content_reads"], 0)
        self.assertEqual(target.stat().st_mode & 0o777, 0o440)
        self.assertEqual(target.stat().st_uid, 0)
        self.assertEqual(target.stat().st_gid, 1000)
        self.assertEqual(target.parent.stat().st_mode & 0o777, 0o750)
        self.assertEqual(output.stat().st_mode & 0o777, 0o750)

    def test_wrong_node_cannot_scan_or_publish(self):
        with patch.object(m.socket, "gethostname", return_value="wrong-node"), \
             patch.object(m, "scan") as scan, self.assertRaises(m.Unsafe):
            m.main()
        scan.assert_not_called()

    def launcher(self, content, pin):
        public = self.root / "public"
        public.mkdir()
        source = public / "locator.py"
        if content is None:
            os.mkfifo(source)
        else:
            source.write_bytes(content)
        template = Path(__file__).with_name("locator-launch.template.sh").read_text()
        body = template.split("<<'PY'\n", 1)[1].rsplit("\nPY", 1)[0]
        body = body.replace("REVIEWED_LOCATOR_SHA256", pin).replace(
            "/home/pmd/voice-nats-preservation/20261003/", str(public) + "/").replace(
            'Path("/usr/bin/python3.13")', 'Path("/usr/local/bin/python3.12")')
        return body

    def test_launcher_captures_and_hashes_private_code_before_exec(self):
        content = b"print('fixture-only')\n"
        body = self.launcher(content, hashlib.sha256(content).hexdigest())
        private = self.root / "captured"
        private.mkdir(mode=0o700)
        with patch.object(tempfile, "mkdtemp", return_value=str(private)), patch.object(os, "execve") as launch:
            exec(compile(body, "<fixture-launcher>", "exec"), {})
        args = launch.call_args.args
        self.assertEqual(Path(args[1][-1]).read_bytes(), content)
        self.assertEqual(Path(args[1][-1]).stat().st_mode & 0o777, 0o600)
        self.assertEqual(args[1][1:3], ["-I", "-S"])
        self.assertNotIn("PYTHONPATH", args[2])

    def test_launcher_tamper_never_executes(self):
        body = self.launcher(b"tampered", "0" * 64)
        private = self.root / "captured"
        private.mkdir(mode=0o700)
        with patch.object(tempfile, "mkdtemp", return_value=str(private)), \
             patch.object(os, "execve") as launch, self.assertRaises(SystemExit):
            exec(compile(body, "<fixture-launcher>", "exec"), {})
        launch.assert_not_called()
        self.assertFalse((private / "locator.py").exists())

    def test_launcher_fifo_never_blocks_or_executes(self):
        body = self.launcher(None, "0" * 64)
        private = self.root / "captured"
        private.mkdir(mode=0o700)
        with patch.object(tempfile, "mkdtemp", return_value=str(private)), \
             patch.object(os, "execve") as launch, self.assertRaises(SystemExit):
            exec(compile(body, "<fixture-launcher>", "exec"), {})
        launch.assert_not_called()


if __name__ == "__main__":
    unittest.main()
