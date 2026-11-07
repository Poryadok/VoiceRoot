"""Optional real Git source checkout and fixed bundle construction."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import space_p3_issuer_bundle as bundle
import space_p3_issuer_source as source


@unittest.skipUnless(os.environ.get('VOICE_SPACE_GITDIR') and os.environ.get('VOICE_SPACE_SOURCE_SHA'),
                     'explicit offline Git fixture required')
class SourceGitTest(unittest.TestCase):
    def test_exact_reviewed_candidate_and_changed_disk_fails(self):
        with tempfile.TemporaryDirectory(prefix='space-source-git-') as temporary:
            root=Path(temporary); repo=root/'repo'; output=root/'bundle'; sha=os.environ['VOICE_SPACE_SOURCE_SHA']
            subprocess.run(['git','clone','--no-checkout','--shared',os.environ['VOICE_SPACE_GITDIR'],str(repo)],
                           check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=90)
            paths=['scripts/dev/'+name for name in source.ISSUER_FILES]+list(source.RUNTIME_FILES)+[
                'scripts/dev/realtime_p3_root_launch.template.sh','deploy/staging/services.yaml']
            subprocess.run(['git','-C',str(repo),'checkout',sha,'--',*paths],check=True,
                           stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=90)
            source.verify(str(repo),sha); digest=bundle.build_git(str(repo),sha,output)
            self.assertEqual(len(digest),64); self.assertTrue((output/'p3-root-launch.sh').is_file())
            file=repo/'src/backend/space/principal_issuer.go'; file.write_bytes(file.read_bytes()+b'\n')
            with self.assertRaises(source.SourceError): source.verify(str(repo),sha)


if __name__ == '__main__': unittest.main()
