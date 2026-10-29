"""Journalled, run-owned local Docker containers, networks and volumes.

This primitive does not intercept Testcontainers or Compose and must not be used
as evidence that those workloads are isolated. Uncertain operations fail closed.
"""
import json
import os
from pathlib import Path
import posixpath
import re
import stat
import subprocess
import uuid

from local_heavy_slots import _try_lock


class OwnershipError(Exception):
    pass


class DockerCLI:
    def __init__(self):
        self._env = dict(os.environ)
        if not self._env.get('DOCKER_HOST') and not self._env.get('DOCKER_CONTEXT'):
            context = self._call('context', 'show')
            if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.-]{0,127}', context):
                raise OwnershipError('Docker context identity is invalid')
            self._env['DOCKER_CONTEXT'] = context

    def _call(self, *args):
        try:
            result = subprocess.run(['docker', *args], env=self._env, capture_output=True, timeout=60, check=False)
        except (OSError, subprocess.TimeoutExpired):
            raise OwnershipError('Docker operation outcome uncertain') from None
        if result.returncode:
            raise OwnershipError('Docker operation failed or outcome uncertain')
        return result.stdout.decode('utf-8').strip()

    def daemon(self):
        fields = self._call('info', '--format', '{{.ID}} {{.OSType}}').split()
        if len(fields) != 2 or fields[1] != 'linux':
            raise OwnershipError('a Linux Docker daemon identity is required')
        return fields[0]

    def create(self, kind, name, labels):
        options = [part for key, value in sorted(labels.items()) for part in ('--label', key + '=' + value)]
        if kind == 'network':
            return self._call('network', 'create', '--driver', 'bridge', '--internal', *options, name)
        if kind == 'volume':
            return self._call('volume', 'create', '--driver', 'local', *options, name)
        raise OwnershipError('unsupported Docker resource')

    def inspect(self, kind, name):
        try:
            rows = json.loads(self._call(kind, 'inspect', name))
            if not isinstance(rows, list) or len(rows) != 1:
                raise ValueError()
            value = rows[0]
            if kind == 'network':
                if value.get('Driver') != 'bridge' or value.get('Scope') != 'local' or value.get('Internal') is not True:
                    raise ValueError()
                identifier = value['Id']
            elif kind == 'volume':
                if value.get('Driver') != 'local':
                    raise ValueError()
                identifier = value['Name']
            elif kind == 'container':
                identifier = value['Id']
                host = value['HostConfig']
                if host.get('Privileged') or host.get('Binds'):
                    raise ValueError()
                mounts = {}
                for mount in value.get('Mounts', []):
                    if mount.get('Type') != 'volume' or mount.get('Destination') in mounts:
                        raise ValueError()
                    mounts[mount['Destination']] = mount['Name']
                bindings = host.get('PortBindings') or {}
                if bindings:
                    raise ValueError()
                running = value.get('State', {}).get('Running') is True
                if running:
                    actual = value.get('NetworkSettings', {}).get('Ports') or {}
                    if not isinstance(actual, dict) or any(entries not in (None, []) for entries in actual.values()):
                        raise ValueError()
                return {'id': identifier, 'name': value['Name'].removeprefix('/'), 'kind': kind,
                        'labels': value['Config'].get('Labels') or {}, 'image': value['Image'],
                        'network': host['NetworkMode'], 'mounts': mounts, 'ports': [], 'running': running}
            else:
                raise ValueError()
            return {'id': identifier, 'name': value['Name'], 'labels': value.get('Labels') or {}, 'kind': kind,
                    'internal': kind == 'network'}
        except (ValueError, KeyError, TypeError, AttributeError):
            raise OwnershipError('Docker resource inspection was invalid') from None

    def remove(self, kind, identifier):
        self._call(kind, 'rm', *(['-f'] if kind == 'container' else []), identifier)

    def exists(self, kind, identifier):
        if kind == 'network':
            rows = self._call('network', 'ls', '--no-trunc', '--format', '{{.ID}}').splitlines()
        elif kind == 'volume':
            rows = self._call('volume', 'ls', '--format', '{{.Name}}').splitlines()
        elif kind == 'container':
            rows = self._call('container', 'ls', '-a', '--no-trunc', '--format', '{{.ID}}').splitlines()
        else:
            raise OwnershipError('unsupported Docker resource')
        return identifier in rows

    def image(self, identifier):
        try:
            rows = json.loads(self._call('image', 'inspect', identifier))
            if not isinstance(rows, list) or len(rows) != 1:
                raise ValueError()
            image = rows[0]
            volumes = image['Config'].get('Volumes') or {}
            if image.get('Os') != 'linux' or image.get('Id') != identifier or not isinstance(volumes, dict):
                raise ValueError()
            return {'id': identifier, 'volumes': sorted(volumes)}
        except (ValueError, KeyError, TypeError, AttributeError):
            raise OwnershipError('Docker image inspection was invalid') from None

    def create_container(self, name, labels, image_id, network_id, mounts, ports):
        options = [part for key, value in sorted(labels.items()) for part in ('--label', key + '=' + value)]
        for target, volume in sorted(mounts.items()):
            options.extend(('--mount', 'type=volume,source=' + volume + ',target=' + target))
        return self._call('container', 'create', '--name', name, '--network', network_id, *options, image_id)

    def start(self, identifier):
        self._call('container', 'start', identifier)


class OwnedRun:
    @classmethod
    def create(cls, checkout, engine):
        checkout = Path(checkout).resolve()
        if not checkout.is_dir():
            raise OwnershipError('checkout is not a directory')
        base = checkout / 'tmp' / 'ci-local'
        base.mkdir(parents=True, exist_ok=True)
        if base.resolve() != base:
            raise OwnershipError('run directory must not traverse a symlink')
        directory = base / ('voiceci-' + uuid.uuid4().hex)
        directory.mkdir(mode=0o700)
        instance = cls(directory, engine)
        try:
            daemon = engine.daemon()
            if not isinstance(daemon, str) or not re.fullmatch(r'[A-Za-z0-9_.:-]{1,128}', daemon):
                raise OwnershipError('invalid Docker daemon identity')
            instance.state = {'version': 1, 'run_id': directory.name, 'daemon': daemon, 'resources': []}
            instance._save()
            return instance
        except BaseException:
            instance.close()
            raise

    @classmethod
    def load(cls, directory, engine):
        instance = cls(Path(directory), engine)
        try:
            journal = instance.directory / 'ownership.json'
            if journal.is_symlink() or not journal.is_file():
                raise OwnershipError('ownership journal is not a regular file')
            instance.state = json.loads(journal.read_text(encoding='utf-8'))
            instance._validate()
            instance._check_daemon()
            return instance
        except BaseException:
            instance.close()
            raise

    def __init__(self, directory, engine):
        directory = Path(directory)
        if (not directory.is_absolute() or directory.resolve() != directory or not directory.is_dir()
                or not re.fullmatch(r'voiceci-[0-9a-f]{32}', directory.name)
                or directory.parent.name != 'ci-local' or directory.parent.parent.name != 'tmp'):
            raise OwnershipError('invalid owned run directory')
        self.directory = directory
        self.engine = engine
        self.state = None
        flags = os.O_RDWR | os.O_CREAT | getattr(os, 'O_BINARY', 0) | getattr(os, 'O_NOFOLLOW', 0)
        lock_path = directory / 'ownership.lock'
        if lock_path.is_symlink():
            raise OwnershipError('ownership lock must not be a symlink')
        descriptor = os.open(lock_path, flags, 0o600)
        try:
            if lock_path.is_symlink() or not stat.S_ISREG(os.fstat(descriptor).st_mode):
                raise OwnershipError('ownership lock is not a regular file')
            if os.fstat(descriptor).st_size == 0:
                os.write(descriptor, b'\0')
            if not _try_lock(descriptor):
                raise OwnershipError('run ownership is already held')
        except BaseException:
            os.close(descriptor)
            raise
        self._lock = descriptor

    @property
    def run_id(self):
        return self.state['run_id']

    def close(self):
        if self._lock is not None:
            descriptor, self._lock = self._lock, None
            os.close(descriptor)

    def _validate(self):
        state = self.state
        if (not isinstance(state, dict) or set(state) != {'version', 'run_id', 'daemon', 'resources'}
                or type(state['version']) is not int or state['version'] != 1
                or state['run_id'] != self.directory.name or not isinstance(state['daemon'], str)
                or not re.fullmatch(r'[A-Za-z0-9_.:-]{1,128}', state['daemon'])
                or not isinstance(state['resources'], list)):
            raise OwnershipError('invalid ownership journal')
        names = set()
        for item in state['resources']:
            if not isinstance(item, dict):
                raise OwnershipError('invalid ownership record')
            kind = item.get('kind')
            keys = {'kind', 'name', 'nonce', 'id', 'state'}
            if kind == 'container':
                keys |= {'image', 'network', 'mounts', 'ports'}
            if (set(item) != keys
                    or kind not in ('network', 'volume', 'container') or item['state'] not in ('creating', 'starting', 'ready', 'removing', 'removed')
                    or not isinstance(item['nonce'], str) or not re.fullmatch(r'[0-9a-f]{32}', item['nonce'])
                    or item['name'] != self.run_id + '-' + item['kind'] + '-' + item['nonce']
                    or item['name'] in names or not isinstance(item['id'], str)):
                raise OwnershipError('invalid ownership record')
            if item['state'] == 'starting' and kind != 'container':
                raise OwnershipError('invalid resource state')
            if item['state'] == 'creating':
                if item['id']:
                    raise OwnershipError('invalid creating resource identity')
            elif ((item['kind'] in ('network', 'container') and not re.fullmatch(r'[0-9a-f]{64}', item['id']))
                  or (item['kind'] == 'volume' and item['id'] != item['name'])):
                raise OwnershipError('invalid resource identity')
            if kind == 'container':
                if (not isinstance(item['image'], str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', item['image'])
                        or not isinstance(item['network'], str) or not re.fullmatch(r'[0-9a-f]{64}', item['network'])
                        or not isinstance(item['mounts'], dict) or not isinstance(item['ports'], list)):
                    raise OwnershipError('invalid container specification')
                self._validate_mounts_ports(item['mounts'], item['ports'])
            names.add(item['name'])
        networks = {item['id'] for item in state['resources'] if item['kind'] == 'network' and item['id']}
        volumes = {item['name'] for item in state['resources'] if item['kind'] == 'volume'}
        for item in state['resources']:
            if item['kind'] == 'container' and (item['network'] not in networks or
                                                any(name not in volumes for name in item['mounts'].values())):
                raise OwnershipError('container references a foreign resource')

    @staticmethod
    def _validate_mounts_ports(mounts, ports):
        for target, volume in mounts.items():
            if (not isinstance(target, str) or not re.fullmatch(r'/[A-Za-z0-9_./-]+', target)
                    or target == '/' or posixpath.normpath(target) != target or not isinstance(volume, str)):
                raise OwnershipError('invalid container mount')
        targets = sorted(mounts)
        if any(b.startswith(a.rstrip('/') + '/') for i, a in enumerate(targets) for b in targets[i + 1:]):
            raise OwnershipError('overlapping container mounts')
        if ports:
            raise OwnershipError('host ports are unavailable for internal-only containers')

    def _save(self):
        self._validate()
        temporary = self.directory / ('ownership-' + uuid.uuid4().hex + '.tmp')
        created = False
        try:
            with temporary.open('x', encoding='utf-8', newline='\n') as output:
                created = True
                json.dump(self.state, output, sort_keys=True)
                output.write('\n')
                output.flush()
                os.fsync(output.fileno())
            os.replace(temporary, self.directory / 'ownership.json')
        finally:
            if created and temporary.exists():
                temporary.unlink()

    def _check_daemon(self):
        if self._lock is None or self.engine.daemon() != self.state['daemon']:
            raise OwnershipError('run lock or Docker daemon identity changed')

    def _inspect(self, item):
        observed = self.engine.inspect(item['kind'], item['name'])
        labels = {'voice.ci.run': self.run_id, 'voice.ci.resource': item['nonce'], 'voice.ci.kind': item['kind']}
        if (not isinstance(observed, dict) or observed.get('kind') != item['kind']
                or observed.get('name') != item['name'] or observed.get('id') != item['id']
                or not isinstance(observed.get('labels'), dict)
                or any(observed['labels'].get(key) != value for key, value in labels.items())):
            raise OwnershipError('Docker resource ownership changed')
        if item['kind'] == 'network' and observed.get('internal') is not True:
            raise OwnershipError('owned network is not internal')
        if item['kind'] == 'container' and any(observed.get(key) != item[key]
                                               for key in ('image', 'network', 'mounts', 'ports')):
            raise OwnershipError('Docker container specification changed')
        return observed

    def create_resource(self, kind):
        self._validate()
        self._check_daemon()
        if kind not in ('network', 'volume') or any(r['state'] in ('creating', 'starting', 'removing') for r in self.state['resources']):
            raise OwnershipError('unsupported resource or unresolved operation')
        nonce = uuid.uuid4().hex
        item = {'kind': kind, 'name': self.run_id + '-' + kind + '-' + nonce,
                'nonce': nonce, 'id': '', 'state': 'creating'}
        self.state['resources'].append(item)
        self._save()
        labels = {'voice.ci.run': self.run_id, 'voice.ci.resource': nonce, 'voice.ci.kind': kind}
        identifier = self.engine.create(kind, item['name'], labels)
        self._check_daemon()
        if (not isinstance(identifier, str) or
                (kind == 'network' and not re.fullmatch(r'[0-9a-f]{64}', identifier))
                or (kind == 'volume' and identifier != item['name'])):
            raise OwnershipError('Docker creation identity is invalid')
        item['id'] = identifier
        self._inspect(item)
        item['state'] = 'ready'
        self._save()
        return item['name']

    def create_container(self, image_id, network_name, mounts, ports):
        self._validate()
        self._check_daemon()
        if (not isinstance(image_id, str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id)
                or not isinstance(mounts, dict) or not isinstance(ports, list)
                or any(r['state'] in ('creating', 'starting', 'removing') for r in self.state['resources'])):
            raise OwnershipError('invalid image or unresolved operation')
        self._validate_mounts_ports(mounts, ports)
        networks = [r for r in self.state['resources'] if r['kind'] == 'network' and r['name'] == network_name and r['state'] == 'ready']
        volumes = {r['name']: r for r in self.state['resources'] if r['kind'] == 'volume' and r['state'] == 'ready'}
        if len(networks) != 1 or any(name not in volumes for name in mounts.values()):
            raise OwnershipError('container network or volumes are not owned by this run')
        image = self.engine.image(image_id)
        if (not isinstance(image, dict) or image.get('id') != image_id or not isinstance(image.get('volumes'), list)
                or set(image['volumes']) != set(mounts)):
            raise OwnershipError('all declared image volumes must be owned and mounted')
        self._inspect(networks[0])
        for name in mounts.values():
            self._inspect(volumes[name])
        nonce = uuid.uuid4().hex
        item = {'kind': 'container', 'name': self.run_id + '-container-' + nonce, 'nonce': nonce,
                'id': '', 'state': 'creating', 'image': image_id, 'network': networks[0]['id'],
                'mounts': dict(mounts), 'ports': sorted(ports)}
        self.state['resources'].append(item)
        self._save()
        labels = {'voice.ci.run': self.run_id, 'voice.ci.resource': nonce, 'voice.ci.kind': 'container'}
        identifier = self.engine.create_container(item['name'], labels, image_id, item['network'], mounts, item['ports'])
        self._check_daemon()
        if not isinstance(identifier, str) or not re.fullmatch(r'[0-9a-f]{64}', identifier):
            raise OwnershipError('Docker container identity is invalid')
        item['id'] = identifier
        self._inspect(item)
        item['state'] = 'ready'
        self._save()
        return item['name']

    def start_container(self, name):
        self._validate()
        self._check_daemon()
        matches = [r for r in self.state['resources'] if r['kind'] == 'container' and r['name'] == name and r['state'] == 'ready']
        if len(matches) != 1 or any(r['state'] in ('creating', 'starting', 'removing') for r in self.state['resources']):
            raise OwnershipError('container is not ready or an operation is unresolved')
        item = matches[0]
        self._inspect(item)
        item['state'] = 'starting'
        self._save()
        self.engine.start(item['id'])
        self._check_daemon()
        if not self._inspect(item).get('running'):
            raise OwnershipError('container did not start')
        item['state'] = 'ready'
        self._save()

    def cleanup(self):
        self._validate()
        self._check_daemon()
        if any(item['state'] in ('creating', 'starting', 'removing') for item in self.state['resources']):
            raise OwnershipError('uncertain operation requires reconciliation')
        order = {'container': 0, 'volume': 1, 'network': 2}
        ready = sorted((item for item in reversed(self.state['resources']) if item['state'] == 'ready'),
                       key=lambda item: order[item['kind']])
        for item in ready:
            self._inspect(item)
        for item in ready:
            self._check_daemon()
            self._inspect(item)
            item['state'] = 'removing'
            self._save()
            self.engine.remove(item['kind'], item['id'])
            self._check_daemon()
            if self.engine.exists(item['kind'], item['id']):
                raise OwnershipError('resource still exists after removal')
            item['state'] = 'removed'
            self._save()
