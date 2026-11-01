"""Private GIS credential material; generation itself grants no write authority."""
import base64
import binascii
import contextlib
import copy
import datetime
import hashlib
import json
import os
import re
import secrets
import subprocess
from pathlib import Path


class MaterialError(RuntimeError):
    pass


_EFFECTS = {
    'gis-signer': 'voice-gameintegration-principal-signing',
    'gis-workload': 'voice-gameintegration-workload-credentials',
    'gis-tls': 'voice-gameintegration-jwks-tls',
    'gis-app-credentials': 'voice-app-secrets',
}
_SHA = re.compile(r'[a-f0-9]{64}')
_UUID = re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')
_RV = re.compile(r'[0-9]{1,20}')
_VERIFIER_REVISIONS = {'4297373f6d91364da34f46f959d5df8ea07f71553ebaf52d048f48782f5da11c': 31,
                       '9924cfc09d6c08c22e0d4b0b412f32cf212476ebfc4806b9fccc2c699f4dfd5e': 32,
                       'bb230fc585ff4d8264e0edc7ec2a395e59c94794a437c13696939206676a8225': 33}
_OPENSSL = '/usr/bin/openssl'
_OPENSSL_ENV = {'PATH': '/usr/bin:/bin', 'HOME': '/nonexistent', 'LC_ALL': 'C'}


def validate_plan(plan):
    """Validate a fixed child plan's shape; provenance remains the parent's duty."""
    try:
        if (type(plan) is not dict or set(plan) != {'schema', 'operation', 'namespace_uid',
                'source_capture_sha256', 'source_contract_sha256', 'issuer', 'effects', 'preserved'}
            or plan['schema'] != 'voice-staging-gis-material-plan-v1'
            or not isinstance(plan['operation'], str) or not re.fullmatch(r'[a-f0-9]{12}', plan['operation'])
            or not isinstance(plan['namespace_uid'], str) or not _UUID.fullmatch(plan['namespace_uid'])
            or any(not isinstance(plan[key], str) or not _SHA.fullmatch(plan[key])
                   for key in ('source_capture_sha256', 'source_contract_sha256'))):
            raise ValueError()
        issuer = plan['issuer']
        if (type(issuer) is not dict or set(issuer) != {'input_receipt_sha256', 'ca_secret_uid',
                'ca_public_sha256', 'issuer_certificate_sha256', 'private_input_sha256'}
            or not isinstance(issuer['ca_secret_uid'], str) or not _UUID.fullmatch(issuer['ca_secret_uid'])
            or any(not isinstance(issuer[key], str) or not _SHA.fullmatch(issuer[key])
                   for key in ('input_receipt_sha256', 'ca_public_sha256', 'issuer_certificate_sha256', 'private_input_sha256'))):
            raise ValueError()
        effects, preserved = plan['effects'], plan['preserved']
        if type(effects) is not list or type(preserved) is not dict:
            raise ValueError()
        identifiers = []
        for effect in effects:
            if (type(effect) is not dict or set(effect) != {'id', 'kind', 'name', 'namespace', 'before',
                    'desired_sha256', 'payload_sha256'} or effect['id'] not in _EFFECTS
                or effect['kind'] != 'Secret' or effect['name'] != _EFFECTS[effect['id']]
                or effect['namespace'] != 'voice-staging'
                or any(not isinstance(effect[key], str) or not _SHA.fullmatch(effect[key])
                       for key in ('desired_sha256', 'payload_sha256'))):
                raise ValueError()
            before = effect['before']
            if effect['id'] == 'gis-app-credentials':
                if (type(before) is not dict or set(before) != {'uid', 'resourceVersion', 'data_sha256'}
                    or not isinstance(before['uid'], str) or not _UUID.fullmatch(before['uid'])
                    or not isinstance(before['resourceVersion'], str) or not _RV.fullmatch(before['resourceVersion'])
                    or not isinstance(before['data_sha256'], str) or not _SHA.fullmatch(before['data_sha256'])):
                    raise ValueError()
            elif (type(before) is not dict or set(before) != {'absent', 'list_rv'} or before['absent'] is not True
                  or not isinstance(before['list_rv'], str) or not _RV.fullmatch(before['list_rv'])):
                raise ValueError()
            identifiers.append(effect['id'])
        if identifiers != [key for key in _EFFECTS if key in identifiers] or set(identifiers) & set(preserved):
            raise ValueError()
        if set(identifiers) | set(preserved) != set(_EFFECTS):
            raise ValueError()
        for effect_id, record in preserved.items():
            if type(record) is not dict or set(record) != {'authority', 'observed'}:
                raise ValueError()
            authority, observed = record['authority'], record['observed']
            if (type(authority) is not dict or set(authority) != {'operation', 'receipt_sha256'}
                or not isinstance(authority['operation'], str) or not re.fullmatch(r'[a-f0-9]{12}', authority['operation'])
                or not isinstance(authority['receipt_sha256'], str) or not _SHA.fullmatch(authority['receipt_sha256'])
                or type(observed) is not dict or set(observed) != {'kind', 'name', 'namespace', 'uid', 'resourceVersion', 'semantic_sha256'}
                or observed['kind'] != 'Secret' or observed['name'] != _EFFECTS[effect_id]
                or observed['namespace'] != 'voice-staging' or not isinstance(observed['uid'], str)
                or not _UUID.fullmatch(observed['uid']) or not isinstance(observed['resourceVersion'], str)
                or not _RV.fullmatch(observed['resourceVersion']) or not isinstance(observed['semantic_sha256'], str)
                or not _SHA.fullmatch(observed['semantic_sha256'])):
                raise ValueError()
        return True
    except (KeyError, TypeError, ValueError):
        raise MaterialError('staging_gis_plan_invalid') from None


def execute(session):
    """Dispatch only parent-authorized effects; the parent owns all mutations."""
    import staging_prerequisites as parent
    plan = parent.require_child(session, 'gis_material')
    validate_plan(plan)
    for effect in plan['effects']:
        effect_id = effect['id']
        parent.record_pending(session, 'gis_material', effect_id)
        observation = parent.submit_effect(session, 'gis_material', effect_id)
        parent.record_done(session, 'gis_material', effect_id, observation)


def _canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':')).encode()


def _secret_semantic(row):
    if type(row) is not dict or row.get('apiVersion') != 'v1' or row.get('kind') != 'Secret':
        raise ValueError()
    meta = row['metadata']
    if type(meta) is not dict or meta.get('deletionTimestamp') or meta.get('ownerReferences') or meta.get('finalizers'):
        raise ValueError()
    semantic_meta = {key: meta[key] for key in ('name', 'namespace', 'labels', 'annotations') if key in meta}
    semantic = {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': semantic_meta,
                'type': row.get('type', 'Opaque'), 'data': row['data']}
    if 'immutable' in row: semantic['immutable'] = row['immutable']
    return semantic


def _verify_tls_material(certificate, key, issuer_certificate, issuer_key, trust):
    if any(type(value) is not bytes or not 0 < len(value) <= 256 * 1024
           for value in (certificate, key, issuer_certificate, issuer_key, trust)):
        raise ValueError()
    with contextlib.ExitStack() as stack:
        def memory(value):
            fd = os.memfd_create('voice-gis-private', os.MFD_CLOEXEC)
            stack.callback(os.close, fd)
            remaining = memoryview(value)
            while remaining:
                count = os.write(fd, remaining)
                if count <= 0: raise OSError('private material write failed')
                remaining = remaining[count:]
            os.lseek(fd, 0, os.SEEK_SET)
            return fd
        cert_fd, key_fd, issuer_fd, issuer_key_fd, trust_fd = map(
            memory, (certificate, key, issuer_certificate, issuer_key, trust))
        def run(args, fds):
            for fd in fds: os.lseek(fd, 0, os.SEEK_SET)
            return subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                pass_fds=tuple(fds), check=True, timeout=30, env=_OPENSSL_ENV).stdout
        def public_cert(fd):
            return run([_OPENSSL, 'x509', '-in', f'/proc/self/fd/{fd}', '-pubkey', '-noout'], [fd])
        def public_key(fd):
            return run([_OPENSSL, 'pkey', '-in', f'/proc/self/fd/{fd}', '-pubout'], [fd])
        def expiry(fd):
            result = run([_OPENSSL, 'x509', '-in', f'/proc/self/fd/{fd}', '-enddate', '-noout'], [fd])
            if not result.startswith(b'notAfter='): raise ValueError()
            return datetime.datetime.strptime(result[len(b'notAfter='):].decode('ascii').strip(), '%b %d %H:%M:%S %Y %Z')
        if (not public_cert(cert_fd) or public_cert(cert_fd) != public_key(key_fd)
            or not public_cert(issuer_fd) or public_cert(issuer_fd) != public_key(issuer_key_fd)
            or expiry(cert_fd) > expiry(issuer_fd)):
            raise ValueError()
        san = run([_OPENSSL, 'x509', '-in', f'/proc/self/fd/{cert_fd}', '-ext', 'subjectAltName', '-noout'], [cert_fd])
        if san.splitlines() != [b'X509v3 Subject Alternative Name: ', b'    DNS:voice-gameintegration']:
            raise ValueError()
        for extension, expected in (
                ('basicConstraints', [b'X509v3 Basic Constraints: critical', b'    CA:FALSE']),
                ('keyUsage', [b'X509v3 Key Usage: critical', b'    Digital Signature, Key Encipherment']),
                ('extendedKeyUsage', [b'X509v3 Extended Key Usage: ', b'    TLS Web Server Authentication'])):
            actual = run([_OPENSSL, 'x509', '-in', f'/proc/self/fd/{cert_fd}', '-ext', extension, '-noout'], [cert_fd])
            if actual.splitlines() != expected: raise ValueError()
        for trusted, extra in ((issuer_fd, ['-partial_chain']), (trust_fd, [])):
            run([_OPENSSL, 'verify', '-trusted', f'/proc/self/fd/{trusted}', *extra,
                 '-verify_hostname', 'voice-gameintegration', '-purpose', 'sslserver',
                 f'/proc/self/fd/{cert_fd}'], [trusted, cert_fd])


def _verify_issuer_tls(parent, session, plan, desired, kube):
    issuer = parent.load_issuer(session, 'gis_material')
    if (type(issuer) is not dict or set(issuer) != {'schema', 'receipt', 'ca_certificate_pem',
            'ca_private_key_pem', 'jwks_ca_bundle_pem', 'ca_secret'}
        or issuer['schema'] != 'voice-staging-gis-issuer-input-v1'):
        raise ValueError()
    ca, ca_key, bundle = (issuer[name].encode('ascii') for name in
        ('ca_certificate_pem', 'ca_private_key_pem', 'jwks_ca_bundle_pem'))
    if (hashlib.sha256(ca).hexdigest() != plan['issuer']['issuer_certificate_sha256']
        or hashlib.sha256(bundle).hexdigest() != plan['issuer']['ca_public_sha256']):
        raise ValueError()
    expected_ca = issuer['ca_secret']
    def check_ca():
        if parent.load_issuer(session, 'gis_material') != issuer: raise ValueError()
        actual = kube.get('secret', 'voice-media-ca')
        if (expected_ca['metadata'].get('name') != 'voice-media-ca'
            or expected_ca['metadata'].get('namespace') != 'voice-staging'
            or expected_ca['metadata'].get('uid') != plan['issuer']['ca_secret_uid']
            or actual['metadata'].get('uid') != expected_ca['metadata'].get('uid')
            or actual['metadata'].get('resourceVersion') != expected_ca['metadata'].get('resourceVersion')
            or _secret_semantic(actual) != _secret_semantic(expected_ca)
            or base64.b64decode(actual['data']['jwks-ca.crt'], validate=True) != bundle):
            raise ValueError()
    if type(desired['data']) is not dict or set(desired['data']) != {'tls.crt', 'tls.key'}:
        raise ValueError()
    check_ca()
    _verify_tls_material(base64.b64decode(desired['data']['tls.crt'], validate=True),
        base64.b64decode(desired['data']['tls.key'], validate=True), ca, ca_key, bundle)
    check_ca()
    return check_ca


def verify_effect(session, effect_id, observation):
    """Verify an app CAS by a fresh read; other effects remain fail-closed."""
    import staging_prerequisites as parent
    import stage_runtime
    try:
        session.check()
        plan = parent.require_child(session, 'gis_material')
        validate_plan(plan)
        if effect_id in plan['preserved']:
            preserved = plan['preserved'][effect_id]
            qualified = parent.load_preserved(session, 'gis_material', effect_id)
            if (type(qualified) is not dict or set(qualified) != {'authority', 'semantic', 'uid'}
                or qualified['authority'] != preserved['authority'] or qualified['uid'] != preserved['observed']['uid']):
                raise ValueError()
            live = stage_runtime.Kube().get('secret', _EFFECTS[effect_id])
            semantic = _secret_semantic(live)
            meta = live['metadata']
            expected = {'kind': 'Secret', 'name': _EFFECTS[effect_id], 'namespace': 'voice-staging',
                'uid': meta['uid'], 'resourceVersion': meta['resourceVersion'],
                'semantic_sha256': hashlib.sha256(_canonical(semantic)).hexdigest()}
            if (semantic != qualified['semantic'] or expected != preserved['observed'] or observation != expected):
                raise ValueError()
            if effect_id == 'gis-tls':
                check_ca = _verify_issuer_tls(parent, session, plan, semantic, stage_runtime.Kube())
                fresh = stage_runtime.Kube().get('secret', _EFFECTS[effect_id])
                if (_secret_semantic(fresh) != semantic or fresh['metadata']['uid'] != expected['uid']
                    or fresh['metadata']['resourceVersion'] != expected['resourceVersion']):
                    raise ValueError()
                check_ca()
            if parent.require_child(session, 'gis_material') != plan: raise ValueError()
            session.check()
            return expected
        if effect_id not in ('gis-app-credentials', 'gis-workload', 'gis-signer', 'gis-tls'): raise ValueError()
        effect = next(row for row in plan['effects'] if row['id'] == effect_id)
        raw = parent.load_payload(session, 'gis_material', effect_id)
        if type(raw) is not bytes or len(raw) > 2 * 1024 * 1024 or hashlib.sha256(raw).hexdigest() != effect['payload_sha256']:
            raise ValueError()
        def pairs(items):
            result = {}
            for key, value in items:
                if key in result: raise ValueError()
                result[key] = value
            return result
        payload = json.loads(raw, object_pairs_hook=pairs)
        if effect_id in ('gis-workload', 'gis-signer', 'gis-tls'):
            desired = _secret_semantic(payload)
            if (desired != payload or desired['metadata'].get('name') != effect['name']
                or desired['metadata'].get('namespace') != 'voice-staging'
                or desired['metadata'].get('annotations', {}).get('voice.poryadok.io/prerequisites-operation') != plan['operation']
                or desired['type'] != ('kubernetes.io/tls' if effect_id == 'gis-tls' else 'Opaque')
                or desired.get('immutable', False) is not False):
                raise ValueError()
            if effect_id == 'gis-workload':
                keys = {'GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64', 'GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64'}
                if type(desired['data']) is not dict or set(desired['data']) != keys: raise ValueError()
                values = [base64.b64decode(base64.b64decode(desired['data'][key], validate=True), validate=True) for key in sorted(keys)]
                if any(len(value) != 32 for value in values) or values[0] == values[1]: raise ValueError()
            elif effect_id == 'gis-signer':
                if type(desired['data']) is not dict or set(desired['data']) != {'current.pem', 'next.pem', 'active-kid'}:
                    raise ValueError()
                values = {key: base64.b64decode(value, validate=True) for key, value in desired['data'].items()}
                if values['active-kid'] != b'current': raise ValueError()
                publics = []
                for key in ('current.pem', 'next.pem'):
                    if not values[key].startswith(b'-----BEGIN PRIVATE KEY-----\n') or len(values[key]) > 16384:
                        raise ValueError()
                    checked = subprocess.run([_OPENSSL, 'rsa', '-check', '-text', '-noout'], input=values[key],
                        stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=True, timeout=15, env=_OPENSSL_ENV)
                    if b'Private-Key: (3072 bit' not in checked.stdout: raise ValueError()
                    public = subprocess.run([_OPENSSL, 'pkey', '-pubout'], input=values[key],
                        stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=True, timeout=15, env=_OPENSSL_ENV).stdout
                    publics.append(public)
                if not publics[0] or publics[0] == publics[1]: raise ValueError()
            else:
                check_ca = _verify_issuer_tls(parent, session, plan, desired, stage_runtime.Kube())
            desired_hash = hashlib.sha256(_canonical(desired)).hexdigest()
            live = stage_runtime.Kube().get('secret', effect['name'])
            if effect_id == 'gis-tls': check_ca()
            meta = live['metadata']
            if (not _UUID.fullmatch(meta.get('uid', '')) or not _RV.fullmatch(meta.get('resourceVersion', ''))
                or desired_hash != effect['desired_sha256'] or _secret_semantic(live) != desired):
                raise ValueError()
            expected = {'kind': 'Secret', 'name': effect['name'], 'namespace': effect['namespace'],
                'uid': meta['uid'], 'resourceVersion': meta['resourceVersion'], 'semantic_sha256': desired_hash}
            if observation != expected: raise ValueError()
            if parent.require_child(session, 'gis_material') != plan: raise ValueError()
            session.check()
            return expected
        if type(payload) is not dict or set(payload) != {'before', 'after_data'}: raise ValueError()
        before, after = payload['before'], payload['after_data']
        original = inspect_app_secret(before)
        if original['credentials'] != 'absent' or {key: original[key] for key in ('uid', 'resourceVersion', 'data_sha256')} != effect['before']:
            raise ValueError()
        selected = {'GAME_INTEGRATION_DB_PASSWORD', 'GAME_INTEGRATION_DATABASE_URL'}
        if type(after) is not dict or set(after) != set(before['data']) | selected or any(after[key] != value for key, value in before['data'].items()):
            raise ValueError()
        if not re.fullmatch(rb'[A-Za-z0-9_-]{43}', base64.b64decode(after['GAME_INTEGRATION_DB_PASSWORD'], validate=True)):
            raise ValueError()
        desired = _secret_semantic(before)
        desired['data'] = after
        desired_hash = hashlib.sha256(_canonical(desired)).hexdigest()
        if desired_hash != effect['desired_sha256']: raise ValueError()
        live = stage_runtime.Kube().get('secret', effect['name'])
        current = inspect_app_secret(live)
        expected_observation = {'kind': 'Secret', 'name': effect['name'], 'namespace': effect['namespace'],
            'uid': current['uid'], 'resourceVersion': current['resourceVersion'], 'semantic_sha256': desired_hash}
        if (current['credentials'] != 'present' or current['uid'] != original['uid']
            or current['resourceVersion'] == original['resourceVersion']
            or _secret_semantic(live) != desired or observation != expected_observation):
            raise ValueError()
        if parent.require_child(session, 'gis_material') != plan: raise ValueError()
        session.check()
        return expected_observation
    except (KeyError, StopIteration, TypeError, ValueError, UnicodeError, binascii.Error, OSError,
            subprocess.SubprocessError, MaterialError):
        raise MaterialError('staging_gis_effect_invalid') from None


def inspect_app_secret(row):
    """Observe the complete shared Secret without authorizing its reuse."""
    try:
        if not isinstance(row, dict) or row.get('apiVersion') != 'v1' or row.get('kind') != 'Secret' or row.get('type', 'Opaque') != 'Opaque':
            raise ValueError()
        metadata, data = row['metadata'], row['data']
        if (not isinstance(metadata, dict) or metadata.get('name') != 'voice-app-secrets'
            or metadata.get('namespace') != 'voice-staging' or metadata.get('deletionTimestamp')
            or not re.fullmatch(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}', metadata.get('uid', ''))
            or not re.fullmatch(r'[0-9]{1,20}', metadata.get('resourceVersion', ''))
            or not isinstance(data, dict) or not data):
            raise ValueError()
        encoded = json.dumps(data, sort_keys=True, separators=(',', ':')).encode()
        if len(encoded) > 1 << 20:
            raise ValueError()
        if any(not isinstance(key, str) or not isinstance(value, str) for key, value in data.items()):
            raise ValueError()
        decoded = {key: base64.b64decode(value, validate=True) for key, value in data.items()}
        if not decoded.get('POSTGRES_PASSWORD'):
            raise ValueError()
        keys = {'GAME_INTEGRATION_DB_PASSWORD', 'GAME_INTEGRATION_DATABASE_URL'}
        present = keys & decoded.keys()
        if present and present != keys:
            raise ValueError()
        if not present:
            if row.get('immutable') is True:
                raise ValueError()
            disposition = 'absent'
        else:
            password = decoded['GAME_INTEGRATION_DB_PASSWORD']
            if not re.fullmatch(rb'[A-Za-z0-9_-]{1,256}', password) or decoded['GAME_INTEGRATION_DATABASE_URL'] != (
                b'postgres://gameintegration_runtime:' + password +
                b'@voice-postgres:5432/game_integration_db?sslmode=disable'):
                raise ValueError()
            disposition = 'present'
        return {'uid': metadata['uid'], 'resourceVersion': metadata['resourceVersion'],
                'data_sha256': hashlib.sha256(encoded).hexdigest(), 'credentials': disposition}
    except (KeyError, TypeError, ValueError, binascii.Error, UnicodeError):
        raise MaterialError('staging_gis_app_secret_invalid') from None


def _signer():
    try:
        result = subprocess.run(
            [_OPENSSL, 'genpkey', '-algorithm', 'RSA', '-pkeyopt', 'rsa_keygen_bits:3072'],
            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=True, timeout=30, env=_OPENSSL_ENV)
        value = result.stdout
        if not isinstance(value, bytes) or not 1 <= len(value) <= 16384 or not value.startswith(b'-----BEGIN PRIVATE KEY-----\n'):
            raise ValueError()
        subprocess.run([_OPENSSL, 'rsa', '-check', '-noout'], input=value,
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True, timeout=15, env=_OPENSSL_ENV)
        return value
    except (OSError, subprocess.SubprocessError, ValueError):
        raise MaterialError('staging_gis_signer_generation_failed') from None


def generate_runtime_material():
    """Generate only GIS credentials in memory; caller must protect their custody."""
    try:
        password = secrets.token_urlsafe(32).encode('ascii')
        first, second = secrets.token_bytes(32), secrets.token_bytes(32)
        if (not re.fullmatch(rb'[A-Za-z0-9_-]{43}', password)
            or type(first) is not bytes or type(second) is not bytes
            or len(first) != 32 or len(second) != 32 or first == second):
            raise ValueError()
    except (ValueError, TypeError, UnicodeError, AttributeError):
        raise MaterialError('staging_gis_entropy_invalid') from None
    current, following = _signer(), _signer()
    if current == following:
        raise MaterialError('staging_gis_signer_generation_failed')
    return {
        'GAME_INTEGRATION_DB_PASSWORD': password,
        'GAME_INTEGRATION_DATABASE_URL': (
            b'postgres://gameintegration_runtime:' + password +
            b'@voice-postgres:5432/game_integration_db?sslmode=disable'),
        'voice-gameintegration-principal-signing': {
            'current.pem': current, 'next.pem': following, 'active-kid': b'current'},
        'voice-gameintegration-workload-credentials': {
            'GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64': base64.b64encode(first),
            'GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64': base64.b64encode(second)},
    }


def issue_jwks_leaf(ca_certificate, ca_key, jwks_bundle):
    """Issue a fixed GIS leaf from supplied material; this grants no CA authority."""
    try:
        if any(type(value) is not bytes or not 0 < len(value) <= 256 * 1024
               for value in (ca_certificate, ca_key, jwks_bundle)):
            raise ValueError()
        private_key = _signer()
        serial = secrets.token_hex(16)
        if not re.fullmatch(r'[a-f0-9]{32}', serial) or int(serial, 16) == 0:
            raise ValueError()
        extension = (b'basicConstraints=critical,CA:FALSE\n'
                     b'keyUsage=critical,digitalSignature,keyEncipherment\n'
                     b'extendedKeyUsage=serverAuth\n'
                     b'subjectAltName=DNS:voice-gameintegration\n')
        with contextlib.ExitStack() as stack:
            def memory(value):
                fd = os.memfd_create('voice-gis-private', os.MFD_CLOEXEC)
                stack.callback(os.close, fd)
                remaining = memoryview(value)
                while remaining:
                    count = os.write(fd, remaining)
                    if count <= 0: raise OSError('short private material write')
                    remaining = remaining[count:]
                os.lseek(fd, 0, os.SEEK_SET)
                return fd
            issuer_fd, issuer_key_fd, trust_fd, key_fd, ext_fd = map(
                memory, (ca_certificate, ca_key, jwks_bundle, private_key, extension))
            def run(args, fds, body=None):
                for fd in fds: os.lseek(fd, 0, os.SEEK_SET)
                result = subprocess.run(args, input=body, stdout=subprocess.PIPE,
                    stderr=subprocess.DEVNULL, pass_fds=tuple(fds), check=True, timeout=30, env=_OPENSSL_ENV)
                if not isinstance(result.stdout, bytes) or len(result.stdout) > 256 * 1024:
                    raise ValueError()
                return result.stdout
            cert_public = run([_OPENSSL, 'x509', '-in', f'/proc/self/fd/{issuer_fd}', '-pubkey', '-noout'], [issuer_fd])
            key_public = run([_OPENSSL, 'pkey', '-in', f'/proc/self/fd/{issuer_key_fd}', '-pubout'], [issuer_key_fd])
            if not cert_public or cert_public != key_public: raise ValueError()
            run([_OPENSSL, 'x509', '-in', f'/proc/self/fd/{issuer_fd}',
                 '-checkend', '7776000', '-noout'], [issuer_fd])
            csr = run([_OPENSSL, 'req', '-new', '-key', f'/proc/self/fd/{key_fd}',
                       '-subj', '/CN=voice-gameintegration'], [key_fd])
            certificate = run([_OPENSSL, 'x509', '-req', '-CA', f'/proc/self/fd/{issuer_fd}',
                '-CAkey', f'/proc/self/fd/{issuer_key_fd}', '-set_serial', '0x' + serial,
                '-days', '90', '-extfile', f'/proc/self/fd/{ext_fd}'],
                [issuer_fd, issuer_key_fd, ext_fd], body=csr)
            leaf_fd = memory(certificate)
            def expiry(fd):
                value = run([_OPENSSL, 'x509', '-in', f'/proc/self/fd/{fd}', '-enddate', '-noout'], [fd])
                if not value.startswith(b'notAfter='): raise ValueError()
                return datetime.datetime.strptime(value[len(b'notAfter='):].decode('ascii').strip(),
                                                  '%b %d %H:%M:%S %Y %Z')
            if expiry(leaf_fd) > expiry(issuer_fd): raise ValueError()
            run([_OPENSSL, 'verify', '-trusted', f'/proc/self/fd/{trust_fd}',
                 '-verify_hostname', 'voice-gameintegration', '-purpose', 'sslserver',
                 f'/proc/self/fd/{leaf_fd}'], [trust_fd, leaf_fd])
            leaf_public = run([_OPENSSL, 'x509', '-in', f'/proc/self/fd/{leaf_fd}', '-pubkey', '-noout'], [leaf_fd])
            generated_public = run([_OPENSSL, 'pkey', '-in', f'/proc/self/fd/{key_fd}', '-pubout'], [key_fd])
            if not leaf_public or leaf_public != generated_public: raise ValueError()
            return {'tls.crt': certificate, 'tls.key': private_key}
    except (AttributeError, OSError, subprocess.SubprocessError, TypeError, ValueError, MaterialError):
        raise MaterialError('staging_gis_tls_generation_failed') from None


def build_create_payloads(operation, app_before, ca_certificate, ca_key, jwks_bundle):
    """Build private bytes only; the parent must establish authority and custody."""
    if not isinstance(operation, str) or not re.fullmatch(r'[a-f0-9]{12}', operation):
        raise MaterialError('staging_gis_payload_invalid')
    if inspect_app_secret(app_before)['credentials'] != 'absent':
        raise MaterialError('staging_gis_payload_invalid')
    materials = generate_runtime_material()
    tls = issue_jwks_leaf(ca_certificate, ca_key, jwks_bundle)
    def secret(effect_id, values, secret_type):
        return _canonical({'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {
            'name': _EFFECTS[effect_id], 'namespace': 'voice-staging',
            'annotations': {'voice.poryadok.io/prerequisites-operation': operation}},
            'type': secret_type, 'data': {name: base64.b64encode(value).decode('ascii')
                                          for name, value in values.items()}})
    after = dict(app_before['data'])
    for key in ('GAME_INTEGRATION_DB_PASSWORD', 'GAME_INTEGRATION_DATABASE_URL'):
        after[key] = base64.b64encode(materials[key]).decode('ascii')
    return {'gis-signer': secret('gis-signer', materials[_EFFECTS['gis-signer']], 'Opaque'),
            'gis-workload': secret('gis-workload', materials[_EFFECTS['gis-workload']], 'Opaque'),
            'gis-tls': secret('gis-tls', tls, 'kubernetes.io/tls'),
            'gis-app-credentials': _canonical({'before': app_before, 'after_data': after})}


def source_contract(approved, source):
    """Decode captured source; the protected coordinator qualifies its custody."""
    import gis_prerequisite
    import yaml
    try:
        if (type(approved) is not dict or type(approved.get('source_files')) is not dict
            or not isinstance(approved.get('source_sha'), str) or not re.fullmatch(r'[a-f0-9]{40}', approved['source_sha'])):
            raise ValueError()
        root = Path(source); files = approved['source_files']
        fixed = {'deploy/staging/services.yaml', 'scripts/dev/migrations.py',
            'docker/postgres/provision-gameintegration-runtime-role.sql',
            'docker/postgres/verify-gameintegration-runtime-role.sql',
            'deploy/templates/provision-gameintegration-runtime-role-job.yaml',
            'deploy/templates/migrate-game-integration-db-job.yaml',
            'deploy/templates/verify-game-integration-db-job.yaml'}
        prefix = 'src/backend/migrations/game_integration_db/'
        migrations = {name for name in files if isinstance(name, str) and name.startswith(prefix)}
        if not migrations or not fixed.issubset(files): raise ValueError()
        directory = root / prefix
        if directory.is_symlink() or not directory.is_dir(): raise ValueError()
        actual = {prefix + entry.name for entry in directory.iterdir()}
        if actual != migrations: raise ValueError()
        for relative in fixed | migrations:
            path = root
            for component in Path(relative).parts:
                path = path / component
                if path.is_symlink(): raise ValueError()
            if not path.is_file() or hashlib.sha256(path.read_bytes()).hexdigest() != files[relative]:
                raise ValueError()
        inputs = gis_prerequisite.source_inputs(root, files)
        versions = [int(row['name'].split('_', 1)[0]) for row in inputs['migrations'] if row['name'].endswith('.up.sql')]
        verifier_hash = files['docker/postgres/verify-gameintegration-runtime-role.sql']
        if (not versions or verifier_hash not in _VERIFIER_REVISIONS
            or max(versions) != _VERIFIER_REVISIONS[verifier_hash]
            or max(versions) != gis_prerequisite.validate_source_revision(inputs)):
            raise ValueError()
        raw = (root/'deploy/staging/services.yaml').read_bytes()
        if len(raw) > 2 << 20: raise ValueError()
        class UniqueLoader(yaml.SafeLoader):
            pass
        def mapping(loader, node, deep=False):
            result = {}
            for key_node, value_node in node.value:
                key = loader.construct_object(key_node, deep=deep)
                if not isinstance(key, str) or key in result: raise ValueError()
                result[key] = loader.construct_object(value_node, deep=deep)
            return result
        UniqueLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, mapping)
        if any(isinstance(token, (yaml.tokens.AliasToken, yaml.tokens.AnchorToken)) for token in yaml.scan(raw)):
            raise ValueError()
        documents = list(yaml.load_all(raw, Loader=UniqueLoader))
        identities = [('Deployment', 'voice-gameintegration'), ('Deployment', 'voice-notification'),
                      ('Deployment', 'voice-realtime'), ('Deployment', 'voice-role'), ('Service', 'voice-gameintegration')]
        selected = {}
        for kind, name in identities:
            rows = [row for row in documents if isinstance(row, dict) and row.get('kind') == kind
                    and row.get('metadata', {}).get('name') == name]
            if len(rows) != 1 or rows[0].get('metadata', {}).get('namespace') != 'voice-staging': raise ValueError()
            selected[kind+'/'+name] = rows[0]
        def container(name):
            spec = selected['Deployment/voice-'+name]['spec']['template']['spec']
            rows = [row for row in spec['containers'] if row.get('name') == name]
            if len(rows) != 1: raise ValueError()
            env = rows[0].get('env', [])
            if len({row['name'] for row in env}) != len(env): raise ValueError()
            return spec, rows[0], {row['name']: row for row in env}
        gis_spec, gis_container, gis_env = container('gameintegration')
        deployment = selected['Deployment/voice-gameintegration']
        if (deployment['spec'].get('selector') != {'matchLabels':{'app':'voice-gameintegration'}}
            or deployment['spec']['template'].get('metadata',{}).get('labels',{}).get('app') != 'voice-gameintegration'):
            raise ValueError()
        def ref(env, key, secret, secret_key):
            if env.get(key) != {'name': key, 'valueFrom': {'secretKeyRef': {'name': secret, 'key': secret_key}}}:
                raise ValueError()
        ref(gis_env, 'DATABASE_URL', 'voice-app-secrets', 'GAME_INTEGRATION_DATABASE_URL')
        for name in ('notification', 'realtime'):
            key = 'GAME_INTEGRATION_'+name.upper()+'_WORKLOAD_KEY_B64'
            ref(gis_env, key, 'voice-gameintegration-workload-credentials', key)
            _, _, caller = container(name)
            ref(caller, key, 'voice-gameintegration-workload-credentials', key)
            if caller.get('GAME_INTEGRATION_CONSENT_URL') != {'name':'GAME_INTEGRATION_CONSENT_URL','value':'http://voice-gameintegration:8080'}:
                raise ValueError()
        for name, value in {'GAME_INTEGRATION_PRINCIPAL_PRIVATE_KEY_FILE':'/etc/voice/principal/keys/current.pem',
            'GAME_INTEGRATION_PRINCIPAL_NEXT_PRIVATE_KEY_FILE':'/etc/voice/principal/keys/next.pem',
            'GAME_INTEGRATION_PRINCIPAL_KID':'current', 'GAME_INTEGRATION_PRINCIPAL_NEXT_KID':'next',
            'GAME_INTEGRATION_PRINCIPAL_JWKS_LISTEN_ADDR':':8443',
            'GAME_INTEGRATION_PRINCIPAL_JWKS_TLS_CERT_FILE':'/etc/voice/principal/tls/tls.crt',
            'GAME_INTEGRATION_PRINCIPAL_JWKS_TLS_KEY_FILE':'/etc/voice/principal/tls/tls.key'}.items():
            if gis_env.get(name) != {'name':name,'value':value}: raise ValueError()
        def named(rows, *, mounts=False):
            values = {row['name']: row for row in rows}
            if len(values) != len(rows): raise ValueError()
            if mounts and len({row['mountPath'] for row in rows}) != len(rows): raise ValueError()
            return values
        volumes = named(gis_spec.get('volumes', []))
        mounts = named(gis_container.get('volumeMounts', []), mounts=True)
        for item, secret, directory_name, keys in (
                ('principal-signing','voice-gameintegration-principal-signing','/etc/voice/principal/keys',('current.pem','next.pem')),
                ('principal-tls','voice-gameintegration-jwks-tls','/etc/voice/principal/tls',('tls.crt','tls.key'))):
            if mounts.get(item) != {'name':item,'mountPath':directory_name,'readOnly':True}: raise ValueError()
            volume = volumes.get(item, {}).get('secret', {})
            if volume.get('secretName') != secret or volume.get('items') != [{'key':key,'path':key} for key in keys]: raise ValueError()
        role_spec, role_container, role_env = container('role')
        def json_pairs(items):
            result = {}
            for key, value in items:
                if key in result: raise ValueError()
                result[key] = value
            return result
        if json.loads(role_env['S2S_JWKS_URLS_JSON']['value'], object_pairs_hook=json_pairs).get('gameintegration') != 'https://voice-gameintegration:8443/internal/v1/principal/jwks.json': raise ValueError()
        if role_env.get('S2S_JWKS_CA_FILE') != {'name':'S2S_JWKS_CA_FILE','value':'/etc/voice/media/ca/jwks-ca.crt'}: raise ValueError()
        role_volumes = named(role_spec.get('volumes', []))
        role_mounts = named(role_container.get('volumeMounts', []), mounts=True)
        if (role_mounts.get('media-ca') != {'name':'media-ca','mountPath':'/etc/voice/media/ca','readOnly':True}
            or role_volumes.get('media-ca',{}).get('secret',{}).get('secretName') != 'voice-media-ca'
            or {'key':'jwks-ca.crt','path':'jwks-ca.crt'} not in role_volumes['media-ca']['secret'].get('items',[])):
            raise ValueError()
        service = selected['Service/voice-gameintegration']['spec']
        ports = service.get('ports', [])
        if (service.get('selector') != {'app':'voice-gameintegration'} or len(ports) != 2
            or {row.get('name'):row.get('port') for row in ports} != {'http':8080,'principal-jwks':8443}
            or any(row.get('targetPort',row.get('port')) != row.get('port') or row.get('protocol','TCP') != 'TCP' for row in ports)):
            raise ValueError()
        if hashlib.sha256((root/'deploy/staging/services.yaml').read_bytes()).hexdigest() != files['deploy/staging/services.yaml']:
            raise ValueError()
        contract = {'schema':'voice-staging-gis-material-source-contract-v1','source_sha':approved['source_sha'],
            'services_blob_sha256':files['deploy/staging/services.yaml'], 'gis_prerequisite_inputs':inputs,'material_wiring':selected}
        return contract, hashlib.sha256(_canonical(contract)).hexdigest()
    except (AttributeError, KeyError, TypeError, ValueError, OSError, UnicodeError, yaml.YAMLError, gis_prerequisite.Blocked):
        raise MaterialError('staging_gis_source_contract_invalid') from None


def build_plan(operation, namespace_uid, source_capture_sha256, source_contract_sha256,
               issuer, before, preserved, payloads):
    """Build closed private descriptors; the coordinator qualifies every input."""
    try:
        if type(before) is not dict or type(preserved) is not dict or type(payloads) is not dict:
            raise ValueError()
        if set(before) != set(payloads) or set(payloads) & set(preserved) or set(payloads) | set(preserved) != set(_EFFECTS):
            raise ValueError()
        def pairs(items):
            result = {}
            for key, value in items:
                if key in result: raise ValueError()
                result[key] = value
            return result
        effects = []
        for effect_id, name in _EFFECTS.items():
            if effect_id not in payloads: continue
            raw = payloads[effect_id]
            if type(raw) is not bytes or len(raw) > 2 * 1024 * 1024: raise ValueError()
            obj = json.loads(raw, object_pairs_hook=pairs)
            if effect_id == 'gis-app-credentials':
                if type(obj) is not dict or set(obj) != {'before', 'after_data'}: raise ValueError()
                previous = inspect_app_secret(obj['before'])
                if (previous['credentials'] != 'absent' or
                    {key: previous[key] for key in ('uid', 'resourceVersion', 'data_sha256')} != before[effect_id]):
                    raise ValueError()
                keys = {'GAME_INTEGRATION_DB_PASSWORD', 'GAME_INTEGRATION_DATABASE_URL'}
                after = obj['after_data']
                if (type(after) is not dict or set(after) != set(obj['before']['data']) | keys
                    or any(after[key] != value for key, value in obj['before']['data'].items())):
                    raise ValueError()
                desired = _secret_semantic(obj['before']); desired['data'] = after
            else:
                desired = _secret_semantic(obj)
                if (desired != obj or desired['metadata'].get('name') != name
                    or desired['metadata'].get('namespace') != 'voice-staging'
                    or desired['metadata'].get('annotations', {}).get('voice.poryadok.io/prerequisites-operation') != operation
                    or desired['type'] != ('kubernetes.io/tls' if effect_id == 'gis-tls' else 'Opaque')):
                    raise ValueError()
            effects.append({'id': effect_id, 'kind': 'Secret', 'name': name, 'namespace': 'voice-staging',
                'before': copy.deepcopy(before[effect_id]), 'desired_sha256': hashlib.sha256(_canonical(desired)).hexdigest(),
                'payload_sha256': hashlib.sha256(raw).hexdigest()})
        plan = {'schema': 'voice-staging-gis-material-plan-v1', 'operation': operation,
            'namespace_uid': namespace_uid, 'source_capture_sha256': source_capture_sha256,
            'source_contract_sha256': source_contract_sha256, 'issuer': copy.deepcopy(issuer),
            'effects': effects, 'preserved': copy.deepcopy(preserved)}
        validate_plan(plan)
        return plan, dict(payloads)
    except (KeyError, TypeError, ValueError, UnicodeError, MaterialError):
        raise MaterialError('staging_gis_plan_build_failed') from None


def preflight_payloads(session):
    """Validate protected inputs once before the first GIS effect, never after a create."""
    import staging_prerequisites as parent
    import stage_runtime
    import guard
    try:
        session.check()
        plan = parent.require_child(session, 'gis_material')
        validate_plan(plan)
        authority = parent.load_authority(session)
        if (type(authority) is not dict or set(authority) != {'approved','execution','source'}
            or getattr(session, 'operation', None) != plan['operation']
            or authority['source'] != str(guard.ROOT/'installed'/'sources'/plan['operation'])):
            raise ValueError()
        _, source_sha = source_contract(authority['approved'], authority['source'])
        if source_sha != plan['source_contract_sha256']: raise ValueError()
        kube = stage_runtime.Kube()
        namespace = kube.get('namespace', 'voice-staging')
        if (namespace['metadata'].get('uid') != plan['namespace_uid']
            or namespace['metadata'].get('deletionTimestamp')): raise ValueError()
        payloads = {effect['id']: parent.load_payload(session, 'gis_material', effect['id'])
                    for effect in plan['effects']}
        rebuilt, _ = build_plan(plan['operation'], plan['namespace_uid'], plan['source_capture_sha256'],
            plan['source_contract_sha256'], plan['issuer'], {row['id']: row['before'] for row in plan['effects']},
            plan['preserved'], payloads)
        if rebuilt != plan: raise ValueError()
        for effect_id, preserved in plan['preserved'].items():
            verify_effect(session, effect_id, preserved['observed'])
        for effect in plan['effects']:
            effect_id = effect['id']
            obj = json.loads(payloads[effect_id])
            if effect_id == 'gis-app-credentials':
                live = kube.get('secret', effect['name'])
                old = obj['before']
                observed = inspect_app_secret(live)
                if (observed['credentials'] != 'absent'
                    or {key: observed[key] for key in ('uid', 'resourceVersion', 'data_sha256')} != effect['before']
                    or _secret_semantic(live) != _secret_semantic(old)):
                    raise ValueError()
                desired = copy.deepcopy(old); desired['data'] = obj['after_data']
                if inspect_app_secret(desired)['credentials'] != 'present': raise ValueError()
                if not re.fullmatch(rb'[A-Za-z0-9_-]{43}', base64.b64decode(obj['after_data']['GAME_INTEGRATION_DB_PASSWORD'], validate=True)):
                    raise ValueError()
                continue
            path = '/api/v1/namespaces/voice-staging/secrets?fieldSelector=metadata.name%3D' + effect['name'] + '&limit=2'
            listing = kube.run(['get', '--raw', path])
            metadata = listing['metadata']
            if (listing.get('apiVersion') != 'v1' or listing.get('kind') != 'SecretList' or listing.get('items') != []
                or metadata.get('continue') or not isinstance(metadata.get('resourceVersion'), str)
                or not _RV.fullmatch(metadata['resourceVersion'])):
                raise ValueError()
            data = obj['data']
            if obj.get('immutable', False) is not False: raise ValueError()
            if effect_id == 'gis-workload':
                keys = {'GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64', 'GAME_INTEGRATION_REALTIME_WORKLOAD_KEY_B64'}
                if type(data) is not dict or set(data) != keys: raise ValueError()
                values = [base64.b64decode(base64.b64decode(data[key], validate=True), validate=True) for key in sorted(keys)]
                if any(len(value) != 32 for value in values) or values[0] == values[1]: raise ValueError()
            elif effect_id == 'gis-signer':
                if type(data) is not dict or set(data) != {'current.pem', 'next.pem', 'active-kid'}: raise ValueError()
                values = {key: base64.b64decode(value, validate=True) for key, value in data.items()}
                if values['active-kid'] != b'current': raise ValueError()
                publics = []
                for key in ('current.pem', 'next.pem'):
                    if not values[key].startswith(b'-----BEGIN PRIVATE KEY-----\n') or len(values[key]) > 16384: raise ValueError()
                    checked = subprocess.run([_OPENSSL, 'rsa', '-check', '-text', '-noout'], input=values[key],
                        stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=True, timeout=15, env=_OPENSSL_ENV)
                    if b'Private-Key: (3072 bit' not in checked.stdout: raise ValueError()
                    publics.append(subprocess.run([_OPENSSL, 'pkey', '-pubout'], input=values[key],
                        stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=True, timeout=15, env=_OPENSSL_ENV).stdout)
                if not publics[0] or publics[0] == publics[1]: raise ValueError()
            elif effect_id == 'gis-tls':
                _verify_issuer_tls(parent, session, plan, obj, kube)()
        if (parent.require_child(session, 'gis_material') != plan or parent.load_authority(session) != authority
            or source_contract(authority['approved'], authority['source'])[1] != source_sha): raise ValueError()
        session.check()
        return True
    except (KeyError, TypeError, ValueError, UnicodeError, binascii.Error, OSError,
            subprocess.SubprocessError, MaterialError):
        raise MaterialError('staging_gis_preflight_invalid') from None
