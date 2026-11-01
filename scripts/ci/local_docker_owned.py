"""Journalled, run-owned local Docker containers, networks and volumes.

This primitive does not intercept Testcontainers or Compose and must not be used
as evidence that those workloads are isolated. Uncertain operations fail closed.
"""
import hashlib
import json
import os
from pathlib import Path
import posixpath
import re
import stat
import subprocess
import sys
import tempfile
import threading
import time
import uuid

from local_heavy_slots import _try_lock
import local_rootless_profile as rootless
import local_image_archive

ROOTLESS_PROFILES = ('rootless-unix-v1', 'rootless-workspace-v1')


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

    def rootless_creator_env(self):
        """Resolve one local endpoint; the creator must still verify daemon ID."""
        env = dict(self._env)
        if any(env.get(key) for key in ('DOCKER_TLS_VERIFY', 'DOCKER_CERT_PATH', 'DOCKER_API_VERSION')):
            raise OwnershipError('Docker TLS or API override is unavailable for rootless creation')
        host, context = env.get('DOCKER_HOST', ''), env.get('DOCKER_CONTEXT', '')
        if bool(host) == bool(context):
            raise OwnershipError('exactly one Docker endpoint or context is required')
        if context:
            if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.-]{0,127}', context):
                raise OwnershipError('Docker context identity is invalid')
            try:
                rows = json.loads(self._call('context', 'inspect', context))
                if not isinstance(rows, list) or len(rows) != 1 or rows[0]['Name'] != context:
                    raise ValueError()
                endpoint = rows[0]['Endpoints']['docker']
                if endpoint.get('SkipTLSVerify', False) is not False or rows[0].get('TLSMaterial') not in (None, {}):
                    raise ValueError()
                host = endpoint['Host']
            except (ValueError, KeyError, TypeError, AttributeError):
                raise OwnershipError('Docker context endpoint is invalid') from None
        pipes = ('npipe:////./pipe/dockerDesktopLinuxEngine', 'npipe:////./pipe/docker_engine')
        unix = isinstance(host, str) and re.fullmatch(r'unix:///[A-Za-z0-9._/-]+', host)
        if unix:
            path = host[len('unix://'):]
            unix = posixpath.normpath(path) == path and path != '/'
        if host not in pipes and not unix:
            raise OwnershipError('rootless creator requires an explicit local Docker socket')
        for key in ('DOCKER_CONTEXT', 'DOCKER_TLS_VERIFY', 'DOCKER_CERT_PATH', 'DOCKER_API_VERSION'):
            env.pop(key, None)
        env['DOCKER_HOST'] = host
        return env

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
                readonly_mounts = []
                for mount in value.get('Mounts', []):
                    if mount.get('Type') != 'volume' or mount.get('Destination') in mounts:
                        raise ValueError()
                    mounts[mount['Destination']] = mount['Name']
                    if mount.get('RW') is False:
                        readonly_mounts.append(mount['Destination'])
                bindings = host.get('PortBindings') or {}
                if bindings:
                    raise ValueError()
                running = value.get('State', {}).get('Running')
                if type(running) is not bool:
                    raise ValueError()
                networks = value.get('NetworkSettings', {}).get('Networks') or {}
                if not isinstance(networks, dict):
                    raise ValueError()
                if running:
                    actual = value.get('NetworkSettings', {}).get('Ports') or {}
                    if not isinstance(actual, dict) or any(entries not in (None, []) for entries in actual.values()):
                        raise ValueError()
                return {'id': identifier, 'name': value['Name'].removeprefix('/'), 'kind': kind,
                        'labels': value['Config'].get('Labels') or {}, 'image': value['Image'],
                         'network': host['NetworkMode'], 'mounts': mounts, 'readonly_mounts': sorted(readonly_mounts), 'ports': [], 'running': running,
                         'command': value['Config'].get('Cmd'), 'entrypoint': value['Config'].get('Entrypoint') or [],
                         'user': value['Config'].get('User') or '', 'network_attachments': sorted(networks)}
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
            return {'id': identifier, 'volumes': sorted(volumes), 'entrypoint': image['Config'].get('Entrypoint') or []}
        except (ValueError, KeyError, TypeError, AttributeError):
            raise OwnershipError('Docker image inspection was invalid') from None

    def verify_rootless(self, item, labels, running):
        try:
            rows = json.loads(self._call('container', 'inspect', item['id']))
            if not isinstance(rows, list) or len(rows) != 1:
                raise ValueError('invalid inspection inventory')
            raw = rows[0]
            rootless.verify_selected_fields(raw, item['id'], item['name'], item['network'], item['mounts'], labels)
            if raw['State']['Running'] is not running:
                raise ValueError('container state changed during inspection')
            if running:
                rootless.verify_runtime_network(raw, item['network'])
        except (ValueError, KeyError, TypeError, AttributeError):
            raise OwnershipError('rootless container profile differs') from None

    def create_container(self, name, labels, image_id, network_id, mounts, ports, command=None,
                         clear_entrypoint=False):
        options = [part for key, value in sorted(labels.items()) for part in ('--label', key + '=' + value)]
        if clear_entrypoint:
            options.extend(('--entrypoint', ''))
        for target, volume in sorted(mounts.items()):
            options.extend(('--mount', 'type=volume,source=' + volume + ',target=' + target))
        return self._call('container', 'create', '--name', name, '--network', network_id, *options,
                          image_id, *(command or []))

    def create_phase0_container(self, name, labels, image_id, source_volume, command):
        options = [part for key, value in sorted(labels.items()) for part in ('--label', key + '=' + value)]
        return self._call('container', 'create', '--name', name, '--network', 'none',
                          '--entrypoint', '', *options, '--mount',
                          'type=volume,source=' + source_volume + ',target=/workspace/repo,readonly',
                          image_id, *command)

    def start(self, identifier):
        self._call('container', 'start', identifier)

    def stop(self, identifier):
        self._call('container', 'stop', '--time', '5', identifier)

    def execute(self, identifier, command, stdout, stderr, timeout, stdin=None):
        try:
            result = subprocess.run(['docker', 'container', 'exec', *(['--interactive'] if stdin is not None else []), identifier, *command],
                                    env=self._env, stdin=stdin, stdout=stdout, stderr=stderr,
                                    timeout=timeout, check=False)
        except (OSError, subprocess.TimeoutExpired):
            raise OwnershipError('Docker execution outcome uncertain') from None
        if result.returncode < 0 or result.returncode == 125:
            raise OwnershipError('Docker execution outcome uncertain')
        return result.returncode

    def execute_verified_stream(self, identifier, command, stdout, stderr, timeout, stdin, expected):
        """Hash the exact bytes sent to the owned Docker exec process."""
        expired = threading.Event()
        process = None
        def expire():
            expired.set()
            try:
                if process.poll() is None:
                    process.kill()
            except OSError:
                pass
        try:
            watchdog = threading.Timer(timeout, expire)
            watchdog.daemon = True
        except Exception:
            raise OwnershipError('Docker watchdog unavailable') from None
        deadline = time.monotonic() + timeout
        watchdog_started = False
        close_failed = False
        try:
            process = subprocess.Popen(['docker', 'container', 'exec', '--interactive', identifier, *command],
                                       env=self._env, stdin=subprocess.PIPE, stdout=stdout, stderr=stderr, bufsize=0)
            watchdog.start()
            watchdog_started = True
            digest = hashlib.sha256()
            for block in iter(lambda: stdin.read(1024 * 1024), b''):
                view = memoryview(block)
                while view:
                    written = process.stdin.write(view)
                    if not written:
                        raise OSError('short Docker input write')
                    digest.update(view[:written])
                    view = view[written:]
            process.stdin.close()
            result = process.wait(timeout=max(.001, deadline - time.monotonic()))
        except BaseException as error:
            if process is not None:
                try:
                    if process.poll() is None:
                        process.kill()
                    process.wait(timeout=30)
                except (OSError, subprocess.TimeoutExpired):
                    raise OwnershipError('Docker client termination is uncertain') from None
            if not isinstance(error, Exception):
                raise
            raise OwnershipError('Docker execution outcome uncertain') from None
        finally:
            watchdog.cancel()
            if watchdog_started:
                watchdog.join(timeout=5)
            if process is not None and not process.stdin.closed:
                try:
                    process.stdin.close()
                except OSError:
                    close_failed = True
        if (close_failed or watchdog.is_alive() or expired.is_set() or digest.hexdigest() != expected or result < 0 or result == 125):
            raise OwnershipError('Docker image stream outcome uncertain')
        return result


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
                if 'command' in item:
                    keys.add('command')
                if 'profile' in item:
                    keys.add('profile')
                if 'readonly_mounts' in item:
                    keys.add('readonly_mounts')
                if 'image_operation' in item:
                    keys.add('image_operation')
            if (set(item) != keys
                    or kind not in ('network', 'volume', 'container') or item['state'] not in ('creating', 'starting', 'executing', 'stopping', 'ready', 'quarantined', 'removing', 'removed')
                    or not isinstance(item['nonce'], str) or not re.fullmatch(r'[0-9a-f]{32}', item['nonce'])
                    or item['name'] != self.run_id + '-' + item['kind'] + '-' + item['nonce']
                    or item['name'] in names or not isinstance(item['id'], str)):
                raise OwnershipError('invalid ownership record')
            if item['state'] in ('starting', 'executing', 'stopping') and kind != 'container':
                raise OwnershipError('invalid resource state')
            if item['state'] == 'quarantined' and (kind not in ('container', 'volume') or
                    (kind == 'container' and item.get('profile') not in ROOTLESS_PROFILES)):
                raise OwnershipError('invalid quarantined resource')
            if item['state'] == 'creating':
                if item['id']:
                    raise OwnershipError('invalid creating resource identity')
            elif ((item['kind'] in ('network', 'container') and not re.fullmatch(r'[0-9a-f]{64}', item['id']))
                  or (item['kind'] == 'volume' and item['id'] != item['name'])):
                raise OwnershipError('invalid resource identity')
            if kind == 'container':
                if (not isinstance(item['image'], str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', item['image'])
                        or not isinstance(item['network'], str) or not (re.fullmatch(r'[0-9a-f]{64}', item['network']) or (item.get('profile') == 'phase0-isolated-v1' and item['network'] == 'none'))
                        or not isinstance(item['mounts'], dict) or not isinstance(item['ports'], list)):
                    raise OwnershipError('invalid container specification')
                self._validate_mounts_ports(item['mounts'], item['ports'])
                if 'command' in item:
                    self._validate_command(item['command'])
                if item.get('profile') == 'rootless-unix-v1' and ('command' in item
                        or item['image'] != rootless.IMAGE or set(item['mounts']) != rootless.TARGETS
                        or len(set(item['mounts'].values())) != 3 or item['ports']):
                    raise OwnershipError('invalid rootless container profile')
                if item.get('profile') == 'rootless-workspace-v1' and ('command' in item
                        or item['image'] != rootless.IMAGE or set(item['mounts']) != rootless.WORKSPACE_TARGETS
                        or len(set(item['mounts'].values())) != 4 or item['ports']):
                    raise OwnershipError('invalid rootless workspace profile')
                if 'image_operation' in item:
                    operation = item['image_operation']
                    tag = r'[A-Za-z0-9][A-Za-z0-9._:/-]*:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}'
                    if (item.get('profile') not in ROOTLESS_PROFILES or item['state'] not in
                            ('executing', 'stopping', 'quarantined', 'removing', 'removed') or
                            not isinstance(operation, dict) or set(operation) !=
                            {'kind', 'archive_sha256', 'image_id', 'source_tag', 'target_tag'} or
                            operation['kind'] != 'alias-load-v1' or
                            not isinstance(operation['archive_sha256'], str) or not re.fullmatch(r'[0-9a-f]{64}', operation['archive_sha256']) or
                            not isinstance(operation['image_id'], str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', operation['image_id']) or
                            any(not isinstance(operation[k], str) or not re.fullmatch(tag, operation[k]) for k in ('source_tag', 'target_tag')) or
                            operation['source_tag'] == operation['target_tag']):
                        raise OwnershipError('invalid rootless image operation')
                if item.get('profile') == 'phase0-isolated-v1' and (item['network'] != 'none' or
                        set(item['mounts']) != {'/workspace/repo'} or item.get('readonly_mounts') != ['/workspace/repo'] or
                        item.get('command') != ['sleep', 'infinity'] or item['ports']):
                    raise OwnershipError('invalid phase0 container profile')
                if ('profile' in item and item['profile'] not in (*ROOTLESS_PROFILES, 'phase0-isolated-v1') or
                        ('readonly_mounts' in item and item.get('profile') != 'phase0-isolated-v1')):
                    raise OwnershipError('invalid container profile')
            names.add(item['name'])
        networks = {item['id'] for item in state['resources'] if item['kind'] == 'network' and item['id']}
        volumes = {item['name'] for item in state['resources'] if item['kind'] == 'volume'}
        for item in state['resources']:
            if item['kind'] == 'container' and ((item['network'] not in networks and item.get('profile') != 'phase0-isolated-v1') or
                                                any(name not in volumes for name in item['mounts'].values())):
                raise OwnershipError('container references a foreign resource')
            if item['kind'] == 'volume' and item['state'] == 'quarantined' and not any(
                    r.get('profile') in ROOTLESS_PROFILES and r['state'] in ('quarantined', 'removing', 'removed')
                    and item['name'] in r['mounts'].values() for r in state['resources']):
                raise OwnershipError('quarantined volume has no matching daemon')

    @staticmethod
    def _validate_command(command):
        if (type(command) is not list or not 1 <= len(command) <= 64 or
                any(type(part) is not str or not part or len(part) > 4096 or
                    any(ord(ch) < 32 or ord(ch) == 127 for ch in part) for part in command)):
            raise OwnershipError('invalid container command')

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
        if item.get('profile') == 'phase0-isolated-v1' and observed.get('readonly_mounts') != item['readonly_mounts']:
            raise OwnershipError('phase0 source mount is not read-only')
        if item.get('profile') == 'phase0-isolated-v1' and (observed.get('user') not in ('', '0', '0:0') or
                observed.get('network_attachments') not in ([], ['none'])):
            raise OwnershipError('phase0 user or network attachments differ')
        if item['kind'] == 'container' and 'command' in item and (
                observed.get('command') != item['command'] or observed.get('entrypoint') != []):
            raise OwnershipError('Docker container command changed')
        if item['kind'] == 'container' and item.get('profile') in ROOTLESS_PROFILES:
            self.engine.verify_rootless(item, labels, observed.get('running'))
        return observed

    def reconcile_creation(self, name):
        """Adopt only an already visible, exact owned resource after a lost response.

        Absence is intentionally not considered proof that an in-flight create
        cannot still complete; callers must retain the journal and slot then.
        """
        self._validate()
        self._check_daemon()
        matches = [item for item in self.state['resources'] if item['name'] == name and item['state'] == 'creating']
        if len(matches) != 1 or sum(item['state'] in ('creating', 'starting', 'executing', 'stopping', 'removing')
                                    for item in self.state['resources']) != 1:
            raise OwnershipError('creation is not uniquely reconcilable')
        item = matches[0]
        try:
            observed = self.engine.inspect(item['kind'], item['name'])
        except (KeyError, OSError, ValueError, TypeError) as error:
            raise OwnershipError('created resource is unavailable or uncertain') from error
        identifier = observed.get('id') if isinstance(observed, dict) else None
        if (not isinstance(identifier, str) or
                (item['kind'] in ('network', 'container') and not re.fullmatch(r'[0-9a-f]{64}', identifier)) or
                (item['kind'] == 'volume' and identifier != item['name'])):
            raise OwnershipError('created resource identity is invalid')
        candidate = dict(item, id=identifier, state='ready')
        self._inspect(candidate)
        self._check_daemon()
        item['id'], item['state'] = identifier, 'ready'
        self._save()

    def create_resource(self, kind):
        self._validate()
        self._check_daemon()
        if kind not in ('network', 'volume') or any(r['state'] in ('creating', 'starting', 'executing', 'stopping', 'removing') for r in self.state['resources']):
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
        self._inspect(dict(item, id=identifier))
        item['id'], item['state'] = identifier, 'ready'
        self._save()
        return item['name']

    def create_phase0_container(self, image_id, source_volume):
        """Create an owned network-none client with one read-only source volume."""
        self._validate()
        self._check_daemon()
        if (not isinstance(image_id, str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id) or
                any(r['state'] in ('creating', 'starting', 'executing', 'stopping', 'removing') for r in self.state['resources'])):
            raise OwnershipError('invalid phase0 image or unresolved operation')
        volumes = [r for r in self.state['resources'] if r['kind'] == 'volume' and r['name'] == source_volume and r['state'] == 'ready']
        if len(volumes) != 1:
            raise OwnershipError('phase0 source volume is not owned')
        image = self.engine.image(image_id)
        if not isinstance(image, dict) or image.get('id') != image_id or image.get('volumes') != []:
            raise OwnershipError('phase0 image identity or volumes differ')
        self._inspect(volumes[0])
        nonce = uuid.uuid4().hex
        item = {'kind': 'container', 'name': self.run_id + '-container-' + nonce, 'nonce': nonce,
                'id': '', 'state': 'creating', 'image': image_id, 'network': 'none',
                'mounts': {'/workspace/repo': source_volume}, 'readonly_mounts': ['/workspace/repo'],
                'ports': [], 'command': ['sleep', 'infinity'], 'profile': 'phase0-isolated-v1'}
        self.state['resources'].append(item)
        self._save()
        labels = {'voice.ci.run': self.run_id, 'voice.ci.resource': nonce, 'voice.ci.kind': 'container'}
        identifier = self.engine.create_phase0_container(item['name'], labels, image_id, source_volume, item['command'])
        self._check_daemon()
        if not isinstance(identifier, str) or not re.fullmatch(r'[0-9a-f]{64}', identifier):
            raise OwnershipError('phase0 container identity is invalid')
        self._inspect(dict(item, id=identifier))
        item['id'], item['state'] = identifier, 'ready'
        self._save()
        return item['name']

    def create_container(self, image_id, network_name, mounts, ports, command=None, *, clear_entrypoint=False):
        self._validate()
        self._check_daemon()
        if (not isinstance(image_id, str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id)
                or not isinstance(mounts, dict) or not isinstance(ports, list)
                or type(clear_entrypoint) is not bool or (clear_entrypoint and command is None)
                or any(r['state'] in ('creating', 'starting', 'executing', 'stopping', 'removing') for r in self.state['resources'])):
            raise OwnershipError('invalid image or unresolved operation')
        self._validate_mounts_ports(mounts, ports)
        if command is not None:
            self._validate_command(command)
        networks = [r for r in self.state['resources'] if r['kind'] == 'network' and r['name'] == network_name and r['state'] == 'ready']
        volumes = {r['name']: r for r in self.state['resources'] if r['kind'] == 'volume' and r['state'] == 'ready'}
        if len(networks) != 1 or any(name not in volumes for name in mounts.values()):
            raise OwnershipError('container network or volumes are not owned by this run')
        image = self.engine.image(image_id)
        if (not isinstance(image, dict) or image.get('id') != image_id or not isinstance(image.get('volumes'), list)
                or not set(image['volumes']).issubset(mounts)):
            raise OwnershipError('all declared image volumes must be owned and mounted')
        if command is not None and image.get('entrypoint') != [] and not clear_entrypoint:
            raise OwnershipError('explicit commands require an image without an entrypoint')
        self._inspect(networks[0])
        for name in mounts.values():
            self._inspect(volumes[name])
        nonce = uuid.uuid4().hex
        item = {'kind': 'container', 'name': self.run_id + '-container-' + nonce, 'nonce': nonce,
                'id': '', 'state': 'creating', 'image': image_id, 'network': networks[0]['id'],
                'mounts': dict(mounts), 'ports': sorted(ports)}
        if command is not None:
            item['command'] = list(command)
        self.state['resources'].append(item)
        self._save()
        labels = {'voice.ci.run': self.run_id, 'voice.ci.resource': nonce, 'voice.ci.kind': 'container'}
        if command is None:
            identifier = self.engine.create_container(item['name'], labels, image_id, item['network'], mounts, item['ports'])
        elif clear_entrypoint:
            identifier = self.engine.create_container(item['name'], labels, image_id, item['network'], mounts,
                                                      item['ports'], command=list(command), clear_entrypoint=True)
        else:
            identifier = self.engine.create_container(item['name'], labels, image_id, item['network'], mounts, item['ports'], command=list(command))
        self._check_daemon()
        if not isinstance(identifier, str) or not re.fullmatch(r'[0-9a-f]{64}', identifier):
            raise OwnershipError('Docker container identity is invalid')
        self._inspect(dict(item, id=identifier))
        item['id'], item['state'] = identifier, 'ready'
        self._save()
        return item['name']

    def create_rootless_container(self, network_name, mounts, creator):
        """Journal a narrow profile; caller must authenticate creator and socket access.

        This primitive alone is not a preflight receipt or complete isolation.
        """
        self._validate()
        self._check_daemon()
        if (not callable(creator) or not isinstance(mounts, dict) or set(mounts) not in (rootless.TARGETS, rootless.WORKSPACE_TARGETS)
                or any(not isinstance(value, str) for value in mounts.values())
                or len(set(mounts.values())) != len(mounts)
                or any(r['state'] in ('creating', 'starting', 'executing', 'stopping', 'removing')
                       for r in self.state['resources'])):
            raise OwnershipError('invalid rootless request or unresolved operation')
        self._validate_mounts_ports(mounts, [])
        networks = [r for r in self.state['resources'] if r['kind'] == 'network'
                    and r['name'] == network_name and r['state'] == 'ready']
        volumes = {r['name']: r for r in self.state['resources'] if r['kind'] == 'volume' and r['state'] == 'ready'}
        if len(networks) != 1 or any(name not in volumes for name in mounts.values()):
            raise OwnershipError('rootless network or volumes are not owned by this run')
        if any(r.get('profile') in ROOTLESS_PROFILES and r['state'] != 'removed' and
               set(r['mounts'].values()) & set(mounts.values()) for r in self.state['resources']):
            raise OwnershipError('rootless daemon volumes are already in use')
        image = self.engine.image(rootless.IMAGE)
        if (not isinstance(image, dict) or image.get('id') != rootless.IMAGE
                or image.get('entrypoint') != ['dockerd-entrypoint.sh']
                or not isinstance(image.get('volumes'), list) or not set(image['volumes']).issubset(mounts)):
            raise OwnershipError('rootless image differs or declares foreign volumes')
        self._inspect(networks[0])
        for name in mounts.values():
            self._inspect(volumes[name])
        nonce = uuid.uuid4().hex
        item = {'kind': 'container', 'name': self.run_id + '-container-' + nonce, 'nonce': nonce,
                'id': '', 'state': 'creating', 'image': rootless.IMAGE, 'network': networks[0]['id'],
                'mounts': dict(mounts), 'ports': [], 'profile':
                    'rootless-workspace-v1' if '/workspace' in mounts else 'rootless-unix-v1'}
        self.state['resources'].append(item)
        self._save()
        request = {'name': item['name'], 'run_id': self.run_id, 'nonce': nonce, 'daemon': self.state['daemon'],
                   'network': item['network'], 'data_volume': mounts['/home/rootless/.local/share/docker'],
                   'legacy_volume': mounts['/var/lib/docker'], 'socket_volume': mounts['/run/user/1000']}
        if '/workspace' in mounts:
            request['workspace_volume'] = mounts['/workspace']
        identifier = creator(request)
        self._check_daemon()
        if not isinstance(identifier, str) or not re.fullmatch(r'[0-9a-f]{64}', identifier):
            raise OwnershipError('rootless container identity is invalid')
        self._inspect(dict(item, id=identifier))
        item['id'], item['state'] = identifier, 'ready'
        self._save()
        return item['name']

    def start_container(self, name):
        self._validate()
        self._check_daemon()
        matches = [r for r in self.state['resources'] if r['kind'] == 'container' and r['name'] == name and r['state'] == 'ready']
        if len(matches) != 1 or any(r['state'] in ('creating', 'starting', 'executing', 'stopping', 'removing') for r in self.state['resources']):
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

    def execute(self, name, command, stdout, stderr, timeout, stdin=None):
        self._validate()
        self._validate_command(command)
        if type(timeout) not in (int, float) or not 0 < timeout <= 86400:
            raise OwnershipError('invalid command timeout')
        self._check_daemon()
        matches = [r for r in self.state['resources'] if r['kind'] == 'container' and
                   r['name'] == name and r['state'] == 'ready' and 'command' in r]
        if len(matches) != 1 or any(r['state'] in ('creating', 'starting', 'executing', 'stopping', 'removing') for r in self.state['resources']):
            raise OwnershipError('container is not ready or an operation is unresolved')
        item = matches[0]
        if not self._inspect(item).get('running'):
            raise OwnershipError('owned container is not running')
        item['state'] = 'executing'
        self._save()
        if stdin is None:
            result = self.engine.execute(item['id'], command, stdout, stderr, timeout)
        else:
            result = self.engine.execute(item['id'], command, stdout, stderr, timeout, stdin=stdin)
        if type(result) is not int or not 0 <= result <= 255 or result == 125:
            raise OwnershipError('invalid execution result')
        self._check_daemon()
        if not self._inspect(item).get('running'):
            raise OwnershipError('owned container stopped during execution')
        item['state'] = 'ready'
        self._save()
        return result

    def load_canonical_rootless_image(self, name, archive_path, archive_sha256, image_id, image_tag,
                                      stdout, stderr, timeout=300):
        """Canonicalize a verified OCI archive before loading into the private daemon.

        The caller must authenticate the identities and control the daemon socket.
        The owned directory must not have concurrent or untrusted writers.
        """
        path = Path(archive_path)
        if not path.is_absolute() or path.resolve() != path or path.parent != self.directory or path.is_symlink():
            raise OwnershipError('image archive must be a regular file in the owned run directory')
        canonical_path = None
        try:
            with path.open('rb') as source:
                before, current = os.fstat(source.fileno()), path.lstat()
                if (not stat.S_ISREG(before.st_mode) or not stat.S_ISREG(current.st_mode) or
                        (before.st_dev, before.st_ino) != (current.st_dev, current.st_ino)):
                    raise OwnershipError('image archive identity differs')
                with tempfile.NamedTemporaryFile(mode='w+b', prefix='canonical-', suffix='.tar',
                                                 dir=self.directory, delete=False) as output:
                    canonical_path = Path(output.name)
                    identity = local_image_archive.sanitize_archive(source, output, archive_sha256,
                                                                    image_id, image_tag)
            self.load_rootless_image(name, canonical_path, identity['archive_sha256'], image_id,
                                     stdout, stderr, timeout)
        except (OSError, ValueError) as exc:
            raise OwnershipError('image canonicalization failed') from exc
        finally:
            if canonical_path is not None:
                active_error = sys.exception()
                try:
                    canonical_path.unlink(missing_ok=True)
                except OSError as cleanup_error:
                    if active_error is None:
                        raise OwnershipError('canonical image archive cleanup failed') from cleanup_error
                    active_error.add_note('Canonical image archive cleanup failed: ' + str(canonical_path))

    def load_canonical_rootless_alias(self, name, archive_path, archive_sha256, image_id,
                                      source_tag, target_tag, stdout, stderr, timeout=300):
        """Add one independently verified tag to an existing owned image.

        Caller controls the private daemon/socket. An uncertain operation must
        be stopped and quarantined; this does not prove daemon-side cancellation.
        """
        tag_pattern = r'[A-Za-z0-9][A-Za-z0-9._:/-]*:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}'
        if (not isinstance(source_tag, str) or not re.fullmatch(tag_pattern, source_tag) or
                not isinstance(target_tag, str) or not re.fullmatch(tag_pattern, target_tag) or
                source_tag == target_tag or not isinstance(archive_sha256, str) or
                not re.fullmatch(r'[0-9a-f]{64}', archive_sha256) or not isinstance(image_id, str) or
                not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id) or type(timeout) not in (int, float) or
                not 0 < timeout <= 86400):
            raise OwnershipError('invalid alias identity')
        path = Path(archive_path)
        if not path.is_absolute() or path.resolve() != path or path.parent != self.directory or path.is_symlink():
            raise OwnershipError('image archive must be a regular file in the owned run directory')
        canonical_path = None
        try:
            with path.open('rb') as source:
                before, current = os.fstat(source.fileno()), path.lstat()
                if (not stat.S_ISREG(before.st_mode) or not stat.S_ISREG(current.st_mode) or
                        (before.st_dev, before.st_ino) != (current.st_dev, current.st_ino)):
                    raise OwnershipError('image archive identity differs')
                with tempfile.NamedTemporaryFile(mode='w+b', prefix='canonical-alias-', suffix='.tar',
                                                 dir=self.directory, delete=False) as canonical:
                    canonical_path = Path(canonical.name)
                    identity = local_image_archive.sanitize_archive(source, canonical, archive_sha256, image_id, target_tag)
                after, current = os.fstat(source.fileno()), path.lstat()
                if (not stat.S_ISREG(current.st_mode) or
                        (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) !=
                        (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns) or
                        (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns) !=
                        (current.st_dev, current.st_ino, current.st_size, current.st_mtime_ns)):
                    raise OwnershipError('image archive changed during canonicalization')
            self._load_rootless_alias(name, canonical_path, identity['archive_sha256'], image_id,
                                      source_tag, target_tag, stdout, stderr, timeout)
        except (OSError, ValueError) as exc:
            raise OwnershipError('alias canonicalization failed') from exc
        finally:
            if canonical_path is not None:
                active_error = sys.exception()
                try:
                    canonical_path.unlink(missing_ok=True)
                except OSError as cleanup_error:
                    if active_error is None:
                        raise OwnershipError('canonical alias archive cleanup failed') from cleanup_error
                    active_error.add_note('Canonical alias archive cleanup failed: ' + str(canonical_path))

    def _load_rootless_alias(self, name, path, archive_sha256, image_id, source_tag, target_tag, stdout, stderr, timeout):
        self._validate(); self._check_daemon()
        matches = [r for r in self.state['resources'] if r['kind'] == 'container' and r['name'] == name
                   and r.get('profile') in ROOTLESS_PROFILES and r['state'] == 'ready']
        if len(matches) != 1 or any(r['state'] in ('creating', 'starting', 'executing', 'stopping', 'removing')
                                    for r in self.state['resources']):
            raise OwnershipError('rootless daemon is not ready or an operation is unresolved')
        item = matches[0]
        if not self._inspect(item).get('running'):
            raise OwnershipError('rootless daemon is not running')
        with Path(path).open('rb') as archive:
            before, current = os.fstat(archive.fileno()), Path(path).lstat()
            if (not stat.S_ISREG(before.st_mode) or not stat.S_ISREG(current.st_mode) or
                    (before.st_dev, before.st_ino) != (current.st_dev, current.st_ino) or
                    hashlib.file_digest(archive, 'sha256').hexdigest() != archive_sha256):
                raise OwnershipError('canonical alias archive identity differs')
            item['image_operation'] = {'kind': 'alias-load-v1', 'archive_sha256': archive_sha256,
                                       'image_id': image_id, 'source_tag': source_tag, 'target_tag': target_tag}
            item['state'] = 'executing'; self._save()
            prefix = ['docker', '--host', 'unix:///run/user/1000/docker.sock']
            def inventory():
                with tempfile.TemporaryFile() as out:
                    code = self.engine.execute(item['id'], prefix + ['image', 'ls', '--all', '--no-trunc',
                        '--format', '{{.Repository}}:{{.Tag}} {{.ID}}'], out, stderr, timeout)
                    out.seek(0); raw = out.read(8 * 1024 * 1024 + 1)
                if type(code) is not int or code != 0 or len(raw) > 8 * 1024 * 1024:
                    raise OwnershipError('private image inventory unavailable')
                try:
                    rows = [line.split(' ') for line in raw.decode('ascii').splitlines()]
                except UnicodeDecodeError:
                    raise OwnershipError('invalid private image inventory') from None
                pattern = r'(?:<none>:<none>|[A-Za-z0-9][A-Za-z0-9._:/-]*:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127})'
                if any(len(row) != 2 or not re.fullmatch(pattern, row[0]) or
                       not re.fullmatch(r'sha256:[0-9a-f]{64}', row[1]) for row in rows):
                    raise OwnershipError('invalid private image inventory')
                return sorted(tuple(row) for row in rows)
            initial = inventory()
            if (initial.count((source_tag, image_id)) != 1 or sum(tag == source_tag for tag, _ in initial) != 1 or
                    any(tag == target_tag for tag, _ in initial)):
                raise OwnershipError('alias source or target differs')
            archive.seek(0)
            if self.engine.execute_verified_stream(item['id'], prefix + ['load'], stdout, stderr, timeout,
                                                   archive, archive_sha256):
                raise OwnershipError('private alias load failed')
            after, current = os.fstat(archive.fileno()), Path(path).lstat()
            archive.seek(0)
            if (not stat.S_ISREG(current.st_mode) or (after.st_dev, after.st_ino) != (current.st_dev, current.st_ino) or
                    (before.st_size, before.st_mtime_ns) != (after.st_size, after.st_mtime_ns) or
                    hashlib.file_digest(archive, 'sha256').hexdigest() != archive_sha256):
                raise OwnershipError('canonical alias archive changed')
            if inventory() != sorted(initial + [(target_tag, image_id)]):
                raise OwnershipError('private alias inventory changed unexpectedly')
        self._check_daemon()
        if not self._inspect(item).get('running'):
            raise OwnershipError('rootless daemon stopped during alias load')
        del item['image_operation']; item['state'] = 'ready'; self._save()

    def load_rootless_image(self, name, archive_path, archive_sha256, image_id, stdout, stderr, timeout=300):
        """Load an archive into an owned daemon; caller identities are not provenance proof.

        Any uncertainty after the operation starts leaves it executing. Recovery
        stops and quarantines the daemon before it can be removed, never reused.
        The caller must control access to the daemon socket during the operation.
        The timeout bounds Docker execution, not a stalled filesystem read.
        """
        if (not isinstance(archive_sha256, str) or not re.fullmatch(r'[0-9a-f]{64}', archive_sha256)
                or not isinstance(image_id, str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id)
                or type(timeout) not in (int, float) or not 0 < timeout <= 86400):
            raise OwnershipError('invalid rootless image identity')
        path = Path(archive_path)
        if not path.is_absolute() or path.resolve() != path or path.parent != self.directory or path.is_symlink():
            raise OwnershipError('image archive must be a regular file in the owned run directory')
        def digest(stream):
            stream.seek(0)
            value = hashlib.sha256()
            for block in iter(lambda: stream.read(1024 * 1024), b''):
                value.update(block)
            return value.hexdigest()
        with path.open('rb') as archive:
            before, current = os.fstat(archive.fileno()), path.lstat()
            if (not stat.S_ISREG(before.st_mode) or not stat.S_ISREG(current.st_mode)
                    or (before.st_dev, before.st_ino) != (current.st_dev, current.st_ino)
                    or digest(archive) != archive_sha256):
                raise OwnershipError('image archive identity differs')
            self._validate()
            self._check_daemon()
            matches = [r for r in self.state['resources'] if r['kind'] == 'container' and r['name'] == name
                       and r.get('profile') in ROOTLESS_PROFILES and r['state'] == 'ready']
            if (len(matches) != 1 or any(r['state'] in ('creating', 'starting', 'executing', 'stopping', 'removing')
                                         for r in self.state['resources'])):
                raise OwnershipError('rootless daemon is not ready or an operation is unresolved')
            item = matches[0]
            if not self._inspect(item).get('running'):
                raise OwnershipError('rootless daemon is not running')
            item['state'] = 'executing'
            self._save()
            archive.seek(0)
            prefix = ['docker', '--host', 'unix:///run/user/1000/docker.sock']
            inspect_command = prefix + ['image', 'inspect', image_id, '--format', '{{.Id}}']
            with tempfile.TemporaryFile() as inspected:
                before_load = self.engine.execute(item['id'], prefix + ['image', 'ls', '--all', '--no-trunc',
                                                  '--format', '{{.ID}}'], inspected, stderr, timeout)
                inspected.seek(0)
                inventory = inspected.read(8 * 1024 * 1024 + 1)
            if type(before_load) is not int or before_load != 0 or len(inventory) > 8 * 1024 * 1024:
                raise OwnershipError('rootless image inventory is unavailable')
            try:
                images = inventory.decode('ascii').splitlines()
            except UnicodeDecodeError:
                raise OwnershipError('rootless image inventory is invalid') from None
            if any(not re.fullmatch(r'sha256:[0-9a-f]{64}', value) for value in images) or image_id in images:
                raise OwnershipError('image must be absent before loading')
            if self.engine.execute_verified_stream(item['id'], prefix + ['load'], stdout, stderr, timeout,
                                                   archive, archive_sha256):
                raise OwnershipError('rootless image load failed')
            after, current = os.fstat(archive.fileno()), path.lstat()
            if (not stat.S_ISREG(current.st_mode) or (after.st_dev, after.st_ino) != (current.st_dev, current.st_ino)
                    or (before.st_size, before.st_mtime_ns) != (after.st_size, after.st_mtime_ns)
                    or digest(archive) != archive_sha256):
                raise OwnershipError('image archive changed during load')
        with tempfile.TemporaryFile() as inspected:
            if self.engine.execute(item['id'], inspect_command, inspected, stderr, timeout):
                raise OwnershipError('loaded image is unavailable')
            inspected.seek(0)
            if inspected.read() != (image_id + '\n').encode('ascii'):
                raise OwnershipError('loaded image identity differs')
        self._check_daemon()
        if not self._inspect(item).get('running'):
            raise OwnershipError('rootless daemon stopped during image load')
        item['state'] = 'ready'
        self._save()

    def cancel_execution(self, name):
        """Stop the exact owned container before reconciling an uncertain exec."""
        self._validate()
        self._check_daemon()
        matches = [r for r in self.state['resources'] if r['kind'] == 'container' and
                   r['name'] == name and r['state'] in ('executing', 'stopping') and
                   ('command' in r or r.get('profile') in ROOTLESS_PROFILES)]
        if len(matches) != 1:
            raise OwnershipError('no matching uncertain owned execution')
        item = matches[0]
        observed = self._inspect(item)
        if type(observed.get('running')) is not bool:
            raise OwnershipError('owned container running state is unknown')
        item['state'] = 'stopping'
        self._save()
        if observed.get('running'):
            self.engine.stop(item['id'])
        self._check_daemon()
        if self._inspect(item).get('running') is not False:
            raise OwnershipError('owned container did not stop')
        if item.get('profile') in ROOTLESS_PROFILES:
            volumes = {r['name']: r for r in self.state['resources'] if r['kind'] == 'volume'}
            if any(name not in volumes or volumes[name]['state'] not in ('ready', 'quarantined')
                   for name in item['mounts'].values()):
                raise OwnershipError('rootless daemon volume state changed')
            item['state'] = 'quarantined'
            for name in item['mounts'].values():
                volumes[name]['state'] = 'quarantined'
        else:
            item['state'] = 'ready'
        self._save()

    def cleanup(self):
        self._validate()
        self._check_daemon()
        if any(item['state'] in ('creating', 'starting', 'executing', 'stopping', 'removing') for item in self.state['resources']):
            raise OwnershipError('uncertain operation requires reconciliation')
        order = {'container': 0, 'volume': 1, 'network': 2}
        ready = sorted((item for item in reversed(self.state['resources']) if item['state'] in ('ready', 'quarantined')),
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
