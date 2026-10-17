#!/usr/bin/env python3
"""Offline DEV issuer for the purpose-bound CompleteMatch transport identities."""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

NAMESPACE = 'voice-dev'
OLD_OPERATION = 'msq-3291e91f800a569465ad084a6bb1a61b'
OLD_SOURCE_SHA = '76882140d4f2abda4a40923bc67a6ec78aa69393'
OLD_TEMPLATE_SHA256 = '5ad1c0250574abfadda4bbcf8baa3cd3b3a1525be811f303ea9ea40161e67e0e'
OLD_IMAGE_ID = 'sha256:3f3c5fb9a9b0dd2fcb7660dd6b6c1b48957f380553b838f16c2a9309fac035fd'
OLD_JOURNAL_SHA256 = '09e04ea84d04a55362570aabf082b84f07d70d0e3d170b73047e867b74eb5e33'
OLD_CA_NAME = 'voice-match-squad-ca'
OLD_CA_VOLUME = 'voice-dev-match-squad-pki'
OLD_CA_CERT_PATH = '/input/retained-ca.crt'
OLD_CA_KEY_PATH = '/retained/' + OLD_OPERATION + '/ca.key'
SERVER_SECRET = 'voice-matchmaking-complete-server-tls'
CLIENT_CA_NAME = 'voice-matchmaking-complete-client-ca'
GATEWAY_SECRET = 'voice-gateway-matchmaking-complete-client-tls'
OPERATION_ANNOTATION = 'voice.dev/complete-match-pki-operation'
SOURCE_ANNOTATION = 'voice.dev/complete-match-pki-source-sha256'
RETAINED_CA_FINGERPRINT_ANNOTATION = 'voice.dev/complete-match-retained-ca-sha256'
LABELS = {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'}
PROFILES = {
    SERVER_SECRET: ('voice-matchmaking.voice-dev.svc', 'serverAuth'),
    GATEWAY_SECRET: ('voice-gateway.voice-dev.svc', 'clientAuth'),
}
EXPECTED_NAMES = {SERVER_SECRET, CLIENT_CA_NAME, GATEWAY_SECRET}


def _openssl(args, *, input_bytes=None):
    result = subprocess.run(['openssl', *args], input=input_bytes, capture_output=True,
                           timeout=60, check=False, env={'PATH': '/usr/bin:/bin', 'HOME': '/nonexistent'})
    if result.returncode:
        raise ValueError('offline CompleteMatch certificate check failed')
    return result.stdout


def _write_exclusive(path, value, mode=0o600):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, mode)
    with os.fdopen(fd, 'wb') as stream:
        stream.write(value)
        stream.flush()
        os.fsync(stream.fileno())


def _private_path(path):
    path = Path(path)
    if path.is_symlink() or not path.is_file():
        raise ValueError('private issuer input is unavailable')
    info = path.stat()
    if (info.st_uid != os.geteuid() or info.st_mode & 0o777 != 0o600 or
            info.st_nlink != 1 or not 1 <= info.st_size <= 16384):
        raise ValueError('private issuer input custody is unsafe')
    return path


def _read_private(path):
    return _private_path(path).read_bytes()


def _read_public_ca(path):
    path = Path(path)
    if path.is_symlink() or not path.is_file():
        raise ValueError('retained public CA certificate is unavailable')
    info = path.stat()
    if not 1 <= info.st_size <= 65536:
        raise ValueError('retained public CA certificate size is invalid')
    raw = path.read_bytes()
    if raw.count(b'-----BEGIN CERTIFICATE-----') != 1 or raw.count(b'-----END CERTIFICATE-----') != 1:
        raise ValueError('retained public CA certificate must be one PEM certificate')
    return raw


def _fingerprint(cert_pem):
    der = _openssl(['x509', '-outform', 'DER'], input_bytes=cert_pem)
    return hashlib.sha256(der).hexdigest()


def _validate_ca(key_bytes, cert_pem):
    if isinstance(key_bytes, (str, Path)):
        key_path = _private_path(key_bytes)
        private_public = _openssl(['pkey', '-in', str(key_path), '-pubout'])
    else:
        private_public = _openssl(['pkey', '-pubout'], input_bytes=key_bytes)
    certificate_public = _openssl(['x509', '-pubkey', '-noout'], input_bytes=cert_pem)
    if private_public != certificate_public:
        raise ValueError('retained CA key does not match its public certificate')
    constraints = _openssl(['x509', '-noout', '-ext', 'basicConstraints'], input_bytes=cert_pem).decode('ascii')
    usage = _openssl(['x509', '-noout', '-ext', 'keyUsage'], input_bytes=cert_pem).decode('ascii')
    if 'CA:TRUE' not in constraints or 'Certificate Sign' not in usage:
        raise ValueError('retained certificate is not a certificate-signing CA')
    return _fingerprint(cert_pem)


def _make_new_ca(work):
    key, cert = work / 'gateway-client-ca.key', work / 'gateway-client-ca.crt'
    _openssl(['req', '-x509', '-newkey', 'rsa:3072', '-nodes', '-sha256', '-days', '3650',
              '-subj', '/CN=Voice DEV CompleteMatch Gateway Client CA',
              '-addext', 'basicConstraints=critical,CA:TRUE,pathlen:0',
              '-addext', 'keyUsage=critical,keyCertSign,cRLSign',
              '-addext', 'nameConstraints=critical,permitted;DNS:voice-gateway.voice-dev.svc',
              '-keyout', str(key), '-out', str(cert)])
    return key, cert


def _make_leaf(work, name, dns_name, purpose, ca_key, ca_cert):
    key, csr, cert, ext = (work / (name + suffix) for suffix in ('.key', '.csr', '.crt', '.ext'))
    _openssl(['req', '-new', '-newkey', 'rsa:2048', '-nodes', '-sha256', '-subj', '/CN=' + dns_name,
              '-keyout', str(key), '-out', str(csr)])
    ext.write_text('basicConstraints=critical,CA:FALSE\n'
                   'keyUsage=critical,digitalSignature,keyEncipherment\n'
                   'extendedKeyUsage=' + purpose + '\n'
                   'subjectAltName=DNS:' + dns_name + '\n', encoding='ascii')
    _openssl(['x509', '-req', '-in', str(csr), '-CA', str(ca_cert), '-CAkey', str(ca_key),
              '-CAcreateserial', '-sha256', '-days', '397', '-extfile', str(ext), '-out', str(cert)])
    _verify_leaf(cert.read_bytes(), ca_cert.read_bytes(), dns_name, purpose)
    if _openssl(['pkey', '-in', str(key), '-pubout']) != _openssl(
            ['x509', '-in', str(cert), '-pubkey', '-noout']):
        raise ValueError('leaf certificate and private key do not match')
    return cert, key


def _verify_leaf(cert_pem, ca_pem, dns_name, purpose):
    with tempfile.TemporaryDirectory(prefix='complete-match-verify-') as temp:
        root = Path(temp)
        ca_path, cert_path = root / 'ca.crt', root / 'leaf.crt'
        ca_path.write_bytes(ca_pem)
        cert_path.write_bytes(cert_pem)
        _openssl(['verify', '-CAfile', str(ca_path),
                  '-purpose', 'sslclient' if purpose == 'clientAuth' else 'sslserver',
                  '-verify_hostname', dns_name, str(cert_path)])
    san = _openssl(['x509', '-noout', '-ext', 'subjectAltName'], input_bytes=cert_pem).decode('ascii')
    eku = _openssl(['x509', '-noout', '-ext', 'extendedKeyUsage'], input_bytes=cert_pem).decode('ascii')
    constraints = _openssl(['x509', '-noout', '-ext', 'basicConstraints'], input_bytes=cert_pem).decode('ascii')
    wanted_eku = 'TLS Web Client Authentication' if purpose == 'clientAuth' else 'TLS Web Server Authentication'
    sans = [line.strip() for line in san.splitlines() if line.strip().startswith('DNS:')]
    ekus = [line.strip() for line in eku.splitlines() if line.strip().startswith('TLS Web ')]
    if sans != ['DNS:' + dns_name] or ekus != [wanted_eku] or 'CA:FALSE' not in constraints:
        raise ValueError('leaf SAN, EKU, or CA constraint is invalid')


def _secret(name, data, operation_id, source_sha, retained_ca_fingerprint):
    annotations = {OPERATION_ANNOTATION: operation_id, SOURCE_ANNOTATION: source_sha}
    if name == SERVER_SECRET:
        annotations[RETAINED_CA_FINGERPRINT_ANNOTATION] = retained_ca_fingerprint
    return {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {
        'name': name, 'namespace': NAMESPACE, 'labels': dict(LABELS), 'annotations': annotations},
        'type': 'kubernetes.io/tls', 'immutable': True,
        'data': {key: base64.b64encode(value).decode('ascii') for key, value in data.items()}}


def build_packet(operation_id, source_sha, retained_ca_key, retained_ca_cert):
    if not re.fullmatch(r'cmp-[a-f0-9]{32}', operation_id):
        raise ValueError('invalid CompleteMatch operation identity')
    if not re.fullmatch(r'[0-9a-f]{40}', source_sha):
        raise ValueError('invalid CompleteMatch source binding')
    old_key = _private_path(retained_ca_key) if isinstance(retained_ca_key, (str, Path)) else bytes(retained_ca_key)
    old_cert = _read_public_ca(retained_ca_cert) if isinstance(retained_ca_cert, (str, Path)) else bytes(retained_ca_cert)
    retained_fp = _validate_ca(old_key, old_cert)
    with tempfile.TemporaryDirectory(prefix='complete-match-packet-') as temp_name:
        work = Path(temp_name)
        old_cert_path = work / 'retained-ca.crt'
        old_cert_path.write_bytes(old_cert)
        server_cert, server_key = _make_leaf(work, SERVER_SECRET, 'voice-matchmaking.voice-dev.svc',
                                             'serverAuth', old_key, old_cert_path)
        client_ca_key, client_ca_cert = _make_new_ca(work)
        gateway_cert, gateway_key = _make_leaf(work, GATEWAY_SECRET, 'voice-gateway.voice-dev.svc',
                                               'clientAuth', client_ca_key, client_ca_cert)
        objects = [
            {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {
                'name': CLIENT_CA_NAME, 'namespace': NAMESPACE, 'labels': dict(LABELS),
                'annotations': {OPERATION_ANNOTATION: operation_id, SOURCE_ANNOTATION: source_sha}},
             'immutable': True, 'data': {'ca.crt': client_ca_cert.read_text(encoding='ascii')}},
            _secret(SERVER_SECRET, {'tls.crt': server_cert.read_bytes(), 'tls.key': server_key.read_bytes()},
                    operation_id, source_sha, retained_fp),
            _secret(GATEWAY_SECRET, {'tls.crt': gateway_cert.read_bytes(), 'tls.key': gateway_key.read_bytes()},
                    operation_id, source_sha, retained_fp),
        ]
        packet = {'apiVersion': 'v1', 'kind': 'List', 'items': objects}
        raw = json.dumps(packet, sort_keys=True, separators=(',', ':')).encode('utf-8')
        client_ca_key_bytes = client_ca_key.read_bytes()
    return raw, client_ca_key_bytes


def verify_packet_profiles(raw, retained_ca_cert):
    value = json.loads(raw)
    if value.get('apiVersion') != 'v1' or value.get('kind') != 'List':
        raise ValueError('CompleteMatch issuer packet is not a v1 List')
    by_name = {item.get('metadata', {}).get('name'): item for item in value.get('items', [])}
    if len(by_name) != 3 or set(by_name) != EXPECTED_NAMES:
        raise ValueError('CompleteMatch issuer packet has an unexpected resource set')
    server = by_name[SERVER_SECRET]['data']
    client_ca = by_name[CLIENT_CA_NAME]['data']['ca.crt'].encode('ascii')
    gateway = by_name[GATEWAY_SECRET]['data']
    retained = bytes(retained_ca_cert)
    for name, (dns, purpose), ca, item in (
            (SERVER_SECRET, PROFILES[SERVER_SECRET], retained, server),
            (GATEWAY_SECRET, PROFILES[GATEWAY_SECRET], client_ca, gateway)):
        cert = base64.b64decode(item['tls.crt'], validate=True)
        key = base64.b64decode(item['tls.key'], validate=True)
        if _openssl(['pkey', '-pubout'], input_bytes=key) != _openssl(
                ['x509', '-pubkey', '-noout'], input_bytes=cert):
            raise ValueError('CompleteMatch leaf key does not match certificate')
        _verify_leaf(cert, ca, dns, purpose)
    ca_constraints = _openssl(['x509', '-noout', '-ext', 'basicConstraints'], input_bytes=client_ca).decode('ascii')
    ca_usage = _openssl(['x509', '-noout', '-ext', 'keyUsage'], input_bytes=client_ca).decode('ascii')
    ca_names = _openssl(['x509', '-noout', '-ext', 'nameConstraints'], input_bytes=client_ca).decode('ascii')
    if ('CA:TRUE' not in ca_constraints or 'Certificate Sign' not in ca_usage or
            'DNS:voice-gateway.voice-dev.svc' not in ca_names):
        raise ValueError('CompleteMatch Gateway client CA profile is invalid')
    return {'retainedCaFingerprint': _fingerprint(retained),
            'clientCaFingerprint': _fingerprint(client_ca),
            'serverLeafFingerprint': _fingerprint(base64.b64decode(server['tls.crt'], validate=True)),
            'gatewayLeafFingerprint': _fingerprint(base64.b64decode(gateway['tls.crt'], validate=True))}


def _state_check(state_dir):
    state_dir = Path(state_dir)
    if state_dir.is_symlink() or not state_dir.is_dir():
        raise ValueError('CompleteMatch issuer state directory is unavailable')
    info = state_dir.stat()
    if info.st_uid != os.geteuid() or info.st_mode & 0o777 != 0o700:
        raise ValueError('CompleteMatch issuer state ownership or mode is unsafe')
    return state_dir


def _load_stage(state_dir, operation_id, source_sha, retained_ca_key, retained_ca_cert):
    state_dir = _state_check(state_dir)
    final = state_dir / operation_id
    if final.is_symlink() or not final.is_dir():
        raise ValueError('exact CompleteMatch staged operation is unavailable')
    directory_info = final.stat()
    if directory_info.st_uid != os.geteuid() or directory_info.st_mode & 0o777 != 0o700:
        raise ValueError('CompleteMatch staged directory custody is unsafe')
    if {path.name for path in final.iterdir()} != {'stage.json', 'gateway-client-ca.key'}:
        raise ValueError('CompleteMatch staged operation has unexpected entries')
    stage_path, key_path = final / 'stage.json', final / 'gateway-client-ca.key'
    manifest = _read_private(stage_path)
    gateway_ca_key = _read_private(key_path)
    value = json.loads(manifest)
    if set(value) != {'operationId', 'sourceSha256', 'manifestSha256', 'manifest'}:
        raise ValueError('CompleteMatch stage record has an unexpected schema')
    raw = base64.b64decode(value['manifest'], validate=True)
    if (value['operationId'] != operation_id or value['sourceSha256'] != source_sha or
            hashlib.sha256(raw).hexdigest() != value['manifestSha256']):
        raise ValueError('CompleteMatch stage binding or integrity is invalid')
    proof = verify_packet_profiles(raw, _read_public_ca(retained_ca_cert))
    packet = json.loads(raw)
    ca_cert = next(row['data']['ca.crt'].encode('ascii') for row in packet['items']
                   if row['kind'] == 'ConfigMap' and row['metadata']['name'] == CLIENT_CA_NAME)
    if _openssl(['pkey', '-pubout'], input_bytes=gateway_ca_key) != _openssl(
            ['x509', '-pubkey', '-noout'], input_bytes=ca_cert):
        raise ValueError('CompleteMatch staged client CA key does not match its public certificate')
    old_key = _private_path(retained_ca_key)
    if _validate_ca(old_key, _read_public_ca(retained_ca_cert)) != proof['retainedCaFingerprint']:
        raise ValueError('CompleteMatch retained signer changed after staging')
    return raw


def stage(state_dir, operation_id, source_sha, retained_ca_key, retained_ca_cert):
    if not re.fullmatch(r'cmp-[a-f0-9]{32}', operation_id) or not re.fullmatch(r'[0-9a-f]{40}', source_sha):
        raise ValueError('CompleteMatch operation/source binding is invalid')
    state_dir = _state_check(state_dir)
    final = state_dir / operation_id
    partial = state_dir / (operation_id + '.building')
    entries = list(state_dir.iterdir())
    if final.is_symlink() or partial.is_symlink():
        raise ValueError('CompleteMatch issuer state contains a symlink')
    if final.exists():
        if partial.exists() or entries != [final]:
            raise ValueError('CompleteMatch issuer final and partial state are ambiguous')
        raw = _load_stage(state_dir, operation_id, source_sha, retained_ca_key, retained_ca_cert)
        return json.dumps({'operationId': operation_id, 'sourceSha256': source_sha,
                           'resourceNames': sorted(EXPECTED_NAMES), 'state': 'staged'}, sort_keys=True)
    if partial.exists():
        info = partial.stat()
        if (entries != [partial] or not partial.is_dir() or info.st_uid != os.geteuid() or
                info.st_mode & 0o777 != 0o700 or any(partial.iterdir())):
            raise ValueError('CompleteMatch partial stage is not the exact private empty operation')
    else:
        if entries:
            raise ValueError('CompleteMatch issuer state already contains an operation')
        partial.mkdir(mode=0o700)
        info = partial.stat()
        if info.st_uid != os.geteuid() or info.st_mode & 0o777 != 0o700:
            raise ValueError('CompleteMatch partial stage custody is unsafe')
    old_key = _private_path(retained_ca_key)
    old_cert = _read_public_ca(retained_ca_cert)
    _validate_ca(old_key, old_cert)
    try:
        with tempfile.TemporaryDirectory(prefix='complete-match-stage-') as temp_name:
            raw, new_client_key = build_packet(operation_id, source_sha, old_key, old_cert)
        payload = {'operationId': operation_id, 'sourceSha256': source_sha,
                   'manifestSha256': hashlib.sha256(raw).hexdigest(),
                   'manifest': base64.b64encode(raw).decode('ascii')}
        _write_exclusive(partial / 'gateway-client-ca.key', new_client_key)
        _write_exclusive(partial / 'stage.json', json.dumps(payload, sort_keys=True).encode('utf-8'))
        os.replace(partial, final)
        verify_packet_profiles(raw, old_cert)
    except BaseException:
        # Partial state is durable evidence. Never clear or regenerate it.
        raise
    return json.dumps({'operationId': operation_id, 'sourceSha256': source_sha,
                       'resourceNames': sorted(EXPECTED_NAMES), 'state': 'staged'}, sort_keys=True)


def export(state_dir, operation_id, source_sha, retained_ca_key, retained_ca_cert):
    return _load_stage(state_dir, operation_id, source_sha, retained_ca_key, retained_ca_cert)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='action', required=True)
    for action in ('stage', 'export'):
        command = sub.add_parser(action)
        command.add_argument('--state-dir', default='/state')
        command.add_argument('--operation-id', required=True)
        command.add_argument('--source-sha256', required=True)
    args = parser.parse_args(argv)
    try:
        if args.action == 'stage':
            print(stage(args.state_dir, args.operation_id, args.source_sha256,
                        OLD_CA_KEY_PATH, OLD_CA_CERT_PATH))
        else:
            sys.stdout.buffer.write(export(args.state_dir, args.operation_id, args.source_sha256,
                                           OLD_CA_KEY_PATH, OLD_CA_CERT_PATH))
        return 0
    except (OSError, ValueError, json.JSONDecodeError, TypeError):
        print('CompleteMatch issuer operation failed closed', file=sys.stderr)
        return 2


if __name__ == '__main__':
    raise SystemExit(main())
