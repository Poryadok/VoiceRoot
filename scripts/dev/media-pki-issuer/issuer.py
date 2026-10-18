#!/usr/bin/env python3
"""Non-root DEV media JWKS issuer. Private material stays in the named volume."""
import argparse
import base64
import json
import os
from pathlib import Path
import secrets
import stat
import subprocess
import sys
import tempfile

STATE = Path('/state')
NAMESPACE = 'voice-dev'
LEAVES = {'voice-voice': 'voice-voice-media-jwks-tls',
          'voice-gameintegration': 'voice-gameintegration-jwks-tls',
          'voice-matchmaking.voice-dev.svc': 'voice-matchmaking-principal-jwks-tls'}
LEAF_SANS = {name: (name,) for name in LEAVES}
CA_EXTENSIONS = (
    '[req]\n'
    'distinguished_name=req_distinguished_name\n'
    '[req_distinguished_name]\n'
    '[v3_ca]\n'
    'basicConstraints=critical,CA:TRUE,pathlen:0\n'
    'keyUsage=critical,keyCertSign,cRLSign\n'
    'subjectKeyIdentifier=hash\n'
    'authorityKeyIdentifier=keyid:always\n'
    'nameConstraints=critical,permitted;DNS:voice-voice,permitted;DNS:voice-gameintegration,'
    'permitted;DNS:voice-matchmaking.voice-dev.svc\n')
CURRENT_PHASE = 'startup'


def phase(value):
    global CURRENT_PHASE
    CURRENT_PHASE = value


def leaf_extensions(dns_name):
    if dns_name not in LEAVES:
        raise ValueError('DNS name is outside the fixed DEV issuer scope')
    sans = ','.join('DNS:' + name for name in LEAF_SANS[dns_name])
    return ('[v3_leaf]\n'
            'basicConstraints=critical,CA:FALSE\n'
            'keyUsage=critical,digitalSignature,keyEncipherment\n'
            'extendedKeyUsage=serverAuth\n'
            'subjectAltName=' + sans + '\n'
            'subjectKeyIdentifier=hash\n'
            'authorityKeyIdentifier=keyid,issuer\n')


def run(*args):
    result = subprocess.run([str(value) for value in args], capture_output=True, timeout=120)
    if result.returncode:
        raise ValueError('issuer operation failed')
    return result.stdout


def must_fail(*args):
    result = subprocess.run([str(value) for value in args], capture_output=True, timeout=120)
    if result.returncode == 0:
        raise ValueError('issuer negative validation unexpectedly passed')


def verify_leaf(root, dns_name, cert):
    for allowed_name in LEAF_SANS[dns_name]:
        run('openssl', 'verify', '-purpose', 'sslserver', '-CAfile', root / 'ca.crt',
            '-verify_hostname', allowed_name, cert)
    for forbidden_name in ('voice-space', 'voice-gateway', 'voice-gateway.example',
                           'voice-matchmaking.voice-staging.svc'):
        must_fail('openssl', 'verify', '-purpose', 'sslserver', '-CAfile', root / 'ca.crt',
                  '-verify_hostname', forbidden_name, cert)


def verify_ca_name_constraints(root, dns_name):
    """Prove the actual CA rejects a matching forbidden SAN, not just a hostname mismatch."""
    with tempfile.TemporaryDirectory(prefix='negative-chain-', dir=root) as directory:
        work = Path(directory)
        key, csr, ext, cert = (work / 'negative.key', work / 'negative.csr',
                               work / 'negative.ext', work / 'negative.crt')
        phase('constraint-negative-keygen')
        run('openssl', 'genpkey', '-algorithm', 'RSA', '-pkeyopt', 'rsa_keygen_bits:2048', '-out', key)
        protect(key, 0o600)
        run('openssl', 'req', '-new', '-sha256', '-key', key,
            '-subj', '/CN=' + dns_name, '-out', csr)
        write_private(ext, ('[v3_negative]\n'
                            'basicConstraints=critical,CA:FALSE\n'
                            'keyUsage=critical,digitalSignature,keyEncipherment\n'
                            'extendedKeyUsage=serverAuth\n'
                            'subjectAltName=DNS:' + dns_name + '\n'))
        phase('constraint-negative-sign')
        run('openssl', 'x509', '-req', '-sha256', '-in', csr,
            '-CA', root / 'ca.crt', '-CAkey', root / 'ca.key', '-days', '1',
            '-extfile', ext, '-extensions', 'v3_negative', '-out', cert,
            '-set_serial', '0x' + secrets.token_hex(16))
        phase('constraint-negative-verify')
        must_fail('openssl', 'verify', '-purpose', 'sslserver', '-CAfile', root / 'ca.crt',
                  '-verify_hostname', dns_name, cert)


def protect(path, mode):
    path = Path(path)
    if path.is_symlink():
        raise ValueError('symlink in private issuer state')
    path.chmod(mode)
    info = path.stat()
    if info.st_uid != os.geteuid() or stat.S_IMODE(info.st_mode) != mode:
        raise ValueError('issuer state ownership or mode is unsafe')


def state_root():
    if sys.platform != 'linux' or os.geteuid() != 10001:
        raise ValueError('dedicated issuer UID 10001 required')
    if STATE.is_symlink() or not STATE.is_dir():
        raise ValueError('named private state volume required')
    protect(STATE, 0o700)
    return STATE


def current_generation(state):
    pointer = state / 'CURRENT'
    if pointer.is_symlink() or not pointer.is_file():
        raise ValueError('issuer has not been initialized')
    protect(pointer, 0o600)
    name = pointer.read_text(encoding='ascii').strip()
    if not name.startswith('g-') or '/' in name or name in ('.', '..'):
        raise ValueError('invalid issuer generation pointer')
    root = state / 'generations' / name
    if root.is_symlink() or not root.is_dir():
        raise ValueError('issuer generation is missing')
    protect(root, 0o700)
    return root


def write_private(path, contents):
    path = Path(path)
    with path.open('xb') as output:
        output.write(contents if isinstance(contents, bytes) else contents.encode())
    protect(path, 0o600)


def issue_leaf(root, dns_name, secret_name):
    if LEAVES.get(dns_name) != secret_name:
        raise ValueError('TLS identity is outside the fixed DEV issuer scope')
    leaf_root = root / 'leaves' / dns_name
    leaf_root.mkdir(parents=True, exist_ok=True, mode=0o700)
    protect(leaf_root, 0o700)
    leaf_id = 'l-' + secrets.token_hex(8)
    work = leaf_root / ('.pending-' + leaf_id)
    work.mkdir(mode=0o700)
    protect(work, 0o700)
    key = work / 'tls.key'
    csr = work / 'tls.csr'
    cert = work / 'tls.crt'
    ext = work / 'leaf.ext'
    phase('leaf-keygen-' + dns_name)
    run('openssl', 'genpkey', '-algorithm', 'RSA', '-pkeyopt', 'rsa_keygen_bits:3072', '-out', key)
    protect(key, 0o600)
    run('openssl', 'req', '-new', '-sha256', '-key', key, '-subj', '/CN=' + dns_name, '-out', csr)
    write_private(ext, leaf_extensions(dns_name))
    phase('leaf-sign-' + dns_name)
    run('openssl', 'x509', '-req', '-sha256', '-in', csr,
        '-CA', root / 'ca.crt', '-CAkey', root / 'ca.key',
        '-days', '90', '-extfile', ext, '-extensions', 'v3_leaf', '-out', cert,
        '-set_serial', '0x' + secrets.token_hex(16))
    protect(cert, 0o600)
    phase('leaf-verify-' + dns_name)
    verify_leaf(root, dns_name, cert)
    csr.unlink()
    ext.unlink()
    final = leaf_root / leaf_id
    os.replace(work, final)
    pointer_tmp = leaf_root / ('.CURRENT-' + secrets.token_hex(8))
    write_private(pointer_tmp, leaf_id + '\n')
    os.replace(pointer_tmp, leaf_root / 'CURRENT')
    return {'apiVersion': 'v1', 'kind': 'Secret',
            'metadata': {'name': secret_name, 'namespace': NAMESPACE},
            'type': 'kubernetes.io/tls',
            'data': {'tls.crt': base64.b64encode((final / 'tls.crt').read_bytes()).decode('ascii'),
                     'tls.key': base64.b64encode((final / 'tls.key').read_bytes()).decode('ascii')}}


def initialize(state):
    if any(state.iterdir()):
        raise ValueError('issuer state is not empty')
    generations = state / 'generations'
    generations.mkdir(mode=0o700)
    protect(generations, 0o700)
    final_name = 'g-' + secrets.token_hex(8)
    pending = generations / ('.pending-' + secrets.token_hex(8))
    pending.mkdir(mode=0o700)
    protect(pending, 0o700)
    try:
        key = pending / 'ca.key'
        cert = pending / 'ca.crt'
        ext = pending / 'ca.ext'
        phase('ca-keygen')
        run('openssl', 'genpkey', '-algorithm', 'RSA', '-pkeyopt', 'rsa_keygen_bits:4096', '-out', key)
        protect(key, 0o600)
        write_private(ext, CA_EXTENSIONS)
        phase('ca-self-sign')
        run('openssl', 'req', '-new', '-x509', '-sha256', '-key', key, '-days', '1825',
            '-subj', '/CN=Voice DEV Media JWKS CA', '-config', ext, '-extensions', 'v3_ca', '-out', cert)
        protect(cert, 0o600)
        for forbidden_name in ('voice-space', 'voice-gateway', 'voice-matchmaking.voice-staging.svc'):
            verify_ca_name_constraints(pending, forbidden_name)
        (pending / 'leaves').mkdir(mode=0o700)
        protect(pending / 'leaves', 0o700)
        for name, secret_name in LEAVES.items():
            issue_leaf(pending, name, secret_name)
        ext.unlink()
        os.replace(pending, generations / final_name)
        pointer_tmp = state / ('.CURRENT-' + secrets.token_hex(8))
        write_private(pointer_tmp, final_name + '\n')
        os.replace(pointer_tmp, state / 'CURRENT')
    except Exception:
        for path in pending.glob('*'):
            if path.is_file() and not path.is_symlink():
                path.unlink()
        pending.rmdir()
        raise


def renew(state):
    root = current_generation(state)
    for dns_name, secret_name in LEAVES.items():
        issue_leaf(root, dns_name, secret_name)


def export(state):
    root = current_generation(state)
    items = [{'apiVersion': 'v1', 'kind': 'Secret',
              'metadata': {'name': 'voice-media-ca', 'namespace': NAMESPACE},
              'type': 'Opaque',
              'data': {'ca.crt': base64.b64encode((root / 'ca.crt').read_bytes()).decode('ascii')}}]
    items.append({'apiVersion': 'v1', 'kind': 'Secret',
                  'metadata': {'name': 'voice-matchmaking-principal-jwks-ca', 'namespace': NAMESPACE},
                  'type': 'Opaque',
                  'data': {'ca.crt': base64.b64encode((root / 'ca.crt').read_bytes()).decode('ascii')}})
    for dns_name, name in LEAVES.items():
        leaf_root = root / 'leaves' / dns_name
        pointer = leaf_root / 'CURRENT'
        if pointer.is_symlink() or not pointer.is_file():
            raise ValueError('leaf material is incomplete')
        protect(leaf_root, 0o700)
        protect(pointer, 0o600)
        leaf_id = pointer.read_text(encoding='ascii').strip()
        if not leaf_id.startswith('l-') or '/' in leaf_id:
            raise ValueError('invalid leaf generation pointer')
        leaf = leaf_root / leaf_id
        if leaf.is_symlink() or not leaf.is_dir():
            raise ValueError('leaf generation is missing')
        protect(leaf, 0o700)
        cert, key = leaf / 'tls.crt', leaf / 'tls.key'
        protect(cert, 0o600)
        protect(key, 0o600)
        items.append({'apiVersion': 'v1', 'kind': 'Secret',
                      'metadata': {'name': name, 'namespace': NAMESPACE},
                      'type': 'kubernetes.io/tls',
                      'data': {'tls.crt': base64.b64encode(cert.read_bytes()).decode('ascii'),
                               'tls.key': base64.b64encode(key.read_bytes()).decode('ascii')}})
    sys.stdout.write(json.dumps({'apiVersion': 'v1', 'kind': 'List', 'items': items}, separators=(',', ':')))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('initialize', 'renew-leaves', 'export'))
    args = parser.parse_args()
    os.umask(0o077)
    try:
        phase('state-check')
        state = state_root()
        if args.action == 'initialize':
            phase('initialize')
            initialize(state)
        elif args.action == 'renew-leaves':
            renew(state)
        else:
            phase('export')
            export(state)
    except (OSError, ValueError, subprocess.TimeoutExpired):
        print('DEV media issuer failed at phase=' + CURRENT_PHASE +
              '; no key or certificate values printed', file=sys.stderr)
        return 1
    if args.action != 'export':
        print('DEV media issuer operation completed; protected volume only')
    return 0


if __name__ == '__main__':
    sys.exit(main())
