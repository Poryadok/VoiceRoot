"""Prepare-only DEV P3 object observation; caller supplies the live claim guard.

Private payloads are held only in memory and in reset_credentials' protected
temporary openssl files. This is not an admission or durable trust receipt.
The caller must set TMPDIR to protected ephemeral storage on pmd and provide
its real prepare-only TT/namespace guard; GuardLost propagates to the caller.
TLS checks prove a leaf against the named trust key, not a peer's actual
configuration. No live traffic, schema or Redis authentication is tested.
"""
import hashlib
import json

import full_cohort_schema_metadata as metadata
import realtime_p3_adoption as adoption
import reset_credentials as reset

OBJECTS = {name: ('ConfigMap' if name in ('voice-app-config', 'voice-p3-principal-public') else 'Secret', keys)
           for name, keys in adoption.ASSETS.items()}
TLS = {
    'voice-realtime-p3-tls': ('realtime-client-ca.crt', 'voice-realtime.voice-dev.svc', ('serverAuth', 'clientAuth')),
    'voice-chat-realtime-p3-tls': ('chat-server-ca.crt', 'voice-chat.voice-dev.svc', ('serverAuth',)),
    'voice-p3-principal-jwks-tls': ('realtime-jwks-ca.crt', 'voice-p3-principal-jwks.voice-dev.svc', ('serverAuth',)),
}


def linux_read(kind, name):
    """Fixed credential-free command; stdout is private and size/time bounded."""
    if name not in OBJECTS or OBJECTS[name][0] != kind:
        raise ValueError('object not allowlisted')
    return metadata.linux_private_bytes(['kubectl', '--kubeconfig', '/home/pmd/.kube/config',
        '--context', 'default', '--namespace', 'voice-dev', '--request-timeout=15s',
        'get', kind.lower(), name, '--ignore-not-found', '-o', 'json'])


def _read(read, kind, name):
    raw = read(kind, name)
    if not raw.strip():
        return None
    if len(raw) > 65536:
        raise ValueError()
    obj = json.loads(raw)
    m = obj['metadata']
    if (obj.get('apiVersion') != 'v1' or obj.get('kind') != kind or
            m.get('name') != name or m.get('namespace') != 'voice-dev' or
            not all(isinstance(m.get(k), str) and m[k] for k in ('uid', 'resourceVersion'))):
        raise ValueError()
    return obj


def _identity(obj):
    return None if obj is None else (obj['metadata']['uid'], obj['metadata']['resourceVersion'])


def inspect(read, guard):
    """Caller authenticates source and claim/namespace before and during reads.

    read must be linux_read in production; injected readers are for tests. No
    callback or output from this function constitutes rollout authorization.
    """
    try:
        metadata._fence(guard)
        objects = {}
        for name, (kind, _) in OBJECTS.items():
            metadata._fence(guard)
            objects[name] = _read(read, kind, name)
        metadata._fence(guard)
        assets = {}
        for name, (kind, required) in OBJECTS.items():
            obj = objects[name]
            if obj is None:
                assets[name] = {'status': 'missing'}
                continue
            keys = set(reset.values(obj))
            present = sorted(required & keys)
            exact = name not in ('voice-app-secrets', 'voice-app-config')
            assets[name] = {'status': 'present', 'uid': obj['metadata']['uid'],
                            'resource_version': obj['metadata']['resourceVersion'],
                            'keys_present': present, 'required_keys_present': required <= keys,
                            'extra_keys': bool(keys - required) if exact else None}
            if name == 'voice-p3-principal-public':
                assets[name]['immutable'] = obj.get('immutable') is True
        checks = {}
        signer, public = objects['voice-realtime-principal-signing'], objects['voice-p3-principal-public']
        if signer is not None and public is not None:
            try:
                reset.verify_p3_public(signer, public)
                checks['signer_jwks'] = 'valid'
            except Exception:
                checks['signer_jwks'] = 'invalid'
        else:
            checks['signer_jwks'] = 'unknown'
        trust = objects['voice-p3-principal-trust']
        for name, (ca_key, hostname, purposes) in TLS.items():
            leaf = objects[name]
            checks[name] = 'unknown'
            if leaf is not None and trust is not None:
                try:
                    data, ca = reset.values(leaf), reset.values(trust)
                    for purpose in purposes:
                        reset.verify_certificate(data['tls.crt'], data['tls.key'], ca[ca_key], hostname, purpose)
                    checks[name] = 'valid'
                except Exception:
                    checks[name] = 'invalid'
        public_hashes = {}
        for name, key, label in [('voice-p3-principal-public', 'realtime.json', 'realtime.json'),
                ('voice-realtime-p3-tls', 'tls.crt', 'realtime-tls.crt'),
                ('voice-chat-realtime-p3-tls', 'tls.crt', 'chat-tls.crt'),
                ('voice-p3-principal-jwks-tls', 'tls.crt', 'jwks-tls.crt')]:
            check = 'signer_jwks' if key == 'realtime.json' else name
            if objects[name] is not None and checks[check] == 'valid':
                value = reset.values(objects[name]).get(key)
                if value is not None: public_hashes[label] = hashlib.sha256(value).hexdigest()
        for name, (kind, _) in OBJECTS.items():
            metadata._fence(guard)
            if _identity(objects[name]) != _identity(_read(read, kind, name)):
                raise ValueError()
        metadata._fence(guard)
        return {'status': 'observed', 'assets': assets, 'public_hashes': public_hashes,
                'crypto_checks': checks, 'admission_qualified': False,
                'unobserved': ['runtime_peer_configuration_and_transport', 'redis_auth', 'schema_job', 'service_binding']}
    except metadata.GuardLost:
        raise
    except Exception:
        return {'status': 'hold', 'reason': 'observation_failed'}
