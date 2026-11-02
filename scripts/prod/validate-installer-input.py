"""Pure preflight for the release owner's immutable installer selection."""
import re
import sys

# Docker distribution/reference name grammar, with a required SHA-256 digest.
# IPv6 registry literals remain unsupported by the native migration adapter;
# reject them here before any earlier deployment helper can mutate the cluster.
# https://github.com/distribution/reference/blob/main/regexp.go
COMPONENT = r'[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*'
DOMAIN_COMPONENT = r'(?:[A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9-]*[A-Za-z0-9])'
DOMAIN = DOMAIN_COMPONENT + r'(?:\.' + DOMAIN_COMPONENT + r')*'
HOST = DOMAIN + r'(?::[0-9]+)?'
NAME = r'(?:' + HOST + r'/)?' + COMPONENT + r'(?:/' + COMPONENT + r')*'
REFERENCE = re.compile(r'(?P<name>' + NAME + r')(?::[\w][\w.-]{0,127})?@sha256:[a-f0-9]{64}', re.ASCII)


def valid(image, source, candidate):
    match = REFERENCE.fullmatch(image)
    return (match is not None and len(match['name']) <= 255
            and re.fullmatch(r'[a-f0-9]{40}', candidate) is not None
            and candidate != '0' * 40 and source == candidate)


if __name__ == '__main__':
    if len(sys.argv) != 4 or not valid(*sys.argv[1:]):
        print('HOLD: select source-bound SPACE_NATIVE_INSTALLER_IMAGE and '
              'SPACE_NATIVE_INSTALLER_SOURCE_SHA for this candidate', file=sys.stderr)
        sys.exit(1)
