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

    @staticmethod
    def call(*args, data=None, timeout=180):
        try:
            p = subprocess.run(['docker', *args], input=data, capture_output=True, timeout=timeout)
        except (OSError, subprocess.TimeoutExpired):
            raise ValueError('Docker outcome uncertain') from None
        if p.returncode:
            raise ValueError('Docker operation failed')
        return p.stdout

    def save(self):
        self.journal.state['flyway_clone']['resources'] = self.records
        self.journal.save()

    def create(self, image, args, command):
        name = 'voice-auth-flyway-' + uuid.uuid4().hex
        record = {'name_hint': name, 'image': image, 'state': 'creating'}
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
                or inspected.get('Config', {}).get('Image') != image):
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
            if (current.get('Id') != record['id'] or current.get('Config', {}).get('Labels', {}).get(
                    'voice.test.owner') != 'auth-flyway-qualification'):
                raise ValueError('resource identity changed')
            self.call('rm', '-f', '-v', record['id'])
            remaining = set(self.call('volume', 'ls', '--format', '{{.Name}}').decode().splitlines())
            if remaining.intersection(record['volumes']):
                raise ValueError('owned volume cleanup incomplete')
            record['state'] = 'removed'
            self.save()
