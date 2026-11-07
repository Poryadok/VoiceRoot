"""Real Docker proof for the isolated, read-only Story probe loader."""
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
import unittest

import staging_cohort_preservation_runtime as runtime
import story_fixture


@unittest.skipUnless(os.environ.get('VOICE_STORY_PROBE'), 'qualified probe binary required')
class StoryProbeLoaderIntegrationTest(unittest.TestCase):
    def test_readonly_probe_streams_pinned_binary_into_owned_tmpfs(self):
        image='ghcr.io/poryadok/voiceroot/story@sha256:8b142297965b3121fc19e0d970b137ad4ff8c8afa2c79eb49e490d59324f3c5f'
        binary=Path(os.environ['VOICE_STORY_PROBE']).read_bytes()
        self.assertEqual(hashlib.sha256(binary).hexdigest(),story_fixture.PRESERVED_CALLER_SHA256)
        errors=[]
        def docker(*args,body=None):
            result=subprocess.run(['docker',*args],input=body,stdout=subprocess.PIPE,stderr=subprocess.PIPE,
                                  check=True,timeout=120)
            errors.append(result.stderr)
            return result.stdout
        name='voice-proof-net-'+secrets.token_hex(12);probe=None;network=None
        try:
            network=docker('create','--name',name,'--network','none','--entrypoint','/bin/sh',image,'-c','sleep 120').decode().strip()
            docker('start',network)
            image_id=docker('image','inspect',image,'--format','{{.Id}}').decode().strip()
            probe_name='voice-media-restore-'+secrets.token_hex(16)
            dsn='postgres://actual_runtime:'+'e'*48+'@127.0.0.1:5432/story_db?sslmode=disable'
            argv=runtime.story_create(probe_name,network,image_id,'story-probe',dsn,mode='invalid')
            probe=docker(*argv).decode().strip()
            info=json.loads(docker('inspect',probe))[0]
            self.assertTrue(info['HostConfig']['ReadonlyRootfs'])
            self.assertEqual(info['HostConfig']['Tmpfs'],{'/tmp':runtime.STORY_PROBE_TMPFS.split(':',1)[1]})
            self.assertEqual(info['Mounts'],[])
            self.assertEqual(info['Config']['Cmd'],argv[argv.index('--entrypoint')+3:])
            result=docker('start','--attach','--interactive',probe,body=binary)
            match=re.fullmatch(rb'([^\n]+)\n{1,2}VOICE_PROBE_EXIT=(0|17)\n',result)
            self.assertIsNotNone(match,repr((result[:1000],errors[-1][:1000])))
            self.assertEqual(int(match.group(2)),17)
            self.assertEqual(json.loads(match.group(1)),{'status':'FAIL','phase':'baseline_stories_input'})
        finally:
            for cid in (probe,network):
                if cid:docker('rm','-f','-v',cid)


if __name__=='__main__':unittest.main()
