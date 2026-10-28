"""One owned DEV account, real public Gateway and recipient-scoped DEV mail."""
import base64
import ipaddress
import json
import re
import secrets
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from auth_user_adoption import RecoveryRequired


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self,*args,**kwargs):
        raise RecoveryRequired('public API redirect refused')


def api(method,path,body=None,token=None,expected=(200,)):
    headers={'Content-Type':'application/json'}
    if token:
        headers['Authorization']='Bearer '+token
    request=urllib.request.Request('https://api-dev.comrade.click'+path,
        data=json.dumps(body).encode() if body is not None else None,headers=headers,method=method)
    try:
        with urllib.request.build_opener(NoRedirect()).open(request,timeout=25) as response:
            if response.status not in expected:
                raise RecoveryRequired('public API status unexpected')
            raw=response.read(1024*1024)
            return json.loads(raw) if raw else {}
    except BaseException:
        raise RecoveryRequired('public API request failed; owned account intent retained') from None


def fixture_mail(kube,email):
    service=kube.call('get','service','voice-dev-mail','-o','json')
    spec=service['spec']; address=ipaddress.ip_address(spec['clusterIP'])
    if (address not in ipaddress.ip_network('10.43.0.0/16')
            or not any(p.get('port')==4180 for p in spec['ports'])):
        raise RecoveryRequired('DEV mail fixture identity invalid')
    url='http://'+str(address)+':4180/emails/latest?to='+urllib.parse.quote(email)
    for _ in range(25):
        try:
            with urllib.request.build_opener(NoRedirect()).open(url,timeout=5) as response:
                return json.load(response)
        except urllib.error.HTTPError as error:
            if error.code!=404:
                raise RecoveryRequired('recipient fixture mail unavailable') from None
            time.sleep(.2)
    raise RecoveryRequired('recipient fixture mail not delivered')


def public_smoke(journal,kube,http=None,mail=None):
    if 'smoke_account' in journal.state:
        raise RecoveryRequired('owned smoke intent exists; reconcile account rather than register again')
    http=http or api
    mail=mail or (lambda email:fixture_mail(kube,email))
    email='dimas16-adoption-'+uuid.uuid4().hex+'@voice-qa.test'
    password=secrets.token_urlsafe(32)+'aA!9'
    journal.state['smoke_account']={'email':email,'password':password,'status':'register_pending'}
    journal.save()
    try:
        session=http('POST','/api/v1/auth/register',{'email':email,'password':password,'guest':False,
                    'device_info_json':'{"platform":"dev-auth-user-adoption"}'})['session']
        account=str(uuid.UUID(session['account_id'])); profile=str(uuid.UUID(session['profile_id']))
        journal.state['smoke_account'].update(account_id=account,profile_id=profile,status='registered')
        journal.save()
        http('POST','/api/v1/auth/otp/send',{'email':email,'otp_type':'email_verify'},session['access_token'],(204,))
        delivered=mail(email)
        recipients=delivered.get('to')
        if recipients!=email and recipients!=[email]:
            raise RecoveryRequired('foreign recipient fixture mail refused')
        match=re.search(r'\b(\d{6})\b',delivered['text'])
        if not match:
            raise RecoveryRequired('fixture verification code absent')
        http('POST','/api/v1/auth/otp/verify',{'email':email,'code':match.group(1),'otp_type':'email_verify'},
             session['access_token'],(200,202,204))
        logged=None
        for _ in range(12):
            logged=http('POST','/api/v1/auth/login',{'email':email,'password':password,
                         'device_info_json':'{"platform":"dev-adoption-fresh-login"}'})['session']
            if logged.get('account_type')=='regular':
                break
            time.sleep(.25)
        if (logged.get('account_type')!='regular' or logged['account_id']!=account or logged['profile_id']!=profile):
            raise RecoveryRequired('fresh login identity not stable and verified')
        payload=logged['access_token'].split('.')[1]
        claims=json.loads(base64.urlsafe_b64decode(payload+'='*(-len(payload)%4)))
        if claims.get('sub')!=account or claims.get('profile_id')!=profile:
            raise RecoveryRequired('JWT account/profile binding changed')
        if not http('GET','/api/v1/users/me/privacy',token=logged['access_token']):
            raise RecoveryRequired('real User privacy response absent')
        http('POST','/api/v1/auth/logout',{'refresh_token':logged['refresh_token']},logged['access_token'],(204,))
        journal.state['smoke_account']['status']='passed_logout_complete_fixture_retained'
        journal.save()
        return {'result':'PASS','account_id':account,'profile_id':profile,
                'regular_fresh_login':True,'jwt_profile_binding':True,'privacy_read':True,'logout':True,
                'cleanup':'Owned account retained for documented deletion; no raw DB delete or shared throttle reset'}
    except BaseException:
        raise RecoveryRequired('owned public smoke incomplete; reconcile retained account intent') from None
