import copy
import base64
import hashlib
import json
import tempfile
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import Mock,patch

import staging_space_native as native


class InspectSourceTest(unittest.TestCase):
    def test_delete_response_must_confirm_success_or_same_job(self):
        native._check_delete_response({'apiVersion':'v1','kind':'Status','status':'Success'},'owned')
        native._check_delete_response({'apiVersion':'batch/v1','kind':'Job','metadata':{'uid':'owned'}},'owned')
        for response in ({},{'apiVersion':'v1','kind':'Status','status':'Failure'},
                         {'apiVersion':'batch/v1','kind':'Job','metadata':{'uid':'foreign'}}):
            with self.assertRaisesRegex(native.Blocked,'space_inspect_delete_invalid'):
                native._check_delete_response(response,'owned')

    def test_pod_list_requires_complete_versioned_observation(self):
        for value in ({'apiVersion':'v1','kind':'PodList','metadata':{'resourceVersion':'8','continue':'next'},'items':[]},
                      {'apiVersion':'v1','kind':'PodList','metadata':{},'items':[]}):
            with patch.object(native.owner,'_observation',return_value=json.dumps(value).encode()):
                with self.assertRaisesRegex(native.Blocked,'space_inspect_absence_invalid'):
                    native._pod_list(SimpleNamespace(),'voice-space-inspect-aaaaaaaaaaaa')

    def test_cleanup_rejects_foreign_job_before_delete(self):
        kube=Mock();kube.get.return_value={'metadata':{'uid':'foreign','resourceVersion':'9'}}
        with patch.object(native,'_mutate') as mutate:
            with self.assertRaisesRegex(native.Blocked,'space_inspect_job_changed'):
                native._cleanup(SimpleNamespace(),kube,'voice-space-inspect-aaaaaaaaaaaa','owned')
            mutate.assert_not_called()

    def test_effect_boundary_rejects_foreign_job_delete(self):
        session=SimpleNamespace(operation='a'*12)
        uid='11111111-1111-4111-8111-111111111111'
        kube=Mock();kube.get.return_value={'metadata':{'uid':uid,'resourceVersion':'9'}}
        options={'apiVersion':'v1','kind':'DeleteOptions','propagationPolicy':'Foreground',
                 'preconditions':{'uid':uid,'resourceVersion':'9'}}
        argv=native.owner.PREFIX+['delete','--raw=/apis/batch/v1/namespaces/voice-staging/jobs/voice-space-inspect-'+session.operation,'-f','/dev/stdin']
        with patch.object(native.owner,'_guard'),patch.object(native.owner,'_no_pending'),patch.object(native,'Kube',return_value=kube), \
             patch.object(native.owner,'_base',return_value=Path('/unused')),patch.object(native.owner,'_read',return_value={'status':'DONE','response':{'uid':'foreign'}}), \
             patch.object(native.owner,'_send') as submit:
            with self.assertRaisesRegex(native.Blocked,'space_inspect_effect_invalid'):
                native._mutate(session,'space-inspect-delete',argv,native.owner.private._bytes(options))
            submit.assert_not_called()

    def test_lost_create_reply_stays_pending_and_cannot_be_replayed(self):
        import commands, guard, resource_claim
        from staging_cohort_preservation_test import CutEffectTests
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(native.owner,'_source'):
                session,_=CutEffectTests().fixture(root,{'spec':{'replicas':1}})
                calls=[]
                def lost(argv,**kwargs):
                    commands.authority_guard(argv);calls.append(argv)
                    raise RuntimeError('create_response_lost')
                argv=native.owner.PREFIX+['create','-f','/dev/stdin','-o','json']
                tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
                      'source_inventory':{'build_source_sha256':'d'*64}}
                kube=Mock();kube.get.return_value={'spec':{'nodeName':'node-a'}}
                body=native.owner.private._bytes(native._job(session,tool,'node-a'))
                with resource_claim.command_scope(session.check),patch.object(commands,'capture',side_effect=lost), \
                     patch.object(native,'Kube',return_value=kube),patch.object(native.space,'load_tool',return_value=tool):
                    with self.assertRaisesRegex(native.Blocked,'space_inspect_effect_invalid'):
                        native._mutate(session,'space-inspect-create',argv,b'{}')
                    self.assertEqual(calls,[])
                    with self.assertRaisesRegex(RuntimeError,'create_response_lost'):
                        native._mutate(session,'space-inspect-create',argv,body)
                    with self.assertRaisesRegex(Exception,'cohort_reconciliation_required'):
                        native._mutate(session,'space-inspect-create',argv,body)
                self.assertEqual(len(calls),1)
                record=native.owner._read(native.owner._base(session)/'effects'/'space-inspect-create.json')
                self.assertEqual(record['status'],'PENDING')

    def test_rejects_admission_injected_container_or_host_access(self):
        session=SimpleNamespace(operation='a'*12)
        tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
              'source_inventory':{'build_source_sha256':'d'*64}}
        wanted=native._job(session,tool,'node-a')['spec']['template']['spec']
        native._check_template(copy.deepcopy(wanted),wanted)
        defaulted=copy.deepcopy(wanted)
        defaulted['containers'][0].update(resources={},terminationMessagePath='/dev/termination-log',terminationMessagePolicy='File')
        native._check_template(defaulted,wanted)
        for field,value in [('initContainers',[{'name':'foreign'}]),('hostNetwork',True),('volumes',[{'name':'foreign'}]),
                            ('dnsConfig',{'nameservers':['192.0.2.1']})]:
            changed=copy.deepcopy(wanted);changed[field]=value
            with self.subTest(field=field),self.assertRaisesRegex(native.Blocked,'space_inspect_job_invalid'):
                native._check_template(changed,wanted)

    def test_fixed_inspection_requires_bound_tool_identity_and_owned_cleanup(self):
        session=SimpleNamespace(operation='a'*12,context={'commit':'b'*40})
        tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
              'source_inventory':{'build_source_sha256':'d'*64},'platform_digest':'sha256:'+'c'*64,
              'config_digest':'sha256:'+'e'*64,'content':{'executable_sha256':'f'*64,'migrator_sha256':'1'*64,'sql_inventory_sha256':'2'*64}}
        identity={'namespace_uid':'ns','engine':{},'database':{'system_identifier':'42','server_start':'t','database_oid':123,
                  'database_owner':'admin','session_user':'admin','schema_owner':'admin'}}
        config={'metadata':{'uid':'config','resourceVersion':'1'},'data':{'POSTGRES_USER':'admin'}}
        secret={'metadata':{'uid':'secret','resourceVersion':'2'},'data':{'POSTGRES_PASSWORD':base64.b64encode(b'private').decode()}}
        env=[{'name':'POSTGRES_USER','valueFrom':{'configMapKeyRef':{'name':'voice-app-config','key':'POSTGRES_USER'}}},
             {'name':'POSTGRES_PASSWORD','valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'POSTGRES_PASSWORD'}}}]
        pgpod={'spec':{'nodeName':'node-a','containers':[{'name':'postgres','env':env}]}}
        job=native._job(session,tool,'node-a');uid='11111111-1111-4111-8111-111111111111'
        job['metadata'].update(uid=uid,resourceVersion='7');complete=copy.deepcopy(job);complete['status']={'succeeded':1}
        pod={'metadata':{'name':'inspection-pod','uid':'22222222-2222-4222-8222-222222222222',
             'ownerReferences':[{'kind':'Job','uid':uid,'controller':True}]},
             'spec':copy.deepcopy(job['spec']['template']['spec']),
             'status':{'containerStatuses':[{'name':'inspect','imageID':'fixture@'+tool['platform_digest'],'state':{'terminated':{'exitCode':0}}}]}}
        result={'event':'inspected-legacy24','schema':'voice.space.native-legacy24-inspection.v1',
            'source_revision':session.context['commit'],'build_sha256':'d'*64,'executable_sha256':'f'*64,
            'migrator_sha256':'1'*64,'inventory_sha256':'2'*64,'target':{'database_name':'space_db','database_oid':123,
                'database_system_id':'42','database_owner':'admin','installer_role':'admin','version':24}}
        values={('configmap','voice-app-config'):config,('secret','voice-app-secrets'):secret,('pod','voice-postgres-0'):pgpod,
                ('job',job['metadata']['name']):complete}
        kube=Mock();kube.get.side_effect=lambda kind,name:copy.deepcopy(values[(kind,name)])
        kube.run.return_value=copy.deepcopy(job)
        observe=[('admin\n'+hashlib.sha256(b'private').hexdigest()+'  -\n').encode(),
                 json.dumps({'apiVersion':'v1','kind':'PodList','metadata':{'resourceVersion':'8'},'items':[pod]}).encode(),json.dumps(result).encode()]
        with patch.object(native,'Kube',return_value=kube),patch.object(native.owner,'_guard',return_value={'fixture':'plan'}), \
             patch.object(native.owner,'_no_pending'),patch.object(native.space,'load_tool',return_value=tool), \
             patch.object(native,'_identity',return_value=identity),patch.object(native,'_mutate',return_value=('path',{},json.dumps(job).encode())), \
             patch.object(native,'_done'),patch.object(native.owner,'_observation',side_effect=observe), \
             patch.object(native,'_cleanup') as cleanup,patch.object(native.owner,'_write'):
            receipt=native.inspect_source(session)
        self.assertEqual(receipt['result'],result);self.assertEqual(receipt['cleanup'],'verified')
        cleanup.assert_called_once_with(session,kube,job['metadata']['name'],uid)
        self.assertNotIn('private',json.dumps(job))


if __name__=='__main__':unittest.main()
