"""Test-only owned enrollment composition. Never installed or called by a CLI.

The caller supplies an already frozen owned authority and independently created
fixture material. This module does not create keys, publish initial Kube objects,
or qualify an installation; its caller must do those steps before invoking it.
"""
import copy
import hashlib

import actor_auth
import actor_root
import actor_verification
import bootstrap_auth
import bootstrap_enrollment
import bootstrap_selection
from bootstrap_root import secret_bytes
from controller import Blocked
from nats_contract_plan import digest
from nats_root_plan import _LEGACY_AUTH_KEYS, _validated_normalized_consumers
from resource_claim import require_installation

RENEWER_SHA256 = '618f1f78e650d19c4948c1d08741ed625c6f9aa706159219dbf11c785a56237c'
# ACL captured at immutable Voice commit 2521ea0b8f8202051e9aee9e348cffef96b05afc.
# New source tuples need an independently reviewed pin, never a caller override.
ACL_SHA256 = '5cee12a03b98b1ad8f609f288f0f3dd03a5b10cfa42ff4a8787657637cee8fc3'


def enroll_owned(kube, binding, credentials, operator, installation, binary,
                 canonical_acl, unchanged, *, previous=None):
    """Seal real enrollment/auth output under the preselected fixture domain.

    This invokes the real pinned verifier, real scratch authentication/prover,
    generic create-only enrollment and final live selection. It never uses the
    installed signer or promotes caller-supplied proof rows.
    """
    installation = require_installation(installation)
    if installation.profile != 'owned-isolated-v1':
        raise Blocked('owned_enrollment_requires_isolated_installation')
    if type(canonical_acl) is not bytes or hashlib.sha256(canonical_acl).hexdigest()!=ACL_SHA256:
        raise Blocked('owned_enrollment_acl_source_changed')
    grants = actor_root.source_grants(canonical_acl).get('bootstrap')
    if (type(grants) is not dict or set(grants) != {'publish','subscribe','no_response'}
        or grants['no_response'] is not True or not grants['publish'] or not grants['subscribe']):
        raise Blocked('owned_enrollment_acl_invalid')
    required = {'pub':grants['publish'], 'sub':grants['subscribe']}
    if (type(binding) is not dict or binding.get('generation') != installation.generation
        or binding.get('account_sha256') != installation.account_sha256
        or binding.get('actor_sha256') != installation.bootstrap_actor_sha256):
        raise Blocked('owned_enrollment_binding_invalid')
    fields={'generation','marker_uid','marker_rv','old_secret','operator_secret','actor_sha256','account_sha256'}
    if previous is not None:fields.add('previous_enrollment_sha256')
    if (set(binding)!=fields or type(binding['operator_secret']) is not dict
        or set(binding['operator_secret'])!={'name','uid','resourceVersion'}
        or type(binding['old_secret']) is not dict
        or set(binding['old_secret'])!={'name','uid','resourceVersion','credential_sha256'}):
        raise Blocked('owned_enrollment_binding_invalid')
    if (type(operator) is not dict or type(operator.get('metadata')) is not dict
        or any(operator['metadata'].get(key)!=value for key,value in binding['operator_secret'].items())):
        raise Blocked('owned_enrollment_operator_changed')
    account = secret_bytes(operator, 'account.public').decode().strip()
    if hashlib.sha256(account.encode()).hexdigest() != installation.account_sha256:
        raise Blocked('owned_enrollment_account_changed')
    if previous is None:
        if 'previous_enrollment_sha256' in binding:
            raise Blocked('owned_enrollment_predecessor_invalid')
    else:
        if (type(previous) is not dict or previous.get('schema') != 'voice-nats-bootstrap-enrollment-scoped-v1'
            or previous.get('installation_sha256') != installation.sha256
            or binding.get('previous_enrollment_sha256') != digest(previous)):
            raise Blocked('owned_enrollment_predecessor_invalid')
        bootstrap_selection.select(kube, previous, installation.generation, installation=installation)
    operator_jwt = secret_bytes(operator, 'operator.jwt').decode().strip()
    account_jwt = secret_bytes(operator, 'account.jwt').decode().strip()
    proof = {}
    sealed = {}

    def authenticate(creds):
        actor_verification.verify(binary, RENEWER_SHA256, creds, account_jwt, operator_jwt,
            installation.bootstrap_actor_sha256, installation.account_sha256, required,
            lambda value: actor_auth.prove(value, operator, binary, unchanged), unchanged)
        live = bootstrap_auth.prove(creds, operator, unchanged, installation=installation)
        if previous is None:
            proof.update({key: copy.deepcopy(live[key]) for key in _LEGACY_AUTH_KEYS})
        else:
            _validated_normalized_consumers(live, previous['auth_proof'])
            proof.update(copy.deepcopy(live))
        return True

    def commit(metadata):
        row = dict(metadata, schema='voice-nats-bootstrap-enrollment-scoped-v1',
                   installation_sha256=installation.sha256, auth_proof=copy.deepcopy(proof))
        if previous is not None:
            row.update(schema='voice-nats-bootstrap-enrollment-scoped-v4',
                       previous_enrollment=copy.deepcopy(previous), previous_enrollment_sha256=digest(previous))
        bootstrap_selection.select(kube, row, installation.generation, installation=installation)
        sealed.update(row)

    bootstrap_enrollment.enroll(bootstrap_enrollment.EnrollmentKube(kube), binding,
                                credentials, unchanged, authenticate, commit)
    if not sealed:
        raise Blocked('owned_enrollment_not_committed')
    return sealed
