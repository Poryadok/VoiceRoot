"""Exact active-contract receipt versions accepted by the rollout bridge."""
import re
from controller import Blocked
from nats_contract_plan import DECLARATIONS,V3_DECLARATIONS,V3_MIGRATION_ID,V4_MIGRATION_ID,V5_SOURCE_HASHES,MIGRATION_ID as CURRENT_NATS_MIGRATION_ID

LEGACY_NATS_MIGRATION_ID='space-chat-social-24h-friend-removed-v1'
SOURCE_PATHS=('deploy/templates/nats-realtime-bootstrap.yaml',
    'deploy/templates/nats-analytics-chat-bootstrap.yaml')
VERSIONS={
    LEGACY_NATS_MIGRATION_ID:('voice-nats-additive-census-proof-v1',(('social_events','rt_realtime1_friend_removed'),)),
    V3_MIGRATION_ID:('voice-nats-additive-census-proof-v2',tuple((s,c['durable_name']) for s,c in V3_DECLARATIONS)),
    V4_MIGRATION_ID:('voice-nats-additive-census-proof-v3',tuple((s,c['durable_name']) for s,c in DECLARATIONS)),
    CURRENT_NATS_MIGRATION_ID:('voice-nats-additive-census-proof-v4',tuple((s,c['durable_name']) for s,c in DECLARATIONS)),
}
PROOF_KEYS={'schema','migration_id','before_sha256','after_sha256','messages',
    'old_consumers_preserved','created_consumers'}

def _sha256(value):return isinstance(value,str) and re.fullmatch(r'[a-f0-9]{64}',value) is not None

def _bounded_text(value):return isinstance(value,str) and 0<len(value)<=4096

def _recovery_attempt_matches(attempt,*,opened=False):
    required={'container_id','server_image','started_at','finished_at'}
    if opened:required=required|{'kind'}
    return (isinstance(attempt,dict) and set(attempt)==required
        and all(_bounded_text(attempt[key]) for key in required)
        and (not opened or attempt['kind']=='nats_contract_broker_opened'))

def _allowed_streams(migration_id):
    return ('chat_events','social_events','role_events')+(('voice_events','user_events') if migration_id==CURRENT_NATS_MIGRATION_ID else ())
def _stream_updates_match(value,migration_id):
    if not isinstance(value,dict) or len(value)>len(_allowed_streams(migration_id)):return False
    for stream,row in value.items():
        if (stream not in _allowed_streams(migration_id) or not isinstance(row,dict)
            or set(row)!={'attempt','target_config_sha256','native_created'}
            or not _recovery_attempt_matches(row.get('attempt'),opened=True)
            or not _sha256(row.get('target_config_sha256')) or not _bounded_text(row.get('native_created'))):
            return False
    return True

def _observations_match(observations,migration_id):
    if not isinstance(observations,list) or len(observations)>4096:return False
    consumer_keys={'stream','name','old_created','recovered_created','recovery','native_state_file_verified'}
    stream_keys={'stream','old_created','recovered_created','native_created','migration_attempt','fixed_update_verified'}
    for row in observations:
        if not isinstance(row,dict):return False
        if set(row)==consumer_keys:
            recovery=row.get('recovery')
            if (not all(_bounded_text(row.get(key)) for key in ('stream','name','old_created','recovered_created'))
                or row.get('native_state_file_verified') is not True or not isinstance(recovery,dict)
                or set(recovery) not in ({'server_image','container_id','started_at','finished_at'},
                    {'server_image','container_id','started_at','finished_at','stream_updates'},
                    {'server_image','container_id','started_at','finished_at','stream_updates','migration_id'})
                or not all(_bounded_text(recovery.get(key)) for key in ('server_image','container_id','started_at','finished_at'))
                or ('migration_id' in recovery and (migration_id!=CURRENT_NATS_MIGRATION_ID or recovery['migration_id']!=migration_id))
                or ('stream_updates' in recovery and not _stream_updates_match(recovery['stream_updates'],migration_id))):return False
        elif set(row)==stream_keys:
            if (row.get('stream') not in _allowed_streams(migration_id)
                or not all(_bounded_text(row.get(key)) for key in ('stream','old_created','recovered_created','native_created'))
                or row.get('fixed_update_verified') is not True
                or not _recovery_attempt_matches(row.get('migration_attempt'),opened=True)):
                return False
        else:return False
    return True

def _pairs(value):
    if not isinstance(value,list):return None
    pairs=[]
    for pair in value:
        if (not isinstance(pair,list) or len(pair)!=2
            or any(not isinstance(part,str) or not part for part in pair)):
            return None
        pairs.append(tuple(pair))
    if len(set(pairs))!=len(pairs):return None
    return pairs

def _proof_matches(proof,migration_id):
    if not isinstance(proof,dict):return False
    expected=PROOF_KEYS
    # Both reviewed producers emit the recovery-observation extension on the
    # changed-contract path; their no-op path emits the base proof shape. The
    # migration ID still fixes the census schema, so an extension never permits
    # a proof from the other producer version.
    if 'ephemeral_recovery_observations' in proof:
        expected=expected|{'ephemeral_recovery_observations'}
    if set(proof)!=expected:return False
    version=VERSIONS.get(migration_id)
    if version is None:return False
    census_schema=version[0]
    if proof.get('schema')!=census_schema or proof.get('migration_id')!=migration_id:
        return False
    if not _sha256(proof.get('before_sha256')) or not _sha256(proof.get('after_sha256')):
        return False
    for key in ('messages','old_consumers_preserved','created_consumers'):
        if type(proof.get(key)) is not int or proof[key]<0:return False
    if 'ephemeral_recovery_observations' in proof and not _observations_match(proof['ephemeral_recovery_observations'],migration_id):return False
    return True

def validate_identity(active_contract,expected,baseline_pairs):
    if isinstance(active_contract,dict) and active_contract.get('schema')=='voice-nats-reset-active-contract-v1':
        from operator_reset import validate_active
        return validate_active(active_contract,expected,baseline_pairs)
    if (not isinstance(active_contract,dict) or active_contract.get('schema')!='voice-nats-active-contract-v1'
        or active_contract.get('migration_id') not in VERSIONS
        or active_contract.get('generation')!=expected['generation']
        or active_contract.get('namespace_uid')!=expected['namespace_uid']
        or active_contract.get('pvc_uid')!=expected['source_claim_uid']):
        raise Blocked('nats_active_contract_identity_changed')
    migration_id=active_contract['migration_id']
    contract=active_contract.get('contract')
    baseline=_pairs(baseline_pairs)
    actual=_pairs(contract.get('consumer_pairs') if isinstance(contract,dict) else None)
    if baseline is None or actual is None:raise Blocked('nats_active_contract_identity_changed')
    additions=VERSIONS[migration_id][1]
    expected_pairs=sorted(set(baseline)|set(additions))
    if sorted(actual)!=expected_pairs:raise Blocked('nats_active_contract_identity_changed')
    sources=active_contract.get('sources')
    paths=tuple(V5_SOURCE_HASHES) if migration_id==CURRENT_NATS_MIGRATION_ID else SOURCE_PATHS
    if (not isinstance(sources,dict) or set(sources)!=set(paths)
        or (migration_id==CURRENT_NATS_MIGRATION_ID and sources!=V5_SOURCE_HASHES)
        or any(not _sha256(sources[path]) for path in paths)
        or not _sha256(active_contract.get('plan_sha256'))
        or not _proof_matches(active_contract.get('proof'),migration_id)):
        raise Blocked('nats_active_contract_identity_changed')
