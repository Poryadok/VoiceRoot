#!/usr/bin/env python3
"""Offline issuer for the isolated voice-dev MatchSquad trust domain."""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import subprocess
import sys
import tempfile

NAMESPACE = 'voice-dev'
CA_NAME = 'voice-match-squad-ca'
OPERATION_ANNOTATION = 'voice.dev/match-squad-pki-operation'
SOURCE_ANNOTATION = 'voice.dev/match-squad-pki-source-sha256'
LABELS = {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'}
PROFILES = {
    'voice-matchmaking-client-tls': ('voice-matchmaking.voice-dev.svc', 'clientAuth'),
    'voice-chat-match-squad-server-tls': ('voice-chat', 'serverAuth'),
    'voice-voice-match-squad-server-tls': ('voice-voice', 'serverAuth'),
}
SIGNING_SECRET = 'voice-matchmaking-principal-signing'
REPLAY_SECRETS = ('voice-match-squad-replay', 'voice-match-squad-member-replay')


def openssl(args, *, input_bytes=None):
    result = subprocess.run(['openssl', *args], input=input_bytes, capture_output=True, timeout=60)
    if result.returncode:
        raise ValueError('offline certificate operation failed')
    return result.stdout


def _write(path, data):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'wb') as output:
        output.write(data)
        output.flush()
        os.fsync(output.fileno())


def _read(path):
    return Path(path).read_bytes()


def _make_ca(work):
    key, cert = work / 'ca.key', work / 'ca.crt'
    openssl(['req', '-x509', '-newkey', 'rsa:3072', '-nodes', '-sha256', '-days', '3650',
             '-subj', '/CN=Voice DEV MatchSquad Root', '-addext', 'basicConstraints=critical,CA:TRUE,pathlen:0',
             '-addext', 'keyUsage=critical,keyCertSign,cRLSign',
             '-addext', 'nameConstraints=critical,permitted;DNS:voice-matchmaking.voice-dev.svc,permitted;DNS:voice-chat,permitted;DNS:voice-voice',
             '-keyout', str(key), '-out', str(cert)])
    return key, cert


def _make_leaf(work, ca_key, ca_cert, name, dns_name, purpose):
    key, csr, cert, ext = (work / (name + suffix) for suffix in ('.key', '.csr', '.crt', '.ext'))
    openssl(['req', '-new', '-newkey', 'rsa:2048', '-nodes', '-sha256', '-subj', '/CN=' + dns_name,
             '-keyout', str(key), '-out', str(csr)])
    ext.write_text('basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=' + purpose + '\nsubjectAltName=DNS:' + dns_name + '\n', encoding='ascii')
    openssl(['x509', '-req', '-in', str(csr), '-CA', str(ca_cert), '-CAkey', str(ca_key),
             '-CAcreateserial', '-sha256', '-days', '397', '-extfile', str(ext), '-out', str(cert)])
    # Verify chain purpose, SAN, and private/public key pairing before staging.
    openssl(['verify', '-CAfile', str(ca_cert), '-purpose', 'sslclient' if purpose == 'clientAuth' else 'sslserver',
             '-verify_hostname', dns_name, str(cert)])
    pub_from_key = openssl(['pkey', '-in', str(key), '-pubout'])
    pub_from_cert = openssl(['x509', '-in', str(cert), '-pubkey', '-noout'])
    if pub_from_key != pub_from_cert:
        raise ValueError('certificate and private key do not match')
    return cert, key


def _secret(name, data, operation_id, source_sha, secret_type='Opaque'):
    return {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {
        'name': name, 'namespace': NAMESPACE, 'labels': dict(LABELS),
        'annotations': {OPERATION_ANNOTATION: operation_id, SOURCE_ANNOTATION: source_sha}},
        'type': secret_type, 'immutable': True,
        'data': {key: base64.b64encode(value).decode('ascii') for key, value in data.items()}}


def build_manifest(work, operation_id, source_sha):
    ca_key, ca_cert = _make_ca(work)
    objects = [{'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {
        'name': CA_NAME, 'namespace': NAMESPACE, 'labels': dict(LABELS),
        'annotations': {OPERATION_ANNOTATION: operation_id, SOURCE_ANNOTATION: source_sha}},
        'immutable': True, 'data': {'ca.crt': _read(ca_cert).decode('ascii')}}]
    for name, (dns_name, purpose) in PROFILES.items():
        cert, key = _make_leaf(work, ca_key, ca_cert, name, dns_name, purpose)
        objects.append(_secret(name, {'tls.crt': _read(cert), 'tls.key': _read(key)}, operation_id,
                               source_sha, 'kubernetes.io/tls'))
    signing = {}
    for label in ('current', 'next'):
        key = work / ('principal-' + label + '.pem')
        openssl(['genpkey', '-algorithm', 'RSA', '-pkeyopt', 'rsa_keygen_bits:2048', '-out', str(key)])
        signing[label + '.pem'] = _read(key)
    signing['active-kid'] = b'current'
    objects.append(_secret(SIGNING_SECRET, signing, operation_id, source_sha))
    replay_values = [secrets.token_urlsafe(48).encode('ascii') for _ in REPLAY_SECRETS]
    if replay_values[0] == replay_values[1]:
        raise ValueError('independent replay values unexpectedly collided')
    for name, value in zip(REPLAY_SECRETS, replay_values):
        objects.append(_secret(name, {'password': value}, operation_id, source_sha))
    return {'apiVersion': 'v1', 'kind': 'List', 'items': objects}, _read(ca_key)


def stage(state_dir, operation_id, source_sha):
    if not re.fullmatch(r'msq-[a-f0-9]{32}', operation_id):
        raise ValueError('invalid operation identity')
    if len(source_sha) != 64 or any(c not in '0123456789abcdef' for c in source_sha):
        raise ValueError('invalid source binding')
    state_dir = Path(state_dir)
    state_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    if state_dir.is_symlink() or not state_dir.is_dir():
        raise ValueError('issuer state directory must be a real directory')
    info = state_dir.stat()
    if info.st_uid != os.geteuid():
        raise ValueError('issuer state directory owner is unsafe')
    os.chmod(state_dir, 0o700)
    final = state_dir / operation_id
    partial = state_dir / (operation_id + '.building')
    if final.exists() or partial.exists() or list(state_dir.iterdir()):
        raise ValueError('issuer state already exists; refusing regeneration or replacement')
    partial.mkdir(mode=0o700)
    try:
        with tempfile.TemporaryDirectory(prefix='match-squad-pki-') as temp_name:
            work = Path(temp_name)
            packet, ca_key = build_manifest(work, operation_id, source_sha)
            raw = json.dumps(packet, sort_keys=True, separators=(',', ':')).encode('utf-8')
        payload = {'operationId': operation_id, 'sourceSha256': source_sha,
                   'manifestSha256': hashlib.sha256(raw).hexdigest(),
                   'manifest': base64.b64encode(raw).decode('ascii'),
                   'caKeySha256': hashlib.sha256(ca_key).hexdigest()}
        _write(partial / 'ca.key', ca_key)
        _write(partial / 'stage.json', json.dumps(payload, sort_keys=True).encode('utf-8'))
        os.replace(partial, final)
    except BaseException:
        # Preserve partial stage for inspection; never silently regenerate after a crash.
        raise
    return raw


def load_stage(state_dir, operation_id, source_sha):
    state_dir = Path(state_dir)
    if state_dir.is_symlink() or not state_dir.is_dir() or state_dir.stat().st_uid != os.geteuid():
        raise ValueError('issuer state directory ownership is unsafe')
    final = state_dir / operation_id
    if final.is_symlink() or not final.is_dir():
        raise ValueError('exact staged operation is unavailable')
    info = final.stat()
    if info.st_uid != os.geteuid() or info.st_mode & 0o077:
        raise ValueError('staged issuer material ownership or mode is unsafe')
    stage_file = final / 'stage.json'
    ca_key_file = final / 'ca.key'
    for path in (stage_file, ca_key_file):
        if path.is_symlink() or not path.is_file():
            raise ValueError('staged issuer material file is unavailable')
        file_info = path.stat()
        if file_info.st_uid != os.geteuid() or file_info.st_mode & 0o077:
            raise ValueError('staged issuer material file ownership or mode is unsafe')
    value = json.loads(stage_file.read_text(encoding='utf-8'))
    if value.get('operationId') != operation_id or value.get('sourceSha256') != source_sha:
        raise ValueError('staged operation binding does not match')
    raw = base64.b64decode(value['manifest'], validate=True)
    if hashlib.sha256(raw).hexdigest() != value.get('manifestSha256'):
        raise ValueError('staged material integrity check failed')
    ca_key = ca_key_file.read_bytes()
    if hashlib.sha256(ca_key).hexdigest() != value.get('caKeySha256'):
        raise ValueError('staged CA key integrity check failed')
    manifest = json.loads(raw)
    ca_cert = next((item.get('data', {}).get('ca.crt') for item in manifest.get('items', [])
                    if item.get('kind') == 'ConfigMap' and item.get('metadata', {}).get('name') == CA_NAME), None)
    if not ca_cert:
        raise ValueError('staged CA certificate is missing')
    private_public = openssl(['pkey', '-pubout'], input_bytes=ca_key)
    certificate_public = openssl(['x509', '-pubkey', '-noout'], input_bytes=ca_cert.encode('ascii'))
    if private_public != certificate_public:
        raise ValueError('staged CA key does not match its public certificate')
    return raw


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='action', required=True)
    init = sub.add_parser('stage')
    init.add_argument('--state-dir', default='/state')
    init.add_argument('--operation-id', required=True)
    init.add_argument('--source-sha256', required=True)
    export = sub.add_parser('export')
    export.add_argument('--state-dir', default='/state')
    export.add_argument('--operation-id', required=True)
    export.add_argument('--source-sha256', required=True)
    args = parser.parse_args(argv)
    try:
        if args.action == 'stage':
            raw = stage(args.state_dir, args.operation_id, args.source_sha256)
            manifest = json.loads(raw)
            print(json.dumps({'operationId': args.operation_id, 'sourceSha256': args.source_sha256,
                              'manifestSha256': hashlib.sha256(raw).hexdigest(),
                              'resourceNames': sorted(item['metadata']['name'] for item in manifest['items'])},
                             sort_keys=True))
            return 0
        else:
            raw = load_stage(args.state_dir, args.operation_id, args.source_sha256)
        sys.stdout.buffer.write(raw)
        return 0
    except (OSError, ValueError, json.JSONDecodeError):
        print('issuer stage failed closed', file=sys.stderr)
        return 2


if __name__ == '__main__':
    raise SystemExit(main())
