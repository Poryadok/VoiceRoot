"""Guarded logical dumps for an admitted single-engine database cohort.

The caller supplies real authority, drain and durable journal implementations.
This module does not establish selection completeness or operation authority.
"""
from datetime import datetime
import json
import re
import subprocess
import tempfile

import story_media_backup_capture as media
import story_media_backup_cohort as cohort


def _instant(value):
    if not isinstance(value, str): raise ValueError('invalid timestamp')
    result = datetime.fromisoformat(value.replace('Z', '+00:00'))
    if result.tzinfo is None or result.utcoffset() is None: raise ValueError('missing timezone')
    return result


class Producer:
    def __init__(self, binding, databases, expected, admin_role, guard, drain, journal, runner=None):
        try:
            self.binding = json.loads(json.dumps(binding, sort_keys=True, allow_nan=False))
            self.databases = json.loads(json.dumps(databases))
            self.members = cohort._members(self.binding, self.databases)
            engine = self.binding['sources']['roles']['engine']
            if (not isinstance(admin_role, str) or not admin_role or len(admin_role) > 63
                    or any(expected[k] != engine[k] for k in
                           ('namespace_uid', 'pod_uid', 'pvc_uid', 'pv_uid', 'container_id', 'restart_count'))
                    or expected['operation'] != self.binding['operation_id']
                    or expected['snapshot_id'] != self.binding['snapshot_id']):
                raise ValueError('source identity differs')
            for member in self.members: _instant(self.binding['sources'][member]['server_start'])
            self.base = media.Producer(expected, guard, drain, journal, runner)
            self.admin_role = admin_role
        except Exception:
            raise media.CaptureError('cohort capture identity unavailable') from None

    def _check(self, member):
        self.base._check()
        source = self.binding['sources'][member]
        db = 'postgres' if member == 'roles' else member
        sql = ("SELECT json_build_object('database',current_database(),'database_oid',"
               "(SELECT oid::bigint FROM pg_catalog.pg_database WHERE datname=current_database()),"
               "'role',current_user,'server_start',pg_catalog.pg_postmaster_start_time())")
        script = ('export PGPASSWORD="$POSTGRES_PASSWORD"; '
                  'exec psql -h 127.0.0.1 -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1" -c "$2"')
        result = self.base.runner(media.BASE + ['exec', media.POD, '-c', 'postgres', '--',
                                  'sh', '-ec', script, '--', db, sql], stdout=subprocess.PIPE,
                                  stderr=subprocess.DEVNULL, timeout=20)
        if result.returncode or len(result.stdout) > 4096: raise ValueError('source observation unavailable')
        observed = json.loads(result.stdout)
        if (observed['database'] != db or observed['role'] != self.admin_role
                or _instant(observed['server_start']) != _instant(source['server_start'])
                or (member != 'roles' and (type(observed['database_oid']) is not int
                                             or observed['database_oid'] != source['database_oid']))):
            raise ValueError('source observation changed')
        self.base.guard(); self.base.drain()

    def __call__(self, member, output):
        if member not in self.members: raise media.CaptureError('capture member outside scope')
        try:
            self._check(member)
        except Exception:
            raise media.CaptureError('capture precondition unavailable') from None
        try:
            nonce = self.base.journal.begin(member)
            if not isinstance(nonce, str) or not re.fullmatch(r'[0-9a-f]{32}', nonce):
                raise ValueError('invalid nonce')
            self._check(member)
            script = ('export PGPASSWORD="$POSTGRES_PASSWORD"; export PGAPPNAME=voice-media-backup; '
                      'export PGOPTIONS="-c statement_timeout=60000 -c lock_timeout=5000"; ')
            script += ('exec pg_dumpall -h 127.0.0.1 -U "$POSTGRES_USER" --roles-only --no-role-passwords'
                       if member == 'roles' else
                       'exec pg_dump -h 127.0.0.1 -U "$POSTGRES_USER" --format=custom --lock-wait-timeout=5s -d "$1"')
            command = media.BASE + ['exec', media.POD, '-c', 'postgres', '--', 'sh', '-ec', script, '--', member]
            with tempfile.TemporaryFile() as stderr:
                result = self.base.runner(command, stdout=output, stderr=stderr, timeout=90)
            if result.returncode: raise ValueError('dump uncertain')
            self._check(member)
            expected = self.base.expected; source = self.binding['sources'][member]
            self.base.journal.complete(member, {'operation_id': expected['operation'],
                'snapshot_id': expected['snapshot_id'], 'member': member,
                **{name: expected[name] for name in ('pod_uid', 'pvc_uid', 'pv_uid', 'container_id', 'restart_count')},
                'server_start': source['server_start'], 'database_oid': source.get('database_oid'),
                'attempt_nonce': nonce, 'exit': 0, 'scope': 'dump-terminal-only'})
        except BaseException:
            try: self.base.journal.uncertain(member)
            except BaseException: pass
            raise media.CaptureUncertain('capture uncertain; retain fence and reconcile') from None
