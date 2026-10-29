"""Closed cryptographic checks for Role's two peer identity Secrets."""
import base64
import copy
import re

import space_creator_runtime_profile_issuer_packet as crypto
import space_creator_runtime_profile_material as material

MEMBERS = {
    'voice-role-media-client-tls': {'tls.crt', 'tls.key'},
    'voice-role-principal-signing': {'current.pem', 'next.pem', 'active-kid'},
}


def qualify(name, obj, media):
    """Validate one actual Secret; return public JWKS only for the signer."""
    try:
        if (name not in MEMBERS or not isinstance(obj, dict) or
                obj.get('apiVersion') != 'v1' or obj.get('kind') != 'Secret' or
                obj.get('type') != 'Opaque' or not isinstance(obj.get('metadata'), dict) or
                obj['metadata'].get('name') != name or obj['metadata'].get('namespace') != 'voice-dev' or
                obj['metadata'].get('deletionTimestamp') or not isinstance(obj.get('data'), dict) or
                set(obj['data']) != MEMBERS[name]):
            raise ValueError()
        values = {key: base64.b64decode(value, validate=True) for key, value in obj['data'].items()}
        if any(not value or len(value) > 131072 for value in values.values()):
            raise ValueError()
        if name == 'voice-role-media-client-tls':
            material._pem(values['tls.crt'], b'CERTIFICATE')
            material._pem(values['tls.key'], b'PRIVATE KEY')
            crypto._certificate(values, media, 'voice-role', 'clientAuth')
            return None
        if values['active-kid'] not in (b'current', b'next'):
            raise ValueError()
        keys = []
        for kid in ('current', 'next'):
            key = values[kid + '.pem']
            material._pem(key, b'PRIVATE KEY')
            crypto._openssl(['pkey', '-in', '{0}', '-check', '-noout'], [key])
            modulus = crypto._openssl(['rsa', '-in', '{0}', '-noout', '-modulus'], [key]).decode('ascii').strip()
            if not modulus.startswith('Modulus='):
                raise ValueError()
            n = int(modulus.removeprefix('Modulus='), 16)
            public = crypto._openssl(['pkey', '-in', '{0}', '-pubout', '-text_pub', '-noout'], [key]).decode('ascii')
            exponent = re.search(r'^Exponent: ([0-9]+)', public, re.MULTILINE)
            if n.bit_length() < 2048 or not exponent:
                raise ValueError()
            encode = lambda value: base64.urlsafe_b64encode(value.to_bytes((value.bit_length() + 7) // 8, 'big')).decode('ascii').rstrip('=')
            keys.append({'kty': 'RSA', 'kid': kid, 'use': 'sig', 'alg': 'RS256', 'n': encode(n), 'e': encode(int(exponent.group(1)))})
        if keys[0]['n'] == keys[1]['n']:
            raise ValueError()
        return copy.deepcopy({'keys': keys})
    except Exception:
        raise ValueError('Role peer material qualification failed') from None
