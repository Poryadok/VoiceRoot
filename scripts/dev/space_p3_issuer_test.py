"""Space-only offline issuance: exact existing CA and one-shot ownership."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import space_p3_issuer as issuer
import reset_credentials as reset


class IssuerTest(unittest.TestCase):
    def test_existing_media_ca_only_and_distinct_signers(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch); archive = root / 'generation1'; archive.mkdir(mode=0o700)
            subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','120',
                '-subj','/CN=fixture-media','-addext','basicConstraints=critical,CA:TRUE',
                '-keyout',str(archive/'media-ca.key'),'-out',str(archive/'media-ca.crt')],
                check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            (archive/'media-ca.key').chmod(0o600)
            media=(archive/'media-ca.crt').read_bytes(); dest=root/('space-p3-'+'a'*32)
            with patch.object(issuer,'ARCHIVE',archive), patch.object(issuer,'PARENT',root), patch.object(issuer,'_check_ancestors'):
                with self.assertRaises(issuer.IssueError): issuer.stage(root/('space-p3-'+'b'*32), b'wrong')
            self.assertTrue((root/'space-p3-issuance.once').exists())
            (root/'space-p3-issuance.once').unlink()
            with patch.object(issuer,'ARCHIVE',archive), patch.object(issuer,'PARENT',root), patch.object(issuer,'_check_ancestors'):
                receipt=issuer.stage(dest,media)
                with self.assertRaises(issuer.IssueError): issuer.stage(root/('space-p3-'+'c'*32),media)
            objects=json.loads((dest/'objects.json').read_bytes())['items']
            self.assertEqual({x['metadata']['name'] for x in objects}, {'voice-space-principal-signing','voice-space-media-server-tls'})
            signer=next(x for x in objects if x['metadata']['name']=='voice-space-principal-signing')
            self.assertEqual(reset.values(signer)['active-kid'],b'current')
            self.assertEqual(receipt['public_jwks'],reset.signer_jwks(signer))
            self.assertEqual((dest/'objects.json').stat().st_mode & 0o777,0o600)
            self.assertNotIn('PRIVATE KEY',json.dumps(receipt))
            self.assertFalse((dest/'media-ca.key').exists())


if __name__ == '__main__': unittest.main()
