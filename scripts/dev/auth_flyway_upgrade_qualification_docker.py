"""Owned Linux Docker resources for private Auth clone qualification."""
import json
import re
import subprocess
import uuid


class OwnedDocker:
    def __init__(self, journal):
        self.journal = journal
        self.records = []
        if 'flyway_clone' in journal.state:
            raise ValueError('existing clone attempt requires reconciliation')
        journal.state['flyway_clone'] = {'status': 'starting', 'resources': []}
        journal.save()
        if self.call('info', '--format', '{{.OSType}}').strip() != b'linux':
            raise ValueError('Linux Docker required')

    def call(self, *args, data=None, timeout=180):
        guard = getattr(self.journal, 'guard', lambda: None)
        guard()
        try:
            p = subprocess.run(['docker', *args], input=data, capture_output=True, timeout=timeout)
        except (OSError, subprocess.TimeoutExpired):
            raise ValueError('Docker outcome uncertain') from None
        if p.returncode:
            raise ValueError('Docker operation failed')
        guard()
        return p.stdout

    def save(self):
        self.journal.state['flyway_clone']['resources'] = self.records
        self.journal.save()

    def create(self, image, args, command):
        if args.count('--network') != 1:
            raise ValueError('exact private network mode required')
        network = args[args.index('--network') + 1]
        if network != 'none' and not re.fullmatch(r'container:[0-9a-f]{64}', network):
            raise ValueError('network is not private to the clone')
        image_info = json.loads(self.call('image', 'inspect', image))[0]
        image_id = image_info.get('Id', '')
        if (not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id) or image_info.get('Os') != 'linux'
                or (image.startswith('sha256:') and image != image_id)
                or (not image.startswith('sha256:') and image not in (image_info.get('RepoDigests') or []))):
            raise ValueError('immutable Linux image identity unavailable')
        name = 'voice-auth-flyway-' + uuid.uuid4().hex
        record = {'name_hint': name, 'image': image, 'image_id': image_id, 'state': 'creating'}
        self.records.append(record)
        self.save()  # Creation can succeed even if the client response is lost.
        result = self.call('create', '--name', name, '--label', 'voice.test.owner=auth-flyway-qualification',
                           *args, image, *command).decode().strip()
        if not re.fullmatch(r'[0-9a-f]{64}', result):
            raise ValueError('container creation identity uncertain')
        record.update(id=result, state='created')
        self.save()
        inspected = json.loads(self.call('inspect', result))[0]
        if (inspected.get('Id') != result or inspected.get('Config', {}).get('Labels', {}).get(
                'voice.test.owner') != 'auth-flyway-qualification'
                or inspected.get('Config', {}).get('Image') != image or inspected.get('Image') != image_id
                or inspected.get('HostConfig', {}).get('NetworkMode') != network
                or inspected.get('HostConfig', {}).get('PortBindings')
                or inspected.get('HostConfig', {}).get('Privileged')):
            raise ValueError('container ownership unverified')
        mounts = inspected.get('Mounts', [])
        if any(m.get('Type') != 'volume' for m in mounts):
            raise ValueError('unexpected shared mount')
        record['volumes'] = [m['Name'] for m in mounts]
        record['state'] = 'owned'
        self.save()
        return result

    def cleanup(self):
        for record in reversed(self.records):
            if record['state'] != 'owned':
                raise ValueError('unknown resource identity requires reconciliation')
            current = json.loads(self.call('inspect', record['id']))[0]
            if (current.get('Id') != record['id'] or current.get('Image') != record['image_id']
                    or current.get('Config', {}).get('Labels', {}).get(
                    'voice.test.owner') != 'auth-flyway-qualification'):
                raise ValueError('resource identity changed')
            record['state'] = 'removing'
            self.save()
            self.call('rm', '-f', '-v', record['id'])
            remaining_ids = set(self.call('ps', '-a', '--no-trunc', '--format', '{{.ID}}').decode().splitlines())
            if record['id'] in remaining_ids:
                raise ValueError('owned container cleanup incomplete')
            remaining = set(self.call('volume', 'ls', '--format', '{{.Name}}').decode().splitlines())
            if remaining.intersection(record['volumes']):
                raise ValueError('owned volume cleanup incomplete')
            record['state'] = 'removed'
            self.save()
