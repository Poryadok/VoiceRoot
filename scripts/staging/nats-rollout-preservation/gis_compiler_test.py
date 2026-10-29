"""The first-install source target cannot update any existing resource."""
import copy
import unittest

import compiler
import nonnats_plan
from controller import Blocked


class Producer:
    def __init__(self, rows):
        self.rows=rows
        self.reads=[]

    def get(self, kind, name):
        self.reads.append((kind,name))
        return copy.deepcopy(self.rows[(kind,name)])


class Tests(unittest.TestCase):
    def test_create_only_pair_is_removed_from_generic_apply_after_source_binding(self):
        service={'kind':'Service','metadata':{'name':'voice-gameintegration'}}
        deployment={'kind':'Deployment','metadata':{'name':'voice-gameintegration'}}
        source_plan={'schema':'voice-gis-first-install-source-v1','objects':[service,deployment]}
        pack={'manifests':[service,deployment],'target':{'manifest_sha256':compiler.digest([service,deployment]),
            'gis_first_install_sha256':compiler.digest(source_plan)},'gis_first_install':source_plan}
        result=compiler.exclude_gis_first_install_from_generic_apply(pack)
        self.assertEqual(result['manifests'],[])
        self.assertEqual(result['target']['manifest_sha256'],compiler.digest([]))
        self.assertEqual(result['gis_first_install'],source_plan)

        widened=copy.deepcopy(pack)
        widened['manifests'].append({'kind':'Deployment','metadata':{'name':'voice-notification'}})
        widened['target']['manifest_sha256']=compiler.digest(widened['manifests'])
        with self.assertRaisesRegex(compiler.Blocked,'gis_first_install_generic_apply_binding_invalid'):
            compiler.exclude_gis_first_install_from_generic_apply(widened)

    def test_compiled_first_install_defers_existing_role_notification_realtime_writes(self):
        service={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-gameintegration','namespace':'voice-staging'},'spec':{'type':'ClusterIP'}}
        deployment={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-gameintegration','namespace':'voice-staging'},'spec':{'replicas':0,'template':{'spec':{'containers':[{'name':'gameintegration','image':'repo@sha256:'+'a'*64}]}}}}
        names=('voice-role','voice-notification','voice-realtime')
        current={('Deployment',name):{'apiVersion':'apps/v1','kind':'Deployment',
            'metadata':{'name':name,'namespace':'voice-staging','uid':name+'-uid','resourceVersion':'1'},
            'spec':{'template':{'metadata':{'labels':{'app':name}},'spec':{'containers':[{'name':name.removeprefix('voice-'),'image':'repo@sha256:'+'b'*64}]}}}}
            for name in names}
        actions=[];objects=[]
        for index,name in enumerate(names):
            manifest=copy.deepcopy(current[('Deployment',name)])
            manifest['spec']['template']['spec']['containers'][0]['image']='repo@sha256:'+'c'*64
            action_id='update-'+str(index)
            actions.append({'id':action_id,'row':'app-config','manifest':manifest})
            objects.append({'kind':'Deployment','name':name,'namespace':'voice-staging',
                'desired':nonnats_plan.semantic(manifest),'action_id':action_id,'disposition':'action'})
        plan={'schema':'voice-nonnats-plan-v1','mode':'app-only','actions':actions,
            'checks':[{'id':'app-config','disposition':'action','objects':objects}]}
        plan['checks'].append({'id':'frontends','disposition':'preserve','objects':[
            {'kind':'ConfigMap','name':'voice-web-runtime-config','namespace':'voice-staging',
             'desired':{'kind':'ConfigMap'},'disposition':'create'},
            {'kind':'ConfigMap','name':'existing','namespace':'voice-staging','desired':{'kind':'ConfigMap'}}]})
        producer=Producer(current)
        target,isolated=compiler.isolate_gis_first_install([service,deployment,
            *[action['manifest'] for action in actions]],plan,producer)
        self.assertEqual({(row['kind'],row['metadata']['name']) for row in target},
            {('Service','voice-gameintegration'),('Deployment','voice-gameintegration')})
        self.assertEqual(isolated['actions'],[])
        self.assertEqual([o['name'] for o in isolated['checks'][1]['objects']],['existing'])
        self.assertEqual(producer.reads,[('Deployment',name) for name in names])
        self.assertTrue(all(item['disposition']=='preserve' and 'action_id' not in item
            for item in isolated['checks'][0]['objects']))
        self.assertEqual([item['desired'] for item in isolated['checks'][0]['objects']],
            [nonnats_plan.semantic(current[('Deployment',name)]) for name in names])

    def test_gis_first_install_refuses_incomplete_or_misbound_resource_pair(self):
        plan={'schema':'voice-nonnats-plan-v1','mode':'app-only','actions':[],'checks':[]}
        service={'kind':'Service','metadata':{'name':'voice-gameintegration'}}
        deployment={'kind':'Deployment','metadata':{'name':'voice-gameintegration'}}
        producer=Producer({})
        with self.assertRaisesRegex(Blocked,'manifest_scope_invalid'):
            compiler.isolate_gis_first_install([service],plan,producer)
        with self.assertRaisesRegex(Blocked,'manifest_scope_invalid'):
            compiler.isolate_gis_first_install([service,dict(deployment,metadata={'name':'voice-unrelated'})],plan,producer)


if __name__=='__main__':
    unittest.main()
