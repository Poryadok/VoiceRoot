"""Require an explicit disposition for Go *_integration_test.go entrypoints.

Unresolved exceptions are inventory gaps, not execution evidence. This audit is
limited to that filename convention and does not certify other test files.
"""
from pathlib import Path
import re


TEST = re.compile(r'(?m)^func\s+(Test\w+)\s*\(\s*\w+\s+\*testing\.T\s*\)')
DEDICATED = {
    ('analytics', 'voice/backend/analytics/internal/store', 'TestClickHouseInsertBatch'):
        'analytics-clickhouse-integration',
    ('role', 'voice/backend/role', 'TestBootstrapActualRole20PG'):
        'role-migration-integration',
    ('space', 'voice/backend/space/internal/nativeinstaller', 'TestMaintenanceWindowWithRealAutovacuum'):
        'space-autovacuum-integration',
}


def audit(root, inventory, exceptions):
    root = Path(root)
    if not isinstance(inventory, dict) or not isinstance(exceptions, list):
        raise ValueError('invalid source inventory')
    included = set()
    for service, cases in inventory.items():
        if not isinstance(service, str) or not re.fullmatch(r'[a-z][a-z0-9]*', service):
            raise ValueError('invalid service')
        if not isinstance(cases, list) or not cases:
            raise ValueError('empty service inventory')
        for case in cases:
            if not isinstance(case, list) or len(case) != 2 or not all(isinstance(x, str) for x in case):
                raise ValueError('invalid inventory case')
            package, name = case
            prefix = 'voice/backend/' + service
            if not (package == prefix or package.startswith(prefix + '/')) or not re.fullmatch(r'Test\w+(?:/[\w.-]+)*', name):
                raise ValueError('inventory case outside service')
            key = (service, package, name)
            if key in included:
                raise ValueError('duplicate inventory case')
            included.add(key)
    omitted = {}
    for item in exceptions:
        if not isinstance(item, dict) or set(item) != {'service', 'package', 'test', 'source', 'status', 'reason'}:
            raise ValueError('invalid source exception')
        if (item['status'] not in ('dedicated', 'unresolved') or
                not isinstance(item['reason'], str) or not item['reason'].strip() or
                not all(isinstance(item[k], str) for k in ('service', 'package', 'test', 'source'))):
            raise ValueError('invalid source exception disposition')
        key = (item['service'], item['package'], item['test'])
        if key in omitted or key in included or item['service'] not in inventory:
            raise ValueError('duplicate or conflicting source exception')
        if item['status'] == 'dedicated' and DEDICATED.get(key) != item['reason']:
            raise ValueError('dedicated source exception lacks a reviewed proof mapping')
        omitted[key] = item['source']
    seen = {}
    for service in inventory:
        service_dir = root / 'src/backend' / service
        if not service_dir.is_dir():
            raise ValueError('service source missing: ' + service)
        for path in service_dir.rglob('*_integration_test.go'):
            package = 'voice/backend/' + service
            relative_dir = path.parent.relative_to(service_dir).as_posix()
            if relative_dir != '.':
                package += '/' + relative_dir
            for name in TEST.findall(path.read_text(encoding='utf-8')):
                key = (service, package, name)
                if key in seen:
                    raise ValueError('duplicate source test: ' + package + '/' + name)
                seen[key] = path.relative_to(root).as_posix()
    for key, source in seen.items():
        if key not in included and omitted.get(key) != source:
            raise ValueError('unclassified integration source: ' + source + '/' + key[2])
    for key, source in omitted.items():
        if seen.get(key) != source:
            raise ValueError('stale integration source exception: ' + source + '/' + key[2])
    return {'source_cases': len(seen), 'unresolved': sum(item['status'] == 'unresolved' for item in exceptions)}
