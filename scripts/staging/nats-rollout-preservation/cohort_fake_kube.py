"""Stateful Kubernetes transport for offline cohort composition tests.

This models the external Kubernetes API only; it does not issue preservation
receipts or replace any of the production admission and qualification checks.
"""
import copy
import json


class FakeKube:
    def __init__(self, objects):
        self.objects = {(kind.lower(), name): copy.deepcopy(value)
                        for (kind, name), value in objects.items()}
        self.calls = []

    def get(self, kind, name):
        return copy.deepcopy(self.objects[(kind.lower(), name)])

    def patch(self, kind, name, changes):
        key = (kind.lower(), name)
        current = self.objects[key]
        updated = copy.deepcopy(current)
        for change in changes:
            parts = [part.replace('~1', '/').replace('~0', '~')
                     for part in change['path'].lstrip('/').split('/')]
            parent = updated
            for part in parts[:-1]:
                parent = parent[int(part)] if isinstance(parent, list) else parent[part]
            leaf = int(parts[-1]) if isinstance(parent, list) else parts[-1]
            op = change['op']
            if op == 'test':
                if parent[leaf] != change['value']:
                    raise RuntimeError('synthetic_kubernetes_conflict')
            elif op in ('add', 'replace'):
                parent[leaf] = copy.deepcopy(change['value'])
            elif op == 'remove':
                del parent[leaf]
            else:
                raise ValueError('unsupported synthetic Kubernetes patch')
        metadata = updated['metadata']
        metadata['resourceVersion'] = str(int(metadata['resourceVersion']) + 1)
        if kind.lower() == 'deployment' and updated.get('spec') != current.get('spec'):
            metadata['generation'] += 1
        self.objects[key] = updated
        self.calls.append((kind, name, copy.deepcopy(changes)))
        return copy.deepcopy(updated)

    def capture_patch(self, argv, *, body, **_kwargs):
        import commands
        commands.authority_guard(argv)
        try:
            index = argv.index('patch')
            kind, name = argv[index + 1:index + 3]
        except (ValueError, IndexError):
            raise AssertionError('unexpected synthetic Kubernetes command') from None
        return json.dumps(self.patch(kind, name, json.loads(body))).encode()
