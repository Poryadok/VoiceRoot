"""Check selected fields of a rootless daemon profile, not run ownership.

This is not a complete security admission check. Callers must independently
verify all other configuration, daemon, network and volume ownership, journal
lifecycle, and cleanup before using this profile.
"""
import ipaddress
import re


IMAGE = 'sha256:e4b2b083b10ff79a07e24bcc38d6352dff451bdc1e629002aeb7e5956b0b5db7'
MASKED_PATHS = frozenset({
    '/proc/asound', '/proc/acpi', '/proc/kcore', '/proc/keys', '/proc/latency_stats',
    '/proc/timer_list', '/proc/timer_stats', '/proc/sched_debug', '/proc/scsi',
    '/sys/firmware', '/sys/devices/virtual/powercap',
})
READONLY_PATHS = frozenset({
    '/proc/bus', '/proc/fs', '/proc/irq', '/proc/sysrq-trigger', '/proc/sys/abi',
    '/proc/sys/debug', '/proc/sys/dev', '/proc/sys/fs', '/proc/sys/fscache',
    '/proc/sys/kernel', '/proc/sys/sunrpc', '/proc/sys/user', '/proc/sys/vm',
})
ENV = frozenset({
    'DOCKERD_ROOTLESS_ROOTLESSKIT_NET=none', 'DOCKER_TLS_CERTDIR=',
    'DOCKERD_ROOTLESS_ROOTLESSKIT_FLAGS=',
    'PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin',
    'DOCKER_VERSION=27.5.1', 'DOCKER_BUILDX_VERSION=0.20.1', 'DOCKER_COMPOSE_VERSION=2.33.0',
    'DIND_COMMIT=c43aa0b6aa7c88343f0951ba9a39c69aa51c54ef',
})
COMMAND = ('dockerd', '--storage-driver=vfs', '--iptables=false', '--ip-masq=false',
           '--host=unix:///run/user/1000/docker.sock')
TARGETS = frozenset({'/home/rootless/.local/share/docker', '/var/lib/docker', '/run/user/1000'})
WORKSPACE_TARGETS = TARGETS | {'/workspace'}
CGROUP_COUNTER_COMMAND = ("set -eu; cd /sys/fs/cgroup || exit 31; command -v awk >/dev/null || exit 32; "
                          "test -r pids.current || exit 33; test -r pids.max || exit 34; "
                          "test -r pids.events || exit 35; test -r memory.current || exit 36; "
                          "test -r memory.max || exit 37; test -r memory.events || exit 38; "
                          "cat pids.current pids.max; "
                          "awk '$1==\"max\" {print $2}' pids.events; cat memory.current memory.max; "
                          "for key in max oom oom_kill; do awk -v key=\"$key\" '$1==key {print $2}' memory.events; done")
CONFIG_DEFAULTS = {
    'Domainname': '', 'AttachStdin': False, 'AttachStdout': False, 'AttachStderr': False,
    'ExposedPorts': {'2375/tcp': {}, '2376/tcp': {}}, 'Tty': False,
    'OpenStdin': False, 'StdinOnce': False,
    'Volumes': {'/home/rootless/.local/share/docker': {}, '/var/lib/docker': {}},
    'WorkingDir': '/', 'OnBuild': None,
}
HOST_DEFAULTS = {
    'Binds': None, 'ContainerIDFile': '', 'LogConfig': {'Type': 'json-file', 'Config': {}},
    'PortBindings': {}, 'RestartPolicy': {'Name': 'no', 'MaximumRetryCount': 0},
    'AutoRemove': False, 'VolumeDriver': '', 'VolumesFrom': None, 'ConsoleSize': [0, 0],
    'CapAdd': None, 'CapDrop': None, 'CgroupnsMode': 'private', 'Dns': None,
    'DnsOptions': None, 'DnsSearch': None, 'ExtraHosts': None, 'GroupAdd': None,
    'IpcMode': 'private', 'Cgroup': '', 'Links': None, 'OomScoreAdj': 0,
    'PidMode': '', 'Privileged': False, 'PublishAllPorts': False, 'ReadonlyRootfs': False,
    'SecurityOpt': None, 'UTSMode': '', 'UsernsMode': '', 'ShmSize': 67108864,
    'Runtime': 'runc', 'Isolation': '', 'CpuShares': 0, 'Memory': 512 * 1024 * 1024,
    'NanoCpus': 1000000000, 'CgroupParent': '', 'BlkioWeight': 0, 'BlkioWeightDevice': None,
    'BlkioDeviceReadBps': None, 'BlkioDeviceWriteBps': None, 'BlkioDeviceReadIOps': None,
    'BlkioDeviceWriteIOps': None, 'CpuPeriod': 0, 'CpuQuota': 0, 'CpuRealtimePeriod': 0,
    'CpuRealtimeRuntime': 0, 'CpusetCpus': '', 'CpusetMems': '', 'Devices': None,
    'DeviceCgroupRules': None, 'DeviceRequests': None, 'MemoryReservation': 0,
    'MemorySwap': 1073741824, 'MemorySwappiness': None, 'OomKillDisable': False,
    'PidsLimit': 256, 'Ulimits': None, 'CpuCount': 0, 'CpuPercent': 0,
    'IOMaximumIOps': 0, 'IOMaximumBandwidth': 0,
}


def _exact_list(value, expected):
    return isinstance(value, list) and len(value) == len(expected) and set(value) == set(expected)


def _same(value, expected):
    if type(value) is not type(expected):
        return False
    if isinstance(expected, dict):
        return set(value) == set(expected) and all(_same(value[key], expected[key]) for key in expected)
    if isinstance(expected, list):
        return len(value) == len(expected) and all(_same(a, b) for a, b in zip(value, expected))
    return value == expected


def verify_selected_fields(item, identifier, name, network, volumes, labels):
    """Reject differences in the selected inspected fields only."""
    try:
        if (not isinstance(item, dict) or not isinstance(identifier, str) or
                not re.fullmatch(r'[0-9a-f]{64}', identifier) or not isinstance(name, str) or not name or
                not isinstance(network, str) or not network or not isinstance(volumes, dict) or
                set(volumes) not in (TARGETS, WORKSPACE_TARGETS) or len(set(volumes.values())) != len(volumes) or
                any(not isinstance(v, str) or not v for v in volumes.values()) or
                not isinstance(labels, dict) or not labels or
                item['Id'] != identifier or item['Name'] != '/' + name or item['Image'] != IMAGE):
            raise ValueError('rootless identity or ownership inputs differ')
        config, host = item['Config'], item['HostConfig']
        config_dynamic = {'Hostname', 'Image', 'User', 'Labels', 'Entrypoint', 'Cmd', 'Env'}
        if (not isinstance(config, dict) or not isinstance(host, dict) or
                set(config) != set(CONFIG_DEFAULTS) | config_dynamic or
                any(not _same(config[key], value) for key, value in CONFIG_DEFAULTS.items()) or
                config['Hostname'] != identifier[:12] or config['Image'] != IMAGE or
                config['User'] != 'rootless' or config['Labels'] != labels or
                config['Entrypoint'] != ['dockerd-entrypoint.sh'] or config['Cmd'] != list(COMMAND) or
                not _exact_list(config['Env'], ENV)):
            raise ValueError('rootless container configuration differs')
        dynamic = {'NetworkMode', 'Mounts', 'MaskedPaths', 'ReadonlyPaths'}
        if (set(host) - {'Sysctls'} != set(HOST_DEFAULTS) | dynamic or
                any(not _same(host[key], value) for key, value in HOST_DEFAULTS.items()) or
                host.get('Sysctls') not in (None, {}) or host['NetworkMode'] != network):
            raise ValueError('rootless host configuration differs')
        if (not _exact_list(host['MaskedPaths'], MASKED_PATHS) or
                not _exact_list(host['ReadonlyPaths'], READONLY_PATHS) or
                any(type(host.get(key)) is not int or host[key] != value for key, value in
                    (('Memory', 512 * 1024 * 1024), ('NanoCpus', 1000000000), ('PidsLimit', 256)))):
            raise ValueError('rootless resource or system path profile differs')
        configured = host['Mounts']
        mounts = item['Mounts']
        if not isinstance(configured, list) or not isinstance(mounts, list) or len(configured) != len(volumes) or len(mounts) != len(volumes):
            raise ValueError('rootless volume inventory differs')
        expected = {(target, volume) for target, volume in volumes.items()}
        config_mounts = {(entry['Target'], entry['Source']) for entry in configured
                         if isinstance(entry, dict) and set(entry) == {'Type', 'Source', 'Target'} and entry['Type'] == 'volume'}
        actual_mounts = {(entry['Destination'], entry['Name']) for entry in mounts
                         if isinstance(entry, dict) and entry.get('Type') == 'volume' and entry.get('RW') is True}
        if config_mounts != expected or actual_mounts != expected:
            raise ValueError('rootless mounts differ')
    except (KeyError, TypeError, AttributeError) as error:
        raise ValueError('invalid rootless container inspection') from error


def verify_runtime_network(item, network_id):
    """Reject published ports or attachment outside the separately owned network."""
    try:
        if (not isinstance(network_id, str) or not re.fullmatch(r'[0-9a-f]{64}', network_id) or
                item['State']['Running'] is not True):
            raise ValueError('rootless daemon is not running on an identified network')
        settings = item['NetworkSettings']
        networks, ports = settings['Networks'], settings['Ports']
        if (not isinstance(networks, dict) or len(networks) != 1 or not isinstance(ports, dict) or
                not set(ports) <= {'2375/tcp', '2376/tcp'} or
                any(bindings not in (None, []) for bindings in ports.values())):
            raise ValueError('rootless daemon has unexpected network exposure')
        endpoint = next(iter(networks.values()))
        if (not isinstance(endpoint, dict) or endpoint['NetworkID'] != network_id or
                not isinstance(endpoint['EndpointID'], str) or not re.fullmatch(r'[0-9a-f]{64}', endpoint['EndpointID'])):
            raise ValueError('rootless network attachment differs')
        if not isinstance(endpoint['IPAddress'], str):
            raise ValueError('rootless address must be a string')
        address = ipaddress.ip_address(endpoint['IPAddress'])
        if (address.version != 4 or not address.is_private or address.is_loopback or address.is_link_local or
                address.is_multicast or address.is_unspecified or address.is_reserved):
            raise ValueError('rootless network address is not private IPv4')
        return str(address)
    except (KeyError, TypeError, AttributeError) as error:
        raise ValueError('invalid rootless network inspection') from error


def verify_owned_network(item, network_id, name, labels, peers):
    """Check an inspected network snapshot; callers verify peer ownership separately.

    This cannot prevent peers attaching after inspection. Safe use requires
    exclusive daemon/network control or equivalent enforcement for the entire
    operation; a cooperative lock and repeated snapshots do not provide it.
    The caller must derive expectations from its locked journal, verify actual
    peer identities, and bind the daemon container's network endpoint and IP.
    """
    try:
        if (not isinstance(network_id, str) or not re.fullmatch(r'[0-9a-f]{64}', network_id)
                or not isinstance(name, str) or not name or not isinstance(labels, dict) or not labels
                or not isinstance(peers, dict) or not peers
                or any(not isinstance(key, str) or not re.fullmatch(r'[0-9a-f]{64}', key)
                       or not isinstance(value, str) or not value for key, value in peers.items())
                or len(set(peers.values())) != len(peers)
                or item['Id'] != network_id or item['Name'] != name or item['Driver'] != 'bridge'
                or item['Scope'] != 'local' or item['Internal'] is not True or item['Labels'] != labels):
            raise ValueError('owned network identity differs')
        attached = item['Containers']
        if not isinstance(attached, dict) or set(attached) != set(peers):
            raise ValueError('owned network peer inventory differs')
        for identifier, expected_name in peers.items():
            endpoint = attached[identifier]
            if (not isinstance(endpoint, dict) or endpoint.get('Name') != expected_name
                    or not isinstance(endpoint.get('EndpointID'), str)
                    or not re.fullmatch(r'[0-9a-f]{64}', endpoint['EndpointID'])):
                raise ValueError('owned network peer identity differs')
    except (KeyError, TypeError, AttributeError) as error:
        raise ValueError('invalid owned network inspection') from error
