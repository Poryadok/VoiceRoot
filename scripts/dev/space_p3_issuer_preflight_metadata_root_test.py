import hashlib
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import space_p3_issuer_preflight_metadata_root as subject


@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'isolated root Linux fixture')
class MetadataTest(unittest.TestCase):
    def test_fixed_subprocess_names_absence_and_transport_failure(self):
        with tempfile.TemporaryDirectory(dir='/root',prefix='space-metadata-') as tmp:
            directory=Path(tmp); binary=directory/'k3s'; config=directory/'config'
            binary.write_text('#!/bin/sh\ncase "$*" in *"get namespace"*) printf ns;; *"get secret voice-space-media-client-tls"*) printf "uid\\nrv\\n\\ntls.crt\\ntls.key\\n";; *) exit 0;; esac\n')
            binary.chmod(0o755); config.write_text('private-sentinel'); config.chmod(0o644)
            digest=hashlib.sha256(binary.read_bytes()).hexdigest()
            with patch.object(subject,'K3S',binary),patch.object(subject,'KUBECONFIG',config),patch.object(subject,'K3S_SHA256',digest):
                result=subject.observe('ns')
                self.assertEqual(result['voice-space-media-client-tls']['keys'],['tls.crt','tls.key'])
                self.assertEqual(result['voice-gateway-principal-signing'],{'state':'absent'})
                self.assertNotIn('private-sentinel',str(result))
                binary.write_text('#!/bin/sh\nexit 1\n'); binary.chmod(0o755)
                with self.assertRaises(ValueError): subject.observe('ns')


if __name__=='__main__': unittest.main()
