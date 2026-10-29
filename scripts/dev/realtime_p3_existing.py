"""Fresh current P3 content/crypto observation; predecessor authority separate."""
import base64
import copy
import hashlib

import frontend_runtime_executor
import media_operation_guard
import realtime_p3_create_only as create
import realtime_p3_create_only_crypto as crypto
import realtime_p3_create_only_packet as packet
import realtime_p3_create_only_source as source
import realtime_p3_create_only_transport as transport


class ExistingError(RuntimeError): pass


def observe(fence,client,lock,namespace_uid,source_root):
    """Read exact current objects and cryptography; does not establish C1 ownership."""
    try:
        if type(client) is not frontend_runtime_executor.KubectlTransport or fence.ledger.client is not client: raise ValueError()
        operation = fence.context['operation_id']
        if not operation.startswith('media-c2-'): raise ValueError()
        def guard(): media_operation_guard.verify(fence,client,lock,namespace_uid)
        def read():
            roots = {}; identities = {}; items = []
            for name,key in create.PUBLIC:
                guard(); raw=transport.public_certificate(name,key); guard(); parts=raw.split(b'\n',2)
                if len(parts)!=3 or not all(parts): raise ValueError()
                cert=base64.b64decode(parts[2],validate=True)
                roots[name] = {'uid':parts[0].decode('ascii'),'resourceVersion':parts[1].decode('ascii'),'certificate':cert}
            for kind,name in sorted(packet.MEMBERS):
                guard(); obj=transport.get(kind,name); guard(); meta=obj['metadata']
                if (obj.get('apiVersion')!='v1' or obj.get('kind')!=kind or meta.get('namespace')!='voice-dev'
                        or meta.get('name')!=name or meta.get('deletionTimestamp')
                        or (kind=='Secret' and obj.get('immutable',False) is not False)
                        or (kind=='ConfigMap' and obj.get('binaryData'))
                        or any(not isinstance(meta.get(k),str) or not meta[k] for k in ('uid','resourceVersion'))): raise ValueError()
                normalized={'apiVersion':'v1','kind':kind,'metadata':{'name':name,'namespace':'voice-dev'},'data':copy.deepcopy(obj.get('data'))}
                if kind=='Secret': normalized['type']=obj.get('type')
                else: normalized['immutable']=obj.get('immutable')
                items.append(normalized); identities[name]={'kind':kind,'uid':meta['uid'],'resourceVersion':meta['resourceVersion']}
            return roots,identities,items
        guard(); source.verify(source_root,source.REVIEWED_ISSUER_SHA,fence.context['commit']); guard()
        before,revision=fence.ledger.read(); roots,identities,items=read()
        digest=lambda name:hashlib.sha256(crypto.der(roots[name]['certificate'])).hexdigest()
        receipt={'status':'staged','namespace':'voice-dev','object_names':sorted(identities),
                 'principal_ca_der_sha256':digest('voice-principal-ca'),
                 'media_ca_der_sha256':digest('voice-media-ca'),
                 'space_jwks_cert_der_sha256':digest('voice-space-media-server-tls'),
                 'runtime_qualified':False,'cluster_applied':False}
        envelope={'operation_id':operation,'source_sha':source.REVIEWED_ISSUER_SHA,'issuer_receipt':receipt,
                  'objects':{'apiVersion':'v1','kind':'List','items':items}}
        packet.validate(envelope,operation,source.REVIEWED_ISSUER_SHA,roots['voice-principal-ca']['certificate'],
                        roots['voice-media-ca']['certificate'],roots['voice-space-media-server-tls']['certificate'])
        public_sources = {name:{'uid':value['uid'],'resourceVersion':value['resourceVersion'],
            'der_sha256':receipt[key]} for name,value,key in (
                ('voice-principal-ca',roots['voice-principal-ca'],'principal_ca_der_sha256'),
                ('voice-media-ca',roots['voice-media-ca'],'media_ca_der_sha256'),
                ('voice-space-media-server-tls',roots['voice-space-media-server-tls'],'space_jwks_cert_der_sha256'))}
        result = {'operation_id':operation,'source_sha':fence.context['commit'],'namespace_uid':namespace_uid,
                'objects':copy.deepcopy(identities),'public_der_sha256':{k:v for k,v in receipt.items() if k.endswith('_sha256')},
                'public_sources':public_sources,
                'scope':'current-content-only-predecessor-unverified'}
        guard(); again=read(); guard(); after,end=fence.ledger.read()
        if again != (roots,identities,items) or before != after or revision != end: raise ValueError()
        guard()
        return result
    except Exception:
        raise ExistingError('current P3 content unavailable') from None


def verify_reuse(fence,client,lock,namespace_uid,predecessor_journal,source_root):
    """Bind current C2 cryptography to the authenticated completed C1 assets."""
    try:
        import full_cohort_predecessor_phase as predecessor
        prior=predecessor.load(fence,client,lock,namespace_uid,predecessor_journal)
        current=observe(fence,client,lock,namespace_uid,source_root)
        receipt=prior['p3_receipt']
        if (receipt.get('operation_id')!=prior['previous_operation_id']
                or receipt.get('issuer_source_sha')!=source.REVIEWED_ISSUER_SHA
                or receipt.get('namespace_uid')!=namespace_uid
                or receipt.get('public_sources')!=current['public_sources']): raise ValueError()
        expected={}
        for identity, entry in receipt['objects'].items():
            name=entry['name']; kind=entry['kind']
            if identity!=kind.lower()+'-'+name or name in expected: raise ValueError()
            expected[name]={'kind':kind,'uid':entry['uid'],'resourceVersion':entry['resourceVersion']}
        saved=copy.deepcopy(predecessor_journal.state['steps']['full-cohort-predecessor'])
        row,revision=fence.ledger.read()
        if (expected!=current['objects'] or predecessor.load(fence,client,lock,namespace_uid,predecessor_journal)!=prior
                or fence.ledger.read()!=(row,revision)
                or predecessor_journal.state['steps']['full-cohort-predecessor']!=saved): raise ValueError()
        def guard(): media_operation_guard.verify(fence,client,lock,namespace_uid)
        for kind,name in sorted(packet.MEMBERS):
            guard(); actual=transport.get(kind,name); guard(); meta=actual['metadata']; wanted=current['objects'][name]
            if (actual.get('apiVersion')!='v1' or actual.get('kind')!=kind or meta.get('name')!=name
                    or meta.get('namespace')!='voice-dev' or meta.get('deletionTimestamp')
                    or meta.get('uid')!=wanted['uid'] or meta.get('resourceVersion')!=wanted['resourceVersion']): raise ValueError()
        for name,key in create.PUBLIC:
            guard(); parts=transport.public_certificate(name,key).split(b'\n',2); guard()
            if len(parts)!=3: raise ValueError()
            expected_public=current['public_sources'][name]
            if (parts[0].decode('ascii')!=expected_public['uid'] or parts[1].decode('ascii')!=expected_public['resourceVersion']
                    or hashlib.sha256(crypto.der(base64.b64decode(parts[2],validate=True))).hexdigest()!=expected_public['der_sha256']): raise ValueError()
        guard()
        if fence.ledger.read()!=(row,revision) or predecessor_journal.state['steps']['full-cohort-predecessor']!=saved: raise ValueError()
        return {'operation_id':current['operation_id'],'source_sha':current['source_sha'],
                'namespace_uid':namespace_uid,'previous_operation_id':prior['previous_operation_id'],
                'objects':copy.deepcopy(current['objects']),'public_sources':copy.deepcopy(current['public_sources']),
                'scope':'c2-p3-exact-predecessor-reuse'}
    except Exception:
        raise ExistingError('P3 predecessor reuse unavailable') from None
