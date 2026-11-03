"""Resolve the post-restart Docker port for the exact diagnostic service."""
import json
import re
import sys


def resolve(container_id, raw):
    if not re.fullmatch(r'[0-9a-f]{64}', container_id):
        raise ValueError('invalid service container identity')
    rows = json.loads(raw)
    if not isinstance(rows, list) or len(rows) != 1 or rows[0].get('Id') != container_id or rows[0].get('State', {}).get('Running') is not True:
        raise ValueError('service container identity or state differs')
    mappings = rows[0].get('NetworkSettings', {}).get('Ports', {}).get('5432/tcp')
    if not isinstance(mappings, list) or not mappings:
        raise ValueError('PostgreSQL published port missing')
    ports = set()
    ipv4 = False
    for mapping in mappings:
        if not isinstance(mapping, dict) or mapping.get('HostIp') not in ('0.0.0.0', '127.0.0.1', '::', '::1'):
            raise ValueError('unexpected PostgreSQL published address')
        port = mapping.get('HostPort')
        if not isinstance(port, str) or not re.fullmatch(r'[1-9][0-9]{0,4}', port) or int(port) > 65535:
            raise ValueError('invalid PostgreSQL published port')
        ports.add(port)
        ipv4 |= mapping['HostIp'] in ('0.0.0.0', '127.0.0.1')
    if len(ports) != 1 or not ipv4:
        raise ValueError('ambiguous PostgreSQL published port')
    return ports.pop()


if __name__ == '__main__':
    print('SPACE_M50_PG_PORT=' + resolve(sys.argv[1], sys.stdin.read()))
