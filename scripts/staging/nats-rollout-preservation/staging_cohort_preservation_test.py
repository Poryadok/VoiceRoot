"""Protected root preservation tests; external Kubernetes responses are fixtures."""
import copy
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
import sys
from types import SimpleNamespace
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
import resource_claim
from resource_claim_test import context, card
import guard
import commands
import staging_cohort_preservation as preservation


class CutEffectTests(unittest.TestCase):
    def fixture(self, root, before,namespace_uid='namespace'):
        c=context(); current=card(c)
        journal=resource_claim.RootJournal(root)
        session=resource_claim.Session(c,'a'*12,'b'*64,lambda:copy.deepcopy(current),journal)
        session.admit(start=True)
        path=root/'installed'/'cohort-preservation'/session.operation
        preservation.private._private_directory(path.parent)
        preservation.private._private_directory(path)
        precut={'context':{'original_snapshots':{'voice-user':before},'marker':{}}}
        plan={key:'c'*64 for key in preservation.PLAN_FIELDS}
        plan.update(schema=preservation.SCHEMA,operation=session.operation,context=c,
            helper_sha256=session.helper,namespace_uid=namespace_uid,actual_before=current['observed']['images'],
            candidate={'commit':c['commit'],'images':c['images']},precut_capture_sha256=preservation.digest(precut))
        preservation._write(path/'precut.json',precut)
        preservation._write(path/'checkpoint.json',{'schema':'voice-staging-cohort-preservation-checkpoint-v1','plan':plan})
        session.bind_cohort_preservation(plan,plan['actual_before'])
        return session,current

    def test_response_uid_generation_or_spec_drift_remains_pending(self):
        for fault in ('uid','generation','image'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory(dir='/root') as directory:
                root=Path(directory);(root/'installed').mkdir(mode=0o700)
                with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                    before={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-user','namespace':'voice-staging',
                        'uid':'user','resourceVersion':'3','generation':1},'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'user','image':'old'}]}}}}
                    session,_=self.fixture(root,before)
                    after=copy.deepcopy(before);after['spec']['replicas']=0;after['metadata'].update(resourceVersion='4',generation=2)
                    if fault=='uid':after['metadata']['uid']='foreign'
                    elif fault=='generation':after['metadata']['generation']=3
                    else:after['spec']['template']['spec']['containers'][0]['image']='foreign'
                    def submit(argv,**kwargs):
                        commands.authority_guard(argv)
                        return json.dumps(after).encode()
                    with resource_claim.command_scope(session.check),patch.object(commands,'capture',side_effect=submit):
                        with self.assertRaisesRegex(Exception,'cohort_cut_response_changed'):
                            preservation.cut_cas(session,'deployment',before,[{'op':'replace','path':'/spec/replicas','value':0}])
                        with self.assertRaisesRegex(Exception,'reconciliation_required'):
                            preservation.version_read(session)
                    record=preservation._read(root/'installed'/'cohort-preservation'/session.operation/'effects'/'cut-deployment-voice-user.json')
                    self.assertEqual(record['status'],'PENDING')

    def test_foreign_snapshot_and_tampered_request_refused_before_api(self):
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                before={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-user','namespace':'voice-staging',
                    'uid':'user','resourceVersion':'3','generation':1},'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'user','image':'old'}]}}}}
                session,_=self.fixture(root,before)
                with resource_claim.command_scope(session.check),patch.object(commands,'capture') as submit:
                    foreign=copy.deepcopy(before);foreign['metadata']['uid']='other'
                    with self.assertRaisesRegex(Exception,'cohort_cut_scope_changed'):
                        preservation.cut_cas(session,'deployment',foreign,[{'op':'replace','path':'/spec/replicas','value':0}])
                    path=preservation._base(session)/'effects';preservation.private._private_directory(path)
                    argv=preservation.PREFIX+['patch','deployment','voice-user','--type=json','--patch-file=/dev/stdin','-o','json']
                    preservation._write(path/'cut-test.json',{'status':'PENDING','operation':session.operation,
                        'effect_id':'cut-test','binding':session.binding(),'argv':argv,'body_sha256':'f'*64})
                    with self.assertRaisesRegex(Exception,'cohort_effect_changed'):
                        preservation._send(session,'cut-test',argv,b'[]')
                    submit.assert_not_called()

    def test_lost_cut_response_retains_pending_and_forbids_replay(self):
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                before={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-user','namespace':'voice-staging',
                    'uid':'user','resourceVersion':'3','generation':1},'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'user','image':'registry/user@sha256:'+'b'*64}]}}}}
                session,current=self.fixture(root,before)
                after=copy.deepcopy(before);after['spec']['replicas']=0;after['metadata'].update(resourceVersion='4',generation=2)
                path=root/'installed'/'cohort-preservation'/session.operation
                calls=[]
                def submit(argv,*,body,**kwargs):
                    commands.authority_guard(argv)
                    record=json.loads((path/'effects'/'cut-deployment-voice-user.json').read_bytes())
                    self.assertEqual(record['status'],'PENDING');calls.append((argv,body))
                    raise RuntimeError('applied_response_lost')
                with resource_claim.command_scope(session.check), patch.object(commands,'capture',side_effect=submit):
                    with self.assertRaisesRegex(RuntimeError,'applied_response_lost'):
                        preservation.cut_cas(session,'deployment',before,[{'op':'replace','path':'/spec/replicas','value':0}])
                    with self.assertRaisesRegex(Exception,'reconciliation_required'):
                        preservation.cut_cas(session,'deployment',before,[{'op':'replace','path':'/spec/replicas','value':0}])
                self.assertEqual(len(calls),1)
                record=json.loads((path/'effects'/'cut-deployment-voice-user.json').read_bytes())
                self.assertEqual(record['status'],'PENDING')

    def test_exact_cut_response_requires_same_uid_and_replica_only_delta(self):
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                before={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-user','namespace':'voice-staging',
                    'uid':'user','resourceVersion':'3','generation':1},'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'user','image':'registry/user@sha256:'+'b'*64}]}}}}
                session,current=self.fixture(root,before)
                after=copy.deepcopy(before);after['spec']['replicas']=0;after['metadata'].update(resourceVersion='4',generation=2)
                def submit(argv,*,body,**kwargs):
                    commands.authority_guard(argv)
                    patch_body=json.loads(body)
                    self.assertEqual(patch_body[:2],[{'op':'test','path':'/metadata/uid','value':'user'},
                        {'op':'test','path':'/metadata/resourceVersion','value':'3'}])
                    return json.dumps(after).encode()
                with resource_claim.command_scope(session.check),patch.object(commands,'capture',side_effect=submit):
                    result=preservation.cut_cas(session,'deployment',before,[{'op':'replace','path':'/spec/replicas','value':0}])
                self.assertEqual(result,after)
                record=json.loads((root/'installed'/'cohort-preservation'/session.operation/'effects'/'cut-deployment-voice-user.json').read_bytes())
                self.assertEqual(record['status'],'DONE');self.assertEqual(record['response'],after)

    def test_isolated_clone_forbids_host_mount_and_lost_create_replay(self):
        import staging_cohort_preservation_runtime as runtime
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                before={'metadata':{'name':'voice-user','namespace':'voice-staging','uid':'user','resourceVersion':'3','generation':1},'spec':{'replicas':1}}
                session,_=self.fixture(root,before)
                with resource_claim.command_scope(session.check):
                    preservation._begin_restore(session,'tooling')
                    preservation._write(preservation._base(session)/'restore-tools.json',{
                        'images':{'postgres':'postgres@sha256:'+'a'*64,'migrate':'migrate@sha256:'+'b'*64},
                        'ids':{'postgres':'sha256:'+'c'*64,'migrate':'sha256:'+'d'*64}})
                    journal=preservation.restore_journal(session);name='voice-media-restore-'+'e'*32
                    journal.begin('migrate',name,{'voice.test.owner':name})
                    argv=runtime.DOCKER+['create','--name',name,'--network','none','--label','voice.test.owner='+name,
                        '--entrypoint','/bin/true','sha256:'+'d'*64]
                    with patch.object(commands,'capture') as capture:
                        foreign=argv[:-1]+['--mount','type=bind,source=/,target=/host',argv[-1]]
                        with self.assertRaisesRegex(Exception,'cohort_restore_create_invalid'):
                            preservation.restore_command(session,foreign)
                        capture.assert_not_called()
                    calls=[]
                    def lost(args,**kwargs):
                        commands.authority_guard(args);calls.append(args)
                        raise RuntimeError('created_reply_lost')
                    with patch.object(commands,'capture',side_effect=lost):
                        with self.assertRaisesRegex(RuntimeError,'created_reply_lost'):
                            preservation.restore_command(session,argv)
                        with self.assertRaisesRegex(Exception,'cohort_reconciliation_required'):
                            preservation.restore_command(session,argv)
                    self.assertEqual(len(calls),1)

    def test_auxiliary_resume_requires_offnode_and_retains_lost_response(self):
        import staging_cohort_preservation_auxiliary as auxiliary
        import staging_p3_prerequisites as p3
        for fault in ('missing_offnode','lost_reply','wrong_image','success'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory(dir='/root') as directory:
                root=Path(directory);(root/'installed').mkdir(mode=0o700)
                with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                    original={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':auxiliary.NAME,'namespace':'voice-staging',
                        'uid':'publisher','resourceVersion':'3','generation':1},'spec':{'replicas':1,
                        'template':{'spec':{'containers':[{'name':'jwks','image':'nginx@sha256:'+'c'*64}]}}}}
                    session,_=self.fixture(root,original)
                    state=preservation._read(preservation._base(session)/'checkpoint.json');state['status']='AWAITING_OFF_NODE'
                    preservation.save(preservation._base(session)/'checkpoint.json',state)
                    paused=copy.deepcopy(original);paused['spec']['replicas']=0;paused['metadata'].update(resourceVersion='4',generation=2)
                    response=copy.deepcopy(original);response['metadata'].update(resourceVersion='5',generation=3)
                    if fault=='wrong_image':response['spec']['template']['spec']['containers'][0]['image']='foreign'
                    child={'preserved':{auxiliary.IDENT:{'observed':{'semantic_sha256':preservation.digest(p3.private_semantic(original))}}}}
                    stage=SimpleNamespace(original_snapshots={auxiliary.NAME:original},snapshots={auxiliary.NAME:paused},
                        kube=SimpleNamespace(get=lambda *args:copy.deepcopy(paused)))
                    calls=[]
                    def send(argv,**kwargs):
                        commands.authority_guard(argv);calls.append(argv)
                        self.assertEqual(json.loads(kwargs['body'])[-1],{'op':'replace','path':'/spec/replicas','value':1})
                        if fault=='lost_reply':raise RuntimeError('resume_applied_reply_lost')
                        return json.dumps(response).encode()
                    with resource_claim.command_scope(session.check),patch.object(auxiliary,'_inputs',return_value=(child,)),\
                        patch.object(preservation,'_paused_stage',return_value=stage),patch.object(auxiliary,'_health',return_value={'owned_pod_fixture':True}),\
                        patch.object(commands,'capture',side_effect=send):
                        if fault=='missing_offnode':
                            with self.assertRaises(FileNotFoundError):auxiliary.resume(session)
                            self.assertEqual(calls,[]);continue
                        with patch.object(preservation,'_offnode_records'):
                            if fault=='success':
                                value=auxiliary.resume(session);self.assertEqual(value['response'],response)
                            else:
                                with self.assertRaises(Exception):auxiliary.resume(session)
                                with self.assertRaisesRegex(Exception,'cohort_reconciliation_required'):auxiliary.resume(session)
                            self.assertEqual(len(calls),1)


class CatalogTests(unittest.TestCase):
    def test_full_catalog_refuses_active_clients_prepared_or_unreadable_database(self):
        import staging_cohort_preservation_pg as pg
        engine={'pod':{'uid':'pod','container_id':'containerd://'+'a'*64,'restart_count':0},'pvc':{'uid':'pvc'},'pv':{'uid':'pv'}}
        row={'name':'postgres','oid':5,'allowconn':True,'template':False,'encoding':'UTF8','collate':'C','ctype':'C',
            'locale_provider':'c','icu_locale':None,'connection_limit':-1,'owner':'postgres','acl':None,'connect':True,'sessions':0,'prepared':0}
        value={'system_identifier':'123','server_start':'2026-10-29','can_read':True,'databases':[row]}
        self.assertEqual(pg.validate_catalog(value,engine,'namespace','a'*12,'b'*64)['databases'],{'postgres':5})
        for field,changed in (('sessions',1),('prepared',1),('connect',False)):
            with self.subTest(field=field):
                bad=copy.deepcopy(value);bad['databases'][0][field]=changed
                with self.assertRaisesRegex(Exception,'cohort_catalog_invalid'):
                    pg.validate_catalog(bad,engine,'namespace','a'*12,'b'*64)


if __name__=='__main__':unittest.main()
