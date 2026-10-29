import copy
import base64
import hashlib
import json
from types import SimpleNamespace
import unittest
from unittest.mock import Mock,patch

import staging_space_native as native


class InspectSourceTest(unittest.TestCase):
    def test_rejects_admission_injected_container_or_host_access(self):
        session=SimpleNamespace(operation='a'*12)
        tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
              'source_inventory':{'build_source_sha256':'d'*64}}
        wanted=native._job(session,tool,'node-a')['spec']['template']['spec']
        native._check_template(copy.deepcopy(wanted),wanted)
        for field,value in [('initContainers',[{'name':'foreign'}]),('hostNetwork',True),('volumes',[{'name':'foreign'}])]:
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
        observe=[('admin\n'+hashlib.sha256(b'private').hexdigest()+'  -\n').encode(),json.dumps({'items':[pod]}).encode(),json.dumps(result).encode()]
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
