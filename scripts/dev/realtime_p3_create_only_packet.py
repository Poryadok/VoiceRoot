"""Validate only the seven issued P3 objects against actual public trust."""
import base64
import hashlib
import json
import re

import realtime_p3_create_only_crypto as crypto

MEMBERS = {
    ('Secret','voice-realtime-principal-signing'):{'current.pem','next.pem'},
    ('Secret','voice-realtime-p3-tls'):{'tls.crt','tls.key'},
    ('Secret','voice-chat-realtime-p3-tls'):{'tls.crt','tls.key'},
    ('Secret','voice-p3-principal-jwks-tls'):{'tls.crt','tls.key'},
    ('Secret','voice-p3-principal-replay'):{'password'},
    ('Secret','voice-p3-principal-trust'):{'chat-server-ca.crt','space-client-ca.crt','space-jwks-ca.crt',
                                          'realtime-client-ca.crt','realtime-jwks-ca.crt'},
    ('ConfigMap','voice-p3-principal-public'):{'realtime.json'},
}


class PacketError(RuntimeError): pass


def validate(envelope, operation_id, issuer_source_sha, principal, media, space_cert):
    """Return in-memory exact objects only after actual trust/crypto verification."""
    try:
        raw = json.dumps(envelope,sort_keys=True,separators=(',', ':'),ensure_ascii=True,allow_nan=False).encode('ascii')
        if len(raw) > 2 * 1024 * 1024: raise ValueError()
        envelope = json.loads(raw)
        if (not isinstance(envelope,dict) or set(envelope) != {'operation_id','source_sha','issuer_receipt','objects'}
                or envelope['operation_id'] != operation_id or not re.fullmatch(r'media-c[12]-[0-9a-f]{32}',operation_id)
                or envelope['source_sha'] != issuer_source_sha or not re.fullmatch(r'[0-9a-f]{40}',issuer_source_sha)):
            raise ValueError()
        receipt = envelope['issuer_receipt']; bundle = envelope['objects']
        if (not isinstance(receipt,dict) or set(receipt) != {'status','namespace','object_names','principal_ca_der_sha256',
                'media_ca_der_sha256','space_jwks_cert_der_sha256','runtime_qualified','cluster_applied'}
                or receipt['status'] != 'staged' or receipt['namespace'] != 'voice-dev'
                or receipt['object_names'] != sorted(name for _,name in MEMBERS)
                or receipt['runtime_qualified'] is not False or receipt['cluster_applied'] is not False
                or not isinstance(bundle,dict) or set(bundle) != {'apiVersion','kind','items'}
                or (bundle['apiVersion'],bundle['kind']) != ('v1','List') or not isinstance(bundle['items'],list)
                or len(bundle['items']) != len(MEMBERS)): raise ValueError()
        for name,value in (('principal_ca_der_sha256',principal),('media_ca_der_sha256',media),
                           ('space_jwks_cert_der_sha256',space_cert)):
            if receipt[name] != hashlib.sha256(crypto.der(value)).hexdigest(): raise ValueError()
        crypto.space_certificate(space_cert,media)
        objects = {}; decoded = {}
        for obj in bundle['items']:
            if not isinstance(obj,dict) or obj.get('apiVersion') != 'v1': raise ValueError()
            kind = obj.get('kind'); meta = obj.get('metadata')
            if not isinstance(meta,dict) or set(meta) != {'name','namespace'} or meta.get('namespace') != 'voice-dev': raise ValueError()
            identity = (kind,meta.get('name'))
            if identity not in MEMBERS or identity in objects: raise ValueError()
            if kind == 'Secret':
                if set(obj) != {'apiVersion','kind','metadata','type','data'} or obj['type'] != 'Opaque': raise ValueError()
                data = obj['data']
                if not isinstance(data,dict) or set(data) != MEMBERS[identity] or any(not isinstance(v,str) for v in data.values()): raise ValueError()
                value = {k:base64.b64decode(v,validate=True) for k,v in data.items()}
            else:
                if set(obj) != {'apiVersion','kind','metadata','data','immutable'} or obj['immutable'] is not True: raise ValueError()
                data = obj['data']
                if not isinstance(data,dict) or set(data) != MEMBERS[identity] or any(not isinstance(v,str) for v in data.values()): raise ValueError()
                value = {k:v.encode() for k,v in data.items()}
            if any(len(v)>131072 for v in value.values()): raise ValueError()
            objects[identity] = obj; decoded[identity] = value
        if set(objects) != set(MEMBERS): raise ValueError()
        if decoded['Secret','voice-p3-principal-replay'] != {'password':b''}: raise ValueError()
        trust = decoded['Secret','voice-p3-principal-trust']
        if any(trust[k] != (media if k == 'space-jwks-ca.crt' else principal) for k in trust): raise ValueError()
        signer = decoded['Secret','voice-realtime-principal-signing']
        public = json.loads(decoded['ConfigMap','voice-p3-principal-public']['realtime.json'])
        if public != crypto.signer_jwks(signer): raise ValueError()
        leaves = {'voice-realtime-p3-tls':('voice-realtime.voice-dev.svc',('serverAuth','clientAuth')),
                  'voice-chat-realtime-p3-tls':('voice-chat.voice-dev.svc',('serverAuth',)),
                  'voice-p3-principal-jwks-tls':('voice-p3-principal-jwks.voice-dev.svc',('serverAuth',))}
        for name,(host,purposes) in leaves.items():
            value = decoded['Secret',name]
            for purpose in purposes: crypto.certificate(value['tls.crt'],value['tls.key'],principal,host,purpose)
        return objects
    except Exception:
        raise PacketError('P3 packet qualification failed') from None
