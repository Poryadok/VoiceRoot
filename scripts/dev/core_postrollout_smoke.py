"""One fenced DEV public-API smoke. Owned accounts and Space are retained.

The caller owns the current claim, rollout and private journal. The source-owned
full_cohort_release verifier must be present and complete before any effect.
No STAGING credential or mailbox adapter is implied by this DEV-only route.
"""
import base64
import copy
import hashlib
import json
import re
import secrets
import urllib.error
import urllib.request
import uuid

from auth_user_adoption_smoke import fixture_mail
import media_operation_guard
import media_operation_journal

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
            with urllib.request.build_opener(_NoRedirect()).open(request, timeout=25) as response:
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
        self._identities = (journal, fence, client, release, token, self.http)
        self.invocation = secrets.token_hex(16)

    def _guard(self):
        from full_cohort_release import verify_completed
        if (any(a is not b for a,b in zip((self.journal,self.fence,self.client,self.release,self.token,self.http),self._identities)) or
                type(self.http) is not DevHTTP or type(self.journal) is not media_operation_journal.Journal or self.journal.poisoned or
                self.journal.intent != self.journal.state.get('intent') or
                self.journal.intent.get('resource_context') != self.fence.context or
                self.journal.intent.get('kube_lock') != self.lock or
                self.journal.intent.get('namespace_uid') != self.namespace_uid or
                self.journal.intent.get('core_smoke') != {'environment':'dev','source_sha':self.fence.context.get('commit')} or
                self.fence.context.get('resource') != 'DEV'):
            raise SmokeError('smoke intent or DEV authority invalid')
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)
        receipt = verify_completed(self.release, self.token)
        if (not isinstance(receipt, dict) or set(receipt) != {'operation_id','source_sha','namespace_uid','package_sha256'} or
                receipt['operation_id'] != self.fence.context.get('operation_id') or
                receipt['source_sha'] != self.fence.context.get('commit') or receipt['namespace_uid'] != self.namespace_uid or
                not isinstance(receipt['package_sha256'],str) or not re.fullmatch(r'[0-9a-f]{64}',receipt['package_sha256'])):
            raise SmokeError('completed release identity unavailable')
        local=self.journal.state['steps'].get('core-smoke')
        if local is not None and local.get('release_sha256') != _digest(receipt):
            raise SmokeError('completed release changed')
        return receipt

    def _row(self, expected):
        self.fence.authority()
        row, revision = self.fence.ledger.read()
        if (not self.fence.owned(row) or row.get('status') != 'active' or
                row.get('core_smoke') != expected or not isinstance(revision, tuple) or len(revision) != 2 or
                any(not isinstance(v,str) or not v for v in revision)):
            raise SmokeError('smoke ledger changed')
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
        self._step('otp-send-'+str(index), {'account_id':account}, lambda:
            self.http.request('POST','/api/v1/auth/otp/send',{'email':email,'otp_type':'email_verify'},initial,(204,)))
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
    required = {f'{step}-{index}' for step in ('register','otp-send','otp-verify','login') for index in (0,1)} | {'create-space','create-invite','join-invite'}
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
    phase._row(ledger); phase._guard(); phase._row(ledger)
    return copy.deepcopy(result)
