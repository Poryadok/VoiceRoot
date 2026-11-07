import base64
import json
import os
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
    def do_GET(self):
        self.server.calls.append((self.path,None,self.headers.get('Authorization')))
        if self.path!='/api/v1/spaces': self.send_error(404); return
        spaces=[] if getattr(self.server,'hide_membership',False) else [{'id':self.space}, {'id':'unrelated-not-recorded'}]
        raw=json.dumps({'space_list':{'spaces':spaces}}).encode()
        self.send_response(200); self.send_header('Content-Length',str(len(raw))); self.end_headers(); self.wfile.write(raw)
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
        self.mail_impl=smoke.fixture_mail
        self.patches=[patch.object(smoke,'DEV_API','http://127.0.0.1:'+str(self.server.server_port)),
                      patch.object(smoke,'fixture_mail',side_effect=lambda client,email:{'to':email,'text':'code 123456'}),
                      patch.object(smoke.frontend_runtime_executor,'KubectlTransport',fixture.Client),
                      patch.object(smoke.resource_fencing,'KubeLedger',fixture.Ledger),
                      patch.dict('sys.modules',{'full_cohort_release':self.release_module})]
        for p in self.patches: p.start(); self.addCleanup(p.stop)
        self.client=fixture.Client(self.base.lock); self.client.journal=self.j; self.base.ledger.client=self.client
        self.phase=smoke.CoreSmoke(self.j,self.base.fence,self.client,self.base.lock,'ns',self.release,self.token)

    def test_real_http_owned_users_space_invite_and_no_replay(self):
        result=self.phase.run_dev()
        self.assertEqual(result['space_id'],Server.space)
        self.assertEqual(result['member_profile_id'],Server.accounts[1][1])
        self.assertEqual(len(self.server.registered),2)
        self.assertFalse(any(path=='/api/v1/auth/otp/send' for path,_,_ in self.server.calls), 'initial registration delivers the code without resending')
        self.assertEqual(self.server.calls[-1][0],'/api/v1/spaces')
        self.assertEqual(self.server.calls[-1][2],'Bearer '+token(*Server.accounts[1]))
        self.assertNotIn('unrelated-not-recorded',json.dumps(self.j.state))
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

    def test_join_response_without_membership_readback_holds(self):
        self.server.hide_membership=True
        with self.assertRaises(smoke.SmokeError): self.phase.run_dev()
        self.assertEqual(self.base.ledger.row['status'],'uncertain')
        self.assertEqual(self.server.calls[-1][0],'/api/v1/spaces')
        before=len(self.server.calls)
        with self.assertRaises(smoke.SmokeError): self.phase.run_dev()
        self.assertEqual(len(self.server.calls),before)

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
        self.assertEqual(len(self.server.calls),0)
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
        self.assertEqual(len(self.server.calls),0)
        self.assertNotIn('core_smoke',self.base.ledger.row)

    def test_completed_proof_rejects_changed_private_receipt(self):
        self.phase.run_dev()
        self.j.state['steps']['core-smoke']['result']['member_profile_id']=Server.accounts[0][1]
        with self.assertRaises(smoke.SmokeError): smoke.verify_completed(self.phase,self.token)

    def test_closed_private_journal_cannot_authorize_completed_proof(self):
        self.phase.run_dev()
        self.j.close()
        with self.assertRaises(smoke.SmokeError): smoke.verify_completed(self.phase,self.token)

    def test_release_closes_journal_before_http_no_effect(self):
        original=self.release_module.verify_completed
        count=0
        def closes(phase,token):
            nonlocal count
            count+=1
            result=original(phase,token)
            if count==4: self.j.close()
            return result
        self.release_module.verify_completed=closes
        with self.assertRaises(smoke.SmokeError): self.phase.run_dev()
        self.assertEqual(len(self.server.calls),0)

    def test_close_during_final_authority_read_refuses_proof(self):
        self.phase.run_dev()
        expected=self.j.state['steps']['core-smoke']['ledger']
        original=self.base.ledger.read
        def closes():
            value=original()
            self.j.close()
            return value
        with patch.object(self.base.ledger,'read',side_effect=closes):
            with self.assertRaises(smoke.SmokeError): self.phase._row(expected)

    def test_close_during_final_guard_refuses_authority(self):
        original=self.client.call
        seen=0
        def closes(*args):
            nonlocal seen
            value=original(*args)
            if args[1]=='configmap':
                seen+=1
                if seen==2: self.j.close()
            return value
        with patch.object(self.client,'call',side_effect=closes):
            with self.assertRaises(smoke.SmokeError): self.phase._guard()

    def test_fixture_mail_ignores_proxy_environment_and_is_bounded(self):
        class MailClient:
            def call(self,*args):
                return {'metadata':{'name':'voice-dev-mail','namespace':'voice-dev','uid':'mail-uid','resourceVersion':'5'},
                        'spec':{'clusterIP':'10.43.1.2','ports':[{'port':4180}]}}
        seen=[]
        class Response:
            def __enter__(self): return self
            def __exit__(self,*args): pass
            def read(self,n):
                seen.append(n)
                return b'{"to":"owned@voice-qa.test","text":"123456"}'
        class Opener:
            def open(self,url,timeout):
                seen.append(url)
                return Response()
        def build(*handlers):
            seen.append(handlers[0].proxies)
            return Opener()
        with patch.dict(os.environ,{'HTTP_PROXY':'http://foreign.invalid:9000','http_proxy':'http://foreign.invalid:9000'}), \
                patch.object(smoke.urllib.request,'build_opener',side_effect=build):
            result=self.mail_impl(MailClient(),'owned@voice-qa.test')
        self.assertEqual(seen[0],{})
        self.assertTrue(seen[1].startswith('http://10.43.1.2:4180/emails/latest?to='))
        self.assertEqual(seen[2],65537)
        self.assertEqual(result['to'],'owned@voice-qa.test')
        class DriftMailClient(MailClient):
            count=0
            def call(self,*args):
                self.count+=1
                obj=super().call(*args)
                if self.count==2: obj['metadata']['resourceVersion']='6'
                return obj
        with patch.object(smoke.urllib.request,'build_opener',side_effect=build):
            with self.assertRaises(smoke.SmokeError): self.mail_impl(DriftMailClient(),'owned@voice-qa.test')


if __name__=='__main__': unittest.main()
