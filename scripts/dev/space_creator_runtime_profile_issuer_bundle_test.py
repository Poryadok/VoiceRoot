import io
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import space_creator_runtime_profile_issuer_bundle as bundle
import space_creator_runtime_profile_issuer_source as contract

class BundleTest(unittest.TestCase):
    def test_fixed_git_source_and_launcher(self):
        source=Path(__file__).parent
        template=(source/'realtime_p3_root_launch.template.sh').read_bytes()
        contents={name:('fixture:'+name).encode() for name in contract.FILES}
        def blob(repo,commit,path):
            return template if path==contract.TEMPLATE else contents[path.removeprefix('scripts/dev/')]
        with tempfile.TemporaryDirectory() as tmp,patch.object(contract,'REVIEWED_SOURCE','a'*40), \
             patch.object(contract.source,'_blob',side_effect=blob),patch.object(contract.source,'_disk',side_effect=lambda repo,path:blob(repo,'b'*40,path)):
            bundle.build_git('/repo','b'*40,Path(tmp)/'out')
            launcher=(Path(tmp)/'out/p3-root-launch.sh').read_text()
            self.assertIn("module = 'space_creator_runtime_profile_issuer'",launcher)
            self.assertIn("{'PATH': '/usr/bin:/bin', 'LANG': 'C', 'LC_ALL': 'C'}",launcher)
            self.assertIn('/home/pmd/voice-space-creator/creator-source.tar',launcher)
            self.assertIn("!= '"+'b'*40+"': raise ValueError()",launcher)
            compile(launcher.split("<<'PY'\n",1)[1].rsplit('\nPY',1)[0],'<root-launch>','exec')
            with tarfile.open(fileobj=io.BytesIO((Path(tmp)/'out/p3-source.tar').read_bytes())) as tar:
                self.assertEqual(set(tar.getnames()),set(contract.FILES)|{'capture-manifest.json'})
            with patch.object(contract.source,'_disk',return_value=b'tampered'):
                with self.assertRaises(contract.SourceError): contract.verify('/repo','b'*40)

    def test_unreviewed_anchor_refuses(self):
        with patch.object(contract,'REVIEWED_SOURCE',None),self.assertRaises(contract.SourceError): contract.verify('/repo','b'*40)

if __name__=='__main__': unittest.main()
