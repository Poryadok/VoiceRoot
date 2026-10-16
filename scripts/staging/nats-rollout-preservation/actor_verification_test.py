"""Private verifier receipt and isolated authentication must bind the same inputs."""
import hashlib,json,os,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import actor_verification as actor

class Tests(unittest.TestCase):
    def fixture(self):
        creds=b'-----BEGIN NATS USER JWT-----\ntoken\n------END NATS USER JWT------\n'
        effective={'pub':{'allow':['social.friend_removed'],'deny':None},'sub':{'allow':['_INBOX.>'],'deny':None},'response':None}
        receipt={'effective':effective,'actor_sha256':'a'*64,'account_sha256':'b'*64,
            'credential_sha256':actor.sha(creds),'account_jwt_sha256':actor.sha(b'account'),
            'operator_sha256':actor.sha(b'operator'),'actor_jwt_sha256':actor.sha(b'token'),'signed_chain_verified':True}
        auth={'server_authentication_verified':True,'credential_sha256':actor.sha(creds),'account_sha256':'b'*64,'server_image':'pinned-fixture'}
        return creds,receipt,auth

    def run_case(self,receipt,auth):
        creds,_,_=self.fixture()
        with tempfile.TemporaryDirectory() as td:
            binary=Path(td)/'binary';binary.write_bytes(b'fixture-binary')
            def regular(p,**kw):return os.open(p,os.O_RDONLY),p.stat()
            with patch.object(actor,'regular',side_effect=regular),patch.object(actor,'invoke',return_value=json.dumps(receipt).encode()) as invoke:
                result=actor.verify(binary,actor.sha(b'fixture-binary'),creds,'account','operator','a'*64,'b'*64,
                    {'pub':['social.friend_removed'],'sub':['_INBOX.reply']},lambda c:auth,lambda:None)
                self.assertEqual(invoke.call_args.kwargs['mode'],'--verify-existing-actor')
                self.assertNotIn('signer_seed',invoke.call_args.args[1])
                return result

    def test_matching_signed_language_and_auth_remain_inference_only(self):
        _,receipt,auth=self.fixture();result=self.run_case(receipt,auth)
        self.assertTrue(result['broker_grant_compatibility_verified'])
        self.assertFalse(result['publication_authorization_exercised'])
        self.assertFalse(result['business_delivery_exercised'])

    def test_changed_chain_credential_identity_or_auth_cannot_promote(self):
        for field in ('actor_sha256','account_sha256','credential_sha256','account_jwt_sha256','operator_sha256','actor_jwt_sha256'):
            _,receipt,auth=self.fixture();receipt[field]='c'*64
            with self.subTest(field=field),self.assertRaises(actor.Blocked):self.run_case(receipt,auth)
        _,receipt,auth=self.fixture();auth['server_authentication_verified']=False
        with self.assertRaises(actor.Blocked):self.run_case(receipt,auth)

    def test_missing_business_language_is_not_fixed_by_successful_auth(self):
        _,receipt,auth=self.fixture();receipt['effective']['pub']['deny']=['social.*']
        with self.assertRaises(ValueError):self.run_case(receipt,auth)

if __name__=='__main__':unittest.main()
