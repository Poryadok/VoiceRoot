"""Offline-only checks for a pinned initial DEV media cohort packet.

No Kubernetes/TT mutations and no rollout entry point. Hash commitments do not
prove the referenced migration, OCI, backup or compatibility evidence.
"""
import hashlib
import json
import re

import resource_fencing

PINNED_PRODUCT = None
PINNED_PACKET_SHA256 = None
PINNED_PROVENANCE_SHA256 = None
COHORT = frozenset(('voice-file/file', 'voice-story/story', 'voice-messaging/messaging'))
PVC_NAMES = frozenset(('pgdata-voice-postgres-0', 'data-voice-minio-0',
                       'chdata-voice-clickhouse-0', 'voice-nats-jsdata'))
REQUIRED_EVIDENCE = frozenset(('oci_publication', 'migration', 'backup_restore',
                               'config_trust', 'dependency_compat'))
HASH = re.compile(r'[0-9a-f]{64}\Z')
COMMIT = re.compile(r'[0-9a-f]{40}\Z')
IMAGE = re.compile(r'[^\s@]+@sha256:[0-9a-f]{64}\Z')


def pairs(rows):
    result = {}
    for key, value in rows:
        if key in result:
            raise ValueError('duplicate packet key')
        result[key] = value
    return result


def load_pinned(raw):
    if (not isinstance(raw, bytes) or len(raw) > 1024 * 1024 or
            not isinstance(PINNED_PRODUCT, str) or not COMMIT.fullmatch(PINNED_PRODUCT) or
            not isinstance(PINNED_PACKET_SHA256, str) or not HASH.fullmatch(PINNED_PACKET_SHA256) or
            hashlib.sha256(raw).hexdigest() != PINNED_PACKET_SHA256):
        raise ValueError('reviewed exact product packet is not pinned')
    value = json.loads(raw, object_pairs_hook=pairs)
    if not isinstance(value, dict) or value.get('product_sha') != PINNED_PRODUCT:
        raise ValueError('packet product mismatch')
    return value


def validate_pinned(raw, provenance_raw, live, card, context, ancestor):
    plan = load_pinned(raw)
    if (not isinstance(provenance_raw, bytes) or len(provenance_raw) > 1024 * 1024 or
            not isinstance(PINNED_PROVENANCE_SHA256, str) or not HASH.fullmatch(PINNED_PROVENANCE_SHA256) or
            hashlib.sha256(provenance_raw).hexdigest() != PINNED_PROVENANCE_SHA256):
        raise ValueError('reviewed independent provenance is not pinned')
    provenance = json.loads(provenance_raw, object_pairs_hook=pairs)
    return _validate_initial(plan, provenance, live, card, context, PINNED_PRODUCT, ancestor)


def _validate_initial(plan, provenance, live, card, context, product, ancestor):
    resource_fencing.validate_context(context)
    required = {'schema', 'product_sha', 'namespace_uid', 'baseline_images', 'init_images', 'pvc_uids', 'mail_runtime',
                'workload_uids', 'workload_spec_sha256',
                'targets', 'sources', 'evidence'}
    if (not isinstance(plan, dict) or set(plan) != required or
            plan['schema'] != 'voice-dev-initial-media-v1' or plan['product_sha'] != product or
            not isinstance(product, str) or not COMMIT.fullmatch(product)):
        raise ValueError('packet identity invalid')
    baseline, targets, sources, evidence = (plan[k] for k in ('baseline_images', 'targets', 'sources', 'evidence'))
    if (not isinstance(baseline, dict) or len(baseline) != 49 or not COHORT <= baseline.keys() or
            any(not isinstance(k, str) or not re.fullmatch(r'voice-[a-z0-9-]+/[a-z0-9-]+', k) or
                not isinstance(v, str) or not (IMAGE.fullmatch(v) or
                (k == 'voice-dev-mail/mail' and v == 'node:24-alpine'))
                for k, v in baseline.items())):
        raise ValueError('complete bounded baseline required')
    if (not isinstance(targets, dict) or set(targets) != COHORT or
            any(not isinstance(v, str) or not IMAGE.fullmatch(v) or
                not v.startswith('ghcr.io/poryadok/voiceroot/' + name.split('/')[1] + '@sha256:')
                for name, v in targets.items())):
        raise ValueError('exact media cohort targets required')
    if context['commit'] != product or context['images'] != targets:
        raise ValueError('resource context candidate mismatch')
    if not isinstance(sources, dict) or set(sources) != COHORT:
        raise ValueError('per-component provenance required')
    if (not isinstance(provenance, dict) or set(provenance) != COHORT or
            any(not isinstance(provenance[name], dict) or
                provenance[name].get('image') != baseline[name] or
                provenance[name].get('source') != sources[name] or
                set(provenance[name]) != {'image', 'source'} for name in COHORT)):
        raise ValueError('independent provenance mismatch')
    for value in sources.values():
        if value == {'state': 'unattributed'}:
            continue
        if (not isinstance(value, dict) or set(value) != {'state', 'commit'} or value['state'] != 'known' or
                not isinstance(value['commit'], str) or not COMMIT.fullmatch(value['commit']) or
                not ancestor(value['commit'], product)):
            raise ValueError('known source must be a qualified ancestor')
    if (not isinstance(evidence, dict) or set(evidence) != REQUIRED_EVIDENCE or
            any(not isinstance(v, str) or not HASH.fullmatch(v) for v in evidence.values())):
        raise ValueError('evidence hash commitments required')
    if (not isinstance(plan['namespace_uid'], str) or not plan['namespace_uid'] or
            not isinstance(plan['pvc_uids'], dict) or set(plan['pvc_uids']) != PVC_NAMES or
            any(not isinstance(v, str) or not v for v in plan['pvc_uids'].values()) or
            len(set(plan['pvc_uids'].values())) != len(PVC_NAMES)):
        raise ValueError('namespace and PVC identities required')
    workload_uids, workload_specs = plan['workload_uids'], plan['workload_spec_sha256']
    if (not isinstance(workload_uids, dict) or len(workload_uids) != 31 or
            not isinstance(workload_specs, dict) or set(workload_uids) != set(workload_specs) or
            any(not isinstance(k, str) or not isinstance(v, str) or not v for k, v in workload_uids.items()) or
            len(set(workload_uids.values())) != 31 or
            any(not isinstance(v, str) or not HASH.fullmatch(v) for v in workload_specs.values())):
        raise ValueError('complete workload identities and specs required')
    names = []
    for key in workload_uids:
        kind, sep, name = key.partition('/')
        if not sep or kind not in ('Deployment', 'StatefulSet') or not re.fullmatch(r'voice-[a-z0-9-]+', name):
            raise ValueError('workload inventory identity invalid')
        names.append(name)
    if len(set(names)) != 31 or {key.split('/')[0] for key in baseline} != set(names):
        raise ValueError('workload and image inventories disagree')
    init_images = plan['init_images']
    if (not isinstance(init_images, dict) or set(init_images) != {'voice-nats-pvc-candidate/nats-config-renderer'} or
            'Deployment/voice-nats-pvc-candidate' not in workload_uids or
            'voice-nats-pvc-candidate/nats' not in baseline or
            not isinstance(init_images['voice-nats-pvc-candidate/nats-config-renderer'], str) or
            not IMAGE.fullmatch(init_images['voice-nats-pvc-candidate/nats-config-renderer'])):
        raise ValueError('exact init image inventory required')
    mail = plan['mail_runtime']
    if (baseline.get('voice-dev-mail/mail') != 'node:24-alpine' or not isinstance(mail, dict) or
            set(mail) != {'pod_uid', 'replicaset_uid', 'deployment_uid', 'node', 'container_id', 'image_id',
                          'restart_count', 'ready', 'started', 'pod_count', 'terminating_count', 'owner_chain_verified'} or
            any(not isinstance(mail[k], str) or not mail[k] for k in ('pod_uid', 'replicaset_uid', 'deployment_uid', 'node')) or
            mail['deployment_uid'] != workload_uids.get('Deployment/voice-dev-mail') or
            not isinstance(mail['container_id'], str) or not re.fullmatch(r'containerd://[0-9a-f]{64}', mail['container_id']) or
            not isinstance(mail['image_id'], str) or
            not re.fullmatch(r'docker\.io/library/node@sha256:[0-9a-f]{64}', mail['image_id']) or
            type(mail['restart_count']) is not int or mail['restart_count'] < 0 or
            mail['ready'] is not True or mail['started'] is not True or mail['owner_chain_verified'] is not True or
            type(mail['pod_count']) is not int or mail['pod_count'] != 1 or
            type(mail['terminating_count']) is not int or mail['terminating_count'] != 0):
        raise ValueError('tagged mail runtime identity required')
    if (not isinstance(live, dict) or live.get('namespace_uid') != plan['namespace_uid'] or
            live.get('images') != baseline or live.get('pvc_uids') != plan['pvc_uids'] or
            live.get('init_images') != init_images or
            live.get('workload_uids') != workload_uids or live.get('workload_spec_sha256') != workload_specs or
            live.get('mail_runtime') != mail):
        raise ValueError('live baseline drift')
    observed = card.get('observed', {}) if isinstance(card, dict) else {}
    candidate = card.get('candidate', {}) if isinstance(card, dict) else {}
    if (not isinstance(context, dict) or not isinstance(card, dict) or
            any(card.get(field) != context.get(key) for field, key in (
                ('owner', 'actor'), ('claim', 'claim'), ('window', 'window'),
                ('generation', 'generation'), ('revision', 'revision'))) or
            card.get('code') != 'DEV' or card.get('environment') != 'dev' or
            card.get('state') != 'held' or card.get('phase') != 'deploy' or
            observed.get('state') != 'known' or observed.get('commit') or observed.get('images') != baseline or
            observed.get('claim') != card.get('claim') or observed.get('window') != card.get('window') or
            type(card.get('operation')) is not int or observed.get('operation') != card['operation'] - 1 or
            candidate.get('state') != 'known' or candidate.get('commit') != product or candidate.get('images') != targets):
        raise ValueError('fresh image-only observation and candidate required')
    return True
