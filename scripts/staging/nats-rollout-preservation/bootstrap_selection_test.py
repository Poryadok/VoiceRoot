import base64,copy,hashlib,os,sys,tempfile,types,unittest
from pathlib import Path
from unittest.mock import patch
if os.name=='nt':sys.modules.setdefault('fcntl',types.ModuleType('fcntl'))
sys.path.insert(0,str(Path(__file__).parents[1]/'nats-known-baseline'))
import root_main
import bootstrap_selection as selection
from bootstrap_root import GENERATION,ACTOR_SHA
from nats_contract_plan import DECLARATION,digest
from docker_runtime import NATS_IMAGE
from controller import Blocked

def fixture():
    account='A'+'A'*55;sha=hashlib.sha256(account.encode()).hexdigest()
    def secret(name,data):return {'type':'Opaque','immutable':True,'metadata':{'name':name,'uid':name+'-uid','resourceVersion':'1','namespace':'voice-staging'},'data':{k:base64.b64encode(v.encode()).decode() for k,v in data.items()}}
    old=secret('voice-nats-bootstrap-credentials-'+GENERATION,{'bootstrap.creds':'old-private-creds'})
    new=secret('voice-nats-bootstrap-contract-'+'a'*24,{'bootstrap.creds':'renewed-private-creds'})
    new['metadata']['labels']={'voice-nats-generation':GENERATION,'voice-nats-existing-bootstrap-renewal':'true'}
    op=secret('voice-nats-operator-'+GENERATION,{'account.public':account,'system-account.public':'A'+'B'*55,'operator.jwt':'a.b.c','account.jwt':'a.b.c','system-account.jwt':'a.b.c'})
    services=secret('voice-nats-service-credentials-'+GENERATION,{'social.creds':'same-social','realtime.creds':'same-realtime'})
    marker={'metadata':{'uid':'marker-uid','resourceVersion':'4'},'data':{'generation':GENERATION,'phase':'active'}}
    ident=lambda row:{k:row['metadata'][k] for k in ('name','uid','resourceVersion')}
    binding={'generation':GENERATION,'marker_uid':'marker-uid','marker_rv':'4','account_sha256':sha,'actor_sha256':ACTOR_SHA,'old_secret':ident(old)|{'credential_sha256':selection.credential_hash(old)},'operator_secret':ident(op)}
    auth={'verified':True,'server_image':NATS_IMAGE,'account_sha256':sha,'reply_prefix':'_INBOX.voice.bootstrap.reply','normalized_consumer':copy.deepcopy(DECLARATION),'normalized_consumer_sha256':digest(DECLARATION)}
    receipt={'schema':'voice-nats-bootstrap-enrollment-v1','verified':True,'binding':binding,'secret':ident(new),'credential_sha256':selection.credential_hash(new),'auth_proof':auth}
    rows={r['metadata']['name']:r for r in (old,new,op,services)}
    class Kube:
        def get(self,kind,name):return copy.deepcopy(marker if kind=='configmap' else rows[name])
        def secret_meta(self,name):return {'uid':'tls-uid','resourceVersion':'1'}
    return Kube(),receipt,rows,sha

class Tests(unittest.TestCase):
    def test_captured_inputs_keep_group_access_under_service_umask(self):
        for mask in (0o022,0o077):
            with self.subTest(umask=oct(mask)),tempfile.TemporaryDirectory() as td:
                kube,receipt,rows,sha=fixture();base=Path(td)
                previous=os.umask(mask)
                try:
                    with patch.object(selection,'ACCOUNT_SHA',sha),patch.object(root_main.os,'chown') as owner:
                        provenance=root_main.capture_inputs(kube,base,{'scripts':[]},GENERATION,bootstrap_enrollment=receipt)
                    inputs=base/'inputs'
                    self.assertEqual(inputs.stat().st_mode&0o777,0o750)
                    owner.assert_any_call(inputs,0,65532)
                    for name,expected in (('bootstrap.creds',b'renewed-private-creds'),('social.creds',b'same-social'),('realtime.creds',b'same-realtime')):
                        path=inputs/name
                        self.assertEqual(path.stat().st_mode&0o777,0o440)
                        self.assertEqual(path.read_bytes(),expected)
                        owner.assert_any_call(path,0,65532)
                        self.assertEqual(provenance[name]['sha256'],hashlib.sha256(expected).hexdigest())
                    self.assertEqual((base/'server.conf').stat().st_mode&0o777,0o440)
                finally:os.umask(previous)

    def capture(self,kube,receipt,sha):
        with tempfile.TemporaryDirectory() as td,patch.object(selection,'ACCOUNT_SHA',sha),patch.object(root_main.os,'chown',create=True):
            provenance=root_main.capture_inputs(kube,Path(td),{'scripts':[]},GENERATION,bootstrap_enrollment=receipt)
            self.assertEqual((Path(td)/'inputs/bootstrap.creds').read_bytes(),b'renewed-private-creds')
            self.assertEqual((Path(td)/'inputs/social.creds').read_bytes(),b'same-social')
            return provenance
    def test_normal_capture_uses_only_enrolled_existing_actor(self):
        kube,receipt,rows,sha=fixture();p=self.capture(kube,receipt,sha)
        self.assertEqual(p['bootstrap.creds']['resource'],receipt['secret']['name'])
        self.assertEqual(p['bootstrap.creds']['uid'],receipt['secret']['uid'])
    def test_changed_new_secret_refused_without_old_fallback(self):
        kube,receipt,rows,sha=fixture();rows[receipt['secret']['name']]['metadata']['resourceVersion']='2'
        with self.assertRaises(Blocked):self.capture(kube,receipt,sha)
    def test_changed_old_chain_or_server_default_proof_refused(self):
        for field in ('old','server','config','reply'):
            kube,receipt,rows,sha=fixture()
            if field=='old':rows[receipt['binding']['old_secret']['name']]['metadata']['resourceVersion']='2'
            elif field=='server':receipt['auth_proof']['server_image']='other'
            elif field=='config':receipt['auth_proof']['normalized_consumer']['max_ack_pending']=7
            else:receipt['auth_proof']['reply_prefix']='other'
            with self.assertRaises(Blocked):self.capture(kube,receipt,sha)
if __name__=='__main__':unittest.main()
