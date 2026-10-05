import copy
import unittest
import apply
import guard

class PausedTest(unittest.TestCase):
    def setUp(self):
        self.row={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-nats-pvc-candidate','namespace':'voice-staging'},'spec':{'replicas':1,'template':{'metadata':{'labels':{'app':'voice-nats-pvc-candidate'}},'spec':{'containers':[{'name':'nats','image':'nats@sha256:'+'a'*64}],'volumes':[{'name':'jsdata','persistentVolumeClaim':{'claimName':'voice-nats-jsdata-d202610040049430b'}}]}}}}
        self.receipt={'pvc':{'name':'voice-nats-jsdata-d202610040049430b'},'target':{'template_hashes':{'voice-nats-pvc-candidate':apply.digest(self.row['spec']['template'])},'images':{'voice-nats-pvc-candidate/nats':'nats@sha256:'+'a'*64}}}
    def test_verified_target_stays_zero_without_mutating_original(self):
        out=apply.paused_documents([self.row],self.receipt)
        self.assertEqual(out[0]['spec']['replicas'],0);self.assertEqual(self.row['spec']['replicas'],1)
    def test_pvc_object_is_never_applied(self):
        with self.assertRaises(guard.Blocked):apply.paused_documents([{'kind':'PersistentVolumeClaim','metadata':{'namespace':'voice-staging','name':'x'}}],self.receipt)
    def test_job_is_never_started(self):
        with self.assertRaises(guard.Blocked):apply.paused_documents([{'kind':'Job','metadata':{'namespace':'voice-staging','name':'voice-nats-realtime-bootstrap'}}],self.receipt)
    def test_target_template_drift_is_rejected(self):
        self.row['spec']['template']['spec']['containers'][0]['image']='wrong'
        with self.assertRaises(guard.Blocked):apply.paused_documents([self.row],self.receipt)
    def test_claim_swap_is_rejected_even_with_template_hash(self):
        self.row['spec']['template']['spec']['volumes'][0]['persistentVolumeClaim']['claimName']='different'
        self.receipt['target']['template_hashes']['voice-nats-pvc-candidate']=apply.digest(self.row['spec']['template'])
        with self.assertRaises(guard.Blocked):apply.paused_documents([self.row],self.receipt)

if __name__=='__main__':unittest.main()
