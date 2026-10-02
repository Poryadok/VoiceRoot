import hashlib
from contextlib import contextmanager, redirect_stderr
import io
import mmap
import signal
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
    def exited_process(self):
        pid = os.fork()
        if pid == 0:
            os._exit(0)
        os.waitpid(pid, 0)
        return Path("/proc/" + str(pid))

    @contextmanager
    def jailed_process(self, close_source_fds=False):
        jail = self.root / "jail"
        jail.mkdir(exist_ok=True)
        ready_r, ready_w = os.pipe()
        stop_r, stop_w = os.pipe()
        pid = os.fork()
        if pid == 0:
            os.close(ready_r); os.close(stop_w)
            if close_source_fds:
                for fd in Path("/proc/self/fd").iterdir():
                    try:
                        if os.readlink(fd) == str(self.file):
                            os.close(int(fd.name))
                    except FileNotFoundError:
                        pass  # The scandir descriptor already closed.
            os.chroot(jail); os.chdir("/")
            os.write(ready_w, b"R")
            os.read(stop_r, 1)
            os._exit(0)
        os.close(ready_w); os.close(stop_r)
        try:
            self.assertEqual(os.read(ready_r, 1), b"R")
            self.assertEqual(Path("/proc/" + str(pid) + "/mountinfo").read_text(), "")
            yield pid
        finally:
            try:
                os.write(stop_w, b"X")
            except BrokenPipeError:
                pass  # A disappearance test already ended the fixture child.
            os.close(stop_w); os.close(ready_r)
            try:
                os.waitpid(pid, 0)
            except ChildProcessError:
                pass

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
                m.Path, "read_text", autospec=True, side_effect=records), patch.object(
                m.Path, "read_bytes", autospec=True, side_effect=lambda p: records(p).encode()):
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
                m.Path, "stat", return_value=source_stat), patch.object(
                m.Path, "read_bytes", autospec=True, side_effect=lambda p: records(p).encode()):
            with self.assertRaisesRegex(m.Unsafe, "SOURCE_MOUNTED"):
                m.no_mounts(source)

    def test_covering_ancestor_bind_fails(self):
        device, coordinate, _ = m.source_mount(self.root)
        host = Path("/proc/self/mountinfo").read_text()
        alias = "1 2 " + device + " " + str(Path(coordinate).parent) + " /data rw - ext4 /dev/fake rw"
        def records(path):
            return host if str(path) == "/proc/self/mountinfo" else alias
        with patch.object(m.Path, "iterdir", return_value=[Path("/proc/123")]), patch.object(
                m.Path, "read_text", autospec=True, side_effect=records), patch.object(
                m.Path, "read_bytes", autospec=True, side_effect=lambda p: records(p).encode()):
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

    def test_valid_network_namespace_object_mount(self):
        record = "370 28 0:4 net:[4026532754] /run/netns/fixture rw shared:336 - nsfs nsfs rw"
        with patch.object(m.Path, "read_text", return_value=record):
            self.assertEqual(m.mount_records(Path("fixture")),
                             [("0:4", "net:[4026532754]", "/run/netns/fixture")])

    def test_namespace_unknown_or_malformed_records_fail(self):
        for record in (
            "370 28 0:4 net:[invalid] /run/netns/fixture rw - nsfs nsfs rw",
            "370 28 0:4 unknown:[123] /run/netns/fixture rw - nsfs nsfs rw",
            "370 28 259:7 net:[123] /run/netns/fixture rw - nsfs nsfs rw",
            "370 28 0:4 net:[123] /run/netns/fixture rw - ext4 /dev/fake rw",
            "370 28 0:4 / /run/netns/fixture rw - nsfs",
        ):
            with self.subTest(record=record), patch.object(m.Path, "read_text", return_value=record):
                with self.assertRaisesRegex(m.Unsafe, "MOUNTINFO_UNCERTAIN"):
                    m.mount_records(Path("fixture"))

    def test_namespace_object_mount_at_source_still_fails(self):
        host = Path("/proc/self/mountinfo").read_text()
        alias = "370 28 0:4 net:[4026532754] " + str(self.root) + " rw - nsfs nsfs rw"
        def records(path):
            return host if str(path) == "/proc/self/mountinfo" else alias
        with patch.object(m.Path, "iterdir", return_value=[Path("/proc/123")]), patch.object(
                m.Path, "read_text", autospec=True, side_effect=records), patch.object(
                m.Path, "read_bytes", autospec=True, side_effect=lambda p: records(p).encode()):
            with self.assertRaisesRegex(m.Unsafe, "SOURCE_MOUNTED"):
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

    def test_live_chroot_empty_mount_view_remains_checked(self):
        real_iterdir = m.Path.iterdir
        with self.jailed_process() as pid:
            with patch.object(m.Path, "iterdir", autospec=True,
                              side_effect=lambda p: [Path("/proc/" + str(pid))] if str(p) == "/proc" else list(real_iterdir(p))):
                m.no_mounts(self.js)
            m.no_open_handles(self.js)

    def test_chroot_inherited_source_fd_is_rejected(self):
        with self.file.open("rb"):
            with self.jailed_process():
                with self.assertRaisesRegex(m.Unsafe, "OPEN_SOURCE_HANDLE"):
                    m.no_open_handles(self.js)

    def test_chroot_closed_fd_source_mapping_is_rejected(self):
        with self.file.open("rb") as stream:
            mapping = mmap.mmap(stream.fileno(), 0, access=mmap.ACCESS_READ)
        try:
            with self.jailed_process(close_source_fds=True):
                with self.assertRaisesRegex(m.Unsafe, "MAPPED_SOURCE"):
                    m.no_open_handles(self.js)
        finally:
            mapping.close()

    def test_failure_category_never_leaks_exception_text(self):
        for error in (m.Unsafe("PRIVATE_SEED_SENTINEL"), OSError(13, "PRIVATE_SEED_SENTINEL")):
            self.assertNotIn("PRIVATE_SEED_SENTINEL", str(m.safe_failure(error)))

    def test_empty_view_different_namespace_and_missing_peer_fail(self):
        real_identity = m.process_identity
        with self.jailed_process() as pid:
            process = Path("/proc/" + str(pid))
            def changed_identity(path):
                value = real_identity(path)
                return value[:3] + (value[3], value[4] + 1) + value[5:] if path == process else value
            with patch.object(m, "process_identity", side_effect=changed_identity):
                with self.assertRaises(m.Unsafe), m.process_mount_view(process):
                    self.fail("different namespace accepted")
            def no_peer(path):
                if path == Path("/proc/1"):
                    raise PermissionError(13, "PRIVATE_SEED_SENTINEL")
                return real_identity(path)
            with patch.object(m, "process_identity", side_effect=no_peer):
                with self.assertRaises(PermissionError), m.process_mount_view(process):
                    self.fail("missing peer accepted")

    def test_empty_view_fresh_identity_change_after_scan_fails(self):
        real_identity = m.process_identity
        with self.jailed_process() as pid:
            process = Path("/proc/" + str(pid))
            for changed_path in (process, Path("/proc/1"), Path("/proc") / str(os.getpid())):
                changed = False
                def identity(path):
                    value = real_identity(path)
                    return value[:-1] + (value[-1] + 1,) if changed and path == changed_path else value
                with self.subTest(process=str(changed_path)), patch.object(m, "process_identity", side_effect=identity):
                    with self.assertRaisesRegex(m.Unsafe, "PROCESS_IDENTITY_CHANGED"):
                        with m.process_mount_view(process):
                            changed = True

    def test_empty_view_peer_mount_bytes_change_after_scan_fails(self):
        real_read = m.Path.read_bytes
        with self.jailed_process() as pid:
            process = Path("/proc/" + str(pid))
            for changed_path in (process, Path("/proc/1"), Path("/proc") / str(os.getpid())):
                changed = False
                def read(path):
                    value = real_read(path)
                    return value + b"\n" if changed and path == changed_path / "mountinfo" else value
                with self.subTest(process=str(changed_path)), patch.object(m.Path, "read_bytes", autospec=True, side_effect=read):
                    with self.assertRaisesRegex(m.Unsafe, "MOUNT_VIEW_CHANGED"):
                        with m.process_mount_view(process):
                            changed = True

    def test_cli_reports_static_category_and_numeric_context(self):
        error = m.Unsafe("OPEN_SOURCE_HANDLE")
        error.pid = 123
        output = io.StringIO()
        with patch.object(m, "main", side_effect=error), redirect_stderr(output):
            self.assertEqual(m.run_cli(), 1)
        self.assertIn('"code": "OPEN_SOURCE_HANDLE"', output.getvalue())
        self.assertIn('"pid": 123', output.getvalue())
        error = OSError(13, "PRIVATE_SEED_SENTINEL", "/private/credential.creds")
        output = io.StringIO()
        with patch.object(m, "main", side_effect=error), redirect_stderr(output):
            self.assertEqual(m.run_cli(), 1)
        self.assertIn('"errno": 13', output.getvalue())
        self.assertNotIn("PRIVATE_SEED_SENTINEL", output.getvalue())
        self.assertNotIn("/private", output.getvalue())

    def test_verified_exit_restarts_entire_process_scan(self):
        gone = self.exited_process()
        live = Path("/proc") / str(os.getpid())
        checked = []
        def check(process):
            checked.append(process)
            with m.process_mount_view(process):
                pass
        with patch.object(m.Path, "iterdir", side_effect=[[live, gone], [live]]):
            m.scan_processes(Path("/proc"), check)
        self.assertEqual(checked, [live, gone, live])

    def test_process_exit_restarts_are_bounded(self):
        gone = self.exited_process()
        with patch.object(m.Path, "iterdir", return_value=[gone]) as listing:
            with self.assertRaisesRegex(m.Unsafe, "PROCESS_SCAN_UNSTABLE"):
                m.scan_processes(Path("/proc"), lambda p: m.proc_mount_bytes(p))
        self.assertEqual(listing.call_count, 3)

    def test_live_missing_proc_reference_and_guard_do_not_retry(self):
        live = Path("/proc") / str(os.getpid())
        for error in (FileNotFoundError(2, "PRIVATE_SEED_SENTINEL"),
                      PermissionError(13, "PRIVATE_SEED_SENTINEL"), m.Unsafe("OPEN_SOURCE_HANDLE")):
            with self.subTest(error=type(error).__name__), patch.object(m.Path, "iterdir", return_value=[live]) as listing:
                def check(process):
                    raise error
                with self.assertRaises(type(error)):
                    m.scan_processes(Path("/proc"), check)
                self.assertEqual(listing.call_count, 1)

    def test_initial_mount_read_reports_pid_and_static_operation(self):
        gone = self.exited_process()
        with self.assertRaises(FileNotFoundError) as caught:
            with m.process_mount_view(gone):
                pass
        diagnostic = m.safe_failure(caught.exception)
        self.assertEqual(diagnostic["pid"], int(gone.name))
        self.assertEqual(diagnostic["operation"], "PROC_MOUNT_TABLE")

    def test_unrelated_enoent_never_restarts_a_gone_pid(self):
        gone = self.exited_process()
        for filename in (None, "/private/other-file", "/proc/1/root"):
            error = FileNotFoundError(2, "PRIVATE_SEED_SENTINEL", filename)
            with self.subTest(filename=filename), patch.object(m.Path, "iterdir", return_value=[gone]) as listing:
                def check(process):
                    raise error
                with self.assertRaises(FileNotFoundError):
                    m.scan_processes(Path("/proc"), check)
                self.assertEqual(listing.call_count, 1)

    def test_live_missing_root_or_namespace_never_restarts(self):
        real_stat = m.Path.stat
        with self.jailed_process() as pid:
            process = Path("/proc/" + str(pid))
            for reference in ("root", "ns/mnt"):
                def missing(path, *args, **kwargs):
                    if path == process / reference:
                        raise FileNotFoundError(2, "PRIVATE_SEED_SENTINEL")
                    return real_stat(path, *args, **kwargs)
                with self.subTest(reference=reference), patch.object(m.Path, "stat", autospec=True, side_effect=missing), patch.object(
                        m.Path, "iterdir", return_value=[process]) as listing:
                    def check(path):
                        with m.process_mount_view(path):
                            self.fail("live missing reference accepted")
                    with self.assertRaises(FileNotFoundError):
                        m.scan_processes(Path("/proc"), check)
                    self.assertEqual(listing.call_count, 1)

    def test_original_data_veto_survives_real_exit_during_unwind(self):
        for code in ("OPEN_SOURCE_HANDLE", "MAPPED_SOURCE", "SOURCE_MOUNTED"):
            with self.subTest(code=code), self.jailed_process() as pid:
                process = Path("/proc/" + str(pid))
                original = m.Unsafe(code)
                def check(path):
                    with m.process_mount_view(path):
                        os.kill(pid, signal.SIGKILL)
                        os.waitpid(pid, 0)
                        raise original
                with patch.object(m.Path, "iterdir", return_value=[process]) as listing:
                    with self.assertRaises(m.Unsafe) as caught:
                        m.scan_processes(Path("/proc"), check)
                self.assertIs(caught.exception, original)
                self.assertEqual(listing.call_count, 1)


if __name__ == "__main__":
    unittest.main()
