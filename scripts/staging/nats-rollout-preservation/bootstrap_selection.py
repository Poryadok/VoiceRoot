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
from resource_claim import require_installation,production_installation

def select(kube,receipt,generation,*,installation=None):
    def fail():raise Blocked('enrolled_bootstrap_selection_refused')
    installation=require_installation(installation)
    if type(receipt) is not dict:fail()
    schema=receipt.get('schema')
    versions={'voice-nats-bootstrap-enrollment-v1':(False,False),'voice-nats-bootstrap-enrollment-v4':(False,True),
        'voice-nats-bootstrap-enrollment-scoped-v1':(True,False),'voice-nats-bootstrap-enrollment-scoped-v4':(True,True)}
    if schema not in versions:fail()
    scoped,successor=versions[schema]
    fields={'schema','binding','secret','credential_sha256','verified','auth_proof'}
    if scoped:fields.add('installation_sha256')
    if successor:fields.update(('previous_enrollment','previous_enrollment_sha256'))
    if (set(receipt)!=fields or receipt.get('verified') is not True
        or (scoped and receipt['installation_sha256']!=installation.sha256)
        or (not scoped and installation!=production_installation())):fail()
    binding=receipt['binding'];auth=receipt['auth_proof'];chosen=receipt['secret']
    binding_fields={'generation','marker_uid','marker_rv','account_sha256','actor_sha256','old_secret','operator_secret'}
    if successor:binding_fields.add('previous_enrollment_sha256')
    if (type(binding) is not dict or set(binding)!=binding_fields or type(chosen) is not dict
        or set(chosen)!={'name','uid','resourceVersion'} or type(auth) is not dict):fail()
    from nats_root_plan import _LEGACY_AUTH_KEYS,_V3_AUTH_KEYS,_V4_AUTH_KEYS
    if set(auth) not in (_LEGACY_AUTH_KEYS,_V3_AUTH_KEYS,_V4_AUTH_KEYS):fail()
    from nats_contract_plan import V3_DECLARATIONS,DECLARATIONS,V6_DECLARATIONS
    normalized=auth.get('normalized_consumer')
    if (auth.get('verified') is not True or auth.get('server_image')!=NATS_IMAGE
        or auth.get('account_sha256')!=installation.account_sha256 or auth.get('reply_prefix')!='_INBOX.voice.bootstrap.reply'
        or not isinstance(normalized,dict) or any(normalized.get(k)!=v for k,v in DECLARATION.items())
        or digest(normalized)!=auth.get('normalized_consumer_sha256')):fail()
    if set(auth)!=_LEGACY_AUTH_KEYS:
        if set(auth)==_V3_AUTH_KEYS:declarations=V3_DECLARATIONS
        elif auth.get('schema')=='voice-bootstrap-auth-v4':declarations=DECLARATIONS
        elif auth.get('schema')=='voice-bootstrap-auth-v5':declarations=V6_DECLARATIONS
        else:fail()
        rows=auth.get('normalized_consumers')
        if type(rows) is not list or len(rows)!=len(declarations) or digest(rows)!=auth.get('normalized_consumers_sha256'):fail()
        for row,(stream,declaration) in zip(rows,declarations):
            if (type(row) is not dict or set(row)!={'stream','durable','config'} or row['stream']!=stream
                or row['durable']!=declaration['durable_name'] or type(row['config']) is not dict
                or any(row['config'].get(k)!=v for k,v in declaration.items())):fail()
        if rows[0]['config']!=normalized:fail()
        if auth.get('schema')=='voice-bootstrap-auth-v5':
            roster=rows[-1]['config']
            if roster.get('deliver_subject','')!='' or roster.get('deliver_group','')!='' or roster.get('filter_subjects',[])!=[]:fail()
    if (type(binding['old_secret']) is not dict or set(binding['old_secret'])!={'name','uid','resourceVersion','credential_sha256'}
        or type(binding['operator_secret']) is not dict or set(binding['operator_secret'])!={'name','uid','resourceVersion'}):fail()
    if successor:
        previous=receipt.get('previous_enrollment')
        expected_previous='voice-nats-bootstrap-enrollment-scoped-v1' if scoped else 'voice-nats-bootstrap-enrollment-v1'
        if (not isinstance(previous,dict) or previous.get('schema')!=expected_previous
            or digest(previous)!=receipt.get('previous_enrollment_sha256')
            or receipt.get('binding',{}).get('previous_enrollment_sha256')!=digest(previous)):fail()
        select(kube,previous,generation,installation=installation)
        # Local import avoids a module-initialization cycle; the same exact
        # prior-one/prior-three to new-four rule governs enrollment and capture.
        # A later proof never rewrites a historical single-row enrollment.
        from nats_root_plan import _validated_normalized_consumers,_LEGACY_AUTH_KEYS,_V3_AUTH_KEYS
        if set(previous.get('auth_proof',{})) not in (_LEGACY_AUTH_KEYS,_V3_AUTH_KEYS):fail()
        _validated_normalized_consumers(receipt['auth_proof'],previous['auth_proof'])
        for key in ('generation','marker_uid','old_secret','operator_secret','actor_sha256','account_sha256'):
            if receipt['binding'].get(key)!=previous['binding'].get(key):fail()
    if generation!=installation.generation or binding.get('generation')!=generation:fail()
    marker=kube.get('configmap','voice-nats-generation')
    if marker['metadata']['uid']!=binding['marker_uid'] or marker['data'].get('generation')!=generation:fail()
    if binding.get('account_sha256')!=installation.account_sha256 or binding.get('actor_sha256')!=installation.bootstrap_actor_sha256:fail()
    operator=kube.get('secret','voice-nats-operator-'+generation)
    if any(operator['metadata'].get(k)!=binding['operator_secret'][k] for k in ('name','uid','resourceVersion')):fail()
    if hashlib.sha256(secret_bytes(operator,'account.public').decode().strip().encode()).hexdigest()!=installation.account_sha256:fail()
    old=kube.get('secret','voice-nats-bootstrap-credentials-'+generation)
    if old.get('immutable') is not True or any(old['metadata'].get(k)!=binding['old_secret'][k] for k in ('name','uid','resourceVersion')) or credential_hash(old)!=binding['old_secret']['credential_sha256']:fail()
    if not re.fullmatch(r'voice-nats-bootstrap-contract-[a-f0-9]{24}',chosen.get('name','')):fail()
    current=kube.get('secret',chosen['name'])
    if (current.get('immutable') is not True or current.get('type')!='Opaque'
        or any(current['metadata'].get(k)!=chosen[k] for k in ('name','uid','resourceVersion'))
        or current['metadata'].get('namespace')!='voice-staging'
        or current['metadata'].get('labels')!={'voice-nats-generation':generation,'voice-nats-existing-bootstrap-renewal':'true'}
        or set(current.get('data',{}))!={'bootstrap.creds'} or credential_hash(current)!=receipt['credential_sha256']):fail()
    return chosen['name']
