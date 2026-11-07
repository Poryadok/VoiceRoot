import io
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import space_p3_issuer_bundle as bundle


class BundleTest(unittest.TestCase):
    def test_fixed_root_modules_and_preflight_only_dispatch(self):
        root=Path(__file__).parent
        def blob(repo,commit,path): return (root/Path(path).name).read_bytes()
        with tempfile.TemporaryDirectory() as tmp, patch.object(bundle.preflight_source,'verify'), \
             patch.object(bundle.source,'_blob',side_effect=blob),patch.object(bundle.source,'_disk',side_effect=lambda r,p:blob(r,'',p)):
            result=bundle.build_preflight_git('/source','a'*40,tmp)
            files=list(Path(tmp).iterdir()); archives=[p for p in files if p.suffix=='.tar']
            self.assertEqual(len(archives),1)
            with tarfile.open(archives[0]) as archive:
                names=set(archive.getnames())
            self.assertIn('space_p3_issuer_preflight_root.py',names)
            self.assertIn('space_p3_issuer_preflight_metadata_root.py',names)
            self.assertTrue(result)


if __name__=='__main__': unittest.main()
