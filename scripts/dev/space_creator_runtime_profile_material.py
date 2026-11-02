"""Private verification of source-selected existing Creator transport material."""
import hashlib
import json
import base64
import fcntl
import os
import re
import subprocess

import reset_credentials as credentials


SECRETS={
    'voice-media-ca':frozenset(('ca.crt','jwks-ca.crt')),
    'voice-space-media-server-tls':frozenset(('tls.crt','tls.key')),
    'voice-space-media-client-tls':frozenset(('tls.crt','tls.key')),
    'voice-gateway-media-client-tls':frozenset(('tls.crt','tls.key')),
    'voice-auth-media-server-tls':frozenset(('tls.crt','tls.key')),
    'voice-role-media-server-tls':frozenset(('tls.crt','tls.key')),
    'voice-space-principal-signing':frozenset(('current.pem','next.pem','active-kid')),
    'voice-gateway-principal-signing':frozenset(('current.pem','next.pem','active-kid')),
}


class MaterialError(RuntimeError): pass


def _pem(raw,label):
    if (not isinstance(raw,bytes) or len(raw)>65536 or not re.fullmatch(
            rb'\s*-----BEGIN '+label+rb'-----\r?\n(?:[A-Za-z0-9+/=]+\r?\n)+-----END '+label+rb'-----\s*',raw)):
        raise ValueError()
    body=b''.join(raw.strip().splitlines()[1:-1])
    der=base64.b64decode(body,validate=True)
    if len(der)<2 or der[0]!=0x30: raise ValueError()
    lead=der[1]
    if lead<128: length=lead; offset=2
    else:
        count=lead & 127
        if count==0 or count>4 or len(der)<2+count or der[2]==0: raise ValueError()
        length=int.from_bytes(der[2:2+count],'big'); offset=2+count
        if length<128: raise ValueError()
    if offset+length!=len(der): raise ValueError()
    return der


def _openssl(args, **values):
    """Use sealed anonymous FDs: private key bytes never touch argv or disk."""
    fds=[]
    try:
        paths={}
        for name,value in values.items():
            if not isinstance(value,bytes) or not value or len(value)>1048576: raise ValueError()
            fd=os.memfd_create('creator-private-material',os.MFD_CLOEXEC|os.MFD_ALLOW_SEALING); fds.append(fd)
            os.fchmod(fd,0o600); view=memoryview(value)
            while view:
                written=os.write(fd,view)
                if written<=0: raise ValueError()
                view=view[written:]
            fcntl.fcntl(fd,fcntl.F_ADD_SEALS,fcntl.F_SEAL_WRITE|fcntl.F_SEAL_GROW|fcntl.F_SEAL_SHRINK|fcntl.F_SEAL_SEAL)
            os.lseek(fd,0,os.SEEK_SET); paths[name]='/proc/self/fd/'+str(fd)
        command=['openssl']+[x.format(**paths) for x in args]
        result=subprocess.run(command,stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,
                              timeout=20,pass_fds=tuple(fds),check=True)
        if len(result.stdout)>65536: raise ValueError()
        return result.stdout
    finally:
        for fd in fds: os.close(fd)


def _cert_der(cert):
    raw=_pem(cert,b'CERTIFICATE')
    if _openssl(['x509','-in','{cert}','-outform','DER'],cert=cert)!=raw: raise ValueError()
    return raw


def _bundle(raw):
    if not isinstance(raw,bytes) or not raw or len(raw)>1048576: raise ValueError()
    pattern=re.compile(rb'-----BEGIN CERTIFICATE-----\r?\n(?:[A-Za-z0-9+/=]+\r?\n)+-----END CERTIFICATE-----')
    offset=0; result=[]
    while offset<len(raw):
        while offset<len(raw) and raw[offset] in b' \t\r\n': offset+=1
        if offset==len(raw): break
        match=pattern.match(raw,offset)
        if match is None: raise ValueError()
        result.append(_cert_der(match.group())); offset=match.end()
    if not result: raise ValueError()
    return result


def _certificate(cert,key,ca,hostname,purpose):
    der=_cert_der(cert); _pem(key,b'PRIVATE KEY')
    if purpose not in ('serverAuth','clientAuth') or not re.fullmatch(r'[A-Za-z0-9.-]+',hostname): raise ValueError()
    _openssl(['verify','-CAfile','{ca}','-purpose','sslserver' if purpose=='serverAuth' else 'sslclient',
              '-verify_hostname',hostname,'{cert}'],ca=ca,cert=cert)
    _openssl(['x509','-in','{cert}','-noout','-checkend','86400'],cert=cert)
    sans=_openssl(['x509','-in','{cert}','-noout','-ext','subjectAltName'],cert=cert).decode('ascii')
    if hostname not in re.findall(r'DNS:([A-Za-z0-9.-]+)',sans): raise ValueError()
    eku=_openssl(['x509','-in','{cert}','-noout','-ext','extendedKeyUsage'],cert=cert)
    if (b'TLS Web Server Authentication' if purpose=='serverAuth' else b'TLS Web Client Authentication') not in eku: raise ValueError()
    constraints=_openssl(['x509','-in','{cert}','-noout','-ext','basicConstraints'],cert=cert)
    if b'CA:FALSE' not in constraints or b'CA:TRUE' in constraints: raise ValueError()
    certpub=_openssl(['x509','-in','{cert}','-pubkey','-noout'],cert=cert)
    keypub=_openssl(['pkey','-in','{key}','-pubout'],key=key)
    if certpub!=keypub: raise ValueError()
    return hashlib.sha256(der).hexdigest(),keypub


def _signer(data):
    if data['active-kid'] not in (b'current',b'next'): raise ValueError()
    records=[]; public=[]
    for kid in ('current','next'):
        key=data[kid+'.pem']; _pem(key,b'PRIVATE KEY')
        _openssl(['pkey','-in','{key}','-check','-noout'],key=key)
        modulus=_openssl(['rsa','-in','{key}','-noout','-modulus'],key=key).decode('ascii').strip().removeprefix('Modulus=')
        number=int(modulus,16)
        text=_openssl(['pkey','-in','{key}','-pubout','-text_pub','-noout'],key=key).decode('ascii')
        exponent=int(re.search(r'^Exponent: ([0-9]+)',text,re.MULTILINE)[1])
        if number.bit_length()<2048: raise ValueError()
        encode=lambda value:base64.urlsafe_b64encode(value.to_bytes((value.bit_length()+7)//8,'big')).decode().rstrip('=')
        records.append({'kty':'RSA','kid':kid,'use':'sig','alg':'RS256','n':encode(number),'e':encode(exponent)})
        public.append(_openssl(['pkey','-in','{key}','-pubout'],key=key))
    return {'keys':records},public


def validate(objects,space_jwks):
    """Verify bytes; caller must authenticate objects and repeat UID/RV readback."""
    try:
        if not isinstance(objects,dict) or set(objects)!=set(SECRETS): raise ValueError()
        data={}; refs={}
        for name in sorted(SECRETS):
            obj=objects[name]; meta=obj['metadata']
            if (obj.get('apiVersion')!='v1' or obj.get('kind')!='Secret' or meta.get('name')!=name or
                    meta.get('namespace')!='voice-dev' or meta.get('deletionTimestamp') or
                    any(not isinstance(meta.get(k),str) or not meta[k] for k in ('uid','resourceVersion')) or
                    obj.get('stringData') not in (None,{}) or obj.get('binaryData') not in (None,{})):
                raise ValueError()
            values=credentials.values(obj)
            if not SECRETS[name] <= set(values) or any(not values[k] or len(values[k])>1048576 for k in SECRETS[name]): raise ValueError()
            data[name]=values; refs[name]={'uid':meta['uid'],'resourceVersion':meta['resourceVersion']}
        ca=data['voice-media-ca']['ca.crt']; bundle=data['voice-media-ca']['jwks-ca.crt']
        ca_der=_cert_der(ca); roots=_bundle(bundle)
        if ca_der not in roots: raise ValueError()
        public={}; all_keys=[]
        for name,hostname,purpose in (
            ('voice-space-media-server-tls','voice-space','serverAuth'),
            ('voice-space-media-client-tls','voice-space','clientAuth'),
            ('voice-gateway-media-client-tls','voice-gateway','clientAuth'),
            ('voice-auth-media-server-tls','voice-auth','serverAuth'),
            ('voice-role-media-server-tls','voice-role','serverAuth')):
            digest,key=_certificate(data[name]['tls.crt'],data[name]['tls.key'],ca,hostname,purpose)
            public[name]=digest; all_keys.append(key)
        space,space_keys=_signer(data['voice-space-principal-signing'])
        gateway,gateway_keys=_signer(data['voice-gateway-principal-signing'])
        all_keys+=space_keys+gateway_keys
        if len(set(all_keys))!=len(all_keys): raise ValueError()
        if not isinstance(space_jwks,bytes) or len(space_jwks)>65536 or json.loads(space_jwks)!=space: raise ValueError()
        canonical=lambda value:json.dumps(value,sort_keys=True,separators=(',',':'),allow_nan=False).encode()
        return {'objects':refs,'certificate_sha256':public,'media_ca_sha256':hashlib.sha256(ca_der).hexdigest(),
                'jwks_ca_sha256':hashlib.sha256(canonical(sorted(hashlib.sha256(root).hexdigest() for root in roots))).hexdigest(),
                'space_jwks_sha256':hashlib.sha256(canonical(space)).hexdigest(),
                'gateway_jwks_sha256':hashlib.sha256(canonical(gateway)).hexdigest(),
                'scope':'public-crypto-only-no-runtime-readiness'}
    except BaseException:
        raise MaterialError('Creator transport material unavailable') from None
