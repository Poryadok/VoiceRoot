import base64
import copy
import hashlib
import unittest
from bootstrap_enrollment import enroll, EnrollmentError, EnrollmentKube

class Kube:
    def __init__(self):self.rows={};self.creates=0
    def optional_secret(self,name):return copy.deepcopy(self.rows.get(name))
    def create_secret(self,row):
        self.creates+=1
        name=row['metadata']['name']
        if name in self.rows:raise RuntimeError('already_exists')
        self.rows[name]=copy.deepcopy(row)
        self.rows[name]['metadata'].update(uid='new-uid',resourceVersion='2')
class Tests(unittest.TestCase):
    def setUp(self):
        self.k=Kube();self.events=[]
        self.binding={'generation':'r20260930a4','marker_uid':'marker','marker_rv':'1',
            'old_secret':{'name':'old','uid':'old-uid','resourceVersion':'1','credential_sha256':hashlib.sha256(b'old').hexdigest()},
            'actor_sha256':'a'*64,'account_sha256':'b'*64}
        self.k.rows['old']={'immutable':True,'type':'Opaque','metadata':{'name':'old','uid':'old-uid','resourceVersion':'1'},'data':{'bootstrap.creds':base64.b64encode(b'old').decode()}}
    def run_enroll(self,proof=lambda creds:True):
        return enroll(self.k,self.binding,b'new',lambda:self.events.append('binding'),proof,lambda r:self.events.append(('commit',r)))
    def test_create_and_auth_before_root_enrollment(self):
        def proof(creds):self.assertEqual(creds,b'new');self.assertEqual(self.k.creates,1);self.events.append('auth');return True
        r=self.run_enroll(proof);self.assertTrue(r['verified']);self.assertEqual(self.events[-2],'binding');self.assertEqual(self.events[-1][0],'commit');self.assertIn('auth',self.events)
        self.assertEqual(self.k.rows['old']['metadata']['uid'],'old-uid')
    def test_crash_after_create_does_not_enroll_and_exact_retry_authenticates(self):
        def crash(creds):raise RuntimeError('interrupted')
        with self.assertRaises(RuntimeError):self.run_enroll(crash)
        self.assertFalse(any(isinstance(e,tuple) for e in self.events));self.run_enroll();self.assertEqual(self.k.creates,1)
    def test_failed_auth_never_enrolls(self):
        with self.assertRaises(EnrollmentError):self.run_enroll(lambda creds:False)
        self.assertFalse(any(isinstance(e,tuple) for e in self.events))
    def test_old_secret_identity_or_bytes_drift_refused_before_create(self):
        self.k.rows['old']['metadata']['resourceVersion']='changed'
        with self.assertRaises(EnrollmentError):self.run_enroll()
        self.assertEqual(self.k.creates,0)
    def test_conflicting_immutable_orphan_never_overwritten(self):
        self.run_enroll();name=next(n for n in self.k.rows if n!='old');self.k.rows[name]['data']['bootstrap.creds']=base64.b64encode(b'foreign').decode()
        with self.assertRaises(EnrollmentError):self.run_enroll()
        self.assertEqual(self.k.creates,1)
    def test_real_adapter_uses_bounded_name_selector_and_create_only(self):
        calls=[]
        class Adapter:
            def run(self,args,body=None):calls.append((args,body));return {'items':[]}
        k=EnrollmentKube(Adapter());self.assertIsNone(k.optional_secret('voice-nats-bootstrap-credentials-r20260930a4'))
        self.assertEqual(calls[0][0],['get','secrets','--field-selector=metadata.name=voice-nats-bootstrap-credentials-r20260930a4','-o','json'])
        with self.assertRaises(EnrollmentError):k.optional_secret('voice-app-secrets')
        self.assertEqual(len(calls),1)
if __name__=='__main__':unittest.main()
