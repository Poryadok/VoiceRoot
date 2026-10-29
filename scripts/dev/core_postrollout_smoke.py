"""One fenced DEV public-API smoke. Owned accounts and Space are retained.

The caller owns the current claim, rollout and private journal. The source-owned
full_cohort_release verifier must be present and complete before any effect.
No STAGING credential or mailbox adapter is implied by this DEV-only route.
"""
import base64
import copy
import fcntl
import hashlib
import ipaddress
import json
import os
import re
import secrets
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

import frontend_runtime_executor
import media_operation_guard
import media_operation_journal
import resource_fencing

DEV_API = 'https://api-dev.comrade.click'


class SmokeError(RuntimeError):
    pass


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        raise SmokeError('public API redirect refused')


class DevHTTP:
    def request(self, method, path, body=None, token=None, expected=(200,)):
        if method not in ('POST', 'GET') or not path.startswith('/api/v1/') or '?' in path or '#' in path:
            raise SmokeError('public API route refused')
        headers = {'Content-Type': 'application/json'}
        if token:
            headers['Authorization'] = 'Bearer ' + token
        request = urllib.request.Request(DEV_API + path, data=json.dumps(body).encode() if body is not None else None,
                                         headers=headers, method=method)
        try:
            with urllib.request.build_opener(urllib.request.ProxyHandler({}),_NoRedirect()).open(request, timeout=25) as response:
                if response.status not in expected:
                    raise SmokeError('public API status refused')
                raw = response.read(1048577)
                if len(raw) > 1048576:
                    raise SmokeError('public API response exceeds bound')
                result = json.loads(raw) if raw else {}
                if not isinstance(result, dict):
                    raise SmokeError('public API response invalid')
                return result
        except BaseException:
            raise SmokeError('public API outcome uncertain') from None


def _mail_service_identity(service):
    try:
        if (service['metadata']['name']!='voice-dev-mail' or service['metadata']['namespace']!='voice-dev' or
                not isinstance(service['metadata']['uid'],str) or not service['metadata']['uid'] or
                not isinstance(service['metadata']['resourceVersion'],str) or not service['metadata']['resourceVersion'] or
                not any(p.get('port')==4180 for p in service['spec']['ports'])):
            raise ValueError()
        address=ipaddress.ip_address(service['spec']['clusterIP'])
        if address not in ipaddress.ip_network('10.43.0.0/16'): raise ValueError()
        return (service['metadata']['uid'],service['metadata']['resourceVersion'],str(address),service['spec']['ports'])
    except (KeyError,TypeError,ValueError):
        raise SmokeError('DEV fixture mail identity invalid') from None


def fixture_mail(client, email):
    identity=_mail_service_identity(client.call('get','service','voice-dev-mail','-o','json'))
    address=identity[2]
    url='http://'+str(address)+':4180/emails/latest?to='+urllib.parse.quote(email,safe='')
    opener=urllib.request.build_opener(urllib.request.ProxyHandler({}),_NoRedirect())
    for _ in range(25):
        try:
            with opener.open(url,timeout=5) as response:
                raw=response.read(65537)
                if len(raw)>65536: raise SmokeError('fixture mail response exceeds bound')
                result=json.loads(raw)
                if not isinstance(result,dict): raise SmokeError('fixture mail response invalid')
                if _mail_service_identity(client.call('get','service','voice-dev-mail','-o','json')) != identity:
                    raise SmokeError('DEV fixture mail identity changed')
                return result
        except urllib.error.HTTPError as error:
            if error.code!=404: raise SmokeError('recipient fixture mail unavailable') from None
            time.sleep(.2)
        except BaseException:
            raise SmokeError('recipient fixture mail unavailable') from None
    raise SmokeError('recipient fixture mail not delivered')


def _digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()).hexdigest()


def _uuid(value):
    parsed = uuid.UUID(value)
    if parsed.int == 0 or str(parsed) != value:
        raise SmokeError('public identity invalid')
    return value


class CoreSmoke:
    def __init__(self, journal, fence, client, lock, namespace_uid, release, token):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid, self.release, self.token = copy.deepcopy(lock), namespace_uid, release, token
        self.http = DevHTTP()
        self._identities = (journal, fence, client, release, token, self.http, fence.ledger, fence.check)
        self._intent = copy.deepcopy(journal.intent)
        self._context = copy.deepcopy(fence.context)
        self._directory_identity = self._fd_identity(journal.dirfd)
        self._lock_identity = self._fd_identity(journal.lockfd)
        self.invocation = secrets.token_hex(16)

    @staticmethod
    def _fd_identity(fd):
        info=os.fstat(fd)
        return (info.st_dev,info.st_ino)

    def _custody(self):
        j=self.journal
        if (type(j) is not media_operation_journal.Journal or j.poisoned or j.dirfd is None or j.lockfd is None or
                self._fd_identity(j.dirfd)!=self._directory_identity or self._fd_identity(j.lockfd)!=self._lock_identity or
                j.intent!=self._intent or j.state.get('intent')!=self._intent or self.fence.context!=self._context):
            raise SmokeError('private smoke journal identity changed')
        j._private(os.fstat(j.dirfd),0o700,directory=True)
        j._private(os.fstat(j.lockfd),0o600)
        path_info=os.stat(j.path,follow_symlinks=False)
        lock_info=os.stat('lock',dir_fd=j.dirfd,follow_symlinks=False)
        if ((path_info.st_dev,path_info.st_ino)!=self._directory_identity or
                (lock_info.st_dev,lock_info.st_ino)!=self._lock_identity):
            raise SmokeError('private smoke journal replaced')
        probe=os.open('lock',os.O_RDWR|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=j.dirfd)
        try:
            try: fcntl.flock(probe,fcntl.LOCK_EX|fcntl.LOCK_NB)
            except BlockingIOError: pass
            else:
                fcntl.flock(probe,fcntl.LOCK_UN)
                raise SmokeError('private smoke journal lock lost')
        finally: os.close(probe)
        fd=os.open('state.json',os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=j.dirfd)
        with os.fdopen(fd,'rb') as stream:
            j._private(os.fstat(stream.fileno()),0o600)
            raw=stream.read(1048577)
        expected=json.dumps(j.state,sort_keys=True,separators=(',',':')).encode()
        if len(raw)>1048576 or raw!=expected:
            raise SmokeError('private smoke journal state changed')

    def _guard(self):
        if (any(a is not b for a,b in zip((self.journal,self.fence,self.client,self.release,self.token,self.http,self.fence.ledger,self.fence.check),self._identities)) or
                type(self.fence) is not resource_fencing.Fence or type(self.fence.ledger) is not resource_fencing.KubeLedger or
                type(self.client) is not frontend_runtime_executor.KubectlTransport or self.fence.ledger.client is not self.client or
                self.client.journal is not self.journal or type(self.http) is not DevHTTP or
                type(self.journal) is not media_operation_journal.Journal or self.journal.poisoned or
                self.journal.intent != self.journal.state.get('intent') or
                self.journal.intent.get('resource_context') != self.fence.context or
                self.journal.intent.get('kube_lock') != self.lock or
                self.journal.intent.get('namespace_uid') != self.namespace_uid or
                self.journal.intent.get('core_smoke') != {'environment':'dev','source_sha':self.fence.context.get('commit')} or
                self.fence.context.get('resource') != 'DEV'):
            raise SmokeError('smoke intent or DEV authority invalid')
        self._custody()
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)
        from full_cohort_release import verify_completed
        receipt = verify_completed(self.release, self.token)
        if (not isinstance(receipt, dict) or set(receipt) != {'operation_id','source_sha','namespace_uid','package_sha256'} or
                receipt['operation_id'] != self.fence.context.get('operation_id') or
                receipt['source_sha'] != self.fence.context.get('commit') or receipt['namespace_uid'] != self.namespace_uid or
                not isinstance(receipt['package_sha256'],str) or not re.fullmatch(r'[0-9a-f]{64}',receipt['package_sha256'])):
            raise SmokeError('completed release identity unavailable')
        local=self.journal.state['steps'].get('core-smoke')
        if local is not None and local.get('release_sha256') != _digest(receipt):
            raise SmokeError('completed release changed')
        self._custody()
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)
        self._custody()
        return receipt

    def _row(self, expected):
        self.fence.authority()
        row, revision = self.fence.ledger.read()
        if (not self.fence.owned(row) or row.get('status') != 'active' or
                row.get('core_smoke') != expected or not isinstance(revision, tuple) or len(revision) != 2 or
                any(not isinstance(v,str) or not v for v in revision)):
            raise SmokeError('smoke ledger changed')
        self._custody()
        return row, revision

    def _step(self, name, private, action):
        self._guard()
        local = self.journal.state['steps']['core-smoke']
        expected = local['ledger']
        if name in local['steps'] or any(x['status'] != 'done' for x in expected['steps'].values()):
            raise SmokeError('smoke step requires reconciliation')
        nonce = secrets.token_hex(16)
        local['steps'][name] = {'status':'pending','nonce':nonce,'private':copy.deepcopy(private)}
        self.journal.save()
        row, revision = self._row(expected)
        updated = copy.deepcopy(expected)
        updated['steps'][name] = {'status':'pending','nonce':nonce}
        row['core_smoke'] = updated
        self.fence.ledger.write(revision,row)
        local['ledger'] = updated
        self.journal.save()
        self._guard(); self._row(updated)
        output = action()
        self._guard(); row, revision = self._row(updated)
        local['steps'][name].update(status='done',output=output)
        self.journal.save()
        complete = copy.deepcopy(updated)
        complete['steps'][name] = {'status':'done','nonce':nonce,'sha256':_digest(local['steps'][name])}
        row['core_smoke'] = complete
        self.fence.ledger.write(revision,row)
        local['ledger'] = complete
        self.journal.save()
        self._guard(); self._row(complete)
        return output

    def _account(self, index):
        email = 'core-smoke-' + uuid.uuid4().hex + '@voice-qa.test'
        password = secrets.token_urlsafe(32) + 'aA!9'
        register = self._step('register-'+str(index), {'email':email,'password':password}, lambda:
            self.http.request('POST','/api/v1/auth/register',{'email':email,'password':password,'guest':False,
                'device_info_json':'{"platform":"dev-auth-user-adoption"}'})['session'])
        account, profile = _uuid(register['account_id']), _uuid(register['profile_id'])
        initial = register['access_token']
        # Registration sends the initial verification message. A second send
        # can hit the per-account resend limit and must not reset shared Redis.
        self._guard()
        delivered = fixture_mail(self.client,email)
        if delivered.get('to') not in (email,[email]):
            raise SmokeError('recipient mail identity differs')
        match = re.search(r'\b(\d{6})\b',delivered.get('text',''))
        if not match:
            raise SmokeError('verification code unavailable')
        self._step('otp-verify-'+str(index), {'account_id':account}, lambda:
            self.http.request('POST','/api/v1/auth/otp/verify',{'email':email,'code':match.group(1),'otp_type':'email_verify'},initial,(200,202,204)))
        logged = self._step('login-'+str(index), {'account_id':account}, lambda:
            self.http.request('POST','/api/v1/auth/login',{'email':email,'password':password,
                'device_info_json':'{"platform":"dev-adoption-fresh-login"}'})['session'])
        if logged.get('account_type') != 'regular' or logged.get('account_id') != account or logged.get('profile_id') != profile:
            raise SmokeError('verified login identity differs')
        access = logged['access_token']
        payload = access.split('.')[1]
        claims = json.loads(base64.urlsafe_b64decode(payload+'='*(-len(payload)%4)))
        if claims.get('sub') != account or claims.get('profile_id') != profile:
            raise SmokeError('session identity differs')
        return {'account_id':account,'profile_id':profile,'access_token':access}

    def run_dev(self):
        attempted = False
        try:
            release_receipt = self._guard()
            steps = self.journal.state['steps']
            if 'core-smoke' in steps:
                raise SmokeError('existing smoke requires reconciliation')
            row,revision = self._row(None)
            state = {'status':'active','nonce':self.invocation,'operation_id':self.fence.context['operation_id'],
                     'release_sha256':_digest(release_receipt),'steps':{}}
            steps['core-smoke'] = {'status':'pending','release_sha256':_digest(release_receipt),'ledger':copy.deepcopy(state),'steps':{}}
            self.journal.save(); attempted = True
            row['core_smoke'] = copy.deepcopy(state); self.fence.ledger.write(revision,row)
            self._guard(); self._row(state)
            owner, member = self._account(0), self._account(1)
            if owner['account_id']==member['account_id'] or owner['profile_id']==member['profile_id']:
                raise SmokeError('owned identities are not distinct')
            created = self._step('create-space', {'owner':owner['profile_id']}, lambda:
                self.http.request('POST','/api/v1/spaces',{'name':'core-smoke-'+self.invocation,'description':'owned rollout smoke'},owner['access_token']))
            space = _uuid(created['space']['id'])
            if created['space'].get('owner_profile_id') != owner['profile_id']:
                raise SmokeError('owned Space owner differs')
            invite = self._step('create-invite', {'space_id':space}, lambda:
                self.http.request('POST','/api/v1/spaces/'+space+'/invites',{},owner['access_token']))['invite']
            invite_id = _uuid(invite['id']); code = invite['code']
            if invite.get('space_id') != space or invite.get('creator_profile_id') != owner['profile_id'] or not isinstance(code,str) or not re.fullmatch(r'[A-Za-z0-9_-]{1,128}',code):
                raise SmokeError('owned invite identity differs')
            joined = self._step('join-invite', {'space_id':space,'invite_id':invite_id,'profile_id':member['profile_id']}, lambda:
                self.http.request('POST','/api/v1/invites/'+code+'/join',None,member['access_token']))['space_membership']
            if joined.get('space_id') != space or joined.get('profile_id') != member['profile_id']:
                raise SmokeError('owned membership identity differs')
            def membership_readback():
                listing=self.http.request('GET','/api/v1/spaces',token=member['access_token'])
                spaces=listing['space_list']['spaces']
                if not isinstance(spaces,list) or sum(isinstance(item,dict) and item.get('id')==space for item in spaces)!=1:
                    raise SmokeError('owned membership readback unavailable')
                return {'space_id':space,'profile_id':member['profile_id'],'matches':1}
            self._step('read-membership', {'space_id':space,'profile_id':member['profile_id']}, membership_readback)
            result = {'status':'done','space_id':space,'invite_id':invite_id,'owner_account_id':owner['account_id'],
                      'owner_profile_id':owner['profile_id'],'member_account_id':member['account_id'],'member_profile_id':member['profile_id']}
            local=steps['core-smoke']; local['result']=result; local['status']='done'; self.journal.save()
            row,revision=self._row(local['ledger']); final=copy.deepcopy(local['ledger']); final['status']='done'; final['receipt_sha256']=_digest(result)
            row['core_smoke']=final; self.fence.ledger.write(revision,row); local['ledger']=final; self.journal.save()
            self._guard(); self._row(final)
            return copy.deepcopy(result)
        except BaseException:
            if attempted:
                try: self.fence.uncertain('core public smoke outcome uncertain')
                except BaseException: pass
            raise SmokeError('public smoke incomplete; retain owned evidence and reconcile') from None


def verify_completed(phase, token):
    if type(phase) is not CoreSmoke or token is not phase.token:
        raise SmokeError('smoke phase identity invalid')
    phase._guard()
    local=phase.journal.state['steps'].get('core-smoke')
    if not isinstance(local,dict) or local.get('status')!='done':
        raise SmokeError('completed smoke unavailable')
    ledger=local['ledger']; result=local['result']
    if ledger.get('status')!='done' or ledger.get('receipt_sha256')!=_digest(result):
        raise SmokeError('smoke receipt changed')
    required = {f'{step}-{index}' for step in ('register','otp-verify','login') for index in (0,1)} | {'create-space','create-invite','join-invite','read-membership'}
    if set(local['steps']) != required:
        raise SmokeError('smoke effect inventory incomplete')
    for name, record in local['steps'].items():
        if record.get('status')!='done' or ledger['steps'].get(name)!={'status':'done','nonce':record['nonce'],'sha256':_digest(record)}:
            raise SmokeError('smoke step receipt changed')
    if set(local['steps'])!=set(ledger['steps']):
        raise SmokeError('smoke effect set changed')
    records=local['steps']
    for index, prefix in ((0,'owner'),(1,'member')):
        register=records[f'register-{index}']['output']; login=records[f'login-{index}']['output']
        if (register.get('account_id') != result[prefix+'_account_id'] or register.get('profile_id') != result[prefix+'_profile_id'] or
                login.get('account_id') != result[prefix+'_account_id'] or login.get('profile_id') != result[prefix+'_profile_id'] or login.get('account_type')!='regular'):
            raise SmokeError('smoke account receipt changed')
    created=records['create-space']['output']['space']; invite=records['create-invite']['output']['invite']; joined=records['join-invite']['output']['space_membership']
    if (created.get('id')!=result['space_id'] or created.get('owner_profile_id')!=result['owner_profile_id'] or
            invite.get('id')!=result['invite_id'] or invite.get('space_id')!=result['space_id'] or invite.get('creator_profile_id')!=result['owner_profile_id'] or
            joined.get('space_id')!=result['space_id'] or joined.get('profile_id')!=result['member_profile_id']):
        raise SmokeError('smoke Space receipt changed')
    if records['read-membership']['output']!={'space_id':result['space_id'],'profile_id':result['member_profile_id'],'matches':1}:
        raise SmokeError('smoke membership readback changed')
    phase._row(ledger); phase._guard(); phase._row(ledger)
    return copy.deepcopy(result)
