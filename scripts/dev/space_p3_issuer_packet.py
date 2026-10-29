"""Private packet qualification for the fixed missing DEV Space identities."""
import base64
import hashlib
import json
import re

import realtime_p3_create_only_crypto as crypto

MEMBERS = {'voice-space-principal-signing':{'current.pem','next.pem','active-kid'},
           'voice-space-media-server-tls':{'tls.crt','tls.key'}}


class PacketError(RuntimeError): pass


def validate(envelope,operation_id,source_sha,media):
    try:
        raw = json.dumps(envelope,sort_keys=True,separators=(',',':'),allow_nan=False).encode()
        if len(raw)>524288: raise ValueError()
        value = json.loads(raw)
        if (set(value) != {'operation_id','source_sha','issuer_receipt','objects'} or value['operation_id'] != operation_id
            or not re.fullmatch(r'media-c[12]-[a-f0-9]{32}',operation_id) or value['source_sha'] != source_sha
            or not re.fullmatch(r'[a-f0-9]{40}',source_sha) or not isinstance(media,bytes)
            or not re.fullmatch(rb'-----BEGIN CERTIFICATE-----\r?\n(?:[A-Za-z0-9+/=]+\r?\n)+-----END CERTIFICATE-----\r?\n?',media)):
            raise ValueError()
        receipt = value['issuer_receipt']; bundle = value['objects']
        if (set(receipt) != {'status','namespace','object_names','media_ca_der_sha256','public_jwks','runtime_qualified','cluster_applied'}
            or receipt['status'] != 'staged' or receipt['namespace'] != 'voice-dev'
            or receipt['object_names'] != sorted(MEMBERS) or receipt['runtime_qualified'] is not False
            or receipt['cluster_applied'] is not False or receipt['media_ca_der_sha256'] != hashlib.sha256(crypto.der(media)).hexdigest()
            or set(bundle) != {'apiVersion','kind','items'} or (bundle['apiVersion'],bundle['kind']) != ('v1','List')
            or not isinstance(bundle['items'],list) or len(bundle['items']) != 2): raise ValueError()
        result = {}; decoded = {}
        for obj in bundle['items']:
            if (set(obj) != {'apiVersion','kind','metadata','type','data'} or (obj['apiVersion'],obj['kind'],obj['type']) != ('v1','Secret','Opaque')
                or set(obj['metadata']) != {'name','namespace'} or obj['metadata']['namespace'] != 'voice-dev'): raise ValueError()
            name = obj['metadata']['name']
            if name not in MEMBERS or name in result or not isinstance(obj['data'],dict) or set(obj['data']) != MEMBERS[name]: raise ValueError()
            values = {key:base64.b64decode(item,validate=True) for key,item in obj['data'].items()}
            if any(not item or len(item)>131072 for item in values.values()): raise ValueError()
            result[name] = obj; decoded[name] = values
        signing = decoded['voice-space-principal-signing']
        if signing['active-kid'] != b'current': raise ValueError()
        jwks = crypto.signer_jwks(signing)
        if jwks != receipt['public_jwks']: raise ValueError()
        tls = decoded['voice-space-media-server-tls']
        crypto.certificate(tls['tls.crt'],tls['tls.key'],media,'voice-space.voice-dev.svc','serverAuth')
        return result,jwks
    except Exception:
        raise PacketError('Space identity packet qualification failed') from None
