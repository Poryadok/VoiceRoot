"""Actual isolated PostgreSQL password observation under the captured fence."""
import base64
import copy
import hashlib
import json
import os
import secrets
import subprocess
import time
import unittest
from unittest.mock import Mock,patch

import cohort_hosted_pg
import commands
import resource_claim
import resource_claim_test
import space_authority
import space_authority_test


@unittest.skipUnless(os.environ.get('VOICE_SPACE_PG_DOCKER')=='1','isolated Docker required')
class SpaceAuthorityPGIntegrationTest(unittest.TestCase):
    def test_real_postgres_password_obeys_captured_one_shot_fence(self):
        name='voice-space-authority-test-'+secrets.token_hex(12);cid=None
        def docker(*args):
            return subprocess.run(['docker',*args],stdout=subprocess.PIPE,stderr=subprocess.PIPE,check=True,timeout=60).stdout
        try:
            cid=docker('create','--name',name,'--network','none','--label','voice.test.owner='+name,
                '-e','POSTGRES_USER=voice','-e','POSTGRES_PASSWORD=isolated-test-only','postgres:16-bookworm').decode().strip()
            docker('start',cid)
            deadline=time.monotonic()+60
            while time.monotonic()<deadline:
                result=subprocess.run(['docker','exec',cid,'pg_isready','-U','voice'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
                if result.returncode==0:break
                time.sleep(.25)
            else:self.fail('isolated PostgreSQL not ready')
            adapter=object.__new__(cohort_hosted_pg.HostedPostgres);adapter.cid=cid
            context=resource_claim_test.context();card=resource_claim_test.card(context)
            session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal())
            parent={'protected':'fixture'};session.admit(start=True)
            session.journal.row.update(phase='prerequisites-captured',target_sha256=resource_claim.digest(parent),cohort_preservation_sha256='c'*64)
            session.expected_target=resource_claim.digest(parent);session._cohort_lineage='c'*64
            kube=space_authority_test.Kube()
            kube.rows[('secret','voice-app-secrets')]['data']['POSTGRES_PASSWORD']=base64.b64encode(b'isolated-test-only').decode()
            prefix=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging']
            kube.run=lambda args:json.loads(adapter.capture(prefix+args,body=b'',fallback=Mock(),timeout=30,limit=2<<20,
                env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'},pass_fds=()))
            prereq=Mock();prereq.load_capture.return_value=parent
            with patch.dict('sys.modules',{'staging_prerequisites':prereq}), \
                 patch.object(commands,'authority_guard',session.check), \
                 patch.object(cohort_hosted_pg,'DOCKER',['docker','--host=unix:///var/run/docker.sock']):
                result=space_authority.capture(kube)
                self.assertEqual(result['postgres_password_sha256'],hashlib.sha256(b'isolated-test-only').hexdigest())
                kube.rows[('secret','voice-app-secrets')]['data']['POSTGRES_PASSWORD']=base64.b64encode(b'wrong-valid-password').decode()
                with self.assertRaises(space_authority.AuthorityError):space_authority.capture(kube)
                self.assertIsNone(session._prerequisite_readonly_permit)
        finally:
            if cid:docker('rm','-f','-v',cid)


if __name__=='__main__':unittest.main()
