import hashlib
import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import root_cli
from controller import Blocked


class RunnerCustodyTests(unittest.TestCase):
    def test_nonroot_candidate_data_binding_needs_explicit_current_owner(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary);code=root/'candidate-data';code.mkdir(mode=0o700)
            payload=b'candidate payload is data, not imported code'
            member=code/'source-record.json';member.write_bytes(payload);member.chmod(0o400)
            manifest={'source-record.json':hashlib.sha256(payload).hexdigest()}
            (code/'capture-manifest.json').write_text(json.dumps(manifest));(code/'capture-manifest.json').chmod(0o400)
            with self.assertRaises(Blocked):root_cli.code_binding(code)
            self.assertEqual(root_cli.code_binding(code,owner_uid=os.geteuid()),manifest)

    def test_runner_owned_operation_directory_still_requires_exact_identity(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary);operation=root/'rollout-012345abcdef';operation.mkdir(mode=0o700)
            with patch.object(root_cli,'ROOT',root):
                with self.assertRaises(Blocked):root_cli.operation_path(operation)
                self.assertEqual(root_cli.operation_path(operation,owner_uid=os.geteuid()),operation)
                with self.assertRaises(Blocked):root_cli.operation_path(root/'rollout-012345abc0',owner_uid=os.geteuid())


if __name__=='__main__':
    unittest.main()
