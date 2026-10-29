"""Fenced HTTPS observation of the Realtime public JWKS publisher."""
import os
import base64
import copy
import hashlib
import ipaddress
import re
import secrets
import selectors
import socket
import ssl
import subprocess
import time

import realtime_p3_create_only_crypto as crypto
import realtime_p3_public_inputs as public
import realtime_jwks_publisher as publisher
import realtime_p3_create_only_transport as transport
import frontend_runtime_executor
import full_cohort_package_phase
import media_operation_guard
import writer_cut

HOST = 'voice-p3-principal-jwks.voice-dev.svc'
DEADLINE = 20


class PublisherHealthError(RuntimeError): pass

STEP = 'realtime-jwks-publisher-https'
LEDGER = 'realtime_jwks_publisher_https'


def _metadata(obj, name):
    meta = obj['metadata']
    if (meta.get('name') != name or meta.get('namespace') != publisher.NAMESPACE
            or meta.get('deletionTimestamp') or any(not isinstance(meta.get(k), str) or not meta[k]
                                                     for k in ('uid', 'resourceVersion'))): raise ValueError()
    return {'uid': meta['uid'], 'resourceVersion': meta['resourceVersion']}


def _routing(client, proof):
    """Bracket the tunnel with the selected Service and its sole ready endpoint."""
    runtime = proof['runtime']; name = publisher.NAME; podname = runtime['pod_name']
    pod = client.call('get', 'pod', podname, '-o', 'json')
    pm = _metadata(pod, podname)
    if pm != {'uid': runtime['pod_uid'], 'resourceVersion': runtime['pod_resourceVersion']}: raise ValueError()
    ip = pod.get('status', {}).get('podIP')
    if not isinstance(ip, str) or ipaddress.ip_address(ip).is_unspecified: raise ValueError()
    svc = client.call('get', 'service', name, '-o', 'json')
    sm = _metadata(svc, name); expected = proof['objects']['service']
    if sm != {k: expected[k] for k in ('uid', 'resourceVersion')}: raise ValueError()
    publisher.check_admission(svc, publisher.desired(proof['node']['hostname'])[2])
    if not svc['spec'].get('clusterIP') or publisher.digest(svc['spec']) != expected['spec_sha256']: raise ValueError()
    ep = client.call('get', 'endpoints', name, '-o', 'json'); em = _metadata(ep, name)
    subsets = ep.get('subsets', [])
    if len(subsets) != 1 or subsets[0].get('notReadyAddresses') or len(subsets[0].get('addresses', [])) != 1: raise ValueError()
    addr = subsets[0]['addresses'][0]; ref = addr.get('targetRef', {})
    if (addr.get('ip') != ip or ref.get('kind') != 'Pod' or ref.get('name') != podname or ref.get('uid') != pm['uid']
            or sum(p.get('name') == 'https' and p.get('port') == 8443 and p.get('protocol', 'TCP') == 'TCP'
                   for p in subsets[0].get('ports', [])) != 1): raise ValueError()
    slices = client.call('get', 'endpointslices', '-l', 'kubernetes.io/service-name=' + name, '-o', 'json')['items']
    if len(slices) != 1: raise ValueError()
    sl = slices[0]; slm = _metadata(sl, sl['metadata']['name'])
    owners = sl['metadata'].get('ownerReferences', [])
    if (sl['metadata'].get('labels', {}).get('kubernetes.io/service-name') != name
            or sl.get('addressType') != ('IPv4' if ipaddress.ip_address(ip).version == 4 else 'IPv6')
            or sum(o.get('kind') == 'Service' and o.get('uid') == sm['uid'] and o.get('controller') is True
                   for o in owners) != 1): raise ValueError()
    endpoints = sl.get('endpoints', [])
    if len(endpoints) != 1: raise ValueError()
    endpoint = endpoints[0]; target = endpoint.get('targetRef', {})
    if (endpoint.get('addresses') != [ip] or endpoint.get('conditions', {}).get('ready') is not True
            or target.get('kind') != 'Pod' or target.get('name') != podname or target.get('uid') != pm['uid']
            or sum(p.get('name') == 'https' and p.get('port') == 8443 and p.get('protocol', 'TCP') == 'TCP'
                   for p in sl.get('ports', [])) != 1): raise ValueError()
    return {'pod': pm, 'service': sm, 'endpoints': em, 'slice': slm, 'pod_ip': ip}


def _materials(phase, proof):
    obj = phase.client.call('get', 'configmap', publisher.PUBLIC_NAME, '-o', 'json')
    meta = _metadata(obj, publisher.PUBLIC_NAME)
    raw = obj['data']['realtime.json'].encode('utf-8')
    if (obj.get('immutable') is not True or obj.get('binaryData') or set(obj['data']) != {'realtime.json'}
            or meta != {k: proof['public'][k] for k in ('uid', 'resourceVersion')}
            or not 0 < len(raw) <= 65536 or hashlib.sha256(raw).hexdigest() != proof['public']['sha256']): raise ValueError()
    tls, cert = publisher._public_tls()
    if tls != {k: proof['tls'][k] for k in ('uid', 'resourceVersion')}: raise ValueError()
    source = phase.publisher
    if type(source) is publisher.PublisherPhase:
        source.p3._check_roots(); root = source.p3.roots[('voice-principal-ca', 'ca.crt')]
        ca = root['certificate']
    else:
        root = source._p3()['public_sources']['voice-principal-ca']
        lines = transport.public_certificate('voice-principal-ca', 'ca.crt').split(b'\n', 2)
        if len(lines) != 3 or lines[0].decode() != root['uid'] or lines[1].decode() != root['resourceVersion']: raise ValueError()
        ca = base64.b64decode(lines[2], validate=True)
        if hashlib.sha256(crypto.der(ca)).hexdigest() != root['der_sha256']: raise ValueError()
    if publisher._verify_cert(cert, ca) != proof['tls']['certificate_sha256']: raise ValueError()
    return raw, ca, cert


def _tunnel(pod, ca, certificate, guard):
    """Observe only the owned Pod's fixed HTTPS public document."""
    if not isinstance(pod, str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9]*[a-z0-9])?', pod) or len(pod) > 253:
        raise ValueError()
    guard(); deadline = time.monotonic() + DEADLINE
    process = subprocess.Popen(public.PREFIX + ['port-forward', '--address=127.0.0.1', 'pod/' + pod, ':8443'],
                               stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    try:
        selector = selectors.DefaultSelector(); selector.register(process.stdout, selectors.EVENT_READ)
        raw = bytearray(); port = None
        try:
            while time.monotonic() < deadline and len(raw) < 4096:
                guard(); events = selector.select(min(1, max(0, deadline - time.monotonic())))
                if not events:
                    if process.poll() is not None: raise ValueError()
                    continue
                chunk = os.read(process.stdout.fileno(), 4096 - len(raw))
                if not chunk: raise ValueError()
                raw.extend(chunk)
                found = re.search(rb'(?m)^Forwarding from 127\.0\.0\.1:([0-9]{1,5}) -> 8443\r?$', raw)
                if found: port = int(found.group(1)); break
        finally: selector.close()
        if port is None or not 1 <= port <= 65535: raise ValueError()
        ctx = ssl.create_default_context(cadata=ca.decode('ascii')); ctx.minimum_version = ssl.TLSVersion.TLSv1_2
        with socket.create_connection(('127.0.0.1', port), timeout=min(2, max(.01, deadline-time.monotonic()))) as tcp:
            with ctx.wrap_socket(tcp, server_hostname=HOST) as tls:
                tls.settimeout(1)
                if tls.getpeercert(binary_form=True) != crypto.der(certificate): raise ValueError()
                tls.sendall(b'GET /realtime.json HTTP/1.1\r\nHost: voice-p3-principal-jwks.voice-dev.svc\r\nConnection: close\r\n\r\n')
                response = bytearray(); body_start = length = None
                while time.monotonic() < deadline:
                    guard()
                    try: chunk = tls.recv(min(4096, 73728-len(response)))
                    except socket.timeout: continue
                    if not chunk: break
                    response.extend(chunk)
                    if body_start is None and b'\r\n\r\n' in response:
                        body_start = response.index(b'\r\n\r\n') + 4
                        if body_start > 8192: raise ValueError()
                        lines = bytes(response[:body_start-4]).split(b'\r\n')
                        if not re.fullmatch(rb'HTTP/1\.[01] 200(?: [^\r\n]*)?', lines[0]): raise ValueError()
                        headers = {}
                        for line in lines[1:]:
                            if b':' not in line: raise ValueError()
                            name, value = line.split(b':', 1); name = name.strip().lower()
                            if not name or name in headers: raise ValueError()
                            headers[name] = value.strip()
                        if b'transfer-encoding' in headers or not re.fullmatch(rb'[0-9]{1,5}', headers.get(b'content-length', b'')): raise ValueError()
                        length = int(headers[b'content-length'])
                        if not 0 < length <= 65536: raise ValueError()
                    if body_start is None and len(response) > 8192: raise ValueError()
                    if body_start is not None and len(response)-body_start >= length: break
                    if len(response) >= 73728: raise ValueError()
                if time.monotonic() >= deadline or body_start is None or len(response)-body_start != length: raise ValueError()
                value = bytes(response[body_start:])
        guard(); return value
    finally:
        process.terminate()
        try: process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill(); process.wait(timeout=5)
        process.stdout.close()


class PublisherHealthPhase:
    """One authenticated publisher observation under the active cut's fence."""
    def __init__(self, journal, fence, client, lock, namespace_uid, publisher_phase, package_phase):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.publisher, self.package = publisher_phase, package_phase
        self.intent = copy.deepcopy(journal.intent); self.record = None; self.plan_sha256 = None
        if (type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal
            or getattr(fence.ledger, 'client', None) is not client
            or type(publisher_phase) not in (publisher.PublisherPhase, publisher.PublisherReuse)
            or type(package_phase) is not full_cohort_package_phase.PackagePhase
            or self.intent.get('resource_context') != fence.context or self.intent.get('namespace_uid') != namespace_uid
            or any(p.fence.context != fence.context or p.lock != lock or p.namespace_uid != namespace_uid
                   for p in (publisher_phase, package_phase))): raise PublisherHealthError('publisher health identity invalid')

    def _guard(self, cut, token, stage=None):
        if (type(cut) is not writer_cut.WriterCut or cut.fence.context != self.fence.context
            or cut.lock != self.lock or cut.namespace_uid != self.namespace_uid or self.journal.poisoned
            or self.journal.intent != self.intent or self.journal.state.get('intent') != self.intent
            or stage is not None and getattr(cut, 'activation', {}).get('stage') != stage): raise ValueError()
        media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid)
        cut._guard(); self.publisher._guard(); self.package.verify_current(token)
        activation = cut.verify_activation_stage()
        if (type(activation) is not dict or set(activation) != {'operation_id', 'cut_invocation', 'plan_sha256', 'stage'}
            or activation['operation_id'] != self.fence.context['operation_id'] or activation['cut_invocation'] != cut.invocation
            or not isinstance(activation['plan_sha256'], str) or not re.fullmatch(r'[0-9a-f]{64}', activation['plan_sha256'])
            or activation['stage'] != getattr(cut, 'activation', {}).get('stage')
            or stage is not None and activation['stage'] != stage
            or self.plan_sha256 is not None and activation['plan_sha256'] != self.plan_sha256): raise ValueError()
        row, revision = self.fence.ledger.read()
        for other in (cut, self.publisher, self.package):
            value, version = other.fence.ledger.read()
            if value != row or version != revision: raise ValueError()
        if self.record is not None:
            local = self.journal.state['steps'].get(STEP)
            if row.get(LEDGER) != self.record or not isinstance(local, dict): raise ValueError()
            if {k: v for k, v in local.items() if k != 'receipt'} != self.record: raise ValueError()
            if self.record['status'] == 'done' and publisher.digest(local.get('receipt')) != self.record['receipt_sha256']: raise ValueError()
        if activation['stage'] != 'publisher_health':
            if self.record is None or self.record.get('status') != 'done': raise ValueError()
            entry = cut.journal.state['steps']['writer-cut-publisher-health']
            details = {'operation_id':self.fence.context['operation_id'], 'cut_invocation':cut.invocation,
                       'activation_plan_sha256':activation['plan_sha256'], 'health_receipt_sha256':self.record['receipt_sha256']}
            if (set(entry) != {'status', 'nonce', 'details', 'details_sha256', 'receipt'} or entry['status'] != 'done'
                or entry['details'] != details or entry['receipt'] != details
                or entry['details_sha256'] != publisher.digest(details)
                or row.get('writer_cut_effects', {}).get('writer-cut-publisher-health') !=
                   {'status':'done', 'nonce':entry['nonce'], 'details_sha256':entry['details_sha256'],
                    'receipt_sha256':publisher.digest(details)}): raise ValueError()
        return activation

    def _observe(self, cut, token, stage):
        guard = lambda: self._guard(cut, token, stage)
        guard(); proof = publisher.verify_publisher(self.publisher)
        if (proof.get('operation_id') != self.fence.context['operation_id']
            or proof.get('candidate_source_sha') != self.fence.context['commit']
            or proof.get('namespace_uid') != self.namespace_uid): raise ValueError()
        first = _routing(self.client, proof); raw, ca, cert = _materials(self, proof)
        guard(); served = _tunnel(proof['runtime']['pod_name'], ca, cert, guard)
        if served != raw: raise ValueError()
        guard()
        if publisher.verify_publisher(self.publisher) != proof or _routing(self.client, proof) != first: raise ValueError()
        raw2, ca2, cert2 = _materials(self, proof)
        if (raw2, ca2, cert2) != (raw, ca, cert): raise ValueError()
        guard()
        return {'schema': 'voice-realtime-jwks-https-v1', 'operation_id': self.fence.context['operation_id'],
                'source_sha': self.fence.context['commit'], 'namespace_uid': self.namespace_uid,
                'cut_invocation': cut.invocation, 'activation_plan_sha256': self.plan_sha256,
                'publisher_sha256': publisher.digest(proof),
                'routing': first, 'ca_sha256': hashlib.sha256(crypto.der(ca)).hexdigest(),
                'certificate_sha256': proof['tls']['certificate_sha256'], 'jwks_sha256': proof['public']['sha256'],
                'scope': 'publisher-pod-https-through-kubernetes-tunnel'}

    def run(self, token, cut):
        pending = False
        try:
            activation = self._guard(cut, token, 'publisher_health')
            self.plan_sha256 = activation['plan_sha256']
            row, rev = self.fence.ledger.read()
            if STEP in self.journal.state['steps'] or LEDGER in row: raise ValueError()
            record = {'status': 'pending', 'nonce': secrets.token_hex(16), 'operation_id': self.fence.context['operation_id'],
                      'source_sha': self.fence.context['commit'], 'namespace_uid': self.namespace_uid,
                      'cut_invocation': cut.invocation, 'activation_plan_sha256': self.plan_sha256}
            pending = True; self.journal.state['steps'][STEP] = copy.deepcopy(record); self.journal.save()
            current, version = self.fence.ledger.read()
            if current != row or version != rev: raise ValueError()
            current[LEDGER] = copy.deepcopy(record); current['step'] = STEP + '-pending'
            self.fence.ledger.write(rev, current); self.record = copy.deepcopy(record)
            receipt = self._observe(cut, token, 'publisher_health')
            self._guard(cut, token, 'publisher_health'); row, rev = self.fence.ledger.read()
            done = record | {'status': 'done', 'receipt_sha256': publisher.digest(receipt)}
            self.journal.state['steps'][STEP] = copy.deepcopy(done) | {'receipt': copy.deepcopy(receipt)}; self.journal.save()
            current, version = self.fence.ledger.read()
            if current != row or version != rev: raise ValueError()
            current[LEDGER] = copy.deepcopy(done); current['step'] = STEP + '-done'
            self.fence.ledger.write(rev, current); self.record = copy.deepcopy(done)
            self._guard(cut, token, 'publisher_health'); return copy.deepcopy(receipt)
        except BaseException:
            if pending:
                try: self.fence.uncertain('publisher HTTPS observation uncertain')
                except BaseException: pass
            raise PublisherHealthError('publisher HTTPS observation unavailable') from None


def verify_completed(phase, cut, token):
    try:
        if type(phase) is not PublisherHealthPhase or phase.record is None or phase.record.get('status') != 'done': raise ValueError()
        stage = getattr(cut, 'activation', {}).get('stage')
        if stage not in ('publisher_health', 'space', 'space_health', 'chat', 'chat_health', 'realtime', 'remaining', 'done'): raise ValueError()
        phase._guard(cut, token, stage)
        receipt = copy.deepcopy(phase.journal.state['steps'][STEP]['receipt'])
        if phase._observe(cut, token, stage) != receipt: raise ValueError()
        phase._guard(cut, token, stage); return receipt
    except Exception:
        if type(phase) is PublisherHealthPhase and phase.record is not None:
            try: phase.fence.uncertain('publisher HTTPS proof changed')
            except BaseException: pass
        raise PublisherHealthError('publisher HTTPS proof unavailable') from None
