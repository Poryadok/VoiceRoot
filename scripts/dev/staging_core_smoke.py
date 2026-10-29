"""One-shot STAGING post-release smoke under the existing root operation lock.

The stage owner invokes this inside root_scope after actual release health and
before Session.complete. The protected source-owned release verifier is required.
"""
import copy
import os
import re
import secrets
import stat

import guard
import media_operation_journal
import resource_claim
import staging_core_credentials
import staging_core_smoke_http as http


class SmokeError(RuntimeError):
    pass


class StagingCoreSmoke:
    def __init__(self, session, release):
        if type(session) is not resource_claim.Session or not re.fullmatch('[a-f0-9]{12}', session.operation):
            raise SmokeError('STAGING operation identity invalid')
        self.session, self.release = session, release
        self._refs = (session, release)
        self.journal = None
        self.credentials = None
        self._accounts = None
        self.receipt = None
        self.http = http.StagingHTTP()
        self._http = self.http
        self._journal = None
        self._credentials = None

    def _guard(self):
        if (self.session is not self._refs[0] or self.release is not self._refs[1] or
                type(self.session) is not resource_claim.Session or self.http is not self._http or
                type(self.http) is not http.StagingHTTP or self.journal is not self._journal or
                self.credentials is not self._credentials or
                type(self.credentials) is not staging_core_credentials.CredentialInput or
                self.credentials.journal is not self.journal or self.credentials.session is not self.session or
                self.credentials.release is not self.release):
            raise SmokeError('STAGING smoke authority changed')
        receipt = self.credentials._guard()
        if receipt != self.receipt or self.credentials.verify() != self._accounts:
            raise SmokeError('STAGING release changed')

    def _step(self, name, action):
        self._guard()
        state = self.journal.state['steps']['staging-core-smoke']
        if name in state['steps'] or any(step['status'] != 'done' for step in state['steps'].values()):
            raise SmokeError('STAGING smoke requires reconciliation')
        state['steps'][name] = {'status': 'pending', 'nonce': secrets.token_hex(16)}
        self.journal.save()
        self._guard()
        result = action()
        self._guard()
        state['steps'][name].update(status='done', output=copy.deepcopy(result))
        self.journal.save()
        self._guard()
        return result

    def run(self):
        attempted = False
        try:
            import staging_core_release
            self.session.check()
            self.receipt = staging_core_release.verify_completed(self.release, self.session)
            self.session.check()
            base = guard.ROOT / ('rollout-' + self.session.operation)
            info = os.stat(base, follow_symlinks=False)
            if (not stat.S_ISDIR(info.st_mode) or info.st_uid != os.geteuid() or info.st_mode & 0o022):
                raise SmokeError('STAGING operation directory unavailable')
            path = base / 'core-smoke'
            if os.path.lexists(path):
                raise SmokeError('STAGING smoke already attempted')
            intent = {'resource_context': copy.deepcopy(self.session.context),
                      'operation': self.session.operation, 'release': copy.deepcopy(self.receipt)}
            self.journal = media_operation_journal.Journal(path, intent)
            self._journal = self.journal
            attempted = True
            self.credentials = staging_core_credentials.CredentialInput(self.session, self.release, self.journal)
            self._credentials = self.credentials
            accounts = self.credentials.capture()
            self._accounts = copy.deepcopy(accounts)
            self._guard()
            state = {'status': 'pending', 'nonce': secrets.token_hex(16), 'steps': {}}
            self.journal.state['steps']['staging-core-smoke'] = state
            self.journal.save()
            owner = self._step('login-0', lambda: http.login(self.http, **accounts[0]))
            member = self._step('login-1', lambda: http.login(self.http, **accounts[1]))
            if owner['account_id'] == member['account_id'] or owner['profile_id'] == member['profile_id']:
                raise SmokeError('STAGING test identities are not distinct')
            space = self._step('create-space', lambda: http.create_space(self.http, state['nonce'], owner))
            invite = self._step('create-invite', lambda: http.create_invite(self.http, space['space_id'], owner))
            joined = self._step('join-invite', lambda: http.join(self.http, invite, member))
            readback = self._step('read-membership', lambda: http.read_membership(self.http, space['space_id'], member))
            result = {'status': 'done', 'space_id': space['space_id'], 'invite_id': invite['invite_id'],
                      'owner_account_id': owner['account_id'], 'owner_profile_id': owner['profile_id'],
                      'member_account_id': member['account_id'], 'member_profile_id': member['profile_id']}
            if joined != {'space_id': result['space_id'], 'profile_id': result['member_profile_id']} or \
                    readback != {'space_id': result['space_id'], 'profile_id': result['member_profile_id'], 'matches': 1}:
                raise SmokeError('STAGING membership proof changed')
            state.update(status='done', result=result)
            self.journal.save()
            return verify_completed(self)
        except BaseException:
            if attempted:
                try: self.session.uncertain('STAGING core smoke outcome uncertain')
                except BaseException: pass
            raise SmokeError('STAGING smoke incomplete; retain evidence and reconcile') from None


def verify_completed(phase):
    if type(phase) is not StagingCoreSmoke or phase.journal is None:
        raise SmokeError('STAGING smoke identity invalid')
    phase._guard()
    state = phase.journal.state['steps'].get('staging-core-smoke')
    required = {'login-0', 'login-1', 'create-space', 'create-invite', 'join-invite', 'read-membership'}
    if (not isinstance(state, dict) or state.get('status') != 'done' or set(state.get('steps', {})) != required or
            any(step.get('status') != 'done' for step in state['steps'].values())):
        raise SmokeError('STAGING smoke proof unavailable')
    steps, result = state['steps'], state['result']
    owner, member = steps['login-0']['output'], steps['login-1']['output']
    space, invite = steps['create-space']['output'], steps['create-invite']['output']
    if (result != {'status': 'done', 'space_id': space['space_id'], 'invite_id': invite['invite_id'],
                   'owner_account_id': owner['account_id'], 'owner_profile_id': owner['profile_id'],
                   'member_account_id': member['account_id'], 'member_profile_id': member['profile_id']} or
            space['owner_profile_id'] != owner['profile_id'] or invite['space_id'] != space['space_id'] or
            steps['join-invite']['output'] != {'space_id': space['space_id'], 'profile_id': member['profile_id']} or
            steps['read-membership']['output'] != {'space_id': space['space_id'], 'profile_id': member['profile_id'], 'matches': 1}):
        raise SmokeError('STAGING smoke proof changed')
    phase._guard()
    return copy.deepcopy(result)
