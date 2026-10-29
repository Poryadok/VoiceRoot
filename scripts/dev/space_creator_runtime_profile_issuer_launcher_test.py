"""Real Git blobs and isolated root launcher source-capture fixture."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import realtime_p3_root_bundle as base
import space_creator_runtime_profile_issuer_bundle as bundle
import space_creator_runtime_profile_issuer_source as contract

@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'isolated root Linux fixture')
class LauncherTest(unittest.TestCase):
    def test_real_git_source_bundle_and_fixed_source_sha(self):
        with tempfile.TemporaryDirectory(dir='/root',prefix='creator-launch-') as scratch:
            root=Path(scratch); repo=root/'repo'; files=repo/'scripts/dev'; files.mkdir(parents=True)
            actual=Path(__file__).parent
            for name in contract.FILES+('realtime_p3_root_launch.template.sh',):
                (files/name).write_bytes((actual/name).read_bytes())
            def git(*args): return subprocess.run(['git','-C',str(repo),*args],check=True,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL).stdout
            git('init','-q'); git('config','user.name','fixture'); git('config','user.email','fixture@example.invalid')
            git('add','scripts'); git('commit','-qm','fixture'); commit=git('rev-parse','HEAD').decode().strip()
            issuance=root/'issuance'; issuance.mkdir(mode=0o700); actual_build=base._build
            def build(items,template,output):
                changes={'/home/pmd/voice-space-creator/creator-source.tar':str(root/'p3-source.tar'),
                         "pathlib.Path('/home/voice-dev/issuance')":f'pathlib.Path({str(issuance)!r})',
                         "pwd.getpwnam('pmd').pw_uid":'os.getuid()'}
                for before,after in changes.items():
                    self.assertEqual(template.count(before),1); template=template.replace(before,after)
                return actual_build(items,template,output)
            with patch.object(contract,'REVIEWED_SOURCE',commit),patch.object(base,'_build',side_effect=build):
                bundle.build_git(str(repo),commit,root)
            (root/'p3-source.tar').chmod(0o600); launch=root/'p3-root-launch.sh'
            result=subprocess.run(['/bin/bash',str(launch),'--source-sha',commit,'--help'],capture_output=True,timeout=30)
            self.assertEqual(result.returncode,0); self.assertIn(b'--consumer-pid',result.stdout)
            bad=subprocess.run(['/bin/bash',str(launch),'--source-sha','0'*40,'--help'],capture_output=True,timeout=30)
            self.assertNotEqual(bad.returncode,0)

if __name__=='__main__': unittest.main()
