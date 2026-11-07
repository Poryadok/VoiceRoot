"""Source-qualified generic Go restore targets, never migration admission.

The pin identifies the audited DEV controller whose generic service Job invokes
golang-migrate `up` over the entire SQL inventory. A changed controller must be
reviewed and this policy updated before it can select targets.
"""
import copy
import re


MIGRATIONS_SOURCE_SHA256 = 'ec1f09b06a794de92541005c9efc53f85b2c52b27e7a6a49d8721b8a4b87f4d2'
SERVICES = frozenset(('bot', 'story', 'moderation', 'subscription', 'user', 'search',
                      'social', 'chat', 'messaging', 'file', 'role', 'notification',
                      'matchmaking', 'voice'))
DATABASES = frozenset(service + '_db' for service in SERVICES)


class PlanError(RuntimeError):
    pass


def for_restore(database, baseline, snapshot):
    """Select the final ledger produced by the audited full-inventory `up`."""
    if database not in DATABASES:
        return copy.deepcopy(baseline)
    try:
        if (not isinstance(snapshot, dict) or not isinstance(snapshot.get('files'), dict) or
                snapshot['files'].get('scripts/dev/migrations.py') != MIGRATIONS_SOURCE_SHA256 or
                not isinstance(baseline, dict) or set(baseline) != {'engine', 'from', 'to', 'files'} or
                baseline['engine'] != 'golang-migrate' or type(baseline['from']) is not int or
                baseline['from'] < 1 or baseline['to'] != baseline['from'] or
                not isinstance(baseline['files'], dict) or not baseline['files']):
            raise ValueError()
        prefix = 'src/backend/migrations/' + database + '/'
        files = {name[len(prefix):]: digest for name, digest in snapshot['files'].items()
                 if name.startswith(prefix) and name.endswith('.sql')}
        if files != baseline['files']:
            raise ValueError()
        entries = {}
        for name, digest in baseline['files'].items():
            match = re.fullmatch(r'([0-9]{6})_[a-z0-9_]+\.(up|down)\.sql', name) if isinstance(name, str) else None
            if not match or not isinstance(digest, str) or not re.fullmatch(r'[0-9a-f]{64}', digest):
                raise ValueError()
            identity = int(match.group(1)), match.group(2)
            if identity in entries:
                raise ValueError()
            entries[identity] = name
        versions = sorted(version for version, direction in entries if direction == 'up')
        if not versions or baseline['from'] not in versions:
            raise ValueError()
        target = versions[-1]
        if target > baseline['from'] and (baseline['from'], 'down') not in entries:
            raise ValueError()
        result = copy.deepcopy(baseline)
        result['to'] = target
        return result
    except (KeyError, TypeError, ValueError):
        raise PlanError('source-qualified Go restore target unavailable') from None
