"""Validate the root-enrolled existing actor before normal capture inputs.

Receipt access is supplied by the protected root reader under operation_lock.
No signer access, renewal, creation, or silent credential fallback occurs here.
"""
import hashlib
import re
from bootstrap_enrollment import credential_hash
from bootstrap_root import GENERATION,ACCOUNT_SHA,ACTOR_SHA,secret_bytes
from docker_runtime import NATS_IMAGE
from nats_contract_plan import DECLARATION,digest
from controller import Blocked

def select(kube,receipt,generation):
    def fail():raise Blocked('enrolled_bootstrap_selection_refused')
    if receipt.get('schema') not in ('voice-nats-bootstrap-enrollment-v1','voice-nats-bootstrap-enrollment-v4') or receipt.get('verified') is not True:fail()
    if receipt['schema']=='voice-nats-bootstrap-enrollment-v4':
        previous=receipt.get('previous_enrollment')
        if (not isinstance(previous,dict) or previous.get('schema')!='voice-nats-bootstrap-enrollment-v1'
            or digest(previous)!=receipt.get('previous_enrollment_sha256')
            or receipt.get('binding',{}).get('previous_enrollment_sha256')!=digest(previous)):fail()
        select(kube,previous,generation)
        # Local import avoids a module-initialization cycle; the same exact
        # prior-one/prior-three to new-four rule governs enrollment and capture.
        # A later proof never rewrites a historical single-row enrollment.
        from nats_root_plan import _validated_normalized_consumers,_LEGACY_AUTH_KEYS,_V3_AUTH_KEYS
        if set(previous.get('auth_proof',{})) not in (_LEGACY_AUTH_KEYS,_V3_AUTH_KEYS):fail()
        _validated_normalized_consumers(receipt['auth_proof'],previous['auth_proof'])
        for key in ('generation','marker_uid','old_secret','operator_secret','actor_sha256','account_sha256'):
            if receipt['binding'].get(key)!=previous['binding'].get(key):fail()
    binding=receipt['binding'];auth=receipt['auth_proof'];chosen=receipt['secret']
    if generation!=GENERATION or binding.get('generation')!=generation:fail()
    marker=kube.get('configmap','voice-nats-generation')
    if marker['metadata']['uid']!=binding['marker_uid'] or marker['data'].get('generation')!=generation:fail()
    if binding.get('account_sha256')!=ACCOUNT_SHA or binding.get('actor_sha256')!=ACTOR_SHA:fail()
    operator=kube.get('secret','voice-nats-operator-'+generation)
    if any(operator['metadata'].get(k)!=binding['operator_secret'][k] for k in ('name','uid','resourceVersion')):fail()
    if hashlib.sha256(secret_bytes(operator,'account.public').decode().strip().encode()).hexdigest()!=ACCOUNT_SHA:fail()
    old=kube.get('secret','voice-nats-bootstrap-credentials-'+generation)
    if old.get('immutable') is not True or any(old['metadata'].get(k)!=binding['old_secret'][k] for k in ('name','uid','resourceVersion')) or credential_hash(old)!=binding['old_secret']['credential_sha256']:fail()
    if not re.fullmatch(r'voice-nats-bootstrap-contract-[a-f0-9]{24}',chosen.get('name','')):fail()
    current=kube.get('secret',chosen['name'])
    if (current.get('immutable') is not True or current.get('type')!='Opaque'
        or any(current['metadata'].get(k)!=chosen[k] for k in ('name','uid','resourceVersion'))
        or current['metadata'].get('namespace')!='voice-staging'
        or current['metadata'].get('labels')!={'voice-nats-generation':generation,'voice-nats-existing-bootstrap-renewal':'true'}
        or set(current.get('data',{}))!={'bootstrap.creds'} or credential_hash(current)!=receipt['credential_sha256']):fail()
    normalized=auth.get('normalized_consumer')
    if (auth.get('verified') is not True or auth.get('server_image')!=NATS_IMAGE or auth.get('account_sha256')!=ACCOUNT_SHA
        or auth.get('reply_prefix')!='_INBOX.voice.bootstrap.reply' or not isinstance(normalized,dict)
        or any(normalized.get(k)!=v for k,v in DECLARATION.items()) or digest(normalized)!=auth.get('normalized_consumer_sha256')):fail()
    return chosen['name']
