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
    def test_owned_scale_revision_is_not_replayed_as_apply_authority(self):
        self.row['metadata'].update({'uid':'651b9c66-c423-4ac0-b8b1-30866d5470c3',
            'resourceVersion':'3166782','annotations':{'voice.io/intent':'retained'}})
        source=copy.deepcopy(self.row)
        out=apply.paused_documents([self.row],self.receipt)[0]
        # The own scale-to-zero CAS advanced live RV to3170058. Declarative
        # target apply retains UID/content, without the captured stale CAS RV.
        expected=copy.deepcopy(source);expected['metadata'].pop('resourceVersion');expected['spec']['replicas']=0
        self.assertEqual(out,expected)
        self.assertEqual(self.row,source)
        for kind in ('ConfigMap','Service','NetworkPolicy','Ingress'):
            row={'kind':kind,'metadata':{'name':'retained','namespace':'voice-staging',
                'uid':'651b9c66-c423-4ac0-b8b1-30866d5470c3','resourceVersion':'3166782'},
                'data':{'key':'retained'},'spec':{'retained':'content'}}
            original=copy.deepcopy(row);expected=copy.deepcopy(row);expected['metadata'].pop('resourceVersion')
            self.assertEqual(apply.paused_documents([row],self.receipt),[expected])
            self.assertEqual(row,original)
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

    def test_gis_first_install_is_excluded_from_generic_apply(self):
        operation='d'*12
        service={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-gameintegration',
            'namespace':'voice-staging','annotations':{'voice.poryadok.io/gis-first-install-operation':operation}},
            'spec':{'selector':{'app':'voice-gameintegration'},'ports':[{'name':'http','port':8080}]}}
        deployment={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-gameintegration',
            'namespace':'voice-staging','annotations':{'voice.poryadok.io/gis-first-install-operation':operation}},
            'spec':{'replicas':1,'selector':{'matchLabels':{'app':'voice-gameintegration'}},
                'template':{'metadata':{'labels':{'app':'voice-gameintegration'}},'spec':{'containers':[
                    {'name':'gameintegration','image':'gameintegration@sha256:'+'a'*64}]}}}}
        receipt={'operation':operation,'pvc':{'name':'unused'},'target':{'gis_first_install_sha256':'b'*64}}
        self.assertEqual(apply.paused_documents([],receipt),[])
        with self.assertRaisesRegex(guard.Blocked,'gis_first_install_manifest_forbidden'):
            apply.paused_documents([service,deployment],receipt)
        self.assertEqual(deployment['spec']['replicas'],1)

    def test_gis_first_install_apply_requires_the_exact_pair(self):
        receipt={'operation':'d'*12,'target':{'gis_first_install_sha256':'b'*64}}
        service={'kind':'Service','metadata':{'name':'voice-gameintegration','namespace':'voice-staging'}}
        with self.assertRaisesRegex(guard.Blocked,'gis_first_install_manifest_forbidden'):
            apply.paused_documents([service],receipt)

if __name__=='__main__':unittest.main()
