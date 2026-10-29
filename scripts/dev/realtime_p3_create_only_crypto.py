"""Canonical P3 OpenSSL checks using anonymous in-memory descriptors."""
import base64
import json
import os
import re
import subprocess


class CryptoError(RuntimeError): pass


def _call(arguments, values):
    fds = []
    try:
        paths = []
        for value in values:
            if not isinstance(value,bytes) or not 0 < len(value) <= 131072: raise ValueError()
            fd = os.memfd_create('p3-check',os.MFD_CLOEXEC); fds.append(fd)
            os.fchmod(fd,0o600); offset = 0
            while offset < len(value):
                written = os.write(fd,value[offset:])
                if written <= 0: raise ValueError()
                offset += written
            os.lseek(fd,0,os.SEEK_SET)
            paths.append('/proc/self/fd/'+str(fd))
        command = [part.format(*paths) for part in arguments]
        result = subprocess.run(['openssl',*command],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,
                                pass_fds=tuple(fds),timeout=30,check=True)
        if len(result.stdout)>262144: raise ValueError()
        return result.stdout
    except Exception:
        raise CryptoError('P3 cryptographic validation unavailable') from None
    finally:
        for fd in fds: os.close(fd)


def der(cert):
    return _call(['x509','-in','{0}','-outform','DER'],[cert])


def certificate(cert,key,ca,hostname,purpose):
    """Apply reset_credentials.verify_certificate checks without key files."""
    if purpose not in ('serverAuth','clientAuth') or not re.fullmatch(r'[a-zA-Z0-9.-]+',hostname):
        raise CryptoError('P3 certificate contract invalid')
    _call(['verify','-CAfile','{1}','-purpose','sslserver' if purpose == 'serverAuth' else 'sslclient',
           '-verify_hostname',hostname,'{0}'],[cert,ca])
    _call(['x509','-in','{0}','-noout','-checkend','86400'],[cert])
    sans = _call(['x509','-in','{0}','-noout','-ext','subjectAltName'],[cert]).decode('ascii')
    if hostname not in re.findall(r'DNS:([A-Za-z0-9.-]+)',sans): raise CryptoError('P3 certificate SAN invalid')
    eku = _call(['x509','-in','{0}','-noout','-ext','extendedKeyUsage'],[cert])
    if (b'TLS Web Server Authentication' if purpose == 'serverAuth' else b'TLS Web Client Authentication') not in eku:
        raise CryptoError('P3 certificate purpose invalid')
    constraints = _call(['x509','-in','{0}','-noout','-ext','basicConstraints'],[cert])
    if b'CA:FALSE' not in constraints or b'CA:TRUE' in constraints: raise CryptoError('P3 leaf invalid')
    if _call(['x509','-in','{0}','-pubkey','-noout'],[cert]) != _call(['pkey','-in','{0}','-pubout'],[key]):
        raise CryptoError('P3 certificate key mismatch')


def signer_jwks(keys):
    """Apply reset_credentials.signer_jwks checks without key files."""
    result = []
    for kid in ('current','next'):
        raw = keys[kid+'.pem']
        if not raw.splitlines() or raw.splitlines()[0] != b'-----BEGIN PRIVATE KEY-----' or raw.count(b'-----BEGIN ') != 1:
            raise CryptoError('P3 signing key format invalid')
        _call(['pkey','-in','{0}','-check','-noout'],[raw])
        modulus = _call(['rsa','-in','{0}','-noout','-modulus'],[raw]).decode().strip().removeprefix('Modulus=')
        number = int(modulus,16)
        if number.bit_length() < 2048: raise CryptoError('P3 signing key too small')
        text = _call(['pkey','-in','{0}','-pubout','-text_pub','-noout'],[raw]).decode()
        match = re.search(r'^Exponent: ([0-9]+)',text,re.MULTILINE)
        if not match: raise CryptoError('P3 signing key exponent unavailable')
        exponent = int(match[1]); encode = lambda value: base64.urlsafe_b64encode(value.to_bytes((value.bit_length()+7)//8,'big')).decode().rstrip('=')
        result.append({'kty':'RSA','kid':kid,'use':'sig','alg':'RS256','n':encode(number),'e':encode(exponent)})
    if result[0]['n'] == result[1]['n']: raise CryptoError('P3 signing keys duplicate')
    return {'keys':result}


def space_certificate(cert,ca):
    _call(['verify','-CAfile','{1}','-purpose','sslserver','-verify_hostname',
           'voice-space.voice-dev.svc','{0}'],[cert,ca])
    _call(['x509','-in','{0}','-noout','-checkend','86400'],[cert])
