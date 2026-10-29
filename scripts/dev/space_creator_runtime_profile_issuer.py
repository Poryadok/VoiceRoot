"""Fixed Creator identities from the existing media CA; no cluster authority.

The private generation primitive must only be invoked by the source-owned root
guard after it establishes the exact protected missing subset. It is not an
operator entry point or a substitute for that guard.
"""
import hashlib
import json
import os
from pathlib import Path
import re

import credentials
import realtime_p3_missing_issuer as existing
import reset_credentials as reset
import space_p3_issuer as ca_custody
import space_creator_runtime_profile_issuer_packet as packet

class IssuerError(RuntimeError): pass

def _prepare(destination,media,missing,operation_id,source_sha,namespace_uid,preflight_sha256):
    """Generate a fixed subset after an external source-owned guard has admitted it."""
    try:
        if (set(os.environ)-{'PATH','LANG','LC_ALL','LC_CTYPE'} or os.environ.get('PATH')!='/usr/bin:/bin' or
            os.environ.get('LANG')!='C' or os.environ.get('LC_ALL')!='C' or
            not isinstance(missing,list) or not missing or missing!=sorted(set(missing)) or set(missing)-set(packet.FAMILIES) or
            not re.fullmatch(r'media-c1-[a-f0-9]{32}',operation_id) or not re.fullmatch(r'[a-f0-9]{40}',source_sha) or
            not isinstance(namespace_uid,str) or not namespace_uid or not re.fullmatch(r'[a-f0-9]{64}',preflight_sha256) or
            not isinstance(media,bytes)): raise ValueError()
        destination=Path(destination); ca_custody._check_ancestors(ca_custody.ARCHIVE)
        info=ca_custody.ARCHIVE.lstat()
        if (not __import__('stat').S_ISDIR(info.st_mode) or info.st_uid!=0 or info.st_mode&0o077 or
            ca_custody.ARCHIVE.resolve()!=ca_custody.ARCHIVE): raise ValueError()
        cert_fd,cert_id=ca_custody._open('media-ca.crt',False)
        try:
            key_fd,key_id=ca_custody._open('media-ca.key',True)
            try:
                cert=Path('/proc')/str(os.getpid())/'fd'/str(cert_fd); key=Path('/proc')/str(os.getpid())/'fd'/str(key_fd)
                public=destination/'media-public.crt'; credentials.write_private(public,media)
                cert_der=reset._openssl('x509','-in',cert,'-outform','DER')
                if (cert_der!=reset._openssl('x509','-in',public,'-outform','DER') or
                    reset._openssl('x509','-in',cert,'-pubkey','-noout')!=reset._openssl('pkey','-in',key,'-pubout') or
                    b'CA:TRUE' not in reset._openssl('x509','-in',cert,'-noout','-ext','basicConstraints')): raise ValueError()
                material=reset.ResetMaterial(destination)
                for name in missing:
                    spec=packet.FAMILIES[name]
                    if spec is None: material.signer(name,active=True)
                    else: material.tls(name,(key,cert),*spec)
                signer=next((item for item in material.items if item['metadata']['name']=='voice-gateway-principal-signing'),None)
                receipt={'schema':'voice-space-creator-profile-issuer-v1','status':'staged','namespace':'voice-dev',
                         'object_names':missing,'media_ca_der_sha256':hashlib.sha256(cert_der).hexdigest(),
                         'public_jwks':reset.signer_jwks(signer) if signer else None,
                         'runtime_qualified':False,'cluster_applied':False}
                envelope={'schema':'voice-space-creator-profile-packet-v1','operation_id':operation_id,'source_sha':source_sha,
                          'namespace_uid':namespace_uid,'preflight_sha256':preflight_sha256,'issuer_receipt':receipt,
                          'objects':{'apiVersion':'v1','kind':'List','items':material.items}}
                packet.validate(envelope,operation_id,source_sha,namespace_uid,preflight_sha256,missing,media)
                if not existing._unchanged(cert_fd,cert_id) or not existing._unchanged(key_fd,key_id): raise ValueError()
                ca_custody._check_ancestors(ca_custody.ARCHIVE)
                def identity(info): return (info.st_dev,info.st_ino,info.st_size,info.st_mtime_ns,info.st_ctime_ns)
                if (identity(os.stat(ca_custody.ARCHIVE/'media-ca.crt',follow_symlinks=False))!=cert_id or
                    identity(os.stat(ca_custody.ARCHIVE/'media-ca.key',follow_symlinks=False))!=key_id): raise ValueError()
                credentials.write_private(destination/'objects.json',json.dumps(envelope,sort_keys=True,separators=(',',':'))+'\n')
                return receipt
            finally: os.close(key_fd)
        finally: os.close(cert_fd)
    except Exception: raise IssuerError('Creator profile generation refused') from None
