"""Closed JSON Patch contract for NATS bootstrap ConfigMap custody."""

import copy
import hashlib
import re


ANNOTATION = 'voice-nats-preservation/contract-operation'
NAMESPACE = 'voice-staging'
SCRIPT_KEY = 'bootstrap.sh'
CONFIGMAP_NAMES = frozenset({
    'voice-nats-realtime-bootstrap',
    'voice-nats-analytics-chat-bootstrap',
    'voice-nats-search-bootstrap',
})
_UID = re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}\Z')
_RV = re.compile(r'[1-9][0-9]*\Z')
_SHA256 = re.compile(r'[a-f0-9]{64}\Z')
_OPERATION = re.compile(r'[a-f0-9]{12}\Z')


class ConfigMapCASError(ValueError):
    """A ConfigMap CAS request or readback violated its closed contract."""


def _require(condition, reason):
    if not condition:
        raise ConfigMapCASError(reason)


def _validate_identity(namespace, name, expected_uid, expected_rv, expected_sha256, operation):
    _require(namespace == NAMESPACE, 'namespace_invalid')
    _require(name in CONFIGMAP_NAMES, 'configmap_name_invalid')
    _require(isinstance(expected_uid, str) and _UID.fullmatch(expected_uid), 'uid_invalid')
    _require(isinstance(expected_rv, str) and _RV.fullmatch(expected_rv), 'resource_version_invalid')
    _require(isinstance(expected_sha256, str) and _SHA256.fullmatch(expected_sha256), 'sha256_invalid')
    _require(isinstance(operation, str) and _OPERATION.fullmatch(operation), 'operation_invalid')


def build_request(namespace, name, expected_uid, expected_rv, expected_sha256,
                  operation, target_script, existing_annotations):
    """Build the only permitted CAS patch; caller must validate GET data digest first."""
    _validate_identity(namespace, name, expected_uid, expected_rv, expected_sha256, operation)
    _require(isinstance(target_script, str) and target_script, 'target_script_invalid')
    target_bytes = target_script.encode('utf-8')
    _require(len(target_bytes) <= 1024 * 1024, 'target_script_too_large')
    annotations_present = existing_annotations is not None
    if not annotations_present:
        existing_annotations = {}
    _require(isinstance(existing_annotations, dict), 'annotations_invalid')
    _require(all(isinstance(k, str) and isinstance(v, str)
                 for k, v in existing_annotations.items()), 'annotations_invalid')

    annotations = copy.deepcopy(existing_annotations)
    annotations[ANNOTATION] = operation
    return [
        {'op': 'test', 'path': '/metadata/uid', 'value': expected_uid},
        {'op': 'test', 'path': '/metadata/resourceVersion', 'value': expected_rv},
        {'op': 'replace', 'path': '/data/bootstrap.sh', 'value': target_script},
        {
            'op': 'replace' if annotations_present else 'add',
            'path': '/metadata/annotations',
            'value': annotations,
        },
    ]


def verify_readback(actual, namespace, name, expected_uid, previous_rv,
                    target_script, operation):
    """Validate the API readback without accepting a replacement object or stale CAS."""
    _require(namespace == NAMESPACE, 'namespace_invalid')
    _require(name in CONFIGMAP_NAMES, 'configmap_name_invalid')
    _require(isinstance(expected_uid, str) and _UID.fullmatch(expected_uid), 'uid_invalid')
    _require(isinstance(previous_rv, str) and _RV.fullmatch(previous_rv), 'resource_version_invalid')
    _require(isinstance(operation, str) and _OPERATION.fullmatch(operation), 'operation_invalid')
    _require(isinstance(target_script, str), 'target_script_invalid')
    _require(isinstance(actual, dict), 'readback_invalid')
    metadata = actual.get('metadata')
    data = actual.get('data')
    _require(isinstance(metadata, dict) and isinstance(data, dict), 'readback_invalid')
    _require(namespace == NAMESPACE and actual.get('metadata', {}).get('namespace') == namespace,
             'readback_namespace_invalid')
    _require(name in CONFIGMAP_NAMES and metadata.get('name') == name, 'readback_name_invalid')
    _require(metadata.get('uid') == expected_uid and isinstance(expected_uid, str)
             and _UID.fullmatch(expected_uid), 'readback_uid_invalid')
    rv = metadata.get('resourceVersion')
    _require(isinstance(rv, str) and _RV.fullmatch(rv) and rv != previous_rv,
             'readback_resource_version_invalid')
    _require(isinstance(target_script, str) and data.get(SCRIPT_KEY) == target_script,
             'readback_script_invalid')
    digest = hashlib.sha256(target_script.encode('utf-8')).hexdigest()
    annotations = metadata.get('annotations') or {}
    _require(isinstance(annotations, dict), 'readback_annotations_invalid')
    _require(annotations.get(ANNOTATION) == operation,
             'readback_operation_invalid')
    return {
        'name': name,
        'uid': expected_uid,
        'resource_version': rv,
        'script_sha256': digest,
        'operation': operation,
    }
