import base64
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import threading
import types
import unittest
from unittest.mock import patch

import media_backup_phase_test as fixture
import media_operation_journal as journal
import core_postrollout_smoke as smoke


def token(account, profile):
    payload = base64.urlsafe_b64encode(json.dumps({'sub': account, 'profile_id': profile}).encode()).rstrip(b'=').decode()
    return 'fixture.' + payload + '.fixture'


class Server(BaseHTTPRequestHandler):
    accounts = [('11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222'),
                ('33333333-3333-4333-8333-333333333333','44444444-4444-4444-8444-444444444444')]
    space = '55555555-5555-4555-8555-555555555555'
    invite = '66666666-6666-4666-8666-666666666666'
    def log_message(self, *args): pass
    def do_POST(self):
        length = int(self.headers.get('Content-Length', '0'))
        body = json.loads(self.rfile.read(length) or b'{}')
        self.server.calls.append((self.path, body, self.headers.get('Authorization')))
        if self.path == '/api/v1/auth/register':
            index = len(self.server.registered); self.server.registered[body['email']] = index
            a,p = self.accounts[index]; result = {'session': {'account_id':a,'profile_id':p,'access_token':token(a,p)}}
        elif self.path == '/api/v1/auth/otp/send':
            self.send_response(204); self.end_headers(); return
        elif self.path == '/api/v1/auth/otp/verify': result = {}
        elif self.path == '/api/v1/auth/login':
            a,p = self.accounts[self.server.registered[body['email']]]
            result = {'session': {'account_id':a,'profile_id':p,'access_token':token(a,p),'account_type':'regular'}}
        elif self.path == '/api/v1/spaces':
            result = {'space': {'id':self.space,'owner_profile_id':self.accounts[0][1]}}
        elif self.path == '/api/v1/spaces/'+self.space+'/invites':
            result = {'invite': {'id':self.invite,'space_id':self.space,'creator_profile_id':self.accounts[0][1],'code':'fixture-code'}}
        elif self.path == '/api/v1/invites/fixture-code/join':
            result = {'space_membership': {'space_id':self.space,'profile_id':self.accounts[1][1]}}
        else: self.send_error(404); return
        raw=json.dumps(result).encode(); self.send_response(200); self.send_header('Content-Length',str(len(raw))); self.end_headers(); self.wfile.write(raw)


class CoreSmokeTest(unittest.TestCase):
    def setUp(self):
        self.base=fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay'); self.base.setUp(); self.addCleanup(self.base.doCleanups)
        self.server=ThreadingHTTPServer(('127.0.0.1',0),Server); self.server.calls=[]; self.server.registered={}
        thread=threading.Thread(target=self.server.serve_forever,daemon=True); thread.start()
        self.addCleanup(self.server.server_close); self.addCleanup(self.server.shutdown)
        p=self.base
        intent={'resource_context':p.context,'kube_lock':p.lock,'namespace_uid':'ns','core_smoke':{'environment':'dev','source_sha':p.context['commit']}}
        self.j=journal.Journal(Path(p.temp.name)/'smoke-journal',intent); self.addCleanup(self.j.close)
        self.release=object(); self.token=object()
        receipt={'operation_id':p.context['operation_id'],'source_sha':p.context['commit'],'namespace_uid':'ns','package_sha256':'b'*64}
        self.release_module=types.SimpleNamespace(verify_completed=lambda phase,token: receipt if phase is self.release and token is self.token else None)
        self.patches=[patch.object(smoke,'DEV_API','http://127.0.0.1:'+str(self.server.server_port)),
                      patch.object(smoke,'fixture_mail',side_effect=lambda client,email:{'to':email,'text':'code 123456'}),
                      patch.dict('sys.modules',{'full_cohort_release':self.release_module})]
        for p in self.patches: p.start(); self.addCleanup(p.stop)
        self.phase=smoke.CoreSmoke(self.j,self.base.fence,fixture.Client(self.base.lock),self.base.lock,'ns',self.release,self.token)

    def test_real_http_owned_users_space_invite_and_no_replay(self):
        result=self.phase.run_dev()
        self.assertEqual(result['space_id'],Server.space)
        self.assertEqual(result['member_profile_id'],Server.accounts[1][1])
        self.assertEqual(len(self.server.registered),2)
        self.assertEqual(self.server.calls[-1][0],'/api/v1/invites/fixture-code/join')
        self.assertEqual(smoke.verify_completed(self.phase,self.token)['status'],'done')
        before=len(self.server.calls)
        with self.assertRaises(smoke.SmokeError): self.phase.run_dev()
        self.assertEqual(len(self.server.calls),before)

    def test_foreign_membership_is_uncertain_and_not_replayed(self):
        original=smoke.DevHTTP.request
        def foreign(http,method,path,body=None,token=None,expected=(200,)):
            result=original(http,method,path,body,token,expected)
            if path.endswith('/join'): result['space_membership']['profile_id']=Server.accounts[0][1]
            return result
        with patch.object(smoke.DevHTTP,'request',foreign):
            with self.assertRaises(smoke.SmokeError): self.phase.run_dev()
        self.assertEqual(self.base.ledger.row['status'],'uncertain')
        before=len(self.server.calls)
        with self.assertRaises(smoke.SmokeError): self.phase.run_dev()
        self.assertEqual(len(self.server.calls),before)

    def test_lost_space_response_holds_and_never_replays(self):
        original=smoke.DevHTTP.request
        def lost(http,method,path,body=None,token=None,expected=(200,)):
            result=original(http,method,path,body,token,expected)
            if path == '/api/v1/spaces': raise TimeoutError('response lost after creation')
            return result
        with patch.object(smoke.DevHTTP,'request',lost):
            with self.assertRaises(smoke.SmokeError): self.phase.run_dev()
        self.assertEqual(sum(path=='/api/v1/spaces' for path,_,_ in self.server.calls),1)
        self.assertEqual(self.base.ledger.row['status'],'uncertain')
        with self.assertRaises(smoke.SmokeError): self.phase.run_dev()
        self.assertEqual(sum(path=='/api/v1/spaces' for path,_,_ in self.server.calls),1)

    def test_release_change_blocks_effect_and_holds(self):
        original=self.release_module.verify_completed
        count=0
        def drift(phase,token):
            nonlocal count
            count+=1
            value=original(phase,token)
            return value if count<3 else value | {'package_sha256':'c'*64}
        self.release_module.verify_completed=drift
        with self.assertRaises(smoke.SmokeError): self.phase.run_dev()
        self.assertEqual(self.server.calls,[])
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_second_journal_cannot_start_same_operation(self):
        self.phase.run_dev()
        other=journal.Journal(Path(self.base.temp.name)/'other-smoke',self.j.intent)
        self.addCleanup(other.close)
        second=smoke.CoreSmoke(other,self.base.fence,fixture.Client(self.base.lock),self.base.lock,'ns',self.release,self.token)
        before=len(self.server.calls)
        with self.assertRaises(smoke.SmokeError): second.run_dev()
        self.assertEqual(len(self.server.calls),before)

    def test_missing_release_authority_prevents_any_effect(self):
        self.release_module.verify_completed=lambda phase,token: None
        with self.assertRaises(smoke.SmokeError): self.phase.run_dev()
        self.assertEqual(self.server.calls,[])
        self.assertNotIn('core_smoke',self.base.ledger.row)

    def test_completed_proof_rejects_changed_private_receipt(self):
        self.phase.run_dev()
        self.j.state['steps']['core-smoke']['result']['member_profile_id']=Server.accounts[0][1]
        with self.assertRaises(smoke.SmokeError): smoke.verify_completed(self.phase,self.token)


if __name__=='__main__': unittest.main()
