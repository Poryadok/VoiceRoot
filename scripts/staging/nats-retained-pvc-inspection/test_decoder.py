import importlib.util
import hashlib
import json
import os
from contextlib import ExitStack, redirect_stdout, redirect_stderr
import io
from pathlib import Path
import struct
import tempfile
import subprocess
import sys
import types
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("decoder", Path(__file__).with_name("decoder.py"))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
guard_spec = importlib.util.spec_from_file_location("guard", Path(__file__).with_name("inspector.py"))
g = importlib.util.module_from_spec(guard_spec)
guard_spec.loader.exec_module(g)


def uv(value):
    result = bytearray()
    while value >= 128:
        result.append((value & 127) | 128)
        value >>= 7
    result.append(value)
    return bytes(result)


class DecoderTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(dir="/root")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.relative = "jetstream/AFIXTURE/streams/S/meta.inf"
        self.file = self.root / self.relative
        self.file.parent.mkdir(parents=True)
        self.file.write_bytes(b'{"storage":"file","description":"PRIVATE_SENTINEL"}')
        s = self.file.stat()
        self.row = {"path": self.relative, "size": s.st_size, "mtime_ns": s.st_mtime_ns,
                    "ctime_ns": s.st_ctime_ns, "inode": s.st_ino, "device": s.st_dev,
                    "sha256": hashlib.sha256(self.file.read_bytes()).hexdigest()}

    def test_metadata_projection_never_emits_private_strings(self):
        raw = json.dumps({"name": "PRIVATE_SENTINEL", "description": "PRIVATE_SENTINEL",
                          "subjects": ["private.id.PRIVATE_SENTINEL"], "retention": "limits",
                          "storage": "file", "max_msgs": 42}).encode()
        result = m.project_meta(raw, "stream")
        self.assertEqual(result["fields"]["retention"], "limits")
        self.assertEqual(result["subject_count"], 1)
        self.assertNotIn("PRIVATE_SENTINEL", json.dumps(result))

    def test_unknown_enum_duplicate_keys_and_malformed_json_are_unsupported(self):
        for raw in (b'{"storage":"PRIVATE_SENTINEL"}', b'{"max_msgs":1,"max_msgs":2}',
                    b'{"private_future_field":"PRIVATE_SENTINEL"}', b"encrypted bytes"):
            result = m.project_meta(raw, "stream")
            self.assertTrue(result["status"].startswith("UNSUPPORTED"))
            self.assertNotIn("PRIVATE_SENTINEL", json.dumps(result))

    def test_consumer_state_v2_numeric_projection(self):
        raw = bytes((22, 2)) + b"".join(uv(n) for n in (2, 4, 3, 8, 0, 0))
        result = m.consumer_state(raw)
        self.assertEqual(result["ack_floor_stream"], 4)
        self.assertEqual(result["delivered_stream"], 8)
        self.assertEqual(result["pending_count"], 0)

    def test_consumer_state_v1_adjustment(self):
        raw = bytes((22, 1)) + b"".join(uv(n) for n in (2, 4, 3, 8, 0, 0))
        result = m.consumer_state(raw)
        self.assertEqual(result["delivered_consumer"], 4)
        self.assertEqual(result["delivered_stream"], 11)

    def test_state_unknown_truncated_overflow_and_trailing_are_unsupported(self):
        valid = bytes((22, 2)) + b"\0" * 6
        for raw in (b"", b"encrypted", bytes((22, 3)), valid[:-1], valid + b"x",
                    bytes((22, 2)) + b"\xff" * 11):
            self.assertNotIn("ack_floor_stream", m.consumer_state(raw))

    def test_block_header_is_noncanonical_and_never_payload_output(self):
        raw = struct.pack("<IQQH", 46, 7, 123, 0) + b"PRIVATE_SENTINEL!" + b"0" * 8
        result = m.block_header(raw)
        self.assertEqual(result["status"], "HEADER_IDENTIFICATION_ONLY")
        self.assertFalse(result["canonical_count"])
        self.assertNotIn("PRIVATE_SENTINEL", json.dumps(result))

    def test_block_deleted_and_unknown_are_explicit(self):
        for flag in (1 << 63, 1 << 62):
            raw = struct.pack("<IQQH", 30, flag | 7, 123, 0) + b"0" * 8
            self.assertTrue(m.block_header(raw)["deletion_semantics_unknown"])
        self.assertTrue(m.block_header(b"cmpcompressed")["status"].startswith("UNSUPPORTED"))

    def test_checked_read_preserves_source_and_rejects_inode_or_hash_mismatch(self):
        before = g.fingerprint(self.file.stat())
        root_fd = g.open_read(self.root, True)
        try:
            self.assertEqual(m.checked_read(g, root_fd, self.row), b'{"storage":"file","description":"PRIVATE_SENTINEL"}')
            for key, value in (("inode", self.row["inode"] + 1), ("sha256", "0" * 64), ("size", self.row["size"] + 1)):
                with self.subTest(key=key), self.assertRaises(g.Unsafe):
                    m.checked_read(g, root_fd, {**self.row, key: value})
        finally:
            os.close(root_fd)
        self.assertEqual(g.fingerprint(self.file.stat()), before)

    def test_checked_read_rejects_symlink_and_live_replacement(self):
        root_fd = g.open_read(self.root, True)
        try:
            real_read = g.os.read
            mutated = False
            def read(fd, count):
                nonlocal mutated
                result = real_read(fd, count)
                if not mutated:
                    mutated = True
                    self.file.unlink()
                    self.file.write_bytes(b"PRIVATE_SENTINEL")
                return result
            with patch.object(g.os, "read", side_effect=read), self.assertRaises(g.Unsafe):
                m.checked_read(g, root_fd, self.row)
            self.file.unlink()
            self.file.symlink_to("/etc/passwd")
            with self.assertRaises((g.Unsafe, OSError)):
                m.checked_read(g, root_fd, self.row)
        finally:
            os.close(root_fd)

    def test_protected_report_mutation_and_symlink_fail_closed(self):
        report = self.root / "metadata.json"
        report.write_bytes(b"{}"); report.chmod(0o440)
        digest = hashlib.sha256(b"{}").hexdigest()
        with patch.object(m, "REPORT_BYTES", 2), patch.object(m, "REPORT_SHA", digest), patch.object(m, "REPORT_GID", 0):
            self.assertEqual(m.protected_report(g, report), b"{}")
            report.chmod(0o640)
            with self.assertRaises(g.Unsafe):
                m.protected_report(g, report)
            report.chmod(0o440)
            with patch.object(m, "REPORT_SHA", "0" * 64), self.assertRaises(g.Unsafe):
                m.protected_report(g, report)
            report.unlink(); report.symlink_to(self.file)
            with self.assertRaises((g.Unsafe, OSError)):
                m.protected_report(g, report)

    def test_checked_read_rejects_ancestor_replacement(self):
        root_fd = g.open_read(self.root, True)
        real_read = g.os.read
        changed = False
        def read(fd, count):
            nonlocal changed
            raw = real_read(fd, count)
            if not changed:
                changed = True
                directory = self.file.parent
                directory.rename(directory.with_name("old"))
                directory.mkdir()
                self.file.write_bytes(raw)
            return raw
        try:
            with patch.object(g.os, "read", side_effect=read), self.assertRaises(g.Unsafe):
                m.checked_read(g, root_fd, self.row)
        finally:
            os.close(root_fd)

    def test_observations_have_no_raw_paths_or_private_values(self):
        result = m.observations([self.row], lambda row: self.file.read_bytes())
        self.assertNotIn("PRIVATE_SENTINEL", json.dumps(result))
        self.assertNotIn("AFIXTURE", json.dumps(result))
        self.assertEqual(result[0]["row_index"], 0)
        self.assertEqual(result[0]["path_sha256"], hashlib.sha256(self.relative.encode()).hexdigest())

    def test_duplicate_pending_state_rejected(self):
        # v2 floor0/0 delivered4/4,2pending,min_timestamp0;
        # duplicate stream delta1/delivery1/timestamp0; no redelivery.
        raw = bytes((22, 2)) + b"".join(uv(n) for n in (0, 0, 4, 4, 2, 0, 1, 1, 0, 1, 1, 0, 0))
        self.assertTrue(m.consumer_state(raw)["status"].startswith("UNSUPPORTED"))

    def fake_authority(self):
        return {"status": "READONLY_FILE_INVENTORY_COMPLETE", "preservation_pass": False, "canonical_census": False,
                "pv": g.PV, "pvc": g.PVC, "node": g.NODE, "source": str(self.root), "time_utc": "fixture",
                "pvc_uid": "fixture-pvc", "pv_uid": "fixture-pv", "files": [self.row]}

    def test_manifest_schema_and_bound_cannot_be_overridden(self):
        with patch.object(g, "SOURCE", self.root), patch.object(m, "FILE_COUNT", 1):
            authority = self.fake_authority()
            self.assertEqual(m.manifest(json.dumps(authority).encode(), g)["files"], [self.row])
            for update in ({"preservation_pass": True}, {"node": "OTHER"}, {"files": [self.row, self.row]},
                           {"files": [{**self.row, "path": "../private.creds"}]}):
                with self.subTest(update=next(iter(update))), self.assertRaises(g.Unsafe):
                    m.manifest(json.dumps({**authority, **update}).encode(), g)

    def test_main_guard_veto_stops_source_read(self):
        output = self.root / "output"
        output.mkdir()
        with ExitStack() as stack:
            for name, value in (("SOURCE", self.root), ("CONFIG", self.root), ("K3S", self.root), ("OUTPUT", output)):
                stack.enter_context(patch.object(g, name, value))
            stack.enter_context(patch.object(m, "load_guard", return_value=g))
            stack.enter_context(patch.object(m, "FILE_COUNT", 1))
            stack.enter_context(patch.object(m, "protected_report", return_value=json.dumps(self.fake_authority()).encode()))
            stack.enter_context(patch.object(g.socket, "gethostname", return_value=g.NODE))
            stack.enter_context(patch.object(g, "kube_state", return_value=[{"metadata": {"uid": "fixture-pvc"}}, {"metadata": {"uid": "fixture-pv"}}]))
            stack.enter_context(patch.object(g, "no_mounts", side_effect=g.Unsafe("SOURCE_MOUNTED")))
            reader = stack.enter_context(patch.object(m, "checked_read"))
            with self.assertRaisesRegex(g.Unsafe, "SOURCE_MOUNTED"):
                m.main()
            reader.assert_not_called()

    def test_main_publishes_only_guarded_synthetic_observations(self):
        output = self.root / "output"
        output.mkdir()
        before = g.fingerprint(self.file.stat())
        with ExitStack() as stack:
            for name, value in (("SOURCE", self.root), ("CONFIG", self.root), ("K3S", self.root), ("OUTPUT", output)):
                stack.enter_context(patch.object(g, name, value))
            stack.enter_context(patch.object(m, "load_guard", return_value=g))
            stack.enter_context(patch.object(m, "FILE_COUNT", 1))
            stack.enter_context(patch.object(m, "protected_report", return_value=json.dumps(self.fake_authority()).encode()))
            stack.enter_context(patch.object(g.socket, "gethostname", return_value=g.NODE))
            stack.enter_context(patch.object(g.grp, "getgrnam", return_value=types.SimpleNamespace(gr_gid=0)))
            stack.enter_context(patch.object(g, "kube_state", return_value=[{"metadata": {"uid": "fixture-pvc"}}, {"metadata": {"uid": "fixture-pv"}}]))
            mounts = stack.enter_context(patch.object(g, "no_mounts"))
            handles = stack.enter_context(patch.object(g, "no_open_handles"))
            printed = io.StringIO()
            with redirect_stdout(printed):
                m.main()
            self.assertEqual(mounts.call_count, 3)
            self.assertEqual(handles.call_count, 3)
        target = Path(printed.getvalue().strip())
        raw = target.read_text()
        self.assertNotIn("PRIVATE_SENTINEL", raw)
        self.assertNotIn("AFIXTURE", raw)
        report = json.loads(raw)
        self.assertFalse(report["preservation_pass"])
        self.assertFalse(report["canonical_census"])
        self.assertIsNone(report["logical_message_count"])
        self.assertEqual(target.stat().st_mode & 0o777, 0o440)
        self.assertEqual(target.parent.stat().st_mode & 0o777, 0o750)
        self.assertEqual(g.fingerprint(self.file.stat()), before)

    def test_census_rejects_oversized_selected_bytes_without_reading_them(self):
        self.file.write_bytes(b"x" * (m.MAX_BYTES + 1))
        with patch.object(g, "SOURCE", self.root), self.assertRaises(g.Unsafe):
            m.source_census(g)

    def test_report_parent_symlink_cannot_supply_authority(self):
        report = self.root / "metadata.json"
        report.write_bytes(b"{}"); report.chmod(0o440)
        alias = self.root / "alias"
        alias.symlink_to(self.root, target_is_directory=True)
        with patch.object(m, "REPORT_BYTES", 2), patch.object(m, "REPORT_SHA", hashlib.sha256(b"{}").hexdigest()), patch.object(m, "REPORT_GID", 0):
            with self.assertRaises(g.Unsafe):
                m.protected_report(g, alias / "metadata.json")

    def test_launcher_executes_only_two_pinned_private_copies(self):
        template = Path(__file__).with_name("decoder-launch.template.sh").read_text()
        code = template.split("<<'PY'\n", 1)[1].rsplit("\nPY", 1)[0]
        guard = self.root / "inspector.py"
        decoder = self.root / "decoder.py"
        guard.write_bytes(b"# public fixture guard\n")
        decoder.write_bytes(b"print('PUBLIC_FIXTURE_EXECUTED')\n")
        code = code.replace("REVIEWED_DECODER_SHA256", hashlib.sha256(decoder.read_bytes()).hexdigest())
        code = code.replace(m.INSPECTOR_SHA, hashlib.sha256(guard.read_bytes()).hexdigest())
        code = code.replace("/home/pmd/voice-nats-preservation/20261002/", str(self.root) + "/")
        code = code.replace("/usr/bin/python3.13", os.path.realpath(sys.executable))
        code = code.replace('dir="/root"', "dir=" + repr(str(self.root)))
        result = subprocess.run([sys.executable, "-I", "-S", "-c", code], capture_output=True, text=True, timeout=10, env=g.ENV)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), "PUBLIC_FIXTURE_EXECUTED")
        private = next(self.root.glob("voice-retained-decoder-*"))
        self.assertEqual(private.stat().st_mode & 0o777, 0o700)
        for name in ("inspector.py", "decoder.py"):
            self.assertEqual((private / name).stat().st_mode & 0o777, 0o600)
        decoder.write_bytes(b"print('PRIVATE_SENTINEL')\n")
        result = subprocess.run([sys.executable, "-I", "-S", "-c", code], capture_output=True, text=True, timeout=10, env=g.ENV)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("PRIVATE_SENTINEL", result.stdout)

    def test_cli_never_prints_private_exception_text_or_path(self):
        for error in (g.Unsafe("PRIVATE_SENTINEL"), FileNotFoundError(2, "PRIVATE_SENTINEL", "/private/secret.creds")):
            output = io.StringIO()
            with patch.object(m, "main", side_effect=error), redirect_stderr(output):
                self.assertEqual(m.run_cli(), 1)
            self.assertNotIn("PRIVATE_SENTINEL", output.getvalue())
            self.assertNotIn("/private", output.getvalue())


if __name__ == "__main__":
    unittest.main()
