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


class HostedPostgresLifecycleTests(unittest.TestCase):
    """Test-only Docker responses; this does not prove hosted isolation."""
    def test_owned_postgres_requires_password_for_host_authentication(self):
        from types import SimpleNamespace
        adapter,cid,state,docker=self.fixture();created=[]
        def record(*args,**kwargs):
            if args[0]=='create':created.append(args)
            if args[0] in ('start','exec'):return b''
            return docker(*args,**kwargs)
        with patch.object(adapter,'_docker',side_effect=record), \
             patch('cohort_hosted_pg.subprocess.run',return_value=SimpleNamespace(returncode=0)):
            with adapter:self.assertEqual(adapter.cid,cid)
        self.assertEqual(len(created),1)
        self.assertIn('POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256',created[0])

    def fixture(self,*,create_lost=False,remove_lost=None,foreign=False,delayed=False):
        from cohort_hosted_pg import HostedPostgres
        adapter=object.__new__(HostedPostgres);adapter.name='voice-cohort-source-'+'a'*24
        adapter.cid=None;adapter.observed_cid=None;adapter.create_uncertain=False;adapter.reconciliation_error=None
        adapter.original_popen=subprocess.Popen
        cid='b'*64;state={'exists':not create_lost,'removed':0}
        def docker(*args,body=None):
            if args[0]=='create':
                state['exists']=not delayed
                if create_lost:raise RuntimeError('synthetic_create_reply_lost')
                return cid.encode()
            if args[:2]==('ps','-a'):
                return (cid+'\n').encode() if state['exists'] else b''
            if args[0]=='inspect':
                return json.dumps([{'Id':cid,'Name':'/'+adapter.name,
                    'Config':{'Labels':{'voice.test.owner':'foreign' if foreign else adapter.name}}}]).encode()
            if args[0]=='rm':
                state['removed']+=1
                if remove_lost=='before':raise RuntimeError('synthetic_remove_failed')
                state['exists']=False
                if remove_lost=='after':raise RuntimeError('synthetic_remove_reply_lost')
                return b''
            raise AssertionError('unexpected fixture command')
        return adapter,cid,state,docker

    def test_lost_create_reply_preserves_identity_for_reconciliation(self):
        adapter,cid,state,docker=self.fixture(create_lost=True)
        with patch.object(adapter,'_docker',side_effect=docker):
            with self.assertRaisesRegex(RuntimeError,'isolated_postgres_reconciliation_required'):adapter.__enter__()
            with self.assertRaisesRegex(RuntimeError,'isolated_postgres_reconciliation_required') as caught:adapter.__exit__(None,None,None)
            self.assertIn('isolated_postgres_create_terminality_unknown',str(caught.exception.__cause__))
        self.assertTrue(state['exists']);self.assertEqual(state['removed'],0)
        self.assertIsNone(adapter.cid);self.assertEqual(adapter.observed_cid,cid)

    def test_delayed_create_after_empty_scan_still_requires_reconciliation(self):
        adapter,cid,state,docker=self.fixture(create_lost=True,delayed=True)
        with patch.object(adapter,'_docker',side_effect=docker):
            with self.assertRaisesRegex(RuntimeError,'isolated_postgres_reconciliation_required'):adapter.__enter__()
            self.assertIsNone(adapter.observed_cid)
            state['exists']=True
            with self.assertRaisesRegex(RuntimeError,'isolated_postgres_reconciliation_required') as caught:adapter.__exit__(None,None,None)
            self.assertIn('isolated_postgres_create_terminality_unknown',str(caught.exception.__cause__))
        self.assertTrue(state['exists']);self.assertEqual(state['removed'],0)

    def test_remove_failure_retains_identity_and_lost_success_is_reconciled(self):
        for fault in ('before','after'):
            with self.subTest(fault=fault):
                adapter,cid,state,docker=self.fixture(remove_lost=fault);adapter.cid=cid
                with patch.object(adapter,'_docker',side_effect=docker):
                    if fault=='before':
                        with self.assertRaisesRegex(RuntimeError,'isolated_postgres_reconciliation_required') as caught:adapter.__exit__(None,None,None)
                        self.assertIn('synthetic_remove_failed',str(caught.exception.__cause__))
                        self.assertEqual((adapter.cid,adapter.observed_cid),(cid,cid));self.assertTrue(state['exists'])
                    else:
                        adapter.__exit__(None,None,None)
                        self.assertIsNone(adapter.cid);self.assertFalse(state['exists'])

    def test_foreign_label_is_never_removed(self):
        adapter,cid,state,docker=self.fixture(foreign=True);adapter.cid=cid
        with patch.object(adapter,'_docker',side_effect=docker):
            with self.assertRaisesRegex(RuntimeError,'isolated_postgres_reconciliation_required') as caught:adapter.__exit__(None,None,None)
            self.assertIn('isolated_postgres_identity_changed',str(caught.exception.__cause__))
        self.assertTrue(state['exists']);self.assertEqual(state['removed'],0);self.assertEqual((adapter.cid,adapter.observed_cid),(cid,cid))

    def test_inventory_failure_reports_known_custody_during_enter_and_exit(self):
        adapter,cid,state,_=self.fixture()
        def docker(*args,body=None):
            if args[0]=='create':return cid.encode()
            raise RuntimeError('synthetic_inventory_unavailable')
        with patch.object(adapter,'_docker',side_effect=docker):
            with self.assertRaisesRegex(RuntimeError,'isolated_postgres_reconciliation_required') as entered:adapter.__enter__()
            self.assertIn(adapter.name,str(entered.exception));self.assertIn('cid='+cid,str(entered.exception))
            self.assertEqual(str(entered.exception.__cause__),'synthetic_inventory_unavailable')
            self.assertEqual(adapter.cid,cid)
            with self.assertRaisesRegex(RuntimeError,'isolated_postgres_reconciliation_required') as exited:adapter.__exit__(None,None,None)
            self.assertIn(adapter.name,str(exited.exception));self.assertIn('cid='+cid,str(exited.exception))
            self.assertEqual(str(exited.exception.__cause__),'synthetic_inventory_unavailable')
            self.assertEqual(adapter.cid,cid)


@unittest.skipUnless(os.environ.get('VOICE_COHORT_DOCKER')=='1','isolated Docker fixture opt-in')
class RestoreTransportTests(unittest.TestCase):
    def test_public_c1_continuation_issues_typed_final_admission(self):
        """Real issuer/PG/CMS/C1; NATS archive verification, GitHub and Kube are fixtures."""
        from bridge_composition_test import CompositionTests
        from cohort_hosted_pg import HostedPostgres
        with HostedPostgres('00000000-0000-0000-0000-000000000001') as postgres:
            CompositionTests._exercise_public_precut(self,enrolled=True,through_nats=True,through_fence=True,
                through_workload=True,through_hub=True,through_storage=True,postgres=postgres,qualify=True,final_admission=True)

    def test_public_qualified_continuation_reaches_c1_final_prepare(self):
        """PG/CMS and C1 are actual; NATS archive verification, GitHub and Kube are fixtures."""
        from bridge_composition_test import CompositionTests
        from cohort_hosted_pg import HostedPostgres
        with HostedPostgres('00000000-0000-0000-0000-000000000001') as postgres:
            CompositionTests._exercise_public_precut(self,enrolled=True,through_nats=True,through_fence=True,
                through_workload=True,through_hub=True,through_storage=True,postgres=postgres,qualify=True,final_prepare=True)

    def test_public_prepare_to_qualified_continuation_with_external_boundaries(self):
        """Real PG/CMS and Session; synthetic NATS archive/GitHub, stop before final prepare."""
        from bridge_composition_test import CompositionTests
        from cohort_hosted_pg import HostedPostgres
        with HostedPostgres('00000000-0000-0000-0000-000000000001') as postgres:
            CompositionTests._exercise_public_precut(self,enrolled=True,through_nats=True,through_fence=True,
                through_workload=True,through_hub=True,through_storage=True,postgres=postgres,qualify=True)

    def test_public_prepare_reaches_real_postgres_restore_before_native_nats(self):
        """Hosted-only PG proof; source/Kube/NATS stay explicit fixtures.

        This stops before native NATS and off-node CMS qualification, so it is
        not a qualified P2 receipt or an end-to-end authorization proof.
        """
        from bridge_composition_test import CompositionTests
        from cohort_hosted_pg import HostedPostgres
        namespace='00000000-0000-0000-0000-000000000001'
        with HostedPostgres(namespace) as postgres:
            CompositionTests._exercise_public_precut(self,enrolled=True,through_nats=True,through_fence=True,
                through_workload=True,through_hub=True,through_storage=True,postgres=postgres)

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
                            target={'tag':session.context['commit'],'mode':'app-only','cohort_preservation_sha256':owner.digest(qualified),
                                    'prerequisites_sha256':owner.digest(capture),'realtime_bootstrap_sha256':owner.digest(rt)}
                            final={**copy.deepcopy(continuation),'target':target,'phase':'AWAITING_OFF_NODE','status':'WAITING',
                                   'cohort_preservation':qualified,'prerequisites':capture,'realtime_bootstrap':rt,'migrations':[]}
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
                                from unittest.mock import Mock
                                stage=Mock(operation=session.operation)
                                final['phase']='DATABASE_MIGRATIONS'
                                self.assertTrue(owner.validate_migration_admission(admission,session,base,final,stage,[],'app-only'))
                                original_target=final['target'];final['target']=dict(original_target,tag='f'*40)
                                with self.assertRaises(Exception):
                                    owner.validate_migration_admission(admission,session,base,final,stage,[],'app-only')
                                final['target']=original_target;final['phase']='PAUSED_APPLY'
                                authorization={'schema':'nats-rollout-preservation-v2',
                                    'installation_sha256':session.installation.sha256,
                                    'target':target,'resource_fence':session.binding()}
                                for fault in ('legacy-schema','missing-installation','foreign-installation'):
                                    with self.subTest(authorization_fault=fault):
                                        rejected=copy.deepcopy(authorization)
                                        if fault=='legacy-schema':rejected['schema']='nats-rollout-preservation-v1'
                                        elif fault=='missing-installation':del rejected['installation_sha256']
                                        else:rejected['installation_sha256']='f'*64
                                        final['authorization']=rejected
                                        with patch.object(guard,'protected_receipt',return_value=rejected):
                                            with self.assertRaisesRegex(Exception,'cohort_final_authorization_incomplete'):
                                                owner.complete_authorization(admission,session,base,final)
                                        self.assertFalse((owner._base(session)/'authorization-complete.json').exists())
                                final['authorization']=authorization
                                with patch.object(guard,'protected_receipt',return_value=authorization):
                                    owner.complete_authorization(admission,session,base,final)
                                    with self.assertRaisesRegex(Exception,'cohort_final_admission_not_issued'):
                                        owner.complete_authorization(admission,session,base,final)
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
