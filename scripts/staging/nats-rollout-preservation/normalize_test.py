import copy
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch
from apply import digest
import normalize
import gis_first_install as gis
from controller import Blocked

class NormalizeTest(unittest.TestCase):
    def setUp(self):
        self.row={'kind':'Deployment','metadata':{'name':'voice-gateway','namespace':'voice-staging'},
                  'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'gateway','image':'repo@sha256:'+'a'*64}]}}}}
        current=copy.deepcopy(self.row);current['metadata']['uid']='aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
        self.stage=SimpleNamespace(expected={'source_claim':'same'},snapshots={'voice-gateway':current})
        self.target={'template_hashes':{'voice-gateway':digest(self.row['spec']['template'])},
                     'images':{'voice-gateway/gateway':'repo@sha256:'+'a'*64}}
        self.kube=Mock();self.kube.get.return_value=copy.deepcopy(current)
        defaulted=copy.deepcopy(current);defaulted['spec']['replicas']=0
        defaulted['spec']['template']['spec']['restartPolicy']='Always'
        self.kube.run.return_value=defaulted
    def test_preserves_input_hash_and_binds_admission_defaulted_live_hash(self):
        result=normalize.normalize_target(self.kube,self.stage,[self.row],self.target)
        self.assertEqual(result['input_template_hashes'],self.target['template_hashes'])
        self.assertNotEqual(result['template_hashes'],self.target['template_hashes'])
        self.assertIn('--dry-run=server',self.kube.run.call_args.args[0])
        self.assertEqual(self.kube.run.call_args.kwargs['body']['spec']['replicas'],0)
    def test_uid_reuse_and_unenrolled_deployment_veto_before_dry_run(self):
        self.kube.get.return_value['metadata']['uid']='bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'
        with self.assertRaises(Blocked):normalize.normalize_target(self.kube,self.stage,[self.row],self.target)
        self.kube.run.assert_not_called()
    def test_admission_image_or_replicas_drift_is_not_authorized(self):
        for kind in ('image','replicas'):
            with self.subTest(kind=kind):
                row=copy.deepcopy(self.kube.run.return_value)
                if kind=='image':row['spec']['template']['spec']['containers'][0]['image']='unexpected'
                else:row['spec']['replicas']=1
                self.kube.run.return_value=row
                with self.assertRaises(Blocked):normalize.normalize_target(self.kube,self.stage,[self.row],self.target)
    def test_gis_normalized_target_is_captured_only_after_identity_validation(self):
        row = copy.deepcopy(self.row)
        row['metadata']['name'] = 'voice-gameintegration'
        row['spec']['template']['spec']['containers'][0]['name'] = 'gameintegration'
        current = copy.deepcopy(row)
        current['metadata']['uid'] = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
        self.stage.snapshots = {'voice-gameintegration': current}
        service = {'apiVersion': 'v1', 'kind': 'Service', 'metadata': {'name': 'voice-gameintegration',
            'namespace': 'voice-staging', 'uid': 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'}, 'spec': {'selector': {'app': 'gis'}}}
        self.kube.get.side_effect = lambda kind, _name: copy.deepcopy(service if kind == 'service' else current)
        normalized = copy.deepcopy(current)
        normalized['spec']['replicas'] = 0
        normalized['metadata']['annotations'] = {'kubectl.kubernetes.io/last-applied-configuration': 'server-result'}
        self.kube.run.return_value = copy.deepcopy(normalized)
        target = {'template_hashes': {'voice-gameintegration': digest(row['spec']['template'])},
                  'images': {'voice-gameintegration/gameintegration': 'repo@sha256:' + 'a' * 64}}
        binding={'service_uid':service['metadata']['uid'],'service':gis.semantic(service)}
        sink = Mock()
        with patch('gis_existing_update.verify_phase') as verify:
            result=normalize.normalize_target(self.kube,self.stage,[row],target,gis_target_sink=sink,gis_binding=binding)
        self.assertEqual(verify.call_count,2)
        self.assertEqual(sink.call_args.args[0], normalized)
        self.assertEqual(result['gis_existing_normalized_sha256'], digest({'deployment_mode':'selected-paused','deployment_uid':current['metadata']['uid'], 'service_uid':service['metadata']['uid'], 'deployment':gis.semantic(normalized), 'service':gis.semantic(service)}))
        self.kube.run.return_value['metadata']['uid'] = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'
        sink.reset_mock()
        with patch('gis_existing_update.verify_phase'), self.assertRaises(Blocked):
            normalize.normalize_target(self.kube,self.stage,[row],target,gis_target_sink=sink,gis_binding=binding)
        sink.assert_not_called()
        self.kube.run.return_value = copy.deepcopy(normalized)
        service['spec']['selector']['app'] = 'gis'
        service['metadata']['uid'] = 'cccccccc-cccc-cccc-cccc-cccccccccccc'
        with patch('gis_existing_update.verify_phase'), self.assertRaises(Blocked):
            normalize.normalize_target(self.kube,self.stage,[row],target,gis_target_sink=sink,gis_binding=binding)
        sink.assert_not_called()
        service['metadata']['uid'] = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'
        service['spec']['selector']['app'] = 'changed'
        with patch('gis_existing_update.verify_phase'), self.assertRaises(Blocked):
            normalize.normalize_target(self.kube,self.stage,[row],target,gis_target_sink=sink,gis_binding=binding)
        sink.assert_not_called()
    def test_images_only_preserves_actual_env_refs_and_strategy(self):
        self.stage.snapshots['voice-gateway']['spec']['strategy']={'type':'Recreate'}
        self.stage.snapshots['voice-gateway']['spec']['template']['spec']['containers'][0]['env']=[{'name':'OLD','value':'preserve'}]
        wanted=copy.deepcopy(self.row);wanted['spec']['template']['spec']['containers'][0]['env']=[{'name':'NEW','value':'never-apply'}]
        target=copy.deepcopy(self.target);target['template_hashes']['voice-gateway']=digest(wanted['spec']['template'])
        rows,updated=normalize.image_only_documents(self.stage,[wanted],target)
        self.assertEqual(rows[0]['spec']['strategy'],{'type':'Recreate'})
        self.assertEqual(rows[0]['spec']['template']['spec']['containers'][0]['env'],[{'name':'OLD','value':'preserve'}])
        self.assertEqual(updated['manifest_sha256'],digest(rows))

if __name__=='__main__':unittest.main()
