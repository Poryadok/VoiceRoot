"""Source-owned fixed bridge restrictions; installed support is separate."""
from controller import Blocked

COMMAND = ('/usr/bin/python3', '-I', '-S', '/var/lib/voice-nats-preservation/installed/code/nats-rollout-preservation/bridge.py')
CAPABILITIES = ('CAP_CHOWN', 'CAP_DAC_OVERRIDE', 'CAP_FOWNER', 'CAP_NET_ADMIN',
                'CAP_SYS_CHROOT', 'CAP_SYS_PTRACE', 'CAP_SYS_ADMIN')
CAPABILITY_MASK = '00000000002c100b'
UNIT_IDENTITY = {'User': 'root', 'Group': 'root', 'NoNewPrivileges': 'yes',
                 'AmbientCapabilities': [], 'CapabilityBoundingSet': int(CAPABILITY_MASK, 16)}
UNIT_DIRECTIVES = 'AmbientCapabilities=\nCapabilityBoundingSet=' + ' '.join(CAPABILITIES) + '\n'


def expected_process():
    # No custom filter variant: a mode-2 installed filter needs separately
    # source-defined support evidence. Never learn the observed filter.
    return {key: CAPABILITY_MASK for key in ('CapPrm', 'CapEff', 'CapBnd')} | {'NoNewPrivs': '1', 'Seccomp': '0'}


def fail(): raise Blocked('hub_bridge_identity_unbound')


def command(raw):
    if (type(raw) is not bytes or len(raw) > 4096
            or raw != b'\0'.join(x.encode('ascii') for x in COMMAND) + b'\0'): fail()


def process(fields):
    if (type(fields) is not dict or fields.get('Uid', '').split() != ['0'] * 4
            or fields.get('Gid', '').split() != ['0'] * 4 or fields.get('NoNewPrivs') != '1'
            or fields.get('CapInh') != '0' * 16 or fields.get('CapAmb') != '0' * 16
            or any(fields.get(key) != value for key, value in expected_process().items())
            or len(fields.get('NSpid', '').split()) != 1): fail()


def loaded(read, guard):
    # Authenticated private loaded-property acquisition remains a separate
    # prerequisite. This predicate never approves capabilities or tool bytes.
    if not callable(read) or not callable(guard): fail()
    guard(); first = read(); guard(); final = read(); guard()
    expected = {**UNIT_IDENTITY, 'ExecStart': list(COMMAND)}
    if type(first) is not dict or first != expected or final != expected: fail()
    return expected
