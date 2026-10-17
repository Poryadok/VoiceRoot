#!/usr/bin/env python3
"""Prepare and apply the constrained DEV-only media JWKS PKI recovery."""
import argparse
import base64
import copy
from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
import sys

NAMESPACE = 'voice-dev'
VOLUME = 'voice-dev-media-pki'
ROLE = 'voice-role'
CA_SECRET = 'voice-media-ca'
MATCHMAKING_CA_SECRET = 'voice-matchmaking-principal-jwks-ca'
VOICE_SECRET = 'voice-voice-media-jwks-tls'
GIS_SECRET = 'voice-gameintegration-jwks-tls'
MATCHMAKING_SECRET = 'voice-matchmaking-principal-jwks-tls'
NAMESPACE_LABELS = {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'}
DEFAULT_STATE = Path('/home/pmd/.local/share/voice-dev-media-pki')
LEAF_SECRETS = {VOICE_SECRET, GIS_SECRET, MATCHMAKING_SECRET}
FORWARD_PHASES = {'trust-added', 'role-trust-switched', 'leaf-cutover-started',
                  'voice-leaf-rotated', 'matchmaking-ca-create-intent',
                  'matchmaking-ca-created', 'gis-create-intent', 'gis-created',
                  'matchmaking-tls-create-intent', 'matchmaking-tls-created',
                  'leaf-cutover-complete'}


@contextmanager
def operator_lock(path):
    if sys.platform != 'linux':
        raise ValueError('operator actions are restricted to the designated Linux host')
    import fcntl
    fd = os.open(path, os.O_CREAT | os.O_RDWR | getattr(os, 'O_NOFOLLOW', 0), 0o600)
    lock = os.fdopen(fd, 'a+b')
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        yield
    finally:
        lock.close()


def run(args, *, input_bytes=None):
    result = subprocess.run([str(arg) for arg in args], input=input_bytes,
                            capture_output=True, timeout=180)
    if result.returncode:
        raise ValueError('DEV media PKI operation failed: ' + Path(args[0]).name)
    return result.stdout


def image_id(value):
    if not re.fullmatch(r'sha256:[0-9a-f]{64}', value):
        raise ValueError('issuer image must be an immutable local image ID')
    return value


def issuer(image, action):
    return run(['docker', 'run', '--rm', '--network', 'none', '--read-only',
                '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges',
                '--user', '10001:10001',
                '--mount', f'type=volume,source={VOLUME},target=/state',
                image_id(image), action])


def kubectl(*args, input_bytes=None):
    return run(['kubectl', '--namespace', NAMESPACE, *args], input_bytes=input_bytes)


def metadata(kind, name):
    raw = kubectl('get', kind, name, '-o', 'go-template={{json .metadata}}')
    value = json.loads(raw)
    if value.get('name') != name or value.get('namespace') != NAMESPACE or not value.get('uid') or not value.get('resourceVersion'):
        raise ValueError('unexpected DEV resource identity')
    return {key: value[key] for key in ('name', 'namespace', 'uid', 'resourceVersion', 'labels', 'annotations') if key in value}


def namespace_metadata(raw):
    value = json.loads(raw)
    if (value.get('name') != NAMESPACE or not value.get('uid') or not value.get('resourceVersion')
            or any(value.get('labels', {}).get(key) != label for key, label in NAMESPACE_LABELS.items())):
        raise ValueError('DEV namespace ownership marker required')
    return {key: value[key] for key in ('name', 'uid', 'resourceVersion', 'labels') if key in value}


def verify_namespace():
    return namespace_metadata(run(['kubectl', 'get', 'namespace', NAMESPACE,
                                  '-o', 'go-template={{json .metadata}}']))


def reject_existing_apply_journal(state_dir):
    path = state_dir / 'last-apply.json'
    if not path.exists() and not path.is_symlink():
        return
    if path.is_symlink() or not path.is_file():
        raise ValueError('existing apply journal is not a regular file')
    info = path.stat()
    if info.st_uid != os.geteuid() or (info.st_mode & 0o077):
        raise ValueError('existing apply journal ownership or mode is unsafe')
    receipt = json.loads(path.read_text(encoding='utf-8'))
    if receipt.get('namespace') != NAMESPACE or receipt.get('volume') != VOLUME:
        raise ValueError('existing journal is not for the fixed DEV operation')
    phase = receipt.get('phase', 'preflight-complete')
    if phase in FORWARD_PHASES:
        raise ValueError('recovery must continue forward; use resume with the pinned issuer image')
    raise ValueError('prior apply journal is preserved; rollback it before starting a new operation')


def safe_secret_list(raw):
    value = json.loads(raw)
    if value.get('kind') != 'List' or value.get('apiVersion') != 'v1':
        raise ValueError('issuer export is not a Secret list')
    items = {item.get('metadata', {}).get('name'): item for item in value.get('items', [])}
    expected = {CA_SECRET, MATCHMAKING_CA_SECRET, *LEAF_SECRETS}
    if len(value.get('items', [])) != len(expected) or set(items) != expected:
        raise ValueError('issuer exported an unexpected Secret set')
    if any(item.get('kind') != 'Secret' or item.get('metadata', {}).get('namespace') != NAMESPACE
           for item in items.values()):
        raise ValueError('issuer export escaped DEV')
    for name in (CA_SECRET, MATCHMAKING_CA_SECRET):
        if set(items[name].get('data', {})) != {'ca.crt'}:
            raise ValueError('issuer CA export contains unexpected keys')
        if items[name].get('type') != 'Opaque':
            raise ValueError('issuer CA Secret type is invalid')
    if items[CA_SECRET]['data']['ca.crt'] != items[MATCHMAKING_CA_SECRET]['data']['ca.crt']:
        raise ValueError('issuer public CA copies do not match')
    for name in LEAF_SECRETS:
        if set(items[name].get('data', {})) != {'tls.crt', 'tls.key'}:
            raise ValueError('issuer TLS export has unexpected keys')
        if items[name].get('type') != 'kubernetes.io/tls':
            raise ValueError('issuer TLS Secret type is invalid')
    return items


def replace_object(kind, metadata_value, body):
    item = copy.deepcopy(body)
    item.setdefault('metadata', {}).update(metadata_value)
    result = kubectl('replace', '-f', '-', '-o', 'go-template={{.metadata.resourceVersion}}',
                     input_bytes=json.dumps(item, separators=(',', ':')).encode())
    return result.decode('ascii').strip()


def write_receipt(path, receipt, temp_path):
    flags = os.O_WRONLY | os.O_CREAT | os.O_TRUNC | getattr(os, 'O_NOFOLLOW', 0)
    fd = os.open(temp_path, flags, 0o600)
    with os.fdopen(fd, 'w', encoding='utf-8') as output:
        output.write(json.dumps(receipt, sort_keys=True, indent=2) + '\n')
        output.flush()
        os.fsync(output.fileno())
    os.replace(temp_path, path)
    directory_fd = os.open(path.parent, os.O_RDONLY | getattr(os, 'O_DIRECTORY', 0))
    try:
        os.fsync(directory_fd)
    finally:
        os.close(directory_fd)


def create_operation_secret(name, item, receipt, resource, phase_prefix, path, temp_path):
    """Create one fixed DEV Secret with durable intent and returned identity pins."""
    item['metadata']['labels'] = dict(NAMESPACE_LABELS)
    item['metadata']['annotations'] = {'voice.io/media-pki-operation': receipt['operationId']}
    receipt['phase'] = phase_prefix + '-create-intent'
    write_receipt(path, receipt, temp_path)
    result = kubectl('create', '-f', '-', '-o', 'go-template={{.metadata.uid}} {{.metadata.resourceVersion}}',
                     input_bytes=json.dumps(item, separators=(',', ':')).encode()).decode('ascii').split()
    if len(result) != 2:
        raise ValueError(name + ' create returned incomplete identity pins')
    resource.update({'uid': result[0], 'afterResourceVersion': result[1]})
    receipt['phase'] = phase_prefix + '-created'
    write_receipt(path, receipt, temp_path)


def reconcile_operation_secret(name, item, pem_key, expected_fingerprint, receipt, resource,
                               phase_prefix, path, temp_path):
    """Adopt only this operation's exact public certificate or create its fixed Secret."""
    found = kubectl('get', 'secret', name, '--ignore-not-found', '-o', 'name').strip()
    if found:
        secret_meta = metadata('secret', name)
        if secret_meta.get('annotations', {}).get('voice.io/media-pki-operation') != receipt['operationId']:
            raise ValueError(name + ' does not belong to this operation')
        expected_keys = {'ca.crt'} if pem_key == 'ca.crt' else {'tls.crt', 'tls.key'}
        actual_keys = set(kubectl('get', 'secret', name,
                                  '-o', 'go-template={{range $key, $value := .data}}{{$key}} {{end}}').decode('ascii').split())
        expected_type = 'Opaque' if pem_key == 'ca.crt' else 'kubernetes.io/tls'
        actual_type = kubectl('get', 'secret', name, '-o', 'go-template={{.type}}').decode('ascii').strip()
        if (actual_keys != expected_keys or actual_type != expected_type or
                any(secret_meta.get('labels', {}).get(key) != value for key, value in NAMESPACE_LABELS.items())):
            raise ValueError(name + ' Secret shape or ownership labels changed')
        public = base64.b64decode(kubectl(
            'get', 'secret', name, '-o', 'go-template={{index .data "' + pem_key + '"}}'), validate=True)
        if fingerprint_pem(public) != expected_fingerprint:
            raise ValueError(name + ' public certificate differs from this operation')
        if resource.get('uid') and (secret_meta['uid'] != resource['uid'] or
                                    secret_meta['resourceVersion'] != resource['afterResourceVersion']):
            raise ValueError(name + ' identity changed since cutover')
        resource.update({'uid': secret_meta['uid'], 'afterResourceVersion': secret_meta['resourceVersion']})
    else:
        if fingerprint_pem(base64.b64decode(item['data'][pem_key], validate=True)) != expected_fingerprint:
            raise ValueError(name + ' issuer certificate differs from this operation')
        create_operation_secret(name, item, receipt, resource, phase_prefix, path, temp_path)


def accept_unchanged_voice_leaf(current_fingerprint, current_resource_version, expected):
    """Accept an explicit no-op leaf only when the receipt pins its exact public state."""
    if (current_fingerprint != expected.get('oldLeafSha256') or
            current_fingerprint != expected.get('newLeafSha256') or
            current_resource_version not in (expected.get('beforeResourceVersion'),
                                             expected.get('afterResourceVersion'))):
        return False
    expected['afterResourceVersion'] = current_resource_version
    return True


def require_rollback_allowed(phase):
    if phase in FORWARD_PHASES or phase == 'rolled-back':
        raise ValueError('leaf cutover has begun; resume forward, never restore the prior exposed key')


def role_outbound_bundle(deployment):
    value = copy.deepcopy(deployment)
    if value.get('kind') != 'Deployment' or value.get('metadata', {}).get('name') != ROLE:
        raise ValueError('unexpected Role Deployment')
    containers = value.get('spec', {}).get('template', {}).get('spec', {}).get('containers', [])
    role = next((item for item in containers if item.get('name') == 'role'), None)
    if role is None:
        raise ValueError('Role container missing')
    jwks_vars = [item for item in role.get('env', []) if item.get('name') == 'S2S_JWKS_CA_FILE']
    if len(jwks_vars) != 1 or jwks_vars[0].get('value') not in ('/etc/voice/media/ca/ca.crt', '/etc/voice/media/ca/jwks-ca.crt'):
        raise ValueError('Role JWKS trust configuration is unknown')
    env = jwks_vars[0]
    env['value'] = '/etc/voice/media/ca/jwks-ca.crt'
    client_vars = [item for item in role.get('env', []) if item.get('name') == 'ROLE_PRINCIPAL_CLIENT_CA_FILE']
    if len(client_vars) != 1 or client_vars[0].get('value') != '/etc/voice/media/ca/ca.crt':
        raise ValueError('Role inbound client CA configuration is unknown')
    spec = value['spec']['template']['spec']
    volume = next((item for item in spec.get('volumes', []) if item.get('name') == 'media-ca'), None)
    if volume is None or volume.get('secret', {}).get('secretName') != CA_SECRET:
        raise ValueError('Role media CA projection is unknown')
    items = volume['secret'].setdefault('items', [])
    ca = [item for item in items if item.get('key') == 'ca.crt']
    if len(ca) != 1 or ca[0].get('path') != 'ca.crt':
        raise ValueError('Role inbound CA projection changed')
    jwks = [item for item in items if item.get('key') == 'jwks-ca.crt']
    if jwks and (len(jwks) != 1 or jwks[0].get('path') != 'jwks-ca.crt'):
        raise ValueError('Role outbound JWKS CA projection is unknown')
    if not jwks:
        items.append({'key': 'jwks-ca.crt', 'path': 'jwks-ca.crt'})
    return value


def voice_tls_restart(deployment, cert_fingerprint):
    value = copy.deepcopy(deployment)
    if value.get('kind') != 'Deployment' or value.get('metadata', {}).get('name') != 'voice-voice':
        raise ValueError('unexpected Voice Deployment')
    template = value.get('spec', {}).get('template', {})
    annotations = template.setdefault('metadata', {}).setdefault('annotations', {})
    annotations['voice.io/media-jwks-cert-sha256'] = cert_fingerprint
    return value


def fingerprint_pem(pem):
    # Fingerprint the public DER representation without printing certificate data.
    result = subprocess.run(['openssl', 'x509', '-noout', '-fingerprint', '-sha256'],
                            input=pem, capture_output=True, timeout=20)
    if result.returncode:
        raise ValueError('public certificate parsing failed')
    return result.stdout.decode('ascii').strip().split('=', 1)[-1]


def append_public_ca(bundle, certificate):
    """Append one public CA while leaving every existing bundle byte intact."""
    if certificate in bundle:
        return bundle
    return bundle + (b'' if bundle.endswith(b'\n') else b'\n') + certificate


def apply(image, state_dir):
    # A local lock prevents two operator invocations from interleaving. It stores no key material.
    state_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    if state_dir.is_symlink() or state_dir.stat().st_uid != os.geteuid() or (state_dir.stat().st_mode & 0o077):
        raise ValueError('operator receipt directory must be private and owned by the caller')
    lock_path = state_dir / 'apply.lock'
    with operator_lock(lock_path):
        reject_existing_apply_journal(state_dir)
        namespace = verify_namespace()
        exported_secrets = safe_secret_list(issuer(image, 'export'))

        # This one Secret is documented as public-only. Do not issue payload reads for any other Secret.
        ca_obj = json.loads(kubectl('get', 'secret', CA_SECRET, '-o', 'json'))
        if ca_obj.get('metadata', {}).get('namespace') != NAMESPACE or set(ca_obj.get('data', {})) != {'ca.crt', 'jwks-ca.crt'}:
            raise ValueError('media CA Secret is not the documented public-only shape')
        old_bundle = base64.b64decode(ca_obj['data']['jwks-ca.crt'], validate=True)
        old_ca = base64.b64decode(ca_obj['data']['ca.crt'], validate=True)
        new_ca = base64.b64decode(exported_secrets[CA_SECRET]['data']['ca.crt'], validate=True)
        ca_meta = metadata('secret', CA_SECRET)
        if (ca_obj.get('metadata', {}).get('uid') != ca_meta['uid'] or
                ca_obj.get('metadata', {}).get('resourceVersion') != ca_meta['resourceVersion']):
            raise ValueError('media CA changed during preflight; retry')
        new_bundle = append_public_ca(old_bundle, new_ca)
        old_ca_fingerprint = fingerprint_pem(old_ca)
        new_ca_fingerprint = fingerprint_pem(new_ca)

        role_meta = metadata('deployment', ROLE)
        role_raw = kubectl('get', 'deployment', ROLE, '-o', 'json')
        role_original = json.loads(role_raw)
        if (role_original.get('metadata', {}).get('uid') != role_meta['uid'] or
                role_original.get('metadata', {}).get('resourceVersion') != role_meta['resourceVersion']):
            raise ValueError('Role Deployment changed during preflight; retry')
        role_container = next(item for item in role_original['spec']['template']['spec']['containers']
                              if item.get('name') == 'role')
        prior_jwks_path = next(item['value'] for item in role_container.get('env', [])
                               if item.get('name') == 'S2S_JWKS_CA_FILE')
        role_volume = next(item for item in role_original['spec']['template']['spec']['volumes']
                           if item.get('name') == 'media-ca')
        prior_projection = copy.deepcopy(role_volume['secret'].get('items', []))
        role_obj = role_outbound_bundle(role_original)
        role_obj.setdefault('metadata', {}).update(role_meta)

        voice_meta = metadata('secret', VOICE_SECRET)
        voice_public = kubectl('get', 'secret', VOICE_SECRET,
                               '-o', 'go-template={{index .data "tls.crt"}}')
        old_voice_cert = base64.b64decode(voice_public, validate=True)
        voice_after_public = metadata('secret', VOICE_SECRET)
        if voice_meta['resourceVersion'] != voice_after_public['resourceVersion'] or voice_meta['uid'] != voice_after_public['uid']:
            raise ValueError('Voice TLS Secret changed during preflight; retry')
        voice_old_fingerprint = fingerprint_pem(old_voice_cert)
        new_voice_cert = base64.b64decode(exported_secrets[VOICE_SECRET]['data']['tls.crt'], validate=True)
        new_voice_fingerprint = fingerprint_pem(new_voice_cert)

        for name in (MATCHMAKING_CA_SECRET, GIS_SECRET, MATCHMAKING_SECRET):
            if kubectl('get', 'secret', name, '--ignore-not-found', '-o', 'name').strip():
                raise ValueError(name + ' already exists; refusing to overwrite or assume key validity')

        receipt = {'namespace': NAMESPACE, 'namespaceUid': namespace['uid'], 'volume': VOLUME,
                   'operationId': secrets.token_hex(16), 'issuerImage': image_id(image),
                   'resources': {'voice-media-ca': {'uid': ca_meta['uid'], 'beforeResourceVersion': ca_meta['resourceVersion'],
                                                    'afterResourceVersion': ca_meta['resourceVersion'],
                                                    'oldCaSha256': old_ca_fingerprint, 'newCaSha256': new_ca_fingerprint},
                                 'voice-role': {'uid': role_meta['uid'], 'beforeResourceVersion': role_meta['resourceVersion'],
                                                'afterResourceVersion': role_meta['resourceVersion']},
                                 VOICE_SECRET: {'uid': voice_meta['uid'], 'beforeResourceVersion': voice_meta['resourceVersion'],
                                                'afterResourceVersion': voice_meta['resourceVersion'],
                                                'oldLeafSha256': voice_old_fingerprint,
                                                'newLeafSha256': new_voice_fingerprint},
                                  MATCHMAKING_CA_SECRET: {'createOnly': True, 'uid': None, 'afterResourceVersion': None,
                                                          'publicCaSha256': new_ca_fingerprint},
                                  GIS_SECRET: {'createOnly': True, 'uid': None, 'afterResourceVersion': None},
                                  MATCHMAKING_SECRET: {'createOnly': True, 'uid': None, 'afterResourceVersion': None}},
                   'newGisLeafSha256': fingerprint_pem(base64.b64decode(
                       exported_secrets[GIS_SECRET]['data']['tls.crt'], validate=True)),
                   'newMatchmakingLeafSha256': fingerprint_pem(base64.b64decode(
                       exported_secrets[MATCHMAKING_SECRET]['data']['tls.crt'], validate=True)),
                   'rollback': {'oldJwksCaBundleBase64': base64.b64encode(old_bundle).decode('ascii'),
                                'rolePriorJwksCaPath': prior_jwks_path,
                                'rolePriorCaProjection': prior_projection,
                                'leafRollback': 'forward-reissue only; do not restore the exposed prior leaf key'}}
        receipt_path = state_dir / 'last-apply.json'
        tmp = state_dir / '.last-apply.json.tmp'
        write_receipt(receipt_path, receipt, tmp)

        # Preserve every existing public CA byte and all unrelated Secret fields.
        phase = 'preflight-complete'
        # Trust additions are already a forward-only cutover: once either trust
        # object changes, rolling it back can strand a leaf issued by this CA.
        # Persist the boundary before the first Kubernetes mutation so a crash
        # between API success and receipt update is resumable.
        receipt['phase'] = 'leaf-cutover-started'
        write_receipt(receipt_path, receipt, tmp)
        if new_bundle != old_bundle:
            ca_obj['metadata'] = ca_meta
            ca_obj['data']['jwks-ca.crt'] = base64.b64encode(new_bundle).decode('ascii')
            receipt['resources'][CA_SECRET]['afterResourceVersion'] = replace_object('secret', {}, ca_obj)
            phase = 'trust-added'
            receipt['phase'] = phase
            write_receipt(receipt_path, receipt, tmp)
        else:
            receipt['resources'][CA_SECRET]['afterResourceVersion'] = ca_meta['resourceVersion']
        if role_obj['spec'] != role_original['spec']:
            receipt['resources'][ROLE]['afterResourceVersion'] = replace_object('deployment', {}, role_obj)
            phase = 'role-trust-switched'
            receipt['phase'] = phase
            write_receipt(receipt_path, receipt, tmp)
        else:
            receipt['resources'][ROLE]['afterResourceVersion'] = role_meta['resourceVersion']
        write_receipt(receipt_path, receipt, tmp)

        reconcile_operation_secret(MATCHMAKING_CA_SECRET, exported_secrets[MATCHMAKING_CA_SECRET],
                                  'ca.crt', new_ca_fingerprint, receipt,
                                  receipt['resources'][MATCHMAKING_CA_SECRET], 'matchmaking-ca',
                                  receipt_path, tmp)

        voice_deploy_meta = metadata('deployment', 'voice-voice')
        voice_deploy_original = json.loads(kubectl('get', 'deployment', 'voice-voice', '-o', 'json'))
        if (voice_deploy_original.get('metadata', {}).get('uid') != voice_deploy_meta['uid'] or
                voice_deploy_original.get('metadata', {}).get('resourceVersion') != voice_deploy_meta['resourceVersion']):
            raise ValueError('Voice Deployment changed during preflight; retry')
        voice_deploy = voice_tls_restart(voice_deploy_original, new_voice_fingerprint)
        receipt['resources']['voice-voice-deployment'] = {
            'uid': voice_deploy_meta['uid'], 'beforeResourceVersion': voice_deploy_meta['resourceVersion']}

        if new_voice_cert != old_voice_cert:
            phase = 'leaf-cutover-started'
            receipt['phase'] = phase
            write_receipt(receipt_path, receipt, tmp)
            exported_secrets[VOICE_SECRET]['metadata'] = voice_meta
            exported_secrets[VOICE_SECRET]['metadata'].setdefault('annotations', {})[
                'voice.io/media-pki-operation'] = receipt['operationId']
            receipt['resources'][VOICE_SECRET]['afterResourceVersion'] = replace_object('secret', {}, exported_secrets[VOICE_SECRET])
            phase = 'voice-leaf-rotated'
            receipt['phase'] = phase
            write_receipt(receipt_path, receipt, tmp)
        else:
            receipt['resources'][VOICE_SECRET]['afterResourceVersion'] = voice_meta['resourceVersion']

        reconcile_operation_secret(GIS_SECRET, exported_secrets[GIS_SECRET], 'tls.crt',
                                   receipt['newGisLeafSha256'], receipt,
                                   receipt['resources'][GIS_SECRET], 'gis', receipt_path, tmp)
        reconcile_operation_secret(MATCHMAKING_SECRET, exported_secrets[MATCHMAKING_SECRET], 'tls.crt',
                                   receipt['newMatchmakingLeafSha256'], receipt,
                                   receipt['resources'][MATCHMAKING_SECRET], 'matchmaking-tls',
                                   receipt_path, tmp)
        if voice_deploy['spec'] != voice_deploy_original['spec']:
            receipt['resources']['voice-voice-deployment']['afterResourceVersion'] = replace_object(
                'deployment', {}, voice_deploy)
        else:
            receipt['resources']['voice-voice-deployment']['afterResourceVersion'] = voice_deploy_meta['resourceVersion']
        phase = 'leaf-cutover-complete'
        receipt['phase'] = phase
        write_receipt(receipt_path, receipt, tmp)

        kubectl('rollout', 'status', 'deployment/' + ROLE, '--timeout=180s')
        kubectl('rollout', 'status', 'deployment/voice-voice', '--timeout=180s')
        print('DEV media JWKS PKI applied; receipt=' + str(receipt_path) +
              ' old_ca_sha256=' + old_ca_fingerprint + ' new_ca_sha256=' + new_ca_fingerprint +
              ' prior_voice_leaf_sha256=' + voice_old_fingerprint + ' new_voice_leaf_sha256=' + new_voice_fingerprint +
              ' phase=' + phase)


def renew(image, state_dir):
    """Renew private-volume leaves only before cutover; later rotations need a reviewed workflow."""
    state_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    if state_dir.is_symlink() or state_dir.stat().st_uid != os.geteuid() or (state_dir.stat().st_mode & 0o077):
        raise ValueError('operator receipt directory must be private and owned by the caller')
    with operator_lock(state_dir / 'apply.lock'):
        journal = state_dir / 'last-apply.json'
        if journal.exists() or journal.is_symlink():
            raise ValueError('issuer renewal is disabled after the one-time cutover journal exists')
        issuer(image, 'renew-leaves')
        print('DEV issuer leaves renewed in the protected volume; no cluster resources changed')


def rollback(state_dir):
    state_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    if state_dir.is_symlink() or state_dir.stat().st_uid != os.geteuid() or (state_dir.stat().st_mode & 0o077):
        raise ValueError('operator receipt directory must be private and owned by the caller')
    receipt_path = state_dir / 'last-apply.json'
    if receipt_path.is_symlink() or not receipt_path.is_file():
        raise ValueError('no protected DEV media PKI receipt exists')
    receipt = json.loads(receipt_path.read_text(encoding='utf-8'))
    if receipt.get('namespace') != NAMESPACE or receipt.get('volume') != VOLUME:
        raise ValueError('receipt is not for the fixed DEV media PKI operation')
    phase = receipt.get('phase', 'preflight-complete')
    require_rollback_allowed(phase)
    with operator_lock(state_dir / 'apply.lock'):
        namespace = verify_namespace()
        if namespace['uid'] != receipt.get('namespaceUid'):
            raise ValueError('DEV namespace identity changed since the recorded operation')
        expected_ca = receipt['resources'][CA_SECRET]
        ca_meta = metadata('secret', CA_SECRET)
        if ca_meta['uid'] != expected_ca['uid'] or ca_meta['resourceVersion'] != expected_ca.get('afterResourceVersion'):
            raise ValueError('media CA changed since this operation; refusing rollback')
        ca_obj = json.loads(kubectl('get', 'secret', CA_SECRET, '-o', 'json'))
        if ca_obj.get('metadata', {}).get('uid') != ca_meta['uid'] or set(ca_obj.get('data', {})) != {'ca.crt', 'jwks-ca.crt'}:
            raise ValueError('media CA Secret shape changed; refusing rollback')
        expected_role = receipt['resources'][ROLE]
        role_meta = metadata('deployment', ROLE)
        if role_meta['uid'] != expected_role['uid'] or role_meta['resourceVersion'] != expected_role.get('afterResourceVersion'):
            raise ValueError('Role Deployment changed since this operation; refusing rollback')
        role_obj = json.loads(kubectl('get', 'deployment', ROLE, '-o', 'json'))
        if role_obj.get('metadata', {}).get('resourceVersion') != role_meta['resourceVersion']:
            raise ValueError('Role Deployment changed during rollback; retry')
        role_container = next(item for item in role_obj['spec']['template']['spec']['containers']
                              if item.get('name') == 'role')
        env = next(item for item in role_container.get('env', []) if item.get('name') == 'S2S_JWKS_CA_FILE')
        env['value'] = receipt['rollback']['rolePriorJwksCaPath']
        volume = next(item for item in role_obj['spec']['template']['spec']['volumes']
                      if item.get('name') == 'media-ca')
        volume['secret']['items'] = receipt['rollback']['rolePriorCaProjection']
        role_obj['metadata'] = role_meta
        replace_object('deployment', {}, role_obj)
        ca_obj['metadata'] = ca_meta
        ca_obj['data']['jwks-ca.crt'] = receipt['rollback']['oldJwksCaBundleBase64']
        replace_object('secret', {}, ca_obj)
        kubectl('rollout', 'status', 'deployment/' + ROLE, '--timeout=180s')
        receipt['phase'] = 'rolled-back'
        write_receipt(receipt_path, receipt, state_dir / '.last-apply.json.tmp')
        print('DEV media PKI pre-cutover changes rolled back with UID/resourceVersion guards')


def resume(image, state_dir):
    """Continue a cutover only when the retained receipt and public state agree."""
    state_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    if state_dir.is_symlink() or state_dir.stat().st_uid != os.geteuid() or (state_dir.stat().st_mode & 0o077):
        raise ValueError('operator receipt directory must be private and owned by the caller')
    path = state_dir / 'last-apply.json'
    if path.is_symlink() or not path.is_file():
        raise ValueError('no protected DEV media PKI receipt exists')
    info = path.stat()
    if info.st_uid != os.geteuid() or (info.st_mode & 0o077):
        raise ValueError('existing apply journal ownership or mode is unsafe')
    receipt = json.loads(path.read_text(encoding='utf-8'))
    if receipt.get('namespace') != NAMESPACE or receipt.get('volume') != VOLUME:
        raise ValueError('receipt is not for the fixed DEV media PKI operation')
    if receipt.get('issuerImage') != image_id(image):
        raise ValueError('resume image does not match the pinned operation image')
    phase = receipt.get('phase', '')
    forward_phases = FORWARD_PHASES
    if phase not in forward_phases:
        raise ValueError('only a journaled forward-only cutover can be resumed')
    with operator_lock(state_dir / 'apply.lock'):
        namespace = verify_namespace()
        if namespace['uid'] != receipt.get('namespaceUid'):
            raise ValueError('DEV namespace identity changed since the recorded operation')
        exported = safe_secret_list(issuer(image, 'export'))
        ca_expected = receipt['resources'][CA_SECRET]
        ca_meta = metadata('secret', CA_SECRET)
        if ca_meta['uid'] != ca_expected['uid']:
            raise ValueError('media CA identity changed since cutover; refusing resume')
        ca_obj = json.loads(kubectl('get', 'secret', CA_SECRET, '-o', 'json'))
        if set(ca_obj.get('data', {})) != {'ca.crt', 'jwks-ca.crt'}:
            raise ValueError('media CA Secret shape changed; refusing resume')
        ca_bytes = base64.b64decode(ca_obj['data']['ca.crt'], validate=True)
        bundle = base64.b64decode(ca_obj['data']['jwks-ca.crt'], validate=True)
        new_ca = base64.b64decode(exported[CA_SECRET]['data']['ca.crt'], validate=True)
        if fingerprint_pem(ca_bytes) != ca_expected['oldCaSha256'] or fingerprint_pem(new_ca) != ca_expected['newCaSha256']:
            raise ValueError('retained CA trust does not match the cutover receipt')
        prior_bundle = base64.b64decode(receipt['rollback']['oldJwksCaBundleBase64'], validate=True)
        desired_bundle = append_public_ca(prior_bundle, new_ca)
        if bundle == prior_bundle:
            if (phase != 'leaf-cutover-started' or
                    ca_meta['resourceVersion'] != ca_expected['beforeResourceVersion']):
                raise ValueError('media CA trust is not at a resumable pre-update version')
            ca_obj['metadata'] = ca_meta
            ca_obj['data']['jwks-ca.crt'] = base64.b64encode(desired_bundle).decode('ascii')
            ca_expected['afterResourceVersion'] = replace_object('secret', {}, ca_obj)
        elif bundle == desired_bundle:
            ca_expected['afterResourceVersion'] = ca_meta['resourceVersion']
        else:
            raise ValueError('media CA bundle differs from the exact recorded transition')
        write_receipt(path, receipt, state_dir / '.last-apply.json.tmp')

        role_expected = receipt['resources'][ROLE]
        role_meta = metadata('deployment', ROLE)
        if role_meta['uid'] != role_expected['uid']:
            raise ValueError('Role Deployment identity changed since cutover; refusing resume')
        role_obj = json.loads(kubectl('get', 'deployment', ROLE, '-o', 'json'))
        desired_role = role_outbound_bundle(role_obj)
        if desired_role['spec'] != role_obj.get('spec'):
            if (phase not in ('leaf-cutover-started', 'trust-added') or
                    role_meta['resourceVersion'] != role_expected['beforeResourceVersion']):
                raise ValueError('Role trust is not at a resumable pre-update version')
            role_expected['afterResourceVersion'] = replace_object('deployment', {}, desired_role)
        else:
            role_expected['afterResourceVersion'] = role_meta['resourceVersion']
        write_receipt(path, receipt, state_dir / '.last-apply.json.tmp')

        reconcile_operation_secret(MATCHMAKING_CA_SECRET, exported[MATCHMAKING_CA_SECRET],
                                  'ca.crt', ca_expected['newCaSha256'], receipt,
                                  receipt['resources'][MATCHMAKING_CA_SECRET], 'matchmaking-ca',
                                  path, state_dir / '.last-apply.json.tmp')

        voice_expected = receipt['resources'][VOICE_SECRET]
        voice_meta = metadata('secret', VOICE_SECRET)
        if voice_meta['uid'] != voice_expected['uid']:
            raise ValueError('Voice TLS Secret identity changed since cutover')
        public = kubectl('get', 'secret', VOICE_SECRET,
                         '-o', 'go-template={{index .data "tls.crt"}}')
        current_cert = base64.b64decode(public, validate=True)
        current_fp = fingerprint_pem(current_cert)
        desired = exported[VOICE_SECRET]
        desired_cert = base64.b64decode(desired['data']['tls.crt'], validate=True)
        desired_fp = fingerprint_pem(desired_cert)
        if desired_fp != voice_expected['newLeafSha256']:
            raise ValueError('issuer leaf changed since cutover; preserve receipt and review forward reissue')
        if current_fp == voice_expected['oldLeafSha256'] == voice_expected['newLeafSha256']:
            if not accept_unchanged_voice_leaf(current_fp, voice_meta['resourceVersion'], voice_expected):
                raise ValueError('unchanged Voice leaf has an unexpected resource version')
            # This operation deliberately did not replace the identical public
            # certificate. Continue to GIS creation without falsely claiming a
            # Voice Secret mutation or downgrading the durable phase.
        elif current_fp == voice_expected['oldLeafSha256']:
            if (phase not in ('leaf-cutover-started', 'trust-added', 'role-trust-switched',
                              'matchmaking-ca-create-intent', 'matchmaking-ca-created') or
                    voice_meta['resourceVersion'] != voice_expected['beforeResourceVersion']):
                raise ValueError('Voice leaf is not at a resumable pre-replacement version')
            desired['metadata'] = voice_meta
            desired['metadata'].setdefault('annotations', {})['voice.io/media-pki-operation'] = receipt['operationId']
            voice_expected['afterResourceVersion'] = replace_object('secret', {}, desired)
            receipt['phase'] = 'voice-leaf-rotated'
            write_receipt(path, receipt, state_dir / '.last-apply.json.tmp')
        elif current_fp == voice_expected['newLeafSha256']:
            if (voice_meta['resourceVersion'] != voice_expected.get('afterResourceVersion') and
                    voice_meta.get('annotations', {}).get('voice.io/media-pki-operation') != receipt['operationId']):
                raise ValueError('Voice leaf matches but its operation identity is not pinned')
            voice_expected['afterResourceVersion'] = voice_meta['resourceVersion']
            receipt['phase'] = 'voice-leaf-rotated'
        else:
            raise ValueError('Voice leaf matches neither receipt fingerprint; refusing resume')
        write_receipt(path, receipt, state_dir / '.last-apply.json.tmp')

        deployment_expected = receipt['resources']['voice-voice-deployment']
        deployment_meta = metadata('deployment', 'voice-voice')
        if deployment_meta['uid'] != deployment_expected['uid']:
            raise ValueError('Voice Deployment identity changed since cutover')
        deployment = json.loads(kubectl('get', 'deployment', 'voice-voice', '-o', 'json'))
        annotations = deployment.get('spec', {}).get('template', {}).get('metadata', {}).get('annotations', {})
        if annotations.get('voice.io/media-jwks-cert-sha256') != current_fp:
            if deployment_meta['resourceVersion'] != deployment_expected['beforeResourceVersion']:
                raise ValueError('Voice Deployment changed outside the recorded operation')
            deployment = voice_tls_restart(deployment, current_fp)
            deployment_expected['afterResourceVersion'] = replace_object('deployment', {}, deployment)
        else:
            deployment_expected['afterResourceVersion'] = deployment_meta['resourceVersion']
        write_receipt(path, receipt, state_dir / '.last-apply.json.tmp')

        reconcile_operation_secret(GIS_SECRET, exported[GIS_SECRET], 'tls.crt',
                                   receipt['newGisLeafSha256'], receipt,
                                   receipt['resources'][GIS_SECRET], 'gis', path,
                                   state_dir / '.last-apply.json.tmp')
        reconcile_operation_secret(MATCHMAKING_SECRET, exported[MATCHMAKING_SECRET], 'tls.crt',
                                   receipt['newMatchmakingLeafSha256'], receipt,
                                   receipt['resources'][MATCHMAKING_SECRET], 'matchmaking-tls', path,
                                   state_dir / '.last-apply.json.tmp')
        receipt['phase'] = 'leaf-cutover-complete'
        write_receipt(path, receipt, state_dir / '.last-apply.json.tmp')
        kubectl('rollout', 'status', 'deployment/' + ROLE, '--timeout=180s')
        kubectl('rollout', 'status', 'deployment/voice-voice', '--timeout=180s')
        print('DEV media JWKS PKI forward recovery completed; receipt=' + str(path))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('prepare', 'renew', 'apply', 'resume', 'rollback'))
    parser.add_argument('--issuer-image', help='immutable local image ID sha256:<64 hex>')
    parser.add_argument('--confirm-dev-pki-change', action='store_true')
    args = parser.parse_args()
    os.umask(0o077)
    try:
        if sys.platform != 'linux':
            raise ValueError('operator actions are restricted to the designated Linux host')
        if args.action != 'rollback':
            if not args.issuer_image:
                raise ValueError('--issuer-image is required')
            image_id(args.issuer_image)
        if args.action == 'prepare':
            run(['docker', 'volume', 'create', '--label', 'io.voice.environment=dev',
                 '--label', 'io.voice.purpose=media-pki', VOLUME])
            issuer(args.issuer_image, 'initialize')
            print('DEV issuer initialized in Docker named volume ' + VOLUME + '; no cluster resources changed')
        elif args.action == 'renew':
            renew(args.issuer_image, DEFAULT_STATE)
        elif args.action == 'rollback':
            if not args.confirm_dev_pki_change:
                raise ValueError('rollback requires --confirm-dev-pki-change after independent review')
            rollback(DEFAULT_STATE)
        elif args.action == 'resume':
            if not args.issuer_image:
                raise ValueError('--issuer-image is required')
            if not args.confirm_dev_pki_change:
                raise ValueError('resume requires --confirm-dev-pki-change after independent review')
            resume(args.issuer_image, DEFAULT_STATE)
        else:
            if not args.confirm_dev_pki_change:
                raise ValueError('apply requires --confirm-dev-pki-change after independent review')
            apply(args.issuer_image, DEFAULT_STATE)
    except (OSError, ValueError, subprocess.TimeoutExpired, json.JSONDecodeError):
        print('DEV media PKI operation failed; no Secret values printed', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
