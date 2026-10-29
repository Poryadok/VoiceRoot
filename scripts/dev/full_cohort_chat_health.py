"""Fenced Chat P3 readiness and protected-listener observations.

The HTTP health and mTLS/h2 transport checks do not invoke the lifecycle RPC
or establish business authorization. Private client keys never enter receipts.
Requires the source-owned Realtime configuration and Chat Service successor
verifiers; missing dependencies fail closed and are not deployment admission.

Call ChatHealthPhase.run(token, cut) after the admitted Chat resume and before
the Realtime resume. WriterCut accepts the completed phase through
verify_completed(phase, cut, token), which repeats both observations and checks
the private receipt against the same active permanent ledger. The phase must
run on the main thread without an existing ITIMER_REAL timer. C1 uses the
actual P3 creation and migration phases; C2 uses its predecessor journal.
"""
import base64
import copy
import contextlib
import ipaddress
import json
import os
from pathlib import Path
import re
import secrets
import selectors
import signal
import socket
import ssl
import subprocess
import time
import threading


class HealthError(RuntimeError):
    pass


class _Deadline(BaseException):
    pass


@contextlib.contextmanager
def _bounded_observation(seconds):
    if threading.current_thread() is not threading.main_thread(): raise HealthError('Chat health requires main thread')
    old_handler = signal.getsignal(signal.SIGALRM)
    if signal.getitimer(signal.ITIMER_REAL) != (0.0, 0.0): raise HealthError('Chat health timer already in use')
    def expired(signum, frame): raise _Deadline()
    signal.signal(signal.SIGALRM, expired)
    signal.setitimer(signal.ITIMER_REAL, seconds)
    try:
        yield
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, old_handler)


def _unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result: raise ValueError()
        result[key] = value
    return result


def _authority_call(function, *args):
    """An unavailable protected dependency is never a resumable health result."""
    import full_cohort_schema_metadata as common
    try:
        return function(*args)
    except Exception:
        raise common.GuardLost('Chat health dependency unavailable') from None


def check_health(raw):
    try:
        if not isinstance(raw, bytes) or len(raw) > 4096: raise ValueError()
        result = json.loads(raw, object_pairs_hook=_unique)
        if result != {'service':'chat', 'status':'ok'}: raise ValueError()
    except (ValueError, TypeError):
        raise HealthError('Chat readiness unavailable') from None


def check_h2_settings(frame):
    if (not isinstance(frame, bytes) or len(frame) < 9 or frame[3] != 4 or frame[4] != 0 or
            frame[5:9] != b'\0\0\0\0' or int.from_bytes(frame[:3], 'big') != len(frame) - 9 or
            len(frame) > 16393 or (len(frame) - 9) % 6):
        raise HealthError('Chat protected transport unavailable')


def _private_material(secret_identity, ca_identity, guard):
    """Read only the fixed client TLS keys and public CA into private memory."""
    import full_cohort_schema_metadata as common
    import realtime_p3_create_only_transport as transport
    try:
        common._fence(guard)
        template = ('{{.metadata.uid}}{{"\\n"}}{{.metadata.resourceVersion}}{{"\\n"}}'
                    '{{index .data "tls.crt"}}{{"\\n"}}{{index .data "tls.key"}}')
        raw = transport._run(['get', 'secret', 'voice-realtime-p3-tls', '-o', 'go-template=' + template])
        common._fence(guard)
        uid, rv, cert, key = raw.split(b'\n')
        if {'uid':uid.decode(), 'resourceVersion':rv.decode()} != secret_identity: raise ValueError()
        cert, key = base64.b64decode(cert, validate=True), base64.b64decode(key, validate=True)
        ca_raw = transport.public_certificate('voice-principal-ca', 'ca.crt')
        common._fence(guard)
        uid, rv, ca = ca_raw.split(b'\n')
        if {'uid':uid.decode(), 'resourceVersion':rv.decode()} != ca_identity: raise ValueError()
        ca = base64.b64decode(ca, validate=True)
        if any(not value or len(value) > 131072 for value in (cert, key, ca)): raise ValueError()
        return cert, key, ca
    except common.GuardLost:
        raise
    except Exception:
        raise HealthError('Chat client identity unavailable') from None


def _recv_exact(stream, size, deadline):
    result = bytearray()
    while len(result) < size:
        remaining = deadline - time.monotonic()
        if remaining <= 0: raise TimeoutError()
        stream.settimeout(remaining)
        chunk = stream.recv(size - len(result))
        if not chunk: raise ValueError()
        result.extend(chunk)
    return bytes(result)


def _reject_without_client_cert(port, ca, deadline):
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
    context.minimum_version = context.maximum_version = ssl.TLSVersion.TLSv1_3
    context.load_verify_locations(cadata=ca.decode('ascii')); context.set_alpn_protocols(['h2'])
    peer = None
    try:
        with socket.create_connection(('127.0.0.1', port), timeout=max(.01, deadline-time.monotonic())) as connection:
            connection.settimeout(max(.01, deadline-time.monotonic()))
            with context.wrap_socket(connection, server_hostname='voice-chat.voice-dev.svc') as tls:
                peer = tls.getpeercert(binary_form=True)
                if not peer or tls.selected_alpn_protocol() != 'h2': raise ValueError()
                _recv_exact(tls, 9, deadline)
    except ssl.SSLError as error:
        if peer and error.reason == 'TLSV13_ALERT_CERTIFICATE_REQUIRED': return peer
        raise
    raise ValueError()


def _protected_listener(pod_name, material):
    """Connect to one Pod through an owned, bounded local port-forward."""
    import frontend_runtime_executor
    if not re.fullmatch(r'voice-chat-[a-z0-9-]+', pod_name): raise HealthError('Chat Pod invalid')
    process = None; selector = selectors.DefaultSelector(); fds = []
    deadline = time.monotonic() + 15
    try:
        cert, key, ca = material
        for value in (cert, key):
            fd = os.memfd_create('voice-chat-health', os.MFD_CLOEXEC)
            fds.append(fd); os.fchmod(fd, 0o600)
            written = 0
            while written < len(value):
                count = os.write(fd, value[written:])
                if count <= 0: raise ValueError()
                written += count
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        context.minimum_version = ssl.TLSVersion.TLSv1_2
        context.load_verify_locations(cadata=ca.decode('ascii'))
        context.load_cert_chain('/proc/self/fd/' + str(fds[0]), '/proc/self/fd/' + str(fds[1]))
        context.set_alpn_protocols(['h2'])
        args = frontend_runtime_executor.KubectlTransport.PREFIX + [
            'port-forward', '--address=127.0.0.1', 'pod/' + pod_name, ':9099']
        process = subprocess.Popen(args, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                   stderr=subprocess.DEVNULL, start_new_session=True)
        os.set_blocking(process.stdout.fileno(), False); selector.register(process.stdout, selectors.EVENT_READ)
        output = bytearray(); port = None
        while port is None:
            remaining = deadline - time.monotonic()
            if remaining <= 0 or not selector.select(remaining): raise TimeoutError()
            chunk = os.read(process.stdout.fileno(), 1025 - len(output))
            if not chunk: raise ValueError()
            output.extend(chunk)
            if len(output) > 1024: raise ValueError()
            if b'\n' in output:
                line = bytes(output).split(b'\n', 1)[0]
                match = re.fullmatch(rb'Forwarding from 127\.0\.0\.1:([0-9]{1,5}) -> 9099', line)
                if not match or not 1 <= int(match[1]) <= 65535: raise ValueError()
                port = int(match[1])
        expected_peer = _reject_without_client_cert(port, ca, deadline)
        with socket.create_connection(('127.0.0.1', port), timeout=max(.01, deadline-time.monotonic())) as connection:
            connection.settimeout(max(.01, deadline-time.monotonic()))
            with context.wrap_socket(connection, server_hostname='voice-chat.voice-dev.svc') as tls:
                if tls.selected_alpn_protocol() != 'h2' or tls.getpeercert(binary_form=True) != expected_peer: raise ValueError()
                tls.sendall(b'PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n' + b'\0\0\0\x04\0\0\0\0\0')
                header = _recv_exact(tls, 9, deadline); size = int.from_bytes(header[:3], 'big')
                if size > 16384: raise ValueError()
                check_h2_settings(header + _recv_exact(tls, size, deadline))
        if process.poll() is not None: raise ValueError()
    except Exception:
        raise HealthError('Chat protected transport unavailable') from None
    finally:
        selector.close()
        if process is not None:
            if process.poll() is None:
                process.terminate()
                try: process.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    process.kill(); process.wait(timeout=3)
            if process.stdout is not None: process.stdout.close()
        for fd in fds: os.close(fd)


def _workloads(client, image_phase, cut, package):
    """Resolve every ready Chat Pod through the actual resumed Deployment."""
    import full_cohort_schema_metadata as common
    common._fence(cut._guard); _authority_call(image_phase.verify_activation_lineage)
    row, _ = image_phase.fence.ledger.read()
    if (not image_phase.started or not image_phase.fence.owned(row) or row.get('status') != 'active' or
            row.get('full_cohort_invocation') != {'nonce':image_phase.invocation,
                                                   'operation_id':image_phase.fence.context['operation_id']}):
        raise HealthError('Chat image authority unavailable')
    _authority_call(image_phase._records, row); _authority_call(image_phase._accounts)
    proof = _authority_call(cut.current_resumed_proof)
    selected = [x for x in proof['controllers'] if x.get('kind') == 'Deployment' and x.get('name') == 'voice-chat']
    if len(selected) != 1: raise HealthError('resumed Chat controller unavailable')
    target = selected[0]; replicas = target['replicas']
    if type(replicas) is not int or not 1 <= replicas <= 64: raise HealthError('resumed Chat replicas unavailable')
    image_record = image_phase.journal.state['steps'].get('full-cohort-image-voice-chat', {})
    image_receipt = image_record.get('receipt', {})
    if (image_record.get('status') != 'done' or image_receipt.get('uid') != target['uid'] or
            image_receipt.get('spec') != image_phase._target('voice-chat') or
            not image_receipt.get('resourceVersion')):
        raise HealthError('Chat image effect unavailable')
    if (package['images'].get('voice-chat/chat') != image_phase.fence.context['images'].get('voice-chat/chat') or
            package['platform_digests'].get('voice-chat/chat') !=
            image_phase.intent['full_cohort']['platform_digests'].get('voice-chat/chat')):
        raise HealthError('Chat package identity changed')
    deployment = client.call('get', 'deployment', 'voice-chat', '-o', 'json')
    meta, status = deployment['metadata'], deployment.get('status', {})
    if (meta.get('uid') != target['uid'] or meta.get('generation') != target['generation'] or
            meta.get('deletionTimestamp') or deployment.get('spec') != target['spec'] or
            status.get('observedGeneration', 0) < target['generation'] or
            status.get('updatedReplicas') != replicas or status.get('availableReplicas') != replicas):
        raise HealthError('Chat Deployment unavailable')
    replicasets = client.call('get', 'replicasets', '-l', 'app=voice-chat', '-o', 'json')['items']
    pods = client.call('get', 'pods', '-l', 'app=voice-chat', '-o', 'json')['items']
    owners = {}
    for rs in replicasets:
        meta = rs['metadata']
        if (meta.get('namespace') != 'voice-dev' or meta.get('deletionTimestamp') or
                image_phase._controller(meta, 'Deployment') != target['uid'] or
                not meta.get('uid') or not meta.get('resourceVersion') or meta['uid'] in owners):
            raise HealthError('Chat ReplicaSet identity invalid')
        owners[meta['uid']] = {'uid':meta['uid'], 'resourceVersion':meta['resourceVersion']}
    if len(pods) != replicas: raise HealthError('Chat Pod count invalid')
    result = []
    for pod in pods:
        meta = pod['metadata']; owner = image_phase._controller(meta, 'ReplicaSet')
        if owner not in owners or not image_phase._valid_pod('voice-chat', pod, target['spec']):
            raise HealthError('Chat Pod identity invalid')
        statuses = [x for x in pod['status']['containerStatuses'] if x.get('name') == 'chat']
        status = statuses[0]
        if (not re.fullmatch(r'voice-chat-[a-z0-9-]+', meta['name']) or not status.get('containerID') or
                'running' not in status.get('state', {}) or
                type(status.get('restartCount')) is not int or status['restartCount'] < 0):
            raise HealthError('Chat container identity invalid')
        ip = pod['status'].get('podIP')
        if not isinstance(ip, str) or str(ipaddress.ip_address(ip)) != ip: raise HealthError('Chat Pod address invalid')
        result.append({'name':meta['name'], 'uid':meta['uid'], 'resourceVersion':meta['resourceVersion'], 'ip':ip,
                       'replicaset':owners[owner], 'container_id':status['containerID'],
                       'restart_count':status['restartCount'], 'image_id':status['imageID']})
    if len({x['uid'] for x in result}) != replicas: raise HealthError('Chat Pod identity invalid')
    service = client.call('get', 'service', 'voice-chat', '-o', 'json')
    sm, spec = service['metadata'], service['spec']
    ports = [p for p in spec.get('ports', []) if p.get('name') == 'realtime-lifecycle']
    if (sm.get('name') != 'voice-chat' or sm.get('namespace') != 'voice-dev' or
            not sm.get('uid') or not sm.get('resourceVersion') or sm.get('deletionTimestamp') or
            spec.get('selector') != {'app':'voice-chat'} or spec.get('type', 'ClusterIP') != 'ClusterIP' or
            not spec.get('clusterIP') or spec['clusterIP'] == 'None' or len(ports) != 1 or
            ports[0].get('port') != 9099 or ports[0].get('targetPort', 9099) != 9099 or
            ports[0].get('protocol', 'TCP') != 'TCP'):
        raise HealthError('Chat Service unavailable')
    slices = client.call('get', 'endpointslices', '-l', 'kubernetes.io/service-name=voice-chat', '-o', 'json')['items']
    targets = {}; slice_ids = []
    for item in slices:
        smeta = item['metadata']
        if (smeta.get('namespace') != 'voice-dev' or smeta.get('deletionTimestamp') or
                smeta.get('labels', {}).get('kubernetes.io/service-name') != 'voice-chat' or
                image_phase._controller(smeta, 'Service') != sm['uid'] or
                not smeta.get('uid') or not smeta.get('resourceVersion')): raise HealthError('Chat endpoint slice invalid')
        matches = [p for p in item.get('ports', []) if p.get('name') == 'realtime-lifecycle']
        if len(matches) != 1 or matches[0].get('port') != 9099 or matches[0].get('protocol', 'TCP') != 'TCP':
            raise HealthError('Chat endpoint port invalid')
        slice_ids.append({'uid':smeta['uid'], 'resourceVersion':smeta['resourceVersion']})
        for endpoint in item.get('endpoints', []):
            ref = endpoint.get('targetRef', {}); conditions = endpoint.get('conditions', {})
            uid = ref.get('uid')
            if (ref.get('kind') != 'Pod' or ref.get('namespace') != 'voice-dev' or not uid or uid in targets or
                    conditions.get('ready') is not True or conditions.get('terminating') is True):
                raise HealthError('Chat endpoint invalid')
            targets[uid] = {'name':ref.get('name'), 'addresses':endpoint.get('addresses')}
    if set(targets) != {p['uid'] for p in result} or any(
            targets[p['uid']] != {'name':p['name'], 'addresses':[p['ip']]} for p in result):
        raise HealthError('Chat endpoint target changed')
    common._fence(cut._guard); _authority_call(image_phase.verify_activation_lineage); _authority_call(image_phase._accounts)
    return {'controller_uid':target['uid'], 'generation':target['generation'],
            'spec_sha256':_digest(target['spec']), 'pods':sorted(result, key=lambda item:item['name']),
            'service':{'uid':sm['uid'], 'resourceVersion':sm['resourceVersion'], 'cluster_ip':spec['clusterIP']},
            'endpoint_slices':sorted(slice_ids, key=lambda item:item['uid'])}


def _digest(value):
    import hashlib
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()).hexdigest()


def _health(pod_name):
    import frontend_runtime_executor
    import full_cohort_schema_metadata
    if not re.fullmatch(r'voice-chat-[a-z0-9-]+', pod_name): raise HealthError('Chat Pod invalid')
    path = '/api/v1/namespaces/voice-dev/pods/' + pod_name + ':8080/proxy/health'
    raw = full_cohort_schema_metadata._bounded_process(frontend_runtime_executor.KubectlTransport.PREFIX +
                                                        ['get', '--raw=' + path], 4096)
    check_health(raw)


STEP = 'full-cohort-chat-health'
LEDGER_KEY = 'full_cohort_chat_health'
LEDGER = LEDGER_KEY


class ChatHealthPhase:
    def __init__(self, journal, fence, client, lock, namespace_uid, image_phase,
                 package_phase, config_journal, source_root, *, p3_session=None,
                 migration=None, predecessor_journal=None):
        import frontend_runtime_executor
        import full_cohort_image_apply
        import full_cohort_package_phase
        import media_operation_journal
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.image_phase, self.package_phase = image_phase, package_phase
        self.config_journal, self.source_root = config_journal, Path(source_root)
        self.p3_session, self.migration, self.predecessor_journal = p3_session, migration, predecessor_journal
        self.intent = copy.deepcopy(journal.intent); self.poisoned = False
        if (type(journal) is not media_operation_journal.Journal or
                type(config_journal) is not media_operation_journal.Journal or
                type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal or
                type(image_phase) is not full_cohort_image_apply.FullCohortImageApply or
                type(package_phase) is not full_cohort_package_phase.PackagePhase or
                not self.source_root.is_absolute() or fence.ledger.client is not client or
                self.intent.get('resource_context') != fence.context or self.intent.get('namespace_uid') != namespace_uid or
                not re.fullmatch(r'media-c[12]-[a-f0-9]{32}', fence.context.get('operation_id', ''))):
            raise HealthError('Chat health phase identity invalid')

    def _guard(self, cut):
        import full_cohort_schema_metadata as common
        try:
            self._check_guard(cut)
        except Exception:
            raise common.GuardLost('Chat health guard unavailable') from None

    def _check_guard(self, cut):
        import media_operation_guard
        import writer_cut
        if (self.poisoned or self.journal.poisoned or self.journal.intent != self.intent or
                self.journal.state.get('intent') != self.intent or type(cut) is not writer_cut.WriterCut):
            raise HealthError('Chat health phase changed')
        for item in (cut, self.image_phase, self.package_phase):
            if (item.fence.context != self.fence.context or item.lock != self.lock or
                    item.namespace_uid != self.namespace_uid or item.fence.ledger.client is not item.client):
                raise HealthError('Chat health fence differs')
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)
        cut._guard(); self.image_phase._identity()
        media_operation_guard.verify(self.image_phase.fence, self.image_phase.client,
                                     self.image_phase.lock, self.image_phase.namespace_uid)
        self.package_phase._guard()
        row, rev = self.fence.ledger.read()
        for item in (cut, self.image_phase, self.package_phase):
            if item.fence.ledger.read() != (row, rev): raise HealthError('Chat health ledger differs')

    def _source(self, cut):
        import realtime_config_transition as config
        if self.fence.context['operation_id'].startswith('media-c1-'):
            import realtime_config_transition_inputs as inputs
            proof = _authority_call(inputs.observe, cut, self.p3_session, self.migration, self.source_root)
        else:
            import realtime_p3_existing as existing
            proof = _authority_call(existing.verify_reuse, self.fence, self.client, self.lock, self.namespace_uid,
                                    self.predecessor_journal, self.source_root)
        deployment = _authority_call(config.verify_deployment_successor, cut, self.config_journal, 'chat')
        service = _authority_call(config.verify_chat_service, cut, self.config_journal)
        if (set(deployment) != {'source','target_spec','receipt','ledger_record'} or
                set(service) != {'source','target_spec','receipt','ledger_record'}):
            raise HealthError('Chat config authority unavailable')
        obj = proof['objects']['voice-realtime-p3-tls']
        ca = proof['public_sources']['voice-principal-ca']
        if obj.get('kind') != 'Secret': raise HealthError('Chat TLS source invalid')
        secret_identity = {key:obj[key] for key in ('uid','resourceVersion')}
        ca_identity = {key:ca[key] for key in ('uid','resourceVersion')}
        return {'secret':secret_identity, 'ca':ca_identity, 'ca_der_sha256':ca['der_sha256'],
                'deployment':deployment, 'service':service}

    def _observe(self, token, cut):
        with _bounded_observation(180):
            return self._observe_bounded(token, cut)

    def _observe_bounded(self, token, cut):
        import hashlib
        deadline = time.monotonic() + 180
        def guarded():
            if time.monotonic() >= deadline: raise HealthError('Chat health deadline elapsed')
            self._guard(cut)
            if time.monotonic() >= deadline: raise HealthError('Chat health deadline elapsed')
        guarded()
        package = _authority_call(self.package_phase.verify_current, token)
        source = self._source(cut)
        resumed = [item for item in _authority_call(cut.current_resumed_proof)['controllers']
                   if item.get('kind') == 'Deployment' and item.get('name') == 'voice-chat']
        if len(resumed) != 1: raise HealthError('Chat controller not resumed')
        expected = copy.deepcopy(source['deployment']['target_spec'])
        containers = [item for item in expected['template']['spec']['containers'] if item.get('name') == 'chat']
        if len(containers) != 1: raise HealthError('Chat config target invalid')
        containers[0]['image'] = self.fence.context['images']['voice-chat/chat']
        expected['replicas'] = resumed[0]['replicas']
        if expected != resumed[0]['spec']: raise HealthError('Chat config successor differs')
        snapshot = _workloads(self.client, self.image_phase, cut, package)
        if any(source['service']['receipt'].get(key) != snapshot['service'][key]
               for key in ('uid', 'resourceVersion')):
            raise HealthError('Chat Service identity differs')
        material = _private_material(source['secret'], source['ca'], guarded)
        ca_der = ssl.PEM_cert_to_DER_cert(material[2].decode('ascii'))
        if hashlib.sha256(ca_der).hexdigest() != source['ca_der_sha256']: raise HealthError('Chat CA changed')
        cert_der = ssl.PEM_cert_to_DER_cert(material[0].decode('ascii'))
        for pod in snapshot['pods']:
            guarded(); _health(pod['name']); guarded()
            _protected_listener(pod['name'], material); guarded()
        if (_private_material(source['secret'], source['ca'], guarded) != material or
                _workloads(self.client, self.image_phase, cut, package) != snapshot or self._source(cut) != source):
            raise HealthError('Chat health inputs changed')
        guarded()
        return {'operation_id':self.fence.context['operation_id'], 'source_sha':self.fence.context['commit'],
                'namespace_uid':self.namespace_uid, 'cut_invocation':cut.invocation, 'workloads':snapshot,
                'client_secret':source['secret'], 'principal_ca':source['ca'],
                'principal_ca_der_sha256':source['ca_der_sha256'],
                'client_cert_der_sha256':hashlib.sha256(cert_der).hexdigest(),
                'config_receipt_sha256':_digest(source['deployment']['receipt']),
                'service_receipt_sha256':_digest(source['service']['receipt']),
                'scope':'chat-readiness-and-protected-listener-only'}

    def run(self, token, cut):
        pending = False
        try:
            self._guard(cut); row, revision = self.fence.ledger.read()
            if STEP in self.journal.state['steps'] or LEDGER_KEY in row: raise HealthError('Chat health already attempted')
            marker = {'status':'pending', 'nonce':secrets.token_hex(16),
                      'operation_id':self.fence.context['operation_id'], 'source_sha':self.fence.context['commit'],
                      'cut_invocation':cut.invocation}
            pending = True; self.journal.state['steps'][STEP] = copy.deepcopy(marker); self.journal.save()
            self._guard(cut)
            if self.fence.ledger.read() != (row, revision): raise HealthError('Chat health ledger changed')
            row[LEDGER_KEY] = copy.deepcopy(marker); self.fence.ledger.write(revision, row)
            self._guard(cut); receipt = self._observe(token, cut)
            row, revision = self.fence.ledger.read()
            if row.get(LEDGER_KEY) != marker or self.journal.state['steps'].get(STEP) != marker:
                raise HealthError('Chat health reservation changed')
            done = marker | {'status':'done', 'receipt_sha256':_digest(receipt)}
            self.journal.state['steps'][STEP] = done | {'receipt':copy.deepcopy(receipt)}; self.journal.save()
            self._guard(cut)
            if self.fence.ledger.read() != (row, revision): raise HealthError('Chat health ledger changed')
            row[LEDGER_KEY] = done; self.fence.ledger.write(revision, row)
            return verify_completed(self, cut, token)
        except BaseException as exc:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('Chat health observation uncertain')
                except BaseException: pass
            import full_cohort_schema_metadata as common
            if isinstance(exc, common.GuardLost): raise
            raise HealthError('Chat health unavailable; reconcile if pending') from None


def verify_completed(phase, cut, token):
    import full_cohort_schema_metadata as common
    if type(phase) is not ChatHealthPhase: raise HealthError('Chat health phase invalid')
    try:
        phase._guard(cut); row, revision = phase.fence.ledger.read()
        local = copy.deepcopy(phase.journal.state['steps'][STEP]); receipt = local.pop('receipt')
        if (local.get('status') != 'done' or local.get('receipt_sha256') != _digest(receipt) or
                row.get(LEDGER_KEY) != local or local.get('operation_id') != phase.fence.context['operation_id'] or
                local.get('source_sha') != phase.fence.context['commit'] or local.get('cut_invocation') != cut.invocation):
            raise ValueError()
        if phase._observe(token, cut) != receipt: raise ValueError()
        phase._guard(cut)
        if phase.fence.ledger.read() != (row, revision) or phase.journal.state['steps'][STEP] != local | {'receipt':receipt}:
            raise ValueError()
        return copy.deepcopy(receipt)
    except common.GuardLost:
        raise
    except _Deadline:
        raise HealthError('Chat health deadline elapsed') from None
    except Exception:
        phase._guard(cut)
        raise HealthError('completed Chat health unavailable') from None
