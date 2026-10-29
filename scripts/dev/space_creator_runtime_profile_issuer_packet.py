"""Closed packet validation; caller independently establishes guarded absence."""
import base64
import copy
import datetime
import hashlib
import json
import os
import re
import subprocess

FAMILIES={
    'voice-space-media-client-tls':('voice-space','clientAuth'),
    'voice-gateway-media-client-tls':('voice-gateway','clientAuth'),
    'voice-auth-media-server-tls':('voice-auth','serverAuth'),
    'voice-role-media-server-tls':('voice-role','serverAuth'),
    'voice-gateway-principal-signing':None,
}

class PacketError(RuntimeError): pass

def _openssl(args,values):
    fds=[]
    try:
        paths=[]
        for value in values:
            if not isinstance(value,bytes) or not 0<len(value)<=131072: raise ValueError()
            fd=os.memfd_create('creator-pki',os.MFD_CLOEXEC); fds.append(fd); os.fchmod(fd,0o600)
            data=memoryview(value)
            while data:
                count=os.write(fd,data)
                if count<=0: raise ValueError()
                data=data[count:]
            os.lseek(fd,0,os.SEEK_SET); paths.append('/proc/self/fd/'+str(fd))
        result=subprocess.run(['/usr/bin/openssl',*[part.format(*paths) for part in args]],pass_fds=tuple(fds),
            env={'PATH':'/usr/bin:/bin','LANG':'C','LC_ALL':'C'},stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=30,check=True)
        if len(result.stdout)>262144: raise ValueError()
        return result.stdout
    finally:
        for fd in fds: os.close(fd)

def _jwks(values):
    result=[]
    for kid in ('current','next'):
        key=values[kid+'.pem']
        if not key.startswith(b'-----BEGIN PRIVATE KEY-----\n') or key.count(b'-----BEGIN ')!=1: raise ValueError()
        _openssl(['pkey','-in','{0}','-check','-noout'],[key])
        modulus=_openssl(['rsa','-in','{0}','-noout','-modulus'],[key]).decode().strip().removeprefix('Modulus=')
        n=int(modulus,16)
        if n.bit_length()!=3072: raise ValueError()
        pub=_openssl(['pkey','-in','{0}','-pubout','-text_pub','-noout'],[key]).decode()
        match=re.search(r'^Exponent: ([0-9]+)',pub,re.MULTILINE)
        if not match: raise ValueError()
        encode=lambda x:base64.urlsafe_b64encode(x.to_bytes((x.bit_length()+7)//8,'big')).decode().rstrip('=')
        result.append({'kty':'RSA','kid':kid,'use':'sig','alg':'RS256','n':encode(n),'e':encode(int(match.group(1)))})
    if result[0]['n']==result[1]['n']: raise ValueError()
    return {'keys':result}

def _certificate(values,media,host,purpose):
    cert,key=values['tls.crt'],values['tls.key']
    if _openssl(['x509','-in','{0}','-noout','-subject','-nameopt','RFC2253'],[cert]).strip()!=('subject=CN='+host).encode(): raise ValueError()
    for hostname in (host,host+'.voice-dev.svc'):
        _openssl(['verify','-trusted','{1}','-purpose','sslserver' if purpose=='serverAuth' else 'sslclient',
                  '-verify_hostname',hostname,'{0}'],[cert,media])
    _openssl(['x509','-in','{0}','-noout','-checkend','86400'],[cert])
    def expiry(value):
        text=_openssl(['x509','-in','{0}','-noout','-enddate'],[value]).decode('ascii').strip()
        if not text.startswith('notAfter='): raise ValueError()
        return datetime.datetime.strptime(text.removeprefix('notAfter='),'%b %d %H:%M:%S %Y GMT')
    if expiry(cert)>expiry(media): raise ValueError()
    sans=_openssl(['x509','-in','{0}','-noout','-ext','subjectAltName'],[cert]).decode('ascii')
    lines=sans.splitlines()
    if len(lines)!=2 or set(item.strip() for item in lines[1].split(','))!={'DNS:'+host,'DNS:'+host+'.voice-dev.svc'}: raise ValueError()
    eku=_openssl(['x509','-in','{0}','-noout','-ext','extendedKeyUsage'],[cert])
    expected=b'TLS Web Server Authentication' if purpose=='serverAuth' else b'TLS Web Client Authentication'
    opposite=b'TLS Web Client Authentication' if purpose=='serverAuth' else b'TLS Web Server Authentication'
    if expected not in eku or opposite in eku: raise ValueError()
    constraints=_openssl(['x509','-in','{0}','-noout','-ext','basicConstraints'],[cert])
    if b'CA:FALSE' not in constraints or b'CA:TRUE' in constraints: raise ValueError()
    if _openssl(['x509','-in','{0}','-pubkey','-noout'],[cert])!=_openssl(['pkey','-in','{0}','-pubout'],[key]): raise ValueError()

def validate(envelope,operation_id,source_sha,namespace_uid,preflight_sha256,missing,media):
    """Return detached objects after cryptographic checks, not root admission."""
    try:
        raw=json.dumps(envelope,sort_keys=True,separators=(',',':'),allow_nan=False).encode()
        if len(raw)>524288: raise ValueError()
        value=json.loads(raw)
        if (set(value)!={'schema','operation_id','source_sha','namespace_uid','preflight_sha256','issuer_receipt','objects'} or
            value['schema']!='voice-space-creator-profile-packet-v1' or value['operation_id']!=operation_id or
            not re.fullmatch(r'media-c1-[a-f0-9]{32}',operation_id) or value['source_sha']!=source_sha or
            not re.fullmatch(r'[a-f0-9]{40}',source_sha) or value['namespace_uid']!=namespace_uid or
            not isinstance(namespace_uid,str) or not namespace_uid or value['preflight_sha256']!=preflight_sha256 or
            not re.fullmatch(r'[a-f0-9]{64}',preflight_sha256) or not isinstance(missing,list) or
            not missing or missing!=sorted(set(missing)) or set(missing)-set(FAMILIES)): raise ValueError()
        receipt=value['issuer_receipt']; bundle=value['objects']; ca_der=_openssl(['x509','-in','{0}','-outform','DER'],[media])
        if (set(receipt)!={'schema','status','namespace','object_names','media_ca_der_sha256','public_jwks','runtime_qualified','cluster_applied'} or
            receipt['schema']!='voice-space-creator-profile-issuer-v1' or receipt['status']!='staged' or receipt['namespace']!='voice-dev' or
            receipt['object_names']!=missing or receipt['media_ca_der_sha256']!=hashlib.sha256(ca_der).hexdigest() or
            receipt['runtime_qualified'] is not False or receipt['cluster_applied'] is not False or
            set(bundle)!={'apiVersion','kind','items'} or (bundle['apiVersion'],bundle['kind'])!=('v1','List') or
            not isinstance(bundle['items'],list) or len(bundle['items'])!=len(missing)): raise ValueError()
        found={}; jwks=None
        for obj in bundle['items']:
            if (set(obj)!={'apiVersion','kind','metadata','type','data'} or (obj['apiVersion'],obj['kind'],obj['type'])!=('v1','Secret','Opaque') or
                set(obj['metadata'])!={'name','namespace'} or obj['metadata']['namespace']!='voice-dev'): raise ValueError()
            name=obj['metadata']['name']; spec=FAMILIES.get(name)
            if name not in missing or name in found or not isinstance(obj['data'],dict): raise ValueError()
            expected={'current.pem','next.pem','active-kid'} if spec is None else {'tls.crt','tls.key'}
            if set(obj['data'])!=expected: raise ValueError()
            values={key:base64.b64decode(item,validate=True) for key,item in obj['data'].items()}
            if any(not item or len(item)>131072 for item in values.values()): raise ValueError()
            if spec is None:
                if values['active-kid']!=b'current': raise ValueError()
                jwks=_jwks(values)
            else: _certificate(values,media,*spec)
            found[name]=obj
        if set(found)!=set(missing) or jwks!=receipt['public_jwks']: raise ValueError()
        return copy.deepcopy(found)
    except Exception: raise PacketError('Creator profile packet qualification failed') from None
