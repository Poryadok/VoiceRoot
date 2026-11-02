"""Offline real Linux kubectl FD compatibility; opt in with SPACE_TEST_KUBECTL."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock

import space_fresh_install_phase_runtime as runtime


@unittest.skipUnless(os.environ.get('SPACE_TEST_KUBECTL'), 'offline Linux kubectl binary not supplied')
class KubectlFDTest(unittest.TestCase):
    def test_real_kubectl_reads_sealed_inherited_patch_fd_offline(self):
        with tempfile.TemporaryDirectory() as directory:
            binary=Path(directory)/'kubectl'; shutil.copyfile(os.environ['SPACE_TEST_KUBECTL'],binary); binary.chmod(0o700)
            resource=Path(directory)/'dummy.json'
            resource.write_text(json.dumps({'apiVersion':'v1','kind':'Secret','metadata':{'name':'dummy'},
                                            'data':{'value':'b2xk','other':'c2VudGluZWw='}}),encoding='utf-8')
            original=subprocess.Popen
            def launch(args, **kwargs):
                patchfile=next(x for x in args if x.startswith('--patch-file='))
                return original([str(binary),'patch','--local','-f',str(resource),'--type=json',patchfile,'-o','json'],**kwargs)
            with mock.patch.object(runtime.subprocess,'Popen',side_effect=launch):
                result=runtime._private_patch([{'op':'replace','path':'/data/value','value':'bmV3'}],dry_run=False)
            self.assertEqual(result['data'],{'value':'bmV3','other':'c2VudGluZWw='})


if __name__ == '__main__': unittest.main()
