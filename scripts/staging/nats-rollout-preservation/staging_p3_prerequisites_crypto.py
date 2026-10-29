"""Offline fixed STAGING P3 issuance and validation; no authority or cluster claim.

Caller must independently authenticate the staging CA, peer certificates and
replay configuration under the root operation. Private bytes stay in memory
and anonymous inherited descriptors; this module does not publish or log them.
"""
import base64
import hashlib
import json
import os
import re
import secrets
import ssl
import stat
import subprocess
import sys

import staging_p3_prerequisites as contract


class CryptoError(RuntimeError): pass


TRUST_KEYS=frozenset(('chat-server-ca.crt','space-client-ca.crt','space-jwks-ca.crt',
                      'realtime-client-ca.crt','realtime-jwks-ca.crt'))
LEAVES={
    'voice-realtime-p3-tls':('voice-realtime.voice-staging.svc',('serverAuth','clientAuth')),
    'voice-chat-realtime-p3-tls':('voice-chat.voice-staging.svc',('serverAuth',)),
    'voice-p3-principal-jwks-tls':('voice-p3-principal-jwks.voice-staging.svc',('serverAuth',)),
}


def _run(args, values=()):
    fds=[]
    try:
        paths=[]
        for value in values:
            if not isinstance(value,bytes) or not 0<len(value)<=131072: raise ValueError()
            fd=os.memfd_create('staging-p3',os.MFD_CLOEXEC); fds.append(fd); os.fchmod(fd,0o600)
            pos=0
            while pos<len(value):
                n=os.write(fd,value[pos:])
                if n<=0: raise ValueError()
                pos+=n
            os.lseek(fd,0,os.SEEK_SET); paths.append('/proc/self/fd/'+str(fd))
        result=subprocess.run(['/usr/bin/openssl',*[x.format(*paths) for x in args]],stdout=subprocess.PIPE,
                              stderr=subprocess.DEVNULL,pass_fds=tuple(fds),timeout=30,check=True,
                              env={'PATH':'/usr/bin:/bin','LANG':'C'})
        if len(result.stdout)>262144: raise ValueError()
        return result.stdout
    except Exception: raise CryptoError('staging P3 cryptography unavailable') from None
    finally:
        for fd in fds: os.close(fd)


def _ca(cert):
    constraints=_run(['x509','-in','{0}','-noout','-ext','basicConstraints'],[cert])
    if b'CA:TRUE' not in constraints: raise ValueError()
    _run(['x509','-in','{0}','-noout','-checkend','86400'],[cert])


def validate_issuer_key(key,cert):
    """Validate a key pair only; this does not establish issuer authority."""
    try:
        _ca(cert)
        if _run(['x509','-in','{0}','-pubkey','-noout'],[cert])!=_run(['pkey','-in','{0}','-pubout'],[key]): raise ValueError()
    except Exception: raise CryptoError('staging P3 issuer key mismatch') from None


def _peer(cert, ca, purpose, hostname=None):
    args=['verify','-trusted','{1}','-purpose','sslserver' if purpose=='serverAuth' else 'sslclient']
    if hostname: args+=['-verify_hostname',hostname]
    _run(args+['{0}'],[cert,ca]); _run(['x509','-in','{0}','-noout','-checkend','86400'],[cert])
    if hostname:
        sans=_run(['x509','-in','{0}','-noout','-ext','subjectAltName'],[cert]).decode('ascii')
        if hostname not in re.findall(r'DNS:([A-Za-z0-9.-]+)',sans): raise ValueError()


def _validity_within(cert,issuer):
    try:
        def expiry(value):
            line=_run(['x509','-in','{0}','-noout','-enddate'],[value]).decode('ascii').strip()
            if not line.startswith('notAfter='): raise ValueError()
            return ssl.cert_time_to_seconds(line.removeprefix('notAfter='))
        if expiry(cert)>expiry(issuer): raise ValueError()
    except Exception: raise CryptoError('staging P3 certificate exceeds issuer validity') from None


def _leaf(cert,key,ca,host,purposes):
    for purpose in purposes: _peer(cert,ca,purpose,host)
    ext=_run(['x509','-in','{0}','-noout','-ext','extendedKeyUsage'],[cert])
    if any((b'TLS Web Server Authentication' if p=='serverAuth' else b'TLS Web Client Authentication') not in ext for p in purposes):
        raise ValueError()
    constraints=_run(['x509','-in','{0}','-noout','-ext','basicConstraints'],[cert])
    if b'CA:FALSE' not in constraints or b'CA:TRUE' in constraints: raise ValueError()
    if _run(['x509','-in','{0}','-pubkey','-noout'],[cert])!=_run(['pkey','-in','{0}','-pubout'],[key]): raise ValueError()


def _jwks(keys):
    rows=[]
    for kid in ('current','next'):
        key=keys[kid+'.pem']
        if not key.startswith(b'-----BEGIN PRIVATE KEY-----\n') or key.count(b'-----BEGIN ')!=1: raise ValueError()
        _run(['pkey','-in','{0}','-check','-noout'],[key])
        modulus=int(_run(['rsa','-in','{0}','-noout','-modulus'],[key]).decode().strip().removeprefix('Modulus='),16)
        if modulus.bit_length()<2048: raise ValueError()
        text=_run(['pkey','-in','{0}','-pubout','-text_pub','-noout'],[key]).decode()
        match=re.search(r'^Exponent: ([0-9]+)',text,re.MULTILINE)
        if not match: raise ValueError()
        encode=lambda n:base64.urlsafe_b64encode(n.to_bytes((n.bit_length()+7)//8,'big')).decode().rstrip('=')
        rows.append({'kty':'RSA','kid':kid,'use':'sig','alg':'RS256','n':encode(modulus),'e':encode(int(match[1]))})
    if rows[0]['n']==rows[1]['n']: raise ValueError()
    return {'keys':rows}


def _unique(pairs):
    result={}
    for key,value in pairs:
        if key in result: raise ValueError()
        result[key]=value
    return result


def validate_values(values, trust, space_client_cert, space_jwks_cert, space_server_ca):
    """Validate explicit inputs only; source/CA custody remains caller authority."""
    try:
        contract.material_objects(values)
        if not isinstance(trust,dict) or set(trust)!=TRUST_KEYS or values['voice-p3-principal-trust']!=trust: raise ValueError()
        for cert in (*trust.values(),space_server_ca): _ca(cert)
        _peer(space_client_cert,trust['space-client-ca.crt'],'clientAuth')
        _peer(space_jwks_cert,trust['space-jwks-ca.crt'],'serverAuth','voice-space.voice-staging.svc')
        _leaf(**{'cert':values['voice-realtime-p3-tls']['tls.crt'],'key':values['voice-realtime-p3-tls']['tls.key'],
                 'ca':trust['realtime-client-ca.crt'],'host':LEAVES['voice-realtime-p3-tls'][0],
                 'purposes':LEAVES['voice-realtime-p3-tls'][1]})
        _leaf(values['voice-realtime-p3-tls']['tls.crt'],values['voice-realtime-p3-tls']['tls.key'],
              space_server_ca,LEAVES['voice-realtime-p3-tls'][0],('serverAuth',))
        _leaf(values['voice-chat-realtime-p3-tls']['tls.crt'],values['voice-chat-realtime-p3-tls']['tls.key'],
              trust['chat-server-ca.crt'],*LEAVES['voice-chat-realtime-p3-tls'])
        _leaf(values['voice-p3-principal-jwks-tls']['tls.crt'],values['voice-p3-principal-jwks-tls']['tls.key'],
              trust['realtime-jwks-ca.crt'],*LEAVES['voice-p3-principal-jwks-tls'])
        expected=_jwks(values['voice-realtime-principal-signing'])
        if json.loads(values['voice-p3-principal-public']['realtime.json'],object_pairs_hook=_unique)!=expected: raise ValueError()
        password=values['voice-p3-principal-replay']['password']
        if len(password)>4096 or any(x in password for x in (b'\x00',b'\n',b'\r')): raise ValueError()
        return {'namespace':'voice-staging','scope':'offline-material-only',
                'trust_der_sha256':{name:hashlib.sha256(_run(['x509','-in','{0}','-outform','DER'],[cert])).hexdigest()
                                    for name,cert in sorted(trust.items())},
                'trust_bytes_sha256':{name:hashlib.sha256(cert).hexdigest() for name,cert in sorted(trust.items())},
                'space_server_ca_sha256':hashlib.sha256(space_server_ca).hexdigest(),
                'space_client_cert_sha256':hashlib.sha256(_run(['x509','-in','{0}','-outform','DER'],[space_client_cert])).hexdigest(),
                'space_jwks_cert_sha256':hashlib.sha256(_run(['x509','-in','{0}','-outform','DER'],[space_jwks_cert])).hexdigest()}
    except Exception: raise CryptoError('staging P3 material not qualified') from None


def issue_private_bytes(key, ca_cert, trust, space_client_cert, space_jwks_cert, space_server_ca, replay_password):
    """Issue offline from bytes; custody and issuer authority are external."""
    try:
        if not isinstance(key,bytes) or not 0<len(key)<=131072: raise ValueError()
        validate_issuer_key(key,ca_cert)
        _run(['x509','-in','{0}','-noout','-checkend','2592000'],[ca_cert])
        def genkey(): return _run(['genpkey','-algorithm','RSA','-pkeyopt','rsa_keygen_bits:2048'])
        signing={'current.pem':genkey(),'next.pem':genkey()}
        values={'voice-realtime-principal-signing':signing,
                'voice-p3-principal-public':{'realtime.json':(json.dumps(_jwks(signing),separators=(',',':'))+'\n').encode()},
                'voice-p3-principal-replay':{'password':replay_password},'voice-p3-principal-trust':dict(trust)}
        for name,(host,purposes) in LEAVES.items():
            leaf_key=genkey(); csr=_run(['req','-new','-key','{0}','-subj','/CN='+host],[leaf_key])
            ext=('basicConstraints=critical,CA:FALSE\nsubjectAltName=DNS:'+host+'\nextendedKeyUsage='+','.join(purposes)+'\n').encode()
            serial='0x'+secrets.token_hex(16)
            cert=_run(['x509','-req','-in','{0}','-CA','{1}','-CAkey','{2}','-set_serial',serial,
                       '-days','30','-sha256','-extfile','{3}'],[csr,ca_cert,key,ext])
            _validity_within(cert,ca_cert)
            values[name]={'tls.crt':cert,'tls.key':leaf_key}
        validate_values(values,trust,space_client_cert,space_jwks_cert,space_server_ca)
        return values
    except Exception: raise CryptoError('staging P3 issuance refused') from None


def issue_values(ca_key_fd, ca_cert, trust, space_client_cert, space_jwks_cert, space_server_ca, replay_password):
    """Issue fixed leaves from an already root-custodied CA descriptor, offline."""
    try:
        if sys.platform!='linux' or os.geteuid()!=0: raise ValueError()
        info=os.fstat(ca_key_fd)
        if (not stat.S_ISREG(info.st_mode) or info.st_uid!=0 or info.st_nlink!=1 or info.st_mode&0o077
                or not 0<info.st_size<=131072): raise ValueError()
        identity=(info.st_dev,info.st_ino,info.st_size,info.st_mtime_ns,info.st_ctime_ns)
        key=os.pread(ca_key_fd,info.st_size+1,0)
        if len(key)!=info.st_size: raise ValueError()
        values=issue_private_bytes(key,ca_cert,trust,space_client_cert,space_jwks_cert,space_server_ca,replay_password)
        after=os.fstat(ca_key_fd)
        if (after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns,after.st_ctime_ns)!=identity: raise ValueError()
        return values
    except Exception: raise CryptoError('staging P3 issuance refused') from None
