import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

import realtime_p3_create_only_transport as transport


class TransportTest(unittest.TestCase):
    def test_private_stdin_and_sanitized_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            script = Path(directory)/'fake.py'
            script.write_text("import sys,json\nraw=sys.stdin.buffer.read()\nsys.stderr.write('private-sentinel')\n"
                              "sys.stdout.write(json.dumps({'received':len(raw)}))\n")
            with patch.object(transport.frontend_runtime_executor.KubectlTransport,'PREFIX',[sys.executable,str(script)]):
                output = transport._run(['create'],b'private-sentinel')
                self.assertEqual(json.loads(output),{'received':16}); self.assertNotIn(b'private-sentinel',output)
                script.write_text("import sys\nsys.stderr.write('private-sentinel')\nsys.exit(1)\n")
                with self.assertRaises(transport.TransportError) as error: transport._run(['create'],b'private-sentinel')
                self.assertNotIn('private-sentinel',str(error.exception))

    def test_oversized_stdout_and_unsupported_identity_fail(self):
        with tempfile.TemporaryDirectory() as directory:
            script = Path(directory)/'fake.py'; script.write_text("import sys\nsys.stdout.buffer.write(b'x'*(2*1024*1024+1))\n")
            with patch.object(transport.frontend_runtime_executor.KubectlTransport,'PREFIX',[sys.executable,str(script)]):
                with self.assertRaises(transport.TransportError): transport._run(['get'])
        with self.assertRaises(transport.TransportError): transport.get('Secret','foreign')


if __name__ == '__main__': unittest.main()
