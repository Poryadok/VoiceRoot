"""Fixed nine-name DEV peer identity inventory; no Secret value output."""
import copy
import os
import re

import space_p3_issuer_preflight_metadata_root as base

TEMPLATE=base.TEMPLATE

PROFILES={
    'creator':('voice-space-media-client-tls','voice-gateway-media-client-tls',
               'voice-auth-media-server-tls','voice-role-media-server-tls','voice-gateway-principal-signing'),
    'space':('voice-space-principal-signing','voice-space-media-server-tls'),
    'role':('voice-role-media-client-tls','voice-role-principal-signing'),
}
MEMBERS={name:({'current.pem','next.pem','active-kid'} if name.endswith('-principal-signing') else {'tls.crt','tls.key'})
         for names in PROFILES.values() for name in names}
EXTRA=set(MEMBERS)-set(base.MEMBERS)
if len(MEMBERS)!=9 or set(base.MEMBERS)!=set(PROFILES['creator']) or len(EXTRA)!=4:
    raise RuntimeError('fixed peer inventory differs from reviewed Creator preflight')

def parse(raw,name):
    if name not in MEMBERS: raise ValueError('unknown peer identity')
    if not raw: return {'state':'absent'}
    lines=raw.decode('ascii').splitlines()
    if (len(lines)<3 or any(not re.fullmatch(r'[A-Za-z0-9.-]{1,128}',v) for v in lines[:2]) or
        lines[2] not in ('','<no value>') or len(lines[3:])!=len(set(lines[3:])) or
        set(lines[3:])!=MEMBERS[name]): raise ValueError('peer identity metadata invalid')
    return {'state':'present','uid':lines[0],'resourceVersion':lines[1],'keys':sorted(lines[3:])}

def observe(namespace_uid,context,lock,ca):
    """Independently recheck TT/namespace/lock/CA and all nine metadata twice."""
    first=base.observe(namespace_uid,context,lock,ca)
    binary,bid=base._open(base.K3S,512*1024*1024)
    try:
        config,cid=base._open(base.KUBECONFIG,1024*1024)
        try:
            if base._digest(binary,bid[2])!=base.K3S_SHA256: raise ValueError()
            def extra():
                return {name:parse(base._read(binary,config,['-n','voice-dev','get','secret',name,
                    '--ignore-not-found=true','-o','go-template='+base.TEMPLATE]),name) for name in sorted(EXTRA)}
            a=extra(); middle=base.observe(namespace_uid,context,lock,ca); b=extra()
            final=base.observe(namespace_uid,context,lock,ca)
            if (first!=middle or middle!=final or a!=b or not base.custody._unchanged(binary,bid) or
                not base.custody._unchanged(config,cid) or
                (base.K3S.lstat().st_dev,base.K3S.lstat().st_ino)!=bid[:2] or
                (base.KUBECONFIG.lstat().st_dev,base.KUBECONFIG.lstat().st_ino)!=cid[:2] or
                base._digest(binary,bid[2])!=base.K3S_SHA256): raise ValueError()
            return copy.deepcopy(first|a)
        finally: os.close(config)
    finally: os.close(binary)
