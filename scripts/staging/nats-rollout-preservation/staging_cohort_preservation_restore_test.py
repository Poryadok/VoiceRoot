"""Actual isolated PG16 dump/clone transport, with synthetic Kube/source admission.

Opt in with VOICE_COHORT_DOCKER=1 in an owned Linux Docker fixture. No live
namespace is addressed: the terminal fixed Kubernetes argv maps only to the
test's network-none source container. RootJournal/Session and clone transport
are real. This is not installed-cohort/source-eligibility or STAGING evidence.
"""
import copy
import importlib
import io
import json
import os
import secrets
import subprocess
import sys
import tarfile
import tempfile
import time
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
sys.path.insert(0,str(Path(__file__).resolve().parents[2]/'dev'))
import commands
import guard
import resource_claim
import staging_cohort_preservation as owner
import staging_cohort_preservation_pg as pg
from staging_cohort_preservation_runtime import DOCKER
import staging_cohort_preservation_test as cut_tests
from auth_user_adoption_runtime import QUALIFIED_TOOLS


@unittest.skipUnless(os.environ.get('VOICE_COHORT_DOCKER')=='1','isolated Docker fixture opt-in')
class RestoreTransportTests(unittest.TestCase):
    def test_actual_pg_dump_and_isolated_restore_with_root_session(self):
        run=subprocess.run;original_popen=subprocess.Popen
        def docker(*args,body=None):
            result=run(DOCKER+list(args),input=body,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120)
            if result.returncode:raise AssertionError('isolated fixture command failed: '+str(args[:2]))
            return result.stdout
        name='voice-cohort-source-'+secrets.token_hex(12)
        cid=docker('create','--name',name,'--network','none','--label','voice.test.owner='+name,
            '-e','POSTGRES_PASSWORD=isolated-test-only','-e','POSTGRES_USER=postgres',QUALIFIED_TOOLS['postgres']).decode().strip()
        try:
            docker('start',cid)
            deadline=time.monotonic()+60
            while True:
                result=run(DOCKER+['exec',cid,'pg_isready','-h','127.0.0.1','-U','postgres'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
                if result.returncode==0:break
                if time.monotonic()>deadline:self.fail('source PG fixture unavailable')
                time.sleep(.2)
            docker('exec','-i',cid,'psql','-X','-q','-v','ON_ERROR_STOP=1','-U','postgres','-d','postgres',
                body=b'CREATE TABLE public.preservation_fixture(id integer PRIMARY KEY, value text); INSERT INTO public.preservation_fixture VALUES(1,\'kept\');')
            with tempfile.TemporaryDirectory(dir='/root') as directory:
                root=Path(directory);(root/'installed').mkdir(mode=0o700)
                before={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-user','namespace':'voice-staging',
                    'uid':'user','resourceVersion':'3','generation':1},'spec':{'replicas':1}}
                source=Path(__file__).resolve().parents[3]
                approved={'source_files':{p.relative_to(source).as_posix():__import__('hashlib').sha256(p.read_bytes()).hexdigest()
                    for prefix in ('src/backend/migrations','src/backend/auth/src/main/resources/db/migration')
                    for p in (source/prefix).rglob('*.sql')}}
                actual_capture=commands.capture
                def capture(argv,**kwargs):
                    if argv[:len(pg.EXEC)]==pg.EXEC:
                        commands.authority_guard(argv)
                        return docker('exec','-i',cid,*argv[len(pg.EXEC):],body=kwargs.get('body'))
                    return actual_capture(argv,**kwargs)
                def popen(argv,*args,**kwargs):
                    if argv[:len(pg.EXEC)]==pg.EXEC:argv=DOCKER+['exec','-i',cid,*argv[len(pg.EXEC):]]
                    return original_popen(argv,*args,**kwargs)
                def shared(session,module):return importlib.import_module(module)
                namespace='11111111-1111-4111-8111-111111111111'
                engine={'pod':{'uid':'22222222-2222-4222-8222-222222222222','container_id':'containerd://'+'c'*64,'restart_count':0},
                    'pvc':{'uid':'33333333-3333-4333-8333-333333333333'},'pv':{'uid':'44444444-4444-4444-8444-444444444444'}}
                with patch.object(guard,'ROOT',root),patch.object(owner,'_source',return_value=(approved,{},{})),\
                    patch.object(owner,'revalidate_paused'),patch.object(owner,'shared_module',side_effect=shared),\
                    patch('realtime_bootstrap.engine_snapshot',return_value=(namespace,engine)),\
                    patch.object(commands,'capture',side_effect=capture),patch.object(subprocess,'Popen',side_effect=popen):
                    session,_=cut_tests.CutEffectTests().fixture(root,before,namespace)
                    owner.private._private_directory(root/'installed'/'sources')
                    owner.private._private_directory(root/'installed'/'sources'/session.operation)
                    with resource_claim.command_scope(session.check):
                        archive=owner._capture_archive(session)
                        self.assertEqual(set(archive['databases']),{'postgres','template1'})
                        self.assertEqual(set(owner.dump_receipts(session)),{'roles','postgres','template1'})
                        receipt=owner._qualify_restore(session,'local')
                        self.assertEqual(receipt['status'],'COMPLETE')
                        self.assertEqual(receipt['proof']['cleanup'],'verified')
                        self.assertEqual(set(receipt['proof']['databases']),{'postgres','template1'})
                        owner._no_pending(session)
        finally:
            docker('rm','-f','-v',cid)


if __name__=='__main__':unittest.main()
