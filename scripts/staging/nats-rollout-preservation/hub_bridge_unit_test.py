import ast
from pathlib import Path
import stat
import types
import unittest
from unittest import mock
import hub_bridge_unit as unit
from controller import Blocked


class BridgeUnitTests(unittest.TestCase):
    def test_actual_fixed_readback_refuses_links_byte_and_path_drift_and_closes_fds(self):
        def row(mode, inode=1, nlink=1):
            return types.SimpleNamespace(st_dev=1, st_ino=inode, st_uid=0, st_gid=0,
                st_mode=mode, st_nlink=nlink, st_size=3, st_mtime_ns=1, st_ctime_ns=1)
        for change in (None, 'hardlink', 'bytes', 'path', 'permission'):
            opened, closed, reads = [], [], []
            before = row(stat.S_IFREG | (0o666 if change == 'permission' else 0o644), nlink=2 if change == 'hardlink' else 1)
            def opening(path, flags, **kwargs):
                opened.append((path, flags)); return len(opened)
            with mock.patch.object(unit.os, 'geteuid', return_value=0, create=True), \
                    mock.patch.object(unit.os, 'open', side_effect=opening), \
                    mock.patch.object(unit.os, 'fstat', side_effect=lambda fd: before if fd == 5 else row(stat.S_IFDIR | 0o755)), \
                    mock.patch.object(unit.os, 'stat', return_value=row(stat.S_IFREG | 0o644, inode=2 if change == 'path' else 1)), \
                    mock.patch.object(unit.os, 'read', side_effect=lambda fd, limit: reads.append(fd) or (b'bad' if change == 'bytes' else b'abc')), \
                    mock.patch.object(unit.os, 'close', side_effect=closed.append):
                if change is None: unit.verify('service', 'abc')
                else:
                    with self.assertRaises(Blocked): unit.verify('service', 'abc')
            self.assertEqual(sorted(closed), [1, 2, 3, 4, 5])
            if change in ('hardlink', 'permission'): self.assertEqual(reads, [])

    def test_installer_calls_exact_readback_before_loaded_activation(self):
        tree = ast.parse(Path(__file__).with_name('installer.py').read_text(encoding='utf-8'))
        install = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == 'install')
        readback = [node for node in ast.walk(install) if isinstance(node, ast.Call)
                    and isinstance(node.func, ast.Attribute) and node.func.attr == 'verify'
                    and isinstance(node.func.value, ast.Name) and node.func.value.id == 'hub_bridge_unit']
        activation = [node for node in ast.walk(install) if isinstance(node, ast.Constant) and node.value == 'daemon-reload']
        self.assertEqual(len(readback), 1)
        self.assertLess(readback[0].lineno, activation[0].lineno)
