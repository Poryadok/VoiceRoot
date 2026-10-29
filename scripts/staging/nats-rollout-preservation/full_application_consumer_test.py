"""FULL23 compiler-to-normalizer boundary with synthetic Kubernetes only."""
import copy
import unittest
import uuid
from types import SimpleNamespace
from unittest.mock import patch

import apply
import gis_first_install
import normalize
from full_application_compiler_test import FullApplicationCompilerTests


class FullApplicationConsumerTests(unittest.TestCase):
    def test_full_gis_keeps_other_workloads_and_rejects_generic_gis_pair(self):
        fixture=FullApplicationCompilerTests('test_full23_cold_gis_defers_only_create_pair')
        fixture.setUp();self.addCleanup(fixture.doCleanups)
        kube,parameters=fixture.inputs(cold=True);pack=fixture.compile(kube,parameters)
        operation='a'*12;plan=gis_first_install.bind_operation(pack['gis_first_install'],operation,pack['target'])
        original_get=kube.get;original_run=kube.run
        ids={identity:str(uuid.uuid4()) for identity in kube.objects}
        def get(kind,name):
            kind=next((value for value,_ in kube.objects if value.lower()==kind.lower()),kind)
            value=original_get(kind,name);value['metadata']['uid']=ids[kind,name];return value
        def run(args,body=None,timeout=30):
            value=original_run(args,body,timeout)
            if args[0]=='apply' and '--dry-run=server' in args:
                value['metadata']['uid']=ids[value['kind'],value['metadata']['name']]
            return value
        kube.get=get;kube.run=run
        snapshots={name:get('Deployment',name) for kind,name in kube.objects if kind=='Deployment'}
        stage=SimpleNamespace(operation=operation,snapshots=snapshots,expected={'source_claim':parameters['dataPVC']})
        with patch.object(gis_first_install,'preflight'):
            target,normalized=normalize.normalize_first_install(kube,stage,pack['manifests'],pack['target'],plan)
        self.assertEqual(set(target['template_hashes']),set(pack['target']['template_hashes']))
        self.assertEqual(target['images'],pack['target']['images'])
        self.assertEqual(target['gis_first_install_normalized_sha256'],gis_first_install.normalized_binding(normalized))
        paused=apply.paused_documents(pack['manifests'],{'target':target,'pvc':{'name':parameters['dataPVC']}})
        self.assertTrue(any(row['kind']=='Deployment' and row['metadata']['name']=='voice-realtime' for row in paused))
        self.assertTrue(all(row.get('spec',{}).get('replicas')==0 for row in paused if row['kind']=='Deployment'))
        foreign=copy.deepcopy(pack['manifests'])+[copy.deepcopy(plan['objects'][0])]
        with self.assertRaisesRegex(Exception,'gis_first_install_manifest_forbidden'):
            apply.paused_documents(foreign,{'target':target,'pvc':{'name':parameters['dataPVC']}})
        bad=copy.deepcopy(target);del bad['source_hashes']['.github/ci/full-application-build-request.json']
        with self.assertRaisesRegex(Exception,'gis_first_install_manifest_forbidden'):
            apply.paused_documents(pack['manifests'],{'target':bad,'pvc':{'name':parameters['dataPVC']}})


if __name__=='__main__':unittest.main()
