"""Private fixed STAGING test-account input; no credentials are public evidence.

The operator transfers the authorized file under the protected operation. This
module does not establish rollout completion or authorize a public API effect.
"""
import json
import copy
import hashlib
import hmac
import os
import re
import secrets
import stat

DIRECTORY = '/home/pmd/voice-staging-core-accounts'


class CredentialError(RuntimeError):
    pass


def _pairs(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise CredentialError('duplicate credential field')
        result[key] = value
    return result


def decode(raw):
    """Validate the two authorized input slots without returning public hashes."""
    try:
        if not isinstance(raw, bytes) or not 1 <= len(raw) <= 65536:
            raise ValueError()
        value = json.loads(raw, object_pairs_hook=_pairs)
        if (not isinstance(value, dict) or set(value) != {'schema', 'provenance', 'accounts'} or
                value['schema'] != 'voice-staging-test-accounts-v1' or
                not isinstance(value['provenance'], str) or not 1 <= len(value['provenance']) <= 4096 or
                not isinstance(value['accounts'], list) or len(value['accounts']) != 2):
            raise ValueError()
        for item, slot in zip(value['accounts'], ('2', '3')):
            if (not isinstance(item, dict) or set(item) != {'email', 'password'} or
                    not isinstance(item['email'], str) or not 1 <= len(item['email']) <= 320 or
                    not re.fullmatch(r'[^@\s]+\+' + slot + r'@[^@\s]+', item['email']) or
                    not isinstance(item['password'], str) or not 1 <= len(item['password']) <= 4096):
                raise ValueError()
        if value['accounts'][0]['email'] == value['accounts'][1]['email']:
            raise ValueError()
        return value
    except BaseException:
        raise CredentialError('private account input invalid') from None


def _open_input():
    """Open only the fixed pmd-owned file through non-symlink ancestors."""
    import pwd
    uid = pwd.getpwnam('pmd').pw_uid
    current = os.open('/', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        for component in ('home', 'pmd', 'voice-staging-core-accounts'):
            nextfd = os.open(component, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=current)
            os.close(current)
            current = nextfd
            info = os.fstat(current)
            if (not stat.S_ISDIR(info.st_mode) or info.st_uid not in (0, uid) or
                    info.st_mode & 0o022):
                raise CredentialError('credential directory unavailable')
        info = os.fstat(current)
        if info.st_uid != uid or stat.S_IMODE(info.st_mode) != 0o700:
            raise CredentialError('credential directory unavailable')
        fd = os.open('accounts.json', os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=current)
        info = os.fstat(fd)
        if (not stat.S_ISREG(info.st_mode) or info.st_uid != uid or stat.S_IMODE(info.st_mode) != 0o600 or
                info.st_nlink != 1 or not 1 <= info.st_size <= 65536):
            os.close(fd)
            raise CredentialError('credential file unavailable')
        return fd
    finally:
        os.close(current)


def _journal_custody(journal, intent, directory, lock):
    import fcntl
    import media_operation_journal
    if (type(journal) is not media_operation_journal.Journal or journal.poisoned or
            journal.dirfd is None or journal.lockfd is None or journal.intent != intent or
            journal.state.get('intent') != intent):
        raise CredentialError('credential journal unavailable')
    def identity(info): return info.st_dev, info.st_ino
    if identity(os.fstat(journal.dirfd)) != directory or identity(os.fstat(journal.lockfd)) != lock:
        raise CredentialError('credential journal changed')
    journal._private(os.fstat(journal.dirfd), 0o700, directory=True)
    journal._private(os.fstat(journal.lockfd), 0o600)
    if (identity(os.stat(journal.path, follow_symlinks=False)) != directory or
            identity(os.stat('lock', dir_fd=journal.dirfd, follow_symlinks=False)) != lock):
        raise CredentialError('credential journal replaced')
    probe = os.open('lock', os.O_RDWR | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=journal.dirfd)
    try:
        try: fcntl.flock(probe, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError: pass
        else:
            fcntl.flock(probe, fcntl.LOCK_UN)
            raise CredentialError('credential journal lock lost')
    finally: os.close(probe)
    fd = os.open('state.json', os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=journal.dirfd)
    with os.fdopen(fd, 'rb') as stream:
        journal._private(os.fstat(stream.fileno()), 0o600)
        raw = stream.read(1048577)
    if raw != json.dumps(journal.state, sort_keys=True, separators=(',', ':')).encode():
        raise CredentialError('credential journal changed')


class CredentialInput:
    def __init__(self, session, release, journal):
        import media_operation_journal
        import resource_claim
        if type(session) is not resource_claim.Session or type(journal) is not media_operation_journal.Journal:
            raise CredentialError('STAGING credential authority unavailable')
        self.session, self.release, self.journal = session, release, journal
        self._refs = (session, release, journal)
        self._intent = copy.deepcopy(journal.intent)
        self._context = copy.deepcopy(session.context)
        self._directory = (os.fstat(journal.dirfd).st_dev, os.fstat(journal.dirfd).st_ino)
        self._lock = (os.fstat(journal.lockfd).st_dev, os.fstat(journal.lockfd).st_ino)

    def _guard(self):
        import staging_core_release
        if (any(a is not b for a, b in zip((self.session, self.release, self.journal), self._refs)) or
                self.session.context != self._context or self._context.get('resource') != 'STAGING' or
                self.journal.intent.get('resource_context') != self._context):
            raise CredentialError('STAGING credential binding changed')
        _journal_custody(self.journal, self._intent, self._directory, self._lock)
        self.session.check()
        receipt = staging_core_release.verify_completed(self.release, self.session)
        if (not isinstance(receipt, dict) or set(receipt) != {'operation_id', 'source_sha', 'namespace_uid', 'target_sha256'} or
                receipt['operation_id'] != self._context.get('operation_id') or
                receipt['source_sha'] != self._context.get('commit') or
                not isinstance(receipt['namespace_uid'], str) or not receipt['namespace_uid'] or
                not isinstance(receipt['target_sha256'], str) or not re.fullmatch('[a-f0-9]{64}', receipt['target_sha256'])):
            raise CredentialError('STAGING release binding unavailable')
        self.session.check()
        _journal_custody(self.journal, self._intent, self._directory, self._lock)
        return receipt

    def capture(self):
        """Capture once in the protected journal; keep source FD and HMAC private."""
        try:
            receipt = self._guard()
            if 'staging-core-credentials' in self.journal.state['steps']:
                raise CredentialError('credential capture requires reconciliation')
            fd = _open_input()
            try:
                before = os.fstat(fd)
                raw = os.read(fd, 65537)
                after = os.fstat(fd)
                if ((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) !=
                        (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns) or len(raw) != before.st_size):
                    raise CredentialError('credential source changed')
                accounts = decode(raw)
                if self._guard() != receipt:
                    raise CredentialError('release changed')
                key = secrets.token_bytes(32)
                material = json.dumps({'context': self._context, 'release': receipt, 'input': accounts},
                                      sort_keys=True, separators=(',', ':')).encode()
                record = {'release': receipt, 'input': accounts, 'hmac_key': key.hex(),
                          'hmac': hmac.new(key, material, hashlib.sha256).hexdigest()}
                self.journal.state['steps']['staging-core-credentials'] = record
                self.journal.save()
                if self._guard() != receipt:
                    raise CredentialError('release changed')
                return copy.deepcopy(accounts['accounts'])
            finally:
                os.close(fd)
        except BaseException:
            raise CredentialError('private STAGING credentials unavailable; reconcile') from None

    def verify(self):
        """Recheck the private HMAC, release binding and protected input."""
        try:
            receipt = self._guard()
            record = self.journal.state['steps']['staging-core-credentials']
            if not isinstance(record, dict) or set(record) != {'release', 'input', 'hmac_key', 'hmac'} or record['release'] != receipt:
                raise ValueError()
            key = bytes.fromhex(record['hmac_key'])
            if len(key) != 32:
                raise ValueError()
            material = json.dumps({'context': self._context, 'release': receipt, 'input': record['input']},
                                  sort_keys=True, separators=(',', ':')).encode()
            if not hmac.compare_digest(record['hmac'], hmac.new(key, material, hashlib.sha256).hexdigest()):
                raise ValueError()
            value = decode(json.dumps(record['input'], separators=(',', ':')).encode())
            if self._guard() != receipt:
                raise ValueError()
            return copy.deepcopy(value['accounts'])
        except BaseException:
            raise CredentialError('private STAGING credential binding changed') from None
