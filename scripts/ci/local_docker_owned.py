"""Journalled, run-owned local Docker networks and volumes.

This primitive does not intercept Testcontainers or Compose and must not be used
as evidence that those workloads are isolated. Uncertain operations fail closed.
"""
import json
import os
from pathlib import Path
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
            return self._call('network', 'create', '--driver', 'bridge', *options, name)
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
                if value.get('Driver') != 'bridge' or value.get('Scope') != 'local':
                    raise ValueError()
                identifier = value['Id']
            elif kind == 'volume':
                if value.get('Driver') != 'local':
                    raise ValueError()
                identifier = value['Name']
            else:
                raise ValueError()
            return {'id': identifier, 'name': value['Name'], 'labels': value.get('Labels') or {}, 'kind': kind}
        except (ValueError, KeyError, TypeError, AttributeError):
            raise OwnershipError('Docker resource inspection was invalid') from None

    def remove(self, kind, identifier):
        self._call(kind, 'rm', identifier)

    def exists(self, kind, identifier):
        if kind == 'network':
            rows = self._call('network', 'ls', '--no-trunc', '--format', '{{.ID}}').splitlines()
        elif kind == 'volume':
            rows = self._call('volume', 'ls', '--format', '{{.Name}}').splitlines()
        else:
            raise OwnershipError('unsupported Docker resource')
        return identifier in rows


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
            if (not isinstance(item, dict) or set(item) != {'kind', 'name', 'nonce', 'id', 'state'}
                    or item['kind'] not in ('network', 'volume') or item['state'] not in ('creating', 'ready', 'removing', 'removed')
                    or not isinstance(item['nonce'], str) or not re.fullmatch(r'[0-9a-f]{32}', item['nonce'])
                    or item['name'] != self.run_id + '-' + item['kind'] + '-' + item['nonce']
                    or item['name'] in names or not isinstance(item['id'], str)):
                raise OwnershipError('invalid ownership record')
            if item['state'] == 'creating':
                if item['id']:
                    raise OwnershipError('invalid creating resource identity')
            elif ((item['kind'] == 'network' and not re.fullmatch(r'[0-9a-f]{64}', item['id']))
                  or (item['kind'] == 'volume' and item['id'] != item['name'])):
                raise OwnershipError('invalid resource identity')
            names.add(item['name'])

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

    def create_resource(self, kind):
        self._validate()
        self._check_daemon()
        if kind not in ('network', 'volume') or any(r['state'] in ('creating', 'removing') for r in self.state['resources']):
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

    def cleanup(self):
        self._validate()
        self._check_daemon()
        if any(item['state'] in ('creating', 'removing') for item in self.state['resources']):
            raise OwnershipError('uncertain operation requires reconciliation')
        ready = [item for item in reversed(self.state['resources']) if item['state'] == 'ready']
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
