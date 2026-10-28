#!/usr/bin/env python3
"""Stage isolated DEV media identities offline; never apply or export secrets."""
import argparse
import base64
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import tempfile

NAMESPACE = 'voice-dev'
OPERATION = re.compile(r'smp-[a-f0-9]{32}\Z')
HASH = re.compile(r'[a-f0-9]{64}\Z')
SERVER_NAMES = ('voice-file-principal.voice-dev.svc',
                'voice-story-principal.voice-dev.svc',
                'voice-messaging-principal.voice-dev.svc')
DOMAINS = {'server': SERVER_NAMES, 'story-client': ('voice-story.voice-dev.svc',),
           'file-client': ('voice-file.voice-dev.svc',)}
LEAVES = {
    'voice-file-principal-grpc-tls': ('server', SERVER_NAMES[0], 'serverAuth'),
    'voice-story-jwks-tls': ('server', SERVER_NAMES[1], 'serverAuth'),
    'voice-messaging-jwks-tls': ('server', SERVER_NAMES[2], 'serverAuth'),
    'voice-story-file-client-tls': ('story-client', DOMAINS['story-client'][0], 'clientAuth'),
    'voice-file-jwks-client-tls': ('file-client', DOMAINS['file-client'][0], 'clientAuth'),
}


def openssl(*args, data=None):
    result = subprocess.run(['openssl', *map(str, args)], input=data, capture_output=True, timeout=60)
    if result.returncode:
        raise ValueError('offline certificate operation failed')
    return result.stdout


def write_private(path, data):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        stream.write(data)
        stream.flush()
        os.fsync(stream.fileno())


def sync_dir(path):
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def safe_directory(path):
    if path.is_symlink() or not path.is_dir():
        raise ValueError('issuer state directory unavailable')
    info = path.stat()
    if info.st_uid != os.geteuid() or info.st_mode & 0o777 != 0o700:
        raise ValueError('issuer state directory custody invalid')


def secret(name, data, operation, source, kind='Opaque'):
    return {'apiVersion': 'v1', 'kind': 'Secret', 'type': kind, 'immutable': True,
            'metadata': {'name': name, 'namespace': NAMESPACE,
                         'labels': {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'},
                         'annotations': {'voice.dev/story-media-pki-operation': operation,
                                         'voice.dev/story-media-issuer-sha256': source}},
            'data': {key: base64.b64encode(value).decode('ascii') for key, value in data.items()}}


def make_ca(work, domain):
    key, cert = work / (domain + '.key'), work / (domain + '.crt')
    constraints = ','.join('permitted;DNS:' + value for value in DOMAINS[domain])
    openssl('req', '-x509', '-newkey', 'rsa:3072', '-nodes', '-sha256', '-days', '3650',
            '-subj', '/CN=Voice DEV Story Media ' + domain, '-addext', 'basicConstraints=critical,CA:TRUE,pathlen:0',
            '-addext', 'keyUsage=critical,keyCertSign,cRLSign', '-addext', 'nameConstraints=critical,' + constraints,
            '-keyout', key, '-out', cert)
    return key, cert


def make_leaf(work, name, domain, dns, purpose, ca):
    key, csr, cert, ext = (work / (name + suffix) for suffix in ('.key', '.csr', '.crt', '.ext'))
    openssl('req', '-new', '-newkey', 'rsa:2048', '-nodes', '-sha256', '-subj', '/CN=' + dns,
            '-keyout', key, '-out', csr)
    ext.write_text('basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\n'
                   'extendedKeyUsage=' + purpose + '\nsubjectAltName=DNS:' + dns + '\n', encoding='ascii')
    openssl('x509', '-req', '-in', csr, '-CA', ca[1], '-CAkey', ca[0], '-CAcreateserial',
            '-sha256', '-days', '397', '-extfile', ext, '-out', cert)
    openssl('verify', '-CAfile', ca[1], '-purpose', 'sslserver' if purpose == 'serverAuth' else 'sslclient',
            '-verify_hostname', dns, cert)
    if openssl('pkey', '-in', key, '-pubout') != openssl('x509', '-in', cert, '-pubkey', '-noout'):
        raise ValueError('leaf certificate mismatch')
    return {'tls.crt': cert.read_bytes(), 'tls.key': key.read_bytes()}


def create_packet(work, operation, source):
    cas = {domain: make_ca(work, domain) for domain in DOMAINS}
    objects = []
    for name, (domain, dns, purpose) in LEAVES.items():
        objects.append(secret(name, make_leaf(work, name, domain, dns, purpose, cas[domain]),
                              operation, source, 'kubernetes.io/tls'))
    for name, domain in (('voice-principal-trust', 'server'),
                         ('voice-file-principal-client-ca', 'story-client'),
                         ('voice-messaging-jwks-client-ca', 'file-client')):
        objects.append(secret(name, {'ca.crt': cas[domain][1].read_bytes()}, operation, source))
    for name in ('voice-story-principal-keys', 'voice-messaging-principal-signing-' + operation[4:16]):
        keys = {}
        for kid in ('current', 'next'):
            path = work / (name + '-' + kid + '.pem')
            openssl('genpkey', '-algorithm', 'RSA', '-pkeyopt', 'rsa_keygen_bits:2048', '-out', path)
            keys[kid + '.pem'] = path.read_bytes()
        if openssl('pkey', '-pubout', data=keys['current.pem']) == openssl('pkey', '-pubout', data=keys['next.pem']):
            raise ValueError('rotation keys must differ')
        objects.append(secret(name, keys, operation, source))
    return objects, {domain: key.read_bytes() for domain, (key, _) in cas.items()}


def receipt_for(operation, source):
    names = set(LEAVES) | {'voice-principal-trust', 'voice-file-principal-client-ca',
                         'voice-messaging-jwks-client-ca', 'voice-story-principal-keys',
                         'voice-messaging-principal-signing-' + operation[4:16]}
    return {'operation': operation, 'issuer_source_sha256': source, 'namespace': NAMESPACE,
            'objects': sorted(names)}


def extension_lines(cert, extension):
    return [line.strip() for line in openssl('x509', '-noout', '-ext', extension, data=cert).decode('ascii').splitlines()]


def validate_ca_constraints(lines, domain):
    expected = ['X509v3 Name Constraints: critical', 'Permitted:',
                *('DNS:' + value for value in DOMAINS[domain])]
    if lines != expected:
        raise ValueError('issuer staged CA constraints invalid')


def validate_leaf_extensions(cert, dns, purpose):
    expected = {
        'basicConstraints': ['X509v3 Basic Constraints: critical', 'CA:FALSE'],
        'keyUsage': ['X509v3 Key Usage: critical', 'Digital Signature, Key Encipherment'],
        'extendedKeyUsage': ['X509v3 Extended Key Usage:',
                             'TLS Web Server Authentication' if purpose == 'serverAuth' else 'TLS Web Client Authentication'],
        'subjectAltName': ['X509v3 Subject Alternative Name:', 'DNS:' + dns],
    }
    for extension, lines in expected.items():
        if extension_lines(cert, extension) != lines:
            raise ValueError('issuer staged leaf extension invalid')


def rsa_public(key, bits):
    openssl('rsa', '-check', '-noout', data=key)
    public = openssl('pkey', '-pubout', data=key)
    details = openssl('pkey', '-pubin', '-text', '-noout', data=public)
    if ('(' + str(bits) + ' bit)').encode() not in details:
        raise ValueError('issuer staged RSA size invalid')
    return public


def validate_signer(key, public_keys):
    if not key.startswith(b'-----BEGIN PRIVATE KEY-----\n'):
        raise ValueError('issuer staged signer must be PKCS8')
    public = rsa_public(key, 2048)
    if public in public_keys:
        raise ValueError('issuer staged signer key invalid')
    public_keys.add(public)


def validate_packet(packet, ca_keys, operation, source):
    receipt = receipt_for(operation, source)
    if (not isinstance(packet, dict) or set(packet) != {'apiVersion', 'kind', 'items'} or
            packet['apiVersion'] != 'v1' or packet['kind'] != 'List' or not isinstance(packet['items'], list)):
        raise ValueError('issuer staged packet profile changed')
    objects = packet['items']
    if (len(objects) != len(receipt['objects']) or
            any(not isinstance(item, dict) or not isinstance(item.get('metadata'), dict) or
                not isinstance(item.get('metadata', {}).get('name'), str) or
                not isinstance(item.get('data'), dict) for item in objects)):
        raise ValueError('issuer staged object inventory changed')
    names = [item.get('metadata', {}).get('name') for item in objects]
    if sorted(names) != receipt['objects']:
        raise ValueError('issuer staged object inventory changed')
    decoded = {}
    for item in objects:
        name = item['metadata']['name']
        try:
            data = {key: base64.b64decode(value, validate=True) for key, value in item['data'].items()}
        except (KeyError, TypeError, ValueError):
            raise ValueError('issuer staged object data invalid') from None
        expected_keys = ({'tls.crt', 'tls.key'} if name in LEAVES else
                         {'current.pem', 'next.pem'} if 'principal-signing-' in name or name == 'voice-story-principal-keys' else {'ca.crt'})
        if set(data) != expected_keys or item != secret(name, data, operation, source,
                'kubernetes.io/tls' if name in LEAVES else 'Opaque'):
            raise ValueError('issuer staged object shape changed')
        decoded[name] = data
    ca_certs = {'server': decoded['voice-principal-trust']['ca.crt'],
                'story-client': decoded['voice-file-principal-client-ca']['ca.crt'],
                'file-client': decoded['voice-messaging-jwks-client-ca']['ca.crt']}
    public_keys = set()
    for domain in DOMAINS:
        cert, key = ca_certs[domain], ca_keys[domain]
        public = rsa_public(key, 3072)
        if public != openssl('x509', '-pubkey', '-noout', data=cert) or public in public_keys:
            raise ValueError('issuer staged CA key mismatch')
        public_keys.add(public)
        basic = extension_lines(cert, 'basicConstraints')
        usage = extension_lines(cert, 'keyUsage')
        if basic != ['X509v3 Basic Constraints: critical', 'CA:TRUE, pathlen:0'] or usage != [
                'X509v3 Key Usage: critical', 'Certificate Sign, CRL Sign']:
            raise ValueError('issuer staged CA profile invalid')
        validate_ca_constraints(extension_lines(cert, 'nameConstraints'), domain)
        openssl('x509', '-checkend', '0', '-noout', data=cert)
    with tempfile.TemporaryDirectory(prefix='story-media-public-certs-') as tmp:
        directory = Path(tmp)
        for domain, cert in ca_certs.items():
            (directory / (domain + '.crt')).write_bytes(cert)
        for name, (domain, dns, purpose) in LEAVES.items():
            cert, key = decoded[name]['tls.crt'], decoded[name]['tls.key']
            public = rsa_public(key, 2048)
            if public != openssl('x509', '-pubkey', '-noout', data=cert) or public in public_keys:
                raise ValueError('issuer staged leaf key mismatch')
            public_keys.add(public)
            validate_leaf_extensions(cert, dns, purpose)
            cert_path = directory / (name + '.crt')
            cert_path.write_bytes(cert)
            openssl('verify', '-CAfile', directory / (domain + '.crt'), '-purpose',
                    'sslserver' if purpose == 'serverAuth' else 'sslclient', '-verify_hostname', dns, cert_path)
    for name in ('voice-story-principal-keys', 'voice-messaging-principal-signing-' + operation[4:16]):
        for key in decoded[name].values():
            validate_signer(key, public_keys)


def load_stage(root, operation, source, export=False):
    folder = root / operation
    safe_directory(folder)
    expected = {'stage.json', *(name + '.key' for name in DOMAINS)}
    if {p.name for p in folder.iterdir()} != expected:
        raise ValueError('issuer staged inventory invalid')
    for path in folder.iterdir():
        if path.is_symlink() or not path.is_file() or path.stat().st_uid != os.geteuid() or path.stat().st_mode & 0o777 != 0o600:
            raise ValueError('issuer staged file custody invalid')
    value = json.loads((folder / 'stage.json').read_bytes())
    if value['operation'] != operation or value['issuer_source_sha256'] != source:
        raise ValueError('issuer staged identity changed')
    receipt = receipt_for(operation, source)
    if value.get('receipt') != receipt:
        raise ValueError('issuer staged receipt changed')
    raw = base64.b64decode(value['packet'], validate=True)
    if hashlib.sha256(raw).hexdigest() != value['packet_sha256']:
        raise ValueError('issuer staged packet changed')
    ca_keys = {domain: (folder / (domain + '.key')).read_bytes() for domain in DOMAINS}
    for domain in DOMAINS:
        if hashlib.sha256(ca_keys[domain]).hexdigest() != value['private_key_sha256'][domain]:
            raise ValueError('issuer staged CA changed')
    packet = json.loads(raw)
    validate_packet(packet, ca_keys, operation, source)
    return raw if export else receipt


def stage(state_dir, operation, source, export_sink=None):
    if not OPERATION.fullmatch(operation) or not HASH.fullmatch(source):
        raise ValueError('issuer identity invalid')
    if hashlib.sha256(Path(__file__).read_bytes()).hexdigest() != source:
        raise ValueError('issuer source mismatch')
    root = Path(state_dir)
    safe_directory(root)
    fd = os.open(root / '.lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    try:
        info = os.fstat(fd)
        if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid() or
                info.st_mode & 0o777 != 0o600 or info.st_nlink != 1):
            raise ValueError('issuer lock custody invalid')
        sync_dir(root)
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            raise ValueError('issuer busy') from None
        if export_sink is not None:
            children = [path for path in root.iterdir() if path.name != '.lock']
            if children != [root / operation]:
                raise ValueError('issuer existing stage required for export')
            raw = load_stage(root, operation, source, export=True)
            export_sink(raw)
            return None
        return stage_locked(root, operation, source)
    finally:
        os.close(fd)


def stage_locked(root, operation, source):
    final, partial = root / operation, root / (operation + '.building')
    children = [path for path in root.iterdir() if path.name != '.lock']
    if final.exists() and not final.is_symlink() and children == [final]:
        return load_stage(root, operation, source)
    if children or partial.is_symlink() or final.is_symlink():
        raise ValueError('issuer existing or incomplete state requires reconciliation')
    partial.mkdir(mode=0o700)
    sync_dir(root)
    write_private(partial / 'intent.json', json.dumps({'operation': operation, 'issuer_source_sha256': source}).encode())
    sync_dir(partial)
    work = partial / 'work'
    work.mkdir(mode=0o700)
    objects, ca_keys = create_packet(work, operation, source)
    raw = json.dumps({'apiVersion': 'v1', 'kind': 'List', 'items': objects}, sort_keys=True, separators=(',', ':')).encode()
    receipt = receipt_for(operation, source)
    value = {'operation': operation, 'issuer_source_sha256': source, 'receipt': receipt,
             'packet': base64.b64encode(raw).decode('ascii'), 'packet_sha256': hashlib.sha256(raw).hexdigest(),
             'private_key_sha256': {name: hashlib.sha256(key).hexdigest() for name, key in ca_keys.items()}}
    for name, key in ca_keys.items():
        write_private(partial / (name + '.key'), key)
    write_private(partial / 'stage.json', json.dumps(value, sort_keys=True).encode())
    shutil.rmtree(work)
    (partial / 'intent.json').unlink()
    sync_dir(partial)
    os.rename(partial, final)
    sync_dir(root)
    return load_stage(root, operation, source)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--state-dir', default='/state')
    parser.add_argument('--operation', required=True)
    parser.add_argument('--issuer-source-sha256', required=True)
    parser.add_argument('--export-packet', action='store_true')
    args = parser.parse_args()
    try:
        if args.export_packet:
            if not stat.S_ISFIFO(os.fstat(sys.stdout.fileno()).st_mode):
                raise ValueError('packet export requires a pipe')
            def send(raw):
                sys.stdout.buffer.write(raw)
                sys.stdout.buffer.flush()
            stage(args.state_dir, args.operation, args.issuer_source_sha256, export_sink=send)
        else:
            receipt = stage(args.state_dir, args.operation, args.issuer_source_sha256)
            print(json.dumps(receipt, sort_keys=True))
    except (OSError, ValueError, KeyError, TypeError, AttributeError, UnicodeError, subprocess.SubprocessError):
        print('issuer stage failed closed; preserve state for reconciliation', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
