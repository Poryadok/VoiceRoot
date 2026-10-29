"""Fixed C1 public truststore producer under the protected operation fence."""

import base64
import copy
import hashlib
import hmac
import json
import os
import re
import secrets
import stat

import realtime_p3_create_only_transport as private
import dev_peer_issuer_preflight as issuer
import dev_peer_issuer_metadata as peer_metadata
import space_creator_runtime_profile_jdk as jdk
import space_creator_runtime_profile_target_trust as target
import space_creator_runtime_profile_trust as trust
import writer_cut

STEP = 'space-creator-trust-effect'
LEDGER = 'space_creator_trust_effect'
PLAN_STEP = 'space-creator-trust-plan'
KEY_FILE = 'space-creator-trust-hmac.key'
SECRET = 'voice-media-jvm-trust'
KEY = 'truststore.p12'
GENERATE = ('umask 077; cat > /scratch/ca; '
    '/opt/java/openjdk/bin/keytool -importkeystore -noprompt '
    '-srckeystore /opt/java/openjdk/lib/security/cacerts -srcstorepass changeit '
    '-destkeystore /scratch/store -deststoretype PKCS12 -deststorepass changeit >/dev/null 2>&1; '
    '/opt/java/openjdk/bin/keytool -importcert -noprompt -alias voice-media-ca '
    '-file /scratch/ca -keystore /scratch/store -storetype PKCS12 -storepass changeit >/dev/null 2>&1; '
    'cat /scratch/store')


class TrustEffectError(RuntimeError):
    pass


def classify_listing(raw, roots, media_ca):
    """Only a readable PKCS12 containing exclusively certificates is replaceable."""
    try:
        if (type(roots) is not set or not roots or
                any(not isinstance(item, str) or not re.fullmatch(r'[0-9a-f]{64}', item) for item in roots) or
                not isinstance(media_ca, str) or not re.fullmatch(r'[0-9a-f]{64}', media_ca) or media_ca in roots or
                re.findall(rb'^Keystore type: ([^\r\n]+)\r?$', raw, re.MULTILINE) != [b'PKCS12']):
            raise ValueError()
        selected = trust.parse(raw)
        return 'preserve' if selected == roots | {media_ca} else 'replace'
    except (TypeError, ValueError):
        raise TrustEffectError('existing truststore is not a known public PKCS12') from None


def secret_data(value):
    """Accept only a current fixed Opaque Secret and decode every data key."""
    try:
        if (type(value) is not dict or value.get('apiVersion') != 'v1' or value.get('kind') != 'Secret' or
                value.get('type') != 'Opaque' or 'stringData' in value or
                (value.get('immutable') is not None and type(value['immutable']) is not bool)):
            raise ValueError()
        meta, data = value['metadata'], value.get('data', {})
        if (type(meta) is not dict or meta.get('name') != 'voice-media-jvm-trust' or
                meta.get('namespace') != 'voice-dev' or meta.get('deletionTimestamp') or
                any(not isinstance(meta.get(key), str) or not meta[key] for key in ('uid', 'resourceVersion')) or
                type(data) is not dict or any(not isinstance(k, str) or not isinstance(v, str) for k, v in data.items())):
            raise ValueError()
        return {key: base64.b64decode(raw, validate=True) for key, raw in data.items()}
    except (KeyError, TypeError, ValueError):
        raise TrustEffectError('existing Secret schema unavailable') from None


def _mac(key, data):
    encoded = {name: base64.b64encode(value).decode('ascii') for name, value in data.items()}
    raw = json.dumps(encoded, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode('ascii')
    return hmac.new(key, b'voice-creator-trust-secret-v1\0' + raw, hashlib.sha256).hexdigest()


class TrustEffectPhase(jdk.TargetJdkPhase):
    """One protected create/CAS attempt; unknown outcomes require reconciliation."""

    def __init__(self, journal, fence, client, lock, namespace_uid, package, inventory, jdk_phase, preflight_phase):
        super().__init__(journal, fence, client, lock, namespace_uid, package, inventory)
        if (not re.fullmatch(r'media-c1-[a-f0-9]{32}', fence.context.get('operation_id', '')) or
                type(jdk_phase) is not jdk.TargetJdkPhase or type(preflight_phase) is not issuer.PeerIssuerPreflightPhase or
                preflight_phase.profile != 'creator' or jdk_phase.package is not package or jdk_phase.inventory is not inventory or
                preflight_phase.package is not package or any(item.fence.context != fence.context or item.lock != lock or
                item.namespace_uid != namespace_uid for item in (jdk_phase, preflight_phase))):
            raise TrustEffectError('C1 peer trust identity unavailable')
        self.jdk, self.preflight = jdk_phase, preflight_phase
        self._target_refs = (jdk_phase, preflight_phase)
        self.key_identity = None

    def _locals(self, row, jdk_digest, preflight_digest):
        try:
            self._local(); self.jdk._local(); self.preflight._local()
            if self._target_refs != (self.jdk, self.preflight) or self.preflight.profile != 'creator': raise ValueError()
            local = self.jdk.journal.state['steps'][jdk.STEP]
            marker = {key: value for key, value in local.items() if key != 'receipt'}
            prelocal = self.preflight.journal.state['steps'][self.preflight.step]
            if (marker != self.jdk.record or row.get(jdk.LEDGER) != marker or marker.get('status') != 'done' or
                    marker.get('receipt_sha256') != jdk_digest or jdk_digest != writer_cut.digest(local.get('receipt')) or
                    row.get(self.preflight.ledger) != self.preflight.record or self.preflight.record.get('status') != 'done' or
                    self.preflight.record.get('receipt_sha256') != preflight_digest or
                    preflight_digest != writer_cut.digest(prelocal.get('receipt'))): raise ValueError()
        except BaseException: raise target.metadata.GuardLost('peer trust custody unavailable') from None

    def _dependencies(self, token):
        try:
            if (self._target_refs != (self.jdk, self.preflight) or type(self.preflight) is not issuer.PeerIssuerPreflightPhase or
                    self.preflight.profile != 'creator'): raise ValueError()
            self._guard(); row, revision = self._ledger()
            roots = jdk.verify_completed(self.jdk, token); source = issuer.verify_current(self.preflight, token)
            required = {'schema', 'operation_id', 'source_sha', 'namespace_uid', 'nonce', 'context_sha256',
                        'lock', 'ca', 'creator_secrets', 'scope', 'profile'}
            if (type(source) is not dict or set(source) != required or source['schema'] != 'voice-dev-peer-custody-v1' or
                    source['profile'] != 'creator' or source['scope'] != 'root-peer-custody-available-not-issued' or
                    source['operation_id'] != self.fence.context['operation_id'] or source['source_sha'] != self.fence.context['commit'] or
                    source['namespace_uid'] != self.namespace_uid or source['context_sha256'] != writer_cut.digest(self.fence.context) or
                    source['lock'] != {key: self.lock['metadata'][key] for key in ('uid', 'resourceVersion')} or
                    not isinstance(source['nonce'], str) or not re.fullmatch(r'[a-f0-9]{32}', source['nonce']) or
                    type(source['creator_secrets']) is not dict or set(source['creator_secrets']) != set(peer_metadata.MEMBERS)):
                raise ValueError()
            for name, item in source['creator_secrets'].items():
                if item == {'state': 'absent'}: continue
                if (type(item) is not dict or set(item) != {'state', 'uid', 'resourceVersion', 'keys'} or
                        item['state'] != 'present' or any(not isinstance(item[key], str) or not item[key] for key in ('uid', 'resourceVersion')) or
                        item['keys'] != sorted(peer_metadata.MEMBERS[name])): raise ValueError()
            self._guard()
            if (self._ledger() != (row, revision) or self.jdk._ledger() != (row, revision) or
                    self.preflight.fence.ledger.read() != (row, revision)): raise ValueError()
            self._locals(row, writer_cut.digest(roots), writer_cut.digest(source))
            return roots, source
        except BaseException: raise target.metadata.GuardLost('peer trust dependency unavailable') from None

    def _state(self, value):
        self._guard(); row, revision = self._ledger()
        old = row.get(LEDGER)
        if old != self.record or self.journal.state['steps'].get(STEP) != old:
            raise TrustEffectError('effect journal changed')
        self.journal.state['steps'][STEP] = copy.deepcopy(value); self.journal.save(); self._guard()
        if self._ledger() != (row, revision): raise TrustEffectError('effect ledger changed')
        row[LEDGER] = copy.deepcopy(value); row['step'] = STEP + '-' + value['status']
        self.fence.ledger.write(revision, row); self.record = copy.deepcopy(value); self._guard()
        if self._ledger()[0].get(LEDGER) != value: raise TrustEffectError('effect record changed')

    def _key(self, create=False):
        if (create and self.key_identity is not None) or (not create and self.key_identity is None): raise ValueError()
        flags = os.O_RDWR | os.O_NOFOLLOW | os.O_NONBLOCK
        if create: flags |= os.O_CREAT | os.O_EXCL
        fd = os.open(KEY_FILE, flags, 0o600, dir_fd=self.journal.dirfd)
        try:
            if create:
                raw = secrets.token_bytes(32)
                if os.write(fd, raw) != len(raw): raise ValueError()
                os.fsync(fd); os.fsync(self.journal.dirfd)
            info = os.fstat(fd)
            identity = (info.st_dev, info.st_ino)
            if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid() or
                    stat.S_IMODE(info.st_mode) != 0o600 or info.st_nlink != 1 or
                    (self.key_identity is not None and identity != self.key_identity)):
                raise ValueError()
            os.lseek(fd, 0, os.SEEK_SET); raw = os.read(fd, 33)
            if len(raw) != 32: raise ValueError()
            self.key_identity = identity
            return raw
        finally:
            os.close(fd)

    def _read_store(self):
        self._guard()
        raw = private._run(['get', 'secret', SECRET, '--ignore-not-found=true', '-o', 'json'])
        self._guard()
        if not raw.strip(): return None, None
        obj = json.loads(raw); data = secret_data(obj)
        if sum(len(k) + len(v) for k, v in data.items()) > 1048576: raise ValueError()
        return obj, data

    def _ca(self, source):
        identity, pem = self._secret('voice-media-ca', 'ca.crt')
        ca_sha = hashlib.sha256(target.material._cert_der(pem)).hexdigest()
        if source.get('ca') != identity | {'der_sha256': ca_sha}: raise ValueError()
        return identity, pem, ca_sha

    @staticmethod
    def _cleanup_owned(cid, name, image_id):
        """Remove only an inspected owned tool, even after the operation guard is lost."""
        if (not re.fullmatch(r'[a-f0-9]{64}', cid) or
                not re.fullmatch(r'voice-creator-trust-effect-[a-f0-9]{32}', name) or
                not re.fullmatch(r'sha256:[a-f0-9]{64}', image_id)): raise ValueError()
        raw = target.metadata._bounded_process(['docker', 'inspect', cid], 1048576)
        detail = json.loads(raw); item = detail[0] if isinstance(detail, list) and len(detail) == 1 else {}
        if (item.get('Id') != cid or item.get('Image') != image_id or
                item.get('Config', {}).get('Labels', {}).get('voice.test.owner') != name): raise ValueError()
        target.metadata._bounded_process(['docker', 'rm', '-f', '-v', cid], 256)
        remaining = target.metadata._bounded_process(['docker', 'ps', '-a', '--no-trunc', '--format', '{{.ID}}'], 1048576)
        if cid in remaining.decode('ascii').splitlines(): raise ValueError()

    def _tool(self, roots, script, payload):
        info = json.loads(self._docker(['image', 'inspect', roots['image']]))
        jdk._image_info(info, roots['image'], roots['config_digest'])
        name = 'voice-creator-trust-effect-' + secrets.token_hex(16)
        pending = self.record | {'tool': {'status': 'create-pending', 'name': name,
                                         'image_id': roots['config_digest']}}
        self._state(pending)
        args = ['create', '-i', '--name', name, '--network', 'none', '--read-only', '--cap-drop', 'ALL',
                '--security-opt', 'no-new-privileges', '--log-driver', 'none', '--tmpfs', '/scratch:' + target.TMPFS,
                '--user', '65532:65532', '--label', 'voice.test.owner=' + name,
                '--entrypoint', '/bin/sh', roots['config_digest'], '-ec', script]
        cid = self._docker(args, 256).decode('ascii').strip()
        if not re.fullmatch(r'[a-f0-9]{64}', cid): raise ValueError()
        try:
            detail = json.loads(self._docker(['inspect', cid]))
            item = detail[0] if isinstance(detail, list) and len(detail) == 1 else {}
            cfg, host = item.get('Config', {}), item.get('HostConfig', {})
            if (item.get('Id') != cid or item.get('Image') != roots['config_digest'] or
                    cfg.get('Labels', {}).get('voice.test.owner') != name): raise ValueError()
            if (cfg.get('Entrypoint') != ['/bin/sh'] or cfg.get('Cmd') != ['-ec', script] or
                    cfg.get('User') != '65532:65532' or cfg.get('OpenStdin') is not True or
                    host.get('NetworkMode') != 'none' or host.get('ReadonlyRootfs') is not True or host.get('CapDrop') != ['ALL'] or
                    host.get('SecurityOpt') != ['no-new-privileges'] or host.get('Tmpfs') != {'/scratch': target.TMPFS} or
                    host.get('LogConfig', {}).get('Type') != 'none' or host.get('PortBindings') or item.get('Mounts')):
                raise ValueError()
            self._state(self.record | {'tool': {'status': 'start-pending', 'name': name, 'image_id': roots['config_digest'], 'id': cid}})
            self._guard(); output = target._input(['docker', 'start', '-ai', cid], payload); self._guard()
            detail = json.loads(self._docker(['inspect', cid])); item = detail[0] if isinstance(detail, list) and len(detail) == 1 else {}
            if item.get('Id') != cid or item.get('State', {}).get('Status') != 'exited' or item.get('State', {}).get('ExitCode') != 0:
                raise ValueError()
            return output
        finally:
            checkpoint_error = None
            try:
                self._state(self.record | {'tool': {'status': 'remove-pending', 'name': name, 'image_id': roots['config_digest'], 'id': cid}})
            except BaseException as error:
                checkpoint_error = error
            self._cleanup_owned(cid, name, roots['config_digest'])
            if checkpoint_error is not None: raise checkpoint_error
            self._state(self.record | {'tool': {'status': 'removed', 'name': name, 'image_id': roots['config_digest'], 'id': cid}})

    def prepare(self, token):
        """Qualify the fixed Secret without mutating it, before the writer cut."""
        pending = False
        try:
            self._guard(); roots, source = self._dependencies(token)
            ca_id, pem, ca_sha = self._ca(source)
            old, old_data = self._read_store()
            if old_data is not None and KEY in old_data and not old_data[KEY]: raise ValueError()
            if ca_sha in set(roots['root_fingerprints']): raise ValueError()
            row, revision = self._ledger()
            if STEP in self.journal.state['steps'] or PLAN_STEP in self.journal.state['steps'] or LEDGER in row: raise ValueError()
            marker = {'status': 'pending', 'nonce': secrets.token_hex(16),
                      'operation_id': self.fence.context['operation_id'], 'source_sha': self.fence.context['commit']}
            pending = True
            self.journal.state['steps'][STEP] = copy.deepcopy(marker); self.journal.save(); self._guard()
            if self._ledger() != (row, revision): raise ValueError()
            row[LEDGER] = copy.deepcopy(marker); row['step'] = STEP + '-pending'
            self.fence.ledger.write(revision, row); self.record = copy.deepcopy(marker); self._guard()
            key = self._key(create=True)
            prior = None if old is None else {'uid': old['metadata']['uid'], 'resourceVersion': old['metadata']['resourceVersion'],
                                              'data_hmac': _mac(key, old_data)}
            mode = 'create' if old is None else 'replace'
            if old_data is not None and KEY in old_data:
                listing = self._tool(roots, target.SCRIPT, old_data[KEY])
                if classify_listing(listing, set(roots['root_fingerprints']), ca_sha) == 'preserve': mode = 'preserve'
            if mode == 'replace' and old.get('immutable') is True: raise ValueError()
            if mode != 'preserve':
                candidate = self._tool(roots, GENERATE, pem)
                if not candidate or len(candidate) > 1048576: raise ValueError()
                candidate_listing = self._tool(roots, target.SCRIPT, candidate)
                trust.compare_roots(set(roots['root_fingerprints']), candidate_listing, ca_sha)
            if (self._read_store() != (old, old_data) or self._ca(source) != (ca_id, pem, ca_sha) or
                    self._dependencies(token) != (roots, source) or self._key() != key): raise ValueError()
            plan = {'operation_id': self.fence.context['operation_id'], 'source_sha': self.fence.context['commit'],
                    'namespace_uid': self.namespace_uid, 'mode': mode, 'old': prior,
                    'media_ca': source['ca'], 'jdk_receipt_sha256': writer_cut.digest(roots),
                    'issuer_receipt_sha256': writer_cut.digest(source), 'root_count': len(roots['root_fingerprints']),
                    'scope': 'c1-public-truststore-read-only-plan'}
            self.journal.state['steps'][PLAN_STEP] = copy.deepcopy(plan); self.journal.save()
            self._state(self.record | {'status': 'prepared', 'plan_sha256': writer_cut.digest(plan)})
            return verify_prepared(self, token)
        except BaseException as error:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('creator trust preflight outcome uncertain')
                except BaseException: pass
            if isinstance(error, target.metadata.GuardLost): raise
            raise TrustEffectError('creator trust preflight unavailable; reconcile if pending') from None

    def run(self, token):
        pending = False
        try:
            plan = verify_prepared(self, token)
            self.preflight.cut.verify_drained()
            roots, source = self._dependencies(token); ca_id, pem, ca_sha = self._ca(source)
            if (plan['jdk_receipt_sha256'] != writer_cut.digest(roots) or
                    plan['issuer_receipt_sha256'] != writer_cut.digest(source) or plan['media_ca'] != source['ca'] or
                    plan['root_count'] != len(roots['root_fingerprints'])): raise ValueError()
            old, old_data = self._read_store(); key = self._key()
            prior, mode = plan['old'], plan['mode']
            current_prior = None if old is None else {'uid': old['metadata']['uid'],
                'resourceVersion': old['metadata']['resourceVersion'], 'data_hmac': _mac(key, old_data)}
            if current_prior != prior: raise ValueError()
            pending = True
            self._state(self.record | {'status': 'pending'})
            if mode != 'preserve':
                if old is not None and old.get('immutable') is True: raise ValueError()
                store = self._tool(roots, GENERATE, pem)
                if not store or len(store) > 1048576: raise ValueError()
                listing = self._tool(roots, target.SCRIPT, store)
                facts = trust.compare_roots(set(roots['root_fingerprints']), listing, ca_sha)
                if self._read_store() != (old, old_data) or self._ca(source) != (ca_id, pem, ca_sha): raise ValueError()
                if self._dependencies(token) != (roots, source): raise ValueError()
                data = {} if old_data is None else dict(old_data)
                data[KEY] = store
                encoded = {name: base64.b64encode(value).decode('ascii') for name, value in data.items()}
                if mode == 'create':
                    body = {'apiVersion': 'v1', 'kind': 'Secret', 'type': 'Opaque',
                            'metadata': {'name': SECRET, 'namespace': 'voice-dev'}, 'data': encoded}
                    args = ['create', '-f', '-', '-o', 'json']
                else:
                    metadata = old['metadata']
                    body = [{'op': 'test', 'path': '/metadata/uid', 'value': metadata['uid']},
                            {'op': 'test', 'path': '/metadata/resourceVersion', 'value': metadata['resourceVersion']},
                            {'op': 'replace' if 'data' in old else 'add', 'path': '/data', 'value': encoded}]
                    args = ['patch', 'secret', SECRET, '--type=json', '--patch-file=/dev/stdin', '-o', 'json']
                self._state(self.record | {'effect': {'status': 'pending', 'mode': mode, 'old': prior}})
                self._guard()
                if self._read_store() != (old, old_data) or self._dependencies(token) != (roots, source) or self._ca(source) != (ca_id, pem, ca_sha):
                    raise ValueError()
                self.preflight.cut.verify_drained()
                response = json.loads(private._run(args, json.dumps(body, separators=(',', ':')).encode('utf8')))
                self._guard(); actual, current_data = self._read_store()
                self.preflight.cut.verify_drained()
                if (actual is None or response != actual or current_data != data or
                        (old is not None and (actual['metadata']['uid'] != old['metadata']['uid'] or
                         actual['metadata']['resourceVersion'] == old['metadata']['resourceVersion']))): raise ValueError()
            else:
                actual, current_data = self._read_store()
                if (actual, current_data) != (old, old_data): raise ValueError()
                facts = {'root_count': plan['root_count'], 'store_count': plan['root_count'] + 1}
            self._guard()
            if self._dependencies(token) != (roots, source) or self._ca(source) != (ca_id, pem, ca_sha): raise ValueError()
            self.preflight.cut.verify_drained()
            identity = {'uid': actual['metadata']['uid'], 'resourceVersion': actual['metadata']['resourceVersion']}
            if mode != 'preserve':
                self._state(self.record | {'effect': {'status': 'done', 'mode': mode, 'old': prior, 'secret': identity}})
            receipt = {'operation_id': self.fence.context['operation_id'], 'source_sha': self.fence.context['commit'],
                       'namespace_uid': self.namespace_uid, 'mode': mode, 'old': prior, 'secret': identity,
                       'data_hmac': _mac(key, current_data), 'media_ca': source['ca'],
                       'jdk_receipt_sha256': writer_cut.digest(roots), 'issuer_receipt_sha256': writer_cut.digest(source),
                       'root_count': facts['root_count'], 'store_count': facts['store_count'],
                       'scope': 'c1-public-truststore-secret-only'}
            done = self.record | {'status': 'done', 'receipt_sha256': writer_cut.digest(receipt)}
            row, revision = self._ledger()
            if row.get(LEDGER) != self.record or self.journal.state['steps'].get(STEP) != self.record: raise ValueError()
            self.journal.state['steps'][STEP] = copy.deepcopy(done) | {'receipt': copy.deepcopy(receipt)}
            self.journal.save(); self._guard()
            if self._ledger() != (row, revision): raise ValueError()
            row[LEDGER] = copy.deepcopy(done); row['step'] = STEP + '-done'
            self.fence.ledger.write(revision, row); self.record = copy.deepcopy(done); self._guard()
            if self._ledger()[0].get(LEDGER) != done: raise ValueError()
            self._locals(self._ledger()[0], writer_cut.digest(roots), writer_cut.digest(source))
            return verify_current(self, token)
        except BaseException as error:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('creator trust Secret outcome uncertain')
                except BaseException: pass
            if isinstance(error, target.metadata.GuardLost): raise
            raise TrustEffectError('creator trust Secret unavailable; reconcile if pending') from None


def verify_prepared(phase, token):
    """Recheck typed dependencies, private plan and exact original Secret."""
    try:
        if type(phase) is not TrustEffectPhase: raise ValueError()
        phase._guard(); row, revision = phase._ledger()
        marker = copy.deepcopy(phase.journal.state['steps'][STEP])
        plan = copy.deepcopy(phase.journal.state['steps'][PLAN_STEP])
        if (marker != phase.record or marker.get('status') != 'prepared' or row.get(LEDGER) != marker or
                marker.get('plan_sha256') != writer_cut.digest(plan) or plan.get('mode') not in ('create', 'replace', 'preserve')):
            raise ValueError()
        roots, source = phase._dependencies(token); phase._ca(source)
        actual, data = phase._read_store(); key = phase._key()
        if actual is None:
            prior = None
        else:
            prior = {'uid': actual['metadata']['uid'], 'resourceVersion': actual['metadata']['resourceVersion'],
                     'data_hmac': _mac(key, data)}
        if (plan.get('old') != prior or (plan['mode'] == 'create') != (actual is None) or
                plan.get('media_ca') != source['ca'] or plan.get('jdk_receipt_sha256') != writer_cut.digest(roots) or
                plan.get('issuer_receipt_sha256') != writer_cut.digest(source) or
                plan.get('root_count') != len(roots['root_fingerprints']) or
                plan.get('operation_id') != phase.fence.context['operation_id'] or
                plan.get('source_sha') != phase.fence.context['commit'] or plan.get('namespace_uid') != phase.namespace_uid):
            raise ValueError()
        phase._guard()
        if (phase._ledger() != (row, revision) or phase.journal.state['steps'].get(STEP) != marker or
                phase.journal.state['steps'].get(PLAN_STEP) != plan): raise ValueError()
        phase._locals(row, writer_cut.digest(roots), writer_cut.digest(source))
        return plan
    except target.metadata.GuardLost: raise
    except BaseException: raise TrustEffectError('creator trust preflight changed') from None


def verify_current(phase, token):
    try:
        if type(phase) is not TrustEffectPhase: raise ValueError()
        phase._guard(); row, revision = phase._ledger()
        local = copy.deepcopy(phase.journal.state['steps'][STEP]); receipt = local['receipt']
        marker = {k: v for k, v in local.items() if k != 'receipt'}
        if (marker != phase.record or marker.get('status') != 'done' or row.get(LEDGER) != marker or
                marker.get('receipt_sha256') != writer_cut.digest(receipt)): raise ValueError()
        plan = phase.journal.state['steps'][PLAN_STEP]
        if (marker.get('plan_sha256') != writer_cut.digest(plan) or
                receipt.get('mode') != plan.get('mode') or receipt.get('old') != plan.get('old') or
                receipt.get('media_ca') != plan.get('media_ca') or
                receipt.get('jdk_receipt_sha256') != plan.get('jdk_receipt_sha256') or
                receipt.get('issuer_receipt_sha256') != plan.get('issuer_receipt_sha256') or
                receipt.get('root_count') != plan.get('root_count') or
                receipt.get('store_count') != plan.get('root_count', -2) + 1): raise ValueError()
        roots, source = phase._dependencies(token); phase._ca(source)
        actual, data = phase._read_store(); key = phase._key()
        if (actual is None or receipt['secret'] != {k: actual['metadata'][k] for k in ('uid', 'resourceVersion')} or
                not hmac.compare_digest(receipt['data_hmac'], _mac(key, data)) or
                receipt['jdk_receipt_sha256'] != writer_cut.digest(roots) or
                receipt['issuer_receipt_sha256'] != writer_cut.digest(source) or receipt['media_ca'] != source['ca']):
            raise ValueError()
        phase._guard()
        if phase._ledger() != (row, revision) or phase.journal.state['steps'].get(STEP) != local: raise ValueError()
        phase._locals(row, writer_cut.digest(roots), writer_cut.digest(source))
        return copy.deepcopy(receipt)
    except target.metadata.GuardLost: raise
    except BaseException: raise TrustEffectError('creator trust Secret receipt unavailable') from None
