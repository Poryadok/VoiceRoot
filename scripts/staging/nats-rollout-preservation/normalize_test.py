import copy
from types import SimpleNamespace
import unittest
from unittest.mock import Mock
from apply import digest
import normalize
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
