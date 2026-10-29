"""Fixed public HTTP operations for the STAGING core smoke.

The caller must hold the source-owned STAGING release and operation authority.
This transport does not itself authorize an operation or credential source.
"""
import base64
import json
import re
import urllib.request
import uuid

ORIGIN = 'https://voice.comrade.click'


class SmokeHTTPError(RuntimeError):
    pass


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        raise SmokeHTTPError('redirect refused')


class StagingHTTP:
    def request(self, method, path, body=None, token=None):
        if method not in ('GET', 'POST') or not path.startswith('/api/v1/') or '?' in path or '#' in path:
            raise SmokeHTTPError('route refused')
        headers = {'Content-Type': 'application/json'}
        if token:
            headers['Authorization'] = 'Bearer ' + token
        request = urllib.request.Request(ORIGIN + path, data=json.dumps(body).encode() if body is not None else None,
                                         headers=headers, method=method)
        try:
            with urllib.request.build_opener(urllib.request.ProxyHandler({}), _NoRedirect()).open(request, timeout=25) as response:
                if response.status != 200:
                    raise SmokeHTTPError('status refused')
                raw = response.read(1048577)
                if len(raw) > 1048576:
                    raise SmokeHTTPError('response exceeds bound')
                value = json.loads(raw) if raw else {}
                if not isinstance(value, dict):
                    raise SmokeHTTPError('response invalid')
                return value
        except BaseException:
            raise SmokeHTTPError('public API outcome uncertain') from None


def login(http, email, password):
    """Perform one ordinary existing-account login and validate its identities."""
    try:
        session = http.request('POST', '/api/v1/auth/login', {
            'email': email, 'password': password,
            'device_info_json': '{"platform":"staging-core-smoke"}',
        })['session']
        account, profile, access = session['account_id'], session['profile_id'], session['access_token']
        for value in (account, profile):
            parsed = uuid.UUID(value)
            if not parsed.int or str(parsed) != value:
                raise ValueError()
        if session['account_type'] != 'regular':
            raise ValueError()
        payload = access.split('.')[1]
        claims = json.loads(base64.urlsafe_b64decode(payload + '=' * (-len(payload) % 4)))
        if claims.get('sub') != account or claims.get('profile_id') != profile:
            raise ValueError()
        return {'account_id': account, 'profile_id': profile, 'access_token': access}
    except BaseException:
        raise SmokeHTTPError('existing-account login unavailable or identity differs') from None


def create_space(http, invocation, owner):
    try:
        space = http.request('POST', '/api/v1/spaces', {
            'name': 'core-smoke-' + invocation, 'description': 'owned rollout smoke',
        }, owner['access_token'])['space']
        parsed = uuid.UUID(space['id'])
        if not parsed.int or str(parsed) != space['id'] or space['owner_profile_id'] != owner['profile_id']:
            raise ValueError()
        return {'space_id': space['id'], 'owner_profile_id': owner['profile_id']}
    except BaseException:
        raise SmokeHTTPError('Space creation outcome uncertain') from None


def create_invite(http, space_id, owner):
    try:
        invite = http.request('POST', '/api/v1/spaces/' + space_id + '/invites', {}, owner['access_token'])['invite']
        parsed = uuid.UUID(invite['id'])
        if (not parsed.int or str(parsed) != invite['id'] or invite['space_id'] != space_id or
                invite['creator_profile_id'] != owner['profile_id'] or not isinstance(invite['code'], str) or
                not re.fullmatch(r'[A-Za-z0-9_-]{1,128}', invite['code'])):
            raise ValueError()
        return {'invite_id': invite['id'], 'space_id': space_id, 'code': invite['code']}
    except BaseException:
        raise SmokeHTTPError('invite creation outcome uncertain') from None


def join(http, invite, member):
    try:
        joined = http.request('POST', '/api/v1/invites/' + invite['code'] + '/join',
                              token=member['access_token'])['space_membership']
        if joined['space_id'] != invite['space_id'] or joined['profile_id'] != member['profile_id']:
            raise ValueError()
        return {'space_id': invite['space_id'], 'profile_id': member['profile_id']}
    except BaseException:
        raise SmokeHTTPError('membership outcome uncertain') from None


def read_membership(http, space_id, member):
    try:
        spaces = http.request('GET', '/api/v1/spaces', token=member['access_token'])['space_list']['spaces']
        if not isinstance(spaces, list) or sum(isinstance(s, dict) and s.get('id') == space_id for s in spaces) != 1:
            raise ValueError()
        return {'space_id': space_id, 'profile_id': member['profile_id'], 'matches': 1}
    except BaseException:
        raise SmokeHTTPError('membership readback unavailable') from None
