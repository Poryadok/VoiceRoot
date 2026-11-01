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
            docker('exec','-i',cid,'psql','-X','-q','-v','ON_ERROR_STOP=1','-U','postgres','-d','postgres',
                body=b"CREATE FUNCTION public.pg_postmaster_start_time() RETURNS timestamptz LANGUAGE sql IMMUTABLE AS 'SELECT timestamptz ''2000-01-01 00:00:00+00'''; ALTER ROLE postgres SET search_path=public,pg_catalog;")
            spoofed=docker('exec',cid,'psql','-X','-qAt','-U','postgres','-d','postgres','-c',
                "SELECT extract(year from pg_postmaster_start_time())::int")
            self.assertEqual(spoofed.strip(),b'2000')
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
                    preflight={'schema':'voice-staging-cohort-preflight-v1','operation':'a'*12,
                        'services':[],'actor':None,'nats':{},'enrollment':{},'contract':{}}
                    session,_=cut_tests.CutEffectTests().fixture(root,before,namespace,preflight=preflight)
                    owner.private._private_directory(root/'installed'/'sources')
                    owner.private._private_directory(root/'installed'/'sources'/session.operation)
                    with resource_claim.command_scope(session.check):
                        checked=pg._query(session,'postgres',"SELECT json_build_object('year',extract(year from pg_postmaster_start_time())::int);",'catalog-shadow-test')
                        self.assertNotEqual(checked['year'],2000)
                        archive=owner._capture_archive(session)
                        self.assertEqual(set(archive['databases']),{'postgres','template1'})
                        self.assertEqual(set(owner.dump_receipts(session)),{'roles','postgres','template1'})
                        receipt=owner._qualify_restore(session,'local')
                        self.assertEqual(receipt['status'],'COMPLETE')
                        self.assertEqual(receipt['proof']['cleanup'],'verified')
                        self.assertEqual(set(receipt['proof']['databases']),{'postgres','template1'})
                        # Actual CMS encrypt/decrypt and a second PG restore of
                        # the extracted capsule. Native NATS bytes/readback and
                        # GitHub custody are explicit external fixtures here.
                        import encrypted_cut
                        base=owner._runtime_base(session);owner.private._private_directory(base)
                        recovery=root/'installed'/'recovery';owner.private._private_directory(recovery)
                        result=run(['/usr/bin/openssl','req','-x509','-newkey','rsa:2048','-nodes',
                            '-keyout',str(recovery/'recovery-key.pem'),'-out',str(recovery/'recovery-cert.pem'),
                            '-subj','/CN=isolated-cohort-test','-days','1'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=30)
                        self.assertEqual(result.returncode,0)
                        for filename in ('recovery-key.pem','recovery-cert.pem'):(recovery/filename).chmod(0o600)
                        owner.private._private_write(base/'rollout-before.tar',b'native-nats-fixture-only')
                        native={'archive_sha256':__import__('hashlib').sha256(b'native-nats-fixture-only').hexdigest()}
                        owner._write(base/'rollout-before-manifest.json',native)
                        cut={'manifest':native,'manifest_sha256':owner._file_identity(base/'rollout-before-manifest.json')['sha256'],
                            'census_sha256':'f'*64}
                        binding=owner._encrypt(session,{'dispatcher_run_id':123,'head_sha':session.context['commit']},cut)
                        owner._write(owner._base(session)/'native-cut.json',cut)
                        owner._write(owner._base(session)/'paused.json',{'external_paused_stage_fixture':True,'events':[]})
                        state=owner._state(session);state.update(status='AWAITING_OFF_NODE',cipher_binding=binding,
                            initial_resource_fence=session.binding(),cut=cut,events=[],contract={},
                            provenance={'external_nats_source_fixture':True})
                        owner._save_state(session,state)
                        import bridge_root
                        import github_custody
                        import staging_cohort_preservation_auxiliary as auxiliary
                        # Actual Actions type/Session and production continuation;
                        # installed source admission, remote artifact retrieval,
                        # NATS and absence of an existing auxiliary are fixtures.
                        with patch.object(bridge_root.root_cli,'code_binding',return_value={}):actions=bridge_root.Actions(source/'scripts'/'staging')
                        actions.tt_session=session
                        request={'operation':session.operation,'action':'authorize','token':'fixture-only','artifact_id':42,
                                 'resource_context':session.context}
                        custody={'schema':'voice-nats-custody-v1','destination':'github:Poryadok/VoiceRoot',
                            **{key:binding[key] for key in ('operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes')},
                            'artifact_id':42,'verified':True}
                        with patch.object(github_custody,'verify_artifact',return_value=custody) as remote,\
                            patch.object(owner,'_native_readback',return_value={'external_nats_fixture':True}),\
                            patch.object(auxiliary,'resume',return_value=None):
                            qualified=owner.qualify_offnode(actions,request)
                            self.assertEqual(owner.qualify_offnode(actions,request),qualified)
                            self.assertEqual(remote.call_count,1)
                        self.assertEqual(owner.load_qualified(session),qualified)
                        # Join the real PG/CMS qualification to the production
                        # continuation intent. Source eligibility and the saved
                        # prepared input are explicit fixtures, not live proof.
                        saved_request={'action':'prepare','nonce':session.operation+'0'*52,'resource_context':session.context,
                                       'mode':'app-only','changed_services':['user']}
                        owner.private._private_directory(owner._request_record(session).parent)
                        owner._write(owner._request_record(session),saved_request)
                        prepared={'request':{'mode':'app-only','changed_services':['user'],'gis_first_install':False},
                                  'approved':approved,'execution':{},'workspace':str(source)}
                        with patch.object(owner,'load_prepared',return_value=prepared):
                            with self.assertRaisesRegex(Exception,'cohort_continuation_request_changed'):
                                owner.begin_continuation(session,dict(request,artifact_id=43))
                            self.assertFalse((owner._base(session)/'continuation.json').exists())
                            selected,loaded=owner.begin_continuation(session,request)
                            self.assertEqual((selected,loaded),(qualified,prepared))
                            self.assertEqual(owner.require_continuation(session,request)['qualified_sha256'],owner.digest(qualified))
                            altered=dict(request,artifact_id=43)
                            with self.assertRaises(Exception):owner.require_continuation(session,altered)
                        # NATS bytes are explicitly synthetic here; PG/CMS,
                        # Session and qualified continuation records are real.
                        with patch('native_store.verify_archive',return_value=True):
                            continuation=owner.load_rollout_continuation(session)
                            self.assertEqual(continuation,{'cut':cut,'custody':custody,
                                'provenance':{'external_nats_source_fixture':True},'contract':{},'events':[]})
                            # Keep the real qualified receipt and continuation;
                            # compiler target and downstream C1/prerequisite
                            # records are synthetic boundary fixtures here.
                            import realtime_bootstrap
                            capture={'fixture':'prerequisite-capture'}
                            rt={'schema':'voice-staging-realtime-bootstrap-receipt-v2','fixture':'realtime'}
                            prerequisite_dir=root/'installed'/'prerequisites'/session.operation
                            owner.private._private_directory(prerequisite_dir.parent);owner.private._private_directory(prerequisite_dir)
                            owner._write(prerequisite_dir/'capture.json',capture)
                            target={'tag':session.context['commit'],'cohort_preservation_sha256':owner.digest(qualified),
                                    'prerequisites_sha256':owner.digest(capture),'realtime_bootstrap_sha256':owner.digest(rt)}
                            final={**copy.deepcopy(continuation),'target':target,'phase':'AWAITING_OFF_NODE','status':'WAITING',
                                   'cohort_preservation':qualified,'prerequisites':capture,'realtime_bootstrap':rt}
                            prepared['approved']={**approved,'source_sha':session.context['commit']}
                            with patch.object(owner,'_final_target',return_value=target), \
                                 patch.object(owner,'_lineage_snapshot',return_value={'fixture':'source-lineage'}), \
                                 patch.object(owner,'load_prepared',return_value=prepared), \
                                 patch.object(owner.private,'load_capture',return_value=capture), \
                                 patch.object(realtime_bootstrap,'load_completed',return_value=rt):
                                bad=copy.deepcopy(final);bad['custody']={'fixture':'foreign'}
                                with self.assertRaisesRegex(Exception,'cohort_final_lineage_changed'):
                                    owner.issue_final_admission(session,base,bad,request)
                                self.assertFalse((owner._base(session)/'authorizing.json').exists())
                                admission=owner.issue_final_admission(session,base,final,request)
                                self.assertIs(type(admission),owner.QualifiedFinalAdmission)
                            state=owner._state(session);original=copy.deepcopy(state)
                            state['events']=[{'foreign':'event'}];owner._save_state(session,state)
                            with self.assertRaisesRegex(Exception,'cohort_continuation_changed'):
                                owner.load_rollout_continuation(session)
                            owner._save_state(session,original)
                        restored=owner._read(owner._base(session)/'decrypted-restore.json')
                        self.assertEqual(restored['cohort_readback']['cleanup'],'verified')
                        self.assertNotIn('cohort-before.key',restored['members'])
                        offnode=owner._read(owner._base(session)/'offnode-restore.json')
                        self.assertNotEqual(offnode['nonce'],receipt['nonce'])
                        self.assertEqual(offnode['proof']['databases'],receipt['proof']['databases'])
                        owner._no_pending(session)
                        altered=copy.deepcopy(offnode);altered['status']='PENDING'
                        owner.save(owner._base(session)/'offnode-restore.json',altered)
                        with self.assertRaisesRegex(Exception,'cohort_qualification_changed'):
                            owner.load_qualified(session)
        finally:
            docker('rm','-f','-v',cid)


if __name__=='__main__':unittest.main()
