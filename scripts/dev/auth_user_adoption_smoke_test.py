import json
import base64
import unittest
from auth_user_adoption import RecoveryRequired
from auth_user_adoption_smoke import public_smoke


class Journal:
    def __init__(self):
        self.state={}
    def save(self):
        pass


class PublicSmokeTest(unittest.TestCase):
    def execute(self,mail_wrong=False,fail=None):
        journal=Journal(); calls=[]
        account='00000000-0000-0000-0000-000000000016'; profile='00000000-0000-0000-0000-000000000116'
        payload=base64.urlsafe_b64encode(json.dumps({'sub':account,'profile_id':profile}).encode()).decode().rstrip('=')
        session={'access_token':'fixture.'+payload+'.private-token','refresh_token':'private-refresh',
                 'account_id':account,'profile_id':profile,'account_type':'regular'}
        def http(method,path,body=None,token=None,expected=(200,)):
            calls.append(path)
            self.assertIn('smoke_account',journal.state,'ownership intent precedes request')
            if fail==path:
                raise RecoveryRequired('HTTP failed')
            if path.endswith('/register') or path.endswith('/login'): return {'session':session}
            if path.endswith('/privacy'): return {'dm_privacy':'friends'}
            return {}
        def mail(email):
            return {'to':'foreign@example.test' if mail_wrong else email,'text':'Code 123456'}
        return journal,calls,http,mail

    def test_real_route_sequence_and_public_receipt_redaction(self):
        journal,calls,http,mail=self.execute()
        result=public_smoke(journal,None,http,mail)
        self.assertEqual(['/api/v1/auth/register','/api/v1/auth/otp/send','/api/v1/auth/otp/verify',
                          '/api/v1/auth/login','/api/v1/users/me/privacy','/api/v1/auth/logout'],calls)
        self.assertEqual('PASS',result['result'])
        self.assertEqual('00000000-0000-0000-0000-000000000116',result['profile_id'])
        serialized=json.dumps(result)
        for value in ['private-token','private-refresh','123456',journal.state['smoke_account']['password']]:
            self.assertNotIn(value,serialized)

    def test_foreign_mail_never_verifies(self):
        journal,calls,http,mail=self.execute(mail_wrong=True)
        with self.assertRaises(RecoveryRequired): public_smoke(journal,None,http,mail)
        self.assertNotIn('/api/v1/auth/otp/verify',calls)

    def test_existing_owned_account_intent_is_never_registered_twice(self):
        journal,calls,http,mail=self.execute(fail='/api/v1/auth/register')
        with self.assertRaises(RecoveryRequired): public_smoke(journal,None,http,mail)
        calls.clear()
        with self.assertRaises(RecoveryRequired): public_smoke(journal,None,http,mail)
        self.assertEqual([],calls)
