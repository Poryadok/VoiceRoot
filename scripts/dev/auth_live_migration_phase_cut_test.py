"""Two real local journals/fences; Kubernetes, OCI and PostgreSQL are fixtures."""
import copy
import json
import unittest
from pathlib import Path
from unittest.mock import patch

import auth_live_migration_phase as live
import cohort_readback_test as fixture
import cohort_restore_journal as restore
import cohort_upload_phase as upload
import media_backup_phase_test as base_fixture
import media_operation_journal
import resource_fencing
import writer_cut
import writer_cut_migration
import writer_cut_test


class TwoJournalTest(unittest.TestCase):
    def test_auth_full_cut_lifecycle_with_separate_journals(self):
        self.exercise('success')

    def test_job_create_lost_reply_is_uncertain_and_not_replayed(self):
        self.exercise('lost-job-reply')

    def exercise(self, mode):
        original=base_fixture.PhaseTest.setUp
        tool={'node_name':'node-one','node_uid':'node-id','flyway_platform_digest':'sha256:'+'c'*64}
        def setup(base):
            original(base)
            base.context['images']['voice-auth/auth']='ghcr.io/poryadok/voiceroot/auth@sha256:'+'d'*64
            base.intent['auth_live_tool']=copy.deepcopy(tool)
            base.intent['auth_live_candidate']={'platform_digest':'sha256:'+'e'*64,'deployment':{
                'uid':'deployment-uid','resourceVersion':'1','generation':3,
                'spec':{'replicas':0,'template':{'spec':{'containers':[{'name':'auth','image':'old'}]}}}}}
            base.ledger=base_fixture.Ledger(base.context)
            base.fence=resource_fencing.Fence(base.context,lambda:copy.deepcopy(base.card),base.ledger)
            base.lock['data']['resource_context']=json.dumps(base.context,sort_keys=True,separators=(',',':'))
        work=fixture.ReadbackTest('test_real_encrypted_readback_and_no_replay')
        cut_journal=None
        try:
            with patch.object(base_fixture.PhaseTest,'setUp',setup): work.setUp()
            _,manifest=work.extract()
            def qualifier(root,key,binding,databases,manifest,metadata,source_root,postgres,migrate,journal):
                name='voice-media-restore-'+'b'*32; cid='c'*64; labels={'voice.test.owner':name}
                nonce=journal.begin('postgres',name,labels); journal.created('postgres',nonce,cid,labels)
                journal.begin_remove('postgres',cid); journal.removed('postgres',cid,True)
                return {'operation_id':binding['operation_id'],'snapshot_id':binding['snapshot_id'],
                        'source_sha':binding['source_sha'],'manifest_sha256':work.base.ledger.row['cohort_bundle']['manifest_sha256'],
                        'databases':{db:{'mode':'restore-only','from':26,'to':26} for db in databases},
                        'scope':'selected-cohort-roles-catalog-and-ledger-only','cleanup':'verified'}
            with patch.object(upload,'ROOT',work.root),patch.object(restore.restore,'qualify',side_effect=qualifier):
                restore.run(work.phase,work.key,work.destination,manifest,{},'/source','pg','tool',lambda:None)
            history=[{'version':str(n),'script':f'V{n}__fixture.sql','checksum':n,'type':'SQL','success':True} for n in (1,26)]
            after=history+[{'version':str(n),'script':f'V{n}__fixture.sql','checksum':n,'type':'SQL','success':True} for n in (28,31,32)]
            metadata={'migrations':{'auth_db':{'files':{}}},'catalog':{'auth_db':{'history':
                [{k:r[k] for k in ('version','script','checksum','success')} for r in history]}}}
            with patch.object(live.restore,'_source'):
                binding,provenance=live.verified_sources(work.phase,metadata,26)
            clone={**provenance,'before_markers':{},'expected_after_markers':{},'roles_sha256':'f'*64,
                   'before_catalog_sha256':'1'*64,'after_catalog_sha256':'2'*64,'after_history':after}
            work.journal.state['steps']['cohort-auth-flyway-private']=clone
            work.journal.state['steps']['cohort-auth-flyway']={}; work.journal.save()
            work.base.ledger.row['cohort_auth_flyway']={}
            state={'jobs':[],'configmaps':[],'pods':[],'migrated':False}
            class Client(writer_cut_test.Client):
                def call(self,*args,**kwargs):
                    if args[0]=='get' and args[1]=='namespace':
                        value=super().call(*args); value['metadata']['uid']=binding['sources']['auth_db']['engine']['namespace_uid']; return value
                    if args[0]=='get' and args[1] in ('jobs','configmaps','pods'):
                        return {'items':copy.deepcopy(state[args[1]])}
                    if args[0]=='get' and args[1]=='nodes':
                        return {'items':[{'metadata':{'name':'node-one','uid':'node-id'},'status':{'conditions':[{'type':'Ready','status':'True'}]}}]}
                    if args[0]=='get' and args[1] in ('job','configmap') and len(args)>2 and args[2]!='voice-dev-image-update-lock':
                        found=[x for x in state[args[1]+'s'] if x['metadata']['name']==args[2]]
                        return copy.deepcopy(found[0]) if found else {}
                    if args[0]=='create':
                        obj=copy.deepcopy(kwargs['body'])
                        if '--dry-run=server' in args: return obj
                        kind=obj['kind']; obj['metadata'].update(uid=kind+'-uid',resourceVersion='1')
                        if kind=='Job':
                            obj['status']={'succeeded':1,'conditions':[{'type':'Complete','status':'True'}]}
                            template=obj['spec']['template']; state['migrated']=True
                            state['pods']=[{'metadata':{'name':'pod','namespace':'voice-dev','uid':'pod-uid',
                                'labels':copy.deepcopy(template['metadata']['labels']),'ownerReferences':[{
                                    'apiVersion':'batch/v1','kind':'Job','name':obj['metadata']['name'],
                                    'uid':obj['metadata']['uid'],'controller':True}]},'spec':copy.deepcopy(template['spec']),
                                'status':{'phase':'Succeeded','podIP':'10.0.0.3','containerStatuses':[{
                                    'name':'operation','imageID':tool['flyway_platform_digest'],'restartCount':0,
                                    'state':{'terminated':{'exitCode':0}}}]}}]
                        state['jobs' if kind=='Job' else 'configmaps'].append(copy.deepcopy(obj))
                        if kind=='Job' and mode=='lost-job-reply': raise TimeoutError('synthetic lost response')
                        return obj
                    if args[0]=='delete':
                        kind='jobs' if '/jobs/' in args[1] else 'configmaps'; uid=kwargs['body']['preconditions']['uid']
                        if len(state[kind])!=1 or state[kind][0]['metadata']['uid']!=uid: raise ValueError()
                        state[kind]=[]
                        if kind=='jobs': state['pods']=[]
                        return {}
                    return super().call(*args)
            producer=Client(work.base.lock,work.journal); work.media.client=producer; work.base.ledger.client=producer
            source=writer_cut_test.workload('Deployment','voice-a','uid-a',1,'old')
            engine=binding['sources']['auth_db']['engine']
            scope={'engine':engine,'server_start':binding['sources']['auth_db']['server_start'],
                   'databases':binding['databases'],'database_oids':{db:binding['sources'][db]['database_oid'] for db in binding['databases']}}
            admission={'controllers':[source],'guards':{'jobs':[],'cronjobs':[],'hpas':[]},
                       'linked':{'jobs':[],'cronjobs':[]},'session_scope':scope}
            intent={'resource_context':work.base.context,'namespace_uid':engine['namespace_uid'],'writer_cut':admission}
            cut_journal=media_operation_journal.Journal(Path(work.base.temp.name)/'cut-journal',intent)
            cut_client=Client(work.base.lock,cut_journal); cut_client.workloads={('Deployment','voice-a'):copy.deepcopy(source)}
            class LedgerView:
                def __init__(self,client): self.client=client
                def read(self): return work.base.ledger.read()
                def write(self,rev,row): return work.base.ledger.write(rev,row)
            fence=resource_fencing.Fence(work.base.context,lambda:copy.deepcopy(work.base.card),LedgerView(cut_client))
            target=live.eligibility.candidate_target(work.media.intent['auth_live_candidate']['deployment']['spec'],work.base.context['images']['voice-auth/auth'])
            candidate={'source_sha':binding['source_sha'],'image':work.base.context['images']['voice-auth/auth'],
                       'policy':{'flyway':'disabled','persistence':'jdbc','selector':'absent'},
                       'route':{'host':'voice-postgres','database':'auth_db','role':'voice'},
                       'platform_digest':'sha256:'+'e'*64,'target_spec_sha256':live._digest(target),'auth_tree':'a'*40,
                       'deployment':{'uid':'deployment-uid','generation':3,'resourceVersion':'1'}}
            def observation(*args): return {'history':copy.deepcopy(after if state['migrated'] else history),'sessions':[],'prepared':0}
            with patch.object(writer_cut.frontend_runtime_executor,'KubectlTransport',Client), \
                 patch.object(live,'_phase_identity'),patch.object(live,'_package_authority'),patch.object(live.restore,'_source'), \
                 patch.object(live,'source_catalog',return_value=({'V26__x.sql':b'sql'},{})),patch.object(live,'candidate_blob',return_value=b'{}'), \
                 patch.object(live,'expected_history',return_value=history),patch.object(live,'validate_clone_receipt'), \
                 patch.object(live.eligibility,'observe_candidate',return_value=candidate),patch.object(live.probe,'inspect',side_effect=observation), \
                 patch.object(live.eligibility,'_source_tree',return_value='a'*40), \
                 patch.object(live.probe,'verify_login',return_value={'uid':'secret','resourceVersion':'1'}), \
                 patch.object(live,'flyway_platform',return_value=tool['flyway_platform_digest']), \
                 patch.object(live,'observe_node',return_value={'name':'node-one','uid':'node-id'}), \
                 patch.object(live,'_inspect_c1_gate',return_value={'gate':True}), \
                 patch.object(live,'_c1_data_snapshot',side_effect=lambda engine,clone,before,guard:live._expected_c1_data(clone,before)), \
                 patch.object(writer_cut_migration,'sessions',side_effect=lambda cut,state,pod:{'scope':scope,'active_sessions':0,'active_transactions':0,'attributed_sessions':[]}):
                cut=writer_cut.WriterCut(cut_journal,fence,cut_client,work.base.lock,engine['namespace_uid'],admission,
                    lambda s:{'scope':s,'active_sessions':0,'active_transactions':0})
                with patch.object(cut,'_patch',side_effect=cut_client.patch):
                    cut.drain(); live.init_c1(work.phase,cut,'.','actor','token',metadata)
                    cut.bind_migration('auth',work.phase,'.')
                    if mode=='lost-job-reply':
                        with self.assertRaises(live.PhaseError): live.create_c1_objects(work.phase,cut,'.','actor','token')
                        self.assertEqual(work.base.ledger.row['status'],'uncertain')
                        self.assertEqual(work.base.ledger.row['auth_live_c1']['job']['state'],'pending')
                        self.assertEqual(len(state['jobs']),1)
                        with self.assertRaises(live.PhaseError): live.create_c1_objects(work.phase,cut,'.','actor','token')
                        self.assertEqual(len(state['jobs']),1)
                        return
                    live.create_c1_objects(work.phase,cut,'.','actor','token')
                    live.finish_c1(work.phase,cut,'.','actor','token')
                    live.cleanup_c1(work.phase,cut,'.')
                self.assertEqual(work.base.ledger.row['auth_live_c1']['status'],'done')
                self.assertFalse(state['jobs'] or state['pods'] or state['configmaps'])
                self.assertIsNone(cut.migration)
                result=live.verify_completed(work.phase,cut,'.',metadata)
                self.assertEqual(result['status'],'migrated')
                self.assertEqual(result['candidate']['route']['database'],'auth_db')
        finally:
            if cut_journal is not None: cut_journal.close()
            if not work.doCleanups(): raise RuntimeError('fixture cleanup failed')


if __name__=='__main__': unittest.main()
