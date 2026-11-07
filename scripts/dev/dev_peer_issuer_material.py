"""Fixed peer material generator; authority is established by the root service."""
import hashlib
import json
import os
from pathlib import Path

import credentials
import dev_peer_issuer_metadata as metadata
import dev_peer_issuer_packet as packet
import realtime_p3_missing_issuer as existing
import reset_credentials as reset
import space_p3_issuer as custody


class MaterialError(RuntimeError):
    pass


def generate(destination, media, profile, missing, operation_id, source_sha,
             namespace_uid, preflight_sha256):
    """Generate only the previously admitted fixed subset; makes no authority claim."""
    try:
        if (profile not in metadata.PROFILES or not isinstance(missing, list) or not missing or
                missing != sorted(set(missing)) or set(missing) - set(metadata.PROFILES[profile]) or
                not isinstance(media, bytes)):
            raise ValueError()
        destination = Path(destination)
        custody._check_ancestors(custody.ARCHIVE)
        info = custody.ARCHIVE.lstat()
        if (not __import__('stat').S_ISDIR(info.st_mode) or info.st_uid != 0 or
                info.st_mode & 0o077 or custody.ARCHIVE.resolve() != custody.ARCHIVE):
            raise ValueError()
        cert_fd, cert_id = custody._open('media-ca.crt', False)
        try:
            key_fd, key_id = custody._open('media-ca.key', True)
            try:
                cert = Path('/proc') / str(os.getpid()) / 'fd' / str(cert_fd)
                key = Path('/proc') / str(os.getpid()) / 'fd' / str(key_fd)
                public = destination / 'media-public.crt'
                credentials.write_private(public, media)
                der = reset._openssl('x509', '-in', cert, '-outform', 'DER')
                if (der != reset._openssl('x509', '-in', public, '-outform', 'DER') or
                        reset._openssl('x509', '-in', cert, '-pubkey', '-noout') !=
                        reset._openssl('pkey', '-in', key, '-pubout') or
                        b'CA:TRUE' not in reset._openssl('x509', '-in', cert, '-noout', '-ext', 'basicConstraints')):
                    raise ValueError()
                material = reset.ResetMaterial(destination)
                for name in missing:
                    if name.endswith('-principal-signing'):
                        material.signer(name, active=True)
                    else:
                        material.tls(name, (key, cert), *packet.TLS[name])
                jwks = {item['metadata']['name']: reset.signer_jwks(item)
                        for item in material.items if item['metadata']['name'].endswith('-principal-signing')}
                receipt = {'schema': 'voice-dev-peer-issuer-receipt-v1', 'status': 'staged',
                           'profile': profile, 'object_names': missing,
                           'media_ca_der_sha256': hashlib.sha256(der).hexdigest(), 'public_jwks': jwks,
                           'runtime_qualified': False, 'cluster_applied': False}
                envelope = {'schema': 'voice-dev-peer-issuer-packet-v1', 'profile': profile,
                            'operation_id': operation_id, 'source_sha': source_sha,
                            'namespace_uid': namespace_uid, 'preflight_sha256': preflight_sha256,
                            'issuer_receipt': receipt,
                            'objects': {'apiVersion': 'v1', 'kind': 'List', 'items': material.items}}
                packet.validate(envelope, profile, operation_id, source_sha, namespace_uid,
                                preflight_sha256, missing, media)
                def identity(value):
                    return (value.st_dev, value.st_ino, value.st_size, value.st_mtime_ns, value.st_ctime_ns)
                if (not existing._unchanged(cert_fd, cert_id) or not existing._unchanged(key_fd, key_id) or
                        identity(os.stat(custody.ARCHIVE / 'media-ca.crt', follow_symlinks=False)) != cert_id or
                        identity(os.stat(custody.ARCHIVE / 'media-ca.key', follow_symlinks=False)) != key_id):
                    raise ValueError()
                custody._check_ancestors(custody.ARCHIVE)
                credentials.write_private(destination / 'objects.json',
                                          json.dumps(envelope, sort_keys=True, separators=(',', ':')) + '\n')
                return receipt
            finally:
                os.close(key_fd)
        finally:
            os.close(cert_fd)
    except Exception:
        raise MaterialError('DEV peer material generation refused') from None
