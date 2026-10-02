import hashlib
import importlib.util
import os
from pathlib import Path
import tempfile
import subprocess
import sys
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("inspector", Path(__file__).with_name("inspector.py"))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


class InspectorTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(dir="/root")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.js = self.root / "jetstream" / "$G" / "streams" / "social_events"
        self.js.mkdir(parents=True)
        self.file = self.js / "meta.inf"
        self.file.write_bytes(b"fixture metadata, never print")
        (self.js / "credential.creds").write_bytes(b"sentinel not to be read")

    def test_readonly_hash_and_metadata(self):
        before = self.file.stat()
        rows = m.inventory(self.root)
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0]["sha256"], hashlib.sha256(b"fixture metadata, never print").hexdigest())
        self.assertEqual(self.file.stat(), before)

    def test_symlink_rejected(self):
        (self.js / "alias").symlink_to(self.file)
        with self.assertRaises(m.Unsafe):
            m.inventory(self.root)

    def test_denied_uid(self):
        with patch.object(m.os, "geteuid", return_value=1000):
            with self.assertRaises(m.Unsafe):
                m.require_root()

    def test_unstable_file(self):
        original = m.os.read
        def change(fd, count):
            result = original(fd, count)
            if result:
                self.file.write_bytes(b"changed")
            return result
        with patch.object(m.os, "read", side_effect=change):
            with self.assertRaises(m.Unsafe):
                m.inventory(self.root)

    def test_uncertain_proc_fails(self):
        with self.assertRaises((m.Unsafe, OSError)):
            m.no_open_handles(self.root, self.root / "missing")

    def test_writer_fails(self):
        proc = self.root / "proc"
        fd = proc / "123" / "fd"
        fd.mkdir(parents=True)
        (fd / "9").symlink_to(self.file)
        (proc / "123" / "maps").write_text("")
        (proc / "123" / "mountinfo").write_text(Path("/proc/self/mountinfo").read_text())
        with self.assertRaises(m.Unsafe):
            m.no_open_handles(self.root / "jetstream", proc)

    def test_mount_and_uncertain_kube_fail(self):
        with self.assertRaises(m.Unsafe):
            m.validate_kube({}, {}, {})
        pvc = {"metadata": {"name": m.PVC, "namespace": m.NAMESPACE, "uid": "fixture"},
               "status": {"phase": "Bound"}, "spec": {"volumeName": m.PV}}
        pv = {"metadata": {"name": m.PV}, "status": {"phase": "Bound"}, "spec": {
            "claimRef": {"name": m.PVC, "namespace": m.NAMESPACE, "uid": "fixture"},
            "local": {"path": str(m.SOURCE)}, "nodeAffinity": {"required": {
                "nodeSelectorTerms": [{"matchExpressions": [{"key": "kubernetes.io/hostname",
                    "operator": "In", "values": [m.NODE]}]}]}}}}
        pods = {"items": []}
        m.validate_kube(pvc, pv, pods)
        pods["items"].append({"metadata": {"namespace": m.NAMESPACE}, "spec": {
            "volumes": [{"persistentVolumeClaim": {"claimName": m.PVC}}]}})
        with self.assertRaises(m.Unsafe):
            m.validate_kube(pvc, pv, pods)

    def bootstrap(self, modified=False):
        public = self.root / "public.py"
        public.write_bytes(b"print('FIXTURE_EXECUTED')\n")
        pin = hashlib.sha256(public.read_bytes()).hexdigest()
        if modified:
            public.write_bytes(b"print('UNREVIEWED_EXECUTED')\n")
        template = Path(__file__).with_name("root-launch.template.sh").read_text()
        code = template.split("<<'PY'\n", 1)[1].rsplit("\nPY", 1)[0]
        # Only fixture paths, interpreter and pin are substituted. Execute the actual
        # launch implementation in a networkless disposable Linux container.
        code = code.replace("REVIEWED_INSPECTOR_SHA256", pin)
        code = code.replace("/home/pmd/voice-nats-preservation/20261002/inspector.py", str(public))
        code = code.replace("/usr/bin/python3.13", os.path.realpath(sys.executable))
        code = code.replace('dir="/root"', "dir=" + repr(str(self.root)))
        return subprocess.run([sys.executable, "-I", "-S", "-c", code],
                              env=m.ENV, capture_output=True, text=True, timeout=10)

    def test_copy_hash_rejects_modified_source(self):
        result = self.bootstrap(modified=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("COPIED_SCRIPT_SHA_MISMATCH_NO_EXECUTION", result.stderr)
        self.assertNotIn("EXECUTED", result.stdout)

    def test_bootstrap_executes_private_copy(self):
        result = self.bootstrap()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), "FIXTURE_EXECUTED")
        private = next(self.root.glob("voice-pvc-inspector-*"))
        self.assertEqual(private.stat().st_mode & 0o777, 0o700)
        self.assertEqual((private / "inspector.py").stat().st_mode & 0o777, 0o600)

    def test_selected_layout_excludes_secrets(self):
        self.assertFalse(m.selected(Path("jetstream/$G/credentials"), "meta.inf"))
        self.assertFalse(m.selected(Path("jetstream/$G/streams/S/msgs"), "seed.blk"))
        self.assertTrue(m.selected(Path("jetstream/$G/streams/S/msgs"), "1.blk"))
        self.assertTrue(m.selected(Path("jetstream/$G/streams/S/obs/C"), "o.dat"))
        self.assertFalse(m.selected(Path("jetstream/$G/streams/S/consumers/C"), "o.dat"))

    def test_mount_namespace_bind_fails(self):
        host = Path("/proc/self/mountinfo").read_text()
        device, coordinate, _ = m.source_mount(self.root)
        alias = "1 2 " + device + " " + coordinate + " /data rw - ext4 /dev/fake rw"
        def records(path):
            return host if str(path) == "/proc/self/mountinfo" else alias
        with patch.object(m.Path, "iterdir", return_value=[Path("/proc/123")]), patch.object(
                m.Path, "read_text", autospec=True, side_effect=records):
            with self.assertRaisesRegex(m.Unsafe, "SOURCE_MOUNTED"):
                m.no_mounts(self.root)

    def test_separate_var_lib_filesystem_bind_fails(self):
        source = Path("/var/lib/rancher/k3s/storage/pvc_fixture")
        source_stat = self.root.stat()
        device = str(os.major(source_stat.st_dev)) + ":" + str(os.minor(source_stat.st_dev))
        host = "1 2 " + device + " / /var/lib rw - ext4 /dev/fake rw"
        alias = "3 2 " + device + " /rancher/k3s/storage/pvc_fixture /data rw - ext4 /dev/fake rw"
        def records(path):
            return host if str(path) == "/proc/self/mountinfo" else alias
        with patch.object(m.Path, "iterdir", return_value=[Path("/proc/123")]), patch.object(
                m.Path, "read_text", autospec=True, side_effect=records), patch.object(
                m.Path, "stat", return_value=source_stat):
            with self.assertRaisesRegex(m.Unsafe, "SOURCE_MOUNTED"):
                m.no_mounts(source)

    def test_covering_ancestor_bind_fails(self):
        device, coordinate, _ = m.source_mount(self.root)
        host = Path("/proc/self/mountinfo").read_text()
        alias = "1 2 " + device + " " + str(Path(coordinate).parent) + " /data rw - ext4 /dev/fake rw"
        def records(path):
            return host if str(path) == "/proc/self/mountinfo" else alias
        with patch.object(m.Path, "iterdir", return_value=[Path("/proc/123")]), patch.object(
                m.Path, "read_text", autospec=True, side_effect=records):
            with self.assertRaisesRegex(m.Unsafe, "SOURCE_MOUNTED"):
                m.no_mounts(self.root)

    def test_fd_and_maps_aliases_fail(self):
        source = self.root / "jetstream"
        device, coordinate, _ = m.source_mount(source)
        proc = self.root / "proc"
        fd = proc / "123" / "fd"
        fd.mkdir(parents=True)
        (proc / "123" / "mountinfo").write_text(
            "1 2 " + device + " " + coordinate + " /data rw - ext4 /dev/fake rw")
        (fd / "9").symlink_to("/data/$G/streams/social_events/meta.inf")
        (proc / "123" / "maps").write_text("")
        with self.assertRaisesRegex(m.Unsafe, "OPEN_SOURCE_HANDLE"):
            m.no_open_handles(source, proc)
        (fd / "9").unlink()
        (proc / "123" / "maps").write_text(
            "1-2 rw-p 0 00:00 1 /data/$G/streams/social_events/meta.inf\n")
        with self.assertRaisesRegex(m.Unsafe, "MAPPED_SOURCE"):
            m.no_open_handles(source, proc)

    def test_uncertain_mount_coordinate_fails(self):
        with patch.object(m.Path, "read_text", return_value="not valid mountinfo"):
            with self.assertRaisesRegex(m.Unsafe, "MOUNTINFO_UNCERTAIN"):
                m.no_mounts(self.root)

    def test_mount_exactly_at_source_fails(self):
        source_stat = self.root.stat()
        device = str(os.major(source_stat.st_dev)) + ":" + str(os.minor(source_stat.st_dev))
        record = "1 2 " + device + " / " + str(self.root) + " rw - ext4 /dev/fake rw"
        with patch.object(m.Path, "iterdir", return_value=[Path("/proc/123")]), patch.object(
                m.Path, "read_text", return_value=record):
            with self.assertRaisesRegex(m.Unsafe, "SOURCE_MOUNTED"):
                m.no_mounts(self.root)

    def test_actual_consumer_state_layout_is_inventoried(self):
        consumer = self.js / "obs" / "consumer_fixture"
        consumer.mkdir(parents=True)
        state = consumer / "o.dat"
        state.write_bytes(b"synthetic consumer state")
        before = state.stat()
        rows = m.inventory(self.root)
        row = next((r for r in rows if r["path"].endswith("obs/consumer_fixture/o.dat")), None)
        self.assertIsNotNone(row)
        self.assertEqual(row["sha256"], hashlib.sha256(b"synthetic consumer state").hexdigest())
        self.assertEqual(state.stat(), before)


if __name__ == "__main__":
    unittest.main()
