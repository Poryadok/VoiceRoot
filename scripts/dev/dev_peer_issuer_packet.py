"""Closed nine-name peer material packet, independent of operation authority."""
import base64
import copy
import hashlib
import json
import re

import dev_peer_issuer_metadata as metadata
import space_creator_runtime_profile_issuer_packet as crypto

TLS={
    'voice-space-media-client-tls':('voice-space','clientAuth'),
    'voice-gateway-media-client-tls':('voice-gateway','clientAuth'),
    'voice-auth-media-server-tls':('voice-auth','serverAuth'),
    'voice-role-media-server-tls':('voice-role','serverAuth'),
    'voice-role-media-client-tls':('voice-role','clientAuth'),
    'voice-space-media-server-tls':('voice-space','serverAuth'),
}

class PacketError(RuntimeError): pass

def validate(envelope,profile,operation_id,source_sha,namespace_uid,preflight_sha256,missing,media):
    try:
        raw=json.dumps(envelope,sort_keys=True,separators=(',',':'),allow_nan=False).encode()
        if len(raw)>524288: raise ValueError()
        value=json.loads(raw)
        if (set(value)!={'schema','profile','operation_id','source_sha','namespace_uid','preflight_sha256','issuer_receipt','objects'} or
            value['schema']!='voice-dev-peer-issuer-packet-v1' or profile not in metadata.PROFILES or value['profile']!=profile or
            value['operation_id']!=operation_id or not re.fullmatch(r'media-c1-[a-f0-9]{32}',operation_id) or
            value['source_sha']!=source_sha or not re.fullmatch(r'[a-f0-9]{40}',source_sha) or
            value['namespace_uid']!=namespace_uid or not isinstance(namespace_uid,str) or not namespace_uid or
            value['preflight_sha256']!=preflight_sha256 or not re.fullmatch(r'[a-f0-9]{64}',preflight_sha256) or
            not isinstance(missing,list) or not missing or missing!=sorted(set(missing)) or
            set(missing)-set(metadata.PROFILES[profile])): raise ValueError()
        receipt=value['issuer_receipt']; bundle=value['objects']
        der=crypto._openssl(['x509','-in','{0}','-outform','DER'],[media])
        if (set(receipt)!={'schema','status','profile','object_names','media_ca_der_sha256','public_jwks','runtime_qualified','cluster_applied'} or
            receipt['schema']!='voice-dev-peer-issuer-receipt-v1' or receipt['status']!='staged' or receipt['profile']!=profile or
            receipt['object_names']!=missing or receipt['media_ca_der_sha256']!=hashlib.sha256(der).hexdigest() or
            receipt['runtime_qualified'] is not False or receipt['cluster_applied'] is not False or
            set(bundle)!={'apiVersion','kind','items'} or (bundle['apiVersion'],bundle['kind'])!=('v1','List') or
            not isinstance(bundle['items'],list) or len(bundle['items'])!=len(missing)): raise ValueError()
        found={}; jwks={}
        for obj in bundle['items']:
            if (set(obj)!={'apiVersion','kind','metadata','type','data'} or (obj['apiVersion'],obj['kind'],obj['type'])!=('v1','Secret','Opaque') or
                set(obj['metadata'])!={'name','namespace'} or obj['metadata']['namespace']!='voice-dev'): raise ValueError()
            name=obj['metadata']['name']
            if name not in missing or name in found or not isinstance(obj['data'],dict) or set(obj['data'])!=metadata.MEMBERS[name]: raise ValueError()
            values={key:base64.b64decode(item,validate=True) for key,item in obj['data'].items()}
            if any(not item or len(item)>131072 for item in values.values()): raise ValueError()
            if name.endswith('-principal-signing'):
                if values['active-kid']!=b'current': raise ValueError()
                jwks[name]=crypto._jwks(values)
            else: crypto._certificate(values,media,*TLS[name])
            found[name]=obj
        if set(found)!=set(missing) or jwks!=receipt['public_jwks']: raise ValueError()
        return copy.deepcopy(found)
    except Exception: raise PacketError('DEV peer packet qualification failed') from None
