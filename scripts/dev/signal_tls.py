#!/usr/bin/env python3
"""Owned dev-only public edge TLS via Certbot manual HTTP01 hooks.

Root Linux operator, after DNS and public HTTP reachability are configured:
  python3 scripts/dev/signal_tls.py issue --agree-tos --email operator@example.org
  python3 scripts/dev/signal_tls.py expand
  python3 scripts/dev/signal_tls.py renew

Only the five fixed dev hosts listed in ROUTES in voice-dev are admitted.
Issue requests all five; expand extends the existing owned LiveKit certificate
lineage to all five. Renew preserves that lineage. The first command explicitly
accepts the ACME subscriber terms. Omit --email only to register
without email. State and all logs stay under /home/voice-dev/tls (root0700).
No timers are installed: run renew regularly before expiration and verify the
public HTTPS endpoint after publication. Keep the challenge Ingress for renewal.
Manual hooks follow https://eff-certbot.readthedocs.io/en/stable/using.html#manual
"""
import argparse
import base64
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys

NS='voice-dev'
HOST='livekit-dev.comrade.click'
ROUTES={HOST:('voice-livekit',7880,'livekit'),
        'dev.comrade.click':('voice-web',80,'web'),
        'api-dev.comrade.click':('voice-gateway',8080,'api'),
        'admin-dev.comrade.click':('voice-admin',80,'admin'),
        'developers-dev.comrade.click':('voice-developer-portal',80,'developers')}
SECRET='voice-dev-signaling-tls'
LABELS={'voice.environment':'dev','app.kubernetes.io/managed-by':'voice-dev'}
STATE=Path('/home/voice-dev/tls')
WEBROOT='/usr/share/nginx/html/.well-known/acme-challenge'
IMAGE='certbot/certbot@sha256:f70ad0adbb7e117f0fe42a63c553f28ea451edabc0148757b6efcd9735acaa20'
LOG=None


def run(command,body=None):
    try:
        result=subprocess.run([str(x) for x in command],input=body,capture_output=True,timeout=600)
    except (OSError,subprocess.TimeoutExpired) as exc:
        raise RuntimeError('TLS operation transport failed') from exc
    if LOG is not None:
        fd=os.open(LOG,os.O_WRONLY|os.O_APPEND|os.O_CREAT|os.O_NOFOLLOW,0o600)
        with os.fdopen(fd,'ab') as output:
            output.write(result.stdout+result.stderr)
    if result.returncode:
        raise RuntimeError('TLS subprocess failed; inspect private logs')
    return result.stdout


class Client:
    def __init__(self,kubeconfig=None):
        self.kubeconfig=kubeconfig

    def call(self,*args,body=None):
        config=['--kubeconfig',str(self.kubeconfig)] if self.kubeconfig else []
        return run(['kubectl',*config,'--namespace',NS,*args],None if body is None else json.dumps(body).encode())

    def get(self,kind,name):
        value=self.call('get',kind,name,'--ignore-not-found','-o','json')
        return json.loads(value) if value.strip() else None


def owned(obj,namespace=False):
    meta=(obj or {}).get('metadata',{})
    if (any(meta.get('labels',{}).get(k)!=v for k,v in LABELS.items()) or
            (meta.get('name')!=NS if namespace else meta.get('namespace')!=NS)):
        raise ValueError('dev resource ownership required')


def guard(client):
    owned(client.get('namespace',NS),namespace=True)
    web=client.get('deployment','voice-web')
    owned(web)
    if web.get('metadata',{}).get('name')!='voice-web':
        raise ValueError('owned web deployment required')


def challenge(env):
    token=env.get('CERTBOT_TOKEN','')
    if env.get('CERTBOT_DOMAIN') not in ROUTES or not re.fullmatch(r'[A-Za-z0-9_-]{1,128}',token):
        raise ValueError('fixed dev domain and safe challenge filename required')
    return token


def hook(client,cleanup=False):
    token=challenge(os.environ)
    guard(client)
    path=WEBROOT+'/'+token
    prefix=['exec','deployment/voice-web','-c','web']
    if cleanup:
        client.call(*prefix,'--','rm','-f','--',path)
    else:
        validation=os.environ.get('CERTBOT_VALIDATION','')
        if not re.fullmatch(re.escape(token)+r'\.[A-Za-z0-9_-]{1,128}',validation):
            raise ValueError('challenge authorization must bind exact token')
        client.call(*prefix,'--','mkdir','-p',WEBROOT)
        # Only public HTTP challenge data goes to stdin; no shell interpolation.
        run(['kubectl','--namespace',NS,*prefix,'-i','--','tee',path],validation.encode())


def ingress(secure,host=HOST):
    if host not in ROUTES:
        raise ValueError('fixed dev host required')
    service,port,suffix=ROUTES[host]
    name=('voice-dev-signaling' if host==HOST else 'voice-dev-edge-'+suffix)+('-https' if secure else '-http01')
    if not secure:
        service,port='voice-web',80
    obj={'apiVersion':'networking.k8s.io/v1','kind':'Ingress',
         'metadata':{'name':name,'namespace':NS,'labels':dict(LABELS),
                     'annotations':{'traefik.ingress.kubernetes.io/router.entrypoints':'websecure' if secure else 'web'}},
         'spec':{'ingressClassName':'traefik','rules':[{'host':host,'http':{'paths':[{
             'path':'/' if secure else '/.well-known/acme-challenge/','pathType':'Prefix',
             'backend':{'service':{'name':service,'port':{'number':port}}}}]}}]}}
    if secure:
        obj['spec']['tls']=[{'hosts':[host],'secretName':SECRET}]
        if host=='api-dev.comrade.click':
            # Keep the signed bucket/object path intact; Kubernetes Prefix
            # routing selects this specific path over the Gateway fallback.
            obj['spec']['rules'][0]['http']['paths'].append({
                'path':'/voice-dev-files','pathType':'Prefix',
                'backend':{'service':{'name':'voice-minio','port':{'number':9000}}}})
    return obj


def secret(cert,key):
    return {'apiVersion':'v1','kind':'Secret','metadata':{'name':SECRET,'namespace':NS,'labels':dict(LABELS)},
            'type':'kubernetes.io/tls','data':{'tls.crt':base64.b64encode(cert).decode(),'tls.key':base64.b64encode(key).decode()}}


def publish(client,obj):
    if obj['metadata'].get('namespace')!=NS:
        raise ValueError('foreign target forbidden')
    current=client.get(obj['kind'].lower(),obj['metadata']['name'])
    if current is not None:
        owned(current)
        if obj['kind']=='Secret' and current.get('data')==obj.get('data') and current.get('type')==obj.get('type'):
            return
        if (obj['kind']=='Ingress' and current.get('spec')==obj.get('spec') and
                all(current['metadata'].get('annotations',{}).get(k)==v for k,v in obj['metadata'].get('annotations',{}).items())):
            return
        obj['metadata'].update({k:current['metadata'][k] for k in ('uid','resourceVersion')})
    # Existing objects use optimistic concurrency; fresh objects use atomic create.
    client.call('replace' if current else 'create','-f','-',body=obj)


def private_state():
    if sys.platform!='linux' or os.geteuid()!=0:
        raise ValueError('root Linux operator required')
    if STATE.resolve()!=STATE:
        raise ValueError('canonical private TLS state required')
    STATE.mkdir(mode=0o700,parents=True,exist_ok=True)
    info=STATE.stat()
    if info.st_uid!=0 or stat.S_IMODE(info.st_mode)!=0o700:
        raise ValueError('TLS state must be root-owned0700')


def validate_action(action,existing,agree_tos):
    if action not in ('issue','expand','renew'):
        raise ValueError('unknown certificate operation')
    if existing is not None:
        owned(existing)
    if action=='issue' and (existing is not None or not agree_tos):
        raise ValueError('issue requires fresh TLS Secret and explicit --agree-tos')
    if action in ('expand','renew') and existing is None:
        raise ValueError('expand and renew require an existing owned TLS Secret')


def certbot_arguments(action,email=None,agree_tos=False):
    if action not in ('issue','expand','renew'):
        raise ValueError('unknown certificate operation')
    common=['--non-interactive','--cert-name',HOST,'--config-dir','/etc/letsencrypt',
            '--work-dir','/etc/letsencrypt/work','--logs-dir','/etc/letsencrypt/logs',
            '--manual-auth-hook','python3 /hook/signal_tls.py auth',
            '--manual-cleanup-hook','python3 /hook/signal_tls.py cleanup']
    if action=='renew':
        return ['renew',*common]
    result=['certonly','--manual','--preferred-challenges','http']
    if action=='expand':
        result+=['--expand']
    if agree_tos:
        result+=['--agree-tos']
    for host in ROUTES:
        result+=['-d',host]
    if email:
        result+=['--email',email]
    elif action=='issue':
        result+=['--register-unsafely-without-email']
    return result+common


def validate_certificate(cert,key):
    for host in ROUTES:
        run(['openssl','x509','-in',cert,'-noout','-checkhost',host,'-checkend','86400'])
    certpub=run(['openssl','x509','-in',cert,'-noout','-pubkey'])
    keypub=run(['openssl','pkey','-in',key,'-pubout'])
    if certpub!=keypub:
        raise ValueError('certificate private key mismatch')
    run(['openssl','verify','-CApath','/etc/ssl/certs','-untrusted',cert,cert])


def operator(action,kubeconfig,email=None,agree_tos=False):
    global LOG
    private_state()
    LOG=STATE/'operator.log'
    import fcntl
    fd=os.open(STATE/'operator.lock',os.O_CREAT|os.O_RDWR|os.O_NOFOLLOW,0o600)
    with os.fdopen(fd,'w') as lock:
        fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
        kubeconfig=Path(kubeconfig).resolve(strict=True)
        client=Client(kubeconfig)
        guard(client)
        existing=client.get('secret',SECRET)
        validate_action(action,existing,agree_tos)
        for host in ROUTES:
            for secure in (False,True):
                current=client.get('ingress',ingress(secure,host)['metadata']['name'])
                if current is not None:
                    owned(current)
        for host in ROUTES:
            publish(client,ingress(False,host))
        cmd=['docker','run','--rm','--network','host',
             '--mount','type=bind,src='+str(STATE)+',dst=/etc/letsencrypt',
             '--mount','type=bind,src=/usr/local/bin/k3s,dst=/usr/local/bin/kubectl,readonly',
             '--mount','type=bind,src='+str(kubeconfig)+',dst=/kubeconfig,readonly',
             '--mount','type=bind,src='+str(Path(__file__).resolve())+',dst=/hook/signal_tls.py,readonly',
             '-e','KUBECONFIG=/kubeconfig',IMAGE]
        run(cmd+certbot_arguments(action,email,agree_tos))
        lineage=STATE/'live'/HOST
        cert,key=lineage/'fullchain.pem',lineage/'privkey.pem'
        for path in (cert,key):
            if not path.resolve(strict=True).is_relative_to(STATE):
                raise ValueError('certificate lineage escaped private state')
        validate_certificate(cert,key)
        guard(client)
        publish(client,secret(cert.read_bytes(),key.read_bytes()))
        for host in ROUTES:
            publish(client,ingress(True,host))
        expiry=run(['openssl','x509','-in',cert,'-noout','-enddate']).decode().strip()
        print('DEV_SIGNAL_TLS_PUBLISHED hosts='+','.join(ROUTES)+' '+expiry)


def main():
    global LOG
    parser=argparse.ArgumentParser(description=__doc__,formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('action',choices=('issue','expand','renew','auth','cleanup'))
    parser.add_argument('--kubeconfig',default='/home/pmd/.kube/config')
    parser.add_argument('--email')
    parser.add_argument('--agree-tos',action='store_true')
    args=parser.parse_args()
    os.umask(0o077)
    try:
        if args.action in ('auth','cleanup'):
            LOG=Path('/etc/letsencrypt/hook.log')
            hook(Client(),cleanup=args.action=='cleanup')
        else:
            operator(args.action,args.kubeconfig,args.email,args.agree_tos)
    except Exception:
        print('Dev signaling TLS failed; inspect private TLS logs',file=sys.stderr)
        return 1
    return 0


if __name__=='__main__': sys.exit(main())
