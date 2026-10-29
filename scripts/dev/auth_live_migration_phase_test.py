import copy
import json
import hashlib
import base64
import hashlib
from pathlib import Path
import unittest
from unittest.mock import patch
from types import SimpleNamespace
import auth_live_migration_phase as live

from auth_live_migration_phase import operation_name, classify_history, verify_transition, expected_history, validate_clone_receipt, c1_objects
import auth_live_migration_phase_probe as probe


class AuthMigrationContract(unittest.TestCase):
    def test_writer_job_and_pod_are_bound_to_exact_template_and_owner(self):
        operation='media-c1-'+'a'*32
        _,authored=live.c1_objects(operation,{'V26__a.sql':b'SELECT 1;'},'node-one')
        job=copy.deepcopy(authored); job['metadata'].update(uid='job-uid',resourceVersion='4')
        pod={'metadata':{'name':'pod','namespace':'voice-dev','uid':'pod-uid',
             'labels':copy.deepcopy(job['spec']['template']['metadata']['labels']),
             'ownerReferences':[{'apiVersion':'batch/v1','kind':'Job','name':job['metadata']['name'],'uid':'job-uid','controller':True}]},
             'spec':copy.deepcopy(job['spec']['template']['spec']),'status':{'phase':'Running','podIP':'10.0.0.2',
             'containerStatuses':[{'name':'operation','imageID':'sha256:'+'c'*64,'restartCount':0}]}}
        execution={'node_name':'node-one','platform_digest':'sha256:'+'c'*64,'container_name':'operation'}
        self.assertEqual(live.verify_writer_job(authored,job)['uid'],'job-uid')
        self.assertEqual(live.verify_writer_pod(authored,job,pod,execution)['uid'],'pod-uid')
        bad=copy.deepcopy(pod); bad['metadata']['ownerReferences'][0]['uid']='foreign'
        with self.assertRaises(live.PhaseError): live.verify_writer_pod(authored,job,bad,execution)
        bad=copy.deepcopy(pod); bad['spec']['containers'][0]['args']=['different']
        with self.assertRaises(live.PhaseError): live.verify_writer_pod(authored,job,bad,execution)

    def test_sql_configmap_verifier_requires_exact_immutable_source(self):
        cm,_=live.c1_objects('media-c1-'+'a'*32,{'V26__a.sql':b'SELECT 1;'},'node-one')
        actual=copy.deepcopy(cm); actual['metadata'].update(uid='cm-uid',resourceVersion='2')
        self.assertEqual(live.verify_writer_configmap(cm,actual)['uid'],'cm-uid')
        actual['data']['auth-V26__a.sql']='SELECT 2;'
        with self.assertRaises(live.PhaseError): live.verify_writer_configmap(cm,actual)
    def test_c1_effect_intent_is_durable_and_lost_ack_poisoned(self):
        class Ledger:
            def __init__(self,record): self.row={'auth_live_c1':copy.deepcopy(record)}; self.rev=0; self.fail=False
            def read(self): return copy.deepcopy(self.row),self.rev
            def write(self,rev,row):
                self.row=copy.deepcopy(row); self.rev+=1
                if self.fail: raise RuntimeError('lost response')
        record={'status':'active','configmap':{'state':'absent','name':'cm','spec_sha256':'a'*64,'uid':None,'resourceVersion':None},
                'job':{'state':'absent','name':'job','spec_sha256':'b'*64,'uid':None,'resourceVersion':None}}
        journal=SimpleNamespace(state={'steps':{'auth-live-c1':copy.deepcopy(record)}},save=lambda:None)
        ledger=Ledger(record); phase=SimpleNamespace(phases=SimpleNamespace(journal=journal,fence=SimpleNamespace(ledger=ledger)))
        with patch.object(live,'verify_writer_successor',side_effect=lambda *args:{'record':copy.deepcopy(ledger.row['auth_live_c1'])}), \
             patch.object(live.readback,'_poison') as poison:
            result=live._c1_resource_transition(phase,'.','configmap','pending')
            self.assertEqual(result['configmap']['state'],'pending')
            ledger.fail=True
            with self.assertRaises(live.PhaseError): live._c1_resource_transition(phase,'.','configmap','owned',('uid','2'))
            poison.assert_called_once_with(phase)
    def test_private_credential_proof_requires_actual_login_and_stable_secret(self):
        engine={k:'x' for k in probe.metadata.ENGINE_IDENTITY}
        engine.update(namespace='voice-dev',pod_name='voice-postgres-0',container_name='postgres',restart_count=0)
        secret={'metadata':{'name':'voice-app-secrets','namespace':'voice-dev','uid':'secret-uid','resourceVersion':'5'},
                'data':{'POSTGRES_PASSWORD':base64.b64encode(b'fixture-password').decode()}}
        class Client:
            def call(self,*args): return copy.deepcopy(secret)
        result={'database':'auth_db','database_oid':42,'server_start':'start','role':'voice'}
        with patch.object(probe.metadata,'kube_engine_snapshot',return_value=lambda:copy.deepcopy(engine)), \
             patch.object(probe,'_login',return_value=result) as login:
            proof=probe.verify_login(Client(),{'engine':engine,'database_oid':42,'server_start':'start'},lambda:None)
            self.assertEqual(proof,{'uid':'secret-uid','resourceVersion':'5'})
            self.assertNotIn('fixture-password',str(proof))
            login.assert_called_once_with(b'fixture-password')
            with patch.object(probe,'_login',return_value={**result,'role':'other'}),self.assertRaises(probe.ProbeError):
                probe.verify_login(Client(),{'engine':engine,'database_oid':42,'server_start':'start'},lambda:None)
    def test_c1_init_is_durable_and_zero_session_only(self):
        class Ledger:
            def __init__(self): self.row={}; self.rev=0
            def read(self): return copy.deepcopy(self.row),self.rev
            def write(self,rev,row):
                if rev!=self.rev: raise ValueError()
                self.row=copy.deepcopy(row); self.rev+=1
        class Journal:
            def __init__(self): self.state={'steps':{'cohort-auth-flyway-private':{},'cohort-auth-flyway':{}}}
            def save(self): pass
        operation='media-c1-'+'a'*32; source='b'*40
        binding={'operation_id':operation,'snapshot_id':'s','source_sha':source,'databases':['auth_db'],
                 'sources':{'auth_db':{'engine':{},'database_oid':42,'server_start':'x'}}}
        metadata={'migrations':{'auth_db':{'files':{}}},'catalog':{'auth_db':{'history':
            [{k:r[k] for k in ('version','script','checksum','success')} for r in self.before]}}}
        tool={'node_name':'node-one','node_uid':'node-id','flyway_platform_digest':'sha256:'+'c'*64}
        ledger=Ledger(); journal=Journal(); p=SimpleNamespace(journal=journal,intent={'auth_live_tool':tool},
            fence=SimpleNamespace(ledger=ledger),client=object()); phase=SimpleNamespace(phases=p)
        observed={'history':self.before,'sessions':[],'prepared':0}
        with patch.object(live,'_phase_identity'),patch.object(live,'_package_authority'),patch.object(live.readback,'_guard'), \
             patch.object(live,'verified_sources',return_value=(binding,{})),patch.object(live,'source_catalog',return_value=({'V26__x.sql':b'sql'},{})), \
             patch.object(live,'candidate_blob',return_value=b'{}'),patch.object(live,'expected_history',return_value=self.before), \
             patch.object(live.eligibility,'observe_candidate',return_value={}),patch.object(live.probe,'inspect',return_value=observed), \
             patch.object(live,'validate_clone_receipt'),patch.object(live,'flyway_platform',return_value=tool['flyway_platform_digest']), \
             patch.object(live.probe,'verify_login',return_value={'uid':'secret-uid','resourceVersion':'5'}), \
             patch.object(live,'observe_node',return_value={'name':'node-one','uid':'node-id'}),patch.object(live,'verify_writer_successor'):
            record=live.init_c1(phase,lambda:None,'.','actor','token',metadata)
            self.assertEqual(record['job']['state'],'absent')
            self.assertEqual(ledger.row['auth_live_c1'],record)
            with self.assertRaises(live.PhaseError): live.init_c1(phase,lambda:None,'.','actor','token',metadata)
    def test_writer_successor_reconstructs_objects_and_rejects_forged_ledger(self):
        operation='media-c1-'+'a'*32; source='b'*40
        tool={'node_name':'node-one','node_uid':'node-id','flyway_platform_digest':'sha256:'+'c'*64}
        cm,job=live.c1_objects(operation,{'V26__authority.sql':b'SELECT 1;'},tool['node_name'])
        inputs={'candidate':{'source_sha':source,'image':'img','policy':{'flyway':'disabled','persistence':'jdbc','selector':'absent'}},
                'metadata':{'migrations':{'auth_db':{'files':{}}}},'history':self.before,'provenance':{},
                'node':{'name':'node-one','uid':'node-id'},'credential':{'uid':'secret-uid','resourceVersion':'5'},
                'clone_receipt_sha256':live._digest({})}
        def obj(x): return {'state':'absent','name':x['metadata']['name'],'spec_sha256':live._digest(x),'uid':None,'resourceVersion':None}
        record={'status':'active','operation_id':operation,'snapshot_id':'snapshot','source_sha':source,'nonce':'e'*32,
                'input_sha256':live._digest(inputs),'terminal_sha256':None,'configmap':obj(cm),'job':obj(job)}
        ledger=SimpleNamespace(read=lambda:( {'auth_live_c1':copy.deepcopy(record)},('ledger','1')))
        p=SimpleNamespace(intent={'auth_live_tool':tool},journal=SimpleNamespace(state={'steps':{'auth-live-c1':copy.deepcopy(record),
            'auth-live-c1-inputs':copy.deepcopy(inputs),'cohort-auth-flyway-private':{}}}),
            fence=SimpleNamespace(context={'commit':source,'operation_id':operation,'images':{'voice-auth/auth':'img'}},ledger=ledger))
        p.journal.intent=p.intent
        phase=SimpleNamespace(phases=p,_guard=lambda:None)
        binding={'operation_id':operation,'source_sha':source,'snapshot_id':'snapshot',
                 'sources':{'auth_db':{'database_oid':42}}}
        with patch.object(live,'_phase_identity'),patch.object(live,'_package_authority'),patch.object(live,'verified_sources',return_value=(binding,{})), \
             patch.object(live,'source_catalog',return_value=({'V26__authority.sql':b'SELECT 1;'},{})), \
             patch.object(live,'validate_clone_receipt'):
            p.journal.state['steps']['cohort-auth-flyway']={}
            ledger.read=lambda:({'auth_live_c1':copy.deepcopy(record),'cohort_auth_flyway':{}},('ledger','1'))
            result=live.verify_writer_successor(phase,'.')
            self.assertEqual(result['job'],job)
            for kind in ('configmap','job'):
                record[kind].update(state='owned',uid=kind+'-uid',resourceVersion='2')
            clone={'after_history':self.after}; inputs['clone_receipt_sha256']=live._digest(clone)
            record['input_sha256']=live._digest(inputs)
            terminal={'operation_id':operation,'snapshot_id':'snapshot','source_sha':source,'job_uid':'job-uid',
                      'pod_uid':'pod-uid','pod_ip':'10.0.0.3','node_uid':'node-id','image_id':'sha256:'+'c'*64,
                      'before_history':self.before,'after_history':self.after,'clone_receipt_sha256':live._digest(clone),
                      'sessions':[]}
            record['terminal_sha256']=live._digest(terminal)
            p.journal.state['steps'].update({'auth-live-c1':copy.deepcopy(record),'auth-live-c1-inputs':copy.deepcopy(inputs),
                'cohort-auth-flyway-private':clone,'auth-live-c1-terminal':terminal})
            self.assertEqual(live.verify_writer_terminal(phase,'.')['pod_uid'],'pod-uid')
            record['configmap']['state']='removed'; record['job']['state']='removed'; record['status']='done'
            p.journal.state['steps']['auth-live-c1']=copy.deepcopy(record)
            self.assertEqual(live.verify_writer_cleanup(phase,'.')['job']['uid'],'job-uid')
            p.journal.state['steps']['auth-live-c1']['job']['name']='foreign'
            with self.assertRaises(live.PhaseError): live.verify_writer_successor(phase,'.')
    def test_noop_holds_without_protected_package_authority(self):
        phase=SimpleNamespace(phases=SimpleNamespace())
        with self.assertRaises(live.PhaseError): live.no_op_c2(phase,lambda:None,'.','actor','token',{})

    def test_node_snapshot_requires_exact_pg_pod_node_and_ready_amd64(self):
        pod={'metadata':{'uid':'pg-id','name':'voice-postgres-0','namespace':'voice-dev'},'spec':{'nodeName':'node-one'}}
        node={'metadata':{'name':'node-one','uid':'node-id','resourceVersion':'5'},'spec':{},'status':{
              'nodeInfo':{'operatingSystem':'linux','architecture':'amd64'},
              'conditions':[{'type':'Ready','status':'True'}]}}
        class Client:
            def call(self,*args): return pod if args[1]=='pod' else node
        intent={'node_name':'node-one','node_uid':'node-id','flyway_platform_digest':'sha256:'+'a'*64}
        self.assertEqual(live.observe_node(Client(),{'pod_uid':'pg-id'},intent,lambda:None)['uid'],'node-id')
        node['status']['nodeInfo']['architecture']='arm64'
        with self.assertRaises(live.PhaseError): live.observe_node(Client(),{'pod_uid':'pg-id'},intent,lambda:None)
    def test_flyway_index_and_platform_bytes_are_verified(self):
        encode=lambda x:json.dumps(x,separators=(',',':')).encode()
        sha=lambda x:'sha256:'+hashlib.sha256(x).hexdigest()
        child=encode({'schemaVersion':2,'config':{'digest':'sha256:'+'a'*64,'size':12}})
        index=encode({'manifests':[{'digest':sha(child),'size':len(child),'platform':{'os':'linux','architecture':'amd64'}}]})
        with patch.object(live,'_docker_raw',side_effect=[index,child]):
            self.assertEqual(live.flyway_platform(sha(index)),sha(child))
        with patch.object(live,'_docker_raw',side_effect=[index,child+b' ']):
            with self.assertRaises(live.PhaseError): live.flyway_platform(sha(index))
    def test_fixed_c1_job_binds_full_operation_and_source(self):
        operation='media-c1-'+'a'*32
        cm,job=c1_objects(operation,{'V26__authority.sql':b'SELECT 1;'},'node-one')
        self.assertEqual(cm['immutable'],True)
        self.assertEqual(cm['data'],{'auth-V26__authority.sql':'SELECT 1;'})
        for obj in (cm,job): self.assertEqual(obj['metadata']['annotations']['voice.dev/cohort-operation'],operation)
        self.assertIn('-target=32 migrate',job['spec']['template']['spec']['containers'][0]['args'][0])
        self.assertEqual(job['spec']['template']['spec']['nodeName'],'node-one')
    def test_c2_noop_records_result_without_a_job_and_rejects_replay(self):
        class Ledger:
            def __init__(self): self.row={}; self.revision=0
            def read(self): return copy.deepcopy(self.row),self.revision
            def write(self,rev,row):
                if rev!=self.revision: raise ValueError()
                self.row=copy.deepcopy(row); self.revision+=1
        class Journal:
            def __init__(self): self.state={'steps':{}}
            def save(self): pass
        ledger=Ledger(); journal=Journal()
        phase=SimpleNamespace(phases=SimpleNamespace(journal=journal,fence=SimpleNamespace(ledger=ledger)))
        binding={'operation_id':'media-c2-'+'a'*32,'snapshot_id':'s','source_sha':'b'*40,
                 'databases':['auth_db'],'sources':{'auth_db':{'engine':{},'database_oid':42,'server_start':'x'}}}
        observation={'history':self.after,'sessions':[],'prepared':0}
        metadata={'migrations':{'auth_db':{'files':{}}},'catalog':{'auth_db':{'history':
            [{k:r[k] for k in ('version','script','checksum','success')} for r in self.after]}}}
        with patch.object(live,'_phase_identity'),patch.object(live,'_package_authority'),patch.object(live,'c2_sources',return_value=(binding,{'marker':'m'})), \
             patch.object(live,'source_catalog',return_value=({},{})),patch.object(live,'candidate_blob',return_value=b'{}'), \
             patch.object(live,'expected_history',return_value=self.after),patch.object(live.eligibility,'observe_candidate',return_value={'policy':'checked'}), \
             patch.object(live.probe,'inspect',return_value=observation),patch.object(live.readback,'_guard'):
            result=live.no_op_c2(phase,lambda:None,'.','actor','token',metadata)
            self.assertEqual(result['status'],'no-op')
            self.assertEqual(ledger.row['auth_live_c2']['status'],'done')
            with self.assertRaises(live.PhaseError): live.no_op_c2(phase,lambda:None,'.','actor','token',metadata)
            clean=Ledger(); newjournal=Journal()
            fresh=SimpleNamespace(phases=SimpleNamespace(journal=newjournal,fence=SimpleNamespace(ledger=clean)))
            with patch.object(live.probe,'inspect',return_value={**observation,'sessions':[{'pid':123}]}):
                with self.assertRaises(live.PhaseError): live.no_op_c2(fresh,lambda:None,'.','actor','token',metadata)
            self.assertEqual(clean.row,{})
            self.assertEqual(newjournal.state['steps'],{})
            def lost_write(rev,row):
                clean.row=copy.deepcopy(row)
                raise RuntimeError('lost response')
            with patch.object(clean,'write',side_effect=lost_write),patch.object(live.readback,'_poison') as poison:
                with self.assertRaises(live.PhaseError): live.no_op_c2(fresh,lambda:None,'.','actor','token',metadata)
                poison.assert_called_once_with(fresh)
    def setUp(self):
        self.before = [{'version':str(n), 'script':f'V{n}__fixture.sql', 'checksum':n,
                        'type':'SQL', 'success':True} for n in (1, 26)]
        self.appended = [{'version':str(n), 'script':f'V{n}__fixture.sql', 'checksum':n,
                          'type':'SQL', 'success':True} for n in (28, 31, 32)]
        self.after = self.before + self.appended

    def test_short_job_name_binds_full_operation(self):
        a='media-c1-'+'a'*32; b='media-c1-'+'b'*32
        self.assertEqual(len(operation_name(a)),31)
        self.assertNotEqual(operation_name(a),operation_name(b))
        for bad in ('media-c3-'+'a'*32, 'media-c1-'+'A'*32, 'bad'):
            with self.assertRaises(ValueError): operation_name(bad)

    def test_c1_transition_preserves_full_prefix_and_clone_checksums(self):
        self.assertEqual(classify_history(self.before),26)
        self.assertEqual(classify_history(self.after),32)
        verify_transition(self.before,self.after,self.after)
        for altered in (self.after[:-1], [dict(self.before[0],checksum=99)] + self.after[1:],
                        self.before + [dict(self.appended[0],checksum=99)] + self.appended[1:],
                        self.before + [dict(self.appended[0],success=False)] + self.appended[1:]):
            with self.assertRaises(ValueError): verify_transition(self.before,altered,self.after)
        before=[dict(row,installed_on='2026-01-01') for row in self.before]
        after=before+self.appended
        verify_transition(before,after,after)
        changed=copy.deepcopy(after); changed[0]['installed_on']='2026-02-01'
        with self.assertRaises(ValueError): verify_transition(before,changed,after)

    def test_unknown_dirty_or_reordered_history_cannot_admit(self):
        for rows in ([], self.before[:-1], self.after[::-1], self.before+[dict(self.appended[0],success=False)],
                     self.before+[self.appended[0],self.appended[2]]):
            with self.subTest(rows=rows), self.assertRaises(ValueError): classify_history(rows)

    def test_profile_binds_complete_real_source_catalog(self):
        root=Path(__file__).resolve().parents[2] / 'src/backend/auth/src/main/resources'
        catalog={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in (root/'db/migration').glob('*.sql')}
        profile=(root/'roleidentityauthority/migration-history-linked-oauth-v1.json').read_bytes()
        before=expected_history(catalog,profile,26)
        after=expected_history(catalog,profile,32)
        verify_transition(before,after,after)
        for changed in ({**catalog,'V28__role_identity_owner.sql':'0'*64},
                        {**catalog,'V99__extra.sql':'0'*64}):
            with self.assertRaises(ValueError): expected_history(changed,profile,32)

    def test_clone_evidence_is_bound_to_full_receipt_and_actual_history(self):
        from frontend_runtime_executor import digest
        receipt={'protocol':'voice-auth-flyway-cohort-clone-v1','operation_id':'operation',
                 'snapshot_id':'snapshot','source_sha':'a'*40,'target':32,'plan_sha256':'b'*64,
                 'manifest_sha256':'c'*64,'custody_sha256':'d'*64,'restore_sha256':'e'*64,
                 'metadata_sha256':'f'*64,'source_catalog':{'V.sql':'1'*64},
                 'scope':'isolated-auth-clone-only','cleanup':'confirmed',
                 'before_history':self.before,'after_history':self.after}
        record={'status':'done','stage':'done','operation_id':'operation','snapshot_id':'snapshot',
                'source_sha':'a'*40,'plan_sha256':'b'*64,'manifest_sha256':'c'*64,'custody_sha256':'d'*64,
                'restore_sha256':'e'*64,'metadata_sha256':'f'*64,'receipt_sha256':digest(receipt),
                'resources':[{'state':'removed'}]*4}
        validate_clone_receipt(receipt,record,record,'operation','snapshot','a'*40,{'V.sql':'1'*64},self.before)
        bad=copy.deepcopy(receipt); bad['after_history'][-1]['checksum']=0
        with self.assertRaises(ValueError):
            validate_clone_receipt(bad,record,record,'operation','snapshot','a'*40,{'V.sql':'1'*64},self.before)

    def test_observer_rejects_changed_engine_oid_role_and_excess_sessions(self):
        engine={k:'x' for k in probe.metadata.ENGINE_IDENTITY}
        engine.update(namespace='voice-dev',pod_name='voice-postgres-0',container_name='postgres',restart_count=0)
        scope={'engine':engine,'databases':['auth_db'],'database_oids':{'auth_db':42},'server_start':'start'}
        value={'database':'auth_db','database_oid':42,'server_start':'start','role':'voice','read_only':'on',
               'can_read':True,'matched_databases':1,'prepared':0,'other_ledger':False,'history':self.before,'sessions':[]}
        with patch.object(probe.metadata,'kube_engine_snapshot',return_value=lambda:copy.deepcopy(engine)), \
             patch.object(probe,'_query',return_value=value):
            self.assertEqual(probe.inspect(scope,lambda:None),value)
            for field,bad in [('database_oid',43),('role','other'),('server_start','other'),
                              ('can_read',False),('other_ledger',True),('sessions',[{}]*101)]:
                changed=copy.deepcopy(value); changed[field]=bad
                with patch.object(probe,'_query',return_value=changed),self.assertRaises(probe.ProbeError):
                    probe.inspect(scope,lambda:None)


if __name__ == '__main__': unittest.main()
