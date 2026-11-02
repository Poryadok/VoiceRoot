"""Large input is a single exact root-fenced restore launch, never a generic limit."""
import copy
import hashlib
import io
import os
import sys
import tarfile
import unittest
from dataclasses import replace
from pathlib import Path
from unittest.mock import patch

import commands
import resource_claim as claim
import resource_claim_test
import staging_cohort_preservation as owner
import staging_cohort_preservation_runtime as runtime


class LargeInputTests(unittest.TestCase):
    def setUp(self):
        self.session,_,self.journal=resource_claim_test.ResourceTests().fixture()
        self.session.admit(start=True)
        self.journal.row.update(phase='cohort-preservation',target_sha256='d'*64,cohort_preservation_sha256='d'*64)
        self.session.expected_target='d'*64;self.session._cohort_lineage='d'*64
        self.body=b'x'*((2<<20)+1)
        self.argv=[sys.executable,'-c','import sys,hashlib; print(hashlib.sha256(sys.stdin.buffer.read()).hexdigest())']
        self.env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'}

    def arm(self,body=None,argv=None,timeout=60,limit=1024,env=None,pass_fds=()):
        s=self.session;body=self.body if body is None else body;argv=self.argv if argv is None else argv
        env=self.env if env is None else env
        sha=hashlib.sha256(body).hexdigest()
        s._cohort_permit={'effect_id':'observe-restore-'+'a'*16,'argv':list(argv),'body_sha256':sha}
        s._cohort_restore_large_permit=claim._CohortRestoreLargePermit(
            s,claim.digest(s.binding()),self.journal.row['cohort_preservation_sha256'],tuple(argv),sha,
            len(body),timeout,limit,tuple(sorted(env.items())),tuple(pass_fds),'1'*64,'2'*64,'a'*64,'postgres','ACTIVE')

    def launch(self,body=None,argv=None,timeout=60,limit=1024,env=None,pass_fds=()):
        body=self.body if body is None else body;argv=self.argv if argv is None else argv
        env=self.env if env is None else env
        with patch.object(commands,'authority_guard',self.session.check),patch.object(runtime,'revalidate_restore_large_input'):
            return commands.capture(argv,body=body,timeout=timeout,limit=limit,env=env,pass_fds=pass_fds)

    def test_exact_one_shot_stream_and_replay_denied(self):
        self.arm();result=self.launch()
        self.assertEqual(result.strip(),hashlib.sha256(self.body).hexdigest().encode())
        self.assertIsNone(self.session._cohort_restore_large_permit)
        self.arm();self.session._cohort_restore_large_permit=None
        with self.assertRaisesRegex(RuntimeError,'cohort_restore_large_input_invalid'):self.launch()

    def test_forged_stale_and_changed_launches_never_spawn(self):
        for fault in ('forged','session','lineage','binding','phase','argv','body','small','oversize','timeout','limit','env','fds','effect'):
            with self.subTest(fault=fault):
                self.journal.row.update(phase='cohort-preservation',target_sha256='d'*64,cohort_preservation_sha256='d'*64)
                self.session.expected_target='d'*64;self.session._cohort_lineage='d'*64;self.arm()
                kwargs={}
                if fault=='forged':self.session._cohort_restore_large_permit=object()
                elif fault=='session':
                    p=self.session._cohort_restore_large_permit
                    self.session._cohort_restore_large_permit=replace(p,session=object())
                elif fault=='lineage':self.session._cohort_lineage=None;self.journal.row['cohort_preservation_sha256']='e'*64
                elif fault=='binding':self.journal.row['target_sha256']='e'*64;self.session.expected_target='e'*64
                elif fault=='phase':self.journal.row['phase']='prepare'
                elif fault=='argv':kwargs['argv']=self.argv+['different']
                elif fault=='body':kwargs['body']=b'y'*len(self.body)
                elif fault=='small':kwargs['body']=b'x'
                elif fault=='oversize':kwargs['body']=b'x'*((64<<20)+1)
                elif fault=='timeout':kwargs['timeout']=61
                elif fault=='limit':kwargs['limit']=1025
                elif fault=='env':kwargs['env']={'PATH':'/bin'}
                elif fault=='fds':kwargs['pass_fds']=(0,)
                else:self.session._cohort_permit['effect_id']='foreign'
                with patch.object(commands.subprocess,'Popen') as spawn:
                    with self.assertRaises((RuntimeError,commands.Blocked)):self.launch(**kwargs)
                    spawn.assert_not_called()
                self.assertIsNone(self.session._cohort_restore_large_permit)

    def test_generic_large_input_remains_forbidden(self):
        with patch.object(commands,'authority_guard',None),patch.object(commands.subprocess,'Popen') as spawn:
            with self.assertRaisesRegex(commands.Blocked,'command_input_limit'):
                commands.capture(self.argv,body=self.body)
            spawn.assert_not_called()

    def test_observer_cannot_replace_guard_or_fence_before_spawn(self):
        for fault in ('guard','fence'):
            with self.subTest(fault=fault):
                self.journal.row.update(status='active',phase='cohort-preservation',target_sha256='d'*64,cohort_preservation_sha256='d'*64)
                self.session.expected_target='d'*64;self.session._cohort_lineage='d'*64;self.arm()
                def observer(action,status):
                    if fault=='guard':commands.authority_guard=lambda argv:None
                    else:self.journal.row['status']='uncertain'
                with patch.object(commands,'diagnostic_observer',side_effect=observer),patch.object(commands.subprocess,'Popen') as spawn:
                    with self.assertRaises(RuntimeError):self.launch()
                    spawn.assert_not_called()
                self.assertIsNone(self.session._cohort_restore_large_permit)

    def test_real_53mb_transport_only_after_verified_owned_restore_command(self):
        binary=os.environ.get('VOICE_MIGRATE_TEST_FILE')
        body=Path(binary).read_bytes() if binary else b'm'*53170328
        self.assertEqual(len(body),53170328)
        cid='a'*64
        argv=runtime.DOCKER+['exec','-i',cid,'sh','-ec','cat > /tmp/voice-migrate && chmod 0555 /tmp/voice-migrate']
        state={'resources':{'clone':{'cid':cid,'kind':'postgres','status':'ACTIVE'}}}
        tools={'images':{},'ids':{},'migrate_binary_sha256':hashlib.sha256(body).hexdigest()}
        path=Path('/fixture/resources.json');original_popen=commands.subprocess.Popen;spawned=[]
        def read(p):return copy.deepcopy(tools if str(p).endswith('restore-tools.json') else state)
        def observation(session,label,actual,data,*,limit,timeout):
            self.assertEqual((session,label,actual),(self.session,'restore',argv))
            session._cohort_permit={'effect_id':'observe-restore-'+'a'*16,'argv':list(actual),'body_sha256':hashlib.sha256(data).hexdigest()}
            def child(_argv,**kwargs):
                spawned.append(True)
                return original_popen([sys.executable,'-c','import sys,hashlib; print(hashlib.sha256(sys.stdin.buffer.read()).hexdigest())'],**kwargs)
            with patch.object(commands,'authority_guard',session.check),patch.object(commands.subprocess,'Popen',side_effect=child):
                return commands.capture(actual,body=data,limit=limit,timeout=timeout,env=self.env)
        with patch.object(runtime,'_resources',return_value=(path,state)),patch.object(owner,'_base',return_value=Path('/fixture')),\
             patch.object(owner,'_read',side_effect=read),patch.object(owner,'_observation',side_effect=observation):
            result=runtime.restore_command(self.session,argv,body=body)
            self.assertEqual(result.strip(),hashlib.sha256(body).hexdigest().encode())
            with self.assertRaisesRegex(runtime.Blocked,'cohort_restore_body_limit'):
                runtime.restore_command(self.session,argv,body=b'n'*len(body))
            for status in ('OBSERVED','REMOVING'):
                state['resources']['clone']['status']=status
                with self.subTest(status=status),self.assertRaisesRegex(runtime.Blocked,'cohort_restore_exec_invalid'):
                    runtime.restore_command(self.session,argv,body=body)
                state['resources']['clone']['status']='ACTIVE'
            for fault in ('resource','tools'):
                def observer(action,status):
                    if fault=='resource':state['resources']['clone']['status']='REMOVING'
                    else:tools['migrate_binary_sha256']='0'*64
                with self.subTest(fault=fault),patch.object(commands,'diagnostic_observer',side_effect=observer):
                    with self.assertRaises(runtime.Blocked):runtime.restore_command(self.session,argv,body=body)
                self.assertEqual(len(spawned),1)
                state['resources']['clone']['status']='ACTIVE';tools['migrate_binary_sha256']=hashlib.sha256(body).hexdigest()
            self.assertIsNone(self.session._cohort_restore_large_permit)

    def test_probe_tar_is_digest_bound_before_large_permit(self):
        import story_fixture
        payload=b'p'*((2<<20)+1);raw=io.BytesIO()
        with tarfile.open(fileobj=raw,mode='w:') as archive:
            member=tarfile.TarInfo('voice-story-restored8-probe');member.size=len(payload);member.mode=0o555
            archive.addfile(member,io.BytesIO(payload))
        body=raw.getvalue();cid='b'*64;argv=runtime.DOCKER+['cp','-',cid+':/tmp/']
        state={'resources':{'probe':{'cid':cid,'kind':'story-probe','status':'ACTIVE'}}}
        tools={'images':{},'ids':{}};path=Path('/fixture/resources.json')
        def read(p):return copy.deepcopy(tools if str(p).endswith('restore-tools.json') else state)
        def observation(session,label,actual,data,*,limit,timeout):
            session._cohort_permit={'effect_id':'observe-restore-'+'a'*16,'argv':list(actual),'body_sha256':hashlib.sha256(data).hexdigest()}
            permit=session.consume_restore_large_input(actual,data,timeout,limit,self.env,())
            session.check(actual)
            runtime.revalidate_restore_large_input(session,permit,actual,data)
            return b'ok'
        with patch.object(story_fixture,'PRESERVED_CALLER_SHA256',hashlib.sha256(payload).hexdigest()),\
             patch.object(runtime,'_resources',return_value=(path,state)),patch.object(owner,'_base',return_value=Path('/fixture')),\
             patch.object(owner,'_read',side_effect=read),patch.object(owner,'_observation',side_effect=observation):
            self.assertEqual(runtime.restore_command(self.session,argv,body=body),b'ok')
            with patch.object(story_fixture,'PRESERVED_CALLER_SHA256','0'*64):
                with self.assertRaisesRegex(runtime.Blocked,'cohort_story_caller_changed'):
                    runtime.restore_command(self.session,argv,body=body)
            for status in ('OBSERVED','REMOVING'):
                state['resources']['probe']['status']=status
                with self.subTest(status=status),self.assertRaisesRegex(runtime.Blocked,'cohort_restore_body_limit'):
                    runtime.restore_command(self.session,argv,body=body)
            self.assertIsNone(self.session._cohort_restore_large_permit)


if __name__=='__main__':unittest.main()
