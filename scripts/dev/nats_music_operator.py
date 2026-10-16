#!/usr/bin/env python3
"""Root-only, one-shot DEV Messaging Music NATS actor maintenance.

The plan is read-only. Apply requires its exact plan digest, re-captures all
state, verifies the existing signed identity, and only renews the current
Messaging actor when the Music INFO/ACK additions alone are missing. Consumer
creation belongs to the separately stored bootstrap identity and is never run
here.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import selectors
import socket
import stat
import subprocess
import sys
import tempfile
import threading
import time

NS = 'voice-dev'
NS_UID = '15950fc8-0392-4053-9156-e95e78b596f9'
PROFILE = 'voice-dev-current-deployment'
NATS_DEPLOYMENT = 'voice-nats-pvc-candidate'
NATS_SELECTOR = {'app': NATS_DEPLOYMENT}
LABELS = {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'}
ROOT = Path('/home/voice-dev/operator')
ISSUER = ROOT / 'nats-jwt-issuer'
PROBE = ROOT / 'nats-dev-music-probe'
KUBECONFIG = ROOT / 'kubeconfig'
SEED = Path('/home/voice-dev/issuance/generation1/signing-seeds/app-account.seed')
BOOTSTRAP_PREPARED = Path('/home/voice-dev/bootstrap/prepared.json')
BOOTSTRAP_ENROLLMENT_JOURNAL = ROOT
BOOTSTRAP_CONFIGMAP_ENROLLMENT = 'voice-nats-analytics-chat-bootstrap-music'
ACL = Path(__file__).resolve().parents[2] / 'deploy/nats/acl-intent.yaml'
PKG = Path(__file__).resolve().parents[2] / 'src/backend/pkg'
STAGING = Path(__file__).resolve().parents[1] / 'staging/nats-rollout-preservation'
SERVICE_ADDITIONS = (
    '$JS.API.CONSUMER.INFO.file_events.messaging_music_metadata',
    '$JS.ACK.file_events.messaging_music_metadata.>',
)
BOOTSTRAP_ADDITIONS = ('$JS.API.CONSUMER.CREATE.file_events.messaging_music_metadata',)
BOOTSTRAP_SECRET = 'voice-nats-bootstrap-credentials'
MUSIC_BOOTSTRAP_CONTRACT = (
    'consumer file_events messaging_music_metadata file.processed '
    '_INBOX.voice.messaging.messaging_music_metadata new messaging_music_metadata')
MAX_PRIVATE = 1 << 20
TIMEOUT = 30


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False,
                      allow_nan=False).encode('utf-8')


def fail(code='music_operator_preflight_refused'):
    raise ValueError(code)


class Kube:
    def run(self, *args, body=None, timeout=20):
        try:
            result = subprocess.run(['kubectl', '--namespace', NS, *args], input=body,
                                    capture_output=True, timeout=timeout,
                                    env={'PATH': '/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin',
                                         'HOME': '/root', 'KUBECONFIG': str(KUBECONFIG)})
        except Exception:
            fail('kubernetes_transport_unavailable')
        if result.returncode:
            fail('kubernetes_request_refused')
        return result.stdout

    def get(self, kind, name, optional=False):
        if optional:
            raw = self.run('get', kind, name, '--ignore-not-found', '-o', 'json')
        else:
            raw = self.run('get', kind, name, '-o', 'json')
        if optional and not raw.strip():
            return None
        row = json.loads(raw)
        if not isinstance(row, dict):
            fail('kubernetes_object_invalid')
        return row

    def list(self, kind, selector=None):
        args = ['get', kind]
        if selector:
            args += ['-l', selector]
        args += ['-o', 'json']
        row = json.loads(self.run(*args))
        if not isinstance(row, dict) or not isinstance(row.get('items'), list):
            fail('kubernetes_list_invalid')
        return row['items']


def metadata(row, namespace=True):
    try:
        value = row['metadata']
        names = ('name', 'namespace', 'uid', 'resourceVersion') if namespace else ('name', 'uid', 'resourceVersion')
        result = {key: value[key] for key in names}
        if namespace and result['namespace'] != NS:
            fail('foreign_namespace')
        if not result['uid'] or not result['resourceVersion']:
            fail('resource_binding_missing')
        return result
    except (KeyError, TypeError):
        fail('resource_metadata_invalid')


def own(row, *, name=None, namespace=True, labels=True):
    meta = row.get('metadata', {})
    if (name is not None and meta.get('name') != name or
            namespace and meta.get('namespace') != NS or
            labels and any(meta.get('labels', {}).get(k) != v for k, v in LABELS.items())):
        fail('dev_resource_ownership_invalid')
    metadata(row, namespace)
    return row


def validate_bootstrap_enrollment(secret, source, baseline_binding_id):
    """Accept only a root-enrolled immutable credential bound to this DEV baseline."""
    if source is None:
        fail('bootstrap_source_secret_missing')
    meta = secret.get('metadata', {})
    annotations = meta.get('annotations', {})
    expected_labels = {**LABELS, 'voice.io/dev-music-binding-id': baseline_binding_id}
    expected_annotations = {
        'voice.io/dev-music-enrollment': 'voice-dev-music-bootstrap-enrollment-v1',
        'voice.io/dev-music-binding-id': baseline_binding_id,
        'voice.io/dev-music-namespace-uid': NS_UID,
        'voice.io/dev-music-source-secret-uid': source.get('metadata', {}).get('uid'),
        'voice.io/dev-music-source-secret-rv': source.get('metadata', {}).get('resourceVersion'),
    }
    journal_path = BOOTSTRAP_ENROLLMENT_JOURNAL / ('bootstrap-enrollment-' + baseline_binding_id + '.json')
    journal_raw = private_file(journal_path)
    try:
        journal = json.loads(journal_raw, object_pairs_hook=unique_json_pairs)
    except Exception:
        fail('bootstrap_enrollment_journal_invalid')
    expected_journal = {
        'schema': 'voice-dev-music-bootstrap-enrollment-v1',
        'baseline_binding_id': baseline_binding_id,
        'namespace_uid': NS_UID,
        'source_secret_uid': source.get('metadata', {}).get('uid'),
        'source_secret_rv': source.get('metadata', {}).get('resourceVersion'),
        'secret_name': BOOTSTRAP_SECRET + '-music-' + baseline_binding_id,
        'secret_uid': meta.get('uid'),
        'secret_rv': meta.get('resourceVersion'),
        'actor_sha256': annotations.get('voice.io/dev-music-actor-sha256'),
        'plan_sha256': annotations.get('voice.io/dev-music-plan-sha256'),
        'credential_sha256': sha(get_data(secret, 'bootstrap.creds')),
    }
    if (meta.get('name') != BOOTSTRAP_SECRET + '-music-' + baseline_binding_id or meta.get('namespace') != NS
            or meta.get('labels') != expected_labels
            or any(annotations.get(k) != v for k, v in expected_annotations.items())
            or not re.fullmatch(r'[a-f0-9]{64}', annotations.get('voice.io/dev-music-actor-sha256', ''))
            or not re.fullmatch(r'[a-f0-9]{64}', annotations.get('voice.io/dev-music-plan-sha256', ''))
            or set(annotations) != set(expected_annotations) | {
                'voice.io/dev-music-actor-sha256', 'voice.io/dev-music-plan-sha256'}
            or secret.get('immutable') is not True or secret.get('type') != 'Opaque'
            or set(secret.get('data', {})) != {'bootstrap.creds'}
            or journal != expected_journal):
        fail('bootstrap_enrollment_binding_invalid')
    return True


def bootstrap_configmap_name(baseline_binding_id, target_sha256):
    if (not re.fullmatch(r'dev-[a-f0-9]{24}', baseline_binding_id)
            or not re.fullmatch(r'[a-f0-9]{64}', target_sha256)):
        fail('bootstrap_configmap_source_invalid')
    return BOOTSTRAP_CONFIGMAP_ENROLLMENT + '-' + baseline_binding_id + '-' + target_sha256[:12]


def bootstrap_configmap_journal_path(baseline_binding_id):
    return BOOTSTRAP_ENROLLMENT_JOURNAL / ('bootstrap-configmap-enrollment-' + baseline_binding_id + '.json')


def validate_bootstrap_configmap_enrollment(configmap, source, target_data, baseline_binding_id):
    """Accept only a root-journaled immutable target ConfigMap for this source state."""
    if source is None or not isinstance(target_data, dict) or not target_data:
        fail('bootstrap_configmap_source_invalid')
    source_data = source.get('data')
    if not isinstance(source_data, dict) or not source_data:
        fail('bootstrap_configmap_base_invalid')
    target_sha = sha(canonical(target_data))
    source_sha = sha(canonical(source_data))
    name = bootstrap_configmap_name(baseline_binding_id, target_sha)
    meta = configmap.get('metadata', {})
    annotations = meta.get('annotations', {})
    journal_raw = private_file(bootstrap_configmap_journal_path(baseline_binding_id))
    try:
        journal = json.loads(journal_raw, object_pairs_hook=unique_json_pairs)
    except Exception:
        fail('bootstrap_configmap_journal_invalid')
    expected_annotations = {
        'voice.io/dev-music-configmap-enrollment': 'voice-dev-music-configmap-enrollment-v1',
        'voice.io/dev-music-binding-id': baseline_binding_id,
        'voice.io/dev-music-namespace-uid': NS_UID,
        'voice.io/dev-music-base-configmap-uid': source.get('metadata', {}).get('uid'),
        'voice.io/dev-music-base-configmap-rv': source.get('metadata', {}).get('resourceVersion'),
        'voice.io/dev-music-base-configmap-sha256': source_sha,
        'voice.io/dev-music-target-source-sha256': target_sha,
        'voice.io/dev-music-plan-sha256': annotations.get('voice.io/dev-music-plan-sha256'),
    }
    expected_journal = {
        'schema': 'voice-dev-music-configmap-enrollment-v1', 'name': name,
        'baseline_binding_id': baseline_binding_id, 'namespace_uid': NS_UID,
        'base_uid': source.get('metadata', {}).get('uid'),
        'base_resource_version': source.get('metadata', {}).get('resourceVersion'),
        'base_data_sha256': source_sha, 'target_source_sha256': target_sha,
        'configmap_uid': meta.get('uid'), 'configmap_resource_version': meta.get('resourceVersion'),
        'plan_sha256': annotations.get('voice.io/dev-music-plan-sha256'),
    }
    if (meta.get('name') != name or meta.get('namespace') != NS
            or meta.get('labels') != {**LABELS, 'voice.io/dev-music-binding-id': baseline_binding_id}
            or annotations != expected_annotations or configmap.get('immutable') is not True
            or not re.fullmatch(r'[a-f0-9]{64}', annotations.get('voice.io/dev-music-plan-sha256', ''))
            or configmap.get('data') != target_data or journal != expected_journal):
        fail('bootstrap_configmap_enrollment_binding_invalid')
    return True


def write_bootstrap_configmap_enrollment_journal(state, configmap, plan_sha256):
    root_info = ROOT.lstat()
    if not stat.S_ISDIR(root_info.st_mode) or root_info.st_uid != 0 or stat.S_IMODE(root_info.st_mode) != 0o700:
        fail('private_operator_directory_required')
    source = state['bootstrap_config']
    target_data = state['bootstrap_config_data']
    target_sha = sha(canonical(target_data))
    journal = {
        'schema': 'voice-dev-music-configmap-enrollment-v1',
        'name': configmap['metadata']['name'], 'baseline_binding_id': state['baseline_binding_id'],
        'namespace_uid': NS_UID, 'base_uid': source['metadata']['uid'],
        'base_resource_version': source['metadata']['resourceVersion'],
        'base_data_sha256': sha(canonical(source.get('data'))),
        'target_source_sha256': target_sha,
        'configmap_uid': configmap['metadata']['uid'],
        'configmap_resource_version': configmap['metadata']['resourceVersion'],
        'plan_sha256': plan_sha256,
    }
    path = bootstrap_configmap_journal_path(state['baseline_binding_id'])
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        with os.fdopen(fd, 'wb') as output:
            output.write(canonical(journal))
            output.flush()
            os.fsync(output.fileno())
        dir_fd = os.open(ROOT, os.O_RDONLY | getattr(os, 'O_DIRECTORY', 0))
        try:
            os.fsync(dir_fd)
        finally:
            os.close(dir_fd)
    except Exception:
        try:
            path.unlink()
        except OSError:
            pass
        raise
    return journal


def create_bootstrap_configmap_enrollment(state, plan_sha256, revalidate=None):
    """Create an immutable source-pinned copy; the shared static ConfigMap is untouched."""
    if revalidate is not None:
        revalidate()
    source = state.get('bootstrap_config')
    target_data = state.get('bootstrap_config_data')
    if source is None or not state.get('bootstrap_config_target_available') or not isinstance(target_data, dict):
        fail('bootstrap_configmap_source_invalid')
    target_sha = sha(canonical(target_data))
    name = bootstrap_configmap_name(state['baseline_binding_id'], target_sha)
    if state.get('bootstrap_config_enrollment') is not None:
        validate_bootstrap_configmap_enrollment(
            state['bootstrap_config_enrollment'], source, target_data, state['baseline_binding_id'])
        return state['bootstrap_config_enrollment']
    annotations = {
        'voice.io/dev-music-configmap-enrollment': 'voice-dev-music-configmap-enrollment-v1',
        'voice.io/dev-music-binding-id': state['baseline_binding_id'],
        'voice.io/dev-music-namespace-uid': NS_UID,
        'voice.io/dev-music-base-configmap-uid': source['metadata']['uid'],
        'voice.io/dev-music-base-configmap-rv': source['metadata']['resourceVersion'],
        'voice.io/dev-music-base-configmap-sha256': sha(canonical(source.get('data'))),
        'voice.io/dev-music-target-source-sha256': target_sha,
        'voice.io/dev-music-plan-sha256': plan_sha256,
    }
    if not re.fullmatch(r'[a-f0-9]{64}', plan_sha256):
        fail('bootstrap_configmap_plan_invalid')
    configmap = {
        'apiVersion': 'v1', 'kind': 'ConfigMap',
        'metadata': {'name': name, 'namespace': NS,
                     'labels': {**LABELS, 'voice.io/dev-music-binding-id': state['baseline_binding_id']},
                     'annotations': annotations},
        'immutable': True, 'data': target_data,
    }
    kube = Kube()
    current_source = kube.get('configmap', 'voice-nats-analytics-chat-bootstrap')
    if current_source != source:
        fail('operator_snapshot_changed')
    if revalidate is not None:
        revalidate()
    kube.run('create', '-f', '-', body=canonical(configmap), timeout=20)
    readback = kube.get('configmap', name)
    if metadata(readback)['uid'] == source['metadata']['uid']:
        fail('bootstrap_configmap_enrollment_readback_mismatch')
    validate_bootstrap_configmap_enrollment_without_journal(
        readback, source, target_data, state['baseline_binding_id'], plan_sha256)
    write_bootstrap_configmap_enrollment_journal(state, readback, plan_sha256)
    validate_bootstrap_configmap_enrollment(readback, source, target_data, state['baseline_binding_id'])
    return readback


def assert_bootstrap_configmap_transition(before, after, expected_enrollment):
    """Allow only the exact immutable ConfigMap enrollment after a plan-bound capture."""
    for key in ('namespace', 'marker', 'profile', 'baseline_binding_id', 'operator', 'service', 'bootstrap_source',
                'bootstrap', 'deployment', 'nats_service', 'hub_config', 'nats_workload', 'nats_replica_set',
                'bootstrap_job', 'bootstrap_config', 'bootstrap_config_data',
                'bootstrap_config_target_available', 'nats_pod'):
        if before.get(key) != after.get(key):
            fail('bootstrap_configmap_unrelated_state_changed')
    original = before.get('bootstrap_config_enrollment')
    observed = after.get('bootstrap_config_enrollment')
    if (expected_enrollment is None or observed != expected_enrollment
            or (original is not None and original != expected_enrollment)
            or (original is None and observed is None)
            or after.get('bootstrap_config_enrollment_name') != expected_enrollment.get('metadata', {}).get('name')
            or (before.get('bootstrap_config_enrollment_name') not in (None, expected_enrollment.get('metadata', {}).get('name')))):
        fail('bootstrap_configmap_transition_invalid')
    validate_bootstrap_configmap_enrollment(
        observed, after.get('bootstrap_config'), after.get('bootstrap_config_data'), after['baseline_binding_id'])
    return True


def validate_bootstrap_configmap_enrollment_without_journal(configmap, source, target_data, baseline_binding_id, plan_sha256):
    """Validate create readback before the root-only adoption journal is durable."""
    meta = configmap.get('metadata', {})
    target_sha = sha(canonical(target_data))
    expected_name = bootstrap_configmap_name(baseline_binding_id, target_sha)
    expected_annotations = {
        'voice.io/dev-music-configmap-enrollment': 'voice-dev-music-configmap-enrollment-v1',
        'voice.io/dev-music-binding-id': baseline_binding_id,
        'voice.io/dev-music-namespace-uid': NS_UID,
        'voice.io/dev-music-base-configmap-uid': source['metadata']['uid'],
        'voice.io/dev-music-base-configmap-rv': source['metadata']['resourceVersion'],
        'voice.io/dev-music-base-configmap-sha256': sha(canonical(source.get('data'))),
        'voice.io/dev-music-target-source-sha256': target_sha,
        'voice.io/dev-music-plan-sha256': plan_sha256,
    }
    if (meta.get('name') != expected_name or meta.get('namespace') != NS
            or meta.get('labels') != {**LABELS, 'voice.io/dev-music-binding-id': baseline_binding_id}
            or meta.get('annotations') != expected_annotations
            or configmap.get('immutable') is not True or configmap.get('data') != target_data):
        fail('bootstrap_configmap_enrollment_readback_mismatch')
    return True


def unique_json_pairs(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            fail('private_json_duplicate_key')
        result[key] = value
    return result


def get_data(row, key):
    try:
        value = base64.b64decode(row['data'][key], validate=True)
    except Exception:
        fail('secret_key_missing_or_invalid')
    if not 0 < len(value) <= MAX_PRIVATE:
        fail('secret_value_size_invalid')
    return value


def private_file(path, expected_size=None):
    import pwd
    path = Path(path)
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        info = os.fstat(fd)
        if (not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_nlink != 1
                or stat.S_IMODE(info.st_mode) != 0o600 or info.st_size <= 0 or info.st_size > MAX_PRIVATE
                or expected_size is not None and info.st_size != expected_size):
            fail('private_input_custody_invalid')
        if Path('/home/pmd') in path.parents or path == Path('/home/pmd'):
            fail('shared_pmd_private_input_refused')
        if pwd.getpwuid(info.st_uid).pw_name == 'pmd':
            fail('shared_pmd_private_input_refused')
        for parent in (path.parent, *path.parent.parents):
            parent_info = parent.lstat()
            if (parent.resolve(strict=True) != parent or not stat.S_ISDIR(parent_info.st_mode)
                    or parent_info.st_uid not in (0, info.st_uid) or parent_info.st_mode & 0o022):
                fail('private_input_parent_invalid')
            if parent == Path('/home/voice-dev'):
                break
        raw = bytearray()
        while len(raw) <= MAX_PRIVATE:
            chunk = os.read(fd, min(65536, MAX_PRIVATE + 1 - len(raw)))
            if not chunk:
                break
            raw.extend(chunk)
        if len(raw) != info.st_size:
            fail('private_input_changed')
        return raw
    finally:
        os.close(fd)


def invoke_issuer(executable, request, mode):
    if mode not in ('--verify-existing-actor', '--renew-existing-service',
                    '--renew-existing-bootstrap'):
        fail('issuer_mode_refused')
    raw = bytearray(canonical(request))
    if not 0 < len(raw) <= MAX_PRIVATE:
        fail('issuer_request_size_invalid')
    read_fd, write_fd = os.pipe()
    deadline = time.monotonic() + TIMEOUT
    output = bytearray()
    errors = []

    def reader():
        try:
            os.set_blocking(read_fd, False)
            with selectors.DefaultSelector() as selector:
                selector.register(read_fd, selectors.EVENT_READ)
                while True:
                    remain = deadline - time.monotonic()
                    if remain <= 0:
                        raise TimeoutError()
                    if not selector.select(min(remain, .2)):
                        continue
                    chunk = os.read(read_fd, 65536)
                    if not chunk:
                        return
                    output.extend(chunk)
                    if len(output) > 262144:
                        raise ValueError()
        except Exception:
            errors.append(True)
        finally:
            os.close(read_fd)

    process = None
    thread = threading.Thread(target=reader, daemon=True)
    thread.start()
    try:
        process = subprocess.Popen([str(executable), mode, str(write_fd)], stdin=subprocess.PIPE,
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                                   pass_fds=(write_fd,), close_fds=True,
                                   env={'PATH': '/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin',
                                        'HOME': '/nonexistent'})
        os.close(write_fd)
        write_fd = None
        process.communicate(input=raw, timeout=max(.001, deadline - time.monotonic()))
        thread.join(max(.001, deadline - time.monotonic()) + .3)
        if process.returncode != 0 or thread.is_alive() or errors or not output:
            fail('issuer_operation_refused')
        return bytes(output)
    except Exception:
        if process is not None and process.poll() is None:
            process.kill()
            process.wait(timeout=2)
        fail('issuer_operation_refused')
    finally:
        if write_fd is not None:
            os.close(write_fd)
        thread.join(max(0, deadline - time.monotonic()) + .3)
        for value in (raw, output):
            for index in range(len(value)):
                value[index] = 0


def service_renewal_request(actor, operator, signer_seed, receipt, grant):
    return {'schema': 'voice-nats-existing-service-renewal-v1',
            'old_creds': actor.decode('ascii'),
            'account_jwt': get_data(operator, 'account.jwt').decode('ascii').strip(),
            'operator_jwt': get_data(operator, 'operator.jwt').decode('ascii').strip(),
            'signer_seed': signer_seed.decode('ascii'),
            'actor_sha256': receipt['actor_sha256'],
            'account_sha256': receipt['account_sha256'],
            'service': 'messaging', 'publish': grant['publish'],
            'subscribe': grant['subscribe'], 'no_response': grant['no_response']}


def bootstrap_renewal_request(actor, operator, signer_seed, receipt):
    return {'schema': 'voice-nats-existing-bootstrap-renewal-v1',
            'old_creds': actor.decode('ascii'),
            'account_jwt': get_data(operator, 'account.jwt').decode('ascii').strip(),
            'operator_jwt': get_data(operator, 'operator.jwt').decode('ascii').strip(),
            'signer_seed': signer_seed.decode('ascii'),
            'actor_sha256': receipt['actor_sha256'],
            'account_sha256': receipt['account_sha256']}


def probe_command(role, port, credentials_file):
    if role not in ('messaging', 'bootstrap'):
        fail('broker_probe_actor_role_invalid')
    return [str(PROBE), '--actor-role', role, '--loopback-port', str(port),
            '--credentials-file', str(credentials_file)]


def run_probe(kube, pod, creds, tempdir, role='messaging'):
    if role not in ('messaging', 'bootstrap'):
        fail('broker_probe_actor_role_invalid')
    # Only the canonical client listener is forwarded; 7422 is leaf protocol.
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        port = sock.getsockname()[1]
    process = None
    credentials_file = Path(tempdir) / (role + '-' + str(time.monotonic_ns()) + '.creds')
    fd = os.open(credentials_file, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        os.write(fd, creds)
        os.fsync(fd)
    finally:
        os.close(fd)
    try:
        process = subprocess.Popen(['kubectl', '--namespace', NS, 'port-forward', '--address', '127.0.0.1',
                                    'pod/' + pod['metadata']['name'], f'{port}:4222'],
                                   stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                   env={'PATH': '/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin',
                                        'HOME': '/root', 'KUBECONFIG': str(KUBECONFIG)})
        deadline = time.monotonic() + 10
        ready = False
        output = bytearray()
        os.set_blocking(process.stdout.fileno(), False)
        with selectors.DefaultSelector() as selector:
            selector.register(process.stdout, selectors.EVENT_READ)
            while time.monotonic() < deadline:
                if process.poll() is not None:
                    break
                if not selector.select(.2):
                    continue
                chunk = os.read(process.stdout.fileno(), 4096)
                if not chunk:
                    break
                output.extend(chunk)
                if len(output) > 8192:
                    fail('broker_port_forward_output_invalid')
                if f'Forwarding from 127.0.0.1:{port} -> 4222'.encode() in output:
                    ready = True
                    break
        if not ready:
            fail('broker_port_forward_unavailable')
        result = subprocess.run(probe_command(role, port, credentials_file),
                                capture_output=True, timeout=15,
                                env={'PATH': '/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin',
                                     'HOME': '/nonexistent'})
        if result.returncode:
            fail('broker_auth_or_info_probe_failed')
        receipt = json.loads(result.stdout)
        if (set(receipt) != {'schema', 'broker_authenticated', 'outcome', 'consumer_info_authorized', 'consumer_exists', 'consumer_matches'}
                or receipt['schema'] != 'voice-dev-music-broker-preflight-v1'
                or receipt['broker_authenticated'] is not True
                or receipt['outcome'] not in ('authorized_present', 'authorized_absent', 'no_response')
                or type(receipt['consumer_info_authorized']) is not bool
                or type(receipt['consumer_exists']) is not bool
                or type(receipt['consumer_matches']) is not bool
                or receipt['consumer_matches'] and not receipt['consumer_exists']):
            fail('broker_probe_receipt_invalid')
        return receipt
    except subprocess.TimeoutExpired:
        fail('broker_probe_timeout')
    finally:
        if process is not None:
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=2)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=2)
            if process.stdout:
                process.stdout.close()


def capture(kube, profile=PROFILE):
    if profile != PROFILE:
        fail('dev_music_profile_unsupported')
    namespace = own(kube.get('namespace', NS), name=NS, namespace=False)
    if namespace['metadata']['uid'] != NS_UID:
        fail('voice_dev_namespace_uid_changed')
    marker = kube.get('configmap', 'voice-nats-generation', optional=True)
    if marker is not None:
        own(marker, name='voice-nats-generation')
        fail('dev_profile_generation_marker_present')
    operator = own(kube.get('secret', 'voice-nats-operator'),
                   name='voice-nats-operator', labels=False)
    service = own(kube.get('secret', 'voice-nats-service-credentials'),
                  name='voice-nats-service-credentials', labels=False)
    bootstrap_source = kube.get('secret', BOOTSTRAP_SECRET, optional=True)
    if bootstrap_source is not None:
        own(bootstrap_source, name=BOOTSTRAP_SECRET, labels=False)
    deployment = own(kube.get('deployment', 'voice-messaging'), name='voice-messaging', labels=False)
    replicas = deployment.get('spec', {}).get('replicas', 0)
    status = deployment.get('status', {})
    if (type(replicas) is not int or replicas < 1 or status.get('observedGeneration') != deployment['metadata'].get('generation')
            or status.get('readyReplicas') != replicas or status.get('updatedReplicas') != replicas
            or status.get('unavailableReplicas', 0) != 0):
        fail('messaging_deployment_not_stable')
    nats_service = own(kube.get('service', 'voice-nats'), name='voice-nats', labels=False)
    hub = own(kube.get('configmap', 'voice-nats-hub-config'), name='voice-nats-hub-config', labels=False)
    nats_workload = own(kube.get('deployment', NATS_DEPLOYMENT), name=NATS_DEPLOYMENT, labels=False)
    svc = nats_service.get('spec', {})
    if svc.get('selector') != NATS_SELECTOR:
        fail('dev_profile_nats_service_selector_changed')
    ports = [p for p in svc.get('ports', []) if p.get('name') == 'client']
    if len(ports) != 1 or ports[0].get('port') != 4222 or ports[0].get('targetPort') != 4222:
        fail('nats_client_listener_changed')
    if nats_service.get('spec', {}).get('type') != 'ClusterIP':
        fail('nats_service_exposure_changed')
    workload_spec = nats_workload.get('spec', {})
    workload_status = nats_workload.get('status', {})
    if (workload_spec.get('selector', {}).get('matchLabels') != NATS_SELECTOR
            or workload_spec.get('replicas') != 1
            or workload_status.get('observedGeneration') != nats_workload['metadata'].get('generation')
            or workload_status.get('readyReplicas') != 1 or workload_status.get('updatedReplicas') != 1
            or workload_status.get('unavailableReplicas', 0) != 0):
        fail('dev_profile_nats_deployment_not_stable')
    pods = [p for p in kube.list('pods', ','.join(f'{k}={v}' for k, v in NATS_SELECTOR.items()))
            if p.get('status', {}).get('phase') == 'Running']
    if len(pods) != 1:
        fail('nats_hub_pod_not_unique_ready')
    pod = pods[0]
    if pod.get('metadata', {}).get('namespace') != NS:
        fail('nats_hub_pod_foreign')
    pod_owners = pod.get('metadata', {}).get('ownerReferences', [])
    if (len(pod_owners) != 1 or pod_owners[0].get('kind') != 'ReplicaSet'
            or pod_owners[0].get('controller') is not True
            or not pod_owners[0].get('name', '').startswith(NATS_DEPLOYMENT + '-')):
        fail('dev_profile_nats_pod_owner_changed')
    replica_set = own(kube.get('replicaset', pod_owners[0]['name']), name=pod_owners[0]['name'], labels=False)
    rs_owners = replica_set.get('metadata', {}).get('ownerReferences', [])
    if (replica_set['metadata']['uid'] != pod_owners[0].get('uid')
            or len(rs_owners) != 1 or rs_owners[0].get('kind') != 'Deployment'
            or rs_owners[0].get('name') != NATS_DEPLOYMENT or rs_owners[0].get('uid') != nats_workload['metadata']['uid']
            or rs_owners[0].get('controller') is not True
            or replica_set.get('spec', {}).get('selector', {}).get('matchLabels') != NATS_SELECTOR
            or replica_set.get('spec', {}).get('replicas') != 1
            or replica_set.get('status', {}).get('readyReplicas') != 1):
        fail('dev_profile_nats_replica_set_owner_changed')
    template = replica_set.get('spec', {}).get('template', {})
    template_spec = template.get('spec', {})
    template_containers = template_spec.get('containers', [])
    pod_containers = pod.get('spec', {}).get('containers', [])
    if (template.get('metadata', {}).get('labels', {}).get('app') != NATS_DEPLOYMENT
            or len(template_containers) != 1 or len(pod_containers) != 1
            or template_containers[0].get('name') != 'nats'
            or pod_containers[0].get('name') != 'nats'
            or pod_containers[0].get('image') != template_containers[0].get('image')
            or not re.fullmatch(r'[A-Za-z0-9./:_-]+@sha256:[a-f0-9]{64}', template_containers[0].get('image', ''))):
        fail('dev_profile_nats_image_not_pinned')
    statuses = pod.get('status', {}).get('containerStatuses', [])
    if (len(statuses) != 1 or statuses[0].get('name') != 'nats' or statuses[0].get('ready') is not True
            or statuses[0].get('image') != template_containers[0]['image']
            or template_containers[0]['image'].split('@sha256:', 1)[1] not in
            statuses[0].get('imageID', '').removeprefix('docker-pullable://')):
        fail('dev_profile_nats_pod_image_changed')
    account_public = get_data(operator, 'account.public').decode('ascii').strip().encode('ascii')
    if not account_public:
        fail('account_public_key_invalid')
    baseline_binding_id = 'dev-' + sha(canonical({
        'profile': profile, 'namespace_uid': NS_UID,
        'nats_service_uid': nats_service['metadata']['uid'],
        'nats_deployment_uid': nats_workload['metadata']['uid'],
        'account_public_sha256': sha(account_public),
    }))[:24]
    enrolled_name = BOOTSTRAP_SECRET + '-music-' + baseline_binding_id
    journal_path = BOOTSTRAP_ENROLLMENT_JOURNAL / ('bootstrap-enrollment-' + baseline_binding_id + '.json')
    enrolled = kube.get('secret', enrolled_name, optional=True)
    if enrolled is not None:
        own(enrolled, name=enrolled_name, labels=False)
        validate_bootstrap_enrollment(enrolled, bootstrap_source, baseline_binding_id)
    else:
        try:
            journal_path.lstat()
        except FileNotFoundError:
            pass
        else:
            fail('bootstrap_enrollment_journal_without_secret')
    bootstrap = enrolled if enrolled is not None else bootstrap_source
    bootstrap_config = kube.get('configmap', 'voice-nats-analytics-chat-bootstrap', optional=True)
    if bootstrap_config is not None:
        own(bootstrap_config, name='voice-nats-analytics-chat-bootstrap', labels=False)
    if bootstrap_config is not None:
        sys.path.insert(0, str(Path(__file__).resolve().parent))
        import bootstrap as dev_bootstrap
        canonical_data = dev_bootstrap.canonical_bootstrap_data()
        expected_config = canonical_data.get('voice-nats-analytics-chat-bootstrap')
        config_matches = (isinstance(expected_config, dict) and
                          expected_config.get('bootstrap.sh', '').splitlines().count(MUSIC_BOOTSTRAP_CONTRACT) == 1)
        enrolled_config_name = bootstrap_configmap_name(baseline_binding_id, sha(canonical(expected_config))) if config_matches else None
        enrolled_config = (kube.get('configmap', enrolled_config_name, optional=True)
                           if enrolled_config_name is not None else None)
        config_journal = bootstrap_configmap_journal_path(baseline_binding_id)
        if enrolled_config is not None:
            own(enrolled_config, name=enrolled_config_name, labels=False)
            validate_bootstrap_configmap_enrollment(enrolled_config, bootstrap_config, expected_config, baseline_binding_id)
        else:
            try:
                config_journal.lstat()
            except FileNotFoundError:
                pass
            else:
                fail('bootstrap_configmap_journal_without_configmap')
    else:
        expected_config = None
        config_matches = False
        enrolled_config = None
        enrolled_config_name = None
    job = kube.get('job', 'voice-nats-analytics-chat-bootstrap', optional=True)
    if job is not None:
        own(job, name='voice-nats-analytics-chat-bootstrap')
    cfg = deployment.get('spec', {}).get('template', {}).get('spec', {})
    containers = cfg.get('containers', [])
    leaves = [c for c in containers if c.get('name') == 'nats-leaf']
    volumes = cfg.get('volumes', [])
    key = 'messaging.creds'
    if len(leaves) != 1 or len([v for v in volumes if v.get('name') == 'nats-service-creds']) != 1:
        fail('messaging_leaf_mount_missing')
    leaf = leaves[0]
    selected = [v for v in volumes if v.get('name') == 'nats-service-creds'][0]
    expected_volume = {'name': 'nats-service-creds', 'secret': {
        'secretName': service['metadata']['name'], 'defaultMode': 256,
        'items': [{'key': key, 'path': key}]}}
    if selected != expected_volume or not any(
            m == {'name': 'nats-service-creds', 'mountPath': '/var/run/nats/creds/messaging.creds',
                  'subPath': key, 'readOnly': True} for m in leaf.get('volumeMounts', [])):
        fail('messaging_leaf_mount_changed')
    port_cfg = hub.get('data', {}).get('nats.conf', '')
    if 'server_name: voice-nats-hub' not in port_cfg or 'listen: 0.0.0.0:7422' not in port_cfg:
        fail('nats_hub_source_config_changed')
    return {'namespace': namespace, 'marker': marker, 'profile': profile,
            'baseline_binding_id': baseline_binding_id, 'operator': operator,
            'service': service, 'bootstrap_source': bootstrap_source, 'bootstrap': bootstrap, 'deployment': deployment,
            'nats_service': nats_service, 'hub_config': hub, 'nats_workload': nats_workload,
            'nats_replica_set': replica_set, 'bootstrap_job': job,
            'bootstrap_config': bootstrap_config, 'bootstrap_config_data': expected_config,
            'bootstrap_config_target_available': config_matches, 'bootstrap_config_enrollment': enrolled_config,
            'bootstrap_config_enrollment_name': enrolled_config_name, 'nats_pod': pod}


def snapshot_hash(state, acl_raw, issuer_sha):
    public = {}
    for key, name in (('namespace', None), ('marker', None), ('operator', None), ('service', None),
                      ('bootstrap_source', None), ('bootstrap', None), ('deployment', None), ('nats_service', None),
                      ('hub_config', None), ('nats_workload', None), ('bootstrap_job', None),
                      ('bootstrap_config', None), ('bootstrap_config_enrollment', None), ('nats_pod', None)):
        row = state.get(key)
        public[key] = None if row is None else metadata(row, key != 'namespace') if key == 'namespace' else metadata(row)
    public.update(profile=state['profile'], baseline_binding_id=state['baseline_binding_id'],
                  legacy_generation_marker_absent=state['marker'] is None,
                  nats_service_state_sha256=sha(canonical(state['nats_service'])),
                  nats_deployment_state_sha256=sha(canonical(state['nats_workload'])),
                  nats_replica_set_state_sha256=sha(canonical(state['nats_replica_set'])),
                  nats_pod_state_sha256=sha(canonical(state['nats_pod'])),
                  acl_sha256=sha(acl_raw), issuer_sha256=issuer_sha,
                  bootstrap_base_configmap_sha256=None if state.get('bootstrap_config') is None else
                      sha(canonical(state['bootstrap_config'].get('data'))),
                  bootstrap_target_source_sha256=None if state.get('bootstrap_config_data') is None else
                      sha(canonical(state['bootstrap_config_data'])))
    return sha(canonical(public)), public


def stable_hash(state, acl_raw, issuer_sha):
    """Binding used only across the intentional Messaging replica transition."""
    deployment = json.loads(json.dumps(state['deployment']))
    deployment.get('spec', {}).pop('replicas', None)
    public = {
        'namespace_uid': state['namespace']['metadata']['uid'], 'profile': state['profile'],
        'baseline_binding_id': state['baseline_binding_id'],
        'legacy_generation_marker_absent': state['marker'] is None,
        'operator': [metadata(state['operator']), sha(canonical(state['operator']))],
        'service': [metadata(state['service']), sha(canonical(state['service']))],
        'bootstrap_source': None if state['bootstrap_source'] is None else
            [metadata(state['bootstrap_source']), sha(canonical(state['bootstrap_source']))],
        'bootstrap': None if state['bootstrap'] is None else
            [metadata(state['bootstrap']), sha(canonical(state['bootstrap']))],
        'deployment_uid': state['deployment']['metadata']['uid'],
        'deployment_spec_without_replicas': sha(canonical(deployment['spec'])),
        'nats_service': [metadata(state['nats_service']), sha(canonical(state['nats_service']))],
        'hub_config': [metadata(state['hub_config']), sha(canonical(state['hub_config']))],
        'nats_workload': [metadata(state['nats_workload']), sha(canonical(state['nats_workload']))],
        'nats_replica_set': [metadata(state['nats_replica_set']), sha(canonical(state['nats_replica_set']))],
        'bootstrap_job': None if state['bootstrap_job'] is None else
            [metadata(state['bootstrap_job']), sha(canonical(state['bootstrap_job']))],
        'bootstrap_config': None if state['bootstrap_config'] is None else
            [metadata(state['bootstrap_config']), sha(canonical(state['bootstrap_config']))],
        'bootstrap_config_enrollment': None if state.get('bootstrap_config_enrollment') is None else
            [metadata(state['bootstrap_config_enrollment']), sha(canonical(state['bootstrap_config_enrollment']))],
        'bootstrap_target_source_sha256': None if state.get('bootstrap_config_data') is None else
            sha(canonical(state['bootstrap_config_data'])),
        'nats_pod_uid': state['nats_pod']['metadata']['uid'],
        'nats_service_state_sha256': sha(canonical(state['nats_service'])),
        'nats_replica_set_state_sha256': sha(canonical(state['nats_replica_set'])),
        'nats_pod_state_sha256': sha(canonical(state['nats_pod'])),
        'acl_sha256': sha(acl_raw), 'issuer_sha256': issuer_sha,
    }
    return sha(canonical(public))


def verify_actor(binary, actor, operator, grant, role='messaging', require_grant=True):
    import actor_root
    import actor_permissions
    actor_public = actor_root.public_user(actor, role)
    account = get_data(operator, 'account.public').decode('ascii').strip()
    account_jwt = get_data(operator, 'account.jwt').decode('ascii').strip()
    operator_jwt = get_data(operator, 'operator.jwt').decode('ascii').strip()
    request = {'schema': 'voice-nats-existing-actor-verification-v1', 'credentials': actor.decode('ascii'),
               'account_jwt': account_jwt, 'operator_jwt': operator_jwt,
               'actor_sha256': sha(actor_public.encode()), 'account_sha256': sha(account.encode())}
    raw = invoke_issuer(binary, request, '--verify-existing-actor')
    receipt = json.loads(raw)
    expected = {'actor_sha256': request['actor_sha256'], 'account_sha256': request['account_sha256'],
                'credential_sha256': sha(actor), 'account_jwt_sha256': sha(account_jwt.encode()),
                'operator_sha256': sha(operator_jwt.encode()), 'signed_chain_verified': True}
    if (not isinstance(receipt, dict) or set(receipt) != set(expected) | {'actor_jwt_sha256', 'effective'}
            or any(receipt.get(k) != v for k, v in expected.items())):
        fail('actor_signed_chain_invalid')
    effective = receipt['effective']
    if require_grant:
        actor_permissions.evaluate(effective, {'pub': grant['publish'], 'sub': grant['subscribe']})
    return receipt, actor_public, effective


def bootstrap_claims(binary, bootstrap, operator, grants):
    if bootstrap is None:
        return 'missing'
    import actor_permissions
    grant = grants.get('bootstrap')
    if not isinstance(grant, dict):
        fail('bootstrap_acl_intent_missing')
    credentials = get_data(bootstrap, 'bootstrap.creds')
    _, _, effective = verify_actor(binary, credentials, operator, grant,
                                   role='bootstrap', require_grant=False)
    try:
        actor_permissions.evaluate(effective, {'pub': grant['publish'], 'sub': grant['subscribe']})
        return 'signed_claims_current'
    except actor_permissions.PermissionError:
        return 'separate_bootstrap_renewal_required'


def decision(effective, grant, broker, additions=SERVICE_ADDITIONS):
    import actor_permissions
    if broker.get('consumer_exists') is True and broker.get('consumer_matches') is not True:
        fail('music_consumer_config_mismatch')
    baseline = {'pub': [x for x in grant['publish'] if x not in additions],
                'sub': grant['subscribe']}
    actor_permissions.evaluate(effective, baseline)
    try:
        actor_permissions.evaluate(effective, {'pub': grant['publish'], 'sub': grant['subscribe']})
        current = True
    except actor_permissions.PermissionError:
        current = False
    if not current:
        if broker.get('broker_authenticated') is not True:
            fail('broker_authentication_not_proven')
        return 'renew_music_grants'
    if broker.get('broker_authenticated') is not True:
        fail('broker_authentication_not_proven')
    if broker.get('consumer_info_authorized') is not True:
        fail('signed_grants_and_broker_info_disagree')
    return 'current'


def bootstrap_action(effective, grant, broker, *, config_matches, job, prepared_available=False):
    """Choose only an explicit bootstrap replay after live INFO proves absence."""
    if not config_matches:
        fail('bootstrap_source_config_mismatch')
    if broker.get('broker_authenticated') is not True or broker.get('consumer_info_authorized') is not True:
        fail('bootstrap_live_info_not_proven')
    if broker.get('consumer_exists') is True:
        if broker.get('consumer_matches') is not True:
            fail('music_consumer_config_mismatch')
        return 'consumer_already_present'
    if broker.get('outcome') != 'authorized_absent':
        fail('bootstrap_consumer_absence_not_proven')
    if not prepared_available:
        fail('bootstrap_prepared_job_missing')
    if job is not None:
        status = job.get('status', {})
        if status.get('active', 0) != 0:
            fail('bootstrap_job_already_active')
        if status.get('succeeded', 0) != 1 and not any(
                row.get('type') == 'Failed' and row.get('status') == 'True'
                for row in status.get('conditions', [])):
            fail('bootstrap_job_not_terminal')
    permissions = decision(effective, grant, broker, BOOTSTRAP_ADDITIONS)
    return ('renew_bootstrap_then_replay' if permissions == 'renew_music_grants'
            else 'replay_exact_bootstrap_job')


def validate_bootstrap_job(job, baseline_binding_id, bootstrap_secret, config_matches, expected_image=None,
                           expected_secret_name=None, expected_configmap_name='voice-nats-analytics-chat-bootstrap'):
    """Validate the existing Job as the bounded source for an explicit replay."""
    if not config_matches or job is None:
        fail('bootstrap_job_source_unavailable')
    spec = job.get('spec', {})
    template = spec.get('template', {})
    pod = template.get('spec', {})
    containers = pod.get('containers', [])
    source_secret_names = [volume.get('secret', {}).get('secretName') for volume in pod.get('volumes', [])
                           if volume.get('name') == 'nats-bootstrap-creds']
    pod_defaults = {
        'serviceAccountName': 'default', 'dnsPolicy': 'ClusterFirst',
        'terminationGracePeriodSeconds': 30, 'schedulerName': 'default-scheduler',
        'enableServiceLinks': True, 'preemptionPolicy': 'PreemptLowerPriority', 'priority': 0,
        'hostNetwork': False, 'hostPID': False, 'hostIPC': False, 'shareProcessNamespace': False,
        'securityContext': {},
    }
    pod_required = {'automountServiceAccountToken': False, 'restartPolicy': 'OnFailure',
                    'containers': containers, 'volumes': pod.get('volumes', [])}
    allowed_pod = set(pod_required) | set(pod_defaults)
    if (len(containers) != 1 or pod.get('initContainers') or pod.get('automountServiceAccountToken') is not False
            or pod.get('restartPolicy') != 'OnFailure' or set(pod) - allowed_pod
            or any(pod.get(k) != v for k, v in pod_defaults.items() if k in pod)
            or pod.get('serviceAccountName', 'default') != 'default'
            or pod.get('hostNetwork', False) or pod.get('hostPID', False) or pod.get('hostIPC', False)
            or pod.get('shareProcessNamespace', False) or pod.get('imagePullSecrets')
            or template.get('metadata', {}).get('annotations')
            or any('hostPath' in volume for volume in pod.get('volumes', []))):
        fail('bootstrap_job_pod_contract_changed')
    container = containers[0]
    if (container.get('name') != 'bootstrap' or container.get('command') != ['/bin/sh', '/config/bootstrap.sh']
            or container.get('image') != 'natsio/nats-box:0.18.0'
            or expected_image is not None and container.get('image') != expected_image):
        fail('bootstrap_job_command_contract_changed')
    container_defaults = {'imagePullPolicy': 'IfNotPresent',
                          'terminationMessagePath': '/dev/termination-log',
                          'terminationMessagePolicy': 'File', 'stdin': False,
                          'stdinOnce': False, 'tty': False}
    container_required = {'name', 'image', 'command', 'env', 'resources', 'volumeMounts'}
    if (set(container) - container_required - set(container_defaults)
            or not container_required <= set(container)
            or any(container.get(k) != v for k, v in container_defaults.items() if k in container)
            or container.get('securityContext') not in (None, {})):
        fail('bootstrap_job_container_policy_changed')
    if len(container.get('env', [])) != 2:
        fail('bootstrap_job_environment_changed')
    if (container.get('args', []) or container.get('ports', []) or container.get('envFrom')
            or container.get('lifecycle') or container.get('volumeDevices')
            or container.get('securityContext', {}).get('privileged') is True
            or container.get('resources') != {'requests': {'cpu': '20m', 'memory': '64Mi'},
                                              'limits': {'cpu': '500m', 'memory': '384Mi'}}):
        fail('bootstrap_job_container_policy_changed')
    env = {row.get('name'): row.get('value') for row in container.get('env', [])}
    if env != {'NATS_URL': 'nats://voice-nats:4222',
               'NATS_CREDS': '/var/run/nats/bootstrap/bootstrap.creds'}:
        fail('bootstrap_job_environment_changed')
    config_mount = {'name': 'config', 'mountPath': '/config', 'readOnly': True}
    secret_mount = {'name': 'nats-bootstrap-creds',
                    'mountPath': '/var/run/nats/bootstrap/bootstrap.creds',
                    'subPath': 'bootstrap.creds', 'readOnly': True}
    if container.get('volumeMounts') != [config_mount, secret_mount]:
        fail('bootstrap_job_mounts_changed')
    volumes = pod.get('volumes', [])
    secret_name = expected_secret_name or bootstrap_secret['metadata']['name']
    expected = {
        'config': {'name': 'config', 'configMap': {
            'name': expected_configmap_name, 'defaultMode': 365}},
        'nats-bootstrap-creds': {'name': 'nats-bootstrap-creds', 'secret': {
            'secretName': secret_name, 'defaultMode': 256,
            'items': [{'key': 'bootstrap.creds', 'path': 'bootstrap.creds'}]}}}
    if len(volumes) != 2 or {v.get('name'): v for v in volumes} != expected:
        fail('bootstrap_job_volumes_changed')
    if next((v for v in volumes if v.get('name') == 'nats-bootstrap-creds'), {}).get('secret', {}).get('secretName') != secret_name:
        fail('bootstrap_job_source_secret_changed')
    if secret_name not in (BOOTSTRAP_SECRET, bootstrap_secret.get('metadata', {}).get('name')):
        fail('bootstrap_secret_binding_mismatch')
    job_defaults = {'parallelism': 1, 'completions': 1, 'completionMode': 'NonIndexed',
                    'manualSelector': False, 'podReplacementPolicy': 'TerminatingOrFailed'}
    job_fields = {'backoffLimit', 'suspend', 'ttlSecondsAfterFinished', 'template', 'selector', *job_defaults}
    selector = spec.get('selector')
    selector_ok = selector is None or (set(selector) == {'matchLabels'} and
        isinstance(selector.get('matchLabels'), dict) and
        set(selector['matchLabels']) <= {'controller-uid', 'job-name',
                                         'batch.kubernetes.io/controller-uid',
                                         'batch.kubernetes.io/job-name'})
    if (set(spec) - job_fields or not selector_ok
            or any(spec.get(k) != v for k, v in job_defaults.items() if k in spec)
            or spec.get('backoffLimit') != 12 or spec.get('suspend', False)
            or spec.get('ttlSecondsAfterFinished') != 86400
            or spec.get('activeDeadlineSeconds') is not None
            or spec.get('parallelism', 1) != 1 or spec.get('completions', 1) != 1):
        fail('bootstrap_job_policy_changed')
    return True


def require_current(original, fresh, issuer_sha, probe_sha, prepared_sha=None):
    now = capture(Kube())
    digest, _ = snapshot_hash(now, ACL.read_bytes(), issuer_sha)
    probe_info = PROBE.lstat()
    current_probe_sha = sha(PROBE.read_bytes())
    if (digest != original or not stat.S_ISREG(probe_info.st_mode) or probe_info.st_uid != 0
            or probe_info.st_nlink != 1 or probe_info.st_mode & 0o077 or current_probe_sha != probe_sha):
        fail('operator_snapshot_changed')
    if prepared_sha is not None and sha(private_file(BOOTSTRAP_PREPARED)) != prepared_sha:
        fail('bootstrap_prepared_source_changed')
    return now


def prepared_bootstrap_job(base_config_data, baseline_binding_id, target_config_data):
    raw = private_file(BOOTSTRAP_PREPARED)
    def unique_pairs(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                fail('bootstrap_prepared_json_duplicate_key')
            result[key] = value
        return result
    try:
        bundle = json.loads(raw, object_pairs_hook=unique_pairs)
        items = bundle['items']
        jobs = [row for row in items if row.get('kind') == 'Job' and
                row.get('metadata', {}).get('name') == 'voice-nats-analytics-chat-bootstrap']
        configs = [row for row in items if row.get('kind') == 'ConfigMap' and
                   row.get('metadata', {}).get('name') == 'voice-nats-analytics-chat-bootstrap']
    except Exception:
        fail('bootstrap_prepared_source_invalid')
    if (bundle.get('apiVersion') != 'v1' or bundle.get('kind') != 'List'
            or len(items) != 2 or len(jobs) != 1 or len(configs) != 1
            or jobs[0].get('metadata', {}).get('namespace') != NS
            or configs[0].get('metadata', {}).get('namespace') != NS
            or configs[0].get('data') != base_config_data):
        fail('bootstrap_prepared_source_mismatch')
    job = jobs[0]
    expected_labels = {'app': 'voice-nats-analytics-chat-bootstrap', **LABELS}
    meta = job.get('metadata', {})
    spec = job.get('spec', {})
    template = spec.get('template', {})
    pod = template.get('spec', {})
    containers = pod.get('containers', [])
    source_secret_names = [volume.get('secret', {}).get('secretName') for volume in pod.get('volumes', [])
                           if volume.get('name') == 'nats-bootstrap-creds']
    job_defaults = {'parallelism': 1, 'completions': 1, 'completionMode': 'NonIndexed',
                    'manualSelector': False, 'podReplacementPolicy': 'TerminatingOrFailed'}
    job_fields = {'backoffLimit', 'template', 'suspend', 'ttlSecondsAfterFinished', *job_defaults}
    if (set(meta) - {'name', 'namespace', 'labels'}
            or meta.get('labels') != expected_labels
            or set(spec) - job_fields or not {'backoffLimit', 'template', 'suspend', 'ttlSecondsAfterFinished'} <= set(spec)
            or any(spec.get(k) != v for k, v in job_defaults.items() if k in spec)
            or spec.get('backoffLimit') != 12 or spec.get('suspend') is not True
            or spec.get('ttlSecondsAfterFinished') != 86400
            or set(template) != {'metadata', 'spec'}
            or template.get('metadata', {}).get('labels') != expected_labels
            or set(template.get('metadata', {})) != {'labels'}
            or pod.get('automountServiceAccountToken') is not False
            or pod.get('restartPolicy') != 'OnFailure' or len(containers) != 1
            or source_secret_names != [BOOTSTRAP_SECRET]
            or any('hostPath' in volume for volume in pod.get('volumes', []))):
        fail('bootstrap_prepared_job_contract_invalid')
    try:
        image = containers[0]['image']
    except (KeyError, IndexError, TypeError):
        fail('bootstrap_prepared_job_invalid')
    if image != 'natsio/nats-box:0.18.0':
        fail('bootstrap_prepared_job_image_invalid')
    validation_job = json.loads(json.dumps(job))
    validation_job['spec']['suspend'] = False
    validate_bootstrap_job(validation_job, baseline_binding_id,
                           {'metadata': {'name': BOOTSTRAP_SECRET}}, True, image,
                           expected_secret_name=BOOTSTRAP_SECRET)
    return job, image, sha(raw), target_config_data


def write_backup(state, old_secret):
    directory = ROOT / 'backups'
    info = directory.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or stat.S_IMODE(info.st_mode) != 0o700:
        fail('private_backup_directory_required')
    name = f"{state['baseline_binding_id']}-messaging-{old_secret['metadata']['uid']}-{old_secret['metadata']['resourceVersion']}.json"
    path = directory / name
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        with os.fdopen(fd, 'wb') as output:
            output.write(canonical(old_secret))
            output.flush()
            os.fsync(output.fileno())
    except Exception:
        try:
            path.unlink()
        except OSError:
            pass
        raise
    return path


def write_bootstrap_backup(state, old_secret):
    directory = ROOT / 'backups'
    info = directory.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or stat.S_IMODE(info.st_mode) != 0o700:
        fail('private_backup_directory_required')
    name = (f"{state['baseline_binding_id']}-bootstrap-{old_secret['metadata']['uid']}-"
            f"{old_secret['metadata']['resourceVersion']}.json")
    path = directory / name
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        with os.fdopen(fd, 'wb') as output:
            output.write(canonical(old_secret))
            output.flush()
            os.fsync(output.fileno())
    except Exception:
        try:
            path.unlink()
        except OSError:
            pass
        raise
    return path


def write_bootstrap_enrollment_journal(state, secret, candidate, actor_sha256, plan_sha256):
    root_info = ROOT.lstat()
    if not stat.S_ISDIR(root_info.st_mode) or root_info.st_uid != 0 or stat.S_IMODE(root_info.st_mode) != 0o700:
        fail('private_operator_directory_required')
    source = state['bootstrap_source']
    journal = {
        'schema': 'voice-dev-music-bootstrap-enrollment-v1',
        'baseline_binding_id': state['baseline_binding_id'], 'namespace_uid': NS_UID,
        'source_secret_uid': source['metadata']['uid'],
        'source_secret_rv': source['metadata']['resourceVersion'],
        'secret_name': secret['metadata']['name'],
        'secret_uid': secret['metadata']['uid'],
        'secret_rv': secret['metadata']['resourceVersion'],
        'actor_sha256': actor_sha256, 'plan_sha256': plan_sha256,
        # The credential digest stays in this root-only journal; it is never
        # copied to Kubernetes annotations or a plan/operation receipt.
        'credential_sha256': sha(candidate),
    }
    path = BOOTSTRAP_ENROLLMENT_JOURNAL / ('bootstrap-enrollment-' + state['baseline_binding_id'] + '.json')
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        with os.fdopen(fd, 'wb') as output:
            output.write(canonical(journal))
            output.flush()
            os.fsync(output.fileno())
        dir_fd = os.open(ROOT, os.O_RDONLY | getattr(os, 'O_DIRECTORY', 0))
        try:
            os.fsync(dir_fd)
        finally:
            os.close(dir_fd)
    except Exception:
        try:
            path.unlink()
        except OSError:
            pass
        raise
    return journal


def create_bootstrap_enrollment(state, candidate, actor_sha256, plan_sha256):
    """Create a new immutable generation credential; never mutate the canonical Secret."""
    source = state.get('bootstrap_source')
    if (source is None or state['bootstrap']['metadata'].get('name') != BOOTSTRAP_SECRET
            or state.get('baseline_binding_id') is None):
        fail('bootstrap_enrollment_source_changed')
    name = BOOTSTRAP_SECRET + '-music-' + state['baseline_binding_id']
    backup = write_bootstrap_backup(state, source)
    secret = {
        'apiVersion': 'v1', 'kind': 'Secret',
        'metadata': {'name': name, 'namespace': NS,
                     'labels': {**LABELS, 'voice.io/dev-music-binding-id': state['baseline_binding_id']},
                     'annotations': {
                         'voice.io/dev-music-enrollment': 'voice-dev-music-bootstrap-enrollment-v1',
                         'voice.io/dev-music-binding-id': state['baseline_binding_id'],
                         'voice.io/dev-music-namespace-uid': NS_UID,
                         'voice.io/dev-music-source-secret-uid': source['metadata']['uid'],
                         'voice.io/dev-music-source-secret-rv': source['metadata']['resourceVersion'],
                         'voice.io/dev-music-actor-sha256': actor_sha256,
                         'voice.io/dev-music-plan-sha256': plan_sha256,
                     }},
        'immutable': True, 'type': 'Opaque',
        'data': {'bootstrap.creds': base64.b64encode(candidate).decode('ascii')},
    }
    if not re.fullmatch(r'[a-f0-9]{64}', actor_sha256) or not re.fullmatch(r'[a-f0-9]{64}', plan_sha256):
        fail('bootstrap_enrollment_receipt_invalid')
    # Kubernetes create is an atomic absent-only operation. A same-generation
    # object or concurrent writer is a hard conflict; never replace it.
    Kube().run('create', '-f', '-', body=canonical(secret), timeout=20)
    readback = Kube().get('secret', name)
    expected_labels = {**LABELS, 'voice.io/dev-music-binding-id': state['baseline_binding_id']}
    expected_annotations = dict(secret['metadata']['annotations'])
    if (metadata(readback)['uid'] == source['metadata']['uid']
            or readback.get('metadata', {}).get('name') != name
            or readback.get('metadata', {}).get('namespace') != NS
            or readback.get('metadata', {}).get('labels') != expected_labels
            or readback.get('metadata', {}).get('annotations') != expected_annotations
            or readback.get('immutable') is not True or readback.get('type') != 'Opaque'
            or set(readback.get('data', {})) != {'bootstrap.creds'}
            or get_data(readback, 'bootstrap.creds') != candidate):
        fail('bootstrap_enrollment_readback_mismatch')
    write_bootstrap_enrollment_journal(state, readback, candidate, actor_sha256, plan_sha256)
    validate_bootstrap_enrollment(readback, source, state['baseline_binding_id'])
    return backup


def replay_bootstrap_job(state, plan_sha256, expected_image, revalidate):
    """Create a uniquely named exact-template Job; never edit/delete the old Job."""
    source = state['bootstrap_job']
    prepared = state.get('bootstrap_job_source')
    if prepared is None:
        fail('bootstrap_prepared_job_missing')
    enrolled_config = state.get('bootstrap_config_enrollment')
    if enrolled_config is None:
        fail('bootstrap_configmap_enrollment_missing')
    validate_bootstrap_configmap_enrollment(
        enrolled_config, state['bootstrap_config'], state['bootstrap_config_data'], state['baseline_binding_id'])
    config_name = enrolled_config['metadata']['name']
    if source is None:
        source_uid = 'prepared-source'
        source_rv = 'prepared-source'
        prepared_for_validation = json.loads(json.dumps(prepared))
        prepared_for_validation['spec']['suspend'] = False
        for volume in prepared_for_validation['spec']['template']['spec'].get('volumes', []):
            if volume.get('name') == 'nats-bootstrap-creds':
                volume['secret']['secretName'] = state['bootstrap']['metadata']['name']
        validate_bootstrap_job(prepared_for_validation, state['baseline_binding_id'], state['bootstrap'],
                               state['bootstrap_config_target_available'], expected_image)
    else:
        validate_bootstrap_job(source, state['baseline_binding_id'], state['bootstrap'],
                               state['bootstrap_config_target_available'], expected_image,
                               expected_secret_name=BOOTSTRAP_SECRET)
        source_uid = source['metadata']['uid']
        source_rv = source['metadata']['resourceVersion']
    name = 'voice-nats-analytics-chat-music-' + plan_sha256[:12]
    labels = dict(prepared.get('metadata', {}).get('labels', {}))
    labels.update(LABELS)
    annotations = {
        'voice.io/dev-music-plan-sha256': plan_sha256,
        'voice.io/dev-music-source-job-uid': source_uid,
        'voice.io/dev-music-source-job-rv': source_rv,
        'voice.io/dev-music-configmap-uid': state['bootstrap_config']['metadata']['uid'],
        'voice.io/dev-music-enrolled-configmap-uid': enrolled_config['metadata']['uid'],
        'voice.io/dev-music-target-source-sha256': sha(canonical(state['bootstrap_config_data'])),
        'voice.io/dev-music-bootstrap-secret-uid': state['bootstrap']['metadata']['uid'],
        'voice.io/dev-music-prepared-sha256': sha(private_file(BOOTSTRAP_PREPARED)),
    }
    job = json.loads(json.dumps(prepared))
    meta = job['metadata']
    for key in ('uid', 'resourceVersion', 'generation', 'creationTimestamp', 'managedFields', 'ownerReferences', 'finalizers'):
        meta.pop(key, None)
    meta.update(name=name, namespace=NS, labels=labels, annotations=annotations)
    job.pop('status', None)
    spec = job.setdefault('spec', {})
    spec.pop('selector', None)
    spec['suspend'] = False
    for volume in spec.get('template', {}).get('spec', {}).get('volumes', []):
        secret_ref = volume.get('secret')
        if volume.get('name') == 'nats-bootstrap-creds' and isinstance(secret_ref, dict):
            secret_ref['secretName'] = state['bootstrap']['metadata']['name']
        config_ref = volume.get('configMap')
        if volume.get('name') == 'config' and isinstance(config_ref, dict):
            config_ref['name'] = config_name
    validate_bootstrap_job(job, state['baseline_binding_id'], state['bootstrap'],
                           state['bootstrap_config_target_available'], expected_image,
                           expected_configmap_name=config_name)
    template_meta = spec.setdefault('template', {}).setdefault('metadata', {})
    for key in ('uid', 'resourceVersion', 'generation', 'creationTimestamp', 'managedFields', 'ownerReferences'):
        template_meta.pop(key, None)
    template_labels = template_meta.setdefault('labels', {})
    for key in ('controller-uid', 'batch.kubernetes.io/controller-uid', 'job-name', 'batch.kubernetes.io/job-name'):
        template_labels.pop(key, None)
    existing = Kube().get('job', name, optional=True)
    # The Job resolves its ConfigMap and Secret by name at scheduling time.
    # Recheck their captured source identities after the collision lookup and
    # immediately before accepting or creating the replay Job.
    revalidate()
    if existing is not None:
        own(existing, name=name)
        if (existing.get('metadata', {}).get('annotations', {}) != annotations
                or existing.get('spec') != job['spec']
                or existing.get('status', {}).get('succeeded') != 1):
            fail('bootstrap_replay_job_name_collision')
        return name
    Kube().run('create', '-f', '-', body=canonical(job), timeout=20)
    Kube().run('wait', '--for=condition=complete', 'job/' + name, '--timeout=180s', timeout=190)
    created = own(Kube().get('job', name), name=name)
    if (created.get('metadata', {}).get('annotations', {}) != annotations
            or created.get('status', {}).get('succeeded') != 1):
        fail('bootstrap_replay_job_not_complete')
    return name


def assert_bootstrap_transition(before, after, candidate, actor_sha256, plan_sha256):
    for key in ('namespace', 'marker', 'profile', 'baseline_binding_id', 'operator', 'service', 'bootstrap_source',
                'deployment', 'nats_service', 'hub_config', 'nats_workload', 'nats_replica_set', 'bootstrap_job', 'bootstrap_config',
                'bootstrap_config_enrollment', 'nats_pod'):
        if before.get(key) != after.get(key):
            fail('bootstrap_unrelated_state_changed')
    old = before['bootstrap']
    new = after['bootstrap']
    source = before['bootstrap_source']
    annotations = {} if new is None else new.get('metadata', {}).get('annotations', {})
    if (old is None or source is None or old['metadata']['name'] != BOOTSTRAP_SECRET
            or old != source or new is None
            or new['metadata'].get('name') != BOOTSTRAP_SECRET + '-music-' + before['baseline_binding_id']
            or new['metadata'].get('uid') == old['metadata'].get('uid')
            or new.get('immutable') is not True or get_data(new, 'bootstrap.creds') != candidate
            or annotations.get('voice.io/dev-music-actor-sha256') != actor_sha256
            or annotations.get('voice.io/dev-music-plan-sha256') != plan_sha256
            or new.get('metadata', {}).get('annotations', {}).get('voice.io/dev-music-source-secret-uid') != source['metadata']['uid']
            or new.get('metadata', {}).get('annotations', {}).get('voice.io/dev-music-source-secret-rv') != source['metadata']['resourceVersion']):
        fail('bootstrap_enrollment_transition_invalid')


def private_directory(path):
    path = Path(path)
    info = path.lstat()
    if (path.resolve(strict=True) != path or not stat.S_ISDIR(info.st_mode) or
            info.st_uid != 0 or stat.S_IMODE(info.st_mode) != 0o700):
        fail('root_private_directory_required')
    parent = path.parent
    while parent != Path('/home/voice-dev'):
        row = parent.lstat()
        if (parent.resolve(strict=True) != parent or not stat.S_ISDIR(row.st_mode) or
                row.st_uid != 0 or row.st_mode & 0o022):
            fail('root_private_directory_parent_invalid')
        if parent == Path('/'):
            fail('operator_directory_outside_voice_dev')
        parent = parent.parent
    row = parent.lstat()
    if parent.resolve(strict=True) != parent or row.st_uid != 0 or row.st_mode & 0o022:
        fail('root_private_directory_parent_invalid')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('plan', 'apply'))
    parser.add_argument('--profile', choices=(PROFILE,), required=True,
                        help='explicit bounded DEV deployment profile')
    parser.add_argument('--mode', choices=('messaging', 'bootstrap'), default='messaging',
                        help='bootstrap is a separate explicit consumer-create operation')
    parser.add_argument('--plan-sha256', help='exact digest printed by a fresh read-only plan')
    args = parser.parse_args()
    if sys.platform != 'linux' or os.geteuid() != 0:
        fail('root_linux_operator_required')
    import fcntl
    old_umask = os.umask(0o077)
    try:
        private_directory(ROOT)
        kubeconfig = KUBECONFIG.lstat()
        if (not stat.S_ISREG(kubeconfig.st_mode) or kubeconfig.st_uid != 0 or
                kubeconfig.st_mode & 0o077 or kubeconfig.st_nlink != 1):
            fail('root_private_kubeconfig_required')
        lockfd = os.open(ROOT / 'nats-music.lock', os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
        with os.fdopen(lockfd, 'w') as lock:
            lock_info = os.fstat(lock.fileno())
            if (lock_info.st_uid != 0 or lock_info.st_nlink != 1 or lock_info.st_mode & 0o077):
                fail('root_private_lock_invalid')
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            state = capture(Kube(), args.profile)
            issuer_info = ISSUER.lstat()
            if (not stat.S_ISREG(issuer_info.st_mode) or issuer_info.st_uid != 0 or issuer_info.st_mode & 0o077):
                fail('issuer_binary_custody_invalid')
            issuer_sha = sha(ISSUER.read_bytes())
            probe_info = PROBE.lstat()
            if (not stat.S_ISREG(probe_info.st_mode) or probe_info.st_uid != 0 or
                    probe_info.st_nlink != 1 or probe_info.st_mode & 0o077):
                fail('broker_probe_binary_custody_invalid')
            probe_sha = sha(PROBE.read_bytes())
            acl_raw = ACL.read_bytes()
            sys.path.insert(0, str(STAGING))
            import actor_root
            grants = actor_root.source_grants(acl_raw)
            grant = grants.get('messaging')
            if (not isinstance(grant, dict) or grant.get('no_response') is not True
                    or not set(SERVICE_ADDITIONS) <= set(grant.get('publish', []))):
                fail('music_acl_intent_changed')
            actor = get_data(state['service'], 'messaging.creds')
            bootstrap_actor = None if state['bootstrap'] is None else get_data(state['bootstrap'], 'bootstrap.creds')
            public = get_data(state['operator'], 'account.public').decode('ascii').strip()
            if not public:
                fail('account_public_key_invalid')
            receipt, actor_public, effective = verify_actor(ISSUER, actor, state['operator'], grant,
                                                            require_grant=False)
            bootstrap_status = bootstrap_claims(ISSUER, state['bootstrap'], state['operator'], grants)
            if args.mode == 'bootstrap':
                if state['bootstrap'] is None:
                    fail('bootstrap_secret_missing')
                bootstrap_grant = grants.get('bootstrap')
                if (not isinstance(bootstrap_grant, dict)
                        or not set(BOOTSTRAP_ADDITIONS) <= set(bootstrap_grant.get('publish', []))):
                    fail('bootstrap_acl_intent_changed')
                bootstrap_job_source, bootstrap_image, bootstrap_prepared_sha, _ = prepared_bootstrap_job(
                    state['bootstrap_config'].get('data') if state['bootstrap_config'] else None,
                    state['baseline_binding_id'], state['bootstrap_config_data'])
                state['bootstrap_job_source'] = bootstrap_job_source
                bootstrap_receipt, bootstrap_public, bootstrap_effective = verify_actor(
                    ISSUER, bootstrap_actor, state['operator'], bootstrap_grant, role='bootstrap', require_grant=False)
                if (state['bootstrap']['metadata']['name'] != BOOTSTRAP_SECRET
                        and state['bootstrap'].get('metadata', {}).get('annotations', {}).get(
                            'voice.io/dev-music-actor-sha256') != bootstrap_receipt['actor_sha256']):
                    fail('bootstrap_enrollment_actor_changed')
                if state['bootstrap_job'] is not None:
                    validate_bootstrap_job(state['bootstrap_job'], state['baseline_binding_id'], state['bootstrap'],
                                           state['bootstrap_config_target_available'], bootstrap_image,
                                           expected_secret_name=BOOTSTRAP_SECRET)
                else:
                    prepared_for_validation = json.loads(json.dumps(bootstrap_job_source))
                    prepared_for_validation['spec']['suspend'] = False
                    for volume in prepared_for_validation['spec']['template']['spec'].get('volumes', []):
                        if volume.get('name') == 'nats-bootstrap-creds':
                            volume['secret']['secretName'] = state['bootstrap']['metadata']['name']
                    validate_bootstrap_job(prepared_for_validation, state['baseline_binding_id'], state['bootstrap'],
                                           state['bootstrap_config_target_available'], bootstrap_image)
            else:
                bootstrap_receipt = None
                bootstrap_public = None
                bootstrap_effective = None
                bootstrap_prepared_sha = None
            state_digest, binding = snapshot_hash(state, acl_raw, issuer_sha)
            with tempfile.TemporaryDirectory(prefix='.nats-music-probe-', dir=ROOT) as scratch:
                os.chmod(scratch, 0o700)
                probe_actor = bootstrap_actor if args.mode == 'bootstrap' else actor
                broker = run_probe(Kube(), state['nats_pod'], probe_actor, scratch,
                                   role='bootstrap' if args.mode == 'bootstrap' else 'messaging')
                if args.mode == 'bootstrap':
                    action = bootstrap_action(bootstrap_effective, grants['bootstrap'], broker,
                                              config_matches=state['bootstrap_config_target_available'],
                                              job=state['bootstrap_job'], prepared_available=True)
                else:
                    action = decision(effective, grant, broker)
                bootstrap_complete = state['bootstrap_job'] is not None and \
                    state['bootstrap_job'].get('status', {}).get('succeeded', 0) == 1
                bootstrap_uid = None if state['bootstrap'] is None else state['bootstrap']['metadata']['uid']
                plan_digest = sha(canonical({'state_sha256': state_digest, 'probe_sha256': probe_sha,
                    'mode': args.mode,
                    'actor_sha256': bootstrap_receipt['actor_sha256'] if args.mode == 'bootstrap' else receipt['actor_sha256'],
                    'effective_permissions_sha256': sha(canonical(bootstrap_effective if args.mode == 'bootstrap' else effective)), 'broker': broker,
                    'decision': action, 'bootstrap_complete': bootstrap_complete,
                    'bootstrap_config_target_available': state['bootstrap_config_target_available'],
                    'bootstrap_target_source_sha256': None if state['bootstrap_config_data'] is None else
                        sha(canonical(state['bootstrap_config_data'])),
                    'bootstrap_config_enrollment_uid': None if state['bootstrap_config_enrollment'] is None else
                        state['bootstrap_config_enrollment']['metadata']['uid'],
                    'bootstrap_secret_uid': bootstrap_uid, 'bootstrap_signed_claims': bootstrap_status,
                    'bootstrap_prepared_sha256': bootstrap_prepared_sha}))
                result = {'schema': 'voice-dev-nats-music-plan-v1', 'plan_sha256': plan_digest,
                          'mode': args.mode,
                          'profile': PROFILE, 'baseline_binding_id': state['baseline_binding_id'],
                          'actor_sha256': bootstrap_receipt['actor_sha256'] if args.mode == 'bootstrap' else receipt['actor_sha256'],
                          'actor_jwt_sha256': bootstrap_receipt['actor_jwt_sha256'] if args.mode == 'bootstrap' else receipt['actor_jwt_sha256'],
                          'effective_permissions_sha256': sha(canonical(bootstrap_effective if args.mode == 'bootstrap' else effective)),
                          'broker_probe_sha256': probe_sha,
                          'broker_authenticated': broker['broker_authenticated'],
                          'broker_info_outcome': broker['outcome'],
                          'consumer_info_authorized': broker['consumer_info_authorized'],
                          'consumer_exists': broker['consumer_exists'],
                          'consumer_matches_source_contract': broker['consumer_matches'], 'decision': action,
                          'bootstrap_job_complete': bootstrap_complete,
                          'bootstrap_config_target_available': state['bootstrap_config_target_available'],
                          'bootstrap_target_source_sha256': None if state['bootstrap_config_data'] is None else
                              sha(canonical(state['bootstrap_config_data'])),
                          'bootstrap_config_enrollment_uid': None if state['bootstrap_config_enrollment'] is None else
                              state['bootstrap_config_enrollment']['metadata']['uid'],
                          'bootstrap_signed_claims': bootstrap_status,
                          'bootstrap_secret_uid': bootstrap_uid,
                          'bootstrap_prepared_sha256': bootstrap_prepared_sha,
                          'binding': binding}
                if args.action == 'plan':
                    print(json.dumps(result, sort_keys=True, separators=(',', ':')))
                    return 0
                if args.plan_sha256 != plan_digest:
                    fail('plan_digest_confirmation_required')
                if action in ('current', 'consumer_already_present'):
                    print(json.dumps(dict(result, outcome='unchanged_actor_already_authorized'), sort_keys=True))
                    return 0
                if args.mode == 'bootstrap':
                    state = require_current(state_digest, state, issuer_sha, probe_sha, bootstrap_prepared_sha)
                    grant = grants['bootstrap']
                    candidate = bootstrap_actor
                    if action == 'renew_bootstrap_then_replay':
                        if state['bootstrap']['metadata']['name'] != BOOTSTRAP_SECRET:
                            fail('bootstrap_enrollment_stale_requires_new_baseline')
                        baseline = {'pub': [x for x in grant['publish'] if x not in BOOTSTRAP_ADDITIONS],
                                    'sub': grant['subscribe']}
                        actor_permissions.evaluate(bootstrap_effective, baseline)
                        seed = private_file(SEED, 59)
                        try:
                            renewal = bootstrap_renewal_request(
                                bootstrap_actor, state['operator'], seed, bootstrap_receipt)
                            candidate = invoke_issuer(ISSUER, renewal, '--renew-existing-bootstrap')
                        finally:
                            for index in range(len(seed)):
                                seed[index] = 0
                        candidate_receipt, candidate_public, candidate_effective = verify_actor(
                            ISSUER, candidate, state['operator'], grant, role='bootstrap')
                        if candidate_public != bootstrap_public:
                            fail('bootstrap_renewal_changed_actor_identity')
                        actor_permissions.evaluate(candidate_effective, {'pub': grant['publish'], 'sub': grant['subscribe']})
                        check_dir = tempfile.TemporaryDirectory(prefix='.nats-bootstrap-validate-', dir=ROOT)
                        try:
                            os.chmod(check_dir.name, 0o700)
                            candidate_info = run_probe(Kube(), state['nats_pod'], candidate, check_dir.name,
                                                       role='bootstrap')
                        finally:
                            check_dir.cleanup()
                        if candidate_info.get('consumer_info_authorized') is not True:
                            fail('renewed_bootstrap_info_not_proven')
                        state = require_current(state_digest, state, issuer_sha, probe_sha, bootstrap_prepared_sha)
                        old_state = state
                        backup = create_bootstrap_enrollment(state, candidate,
                                                             candidate_receipt['actor_sha256'], args.plan_sha256)
                        state = capture(Kube())
                        assert_bootstrap_transition(old_state, state, candidate,
                                                    candidate_receipt['actor_sha256'], args.plan_sha256)
                        state['bootstrap_job_source'] = bootstrap_job_source
                    else:
                        backup = None
                    if sha(private_file(BOOTSTRAP_PREPARED)) != bootstrap_prepared_sha:
                        fail('bootstrap_prepared_source_changed')
                    configmap_before = state
                    configmap_before_digest, _ = snapshot_hash(state, acl_raw, issuer_sha)
                    def revalidate_configmap_source():
                        return require_current(configmap_before_digest, configmap_before,
                                               issuer_sha, probe_sha, bootstrap_prepared_sha)
                    enrolled_config = create_bootstrap_configmap_enrollment(
                        configmap_before, args.plan_sha256, revalidate=revalidate_configmap_source)
                    state = capture(Kube())
                    assert_bootstrap_configmap_transition(configmap_before, state, enrolled_config)
                    state['bootstrap_job_source'] = bootstrap_job_source
                    if state['bootstrap_config_enrollment'] is None:
                        fail('bootstrap_configmap_enrollment_not_observed')
                    replay_digest, _ = snapshot_hash(state, acl_raw, issuer_sha)
                    created = replay_bootstrap_job(
                        state, args.plan_sha256, bootstrap_image,
                        revalidate=lambda: require_current(
                            replay_digest, state, issuer_sha, probe_sha, bootstrap_prepared_sha))
                    with tempfile.TemporaryDirectory(prefix='.nats-music-confirm-', dir=ROOT) as confirm_dir:
                        os.chmod(confirm_dir, 0o700)
                        after = run_probe(Kube(), state['nats_pod'], candidate, confirm_dir, role='bootstrap')
                    if (after.get('consumer_exists') is not True or after.get('consumer_matches') is not True
                            or after.get('consumer_info_authorized') is not True):
                        fail('music_consumer_not_proven_after_bootstrap')
                    print(json.dumps({'schema': 'voice-dev-nats-music-bootstrap-v1',
                                      'profile': PROFILE, 'baseline_binding_id': state['baseline_binding_id'],
                                      'bootstrap_actor_sha256': bootstrap_receipt['actor_sha256'],
                                      'consumer_exists': True, 'job_name': created,
                                      'bootstrap_credential_backup': None if backup is None else backup.name,
                                      'restart_required': False, 'outcome': 'consumer_created_from_exact_bootstrap_job'},
                                     sort_keys=True, separators=(',', ':')))
                    return 0
                # Fresh revalidation precedes issuing and again before mutation.
                state = require_current(state_digest, state, issuer_sha, probe_sha)
                stable = stable_hash(state, acl_raw, issuer_sha)
                import actor_permissions
                baseline = {'pub': [x for x in grant['publish'] if x not in SERVICE_ADDITIONS],
                            'sub': grant['subscribe']}
                actor_permissions.evaluate(effective, baseline)
                seed = private_file(SEED, 59)
                try:
                    request = service_renewal_request(actor, state['operator'], seed, receipt, grant)
                    candidate = invoke_issuer(ISSUER, request, '--renew-existing-service')
                finally:
                    for i in range(len(seed)):
                        seed[i] = 0
                candidate_receipt, candidate_public, candidate_effective = verify_actor(
                    ISSUER, candidate, state['operator'], grant)
                if candidate_public != actor_public:
                    fail('renewal_changed_actor_identity')
                actor_permissions.evaluate(candidate_effective, {'pub': grant['publish'], 'sub': grant['subscribe']})
                candidate_broker = run_probe(Kube(), state['nats_pod'], candidate, scratch)
                if candidate_broker.get('consumer_info_authorized') is not True:
                    fail('renewed_actor_broker_info_not_proven')
                state = require_current(state_digest, state, issuer_sha, probe_sha)
                deployment = state['deployment']
                replicas = deployment.get('spec', {}).get('replicas', 1)
                if type(replicas) is not int or replicas < 1:
                    fail('messaging_replica_state_invalid')
                if state['service'].get('immutable') is True:
                    fail('immutable_service_secret_requires_separate_review')
                # A root-only exact backup is durable before stopping the app.
                backup = write_backup(state, state['service'])
                kube = Kube()
                before = kube.get('deployment', 'voice-messaging')
                if before != deployment:
                    fail('messaging_deployment_changed')
                kube.run('scale', 'deployment/voice-messaging', '--replicas=0',
                         '--current-replicas=' + str(replicas),
                         '--resource-version=' + deployment['metadata']['resourceVersion'])
                deadline = time.monotonic() + 30
                selector = deployment['spec']['selector']['matchLabels']
                while time.monotonic() < deadline:
                    pods = kube.list('pods', ','.join(f'{k}={v}' for k, v in selector.items()))
                    if not pods:
                        break
                    time.sleep(.25)
                else:
                    fail('messaging_pods_did_not_stop')
                stopped_state = capture(Kube())
                if stable_hash(stopped_state, ACL.read_bytes(), issuer_sha) != stable:
                    fail('operator_state_changed_during_stop')
                updated = json.loads(json.dumps(state['service']))
                updated['data']['messaging.creds'] = base64.b64encode(candidate).decode('ascii')
                kube.run('replace', '-f', '-', body=canonical(updated), timeout=20)
                readback = kube.get('secret', state['service']['metadata']['name'])
                if (metadata(readback)['uid'] != metadata(state['service'])['uid'] or
                        get_data(readback, 'messaging.creds') != candidate or
                        any(readback['data'].get(k) != v for k, v in state['service']['data'].items()
                            if k != 'messaging.creds')):
                    fail('messaging_secret_readback_mismatch')
                scaled = kube.get('deployment', 'voice-messaging')
                scaled_spec = json.loads(json.dumps(scaled.get('spec', {})))
                original_spec = json.loads(json.dumps(deployment.get('spec', {})))
                scaled_spec.pop('replicas', None)
                original_spec.pop('replicas', None)
                if (scaled.get('spec', {}).get('replicas') != 0 or
                        scaled['metadata']['uid'] != deployment['metadata']['uid'] or scaled_spec != original_spec):
                    fail('messaging_scale_state_changed')
                kube.run('scale', 'deployment/voice-messaging', '--replicas=' + str(replicas),
                         '--current-replicas=0', '--resource-version=' + scaled['metadata']['resourceVersion'])
                kube.run('rollout', 'status', 'deployment/voice-messaging', '--timeout=60s', timeout=70)
                restored = kube.get('deployment', 'voice-messaging')
                if restored['metadata']['uid'] != deployment['metadata']['uid'] or restored.get('spec', {}).get('replicas') != replicas:
                    fail('messaging_deployment_restore_mismatch')
                if restored.get('status', {}).get('readyReplicas') != replicas:
                    fail('messaging_deployment_not_ready_after_restore')
                with tempfile.TemporaryDirectory(prefix='.nats-music-confirm-', dir=ROOT) as confirm_dir:
                    os.chmod(confirm_dir, 0o700)
                    after = run_probe(kube, state['nats_pod'], candidate, confirm_dir)
                if after.get('consumer_info_authorized') is not True:
                    fail('restored_actor_broker_info_not_proven')
                ready_pods = kube.list('pods', ','.join(f'{k}={v}' for k, v in deployment['spec']['selector']['matchLabels'].items()))
                if len(ready_pods) != replicas or any(
                        p.get('status', {}).get('phase') != 'Running' or
                        any(c.get('ready') is not True for c in p.get('status', {}).get('containerStatuses', []))
                        for p in ready_pods):
                    fail('messaging_pods_not_ready_after_restore')
                print(json.dumps({'schema': 'voice-dev-nats-music-renewal-v1',
                                  'profile': PROFILE, 'baseline_binding_id': state['baseline_binding_id'],
                                  'actor_sha256': candidate_receipt['actor_sha256'],
                                  'credential_sha256': sha(candidate), 'backup': backup.name,
                                  'consumer_exists': candidate_broker['consumer_exists'],
                                  'restart_required': True, 'outcome': 'secret_replaced_and_workload_restored'},
                                 sort_keys=True, separators=(',', ':')))
                return 0
    except Exception:
        print('DEV Music NATS maintenance refused; inspect root-only operator state', file=sys.stderr)
        return 1
    finally:
        os.umask(old_umask)


if __name__ == '__main__':
    sys.exit(main())
