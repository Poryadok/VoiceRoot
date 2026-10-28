"""Finite F54 prototype observations. No approved production R/K exists here.

Append real producer events; validate their exact bytes and conservation. Raw
Git inventories are unit bindings/dispositions, never manufactured owner rows.
Neither history-boundary nor composition observation grants admission.
"""
import argparse
import base64
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shlex
import socket
import secrets
import subprocess
import sys
import tempfile
import zlib
from functools import lru_cache
from types import MappingProxyType

M = 'f54bcadab4aa19969281d7d35d76ebad91553d4d'
P1 = '0a0024900b5bb3ba2f6fee391615f0f4fa0864e3'
P2 = 'f15d718b2d20837ae5503dc049134b66592b3675'
O = '4d943fac27fd247d136d84ae145440e54485d3ab'
BASES = ['f3feccef198b444699e66c960b84dc9c77ba4bc0', '516e6d9d92f477bbf09b2c66ef2476b6956a9b15']
TREE = '4c24a19991d52acb2507590d4c95ff19c983d359'
SOURCE = 'd38b699d3d3d641fe2415ed7f841dfe1f7360bc1'
SCHEMA = 'F54-PRODUCER-OBSERVATION-v2'
ROOT = Path(__file__).resolve().parents[2]
_REPLAY_ROOT_BINDING = None
_TRACE_USED = False
PRODUCTION_AUTHORITY = False
PROTOTYPE_BASE = '6d1f60f955a79e0378a3532750d209e3a11c8f23'
AUTHOR_REFERENCE = 'd6ac6d2f4b85eb691a00f729eb2192e63ae0091f'
AUTHOR_REFERENCE_TREE = 'bb525a189552c98656392ba0c02f0cdcd3978a0c'
AUTHOR_REFERENCE_DELTA_SHA = '86c0e832ca3ba454016ae5ff35c40c1c7d2bd59aa0f0e52f8caacef7e80845ef'
AUTHOR_EVENT_BASE = 'ea845cc695b3cde2a13ba9a97345dd78795833e0'
AUTHOR_NATIVE_CONTENT_BASE = '8191a36ca3bea7d053df0dd25963ca5133284b41'
AUTHOR_HISTORICAL_SCOPE = dict(base='698d4f064f032ea74abf3ede2017100b26c8ee69',target='53f75f74523735ac594e21582bea659f567a7288',allow_direct_followup='true',partition_source_history='false')
AUTHOR_SOURCE_PATHS = ('scripts/ci/f13_f54_history.py','scripts/ci/f13_f54_history_test.py','scripts/ci/voice-db-runtime-provisioning-contract_test.sh','.agent/execplans/native-p3-master-source.md')
SPAN_AGGREGATE = '8afe8e90285aa59c19ac011ded7260d2f191fa59'
SPAN_BASE = '2445e81de07681cfd5f1491e4c793aaac747f571'
SPAN_HEAD = '23551116ac5e741d3ec4ff7b412b7c9e248539ff'
SPAN_NODES = ['4f5b30c27cb288b249ef5e517c79c1d766f4e6e1',SPAN_HEAD]
ORIGINAL_TRACE_SHA = 'e2d4184b796496e5d98448c146636b779024a6315f65bc055262c1e029a18c25'
ORIGINAL_PRODUCER_SHA = 'b09dfe5eee5cca0d760c833b8fe32f7547563558f83d235d4fa27bd899ddd7d8'
ORIGINAL_NATIVE_SHA = '0f6b40dce786a964dd8616e12cdbf51c9c9fb2c4b255e147c9df142a53d3c294'
J3071 = '3071ddad390443de4119c796cdad9899ff08f7fd'
J3071_P1 = 'a45862ca7e2e885ae4e36afc758c9e9fe39ca86c'
J3071_P2 = '140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd'
J3071_BASES = ['68927e5912ebc218050fcc9daf4b76cf4a9e3c86','e020da4b4edcded41bde9785615262beb1946ad2']
J3071_O = '7bd7dfba7e8f85bc0572617c6ef3af4a15e59172'
J3071_TREE = '9b60a7639f0108c61da3c7808d05e7ea1421faaa'
J3071_PACKET_SHA = 'ef81722052fe9e969a1dbf7a9a0222928500b79f11fdf9f95eb7c2eaadf3d306'
J3071_RULING_SHA = '5862f23d87e008b6a996a395bae28bc2ee6286cd512e827cb9099fdaedf7ec16'
J3071_PROTOTYPE_BASE = '3411f205844642954394a494b612e7b17b73f935'
J3071_MANUAL_REVIEW_SHA = 'd5da0faab44ab7b76f319fe23e316c57ffc4fed06d2817cb587e1b4e4517214d'
J3071_MANUAL_PACKET_SHA = '0b1d6efdf6441ca3bece84d968d72ab464f4dba378aecbfcd370813ee3e5148f'
J3071_AUTO_TREE = '73eb0ec068685e8f69ea1c119c4bc4754e2ff932'
J3071_DOC = 'docs/DEPLOYMENT.md'
J3071_DOC_OIDS = dict(O='d66b3db53db02d2df5d548ff1cca47cfb3dd80f1',bestA='d66b3db53db02d2df5d548ff1cca47cfb3dd80f1',bestB='d66b3db53db02d2df5d548ff1cca47cfb3dd80f1',
    P1='173f87eec759f2b7228454fd8c21ed3efef4591f',P2='90b7060f52156e96e0cf4f01da0534c3b1ff336d',AUTO='ffb31b64dc889a616a90be7073fbe2cdca444cc4',J='64b197ec4e2d811fa5516ad17aa1c6523262331c')
J3071_DOC_HASHES = dict(O='e035fe905332326f2482735af309c6d21df9a6e3606d490baf3c68283eac41ac',bestA='e035fe905332326f2482735af309c6d21df9a6e3606d490baf3c68283eac41ac',bestB='e035fe905332326f2482735af309c6d21df9a6e3606d490baf3c68283eac41ac',
    P1='34e842635d5d743d14576cdde8fd6e9ef791527d77a1b9b8af59de255e0fc564',P2='a4c2cc6316c3b046fe03bfebcb2c9478c7d3b3dc14dca1d191756dfcedf903a7',AUTO='7d7b20429c07361d058551f4d0e1a77ff090d62fce778e3660c5116e418cd890',J='342b0f63411e327250676c20e2853455701ce3787656cdfa6e40b2394b986b53')
J3071_INSERTIONS = [(39,2844,'4dd4d04a9699ea1ffd28b7b8e3ca471784941bcbe68c444c31ee7152f67c9793'),(39,626,'285dad0f5994b4acb2b044beff212cbc1f445f91d7c96ba41526346aa540b212')]
J3071_RANGE_RULING_SHA = 'b1bfbcacdeaf993e975afd993c8176e1379a6206c51503ddd0f0279d23df8f1a'
J3071_RANGE_FAILED_TRACE_SHA = '8c16448aa157f0666c9dd8cec95e90500bc1425b3b2beb26aa148923b2dea2bc'
J3071_RANGE = dict(integration='2cc9482255ba18cd948aacb567f4c4bdf8976b14',first='8191a36ca3bea7d053df0dd25963ca5133284b41',
    base='d4abb829622e9bd91b26842b61522e6840840867',target='909090a34743902799306974d6f3ebabe3c02f10',
    baseline='c08cf1ed3a18e54e86541c54fa731fc9b24e1a8c',side='a80f649cbe0a03671c25531ace4ac1bd6bf54756',
    direct='40afa80e6d1ca7faebf528b7cb78f32cdf707e5d',merge='fa1528a7c232a644218edf49811e2bb2e373105f',
    side_nodes=['447d77729c904943800f78b78937f46ab67739cd','ff35cc20928280ac88e1dbf6f013a8d99839afdf',
        '9dde1dedbfbc32d283056583de6c138ca241a56f','ae815afbea96f4da9006bb1d3b5c46f1408c637d',
        'a3502b516fa4cf109fec2d53c16db420700122a3','a80f649cbe0a03671c25531ace4ac1bd6bf54756'])
F_CORRECTED_UNIT_SHA = 'dc4a28dc896a6bc1da54fcc01aceba94eaa63fd4b1b1eb175eeb9dc8cb9c31a3'
F_CORRECTED_TUPLE = dict(merge='fa1528a7c232a644218edf49811e2bb2e373105f',
    first='40afa80e6d1ca7faebf528b7cb78f32cdf707e5d',source='a80f649cbe0a03671c25531ace4ac1bd6bf54756',
    base='c08cf1ed3a18e54e86541c54fa731fc9b24e1a8c',automatic_tree='ab7a4a4d4ed4cbf9663b4a752b95c9aeb21eac3f',
    result_tree='454e4a883b40a6cf1f59e4cfc1d7b0d95ac916ed',correction='909090a34743902799306974d6f3ebabe3c02f10',
    current='1c47dce37a88c2539c8dabb074d9eda46fb84999',source_ancestor='5bf6c4857c19fba2b91fb44eb9e9bc6ae816c485')
F_CORRECTED_REPAIR_PATHS = ['src/backend/voice/main.go','src/backend/voice/internal/spacelifecycle/service.go',
    'src/backend/voice/internal/spacelifecycle/effects.go','src/backend/voice/internal/spacelifecycle/recovery_publication_test.go']
UNIT_HASHES = {
    'corrected-contract-v3-PROPOSAL.json':'62a00947187fa58b2815b13877a2c15a94ecb588c8d5fc06ca46aceb1cc24dde',
    'corrected-history-composition-contract-v3-PROPOSED.md':'04ab09c4094a35e256c4cb6642f54cd24c22f58b166ad013e29f7470953ed943',
    'rudeus410-scalar-adjudication.md':'a4ae57d62556172567844319c00475b25cf61f9276a5580794af17293daa7ffc',
    'contract-PROPOSAL.json':'f8785bc3cc33dfb2ef68f0f3160d9e8862b1e82ce78fdea7ca65b409414edb33',
    'proposal.json':'fb1d3de95269e4e1bd860aa14b6d59d46dec85c87e5783dc3872c2807e39206a'}


class Rejected(ValueError): pass


def require(value, reason):
    if not value: raise Rejected(reason)


def bind_verified_replay_root(repo, source, source_hashes):
    """Configure only this new successor instance's historical observation root.

    Executable source identity is separate from the historical carrier files.
    The caller must independently bind this module to its frozen harness source.
    """
    global ROOT, _REPLAY_ROOT_BINDING
    require(_REPLAY_ROOT_BINDING is None and not _TRACE_USED, 'replay_root_bind_once_before_use')
    root = Path(repo)
    require(root.is_absolute() and root == root.resolve() and root.is_dir(), 'replay_absolute_observation_root')
    require(type(source) is str and re.fullmatch('[0-9a-f]{40}', source), 'replay_full_historical_source')
    require(type(source_hashes) is dict and set(source_hashes) == set(AUTHOR_SOURCE_PATHS), 'replay_complete_historical_carriers')
    for name, digest in source_hashes.items():
        require(type(digest) is str and re.fullmatch('[0-9a-f]{64}', digest), 'replay_historical_digest_shape')
        raw = (root / name).read_bytes()
        require(sha(raw) == digest and raw == git(root, 'show', source + ':' + name), 'replay_historical_carrier_bytes')
    identity = dict(observation_root=str(root), historical_source=source,
                    historical_carriers=tuple(sorted(source_hashes.items())),
                    executable_sha256=sha(Path(__file__).read_bytes()),
                    validator='validate_trace')
    ROOT = root
    _REPLAY_ROOT_BINDING = MappingProxyType(identity)
    return _REPLAY_ROOT_BINDING


def require_pending_operation(pending, key, operation):
    require(key in pending and pending[key]['op'] == operation, 'exact_pending_operation_ownership')
    return pending[key]


def validate_exact_conservation(item, operation, receipt):
    require(item['op'] == operation, 'exact_pending_operation_ownership')
    expected = 'exact_unit_supersession' in item
    require(('exact_unit_supersession' in receipt) == expected, 'conservation_exact_supersession_presence_iff_pending')
    if expected:
        require(receipt['exact_unit_supersession'] == item['exact_unit_supersession'],
                'conservation_consumes_exact_supersession_not_unused_sidecar')
    expected_J = 'exact_J_owner_permission' in item
    require(('exact_J_owner_permission' in receipt) == expected_J,
            'conservation_exact_J_permission_presence_iff_pending')
    if expected_J:
        require(receipt['exact_J_owner_permission'] == item['exact_J_owner_permission'],
                'conservation_consumes_distinct_exact_J_permission')
    expected_finite = 'finite_J_owner_permission' in item
    require(('finite_J_owner_permission' in receipt) == expected_finite,
            'conservation_finite_J_presence_iff_pending')
    if expected_finite:
        require(receipt['finite_J_owner_permission'] == item['finite_J_owner_permission'],
                'conservation_consumes_exact_finite_J_permission')


def sha(data): return hashlib.sha256(data).hexdigest()


def canonical(value): return json.dumps(value, sort_keys=True, separators=(',', ':')).encode()


def git(repo, *args):
    p = subprocess.run(['git', '--no-replace-objects', '-C', str(repo), *args], capture_output=True)
    require(p.returncode == 0, 'git_' + args[0]); return p.stdout


def raw_binding(value):
    data = value.encode() if isinstance(value, str) else value
    return dict(raw=data.decode(), bytes=len(data), sha256=sha(data))


def file_binding(name):
    p = Path(name)
    if not p.is_file(): return None
    data=p.read_bytes();return dict(bytes=len(data),sha256=sha(data))


def row(value):
    if value is None: return None
    require(value == raw_binding(value['raw']), 'row_byte_binding')
    parts = value['raw'].split('\t')
    require(len(parts) == 8 and parts[1] in ('true', 'false'), 'eight_actual_fields')
    return parts


def tree_entry(repo, rev, path):
    return git(repo, 'ls-tree', '-z', rev, '--', ':(literal)' + path).decode()


def entry_binding(repo, rev, path):
    raw=tree_entry(repo,rev,path);blob=None
    if raw:
        _,kind,value=raw.split('\t',1)[0].split()
        data=git(repo,'cat-file',kind,value);blob=dict(bytes=len(data),sha256=sha(data))
    return dict(tree_record=raw_binding(raw),blob=blob)


def last_event(path):
    if not path.exists() or not path.stat().st_size:return None
    size=path.stat().st_size;length=min(size,65536)
    with path.open('rb') as stream:
        while True:
            stream.seek(size-length);data=stream.read(length).rstrip(b'\n')
            if b'\n' in data or length==size:return json.loads(data.split(b'\n')[-1])
            length=min(size,length*2)


def classifier_binding():
    p = ROOT / 'scripts/ci/voice-r22-runtime-scope.sh'
    return dict(path='scripts/ci/voice-r22-runtime-scope.sh', sha256=sha(p.read_bytes()),
        producer_sha256=sha((ROOT/'scripts/ci/voice-db-runtime-provisioning-contract_test.sh').read_bytes()))


def classify(paths):
    p = subprocess.run(['bash', '-c', 'source "$1"; voice_r22_runtime_changed', 'f54-classifier',
        str(ROOT/'scripts/ci/voice-r22-runtime-scope.sh')], input=('\n'.join(paths)+'\n').encode(), capture_output=True)
    require(p.returncode in (0, 1), 'classifier_execution')
    return 'true' if p.returncode == 0 else 'false'


def write_events(path, events):
    """Test/private transport helper; rehashing does not make forged events valid."""
    previous = '0' * 64; output = []
    for sequence, source in enumerate(events, 1):
        e = dict(source, sequence=sequence, previous=previous)
        e.pop('digest', None); e['digest'] = sha(canonical(e)); previous = e['digest']
        output.append(json.dumps(e, sort_keys=True))
    Path(path).write_text('\n'.join(output)+'\n')


def append(path, kind, frame, operation, repo, values):
    """Called synchronously at actual Bash producer boundaries, before mutation."""
    p = Path(path)
    previous = last_event(p)
    sequence = previous['sequence'] + 1 if previous else 1
    payload = {}
    identity = ''
    if kind == 'frame_enter':
        parent, start, target, allow, partition, output, parent_operation = values
        identity = 'F' + str(sequence)
        payload = dict(id=identity, parent=parent or None,parent_operation=parent_operation or None,repo=str(Path(repo).resolve()),base=start,target=target,
            allow_direct_followup=allow, partition_source_history=partition, output=output,
            first_parent_operations=git(repo,'rev-list','--first-parent','--reverse',start+'..'+target).decode().splitlines(),
            initial_map=file_binding(output))
    elif kind == 'frame_exit':
        code, output, category = values
        payload = dict(exit=int(code), final_map=file_binding(output), category=Path(category).read_text().strip() if category and Path(category).is_file() else None)
    elif kind == 'operation_attempt':
        commit,index,first,source,classification=values
        identity=frame+'/'+commit+'/'+index
        parents=git(repo,'rev-list','--parents','-n','1',commit).decode().split()[1:]
        payload=dict(id=identity,commit=commit,parent_index=int(index),ordered_parents=parents,first_parent=first,
            source_head=source,classification=classification,candidates=git(repo,'merge-base','--all',first,source).decode().splitlines())
    elif kind == 'operation_begin':
        commit, index, first, boundary, source, candidates, classification, delta = values
        identity = frame + '/' + commit + '/' + index
        paths = Path(delta).read_text().splitlines()
        parents = git(repo,'rev-list','--parents','-n','1',commit).decode().split()[1:]
        payload = dict(id=identity, commit=commit, ordered_parents=parents, parent_index=int(index),
            first_parent=first, boundary=boundary, source_head=source, candidates=candidates.split(','),
            classification='direct' if classification=='true' else 'merge' if classification=='false' else classification,
            delta=raw_binding(Path(delta).read_bytes()), paths=paths,
            entries={name:dict(base=entry_binding(repo,boundary,name),source=entry_binding(repo,source,name),result=entry_binding(repo,commit,name)) for name in paths},
            ordered_parent_deltas=[dict(parent=x,delta=raw_binding(git(repo,'diff','--name-only',x,commit))) for x in parents])
    elif kind=='dc0_residual_begin':
        spec,output=values
        identity=frame+'/'+DC0_MERGE+'/manual-result'
        payload=dict(id=identity,spec=json.loads(Path(spec).read_bytes()),map_before=file_binding(output))
    elif kind=='dc0_residual_end':
        receipt,output=values
        payload=dict(receipt=json.loads(Path(receipt).read_bytes()),map_after=file_binding(output))
    elif kind=='manual_F_begin':
        spec,output=values;value=json.loads(Path(spec).read_bytes())
        identity=frame+'/'+value['merge']+'/manual-result'
        payload=dict(spec=value,map_before=file_binding(output),id=identity)
    elif kind=='manual_F_gate':
        name,output,composition_status,native_status,game_status,eligibility=values
        payload=dict(path=name,map_before=file_binding(output),original_composition_status=int(composition_status),
            qualified_composition='REVIEWED_CORRECTED_F_WHOLE',native_owner_status=int(native_status),
            game_owner_status=int(game_status),eligibility=eligibility,classifier=classifier_binding())
    elif kind=='manual_F_association':
        name,output,disposition=values
        payload=dict(path=name,map_before=file_binding(output),disposition=disposition)
    elif kind=='manual_F_end':
        receipt,output=values
        payload=dict(receipt=json.loads(Path(receipt).read_bytes()),map_after=file_binding(output))
    elif kind == 'classification':
        trigger, delta, *status = values
        payload = dict(trigger=trigger, unapproved_delta=raw_binding(Path(delta).read_bytes()), classifier=classifier_binding())
        if status:payload['exit']=int(status[0])
    elif kind == 'sync_gate_passed':
        payload=dict(gate='f13_validate_sync_merge_resolution',classifier=classifier_binding())
    elif kind == 'native_rebind_gate':
        name,status,prior_owner,prior_source,current_owner,current_source,integration,outcome,binding=values
        payload=dict(path=name,status=int(status),gate='native_p3_duplicate_owner_qualified',
            prior_owner=prior_owner,prior_source=prior_source,current_owner=current_owner,
            current_source=current_source,integration=integration,outcome=outcome,
            binding=raw_binding(Path(binding).read_bytes()),classifier=classifier_binding(),
            context={key:os.environ.get(key,'') for key in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','VOICE_CI_PUSH_BEFORE_SHA','GITHUB_BASE_REF')})
    elif kind in ('row_before','path_gate_before','final_gate_before','sync_gate_before'):
        name, output, *incoming = values
        matches=[v for v in Path(output).read_text().splitlines() if v.split('\t')[0] == name]
        payload=dict(path=name,prior=raw_binding(matches[-1]) if matches else None,
            prior_rows=[raw_binding(v) for v in matches],incoming=raw_binding(incoming[0]) if incoming else None,
            map_before=file_binding(output),phase='before_qualification')
    elif kind=='reviewed_F_deletion_guard':
        name,output,target,stored,owner,source,result=values
        matches=[raw_binding(v) for v in Path(output).read_text().splitlines() if v.split('\t')[0]==name]
        payload=dict(path=name,prior_rows=matches,map_before=file_binding(output),target=target,
            stored_entry=raw_binding(stored),guarded_owner=owner,source_head=source,target_entry=raw_binding(result),
            gate='f13_guarded_entry_matches_final_target',original_status=1,phase='after_original_rejection_before_disposition')
    elif kind in ('row_after', 'row_remove'):
        name, value, reason = values
        payload=dict(path=name,row=raw_binding(value),reason=reason,phase='before_mutation')
    elif kind == 'row_committed':
        name, output = values
        matches=[v for v in Path(output).read_text().splitlines() if v.split('\t')[0]==name]
        require(matches,'committed_row_exists')
        payload=dict(path=name,row=raw_binding(matches[-1]),retained_rows=[raw_binding(v) for v in matches],map_after=file_binding(output))
    elif kind == 'disposition':
        name, reason, output = values
        payload=dict(path=name,reason=reason,map_before=file_binding(output),operation_reference=operation)
    elif kind == 'failure':
        category, output = values
        payload=dict(category=category,map_retained=file_binding(output) if output else None)
    elif kind == 'boundary_observation':
        output, boundary = values
        data=Path(output).read_bytes(); saved=p.with_suffix('.prefix-map')
        require(not saved.exists(),'duplicate_boundary_observation');saved.write_bytes(data)
        payload=dict(history_boundary=boundary,prefix=raw_binding(data),prefix_copy=str(saved),admission=False)
    elif kind == 'composition_observation':
        payload=json.loads(values[0]);require(payload['admission'] is False,'observation_not_authority')
    elif kind == 'scalar_seam_observation':
        payload=dict(status=int(values[0]),original_gate='single_best_base_required',admission=False)
    elif kind in ('span_link','range_side_link','J_prior_span_link'):
        require(previous is None and not frame and not operation,'separate_supplemental_trace')
        payload=json.loads(values[0] if kind=='span_link' else Path(values[0]).read_bytes())
        require(payload['admission'] is False,'span_not_admission')
    elif kind == 'history_132_capability':
        payload=json.loads(values[0]);require(payload['admission'] is False,'exact132_not_path_authority')
    elif kind in ('history_capability','history_3071_capability'):
        capability,receipt=values
        payload=dict(capability=raw_binding(Path(capability).read_bytes()),receipt=json.loads(receipt),admission=False)
    elif kind in ('composition_capability','composition_132_capability','composition_3071_capability','corrected_F_composition','range_3071_branch','J_prior_history_ready'):
        payload=json.loads(Path(values[0]).read_bytes() if kind in ('range_3071_branch','J_prior_history_ready') else values[0])
        require(payload['admission'] is False,'composition_not_production_authority')
    elif kind=='reviewed_F_deletion':
        receipt,output=values
        value=json.loads(Path(receipt).read_bytes())
        payload=dict(receipt=value,receipt_sha256=sha(canonical(value)),map_after=file_binding(output),admission=False)
    elif kind == 'boundary_3071_observation':
        output,boundary=values
        payload=dict(history_boundary=boundary,prefix=raw_binding(Path(output).read_bytes()),admission=False)
    elif kind == 'qualification_pending':
        name,output,incoming=values
        payload=dict(path=name,map_before=file_binding(output),after_gate_row=raw_binding(incoming),
            reason='complete_ledger_consumer_required_before_mutation',admission=False)
    elif kind in ('M_ledger_ready','range_I_ledger_ready','range_J_ledger_ready'):
        payload=dict(ledger=raw_binding(Path(values[0]).read_bytes()),admission=False)
    elif kind == 'complete_ledger_consumed':
        name,receipt,status=values
        payload=dict(path=name,receipt=json.loads(Path(receipt).read_bytes()),scope_exit=int(status),admission=False)
    elif kind in ('complete_ledger_conserved','range_I_ledger_conserved','range_J_ledger_conserved'):
        name,receipt=values
        payload=dict(path=name,receipt=json.loads(Path(receipt).read_bytes()),policy_result=None,admission=False)
    elif kind in ('exact_unit_supersession', 'exact_J_owner_permission', 'finite_J_owner_permission', 'r23_132_owner_permission'):
        name,receipt=values
        payload=dict(path=name,receipt=json.loads(Path(receipt).read_bytes()),admission=False)
    elif kind in ('master_entry','master_wrapper_result'):
        payload=json.loads(Path(values[0]).read_bytes())
    else: raise Rejected('unknown_event_kind')
    e=dict(schema=SCHEMA,sequence=sequence,previous=previous['digest'] if previous else '0'*64,
        kind=kind,frame=identity if kind=='frame_enter' else frame or None,
        operation=identity if kind in ('operation_attempt','operation_begin','manual_F_begin','dc0_residual_begin') else operation or None,payload=payload)
    e['digest']=sha(canonical(e))
    with p.open('a',encoding='utf-8',newline='\n') as stream: stream.write(json.dumps(e,sort_keys=True)+'\n')
    return identity



MERGE_132='132437c43f6f0625fae8954542f8d9c2f6aa3037'
SOURCE_132_UNIT_SHA='bfddab635adeb1f19429731301e4443013195ebc6c5c0fa034e04de87d2fb022'
SOURCE_132_UNIT_RAW=__import__('zlib').decompress(__import__('base64').b64decode('eNrsvdtyZDeSJfreXyHT8zAJxx3nrWxap63MuqvHSjrnPByN0RxwIJMSk2QFmVKljfW/z/LgJZlMRhA7yK2+DFUXMZmxfe/YC3BfC3C4/69/+O677/nT9cVHvj5tJx/75n3//v/67n/h1/iL/vfTa/zJ/LebP15dS99s7v8av6mfr/vVl0/oZz6wDRG/+r67apr3tuTRqJEvPOrwLZcSRy3W28TdU/fRl1qcb+xLKIVqysHWHML3W5P/9uXeF5+un7q3pydubuyI3TrTRsycrYRUY3NFcpVosnjjGglukiO3Uu3odUTjUjIx2mJz/frm15uub+V7yhZGqmUTanR4fsskoVI2FDmGXIfgV5n06u2131e+2j7k/3/7QlPysdRcjbDLLbTmqvSSY8MD+07cOPhR6fvbG9vIhFeEl9hbs2G4EKJz1gepVPBXMfU2TB56w/+5vWH7tNn08+uTzafz69OP/QTYfrjYnF5/xkMMPrvq209JP7vmL891/055XPfNFrxlD6rftI+LzfYtOeOrL9JGqClRH9wDJ/EizDazE1OLSA3y5dpLvm4fHmD7AN1Ugg3/7cvvv0DMGColZ+4UKVdnczE1luIixhvgdrX7FKn36noxfUSXpVAMdqRsuRb6/tbovz18jA9fXsr2V+/en15/+FSPf7/Y/DrOLn6/Om6n7z5/PPv+wSP9C//ax+lZf/g7uWi/9s3x5cXV9ftNx1UXH/FzP/p4+n7D1/1I6tW7qw8zV2w6nymSt5eeXpxfneDVXL+7/Lzv8n5+9WnTj+6sXLUP/SN/e8ur459++PGnP//ln959lG/+anS+/qTGPio+H/nX0/P3jz53ubm4vrg6/u3itPXjjXUn7eL8esPt+uQjn58Ofc5fri7O5y7Rr/Xtb969v3h4+VXbnF5eKw7Hv/d6dHXNm+tPl0d1A3D65t3HX66e//S7rd0dH8Vn3uOLHuONtV/x+q/6uRz92j8/euHPffwpkB5fc87XV0ebi7MzuLajS71289sW4uPzi3P9y5PLMz4/2MjVxadN61sbTz7Nph1XxrA5l2N1E8f6iy0Gv/BvfAvQVx94v7lsx3/CT/9jc3reTi/57EfcrG/+GUi3z+2s/7QFHFfvus/lr++P70b05Z2R41tn9RhpXDc2GAp64dlpPf50+nAk3vx8onPi/P3JBR7jjD+/EwC808j2y91cdvW3TywneJe9376cyStvbv7othMWMPyOx9mnazjYk3pxcX2FEX757tEQfHzB6bn0v7/7cA2Hc/up/3nvrTb8+9Mu05EpT3pMbzIF5yUliwAsIQxT4DARJV0MAWGmG2cQCuHb00Cw7BEhAH+XU24Rv7r3mA+D47exYzporR078AiOnnwTBeYLiRsgJtwLpYGAlwf7VvHYUQaPkSQP4lpDTibXWEnfSmmSvDVTsePGJd+54Hfb+XR0dcn4/49dTvlxHNn63I+nbXOhUxgfxhzffvr2j0+56K8/fnuLJz/+wBNefgArMUfj9O/q3vc5Knz42tG9k8VDvz9HhDltVzsukP7b8ft+3rdB7uv77LkCTusaA+DRBUcXv8PQHqelzuTqop3y2RdXgtAxTt8/4Umev+jJaPPMlWenV9f6dU8aP3f5jT89xeTenOPKexu//P7r1fHlpwpTH/B1X2pg7jH0mpNNb+q/Pi+45HYknFxsZPZWH/n0XD8078AskX9y2rbhbOcW8uiumWBGlmqlxCCYrwZOIkBkSBT8l13JPRVbvBiCE/G+hT7pwLgleMLSXaAOfxgLuVxcyzEx2SQh+jQiUVjfgUXcNz35KuoIxvqSPIiuMyELtVhLD8kKBZuap5KNN6HA23YIE84syVq2rvXqjZ1jv4y5fH3c/96bkogr5Rqnv2GeuqMbbnG0ZT2XFxiUj9zNN5feuDKWj6dXVyAnR3eD72aWX304vXxs4UXUe5Hb/U9C1f/xTz/96eTHn/71rz/8+FQseBmTfyKcfOxXV1uOOR+BwEMXxKtXDG/bj+vLVmaoXvn6A+CQqyOMMR1vXRQe+XgB6Xuko/JoS633GPny7fn6mvHWMJ6P1PviX4JB+x9JCa0Y0VfTWetThTdBt76gax/4+gsneX96NaPqvrr4jh/s+Mz9NPxyl+0IOP4yK09uZuWJzko4XunPEZOdNrcO6UQjygkmp3xqIDn3zvwAqzdOSN3RrZR70dUve4C7ePNiA88+xn30+2LwROqxwT82nDzxjt9t/enV384OMhlPHgwFna234+HqRWYdnfDV5/N2sl0dv5DXNQbfeKgpe3KDyYmCcnKOgPbCL/qUwenn04B/Z4ncyXv+2IHBluKd4EMLUdi6qAfm2J0g/uuSCXxhvfj74bb87bjbUsCTS3e4pbsRfE9lT74w2FWM7sdi6Yraf07xfPcNcc1tZDp5/iveX/SxX2+U4cx/dPp5tkH9DKLohNuZOrSLceBl07f8/WruUydQT9sFSlVPC6yfXH++VP9w3XmjA/rZS+ECHgfS+4GOSby1caMcF66RKD25+q0d3y7TQv1uDjbxYPb/crGXcUxaOPBBbua9/unQazFYPv5nWWf6Miz07b0H1d/yUf3hauHF2zd//MVBCrimqtkXmvniZ1/Jzs141w9P8sZl1l/Z8Kb/dtG21g4y1PhS9ZecyIZPDzRxcbGRU/iHi81Lrz/8hdy6py10V+d8efXh4vp1XvWlO6m82ZxixryWvZuVotMX29l+201XrqymXuvxdHK/kjHeXH8+Obu4unrB5bcE8nUe6E4C3U+gVzLzSo+3ufilt62N+7yPBYa2kbvx2dnJ9selV8INnF69+NKl3/6Bbv82NryKsUMixANDt9Oh8eb8IHgfmLonv/8R93f2fugheXqeuO+8ZtcTvaUJLE8ToJCdeXJvafjWhkmUpYZqA5kcyKdsBhdf+3DO2grJaoen7rox3qTCjrzNjmPrcXKbzWXLPSbbjItRzKBEjcj7aqqPpdmCG/oeePVtNnLW+ad3HGn0YHsZNpJYskV8lxxdrs613ootzg0/6rC1jFBi78wpdNGHz7jS/AFJZv8Vd7pefStrrZ0mpoe7Q/Jpw/X0DFF35xbT2zbR2zbR/4HbREqCfufPdw7kZvn/wbK94oWxcLMkNrGU+F9/xfP/9HShN4b3YoYXwo7kIa6uN2tTMmwrWe87fuqxVonkSimhg+FlZ4ued3Bj2FLJSShEbpiSZJLggVd5l5p3Iw4TbQBLyyX44O3IAoI3IrMzLq2fR5XSrlMEuY6UbGo5JS+tdHxbyr37FKjkUZMdplHwlUscIxqQvWiEXPIgxTWk+pZH9ZZH9V82j2oxu33Lwnqj12/0+r9gFtZr8ve3jK63jK63jK63jK63jK63jK63jK63jK63jK63jK63jK63jK63jK63jK63jK63jK63jK63jK63/b5DMroi7UhjinYIlUzWON+MbcSlCgWntROKd8WZGGqUJiNwTW1kT324PkyozUcf/oDKL8srju3L6LIuJft0chtVQ6Zz4lhyM5lbbs2PFpsdIRrm0Ur0lhulTCZGTia6EJtLricb7VtG13+5jK63TaO3TaO3WmxveVRveVRvvOrJPCpr3dNcwpIfoToyiSXbWNgS1WwDBy5BUs8F9GHISNEXG0BknIB5Nc8cE9v6B+RRLa9ltYdW5ZDz01lUpbBvoJCjgWHiP2aAy4Uw8Kg9g0DGJKXkxCH1UmMK7BI7zh0/E1jebD29F6SmvHg3e0kBxqdZeK1ZrKa0pZKlcR2hZXHWcopsc6yxDWsFTNeBlbecJdUyegePHsP6VP6ItLvFZzL2DBfno7N5RyU3731OBg/lOyRGtSV3PJ13NbPghTgKld3A4OlsxBQT/fAVzDw0ipX+46fdzWSa/cFpYv+h62e9FRV6S0F5S0F5S0F5S0H5D5CC8pac8Zac8Zac8e+SnPGWVvGWVvGWVvGWVvGWVvGWVvGWVrF6WsVrpzEs2K03uYQdi2NdEldf2bcxWjDVhlQ4hB4j4c+xEknaLjJ3LrZIy1RC8V6G79m5R+uE973BPpzqO/p8Ui8+nQtvPi8+Z/v9r6fnolf933/+y59/+uGEIPP++z//8Ke/nPzLD3/9px9OfvzX/+ev//2Hk7/+8P/++Yf/7+aKu45yy1Ylv99udCBawjOp6P7SQW3hevb8gub9W9KFxB1t02438W7f45eHohCyrv8b14x3eH/cS5FqgEvFPQubGEZx9J00w006BmfqoXly2sBMRqKRo5CtY3hDqba7p/cJL6X40brr+F7Jdx+zcHEpN5ur9y2z3um7WRi/m33UuydIllPjUruxYpiD86Nb7Z4jnMk5KRRcwL2+i900yQZfyMOKrdkM/LUxnEPTjQBvTcK3qXeGK5NnTfqotnPKeKpEMeCxWbM+KHJvzvdivpt+B7OPevcEhuLA72CGggSfi/HkjYmZaXhpyZE1jGdcjFrLwTTbPUZjtaRNCJPDAzYR6XnwiIJHjdF9N/0OZh/1vm+gSV1acFqvKTu8Bucw2j2l0SxbQ3AUVtH4buEx++kMn+9mn+DOsFCvxTnCe9XOfb42LbPUOjyhrdWUKpm4uvrd9MtdmIz0PfyJCx6YyiitOtvheOFigowOxyweb6QQ/jX9BPfDoXIW+JzojBHnmq/E0bZA4tgKUI9OguP23ewT3BmeTZmaf2ezj3rvXcPoPhkM3VKcbQ2TIwPYMNIwQNmxpAG3z99Nw7u0Q+asf/9u9lG/BILbnY8n2mRCV3zcdkY9oEwFwhk0ztXXnGDpTtXDi2/7k27TCjbb6Pz9/d/928u3tA661a70t0fG2nZn/vrRrx9yJDKUH9QweUSUQB3SIHCHHEO1wqbVloKzXIxxOWAs1UpRbALJGD0RptZIxVoZBCdE4fsHhv/t4V3uiU49u6jff/U3ugO25TNwv95//XcXp9uLsgSbyfRqWx/WesPqZ7KLadRgOMXYS0fA+PriR2/2xv53+gDfzRr8+XrXm//5k64IP43WfSLiUnSsCcnvBKdRQVTTJ9VOrQhfpmqPX4Tm6EOBuwGBda6rI2DnovEFn5IC70Qu1OpXAsf2SCmk4DsLnGxHKJKEwQMW6ZoUeErBIDFlGpxZgz9f373ofWBMZYAunkbgLLuBSlAPfYh1oTf4UQN3WhzEBwYVfC6YhQXHkgbXK0ylDtdB34OtHU4P3KOsBFSKpiUShPHsk825arYyPl9KrzVWF0poJtc0DdSswZ+vZ0B4HsRn03KXwpgQ4naiKFGDOqgpomXX9JFi4euCSO6uwPmBGwct41khGjE6a+JWijbAdnFo0F8JxYrInqSx+OgdXK+1bGzsDTG+EChiybX5FPI0irMG71B8DoVDcNyXLH1IvJzOql7soSPtcdCWM/ySigcbRTyGgYNv7ibYBoVos4DDUzHZEjjZKAOuXCBBhCs8eV8resaYiUD4eo4G1Cwmg/g9CCLMp9hC7d13Z4ObHjGzBr8dMbtweGbMfJP3sxQ2bcfrduI2QH+HsAeBqeDO1lfnUo0jRuvhxyFIWwYv0nlRwYlrLaW1jAkCr8DFutUCqwdhh3tJJSmfxtNmqIiWbGADKlygM0YbY0FgnTO4xe3xO38WogcHEpbCE2wJdic85Iv3I3dxNLg5JzSSgK4RGCm4UM66DgJSMJoZxUVM0SQFnM4F6yFG1gqnPWdIyzBAgaVT66lXR3XEkBkKkGo2EqrHV5iFZ9bgLTxf3vez0Ow6ELKY9mjt6p04Gc8upDYwd4AB5KNPVHV9EgwUsjCk0WmA4piRm+luWwbQx56GAxfyLq6Ek/FEmhYaBE4Ag5864ekgjzGaPJkRWxEwmXl+OmvwFqcdL/9Z0CZyGRfznRDMbvHX8OCjmuwwx1xPJhQHPV9jTrUk/c41yzAu+WZdzSWMorQ21RAtmJ9bS1/o6THPNnUITA6Sc3A1gDvjDyWDY47uCb+SafxmDd7i9zwOC6F8nGO6FEWdV7u9JVirCVKHIDwP8WmbAU1gIBZjlbhD0SNQp2GCMbZEN0ay3tUSqrrSwWuhmAJXygXaYGCawDWb4huYUsnFpNBcgvY2ZT6YzRp8EsVHECwE8JuTcov9KDTSHh7pgh6lTNQdwl0xprpBAVEBKiuIBcVkYddGKYSAjclakwx81vusS/JxrKYfKw8xlfEIuTLCk4X4K+BObAZorEkmp17aAv04Z/BJCB+DsBDDb1K3F89CsKbdvjSCHYNh+WRAljlbU3MISl4c5hp4P+tidDKEEAKFVbWxOD5Qo/LJWMtaEBYD3qeiXfddEIxrg2IteI4KtZ686xnktvU+DeGswSchfIzBsxAuO6uyXCTksic4lja8rrrBR9bKXkO82KYrbs5hFMfUmGChYux2sQaMlZwvAl2HENrXmpPJWsMj6wQqHPCu9bBUQnwOUGBwG6z8qyxY05k0eAvoIkSm8V10XGKxhHc57Q6fPRvoo0QOfG4oS5U8vKNSoI58rqPUOlhaaKFrxIR2igg/sSX8NshYi8Ra27MVaxHZIwR2jqVkuAznozG9RoLwjjnbeQ0/a/ARzkuQmYZ75mDLIatAs8fQF4dvl7+qC/FoCCFKw8en7HuJsVtqcJEkWQSKKHpm36DlCkOpIn6T4WbtILaYaZU6JPlKQ4jhjjqUWLPQx1nYgFKYpHuEabQuKVXVYY6nh9CswZ+vJ4HYN2AOqQ+weBnCBR92h3QABw6Wh/NwCF58Ls5Y+MVaeuYkDry6NQQ7ZxsiIKhYscH3oHvECf50rdU9vGZD1rYG0ijFYh7noNt2IxexUGe+D+vcvLydNbgP1p2Y7IP4+RoOy3k2WPTuFf4IppKa9cAQCqk304L33lSB6C0UewwudRBQb+D647CUqWm7hlZ95pDXkkrg+FEXflIWlxCFsi8El4ybqz5nO1TnRDe/2zlr8OfrZyGYxG9PWY3lK+7Z7YYQdNPaqFueZPNAaHbV5k5wq1VMY5Ds5o0ly61WqqY3G421xYKzdi/drEXLTLbJFzh/Yh6u+558jIl79KNnVx3FQTTmJ+Wswa8g3I3CJIy7ip0cEoyfq4ryEpvPlE9Zvrsb3W5px3CGJKYbryuX6hRcZp1RsRgqJrnWJFLOINGhp6KZLJh2xkmiElNcazkaI8OT6XBoo7VOvhQ8CEaNp+5TSohemaHG5sP7rMEvQ24/DDNjbqaqzVI0vUl5J5g5JDKaR1sSDYCllNjDvRuna4HRcAySiguaPmU00bMYbemTvc2mwp+sxdVSYVXUqSYL5wYFFptuT5UKdaZ5HnjuzBzmudqkwa/BfAaKGUT3FRxavORS4m7hFgTSGxyaGLTbtQDOzc2xlNhziNFC1XnXW1W+3TFD63b7HqEgNOi5tta0ZG33VBHESqogh6GRevERuqbWBTOcSS3nPL/uOWvwC5J7IHgJggezMk9pT9YFcCLw8AhBBKpSbUo5hNF7w+TzumLN4NqjpQz9mgSKFZy8+iQI50NtrJSBFpvUASbRPEgF+DDUvyZf2YgBxpVj7lF6oPkMtEmDz8O4hJotr811QGZU3L3TDortRhhekyNKH1qPxEZMxdgoQ1g4sHBdfDFDIjt4XETOmhOL1e65Qdaiaz7gFiEItJvogAOrwmBzdpsfXJvK92Slz6+izRr8Ft1pZF4H7qeqqC1m6PRVUZrHmxmWNOHAObbZiu9UEvi4EQp6usAOrrY2RNNQSnJBsywYSjMV3zFIuK2VFAOt31ztLcKD1pA9xJ5mPLek8T3qY+RmyM3vKs4aXAL5E+jshX3NmnfLHYFNu3MFVI9JjxxGFPh314kyN0mOq4QBksnddh9BNl0UwYwpJpqGkdYNJtVqKTdhOMxUCcnh35lisHXUVKumZxo9MANSDwI1n9MxaxCjYkXwZgfNXJmg5f4BL2TPgrtQ6jr1Pd5JRFggktoxfYrz8KDK6vARFt2dj2yssY7LIAsP6ga3tQJ+ztI7Hsh4yGxXQ8+YvwExOZvCxWYNStXPy6lZg1+PhClEFsF7V8hpMYo+7Nk2gciFJ4d7B5RB17hTUmdXSrFsWYb3gJaHjMouyCBQPGOCZtF6FpPW2u8UZ2sBnWheD7SIFH22YILTfCMu3lqXqJX5dZhZg0+gePviZ8E6qKTeYicdwLR3J3T1AQCNbY567zW5bEcEf8m1aE5atfi6XtMtYkCQQ9zOERyuOYm22Rjcaol3vTaXquBNJ5AEx0xghwxe0akywd+AVSa7IPFuzuDXoB4C0Cz4LyqndtBK2uvVXVvh9k8VaPsDbvMHfaWHJd/+mDu93hd7QRG55YejdnuqDJVRUwGPjIhEPVYTc/VQkyFpaVSRbAv4pKma1+Ccnv4lOCpIb2rQZGvtzVVriVJGRIwu67KpDQxio+e4LI8+EPN7dXXeU80a/NpTvQClaYf1wtJ/y9kI7RwMJQFtk6EpwSaDMJkg0ejZ5ZKNH8NqKYjSpY1sMGxKikIjDjc6u0ZhrUUG57JUCMKMuJmGHhGKgphabHQOAqeD/Q4xMn9wZ9bggsHwPFKvMSB2F21c0x/tqu646j33loH8w+/8+t94SWHJxevEdvdCIoN5xlIimGYJQzQBtnC1oslpI4dokqmciq6r5jFUhZseHcdYBVOi17WynHvTg/+evesulto9lTLgkQuUHjRt9Fp8hucXEmcN7pzjCwA6YGrPVwJdnIuzJz+6BENg5RVvQ5Ke0PJR11kHoPfGVsYL0TPpNeniknRDQ7Izw0PshMa01qYdRBL7Zlqh7FPtuY6YtK7IKMlD+hcJufuW5pcMZg3uBH8anhdDv7tw62JJuu+EJfduMcXB6GIKYEAhJNHD055cAPa9BzaYEEMaHkosh9yMBc1rqUgnWkuRUogpU0o16PzEMxXDkrUBlKSWBmarsK1hfplh1uAk8jvReTHuU2V2XzHeLK7H+9J7P9vTZfmSi9sjZIoexA7eOd9yzy5R7J6da5XK4BR75kCpOSnwfMKZPWtS16hJi3VwWou7Vs6m2i45x5q0HpiLgb2eIrXdFQBhoEOsmY9rswa/HuDPgTE7nPc33FleRcDtzhotppCJUGdBgskm0fDVVgrZWxELeTpspC4Dkq5xJzCUZLPvKRszrDNr5RjmgiciRE9Jelw9CFRDg0ZyjRtBERnfII0WADpr8FtAdwLxcjQPXheNeU8QyjlzjsayM0lXPVmlZQOaUetv6UJELC35FrnBDIGdjYihPrRqmqlk10oEDk7PY2qKT60Sa6kpR6YA9jNajj5J0lpy8ykKswZnIV26CDrbp+qlLn5/bfYDKizF3ceI4O6C55q9SxJNFCq2tAAP3suwLFRxeTBKXUvv1EwduUHOjIBh1vxq+yQ1d8OVNJiE6I0UPCbRaGQdeLRWOyhx1DE/dmYNfj129iIxO2yeqpq/WHo62o2hhy/LhvQ4WLWjCXSIpK5FIxjAbSsIgqplamCm2pWqGEx/rZpnJcXsVss0CxpFi2ld92GINO2gEGcboY8S6UETZ3yad+mzBndg+OX1H4rcwe7b056tymjwlYome+Zore9SKZEzBZ46k/QcoBiAsPTCrTQJrIXP2IwyPBSFWW150GTLCJ3VhpKa5NiKqdB7lDP8BPwvoivzgoSUWYP74VvqtJ9rLHGAOw17lnoRa1ODUBLhAVHoCyJu4dpcNl1a0pM3xWGOUnZQh2CXLYmjmGrUQ/KrJRfBzTcQu9qybkekGEmnvma2+OyyjOFckQXp/7MGd2D5NBavAenhDKuksCcnBEq+hDyMSd77DmxBkrtjAM2+VqiH5AoV9tTgoiABcWVObYzYrJ6MWamSSCkpdcTsrmcnIwuGVTCjCjQabUvytY5htqCSyKTBeVgPnq6/HxQnraY17S7cw0DMeoevVzgP2/XM+4BXjYMKIiGP6LpLTL4jhEaTSm1pu+hRra3Orna+KtVmu6oVF3tx1ZUQRgURI4wjwOHA3q1tC85XzRncAePvV4cB9nR/nOUO1u+BMDNX0xNcK1mtEiexdLgdOGWnFTNrdhIs0MsYqFqFI8AY4uooDCXrw1oQ+lghOGukFKkKnqSYnEwKNXpnq1AMWs5zzB+HnjW4E8InwTgE1F3Ni16scg7ocvTSey5ph7T8EDZiwm5ikK3XUpRi9dCAaJ1/zEq4UzElEAhtMjqWQZF67DZ4K01TfSzlVAq4/GrEAGKugj1D6ZVRU7/pxGkcYtpwPsIHpkHGLyAGcwa/HrcLcJkdwgvaVi0OMqXsOQGajOsUtF8uomZk54NhOKuhhaRJvzylDlZAiC3BNVBC+KjWR65UBIFprQW2GjW/HaLXilH5G7TOfvI1QP9GjfEIGZR5/hDvrME5pL+F5ZWBPnheW0C1O288YtJaoyQwgeiNphUUML5ViHsayhtMsNoLo8TkGuZ8hbwbEK29mGTFr1ZXZ7TUmi52DriQ2FvXRhwpgeuI9dpRwCZjwoK6OnMGD0L7leb2153gFmt0KJfdOI+Y8SXxAJ2Kh6YDgYyc4Adq5jFAm4d0BOaMf/TNcN92d6lx1EJ6lnGtNRZdA8m1tQw+CzfDBnElVISUqp0wimhXi2zmRfqswSmcvwLkVcC9a9W3ODgn8nZPcNYkPLaxOyLK0ZkModeKEEJxrK6NaGLOCMNJ+/bAY0MstNFNjTEyr7WANgo1A/6uC0C9b4t2BESIWjyGWwuheo0lMp+tN2twAbi3gBwI7p5eiq9M9XbcaS1muaPH4/L6pnn3QTawEHEdGiiAZW0L+YzQtG6kdS00K9SSTmLRDimcTEc49ioAifChzrLWEYc4WqEyRho5VzZavquHAscYM0Rab9rfJomZP+IwaXDvqH0akAOH7bNNONcYTXu7df6hN9zd1vOPfYx/ryf4ulHoenf8tqPo8jrWZnfU81o0qCKMMcRHTZSLHng04n00mp9mW7Q2gu10uA0Pjdpiq0aPkGpLutVy1MWmZouero6DjVaNRUTu1BtiOFMF3WbQ6gVbf7MGn/cfO2B5uRd51Ph1sURJfk8hiqybnpBiwtWEBLqiLaA67u0c+LzRo1ShS7eOuwS2UsUXl7WsebcIE3W1Hjrqy03g4Yhr09PVI0YB0aYk29ogPQ9EsAU9dOYMTuP8FSivCvLBYtRRdLsXmXwkrVGrHR/ioFCNrwUyM1Zv9DyGlomJQLxrhk+AVh8Ns8H40SPYb3JrdQgxNnjNBSObrPaC9AYkxIc+LJhLqX34QI3aPI+dNXgA0i+Uoof0UV4vgDzTcHnx4LNut0CmkbutJg/DgwREjWRkq6JZa93o4aDUe8mQG9rVsKQAYelJ6zAMMd6vlQlbIkPdeV9uinPbhCchO6ILCWKdtKRqMJ3m91ZmDc6Ovf0YvXwY7miSvTyncE/xBB8GEB8iTrsXlhwiDd9KzEa3vSO8D+MF1lG1mrY3+nHr9MwYF9MhQdfKgjahW6MH+LUdUouhpdZTGDX37F2VQY27HQvy3ycNLsD+CWxeBfOJhuaLyWQwu8VoEx5VjCLePJvibTRB+YUVC8pompHht30nmZumtrAwtd6aFBfqajlI0MA2O/ACPQ7fY+xDBqIEoiKl6HTRw43Mbj70zBpcMASeh+pVRsTOXvSLMyWgAXaPg+R8p+zBKfWASItVT+GLi6VjMIBzm1QrdATcvk2IBvAb8BngKBkzyqxWlFYot6EZGsbamhvnRGlAE/ioBxIrJIKxMup8GJg1uGAc7ALoFdDnzfXnk7OLq4MWoUrcXXsyaYFTY0ZzkTh03xEQTdX6rsZ4cdVigiRLop0TtYW5douATxihQGdStqtVZCg+gO0aqRGKIFA224V5k72U3NKg2mtYkHo8a3Aa7oeIvBbAt0epXmWOp/1F03osNZkIpRFbs12rIAXd50Q0H+SG9rvSNjZ+aHfrODC1ISVbxhutq511hBxofkiDpMk9SOMhWnlw204G1KRF9gjQbT4HY9bgMtB3o/QK4+CunsL9YtVBgd6UPTthSTf8cgR3r77hNXgB9c9ejwQ2562ztVXQP6oeAsANM0jd4cAlPLxZLYmKR+89NQlZbM/NJc/DesZzADVIYU4evigsqCY/aXAa/aewWQPy19F60djdCw16ksb5YjiTppZj9mOC6Gka6lJ68gmCCL7AamnrTK1WjJA4wkgF3tLZ1UoOt1IrdV9NkQZBFgWag0dklhwL3FGrRkA+5ksOTxo8fAys5Ac2F7/0trWpldkuNqfXnw/zBH537AfRtUqBSHeYWAubNC/Nm8LBFoIq0hcUW2cN9aDEejCBKWszZQMNsJYjECkmSQ+ZhJIrsekDpjScDdZpwTvHjgbNHzuYNTg9CJ4G51DUt4lzjc/OTrY/HjTZU9izzxhIhEHdgqWOmZ6qLrQydYKDT6I9hhAvSmstgMR77QOmnRUjOGFCiFhP2g2XMxmnC9oGkqyHtE2xcrV5Dw/NPjDUxzzHmzW4H+dv4XgRsBsMmavDkQ0Yn7uTaauTQaVxH01PD4XsegbXMfBvvnNKVLhwVDdHID8I5lxr56gl5j3Z1ZqCeRu0sUkz2kigihkDPzZfi1TpWkkmcQpuwXrxrMEJZL/G47WgPXxNhkCz9+z8FBA1iGwDCluLHirKlqHPbCLSoo/BhhEDjZicWG0N48vA/6wY10ddS4z3JEGgCjoezEoLNYJNWXYcE4gCafU+Q7zgYNiswWX4vjQcPyhR922WwGqr/0/eddVcgQd3vF3CaLw5fxn9JOf2nJhz3TVte5/gjw24B0dG9JGI/6aqoZpGcV26abEhQlvRc2cCXhKL087ga+VrZa8pvwnRwqVmPFwnZhlEr8/WgQxJt0XLo83na00anBjX+/F5hSF+f0L2sOPte1Lnq7Hw3px9HIwJXmMXgeYQabk3LapiigsFYjNyyNoUKtWYQT+7F0hSs1aeU6Mcoq9W60PYkbixc8ZlyXA7BVG3x21a94LSSpMGp9F+CMkyhDVJbvuUv/XN51d2VVvbt912TnDtqyfhHVpQ2Jm855A1lUE9w69oAQK7LYKEgRgqyBM0gbaGSA6kuAjMQCRacCrRqWpD3ebfrbXskZMZZHrx3RmMlszJh4QIr4VwTdRNlzrSWNDOY9LgU6Pw4yH5MA+TwV9Q5sCXsJsImUhabM3FBOlS2Yly2ubHto+c1Y5qcKdNE56ykLMRXnd065PLrhSmtVYse4WqytaO4P2AeHa5DcmFSrA1Qls760IeNL9mNWvwKfCexuHFWB5OAfY27gCGrrRIvRIiem/Fei1TUQOEp9W2SKF5q82wHMGlhoQ46TVpW6iJtbJWybXgyQ+yxlUHbVWjF/YYVQlxqaeeg6Yl4RXNn7uYNTiJ6GzAH5stPHJ8dlqPP51iZoOzfuRft91Rt/xVl6zO329PGJ7x53cCf758ZwljdU9bs0AQa07LJ1oaBjEx1xRr9lYP1GCCpkDGZj1AzwLeh7dgaosRjIhSXSuZqRn2eMTh4BX9GBVDKtltO41YkksBT2RGavMIzxq8QfggYKbR3iZH35i5+tsnlpPbE3jbUXMQxC7tO1UT49BDrAOzsg3fNHcIzI0xhX31YHADzjkbq9UvWOt7jFa1z1npWnSy2NUaXmoWmaEYBWLCmkrR1N5sHlrkjsaA0vTwpQt6sUwafITxs3AcAOzNWHk0Sl4AsM97FpigvhKiUMwdsRVxFSqMtNdbCo0C6LqVMqKENqhybJBneA1Q6iMRuRzDWl7ac9dN62FGH0nESnWmgf010dboLK2UPKqfn8OzBnfi+wwq0zj/3uvxOPt0rTuM9eLi+up6w5fvDu1m+djyTa37D9cfD6iPbYvfo+etNrDMFuMCci5zQWhnDI2Uhgzbo22cchk+CjyF771W/KloQ2NIJVqr5Uqo1UOMkZbhDhl8EWzDjQgaWRsx1GWllP2CQ/2zBh8Nk69f/Tdj4R8effHtE2y/dNcCNJhLcD/ZpMGpO2ETWzeIONaqC/LuLv3u9vo7/ADpx4+n1zfrdcnHUrXlJINRhtZclV5ybMYO3wlKNcBbf1lp/x5DYXPavx5137/j9/j9sXZk1wZSV7cZskeX7uimr9TRtpPc5QXE7KEd1r+5x5YDHd2vhB3didqjL0tih97q/en1h0/1+PeLza9aY0E7yb77fMDUIEQ2Q3v6lY0q4voY2oqsjl671b01RA2SlqquCo2ECVHxZ7CIbJoLASyojSzVr7X8gWCcJIakkRtPJ6JnKH1gg8lrCrOHQ+wYHPPZFpMGf77e9eb3ecl/4V+7dhQ4pMvDnh7shh1VMc4VaOZWjOvZuR4gTHzWQzJWawOBwzoeo8ek+c7gMApo1xI7a3muMUr0tpqmnQQQbRmsqQTX9Mic5mT10dlUM78RPmvw5+u7F70PDLmATNkc3TaqebdVK0c3M3WrVg6bRp7c7hCTMpWuaccBTq9hJEUtJQnW0QucGASW9SCZ8MLKNEsdTqs4QzV37Zbc0lqbmCmalkhs5OyTzbk6KVompBTEuFjhnAN4RJ1fRJw1+PP1DAjPg3if0nDXdujopoZLP5J69e7qwwGJ5sXtKV7bbJGgWeTFaXkcYcQ4hoRImFuNC0ExFHJkIoZpoGHBTokjFBRiLK3lC53mieSs7bRHD3bL7Lu31K3zI2GWCGQQ5Mt8qbxJg3coPofCITje1QA6+lLt+uCOnMHk3Xkkw9aR4AuzF8zEZEvTdMEm1gSDGcgFYim0YrXkhC7bpVbwUpzyGYzmulYJjepdr9bk2N3Q48et+qI75VUL+OjRo6G9m92CEqSTBneDugeSJQj38yvtmn1n9Qq06yMfMldttHHPiQB8xwjc4HxsdrYGJxD3Rndqy8i81UojdgGjwfR126Oomhnss+U41pqrdZjKnRNrkQtjrVhfgQsosWSbwWoRz6BYF7RAmjT4Lay7cHgGy6vjf/zTT386+fGnf/3rDz8+wV4ncvtC2O1jB5j8ENZm7CAvRr+NSzVCplhtbUG2Iurr8pSB/41Sa9HquS2WUPRg8FoHuSwmf25VW/cma5w+bTZGy84oRdTd3RFHW7DRMWtwi9vjd/4sRD/98ONPf/7LPx0CD8DZk3WHWZKcwdNl7WVZm4YHVhqj1Ser1SpbEMVKQkfM2y0Po2mLGkMcxZpX60cOeV4iY9RE5wJ0imSJ+CKWPQXNA7cW7HFJP/I5g7fwfHnfz0IDqqrnt68errwcghM5uO09SfLQKi02hSk5hhdo8H6aBEtNa7rl1HUhu+E3prTkNJm8VsyhGEzMg1c7CCOx6gFgS7Vrkj5+gSDKjJHPpmQ9AR6GtQsOwswZvMVpx8t/FrSPp21zoY2owVIfdJc6uv3VIfilEMzueQboytA8wAb11pMJUG5QQDGnWsC9CGFbhgENw7yC+A5De4r7VEO04NxuLVbSoMM829QxMTiIHrKoWvOFtBsx2D1oIuFX80vTswZv8Xseh4VQahnIl6AYfaI9tbVH0rrKQ6s+DPEIW1aIgm0WQ5S4twrRZxJYiTG2RDe2hzsqBnDGlF1tFrZ0c3oYqmxolVLrTfHNkdXmtSk0l0D8TZkPZrMGn0TxEQQLAbyRjC9AUNXpnu4W8P0R3pK64+aKMdUNCiMP6NvtcTLDwpp0XAgBG5O1au81x95nKYgdYzXlXnkIqB8eIVdOoPZ6uhncCcw2Za3DABdf2gLlPmfwSQgfg7AQw1vx/4JZCNa025dGaB4wLJ9MBCfJ1tQcAqtjxVwbA2xLCzEbRJLQpVTjTcAH6rbQTKyr5RkXxG+nyyVaG4TAe1sJoeA5atXu765nkNvW+4IiiXMGn4TwMQbPQqjKT7d9mY6+NMk8kk8brqdnp9efj3QV/EiXwQ8TCbnsCY5Fu01AxmbNCWcvKRqxLSBUbgs6xNT0YECpKvOg5P3I5HwR7rohtta+XkpaG2xoT91aOOBdZ91sRHwODiSEAms7xrJgNW3S4C2gixCZxldbzNy0zrw6+q1vdBuji64FiL6Ss5fBbF3ek2jbs4E+SgTlroUtoDvy0FYkBfrD5zoK5AZLCy10jZjQThHhJ2pRdQky4lpa0Gp5emsR2aMeM4+g03AZzkdjeo3kE/xMtvMbc7MGH+G8BJlpuL9wqgdj6Kbr7dG2g92B+1WXm4vri1s3c7yxWnri/HrD7frkI5+fDl1G+uXq4vyAdfe4b2+Xhw9MvmpGbs6uWJMxYAolCFiGt4DDHLqXlYc2VxDqhuDzt/2rBqjaar4/NIxWV8glFhCF6jnxaEVSUr3S9YxfawtcxazBn68ngdg3YHab2KYUfPObg46W+LKn7nHQE0KxsI0JgQ7K3IJsRkQ4QxrlmQpXVw1E/ICCGBrZraWYnCEvUdZahsBrj3ZY8WxoZNtAHKG+LfhlMohEGbGZQRTnPcOswX2w7sRkb1pH25xeXuvW5vHlB77q5mic/l2l88GL8AT+SLvXayOYSmpWZyoUUm+mBe+9qQLRq7nO2n2ta6qsgeuPQw+BNUiK1KrWEcyr5WIJhpWmHOueB6JQ9oXgknFz1edsh+ocbbEy3xxjzuDP189CMInftaOj24T3Iznl9+cX8PTt6hAMbcx7Ss+DblobW8b3s3lUrc1hc9d65FVMY5Ds5o0ly61WqqY3G/WEtgVn7V76Wp2kIFxs8qVbIubhupaJjzFxj37oiTxHccDTz7epmTX4FYS7UZiE8fde7w3UzcXvoOrvPn6bWvUshtmbPS1qIGkDJZCyFq1uuXMGk6q6Jl0ROJ0kj19VqR6fctpCeJQG+cvSM5zSaqlQnYwdLvqK8GxKqWlIDdu60dudSDyYpR7npdKswa8g3IHAcvzebSfwIeBpE489O5nDuKan3402hEaYgKIPJsClBMrdWehg1ta5knTlqXEm1hRn0yRbW9cCj13Q9CHtGLOl5QLv1quEaoyWbTOlsa8y5pt8zBrcBd79659BTvpvx++1xaZuZ3/thw+Kgj66tCfpHPCI6QbiVkcgVGtmDRCxGComudYkUs6ap9wTRHAAfAloJ92rj6sVtiDCnOiIz6O1TmBmeBA4QU/dp5QiIjL3XOcTDGYNfkFwPwyzQG769oDaIwM3yXiHoOlN2l24IIcEJ+MdGBtpe0VVeB5sxThd2o6GY5BUXNDERqg9ycUAXJe95pgjPK41HVPZVqNNFU4+azp0bLrbWrRZSAys7SIy83z28azBr8F8BooZRDGdVageb/M2oXuv+rkc/do/H4JkLHl3VES0aJZZ1+91BQnf11FwxSEEIlIiYBRPUQyDnGbP3YPFeZA5LVaAL1FXO7+VnfHDp+Yb3jKUrlY4rcqKvRlUoYJscz0uKTM2Z/ALknsgeAmCB4sMb/c0G4agd3CpIZocqeUWoBAT5CLQrdqOm7ZpTR7asZkeBbQnZ0TRWIe26OO19tQ0r4H0GEZwg3NjrWRjJWm90pwqRG1MRZPS573rpMHnYVyiNO5saBfGo83F2dnFp+ujS7W1+W2bLHR8fnG+bdG4XZI6LHbGPVVgK5R9slarYBqbKKXiPbvhI9AlUITsEUnxElqIQxIck2aOUHN6PE9Wi53wfbZEr2dMKnSdNC5SVUFUaqWwdlqzCtWCwzxzBr9FdxqZ14H7Jqt+a/Pg+YwIsqfguGta34uiTdabIM041pQh1pWGWrSpAKY0PELHuCkNKCdT7XbRGDI0rpYrFIYY25P0DjSYvHdDi0xTaHqEMtYKyatTcx7ySYNLIH8CndnDtlo26lh/sV1Z+oV/49tlp68+oI16jv+En/7HXe+bH3Hzvvnnu7IAP22XGnH1ckcQ3J5RAU2qDcFMH9p6nHIB4w5kHai1aVW8Vo50vjfSrlJZ288zftvFglS79RpaRx+A2bbWTfKqqinYkJ0Uh7FrQPG9HjIM841pZg1+fTb3tcGbHTTtA18/6OJ0enXfEun4Bcfurb6QPftHQqkn3fwMwUatnE1Se48B7wj+olQu+AiLJptE7d5gHZdBCJYW02ktQZxzxlTGAxmvVZBq6Dk3E6L0bAoXmxljsfoFrScmDX49EqYQWQTvoaUvrA97dgEhcsd2hgJKzFbQ8GQIw7wUy5ah80H6Bg+4QnZBBpELxoTRE3sWk9bawkFkqYXMaF7rcYgUfbZggtPS3JACykQ0Is+nsk0afALFhZUvtA7P7/z5LrH7RA/MnHzZ7jtRbcab65te2gcXUNDmx7tPLvUBAA30hDaAqRBSdkTQs1xLyDyq9duKH0WLZ0lkreIdvGtOom02BreapOq1uVQFbzqJeMdMiQInS9oPl+BvugXLXCCp5gx+DeohAM2Cf7/B+7hqzwP7N/u929Lw7zcsr9cBeefNb4pFbL/n5eZCPrW+Obk/ZrDm7W820HUr/faQ6x9zmz/oK90l+/9xd3q9L3Z/yOTLnU+kHusot+HkiQHzbpufcPW3s0NOWe5exnOWaioBshmRqMdqYoagDF1LzrITgRyz3Zuat208jW7+wFF5ttSYV+tTVq0l3aiBk9XuNl7PDYDYUNFWS6MPxPxeXV1wkGTS4Nee6gUoTTusfbeID92iLlvc+q6rFwwGv6ePuopvaO1swCYD+DVkZzSg2b1kcPFhE1eteNlGhsSsJemB1Tjc0DP3Su5XOgHoslQXW0bcTGNwliiIqcVGp/VFO9jv0MYQ8ycAJw0uGAzPI/UaA8LRCV99Pm9a2/gCb27nMHhVf/TNXT9drn9Pe3Ljfk/U/56cY5Rc/UHf9qk7v/431rzwu1uS9if52DFubiqK4kMvmuPe7qntBeYZS4lgmiUM0XzuwtWK5lqOHKJJpnIqedvQb4Thoula3i1WwZRYrb9E6y1Zjp696y6W2r1WIYJHLlB60LTRt6APNp+0P2lw5xxfANABU/umwfMX8+zueldcfLquF38/HPywJ92/BENg5VXLdyY9cOhjd1DPgN4bWxkvpFgLXziiQHQbGtqrdHiIndCY1tq0I13621Z9zD5pZ6Oha/ieRkke0r9I0DapaUEvsUmDO8GfhufF0PuTh5XkLt3hwFPcgzz3bjHFWVMnAhhQCEm0CoMnF4B974ENJsSQhocSyyE3Y0HzdF1ZE2HWQj7ElCmlGnR+4pmKYcmhRpHU0sBs1SJeYX6ZYdbgJPI70Xkx7nfs8ak60mvEufm7v1Ksu/z1/fHdmfpXWQ2l4PYIGW/NGCn67dEy38BHtVZFs3ACGO7Wh6F10VIZRLpv0tkYXTjULWG2iHZrcdemVTTFmOagM1JvAf43hJgdoq6PYJopZVnQcHnW4NcD/DkwZoez2rm6aKd89sUKgBun7w/re+nCnnIkhUyEOgsSTDaJhp7AppC9FbGQp0M77Qpg9Y07gaEkm33XssADsK6VMpsLnogQPSUlhzcvUA0NGsk1bgRFZHSvvZp5ojJr8FtAdwLxcjQP722W9wShnDE7o7HsTNJVT1Zp2YBmTPhZFyJiacm3yE2bYICdjWgBsXAmU2mtpjaafF208nRNtUqspaYcmQLYz2g5+iTgw3lBQ7tZg7OQLl0EfcqSihnNMDtp/GqLWHce5fi+GPpLikwTpvvu9JYcq25mZu+SRBOFii0tiIN8GZaFKi7XIsVplN6pmTpyg5wZAcOs+dX2SWruhitpF/gQvZGCxyQaiD4OPFqLd5Q46gL/Pmvw67GzF4nZYXNv5GO/3mj29iGVwt2eMu8eviwb7YaTqh3asqpL6loDhQFc1g1tULVMDcw0JrbFYPp7W7qVpAFurZzBEAkS0bSu+zBgB5FiIc42Qh8lLY3bQBMWtEyZNbgDwy+v/1DkDnbfnvZsVUaDr1Q02TNHa32XSknb3sBTZ5Keg1E61aQXbqVJYJcDeNYow2sjidWWB03WFiRUbSipSY6tmAq9R1lL6sP/IroyhM88xZo0uB++pU77/vptrtJ274vbmS4sX4wD3WnYs9SLWJvAh5MID6elTBFxwSCbNhWXliLmo5ZTFMoO6tAaq6XAKaYateZDWCu5CG6+gdjVlnU7IsVIOvUljOyzA68dzhVZcJpl1uAOLJ/G4jUgPZxhlRT25IRAyZeQ9bin974DW5Dk7hhAs9d6mTm5bTUjanBRkIC4Mqc2RmxWD3qtVM+0lJQ6YrbW5ZTIgmEVzKjSPSigxuzWMczmYZ01OA/rwdP194PipHUu7jmoxEDMeoevVzgP27WEw2BN1KWCSMgjuu4Sk+9ah9ikUlvaLnpUa+tqbWCDpNpsV7XiYi+uuhLCqCBihHEEOBzYu7VtwXHBOYM7YPz96jDATj58qts64Frg9vAUEPJ7IMzM1fQE10o2J2kSS4fbgVN2w+RRs5NggV7GQNWiMgHGEFdHYShZH9aC0MeatbYnpUhV8CTF5GRSqNE7W4WinjvKC8puzxrcCeGTYBwC6vXnS93Gue680QW011M5F2ff9IC6X1gjt80jObkpt71W4zlNWLz6rR3ftijAKHtBW0OC69lDDLLVY3hGrFbOlI6AiVkJdyqmBAKhTUbHMihSj90GTcjWVB9LWncNXH41YgAxV8GeofTKqKkbPI+JxlUtyqep8ykNMn4BMZgzuLch2D5cDmz7dm/ywbrzLxcHZh6WsudAczKuU8ilsPbBY+e1xzzZse3Lql+eUgcrIMSW4BooIXyU9kjTI5YITGstsNUYwL4heq0Ylb8hJj2CXAP0b9QYj5BBmefPpM8anEP6W1heGeiD57UFVHu6wWDSWqMkMIHojUY89Oy6CnFPQ3mDCdr6h0tMrmHOV8i7AdHai0lW/GplokZLreli54ALiVqHGE4lJXAdsZrpnmwyJiwoEzVn8CC0X2lu3+zF6J8O0uh5X9efETO+JB6gU/HQdCCQkRP8QM08BmjzkI7AnPGPvhnuXH3lGkctpGcZ11pj0TUQLeKbwWfhZtggroSqhxRcCFxAkOBazLxInzU4hfNXgLwKuJAUHw8Kzom83ROcNQmPbeyOiHJ0JkPotSKEUByrayOamDPCMMRR4rjdF2qjmxpjZF5rAW0Uagb8XReAetelgxwQIeq24kALoXqNJTKfrTdrcAG4t4AcCO79Oro22Dy+/FTPTq8+9M0KVG/HnVZraaweThOytwdm9IeD5Cw0zu6j4mAh4jo0UADLgsxhHqFpGVTrmp7dpJZ0EoOFaC9P0xGOvQpAInyo81rFyiWOpqXiRxo5VzZaja6HkrXtBkRab0GCTzJ/ZGnW4P4es08CcmgPYTW2DV/H33bJXms0Pb7fqv2x997wRprph1871X7ZY/x7PcGm/3bRtrdd946NL7W8gZzI5sAzUsGa3VHPaw0sbWDAEB81US7cJBnxPhrNT9NqPjaC7XS4DQ+N2mKrxrhtidOyWo662NRsSRQw79loEWRE5E69IYYzVdBtBq1esPU3a/B5/7EDlpd7kXaB5z495+uLzUESJflo9yyZpSyQYsLVhAS6ovWZOu7tHPi80aNUoUu3jrsWPpcqvjjwIWU/CBNrLT1kUV9uAg9HrC1hxY8YBUSbkmxrg/Q8EMHmMz4mDU7j/BUorwrywWLUUdxziNlH0pLLpiQXB4VqfC2QmbF6o+cxtExMBOJa+R6QJz8aZoPxo0ew3+TWaj9rbPAGDIRssnZk8QYkxIc+7LbHaB8+UKM2z2NnDR6A9Aul6AOjt0uX21h5dc6XVx8urv/gkHXpTipvNqfguRM3fn7wWbdbINPI3VaTtR8uCYgaychWRXOMlvVwUNLCMJAbADKXFCAsvfaW09LL3q+VCVsiQ915X25qzduEJyE7ogtJm1VpheBgOs3vrcwanB17+zF6+TC81BKeeiD09EXYJ5f2VFEJQ2uoibhCCB45RBq+lZiNbntHr9U1otdSMtoFzejHrdMzY1xMz2Et/UwmdGv0AH9FVGsxtNR60pq+PXtXZVDjbheU1Jg1uAD7J7B5Fcy3bmfTFVQ1/RrTPwST93TO4lHFKOLNsyneRhOUX1ixoIymGRlet/Ycc9PUFham1luT4kJdLQcJGthmB16gx+F7jH3IQJRAVKQUtegPBGRmNx96Zg0uGALPQ/UqI0I176uMgwgNsHscJOc7ZQ9OqQdEWqx6Cl9cLB2DAZzbpFqhI+D2bUI0gN+AzwBHyZhRJq8mKii3oRkaxtqaG+dEaUAT+KgHEiskgrEyalvQQmjO4IJxsAugV0CfN9efT84urg5ahCpxd+3JpPV6jRnNReLQfUdANFXLFRvjxWmb1ZEsQW5Iy1S2zU/gE0Yo0JmU7WoVGYoPYLtGaoQiCJTNdmHeZC8ltzSo9hr8kiJ3cwan4X6IyGsBfHuU6lXmOJzZnjZhrceiFZGgNGJrtrtuJOg+J6L5IDeSaFet4LTlWS9xaHNMKM6MN1pXO+sIOdD8kAZJk3uQxkNSAy3RjB9Qk6bdyqi1+RyMWYPLQN+N0iuMg7t6CveLVQcFelP27IQl3fDLEdy9+obX4AXUP3s9Etict87WVrWEVvUQAG6YQeoOBy7h4c1qSVQ8eu+pSchie24ueR7WM54DqEEKc/LwRWGe7M0anEb/KWzWgPx1tF40dvdCg56kcb4YzqSp5Zj9mCB6moa6lJ58giCCL7BaqT1TqxUjRNsIpgJv6exqJYdbqZW6r6ZIgyCLAs3B2sVCcixwR60aAfmYL4o5afDwMbCSH9hc/NLb1qZWZrvYnF5/PswT7OnqCaJrlQKR7jCxFjZpXpo3hbWaOFSRvqDYOmuoByXWgwlMuUdKBhpgLUcgUkySHjIJJVdi0wdMaTgbtJso/o8dDZo/djBrcHoQPA3OoahvE+can52dbH88aLKnsGefMZAIg7oFSx0zPVVdaGXqBAefRFtmIV6U1loAiffa1i6k0SM4YUKIWE/aDZczGacL2gaSrIe0TbFytXkPD80+MNTHPMebNbgf52/heBGwGwyZq8ORDRifu5Npq5NBpWlDbz09FLLrGVzHwL/5zilR4cJR3RyB/CCY87Y8tZaY92RX63HnbWh55Ga0L0YVMwZ+bL4WqdK1kkziFNyC9eJZgxPIfo3Ha0F7+JoMgWbv2fkpIGoQ2QYUthY9VJQtQ5/ZRKRFH4MNIwbalgO1HhzXl4H/WTGuj7qWGO9JtKE6+JVAJLdQI9iUZccxgSiQVu8zxAsOhs0aXIbvS8PxgxJ132YJrLb6/+RdV80VeHDH2yWMxpvzl9FPcm7PiTnXHZRH7gn+2IB7cGREH4n4b6oaqmkU16WbFhsitBU9dybgJbE4m/xafmtkrym/CdHCpWY8XCdmGUSvz9aBDEm3RcujzedrTRqcGNf78XmFIX5/Qvaw4+17UuersfDenH0cjAleYxeB5hBpuTctqmKKCwViM3LIaeSSatTOfd0LJKlZK8+pUQ7RV2u7K3YkbuyccVky3E5B1O1xm9a9oLTSpMFptB9CsgxhTZLbPuVvffP5lV3V1vZtU50TXPvqSXiHFhR2Ju85ZE1lUM/wK1qAwG6LIGEghgryBE0AlTuSAykuAjMQiRacSnSq2lC3+XdrLXvkZAaZXnx3BqMlc/IhIcJrIVwTddOljjTmu3LPGnxqFH48JB/mYTL4C8oc+BJ2EyETSYutuZggXSo7UU7b/Mih1Wa1QSDcadOEpyzkbITXHd365LIrhWm1XiwVqipbO4L3A+LZ5TYkFyrB1ght7awLedD8mtWswafAexqHF2N5OAXQ/hu7C285caVFLdqDiN5bsV7LVNQA4Wm1LVJo3mozLEdwqSEhTnpN2hZqYq2sVXItePKDrHHVOW2z4IU9RlVCXOqp56BpSXhFCxp3TBqcRHQ24I/NFh45Pjutx59OMbPBWT/yr9tmv1v+qktW5++3JwzP+PM7gT8/4Kin27O+FF3SU0Syzc6TmhOiYEtMQ0ozZDQbOEijNFqsqWTgb5x2NRPXhrW0msctPeGle13vbE4bAlg4SV8b+AjYpmQoSS0AOe9xJw3eQHwQMtNwb7Ojb8xc/e0Ty8ntEbztsDkI421rst2bCdqI2WXjAadtw3uSpOvERJDkJXmIOld5FC4aXcHgPWGuZ/wG9DetdqymQFvo8WltCtwpVSkYcoyRhfgeLesq54ic51cgZg0+wvhZOA4A9masPBolLwBYq3fsZk0BgjxhBJOYMlwcsWkj3mqALYKTKWD0YPZdD89qdoDDXICca4Vb1WyytUpTFd9GsHm0zsNgYCE8NIOx5nPRSsUdFJxyWlCaatLgTnyfQWUa5997PR5nn651i7FeXFxfXW/48t0BTUOT2ZPr53s3FCDIRvXAi4M2EI2FNAnK1ua5xG6bdQOxSXpEDNaJHkGtcBnn1Q4zFwkYRtrIq/SBwaSpaMaOhtu62jvVAGGYFxxmnjP4CNVdGCwC8aavwIfrj2cHqOk9x5NzcoDC1lpdqPCqYQxr9OyjiJaEgIoxLkT2RKYgEFmM6SzAU7O04iirNes1xeLlii0dod0FmxwmkR+mBd1d7saXOsySZr2TBp/A7sur/waxf3j0xbdPcNO+q3tnoA5D7qYxPHrpJeHmJSQ2DE3B2YS7emm319/hB0g/fjy93m5hRi3e57U6V2s2DBdCBG33QSrCn67a9qaFLu7fw/cYCpvT/vUE//4dv8fvj/vfe9NmXVe32chHl+7opofX0bZr3+XF6fn1u49ykPL+5h5bvnl0v+p4dLeAcPRl+fHQW70/vf7wqR7/frH5VetZaHPed58PmBoY1XDDOyeHHeDV1PCvGKoVNpCHKTjLRaVw6CnVSlGgOOwA/yYaeSQt7QyKzpbCakclEFqgyattHdTWG9YhgFmeRg2gETH2oqxiwVGJOYM/X+968/t82b/wr127NxzQUSOk3btVTbtKsD4pb0sRkamNfRHD0YfCUbvGOte1lTyDTugET9AKzoBWhFpX68SpG9ohBa85btUhMIokDJ4MZ9AELsgIBolZ0Ilz0uDP13cveh8YcgFJuDm6bQr0bqsMj25m6lYZHjaN8Hi7Z1FAMAETKBYBJsbYtNpZzKprEmivl5CMlkEwvC3YQx0zSAvQguhLdxTXAkozU0MbxVejGhGjRae6BzPXwl6pW/LkDM8Lt1mDP1/PgPA8iPfpI3ctno5u6uX0I6lX764+HJDUH3cTBYngAsWVLPAFobSsi9Il6L6LK9ooLbUQQOwq9KumwyduYL0Bsy9q64+1DhPVKlr3icVH7+B6rWVjY2+x1KKH17X3sAcA83VMJg3eofgcCofgeFdv6ehLZfFd3U+n4uXj+/TzK+2TfXe7KwT/j3zIiLH7iuE3q4vHRcst26in71i3LkC1gm3Z6toqJgVpjSOCHBhlwJVDHDbhCk/e14qeMWaikmPP0UCbxWR0D5CkGZ9iC6Dxvjsb5vNCZw1+O2J24fDMmLk6/sc//fSnkx9/+te//vDjExxqIpsvhN1rqVBxBKWm7dertsLVyrGpQqtHbQotBEWXs7YxN7bGKLVqD+SMCQKvwGW1jri26xIB3EsCkTZOnzYbo4VmgjJqKTziaAu2NmYNbnF7/M6fheinH3786c9/+adD4IF4DnaP2i7ej9zF0eCm53JHEoROAiM1WpffpMQgBaOZUZzKbE2/9pBx1g/wpNVy7LOnAfmoJSao9dSrozqidmL838R92XJcR7Lk+3zGfQeRkXs+ttlcGxuz2WxajzKDRW4SJJLgBUCp+ffjXgB3VCHrAOdOd0sNgkBU1fHMCPfMWHRKzaaH6kM8I8d+zeAjPF+f97PQzKGs2L779qhlC05igxw/FTFeXUhtYu8AA+OiT4LQ4SYYKJQck+I4YruaQyYSaWnwPo40XW+sJtorLR5SvuXumB8RsPhlCN5dZ6kiVL6ZsZWe4hn8dNXgI05HHv6zoL27brc3HD0NrvTNPKmLx29twS+FYI7T1oY3Ppn5hz3mRjKhuOnh8XKqJfEz19yncck362ouYRZ2o081RFtdd3vR1lbwN2rTgMDU0FlWUdnlRTh/2AVQMS/41jptXTX4iN/zOJwJJRs/vgRF7qsT3bQhjUKvk30eZvcIW7aLgIFAjajoaLxXMWmaYIwt0c1DOUctoR5GnOzVfq+lh3rhUsZkX1L45+IbmBLH1abQXIL2NmU9mK0afBLFHyA4E8AH4fICBMWIE3eiifb03TmriQOySncZyGFnMk01tCgcjwhhj5gpPTAbP9Q0EvwPJFhJuwl9vCVnqvENnsFD+uU6eKzdJZVonQPv7S6dlVi+aPBJCH8E4UwMHyXoC3ZhKnI84QI7sIEQA8GE/TjYlAI0hUM62BqSlcTYeVihDnSzIGioCcLrzu69qX6/PuiBbZ0lh1xa7Szhr2C0LOLubdYI1+ARscu6FFg1+CSEP2LwLISUhLznVbn4Ohbzon+81Xr99vr+0wXPYi94GLtNJORyIjgWzpeYRTKzwNUzxHfbeOJ2aOGAD81SgFLVzNGtAWMV50uHrkMI3SvfIiV2A5ucolsL/ECB0AyKZYWgBs4lQcm/yvqeXDX4COhZiCzjy6EyD8My7y7+Grc8TB+dhwSdj+Tty2C2Lp9IrR3ZwIcmceBzkyy158nhIwXqyOc6S61TewstDEZMaKeI8BPZRr2HPvcisdayIb218CuRheWxlGxCdj4aM2oUCO+Ys13fuKsGf8D5HGSW4f7Kqb5ZQw9zbi8OM+s23pp8uL25v3l0M5e3ls0m3t/faru/eqfvryfPl/64u3m/ITk7iz9+DOQT2ONMmZW/cVhp0NrSc+9QROxS6Bu0XFEoVTbLNdqsnaIWO63KgCTfK28H7mhAiTULfZw7Is5ESNLqappt9JQqdZg7o0B00eCv94tAnFowx00ccgh++s6mYhIHunWcWAO4DMc4nYdD8N3n4oyFX6xlZKaygFe3xgERtpVkepFiA2tKwqHR6F6pOhGP2Yi1rWmEk7LYx4esTiaCdwt15sckmVo/3Vs0eArWo5icTAFot9cf7nnBdvnhd70b5mJe/4vS+dhR8MJtJ+tvjyfWd4UIopCt8PCDd2fVTIW/9zL9DAJGwPLXzMZpERp4DK0QSTK0Mf9+r9vOHEVqZdEDp+jk4tlLCDyqeikgjVRz4BTrOZSrBn+9fxaCRfzunVw8prhf9Gv97f0NPH2724Kh5YM/MfqvGAXPhLAtIZVaciqN+egObxN0O7nAmigzsHdLzU7mgN7gXduo3qS4m6+d0Gyst/EyPK/Sh0VM8BEeJHmpBtKttWHP8LVrBr+D8DgKizD+PeoXA/X25m9Q9Tfvfs6lWqtTeNLsm8O6eqnNPv66/I1TCHkL9f3C3eQ2fD6RljviqFhMIMJaChyEL8AlDw0O7rJqCNKi9RMEscNPtlH0sOZYqoMQ2fZS51jgylYjgbeRDiw21uniaG7OYGxIlnXpiFLrRaOLBr8uudMwrKw5WrgdhxqeHww85NBsQdNbd9x/QJ5l9QLBynZ8MySWSQZAOtgN1OiAyx+Hsggmu6jYYtLkTMGkodvdzqxLkYpdjjCcrQxjyzQlIiIPmT3hvdUEUhnjOpirBr8H8xkoVhDFPiezvzykW0Eo3I33/eLP8WkLkrGc6PIaOqQ3ODQjQXQtgHNrc9oLe/KwBSMzbhngwbdH6+BkvL634GgNeq7tNgTSYb9Uy46PFeQwNPFl2MmoBDFtpuOpUM5n1CctGvyK5AkIXoLgZlaG7XYi6wI4iWNrRHZOwQdNKYcwx2i5c3JQ4YDfPlvK0K+pI5KDk1ef+vBwt2GvDZlj63UaBFmvUM6hQf0z+cpGLDCQwphHZN3tOidbNPg8jOdQs882OKju4vbm7dubj/cXH2jr9q9D2sXl+5v3hyl2Bw2/KXbK6c5ZbobpmRxRxlQ3h43YirFJhrBwEgYPX8zsUZ3ppnZXc9Jup2H/6r1GffqAlwihQ7t1LrgRPRabs4pFKLVRvicwrvVTtFWDP6O7jMzrwP2QDHuwuXk/W3EnKlycFSYccLxItt0PKcmKNV04PanZqdXWZrMJhcnYzLJQKM1UPPtXa9vtJNyFBhHQIjxoDdmPGBpbu0O4R4l8G7kZOWO666rBcyB/Ap3VekR21rnkNw5S/A/9Sx91+nc/wFkml//AV//n83iQf+LFx+3/+Fw5/cvhbAa/vcER2BMdcyGgax9Rw4wd/t0Nkczu+04rZ6har8MODx4NDd47dgxnqDWstGGwqXZLuQlguHBOIXGMZmZvo8o5XhV72ZqJ3Qz1nGxbz+lYNfh9+eJrg7e6aNrvev/NoJvru69z3l9QmWz5QE4cuHdJg1vf45lENheWXge2T3GexYBgdfgR7bydj2xwb52WKRYe1E1tux3C5D4G3pDxbBRTw8jYvwExOUP3FJsZlKo/ozv/osHvV8ISImfBu7U7gPXhxLXJDBWeHO4dUAaecadEZ1dKsWq1T+8Brc4+q7rQp4DiGROYReu1mzR3u7K2tYBONM+WBb0XvrdggmO+kRZvrUvSSjnjynrN4BMontkcgK1K/tZPn1Nkr5jnfvX1fuSK2kxv7x/GDW+uMed82OMJXWMCQGOb44yMmly2M4K/5FqYk1atPzRFKOwv1MnLcwSHa65DWdoY3G6Jd6M2lyokeUsgCU5VwA4VvIIjQwX+Bqwy2TMS79YMfg/qFoBWwf9yI/ZjY5Nv7D9ckB26Z/92q/31hsQeffGHevrD5/xwe9M/tnF79SVhe8+Xf7hx5N3jY23af87L/Cd9pM/Z0f95r/R6H+xLuv7XV77q9ZKr3IarJxbMm8OF7t1/vN1QHHWiqBMqo6YCHhkRiUashgezUJPsyqmu92wL+KSp+TDp0EicAkcF6S0Nmmyvu7lqrUjKnPTAASCeidYgNqzjsjrHRMwf1dV1T7Vq8HtP9QKUlh3WqZeI37pFHls8+q67FywGf2LUdALaJkNTgk2GrmJCj4ZlpSUbP6dNWtkUsM1ssGxKYj5dnG4OdU3CXocMzuVeIQgz4maaLBGKHTG12OjYgnGA/U72zl9eDKsGz1gMzyP1GgvCyZXefXrf2P71Bk/u6DJ4VX/006t+/LD/a9qrB/d7Rf979R6r5O4/6dM+9cqv/4mZSPv5JYUjHN4NrJuHpov4oRftcW9PtD8C84ylRDDNEtgcZo6i1XYmp032KEimair5MPNsUoWbwQ5YsXZsid1a8LfRktXo1bvhYqnDSykTHrlA6UHTRt8C39h6lvOiwaN7/AyANmzthxm4X82r+9ze/+bjfb3513bww4k6uxKMgJVXdjhMrNDykeesE9B7Y6vigbAmvSYeLvVhZHKc4/TMpm4qe+ViQiSpPzTGyz5x+MuMqRgvsyQP6V964CTJdMa4pUWDR8FfhufF0Purb5ttfXDbgZdTFZY6hsUWB6OLKYABhZD6iDZ4cQHYjxHUYEPM3vCmutWQm7GgeS2VPkT2UqQSYsqSUg3cn3hPxWjPocbeU0sTu7WrrWH9mGHV4CLyR9F5Me6f2eNTrXb3iHPrr/5Kse7Dn79dfq5OfpXTUAnuhJApLMQOnk0288guSRxenWtVytQUR9Ygqble4Pm6ZvWcbepmTWzWoWkv7lo1m2o5rCrWBLcqLgb1rCJls1YAYaBDrFmPa6sGv1/gz4Gxupxp5+6mXevbr1YA3Lz+bdNowFBO9IIAXB4O2ygnJ1TvWG8Lf9UhUIfNCEjNJct2UsLuIz2XKhX/jm1IiXWvIRE5MYiy+X22hsNEe+i8mWtB+S5YBaytlvV5YKsGfwb0KBAvR3PzuWiWE1NmeU9doLvgo2u3ZhQWuEQwMtcgJ3PH53SFw3JSM9BhNSFGz8oyO1LW3QYAgexI40hbaMA2glUnbgZjeJtmrR2ebQVDW9eXqwZXIT33EPQpSxQzzDC7aro9zTue6AUQbYJkVteCB3nUxAFvKbsKfdFrD8z/PexV77rhkbgzySdfJDYLnr5bEew0LbVuiihHNsR26OUa8oSmQYAYQyIni50xMmDV4PPgPgHJKsKfffnll07dL+mALGLi8QIuBJrgtWbvUsdng3OypQXEzlGm1S4Vv84OumkWfHps25kbhOQMmrT53W6oKpaTVmEYD9GbDp8fRGYT66Bg2GeiRDiP9cSiVYPfA3sSibPhfDfub5lovKWNtTvRg9wb0CLDUS2pWjqmOnoabNeBVZyyyuQ0riwNmiAmtcVEuClbhu0J63u3HL9A/lJMG7wBE2HCBzZWthHKNAlLfJzxZ2zOVYNHMPz6+Lcit9m3ejlxSRwNPlKZCRwhWutHr5I4k8UjakofOUCrAeE+irbS4GzZck7NLNNzysFuB7Mmcz6GVAsi13qOrZjK8l52doV3BAJd9YxUoFWDp+Hb7EwPWWKHW0dtb3mkfzM3utNw4pC9ppgaJCorSiDHfcnii9bGideguKx5Km6y9ZmDLgevb6k7AWWKbE+wW1oX3HybFv/KvAhKMQq3PnOKfHa5z+lc6bOc0QJ1zeARLJ/G4jUg3T7atKRwIhtnjFDYvg28xvsBbKOM4UCEuvpaQX8Sp/6qlwYXBfGN38wJ7B4EKLrdRpuWktJAzB6sWo3asayCmWAuXuXQDLENLLMzergsGlyHdfN2/XtTnLRMKDveMkmBmPUOH69onnbYwQrlhtAPVZmdgucNl1T8qCzgT6W2dDhuqtbW3SRK6Kk2yz7gycVRHHRSCLOCiAnWEeBwLbEJ7/ruXDR4BMa/77YBdvX7x3roUc2OoNuTb8SfgDCrVjMSXKtY9ufrsQy4HThlNw99I10PFuhlLFT2P4FeATmKs6jG6MNeEPpYc7E1SopSO95JMTmZFGr0ztYuMbCR6jyjzGjR4FEInwRjC6j3nz7wAu1+6O1hJP1rJUnwCubHVJovR5riDhk8Vw/9ifeaisZU0bu/2uVj+3ysshfM3GOv+hPEIFsfhafvLNeAtASsrsCdQmwGAaFNhmsZFGnEYYO3vTHJykpOpYDL70YMIOYq2DOUXpk1DdMPOcUOMW06H+ED0xTjzyAGawZPTqs6hcvGmWRfTH5z4v/Hzcacz1JOdFJPxg0JuRTlkDZ1ngPQxc7D0FB+eEkDrEAQW4JroITwURzgVaV0BKa9Ok3VyMoCiF7bDeVviMmP5GuA/mXznYqQIVnXm0WvGlxD+mdYXhnozfvaAqrjGfsRm9YaksAEojcbe1dgfVOIe5nkDSZYG6aWmFzDnq+QdxOidRSTbN/rYKyY2VJrrmmbcCFxtOHhVFJqHNPuPbtbJmPWi+VXDW5C+5X29sMtGP+0SaNDuRzHecaMD4k3MKR4aDoQyKiJB9xZ5wRtnn0gMGf8h09Gh1ZftcZZC5zBbhNplGcgubaWwWfhZtQgroSKkFJdCFpAkOBazBlThxYNLuH8HSCvAi4kxbtNwTmJtyeCM9Mf1cbhRCRHZzKEXitdEIpjdW1GE3NGGIY4SgqPzWPhOUyNMarudYA2izQD/s4DoDEO7VICIkQtHsuthVA9Y0lfz5NcNXgGuI+AbAT3y3E5pz9efvhY317f/T5ut7lpe1w1JHhp1pOa7DLiEmRfmb5A1tbYdLKQKnbEMSz34mvX3Kpjjr+4oC1COe3lpUWH66x3OTQdgXJhl/QSC5VKd1qyq3nMdeG3avAkwMdBeV2UX3CaKsfpVwT7VHY5QdxGNJZqsaNL6n2ySTCTCDhxy8OdM0nUudJ6Md51FmhozHt1r5ECBdACjwPnaK62B36EtwIR64rzvReRMzrSrRrcAvVL5/8yvLMO5FCnxy82neVA4B+vnwUF747tFAMkxqF/2AyN7Wqta6HZLi0xgvVxmLJqBh6N5+mHCH5o6F4w9zhbkTInxxBWNewaOABSzjEHB6ACVmDqZr2yatHg6em/TwLyEnQP3O3y5/nlewwSf+r1dp1cfvIFH84l+MOvXeFz3tv4//UObsdfN+3wsvu+YtMP7KrSr/rtxtLMYM1xyud9VgTJORTKuyaECNZZm+59NEyL5dBfG0H1B9yGl5FabNWwcl2LLbuVxnSbmi1s6hCnGjarBh0dMhoILGIZtOZh7On6vfeqwef9xxFYXu5F2g3e9/V7vb/ZRvw4Q/DEeXHKvUnuWk1I4OpVcx94bYeA3wwrOMNg1zAdPajttfviMqcpDIswUXcb3UVfboJOJ8phvd3PGDtUJsK3gqEwQR8R7IzRXWsGl3H+DpRXBXkz73MST+Sf+QhxkwwHzcQpoRpfSzEJdNCwDExc9hGIDxtBkkZiBlcEdZgjQvolt9dgImOZ9peM2GTtzN0bkBAfxrRgLqWO6QN4alsXcasGNyD9Grzvwejjuf0hVt691w93v9/c/yeHrA/uqurt7TXo7MILP7/4rDsx+XTmwbm00+iUDqImfWbLEyO22GJNYhqjZGhtAAk1EthWTdj+ZXbj/V4J+CVC8SXvy8NMAJvwTsTO6EKqmYM5zAhmyBn6ctHg6to7jdHLl+EHtlplHfr1i7BP7kTPFh8mEJ+9uyIIHjlEmWwkmg1zPiK8j+IB1ok1YYs3/HHrWKqqxYwc9jo8EhOGNewbwilsLYbG9vNhQv9nzuOd0nTYeUbZzaLBM7B/AptXwfzgdm4HQaXp19j+nE1wvH9X11m7IeLNqyneRhPIL2y3oIymmT4977WdamNel3aVNlrrhfNzd1sCLVmOTzTswjFiHLNPRAlERUnR8cTPzaxuPfSsGjxjCTwP1ausCGreV1kHERrg+Drg6EfJHpySdWktVjb/6C6WgcUAzm1SrdARcPs2IRrAb8BngKNk7CizWy/sLrlNpicZa2tumpOkCU3gI+ugKySCsX3W9TCwavCMdXAMoFdAX2/vP129vbnbdAhV4vEiFibBQxPO5qJoGH4gIJoaOna/8d1Viw2SrHQObM1S8Bc5wSfMUKAzJdvdGsEUH8B2Ta8RiiBINodbKZN9L7mlKXXU4OWMRjBrBpfh/haR1wL4sYLzVfZ4Ot2rccRSk4lQGrE1O9h8LfCSH9F8ipscs8fpWX46zjqY2NqQki0blg7slgootvnZGyRNHqHzVoMNTw9TrEBNWlSPAN3WE5BWDZ4H+nGUXmEdfG7j8uWwalOgN+XENXDibXeO4O7VNzwG34Vj41mJ3Jy3ztZWQf+keggAN80UusOJX9HpzW4ZhDrHGKn1kLsdubnEOiuveB9ADVJYk4cvCmcMsVg0uIz+U9jsAfnraL1o7PGDhtiMOl+MZmFdBXY/NohUk2T0MpJPEETwBdZwyoK0WrFC4gwzFXhLZ/die7YVTmXw1ZTeIMhih+bgGDTtORa4o1ZNB/lYn4WzaHD7GtjJD9ze/DHawSYbQt7cXt9/2uYJTrS7B9G1pEDCGyZlP6Xme/Ns/m6LQBXxAcU2lKEelJhVOSqZM9wNNMBejqD3YlIfIUuXxNJavsGUprPBOvbZdOrkrGF0iwaXF8HT4GxF/ZA12vTt26vDl5s2ewon7hmD9K6gbsHKwE5PlQetKkPg4FPnaDPEi9JaCyDx3rVZONA1ghMmhIj9pN10OYtxPNA2HNsR0iG/0NXmPTy0+qBQH+scb9XgaZx/huNFwN5iydxtRzZgfR7PCamusy5bOT6yTR+yGxlcx8C/+aEpSdGikW5OQH4QzLXWodEK3L7Yslflo7eB85SaGUChdjMnvmy+ss52sIFV0hTcGefFqwYXkP0ej9eCdvuZjIBmn7j5KSBqENkGFLYWVtRlq9BnNomw12ywYcYgMybXLSdS+TLxj+3GjVn3EuMj9dChCgYH8PQWagSbsuo0JhAFYdNQw2kb6+ps0eB5+L40HH/TGfPnLIHdTv+ffNVdcwW+ecXHI4ymt+9fRj/FuRPlom44KI88EvyxAffQqIg+PeJ/qTJUyyxu9GFabIjQtrPoshcOSXI2+b381sye+e6J47RSMx6uE7sMotdn60CG+rCFXRnXkxUXDS6s69P4vMIS/1Ievum4IZ+oG6nGwntr9nFy8k6No/fAKWstc3QSHFpxoUBsRg2Zs+hSjRn0c/gOSWr2ynNqkkP01bItjZ1JmzpnXO4Zbqcg6o54qGk4o6PbosFltL+F5DyEmQt3eJd/jdtP2++MUjnRpw+6sVanpWPTqo8msz+Mq4VDyh0PDYawawwHsufRPScsKWfZGJOSaXvViATfsIbEh0PQcLmGCPorpjTjfK0tMXkWLHC9GmjR4FOoPgnDBiQfR6pd4e29IOcUlOE41WiclOVy6naGPsqMU3jr13hAqrm6Arg59LOVlkRnFyvU0qwAG2GUvZr1c6hIMKGB/LHVvOYced1UXQtsf+EQNwr8xHoy4qrBo3A+icV5mG5t1M+GCsflvBoXe9IOJ+Og14WVshB+sUjJwnThXkIbwwtg7JD42XE8mBUEqeJr3KsHw6g8hLWQYaqOhRgzdyPWZV4fT2hRzlINeawjuGjwKQTfbUn4+rbU5wVNbHwJx7eficImpi4maPOqrlO04WMe5rPaNAp0eGj2MK9OnI2gFXNYn1x2pajU3cALPWQLn+D9bGAxubGRmZRga7TTOetCnqJngLdm8CnwnsbhxVhu96cnB2IBQ1dalFEFlHW0Yj2bENUw4gSIGXB6EFvJTsAZQuLYSZbkdGnd2j52i4/ipzBQO9dnjfDuHqsqIcaNNODk2+h4ROGM+LhmcBHRVY86bw/w9Mu31/Xy4zV2NkTZO/3zMHX8INB4Jvv+t0P9+Fv99KbDbZ9/dYq1ehzhEKTW7tiWGG7UgPSBC8Sa2UMtTGzQFBBEM9ujaIewwVMwtcUIyi+p7pWt14yCcoXpfIBUmohsPdnDmKpYkkPws9nM1NYRXjX4gPAmYJbRPmT/P5i5+4+P2q8e66sPq2YTxC6dqpmMcbJFAfhOa9M3JsdBmii2sK8eEmXCOWdj2dtI2b1ptoqnUctgM+did2O5TJM0EhHwoSFNlWjqaDZPNo+VCfY2PHzpGTPOFg3+gPGzcGwA9mGt/LBKXgCwzydOUJuThCgU80BsRVxNrku0QVNoEqBHbQcV7qCLUpV9IcENnQH/TSIux7CXl/Y6mJUxzRwz9W57daTiCA0B2lh7KyWzGGz9BHXR4FF8n0FlGee/R72cbz/e8wq93tzc393f6oc3W6dE/2j5YYbM7/fvNsydsMWfOLCC3DGcSwoSpiZrQWhXLI3EDqh2RNs05TJBstlZc4xa8acCXmNdsbKXOoIC93Y44XiLkMEXwTbcjKCRtYn6zm4N2Z/RsmXV4A/L5PtH/9Na+C8/fPDDO3ioaZWm1adhBu8osBFB7mWCVhne0ZYYTSObeLD1+Puf8QOk795d3z/MHIQJCFEXZGCFQ/K4XByHxKtwlGz0bEQsX7fLv2Ep3F6P71fdv73R3/D9y/Gv0TiY8e4xBfzig7t4mNd4cZjQ+uHm+v39m3f9/POU4o7f0s0skbNfbLIte88GFB4Ke0TJkNsWwAxlCsaAOO8TEIE3smVDO3SfnXudkqUJmu6maVIzBFcKh9m1hWd0qQ6EuazWpLkuwFcN/np/JhynXNBPpg7M8+LLAfvF50Oai68n7RsALtG7Uw3AnVqFZ8jVTN8jRGuAJMCSNfC8QJYDM41r+EtJztXUQStqHyWF2dvYrYynpynZYrsoFmAyAWIkBsE7ozjL2IEKeWbOKONZM/gEwEuonMT5t+v73z/Wy79vbv9k06i7y3b95tOGaCAgc0ZOjL6dtcNbzcmpttildVjmS4AoSW+p8qR/JsSAij+DOONDuxBA/PH5e/V7bVbwzwS/kUhW8e56Z1MILC2DeGWKwl0YM/xYv1JfNQgsjzz5U2j9T/1zcDjVloFh/sRZijqp3ThXpphWjBvZuRGgxX1m4aNls0PINqcTey6xhgW0nYAO9gzcK1jPCfdgq2kcSgW/pxAKJWCX5JiZZzvmUFPNenLTqsFf7z8/6FNg9Bso89uLx5mHbw4C/eJhQx4E+rZtFFI40UHf5+5dLTZUhPrY2FI0ZnBRm8CvfA/JsNeQ0UNXPBmpQIsEZu93RL+4V+MhZsCD8Rcsbij3FjRMadZnyPk8TQIV9OKMrourVYO/3q+A8DyIX9LUPk+wvHhoSjcuer17c/f7huKhExdBxTRbemBlUHHs99d19KhQzQl7q2kRiGQwMjERyzTItBBkojGbCVope/lCx9y/nKXENEewBzE7vJVhnZ8Ju6RD+UOxr/f+XTT4GcXnUNiC4+emhhdfB6dsHu4eTD6+NaetM2Uy0W6nJFsaU8BbtyYgnldQc1MDNip7aPGkOrWCh+IS/VAJda+tWT0VgslxuMmWEq36wuynyo6ELCfFl3T/6x3gFg0eB/UEJOcgPN7ffbwdn3f+xR2o7TvdsldttPFElRc+YwRuxhSbna3BgXNOw+ybMrMejgdmHB2MBtvXHdoLsNrDZ6v7iYw6TdWhSdm1y1jbra/AZRjfs82jgDHb7uIZ0zQXDf4M6zEcnsHy7vK//uOXf1z985f//X///Z9bpIMDUT6BmynYZIGfRVig4wdIWeKAODcthfMUF7MPYQY2x/eHm02Oy0oKN1X26sltyfInXH0xPnv2iWbuKJhwM9IDtVwwcP3rkXLV4AG3H5/5sxD98u///OW//6//tgWeENyJq1fskuRML5o5Fr1ypMpQjePQTrtatg21CKEgoTPmwy2fYSo6Ywh4TM17SbvIFmFRrYToXIBO6blHfBCrXgJre6wFe1z3lqsGH+H5+ryfhQZUlT057r49bNyCE3iUNSe2EbRKi40wJafwAg3ej4UN0tikNqfBu5uG75jSkmOBUK2htBhMzFN3K27ssbKpg5U6WHiFbyCIqpre1JTMrh5hWntGceOawUecjjz8Z0F7d91u4SRv/wJL/WZQ6cXjtzYdkYUsJ8pWwB7huYNwVD1n/tYwlAk67IvRC/Zd70z3hpqDd4wcYOCtAUGbDqRlt3xYE0PmMSPLZ7yphWOpoYpJi6xvfkCYIXq29SOUVYOP+D2Pw5lQsq/1S1BkmuNxFIeB5zC1ImwZw8b1CFRFsd+qtuDqQJz2ifek+BEDxVSTy7U7w5EiBTJ3r3vTejiALYPn+RNEwSOodlNcVrybzDFv0hBP1+9NFw0+ieIPEJwJ4INkfAGCYiDYTmi+is/jnAXbDz6UDoBUZjUsROBBC08kZi8lWIRt1luFmkbC1ob4LWk3ecBDHVONbwaMFaI7cyV5vJlUonVYPA4a7azSoUWDT0L4IwhnYvgo/l/gS92pK604E6/0e7cyEczxv9EgcUGxihgnPOAsYFzOmmK0Vsvq/66SsUuz+N1KAM1gIX4ahb0suwsRzDeA82IHDQVnGm4InPoZvYYWDT4J4Y8YPAshlR8zHVQuvs5bv+gfb7Vev72+/3TBw+4LnnZvEwn5+AitFgvHZ0HGZtb5qO8pmm7hUt04NOmJqbHYq1TKPCh5P7M4X7oO3gHvdb+QEvs9zpxTRiALlWMkgyZXc3AgIRKUk73P6Bq6avAR0LMQWcaXo/EeprDfXfw1bnlbMTrPAjofyduXwWwDtPeJlJQyRYMaB2YaUyqtkKuS3EH7uxIyZXvxIpmj1GPhXW5N1gWoYNP30vCWWTLQZjkOrUMsu5K1UqG9kxXfrA0+lpTWxcaqwR9wPgeZZbi/cqpv1tD1AcaLwzDkDSh7c+JoHDIL+r1xkg8+rAGAUm1SRCJ4bWP9Id4ast4IOZPMjDWybl8Dhcxu2fVNWk39cDfG0wYtoUyEhspLME5TtBC2mSMV12sm1gz+APICHqew/XB7c3/z6N0vby27OL2/v9V2f/VO319Pnt79cXfzfkNGaDwVcqFGgoqvLG7J2RVrcm+hSNKMTTqccWbyCjFPDunixbDYWGW03iee0V5lTyVQV4CMYrGx43L1ePKzlZ4SZeJgKkZrZ3joVYO/3i8CsQ3LQ/LST9/ZVKXpy4n5GYHFtmzOHhNYb+5wwsDNjQzFYmJUKVpdNXDbcw4/ewzDWokJisb32Pc6/cFjj3ba7tVANdtW4BhICcUnAwKQWcdRRNeTg1YNnoL1KCYnE8ja7fWHe94oX374Xe+GuZjX/+KJxea7DxE2tjh+ntc1GFPCLNVJGaTG1UxsX+fBFWfgYSv7SmR2JEUQ8mNwfHCWoY2FbXud5+UoUiurCXm0kYtnkz5/6JNfZmKGkwWVW88KXDX46/2zECzid+/k4rGC5qJf62/vb+DQ290WDC0f/PHS6UzZAu+po4RUKhdrcyUWx8b6GbuPtWjWDN/AiLOTORBcebk8qjcp7tXlhgdYykJWL8OHatm3F2IaQXwmL9VAMbc21uXNqsHvIDyOwiKMf4/6xUC9vfkbCunNu5+TOJ+fxu7NiVGH3eYgKSeWvTDTQXMPvYLsm4rA6Xry+FbtFXouQcWJm4C4Te0DGtbvlnQ5xICbYpsgPJtSapodTJzzRw4XwHhjVkZcL2haNfgdhEcQOB+/N4cNvAU8DoM7cYE8DXSH5WFCjwwTOnswcKlAKw9nSesbB4YlONfWNIuymMK0DtlS9wJPXWDWFicPKhYTgjE3fA/VGHZANaDOvva5Pixu1eAx8L48/hXk+vjr8jdOZGcWwfd+eFMU9PnEpRZWXIVvBLvRUhDvfIGbyUODi6VXFr9gW/rZ4EendW0AYLpQlnSrCW2vMz74a2VLulBGtS4bjRV7ZzRHEGxIlv2LUj9jTOWiwa8InoZhFcjbcaj1/sHAQ6rjFjS9dcd9adeR1bPGl22bZ0hspxGY68Cu8UYHGMw4VBcmCA3Waps0OXg9aejW7RUOC8VBoN/LcHHGFmyYaBGvZHZWp9bULCj0OpirBr8H8xkoVhDFdqYevTykJF/A4HjfL/4cn7YgGUs+jiSiRYM69qy/GqEmKEcJrjiEQERKBAwQgQhEizfZ6/Bqp4emZt8ffIi6W6VodsZPn5pnfhuoMpuF14bH782UChVkmxvxnI6dawa/InkCgpcguFlkeDDU4/Gxi5MsIRpw8JZ58cW+CBHoVt+syCGbDDRcmxmxg/bkjCgK38QbM93Lu1o/8HAhY4ObmhudRLM9sdI/pwpRG1Nh+cv6Kd6iwedhPEdpfLbBad4Xtzdv3958vL/4QFu3fx1ytC7f37w/jPo+nARuip0xnmioXqHsk7VsKG1skpTYB8ORlldWDdfsPTzVrC1EOKYYWFDUpDkWAvdYdisbzBb6jtVs1UXXmyKSJ5NtlVaKcmKvJVRnlA2uGfwZ3WVkXgfuh4KRg83N+9lac2J2h2tslSnRJutN6M048Fm20MhJauF8HmxpeISBdVMaUE6mWnx/qrH4rb3mJ4QJlTtSR3z3VsV70JpuvITGYu1Ya7GOW3Md8kWD50D+BDqrZf3swHjJbxxOlv7Qv/Tx2Om7H+DAx8t/4Kv/83la3D/x4uP2f3zusPPL4agRv70hxd2dWBXQpBwsawY41fSSS3QpCOhyha6r3bMJs/OjCaeT5uC1Kr47up05ODV7NeWIPgCzQ9u45KmqedCTXS8Oa9dAbHqWM59R9LNq8PsuAK8N3uqiab/r/TfTQK/vvgwRvHxBgw/LB3L8QArQ4xmUKW7YDEZqoBfh+/Ngj0oTjcxSDxWkMTY7Rx8hDjujZmjLkXfrwnmISt7bZlNlsaqRxr7/vvFohemYLRbJ6yth1eD3K2EJkbPg3dpkx/pwonOdNQbc22YOQhDHfrmxAFJuZ9UqJdlgwVedT2n06I22Dhrra60B8qzslkHYNYrTIrEK4s80AZ8iVHdIkmNfNRbTjJDP6Jm7ZvAJFM/sscOWdn/rp8/59FesU7r6eqt3RW2mt/dXbzmMaXOrlhD88a0J71xKMdATnKXGdDPsO6YY1hKyzmq9tb7kwg5gPSoHYgTPYatQljYGt5ukGrW5VCHJW+rdO1VJEjRB2UqF/hFOAkv2DEm1ZvB7ULcAtAr+l3vcHxvgfWP/4Vr3MGXlt1vtL+i1Wk6MV24zDE786D1OU91IFtiH4WtgbX8ThGqD8JZTs6PxdAjKbbjGeD7LTHtlz4BmNetii2RW0AmJ44nw3pz60uCLQLezt2cE6lWD36+Bl+D04rXw0CXosOw+3N70j23cXn0pttk+gTmVcKKmbZgUvKkNZB7KPGB1pJI05DInx0hE4D5GYnW+4+Asbqfe4SA4xa7s1donZbwQbyE9p3Swwz+WZogRC9tyak/twpKJ9auHVYOLq2EJqResh4e8Hmb4PLab2IA73PiJJgwd/h6chUOKR2aLw6KI5sCcHcRTdYgS2CYsCnMCX+oz6FpPGsfM6sN+V06xq03RMd+LZcoNHnywoztWoDMBRAPu25/R52PR4LO4P4HIK+K7ffxNOFG8n4oxntx6zoyV7z1LCmyWHkt3OmoFpcslpzAaz+LgALJo82lWprDsxsS55iKggFrkgEUdOTfQLLDIUfuQnqcNvvT1w5lVg2eD/Lo7+XPp3Saybk/0VPRmVJuyNUOjzhiN54AreDayIKjT6jlfNiLYWZ9MoqMTDoZsvEBwu01TjYivRrLmEubQxJHNswORVtktwtuSJlMV1rn6qsFzcP4GlNdF+QXDS/1xv+3YC98k4f2/1jJCSaDnAXAa11KQqfhtpsNCTeNZSIom12rh0jPbUe7F4KPpJeF1wAiUM6fAKFWhD7H4TJfoCzst2TMSdlYNboH67F39pd756wtc9XrJ37bh6gku8OaQmnn3H+f3kcAnO56sZeGubaw5ZFAWuDe8wVanHwFODwIWgr40OPoxFIx9KPNJlCduJhUE892ayVvOTyzD4i0EFc5Okhmx5IZNrFwDZzAtnJFpsGrwB+y3o/QqCyF+KxV5lfOoE+5esBjEnuijO+DAk582hVoSZ79ZPEN7aN04jNepMSNozBGMr2ACzCyCp3TwC8bvljNkRDKcUBfbakXcNbZWLMGoTvNkD4U5O09bzhhtt2bwjMXwPFKvsSCcXOndp/eNo5Nu8OS2LwNxcsIpNE2mQtxUPAatPfTmIptaICDAh9ZoZs5ueJabuzCcaTPymK7MyFKS3ZyCZ4bTBPWQHCEWUw0Rr8nWzmwfEkMLUktej/2rBtfXwVGAdkH/44dtLsCeEHHsUW+bleIbNnvpOfEYM2M3OPj7UGtintm0GRHBlRqqCGlgMwUSLpa9cl36ZLs+KQxTPnOCT9dYmZNTSqvZMg2XU7XXj3IWDb4A+wd4XgV5e/XAOa5IOq7e43m9IATYE3yglVBqL64VFwuIHnSb+lixHAI2SBOHiCehe4QGqaALBj7Am6wpacoupt16deYYLEewSfba8PoWErNnq96PPqFV2ARwzDN6da4ZPAP+Uxjttgg2uoBcjqcughnb2VpHINTIhqYjQd7JIRCY2UAXQxZfMtP8vXOxO3iFhvDPIJpFdvMAbPvO3EKE6jbYEhHuadThIqSpmcKuWcXpGR5gzeALl8BmL8Ai9s+WhQOy3w0wi4eRVvihF7FAkRM+AIKojGwQ4bsVEEDhjJLcZ1H2zlEDmhzY5po96iuTcSpCQIX4r8YX6XtlV0GcGk68qsVVtRaqxMxSOHVFHCipJB1VUlnvl7Nq8OgCOAOhDfgfLvO/Ma/u8/Tkm4/39eZf29HP6cQhbhELv28ghr0LhyK00psFsnOKZzVB0qgBa4MdhVzO2kGfM9ZIHybJbonLaebpEbnGrMGwZEHYd8x6qvjABoAshpT11LpVg0fBX4bnxdD7q29HPXxwL9j2+UR/nxnB8Di1KrJtGVPtUhyRVSKzh2CnCeD9Js7YMyeODa+xIE7GGTKnQe9WdBAtR5GGxuxqtsAFZcuuItyIjJjwHyiQYtbV36rBReSPovNi3D8fMDw1yXDzEkjmuOP3YqHmY87F5+GDz63z8EfwVWqh+OhABrBffEv4EcR8l6uD4GsZD0zqXjONq6YA+gGZ1mI/tDPDzhXOJE8Fb9MJJIjx/Yy+hYsGF1fACk67LoaNJLC4E+NZVGYzBvs+d+DvQnRaOObCJaANx6+DZ8NlSlHheEbTM3YUGNTgiMa9dECCKOnWDqy8NLROUHaNTAwOIWVEKTZndNOd0VB90eArLIXziOCHP3+7/Nwd81US7CS4Ewl23po5U/RZM/ZCc03YdfbwAFwW68PkUI8EvIWpuGCBEH0QyMZPtXm3u1u8D3AwdrpyVmcaLSj7vsfsilYfsTx5CTHlDM63ZvB7vJ8D4xxQ727atb79agXAzevfNk2YDCdUHBhN87MkNspi3rwzHfvVS4+5Dpt9aPB3dgwjAtlOKVQh6DN+Frs77ubLcyrBcLIl510GzzTOzsQ/4MB3Mduc2mpZ7xO2avBnQI8C8XI0N9/TZTmRDg03LGRnIabarRmlefyxFoN9KhV6vFVXvE4Ld4yYBkXW0oRL44gL6Jq9IAUhFCy1xlZzbN6pTtwMxszM1D+4VvgOQWxYP5ZdNLgK6bl3c09ZoqBn0eJV0+0pdDEcbwoXbUIIZbchD4KtKU11CbR0Ds5pD+yQcNir3nXDLEtnwFYgt2OzVqPbS3dN01LrpohyWnxshyl7IUMrsDpuDBClWMcZ08pXDT4P7hOQrCL82ZdffhkS/JLZlGJOZUWxFSMoAQcqRJ1+AjAf8AtjMDm259Z6d9IgRchEOUKhB0AqcMimIgTvdaKqNkaF8PWcx6Tqsa16F8/x8ZyUE01V08v6Deuqwe+BPYnE2XC+G/e3bMWwJaPRhRMYZuDCa/JqS3BMZSoZQmmO0EbN+FqLDE02IrqmauOhWi9z6JsLLYa9PK9lPYCN2c+e8OALVDmWD3tZQZNpM1JHDnyCy/WGiwaPYPj18W9F7gVNhsyJaaKl9llzsmD0OVgeagj2l7DxB3lfMgLOUKsETv3AxuvAMweWk3iEoL18a8mc9ZBl9JDZwWE0h3jHAYgQWl3MgAQLc6wHzlWDp+Hb7EwPhYeHRHZtb5kRcTO3Tdr29jiW+LyFMsXHBE4b1Jthg2XecEm52IGgWbUa9Z2+tVvN0G/SGlyrs3a35OLuOHcF/x3WgVx3tj5Q8HNj64yRcVq8xhzPaJ66ZvAIlk9j8RqQbt6iyZ7YojEpUGMLcc9Wha50o6YEj1DI7ouaPcudfcUabq53NudOh2GMVS12814pB1kMCXaLh8OAykoYabPguUMwF+azRxDVM+jPqsF1WDdv17+3JY16k06pToS9MJ342NzACgaqCCYFzNeLYvEiGo5cIb/N6BM/KpArcM3Eswxvdzs2YhdAjtoNtiKKaeHwMeF0Bw54iPjLMXJyZxwbrRk8AuPfd9sAu/r9Yz2MD+WQsO31XOLT8ZMgUPJkaqg+x9EUYEsLrozmjWEbfAQXOCJliEF8LL1UNpWI+ANndXe/V0M+X/OIdk7XZrDMrwcXCzzFBxQcoSw5Z1/OuP1bNXgUwifB2ALq/acPvES+H3rL4+LtSb4uxhOdT6J0FxIYzWTN+WRPm+yMG8NgvRZwJMTQyTy4mSL2aGQJV2d3jaEu7DUEBZI+q9hpbRqBGTncUQlOsGQpTawasOx0hgxZNXgc1iNwLCN783b8WNrz5bxY3KHE6+phGOf2UFpO3OP1ajobEYYQBVQh8yCfeTopOZdMh+MtLjDvudg4wJEiGxv7nNjxr0nZrZdCyzlUEBm8sxJm93YmDVC9Kur64T4hm6jrbHfV4A9Inw/PKvIP1fdf67BvP7S7v9rl41RvAHT7ghpcB0VyPINfNJZKAZ7LGCnliQibpA+xZQgUgG819xgFkbaCL7H5RO7Wec77DbvV4IKeN6ca2aohq8v1MNpdnJkGKivP0D00Sl7f3KsGv4f8DFxeCvU3N8F/3GwroXcSTxGs5AfEjZEhPswK2dp6yQOOzmNTT3i4Bg7mooaM/ZwcfoTtc4Zni+ske0XnXvEaiCeVZSTRUH6B/Zmi3VSnvWEhKoLOem3lqsE1pH+G5ZWB3ryvEaZO3MpZRuQIsWBT4KT0oNXNzFRbF4yYkDKkr4wKDupNzRwzro4cpoGOYp3sditnEEvY4Y5J01PC4JxitlHjIWfF/7XRhrFn3MqtGdyE9ivt7YeLXv5p0/FidicqsNg3IZQexggcftyH9dFEqKYGH2fbZMf+DrhZoBQ04zFVZa/MBL+n+1XhdNZt4vG70KEAmBPOE3pvEF7cYLE/3hIH06zv6kWDSzh/B8irgAs5/W5TcI5yop2ZtRC3YU7OCikifcZRS222wUH7lBO0UwgIyGxbBDRjwo+NMHg3a9o0sltelQbbZ8tBYu6pIbJWm9tsyWvlTZMzEe81npFXtWbwDHAfAdkI7peroj/+/vPu8sPH+vb67vdxu81Nn+h+4XoxM/uQJiUUaLcrhbcFCX6ou8pmBx3xWEqI2sxEYJMxG/49xQ63W8mEyZGTIM2wOcCDAoPENnOJPcY0zmoh8wJ8zTr7WjR4EuDjoLwuyttbW4icOByJ1St7oGNTI9hKtbxISL3P6qGigwfWQXzMHk+mOUd2ZjzUCOcnYVfs1YeKRZwQdFn8HLyrQSgFGcBbSb0hjHoOkpAzhlCsGtwC9Qvj8SG8s63Ooe0Zv9h0jinlRJMD5TgYz0qH4oOGCnEcUqo9Z/g360DaWrYlSZ2Od7ScqlN60+4sOWsyu3WgDCzATnGm4WeonXfL8CssyXUJGi8lLTWdcWe7aPAkzE8D8hJ0D9zt8msOXb/Gt+/b75vkVEzHmxykXieUsB3MQmBHWQhiHmE757I2RGB8bQr7G9QAnhXj6L31yBbsKeSwl+dm817W0nXLpkLAhbS+dd6LzRTNYUAXG4itH4otGnwe5xPQvCbiX7MmN+VjuHg8s7VEaXOkMHvgvEXEL7DS4fArvpusTEWZMcXgjAGTKXWU6Bpz4C2EdklptwYmHqINOxHEPoYJsTumBXMCnxpmIODIKKBWekYDkzWD52P+LTi7gP5wCMcffmmfKicnRg+NPrT04t1QHjLMMhDJAzx7DtXkyiHHzQgc/pixZGjSxNTgjohYGlTZbilXLpk4Z5XgfBkswOREwiipjjBN9wCyYcOuZ3WsGnzBSjiK2D7r4zWWhrc2n+hqSAfhIM9aaMrUOw4FjGqxnSwCgINmYy5QxbeyEReKqCkTijyxUs7vpchbbAMey0IuI1hpF2W/Aq0WvxEEylJiU4T09RFyiwZfsjZ2Xha346+bdrC+6SpF4nF+oN5EZzR7g/0fjYAWGmhc0MPCFIxD5ABPNl6lj2xcApfIfjSfrS++7HVpFuNsKnydkr2D1gBHMwVerPpD/7lWp+HV7XpnnEWD56+C78B5OepNP3C4RL/qtxs71EZ7Imm+wi8G6Wmm6aDQQ05zRETN1LzMhFiaQ2a6kYlpmEBdIDYn0InK0/jdKl+xE6vTbIbLoJ2zBB2hI7QfThUy8BLnW5P1o5pVg6t4/wTLKyB9g/d9/V7vb263NbzCRzt+JBeSN8Y69q7x3akR22U4oOzBAcQC2WArj63wQyb3MtVnyKQKZ1D2K3KFrg6e9SfQlRz2raNphNzGG821c6COIWlf73KwanAZ5+9AeVWQX5D1kN2JxuHO9mBa5tRbZxPW/Rg1OibosIyJo1L7MBqt6LCV9THNd2717EH9zV5xXHLJY4B8OkQN61N1VVzhFMjcUwnF225nsOtsf9XgBqRfL2Y/XqsfuOLde/1w9/vN/atwuexPXJkisqlaN0z2UzhJLXQjDR7ecF4Vm9sNF4RpXCLTUid5W0w0Qz3HlOx1ZZp9cG6qGlZ+hFqdLW4UY1IZEW4IoReSQ+368eyqwdUlsIrWy9fFB3dV9fb2ety+juiz8fjRHus5G4lNmZJHdpD5EfE9gL+Njj9GUWetKbHAH7C9BQSTgeuQCubQ514ZisazfwZPobofwToXcnXBZihTINiCZzEuEF0/rF80uLoaTmP0KmvgoUf69YuwT/4E9gqPqNq8Fzm0fBohJPAf4dU59E+pPVXmkfgUTDClgPUjXET+erOx7hX2mZ9RWFptayQZYSslpmMJdrOfYYTG1qpnnN6vGjwD+yeweRXMD97ldhBUmn6N7R+COx4MHPSb9R3sllO1Z0vAfeQ0qOX7bBD11XtW1LEPiAbWN0evYP/dxw5Rt1ueeQvCoRjZYQFzhrn4DrKWcitG4NGlVf77jDzzNYNnLIHnoXqVFcELhFdZBzGdGFINUV9z88AW4tbZWYs6LYH9ySUop5gkk4KGkUCmYuT8vFTZGm82MyTsdQpcQ4IAda7N5Ipn2VKKpduYwVAhPx66VaW+zgtXDZ6xDo4B9Aro6+39p6u3N3fbbvTkhOITl0aCrCutN7axchnRXAtnCbmsnDoQ4DKHzsJT3gZvwNmP2edpebe724QKgbCcRqY/VDSb0FyvxXGAAm/WZ+4tS0zrWe2rBpfx/haS10L4sU/Sq2zyfIr5d3jAgHdVOFFOWF4t4jhSj929E0sb0jA99Dwo8oMrCT8b1bEyRdNet/XOclYuiEXGaqshh9kQnhXaw0KG2H6oALJnzMBeNXge6MdReoV18Llt9peTwk2RPscTXWy7CyOOMVzloSxCOxt9SHEtQvkl6r1pPXS/DY6jl0vAhmFjfzwxJo/vlTzpBx6PryYbz6giuVpwM2+skwEa6s1MPktev9xdNbiM/lPY7AH561zjiD3RzU67Q5z3w7CpifOcIFfwj8059ppzG9P1YWeYXSNcBUdbRGM5y6MUUP/dBsXGqAMOOUR4IEQlZ1ocQAqShG1XrWXGjR3rXVBWDW5fAzv5gdubP0Y72OS0wpvb6/tPmzxB8u5EznwvWiN7o0SfZoljVpboF17jNN56VTsl4ukFwyo2/AsC0TSbnPbi97rFgZzotqdZuxdG5pw6B2wUzpWZs1eTRg4I5uuxf9Hg8iJ4GpytqB9KbZq+fXt1+HLTyU7Chz2Oc3AzqaOrrzJ8c8G7yuIm02qe2nN3UUzrgyOFwYhbHTMVRn/B0ii7nfXGznl44gZ0WJQatIaKN9pDTDl0ZR/VMvv67c2qwdM4/wzHi4C9xZK5245ssN4dRzbLBH0HWnW61nqcpWY1vY9oO6u/y8yFPxurT8V02zqWwbAWXw02et7rvsbEGjkqHm/M5Ao3YtkEu7L/SU7TsiUq+Pb6od2qwQVkv8fjtaDdfigj5UTNMcR2TUw6MAEidWADJ1HWOElq8XAq76MyrTbCKRtnbDBtDp89e0+7sNepDLxmBCecVdlYAwG2O7xm9tOAIlYJpWMPujOGSq0aPA/fl4bjb6bP/ZzYt/0UPtvjiRWxmOYNIjHYqS3Q4hDcBmA7xGPnnE2jSe7qRTPza7H4NXAOuEUo86DreymzhHVaaqtQiNGqAV+M8DLTNmdtbKkn/DnqeoLtqsEFwJ8D6FXRf1n+ZXbHGbkHK0GsjV0gsAMzp/E0nKm5pJI1lWqgw2zE/rYQs3NQmxUOeXbQayHuFaTd7K0lMGWIBY42m7FM3g94bxyCCZxNZReZMzT5osENyL9G/uU3hh/P85revn+ZFIN+kuMsfBS2izXMv5HYuqUet82W0EyILjSIbibbwxnM7kOpGnpM0hDgeT27W40yliIbBhQ3izYb06GtffVFE6fWWvZDGCb19VrGVYPLwB/D5xWg/9J0bdt5K1j0cbilqXfiq4/VWSM8hWBFcskgqZzWHp2KkzDVSlCJEWwWkoXNjZSyZa971gh9B2cyxQTnE7t/e470tcW4OI1owXIMeV15rxpchvtbTM6DmGU2h3f517j9tP0GNZUTndQytFXFTu3YnOqjyWy76mpJJnQ8g1zB2eahrtPm0b0y4SKMjk+Rkml7nakF30CPxQcOGTcu1xCldzF49s7X2lKuqYJGrxfGLBp8CtUnYdiA5GEw+McPV3h7Lyhnk1P5rQ2UizmAzA3q0IwRqxgspRkwazAvVwC3640zoUDIZxcO9Z1V8DecCbJXb0rocd7AYy+1PrGUco7k+dW1UHhnpa2V4ft6BvyqwaNwPonFeZi+25iw6liAdHxDwst0SH8PHMGW2eJm9K6xSMnCSsSOADuGF8DYNdjskg5n4Wp98TXuRaQH6yOmhV9UdTyimLkbsS4HCLlZcy82+JDX6dSqwacQfLclJ/XbLgIv6A2LVXd8+02by6jOcJgw+GEFOOOhlYnl9LnaWFwALwsSlDhxKabqUoZkMH6YMvc61qhjzmR0gKRxLru3rbLBNHcNiz5d0+k56Hz9EnrR4FPgPY3Di7Hc3rAjJH+iuAxqBp8Na7UcUosM/KgtHCsizLC3JqnOwSxc1Tocj3ugfcB7cufImb3UjWl1ZFfzDNk3h3/gz7ObHHqIuNw6XAb4iqxfM68aXER01aPO2wM8/fLtdb38eI2dDQH8Tv/kdN7D11e8oHj/26Et21v99KbDbW8Ime6Ex40umVkhYuGSZq85xQIhrzI7c0cMz90DQmaaLZLtevIHSL7UHSStld3qB8tALO/M7DfN4R0Um7P62nrLjXffAa/fz8gkWDX4APEmZJbhPlQWP5i5+4+P2q8eezcdls0mjLFm7fFDiuTB3x12L+C0DYyBNSHsOi9Joel9idNVhcBjjaCA4XoZrWR8pwSXdssOKnmwvUCHRJIhqfaCJadYWaP0aLU3b2bUfIZWXTT4A8bPwrEB2Ie18sMqeQHAOZ/oqyTBIKSyi0Vn1V6csfkcZzXA1mXOMh3gv36Ic5HTAUF34bhjg6Jnr6W9UsIjpOMER5tt6ORZYQnaDNaaz6U2w1yGIjmtZwGuGjyK7zOoLOP896iX8+3HeyaU1Jub+7v7W/3w5o+7DeOrjtfziuc4ExDCOasHXhpasAUUw7vabW1eSxwW4nx2RYiKTjw3ehSeTwTdC9WQSw+R7auSlDGxmA7FhHY2vKyrY8ghZeeMxoeLBn9A9RgGZ4GIBzH+9eb3+3cbxg6frNNJDlBYHi5w9LgLc1rDuxHSRcvsReNCVC/Q4GyugzWdO/CkjItzNyUaTLF4uJ1tFA3elU0Om8hPzorPzCvx7H97RqrWqsEnsPv66H9C7L/88MEP7+AQyFxnalO1wUzuhAhHNwVPUxKi1TQWy8ja+Zjn8vj7n/EDpO/eXd8fjsezVZBY2yA3YzdTkjQRTvysPpYGrjfVj2+6E/wblsLt9fh+g//bG/0N378c/xrtw1t9f/dYY3LxwV3c3Xy8beOi/T7anx9urt/fv3nXf1xiPzzXp5fsT69x4JsXX47PLz4fzVx8PUff+lK/Xd///rFe/n1z+yebAN9dtus3n95tGceKNXK8boH5yR1BeWJTQDfMUQczlrFHovSWKnvGzVTh0/FnOncSthDh67FJqt8rjxF6JfUYsBRABB0of4YG8EGNsnRCwRWNGQii66pi0eCv98ee/Clf9j/1zzGv347zJR9r4U9kkjup3XC+i5hWWEjv3AgtCoKsLQefpt05h/1GHZiKm60SUITfbPfyXBP73YOrtNhlago8SgA5bdDcudo4OOnCYPuud5FYNPjr/ecHfQqMfgNJeHvxUFYy3hyU4cXDTj0ow23bKIB7nxiflTuYQLEIMBGOsHXBW6euSaC9vodkwBOqUZMnB9qmYqsEqFztw0ncrfrHeRbkFCxuasSggX3kPZh5yNOkYcXzaGBduK0a/PV+BYTnQfySLHj5aOjioZ/yuOj17s3d7+dzPHfiBqIYRJvOAOqLKyxw4swTzdMl7C0O4gENKuLERCzTINNmqDON2YDnV9kvpxtRNmcpMc0R7GggKMNbGdZ5hNuWu0tejJ6R071m8DOKz6GwBcfPXdEvvs67fOD+Hz5tOAg9UZ0xbZ0JvjD7bsEtbGnYh7Z1awIHLIDnmRoa+9QMz84EqRU8FJfoh1iqsddBqGeFOMdCu0PaMHvC6cD/ZW9AtW3Cl3T/6wehiwaPg3oCknMQHu/vPt6Ozzv/4g60651u2asW+ulU/UWBtC7WgPVmZ2twHcLEhA4uOrOaOWaacXQwGranD5WTp2coHnQzzr32Kgh21aFJc23NMEvQV+AC7t2zzaNkxLPu4voxyqrBn2E9hsMzWN5d/td//PKPq3/+8r//77//8wn2utAVL5yYRcheUdDI8Da87TX8NC7VOGO0fvAWsLK+BFzUwP/GXiviZ8stllDggO1eqRqHKb0N3oBpNcbx3WbDsaaWFNH1ojNy3ud61vyiwQNuPz7zZyH65d//+ct//1//bQs8IZw6ga4HOYx3lwu0cOWIzKEaBzyKMsOkTsfe2SChk3V+LiaTgBJjCHhM3a2hAfa0L5H5HNG5AJ3Sc4/4IBayPXjoEmvBHs+oYV00+AjP1+f9LDSgquxXc/ftIdcWnMCjTgziaCwkaLERpuQUXqDB+8VJPpbBOTntylrb8B1TWnJR/awVeygGE/PU3WoOe4SA5MilOnioj28giKoalkaW3EHtw7T2jJrDNYOPOB15+M+C9u663cJJ3v4Floo/jbs7/Q2/evH4rS34pRDMifJBhK5ZTXYN6o0d6aHcoIA44rGAewlnakzDPqnWQXyHCT+C1VlDtNX13UaycraLV5sGNoaGnnNwNYg2/KFkF8CMsDugZNZ7wC0afMTveRzOhJIjWV6CYvRJjl/JxsncJPiOOPLk9CPEZ5FgG3SgckYib7RMAisxxpboJgcmQyaGepiFvtcubFDQVegaxpzqg/Wm+ObEllxMCs0lEH9T1oPZqsEnUfwBgjMBfJCML0CQjQpOpI6WOn13zoLtBx8Ka/pUsDMHG7giAPBEYvZSghX4mhk11MMYAba8Kmk3ecBDHVONb6xagOjOdfBCoUsq0ToHnw+NBjlzxszHNYNPQvgjCGdi+Cj+X7ALU5Hjp2TYgU0QDlJI2I/DJGOySS1Hj2A34YZzZCvi5EA3C3WtCcKL5u69qb7uleoSwujRSA65tIqgpeC7MSa2RGiTc2+7r5yyvn6+v2jwSQh/xOBZCKn8eMOucqH394poCogu+sdbrddvr+8/XfAU/ILH4FsQ9dEeP5O2nI3H4o2uibsx45ODeWoZvC1V40X7qElD9COGNKDwuwljVJ3TxbjblLEKMjxKMEM6FGUPcAWcyoDVZ0YerYQm05xRXb1q8BHQsxBZxpejzu9/h/bvdxd/jVteY4zOs4DOR/L2ZTCzU+Lx8DkyWx4mgXLH5xyHZDXPOyvoD58rHgRA7y20MBgx2WcA4Se2hO+GPuNuo69Hhs628Csx1AGpULIJbHZjzKiREy5jztadMfp6zeAPOJ+DzDLcXznVN2vo+gDjBbvwbL2v+nB7c3/z6GYuby27Or2/v9V2f/VO319PHiP9cXfz/vzwzXSZ49WbOn1Q8RXip+bsijUZC6ZIgoBVhyXlEAHmZJMiY1KXYcTGKoMjVkHV9kqKw87FanVFXFKOs6ieLaFawTanXhngEcwLXg/fqwZ/vV8E4tSCOW7ikL3x03e2DVUvJ9qnB/VDY1HL5MYJZW5BNqODw5BkmFFUtCKcQ8RPKAjw7zCsFYR4I77HvtcxBB57tNN2rwZRyTbIbBAnW7CDjZnsiClaRNc9w6rBU7AexeRk8kW7vf5wz6vNyw+/690wF/P6X5TOmw/hRaw90RSrdoUIopCt8PADordVM7F9nZfpZxAXM0vrc4OUitDAYyizeGVoq6PvdV6bcxSp1UMD9K4ssPGUrBzRIgWkkWouwGuvHywtGvz1/lkIFvG7d3LxWENw0a/1t/c38PTtbguGlg/+eB4NxJ6CZ0LYlpBK5WJtrsTiOLUoY/exGsea4VtmwrlM9r7hLeeo3qS9wjUJYlWQQXD74UO1fVgHVRdbnslLNZBurY31M6dVg99BeByFRRj/HvWLgXp78zeo+pt3G7LYsjfHIZRuc5AEUtai5ZW7ZjCpyjPpisDpevL4Vu3V46fcEHETEDfwrZHhlHZLhRpiLKi7rwjPppSaZq8s10/zcBOJN2ZlnJMKtWjwOwiPIHA+fm8OG3gLeD6KOXGTOY1j2qEY1yPDhM4eDFwq0Mrj/xX3LstxJUmy4L4/o/dMuPnbly1yW0ZGpGfmyq1a1gjE/JWJTJJgA2BmcXH/fVQDfBNx6HGAU1NdxQZBwCLiqLuZqrs9nM2RzlJACnjy1DQLJGJ3pvVsbT0KPHWB6UOQ2Y+0nMMjR+2hGvD5PE1p6muf64XtqwbPgff58a8g18efV7+Ot+N0nf2tH94VBX3euF3BiqvwjWA3WgrinS9wM3locLH0qiEIJ2ZP6J0+rWsDANOFjma6mnDYVBj4a7j0OENhrzeIslgnmzc6gmBD4mzckPo6rVk1+AXBbRhWgbwbp2rX7ww8JuPtQdPbjcYxnVPuPascDSTcDKBqKQVeujtFHNQBBjOAZ+4JQoPVqiZxDC2Ie+jWHRUOC8UBO17YDBdnbMGGiRbxSmZnfV5NzRoekixLj0WD34L5EyhWEMV2plC9OuVtQvfej7f91R/jwx4kY8kbzZ2GNKvK83sdoSYoRwmcEJx4fIGAASIQgSi753gd7N7py6wVEgUfotajCh05Wm361DwTrUCVxyi5Njx+b6ZUqCDb3Ljg0nPV4BckNyB4DoK7RYYHQz0fH7twzl6IBhy85RagEBPkItCtvlmRU1oTaLg2M2IH7ckZURS+CQx96mEN+fzAw2VnUDc1NzoJFl2y1jmnClEbUzFZ1zs5rRr8OYyXKI1PNt7qw/2ru9vXr2/fP7x6R1t3f56Sha7e3r7lP16fjqR2xc64MerYVyh7dhtMs7E6gaPnvTrSckgrUITsPTzVrC1EOKYYlJkj0hziTemxHDY3M1vouwZEOLC2N0UkTybbKq0U9bZj5QGqC+Zmrhn8Ed1lZF4G7ses+pPN3fvZWuM2ctObLVkk2mS9Ceyzq0wZUpOTsGNT4paGRxhYN6UB5WSqPR0aG4vfOuweYELljtQR371V8R60phsvofVZY6ycWcytuQ75osFLIH8CndXCZjZkvOI3TidLv+uf+vHY6Zsf4DTtq//AV//z0yjev+HFx91/feox8vfTUSN+e0eutdtYFdCk3mMxDHCq6SWXyE5YoMsVuq52j80SnR9NSpCag9eq+O7gCPTg1BzVliD60NnRP1FEU1XzoCe7XhzWroHY9F1FwnpjiVWD39ZBvzR4q4um/aYPX41av7n/PKH56hktDiwfyMb9UZc0kgvJh2A5ZBMOgVM1AodT9VaqFvyIdiabRDgFw0ZQUxAsLbZTO+xMMWMrs9GYj0ldDSNnNqLiGL/CvvCKtVi9XnCmuGbw25WwhMhF8O5tM2I58+A8bQtw2Nyh7KLvScOTESzzUqxaZU8VTsaccIXqQp8iLhgT5khQ0R2S6rAMDFuLmNk8S9N6L3xvwQQn3kVIATIRRuQLMjDWDD6B4oVdRtjV6y/98Cmx+5oFM9dfrvuuqc307uH6NWcw7W7+k2zZaMIIMmpyAQ2LsQCvmjxCt5wKE31oORsIZtNC7rEl70Dc2ZetWg+eU7CRDyuIqU44aDOyd94MvrJ/fWS7yFCgbbM3wnaxFxTErBn8FtQ9AK2C//mC9/sWYF/Zf7zvPU1d+fVO+9lFsHTzu/Tij405Tp/z3d1tf9/G3fXnMoMjX/7xAp1X6R+LXP81L/Mv+kifkv3/da/0ch/sc5HJl1e+7vWKq9yG6ycWzC+n/IT7/369p8pya4qj1FQCZDMi0YjV8GBWwmCTHXW9Q47Z4U3NrBh1RuIU15xXK03VHjatx1rhRU3w0WUZnnUDIDaC92J1jpmZAuXqBYUkiwa/9VTPQGnZYW29RPzaLfLY4qPvun/GYvAbE/wovqG1swGbDODXkJ3RgGaPksHFp01afRm9zQyJWRHfusw43RzqGsn9QRWALvfqYssIkGlORdTsNblio3PstQH2C8HY19s8rBq8YDH8HKmXWBBOrvX+w9vGqQG3eHJnl8GL+qMfXvX9u+Nf014/ut9r+t/rt1gl9/+iT/vUK7/8J2Ze+KeXFI7+ejOwbh7bEOOHnrXHvd3oM2qtj6XEaFsJ7DI1R9Fqu3fYx2x2kkzVVHJgJsoM00UzotMYa8eWGEed9rfREmcDqnecq1KHl1ImPHIBuYSmjb4FvrH1pP1Fg2f3+AUA7djap1OJr8yr+zQV6vb9Q739537wg5y/6oEWERVb8TR6YsGhj8NBPQN6b2xVPJBiLXzhjB2i28js2ZnpWRzQVI7SJcKjv2ZakexTHWxVnIrxMkvynbOXQx6+pQtmNi8aPAv+MjzPht5ff921753bD7zEDeR1DOiwqUydCGBAISTO+AheXAD2YwSFHjWzN7ypbjXkZixoHs+VmQhzFPIcrCIp1cD9yXFCRnsONfaeOEi6lq62hvVjhlWDi8ifRefZuH9ij081nz8izq2/+gvFund//Hr1qab+RU5DJbgNIeOtmTOxQXzO0zfwUfaqaBZOAMvd+jBTaJLKFOG9yVBjeHDIK2G1iHZHcdcmo0o3pjnojDRaUA7Li9kh6voIpplS7nP9FnvV4LcL/GdgrC5n2rm/bTf6+osVADdvft3VED2UjaYykpqHwzbqCi85HcvH4a86BOqwGQGpuWTZl05cTAhUpUrFn7FxIl09qs19TgyiUwrbswdvmKMlJVngwHfBonZttay3uV81+COgZ4F4Ppq7z0WzbNxdBZa3QHfBR9duzSis14pgZI5Ni3PH5+QoOejMZqDDakKMnpVVo6Ss7ihIOTIFS61RA7LkX524GYyZ2bKKfNB3cOTrekOnRYOrkF56CPqUJYoZZphdN91ftRA3WltEmwznwLbgQR41JZCNlF2Fvui1B6azn/aqd90MPBhnkk++SGwcCn5YTfc0LbVuiijexYjNw1O2kCc0zfBgRRJNrCOtC4tVgz8H9wlIVhH+5MuvPs+0eE4rdRGzkViUY+U1cvYudXw2OCdbGmdSjDKtdqn49WAoGgo+fWPT7QYhOYMmbf6wG6qK5aRVFNEuRG86xw2ITMR9BwXDtiklwnlc0EJo0eC3wG4icTGcb8bDHfPm94xrdRsTe73xLRtpE3LI0jHV0dNg9xms4pSZSgCSnKVBE8SktpgIN2XLsD2RWhyVPB0igncxbaQc8bB7lIiNlW2EMk1YV9pA0C7YnKsGz2D45fHvRe4Z03Y3LomjwUcqM4EjRGv96FWScHwFoqb0kYMhkW19FG2lwdm6HMBwZ5keWu6wpA9nstXkpFoQudZzbMVUVquzRTS8IxDoCsm5Tm4XDW7Dt9uZnrLETreO2l7zSP927nSnYeOQvaaYoEQSC6Qgx33J4sHdm8tmgOJG7Ec2suySHXS5Nbal7gSUKbLbxmH1+nDzbVr8kXkRlGIUbv0eZvbZQVFM50qf5YJeymsGz2D5NBYvAenuLRpLChvZOGOEwj6Q4DXeD2AbZQwHItTVs1NpTu7UR0oaXBTEN34zJ7B7EKDojjpXNaWkNBCz2RG1R+1YVsFMMBevIozZbWCZrcO6anAd1t3b9a9dcdI6FzdKxBSIWe/w8YrmaYcdLLhnijRUZXYKnjdcUvGjsh9FKrWl03FTtbYeJlFCT7VZDhRILo7ioJNCmNVyKJFawOFaYjfv9d25aPAMjH/d7wPs+rf39dTsnq2FnzF5y29AmFWrGQmuVWxOvfVYBtwOp8jNUwNa14MFehkLle18oFdAjuIsqjH6cBSEPtbMrqqSotSOd1JMTiaFGr2ztUtkxVeeF5QZLRo8C+GTYOwB9eHDO16gPQy949HliyVJ8Arm+1Saz0ea4k4ZPNePjc5f7DW/mx/IVNH7P9vVxzkcWGXPGNXMoRcbxCBbFkCazrQpC2kZOQ0O7hRiMwgIbTJcy6BIIw4bmAo/pKoVdrwDlz+MGEDMVbBnKL0yaxoG78dE4yrbIbJoAVJfjL+AGKwZ3JzruIXLzvGdn01+deL/++3OnM9SNkYyJOOGhFyKdo7fdj4YhbOajtN4+eElDbACQWwJroESwke1MTOLWxGYjmqcVmMA+4botd1Q/oaY/Ei+Buhf9pKqCBmSdb2x+arBNaR/hOWFgd4/uQxQnS/didi01pAEJhC92USnYH1TiHuZ5A0mWBumlphcw56vkHcTonUUk2w/6mCsmNlSa65pm3AhkR2g4VRSAtfpljUGySZj1kt3Vg3uQvuF9vbjLRj/tkujQ7lsFKjHjA+JNzCkeGg6EMioiQfcHAgC2jz7QGDO+A+fjA6tvmqNsxY4g8NGWynPQNg+OYPPws2o4ZDHyvIQF4IWECS4FnPB+LJFg0s4fwPIi4ALSfFmX0a3bI0fPKU/qo3DiUiOzmQIvVa6IBTH6tqMJuaMMAxxlDSebuTaHKbGGFWPOkCbRZqJbKJm3Rg8OsgBEaKeej20EKpnLLlgxvaqwQvA/QjITnA/H5dzWuzVu/f19c39b+Nun5u251VD6my4NLLJjhOIIfvYIhmytsams9fGk2+ELdHia9fcqsOeLuKCtgjldJSXFh2us97l1EMHyiVyKlMsVCrdaeGsyDHXhd+qwU2Az4Pysig/4zRVztOvCPapbNqDuI1oLNViR5fU+6w+xB684+g+D3fOJFHnSuvFeNddc11jPqoZkxQogBZ4HDhHc7U98iO8FYhYV5xn5zO5oGvaqsE9UD8zHp/CO+tATnV6/GLXWQ4E/vmeW6Dg3bE7aIDEgMZXnaGx+7J1jSXj0hIjGCi4B0s1A4/G8/RDBD809CiYe5yNEyom55lWNaB/OgASp/0EB6ACVmC6YAT3qsFNmJ8G5Dnonrjb1ZcEo36Dbz+03w4Q6U++3peEpn/1Cz6eS/CHX7rC57K38f/XO7gbf96208se+4pN37GrSr/udztLM4M15ymf91lPc1MUyrsmhAhtPZnufTRMi2UTMRtB9QfchpeRWmzVGIQM5XSyo/h8t6nZkiRg36th73XQ0SGjgcAilkFrnuYnr997rxr8uf84A8vzvUi7xfu+easPt/uIH4eRbpwXp9yb5K7VhASuzrZwA6/tEPCbkTFGGGyCp4PzFnrtvjiIAVJ/hImjmF/u9OUm6HSitY3e/YyxQ2UifCsYChP0EcHWE80WDS7j/A0oLwrybt7nJG7kn/kIcZOMKQmUV0I1vpZiEuigYRmYuOwjEOfADUCemMEVQR3miJB+yclRd3SWaX+JU6qsnbl7AxLiw5gWzKXUMX0AT23rIm7V4A6kX4L3PRr9eG5/ipX3b/Xd/W+3D//ikPXOXVe9u7sBnV144Z8vPus2RijPPDjgehqd0kHUpM9seWIUo1XWJCb2o4LWBpBQI4Ft1TjS0rEZij8qAb9EKL7kfXkccWET3onYGV1InJHHxuRs0H6Bvlw0uLr2tjF6/jJ8x87BrEO/eRb2yaWN5k1hsnVj764IgkcOUSb74mbDnI/o2dQnenaw4vBFwx+3jqWqWszI4ajDIzFhWMO+IRVRrcXQOE2BrcRH5mDvKU2HvaCTz6rBC7B/ApsXwfzkdu4GQaXpl9j+HLWxMbBPZ+2GiDevpngbTSC/sN2CMppm+vS813aqjXld2lXaaK0XDuI+bAm0ZDmH1bALx4hxzD4RJRAVJUX2GoOAzOrWQ8+qwQuWwM+hepEVQc37IusgQgOcXwecISvZg1OyLq3FCvYZu4tlYDGAc5tUK3QE3L5NiAbwG/AZ4CgZO8rkw0SF5DaZnmSsrblpTpImNIGPrIOukAjG9lnbBZPL1gxesA7OAfQC6Ovdw4fr17f3uw6hSjxfxMIkeGjC2VwUDcMPBERTQ8fuN747TneeyQrkRm9ZCv4hJ/iEGQp0pmR7WG/N4gPYruk1QhEEyeZ0K2Wy7yW3NKWOGvwlvTXXDC7D/TUiLwXwxwrOF9njcGYb0wnbiIWN2KA0Ymt2uGF64CU/ovkUN1PnML/gOGmRnYI5kxeKMxuWDhyWCii2+dkbJE0eofNWIzXQEqa7gZo0DkmU1tYTkFYNXgb6eZReYB18auPy+bBqV6A3ZeMaOPG2O0dw9+obHoPvoP7ZsxK5OW+dra2yc1/1EABumil0hxO/otObwzIIdY4xUushdztyc4l1Vl7xPoAapLAmD18U1sneqsFl9J/C5gjIX0brRbMxwis2o84Xo1lYV4Hdjw0i1SQZvYzkEwQRfIE1HBoirVasEE4vTQXe0tmj2B4Hn1cZvprSGwRZ7NAcnOqnPccCd9Sq6SAf6714Fw3uXwMH+YG7299HO9lkQ8jbu5uHD/s8wUa7exBdSwokvGFS9lNqvjfP5u+2CFQRH1BsQxnqQYlZlaOSR5RkoAGOcgS9F5P6CFm6JJbW8g2mNJ0NHGKMP9TJRbMVFw0uL4KnwdmL+ilrtOnr19enL3dt9hQ27hmD9K6gbsHKwE5PlQetKkPg4BNUHGR+yaW1FkDivWuzhDRHBCdMCBHHSbvpchbjeKBtOIUmpFN+oavNe3ho9UGhPtY53qrBbZx/hONZwN5hydzvRzZgfZ7PCamusy5bOQ21TR+yGxlcx8C/+aEpSdGikW5OQH4QzPXUFd8K3L7YclTlo7eh5ZmbGUChdjMnvmy+ss52sIFV0hTcBefFqwYXkP0Wj5eCdv+ZjIBmb9z8FBA1iGwDClsLK+qyVegzm0TYazbYMGOQUxdi68FxfZn4n+3GjVmPEuMj9dChCgbnSfUWagSbsuo0JhAFMTEYw2kb6+ps0eBl+D43HH/VGfPHLIHDTv+ffNVDcwW+esWPRxhN794+j36Kcxvlom44KI88EvyxAffQqIg+PeK/qTJUyyxu9GFabIjQtrPoshfO/HI2+aP81sye+e6J0+FSMx6uE7sMotdn60CG+rCFXRnXkxUXDS6s6218XmCJfy4P33XckDfqRqqx8N6afZycvFPj6D1waGDLHJ0Eh1ZcKBCbUUNOM5dUIweGDt8hSc1ReU5Ncoi+WjtcsTNpU+eMyz3D7RRE3RFPNQ0XdHRbNLiM9teQXIYwc+FO7/LPcfdh/51RKht9+qAba3VaOjat+mgy+8O4WpJh7wXJdQi7xmB72zy654Ql5QRCw6kv7agakeAb1pD4cAoaLtcQQX/FlGacr7UlJs+CBa5XAy0afArVJ2HYgeTHyWnXeHvPyDkFZThPNRonZbmcup2hjzLjFN76NR6Qaq6uAG7XwSMLh57OLlaopVkBNsIoR7WnGsKb5tBA/thqXnOOvG6qrgW2v3Ccqgs/sZ6MuGrwLJxPYnEZpnsb9bOhwnk5r8bFnrTDyTjodWGlLIRfLFKyMF24l9DG8AIYOyR+dhwPZgVBqvgaj+rBMCoPYS1kmKpjIcbM3Yh1mdfHE1q02ADlsj48ctXgUwi+2ZPw9XWpzzOa2PgSzm8/E4VNTF1M0OZVXadow8fModVm0yjQ4aHZ07w6cTaCVsxhfXLZlaJy2IyzGnrIFj7B+9nAYnJjIzMpwdZop3PWhTxFLwBvzeBT4D2Nw7Ox3O9POdfq/HZ03ZUW2QwPlHW0Yj2bENUw4gSIGXB6EFvJTsAZQuLYSZbkdGnd2j4Oi4/ipzBQO8fxRfDuHqsqIcaNNODk2+h4RBcMxFo0uIjoqkeddyd4+tXrm3r1/gY7G6Lsjf7BBvmnr695Jvv211P9+Gv98EuH294RMt2Gx40usUa0n9JPe80JNK8lldlBDsQw3T0gZKbZYk0le/IH43rqrk1r5bCCujIQy7vngX5zHLRjc1ZfGwg35FTPAa+Pd7F+k7Jo8BHiXcgsw31K/380c//f77VffyywPi2bXRi77O35w1Nm5haXjQectoExSE+8CBFJmkryJU5XdRYtyXHuPbYD9nrGd6Dv0mFFkwXimc0xYuRBbqq9YMkpVtYoPVrlMf6MmteP2FYNfofxT+HYAezjWvlulTwDYPZmOp/2GIztCStYuinTxRmbz3FWA2wRnEyBZIV0HWyNwPQX0N2kGlvRVpkuedS4suLbBEebbeg0WFgID81grflcOAFgQGNKTuuH46sGz+L7E1SWcf5r1Kv5+v0D79Dr7e3D/cOdvvtlxzDuZDaSWT37roIQzlk98NLAwdygwszys7V5LXHYZt1EbOojIgZzo0dQK/ya5sNaVZQesIw4ILOMicXEXEtjZ8PLujoG2GwGn72gVcWawe9QPYfBRSA+zuv57eHN6x3HRRvNJ3JygMLycCFUeNUwpzWsbO+dDX9mQgwNUb1AgyMQWazp3IEnZVychynRYIrFw+22nKZkBZscNpGfpgWmTwzjS50mruuYVYNPYPfl0f+A2L9998FP7+D0WPEMnZPB1sRgJtWBikfQUBYRBYOnSpIi9WM3h4+//wk/QPrmzc3D43xH611q3s2Id8e2EANclo0UAENplpfrQOjLsdm/Yync3YxvN/i//6K/4vtX45+jcQjm/cd0+1fv3KvH2ZivTtNw393evH345U2/3DsgaJ/vhJAlcs6OTbZl79nsw/c6RxQEbviH6ocy3WWMMvvEQ4N/YHuMdur0O486kUxz8rmaJjVD3CaOEAxYE6nGxL0MWmFBQ9YPO1YN/uPhQji2HMUPpk4s/9Xny4xXnw7EXn251dgBcIn+vPhitodarTXnaqbv0bAHuekyjKk+AFks/WBcwz9Kcg4k3vhW+ygpzN7GYSVTII+SrWtZsQCTCRB+MYCDnIRwNk0RmS8YQrpq8AmAl1DZxPnXm4ff3terv27v/mCDrvurdvPLhx3RQODJzcaYYTtrBw+diAOq2KV1WOamzBmlt1R5qzJTBY3B38lnqFFCBL2BQ6r+qM1qdCb4jSQN2seN3tmAA0vLaClgjXAXxgzwxvVsxUWDwPLMk99C6//SPwYHge0ZzuY3zq3USe3GuTLFtGLcQJAZoUUBr7TlFMa1O+d0Ys8l1gvNVgnoYH/Go4L1nHAPoOeNA8Dg99RmAz3WWHLOnOYxh5pq1hPJVg3+4+HTg94Co9+2P8bdq4/zJX85HYa8etyQp8OQfdsoQG5utLbPHeS3WHCqGGNj+9aYKeUTlJ7vIRn2dTJ66kAoIxVbJbBSoiP6RX/YxE+P1TILFjePRYKGKc16iFF2Kk3DihdntF8w8XPN4D8eVkD4OYifUwI/TQt99dgAcLzq9f6X+992FGptXLoV0ywYP6uwimNvxQ55Clk+XcLealoEzL+IExOxTINMm4MXjdlA2lY5yhc65lnmLCWmOYIdDZx8eCvDOk9mmbtLXsz6MfGqwU8o/gyFPTh+aiD56suQmk+j2nfkYebzW3PaOlMmE+12cm5vY7p969YExPMKaWNqwEZlvzLeCqRW8FBcoh8qoR61NauHRLAmx+Em23e06gszzSq7P7J0F1/S/a9321s0eB7UDUguQXi8vX9/Nz7t/Ff3oLZvdM9etdHGjYo6fMYI3AyEXna2BgfOOQ0zncrMauaYacbRwWiwfd2plQMra3y2epzIgKasOjQpO6QZa7v11IKQmz3bPAoYs+0uXjC5dNHgj7Cew+EnWN5f/Y//+Pt/XP/t7//P//rPv+2RDg5EeQM3U7DJAj+LsBjKD5CyxGF8btrGpFlxMfsQZuAgAn+6ReZosqRwU+Wo/ueWLH/C1RcehrAnN/N0wYSbkR6o5YKB61+PlKsGT7h9/8x/CtHf//Nvf/8//+//Yw88IWxdutTTCVAvmjmCvnJ8zVCN49S6vFq2aLUIoSChM+bTjaph2j9jCHhMPepoL0e2Y4tqJUTnAnRKzz3ig1j1ElhHZS3Y47q3XDX4EZ4vz/un0ICqsv/J/dfnuntwAo+yZmMbQau02AhTcgov0OD9WEQijQ2BcxqcNdTwHVNacizGqjWUFoOJeephhaQ9VjbQsFIH77HwDQRRVdObmpLZQSVMay8oJF0z+BGnMw//p6C9uWl3cJJ3f4KlfjUU9tXHb+06IgtZNkqEwB7huYMUqfTi4CBDmQzFHiS9YN/1ztR6qDl4x8hhEd4aELTpQFoOyz02MWSbemCpkje1cAQ4VDFpkfXNDwgzRM+2foSyavAjfj/H4UIo2UP8OSgypfQ8isPAc5haEbaM4ZAABKqi2G9VW+AFQR0+1RYjfsRAMdXkcu3OcHxLgcw9Kveyng5gy5h2xAmi4BFUuykuK95N5kg9aYin67mXiwafRPE7CC4E8FEyPgNBMRBsG5qv4vM4Z8H2gw+lAyCVWQ2LPnjQwhOJ2UsJFmGbtW2hppGwtSF+SzpMHvBQx1TjmwFjhejOXEkebyaVaB0Wj4NGu6hMa9HgkxB+D8KFGH4U/8/wpQ5M8LwvnSlMtrO0MhHM8d/RIHFBsYoYJzzgLGBczppitFbLTgtdJWOXZvGHlVuawaYHaRT2De0uRDDfAM6LHTQUnGm4IXDqF/R1WjT4JITfY/BTCKn8mFSi8urLbPtX/f2d1pvXNw8fXvGw+xVPu/cg6uNG/az1pXQzbelwL9iNOYQB5qllMEFAjRftoyYN8aQeBhR+NwHeVud0MR7VLClUkGH2Rh7SoSh7gCuoY5baxIw8WglNpskXJHctGvwI6EWILOPLMYSPE+/vX/057nhbMTrPAjofyevnwWwDtPdGJ94yRYMaB2YaUyqtkKuS3EH7uxIyZXvxIplj62PheMmarAtQwaYfpeHtdLNDm+U4tA6x7ADXSoX2TlZ8szb4WNIF2SGrBr/D+RJkluH+wqm+WkM3JxhfnQZP79nMZuNoHDIL+r1xahI+rAGAUm3CJm7w2sb6U7w1ZL0RciaZGWtkjwQNFDKHVTI0aTX1090YTxu0hDIRGiovwTi50kLY8ir/gvqUNYPfgbyAxxa27+5uH24/everO8uOWW8f7rQ9XL/RtzeTp3e/39++3ZF9G7dCLtRIUPGVhUQ5u2JN7i0USZqxSYczzkxeIebJgWi8GBYbq4zW+8QzOqrEDA4TugJkFIuN3a2rx5OfrcC7UiYOZk+0dkEP7VWD/3hYBGIflqc8sR++s6siFqH1/KlCYGEzG+HHBNabO5wwcHMjQ7EY5i4Wra4auO05h589hmGtxARF43vsR53+4LFHO233akAGbCtwDKSE4pMxc2TWzBTR9dY1qwa3YD2LyWaaV7u7effAG+Wrd7/p/TCv5s0/eWKx++5DhE1Ezp/ndQ3GlABS4aQMUuNqJrav8+CKM/CwlT08Mru/Igj5MTiqOcvQxiLCo87zchSplZWbPNrIxbMhoj/NJCgz4R+DDTWuU6lVg/94+CkEi/g9OHn1sVrpVb/RX9/ewqG3+z0YWj748xl7mbIF3lNBFVOpXKzNlVgchxhk7D7W/VkzfMulZidzILjycnlUb1I8Kg+eB1jKomEvw4dq2SMZYhpBfCYv1UAxtzbW5c2qwW8gPI/CIox/jfrZQL27/QsK6Zc3O/JlszcbYyW7zUFSTiwxYqaD5h56taC+FYHT9eTxrdor9FyCihM3AXGbkDnQsP6wpMshBtwU2wTh2ZRS0+xg4pz1croAxhuzMi5Julw0+A2EZxC4HL9fTht4D3gcvLdxgTyNY4KzGHBUhgmdPRi4VKCVh7Ok9Y3D2RKca2uaRStP+VqHbKlHgacuMGuLUx4ViwnBmBu+h2oMu80aUGdf+1wfzLdq8Bx4nx//CnJ9/Hn163g7TlkE3/rhXVHQ541LLay4Ct8IdqOlIN75AjeThwYXS68agmBb+tngR6d1bQBgulCWz6sJ7agzPvhrZfu/UEa1LhuNFXtnNEcQbEiWvaJSv2Ak6KLBLwhuw7AK5N041dV/Z+Ax1XEPmt66876068jqWU/NFtkzJLYuCcx1YId+owMMZpwqOROEBuviTZoccp80dOuOCoeF4iDQ72W4OGMLNky0iFcyOyuBa2rW8GxqWXosGvwWzJ9AsYIotjP16NUpJfkVDI63/dUf48MeJGPJ55FEtGhQx9DEVkeoCcpRgisOIRCREgEDRCAC0eJN9jq82umhqdljCR+iHlaVm53x06fmmd8GqszG7LXh8XszpUIF2eZGvKQ76prBL0huQPAcBHeLDA+Gej4+dnGSJUQDDt4yL77YgyIC3eqbFTllk4GGazMjdtCenBFF4Zt4Y6ZHeVfrBx4uZGxwU3Ojk2i2J3ZVyKlC1MZUTNb10eerBn8O4yVK45MNTk5/dXf7+vXt+4dX72jr7s9TjtbV29u3p7Hqp5PAXbEzxo3m9RXKPlnL5t2sg0qJPUccaXllhXbN3sNTzdpChGOKASG2NWmORdc9HnW5ElK20HcNiFQXXW+KSJ5MtlVaKcrpyJZQrVPXRYM/oruMzMvA/VgwcrK5ez9bazbmpLjGtqQSbbLehN6MA59lu5KcpBbOQsKWhkcYWDelAeVkqi28oDEWv3XY9cuEyh2pI757q+I9aE03XkJjYXystVjHrbkO+aLBSyB/Ap3VFgrsdnnFb5xOln7XP/XjsdM3P8Dhmlf/ga/+56fJfH/Di4+7//rUzejvp6NG/PaOFHe3sSqgSTnE1wxwqukll+hSENDlCl1Xu2fDa+dHE06CzcFrVXx3dDtzcGqOaoASfQBmpxZ9yVNV86Anu14c1q6B2PSdI/3WM1ZWDX7bceGlwVtdNO03ffhq8urN/eeBjVfPaKZi+UDOH0gBejyDMsUNm8FIDfQifH8e7AdqopFZakGE4I2OnaOPEAcrFzO05ciHdTw9RSXvbbMJ8qdYI40zFnzj0QrTMVssktdXwqrBb1fCEiIXwbu3oZH1YaNLoDUG3NtmDp0Qx97EsQBSbmfVKiXZYMFXnU9p9OiNtg4a62utAfKsHJZB2DWK0yKxCuLPNAGfIlR3SpJjDzsW04yQL+hPvGbwCRQv7GfE9oF/6YdP+fTXrFO6/nKrd01tpncP1685+Gp3W5xkS9hIhQF8uYCGxVgCRFICYHBaLIH2oeVsIJhNC7nHlrwDcfcBkdt68JwyyzisDqk6qc0EyHKw6OBr9LNhUZUYCrRt9gZe1rsL6pDWDH4L6h6AVsH/fI/7fbPBr+w/XuueJtr8eqf9GX1ty8Yo6zbD4HSV3uM01Q0umByGr3hYTZsgVBuEt5yaHY2nQ1BuwzXG81lmOmoNgGY162KLZFbplLYzFe/NqS8Nvgh0O3t7QaBeNfjtGngOTs9eC48dmU7L7t3dbX/fxt3152Kb/dOu04ZHKGaYFLypDWQeyjxgdaSSNOQyJ0d2ROA+RmJ1vuOQstS7793EwImB5aihhynjhXgL6TkRhdMUsDTptKy3nJBUu7BkYv3qYdXg4mpYQuoZ6+Exr4cZPh/bTezAPZeNiWOzZ53gLBwIPTLbSRZFNAfm7NaeqivGYpuwKIye1PkMutaTxjGzIlQcduUUu9oUHfO9WKbckjDZPTCN0cGfY6VCirkLrpzWDP4U9ycQeUF8948aChvF+6kY48mt58xY+d6zpMBm6bF0p6NWULpccgqj8SwODiCLNp9mZQrLYUycay4CCqhFDrPUkXMDzQKLHLUP6Xna4EtfP5xZNXgxyC+7kz+V3u0i63ajf6U3o9qUrRkadcZoPIeJwbONyoO2XD1n+UYEO+uTSXR0wiGcjRcI7rDJtRHx1UjWXMIcmjgee3Yg0iq7RXhb0mSqwjpXXzV4Cc5fgfKyKD9jUKw/77cd5w6YJLz/11pGKKm5HgCncS0FmYrfZjos1DSehaRoMgg8XHpm68+jDt+i6SXhdcAIlPO9wChVoQ+x+EyX6IuTofaChJ1Vg3ugvnhXf653/vIC171e8bdtuH6CC/xySs28/+8dzbn8+ZuUauGubaw5sCUe3BveYKvTjwCnBwELQV8aHP0YCsY+lPkkyhM3kwqC+WGN+y1nVZZh8RaCCudUyYxYcsMmVq6BM7Cr1nqmwarB77Dfj9KLLIT4tVTkVc5HnXD/jMUgdqNn8YADT37axEZtnLNn8Qytb9PoMF6nxoygMUcwvoIJMLMIntLBLxh/WM6QEclwQl1sqxVx19hasQSjOs2TPRTm7DxtuWCM4JrBCxbDz5F6iQXh5FrvP7xtHFN1iye3fxmIkw2n0DSZCnFT8Ri09tCbi2xqgYAAH1qjmTm74Vlu7sJwps3IY7oyoyZXD3MKnhlOE9RDcoRYTDVEHiGFZtk+JIYWpJa8HvtXDa6vg7MAHYL++3f7XIDdEHGcB2CbleIbNnvpOfEYM2M3OPj7UGtintm0GRHBlRqqCGkg+6r6EMtRuS59sl2fFIYpnzktqWuszMkppdVsmYbLCebrRzmLBp+B/SM8L4K8vX7kHNckHddv8byeEQLsBh9oJZTai2vFxQKiB92mPlYsh4AN0sQh4knoHqFBKuiCgQ/wJmtKmrKL6bA6uByD5bg7yV4bXt9CYvZs1fvRJ7QKmwCOeUEd3JrBC+DfwuiwRbDTBeRyPnURzNjO1joCoUbPUcIJ8k5OgcDMBroYsviSmebvnYvdwSs0hH8G0SxymAdgi33mFiJUt8GWiHBPow4XIU3NFHbNKk4v8ABrBp+5BHZ7ARaxf7IsHEb+ZoBZPI4Pww89iwWKbPgACKIyskGE71ZAAIXzYHKfRdk7Rw1ocgAxrJwHUJmMUxECKsR/Nb5IPyq7CuLUcLpYLa6qtVAlZpbCCTfiQEkl6aiSynq/nFWDZxfABQjtwP90mf+VeXWfJlXfvn+ot//cj35OG4e4RSz8voEY9i6citBKbxbIzime1QRJowasDXYUcjlrB33OWCN9mCSHJS6nmadH5BqzBsOSBWHfMeup4gMbALIYUtZT61YNngV/GZ5nQ++vvx6r8c49Y9vnjf4+M4LhcUJYZNsyptqlOCKrRGYPwU4TwPtNnLFnTncbXmNBnIwzZE7ePqzoIFqOfQ2N2dVsgQvKll1FuBEZMeE/UCDFrKu/VYOLyJ9F59m4fzpgeGpq5O4lkMx5x+/FQs3HzB7iwwefW+fhj+Cr1ELx0YEMYL/4lvAjiPkuVwfB1zJnDdSj5kdXTQH0AzKtxX5qZ4adK5z/ngrephNIEOP7BX0LFw0uroAVnA5dDDtJYHHnj4JUZTZjsO9zB/4uRKcljQK8gTYcvw6eDZcpRYWjME3P2FFgUIPjMI/SAQmipFs7sPLS0DpB2TUyMTiElBGl2JzRTXdBQ/VFgy+wFC4jgu/++PXqU3fMF0mwk+A2Euy8NXOm6LNm7IXmmrDr7OkBuCzWBzyZBjI0sTekO7BAiD4IZOOn2nzY3S3eBzgYO105qzONFpR932N2RauPWJ68hJhyAedbM/gt3j8D4xJQ72/bjb7+YgXAzZtfd03zDBsqDoym+VkSG2Uxb96Zjv3qpcdcOTEiNPg7y6EqAtlOKVQh6DN+Frs7HubLcyrBcIooZ4sGzzTOzsQ/4MB3Mduc2mpZ7xO2avBHQM8C8Xw0d9/TZdlIh4YbFrKzEFPt1ozSPP5ai8E+lQo93qorXqeFO0ZMgyJracKlccQFdM1RkIIQCpZaY6s5Nu9UJ24GY2a27Ac56DsEsWH9WHbR4Cqkl97NPWWJgp5Fi9dN96fQxXC+KVy0CSGU3YY8CLamNNUl0NI5Sq89sEPCaa9610G6k3MGbAVyOzZrNbqjdNc0LbVuiijexYjtNNEwZGgFVseNAaIU67hgMvyqwZ+D+wQkqwh/8uVXnwcyP2cOqJitrCi2YgQl4ECFqNNPAOYDfmEMndX23FrvThqkCJkoRyj0AEgFDtlUhOCjTlTVxqgQvgj5hcMosK16Fx9qrJyUE01V08v6DeuqwW+B3UTiYjjfjIc7tmLYk9HowgaGGbjwmrzaEhxTmUqGUJojtFEzvtYiQ5ONiK6p2niq1oOEcs6FFsNRnteyHsDG7GdPePAFqhzLh72soMm0GakjBz7B5XrDRYNnMPzy+Pci94wmQ2ZjcmupfdacLBh9DpaHGoL9JWz8Qd6XjIAz1CqBUz+w8TrwzIHlJB4hyB828pGzHrKMHjI7OIzmEO+yqRRaXcyABAtz5AtGPq4Z3IZvtzM9FR6eEtm1vWZGxO3cN9Xc2/NY4vMWyhQfEzhtUG+GDZZ5wyXlYgeCZtVq1Hf61m41Q79Ja3CtztrDkou749wV/N+wDuS6s/WBgp8bW2eMjNPiNeZ4QfPUNYNnsHwai5eAdPcWTXZji8akQI0txD1bFbrSjRpOobOW3Rc1e5Y7+4o13FzvbM6dxky9V7XYzUelHGQxJNgtng4D4CCgoNoseO4QzIX57CwCuYD+rBpch3X3dv1rX9KoN2lLdSLshenEx+YGVjBQRTApYL5eFIsX0XDkCvltRp/4UYFcgWsmnmV4e9ixEbsAGps5fgVRTAuHjwmnO3DAQ8Q/jpHTBcVAiwbPwPjX/T7Arn97X0+TWjkkbH89l/h0/iQIlDyZGqrPcTQF2NKCK6N5Y9gGH8EFjkgZYhAfS+dIywISPz3nond/VEM+X/OIdk7HobfMrwcXCzzFBxSAYUrOmT181y+AFg2ehfBJMPaA+vDhHS+RH4be8bh4f5Kvi3Gj80mU7kICo5msOeeUT83OuDEM1msBR0IMncyDmylij0aWcHV21xjqwlFDUCDps4qd1qYRmJHDHZXgBEuW0sSqActOF8iQVYPnYT0DxzKyt6/H96U9n8+LxZ1KvK4fh3HuD6Vl4x6vV9PZiDCEKKAKmQf5zNNJyblkOhxvcYF5z8XGAY4U2djY58SOf03KYb0UWs6hgsjgnZUwu7czaYDqVVHXT/cJ2US9YFTyosHvkL4cnlXkH6vvv9Rh371r93+2q48D1AHQ3TNqcDnM+3wGv2gslQI8lzFSyhMRNkkfwlnEUAC+1dxjFETaCr7E5hO5W+cNuFYIR0EOet6camSrhqwu14p3U8SZaaCy8gzdQ6Pk9c29avBbyC/A5blQf3UT/PvtvhJ6J3GLYCU/IG5YX+3DrJCtrZc84Og8NvWEh2vgYC5qyNjPyeFH2D5neLa4TnJUdO4Vr4F4UllGEg3lF9ifKdpNddobFqIi6KzXVq4aXEP6R1heGOjd+xphauNWzjIiR4gFm0KfFgGtupmZauuCERNShvSVUcFBvam5NlF15DANdBTr5LBbOYNYwg53TJqeEgbnFLONGg85K/5fGxxhf8Gt3JrBXWi/0N5+vOjl33YdL2a3UYE1Rk2h9DBG4PDjPqyPJkI1Nfg42yY79nfAzQKloBmPqSp7ZSb4PT2uCqezbhOP34UOBcCccJ7Qe4Pw4gaL/fGWOJhmfVcvGlzC+RtAXgRcyOk3u4JzlI12ZtZC3IY5S9dRRPqMo5babIOD9iknaKcQEJDZtghoxoQfG2Hwbta0aeSwvCoNts+Wg8TcU0NkrTa32ZLXypsmZyLea7wgr2rN4AXgfgRkJ7ifr4p+/+uP+6t37+vrm/vfxt0+N73R/cL1Ymb2IU1KKNBuVwpvCxL8UHeVzQ464rGUELWZicAmYzb8OcUOd1jJhMmRkyDNsDnAgwKDxDZziT3GNM5qIfMCfM06+1o0uAnweVBeFuX9rS1ENg5HYvXKHujY1Ai2Ui0vElLvs3qo6OCBdRAfs8eTac6RnRkPNcL5SdgVR/WhYhEnBF0WPwfvahBKQQbwVlJvCKOegyTkgiEUqwb3QP3MeHwK72yrc2p7xi92nWNK2WhyoBwH41npUHzQUCGOQ0q15wz/Zh1IW8u2JKnT8Y6WU3VKb9qdJWdN5rAOlIEF2CnONPwMtfNuGX6FJbkuQeOlpKWmC+5sFw1uwvw0IM9B98Tdrr7k0PUbfPuh/bZLTsV0vslB6nVCCdvBLAR2lIUg5hG2cy5rQwTG16awv0EN4Fkxjt5bj2zBnkIOR3luNu9lLV23bCoEXEjrW+e92EyR+YglsYHY+qHYosGf47wBzUsi/iVrclc+hovnM1tLlDZHCrMHzltE/AIrHQ6/4rvJylSUGVMMzhgwmVJHia4xB95CaJeUDmtg4iHasBNB7GOYELtjWjAn8KlhBgKOjAJqpRc0MFkzeDnmX4NzCOiPh3D84ef2qXKyMXpo9KGlF++G8pBhloFIHuDZc6gmVw45bkbg8MeMJUOTJqYGd0TE0qDKDku5csnEOasE58tgAebUWqOkOsI03QPIhg27ntWxavAZK+EsYsesj5dYGt7afJ7oGToIB3nWQlOm3nEoYFSL7WQRABw0G3OBKr6VjbhQRE2ZUOSJlXL+KEXeYhvwWBZyGcFKuyj7FWi1+I0gUJYSmyKkr4+QWzT4nLVx8LK4G3/etpP1XVcpEs/zA/UmOqPZG+z/aCSy/6NloXhhCsYpcoAnG6/SRzYugUtkP5rP1hdfjro0i3E2Fb5Oyd5Ba4CjmQIvVv2p/1yr0/Dqdr0zzqLBy1fBN+A8H/Wm7zhcol/3u50daqPdSJqv8ItBepppOij0kNMcEVEzNS8zIZbmkJluZGIaJlAXiM0JdKLyNP6wylfsxOo0m+EyaOcsQUfoCO2nU4UMvMT51mT9qGbV4CreP8DyAkjf4n3fvNWH27t9Da/w0c4fyYXkjbGOvWt8d2rEdhkOKHtwALFANtjKYyv8kMm9TPUZMqnCGZTjilyhq4Nn/Ql0JYd962gaIbfxRnPtHKhjSNrXuxysGlzG+RtQXhTkZ2Q9ZLfRONzZHkzLnHrrbMK6H6NGxwQdljFxVGofRqMVHbayPqb5zq2ePai/OSqOSy55DJBPh6hhfaqucs401ljuqYTibbcz2HW2v2pwB9IvF7M/XqufuOL9W313/9vtw4twuew3rkwR2VStGyb7KZykFrqRBg9vOK+Kze2GC8I0LpFpqZO8LSaaoZ5jSo66Ms0+ODdVDSs/Qq3OFjeKMamMCDeE0AvJoXb9eHbV4OoSWEXr+evinbuuend3M+5eRvTZeP5oj/WcjcSmTMkjO8j8iPgewN9Gx1+jqLPWlFjgD9jeAoLJwHVIBXPo86gMRePZP4OnUN2PYJ0LubpgM5QpEGzBsxgXiK4f1i8aXF0N2xi9yBp47JF+8yzsk9/AXuERVZv3IqeWTyOEBP4jvDqH/im1p8o8Ep+CCaYUsH6Ei8hfbzbWo8I+8zMKS6ttjSQjbKXEdCzBbvYzjNDYWvWC0/tVgxdg/wQ2L4L5ybvcDYJK0y+x/UNw54OBg36zvoPdcqr2bAm4j5wGtXyfDaK+es+KOvYB0cD65ugV7L/72CHqDsszb0E4FCM7LGDOMBffQdZSbsUIPLq0yj8vyDNfM3jBEvg5VC+yIniB8CLrIKaNIdUQ9TU3D2whbp2dtajTEtifXIJyikkyKWgYCWQqRs7PS5Wt8WYzQ8JRp8A1JAhQ59pMrniWLaVYuo0ZDBXy47FbVerrvHDV4AXr4BxAL4C+3j18uH59e7/vRk82FJ+4NBJkXWm9sY2Vy4jmWjhLyGXl1IEAlzl0Fp7yNngDzn7MPk/Lu93DJlQIhOU0Mv2potmE5notjgMUeLM+c29ZYlrPal81uIz315C8FMIf+yS9yCbPW8y/wwMGvKvCiXLC8moRx5F67O6dWNqQhumh50GRH1xJ+NmojpUpmo66rXeWs3JBLDJWWw05zIbwrNAeFjLE9lMFkL1gBvaqwctAP4/SC6yDT22zP58U7or0OW50se0ujDjGcJWHsgjtbPQhxbUI5Zeo96b10P02OI5eLgEbho398cSYPH5U8qQfeDy+mmw8o4rkasHNvLFOBmioNzP5LHn9cnfV4DL6T2FzBOQvc40jdqObnXaHOO+HYVMT5zlBruB/NufYa85tTNeHnWF2jXAVHG0RjeUsj1JA/Q8bFBujDjjkEOGBEJWcaXEAKUgStl21lhk3dqx3QVk1uH8NHOQH7m5/H+1kk9MKb+9uHj7s8gTJu42c+V60RvZGiT7NEsesLNEvvMZpvPWqdkrE0wuGVWz4AwLRNJuc9uKPusWBnOi2p1m7F0bmnDoHbBTOlZmzV5NGDgjm67F/0eDyInganL2on0ptmr5+fX36ctfJTsKHPY9zcDOpo6uvMnxzwbvK4ibTap7ac3dRTOuDI4XBiFsdMxVGf8HSKIed9cYei1RxAzosSg1aQ8Ub7SGmHLqyj2qZff32ZtXgNs4/wvEsYO+wZO73I8vJeeeRzTJB34FWna61HmepWU3vI9rO6u8yc+HPxupTMd22jmUwrMVXg42ej7qvMbFGjorHGzO5wo1YNsGu7H+S07RsiQq+vX5ot2pwAdlv8XgpaPcfykjZqDmG2K6JSQcmQKQObOAkyhonSS2eTuV9VKbVRjhl44wNps3hs2fvaReOOpWB14zghLMqG2sgwHaH18x+GlDEKqF07EF3wVCpVYOX4fvccPzV9LkfE/v2n8JnuzE0tJjGyZkD7NQWaHEIbgOwHeKxc86m0SR39aKZ+bVY/Bo4B9wilHnQ9aOUWcI6LbVVKMRo1YAvRniZaZuzNrbUE/4edT3BdtXgAuA/A+hF0X9e/mV25xm5BytBrI1dILADM6fxNJypuaSSNZVqoMNsxP62ELNzUJsVDnl20GshHhWk3eytJTBliAWONpuxTN4PeG8cggmcTWUXmQs0+aLBHci/RP7lV4Y/nuc1vXv7PCkG/STnWfgobBdrmH8jsXVLPW6bLYEjdV1oEN1MtoczmJ0TgjX0mKQhwPN69rAaZSxFNgwobhZtNqZTW/vqiyZOrbXshzBM6uu1jKsGl4E/h88LQP+56dq+81aw6PNwS1PvxFcfq7NGeArBiuSSQVI5rT06FSdhqpWgEiPYLCQLmxspZctR96wR+g7OZIoJzid2//Yc6WuLcXEa0YLlGPK68l41uAz315hcBjHLbE7v8s9x92H/DWoqG53UMrRVxU7t2Jzqo8lsu+pqSSZ0PINcwdnmqa7T5tG9MuEijI5PkZJpR52pBd9Aj8UHE4MxLtcQpXcxePbO19pSrqmCRq8XxiwafArVJ2HYgeRpMPj7d9d4e88oZ5Ot/NYGysUcQOYGdWjGiFUMltIMmDWYlyuA2/XGmVAg5LMLh/rOKvgXzgQ5qjcl9Dhv4LGXWp9YSjlH8vzqWii8s9LWyvB9PQN+1eBZOJ/E4jJM3+xMWHUsQDq/IeFlOqS/B45gy2xxM3rXWKRkYSViR4Adwwtg7BpsdkmHs3C1vvgajyLSg/UR08IvqjoeUczcjViXA4TcrLkXG3zI63Rq1eBTCL7Zk5P6dReBZ/SGxao7v/2mzWVUZzhMGPywApzx2MrEcvpcbSwugJcFCUqcuBRTdSlDMhg/TJlHHWvUMWcyOkDSOJfd21bZYJq7hkWfrun0HHS+fgm9aPAp8J7G4dlY7m/YEZLfKC6DmsFnw1otp9QiAz9qC8eKCDPsrUmqczALV7UOx+MeaB/wntw5cuYodWNaHdnVPEP2zeF/8OfZTQ49RFxuHS4DfEXWr5lXDS4iuupR590Jnn71+qZevb/BzoYAfqN/cDrv6etrXlC8/fXUlu21fvilw23vCJluw+NGl8ysELFwSbPXnGKBkFeZnbkjhufuASEzzRbJdj35AyRf6g6S1sph9YNlIJZ3Zvab5vAOis1ZfW295ca774DX7xdkEqwafIR4FzLLcJ8qix/N3P/3e+3XH3s3nZbNLoyxZu35Q4rkwd8ddi/gtA2MgTUh7DovSaHpfYnTVYXAY42ggOF6Ga1kfKcElw7LDip5sL1Ah0SSIan2giWnWFmj9Gi1N29m1HyBVl00+B3GP4VjB7CPa+W7VfIMgHPe6KskwSCksotFZ9VenLH5HGc1wNZlzjId4L9+iHOR0wFBd+G4Y4OiZ6+lo1LCI6TjBEebbejkWWEJ2gzWms+lNsNchiI5rWcBrho8i+9PUFnG+a9Rr+br9w9MKKm3tw/3D3f67pff73eMrzpfzyue40xACOesHnhpaMEWUAzvare1eS1xWIjz2RUhKjrx3OhReD4R9ChUQy49RLavSlLGxGI6FRPa2fCyro4hp5SdCxofLhr8DtVzGFwEIh7E+Ocvvz282TF2eLNOJzlAYXm4wNHjLsxpDe9GSBctsxeNC1G9QIOzuQ7WdO7AkzIuzsOUaDDF4uF2tlE0eFc2OWwiPzkrPjOvxLP/7QWpWqsGn8Duy6P/AbF/++6Dn97BY52Rba5UaPrAPkQ6rEoPVbKRqDHkOju+lT/2mKKd//ff/ve//X+iaDW8'))


def source_132_unit(repo):
    require(sha(SOURCE_132_UNIT_RAW)==SOURCE_132_UNIT_SHA,'exact132_source_unit_raw')
    unit=json.loads(SOURCE_132_UNIT_RAW)
    parents=unit['ordered_parents'];bases=unit['bases'];boundary=unit['history_boundary']
    require(unit['merge']==MERGE_132 and git(repo,'rev-list','--parents','-n','1',MERGE_132).decode().split()==[MERGE_132,*parents],'exact132_ordered_parents')
    require(git(repo,'merge-base','--all',*parents).decode().splitlines()==bases,'exact132_complete_best_bases')
    require(len(bases)==2 and boundary not in bases and git(repo,'merge-base','--all',*bases).decode().splitlines()==[boundary],'exact132_distinct_history_boundary')
    for ref in [*bases,*parents]:git(repo,'merge-base','--is-ancestor',boundary,ref)
    for version in unit['versions']:
        require(git(repo,'rev-parse',version['commit']+'^{tree}').decode().strip()==version['tree'],'exact132_complete_tree_identity')
    for edge in unit['deltas']:
        before,after=edge['before'],edge['after']
        require(git(repo,'diff','--name-only',before,after).decode().splitlines()==edge['paths'],'exact132_complete_edge_paths')
        require(file_digest(git(repo,'diff','--raw','--no-abbrev','--no-renames',before,after))==edge['raw'],'exact132_complete_edge_raw')
        require(file_digest(git(repo,'diff','--binary','--no-ext-diff','--no-renames','--abbrev=9',before,after))==edge['patch'],'exact132_complete_edge_patch')
    history=git(repo,'rev-list','--reverse','--topo-order','--parents',boundary+'..'+parents[1]).decode().splitlines()
    require(history==unit['source_history'],'exact132_complete_source_history')
    auto=subprocess.run(['git','--no-replace-objects','-C',str(repo),'merge-tree','--write-tree','--name-only','-z','--no-messages',*parents],capture_output=True)
    expected=unit['automatic_merge']
    require(auto.returncode==expected['exit']==0 and file_digest(auto.stdout)==expected['stdout'] and file_digest(auto.stderr)==expected['stderr']
        and auto.stdout==(expected['tree']+'\0').encode() and unit['versions'][-1]['tree']==expected['tree'],'exact132_whole_automatic_result')
    return dict(type='F54_EXACT132_SOURCE_UNIT',source_unit_sha256=SOURCE_132_UNIT_SHA,merge=MERGE_132,
        parents=parents,best_bases=bases,history_boundary=boundary,source_history=history,
        automatic_tree=expected['tree'],admission=False,path_authority=False)


def boundary_132_capability(repo, merge, first, source, collector_base, collector_target):
    unit=source_132_unit(repo)
    require([merge,first,source]==[unit['merge'],*unit['parents']],'exact132_literal_tuple')
    production=production_applicability(repo)
    production_base=production['collection_base'] if production.get('type')=='F13-PRODUCTION-SOURCE-v11' else production['base']
    require([collector_base,collector_target]==[production_base,production['source']],
            'exact132_actual_production_outer_scope')
    require(production['admission'] is False and production['runtime_pass'] is False
        and production['current_or_future_path_authority'] is False,'exact132_no_production_grant')
    return dict(type='F54_EXACT132_HISTORY_CAPABILITY',unit=unit,production=production,
        collector_scope=dict(base=collector_base,target=collector_target,
            allow_direct_followup='true',partition_source_history='true'),
        history_boundary=unit['history_boundary'],best_bases=unit['best_bases'],admission=False)


def composition_132_capability(repo, merge, boundary, first, source, name, result, collector_base, collector_target):
    capability=boundary_132_capability(repo,merge,first,source,collector_base,collector_target)
    unit=capability['unit']
    require(boundary==unit['history_boundary'],'exact132_composition_boundary')
    require(name in git(repo,'diff','--name-only',boundary,source).decode().splitlines(),
            'exact132_composition_actual_delta_path')
    entries={label:tree_entry(repo,revision,name).rstrip('\0') for label,revision in
        (('boundary',boundary),('first',first),('source',source),('result',merge),('automatic',unit['automatic_tree']))}
    require(result==entries['result']==entries['automatic'],'exact132_composition_exact_result')
    return dict(type='F54_EXACT132_WHOLE_COMPOSITION',merge=merge,parents=[first,source],
        history_boundary=boundary,best_bases=unit['best_bases'],automatic_tree=unit['automatic_tree'],
        capability_sha256=sha(canonical(capability)),path=name,result_entry=result,entries=entries,
        admission=False,owner_permission=False)


R23_132_SOURCE_UNIT_RAW = zlib.decompress(base64.b64decode('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'))
R23_132_SOURCE_UNIT_SHA = '7716f9a7aafa6e42370a0c088a282d0ca2b7046a370ce4523eb10c4c92dca033'


def r23_132_source_unit(repo):
    require(sha(R23_132_SOURCE_UNIT_RAW)==R23_132_SOURCE_UNIT_SHA,'r23_132_source_raw')
    unit=json.loads(R23_132_SOURCE_UNIT_RAW)
    require(unit['merge']==MERGE_132 and len(unit['rows'])==2,'r23_132_literal_unit')
    for version in unit['versions']:
        ref=version['commit']
        require(git(repo,'rev-list','--parents','-n','1',ref).decode().split()==[ref,*version['parents']]
            and git(repo,'rev-parse',ref+'^{tree}').decode().strip()==version['tree'],'r23_132_version')
        for entry in version['entries']:
            require(tree_entry(repo,ref,entry['path']).rstrip('\0')==entry['entry']
                and file_digest(git(repo,'show',ref+':'+entry['path']))==entry['content'],'r23_132_version_entry')
    edge=unit['origin_contribution'];before,after=edge['before'],edge['after']
    require(git(repo,'diff','--name-only',before,after).decode().splitlines()==edge['paths'],'r23_132_complete_origin_paths')
    require(file_digest(git(repo,'diff','--raw','--no-abbrev','--no-renames',before,after))==edge['raw']
        and file_digest(git(repo,'diff','--binary','--no-ext-diff','--no-renames','--abbrev=9',before,after))==edge['patch'],'r23_132_complete_origin_diff')
    for entry in edge['entries']:
        for side,ref in (('before',before),('after',after)):
            require(tree_entry(repo,ref,entry['path']).rstrip('\0')==entry[side]
                and file_digest(git(repo,'show',ref+':'+entry['path']))==entry[side+'_content'],'r23_132_origin_entry')
    for before,after in unit['ancestry']:git(repo,'merge-base','--is-ancestor',before,after)
    require(git(repo,'rev-parse',unit['child_source']+'^{tree}')==git(repo,'rev-parse',unit['origin']+'^{tree}'),
            'r23_132_explicit_merge_retains_whole_origin')
    return unit


def r23_132_owner_evidence(repo, state):
    """Only a fullfold-derived live pending state can authorize this transition."""
    unit=r23_132_source_unit(repo);name=state['path']
    rows=[v for v in unit['rows'] if v['path']==name]
    require(len(rows)==1,'r23_132_exact_path');expected=rows[0]
    require(state['operation']==state['frame']+'/'+MERGE_132+'/2','r23_132_exact_operation')
    require(state['incoming']==expected['incoming'] and state['prior_rows']==[expected['prior']]
        and state['child_row']==expected['child_row'],'r23_132_exact_live_owner_rows')
    require(state['aggregate'] is True and state['obligations'] and state['child_obligations'],
            'r23_132_preserve_true_aggregate_and_all_obligations')
    prior_writers=[v for v in state['obligations'] if v['operation'].endswith('/'+unit['prior_owner']+'/2')]
    require(prior_writers and any(v['classification']['trigger']=='true' for v in prior_writers),
            'r23_132_original_true_writer_required')
    require(state['obligations'][-1]['incoming']['raw']==expected['incoming'],
            'r23_132_pending_obligation_identity')
    context=state['frame_context'];cap=boundary_132_capability(repo,MERGE_132,*unit['parents'],context['base'],context['target'])
    require(state['capability']==cap and context['allow_direct_followup']=='true'
        and context['partition_source_history']=='true','r23_132_current_capability')
    return dict(type='F54_EXACT132_R23_OWNER_PERMISSION',source_unit_sha256=R23_132_SOURCE_UNIT_SHA,
        **state,admission=False,runtime_pass=False,current_or_future_path_authority=False)


def exact_boundary(repo, merge, first, source):
    require([merge,first,source]==[M,P1,P2],'fixed_historical_tuple')
    require(git(repo,'rev-list','--parents','-n','1',merge).decode().split()==[M,P1,P2],'ordered_parents')
    require(git(repo,'merge-base','--all',P1,P2).decode().splitlines()==BASES,'complete_best_base_order')
    require(git(repo,'merge-base','--all',*BASES).decode().splitlines()==[O],'unique_history_boundary')
    for ref in [*BASES,P1,P2]: git(repo,'merge-base','--is-ancestor',O,ref)
    return dict(type='F54_HISTORY_BOUNDARY_OBSERVATION',history_boundary=O,best_bases=BASES,
        merge=M,parents=[P1,P2],admission=False)


def composition(repo, boundary, first, source, name, result):
    exact_boundary(repo,M,first,source);require(boundary==O,'history_boundary_not_best_base')
    require(tree_entry(repo,M,name).rstrip('\0')==result,'exact_result_entry')
    p=subprocess.run(['git','--no-replace-objects','-C',str(repo),'merge-tree','--write-tree','--name-only','-z','--no-messages',first,source],capture_output=True)
    require(p.returncode==0 and p.stdout==TREE.encode()+b'\0','whole_parent_composition')
    require(git(repo,'rev-parse',M+'^{tree}').decode().strip()==TREE,'committed_result_tree')
    return dict(type='F54_WHOLE_COMPOSITION_OBSERVATION',merge=M,parents=[P1,P2],best_bases=BASES,
        history_boundary=O,result_tree=TREE,path=name,result_entry=raw_binding(result),admission=False)


def exact_3071_boundary(repo, merge, first, source):
    require([merge,first,source]==[J3071,J3071_P1,J3071_P2],'fixed3071_historical_tuple')
    require(git(repo,'rev-list','--parents','-n','1',merge).decode().split()==[J3071,J3071_P1,J3071_P2],'fixed3071_ordered_parents')
    require(git(repo,'merge-base','--all',first,source).decode().splitlines()==J3071_BASES,'fixed3071_all_best_bases')
    require(git(repo,'merge-base','--all',*J3071_BASES).decode().splitlines()==[J3071_O],'fixed3071_unique_older_history_boundary')
    for ref in [*J3071_BASES,first,source]:git(repo,'merge-base','--is-ancestor',J3071_O,ref)
    require(git(repo,'rev-parse',merge+'^{tree}').decode().strip()==J3071_TREE,'fixed3071_committed_tree_identity')
    # Structural permission to observe history, not a selected best base,
    # completed child, clean merge result, source or mutation qualification.
    return dict(type='F54_FIXED3071_STRUCTURAL_BOUNDARY_PROTOTYPE',merge=merge,parents=[first,source],
        best_bases=J3071_BASES,history_boundary=J3071_O,committed_tree=J3071_TREE,
        history='UNOBSERVED',composition='UNQUALIFIED',admission=False)


def validate_3071_packet(repo, directory):
    folder=Path(directory);raw=(folder/'structural-facts-v1.json').read_bytes()
    require(sha(raw)==J3071_PACKET_SHA,'fixed3071_complete_structural_packet_hash')
    packet=json.loads(raw)
    require(packet['version']=='F54-FIXED3071-STRUCTURAL-FACTS-v1' and packet['admission'] is False,'fixed3071_structural_not_authority')
    exact_3071_boundary(repo,packet['merge'],*packet['parents'])
    require(packet['best_bases']==J3071_BASES and packet['older_history_boundary']==J3071_O and packet['committed_tree']==J3071_TREE,'fixed3071_packet_tuple')
    require(packet['ruling_sha256']==J3071_RULING_SHA and sha((folder/'rudeus410-3071-boundary-decision.md').read_bytes())==J3071_RULING_SHA,'fixed3071_independent_boundary_contract')
    nodes=git(repo,'rev-list','--topo-order','--reverse',J3071,'^'+J3071_O).decode().splitlines()
    source_nodes=git(repo,'rev-list','--topo-order','--reverse',J3071_P2,'^'+J3071_O).decode().splitlines()
    require([v['node'] for v in packet['nodes']]==nodes and packet['source_nodes']==source_nodes,'fixed3071_complete_ordered_dag')
    require(packet['actual_source_first_parent_operations']==git(repo,'rev-list','--first-parent','--reverse',J3071_O+'..'+J3071_P2).decode().splitlines(),'fixed3071_actual_source_traversal_inventory')
    trees={}
    def entries(ref):
        if ref not in trees:
            values=git(repo,'ls-tree','-r','-z',ref).decode().split('\0')
            trees[ref]={v.split('\t',1)[1]:v+'\0' for v in values if v}
        return trees[ref]
    names=set()
    for node in packet['nodes']:
        parents=git(repo,'rev-list','--parents','-n','1',node['node']).decode().split()[1:]
        require(node['ordered_parents']==parents and [e['parent'] for e in node['parent_edges']]==parents,'fixed3071_every_parent_edge')
        require(node['source_side']==(node['node'] in source_nodes),'fixed3071_actual_source_side')
        for edge in node['parent_edges']:
            paths=git(repo,'diff','--name-only','--no-renames',edge['parent'],node['node']).decode().splitlines();names.update(paths)
            require(edge['paths']==paths and edge['raw_diff']==raw_binding(git(repo,'diff','--raw','--no-abbrev','--no-renames','-z',edge['parent'],node['node'])),'fixed3071_complete_internal_deltas_not_net_only')
            require(edge['entries']=={name:dict(before=entries(edge['parent']).get(name,''),after=entries(node['node']).get(name,'')) for name in paths},'fixed3071_internal_mode_type_oid_versions')
    require(packet['all_internal_touched_paths']==sorted(names) and set(packet['all_internal_versions'])==names,'fixed3071_no_omitted_reverted_internal_path')
    refs=dict(O=J3071_O,bestA=J3071_BASES[0],bestB=J3071_BASES[1],P1=J3071_P1,P2=J3071_P2,J=J3071,
        H53=packet['original_caller']['target'],H4=json.loads(packet['original_native_binding']['raw'])['snapshot'])
    require(packet['all_internal_versions']=={name:{label:entries(ref).get(name,'') for label,ref in refs.items()} for name in sorted(names)},'fixed3071_all_versioned_path_entries')
    actual_sets={name:git(repo,'diff','--name-only',a,b).decode().splitlines() for name,a,b in
        [('candidate_difference',*J3071_BASES),('history_to_source',J3071_O,J3071_P2),('first_to_result',J3071_P1,J3071),
         ('source_to_result',J3071_P2,J3071),('older_to_result',J3071_O,J3071)]}
    require(packet['Git_only_path_sets']==actual_sets,'fixed3071_precise_inventory_meanings')
    require(packet['source_qualification']['status']=='UNRESOLVED' and packet['actual_history']=='UNOBSERVED' and packet['actual_rows']=='UNOBSERVED','fixed3071_no_inventory_as_source_or_raw_row_grant')
    return packet


def current_author_J_context_applicability(repo,native,historical,current,new):
    """Reconstruct the pinned v4 record; event qualification remains with caller."""
    verifier=native.documentation
    previous=verifier.load_record(repo)
    predecessor=dict(current,version=previous['version'],content_base=previous['base'],
        snapshot=previous['source'],record=verifier.PINNED_RECORD_COMMIT,
        record_sha256=verifier.PINNED_RECORD_SHA256,paths=[r['path'] for r in previous['projection']])
    predecessor.pop('source_applicability',None)
    preserved=current_author_J_native_applicability(repo,native,historical,predecessor)
    require(current['content_base']==new['base'] and current['snapshot']==new['source']
        and current['paths']==predecessor['paths'],'current_J_context_complete_projection')
    git(repo,'merge-base','--is-ancestor',predecessor['record'],new['source'])
    git(repo,'merge-base','--is-ancestor',current['record'],current['source'])
    raw=git(repo,'show',current['record']+':'+verifier.CONTEXT_RECORD_PATH)
    require(sha(raw)==current['record_sha256'],'current_J_context_record_hash')
    return dict(type='F54_CURRENT_AUTHOR_J_NATIVE_SOURCE_APPLICABILITY',historical=preserved['historical'],
        current=dict(**{k:current[k] for k in ('base','content_base','snapshot','record','record_sha256','paths')},
            **({'source_applicability':current['source_applicability']} if 'source_applicability' in current else {}),
            classification=new['classification'],record_raw=raw_binding(raw)),
        predecessor=preserved['current'],predecessor_applicability=preserved,successor_lineage=new['predecessor'],
        added_paths=preserved['added_paths'],path_entries=[dict(path=name,
            historical_entry=entry_binding(repo,historical['snapshot'],name),current_entry=entry_binding(repo,current['snapshot'],name),
            membership='HISTORICAL_AND_CURRENT' if name in historical['paths'] else 'CURRENT_RECORD_ONLY; ORIGINAL_GATES_REQUIRED')
            for name in current['paths']],admission=False,runtime_pass=False)


def current_author_J_native_applicability(repo, native, historical, current):
    """Preserve old reviewed facts; independently derive current source authority."""
    if current.get('version') == 'NATIVE-P3-SOURCE-v5':
        verifier,new=native.verified_record(repo,current)
        documentation,previous=verifier.predecessor_record(repo,current)
        require(documentation is native.documentation,'current_J_release_predecessor_identity')
        predecessor=dict(version=documentation.CONTEXT_VERSION,base=documentation.CONTEXT_EVENT_BASE,
            content_base=previous['base'],snapshot=previous['source'],record=documentation.CONTEXT_RECORD_COMMIT,
            record_sha256=documentation.CONTEXT_RECORD_SHA256,paths=[r['path'] for r in previous['projection']],
            source=current['snapshot'])
        preserved=current_author_J_context_applicability(repo,native,historical,predecessor,previous)
        require(current['content_base']==new['base'] and current['snapshot']==new['source']
            and current['paths']==[r['path'] for r in new['projection']]
            and set(predecessor['paths'])<=set(current['paths']),'current_J_release_complete_projection')
        git(repo,'merge-base','--is-ancestor',documentation.CONTEXT_RECORD_COMMIT,current['snapshot'])
        raw_previous=git(repo,'show',documentation.CONTEXT_RECORD_COMMIT+':'+documentation.CONTEXT_RECORD_PATH)
        require(sha(raw_previous)==documentation.CONTEXT_RECORD_SHA256,'current_J_release_predecessor_hash')
        raw=git(repo,'show',current['record']+':'+verifier.RECORD_PATH)
        require(sha(raw)==current['record_sha256'],'current_J_release_record_hash')
        return dict(type='F54_CURRENT_AUTHOR_J_NATIVE_SOURCE_APPLICABILITY',historical=preserved['historical'],
            current=dict(**{k:current[k] for k in ('base','content_base','snapshot','record','record_sha256','paths')},
                classification=new['classification'],record_raw=raw_binding(raw)),
            predecessor=preserved['current'],predecessor_applicability=preserved,
            successor_lineage=new['release_record']['predecessor'],
            added_paths=sorted(set(current['paths'])-set(historical['paths'])),path_entries=[dict(path=name,
                historical_entry=entry_binding(repo,historical['snapshot'],name),current_entry=entry_binding(repo,current['snapshot'],name),
                membership='HISTORICAL_AND_CURRENT' if name in historical['paths'] else 'CURRENT_RECORD_ONLY; ORIGINAL_GATES_REQUIRED')
                for name in current['paths']],admission=False,runtime_pass=False)
    if current.get('version') == 'NATIVE-P3-SOURCE-v4':
        verifier,new=native.verified_record(repo,current)
        require(verifier is native.documentation,'current_J_context_identity')
        return current_author_J_context_applicability(repo,native,historical,current,new)
    if current.get('version') == 'NATIVE-P3-SOURCE-v3':
        require(hasattr(native,'verified_record'),'current_J_documentation_dispatcher')
        verifier,new=native.verified_record(repo,current)
        require(verifier is native.documentation,'current_J_documentation_identity')
        previous=native.successor.load_record(repo)
        predecessor=dict(current,version=previous['version'],content_base=previous['base'],
            snapshot=previous['source'],record=native.successor.PINNED_RECORD_COMMIT,
            record_sha256=native.successor.PINNED_RECORD_SHA256,
            paths=[row['path'] for row in previous['projection']])
        preserved=current_author_J_native_applicability(repo,native,historical,predecessor)
        require(current['content_base']==new['base'] and current['snapshot']==new['source'] and
            current['paths']==predecessor['paths'],'current_J_documentation_same_complete_projection')
        git(repo,'merge-base','--is-ancestor',predecessor['record'],new['source'])
        git(repo,'merge-base','--is-ancestor',current['record'],current['source'])
        raw=git(repo,'show',current['record']+':'+verifier.RECORD_PATH)
        require(sha(raw)==current['record_sha256'],'current_J_documentation_record_hash')
        return dict(type='F54_CURRENT_AUTHOR_J_NATIVE_SOURCE_APPLICABILITY',historical=preserved['historical'],
            current=dict(**{k:current[k] for k in ('base','content_base','snapshot','record','record_sha256','paths')},
                classification=new['classification'],record_raw=raw_binding(raw)),
            predecessor=preserved['current'],predecessor_applicability=preserved,successor_lineage=new['predecessor'],
            added_paths=preserved['added_paths'],path_entries=[dict(path=name,
                historical_entry=entry_binding(repo,historical['snapshot'],name),current_entry=entry_binding(repo,current['snapshot'],name),
                membership='HISTORICAL_AND_CURRENT' if name in historical['paths'] else 'CURRENT_RECORD_ONLY; ORIGINAL_GATES_REQUIRED')
                for name in current['paths']],admission=False,runtime_pass=False)
    if current.get('version') == 'NATIVE-P3-SOURCE-v2':
        # First run the unchanged v1 historical reconstruction, then bridge only
        # its exact fixed predecessor to the independently pinned successor.
        require(hasattr(native,'verified_record'),'current_author_J_successor_dispatcher')
        verifier,new=native.verified_record(repo,current)
        require(verifier is native.successor,'current_author_J_successor_identity')
        previous=native.legacy.load_record(repo)
        predecessor=dict(current,version=previous['version'],content_base=previous['base'],
            snapshot=previous['source'],record=native.legacy.PINNED_RECORD_COMMIT,
            record_sha256=native.legacy.PINNED_RECORD_SHA256,
            paths=[row['path'] for row in previous['projection']])
        preserved=current_author_J_native_applicability(repo,native.legacy,historical,predecessor)
        require(current['content_base']==new['base'] and current['snapshot']==new['source']
            and current['paths']==[row['path'] for row in new['projection']],
            'current_author_J_successor_complete_binding')
        require(set(predecessor['paths'])<=set(current['paths']),
            'current_author_J_successor_preserves_projection')
        git(repo,'merge-base','--is-ancestor',predecessor['record'],new['source'])
        git(repo,'merge-base','--is-ancestor',current['record'],current['source'])
        raw=git(repo,'show',current['record']+':'+verifier.RECORD_PATH)
        require(sha(raw)==current['record_sha256'],'current_author_J_successor_record_hash')
        old_paths=historical['paths']
        return dict(type='F54_CURRENT_AUTHOR_J_NATIVE_SOURCE_APPLICABILITY',
            historical=preserved['historical'],
            current=dict(**{k:current[k] for k in ('base','content_base','snapshot','record','record_sha256','paths')},
                classification=new['classification'],record_raw=raw_binding(raw)),
            predecessor=preserved['current'],successor_lineage=new['predecessor'],
            added_paths=sorted(set(current['paths'])-set(old_paths)),
            path_entries=[dict(path=name,historical_entry=entry_binding(repo,historical['snapshot'],name),
                current_entry=entry_binding(repo,current['snapshot'],name),
                membership='HISTORICAL_AND_CURRENT' if name in old_paths else 'CURRENT_RECORD_ONLY; ORIGINAL_GATES_REQUIRED')
                for name in current['paths']],admission=False,runtime_pass=False)
    require(historical['content_base']==current['content_base']==AUTHOR_NATIVE_CONTENT_BASE,
        'current_author_J_same_native_content_baseline')
    old_raw=git(repo,'show',historical['record']+':'+native.RECORD_PATH)
    require(sha(old_raw)==historical['record_sha256'],'current_author_J_historical_record_hash')
    old=json.loads(old_raw);new=native.load_record(repo)
    require(old['version']==new['version']=='NATIVE-P3-SOURCE-v1'
        and old['classification']==new['classification']=='DORMANT-A3-RESTRICTIVE-P3-SOURCE'
        and old['base']==historical['content_base'] and old['source']==historical['snapshot']
        and old['consumers']==new['consumers']==['DEVELOP-SOURCE-v1','MASTER-SOURCE-v1'],
        'current_author_J_distinct_source_only_record_identities')
    require(native.entry(repo,historical['record'],native.RECORD_PATH)['mode']=='100644',
        'current_author_J_historical_record_mode')
    old_paths=native.rows(old['projection'],('path','before','after'))
    native.rows(old['context'],('path','entry'))
    require(old_paths==historical['paths'] and set(old_paths)<=set(current['paths']),
        'current_author_J_complete_historical_membership')
    for item in old['projection']:
        require(item['before']!=item['after'] and native.entry(repo,old['base'],item['path'])==item['before']
            and native.entry(repo,old['source'],item['path'])==item['after'],'current_author_J_historical_projection_entries')
    for item in old['context']:
        require(item['entry'] is not None and native.entry(repo,old['source'],item['path'])==item['entry'],
            'current_author_J_historical_context_entries')
    for a,b in ((historical['content_base'],historical['snapshot']),
        (historical['snapshot'],historical['record']),(historical['record'],current['source']),
        (historical['snapshot'],current['snapshot'])):
        git(repo,'merge-base','--is-ancestor',a,b)
    def metadata(binding,record,raw):
        return dict(**{k:binding[k] for k in ('base','content_base','snapshot','record','record_sha256','paths')},
            classification=record['classification'],record_raw=raw_binding(raw))
    return dict(type='F54_CURRENT_AUTHOR_J_NATIVE_SOURCE_APPLICABILITY',
        historical=metadata(historical,old,old_raw),
        current=metadata(current,new,git(repo,'show',current['record']+':'+native.RECORD_PATH)),
        added_paths=sorted(set(current['paths'])-set(old_paths)),
        path_entries=[dict(path=name,historical_entry=entry_binding(repo,historical['snapshot'],name),
            current_entry=entry_binding(repo,current['snapshot'],name),
            membership='HISTORICAL_AND_CURRENT' if name in old_paths else 'CURRENT_RECORD_ONLY; ORIGINAL_GATES_REQUIRED')
            for name in current['paths']],admission=False,runtime_pass=False)


def production_module():
    spec=importlib.util.spec_from_file_location('f13_fixed_production',ROOT/'scripts/ci/f13_production_source.py')
    module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
    return module


def production_applicability(repo):
    return production_module().qualify(repo)


def prepare_3071_capabilities(repo, directory, native_path, author_applicability=None, production=None):
    require(author_applicability is None or production is None,'fixed3071_no_mixed_authority')
    packet=validate_3071_packet(repo,directory);target=git(repo,'rev-parse','HEAD').decode().strip()
    allowed={'scripts/ci/f13_f54_history.py','scripts/ci/f13_f54_history_test.py','scripts/ci/voice-db-runtime-provisioning-contract_test.sh','.agent/execplans/native-p3-master-source.md'}
    if production is not None:
        require(production==production_applicability(repo),'production_J_independent_complete_applicability')
        require(packet['original_caller']==AUTHOR_HISTORICAL_SCOPE,'production_J_original_historical_caller')
    elif author_applicability is None:
        require(set(git(repo,'diff','--name-only',J3071_PROTOTYPE_BASE,target).decode().splitlines())<=allowed,'fixed3071_current_carrier_only_four_owned_paths')
    else:
        require(author_applicability==current_author_applicability(repo),'current_author_J_independent_complete_applicability')
        require(packet['original_caller']==AUTHOR_HISTORICAL_SCOPE,'current_author_J_original_historical_caller')
    sources=['scripts/ci/f13_f54_history.py','scripts/ci/voice-db-runtime-provisioning-contract_test.sh','scripts/ci/voice-r22-runtime-scope.sh']
    require(all((ROOT/name).read_bytes()==git(repo,'show',target+':'+name) for name in sources),'fixed3071_actual_source_carrier')
    native_raw=Path(native_path).read_bytes();binding=json.loads(native_raw);original=json.loads(packet['original_native_binding']['raw'])
    if author_applicability is None and production is None:
        require(all(binding[k]==original[k] for k in ('base','content_base','snapshot','record','record_sha256','paths')),'fixed3071_no_borrowed_new_native_authority')
    context={k:os.environ.get(k,'') for k in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','VOICE_CI_PUSH_BEFORE_SHA','GITHUB_BASE_REF')}
    spec=importlib.util.spec_from_file_location('fixed3071_native',ROOT/('scripts/ci/f13_native_p3_authority.py' if (ROOT/'scripts/ci/f13_native_p3_authority.py').is_file() else 'scripts/ci/f13_native_p3_source.py'))
    native=importlib.util.module_from_spec(spec);spec.loader.exec_module(native)
    try:
        actual=native.qualify(repo,binding['base'],binding['source'],binding['target'],context['GITHUB_EVENT_NAME'],context['GITHUB_REF'],context['GITHUB_SHA'],context['VOICE_CI_PUSH_BEFORE_SHA'],context['GITHUB_BASE_REF'])
    except native.Rejected as error:raise Rejected('fixed3071_native_'+str(error)) from error
    require(actual==binding and binding['target']==target,'fixed3071_actual_native_context_independently_qualified')
    current_fields={}
    if production is not None:
        require(binding['consumer']=='MASTER-SOURCE-v1' and binding['base']==production['base']
                and binding['source']==production['source'],'production_J_actual_master_event')
        current_fields=dict(production_applicability=production,historical_native_binding=packet['original_native_binding'],
            native_applicability=current_author_J_native_applicability(repo,native,original,binding))
    if author_applicability is not None:
        require(binding['consumer']=='MASTER-SOURCE-v1' and binding['base']==AUTHOR_EVENT_BASE,
            'current_author_J_actual_master_event_base')
        git(repo,'merge-base','--is-ancestor',AUTHOR_REFERENCE,binding['source'])
        current_fields=dict(author_applicability=author_applicability,historical_native_binding=packet['original_native_binding'],
            native_applicability=current_author_J_native_applicability(repo,native,original,binding))
    return dict(type='F54_FIXED3071_PRODUCTION_CAPABILITIES_v1' if production is not None else
        ('F54_FIXED3071_CURRENT_AUTHOR_CAPABILITIES' if author_applicability is not None else 'F54_FIXED3071_STRUCTURAL_CAPABILITIES_PROTOTYPE'),
        **current_fields,directory=str(Path(directory).resolve()),packet_sha256=J3071_PACKET_SHA,
        tuple=exact_3071_boundary(repo,J3071,J3071_P1,J3071_P2),native_path=str(native_path),native_binding=raw_binding(native_raw),context=context,
        source_context=router_source_context(repo),caller=packet['original_caller'],git_version=git(repo,'--version').decode().strip(),
        history='UNOBSERVED',source_qualification='UNRESOLVED',supersession='NO_NEW_AUTHORITY',admission=False)


def prepare_3071_author_capabilities(repo, directory, native_path):
    return prepare_3071_capabilities(repo,directory,native_path,current_author_applicability(repo))


def prepare_3071_production_capabilities(repo,directory,native_path):
    return prepare_3071_capabilities(repo,directory,native_path,production=production_applicability(repo))


def checked_3071_capability_value(repo, cap):
    choices={'F54_FIXED3071_CURRENT_AUTHOR_CAPABILITIES':prepare_3071_author_capabilities,
        'F54_FIXED3071_STRUCTURAL_CAPABILITIES_PROTOTYPE':prepare_3071_capabilities,
        'F54_FIXED3071_PRODUCTION_CAPABILITIES_v1':prepare_3071_production_capabilities}
    require(cap.get('type') in choices,'fixed3071_distinct_capability_type')
    prepare=choices[cap['type']]
    require(cap==prepare(repo,cap['directory'],cap['native_path']),'fixed3071_complete_current_capability_binding')
    return cap


def verified_3071_capabilities(repo, path):
    return checked_3071_capability_value(repo,json.loads(Path(path).read_bytes()))


J3071_PRIOR_SPAN=dict(aggregate='dc0acdefec7e5c413988adf71f86d12bff4017bc',
    first='6e0cd809884f9a2b80f33d00a85c55ff42077aab',base='d4abb829622e9bd91b26842b61522e6840840867',
    target='645d9c54966c1270a1e1dbe9d767ddad6aac40ff',nodes=[
        'adf452b9141f96b8dd4ae847df55edb43d68180a','1554ff343633274332c2350b31ac6a2613558054',
        '2c133336d8745d44ea382932463c8707ae8dfed0','cb98a67bfab9390a4fa6adfb5416910db8dd75c1',
        '645d9c54966c1270a1e1dbe9d767ddad6aac40ff'])


def J_prior_origin_value(repo, capability, state, trace_path, prefix):
    """New observation permission for one real prior aggregate, never its rows."""
    r=J3071_PRIOR_SPAN;operation=state['frame']+'/'+r['aggregate']+'/2'
    require(state['capability_sha256']==sha(canonical(capability)),'fixed3071_prior_same_capability')
    require(operation in state['prior_operations'],'fixed3071_prior_actual_aggregate_before_J')
    aggregate=state['prior_operations'][operation]
    require([aggregate[k] for k in ('commit','first_parent','boundary','source_head','candidates')]==
        [r['aggregate'],r['first'],r['base'],r['target'],[r['base']]],'fixed3071_prior_exact_actual_source_tuple')
    require(git(repo,'rev-list','--topo-order','--reverse',r['base']+'..'+r['target']).decode().splitlines()==r['nodes'],
        'fixed3071_prior_complete_five_node_range')
    parent=r['base'];edges=[]
    for node in r['nodes']:
        actual=git(repo,'rev-list','--parents','-n','1',node).decode().split()[1:]
        require(actual==[parent],'fixed3071_prior_actual_linear_edges')
        edges.append(dict(node=node,ordered_parents=actual));parent=node
    return dict(type='F54_FIXED3071_PRIOR_SPAN_ORIGIN',capability=capability,J_operation=state['operation'],
        aggregate_operation=operation,aggregate_binding=aggregate,aggregate_classification=state['prior_classifications'][operation],
        aggregate_rows=state['prior_committed_rows'][operation],outer_map_binding=state['ledger'][state['frame']]['map_binding'],
        outer_map_path=state['frame_context']['output'],
        main_trace=str(trace_path),prefix=prefix,range=r,ordered_edges=edges,
        supplemental_flags=dict(allow_direct_followup='true',partition_source_history='false'),
        source_context=router_source_context(repo),admission=False)


def prepare_J_prior_origin(repo, capability_path, trace_path):
    cap=verified_3071_capabilities(repo,capability_path);raw=Path(trace_path).read_bytes()
    events=[json.loads(x) for x in raw.splitlines()]
    operations=[e['operation'] for e in events if e['kind']=='history_3071_capability']
    require(len(operations)==1,'fixed3071_prior_one_actual_J')
    state=validate_trace(repo,trace_path,sha(raw),J_history_checkpoint=operations[0])['J_history_checkpoint']
    return J_prior_origin_value(repo,cap,state,trace_path,file_digest(raw))


def verify_J_prior_origin(repo, origin):
    cap=origin['capability'];require(cap==checked_3071_capability_value(repo,cap), 'fixed3071_prior_independent_capability')
    current=Path(origin['main_trace']).read_bytes();bound=origin['prefix']
    require(len(current)>=bound['bytes'] and sha(current[:bound['bytes']])==bound['sha256'],'fixed3071_prior_original_prefix_immutable')
    # Read only the sealed prefix, keeping its original trace identity. Existing
    # range receipts bind that exact path; a temporary alias is not their origin.
    state=validate_trace(repo,origin['main_trace'],bound['sha256'],
        J_history_checkpoint=origin['J_operation'],sealed_prefix=bound)['J_history_checkpoint']
    require(origin==J_prior_origin_value(repo,cap,state,origin['main_trace'],bound),'fixed3071_prior_exact_original_context_and_owner')
    return origin


def verify_J_prior_span(repo, origin, trace_path, map_path):
    verify_J_prior_origin(repo,origin);raw=Path(trace_path).read_bytes();events=[json.loads(x) for x in raw.splitlines()]
    require(events and events[0]['kind']=='J_prior_span_link' and events[0]['payload']==origin,'fixed3071_prior_real_separate_observation_link')
    checked=validate_trace(repo,trace_path,sha(raw));root=checked['root_frame']
    require(not checked['failures'],'fixed3071_prior_observation_failure_not_swallowed')
    frame=next(e['payload'] for e in events if e['kind']=='frame_enter');r=J3071_PRIOR_SPAN
    require([frame[k] for k in ('base','target','allow_direct_followup','partition_source_history')]==[r['base'],r['target'],'true','false'],
        'fixed3071_prior_real_flags_not_original_outer_flags')
    begins=[e for e in events if e['kind']=='operation_begin']
    require([e['payload']['commit'] for e in begins]==r['nodes'] and all(e['payload']['classification']=='direct' and e['payload']['parent_index']==1 for e in begins),
        'fixed3071_prior_all_five_real_operations_not_Git_rows')
    ledger=checked['ledger'][root];actual=Path(map_path).read_bytes()
    require(ledger['map_binding']==file_digest(actual) and ledger['rows']==actual.decode().splitlines(),'fixed3071_prior_full_real_map_not_selected_paths')
    return dict(type='F54_FIXED3071_PRIOR_SPAN_OBSERVATION',origin=origin,trace=str(trace_path),trace_sha256=sha(raw),
        map_path=str(map_path),map_binding=file_digest(actual),ledger=ledger,
        actual_operations=[dict(operation=e['operation'],begin_digest=e['digest'],binding=e['payload']) for e in begins],
        classifications=[dict(operation=e['operation'],event_digest=e['digest'],classification=e['payload']) for e in events if e['kind']=='classification'],
        admission=False)


def prepare_J_prior_span(repo, origin_path, trace_path, map_path, outer_map_path):
    origin=json.loads(Path(origin_path).read_bytes())
    require(Path(outer_map_path).resolve()==Path(origin['outer_map_path']).resolve() and file_binding(outer_map_path)==origin['outer_map_binding'],
        'fixed3071_prior_observation_cannot_replace_outer_map')
    return verify_J_prior_span(repo,origin,trace_path,map_path)


def fixed3071_range_structure(repo, base, target, allow, partition):
    """Exact Git witness only. It neither qualifies rows nor opens the engine."""
    r=J3071_RANGE
    require([base,target,allow,partition]==[r['base'],r['target'],'true','false'],'fixed3071_range_exact_invocation')
    parents=lambda node:git(repo,'rev-list','--parents','-n','1',node).decode().split()[1:]
    require(parents(r['integration'])==[r['first'],target],'fixed3071_range_actual_I_ordered_parents')
    expected={r['direct']:[r['baseline']],r['merge']:[r['direct'],r['side']],target:[r['merge']]}
    previous=base
    for node in r['side_nodes']:
        expected[node]=[previous];previous=node
    require(previous==r['side'] and len(expected)==9,'fixed3071_range_six_distinct_side_nodes')
    nodes=git(repo,'rev-list','--topo-order','--reverse',base+'..'+target).decode().splitlines()
    require(len(nodes)==9 and set(nodes)==set(expected),'fixed3071_range_complete_nine_node_closure')
    require(all(parents(node)==value for node,value in expected.items()),'fixed3071_range_every_ordered_parent_edge')
    spine=git(repo,'rev-list','--first-parent','--reverse',base+'..'+target).decode().splitlines()
    side=git(repo,'rev-list','--first-parent','--reverse',base+'..'+r['side']).decode().splitlines()
    require(spine==[r['direct'],r['merge'],target] and side==r['side_nodes'],'fixed3071_range_exact_three_and_six_partition')
    require(set(spine).isdisjoint(side) and set(spine+side)==set(nodes),'fixed3071_range_disjoint_novel_operation_accounting')
    require(git(repo,'merge-base','--all',r['first'],target).decode().splitlines()==[base],'fixed3071_range_actual_I_source_boundary')
    require(git(repo,'merge-base','--all',r['direct'],r['side']).decode().splitlines()==[r['baseline']],'fixed3071_range_F_keeps_real_C_source_boundary')
    require(subprocess.run(['git','--no-replace-objects','-C',str(repo),'merge-base','--is-ancestor',r['baseline'],base],capture_output=True).returncode==0,'fixed3071_range_C_baseline_ancestry')
    chain=git(repo,'rev-list','--first-parent',target).decode().splitlines()
    sync=[node for node in chain if len(parents(node))==2 and parents(node)[1]==base]
    require(base not in chain and not sync,'fixed3071_range_original_guard_still_refuses')
    edges={node:[dict(parent=parent,delta=raw_binding(git(repo,'diff','--raw','--no-abbrev','--no-renames','-z',parent,node))) for parent in expected[node]] for node in nodes}
    return dict(type='F54_FIXED3071_NINE_NODE_STRUCTURAL_WITNESS',tuple=r,invocation=dict(base=base,target=target,allow=allow,partition=partition),
        nodes=nodes,ordered_parent_edges=expected,raw_edges=edges,spine=spine,side_nodes=side,
        original_guard=dict(status=1,base_in_first_parent=False,exact_sync_boundaries=[]),
        F_source_delta=raw_binding(git(repo,'diff','--name-only',r['baseline'],r['side'])),
        baseline_closure=raw_binding(git(repo,'rev-list','--topo-order',base)),
        history='UNOBSERVED',qualification='HOLD',admission=False)


def range_3071_origin_value(repo, capability, state, trace_path, prefix):
    folder=Path(capability['directory'])
    require(sha((folder/'rudeus410-3071-child-range-decision.md').read_bytes())==J3071_RANGE_RULING_SHA,'fixed3071_exact_range_decision')
    failed=(folder/'range/failed-trace.jsonl').read_bytes()
    require(sha(failed)==J3071_RANGE_FAILED_TRACE_SHA,'fixed3071_original_failed_range_immutable')
    r=J3071_RANGE
    witness=fixed3071_range_structure(repo,r['base'],r['target'],'true','false')
    return dict(type='F54_FIXED3071_RANGE_AUTHOR_ORIGIN',capability=capability,state=state,witness=witness,
        main_trace=str(trace_path),main_prefix=prefix,source_context=router_source_context(repo),
        ruling_sha256=J3071_RANGE_RULING_SHA,original_failed_trace_sha256=J3071_RANGE_FAILED_TRACE_SHA,
        side_invocation=dict(base=r['base'],target=r['side'],allow='true',partition='false'),admission=False)


def prepare_3071_range_origin(repo, capability_path, trace_path):
    capability=verified_3071_capabilities(repo,capability_path)
    raw=Path(trace_path).read_bytes();events=[json.loads(x) for x in raw.splitlines()]
    require(events and events[-1]['kind']=='frame_enter','fixed3071_range_before_any_engine_operation')
    frame=events[-1]['frame']
    state=validate_trace(repo,trace_path,sha(raw),range_checkpoint=frame)['range_checkpoint']
    require(state['capability_sha256']==sha(canonical(capability)),'fixed3071_range_exact_parent_capability')
    value=range_3071_origin_value(repo,capability,state,trace_path,dict(bytes=len(raw),sha256=sha(raw)))
    return dict(value,capability_path=str(capability_path))


def verify_3071_range_side(repo, origin, trace_path, map_path):
    r=J3071_RANGE;raw=Path(trace_path).read_bytes();events=[json.loads(x) for x in raw.splitlines()]
    require(events and events[0]['kind']=='range_side_link' and events[0]['payload']==origin,'fixed3071_supplemental_same_actual_origin')
    verified=validate_trace(repo,trace_path,sha(raw))
    require(not verified['failures'] and verified['frames']==1 and verified['operations']==6,'fixed3071_six_real_successful_side_operations')
    frame=next(e['payload'] for e in events if e['kind']=='frame_enter')
    require([frame[k] for k in ('base','target','allow_direct_followup','partition_source_history')]==[r['base'],r['side'],'true','false'],'fixed3071_exact_six_node_side_frame')
    beginnings=[e for e in events if e['kind']=='operation_begin']
    require([e['payload']['commit'] for e in beginnings]==r['side_nodes'] and all(e['payload']['classification']=='direct' for e in beginnings),'fixed3071_no_synthetic_or_net_only_side_operations')
    ledger=verified['ledger'][frame['id']];actual=Path(map_path).read_bytes()
    require(ledger['map_binding']==dict(bytes=len(actual),sha256=sha(actual)) and ledger['rows']==actual.decode().splitlines(),'fixed3071_actual_supplemental_final_map')
    return dict(type='F54_FIXED3071_RANGE_ENGINE_OBSERVATION',origin=origin,
        side_trace=str(trace_path),side_trace_sha256=sha(raw),side_map=str(map_path),side_map_binding=ledger['map_binding'],
        side_ledger=ledger,side_operations=[dict(operation=e['operation'],begin_digest=e['digest']) for e in beginnings],
        original_guard_status=1,route='EXACT_NINE_NODE_AUTHOR_OBSERVATION',qualified_nested_result=False,admission=False)


def prepare_3071_range_branch(repo, origin_path, trace_path, map_path):
    origin=json.loads(Path(origin_path).read_bytes());prefix=origin['main_prefix'];raw=Path(origin['main_trace']).read_bytes()
    require(len(raw)==prefix['bytes'] and sha(raw)==prefix['sha256'],'fixed3071_still_unchanged_main_prefix_before_engine')
    require(origin==prepare_3071_range_origin(repo,origin['capability_path'],origin['main_trace']),'fixed3071_current_range_origin_rechecked')
    return verify_3071_range_side(repo,origin,trace_path,map_path)


def J_history_coverage(repo, main_events, M_ready, range_branches, prior_span):
    """Actual operations/dispositions, never a Git-node-to-owner-row conversion."""
    nodes=git(repo,'rev-list','--topo-order','--reverse',J3071,'^'+J3071_O).decode().splitlines()
    require(nodes and nodes[-1]==J3071,'fixed3071_J_complete_history_before_integration');nodes.remove(J3071)
    references={n:[] for n in nodes}
    sources=[dict(kind='original_main',events=main_events,sha256=None)]
    external={}
    for ready in M_ready.values():external[ready['span_trace']]=('actual_M_span',ready['span_trace_sha256'])
    for branch in range_branches.values():external[branch['side_trace']]=('actual_I_range_side',branch['side_trace_sha256'])
    external[prior_span['trace']]=('actual_J_prior_span',prior_span['trace_sha256'])
    for path,(kind,digest) in external.items():
        raw=Path(path).read_bytes();require(sha(raw)==digest,'fixed3071_J_actual_supplemental_trace_hash')
        checked=validate_trace(repo,path,digest);require(not checked['failures'],'fixed3071_J_no_failed_supplemental_disposition')
        sources.append(dict(kind=kind,events=[json.loads(x) for x in raw.splitlines()],sha256=digest))
    for source in sources:
        for e in source['events']:
            if e['kind']!='operation_begin':continue
            value=e['payload'];node=value['commit']
            ref=dict(type='actual_operation',provenance=source['kind'],operation=e['operation'],begin_digest=e['digest'],
                trace_sha256=source['sha256'],parent_index=value['parent_index'],ordered_parents=value['ordered_parents'])
            if node in references:references[node].append(ref)
            node=value['source_head']
            if node in references and node!=value['commit']:
                actual=git(repo,'rev-list',value['boundary']+'..'+node).decode().splitlines()
                if actual==[node]:
                    references[node].append(dict(ref,type='exact_single_commit_source_of_actual_operation',
                        source_head=node,source_boundary=value['boundary'],ordered_source_parents=git(repo,'rev-list','--parents','-n','1',node).decode().split()[1:]))
    result=[]
    for node in nodes:
        refs=references[node];parents=git(repo,'rev-list','--parents','-n','1',node).decode().split()[1:]
        require(refs,'fixed3071_J_missing_real_history_disposition_'+node)
        actual=[v for v in refs if v['type']=='actual_operation']
        if actual:
            required=set(range(2,len(parents)+1)) if len(parents)>1 else {1}
            require(required<={v['parent_index'] for v in actual} and all(v['ordered_parents']==parents for v in actual),
                'fixed3071_J_all_actual_parent_slots_'+node)
        else:require(all(v['ordered_source_parents']==parents for v in refs),'fixed3071_J_exact_single_source_parent_facts')
        result.append(dict(node=node,ordered_parents=parents,dispositions=refs))
    return result


def prepare_3071_J_ledger(repo, capability_path, trace_path):
    cap=verified_3071_capabilities(repo,capability_path);raw=Path(trace_path).read_bytes()
    events=[json.loads(x) for x in raw.splitlines()]
    operations=[e['operation'] for e in events if e['kind']=='history_3071_capability'];require(len(operations)==1,'fixed3071_J_one_ledger_operation')
    state=validate_trace(repo,trace_path,sha(raw),J_checkpoint=operations[0])['J_checkpoint']
    require(state['capability_sha256']==sha(canonical(cap)),'fixed3071_J_same_current_capability')
    return build_3071_J_ready(state,file_digest(raw))


def build_3071_J_ready(state, prefix):
    """Transport of the independently reconstructed fixed-J checkpoint.

    Source/coverage qualification belongs to J_checkpoint_state, not supplied
    JSON or this assembly function. It never imports the supplemental map.
    """
    required={}
    require(state['operation_binding']['commit']==J3071 and state['operation'].split('/')[1:]==[J3071,'2'], 'fixed3071_J_transport_exact_operation')
    require(state['classification']['trigger'] in ('true','false'), 'fixed3071_J_raw_classifier_boolean')
    def item(name):
        return required.setdefault(name,dict(aggregate=False,retained_evidence=[],inherited_qualification=[],prior_span_evidence=[],J_classification=None))
    for frame,ledger in state['ledger'].items():
        for name,value in ledger['paths'].items():
            require(type(value['aggregate']) is bool,'fixed3071_J_retained_boolean')
            target=item(name);target['aggregate']|=value['aggregate']
            target['retained_evidence'].append(dict(frame=frame,map_binding=ledger['map_binding'],evidence=value['evidence']))
    for kind,table in (('M',state['M_ready']),('I',state['I_ready'])):
        for operation,ready in table.items():
            inherited_sha=None
            for name,value in ready['required_qualification'].items():
                require(type(value['aggregate']) is bool,'fixed3071_J_inherited_boolean')
                if inherited_sha is None:inherited_sha=sha(canonical(ready))
                target=item(name);target['aggregate']|=value['aggregate']
                target['inherited_qualification'].append(dict(kind=kind,operation=operation,ledger_sha256=inherited_sha,obligation=value))
    span=state['prior_span'];side=span['ledger']
    for name,value in side['paths'].items():
        require(type(value['aggregate']) is bool,'fixed3071_J_prior_span_boolean')
        target=item(name);target['aggregate']|=value['aggregate']
        target['prior_span_evidence'].append(dict(trace_sha256=span['trace_sha256'],map_binding=side['map_binding'],
            rows=[raw_binding(x) for x in side['rows'] if x.split('\t')[0]==name],evidence=value['evidence'],actual_operations=span['actual_operations']))
    for name in state['operation_binding']['paths']:
        target=item(name);target['J_classification']=state['classification'];target['aggregate']|=state['classification']['trigger']=='true'
    outer=state['ledger'][state['frame']]
    return dict(type='F54_FIXED3071_ACTUAL_J_COMPLETE_LEDGER',operation=state['operation'],frame=state['frame'],
        operation_binding=state['operation_binding'],classification=state['classification'],capability_sha256=state['capability_sha256'],
        prefix=prefix,frame_ancestry=state['frame_ancestry'],ledger=state['ledger'],coverage=state['coverage'],
        prior_span=span,inherited_M_ledgers=state['M_ready'],inherited_I_ledgers=state['I_ready'],composition=state['composition'],
        terminal_dispositions=terminal_dispositions(state['ledger']),manual_result_operations=manual_operations(state['ledger']),
        outer_rows=outer['rows'],outer_map_binding=outer['map_binding'],required_qualification=required,
        qualification='HOLD',admission=False,runtime_pass=False)


EXACT_J_AUDIT_PATH = 'src/backend/space/internal/grpcsvc/audit.go'
EXACT_J_AUDIT_PRIOR = '\t'.join((EXACT_J_AUDIT_PATH, 'false',
    '2445e81de07681cfd5f1491e4c793aaac747f571', '5cc425b35bc71738d86261c1cf4772c7babd4519',
    'e3dc9ba836100707a9c4313d83bea1872f1f9016', 'c14f59c1a43b486d48322d62e7c5baa2816e1253',
    'c6225ae5b968345b04a2c7d410875fe7d370b300', 'e3dc9ba836100707a9c4313d83bea1872f1f9016'))
EXACT_J_AUDIT_INCOMING = '\t'.join((EXACT_J_AUDIT_PATH, 'true', J3071, J3071_P1,
    J3071_O, J3071_P2, 'c6225ae5b968345b04a2c7d410875fe7d370b300', ','.join(J3071_BASES)))


# Reviewed bytes are embedded so a four-file AUTHOR carrier needs no new
# transport file. The unchanged testdata copy is the review artifact.
EXACT_J_SOURCE_UNIT_RAW = r'''{
  "schema": "F54-EXACT-J-AUDIT-SOURCE-UNIT-PROPOSED-v1",
  "permission": "PROPOSED_NOT_GRANTED",
  "path": "src/backend/space/internal/grpcsvc/audit.go",
  "versions": {
    "prior_owner": {
      "ref": "2445e81de07681cfd5f1491e4c793aaac747f571",
      "tree_record": "100644 blob d24e08d221c42661e8b1431defe61d742b23e8d9\tsrc/backend/space/internal/grpcsvc/audit.go\u0000",
      "bytes": 3560,
      "sha256": "b358710da05deb7b9aabaed5d44b12104c3415da182d222251f7504285c031ba",
      "present": true,
      "ordered_parents": [
        "5cc425b35bc71738d86261c1cf4772c7babd4519",
        "c14f59c1a43b486d48322d62e7c5baa2816e1253"
      ]
    },
    "prior_first": {
      "ref": "5cc425b35bc71738d86261c1cf4772c7babd4519",
      "tree_record": "100644 blob 4a3d066e9642e7eaca6e58761f79726e838d599d\tsrc/backend/space/internal/grpcsvc/audit.go\u0000",
      "bytes": 3420,
      "sha256": "a2f43085a036752c31e6cf4a298dc3a2a3fb63e5247cc1c14094d53d5eacb6a8",
      "present": true,
      "ordered_parents": [
        "2cc9482255ba18cd948aacb567f4c4bdf8976b14"
      ]
    },
    "prior_base": {
      "ref": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
      "tree_record": "100644 blob 4a3d066e9642e7eaca6e58761f79726e838d599d\tsrc/backend/space/internal/grpcsvc/audit.go\u0000",
      "bytes": 3420,
      "sha256": "a2f43085a036752c31e6cf4a298dc3a2a3fb63e5247cc1c14094d53d5eacb6a8",
      "present": true,
      "ordered_parents": [
        "420c4c7be8c64583ab3e26f0560e5970f705a46e",
        "34640fde433c13073246003b1d009d2c7dba530c"
      ]
    },
    "prior_source": {
      "ref": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
      "tree_record": "100644 blob d24e08d221c42661e8b1431defe61d742b23e8d9\tsrc/backend/space/internal/grpcsvc/audit.go\u0000",
      "bytes": 3560,
      "sha256": "b358710da05deb7b9aabaed5d44b12104c3415da182d222251f7504285c031ba",
      "present": true,
      "ordered_parents": [
        "18206e47ff06f1071f69319e9a6eeddf29f079b1"
      ]
    },
    "content": {
      "ref": "c6225ae5b968345b04a2c7d410875fe7d370b300",
      "tree_record": "100644 blob 5cbf6687439bee5cba8182ac052483741d9ffcd8\tsrc/backend/space/internal/grpcsvc/audit.go\u0000",
      "bytes": 2546,
      "sha256": "361532fd6bceb1b74896e6ef8a2999b49146ac8a7c58b2a188945b34635baa85",
      "present": true,
      "ordered_parents": [
        "85b036f7a44d199cfc4f5d99e569bfc2140b0494",
        "6b568d3cc5469a8a597e11f8e4acbcc6ad1d9df0"
      ]
    },
    "O": {
      "ref": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
      "tree_record": "100644 blob 4a3d066e9642e7eaca6e58761f79726e838d599d\tsrc/backend/space/internal/grpcsvc/audit.go\u0000",
      "bytes": 3420,
      "sha256": "a2f43085a036752c31e6cf4a298dc3a2a3fb63e5247cc1c14094d53d5eacb6a8",
      "present": true,
      "ordered_parents": [
        "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe"
      ]
    },
    "bestA": {
      "ref": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
      "tree_record": "100644 blob d24e08d221c42661e8b1431defe61d742b23e8d9\tsrc/backend/space/internal/grpcsvc/audit.go\u0000",
      "bytes": 3560,
      "sha256": "b358710da05deb7b9aabaed5d44b12104c3415da182d222251f7504285c031ba",
      "present": true,
      "ordered_parents": [
        "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
        "f954480f69878a5b3890bf6fcb5af978d68dadf9"
      ]
    },
    "bestB": {
      "ref": "e020da4b4edcded41bde9785615262beb1946ad2",
      "tree_record": "100644 blob 4a3d066e9642e7eaca6e58761f79726e838d599d\tsrc/backend/space/internal/grpcsvc/audit.go\u0000",
      "bytes": 3420,
      "sha256": "a2f43085a036752c31e6cf4a298dc3a2a3fb63e5247cc1c14094d53d5eacb6a8",
      "present": true,
      "ordered_parents": [
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172"
      ]
    },
    "P1": {
      "ref": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
      "tree_record": "100644 blob d24e08d221c42661e8b1431defe61d742b23e8d9\tsrc/backend/space/internal/grpcsvc/audit.go\u0000",
      "bytes": 3560,
      "sha256": "b358710da05deb7b9aabaed5d44b12104c3415da182d222251f7504285c031ba",
      "present": true,
      "ordered_parents": [
        "dc0acdefec7e5c413988adf71f86d12bff4017bc",
        "6780407299a3beb77e92b32c551ca7cfe3e63e94"
      ]
    },
    "P2": {
      "ref": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
      "tree_record": "100644 blob d24e08d221c42661e8b1431defe61d742b23e8d9\tsrc/backend/space/internal/grpcsvc/audit.go\u0000",
      "bytes": 3560,
      "sha256": "b358710da05deb7b9aabaed5d44b12104c3415da182d222251f7504285c031ba",
      "present": true,
      "ordered_parents": [
        "48068711871938ee3a5b1ba04b9efaaf8dc56a74"
      ]
    },
    "J": {
      "ref": "3071ddad390443de4119c796cdad9899ff08f7fd",
      "tree_record": "100644 blob d24e08d221c42661e8b1431defe61d742b23e8d9\tsrc/backend/space/internal/grpcsvc/audit.go\u0000",
      "bytes": 3560,
      "sha256": "b358710da05deb7b9aabaed5d44b12104c3415da182d222251f7504285c031ba",
      "present": true,
      "ordered_parents": [
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd"
      ]
    },
    "H53": {
      "ref": "53f75f74523735ac594e21582bea659f567a7288",
      "tree_record": "100644 blob d24e08d221c42661e8b1431defe61d742b23e8d9\tsrc/backend/space/internal/grpcsvc/audit.go\u0000",
      "bytes": 3560,
      "sha256": "b358710da05deb7b9aabaed5d44b12104c3415da182d222251f7504285c031ba",
      "present": true,
      "ordered_parents": [
        "d06ace9e7d355bffe421a6418e9a4951dd096b8f"
      ]
    }
  },
  "edges": {
    "prior_integration": {
      "from": "5cc425b35bc71738d86261c1cf4772c7babd4519",
      "to": "2445e81de07681cfd5f1491e4c793aaac747f571",
      "raw_sha256": "047e26da4dd41806604d13e3c116e2c226e3e0c39205f8f997c4096df5eecfbc",
      "patch_sha256": "17b60cb9c92af8781226d29d8525e8ac043153e9d1a510ae0b09cba54bcea8c2"
    },
    "prior_source_change": {
      "from": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
      "to": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
      "raw_sha256": "047e26da4dd41806604d13e3c116e2c226e3e0c39205f8f997c4096df5eecfbc",
      "patch_sha256": "17b60cb9c92af8781226d29d8525e8ac043153e9d1a510ae0b09cba54bcea8c2"
    },
    "P1_to_J": {
      "from": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
      "to": "3071ddad390443de4119c796cdad9899ff08f7fd",
      "raw_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "patch_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    },
    "P2_to_J": {
      "from": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
      "to": "3071ddad390443de4119c796cdad9899ff08f7fd",
      "raw_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "patch_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    },
    "O_to_P2": {
      "from": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
      "to": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
      "raw_sha256": "047e26da4dd41806604d13e3c116e2c226e3e0c39205f8f997c4096df5eecfbc",
      "patch_sha256": "17b60cb9c92af8781226d29d8525e8ac043153e9d1a510ae0b09cba54bcea8c2"
    },
    "earliest_to_prior": {
      "from": "c6225ae5b968345b04a2c7d410875fe7d370b300",
      "to": "2445e81de07681cfd5f1491e4c793aaac747f571",
      "raw_sha256": "e873557ade01bd23336898df8f92b5accc01c89a7ee24f383b9ee09cc15f9420",
      "patch_sha256": "c8a6d3d96ab0af4a2e0216e337e27ebc772a089957192e0a1495dca8a058fb9b"
    }
  },
  "source_semantic_review": "PENDING independent CONTRACT review; immutable Git bytes are facts, not a self-qualified grant",
  "historical_caller": "698/H53 true/false",
  "source547_target": "c6a7876239ddbf70ac1d084c64a1e3f95eeda846",
  "product_changed": false
}
'''.encode()


FINITE_J_SOURCE_UNIT_RAW = r'''{
  "admission": false,
  "best_bases": [
    "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
    "e020da4b4edcded41bde9785615262beb1946ad2"
  ],
  "cases": [
    {
      "edges": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1..c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "after": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "before": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "2cc9482255ba18cd948aacb567f4c4bdf8976b14..5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "after": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "before": "2cc9482255ba18cd948aacb567f4c4bdf8976b14",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "34640fde433c13073246003b1d009d2c7dba530c..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "34640fde433c13073246003b1d009d2c7dba530c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "420c4c7be8c64583ab3e26f0560e5970f705a46e..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "420c4c7be8c64583ab3e26f0560e5970f705a46e",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "48068711871938ee3a5b1ba04b9efaaf8dc56a74..140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "after": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "before": "48068711871938ee3a5b1ba04b9efaaf8dc56a74",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "patch": {
            "bytes": 2328,
            "sha256": "50d06053180eef254e072a9c7900f8c717bd7c01a58fee64f3574f989ec7e346"
          },
          "raw": {
            "bytes": 157,
            "sha256": "ae95005d8d40e7476266d921ab30fb72965a34cccb4ad0468ede742fd56d69ee"
          }
        },
        "6780407299a3beb77e92b32c551ca7cfe3e63e94..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "6780407299a3beb77e92b32c551ca7cfe3e63e94",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172..e020da4b4edcded41bde9785615262beb1946ad2": {
          "after": "e020da4b4edcded41bde9785615262beb1946ad2",
          "before": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe..7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "after": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "before": "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "dc0acdefec7e5c413988adf71f86d12bff4017bc..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "dc0acdefec7e5c413988adf71f86d12bff4017bc",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016..18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "after": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "before": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "patch": {
            "bytes": 2328,
            "sha256": "50d06053180eef254e072a9c7900f8c717bd7c01a58fee64f3574f989ec7e346"
          },
          "raw": {
            "bytes": 157,
            "sha256": "ae95005d8d40e7476266d921ab30fb72965a34cccb4ad0468ede742fd56d69ee"
          }
        },
        "f954480f69878a5b3890bf6fcb5af978d68dadf9..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "f954480f69878a5b3890bf6fcb5af978d68dadf9",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        }
      },
      "incoming": "src/backend/space/internal/grpcsvc/audit_identity_test.go\ttrue\t3071ddad390443de4119c796cdad9899ff08f7fd\ta45862ca7e2e885ae4e36afc758c9e9fe39ca86c\t7bd7dfba7e8f85bc0572617c6ef3af4a15e59172\t140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd\t5cc425b35bc71738d86261c1cf4772c7babd4519\t68927e5912ebc218050fcc9daf4b76cf4a9e3c86,e020da4b4edcded41bde9785615262beb1946ad2",
      "path": "src/backend/space/internal/grpcsvc/audit_identity_test.go",
      "prior": "src/backend/space/internal/grpcsvc/audit_identity_test.go\tfalse\t2445e81de07681cfd5f1491e4c793aaac747f571\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016\tc14f59c1a43b486d48322d62e7c5baa2816e1253\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016",
      "versions": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "content": {
            "bytes": 2009,
            "sha256": "e3559f512c0e701c676125ddde7968dedbbd8426bb9aa0048ed1ddb8b1b6f6bd"
          },
          "ordered_parents": [
            "48068711871938ee3a5b1ba04b9efaaf8dc56a74"
          ],
          "tree_record": "100644 blob 788a201984a41d690cd478f74cedc9f8c6334372\tsrc/backend/space/internal/grpcsvc/audit_identity_test.go\u0000"
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "content": {
            "bytes": 2009,
            "sha256": "e3559f512c0e701c676125ddde7968dedbbd8426bb9aa0048ed1ddb8b1b6f6bd"
          },
          "ordered_parents": [
            "e3dc9ba836100707a9c4313d83bea1872f1f9016"
          ],
          "tree_record": "100644 blob 788a201984a41d690cd478f74cedc9f8c6334372\tsrc/backend/space/internal/grpcsvc/audit_identity_test.go\u0000"
        },
        "2445e81de07681cfd5f1491e4c793aaac747f571": {
          "content": {
            "bytes": 2009,
            "sha256": "e3559f512c0e701c676125ddde7968dedbbd8426bb9aa0048ed1ddb8b1b6f6bd"
          },
          "ordered_parents": [
            "5cc425b35bc71738d86261c1cf4772c7babd4519",
            "c14f59c1a43b486d48322d62e7c5baa2816e1253"
          ],
          "tree_record": "100644 blob 788a201984a41d690cd478f74cedc9f8c6334372\tsrc/backend/space/internal/grpcsvc/audit_identity_test.go\u0000"
        },
        "3071ddad390443de4119c796cdad9899ff08f7fd": {
          "content": {
            "bytes": 2009,
            "sha256": "e3559f512c0e701c676125ddde7968dedbbd8426bb9aa0048ed1ddb8b1b6f6bd"
          },
          "ordered_parents": [
            "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
            "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd"
          ],
          "tree_record": "100644 blob 788a201984a41d690cd478f74cedc9f8c6334372\tsrc/backend/space/internal/grpcsvc/audit_identity_test.go\u0000"
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "content": null,
          "ordered_parents": [
            "2cc9482255ba18cd948aacb567f4c4bdf8976b14"
          ],
          "tree_record": ""
        },
        "68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "content": {
            "bytes": 2009,
            "sha256": "e3559f512c0e701c676125ddde7968dedbbd8426bb9aa0048ed1ddb8b1b6f6bd"
          },
          "ordered_parents": [
            "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
            "f954480f69878a5b3890bf6fcb5af978d68dadf9"
          ],
          "tree_record": "100644 blob 788a201984a41d690cd478f74cedc9f8c6334372\tsrc/backend/space/internal/grpcsvc/audit_identity_test.go\u0000"
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "content": null,
          "ordered_parents": [
            "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe"
          ],
          "tree_record": ""
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "content": {
            "bytes": 2009,
            "sha256": "e3559f512c0e701c676125ddde7968dedbbd8426bb9aa0048ed1ddb8b1b6f6bd"
          },
          "ordered_parents": [
            "dc0acdefec7e5c413988adf71f86d12bff4017bc",
            "6780407299a3beb77e92b32c551ca7cfe3e63e94"
          ],
          "tree_record": "100644 blob 788a201984a41d690cd478f74cedc9f8c6334372\tsrc/backend/space/internal/grpcsvc/audit_identity_test.go\u0000"
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "content": {
            "bytes": 2009,
            "sha256": "e3559f512c0e701c676125ddde7968dedbbd8426bb9aa0048ed1ddb8b1b6f6bd"
          },
          "ordered_parents": [
            "18206e47ff06f1071f69319e9a6eeddf29f079b1"
          ],
          "tree_record": "100644 blob 788a201984a41d690cd478f74cedc9f8c6334372\tsrc/backend/space/internal/grpcsvc/audit_identity_test.go\u0000"
        },
        "e020da4b4edcded41bde9785615262beb1946ad2": {
          "content": null,
          "ordered_parents": [
            "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172"
          ],
          "tree_record": ""
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "content": null,
          "ordered_parents": [
            "420c4c7be8c64583ab3e26f0560e5970f705a46e",
            "34640fde433c13073246003b1d009d2c7dba530c"
          ],
          "tree_record": ""
        }
      }
    },
    {
      "edges": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1..c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "after": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "before": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "2cc9482255ba18cd948aacb567f4c4bdf8976b14..5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "after": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "before": "2cc9482255ba18cd948aacb567f4c4bdf8976b14",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "34640fde433c13073246003b1d009d2c7dba530c..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "34640fde433c13073246003b1d009d2c7dba530c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "420c4c7be8c64583ab3e26f0560e5970f705a46e..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "420c4c7be8c64583ab3e26f0560e5970f705a46e",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "48068711871938ee3a5b1ba04b9efaaf8dc56a74..140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "after": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "before": "48068711871938ee3a5b1ba04b9efaaf8dc56a74",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "patch": {
            "bytes": 1517,
            "sha256": "135a120c9e53805b7585e4f505a4a04c78f24838747cbd7f7dad9e74c3f73416"
          },
          "raw": {
            "bytes": 160,
            "sha256": "718ba7eac9d19bad03e360b7b9cea22367af483b8604963784d4bcb7d62b1118"
          }
        },
        "6780407299a3beb77e92b32c551ca7cfe3e63e94..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "6780407299a3beb77e92b32c551ca7cfe3e63e94",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172..e020da4b4edcded41bde9785615262beb1946ad2": {
          "after": "e020da4b4edcded41bde9785615262beb1946ad2",
          "before": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe..7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "after": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "before": "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "dc0acdefec7e5c413988adf71f86d12bff4017bc..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "dc0acdefec7e5c413988adf71f86d12bff4017bc",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016..18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "after": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "before": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "patch": {
            "bytes": 1517,
            "sha256": "135a120c9e53805b7585e4f505a4a04c78f24838747cbd7f7dad9e74c3f73416"
          },
          "raw": {
            "bytes": 160,
            "sha256": "718ba7eac9d19bad03e360b7b9cea22367af483b8604963784d4bcb7d62b1118"
          }
        },
        "f954480f69878a5b3890bf6fcb5af978d68dadf9..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "f954480f69878a5b3890bf6fcb5af978d68dadf9",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        }
      },
      "incoming": "src/backend/space/internal/grpcsvc/audit_integration_test.go\ttrue\t3071ddad390443de4119c796cdad9899ff08f7fd\ta45862ca7e2e885ae4e36afc758c9e9fe39ca86c\t7bd7dfba7e8f85bc0572617c6ef3af4a15e59172\t140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd\t5cc425b35bc71738d86261c1cf4772c7babd4519\t68927e5912ebc218050fcc9daf4b76cf4a9e3c86,e020da4b4edcded41bde9785615262beb1946ad2",
      "path": "src/backend/space/internal/grpcsvc/audit_integration_test.go",
      "prior": "src/backend/space/internal/grpcsvc/audit_integration_test.go\tfalse\t2445e81de07681cfd5f1491e4c793aaac747f571\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016\tc14f59c1a43b486d48322d62e7c5baa2816e1253\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016",
      "versions": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "content": {
            "bytes": 19576,
            "sha256": "5f8c49d1cccda996b83ed050e782aab277c77def1ddccf7fc70795ab46e239f2"
          },
          "ordered_parents": [
            "48068711871938ee3a5b1ba04b9efaaf8dc56a74"
          ],
          "tree_record": "100644 blob 8440d9a59c0dab16b9359c7270b62bcc2921581c\tsrc/backend/space/internal/grpcsvc/audit_integration_test.go\u0000"
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "content": {
            "bytes": 19576,
            "sha256": "5f8c49d1cccda996b83ed050e782aab277c77def1ddccf7fc70795ab46e239f2"
          },
          "ordered_parents": [
            "e3dc9ba836100707a9c4313d83bea1872f1f9016"
          ],
          "tree_record": "100644 blob 8440d9a59c0dab16b9359c7270b62bcc2921581c\tsrc/backend/space/internal/grpcsvc/audit_integration_test.go\u0000"
        },
        "2445e81de07681cfd5f1491e4c793aaac747f571": {
          "content": {
            "bytes": 19576,
            "sha256": "5f8c49d1cccda996b83ed050e782aab277c77def1ddccf7fc70795ab46e239f2"
          },
          "ordered_parents": [
            "5cc425b35bc71738d86261c1cf4772c7babd4519",
            "c14f59c1a43b486d48322d62e7c5baa2816e1253"
          ],
          "tree_record": "100644 blob 8440d9a59c0dab16b9359c7270b62bcc2921581c\tsrc/backend/space/internal/grpcsvc/audit_integration_test.go\u0000"
        },
        "3071ddad390443de4119c796cdad9899ff08f7fd": {
          "content": {
            "bytes": 19576,
            "sha256": "5f8c49d1cccda996b83ed050e782aab277c77def1ddccf7fc70795ab46e239f2"
          },
          "ordered_parents": [
            "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
            "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd"
          ],
          "tree_record": "100644 blob 8440d9a59c0dab16b9359c7270b62bcc2921581c\tsrc/backend/space/internal/grpcsvc/audit_integration_test.go\u0000"
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "content": {
            "bytes": 19619,
            "sha256": "974200d9d7ad96468963c0e643f6fce9d140e698fdc867abeadf601ac310b031"
          },
          "ordered_parents": [
            "2cc9482255ba18cd948aacb567f4c4bdf8976b14"
          ],
          "tree_record": "100644 blob a5199fd55af386319badc84daf38383ae06b1677\tsrc/backend/space/internal/grpcsvc/audit_integration_test.go\u0000"
        },
        "68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "content": {
            "bytes": 19576,
            "sha256": "5f8c49d1cccda996b83ed050e782aab277c77def1ddccf7fc70795ab46e239f2"
          },
          "ordered_parents": [
            "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
            "f954480f69878a5b3890bf6fcb5af978d68dadf9"
          ],
          "tree_record": "100644 blob 8440d9a59c0dab16b9359c7270b62bcc2921581c\tsrc/backend/space/internal/grpcsvc/audit_integration_test.go\u0000"
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "content": {
            "bytes": 19619,
            "sha256": "974200d9d7ad96468963c0e643f6fce9d140e698fdc867abeadf601ac310b031"
          },
          "ordered_parents": [
            "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe"
          ],
          "tree_record": "100644 blob a5199fd55af386319badc84daf38383ae06b1677\tsrc/backend/space/internal/grpcsvc/audit_integration_test.go\u0000"
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "content": {
            "bytes": 19576,
            "sha256": "5f8c49d1cccda996b83ed050e782aab277c77def1ddccf7fc70795ab46e239f2"
          },
          "ordered_parents": [
            "dc0acdefec7e5c413988adf71f86d12bff4017bc",
            "6780407299a3beb77e92b32c551ca7cfe3e63e94"
          ],
          "tree_record": "100644 blob 8440d9a59c0dab16b9359c7270b62bcc2921581c\tsrc/backend/space/internal/grpcsvc/audit_integration_test.go\u0000"
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "content": {
            "bytes": 19576,
            "sha256": "5f8c49d1cccda996b83ed050e782aab277c77def1ddccf7fc70795ab46e239f2"
          },
          "ordered_parents": [
            "18206e47ff06f1071f69319e9a6eeddf29f079b1"
          ],
          "tree_record": "100644 blob 8440d9a59c0dab16b9359c7270b62bcc2921581c\tsrc/backend/space/internal/grpcsvc/audit_integration_test.go\u0000"
        },
        "e020da4b4edcded41bde9785615262beb1946ad2": {
          "content": {
            "bytes": 19619,
            "sha256": "974200d9d7ad96468963c0e643f6fce9d140e698fdc867abeadf601ac310b031"
          },
          "ordered_parents": [
            "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172"
          ],
          "tree_record": "100644 blob a5199fd55af386319badc84daf38383ae06b1677\tsrc/backend/space/internal/grpcsvc/audit_integration_test.go\u0000"
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "content": {
            "bytes": 19619,
            "sha256": "974200d9d7ad96468963c0e643f6fce9d140e698fdc867abeadf601ac310b031"
          },
          "ordered_parents": [
            "420c4c7be8c64583ab3e26f0560e5970f705a46e",
            "34640fde433c13073246003b1d009d2c7dba530c"
          ],
          "tree_record": "100644 blob a5199fd55af386319badc84daf38383ae06b1677\tsrc/backend/space/internal/grpcsvc/audit_integration_test.go\u0000"
        }
      }
    },
    {
      "edges": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1..c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "after": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "before": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "2cc9482255ba18cd948aacb567f4c4bdf8976b14..5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "after": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "before": "2cc9482255ba18cd948aacb567f4c4bdf8976b14",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "34640fde433c13073246003b1d009d2c7dba530c..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "34640fde433c13073246003b1d009d2c7dba530c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "420c4c7be8c64583ab3e26f0560e5970f705a46e..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "420c4c7be8c64583ab3e26f0560e5970f705a46e",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "48068711871938ee3a5b1ba04b9efaaf8dc56a74..140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "after": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "before": "48068711871938ee3a5b1ba04b9efaaf8dc56a74",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "patch": {
            "bytes": 1508,
            "sha256": "fb8100c0cc3e3755a456d1718df1153079081ba54b06f87cfe8b3079206bdcc8"
          },
          "raw": {
            "bytes": 155,
            "sha256": "67de627e08ca0675489d1bb1b9b89fa6e83cc03509cb3e5298b70a5b8b60962c"
          }
        },
        "6780407299a3beb77e92b32c551ca7cfe3e63e94..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "6780407299a3beb77e92b32c551ca7cfe3e63e94",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "6b568d3cc5469a8a597e11f8e4acbcc6ad1d9df0..c6225ae5b968345b04a2c7d410875fe7d370b300": {
          "after": "c6225ae5b968345b04a2c7d410875fe7d370b300",
          "before": "6b568d3cc5469a8a597e11f8e4acbcc6ad1d9df0",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172..e020da4b4edcded41bde9785615262beb1946ad2": {
          "after": "e020da4b4edcded41bde9785615262beb1946ad2",
          "before": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "85b036f7a44d199cfc4f5d99e569bfc2140b0494..c6225ae5b968345b04a2c7d410875fe7d370b300": {
          "after": "c6225ae5b968345b04a2c7d410875fe7d370b300",
          "before": "85b036f7a44d199cfc4f5d99e569bfc2140b0494",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe..7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "after": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "before": "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "dc0acdefec7e5c413988adf71f86d12bff4017bc..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "dc0acdefec7e5c413988adf71f86d12bff4017bc",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016..18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "after": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "before": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "patch": {
            "bytes": 1508,
            "sha256": "fb8100c0cc3e3755a456d1718df1153079081ba54b06f87cfe8b3079206bdcc8"
          },
          "raw": {
            "bytes": 155,
            "sha256": "67de627e08ca0675489d1bb1b9b89fa6e83cc03509cb3e5298b70a5b8b60962c"
          }
        },
        "f954480f69878a5b3890bf6fcb5af978d68dadf9..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "f954480f69878a5b3890bf6fcb5af978d68dadf9",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        }
      },
      "incoming": "src/backend/space/internal/grpcsvc/audit_public_read.go\ttrue\t3071ddad390443de4119c796cdad9899ff08f7fd\ta45862ca7e2e885ae4e36afc758c9e9fe39ca86c\t7bd7dfba7e8f85bc0572617c6ef3af4a15e59172\t140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd\tc6225ae5b968345b04a2c7d410875fe7d370b300\t68927e5912ebc218050fcc9daf4b76cf4a9e3c86,e020da4b4edcded41bde9785615262beb1946ad2",
      "path": "src/backend/space/internal/grpcsvc/audit_public_read.go",
      "prior": "src/backend/space/internal/grpcsvc/audit_public_read.go\tfalse\t2445e81de07681cfd5f1491e4c793aaac747f571\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016\tc14f59c1a43b486d48322d62e7c5baa2816e1253\tc6225ae5b968345b04a2c7d410875fe7d370b300\te3dc9ba836100707a9c4313d83bea1872f1f9016",
      "versions": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "content": {
            "bytes": 2449,
            "sha256": "a7287b6a92acd20228fa8ba579075f21de9d9279d55c7261f7d42fd504d29e02"
          },
          "ordered_parents": [
            "48068711871938ee3a5b1ba04b9efaaf8dc56a74"
          ],
          "tree_record": "100644 blob 1e83d56c2d657a4c213327c1937120fccaeded77\tsrc/backend/space/internal/grpcsvc/audit_public_read.go\u0000"
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "content": {
            "bytes": 2449,
            "sha256": "a7287b6a92acd20228fa8ba579075f21de9d9279d55c7261f7d42fd504d29e02"
          },
          "ordered_parents": [
            "e3dc9ba836100707a9c4313d83bea1872f1f9016"
          ],
          "tree_record": "100644 blob 1e83d56c2d657a4c213327c1937120fccaeded77\tsrc/backend/space/internal/grpcsvc/audit_public_read.go\u0000"
        },
        "2445e81de07681cfd5f1491e4c793aaac747f571": {
          "content": {
            "bytes": 2449,
            "sha256": "a7287b6a92acd20228fa8ba579075f21de9d9279d55c7261f7d42fd504d29e02"
          },
          "ordered_parents": [
            "5cc425b35bc71738d86261c1cf4772c7babd4519",
            "c14f59c1a43b486d48322d62e7c5baa2816e1253"
          ],
          "tree_record": "100644 blob 1e83d56c2d657a4c213327c1937120fccaeded77\tsrc/backend/space/internal/grpcsvc/audit_public_read.go\u0000"
        },
        "3071ddad390443de4119c796cdad9899ff08f7fd": {
          "content": {
            "bytes": 2449,
            "sha256": "a7287b6a92acd20228fa8ba579075f21de9d9279d55c7261f7d42fd504d29e02"
          },
          "ordered_parents": [
            "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
            "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd"
          ],
          "tree_record": "100644 blob 1e83d56c2d657a4c213327c1937120fccaeded77\tsrc/backend/space/internal/grpcsvc/audit_public_read.go\u0000"
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "content": {
            "bytes": 2599,
            "sha256": "67f3415e221d035d9db7233b02b3939a9dee04b86cc1951954edb6fc4e4ad443"
          },
          "ordered_parents": [
            "2cc9482255ba18cd948aacb567f4c4bdf8976b14"
          ],
          "tree_record": "100644 blob 86f67140519af94e0139aca25122c34f04dde10a\tsrc/backend/space/internal/grpcsvc/audit_public_read.go\u0000"
        },
        "68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "content": {
            "bytes": 2449,
            "sha256": "a7287b6a92acd20228fa8ba579075f21de9d9279d55c7261f7d42fd504d29e02"
          },
          "ordered_parents": [
            "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
            "f954480f69878a5b3890bf6fcb5af978d68dadf9"
          ],
          "tree_record": "100644 blob 1e83d56c2d657a4c213327c1937120fccaeded77\tsrc/backend/space/internal/grpcsvc/audit_public_read.go\u0000"
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "content": {
            "bytes": 2599,
            "sha256": "67f3415e221d035d9db7233b02b3939a9dee04b86cc1951954edb6fc4e4ad443"
          },
          "ordered_parents": [
            "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe"
          ],
          "tree_record": "100644 blob 86f67140519af94e0139aca25122c34f04dde10a\tsrc/backend/space/internal/grpcsvc/audit_public_read.go\u0000"
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "content": {
            "bytes": 2449,
            "sha256": "a7287b6a92acd20228fa8ba579075f21de9d9279d55c7261f7d42fd504d29e02"
          },
          "ordered_parents": [
            "dc0acdefec7e5c413988adf71f86d12bff4017bc",
            "6780407299a3beb77e92b32c551ca7cfe3e63e94"
          ],
          "tree_record": "100644 blob 1e83d56c2d657a4c213327c1937120fccaeded77\tsrc/backend/space/internal/grpcsvc/audit_public_read.go\u0000"
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "content": {
            "bytes": 2449,
            "sha256": "a7287b6a92acd20228fa8ba579075f21de9d9279d55c7261f7d42fd504d29e02"
          },
          "ordered_parents": [
            "18206e47ff06f1071f69319e9a6eeddf29f079b1"
          ],
          "tree_record": "100644 blob 1e83d56c2d657a4c213327c1937120fccaeded77\tsrc/backend/space/internal/grpcsvc/audit_public_read.go\u0000"
        },
        "c6225ae5b968345b04a2c7d410875fe7d370b300": {
          "content": null,
          "ordered_parents": [
            "85b036f7a44d199cfc4f5d99e569bfc2140b0494",
            "6b568d3cc5469a8a597e11f8e4acbcc6ad1d9df0"
          ],
          "tree_record": ""
        },
        "e020da4b4edcded41bde9785615262beb1946ad2": {
          "content": {
            "bytes": 2599,
            "sha256": "67f3415e221d035d9db7233b02b3939a9dee04b86cc1951954edb6fc4e4ad443"
          },
          "ordered_parents": [
            "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172"
          ],
          "tree_record": "100644 blob 86f67140519af94e0139aca25122c34f04dde10a\tsrc/backend/space/internal/grpcsvc/audit_public_read.go\u0000"
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "content": {
            "bytes": 2599,
            "sha256": "67f3415e221d035d9db7233b02b3939a9dee04b86cc1951954edb6fc4e4ad443"
          },
          "ordered_parents": [
            "420c4c7be8c64583ab3e26f0560e5970f705a46e",
            "34640fde433c13073246003b1d009d2c7dba530c"
          ],
          "tree_record": "100644 blob 86f67140519af94e0139aca25122c34f04dde10a\tsrc/backend/space/internal/grpcsvc/audit_public_read.go\u0000"
        }
      }
    },
    {
      "edges": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1..c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "after": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "before": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "2cc9482255ba18cd948aacb567f4c4bdf8976b14..5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "after": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "before": "2cc9482255ba18cd948aacb567f4c4bdf8976b14",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "34640fde433c13073246003b1d009d2c7dba530c..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "34640fde433c13073246003b1d009d2c7dba530c",
          "patch": {
            "bytes": 1157,
            "sha256": "c5a7611025eee66492b006d6786d8654965631753254a5f1c1e90cee1f65aaa3"
          },
          "raw": {
            "bytes": 160,
            "sha256": "cc93289ebdb24c83979adb7285b3df782e75b85d60f127fed2ec34684b8134da"
          }
        },
        "420c4c7be8c64583ab3e26f0560e5970f705a46e..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "420c4c7be8c64583ab3e26f0560e5970f705a46e",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "48068711871938ee3a5b1ba04b9efaaf8dc56a74..140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "after": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "before": "48068711871938ee3a5b1ba04b9efaaf8dc56a74",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "patch": {
            "bytes": 1853,
            "sha256": "b000f7abe474c1cc978872f80d75ad0ea72e570f4490c33091b35056da3992ed"
          },
          "raw": {
            "bytes": 160,
            "sha256": "eb24155272d6101969578cf8feafb49b79be0fc1ae0718aa6aa4a65f8c02bf3d"
          }
        },
        "6780407299a3beb77e92b32c551ca7cfe3e63e94..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "6780407299a3beb77e92b32c551ca7cfe3e63e94",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "6b568d3cc5469a8a597e11f8e4acbcc6ad1d9df0..c6225ae5b968345b04a2c7d410875fe7d370b300": {
          "after": "c6225ae5b968345b04a2c7d410875fe7d370b300",
          "before": "6b568d3cc5469a8a597e11f8e4acbcc6ad1d9df0",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172..e020da4b4edcded41bde9785615262beb1946ad2": {
          "after": "e020da4b4edcded41bde9785615262beb1946ad2",
          "before": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "85b036f7a44d199cfc4f5d99e569bfc2140b0494..c6225ae5b968345b04a2c7d410875fe7d370b300": {
          "after": "c6225ae5b968345b04a2c7d410875fe7d370b300",
          "before": "85b036f7a44d199cfc4f5d99e569bfc2140b0494",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe..7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "after": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "before": "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "dc0acdefec7e5c413988adf71f86d12bff4017bc..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "dc0acdefec7e5c413988adf71f86d12bff4017bc",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016..18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "after": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "before": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "patch": {
            "bytes": 1853,
            "sha256": "b000f7abe474c1cc978872f80d75ad0ea72e570f4490c33091b35056da3992ed"
          },
          "raw": {
            "bytes": 160,
            "sha256": "eb24155272d6101969578cf8feafb49b79be0fc1ae0718aa6aa4a65f8c02bf3d"
          }
        },
        "f954480f69878a5b3890bf6fcb5af978d68dadf9..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "f954480f69878a5b3890bf6fcb5af978d68dadf9",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        }
      },
      "incoming": "src/backend/space/internal/grpcsvc/audit_public_read_test.go\ttrue\t3071ddad390443de4119c796cdad9899ff08f7fd\ta45862ca7e2e885ae4e36afc758c9e9fe39ca86c\t7bd7dfba7e8f85bc0572617c6ef3af4a15e59172\t140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd\tc6225ae5b968345b04a2c7d410875fe7d370b300\t68927e5912ebc218050fcc9daf4b76cf4a9e3c86,e020da4b4edcded41bde9785615262beb1946ad2",
      "path": "src/backend/space/internal/grpcsvc/audit_public_read_test.go",
      "prior": "src/backend/space/internal/grpcsvc/audit_public_read_test.go\tfalse\t2445e81de07681cfd5f1491e4c793aaac747f571\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016\tc14f59c1a43b486d48322d62e7c5baa2816e1253\tc6225ae5b968345b04a2c7d410875fe7d370b300\te3dc9ba836100707a9c4313d83bea1872f1f9016",
      "versions": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "content": {
            "bytes": 3351,
            "sha256": "cc6c1ad667a246720a8fd28e43f8d469820062e1c00d934143091b00c462ed57"
          },
          "ordered_parents": [
            "48068711871938ee3a5b1ba04b9efaaf8dc56a74"
          ],
          "tree_record": "100644 blob 745f4f4e6f4d220bc7a799a784570c898e5190d2\tsrc/backend/space/internal/grpcsvc/audit_public_read_test.go\u0000"
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "content": {
            "bytes": 3351,
            "sha256": "cc6c1ad667a246720a8fd28e43f8d469820062e1c00d934143091b00c462ed57"
          },
          "ordered_parents": [
            "e3dc9ba836100707a9c4313d83bea1872f1f9016"
          ],
          "tree_record": "100644 blob 745f4f4e6f4d220bc7a799a784570c898e5190d2\tsrc/backend/space/internal/grpcsvc/audit_public_read_test.go\u0000"
        },
        "2445e81de07681cfd5f1491e4c793aaac747f571": {
          "content": {
            "bytes": 3351,
            "sha256": "cc6c1ad667a246720a8fd28e43f8d469820062e1c00d934143091b00c462ed57"
          },
          "ordered_parents": [
            "5cc425b35bc71738d86261c1cf4772c7babd4519",
            "c14f59c1a43b486d48322d62e7c5baa2816e1253"
          ],
          "tree_record": "100644 blob 745f4f4e6f4d220bc7a799a784570c898e5190d2\tsrc/backend/space/internal/grpcsvc/audit_public_read_test.go\u0000"
        },
        "3071ddad390443de4119c796cdad9899ff08f7fd": {
          "content": {
            "bytes": 3351,
            "sha256": "cc6c1ad667a246720a8fd28e43f8d469820062e1c00d934143091b00c462ed57"
          },
          "ordered_parents": [
            "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
            "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd"
          ],
          "tree_record": "100644 blob 745f4f4e6f4d220bc7a799a784570c898e5190d2\tsrc/backend/space/internal/grpcsvc/audit_public_read_test.go\u0000"
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "content": {
            "bytes": 3749,
            "sha256": "95b26ac5f759f58a2a6fbaf6262ffaa16bace6dddf985b6cdcb1a7157b624b40"
          },
          "ordered_parents": [
            "2cc9482255ba18cd948aacb567f4c4bdf8976b14"
          ],
          "tree_record": "100644 blob d46a4bcc3eb72fe2c3daf3eaf3294cc57e3b81f0\tsrc/backend/space/internal/grpcsvc/audit_public_read_test.go\u0000"
        },
        "68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "content": {
            "bytes": 3351,
            "sha256": "cc6c1ad667a246720a8fd28e43f8d469820062e1c00d934143091b00c462ed57"
          },
          "ordered_parents": [
            "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
            "f954480f69878a5b3890bf6fcb5af978d68dadf9"
          ],
          "tree_record": "100644 blob 745f4f4e6f4d220bc7a799a784570c898e5190d2\tsrc/backend/space/internal/grpcsvc/audit_public_read_test.go\u0000"
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "content": {
            "bytes": 3749,
            "sha256": "95b26ac5f759f58a2a6fbaf6262ffaa16bace6dddf985b6cdcb1a7157b624b40"
          },
          "ordered_parents": [
            "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe"
          ],
          "tree_record": "100644 blob d46a4bcc3eb72fe2c3daf3eaf3294cc57e3b81f0\tsrc/backend/space/internal/grpcsvc/audit_public_read_test.go\u0000"
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "content": {
            "bytes": 3351,
            "sha256": "cc6c1ad667a246720a8fd28e43f8d469820062e1c00d934143091b00c462ed57"
          },
          "ordered_parents": [
            "dc0acdefec7e5c413988adf71f86d12bff4017bc",
            "6780407299a3beb77e92b32c551ca7cfe3e63e94"
          ],
          "tree_record": "100644 blob 745f4f4e6f4d220bc7a799a784570c898e5190d2\tsrc/backend/space/internal/grpcsvc/audit_public_read_test.go\u0000"
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "content": {
            "bytes": 3351,
            "sha256": "cc6c1ad667a246720a8fd28e43f8d469820062e1c00d934143091b00c462ed57"
          },
          "ordered_parents": [
            "18206e47ff06f1071f69319e9a6eeddf29f079b1"
          ],
          "tree_record": "100644 blob 745f4f4e6f4d220bc7a799a784570c898e5190d2\tsrc/backend/space/internal/grpcsvc/audit_public_read_test.go\u0000"
        },
        "c6225ae5b968345b04a2c7d410875fe7d370b300": {
          "content": null,
          "ordered_parents": [
            "85b036f7a44d199cfc4f5d99e569bfc2140b0494",
            "6b568d3cc5469a8a597e11f8e4acbcc6ad1d9df0"
          ],
          "tree_record": ""
        },
        "e020da4b4edcded41bde9785615262beb1946ad2": {
          "content": {
            "bytes": 3749,
            "sha256": "95b26ac5f759f58a2a6fbaf6262ffaa16bace6dddf985b6cdcb1a7157b624b40"
          },
          "ordered_parents": [
            "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172"
          ],
          "tree_record": "100644 blob d46a4bcc3eb72fe2c3daf3eaf3294cc57e3b81f0\tsrc/backend/space/internal/grpcsvc/audit_public_read_test.go\u0000"
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "content": {
            "bytes": 3749,
            "sha256": "95b26ac5f759f58a2a6fbaf6262ffaa16bace6dddf985b6cdcb1a7157b624b40"
          },
          "ordered_parents": [
            "420c4c7be8c64583ab3e26f0560e5970f705a46e",
            "34640fde433c13073246003b1d009d2c7dba530c"
          ],
          "tree_record": "100644 blob d46a4bcc3eb72fe2c3daf3eaf3294cc57e3b81f0\tsrc/backend/space/internal/grpcsvc/audit_public_read_test.go\u0000"
        }
      }
    },
    {
      "edges": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1..c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "after": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "before": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "2cc9482255ba18cd948aacb567f4c4bdf8976b14..5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "after": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "before": "2cc9482255ba18cd948aacb567f4c4bdf8976b14",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "34640fde433c13073246003b1d009d2c7dba530c..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "34640fde433c13073246003b1d009d2c7dba530c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "420c4c7be8c64583ab3e26f0560e5970f705a46e..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "420c4c7be8c64583ab3e26f0560e5970f705a46e",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "48068711871938ee3a5b1ba04b9efaaf8dc56a74..140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "after": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "before": "48068711871938ee3a5b1ba04b9efaaf8dc56a74",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "patch": {
            "bytes": 10618,
            "sha256": "b16be77664c8daa0a6f98a36fa74ff6a20c2bae20e080235eacfad1340d71466"
          },
          "raw": {
            "bytes": 160,
            "sha256": "7b66c0339f12a911f5305bbc6b808522699450fcf8ad1e1f8b8714f19f540804"
          }
        },
        "6780407299a3beb77e92b32c551ca7cfe3e63e94..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "6780407299a3beb77e92b32c551ca7cfe3e63e94",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172..e020da4b4edcded41bde9785615262beb1946ad2": {
          "after": "e020da4b4edcded41bde9785615262beb1946ad2",
          "before": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe..7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "after": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "before": "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "dc0acdefec7e5c413988adf71f86d12bff4017bc..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "dc0acdefec7e5c413988adf71f86d12bff4017bc",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016..18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "after": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "before": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "patch": {
            "bytes": 10618,
            "sha256": "b16be77664c8daa0a6f98a36fa74ff6a20c2bae20e080235eacfad1340d71466"
          },
          "raw": {
            "bytes": 160,
            "sha256": "7b66c0339f12a911f5305bbc6b808522699450fcf8ad1e1f8b8714f19f540804"
          }
        },
        "f954480f69878a5b3890bf6fcb5af978d68dadf9..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "f954480f69878a5b3890bf6fcb5af978d68dadf9",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        }
      },
      "incoming": "src/backend/space/internal/grpcsvc/be041_audit_route_test.go\ttrue\t3071ddad390443de4119c796cdad9899ff08f7fd\ta45862ca7e2e885ae4e36afc758c9e9fe39ca86c\t7bd7dfba7e8f85bc0572617c6ef3af4a15e59172\t140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd\t5cc425b35bc71738d86261c1cf4772c7babd4519\t68927e5912ebc218050fcc9daf4b76cf4a9e3c86,e020da4b4edcded41bde9785615262beb1946ad2",
      "path": "src/backend/space/internal/grpcsvc/be041_audit_route_test.go",
      "prior": "src/backend/space/internal/grpcsvc/be041_audit_route_test.go\tfalse\t2445e81de07681cfd5f1491e4c793aaac747f571\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016\tc14f59c1a43b486d48322d62e7c5baa2816e1253\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016",
      "versions": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "content": {
            "bytes": 10133,
            "sha256": "f8dba79d0ef8b8d2c86f3fc1239221852f2decd4ddec7cf18905ea300cd9e2b4"
          },
          "ordered_parents": [
            "48068711871938ee3a5b1ba04b9efaaf8dc56a74"
          ],
          "tree_record": "100644 blob ec52e085a08b722f7c8d9c2e2b842e7eea24df46\tsrc/backend/space/internal/grpcsvc/be041_audit_route_test.go\u0000"
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "content": {
            "bytes": 10133,
            "sha256": "f8dba79d0ef8b8d2c86f3fc1239221852f2decd4ddec7cf18905ea300cd9e2b4"
          },
          "ordered_parents": [
            "e3dc9ba836100707a9c4313d83bea1872f1f9016"
          ],
          "tree_record": "100644 blob ec52e085a08b722f7c8d9c2e2b842e7eea24df46\tsrc/backend/space/internal/grpcsvc/be041_audit_route_test.go\u0000"
        },
        "2445e81de07681cfd5f1491e4c793aaac747f571": {
          "content": {
            "bytes": 10133,
            "sha256": "f8dba79d0ef8b8d2c86f3fc1239221852f2decd4ddec7cf18905ea300cd9e2b4"
          },
          "ordered_parents": [
            "5cc425b35bc71738d86261c1cf4772c7babd4519",
            "c14f59c1a43b486d48322d62e7c5baa2816e1253"
          ],
          "tree_record": "100644 blob ec52e085a08b722f7c8d9c2e2b842e7eea24df46\tsrc/backend/space/internal/grpcsvc/be041_audit_route_test.go\u0000"
        },
        "3071ddad390443de4119c796cdad9899ff08f7fd": {
          "content": {
            "bytes": 10133,
            "sha256": "f8dba79d0ef8b8d2c86f3fc1239221852f2decd4ddec7cf18905ea300cd9e2b4"
          },
          "ordered_parents": [
            "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
            "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd"
          ],
          "tree_record": "100644 blob ec52e085a08b722f7c8d9c2e2b842e7eea24df46\tsrc/backend/space/internal/grpcsvc/be041_audit_route_test.go\u0000"
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "content": null,
          "ordered_parents": [
            "2cc9482255ba18cd948aacb567f4c4bdf8976b14"
          ],
          "tree_record": ""
        },
        "68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "content": {
            "bytes": 10133,
            "sha256": "f8dba79d0ef8b8d2c86f3fc1239221852f2decd4ddec7cf18905ea300cd9e2b4"
          },
          "ordered_parents": [
            "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
            "f954480f69878a5b3890bf6fcb5af978d68dadf9"
          ],
          "tree_record": "100644 blob ec52e085a08b722f7c8d9c2e2b842e7eea24df46\tsrc/backend/space/internal/grpcsvc/be041_audit_route_test.go\u0000"
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "content": null,
          "ordered_parents": [
            "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe"
          ],
          "tree_record": ""
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "content": {
            "bytes": 10133,
            "sha256": "f8dba79d0ef8b8d2c86f3fc1239221852f2decd4ddec7cf18905ea300cd9e2b4"
          },
          "ordered_parents": [
            "dc0acdefec7e5c413988adf71f86d12bff4017bc",
            "6780407299a3beb77e92b32c551ca7cfe3e63e94"
          ],
          "tree_record": "100644 blob ec52e085a08b722f7c8d9c2e2b842e7eea24df46\tsrc/backend/space/internal/grpcsvc/be041_audit_route_test.go\u0000"
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "content": {
            "bytes": 10133,
            "sha256": "f8dba79d0ef8b8d2c86f3fc1239221852f2decd4ddec7cf18905ea300cd9e2b4"
          },
          "ordered_parents": [
            "18206e47ff06f1071f69319e9a6eeddf29f079b1"
          ],
          "tree_record": "100644 blob ec52e085a08b722f7c8d9c2e2b842e7eea24df46\tsrc/backend/space/internal/grpcsvc/be041_audit_route_test.go\u0000"
        },
        "e020da4b4edcded41bde9785615262beb1946ad2": {
          "content": null,
          "ordered_parents": [
            "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172"
          ],
          "tree_record": ""
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "content": null,
          "ordered_parents": [
            "420c4c7be8c64583ab3e26f0560e5970f705a46e",
            "34640fde433c13073246003b1d009d2c7dba530c"
          ],
          "tree_record": ""
        }
      }
    },
    {
      "edges": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1..c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "after": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "before": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "2cc9482255ba18cd948aacb567f4c4bdf8976b14..5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "after": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "before": "2cc9482255ba18cd948aacb567f4c4bdf8976b14",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "34640fde433c13073246003b1d009d2c7dba530c..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "34640fde433c13073246003b1d009d2c7dba530c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "420c4c7be8c64583ab3e26f0560e5970f705a46e..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "420c4c7be8c64583ab3e26f0560e5970f705a46e",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "48068711871938ee3a5b1ba04b9efaaf8dc56a74..140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "after": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "before": "48068711871938ee3a5b1ba04b9efaaf8dc56a74",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "patch": {
            "bytes": 1082,
            "sha256": "cb066610b6195b71e200f670cc4ab80d6b6ce8eeb94b2a4381511279272833e8"
          },
          "raw": {
            "bytes": 171,
            "sha256": "9e367f267c6b075013ba4f28777bb90fc06ba3c2ef95353d0a601e2d2d94588f"
          }
        },
        "6780407299a3beb77e92b32c551ca7cfe3e63e94..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "6780407299a3beb77e92b32c551ca7cfe3e63e94",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172..e020da4b4edcded41bde9785615262beb1946ad2": {
          "after": "e020da4b4edcded41bde9785615262beb1946ad2",
          "before": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe..7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "after": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "before": "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "dc0acdefec7e5c413988adf71f86d12bff4017bc..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "dc0acdefec7e5c413988adf71f86d12bff4017bc",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016..18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "after": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "before": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "patch": {
            "bytes": 1082,
            "sha256": "cb066610b6195b71e200f670cc4ab80d6b6ce8eeb94b2a4381511279272833e8"
          },
          "raw": {
            "bytes": 171,
            "sha256": "9e367f267c6b075013ba4f28777bb90fc06ba3c2ef95353d0a601e2d2d94588f"
          }
        },
        "f954480f69878a5b3890bf6fcb5af978d68dadf9..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "f954480f69878a5b3890bf6fcb5af978d68dadf9",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        }
      },
      "incoming": "src/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go\ttrue\t3071ddad390443de4119c796cdad9899ff08f7fd\ta45862ca7e2e885ae4e36afc758c9e9fe39ca86c\t7bd7dfba7e8f85bc0572617c6ef3af4a15e59172\t140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd\t5cc425b35bc71738d86261c1cf4772c7babd4519\t68927e5912ebc218050fcc9daf4b76cf4a9e3c86,e020da4b4edcded41bde9785615262beb1946ad2",
      "path": "src/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go",
      "prior": "src/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go\tfalse\t2445e81de07681cfd5f1491e4c793aaac747f571\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016\tc14f59c1a43b486d48322d62e7c5baa2816e1253\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016",
      "versions": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "content": {
            "bytes": 13690,
            "sha256": "5e1cbbcb7f65605fff266991b56843db3988e28da98a0de3f6044add9b5d0e98"
          },
          "ordered_parents": [
            "48068711871938ee3a5b1ba04b9efaaf8dc56a74"
          ],
          "tree_record": "100644 blob 49a40875541c8061170b4a0169cb06eb55a89505\tsrc/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go\u0000"
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "content": {
            "bytes": 13690,
            "sha256": "5e1cbbcb7f65605fff266991b56843db3988e28da98a0de3f6044add9b5d0e98"
          },
          "ordered_parents": [
            "e3dc9ba836100707a9c4313d83bea1872f1f9016"
          ],
          "tree_record": "100644 blob 49a40875541c8061170b4a0169cb06eb55a89505\tsrc/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go\u0000"
        },
        "2445e81de07681cfd5f1491e4c793aaac747f571": {
          "content": {
            "bytes": 13690,
            "sha256": "5e1cbbcb7f65605fff266991b56843db3988e28da98a0de3f6044add9b5d0e98"
          },
          "ordered_parents": [
            "5cc425b35bc71738d86261c1cf4772c7babd4519",
            "c14f59c1a43b486d48322d62e7c5baa2816e1253"
          ],
          "tree_record": "100644 blob 49a40875541c8061170b4a0169cb06eb55a89505\tsrc/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go\u0000"
        },
        "3071ddad390443de4119c796cdad9899ff08f7fd": {
          "content": {
            "bytes": 13690,
            "sha256": "5e1cbbcb7f65605fff266991b56843db3988e28da98a0de3f6044add9b5d0e98"
          },
          "ordered_parents": [
            "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
            "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd"
          ],
          "tree_record": "100644 blob 49a40875541c8061170b4a0169cb06eb55a89505\tsrc/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go\u0000"
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "content": {
            "bytes": 13620,
            "sha256": "67521c5bdf62e92a9a098c3cb6cbdb413e71c3da80a2d61abcdf42eaed7cfd54"
          },
          "ordered_parents": [
            "2cc9482255ba18cd948aacb567f4c4bdf8976b14"
          ],
          "tree_record": "100644 blob 8e99f538d9ca23b342a6f9d0bd0053d59674ea60\tsrc/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go\u0000"
        },
        "68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "content": {
            "bytes": 13690,
            "sha256": "5e1cbbcb7f65605fff266991b56843db3988e28da98a0de3f6044add9b5d0e98"
          },
          "ordered_parents": [
            "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
            "f954480f69878a5b3890bf6fcb5af978d68dadf9"
          ],
          "tree_record": "100644 blob 49a40875541c8061170b4a0169cb06eb55a89505\tsrc/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go\u0000"
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "content": {
            "bytes": 13620,
            "sha256": "67521c5bdf62e92a9a098c3cb6cbdb413e71c3da80a2d61abcdf42eaed7cfd54"
          },
          "ordered_parents": [
            "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe"
          ],
          "tree_record": "100644 blob 8e99f538d9ca23b342a6f9d0bd0053d59674ea60\tsrc/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go\u0000"
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "content": {
            "bytes": 13690,
            "sha256": "5e1cbbcb7f65605fff266991b56843db3988e28da98a0de3f6044add9b5d0e98"
          },
          "ordered_parents": [
            "dc0acdefec7e5c413988adf71f86d12bff4017bc",
            "6780407299a3beb77e92b32c551ca7cfe3e63e94"
          ],
          "tree_record": "100644 blob 49a40875541c8061170b4a0169cb06eb55a89505\tsrc/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go\u0000"
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "content": {
            "bytes": 13690,
            "sha256": "5e1cbbcb7f65605fff266991b56843db3988e28da98a0de3f6044add9b5d0e98"
          },
          "ordered_parents": [
            "18206e47ff06f1071f69319e9a6eeddf29f079b1"
          ],
          "tree_record": "100644 blob 49a40875541c8061170b4a0169cb06eb55a89505\tsrc/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go\u0000"
        },
        "e020da4b4edcded41bde9785615262beb1946ad2": {
          "content": {
            "bytes": 13620,
            "sha256": "67521c5bdf62e92a9a098c3cb6cbdb413e71c3da80a2d61abcdf42eaed7cfd54"
          },
          "ordered_parents": [
            "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172"
          ],
          "tree_record": "100644 blob 8e99f538d9ca23b342a6f9d0bd0053d59674ea60\tsrc/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go\u0000"
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "content": {
            "bytes": 13620,
            "sha256": "67521c5bdf62e92a9a098c3cb6cbdb413e71c3da80a2d61abcdf42eaed7cfd54"
          },
          "ordered_parents": [
            "420c4c7be8c64583ab3e26f0560e5970f705a46e",
            "34640fde433c13073246003b1d009d2c7dba530c"
          ],
          "tree_record": "100644 blob 8e99f538d9ca23b342a6f9d0bd0053d59674ea60\tsrc/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go\u0000"
        }
      }
    },
    {
      "edges": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1..c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "after": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "before": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "2cc9482255ba18cd948aacb567f4c4bdf8976b14..5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "after": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "before": "2cc9482255ba18cd948aacb567f4c4bdf8976b14",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "34640fde433c13073246003b1d009d2c7dba530c..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "34640fde433c13073246003b1d009d2c7dba530c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "420c4c7be8c64583ab3e26f0560e5970f705a46e..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "420c4c7be8c64583ab3e26f0560e5970f705a46e",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "48068711871938ee3a5b1ba04b9efaaf8dc56a74..140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "after": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "before": "48068711871938ee3a5b1ba04b9efaaf8dc56a74",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "patch": {
            "bytes": 1925,
            "sha256": "9d0aa625e06d6cc0bd3fd21a90bb282f5a2330214eb8f14a9d1dba02db3a57ec"
          },
          "raw": {
            "bytes": 143,
            "sha256": "4e06fa93f5e63dbb4f6cd4db9e57f88eb7f3db033dae60e08ebcb8216e52f067"
          }
        },
        "6780407299a3beb77e92b32c551ca7cfe3e63e94..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "6780407299a3beb77e92b32c551ca7cfe3e63e94",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172..e020da4b4edcded41bde9785615262beb1946ad2": {
          "after": "e020da4b4edcded41bde9785615262beb1946ad2",
          "before": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe..7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "after": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "before": "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "dc0acdefec7e5c413988adf71f86d12bff4017bc..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "dc0acdefec7e5c413988adf71f86d12bff4017bc",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016..18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "after": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "before": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "patch": {
            "bytes": 1925,
            "sha256": "9d0aa625e06d6cc0bd3fd21a90bb282f5a2330214eb8f14a9d1dba02db3a57ec"
          },
          "raw": {
            "bytes": 143,
            "sha256": "4e06fa93f5e63dbb4f6cd4db9e57f88eb7f3db033dae60e08ebcb8216e52f067"
          }
        },
        "f954480f69878a5b3890bf6fcb5af978d68dadf9..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "f954480f69878a5b3890bf6fcb5af978d68dadf9",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        }
      },
      "incoming": "src/backend/space/internal/grpcsvc/roles.go\ttrue\t3071ddad390443de4119c796cdad9899ff08f7fd\ta45862ca7e2e885ae4e36afc758c9e9fe39ca86c\t7bd7dfba7e8f85bc0572617c6ef3af4a15e59172\t140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd\t5cc425b35bc71738d86261c1cf4772c7babd4519\t68927e5912ebc218050fcc9daf4b76cf4a9e3c86,e020da4b4edcded41bde9785615262beb1946ad2",
      "path": "src/backend/space/internal/grpcsvc/roles.go",
      "prior": "src/backend/space/internal/grpcsvc/roles.go\tfalse\t2445e81de07681cfd5f1491e4c793aaac747f571\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016\tc14f59c1a43b486d48322d62e7c5baa2816e1253\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016",
      "versions": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "content": {
            "bytes": 7403,
            "sha256": "bdbc877e13825ac2845e3531c7a11cfb67cc3f4873ebb2bd360b410d66fc9a93"
          },
          "ordered_parents": [
            "48068711871938ee3a5b1ba04b9efaaf8dc56a74"
          ],
          "tree_record": "100644 blob 7cd791cd516e7ebb8c9f909ee8f241a70533e339\tsrc/backend/space/internal/grpcsvc/roles.go\u0000"
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "content": {
            "bytes": 7403,
            "sha256": "bdbc877e13825ac2845e3531c7a11cfb67cc3f4873ebb2bd360b410d66fc9a93"
          },
          "ordered_parents": [
            "e3dc9ba836100707a9c4313d83bea1872f1f9016"
          ],
          "tree_record": "100644 blob 7cd791cd516e7ebb8c9f909ee8f241a70533e339\tsrc/backend/space/internal/grpcsvc/roles.go\u0000"
        },
        "2445e81de07681cfd5f1491e4c793aaac747f571": {
          "content": {
            "bytes": 7403,
            "sha256": "bdbc877e13825ac2845e3531c7a11cfb67cc3f4873ebb2bd360b410d66fc9a93"
          },
          "ordered_parents": [
            "5cc425b35bc71738d86261c1cf4772c7babd4519",
            "c14f59c1a43b486d48322d62e7c5baa2816e1253"
          ],
          "tree_record": "100644 blob 7cd791cd516e7ebb8c9f909ee8f241a70533e339\tsrc/backend/space/internal/grpcsvc/roles.go\u0000"
        },
        "3071ddad390443de4119c796cdad9899ff08f7fd": {
          "content": {
            "bytes": 7403,
            "sha256": "bdbc877e13825ac2845e3531c7a11cfb67cc3f4873ebb2bd360b410d66fc9a93"
          },
          "ordered_parents": [
            "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
            "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd"
          ],
          "tree_record": "100644 blob 7cd791cd516e7ebb8c9f909ee8f241a70533e339\tsrc/backend/space/internal/grpcsvc/roles.go\u0000"
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "content": {
            "bytes": 6445,
            "sha256": "e6916640dc22474f50ec6067eb56f756e2cab06075e47d2c01979fc5beba76c4"
          },
          "ordered_parents": [
            "2cc9482255ba18cd948aacb567f4c4bdf8976b14"
          ],
          "tree_record": "100644 blob 5c158efc41c2a2de91cb16622d96ffe0754f8af9\tsrc/backend/space/internal/grpcsvc/roles.go\u0000"
        },
        "68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "content": {
            "bytes": 7403,
            "sha256": "bdbc877e13825ac2845e3531c7a11cfb67cc3f4873ebb2bd360b410d66fc9a93"
          },
          "ordered_parents": [
            "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
            "f954480f69878a5b3890bf6fcb5af978d68dadf9"
          ],
          "tree_record": "100644 blob 7cd791cd516e7ebb8c9f909ee8f241a70533e339\tsrc/backend/space/internal/grpcsvc/roles.go\u0000"
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "content": {
            "bytes": 6445,
            "sha256": "e6916640dc22474f50ec6067eb56f756e2cab06075e47d2c01979fc5beba76c4"
          },
          "ordered_parents": [
            "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe"
          ],
          "tree_record": "100644 blob 5c158efc41c2a2de91cb16622d96ffe0754f8af9\tsrc/backend/space/internal/grpcsvc/roles.go\u0000"
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "content": {
            "bytes": 7403,
            "sha256": "bdbc877e13825ac2845e3531c7a11cfb67cc3f4873ebb2bd360b410d66fc9a93"
          },
          "ordered_parents": [
            "dc0acdefec7e5c413988adf71f86d12bff4017bc",
            "6780407299a3beb77e92b32c551ca7cfe3e63e94"
          ],
          "tree_record": "100644 blob 7cd791cd516e7ebb8c9f909ee8f241a70533e339\tsrc/backend/space/internal/grpcsvc/roles.go\u0000"
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "content": {
            "bytes": 7403,
            "sha256": "bdbc877e13825ac2845e3531c7a11cfb67cc3f4873ebb2bd360b410d66fc9a93"
          },
          "ordered_parents": [
            "18206e47ff06f1071f69319e9a6eeddf29f079b1"
          ],
          "tree_record": "100644 blob 7cd791cd516e7ebb8c9f909ee8f241a70533e339\tsrc/backend/space/internal/grpcsvc/roles.go\u0000"
        },
        "e020da4b4edcded41bde9785615262beb1946ad2": {
          "content": {
            "bytes": 6445,
            "sha256": "e6916640dc22474f50ec6067eb56f756e2cab06075e47d2c01979fc5beba76c4"
          },
          "ordered_parents": [
            "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172"
          ],
          "tree_record": "100644 blob 5c158efc41c2a2de91cb16622d96ffe0754f8af9\tsrc/backend/space/internal/grpcsvc/roles.go\u0000"
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "content": {
            "bytes": 6445,
            "sha256": "e6916640dc22474f50ec6067eb56f756e2cab06075e47d2c01979fc5beba76c4"
          },
          "ordered_parents": [
            "420c4c7be8c64583ab3e26f0560e5970f705a46e",
            "34640fde433c13073246003b1d009d2c7dba530c"
          ],
          "tree_record": "100644 blob 5c158efc41c2a2de91cb16622d96ffe0754f8af9\tsrc/backend/space/internal/grpcsvc/roles.go\u0000"
        }
      }
    },
    {
      "edges": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1..c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "after": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "before": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "2cc9482255ba18cd948aacb567f4c4bdf8976b14..5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "after": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "before": "2cc9482255ba18cd948aacb567f4c4bdf8976b14",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "34640fde433c13073246003b1d009d2c7dba530c..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "34640fde433c13073246003b1d009d2c7dba530c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "420c4c7be8c64583ab3e26f0560e5970f705a46e..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "420c4c7be8c64583ab3e26f0560e5970f705a46e",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "48068711871938ee3a5b1ba04b9efaaf8dc56a74..140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "after": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "before": "48068711871938ee3a5b1ba04b9efaaf8dc56a74",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "patch": {
            "bytes": 1322,
            "sha256": "f4497c28e58da3b46436c7347d94c51a71fb6e5b6910e22e20e3a489240f270d"
          },
          "raw": {
            "bytes": 160,
            "sha256": "e3205b3d45841018fc58556ddc1518f939223b3ac73dae1086e5bcc9f97ebcff"
          }
        },
        "6780407299a3beb77e92b32c551ca7cfe3e63e94..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "6780407299a3beb77e92b32c551ca7cfe3e63e94",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172..e020da4b4edcded41bde9785615262beb1946ad2": {
          "after": "e020da4b4edcded41bde9785615262beb1946ad2",
          "before": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe..7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "after": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "before": "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "dc0acdefec7e5c413988adf71f86d12bff4017bc..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "dc0acdefec7e5c413988adf71f86d12bff4017bc",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016..18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "after": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "before": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "patch": {
            "bytes": 1322,
            "sha256": "f4497c28e58da3b46436c7347d94c51a71fb6e5b6910e22e20e3a489240f270d"
          },
          "raw": {
            "bytes": 160,
            "sha256": "e3205b3d45841018fc58556ddc1518f939223b3ac73dae1086e5bcc9f97ebcff"
          }
        },
        "f954480f69878a5b3890bf6fcb5af978d68dadf9..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "f954480f69878a5b3890bf6fcb5af978d68dadf9",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        }
      },
      "incoming": "src/backend/space/internal/grpcsvc/space_integration_test.go\ttrue\t3071ddad390443de4119c796cdad9899ff08f7fd\ta45862ca7e2e885ae4e36afc758c9e9fe39ca86c\t7bd7dfba7e8f85bc0572617c6ef3af4a15e59172\t140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd\t5cc425b35bc71738d86261c1cf4772c7babd4519\t68927e5912ebc218050fcc9daf4b76cf4a9e3c86,e020da4b4edcded41bde9785615262beb1946ad2",
      "path": "src/backend/space/internal/grpcsvc/space_integration_test.go",
      "prior": "src/backend/space/internal/grpcsvc/space_integration_test.go\tfalse\t2445e81de07681cfd5f1491e4c793aaac747f571\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016\tc14f59c1a43b486d48322d62e7c5baa2816e1253\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016",
      "versions": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "content": {
            "bytes": 41383,
            "sha256": "ddbce3afa3f6a5955b0e946f66093272c7afaa8993f93fa3db989e7e2ec1ee5f"
          },
          "ordered_parents": [
            "48068711871938ee3a5b1ba04b9efaaf8dc56a74"
          ],
          "tree_record": "100644 blob da13f11ef9f4ffd85c707aed6af634121c63262a\tsrc/backend/space/internal/grpcsvc/space_integration_test.go\u0000"
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "content": {
            "bytes": 41383,
            "sha256": "ddbce3afa3f6a5955b0e946f66093272c7afaa8993f93fa3db989e7e2ec1ee5f"
          },
          "ordered_parents": [
            "e3dc9ba836100707a9c4313d83bea1872f1f9016"
          ],
          "tree_record": "100644 blob da13f11ef9f4ffd85c707aed6af634121c63262a\tsrc/backend/space/internal/grpcsvc/space_integration_test.go\u0000"
        },
        "2445e81de07681cfd5f1491e4c793aaac747f571": {
          "content": {
            "bytes": 41383,
            "sha256": "ddbce3afa3f6a5955b0e946f66093272c7afaa8993f93fa3db989e7e2ec1ee5f"
          },
          "ordered_parents": [
            "5cc425b35bc71738d86261c1cf4772c7babd4519",
            "c14f59c1a43b486d48322d62e7c5baa2816e1253"
          ],
          "tree_record": "100644 blob da13f11ef9f4ffd85c707aed6af634121c63262a\tsrc/backend/space/internal/grpcsvc/space_integration_test.go\u0000"
        },
        "3071ddad390443de4119c796cdad9899ff08f7fd": {
          "content": {
            "bytes": 41383,
            "sha256": "ddbce3afa3f6a5955b0e946f66093272c7afaa8993f93fa3db989e7e2ec1ee5f"
          },
          "ordered_parents": [
            "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
            "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd"
          ],
          "tree_record": "100644 blob da13f11ef9f4ffd85c707aed6af634121c63262a\tsrc/backend/space/internal/grpcsvc/space_integration_test.go\u0000"
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "content": {
            "bytes": 40759,
            "sha256": "5a42315fa495baada715e3013e2364d5787792d210e77db0f30014b7fb07430f"
          },
          "ordered_parents": [
            "2cc9482255ba18cd948aacb567f4c4bdf8976b14"
          ],
          "tree_record": "100644 blob 5e853f45feffd87a828784e78d3e36af1d7d8bb2\tsrc/backend/space/internal/grpcsvc/space_integration_test.go\u0000"
        },
        "68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "content": {
            "bytes": 41383,
            "sha256": "ddbce3afa3f6a5955b0e946f66093272c7afaa8993f93fa3db989e7e2ec1ee5f"
          },
          "ordered_parents": [
            "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
            "f954480f69878a5b3890bf6fcb5af978d68dadf9"
          ],
          "tree_record": "100644 blob da13f11ef9f4ffd85c707aed6af634121c63262a\tsrc/backend/space/internal/grpcsvc/space_integration_test.go\u0000"
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "content": {
            "bytes": 40759,
            "sha256": "5a42315fa495baada715e3013e2364d5787792d210e77db0f30014b7fb07430f"
          },
          "ordered_parents": [
            "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe"
          ],
          "tree_record": "100644 blob 5e853f45feffd87a828784e78d3e36af1d7d8bb2\tsrc/backend/space/internal/grpcsvc/space_integration_test.go\u0000"
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "content": {
            "bytes": 41383,
            "sha256": "ddbce3afa3f6a5955b0e946f66093272c7afaa8993f93fa3db989e7e2ec1ee5f"
          },
          "ordered_parents": [
            "dc0acdefec7e5c413988adf71f86d12bff4017bc",
            "6780407299a3beb77e92b32c551ca7cfe3e63e94"
          ],
          "tree_record": "100644 blob da13f11ef9f4ffd85c707aed6af634121c63262a\tsrc/backend/space/internal/grpcsvc/space_integration_test.go\u0000"
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "content": {
            "bytes": 41383,
            "sha256": "ddbce3afa3f6a5955b0e946f66093272c7afaa8993f93fa3db989e7e2ec1ee5f"
          },
          "ordered_parents": [
            "18206e47ff06f1071f69319e9a6eeddf29f079b1"
          ],
          "tree_record": "100644 blob da13f11ef9f4ffd85c707aed6af634121c63262a\tsrc/backend/space/internal/grpcsvc/space_integration_test.go\u0000"
        },
        "e020da4b4edcded41bde9785615262beb1946ad2": {
          "content": {
            "bytes": 40759,
            "sha256": "5a42315fa495baada715e3013e2364d5787792d210e77db0f30014b7fb07430f"
          },
          "ordered_parents": [
            "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172"
          ],
          "tree_record": "100644 blob 5e853f45feffd87a828784e78d3e36af1d7d8bb2\tsrc/backend/space/internal/grpcsvc/space_integration_test.go\u0000"
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "content": {
            "bytes": 40759,
            "sha256": "5a42315fa495baada715e3013e2364d5787792d210e77db0f30014b7fb07430f"
          },
          "ordered_parents": [
            "420c4c7be8c64583ab3e26f0560e5970f705a46e",
            "34640fde433c13073246003b1d009d2c7dba530c"
          ],
          "tree_record": "100644 blob 5e853f45feffd87a828784e78d3e36af1d7d8bb2\tsrc/backend/space/internal/grpcsvc/space_integration_test.go\u0000"
        }
      }
    },
    {
      "edges": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1..c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "after": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "before": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "2cc9482255ba18cd948aacb567f4c4bdf8976b14..5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "after": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "before": "2cc9482255ba18cd948aacb567f4c4bdf8976b14",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "34640fde433c13073246003b1d009d2c7dba530c..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "34640fde433c13073246003b1d009d2c7dba530c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "420c4c7be8c64583ab3e26f0560e5970f705a46e..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "420c4c7be8c64583ab3e26f0560e5970f705a46e",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "48068711871938ee3a5b1ba04b9efaaf8dc56a74..140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "after": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "before": "48068711871938ee3a5b1ba04b9efaaf8dc56a74",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "patch": {
            "bytes": 1094,
            "sha256": "9c3689aed5991a5dd15d671331861c03da24d7e72962a8c8d05abf73f34b990a"
          },
          "raw": {
            "bytes": 159,
            "sha256": "76b23a31bd48be13092a5e21c18b7181ef3208fe1dfce3b0a4b225e0f5e1e106"
          }
        },
        "6780407299a3beb77e92b32c551ca7cfe3e63e94..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "6780407299a3beb77e92b32c551ca7cfe3e63e94",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "6b568d3cc5469a8a597e11f8e4acbcc6ad1d9df0..c6225ae5b968345b04a2c7d410875fe7d370b300": {
          "after": "c6225ae5b968345b04a2c7d410875fe7d370b300",
          "before": "6b568d3cc5469a8a597e11f8e4acbcc6ad1d9df0",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172..e020da4b4edcded41bde9785615262beb1946ad2": {
          "after": "e020da4b4edcded41bde9785615262beb1946ad2",
          "before": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "85b036f7a44d199cfc4f5d99e569bfc2140b0494..c6225ae5b968345b04a2c7d410875fe7d370b300": {
          "after": "c6225ae5b968345b04a2c7d410875fe7d370b300",
          "before": "85b036f7a44d199cfc4f5d99e569bfc2140b0494",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe..7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "after": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "before": "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "dc0acdefec7e5c413988adf71f86d12bff4017bc..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "dc0acdefec7e5c413988adf71f86d12bff4017bc",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016..18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "after": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "before": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "patch": {
            "bytes": 1094,
            "sha256": "9c3689aed5991a5dd15d671331861c03da24d7e72962a8c8d05abf73f34b990a"
          },
          "raw": {
            "bytes": 159,
            "sha256": "76b23a31bd48be13092a5e21c18b7181ef3208fe1dfce3b0a4b225e0f5e1e106"
          }
        },
        "f954480f69878a5b3890bf6fcb5af978d68dadf9..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "f954480f69878a5b3890bf6fcb5af978d68dadf9",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        }
      },
      "incoming": "src/backend/space/internal/lifecycleprincipal/audit_test.go\ttrue\t3071ddad390443de4119c796cdad9899ff08f7fd\ta45862ca7e2e885ae4e36afc758c9e9fe39ca86c\t7bd7dfba7e8f85bc0572617c6ef3af4a15e59172\t140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd\tc6225ae5b968345b04a2c7d410875fe7d370b300\t68927e5912ebc218050fcc9daf4b76cf4a9e3c86,e020da4b4edcded41bde9785615262beb1946ad2",
      "path": "src/backend/space/internal/lifecycleprincipal/audit_test.go",
      "prior": "src/backend/space/internal/lifecycleprincipal/audit_test.go\tfalse\t2445e81de07681cfd5f1491e4c793aaac747f571\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016\tc14f59c1a43b486d48322d62e7c5baa2816e1253\tc6225ae5b968345b04a2c7d410875fe7d370b300\te3dc9ba836100707a9c4313d83bea1872f1f9016",
      "versions": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "content": {
            "bytes": 3703,
            "sha256": "e7c23d419da7d83e908452c03682de3afddce3a0c8bff8b58fbbbe1e80cc5692"
          },
          "ordered_parents": [
            "48068711871938ee3a5b1ba04b9efaaf8dc56a74"
          ],
          "tree_record": "100644 blob 7d308d251bef0ddc0e6954b543dcf2a0b1be5c45\tsrc/backend/space/internal/lifecycleprincipal/audit_test.go\u0000"
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "content": {
            "bytes": 3703,
            "sha256": "e7c23d419da7d83e908452c03682de3afddce3a0c8bff8b58fbbbe1e80cc5692"
          },
          "ordered_parents": [
            "e3dc9ba836100707a9c4313d83bea1872f1f9016"
          ],
          "tree_record": "100644 blob 7d308d251bef0ddc0e6954b543dcf2a0b1be5c45\tsrc/backend/space/internal/lifecycleprincipal/audit_test.go\u0000"
        },
        "2445e81de07681cfd5f1491e4c793aaac747f571": {
          "content": {
            "bytes": 3703,
            "sha256": "e7c23d419da7d83e908452c03682de3afddce3a0c8bff8b58fbbbe1e80cc5692"
          },
          "ordered_parents": [
            "5cc425b35bc71738d86261c1cf4772c7babd4519",
            "c14f59c1a43b486d48322d62e7c5baa2816e1253"
          ],
          "tree_record": "100644 blob 7d308d251bef0ddc0e6954b543dcf2a0b1be5c45\tsrc/backend/space/internal/lifecycleprincipal/audit_test.go\u0000"
        },
        "3071ddad390443de4119c796cdad9899ff08f7fd": {
          "content": {
            "bytes": 3703,
            "sha256": "e7c23d419da7d83e908452c03682de3afddce3a0c8bff8b58fbbbe1e80cc5692"
          },
          "ordered_parents": [
            "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
            "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd"
          ],
          "tree_record": "100644 blob 7d308d251bef0ddc0e6954b543dcf2a0b1be5c45\tsrc/backend/space/internal/lifecycleprincipal/audit_test.go\u0000"
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "content": {
            "bytes": 3516,
            "sha256": "39647ecf36614e2b511b32090c612f03de966e0b697dad9056b938c9ba853f41"
          },
          "ordered_parents": [
            "2cc9482255ba18cd948aacb567f4c4bdf8976b14"
          ],
          "tree_record": "100644 blob 16ca486d1d7aa6a921092436ffeecb01577e031e\tsrc/backend/space/internal/lifecycleprincipal/audit_test.go\u0000"
        },
        "68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "content": {
            "bytes": 3703,
            "sha256": "e7c23d419da7d83e908452c03682de3afddce3a0c8bff8b58fbbbe1e80cc5692"
          },
          "ordered_parents": [
            "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
            "f954480f69878a5b3890bf6fcb5af978d68dadf9"
          ],
          "tree_record": "100644 blob 7d308d251bef0ddc0e6954b543dcf2a0b1be5c45\tsrc/backend/space/internal/lifecycleprincipal/audit_test.go\u0000"
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "content": {
            "bytes": 3516,
            "sha256": "39647ecf36614e2b511b32090c612f03de966e0b697dad9056b938c9ba853f41"
          },
          "ordered_parents": [
            "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe"
          ],
          "tree_record": "100644 blob 16ca486d1d7aa6a921092436ffeecb01577e031e\tsrc/backend/space/internal/lifecycleprincipal/audit_test.go\u0000"
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "content": {
            "bytes": 3703,
            "sha256": "e7c23d419da7d83e908452c03682de3afddce3a0c8bff8b58fbbbe1e80cc5692"
          },
          "ordered_parents": [
            "dc0acdefec7e5c413988adf71f86d12bff4017bc",
            "6780407299a3beb77e92b32c551ca7cfe3e63e94"
          ],
          "tree_record": "100644 blob 7d308d251bef0ddc0e6954b543dcf2a0b1be5c45\tsrc/backend/space/internal/lifecycleprincipal/audit_test.go\u0000"
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "content": {
            "bytes": 3703,
            "sha256": "e7c23d419da7d83e908452c03682de3afddce3a0c8bff8b58fbbbe1e80cc5692"
          },
          "ordered_parents": [
            "18206e47ff06f1071f69319e9a6eeddf29f079b1"
          ],
          "tree_record": "100644 blob 7d308d251bef0ddc0e6954b543dcf2a0b1be5c45\tsrc/backend/space/internal/lifecycleprincipal/audit_test.go\u0000"
        },
        "c6225ae5b968345b04a2c7d410875fe7d370b300": {
          "content": null,
          "ordered_parents": [
            "85b036f7a44d199cfc4f5d99e569bfc2140b0494",
            "6b568d3cc5469a8a597e11f8e4acbcc6ad1d9df0"
          ],
          "tree_record": ""
        },
        "e020da4b4edcded41bde9785615262beb1946ad2": {
          "content": {
            "bytes": 3516,
            "sha256": "39647ecf36614e2b511b32090c612f03de966e0b697dad9056b938c9ba853f41"
          },
          "ordered_parents": [
            "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172"
          ],
          "tree_record": "100644 blob 16ca486d1d7aa6a921092436ffeecb01577e031e\tsrc/backend/space/internal/lifecycleprincipal/audit_test.go\u0000"
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "content": {
            "bytes": 3516,
            "sha256": "39647ecf36614e2b511b32090c612f03de966e0b697dad9056b938c9ba853f41"
          },
          "ordered_parents": [
            "420c4c7be8c64583ab3e26f0560e5970f705a46e",
            "34640fde433c13073246003b1d009d2c7dba530c"
          ],
          "tree_record": "100644 blob 16ca486d1d7aa6a921092436ffeecb01577e031e\tsrc/backend/space/internal/lifecycleprincipal/audit_test.go\u0000"
        }
      }
    },
    {
      "edges": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1..c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "after": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "before": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "2cc9482255ba18cd948aacb567f4c4bdf8976b14..5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "after": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "before": "2cc9482255ba18cd948aacb567f4c4bdf8976b14",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "31276e7cdd661ec93cb9c2e0369a0f102adf7efd..a635c0ad19a233f9bd934dca57943bc6363a6da3": {
          "after": "a635c0ad19a233f9bd934dca57943bc6363a6da3",
          "before": "31276e7cdd661ec93cb9c2e0369a0f102adf7efd",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "34640fde433c13073246003b1d009d2c7dba530c..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "34640fde433c13073246003b1d009d2c7dba530c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "420c4c7be8c64583ab3e26f0560e5970f705a46e..e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "after": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "before": "420c4c7be8c64583ab3e26f0560e5970f705a46e",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "48068711871938ee3a5b1ba04b9efaaf8dc56a74..140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "after": "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd",
          "before": "48068711871938ee3a5b1ba04b9efaaf8dc56a74",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "5cc425b35bc71738d86261c1cf4772c7babd4519",
          "patch": {
            "bytes": 1699,
            "sha256": "11e35a9a60f6557aaacf178d96cdd2ee88e985a10b7c8bd1bdc000cbb735ad05"
          },
          "raw": {
            "bytes": 160,
            "sha256": "600a88e5347dc6202c539efb849a06eb41239fde5f7a2641dceed4fb9843b44e"
          }
        },
        "6780407299a3beb77e92b32c551ca7cfe3e63e94..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "6780407299a3beb77e92b32c551ca7cfe3e63e94",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172..e020da4b4edcded41bde9785615262beb1946ad2": {
          "after": "e020da4b4edcded41bde9785615262beb1946ad2",
          "before": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c..3071ddad390443de4119c796cdad9899ff08f7fd": {
          "after": "3071ddad390443de4119c796cdad9899ff08f7fd",
          "before": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe..7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "after": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
          "before": "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253..2445e81de07681cfd5f1491e4c793aaac747f571": {
          "after": "2445e81de07681cfd5f1491e4c793aaac747f571",
          "before": "c14f59c1a43b486d48322d62e7c5baa2816e1253",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "dc0acdefec7e5c413988adf71f86d12bff4017bc..a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "after": "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
          "before": "dc0acdefec7e5c413988adf71f86d12bff4017bc",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016..18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "after": "18206e47ff06f1071f69319e9a6eeddf29f079b1",
          "before": "e3dc9ba836100707a9c4313d83bea1872f1f9016",
          "patch": {
            "bytes": 1699,
            "sha256": "11e35a9a60f6557aaacf178d96cdd2ee88e985a10b7c8bd1bdc000cbb735ad05"
          },
          "raw": {
            "bytes": 160,
            "sha256": "600a88e5347dc6202c539efb849a06eb41239fde5f7a2641dceed4fb9843b44e"
          }
        },
        "f954480f69878a5b3890bf6fcb5af978d68dadf9..68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "after": "68927e5912ebc218050fcc9daf4b76cf4a9e3c86",
          "before": "f954480f69878a5b3890bf6fcb5af978d68dadf9",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        },
        "fdf42784969cbf76bbec7eb1cc6ac878c31acd6e..a635c0ad19a233f9bd934dca57943bc6363a6da3": {
          "after": "a635c0ad19a233f9bd934dca57943bc6363a6da3",
          "before": "fdf42784969cbf76bbec7eb1cc6ac878c31acd6e",
          "patch": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          },
          "raw": {
            "bytes": 0,
            "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
          }
        }
      },
      "incoming": "src/backend/space/internal/lifecycleprincipal/interceptor.go\ttrue\t3071ddad390443de4119c796cdad9899ff08f7fd\ta45862ca7e2e885ae4e36afc758c9e9fe39ca86c\t7bd7dfba7e8f85bc0572617c6ef3af4a15e59172\t140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd\ta635c0ad19a233f9bd934dca57943bc6363a6da3\t68927e5912ebc218050fcc9daf4b76cf4a9e3c86,e020da4b4edcded41bde9785615262beb1946ad2",
      "path": "src/backend/space/internal/lifecycleprincipal/interceptor.go",
      "prior": "src/backend/space/internal/lifecycleprincipal/interceptor.go\tfalse\t2445e81de07681cfd5f1491e4c793aaac747f571\t5cc425b35bc71738d86261c1cf4772c7babd4519\te3dc9ba836100707a9c4313d83bea1872f1f9016\tc14f59c1a43b486d48322d62e7c5baa2816e1253\ta635c0ad19a233f9bd934dca57943bc6363a6da3\te3dc9ba836100707a9c4313d83bea1872f1f9016",
      "versions": {
        "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd": {
          "content": {
            "bytes": 4456,
            "sha256": "8d7288187e687f3144f39f7204febdf5b0b2a9084e3ff7ef5b0e754f383dfb19"
          },
          "ordered_parents": [
            "48068711871938ee3a5b1ba04b9efaaf8dc56a74"
          ],
          "tree_record": "100644 blob b1c7198a1e68addcfca97dbfa712d4e779d51dc1\tsrc/backend/space/internal/lifecycleprincipal/interceptor.go\u0000"
        },
        "18206e47ff06f1071f69319e9a6eeddf29f079b1": {
          "content": {
            "bytes": 4456,
            "sha256": "8d7288187e687f3144f39f7204febdf5b0b2a9084e3ff7ef5b0e754f383dfb19"
          },
          "ordered_parents": [
            "e3dc9ba836100707a9c4313d83bea1872f1f9016"
          ],
          "tree_record": "100644 blob b1c7198a1e68addcfca97dbfa712d4e779d51dc1\tsrc/backend/space/internal/lifecycleprincipal/interceptor.go\u0000"
        },
        "2445e81de07681cfd5f1491e4c793aaac747f571": {
          "content": {
            "bytes": 4456,
            "sha256": "8d7288187e687f3144f39f7204febdf5b0b2a9084e3ff7ef5b0e754f383dfb19"
          },
          "ordered_parents": [
            "5cc425b35bc71738d86261c1cf4772c7babd4519",
            "c14f59c1a43b486d48322d62e7c5baa2816e1253"
          ],
          "tree_record": "100644 blob b1c7198a1e68addcfca97dbfa712d4e779d51dc1\tsrc/backend/space/internal/lifecycleprincipal/interceptor.go\u0000"
        },
        "3071ddad390443de4119c796cdad9899ff08f7fd": {
          "content": {
            "bytes": 4456,
            "sha256": "8d7288187e687f3144f39f7204febdf5b0b2a9084e3ff7ef5b0e754f383dfb19"
          },
          "ordered_parents": [
            "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
            "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd"
          ],
          "tree_record": "100644 blob b1c7198a1e68addcfca97dbfa712d4e779d51dc1\tsrc/backend/space/internal/lifecycleprincipal/interceptor.go\u0000"
        },
        "5cc425b35bc71738d86261c1cf4772c7babd4519": {
          "content": {
            "bytes": 4190,
            "sha256": "b3ab69fe0da12f2c2062d2639561a62bb080af5e682f472022f181f0afc09f08"
          },
          "ordered_parents": [
            "2cc9482255ba18cd948aacb567f4c4bdf8976b14"
          ],
          "tree_record": "100644 blob 5a10c79ade74f91ecefcd6ccc915ee166b978713\tsrc/backend/space/internal/lifecycleprincipal/interceptor.go\u0000"
        },
        "68927e5912ebc218050fcc9daf4b76cf4a9e3c86": {
          "content": {
            "bytes": 4456,
            "sha256": "8d7288187e687f3144f39f7204febdf5b0b2a9084e3ff7ef5b0e754f383dfb19"
          },
          "ordered_parents": [
            "6c35de6ae45e0c53b2cf40fbf5bef235bb3445ea",
            "f954480f69878a5b3890bf6fcb5af978d68dadf9"
          ],
          "tree_record": "100644 blob b1c7198a1e68addcfca97dbfa712d4e779d51dc1\tsrc/backend/space/internal/lifecycleprincipal/interceptor.go\u0000"
        },
        "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172": {
          "content": {
            "bytes": 4190,
            "sha256": "b3ab69fe0da12f2c2062d2639561a62bb080af5e682f472022f181f0afc09f08"
          },
          "ordered_parents": [
            "b2cdc3ad251fdc64d1d052c52a8bea2d5d28dabe"
          ],
          "tree_record": "100644 blob 5a10c79ade74f91ecefcd6ccc915ee166b978713\tsrc/backend/space/internal/lifecycleprincipal/interceptor.go\u0000"
        },
        "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c": {
          "content": {
            "bytes": 4456,
            "sha256": "8d7288187e687f3144f39f7204febdf5b0b2a9084e3ff7ef5b0e754f383dfb19"
          },
          "ordered_parents": [
            "dc0acdefec7e5c413988adf71f86d12bff4017bc",
            "6780407299a3beb77e92b32c551ca7cfe3e63e94"
          ],
          "tree_record": "100644 blob b1c7198a1e68addcfca97dbfa712d4e779d51dc1\tsrc/backend/space/internal/lifecycleprincipal/interceptor.go\u0000"
        },
        "a635c0ad19a233f9bd934dca57943bc6363a6da3": {
          "content": {
            "bytes": 3592,
            "sha256": "64c7424ea672ca6c28ffec5d4473de84fa3e951d3416544aa4ae9df2cdc85206"
          },
          "ordered_parents": [
            "31276e7cdd661ec93cb9c2e0369a0f102adf7efd",
            "fdf42784969cbf76bbec7eb1cc6ac878c31acd6e"
          ],
          "tree_record": "100644 blob d54c14fee68d5c97fcb665d7faadd9b5c3df69fb\tsrc/backend/space/internal/lifecycleprincipal/interceptor.go\u0000"
        },
        "c14f59c1a43b486d48322d62e7c5baa2816e1253": {
          "content": {
            "bytes": 4456,
            "sha256": "8d7288187e687f3144f39f7204febdf5b0b2a9084e3ff7ef5b0e754f383dfb19"
          },
          "ordered_parents": [
            "18206e47ff06f1071f69319e9a6eeddf29f079b1"
          ],
          "tree_record": "100644 blob b1c7198a1e68addcfca97dbfa712d4e779d51dc1\tsrc/backend/space/internal/lifecycleprincipal/interceptor.go\u0000"
        },
        "e020da4b4edcded41bde9785615262beb1946ad2": {
          "content": {
            "bytes": 4190,
            "sha256": "b3ab69fe0da12f2c2062d2639561a62bb080af5e682f472022f181f0afc09f08"
          },
          "ordered_parents": [
            "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172"
          ],
          "tree_record": "100644 blob 5a10c79ade74f91ecefcd6ccc915ee166b978713\tsrc/backend/space/internal/lifecycleprincipal/interceptor.go\u0000"
        },
        "e3dc9ba836100707a9c4313d83bea1872f1f9016": {
          "content": {
            "bytes": 4190,
            "sha256": "b3ab69fe0da12f2c2062d2639561a62bb080af5e682f472022f181f0afc09f08"
          },
          "ordered_parents": [
            "420c4c7be8c64583ab3e26f0560e5970f705a46e",
            "34640fde433c13073246003b1d009d2c7dba530c"
          ],
          "tree_record": "100644 blob 5a10c79ade74f91ecefcd6ccc915ee166b978713\tsrc/backend/space/internal/lifecycleprincipal/interceptor.go\u0000"
        }
      }
    }
  ],
  "contribution_context": {
    "owner": "2445e81de07681cfd5f1491e4c793aaac747f571",
    "paths": [
      {
        "J_blob": "76bafd0ba2878ba7eb21d29b1fa0f78c07087e9c",
        "owner_blob": "80c9845acd6d3161f40f6f03b2dfba9640247a4f",
        "path": "docs/microservices/space-service.md"
      },
      {
        "J_blob": "d4df819e22421b89a348bad86182854b7e4dd471",
        "owner_blob": "d4df819e22421b89a348bad86182854b7e4dd471",
        "path": "docs/testing/be041-verified-audit-identity-exec-plan.md"
      },
      {
        "J_blob": "42dfdc6a82f218abd1be7d53ba1ca47589999c95",
        "owner_blob": "e9b43fdda794f67b8285681d5acc8f0a0e354440",
        "path": "docs/todo/backend.md"
      },
      {
        "J_blob": "6a313db744abd6c626c4dcb988e94466bae6f72f",
        "owner_blob": "6a313db744abd6c626c4dcb988e94466bae6f72f",
        "path": "src/backend/gateway/be041_audit_route_test.go"
      },
      {
        "J_blob": "68daa3a03533f7410e8dcc527d2efa427f36ed2a",
        "owner_blob": "68daa3a03533f7410e8dcc527d2efa427f36ed2a",
        "path": "src/backend/pkg/principal/user_identity.go"
      },
      {
        "J_blob": "73610ccb065c86e16347e8f58f75cc1b65d329d5",
        "owner_blob": "73610ccb065c86e16347e8f58f75cc1b65d329d5",
        "path": "src/backend/pkg/principal/user_identity_test.go"
      },
      {
        "J_blob": "d24e08d221c42661e8b1431defe61d742b23e8d9",
        "owner_blob": "d24e08d221c42661e8b1431defe61d742b23e8d9",
        "path": "src/backend/space/internal/grpcsvc/audit.go"
      },
      {
        "J_blob": "788a201984a41d690cd478f74cedc9f8c6334372",
        "owner_blob": "788a201984a41d690cd478f74cedc9f8c6334372",
        "path": "src/backend/space/internal/grpcsvc/audit_identity_test.go"
      },
      {
        "J_blob": "8440d9a59c0dab16b9359c7270b62bcc2921581c",
        "owner_blob": "8440d9a59c0dab16b9359c7270b62bcc2921581c",
        "path": "src/backend/space/internal/grpcsvc/audit_integration_test.go"
      },
      {
        "J_blob": "1e83d56c2d657a4c213327c1937120fccaeded77",
        "owner_blob": "1e83d56c2d657a4c213327c1937120fccaeded77",
        "path": "src/backend/space/internal/grpcsvc/audit_public_read.go"
      },
      {
        "J_blob": "745f4f4e6f4d220bc7a799a784570c898e5190d2",
        "owner_blob": "745f4f4e6f4d220bc7a799a784570c898e5190d2",
        "path": "src/backend/space/internal/grpcsvc/audit_public_read_test.go"
      },
      {
        "J_blob": "ec52e085a08b722f7c8d9c2e2b842e7eea24df46",
        "owner_blob": "ec52e085a08b722f7c8d9c2e2b842e7eea24df46",
        "path": "src/backend/space/internal/grpcsvc/be041_audit_route_test.go"
      },
      {
        "J_blob": "49a40875541c8061170b4a0169cb06eb55a89505",
        "owner_blob": "49a40875541c8061170b4a0169cb06eb55a89505",
        "path": "src/backend/space/internal/grpcsvc/ownership_freeze_integration_test.go"
      },
      {
        "J_blob": "7cd791cd516e7ebb8c9f909ee8f241a70533e339",
        "owner_blob": "7cd791cd516e7ebb8c9f909ee8f241a70533e339",
        "path": "src/backend/space/internal/grpcsvc/roles.go"
      },
      {
        "J_blob": "da13f11ef9f4ffd85c707aed6af634121c63262a",
        "owner_blob": "da13f11ef9f4ffd85c707aed6af634121c63262a",
        "path": "src/backend/space/internal/grpcsvc/space_integration_test.go"
      },
      {
        "J_blob": "7d308d251bef0ddc0e6954b543dcf2a0b1be5c45",
        "owner_blob": "7d308d251bef0ddc0e6954b543dcf2a0b1be5c45",
        "path": "src/backend/space/internal/lifecycleprincipal/audit_test.go"
      },
      {
        "J_blob": "b1c7198a1e68addcfca97dbfa712d4e779d51dc1",
        "owner_blob": "b1c7198a1e68addcfca97dbfa712d4e779d51dc1",
        "path": "src/backend/space/internal/lifecycleprincipal/interceptor.go"
      }
    ],
    "scope": "Original contribution changed-source context only; not a full compiled closure or additional permission set"
  },
  "history_boundary": "7bd7dfba7e8f85bc0572617c6ef3af4a15e59172",
  "operation": "3071ddad390443de4119c796cdad9899ff08f7fd",
  "ordered_parents": [
    "a45862ca7e2e885ae4e36afc758c9e9fe39ca86c",
    "140f6612e2f9df22aa9ac0c60744d5d41fe7fbcd"
  ],
  "original_audit_permission_unchanged": true,
  "runtime_pass": false,
  "schema": "F54-FINITE-J-HISTORICAL-SPACE-OWNER-v1",
  "scope": "Exact historical owner transitions only; no current source equality or runtime permission",
  "source_review": {
    "bytes": 8328,
    "sha256": "ca00b8ef37970dbdd3d9106cbd8c9bb29f8e62f209f03710aa15b60d0c607585"
  }
}
'''.encode()
FINITE_J_SOURCE_UNIT_SHA = 'e455c1059f0e59ffe127843957dc603e9f0a5917aff1e880976400e651d6ebad'


def finite_J_source_unit(repo):
    raw=FINITE_J_SOURCE_UNIT_RAW
    require(sha(raw)==FINITE_J_SOURCE_UNIT_SHA,'finite_J_frozen_source_unit')
    unit=json.loads(raw)
    require(len(unit['cases'])==10 and unit['operation']==J3071 and unit['admission'] is False
            and unit['runtime_pass'] is False,'finite_J_exact_historical_unit')
    require(len({case['path'] for case in unit['cases']})==10
            and sum(len(case['versions']) for case in unit['cases'])==114
            and sum(len(case['edges']) for case in unit['cases'])==168,'finite_J_complete_source_inventory')
    require(git(repo,'rev-list','--parents','-n','1',J3071).decode().split()[1:]==unit['ordered_parents'],
            'finite_J_ordered_parents')
    require(git(repo,'merge-base','--all',*unit['ordered_parents']).decode().splitlines()==unit['best_bases'],
            'finite_J_actual_best_bases')
    for case in unit['cases']:
        name=case['path'];prior=case['prior'].split('\t');incoming=case['incoming'].split('\t')
        require(name!=EXACT_J_AUDIT_PATH and prior[0]==incoming[0]==name
                and prior[1]=='false' and incoming[1]=='true','finite_J_distinct_exact_rows')
        for ref,version in case['versions'].items():
            entry=git(repo,'ls-tree','-z',ref,'--',':(literal)'+name).decode()
            require(entry==version['tree_record'],'finite_J_historical_tree_entry')
            require((file_digest(git(repo,'show',ref+':'+name)) if entry else None)==version['content'],
                    'finite_J_historical_complete_bytes')
            require(git(repo,'rev-list','--parents','-n','1',ref).decode().split()[1:]==version['ordered_parents'],
                    'finite_J_historical_ordered_parents')
        for edge in case['edges'].values():
            require(file_digest(git(repo,'diff','--raw','-z','--no-abbrev','--no-renames',edge['before'],edge['after'],'--',name))==edge['raw']
                    and file_digest(git(repo,'diff','--no-ext-diff','--no-renames','--abbrev=9',edge['before'],edge['after'],'--',name))==edge['patch'],
                    'finite_J_historical_complete_edges')
        git(repo,'merge-base','--is-ancestor',prior[2],J3071_P1)
        git(repo,'merge-base','--is-ancestor',prior[5],J3071_P2)
    for item in unit['contribution_context']['paths']:
        for ref,key in ((unit['contribution_context']['owner'],'owner_blob'),(J3071,'J_blob')):
            require(git(repo,'rev-parse',ref+':'+item['path']).decode().strip()==item[key],
                    'finite_J_original_contribution_context')
    return unit


def finite_J_paths():
    # Selection only. No permission is established by this finite name list.
    require(sha(FINITE_J_SOURCE_UNIT_RAW)==FINITE_J_SOURCE_UNIT_SHA,'finite_J_frozen_source_unit')
    return [case['path'] for case in json.loads(FINITE_J_SOURCE_UNIT_RAW)['cases']]


def finite_J_owner_evidence(repo, capability, ready, events, prefix, actual_map):
    """Called with process-local fullfold state or inside the independent full fold."""
    require(capability['type']=='F54_FIXED3071_PRODUCTION_CAPABILITIES_v1','finite_J_production_only')
    checked_3071_capability_value(repo,capability)
    unit=finite_J_source_unit(repo);names=[c['path'] for c in unit['cases']]
    before=events[-1];value=before['payload'];name=value.get('path')
    require(name in names,'finite_J_exact_path')
    index=names.index(name);case=unit['cases'][index]
    require(before['kind']=='row_before' and before['frame']==ready['frame']
            and before['operation']==ready['operation'] and ready['operation'].split('/')[1:]==[J3071,'2'],
            'finite_J_actual_pending_operation')
    paths=ready['operation_binding']['paths'];positions=[paths.index(n) for n in names]
    require(positions==list(range(positions[0],positions[0]+10)),'finite_J_exact_contiguous_operation_order')
    require(value['incoming']==raw_binding(case['incoming']) and value['prior']==raw_binding(case['prior'])
            and value['prior_rows']==[raw_binding(case['prior'])],'finite_J_complete_literal_rows')
    require(value['map_before']==file_digest(actual_map)
            and [x for x in actual_map.decode().splitlines() if x.split('\t')[0]==name]==[case['prior']],
            'finite_J_complete_pending_map')
    completed=[e['payload']['path'] for e in events if e['kind']=='row_committed'
               and e['operation']==ready['operation'] and e['payload']['path'] in names]
    require(completed==names[:index],'finite_J_exact_prior_unit_order')
    require(not any(e['kind']=='finite_J_owner_permission' and e['operation']==ready['operation']
                    and e['payload']['path']==name for e in events),'finite_J_no_replayed_permission')
    require(ready['capability_sha256']==sha(canonical(capability))
            and ready['composition']['whole']==manual_3071_composition(repo),'finite_J_current_whole_composition')
    obligation=ready['required_qualification'][name]
    require(obligation['aggregate'] is True and obligation['retained_evidence']
            and obligation['inherited_qualification'] and isinstance(obligation['prior_span_evidence'],list),
            'finite_J_complete_true_obligations')
    writers=[e for e in events if e['kind']=='row_committed' and e['frame']==before['frame']
             and e['payload']['row']==raw_binding(case['prior'])]
    require(len(writers)==1,'finite_J_one_actual_prior_writer')
    prior_op=writers[0]['operation']
    references=[e['digest'] for e in events if e['operation']==prior_op and
                (e['kind'] in ('operation_begin','classification') or
                 (e['kind'] in ('row_after','row_committed') and e['payload'].get('path')==name))]
    require(references,'finite_J_actual_prior_operation_references')
    return dict(type='F54_FINITE_J_HISTORICAL_SPACE_OWNER_PRODUCTION_v1',source_unit_sha256=sha(FINITE_J_SOURCE_UNIT_RAW),
        path=name,index=index,paths=names,operation=ready['operation'],frame=ready['frame'],
        capability_sha256=sha(canonical(capability)),ledger_sha256=sha(canonical(ready)),
        prior_operation=prior_op,prior_references=references,prior_rows=value['prior_rows'],incoming=value['incoming'],
        map_before=file_digest(actual_map),trace_prefix=prefix,pending_digest=before['digest'],
        obligations=obligation,aggregate=True,source_rebind=False,current_source_equivalence=False,admission=False)


def finite_J_conservation(ready, permission, actual_map):
    name=permission['path'];incoming=permission['incoming'];rows=actual_map.decode().splitlines()
    require(permission['type']=='F54_FINITE_J_HISTORICAL_SPACE_OWNER_PRODUCTION_v1'
            and permission['ledger_sha256']==sha(canonical(ready)) and permission['map_before']==file_digest(actual_map),
            'finite_J_conservation_current_binding')
    require(permission['obligations']==ready['required_qualification'][name]
            and permission['obligations']['aggregate'] is True,'finite_J_conservation_complete_obligations')
    prior=[raw_binding(x) for x in rows if x.split('\t')[0]==name]
    require(prior==permission['prior_rows'] and len(prior)==1 and row(incoming)[1]=='true',
            'finite_J_conservation_exact_replacement')
    output=[x for x in rows if x.split('\t')[0]!=name]+[incoming['raw']]
    return dict(path=name,ledger_sha256=permission['ledger_sha256'],incoming=incoming,after_gate_row=incoming,
        finite_J_owner_permission=permission,prior_rows=prior,map_before=file_digest(actual_map),transition='replace',
        expected_post_map=file_digest(('\n'.join(output)+'\n').encode()),aggregate=True,
        obligations=ready['required_qualification'][name],policy_result=None,admission=False)


def exact_J_audit_source_unit(repo):
    raw = EXACT_J_SOURCE_UNIT_RAW
    require(sha(raw) == 'e8fce2a60ae0975ddcaee47db3cf36a51e57744194f5885bb8953ee2406a8f02',
            'exact_J_frozen_full_source_unit')
    unit = json.loads(raw)
    require(unit['path'] == EXACT_J_AUDIT_PATH and len(unit['versions']) == 12 and len(unit['edges']) == 6,
            'exact_J_complete_versions_and_edges')
    for version in unit['versions'].values():
        require(git(repo, 'ls-tree', '-z', version['ref'], '--', EXACT_J_AUDIT_PATH).decode() == version['tree_record'],
                'exact_J_actual_serialized_tree_record')
        content = git(repo, 'show', version['ref']+':'+EXACT_J_AUDIT_PATH)
        require(file_digest(content) == dict(bytes=version['bytes'], sha256=version['sha256']),
                'exact_J_full_historical_source_bytes')
        require(git(repo, 'rev-list', '--parents', '-n', '1', version['ref']).decode().split()[1:] == version['ordered_parents'],
                'exact_J_source_ordered_parents')
    for edge in unit['edges'].values():
        require(sha(git(repo, 'diff', '--raw', '-z', '--no-abbrev', '--no-renames', edge['from'], edge['to'], '--', EXACT_J_AUDIT_PATH)) == edge['raw_sha256']
            and sha(git(repo, 'diff', '--no-ext-diff', '--no-renames', '--abbrev=9', edge['from'], edge['to'], '--', EXACT_J_AUDIT_PATH)) == edge['patch_sha256'],
            'exact_J_complete_source_parent_edge')
    git(repo, 'merge-base', '--is-ancestor', EXACT_J_AUDIT_PRIOR.split('\t')[2], J3071_P1)
    git(repo, 'merge-base', '--is-ancestor', EXACT_J_AUDIT_PRIOR.split('\t')[5], J3071_P2)
    return dict(sha256=sha(raw), source_unit=unit)


def exact_J_owner_evidence(repo, capability, ready, events, prefix, actual_map):
    """Pure permission constructor, called only with independently folded state.

    Both the pending checkpoint and permission event verifier reconstruct all
    earlier events and J_ready before calling this function. A ready JSON alone
    is deliberately not a public constructor input.
    """
    if capability['type']=='F54_FIXED3071_PRODUCTION_CAPABILITIES_v1':
        checked_3071_capability_value(repo,capability)
        permission_type='F54_EXACT_J_AUDIT_OWNER_TRANSITION_PRODUCTION_v1'
    else:
        require(capability['type'] == 'F54_FIXED3071_CURRENT_AUTHOR_CAPABILITIES', 'exact_J_AUTHOR_only')
        permission_type='F54_EXACT_J_AUDIT_OWNER_TRANSITION_AUTHOR_v1'
    before = events[-1]
    require(before['kind'] == 'row_before' and before['operation'] == ready['operation']
        and before['frame'] == ready['frame'] and ready['operation'].split('/')[1:] == [J3071, '2'],
        'exact_J_actual_pending_operation')
    value = before['payload']
    require(value['path'] == EXACT_J_AUDIT_PATH and value['incoming'] == raw_binding(EXACT_J_AUDIT_INCOMING)
        and value['prior_rows'] == [raw_binding(EXACT_J_AUDIT_PRIOR)]
        and value['prior'] == raw_binding(EXACT_J_AUDIT_PRIOR), 'exact_J_complete_fixed_raw_rows')
    require(value['map_before'] == file_digest(actual_map)
        and [x for x in actual_map.decode().splitlines() if x.split('\t')[0] == EXACT_J_AUDIT_PATH] == [EXACT_J_AUDIT_PRIOR],
        'exact_J_independently_folded_complete_map')
    require(ready['capability_sha256'] == sha(canonical(capability))
        and ready['composition']['whole'] == manual_3071_composition(repo), 'exact_J_current_capability_whole_composition')
    require(ready['required_qualification'][EXACT_J_AUDIT_PATH]['aggregate'] is True,
            'exact_J_complete_retained_true_obligation')
    # Keep the actual prior writer references, not just an equal endpoint blob.
    prior_commits = [e for e in events if e['kind'] == 'row_committed'
                    and e['payload']['row'] == raw_binding(EXACT_J_AUDIT_PRIOR) and e['frame'] == before['frame']]
    require(len(prior_commits) == 1, 'exact_J_one_actual_prior_writer')
    prior_op = prior_commits[0]['operation']
    references = [e['digest'] for e in events if e['operation'] == prior_op
                  and (e['kind'] in ('operation_begin', 'classification') or
                       (e['kind'] in ('row_after', 'row_committed') and e['payload'].get('path') == EXACT_J_AUDIT_PATH))]
    require(references, 'exact_J_actual_prior_operation_references')
    return dict(type=permission_type,
        source_unit=exact_J_audit_source_unit(repo), capability_sha256=sha(canonical(capability)),
        operation=ready['operation'], frame=ready['frame'], frame_ancestry=ready['frame_ancestry'],
        operation_binding=ready['operation_binding'], prior_operation=prior_op, prior_references=references,
        prior_rows=value['prior_rows'], incoming=value['incoming'], composition=ready['composition'],
        ledger_prefix=ready['prefix'], ledger_sha256=sha(canonical(ready)), map_before=file_digest(actual_map),
        obligations=ready['required_qualification'][EXACT_J_AUDIT_PATH], aggregate=True,
        trace_prefix=prefix, pending_digest=before['digest'], source_rebind=False, admission=False)


def qualify_exact_J_owner(repo, capability_path, ledger_path, ledger_sha, trace_path, name, actual_map, incoming):
    raw = Path(trace_path).read_bytes()
    result = validate_trace(repo, trace_path, sha(raw), exact_J_checkpoint='pending')
    receipt = result['exact_J_permission']
    require(name == EXACT_J_AUDIT_PATH and incoming == EXACT_J_AUDIT_INCOMING
        and receipt['map_before'] == file_digest(actual_map), 'exact_J_caller_exact_pending_binding')
    ledger = Path(ledger_path).read_bytes()
    require(sha(ledger) == ledger_sha and sha(canonical(json.loads(ledger))) == receipt['ledger_sha256'],
            'exact_J_caller_complete_reconstructed_ledger')
    require(sha(canonical(verified_3071_capabilities(repo, capability_path))) == receipt['capability_sha256'],
            'exact_J_caller_current_AUTHOR_capability')
    return receipt


def qualified_J_native_normalization(value, events, before, name, actual_map, prior, incoming, after):
    """Reconstruct the existing clean-zero shell transition, never a new grant."""
    previous='0'*64
    for sequence,event in enumerate(events,1):
        body=dict(event);digest=body.pop('digest',None)
        require(event.get('schema')==SCHEMA and event.get('sequence')==sequence
            and event.get('previous')==previous and sha(canonical(body))==digest,'fixed3071_J_native_append_only_trace')
        previous=digest
    require(before['frame']==value['frame'] and before['operation']==value['operation']
        and events[before['sequence']-1]==before,'fixed3071_J_native_pending_frame')
    tail=events[before['sequence']:]
    require(len(tail)==1 and tail[0]['kind']=='native_rebind_gate'
        and tail[0]['frame']==before['frame'] and tail[0]['operation']==before['operation'],'fixed3071_J_native_exact_pending_gate')
    gate=tail[0]['payload'];operation=value['operation_binding']
    require(prior and gate['path']==name and gate['gate']=='native_p3_duplicate_owner_qualified'
        and type(gate['status']) is int and gate['status']==0 and gate['outcome'] in ('clean','guarded_conflict')
        and gate['classifier']==classifier_binding(),'fixed3071_J_native_original_clean_zero')
    require([gate[k] for k in ('prior_owner','prior_source','current_owner','current_source','integration')]
        ==[row(prior[-1])[2],row(prior[-1])[5],incoming[2],incoming[5],operation['commit']],
        'fixed3071_J_native_exact_owner_source_arguments')
    parents=git(ROOT,'rev-list','--parents','-n','1',J3071).decode().split()
    require(parents==[J3071,J3071_P1,J3071_P2] and operation['commit']==J3071
        and operation['first_parent']==J3071_P1 and operation['source_head']==J3071_P2 and operation['boundary']==J3071_O
        and incoming[3]==parents[1] and incoming[4:6]==[J3071_O,J3071_P2]
        and incoming[7]==','.join(operation['candidates']) and after[4:6]==[parents[1],parents[0]],'fixed3071_J_native_immutable_parent_normalization')
    # Fold actual already-conserved J transitions from the sealed initial map.
    # A rehashed pending map cannot erase or reorder unrelated retained rows.
    current=list(value['outer_rows']);receipts={}
    for event in events:
        if event['sequence']>=before['sequence']:break
        # Prefix bytes may use noncanonical whitespace. Its last sequence is
        # determined by its already-verified raw byte seal by the caller.
        if event['sequence']<=value['_prefix_events']:continue
        if event['frame']!=value['frame']:continue
        payload=event['payload'];path=payload.get('path')
        if event['kind']=='range_J_ledger_conserved':
            receipt=payload['receipt']
            require(event['operation']==value['operation'] and receipt['ledger_sha256']==value['_ledger_sha256']
                and receipt['map_before']==file_digest((''.join(x+'\n' for x in current)).encode()),'fixed3071_J_native_prior_conservation')
            receipts[path]=receipt
        elif event['kind']=='row_remove':
            require(path in receipts and receipts[path]['transition']=='replace'
                and payload['row']==receipts[path]['after_gate_row'],'fixed3071_J_native_prior_exact_remove')
            current=[x for x in current if x.split('\t')[0]!=path]
        elif event['kind']=='row_committed':
            require(path in receipts and payload['row']==receipts[path]['after_gate_row'],'fixed3071_J_native_prior_exact_commit')
            row(payload['row']);current.append(payload['row']['raw'])
            require(payload['map_after']==receipts[path]['expected_post_map']==file_digest((''.join(x+'\n' for x in current)).encode()),'fixed3071_J_native_prior_postmap')
            del receipts[path]
    require(not receipts and actual_map==(''.join(x+'\n' for x in current)).encode(),'fixed3071_J_native_complete_current_map')
    binding=gate['binding'];require(binding==raw_binding(binding['raw']),'fixed3071_J_native_binding_raw_hash')
    require(events[0]['kind']=='master_entry' and events[0]['payload']['native_binding']==binding
        and events[0]['payload']['context']==gate['context'],'fixed3071_J_native_original_master_context')
    spec=importlib.util.spec_from_file_location('f54_J_native_rebind',ROOT/('scripts/ci/f13_native_p3_authority.py' if (ROOT/'scripts/ci/f13_native_p3_authority.py').is_file() else 'scripts/ci/f13_native_p3_source.py'))
    native=importlib.util.module_from_spec(spec);spec.loader.exec_module(native)
    bound=json.loads(binding['raw']);context=gate['context']
    try:
        qualified=native.qualify(ROOT,bound['base'],bound['source'],bound['target'],context['GITHUB_EVENT_NAME'],
            context['GITHUB_REF'],context['GITHUB_SHA'],context['VOICE_CI_PUSH_BEFORE_SHA'],context['GITHUB_BASE_REF'])
    except native.Rejected as error:raise Rejected('fixed3071_J_native_authority_'+str(error)) from error
    require(qualified==bound and bound['target']==git(ROOT,'rev-parse','HEAD').decode().strip()
        and name in bound['paths'],'fixed3071_J_native_independent_current_authority')
    for candidate in (gate['prior_owner'],gate['prior_source'],gate['current_owner'],gate['current_source'],gate['integration']):
        git(ROOT,'merge-base','--is-ancestor',candidate,bound['snapshot'])
    require(tree_entry(ROOT,J3071,name)==tree_entry(ROOT,bound['snapshot'],name),'fixed3071_J_native_actual_snapshot_restoration')
    return dict(event_digest=tail[0]['digest'],binding_sha256=binding['sha256'],normalized_source=after[4:6])


def consume_3071_J_path(ready_path, expected_sha, trace_path, name, actual_map, after_raw, transition):
    require(name not in finite_J_paths(),'finite_J_requires_live_validated_worker')
    raw=Path(ready_path).read_bytes();require(sha(raw)==expected_sha,'fixed3071_J_complete_ledger_raw_hash')
    value=json.loads(raw);require(value['type']=='F54_FIXED3071_ACTUAL_J_COMPLETE_LEDGER' and value['admission'] is False,'fixed3071_J_distinct_ledger_type')
    state=dict(frame=value['frame'],operation=value['operation'],operation_binding=value['operation_binding'],classification=value['classification'],
        capability_sha256=value['capability_sha256'],frame_ancestry=value['frame_ancestry'],ledger=value['ledger'],coverage=value['coverage'],
        prior_span=value['prior_span'],M_ready=value['inherited_M_ledgers'],I_ready=value['inherited_I_ledgers'],composition=value['composition'])
    require(canonical(value)==canonical(build_3071_J_ready(state,value['prefix'])),'fixed3071_J_reads_complete_evidence_not_only_OR_bit')
    require(name in value['operation_binding']['paths'],'fixed3071_J_actual_operation_path')
    trace=Path(trace_path).read_bytes();prefix=value['prefix']
    require(len(trace)>=prefix['bytes'] and sha(trace[:prefix['bytes']])==prefix['sha256'],'fixed3071_J_sealed_real_prefix')
    require(value['terminal_dispositions']==terminal_dispositions_in_prefix(value['ledger'],trace,prefix),'fixed3071_J_retains_terminal_obligations')
    require(value['manual_result_operations']==manual_operations_in_prefix(value['ledger'],trace,prefix),'fixed3071_J_retains_actual_manual_operations')
    events=[json.loads(x) for x in trace.splitlines()]
    before=next(e for e in reversed(events) if e['kind']=='row_before' and e['operation']==value['operation'])
    require(before['payload']['path']==name and before['payload']['map_before']==file_digest(actual_map),'fixed3071_J_current_map_and_pending_path')
    rows=actual_map.decode().splitlines();prior=[raw_binding(x) for x in rows if x.split('\t')[0]==name]
    require(prior==before['payload']['prior_rows'] and prior==[raw_binding(x) for x in value['outer_rows'] if x.split('\t')[0]==name],'fixed3071_J_all_original_prior_rows')
    incoming=row(before['payload']['incoming']);after=row(raw_binding(after_raw))
    require(incoming[2]==J3071 and all(after[i]==incoming[i] for i in (0,2,3,6,7)),'fixed3071_J_no_owner_source_content_rebind')
    normalization=None
    if after[4:6]!=incoming[4:6]:
        require(transition=='replace','fixed3071_J_native_normalization_requires_existing_owner')
        native_value=dict(value,_prefix_events=len(trace[:prefix['bytes']].splitlines()),_ledger_sha256=sha(canonical(value)))
        normalization=qualified_J_native_normalization(native_value,events,before,name,actual_map,prior,incoming,after)
    aggregate=value['required_qualification'][name]['aggregate'] or any(row(x)[1]=='true' for x in prior) or incoming[1]=='true' or after[1]=='true'
    require(transition in ('append','replace') and (transition!='replace' or prior),'fixed3071_J_actual_transition')
    require((transition=='append' and after==incoming) or (transition=='replace' and (after[1]==incoming[1] or (after[1]=='true' and aggregate))),'fixed3071_J_raw_append_conserved_reduction')
    exact_permission=None
    if before['payload']['incoming']==raw_binding(EXACT_J_AUDIT_INCOMING) and normalization is None:
        require(transition=='replace' and after_raw==EXACT_J_AUDIT_INCOMING,
                'exact_J_consumer_preserves_whole_incoming_identity')
        checked=validate_trace(ROOT,trace_path,sha(trace),exact_J_checkpoint='permitted')
        exact_permission=checked['exact_J_permission']
        require(exact_permission['ledger_sha256']==sha(canonical(value))
            and exact_permission['map_before']==file_digest(actual_map), 'exact_J_consumer_current_full_binding')
    output=([x for x in rows if x.split('\t')[0]!=name] if transition=='replace' else rows)+[after_raw]
    return dict(path=name,ledger_sha256=sha(canonical(value)),incoming=before['payload']['incoming'],after_gate_row=raw_binding(after_raw),
        **(dict(exact_J_owner_permission=exact_permission) if exact_permission else {}),
        **(dict(native_normalization=normalization) if normalization else {}),
        prior_rows=prior,map_before=file_digest(actual_map),transition=transition,expected_post_map=file_digest(('\n'.join(output)+'\n').encode()),
        aggregate=aggregate,obligations=value['required_qualification'][name],policy_result=None,admission=False)


def build_3071_I_ready(state, prefix):
    """A complete actual ledger, not six Git-node rows or a sidecar true bit."""
    required={}
    require(state['classification']['trigger'] in ('true','false'),'fixed3071_I_original_raw_classifier_type')
    def item(name):
        return required.setdefault(name,dict(aggregate=False,retained_evidence=[],range_evidence=[],inherited_qualification=[],I_classification=None))
    for frame,ledger in state['ledger'].items():
        for name,value in ledger['paths'].items():
            require(type(value['aggregate']) is bool,'fixed3071_I_retained_aggregate_boolean')
            target=item(name);target['aggregate']|=value['aggregate']
            target['retained_evidence'].append(dict(frame=frame,map_binding=ledger['map_binding'],evidence=value['evidence']))
    for operation,ready in state['M_ready'].items():
        inherited_sha=sha(canonical(ready))
        for name,value in ready['required_qualification'].items():
            require(type(value['aggregate']) is bool,'fixed3071_I_inherited_aggregate_boolean')
            target=item(name);target['aggregate']|=value['aggregate']
            target['inherited_qualification'].append(dict(operation=operation,ledger_sha256=inherited_sha,obligation=value))
    branch=state['range_branch'];side=branch['side_ledger']
    for name,value in side['paths'].items():
        require(type(value['aggregate']) is bool,'fixed3071_I_supplemental_aggregate_boolean')
        target=item(name);target['aggregate']|=value['aggregate']
        target['range_evidence'].append(dict(trace_sha256=branch['side_trace_sha256'],map_binding=side['map_binding'],
            rows=[raw_binding(x) for x in side['rows'] if x.split('\t')[0]==name],evidence=value['evidence'],
            actual_operations=branch['side_operations']))
    for name in state['operation_binding']['paths']:
        target=item(name);target['I_classification']=state['classification']
        target['aggregate']|=state['classification']['trigger']=='true'
    outer=state['ledger'][state['frame']]
    return dict(type='F54_FIXED3071_ACTUAL_I_RANGE_LEDGER',operation=state['operation'],frame=state['frame'],classification=state['classification'],operation_binding=state['operation_binding'],
        capability_sha256=state['capability_sha256'],prefix=prefix,frame_ancestry=state['frame_ancestry'],
        ledger=state['ledger'],terminal_dispositions=terminal_dispositions(state['ledger']),
        manual_result_operations=manual_operations(state['ledger']),
        range_branch=branch,coverage=state['coverage'],inherited_M_ledgers=state['M_ready'],
        outer_rows=outer['rows'],outer_map_binding=outer['map_binding'],required_qualification=required,
        qualification='HOLD',admission=False,runtime_pass=False)


def prepare_3071_I_ledger(repo, capability_path, trace_path, branch_path):
    cap=verified_3071_capabilities(repo,capability_path);branch=json.loads(Path(branch_path).read_bytes())
    raw=Path(trace_path).read_bytes();operation=branch['origin']['state']['integration_operation']
    state=validate_trace(repo,trace_path,sha(raw),I_checkpoint=operation)['I_checkpoint']
    require(state['capability_sha256']==sha(canonical(cap)) and state['range_branch']==branch,'fixed3071_I_same_actual_capability_and_range')
    return build_3071_I_ready(state,dict(bytes=len(raw),sha256=sha(raw)))


def consume_3071_I_path(ready_path, expected_sha, trace_path, name, actual_map, after_raw, transition):
    raw=Path(ready_path).read_bytes();require(sha(raw)==expected_sha,'fixed3071_I_complete_ledger_raw_hash')
    value=json.loads(raw);require(value['type']=='F54_FIXED3071_ACTUAL_I_RANGE_LEDGER' and value['admission'] is False,'fixed3071_I_complete_ledger_type')
    reconstructed=build_3071_I_ready(dict(frame=value['frame'],operation=value['operation'],operation_binding=value['operation_binding'],
        classification=value['classification'],capability_sha256=value['capability_sha256'],frame_ancestry=value['frame_ancestry'],
        ledger=value['ledger'],range_branch=value['range_branch'],coverage=value['coverage'],M_ready=value['inherited_M_ledgers']),value['prefix'])
    require(canonical(value)==canonical(reconstructed),'fixed3071_I_consumer_reads_full_retained_and_supplemental_evidence_not_only_a_bit')
    require(value['operation_binding']['commit']==J3071_RANGE['integration'] and name in value['operation_binding']['paths'],'fixed3071_I_actual_operation_path')
    trace=Path(trace_path).read_bytes();prefix=value['prefix']
    require(len(trace)>=prefix['bytes'] and sha(trace[:prefix['bytes']])==prefix['sha256'],'fixed3071_I_consumes_sealed_real_prefix')
    require(value['terminal_dispositions']==terminal_dispositions_in_prefix(value['ledger'],trace,prefix),
        'fixed3071_I_consumes_terminal_obligations_before_overwrite')
    require(value['manual_result_operations']==manual_operations_in_prefix(value['ledger'],trace,prefix), 'manual_F_refreshed_I_ledger_consumed')
    events=[json.loads(x) for x in trace.splitlines()]
    before=next(e for e in reversed(events) if e['kind']=='row_before' and e['operation']==value['operation'])
    require(before['payload']['path']==name and before['payload']['map_before']==dict(bytes=len(actual_map),sha256=sha(actual_map)),'fixed3071_I_actual_current_map_and_path')
    rows=actual_map.decode().splitlines();prior=[raw_binding(x) for x in rows if x.split('\t')[0]==name]
    require(prior==before['payload']['prior_rows'] and prior==[raw_binding(x) for x in value['outer_rows'] if x.split('\t')[0]==name],'fixed3071_I_all_original_prior_rows')
    incoming=row(before['payload']['incoming']);after=row(raw_binding(after_raw))
    require(all(after[i]==incoming[i] for i in (0,2,3,6,7)),'fixed3071_I_immutable_after_gate_identity')
    if after[4:6]!=incoming[4:6]:
        gates=[e for e in events[before['sequence']:] if e['kind']=='native_rebind_gate' and e['operation']==value['operation'] and e['payload']['path']==name]
        require(len(gates)==1 and gates[0]['payload']['status']==0 and after[4:6]==[J3071_RANGE['first'],J3071_RANGE['integration']],'fixed3071_I_only_original_native_gate_rebind')
    aggregate=value['required_qualification'][name]['aggregate'] or any(row(x)[1]=='true' for x in prior) or incoming[1]=='true' or after[1]=='true'
    require(transition in ('append','replace') and (transition!='replace' or prior),'fixed3071_I_actual_transition')
    require((transition=='append' and after==incoming) or (transition=='replace' and (after[1]==incoming[1] or (after[1]=='true' and aggregate))),'fixed3071_I_raw_append_and_conserved_reduction_flag')
    output=([x for x in rows if x.split('\t')[0]!=name] if transition=='replace' else rows)+[after_raw]
    post=('\n'.join(output)+'\n').encode()
    return dict(path=name,ledger_sha256=sha(canonical(value)),incoming=before['payload']['incoming'],after_gate_row=raw_binding(after_raw),
        prior_rows=prior,map_before=dict(bytes=len(actual_map),sha256=sha(actual_map)),transition=transition,
        expected_post_map=dict(bytes=len(post),sha256=sha(post)),aggregate=aggregate,
        obligations=value['required_qualification'][name],policy_result=None,admission=False)


def boundary_3071_capability(repo, path, merge, first, source):
    boundary=exact_3071_boundary(repo,merge,first,source);cap=verified_3071_capabilities(repo,path)
    return dict(boundary,capability_sha256=sha(canonical(cap)))


def verify_3071_document_equation(contents):
    require(set(contents)==set(J3071_DOC_HASHES) and all(sha(contents[k])==v for k,v in J3071_DOC_HASHES.items()),'fixed3071_exact_independently_reviewed_document_bytes')
    old=contents['O'];require(contents['bestA']==contents['bestB']==old,'fixed3071_reviewed_document_bases')
    insertions=[]
    for label,(offset,size,digest) in zip(('P1','P2'),J3071_INSERTIONS):
        addition=contents[label][offset:offset+size]
        require(sha(addition)==digest and contents[label]==old[:offset]+addition+old[offset:],'fixed3071_exact_parent_insertion_preserved')
        insertions.append(addition)
    offset=J3071_INSERTIONS[0][0]
    require(J3071_INSERTIONS[1][0]==offset and contents['J']==old[:offset]+b''.join(insertions)+old[offset:],'fixed3071_reviewed_additive_resolution_not_marker_stripping')
    return dict(offset=offset,parent_insertions=[raw_binding(v) for v in insertions],result_sha256=sha(contents['J']))


def manual_3071_composition(repo):
    exact_3071_boundary(repo,J3071,J3071_P1,J3071_P2)
    arguments=['merge-tree','--write-tree','--name-only','-z','--no-messages',J3071_P1,J3071_P2]
    result=subprocess.run(['git','--no-replace-objects','-C',str(repo),*arguments],capture_output=True)
    require(result.returncode==1 and result.stdout==(J3071_AUTO_TREE+'\0'+J3071_DOC+'\0').encode() and not result.stderr,'fixed3071_actual_conflict_outcome_not_clean_or_native2')
    messages=subprocess.run(['git','--no-replace-objects','-C',str(repo),'merge-tree','--write-tree','-z',J3071_P1,J3071_P2],capture_output=True)
    require(messages.returncode==1 and not messages.stderr and sha(messages.stdout)=='11e08612d3f266710b5390cf341bc89bffd61bff61b2f4e9a2d2331a6ae3aefc','fixed3071_actual_content_conflict_stages_and_messages')
    changed=git(repo,'diff','--name-only',J3071_AUTO_TREE,J3071).decode().splitlines()
    require(changed==[J3071_DOC],'fixed3071_complete_other_tree_equality_no_path_exclusion')
    raw=git(repo,'diff','--raw','--no-abbrev','--no-renames','-z',J3071_AUTO_TREE,J3071)
    patch=git(repo,'diff','--binary','--abbrev=9',J3071_AUTO_TREE,J3071)
    require(sha(raw)=='9f7f1689717879d2877428fae2834a6f69385c118a9d533bbc0b8ed2a8fa0920' and sha(patch)=='78ebeaf659af962865b25a662045825e2bf9c06549fca4d64e6fc762e42358ea','fixed3071_exact_whole_tree_manual_resolution_diff')
    refs=dict(O=J3071_O,bestA=J3071_BASES[0],bestB=J3071_BASES[1],P1=J3071_P1,P2=J3071_P2,J=J3071,AUTO=J3071_AUTO_TREE)
    entries={label:tree_entry(repo,ref,J3071_DOC) for label,ref in refs.items()}
    require(all(entries[label]=='100644 blob '+oid+'\t'+J3071_DOC+'\0' for label,oid in J3071_DOC_OIDS.items()),'fixed3071_all_seven_reviewed_mode_type_oid_entries')
    contents={label:git(repo,'show',ref+':'+J3071_DOC) for label,ref in refs.items()};equation=verify_3071_document_equation(contents)
    return dict(type='F54_FIXED3071_REVIEWED_MANUAL_WHOLE_PROTOTYPE',merge=J3071,parents=[J3071_P1,J3071_P2],
        best_bases=J3071_BASES,history_boundary=J3071_O,auto_tree=J3071_AUTO_TREE,result_tree=J3071_TREE,
        git_outcome='CONFLICT',merge_tree_exit=1,composition='REVIEWED_MANUAL_WHOLE',git_version=git(repo,'--version').decode().strip(),
        arguments=arguments,stdout=raw_binding(result.stdout),stderr=raw_binding(result.stderr),messages=raw_binding(messages.stdout),
        complete_conflict_paths=[J3071_DOC],complete_difference_paths=changed,raw_difference=raw_binding(raw),patch=raw_binding(patch),
        document_entries=entries,reviewed_equation=equation,review_sha256=J3071_MANUAL_REVIEW_SHA,packet_sha256=J3071_MANUAL_PACKET_SHA,admission=False)


def composition_3071_capability(repo, path, merge, boundary, first, source, name, result):
    require([merge,boundary,first,source]==[J3071,J3071_O,J3071_P1,J3071_P2],'fixed3071_composition_tuple')
    cap=verified_3071_capabilities(repo,path);folder=Path(cap['directory'])
    require(sha((folder/'rudeus410-3071-manual-composition-decision.md').read_bytes())==J3071_MANUAL_REVIEW_SHA,'fixed3071_independent_manual_resolution_review')
    packet_raw=(folder/'manual/facts.json').read_bytes();require(sha(packet_raw)==J3071_MANUAL_PACKET_SHA,'fixed3071_frozen_manual_resolution_packet')
    packet=json.loads(packet_raw)
    for filename,expected in packet['bindings'].items():
        require(Path(filename).name==filename and file_binding(folder/'manual'/filename)==expected,'fixed3071_all_manual_evidence_bindings')
    require(name in git(repo,'diff','--name-only',J3071_O,J3071_P2).decode().splitlines(),'fixed3071_actual_source_path_not_future_authority')
    require(result==tree_entry(repo,J3071,name).rstrip('\0'),'fixed3071_exact_consumer_result_entry')
    whole=manual_3071_composition(repo)
    return dict(whole,capability_sha256=sha(canonical(cap)),path=name,result_entry=raw_binding(result),
        path_composition='REVIEWED_DOCUMENT_RESOLUTION' if name==J3071_DOC else 'DETERMINISTIC_UNCONFLICTED_ENTRY_IN_REVIEWED_WHOLE')


def corrected_F_whole(repo, unit_path, target):
    """Fixed source composition with required current repair; never bug-free F.

    This reuses the complete historical-composition/current-correction pattern
    in the explicit author entrypoint. No production R/K or runtime grant.
    """
    raw=Path(unit_path).read_bytes();require(sha(raw)==F_CORRECTED_UNIT_SHA,'corrected_F_fixed_unit_hash')
    u=json.loads(raw);f=F_CORRECTED_TUPLE
    require(set(u)=={'type','merge','parents','best_bases','automatic_tree','result_tree','conflicts','manual_paths','entries',
        'required_current_correction','qualified_current','current_bindings','repair_bindings','historical_gap','source_condition','evidence',
        'producer_only','runtime_pass','activation_authority','current_or_future_path_authority'},'corrected_F_unit_shape')
    require(u['type']=='F13_FIXED_HISTORICAL_COMPOSITION_WITH_REQUIRED_CURRENT_CORRECTION'
        and u['producer_only'] is True and all(u[k] is False for k in ('runtime_pass','activation_authority','current_or_future_path_authority')),'corrected_F_source_only')
    require([u['merge'],u['parents'],u['best_bases'],u['automatic_tree'],u['result_tree'],u['required_current_correction'],u['qualified_current']]
        ==[f['merge'],[f['first'],f['source']],[f['base']],f['automatic_tree'],f['result_tree'],f['correction'],f['current']],'corrected_F_literal_tuple')
    require(u['historical_gap'] and u['source_condition'] and len(u['evidence'])==4
        and all(re.fullmatch('[0-9a-f]{64}',x) for x in u['evidence'].values()),'corrected_F_preserved_gap_and_evidence')
    require(git(repo,'rev-list','--parents','-n','1',f['merge']).decode().split()==[f['merge'],f['first'],f['source']],'corrected_F_actual_parents')
    require(git(repo,'merge-base','--all',f['first'],f['source']).decode().splitlines()==[f['base']],'corrected_F_actual_unique_base')
    require(git(repo,'rev-parse',f['merge']+'^{tree}').decode().strip()==f['result_tree'],'corrected_F_actual_result_tree')
    for older,newer in ((f['merge'],f['correction']),(f['correction'],f['source_ancestor']),
        (f['source_ancestor'],f['current']),(f['source_ancestor'],target)):
        git(repo,'merge-base','--is-ancestor',older,newer)
    mt=subprocess.run(['git','--no-replace-objects','-C',str(repo),'merge-tree','--write-tree','--name-only','-z','--no-messages',f['first'],f['source']],capture_output=True)
    parts=mt.stdout.split(b'\0')
    require(mt.returncode==1 and not mt.stderr and parts[0].decode()==f['automatic_tree'] and parts[-1]==b''
        and [v.decode() for v in parts[1:-1]]==u['conflicts'] and len(u['conflicts'])==2,'corrected_F_actual_conflict_not_clean_alias')
    names=git(repo,'diff','--name-only',f['automatic_tree'],f['merge']).decode().splitlines()
    require(names==u['manual_paths'] and len(names)==33 and names==sorted(set(names))
        and set(u['entries'])==set(names),'corrected_F_complete33_not_selected_path')
    refs=dict(base=f['base'],first=f['first'],source=f['source'],automatic=f['automatic_tree'],historical=f['merge'])
    for name in names:
        require(u['entries'][name]=={label:tree_entry(repo,ref,name) for label,ref in refs.items()},'corrected_F_every_parent_auto_manual_entry')
    require(set(u['repair_bindings'])==set(F_CORRECTED_REPAIR_PATHS),'corrected_F_complete_repair_surface')
    for name,entries in u['repair_bindings'].items():
        require(entries==dict(correction=tree_entry(repo,f['correction'],name),candidate=tree_entry(repo,f['current'],name)),
            'corrected_F_real_correction_entries')
    required=set(names)|set(F_CORRECTED_REPAIR_PATHS)
    require(set(u['current_bindings'])==required,'corrected_F_complete_current_bindings')
    for name,expected in u['current_bindings'].items():
        # Absence is an explicit versioned entry, never an omitted binding.
        # The historical retired test remains in all33 source/history entries;
        # every startup-repair file must instead remain present and exact.
        require(isinstance(expected,str) and (not expected or expected.startswith('100644 blob '))
            and (name not in F_CORRECTED_REPAIR_PATHS or bool(expected)) and expected==tree_entry(repo,f['current'],name)
            and expected==tree_entry(repo,target,name),'corrected_F_required_current_repair_not_regressed')
    return dict(type='F54_CORRECTED_HISTORICAL_WHOLE_PROTOTYPE',merge=f['merge'],parents=u['parents'],best_bases=u['best_bases'],
        automatic_tree=f['automatic_tree'],result_tree=f['result_tree'],merge_tree_exit=1,git_outcome='CONFLICT',
        complete_conflict_paths=u['conflicts'],complete_manual_paths=names,unit_sha256=sha(raw),
        raw_difference=file_digest(git(repo,'diff','--raw','--no-abbrev','--no-renames','-z',f['automatic_tree'],f['merge'])),
        patch=file_digest(git(repo,'diff','--binary',f['automatic_tree'],f['merge'])),
        required_current_correction=f['correction'],qualified_comparison_source=f['current'],current_target=target,
        historical_gap=u['historical_gap'],current_bindings=u['current_bindings'],evidence=u['evidence'],
        retain_runtime_and_content_taint=True,runtime_pass=False,activation_authority=False,current_or_future_path_authority=False,admission=False)


def file_digest(raw):
    return dict(bytes=len(raw),sha256=sha(raw))


def prepare_corrected_F_capability(repo, unit_path):
    target=git(repo,'rev-parse','HEAD').decode().strip();whole=corrected_F_whole(repo,unit_path,target)
    require(Path(repo).resolve()==ROOT.resolve(),'corrected_F_actual_root')
    require(all((ROOT/n).read_bytes()==git(repo,'show',target+':'+n) for n in
        ('scripts/ci/f13_f54_history.py','scripts/ci/voice-db-runtime-provisioning-contract_test.sh','scripts/ci/voice-r22-runtime-scope.sh')),'corrected_F_executed_carrier')
    context={k:os.environ.get(k,'') for k in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','GITHUB_BASE_REF','VOICE_CI_PUSH_BEFORE_SHA')}
    require(context['GITHUB_SHA']==target and ((context['GITHUB_EVENT_NAME']=='pull_request'
        and context['GITHUB_REF'].startswith('refs/pull/') and context['GITHUB_BASE_REF']=='master')
        or (context['GITHUB_EVENT_NAME']=='push' and context['GITHUB_REF']=='refs/heads/master')),'corrected_F_actual_master_event_context')
    return dict(type='F54_CORRECTED_HISTORICAL_CAPABILITY_PROTOTYPE',unit_path=str(Path(unit_path).resolve()),
        unit_sha256=F_CORRECTED_UNIT_SHA,whole=whole,source_context=router_source_context(repo),context=context,admission=False)


def prepare_corrected_F_production_capability(repo,unit_path):
    module=production_module();binding=module.qualify(repo);record=module.load_record(repo)
    target=git(repo,'rev-parse','HEAD').decode().strip()
    historical=corrected_F_whole(repo,unit_path,F_CORRECTED_TUPLE['current'])
    fixed=record['corrected_F']
    require(set(fixed)=={'unit_sha256','historical_comparison','current_bindings','historical_gap'}
        and fixed['unit_sha256']==F_CORRECTED_UNIT_SHA and fixed['historical_comparison']==F_CORRECTED_TUPLE['current']
        and fixed['historical_gap']==historical['historical_gap'],'production_F_exact_predecessor')
    require(set(fixed['current_bindings'])==set(historical['current_bindings']),'production_F_complete34')
    require(all(value==tree_entry(repo,record['source'],name)==tree_entry(repo,target,name)
        and (name not in F_CORRECTED_REPAIR_PATHS or value.startswith('100644 blob '))
        for name,value in fixed['current_bindings'].items()),'production_F_current_repair_bindings')
    spec=importlib.util.spec_from_file_location('production_F_native',ROOT/'scripts/ci/f13_native_p3_authority.py')
    native=importlib.util.module_from_spec(spec);spec.loader.exec_module(native)
    event=binding['event']
    current=native.qualify(repo,binding['base'],binding['source'],binding['target'],event['GITHUB_EVENT_NAME'],
        event['GITHUB_REF'],event['GITHUB_SHA'],event['VOICE_CI_PUSH_BEFORE_SHA'],event['GITHUB_BASE_REF'])
    verifier,native_record=native.verified_record(repo,current)
    if binding['type']=='F13-PRODUCTION-SOURCE-v11':
        require(current['version']=='NATIVE-P3-SOURCE-v5' and
            verifier.production_record(repo)['native_record']==dict(version=current['version'],
                source=current['snapshot'],path=verifier.RECORD_PATH,sha256=current['record_sha256']),
            'production_F_exact_release_native_dependency')
    elif binding['type']=='F13-PRODUCTION-SOURCE-v10':
        fixture=module.load_bootstrap_fix(repo)[0]
        require(fixture['native_record']==dict(version=current['version'],source=current['snapshot'],
            record=current['record'],path=verifier.CONTEXT_RECORD_PATH,sha256=current['record_sha256'])
            and current['source_applicability']==verifier.bootstrap_bridge_binding(binding['source'],binding['target']),
            'production_F_exact_retained_bootstrap_context_applicability')
    elif binding['type']=='F13-PRODUCTION-SOURCE-v9':
        fixture=module.load_fixture_fix(repo)[0]
        require(fixture['native_record']==dict(version=current['version'],source=current['snapshot'],
            record=current['record'],path=verifier.CONTEXT_RECORD_PATH,sha256=current['record_sha256'])
            and current['source_applicability']==verifier.fixture_bridge_binding(binding['source'],binding['target']),
            'production_F_exact_retained_context_applicability')
    elif binding['type']=='F13-PRODUCTION-SOURCE-v8':
        integrated=module.load_integration(repo)[0]
        require(current['version']==verifier.CONTEXT_VERSION and current['record']==module.INTEGRATION_RECORD_COMMIT
            and integrated['native_record']==dict(version=current['version'],source=current['snapshot'],
                path=verifier.CONTEXT_RECORD_PATH,sha256=current['record_sha256'],record_relation='SAME_SOURCE_RECORD_COMMIT'),
            'production_F_exact_current_context_dependency_closure')
    elif binding['type']=='F13-PRODUCTION-SOURCE-v7':
        social,_,_,_,_,_=module.load_social_fix(repo)
        require(social['native_record']==dict(version=current['version'],commit=current['record'],
            source=current['snapshot'],path=verifier.RECORD_PATH,sha256=current['record_sha256']),
            'production_F_exact_retained_dependency_closure')
    elif binding['type']=='F13-PRODUCTION-SOURCE-v6':
        rendering,_,_,_,_=module.load_rendering_fix(repo)
        require(rendering['native_record']==dict(version=current['version'],commit=current['record'],
            source=current['snapshot'],path=verifier.RECORD_PATH,sha256=current['record_sha256']),
            'production_F_exact_retained_dependency_closure')
    elif binding['type']=='F13-PRODUCTION-SOURCE-v5':
        fixes,_,_,_=module.load_test_fixes(repo)
        require(fixes['native_record']==dict(version=current['version'],commit=current['record'],
            source=current['snapshot'],path=verifier.RECORD_PATH,sha256=current['record_sha256']),
            'production_F_exact_retained_dependency_closure')
    elif binding['type']=='F13-PRODUCTION-SOURCE-v4':
        transport,_,_=module.load_transport(repo)
        require(transport['native_record']==dict(version=current['version'],commit=current['record'],
            source=current['snapshot'],path=verifier.RECORD_PATH,sha256=current['record_sha256']),
            'production_F_exact_retained_dependency_closure')
    elif binding['type']=='F13-PRODUCTION-SOURCE-v3':
        successor,_=module.load_successor(repo)
        require(current['version']=='NATIVE-P3-SOURCE-v3' and current['record']==module.SUCCESSOR_RECORD_COMMIT and
            successor['native_record']==dict(path=verifier.RECORD_PATH,sha256=current['record_sha256'],scope='SAME_RECORD_COMMIT'),
            'production_F_exact_reviewed_dependency_closure')
    else:
        require(record['native_record']==dict(commit=current['record'],path=verifier.RECORD_PATH,
            sha256=current['record_sha256']),'production_F_exact_reviewed_dependency_closure')
    whole=dict(historical,type='F54_CORRECTED_HISTORICAL_PRODUCTION_WHOLE_v1',
        historical_validation_target=F_CORRECTED_TUPLE['current'],current_target=target,
        historical_current_bindings=historical['current_bindings'],current_bindings=fixed['current_bindings'],
        production_applicability=binding,native_dependency_binding=current)
    return dict(type='F54_CORRECTED_HISTORICAL_PRODUCTION_CAPABILITY_v1',unit_path=str(Path(unit_path).resolve()),
        unit_sha256=F_CORRECTED_UNIT_SHA,whole=whole,source_context=router_source_context(repo),context=event,admission=False)


def checked_corrected_F_capability(repo,cap):
    prepare=prepare_corrected_F_production_capability if cap.get('type')=='F54_CORRECTED_HISTORICAL_PRODUCTION_CAPABILITY_v1' else prepare_corrected_F_capability
    require(cap==prepare(repo,cap['unit_path']),'corrected_F_complete_current_capability')
    return cap


def corrected_F_composition(repo, capability_path, merge, boundary, first, source, name, result):
    return corrected_F_receipt(repo,json.loads(Path(capability_path).read_bytes()),merge,boundary,first,source,name,result)


def corrected_F_receipt(repo, cap, merge, boundary, first, source, name, result):
    f=F_CORRECTED_TUPLE
    checked_corrected_F_capability(repo,cap)
    require([merge,boundary,first,source]==[f['merge'],f['base'],f['first'],f['source']],'corrected_F_actual_call')
    require(name in git(repo,'diff','--name-only',f['base'],f['source']).decode().splitlines(),'corrected_F_actual_source_path_not_future')
    require(result==tree_entry(repo,f['merge'],name).rstrip('\0'),'corrected_F_actual_result_entry')
    return dict(cap['whole'],path=name,result_entry=raw_binding(result),capability=cap,
        capability_sha256=sha(canonical(cap)),composition='HISTORICAL_SOURCE_WITH_REQUIRED_CURRENT_CORRECTION')


F_DELETION_PATH = 'src/backend/voice/internal/store/space_lifecycle_test.go'
F_DELETION_ROW_SHA = 'fd8bbc01415e1cc4ea13f4ad3de8ea10bef6be64e7cd79adeae5693392ea7f87'
F_DELETION_ENTRY = '100644 blob c0569cce36543e032bd4e42e077d63c1df1ed72d\tsrc/backend/voice/internal/store/space_lifecycle_test.go\0'
F_DELETION_RULING_SHA = '06c8b2bc49910714a29b2976a28a41a24d88b4a2caab82ef558c613ba52f9077'



F_MANUAL_RULING_SHA = '0a24b9c23f26f633fc856534b4d361966468b59bc5e71c564022b0ba9ebf7932'
F_MANUAL_NEW_PATHS = [
    'src/backend/voice/internal/grpcsvc/voice_grpc.go',
    'src/backend/voice/internal/grpcsvc/voice_grpc_test.go',
    'src/backend/voice/internal/spacegatetest/gates.go',
    'src/backend/voice/internal/spacelifecycle/store.go',
    'src/backend/voice/internal/spacemedia/communication_timeout_test.go',
    'src/backend/voice/internal/spacemedia/consumer_test.go',
    'src/backend/voice/internal/spacemedia/coordinator_floor_test.go',
    'src/backend/voice/internal/spacemedia/coordinator_test.go',
    'src/backend/voice/internal/spacemedia/p3_gate_integration_test.go',
    'src/backend/voice/internal/store/space_gate_incarnation_test.go']


def manual_authority_expected(repo, context):
    # Every authority bit/file below is reconstructed from actual immutable
    # source/Git/record inputs. Supplied snapshots are comparison candidates.
    event=context['event'];target=git(repo,'rev-parse','HEAD').decode().strip()
    parents=git(repo,'rev-list','--parents','-n','1',target).decode().split()
    require(len(parents)==3 and all(event.get(k,'')==os.environ.get(k,'') for k in event), 'manual_F_authority_actual_master_event')
    base,source=parents[1:];temporary=context['values']['TMP_DIR']
    require(event['GITHUB_SHA']==target and context['values']['ROOT']==str(Path(repo).resolve()), 'manual_F_authority_actual_root_target')
    native_spec=importlib.util.spec_from_file_location('manual_F_native_authority',ROOT/('scripts/ci/f13_native_p3_authority.py' if (ROOT/'scripts/ci/f13_native_p3_authority.py').is_file() else 'scripts/ci/f13_native_p3_source.py'))
    native=importlib.util.module_from_spec(native_spec);native_spec.loader.exec_module(native)
    binding=native.qualify(repo,base,source,target,event['GITHUB_EVENT_NAME'],event['GITHUB_REF'],event['GITHUB_SHA'],
        event.get('VOICE_CI_PUSH_BEFORE_SHA',''),event.get('GITHUB_BASE_REF',''))
    native_raw=(json.dumps(binding,sort_keys=True)+'\n').encode()
    paths_raw=('\n'.join(binding['paths'])+'\n').encode()
    blob=lambda raw:hashlib.sha1(b'blob '+str(len(raw)).encode()+b'\0'+raw).hexdigest()
    require(git(repo,'rev-parse','--show-object-format').decode().strip()=='sha1', 'manual_F_authority_actual_object_format')
    shell=(ROOT/'scripts/ci/voice-db-runtime-provisioning-contract_test.sh').read_text()
    fixed=dict(re.findall(r"(?m)^(accepted_[a-z0-9_]+|game_checkpoint_snapshot_[a-z_]+)='([0-9a-f]{40})'$",shell))
    ancestor=lambda a,b:subprocess.run(['git','--no-replace-objects','-C',str(repo),'merge-base','--is-ancestor',a,b],capture_output=True).returncode==0
    enabled='true' if ancestor(fixed['accepted_game_checkpoint'],target) else 'false'
    manifest=subprocess.run(['git','--no-replace-objects','-C',str(repo),'cat-file','-e',base+':protos/voice/r23_contract_manifest.json'],capture_output=True).returncode==0
    values=dict(fixed,ROOT=str(Path(repo).resolve()),base_sha=base,game_checkpoint_enabled=enabled,
        native_p3_active='true',native_p3_paths=temporary+'/f13-native-p3-paths',native_p3_paths_hash=blob(paths_raw),
        native_p3_binding=temporary+'/f13-native-p3-binding.json',native_p3_binding_hash=blob(native_raw),
        native_p3_source=source,native_p3_target=target,native_p3_snapshot=binding['snapshot'],
        r23_allowlist_enabled='true' if ancestor(fixed['accepted_r23_base'],base) and not manifest else 'false')
    files={values['native_p3_paths']:raw_binding(paths_raw),values['native_p3_binding']:raw_binding(native_raw),
        temporary+'/game-checkpoint-tree':raw_binding(git(repo,'ls-tree','-r',fixed['accepted_game_checkpoint'])),
        temporary+'/be255-checkpoint-tree':raw_binding(git(repo,'ls-tree','-r',fixed['accepted_be255_checkpoint']))}
    # Reconstruct the exact original root wrapper's Game aggregate decision,
    # including its complete output, rather than replaying supplied membership.
    with tempfile.TemporaryDirectory(prefix='f54-manual-authority-') as folder:
        work=Path(folder);delta=work/'source-delta';delta.write_bytes(git(repo,'diff','--name-only','--no-renames',base,source))
        approved=work/'paths';approved.write_bytes(b'')
        variables=dict(fixed,ROOT=str(Path(repo).resolve()),TMP_DIR=str(work),game_checkpoint_enabled=enabled,
            game_checkpoint_aggregate_active='false',game_checkpoint_aggregate_base='',game_checkpoint_aggregate_head='',
            game_checkpoint_aggregate_target='',game_checkpoint_aggregate_paths='',game_checkpoint_aggregate_record='',
            game_checkpoint_aggregate_map='',game_checkpoint_aggregate_alias_map='',F54_BASE=base,F54_SOURCE=source,
            F54_TARGET=target,F54_DELTA=str(delta),F54_PATHS=str(approved))
        script='set -euo pipefail\n'+original_policy_functions(shell)+'\n'
        script+='while (( $# )); do printf -v "$1" "%s" "$2"; shift 2; done\n'
        script+='status=0; game_checkpoint_aggregate_authorized "$ROOT" "$F54_BASE" "$F54_SOURCE" "$F54_TARGET" "$F54_DELTA" "$F54_PATHS" || status=$?\nprintf "%s\\n" "$status"\n'
        executable=work/'original-game-authority.sh';executable.write_text(script)
        result=subprocess.run(['bash',str(executable)]+sum(([k,v] for k,v in variables.items()),[]),capture_output=True)
        require(result.returncode==0 and result.stdout.strip() in (b'0',b'1'), 'manual_F_original_Game_authority_derivation')
        active=result.stdout.strip()==b'0';approved_raw=approved.read_bytes()
    output=temporary+'/f13-contributions'
    values.update(game_checkpoint_aggregate_active='true' if active else 'false',
        game_checkpoint_aggregate_base=base if active else '',game_checkpoint_aggregate_head=source if active else '',
        game_checkpoint_aggregate_target=target if active else '',game_checkpoint_aggregate_record=fixed['game_checkpoint_snapshot_record'] if active else '',
        game_checkpoint_aggregate_paths=output+'.game-snapshot-paths' if active else '',game_checkpoint_aggregate_map='',
        game_checkpoint_aggregate_alias_map=output+'.game-snapshot-alias-approvals')
    if active:files[output+'.game-snapshot-paths']=raw_binding(approved_raw)
    for suffix in ('.game-snapshot-approvals','.game-snapshot-alias-approvals'):files[output+suffix]=raw_binding(b'')
    return dict(values=values,files=files)


def validate_manual_authority(context, expected):
    require(set(context['snapshots'])=={name for name,value in context['files'].items() if value is not None}, 'manual_F_complete_authority_snapshots')
    require(all(context['values'].get(name)==value for name,value in expected['values'].items()), 'manual_F_source_derived_authority_flags_membership_hashes')
    for name,value in expected['files'].items():
        require(context['snapshots'].get(name)==value and context['files'].get(name)==file_digest(value['raw'].encode()),
            'manual_F_immutable_authority_input_not_self_consistent_supplied_hash:'+name)


def manual_F_spec(repo, capability, frame_context, actual_map, gate_context):
    """Real result-parent edge; diagnostic AUTO is never a commit ancestor."""
    f=F_CORRECTED_TUPLE
    checked_corrected_F_capability(repo,capability)
    require([frame_context[k] for k in ('base','target','allow_direct_followup','partition_source_history')]
        ==[J3071_RANGE['base'],f['correction'],'true','false'], 'manual_F_exact_same_state_frame')
    require(gate_context['source_context']==capability['source_context'] and gate_context['values']['ROOT']==str(Path(repo).resolve()), 'manual_F_original_gate_source_context')
    require(all(gate_context['event'].get(k,'')==v for k,v in capability['context'].items()), 'manual_F_original_gate_event_context')
    for name,value in gate_context['snapshots'].items():
        require(value==raw_binding(value['raw']) and gate_context['files'][name].get('sha256')==value['sha256'], 'manual_F_original_gate_file_snapshots')
    source=(ROOT/'scripts/ci/voice-db-runtime-provisioning-contract_test.sh').read_text()
    fixed=dict(re.findall(r"(?m)^(accepted_[a-z0-9_]+|game_checkpoint_snapshot_[a-z_]+)='([0-9a-f]{40})'$",source))
    require(all(gate_context['values'].get(k)==v for k,v in fixed.items()), 'manual_F_gate_fixed_source_constants')
    validate_manual_authority(gate_context,manual_authority_expected(repo,gate_context))
    rows=actual_map.decode().splitlines();names=capability['whole']['complete_manual_paths'];dispositions={}
    require(names==git(repo,'diff','--name-only',f['automatic_tree'],f['merge']).decode().splitlines()
        and len(names)==33, 'manual_F_indivisible_full_residual')
    for name in names:
        prior=[raw_binding(x) for x in rows if x.split('\t')[0]==name]
        if name in F_MANUAL_NEW_PATHS:
            entries=[tree_entry(repo,ref,name) for ref in (f['base'],f['first'],f['source'],f['automatic_tree'])]
            result=tree_entry(repo,f['merge'],name)
            require(not prior and len(set(entries))==1 and result and result!=entries[0], 'manual_F_genuinely_missing_result_only_path')
            kind='NEW_RESULT_PARENT_ROW'
        elif name==F_DELETION_PATH:
            require(len(prior)==1 and sha(prior[0]['raw'].encode())==F_DELETION_ROW_SHA
                and not tree_entry(repo,f['merge'],name), 'manual_F_retains_original_deleted_D_true')
            kind='RETAINED_DELETION_PENDING_TERMINAL_GUARD'
        else:
            require(prior, 'manual_F_covered_residual_requires_actual_prior_producer')
            kind='RETAINED_PARENT_ASSOCIATION'
        dispositions[name]=dict(type=kind,prior_rows=prior,
            result=entry_binding(repo,f['merge'],name),parent=entry_binding(repo,f['first'],name))
    require(set(F_MANUAL_NEW_PATHS)<=set(names), 'manual_F_exact_ten_inside_whole33')
    return dict(type='EXACT_F_MANUAL_RESULT_v1',ruling_sha256=F_MANUAL_RULING_SHA,capability=capability,
        frame_context=frame_context,gate_context=gate_context,map_before=file_digest(actual_map),paths=names,dispositions=dispositions,
        merge=f['merge'],ordered_parents=[f['first'],f['source']],original_merge_bases=[f['base']],
        diagnostic_automatic_tree=f['automatic_tree'],result_tree=f['result_tree'],
        result_parent_edge=dict(base=f['first'],head=f['merge'],type='RESULT_PARENT_NOT_MERGE_BASE'),
        delta=raw_binding(git(repo,'diff','--name-only',f['automatic_tree'],f['merge'])),
        patch=file_digest(git(repo,'diff','--binary',f['automatic_tree'],f['merge'])),admission=False)


def prepare_manual_F(repo, capability_path, trace_path, map_path, context_path):
    raw=Path(trace_path).read_bytes()
    state=validate_trace(repo,trace_path,sha(raw),manual_checkpoint=True)['manual_checkpoint']
    actual=Path(map_path).read_bytes()
    require(state['map_before']==file_digest(actual), 'manual_F_actual_live_map')
    return manual_F_spec(repo,json.loads(Path(capability_path).read_bytes()),state['frame_context'],actual,json.loads(Path(context_path).read_bytes()))


def manual_operations(ledger):
    return [dict(frame=frame,operation=op,receipt=value) for frame,item in ledger.items()
        for op,value in item.get('manual_result_operations',{}).items()]


def manual_result_reference(operation, spec):
    # The complete independently reconstructed spec is stored once in the
    # operation receipt. Every path retains an exact shared reference, not a
    # duplicated megabyte of authority snapshots or a standalone boolean.
    return dict(operation=operation,type=spec['type'],spec_sha256=sha(canonical(spec)),
        result_parent_edge=spec['result_parent_edge'],original_merge_bases=spec['original_merge_bases'],
        complete_delta_sha256=spec['delta']['sha256'])


@lru_cache(maxsize=1)
def _collection_trace_index(raw):
    """Pure extraction only: one exact-byte snapshot, no validated authority.

    Retain serialized selected events rather than the full decoded trace. Every
    caller still checks fresh bytes, bindings and source/policy context. Returning
    immutable bytes prevents callers from poisoning later validation requests.
    """
    kinds={'manual_F_end','reviewed_F_deletion','range_I_ledger_ready','range_J_ledger_ready'}
    selected=[]
    for line in raw.splitlines():
        event=json.loads(line)
        if event['kind'] in kinds:selected.append(line)
    return b'\n'.join(selected)


def collection_trace_events(raw):
    return [json.loads(line) for line in _collection_trace_index(raw).splitlines()]


def manual_operations_in_prefix(ledger, trace, prefix):
    require(len(trace)>=prefix['bytes'] and sha(trace[:prefix['bytes']])==prefix['sha256'], 'manual_F_sealed_prefix')
    events=collection_trace_events(trace[:prefix['bytes']])
    actual=[dict(frame=e['frame'],operation=e['operation'],receipt=e['payload']['receipt'])
        for e in events if e['kind']=='manual_F_end']
    expected=manual_operations(ledger)
    require(actual==expected, 'manual_F_all_actual_operations_consumed_not_sidecar')
    return expected


def manual_delta_for_binding(binding):
    name=binding['path'];values=binding.get('manual_result_operations',[])
    matches=[v for v in values if name in v['receipt']['spec']['paths']]
    require(len(matches)==1, 'manual_F_exact_sealed_operation_delta')
    value=matches[0]['receipt'];spec=value['spec']
    require(spec['type']=='EXACT_F_MANUAL_RESULT_v1' and spec['admission'] is False
        and value['classification']['trigger'] in ('true','false')
        and set(value['dispositions'])==set(spec['paths']), 'manual_F_complete_typed_delta_context')
    require(spec['delta']==raw_binding(spec['delta']['raw']) and spec['delta']['raw'].splitlines()==spec['paths']
        and len(spec['paths'])==33 and set(F_MANUAL_NEW_PATHS)<=set(spec['paths']), 'manual_F_indivisible_qualification_delta')
    return spec['delta']['raw']



def snapshot_manual_F_context(repo, map_path, pairs):
    # This is before the later global classifier/router. Do not invent that
    # result merely to reuse its context transport.
    value=policy_context(repo,map_path,pairs,require_runtime=False)
    require(value['source_context']==router_source_context(value['values']['ROOT']), 'manual_F_snapshot_actual_source')
    value['snapshots']={name:raw_binding(Path(name).read_bytes()) for name,b in value['files'].items() if b is not None}
    return value


@lru_cache(maxsize=512)
def replay_manual_F_gate(repo, spec_raw, name):
    """Read-only replay of the same original predicates with captured inputs.

    Only their scratch files are relocated. No accepting predicate replacement,
    bare-policy admission or substituted source map is installed.
    """
    spec=json.loads(spec_raw);context=spec['gate_context'];f=F_CORRECTED_TUPLE
    source=(ROOT/'scripts/ci/voice-db-runtime-provisioning-contract_test.sh').read_text()
    functions=original_policy_functions(source)
    with tempfile.TemporaryDirectory(prefix='f54-manual-gates-') as folder:
        temporary=Path(folder);old_tmp=context['values']['TMP_DIR'];new_tmp=str(temporary/'scratch');Path(new_tmp).mkdir()
        relocated={}
        for index,(path,binding) in enumerate(context['snapshots'].items()):
            target=Path(new_tmp+path[len(old_tmp):]) if path.startswith(old_tmp+'/') else temporary/('input-'+str(index))
            target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(binding['raw'].encode());relocated[path]=str(target)
        values={k:(new_tmp if k=='TMP_DIR' else relocated.get(v,v)) for k,v in context['values'].items()}
        delta=temporary/'full33';delta.write_text(spec['delta']['raw']);entry=tree_entry(repo,f['merge'],name).rstrip('\0')
        arguments=dict(values,F54_REPO=str(repo),F54_NAME=name,F54_ENTRY=entry,F54_DELTA=str(delta),F54_F=f['merge'],F54_C=f['base'],F54_D=f['first'],F54_A=f['source'])
        script='set -euo pipefail\n'+functions+'\nwhile (( $# )); do printf -v "$1" "%s" "$2"; shift 2; done\n'
        script+='''status=0
f13_checkpoint_merge_tree_entry_matches "$F54_REPO" "$F54_C" "$F54_D" "$F54_A" "$F54_NAME" "$F54_ENTRY" "$F54_F" '' '' '' '' || status=$?
native_status=0;game_status=0
if [[ "${native_p3_active:-false}" == true ]] && native_p3_path_bound "$F54_REPO" "$F54_NAME"; then
  native_p3_pre_h_owner "$F54_REPO" "$F54_NAME" "$F54_F" || native_status=$?
fi
if [[ "${game_checkpoint_aggregate_active:-false}" == true ]] && grep -Fxq -- "$F54_NAME" "$game_checkpoint_aggregate_paths"; then
  git -C "$F54_REPO" merge-base --is-ancestor "$F54_F" "$game_checkpoint_snapshot_head" || game_status=$?
fi
eligibility=unapproved
if game_checkpoint_aggregate_path_allowed "$F54_REPO" "$F54_F" "$F54_NAME" "$F54_ENTRY"; then eligibility=game_aggregate
elif game_checkpoint_contribution_allowed "$F54_REPO" "$F54_F" "$F54_C" "$F54_A" "$F54_DELTA" && game_checkpoint_domain_path "$F54_NAME"; then eligibility=game_snapshot
elif game_checkpoint_path_authorized_in_delta "$F54_NAME" "$F54_DELTA" "$(awk '{print $3}' <<<"$F54_ENTRY")" || be255_checkpoint_path_authorized_in_delta "$F54_NAME" "$F54_DELTA" "$(awk '{print $3}' <<<"$F54_ENTRY")"; then eligibility=checkpoint_blob
fi
printf '%s\t%s\t%s\t%s\n' "$status" "$native_status" "$game_status" "$eligibility"
'''
        executable=temporary/'original-gates.sh';executable.write_text(script)
        p=subprocess.run(['bash',str(executable)]+sum(([k,v] for k,v in arguments.items()),[]),capture_output=True)
        require(p.returncode==0, 'manual_F_original_gate_replay_execution:'+p.stderr.decode(errors='replace')[-200:])
        fields=p.stdout.decode().strip().split('\t');require(len(fields)==4,'manual_F_original_gate_replay_shape')
        return dict(original_composition_status=int(fields[0]),native_owner_status=int(fields[1]),game_owner_status=int(fields[2]),eligibility=fields[3])


def prepare_manual_F_end(repo, trace_path, operation):
    raw=Path(trace_path).read_bytes()
    # The end event is itself checked by validate_trace; this constructor reads
    # actual recorded values and never assigns a flag or fabricates a row.
    events=[json.loads(x) for x in raw.splitlines()]
    begin=next(e for e in events if e['kind']=='manual_F_begin' and e['operation']==operation)
    spec=begin['payload']['spec'];gates={e['payload']['path']:e['payload'] for e in events if e['kind']=='manual_F_gate' and e['operation']==operation}
    classification=next(e['payload'] for e in events if e['kind']=='classification' and e['operation']==operation)
    # Validate the live prefix with an explicit manual-end checkpoint, including
    # all prior F operations, maps and every mutation, before sealing anything.
    state=validate_trace(repo,trace_path,sha(raw),manual_end_checkpoint=operation)['manual_end_checkpoint']
    return state


def reviewed_F_deletion_value(repo, cap, state, ruling_path):
    """A terminal-state obligation, never a replacement contribution or flag."""
    f=F_CORRECTED_TUPLE;name=F_DELETION_PATH
    require(sha(Path(ruling_path).read_bytes())==F_DELETION_RULING_SHA,'reviewed_F_deletion_exact_ruling')
    checked_corrected_F_capability(repo,cap)
    frame=state['frame_context'];operation=state['F_operation'];binding=state['operation_binding']
    require(state['source_context']==cap['source_context'],'reviewed_F_deletion_actual_carrier_context')
    guard=state['original_guard']
    require(guard['gate']=='f13_guarded_entry_matches_final_target' and guard['original_status']==1
        and guard['path']==name and guard['target']==frame['target']
        and guard['stored_entry']==raw_binding(F_DELETION_ENTRY.rstrip('\0'))
        and guard['target_entry']==raw_binding('') and guard['guarded_owner']==f['first']
        and guard['source_head']==f['first'] and guard['prior_rows']==state['prior_rows']
        and guard['map_before']==state['map_binding'],'reviewed_F_deletion_actual_original_guard_arguments')
    require(frame['target']==f['correction'] and frame['base']==J3071_RANGE['base']
        and frame['allow_direct_followup']=='true' and frame['partition_source_history']=='false',
        'reviewed_F_deletion_exact_actual_range_frame')
    require(re.fullmatch('[0-9a-f]{64}',state['range_branch_sha256']) is not None,
        'reviewed_F_deletion_consumes_actual_range_origin')
    require(operation==frame['id']+'/'+f['merge']+'/2' and binding['commit']==f['merge']
        and [binding[k] for k in ('first_parent','boundary','source_head')]==[f['first'],f['base'],f['source']],
        'reviewed_F_deletion_actual_completed_F_operation')
    require(state['classification']['trigger']=='true' and state['composition_receipts']
        and all(x['capability']==cap for x in state['composition_receipts']),
        'reviewed_F_deletion_actual_whole_consumption_and_classifier')
    require(name in cap['whole']['complete_manual_paths'] and cap['whole']['current_bindings'][name]=='',
        'reviewed_F_deletion_reviewed_complete_unit_absence')
    require(tree_entry(repo,f['first'],name)==F_DELETION_ENTRY
        and tree_entry(repo,f['automatic_tree'],name)==F_DELETION_ENTRY,
        'reviewed_F_deletion_exact_prior_mode_type_blob')
    for ref in (f['base'],f['source'],f['merge'],f['correction'],f['current'],frame['target']):
        # tree_entry uses checked Git output; a missing object is not ABSENT.
        require(tree_entry(repo,ref,name)=='','reviewed_F_deletion_checked_absent_entry')
    require(git(repo,'diff','--name-status','--no-renames',f['first'],f['merge'],'--',name).decode()=='D\t'+name+'\n',
        'reviewed_F_deletion_real_D_to_F_transition')
    require(not git(repo,'rev-list','--full-history',f['merge']+'..'+frame['target'],'--',name).strip(),
        'reviewed_F_deletion_no_unaccounted_later_writer')
    rows=state['prior_rows'];require(len(rows)==1 and rows[0]==raw_binding(rows[0]['raw'])
        and rows[0]['sha256']==F_DELETION_ROW_SHA,'reviewed_F_deletion_exact_retained_raw_D_row')
    parts=row(rows[0]);require(parts==[name,'true',f['first'],f['base'],f['base'],f['first'],f['base'],f['base']],
        'reviewed_F_deletion_real_owner_source_content_true')
    require(state['aggregate'] is True and state['path_evidence'],'reviewed_F_deletion_complete_true_history')
    return dict(type='F54_REVIEWED_F_DELETION_v1',path=name,expected_terminal_state='ABSENT',
        ruling_path=str(ruling_path),ruling_sha256=F_DELETION_RULING_SHA,capability=cap,
        state=state,prior_entry=raw_binding(F_DELETION_ENTRY),terminal_entry=raw_binding(''),
        map_before=state['map_binding'],map_after=state['map_binding'],aggregate=True,
        historical_rows_unchanged=True,admission=False,runtime_pass=False)


def prepare_reviewed_F_deletion(repo, capability_path, trace_path, map_path, ruling_path):
    cap=json.loads(Path(capability_path).read_bytes());raw=Path(trace_path).read_bytes()
    events=[json.loads(v) for v in raw.splitlines()]
    require(events and events[-1]['kind']=='reviewed_F_deletion_guard'
        and events[-1]['payload']['path']==F_DELETION_PATH,'reviewed_F_deletion_actual_terminal_guard')
    state=validate_trace(repo,trace_path,sha(raw),deletion_checkpoint=events[-1]['frame'])['deletion_checkpoint']
    actual=Path(map_path).read_bytes()
    require(state['map_binding']==file_digest(actual),'reviewed_F_deletion_unchanged_actual_map')
    return reviewed_F_deletion_value(repo,cap,state,ruling_path)


def terminal_dispositions(ledger):
    """Conservation consumers carry the typed state with its raw true history."""
    result=[]
    for frame,value in ledger.items():
        for name,receipt in value.get('terminal_dispositions',{}).items():
            require(name==F_DELETION_PATH and receipt['type']=='F54_REVIEWED_F_DELETION_v1'
                and receipt['expected_terminal_state']=='ABSENT' and receipt['aggregate'] is True,
                'reviewed_F_deletion_typed_terminal_state_not_false_flag')
            item=value['paths'].get(name)
            require(item is not None and item['aggregate'] is True and item['evidence'],
                'reviewed_F_deletion_cannot_replace_retained_history')
            actual=[raw_binding(x) for x in value['rows'] if x.split('\t')[0]==name]
            require(len(actual)==1 and actual==receipt['state']['prior_rows'] and actual[0]['sha256']==F_DELETION_ROW_SHA,
                'reviewed_F_deletion_no_row_drop_rewrite_or_synthetic_owner')
            require(receipt['map_before']==receipt['map_after']
                and receipt['state']['frame_context']['id']==frame,'reviewed_F_deletion_immutable_map_and_origin')
            result.append(dict(frame=frame,path=name,receipt=receipt,receipt_sha256=sha(canonical(receipt))))
    return result


def terminal_dispositions_in_prefix(ledger, trace, prefix):
    require(len(trace)>=prefix['bytes'] and sha(trace[:prefix['bytes']])==prefix['sha256'],
        'reviewed_F_deletion_consumed_with_exact_complete_prefix')
    events=collection_trace_events(trace[:prefix['bytes']])
    actual=[]
    for event in events:
        if event['kind']=='reviewed_F_deletion':
            value=event['payload'];receipt=value['receipt']
            require(value['receipt_sha256']==sha(canonical(receipt)) and event['operation'] is None
                and value['map_after']==receipt['map_before']==receipt['map_after'],
                'reviewed_F_deletion_prefix_receipt_binding')
            actual.append(dict(frame=event['frame'],path=receipt['path'],receipt=receipt,receipt_sha256=value['receipt_sha256']))
    expected=terminal_dispositions(ledger)
    key=lambda value:(value['frame'],value['path'])
    require(sorted(actual,key=key)==sorted(expected,key=key),
        'reviewed_F_deletion_no_omitted_substituted_or_unconsumed_disposition')
    return expected


def retained_row_for_binding(binding):
    dispositions=binding.get('terminal_dispositions',[])
    if not dispositions:return None
    require(binding['path']==F_DELETION_PATH and binding['covered'] is True and binding['aggregate'] is True
        and binding['obligations']['aggregate'] is True and binding['obligations']['retained_evidence'],
        'reviewed_F_deletion_original_router_keeps_full_true_history')
    rows=[]
    for disposition in dispositions:
        receipt=disposition['receipt'];prior=receipt['state']['prior_rows']
        require(disposition['path']==F_DELETION_PATH and disposition['receipt_sha256']==sha(canonical(receipt))
            and receipt['type']=='F54_REVIEWED_F_DELETION_v1' and receipt['expected_terminal_state']=='ABSENT'
            and len(prior)==1 and prior[0]==raw_binding(prior[0]['raw']) and prior[0]['sha256']==F_DELETION_ROW_SHA,
            'reviewed_F_deletion_original_D_source_content_projection')
        rows.append(prior[0]['raw'])
    require(len(set(rows))==1,'reviewed_F_deletion_one_genuine_historical_provenance')
    return rows[0]


def load_unit(repo, directory):
    """Observe the fixed reviewed factual packet, not an approved source record.

    Directory selects transport location only; no caller-selected hash, tuple,
    source/context or flag is accepted. Existing 177 source reviews are reused
    through exact byte bindings; no new semantic/runtime acceptance is claimed.
    """
    folder=Path(directory);loaded={}
    for name,digest in UNIT_HASHES.items():
        path=folder/name;require(path.is_file(),'fixed_unit_file')
        data=path.read_bytes();require(sha(data)==digest,'fixed_unit_raw_hash')
        if name.endswith('.json'):loaded[name]=json.loads(data)
    contract=loaded['contract-PROPOSAL.json'];packet=loaded['proposal.json']
    exact_boundary(repo,contract['merge'],*contract['ordered_parents'])
    require(contract['complete_best_bases']==BASES and packet['older_common_base']==O,'unit_complete_bases')
    require(packet['current_consumer']==SOURCE and len(contract['artifacts'])==177,'fixed_source_context_and_artifacts')
    require(len(packet['revisions'])==9 and len(packet['entries'])==41,'complete_nine_version_unit')
    require(git(repo,'diff','--name-only',*BASES).decode().splitlines()==packet['all_base_diff_paths'],'complete_41_base_paths')
    require([e['path'] for e in packet['entries']]==packet['all_base_diff_paths'],'complete_ordered_entry_unit')
    for entry in packet['entries']:
        require(set(entry['versions'])==set(packet['revisions']),'all_nine_versions')
        for label,revision in packet['revisions'].items():
            actual=tree_entry(repo,revision,entry['path'])
            expected=entry['versions'][label]
            if expected is None:require(actual=='','actual_absence')
            else:
                require(actual==expected['mode']+' '+expected['type']+' '+expected['oid']+'\t'+entry['path']+'\0','actual_versioned_entry')
    names=set()
    for artifact in contract['artifacts']:
        name=artifact['name'];require(name not in names and Path(name).name==name,'unique_literal_artifact')
        names.add(name);path=folder/name;require(path.is_file(),'complete_177_artifacts')
        data=path.read_bytes();require(len(data)==artifact['bytes'] and sha(data)==artifact['sha256'],'exact_artifact_bytes')
    require(git(repo,'rev-parse',M+'^{tree}').decode().strip()==TREE,'unit_committed_whole_tree')
    carrier=git(repo,'rev-parse','HEAD').decode().strip()
    changed=git(repo,'diff','--name-only',SOURCE,carrier).decode().splitlines()
    return dict(type='F54_REVIEWED_SOURCE_FACTS_OBSERVATION',source_context=SOURCE,observer_carrier=carrier,
        changed_source_context_paths=[v for v in changed if v in packet['all_base_diff_paths']],paths=41,versions=9,
        artifacts=177,unit_raw_hash=UNIT_HASHES['contract-PROPOSAL.json'],ruling_raw_hash=UNIT_HASHES['rudeus410-scalar-adjudication.md'],admission=False)


def guarded_names(names):
    """Use the actual collector predicate, not a second path policy."""
    shell=(ROOT/'scripts/ci/voice-db-runtime-provisioning-contract_test.sh').read_text()
    match=re.search(r'(?ms)^f13_path_is_guarded\(\) \{\n.*?^\}\n',shell)
    require(match is not None,'actual_guarded_predicate')
    program=match.group(0)+'\nfor path; do if f13_path_is_guarded "$path"; then printf "%s\\n" "$path"; fi; done\n'
    p=subprocess.run(['bash','-c',program,'f54-actual-guarded',*names],capture_output=True)
    require(p.returncode==0,'guarded_predicate_execution');return p.stdout.decode().splitlines()


def current_author_applicability(repo):
    """Fixed current input, not a production record or new path permission."""
    require(Path(repo).resolve()==ROOT.resolve(),'current_author_actual_ROOT')
    reference=git(repo,'rev-parse',AUTHOR_REFERENCE+'^{commit}').decode().strip()
    require(reference==AUTHOR_REFERENCE and git(repo,'rev-parse',reference+'^{tree}').decode().strip()==AUTHOR_REFERENCE_TREE,
        'current_author_fixed_reference_tree')
    reference_raw=git(repo,'diff','--raw','--no-abbrev','--no-renames',PROTOTYPE_BASE,reference)
    require(sha(reference_raw)==AUTHOR_REFERENCE_DELTA_SHA,'current_author_complete_reference_delta')
    target=git(repo,'rev-parse','HEAD').decode().strip()
    git(repo,'merge-base','--is-ancestor',PROTOTYPE_BASE,reference)
    git(repo,'merge-base','--is-ancestor',reference,target)
    own_paths=git(repo,'diff','--name-only',reference,target).decode().splitlines()
    require(set(own_paths)<=set(AUTHOR_SOURCE_PATHS),'current_author_only_own_four_file_delta')
    names=git(repo,'diff','--name-only',PROTOTYPE_BASE,reference).decode().splitlines()
    entries=[dict(path=name,prototype_entry=tree_entry(repo,PROTOTYPE_BASE,name),current_entry=tree_entry(repo,reference,name),
        disposition='AUTHOR_CARRIER_ONLY' if name in AUTHOR_SOURCE_PATHS else 'FIXED_CURRENT_INPUT; ORIGINAL_GATES_REQUIRED') for name in names]
    source_files={}
    for name in AUTHOR_SOURCE_PATHS:
        entry=tree_entry(repo,target,name);path=ROOT/name
        raw=git(repo,'show',target+':'+name) if entry else None
        require((path.read_bytes() if path.is_file() else None)==raw,'current_author_source_bytes_match_Git')
        source_files[name]=dict(entry=entry,sha256=sha(raw) if raw is not None else None)
    return dict(type='F54_FIXED_CURRENT_AUTHOR_APPLICABILITY',reference=reference,reference_tree=AUTHOR_REFERENCE_TREE,
        reference_delta_raw=raw_binding(reference_raw),reference_delta=entries,current_carrier=target,
        current_tree=git(repo,'rev-parse',target+'^{tree}').decode().strip(),source_files=source_files,
        own_delta_paths=own_paths,own_delta=raw_binding(git(repo,'diff','--raw','--no-abbrev','--no-renames',reference,target)),
        historical_collector=dict(AUTHOR_HISTORICAL_SCOPE),native_content_baseline=AUTHOR_NATIVE_CONTENT_BASE,
        event_base=AUTHOR_EVENT_BASE,admission=False,runtime_pass=False)


def prepare_capabilities(repo, directory, author_applicability=None, production=None):
    """Two independently checked source capabilities for the finite prototype.

    Constants select the question. Complete immutable source facts, actual Git
    parents/bases/entries/DAG and whole-parent merge computation answer it.
    This entrypoint grants no production R/K, native or runtime authority.
    """
    require(author_applicability is None or production is None,'production_no_mixed_M_authority')
    facts=load_unit(repo,directory);folder=Path(directory)
    packet=json.loads((folder/'proposal.json').read_bytes())
    contract=json.loads((folder/'contract-PROPOSAL.json').read_bytes())
    producer_sources=['scripts/ci/'+name for name in ('f13_f54_history.py','voice-db-runtime-provisioning-contract_test.sh','voice-r22-runtime-scope.sh')]
    # The nine historical versions remain exact. A new verifier implementation
    # is a separate current carrier, not a replacement historical source fact.
    if production is not None:
        require(production==production_applicability(repo),'production_M_independent_complete_applicability')
        require(dict(base=packet['traversal_base'],target=packet['recursive_target'],allow_direct_followup='true',partition_source_history='false')==AUTHOR_HISTORICAL_SCOPE,
            'production_M_exact_historical_collector_scope')
    elif author_applicability is None:
        require(all(name in producer_sources for name in facts['changed_source_context_paths']),'unchanged_historical_candidate_product_context')
    else:
        require(author_applicability==current_author_applicability(repo),'current_author_independent_complete_applicability')
        require(dict(base=packet['traversal_base'],target=packet['recursive_target'],allow_direct_followup='true',partition_source_history='false')==AUTHOR_HISTORICAL_SCOPE,
            'current_author_exact_historical_collector_scope')
    names=packet['all_base_diff_paths'];guarded=guarded_names(names)
    require(len(guarded)==15 and guarded==packet['guarded_base_difference_paths']==contract['guarded_difference_paths'],'complete_actual_15_guarded_bindings')
    require(names==contract['complete_candidate_difference_paths'],'complete_contract_41_bindings')
    actual=exact_boundary(repo,M,P1,P2)
    whole=git(repo,'merge-tree','--write-tree','--name-only','-z','--no-messages',P1,P2)
    require(whole==TREE.encode()+b'\0' and git(repo,'rev-parse',M+'^{tree}').decode().strip()==TREE,'exact_actual_whole_parent_tree')
    nodes=git(repo,'rev-list','--topo-order','--reverse',M,'^'+O).decode().splitlines()
    require(len(nodes)==33,'complete_actual_33_source_dag')
    dag=[]
    for node in nodes:
        parents=git(repo,'rev-list','--parents','-n','1',node).decode().split()[1:]
        dag.append(dict(node=node,ordered_parents=parents,parent_deltas=[dict(parent=p,delta=raw_binding(git(repo,'diff','--name-only',p,node))) for p in parents]))
    target=git(repo,'rev-parse','HEAD').decode().strip()
    allowed={'scripts/ci/f13_f54_history.py','scripts/ci/f13_f54_history_test.py','scripts/ci/voice-db-runtime-provisioning-contract_test.sh','.agent/execplans/native-p3-master-source.md'}
    if author_applicability is None and production is None:
        require(set(git(repo,'diff','--name-only',PROTOTYPE_BASE,target).decode().splitlines())<=allowed,'exact_prototype_source_context')
    for path in producer_sources:
        require((ROOT/path).read_bytes()==git(repo,'show',target+':'+path),'actual_capability_source_carrier')
    return dict(type='F54_FIXED_PRODUCTION_CAPABILITIES_v1' if production is not None else
        ('F54_FIXED_CURRENT_AUTHOR_CAPABILITIES' if author_applicability is not None else 'F54_FIXED_COMPLETE_SOURCE_CAPABILITIES_PROTOTYPE'),phase='SOURCE_PROVENANCE_ONLY',
        **(dict(author_applicability=author_applicability) if author_applicability is not None else {}),
        **(dict(production_applicability=production) if production is not None else {}),
        unit_directory=str(folder.resolve()),source_facts=facts,prepared_at_target=target,
        producer_binding=classifier_binding(),observer_sha256=sha((ROOT/'scripts/ci/f13_f54_history.py').read_bytes()),
        history_boundary=dict(actual,type='F54_EXACT_HISTORY_BOUNDARY_CAPABILITY'),
        whole_parent_composition=dict(type='F54_EXACT_WHOLE_PARENT_COMPOSITION_CAPABILITY',merge=M,parents=[P1,P2],best_bases=BASES,
            history_boundary=O,result_tree=TREE,actual_merge_tree=raw_binding(whole),admission=False),
        complete_41_entries=packet['entries'],guarded_difference_paths=guarded,complete_dag=dag,
        carrier_context_transition=[dict(path=name,historical_source=SOURCE,current_carrier=target,
            historical_entry=entry_binding(repo,SOURCE,name),current_entry=entry_binding(repo,target,name),
            disposition='CURRENT_VERIFIER_CARRIER_ONLY; historical tuple/entries unchanged' if name in producer_sources else 'FIXED_CURRENT_INPUT_ONLY; ORIGINAL_GATES_REQUIRED') for name in facts['changed_source_context_paths']],
        collector_scope=dict(base=packet['traversal_base'],target=packet['recursive_target'],allow_direct_followup='true',partition_source_history='false'),
        actual_source_delta=raw_binding(git(repo,'diff','--name-only',O,P2)),admission=False)


def prepare_author_capabilities(repo, directory):
    return prepare_capabilities(repo,directory,current_author_applicability(repo))


def prepare_production_capabilities(repo,directory):
    return prepare_capabilities(repo,directory,production=production_applicability(repo))


def checked_capability_value(repo, value):
    choices={'F54_FIXED_CURRENT_AUTHOR_CAPABILITIES':prepare_author_capabilities,
        'F54_FIXED_COMPLETE_SOURCE_CAPABILITIES_PROTOTYPE':prepare_capabilities,
        'F54_FIXED_PRODUCTION_CAPABILITIES_v1':prepare_production_capabilities}
    require(value['type'] in choices,'distinct_fixed_capability_type')
    prepare=choices[value['type']]
    require(value==prepare(repo,value['unit_directory']),'complete_exact_capability_binding')
    return value


def verified_capabilities(repo, path):
    return checked_capability_value(repo,json.loads(Path(path).read_bytes()))


def current_author_frame_scope(value, collector_base, collector_target):
    if value['type'] in ('F54_FIXED_CURRENT_AUTHOR_CAPABILITIES','F54_FIXED_PRODUCTION_CAPABILITIES_v1'):
        require([collector_base,collector_target]==[value['collector_scope']['base'],value['collector_scope']['target']],
            'current_author_only_bound_historical_frame')


def boundary_capability(repo, path, merge, first, source, collector_base='', collector_target=''):
    require([merge,first,source]==[M,P1,P2],'fixed_historical_tuple')
    value=verified_capabilities(repo,path);current_author_frame_scope(value,collector_base,collector_target)
    return value['history_boundary']


def entry_capability(repo, path, base, target):
    value=verified_capabilities(repo,path)
    require(value['collector_scope']==dict(base=base,target=target,allow_direct_followup='true',partition_source_history='false'),'exact_original_collector_context')
    return dict(type='F54_EXACT_COLLECTOR_PROTOTYPE_CONTEXT',capability_sha256=sha(canonical(value)),admission=False)


def composition_capability(repo, path, merge, boundary, first, source, name, result, collector_base='', collector_target=''):
    require([merge,boundary,first,source]==[M,O,P1,P2],'fixed_historical_composition_tuple')
    value=verified_capabilities(repo,path)
    current_author_frame_scope(value,collector_base,collector_target)
    require(name in value['actual_source_delta']['raw'].splitlines(),'exact_source_path_not_future_authority')
    actual=composition(repo,boundary,first,source,name,result)
    return dict(actual,type=value['whole_parent_composition']['type'],capability_sha256=sha(canonical(value)))


def master_context(repo, capability, binding_raw, context):
    """Bind the original B/S/T entry; M capability remains nested H53-only."""
    spec=importlib.util.spec_from_file_location('f54_master_native',ROOT/('scripts/ci/f13_native_p3_authority.py' if (ROOT/'scripts/ci/f13_native_p3_authority.py').is_file() else 'scripts/ci/f13_native_p3_source.py'))
    native=importlib.util.module_from_spec(spec);spec.loader.exec_module(native)
    binding=json.loads(binding_raw)
    try:
        actual=native.qualify(repo,binding['base'],binding['source'],binding['target'],context['GITHUB_EVENT_NAME'],
            context['GITHUB_REF'],context['GITHUB_SHA'],context['VOICE_CI_PUSH_BEFORE_SHA'],context['GITHUB_BASE_REF'])
    except native.Rejected as error:raise Rejected('master_native_'+str(error)) from error
    require(actual==binding and binding['target']==git(repo,'rev-parse','HEAD').decode().strip(),'actual_master_native_B_S_T_context')
    author_roles=None
    production_roles=None
    if capability.get('type')=='F54_FIXED_CURRENT_AUTHOR_CAPABILITIES':
        require(capability['author_applicability']==current_author_applicability(repo)
            and capability['prepared_at_target']==binding['target'],'current_author_same_exact_carrier')
        require(capability['collector_scope']==AUTHOR_HISTORICAL_SCOPE,'current_author_original_historical_scope')
        require(binding['base']==AUTHOR_EVENT_BASE and binding['content_base']==AUTHOR_NATIVE_CONTENT_BASE,
            'current_author_separate_event_and_native_bases')
        git(repo,'merge-base','--is-ancestor',AUTHOR_REFERENCE,binding['source'])
        git(repo,'merge-base','--is-ancestor',AUTHOR_HISTORICAL_SCOPE['base'],binding['base'])
        git(repo,'merge-base','--is-ancestor',AUTHOR_NATIVE_CONTENT_BASE,AUTHOR_HISTORICAL_SCOPE['target'])
        author_roles=dict(historical_collector=dict(AUTHOR_HISTORICAL_SCOPE),native_content_baseline=AUTHOR_NATIVE_CONTENT_BASE,
            actual_event_base=binding['base'],reference=AUTHOR_REFERENCE)
    elif capability.get('type')=='F54_FIXED_PRODUCTION_CAPABILITIES_v1':
        current=production_applicability(repo)
        require(capability['production_applicability']==current and capability['prepared_at_target']==binding['target'],
                'production_same_exact_current_carrier')
        require(capability['collector_scope']==AUTHOR_HISTORICAL_SCOPE
                and binding['base']==current['base'] and binding['source']==current['source']
                and binding['content_base']==current['native_content_baseline'],'production_separate_event_native_history')
        git(repo,'merge-base','--is-ancestor',AUTHOR_HISTORICAL_SCOPE['base'],binding['base'])
        git(repo,'merge-base','--is-ancestor',AUTHOR_NATIVE_CONTENT_BASE,AUTHOR_HISTORICAL_SCOPE['target'])
        production_roles=dict(historical_collector=dict(AUTHOR_HISTORICAL_SCOPE),native_content_baseline=AUTHOR_NATIVE_CONTENT_BASE,
            actual_event_base=binding['base'],record=current['record'],reviewed_source=current['reviewed_source'])
    else:
        require(binding['base']==capability['collector_scope']['base'],'master_actual_original_base')
    git(repo,'merge-base','--is-ancestor',capability['collector_scope']['target'],binding['source'])
    parents=git(repo,'rev-list','--parents','-n','1',binding['target']).decode().split()
    require(parents==[binding['target'],binding['base'],binding['source']],'actual_master_two_parent_shape')
    source_delta=git(repo,'diff','--name-only',binding['base'],binding['source'])
    require(source_delta==git(repo,'diff','--name-only',binding['base'],binding['target']),'actual_master_source_result_delta_parity')
    entries={name:entry_binding(repo,binding['source'],name) for name in source_delta.decode().splitlines()}
    require(all(entry_binding(repo,binding['target'],name)==value for name,value in entries.items()),'actual_master_source_result_entry_parity')
    history_base=None
    if production_roles is not None and current['type']=='F13-PRODUCTION-SOURCE-v11':
        require(binding['version']=='NATIVE-P3-SOURCE-v5','production_history_exact_native_version')
        verifier,_=native.verified_record(repo,binding)
        history_base=verifier.collection_base(repo,binding)
        require(history_base==current['collection_base'] and history_base==binding['collection_base'],
            'production_history_qualified_collection_base')
    return dict(type=('F54_EXPLICIT_MASTER_PRODUCTION_HISTORY_ENTRY_v1' if history_base is not None else 'F54_EXPLICIT_MASTER_PRODUCTION_ENTRY_v1') if production_roles is not None else
        ('F54_EXPLICIT_MASTER_CURRENT_AUTHOR_ENTRY' if author_roles is not None else 'F54_EXPLICIT_MASTER_PROTOTYPE_ENTRY'),
        **(dict(collection_base=history_base) if history_base is not None else {}),
        **(dict(author_base_roles=author_roles) if author_roles is not None else {}),native_binding=raw_binding(binding_raw),context=context,
        **(dict(production_base_roles=production_roles) if production_roles is not None else {}),
        capability=raw_binding(canonical(capability)),base=binding['base'],source=binding['source'],target=binding['target'],
        source_delta=raw_binding(source_delta),entries=entries,source_context=router_source_context(repo),admission=False)


def prepare_master_entry(repo, capability_path, binding_path):
    context={key:os.environ.get(key,'') for key in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','VOICE_CI_PUSH_BEFORE_SHA','GITHUB_BASE_REF')}
    return master_context(repo,verified_capabilities(repo,capability_path),Path(binding_path).read_bytes(),context)


def master_wrapper_result(entry_path, source_map, output):
    entry=json.loads(Path(entry_path).read_bytes());raw=Path(source_map).read_bytes()
    require(raw==Path(output).read_bytes(),'actual_original_wrapper_map_copy_parity')
    return dict(type='F54_ACTUAL_MASTER_WRAPPER_RESULT',entry_sha256=sha(canonical(entry)),source_map=file_binding(source_map),
        output=file_binding(output),source_map_path=str(source_map),output_path=str(output),admission=False)


def dag_dispositions(repo, events):
    """Every raw node gets a factual disposition, never an invented owner row.

    Absence of a real producer operation stays UNREACHED. Merely appearing in
    an ancestor range or equal endpoint is not sufficient to label it covered.
    Shared visits reference the actual same immutable commit/parent operation.
    """
    nodes=git(repo,'rev-list','--topo-order','--reverse',M,'^'+O).decode().splitlines()
    require(len(nodes)==33,'fixed_complete_33_dag')
    operations=[e for e in events if e.get('kind')=='operation_begin']
    attempts=[e for e in events if e.get('kind')=='operation_attempt']
    result=[]
    for node in nodes:
        parents=git(repo,'rev-list','--parents','-n','1',node).decode().split()[1:]
        observed=[e for e in operations if e['payload']['commit']==node]
        expected=list(range(2,len(parents)+1)) if len(parents)>1 else [1]
        per_parent=[]
        for index in expected:
            refs=[e['payload']['id'] for e in observed if e['payload']['parent_index']==index]
            kind='shared_real_operation' if len(refs)>1 else 'visited_real_operation' if refs else 'UNREACHED'
            if not refs and len(parents)==1:
                refs=[e['payload']['id'] for e in operations if e['payload']['source_head']==node and e['payload']['boundary']==parents[0]]
                if refs:kind='exact_single_commit_source_of_real_operation'
            if not refs:
                refs=[e['payload']['id'] for e in attempts if e['payload']['commit']==node and e['payload']['parent_index']==index]
                if refs:kind='attempted_boundary_no_classification'
            per_parent.append(dict(parent_index=index,disposition=kind,references=refs))
        result.append(dict(node=node,ordered_parents=parents,ordered_parent_deltas=[dict(parent=p,
            delta=raw_binding(git(repo,'diff','--name-only',p,node))) for p in parents],operations=per_parent))
    return result


def span_origin(repo, original_path, original_native_path, current_native_path, author_capability_path=None):
    """Fixed old observation is provenance, never an admission record.

    Only this exact missing two-node span is permitted. Legacy callers retain
    identical H4 pins. An explicit current AUTHOR certificate independently
    qualifies distinct current authority without changing historical evidence.
    """
    raw=Path(original_path).read_bytes();require(sha(raw)==ORIGINAL_TRACE_SHA,'fixed_original_trace_hash')
    events=[json.loads(v) for v in raw.splitlines()];previous='0'*64
    for sequence,e in enumerate(events,1):
        value=dict(e);digest=value.pop('digest')
        require(e['sequence']==sequence and e['previous']==previous and sha(canonical(value))==digest,'original_append_only_chain');previous=digest
    reference='F1/'+SPAN_AGGREGATE+'/2'
    operation=next(e for e in events if e['kind']=='operation_begin' and e['operation']==reference)
    frame=next(e for e in events if e['kind']=='frame_enter' and e['payload']['id']==operation['frame'])
    classified=next(e for e in events if e['kind']=='classification' and e['operation']==reference)
    value=operation['payload'];parents=git(repo,'rev-list','--parents','-n','1',SPAN_AGGREGATE).decode().split()[1:]
    require(len(parents)==2 and value['ordered_parents']==parents and value['first_parent']==parents[0] and value['parent_index']==2 and parents[1]==SPAN_HEAD,'fixed_aggregate_ordered_parents')
    require(value['boundary']==SPAN_BASE and value['source_head']==SPAN_HEAD,'fixed_aggregate_source_span')
    require(git(repo,'merge-base','--all',parents[0],SPAN_HEAD).decode().splitlines()==[SPAN_BASE],'actual_aggregate_boundary')
    require(git(repo,'rev-list','--first-parent','--reverse',SPAN_BASE+'..'+SPAN_HEAD).decode().splitlines()==SPAN_NODES,'exact_two_internal_nodes')
    require(frame['payload']['partition_source_history']=='false','original_aggregate_not_a_child')
    require(classified['payload']['classifier']['producer_sha256']==ORIGINAL_PRODUCER_SHA and classified['payload']['classifier']['sha256']==classifier_binding()['sha256'],'fixed_original_classifier')
    require(classified['payload']['trigger']==classify(classified['payload']['unapproved_delta']['raw'].splitlines()),'observed_aggregate_raw_classification')
    require(value['delta']==raw_binding(git(repo,'diff','--name-only',SPAN_BASE,SPAN_HEAD)),'original_aggregate_delta')
    old_raw=Path(original_native_path).read_bytes();require(sha(old_raw)==ORIGINAL_NATIVE_SHA,'fixed_original_native_context')
    old=json.loads(old_raw);current_raw=Path(current_native_path).read_bytes();current=json.loads(current_raw)
    author_fields={}
    if author_capability_path is None:
        require({k:v for k,v in old.items() if k not in ('source','target')}=={k:v for k,v in current.items() if k not in ('source','target')},'unchanged_inherited_native_pins')
    else:
        capability_raw=Path(author_capability_path).read_bytes()
        capability=verified_capabilities(repo,author_capability_path)
        is_production=capability['type']=='F54_FIXED_PRODUCTION_CAPABILITIES_v1'
        if not is_production:
            require(capability['type']=='F54_FIXED_CURRENT_AUTHOR_CAPABILITIES','current_author_span_explicit_capability_type')
        context={key:os.environ.get(key,'') for key in ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','VOICE_CI_PUSH_BEFORE_SHA','GITHUB_BASE_REF')}
        entry=master_context(repo,capability,current_raw,context)
        require(entry['type'] in (('F54_EXPLICIT_MASTER_PRODUCTION_ENTRY_v1','F54_EXPLICIT_MASTER_PRODUCTION_HISTORY_ENTRY_v1') if is_production else
            ('F54_EXPLICIT_MASTER_CURRENT_AUTHOR_ENTRY',)),'current_author_span_actual_master_entry')
        spec=importlib.util.spec_from_file_location('current_author_span_native',ROOT/('scripts/ci/f13_native_p3_authority.py' if (ROOT/'scripts/ci/f13_native_p3_authority.py').is_file() else 'scripts/ci/f13_native_p3_source.py'))
        native=importlib.util.module_from_spec(spec);spec.loader.exec_module(native)
        native_applicability=current_author_J_native_applicability(repo,native,old,current)
        # Reconstruct full authority before sealing it. Do not duplicate large
        # source records/certificates in argv; consumers repeat reconstruction,
        # so these are bindings to checked inputs, not supplied authority.
        author_fields=dict(author_capability_path=str(Path(author_capability_path).resolve()),
            author_capability=dict(bytes=len(capability_raw),sha256=sha(capability_raw)),
            master_entry=dict(bytes=len(canonical(entry)),sha256=sha(canonical(entry))),
            native_applicability=dict(bytes=len(canonical(native_applicability)),sha256=sha(canonical(native_applicability))))
    target=git(repo,'rev-parse','HEAD').decode().strip();target_parents=git(repo,'rev-list','--parents','-n','1',target).decode().split()[1:]
    require(current['target']==target and target_parents==[current['base'],current['source']],'actual_supplemental_current_target')
    require(git(repo,'rev-parse',target+'^{tree}').strip()==git(repo,'rev-parse',current['source']+'^{tree}').strip(),'supplemental_source_result_parity')
    for name in ('f13_f54_history.py','voice-db-runtime-provisioning-contract_test.sh','voice-r22-runtime-scope.sh'):
        path='scripts/ci/'+name
        require((ROOT/path).read_bytes()==git(repo,'show',target+':'+path),'actual_supplemental_observer_source_bytes')
    allowed={'scripts/ci/f13_f54_history.py','scripts/ci/f13_f54_history_test.py','scripts/ci/voice-db-runtime-provisioning-contract_test.sh','.agent/execplans/native-p3-master-source.md'}
    if author_capability_path is None:
        require(set(git(repo,'diff','--name-only',PROTOTYPE_BASE,current['source']).decode().splitlines())<=allowed,'no_foreign_context_or_pin_drift')
    retained=[e for e in events if e['kind']=='row_committed' and e['operation']==reference]
    require([e['payload']['path'] for e in retained]==value['paths'],'original_aggregate_all_committed_paths')
    return dict(type=('F54_FIXED_PRODUCTION_AGGREGATE_SPAN_OBSERVATION_v1' if is_production else
        'F54_FIXED_CURRENT_AUTHOR_AGGREGATE_SPAN_OBSERVATION') if author_capability_path is not None else 'F54_FIXED_AGGREGATE_SPAN_OBSERVATION',
        **author_fields,original_trace_sha256=ORIGINAL_TRACE_SHA,
        original_operation=operation,original_frame=frame,original_classification=classified,
        original_committed=retained,original_native_sha256=ORIGINAL_NATIVE_SHA,current_native=current,
        current_native_sha256=sha(current_raw),supplemental_classifier=classifier_binding(),
        observer_sha256=sha((ROOT/'scripts/ci/f13_f54_history.py').read_bytes()),
        base=SPAN_BASE,head=SPAN_HEAD,allow_direct_followup='false',partition_source_history='true',admission=False)


def span_author_origin(repo, original_path, original_native_path, current_native_path, author_capability_path):
    return span_origin(repo,original_path,original_native_path,current_native_path,author_capability_path)


def validate_aggregate_span(repo, original_path, span_path, span_sha, original_native_path, current_native_path):
    raw=Path(span_path).read_bytes();require(sha(raw)==span_sha,'supplemental_trace_raw_hash')
    events=[json.loads(v) for v in raw.splitlines()]
    require(events and events[0]['kind']=='span_link','supplemental_origin_first_link')
    supplied=events[0]['payload']
    scoped_span=supplied.get('type') in ('F54_FIXED_CURRENT_AUTHOR_AGGREGATE_SPAN_OBSERVATION','F54_FIXED_PRODUCTION_AGGREGATE_SPAN_OBSERVATION_v1')
    author_path=supplied.get('author_capability_path') if scoped_span else None
    if scoped_span:
        require(isinstance(author_path,str) and bool(author_path),'current_author_span_explicit_capability_path')
    origin=span_origin(repo,original_path,original_native_path,current_native_path,author_path)
    require(events and events[0]['kind']=='span_link' and events[0]['payload']==origin,'exact_observed_aggregate_link')
    require(sum(e['kind']=='span_link' for e in events)==1,'one_supplemental_link')
    verified=validate_trace(repo,span_path,span_sha)
    entered=[e for e in events if e['kind']=='frame_enter'];exits=[e for e in events if e['kind']=='frame_exit']
    require(len(entered)==len(exits)==1 and entered[0]['payload']['parent'] is None,'separate_exact_supplemental_frame')
    frame=entered[0]['payload'];identity=frame['id']
    require([frame['base'],frame['target'],frame['allow_direct_followup'],frame['partition_source_history']]==[SPAN_BASE,SPAN_HEAD,'false','true'],'exact_recursive_span_parameters')
    require(exits[0]['payload']['exit']==0 and not verified['failures'],'supplemental_gate_failure_keeps_hold')
    operations=[e for e in events if e['kind']=='operation_begin']
    require([e['payload']['commit'] for e in operations]==SPAN_NODES and all(e['payload']['parent_index']==1 for e in operations),'complete_actual_internal_operations')
    # These two fixed direct operations have no checkpoint exclusions. Their
    # actual classifier inputs must retain every operation path, even when a
    # forged shorter input happens to yield the same false Boolean.
    require(not any(e['kind']=='disposition' for e in events),'no_unreviewed_span_classifier_exclusion')
    for operation in operations:
        classification=next(e for e in events if e['kind']=='classification' and e['operation']==operation['operation'])
        require(classification['payload']['unapproved_delta']==operation['payload']['delta'],'complete_exact_span_classifier_input')
    original_owner_rows={e['payload']['path']:e for e in origin['original_committed']}
    obligations={}
    for name,value in verified['ledger'][identity]['paths'].items():
        retained=original_owner_rows.get(name)
        if retained:
            parts=row(retained['payload']['row'])
            require(parts[2]==SPAN_AGGREGATE and parts[4:6]==[SPAN_BASE,SPAN_HEAD],'original_integration_owner_is_not_replaced')
            require(entry_binding(repo,SPAN_HEAD,name)==entry_binding(repo,operations[-1]['payload']['commit'],name),'exact_source_content_lineage')
        obligations[name]=dict(aggregate=value['aggregate'] or bool(retained and any(row(x)[1]=='true' for x in retained['payload']['retained_rows'])),
            internal_evidence=value['evidence'],original_aggregate_row=retained,
            hidden_from_net_delta=retained is None)
    dispositions=[dict(node=e['payload']['commit'],parent_index=1,disposition='aggregate_span_qualified',
        original_trace_sha256=ORIGINAL_TRACE_SHA,aggregate_reference=origin['original_operation']['operation'],
        supplemental_trace_sha256=span_sha,frame=identity,operation_reference=e['operation'],
        classification_reference=next(x['digest'] for x in events if x['kind']=='classification' and x['operation']==e['operation']),
        completed_frame_reference=exits[0]['digest']) for e in operations]
    return dict(type='F54_FIXED_AGGREGATE_SPAN_QUALIFICATION',original_trace_sha256=ORIGINAL_TRACE_SHA,
        supplemental_trace_sha256=span_sha,original_owner=SPAN_AGGREGATE,dispositions=dispositions,
        obligations=obligations,verified_supplemental=verified,admission=False)


def consume_span_aggregate(verified, name, actual_trigger):
    value=verified['obligations'][name]
    require(actual_trigger in ('true','false') and (actual_trigger=='true')==value['aggregate'],'aggregate_span_consumer_must_keep_internal_taint')
    require(value['internal_evidence'],'actual_span_obligations_consumed')
    return dict(path=name,aggregate=value['aggregate'],evidence=value,
        supplemental_trace_sha256=verified['supplemental_trace_sha256'],admission=False)


def observe_qualified_unit(repo, main_path, main_sha, original_path, span_path, span_sha, original_native_path, current_native_path):
    """Consume two source-specific actual traces, without merging their rows.

    Only the exact two missing nodes may obtain a supplemental disposition.
    Original f54 remains attempted/HOLD and original integration rows remain
    the only outer rows; internal owners are referenced evidence only.
    """
    main=validate_trace(repo,main_path,main_sha)
    span=validate_aggregate_span(repo,original_path,span_path,span_sha,original_native_path,current_native_path)
    events=[json.loads(v) for v in Path(main_path).read_bytes().splitlines()]
    original_events=[json.loads(v) for v in Path(original_path).read_bytes().splitlines()]
    reference='F1/'+SPAN_AGGREGATE+'/2'
    old=next(e for e in original_events if e['kind']=='operation_begin' and e['operation']==reference)
    current=next(e for e in events if e['kind']=='operation_begin' and e['operation']==reference)
    require(current['payload']==old['payload'],'current_aggregate_exact_original_operation')
    old_rows=[e['payload'] for e in original_events if e['kind']=='row_committed' and e['operation']==reference]
    current_rows=[e['payload'] for e in events if e['kind']=='row_committed' and e['operation']==reference]
    require(current_rows==old_rows,'original_aggregate_owners_maps_flags_unchanged')
    coverage=dag_dispositions(repo,events)
    replacements={v['node']:v for v in span['dispositions']}
    require(set(replacements)==set(SPAN_NODES),'complete_fixed_span_dispositions')
    for value in coverage:
        if value['node'] in replacements:
            require(len(value['operations'])==1 and value['operations'][0]['disposition']=='UNREACHED','supplement_only_original_missing_node')
            value['operations']=[dict(replacements[value['node']],current_aggregate_trace_sha256=main_sha,
                current_aggregate_operation_reference=reference)]
    root=main['ledger']['F1'];required={name:dict(aggregate=v['aggregate'],outer_evidence=v['evidence'],internal_evidence=None)
        for name,v in root['paths'].items()}
    for name,value in span['obligations'].items():
        item=required.setdefault(name,dict(aggregate=False,outer_evidence=[],internal_evidence=None))
        item['aggregate']=item['aggregate'] or value['aggregate'];item['internal_evidence']=value
    return dict(type='F54_FIXED_COMPLETE_UNIT_OBSERVATION',main_trace_sha256=main_sha,
        supplemental_trace_sha256=span_sha,coverage=coverage,required_qualification=required,
        outer_map_binding=root['map_binding'],outer_rows=root['rows'],main_failures=main['failures'],
        original_owner=SPAN_AGGREGATE,qualification='HOLD',admission=False)


def consume_unit_path(verified, name, actual_map, actual_trigger):
    value=verified['required_qualification'][name]
    require(verified['outer_map_binding']==dict(bytes=len(actual_map),sha256=sha(actual_map)) and actual_map.decode().splitlines()==verified['outer_rows'],'qualification_consumes_original_complete_outer_map')
    require(actual_trigger in ('true','false') and (actual_trigger=='true')==value['aggregate'],'qualification_consumes_outer_and_internal_or')
    require(value['outer_evidence'] or value['internal_evidence'],'qualification_consumes_full_owner_content_evidence')
    return dict(path=name,aggregate=value['aggregate'],obligations=value,
        main_trace_sha256=verified['main_trace_sha256'],supplemental_trace_sha256=verified['supplemental_trace_sha256'],admission=False)


def original_scope_exit(name, trigger):
    """Execute the unchanged gate, with the consumed aggregate, not a new policy."""
    source=(ROOT/'scripts/ci/voice-db-runtime-provisioning-contract_test.sh').read_text()
    functions=[]
    for symbol in ('identity_publisher_path_allowed','f13_cross_scope_path_allowed'):
        start=source.index(symbol+'() {');end=source.index('\n}\n',start)+3
        functions.append(source[start:end])
    result=subprocess.run(['bash','-c','\n'.join(functions)+'\nf13_cross_scope_path_allowed "$1" "$2"',
        'f54-original-scope',name,trigger],capture_output=True)
    require(result.returncode in (0,1),'actual_original_scope_execution')
    return result.returncode


def build_M_ready(repo, state, events, prefix_binding, original_path, span_path, span_sha, original_native, current_native):
    """Conserve real ledgers separately; never turn DAG nodes into owner rows."""
    span=validate_aggregate_span(repo,original_path,span_path,span_sha,original_native,current_native)
    original=[json.loads(v) for v in Path(original_path).read_bytes().splitlines()]
    reference='F1/'+SPAN_AGGREGATE+'/2'
    old=next(e for e in original if e['kind']=='operation_begin' and e['operation']==reference)
    current_reference=state['frame']+'/'+SPAN_AGGREGATE+'/2'
    actual=next(e for e in events if e['kind']=='operation_begin' and e['operation']==current_reference)
    require({k:v for k,v in actual['payload'].items() if k!='id'}=={k:v for k,v in old['payload'].items() if k!='id'},'M_prefix_original_aggregate_operation')
    require([e['payload'] for e in events if e['kind']=='row_committed' and e['operation']==current_reference]==
        [e['payload'] for e in original if e['kind']=='row_committed' and e['operation']==reference],
        'M_prefix_original_aggregate_rows')
    coverage=state['coverage'];replacements={v['node']:v for v in span['dispositions']}
    for value in coverage:
        if value['node'] in replacements:
            require(len(value['operations'])==1 and value['operations'][0]['disposition']=='UNREACHED','M_only_observed_missing_span')
            value['operations']=[replacements[value['node']]]
    accepted={'visited_real_operation','shared_real_operation','exact_single_commit_source_of_real_operation','aggregate_span_qualified'}
    require(all(v['disposition'] in accepted for node in coverage for v in node['operations']),'M_complete_actual_33_dispositions')
    required={}
    for frame,ledger in state['ledger'].items():
        for name,value in ledger['paths'].items():
            item=required.setdefault(name,dict(aggregate=False,retained_evidence=[],span_evidence=None,M_classification=None))
            item['aggregate']=item['aggregate'] or value['aggregate']
            item['retained_evidence'].append(dict(frame=frame,aggregate=value['aggregate'],evidence=value['evidence']))
    for name,value in span['obligations'].items():
        item=required.setdefault(name,dict(aggregate=False,retained_evidence=[],span_evidence=None,M_classification=None))
        item['aggregate']=item['aggregate'] or value['aggregate'];item['span_evidence']=value
    for name in state['paths']:
        item=required.setdefault(name,dict(aggregate=False,retained_evidence=[],span_evidence=None,M_classification=None))
        item['M_classification']=dict(operation=state['operation'],classification=state['classification'])
        item['aggregate']=item['aggregate'] or state['classification']['trigger']=='true'
    outer=state['ledger'][state['frame']]
    return dict(type='F54_ACTUAL_M_COMPLETE_RETAINED_LEDGER',operation=state['operation'],
        capability_sha256=state['capability_sha256'],prefix=prefix_binding,operation_binding=state['operation_binding'],
        frame_ancestry=state['frame_ancestry'],aggregate_references=dict(original=reference,current=current_reference),
        original_trace=str(original_path),original_native=str(original_native),current_native=str(current_native),
        span_trace=str(span_path),span_trace_sha256=span_sha,ledger=state['ledger'],coverage=coverage,
        outer_rows=outer['rows'],outer_map_binding=outer['map_binding'],required_qualification=required,
        M_rows_pending=True,qualification='HOLD',admission=False,runtime_pass=False)


def prepare_M_ledger(repo, capability_path, trace_path, original_path, span_path, original_native, current_native):
    capability=verified_capabilities(repo,capability_path)
    raw=Path(trace_path).read_bytes();digest=sha(raw);events=[json.loads(v) for v in raw.splitlines()]
    operation=next(e['operation'] for e in events if e['kind']=='history_capability')
    state=validate_trace(repo,trace_path,digest,checkpoint=operation)['M_checkpoint']
    require(state['capability_sha256']==sha(canonical(capability)),'M_ready_actual_capability')
    return build_M_ready(repo,state,events,dict(bytes=len(raw),sha256=digest),original_path,span_path,
        sha(Path(span_path).read_bytes()),original_native,current_native)


def consume_M_path(ready, expected_sha, trace_path, name, actual_map, after_gate_raw, transition='append'):
    """Only the constructor-generated full ledger is consumed by the real gate.

    The final validator independently reconstructs it at the append boundary.
    This transport has no authority for an ordinary production collector.
    """
    raw=Path(ready).read_bytes();require(sha(raw)==expected_sha,'M_consumer_complete_ledger_hash')
    value=json.loads(raw);require(value['type']=='F54_ACTUAL_M_COMPLETE_RETAINED_LEDGER' and value['admission'] is False,'M_consumer_type')
    require(value['operation'].split('/')[1:]==[M,'2'],'M_consumer_fixed_operation_not_J_transport')
    require(name in value['operation_binding']['paths'],'M_consumer_exact_operation_path')
    prefix=Path(trace_path).read_bytes();bound=value['prefix']
    require(len(prefix)>=bound['bytes'] and sha(prefix[:bound['bytes']])==bound['sha256'],'M_consumer_actual_prefix')
    events=[json.loads(v) for v in prefix.splitlines()]
    before=next(e for e in reversed(events) if e['kind']=='row_before' and e['operation']==value['operation'])
    require(before['payload']['path']==name,'M_consumer_current_original_row')
    require(before['payload']['map_before']==dict(bytes=len(actual_map),sha256=sha(actual_map)),'M_consumer_entire_still_current_map')
    rows=actual_map.decode().splitlines();prior=[raw_binding(x) for x in rows if x.split('\t')[0]==name]
    require(prior==before['payload']['prior_rows'] and prior==[raw_binding(x) for x in value['outer_rows'] if x.split('\t')[0]==name],'M_consumer_retains_all_original_prior_rows')
    incoming=row(before['payload']['incoming']);after=row(raw_binding(after_gate_raw))
    require(all(after[i]==incoming[i] for i in (0,2,3,6,7)),'M_consumer_immutable_row_identity')
    if after[4:6]!=incoming[4:6]:
        gates=[e for e in events[before['sequence']:] if e['kind']=='native_rebind_gate' and e['operation']==value['operation'] and e['payload']['path']==name]
        require(len(gates)==1 and gates[0]['payload']['status']==0 and after[4:6]==[P1,M],'M_consumer_actual_source_rebind_gate')
    aggregate=value['required_qualification'][name]['aggregate'] or any(row(x)[1]=='true' for x in prior) or incoming[1]=='true' or after[1]=='true'
    require(transition in ('append','replace') and (transition!='replace' or prior),'M_consumer_actual_transition')
    after_rows=([x for x in rows if x.split('\t')[0]!=name] if transition=='replace' else rows)+[after_gate_raw]
    after_map=('\n'.join(after_rows)+'\n').encode()
    supersessions=[e['payload']['receipt'] for e in events[before['sequence']:] if e['kind']=='exact_unit_supersession' and e['operation']==value['operation'] and e['payload']['path']==name]
    require(len(supersessions)<=1,'one_actual_exact_supersession')
    receipt=dict(path=name,ledger_sha256=sha(canonical(value)),incoming=before['payload']['incoming'],after_gate_row=raw_binding(after_gate_raw),
        prior_rows=prior,map_before=dict(bytes=len(actual_map),sha256=sha(actual_map)),transition=transition,
        expected_post_map=dict(bytes=len(after_map),sha256=sha(after_map)),aggregate=aggregate,
        obligations=value['required_qualification'][name],policy_result=None,admission=False)
    if supersessions:
        exact=supersessions[0]
        require(transition=='replace' and exact['operation']==value['operation'] and exact['path']==name
            and exact['ledger_sha256']==sha(canonical(value)) and exact['incoming']==before['payload']['incoming'] and exact['prior_rows']==prior
            and exact['map_before']==receipt['map_before'] and exact['aggregate']==aggregate,'conservation_same_exact_qualified_transition')
        receipt['exact_unit_supersession']=exact
    return receipt


def supersession_evidence(repo, capability, ready, events, name, actual_map, incoming_raw, composition_receipt):
    """Finite transition qualification; neither ancestry nor source is rebound.

    The runtime constructor and final trace validator both call this predicate.
    A caller-supplied hash alone cannot replace its real operation references.
    """
    operation=ready['operation'];frame=operation.split('/')[0]
    require(operation==frame+'/'+M+'/2' and ready['capability_sha256']==sha(canonical(capability)), 'supersession_exact_M_capability')
    require(name in capability['guarded_difference_paths'] and name in capability['actual_source_delta']['raw'].splitlines(), 'supersession_complete_unit_path')
    require(all(v['disposition'] in ('visited_real_operation','shared_real_operation','exact_single_commit_source_of_real_operation','aggregate_span_qualified')
        for node in ready['coverage'] for v in node['operations']) and len(ready['coverage'])==33, 'supersession_complete_actual_history')
    before=next((e for e in reversed(events) if e['kind']=='row_before' and e['operation']==operation),None)
    require(before is not None and before['payload']['path']==name,'supersession_actual_pending_row')
    value=before['payload'];incoming=row(raw_binding(incoming_raw))
    require(value['incoming']==raw_binding(incoming_raw) and incoming[:1]+incoming[2:6]+incoming[7:]==[name,M,P1,O,P2,','.join(BASES)],'supersession_no_source_or_owner_rebind')
    require(value['map_before']==dict(bytes=len(actual_map),sha256=sha(actual_map)),'supersession_actual_full_premap')
    prior=[raw_binding(x) for x in actual_map.decode().splitlines() if x.split('\t')[0]==name]
    require(prior and prior==value['prior_rows']==[raw_binding(x) for x in ready['outer_rows'] if x.split('\t')[0]==name],'supersession_all_prior_rows')
    expected=composition(repo,O,P1,P2,name,tree_entry(repo,M,name).rstrip('\0'))
    expected=dict(expected,type=capability['whole_parent_composition']['type'],capability_sha256=sha(canonical(capability)))
    require(composition_receipt==expected,'supersession_explicit_whole_parent_composition')
    require(any(e['kind']=='composition_capability' and e['operation']==operation and e['payload']==expected
        for e in events[before['sequence']:]),'supersession_real_transition_composition_reference')
    references=[]
    for raw in prior:
        parts=row(raw);require(parts[6]==incoming[6],'supersession_earliest_content_base')
        ancestor=lambda a,b:subprocess.run(['git','--no-replace-objects','-C',str(repo),'merge-base','--is-ancestor',a,b],capture_output=True).returncode==0
        require(ancestor(parts[2],P1) and ancestor(parts[5],P2),'supersession_checked_parent_containment')
        entry=tree_entry(repo,parts[2],name)
        require(entry and entry==tree_entry(repo,parts[5],name)==tree_entry(repo,P1,name),'supersession_actual_prior_owner_source_entries')
        versions=next(v['versions'] for v in capability['complete_41_entries'] if v['path']==name)
        mode,kind,rest=entry.rstrip('\0').split(' ',2);oid=rest.split('\t')[0]
        require(dict(mode=mode,type=kind,oid=oid) in versions.values(),'supersession_prior_entry_in_fixed_versions')
        commits=[e for e in events[:before['sequence']-1] if e['kind']=='row_committed' and e['frame']==frame and e['payload']['row']==raw]
        require(commits,'supersession_real_prior_owner_commit')
        committed=commits[-1];origin=committed['operation']
        require(origin==frame+'/'+parts[2]+'/'+origin.rsplit('/',1)[1], 'supersession_actual_prior_operation_owner')
        starts=[e for e in events[:committed['sequence']] if e['operation']==origin and e['kind']=='operation_begin']
        classifications=[e for e in events[:committed['sequence']] if e['operation']==origin and e['kind']=='classification']
        changes=[e for e in events[:committed['sequence']] if e['operation']==origin and e['kind']=='row_after' and e['payload']['row']==raw]
        require(len(starts)==len(classifications)==1 and changes,'supersession_real_prior_classification_and_transformation')
        require(any(item['operation']==origin for item in ready['ledger'][frame]['paths'][name]['evidence']),'supersession_prior_owner_consumed_ledger')
        references.append(dict(row=raw,operation=origin,begin=starts[0]['digest'],classification=classifications[0]['digest'],
            row_after=changes[-1]['digest'],committed=committed['digest'],entry=raw_binding(entry),
            owner_to_O=ancestor(parts[2],O),source_to_O=ancestor(parts[5],O)))
    aggregate=ready['required_qualification'][name]['aggregate'] or any(row(v)[1]=='true' for v in prior) or incoming[1]=='true'
    return dict(type='F54_EXACT_UNIT_SUPERSESSION_PROTOTYPE',operation=operation,path=name,capability_sha256=sha(canonical(capability)),
        ledger_sha256=sha(canonical(ready)),prior_rows=prior,incoming=raw_binding(incoming_raw),map_before=value['map_before'],
        prior_operations=references,composition=composition_receipt,aggregate=aggregate,source_rebind=False,admission=False)


def qualify_supersession(repo, capability_path, ready_path, ready_sha, trace_path, name, map_path, incoming_raw):
    capability=verified_capabilities(repo,capability_path)
    raw=Path(ready_path).read_bytes();require(sha(raw)==ready_sha,'supersession_complete_ledger_raw_hash');ready=json.loads(raw)
    trace=Path(trace_path).read_bytes();prefix=ready['prefix']
    require(len(trace)>=prefix['bytes'] and sha(trace[:prefix['bytes']])==prefix['sha256'],'supersession_sealed_actual_prefix')
    events=[json.loads(v) for v in trace.splitlines()]
    composition_receipt=next((e['payload'] for e in reversed(events) if e['kind']=='composition_capability' and e['operation']==ready['operation'] and e['payload']['path']==name),None)
    receipt=supersession_evidence(repo,capability,ready,events,name,Path(map_path).read_bytes(),incoming_raw,composition_receipt)
    return dict(receipt,trace_prefix=dict(bytes=len(trace),sha256=sha(trace)))


def include_typed_collection_required(required, I_ready, J_ready):
    """Keep every I/J obligation, including paths absent from their net delta."""
    required=json.loads(json.dumps(required))
    for kind,table,expected in (('I',I_ready,'F54_FIXED3071_ACTUAL_I_RANGE_LEDGER'),('J',J_ready,'F54_FIXED3071_ACTUAL_J_COMPLETE_LEDGER')):
        for operation,ready in table.items():
            require(ready['type']==expected and ready['operation']==operation and ready['admission'] is False,'fixed3071_collection_exact_typed_ledger')
            inherited_sha=None
            for name,value in ready['required_qualification'].items():
                require(type(value['aggregate']) is bool,'fixed3071_collection_typed_boolean')
                if inherited_sha is None:inherited_sha=sha(canonical(ready))
                item=required.setdefault(name,dict(aggregate=False,retained_evidence=[],span_evidence=None,M_classification=None))
                item['aggregate']|=value['aggregate']
                item['retained_evidence'].append(dict(typed_ledger=kind,operation=operation,ledger_sha256=inherited_sha,aggregate=value['aggregate'],evidence=value))
    return required


def finalize_collection(repo, trace_path, map_path):
    raw=Path(trace_path).read_bytes();verified=validate_trace(repo,trace_path,sha(raw))
    require(not verified['failures'] and len(verified['M_ready'])==1,'successful_exact_M_collection_required')
    operation,ready=next(iter(verified['M_ready'].items()))
    require(verified['M_consumed'][operation]==ready['operation_binding']['paths'],'all_M_transitions_conserved')
    root=verified['root_frame'];outer=verified['ledger'][root];actual=Path(map_path).read_bytes()
    require(outer['map_binding']==dict(bytes=len(actual),sha256=sha(actual)) and outer['rows']==actual.decode().splitlines(),'final_full_original_map')
    required=ready['required_qualification']
    for frame,ledger in verified['ledger'].items():
        for name,value in ledger['paths'].items():
            item=required.setdefault(name,dict(aggregate=False,retained_evidence=[],span_evidence=None,M_classification=None))
            item['aggregate']=item['aggregate'] or value['aggregate']
            item['retained_evidence'].append(dict(final_frame=frame,aggregate=value['aggregate'],evidence=value['evidence']))
    required=include_typed_collection_required(required,verified['I_ready'],verified['J_ready'])
    events=[json.loads(v) for v in raw.splitlines()]
    transitions=[e for e in events if e['kind'] in ('complete_ledger_conserved','row_remove','row_after','row_committed') and e['operation']==operation]
    return dict(type='F54_SEALED_SUCCESSFUL_COLLECTION',collection_prefix=dict(bytes=len(raw),sha256=sha(raw)),
        collection_trace=str(trace_path),map_path=str(map_path),map_binding=outer['map_binding'],outer_rows=outer['rows'],
        ledger=verified['ledger'],terminal_dispositions=terminal_dispositions(verified['ledger']),
        manual_result_operations=manual_operations(verified['ledger']),
        I_ledgers=verified['I_ready'],J_ledgers=verified['J_ready'],
        required_qualification=required,coverage=ready['coverage'],
        child_span=dict(path=ready['span_trace'],sha256=ready['span_trace_sha256']),M_transitions=transitions,
        capability_sha256=ready['capability_sha256'],M=M,source_context=router_source_context(repo),
        master_entry=verified['master_entry'],master_wrapper=verified['master_wrapper'],root_frame=root,M_frame=operation.split('/')[0],
        qualification='HOLD',admission=False,runtime_pass=False)


def router_source_context(repo):
    return dict(ROOT=str(Path(repo).resolve()),target=git(repo,'rev-parse','HEAD').decode().strip(),M=M,
        files={name:file_binding(ROOT/name) for name in ('scripts/ci/f13_f54_history.py',
            'scripts/ci/voice-db-runtime-provisioning-contract_test.sh','scripts/ci/voice-r22-runtime-scope.sh')})


def load_collection(repo, path, expected_sha, map_path):
    raw=Path(path).read_bytes();require(sha(raw)==expected_sha,'sealed_collection_document_hash')
    value=json.loads(raw);require(value['type']=='F54_SEALED_SUCCESSFUL_COLLECTION' and value['admission'] is False,'sealed_collection_type')
    trace=Path(value['collection_trace']).read_bytes();seal=value['collection_prefix']
    require(len(trace)==seal['bytes'] and sha(trace)==seal['sha256'],'immutable_completed_collection_prefix')
    require(value['source_context']==router_source_context(repo),'actual_collection_source_context')
    actual=Path(map_path).read_bytes()
    require(value['map_path']==str(map_path) and value['map_binding']==dict(bytes=len(actual),sha256=sha(actual)) and value['outer_rows']==actual.decode().splitlines(),'router_full_original_map_not_selected_rows')
    require('terminal_dispositions' in value and 'ledger' in value,'complete_collection_terminal_state_fields')
    require(value['terminal_dispositions']==terminal_dispositions_in_prefix(value['ledger'],trace,seal),
        'collection_requires_all_typed_terminal_dispositions')
    require(value.get('manual_result_operations',[])==manual_operations_in_prefix(value['ledger'],trace,seal), 'manual_F_collection_complete_operations')
    events=collection_trace_events(trace)
    tables=[]
    for kind,field in (('range_I_ledger_ready','I_ledgers'),('range_J_ledger_ready','J_ledgers')):
        expected={}
        for e in events:
            if e['kind']==kind:
                binding=e['payload']['ledger'];require(binding==raw_binding(binding['raw']),'fixed3071_collection_actual_ledger_event')
                require(e['operation'] not in expected,'fixed3071_collection_no_duplicate_typed_ledger')
                expected[e['operation']]=json.loads(binding['raw'])
        require(value.get(field,{})==expected,'fixed3071_collection_all_actual_typed_ledgers')
        tables.append(expected)
    minimum=include_typed_collection_required({},*tables)
    for name,obligation in minimum.items():
        actual=value['required_qualification'].get(name)
        require(actual is not None and type(actual['aggregate']) is bool and (not obligation['aggregate'] or actual['aggregate']), 'fixed3071_collection_no_lost_prior_or_reverted_taint')
        require(all(record in actual['retained_evidence'] for record in obligation['retained_evidence']), 'fixed3071_collection_no_lost_typed_owner_history')
    for disposition in value['terminal_dispositions']:
        name=disposition['path'];item=value['required_qualification'].get(name)
        require(item is not None and item['aggregate'] is True and item['retained_evidence'],
            'deleted_path_still_requires_runtime_and_retained_history_router')
        require(tree_entry(repo,value['source_context']['target'],name)==''
            and not git(repo,'rev-list','--full-history',F_CORRECTED_TUPLE['merge']+'..'+value['source_context']['target'],'--',name).strip(),
            'reviewed_F_deletion_no_stale_obligation_after_later_writer')
    return value


class FiniteJSession:
    """One live process's independently folded state, never a disk checkpoint."""
    def __init__(self, repo, capability_path, ready_path, ready_sha, trace_path, map_path):
        self.repo=Path(repo);self.capability_path=Path(capability_path)
        self.ready_path=Path(ready_path);self.ready_sha=ready_sha
        self.trace_path=Path(trace_path);self.map_path=Path(map_path)
        self.event_context=finite_J_event_context()
        raw=self.trace_path.read_bytes()
        # Always execute the whole fold on startup, including after a restart.
        verified=validate_trace(self.repo,self.trace_path,sha(raw),finite_J_checkpoint=True)
        self.initial=verified['finite_J_checkpoint']
        self.ready=self.initial['ready'];self.capability=self.initial['capability']
        self.base_events=[json.loads(line) for line in raw.splitlines()]
        self.prefix=file_digest(raw);self.last_seen=self.prefix
        self.names=finite_J_paths();self.finished=False
        self.fresh_inputs()
        require(self.map_path.read_bytes()==self.initial['map'].encode(),'finite_J_start_actual_map')

    def fresh_inputs(self):
        require(finite_J_event_context()==self.event_context,'finite_J_live_event_context')
        require(verified_3071_capabilities(self.repo,self.capability_path)==self.capability,
                'finite_J_fresh_current_capability')
        raw=self.ready_path.read_bytes()
        require(sha(raw)==self.ready_sha and json.loads(raw)==self.ready,'finite_J_fresh_complete_ready')
        finite_J_source_unit(self.repo)

    def inspect(self):
        require(not self.finished,'finite_J_finished_session')
        self.fresh_inputs()
        raw=self.trace_path.read_bytes()
        require(len(raw)>=self.last_seen['bytes'] and file_digest(raw[:self.last_seen['bytes']])==self.last_seen,
                'finite_J_no_trace_rewrite_or_rollback')
        prefix=raw[:self.prefix['bytes']]
        require(file_digest(prefix)==self.prefix,'finite_J_fresh_validated_prefix')
        hasher=hashlib.sha256(prefix);offset=len(prefix)
        events=list(self.base_events);previous=events[-1]['digest'];sequence=events[-1]['sequence']
        phase=self.initial['phase'];index=self.initial['index']
        permission=self.initial['permission'];conserved=None
        current=self.initial['map'].encode()
        for line in raw[offset:].splitlines(keepends=True):
            require(line.endswith(b'\n'),'finite_J_complete_tail_frame')
            event=json.loads(line);body=dict(event);digest=body.pop('digest',None)
            require(event.get('schema')==SCHEMA and event.get('sequence')==sequence+1
                    and event.get('previous')==previous and digest==sha(canonical(body)),
                    'finite_J_complete_append_chain')
            require(index<len(self.names) and event['frame']==self.ready['frame']
                    and event['operation']==self.ready['operation'],'finite_J_exact_tail_operation')
            name=self.names[index];value=event['payload'];kind=event['kind']
            require(value.get('path')==name,'finite_J_exact_tail_order')
            if phase=='before':
                require(kind=='row_before','finite_J_tail_requires_pending')
                case=json.loads(FINITE_J_SOURCE_UNIT_RAW)['cases'][index]
                require(value==dict(path=name,incoming=raw_binding(case['incoming']),prior=raw_binding(case['prior']),
                        prior_rows=[raw_binding(case['prior'])],map_before=file_digest(current),
                        phase='before_qualification'),'finite_J_tail_complete_pending_binding')
                phase='permission'
            elif phase=='permission':
                require(kind=='finite_J_owner_permission','finite_J_tail_requires_permission')
                permission=finite_J_owner_evidence(self.repo,self.capability,self.ready,events,
                    dict(bytes=offset,sha256=hasher.hexdigest()),current)
                require(value==dict(path=name,receipt=permission,admission=False),
                        'finite_J_tail_exact_permission')
                phase='consume'
            elif phase=='consume':
                require(kind=='range_J_ledger_conserved','finite_J_tail_requires_conservation')
                conserved=finite_J_conservation(self.ready,permission,current)
                require(value==dict(path=name,receipt=conserved,policy_result=None,admission=False),
                        'finite_J_tail_complete_conservation')
                validate_exact_conservation(dict(op=self.ready['operation'],finite_J_owner_permission=permission),
                                           self.ready['operation'],value['receipt'])
                phase='remove'
            elif phase=='remove':
                require(kind=='row_remove' and value==dict(path=name,row=permission['incoming'],
                    reason='original_guarded_map_replacement',phase='before_mutation'),'finite_J_tail_exact_remove')
                current=(''.join(x+'\n' for x in current.decode().splitlines() if x.split('\t')[0]!=name)).encode()
                phase='after'
            elif phase=='after':
                require(kind=='row_after' and value==dict(path=name,row=permission['incoming'],
                    reason='original_owner_and_row_assignment',phase='before_mutation'),'finite_J_tail_exact_after')
                phase='committed'
            elif phase=='committed':
                require(kind=='row_committed','finite_J_tail_requires_commit')
                current+= (permission['incoming']['raw']+'\n').encode()
                require(value==dict(path=name,row=permission['incoming'],map_after=file_digest(current),
                                    retained_rows=[permission['incoming']])
                        and conserved['expected_post_map']==file_digest(current),'finite_J_tail_complete_committed_map')
                index+=1;phase='finish' if index==len(self.names) else 'before';permission=None
            else:raise Rejected('finite_J_unexpected_after_finish')
            events.append(event);previous=digest;sequence+=1;offset+=len(line);hasher.update(line)
        require(self.map_path.read_bytes()==current,'finite_J_fresh_complete_current_map')
        self.last_seen=file_digest(raw)
        return phase,index,permission,current,events,self.last_seen

    def request(self, command, name):
        require(command in ('permission','consume','finish'),'finite_J_request_command')
        phase,index,permission,current,events,prefix=self.inspect()
        if command=='finish':
            require(name=='' and phase=='finish' and index==len(self.names),'finite_J_finish_after_tenth_commit')
            self.finished=True;return ''
        require(index<len(self.names) and name==self.names[index] and command==phase,
                'finite_J_request_exact_pending_phase')
        if command=='permission':
            value=finite_J_owner_evidence(self.repo,self.capability,self.ready,events,prefix,current)
        else:value=finite_J_conservation(self.ready,permission,current)
        return json.dumps(value)+'\n'


def finite_J_event_context():
    return {name:os.environ.get(name,'') for name in
            ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','VOICE_CI_PUSH_BEFORE_SHA','GITHUB_BASE_REF','GITHUB_HEAD_REF')}


def finite_J_worker_request(socket_path, session, command, name):
    request=dict(session=session,nonce=secrets.token_hex(32),command=command,path=name,context=finite_J_event_context())
    encoded=json.dumps(request).encode();require(len(encoded)<=65536,'finite_J_client_request_limit')
    with socket.socket(socket.AF_UNIX,socket.SOCK_STREAM) as connection:
        connection.set_inheritable(False);connection.settimeout(300)
        connection.connect(socket_path);connection.sendall(encoded);connection.shutdown(socket.SHUT_WR)
        response=_router_socket_read(connection,64*1024*1024)
    require(isinstance(response,dict) and set(response)==set(request)|{'status','output'}
            and all(response[key]==value for key,value in request.items()),'finite_J_client_response_identity')
    require(type(response['status']) is int and response['status']==0 and isinstance(response['output'],str),
            'finite_J_client_response_success')
    if command=='finish':require(name==response['output']=='','finite_J_client_finish_ack')
    return response['output']


def finite_J_worker(socket_path, session, repo, capability, ready_path, ready_sha, trace_path, map_path):
    # Bootstrap before publishing readiness. No caller can supply validated state.
    print('CI_DIAG stage=finite-J-worker phase=full-prefix-reconstruction',file=sys.stderr,flush=True)
    state=FiniteJSession(repo,capability,ready_path,ready_sha,trace_path,map_path)
    path=Path(socket_path);ready=Path(socket_path+'.ready');pending=Path(socket_path+'.pending')
    require(all(not p.exists() and not p.is_symlink() for p in (path,ready,pending)),
            'finite_J_fresh_worker_paths')
    require(path.parent.stat().st_mode & 0o077==0 and re.fullmatch('[0-9a-f]{64}',session) is not None,
            'finite_J_private_worker_identity')
    with socket.socket(socket.AF_UNIX,socket.SOCK_STREAM) as server:
        server.set_inheritable(False);server.bind(str(path));os.chmod(path,0o600);server.listen(1)
        try:
            descriptor=os.open(pending,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
            with os.fdopen(descriptor,'w') as signal:signal.write(session+'\n')
            os.link(pending,ready);pending.unlink()
            print('CI_DIAG stage=finite-J-worker phase=validated-live-session',file=sys.stderr,flush=True)
            while True:
                connection,_=server.accept()
                with connection:
                    connection.set_inheritable(False);connection.settimeout(60)
                    request=_router_socket_read(connection,65536)
                    require(set(request)=={'session','nonce','command','path','context'}
                            and request['session']==session and isinstance(request['nonce'],str)
                            and re.fullmatch('[0-9a-f]{64}',request['nonce']) is not None,
                            'finite_J_worker_request_identity')
                    require(request['context']==state.event_context,'finite_J_fresh_client_event_context')
                    require(isinstance(request['path'],str) and request['command'] in ('permission','consume','finish'),
                            'finite_J_worker_command_domain')
                    output=state.request(request['command'],request['path'])
                    response=json.dumps(dict(request,status=0,output=output)).encode()
                    require(len(response)<=64*1024*1024,'finite_J_worker_response_limit')
                    connection.sendall(response)
                if request['command']=='finish':return
        finally:
            pending.unlink(missing_ok=True);ready.unlink(missing_ok=True);path.unlink(missing_ok=True)


def _router_socket_read(connection, limit):
    chunks=[];size=0
    while True:
        chunk=connection.recv(min(65536,limit-size+1))
        if not chunk:break
        size+=len(chunk);require(size<=limit,'router_worker_frame_limit')
        chunks.append(chunk)
    return json.loads(b''.join(chunks))


def router_worker(socket_path, session, repo, seal, digest, map_path, context_path):
    """Private invocation transport; each request runs the original validator."""
    path=Path(socket_path)
    ready=Path(socket_path+'.ready')
    pending=Path(socket_path+'.pending')
    require(not path.exists() and not path.is_symlink(),'router_worker_fresh_socket')
    require(not ready.exists() and not ready.is_symlink(),'router_worker_fresh_readiness')
    require(not pending.exists() and not pending.is_symlink(),'router_worker_fresh_pending')
    require(path.parent.stat().st_mode & 0o077==0,'router_worker_private_directory')
    require(re.fullmatch('[0-9a-f]{64}',session) is not None,'router_worker_session')
    with socket.socket(socket.AF_UNIX,socket.SOCK_STREAM) as server:
        server.set_inheritable(False);server.bind(str(path));os.chmod(path,0o600);server.listen(1)
        try:
            descriptor=os.open(pending,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
            with os.fdopen(descriptor,'w') as signal:signal.write(session+'\n')
            # Publish complete bytes atomically without replacing another inode.
            os.link(pending,ready);pending.unlink()
            while True:
                connection,_=server.accept()
                with connection:
                    connection.set_inheritable(False);connection.settimeout(60)
                    request=_router_socket_read(connection,65536)
                    require(set(request)=={'session','nonce','command','path'},'router_worker_request_shape')
                    require(request['session']==session and re.fullmatch('[0-9a-f]{64}',request['nonce']) is not None,
                            'router_worker_request_identity')
                    command=request['command'];name=request['path']
                    require(command in ('bind','runtime','history','manual','stop') and isinstance(name,str)
                            and '\x00' not in name and '\n' not in name,'router_worker_request_command')
                    status=0;output=''
                    if command=='stop':require(name=='','router_worker_stop_path')
                    else:
                        value=router_binding(repo,seal,digest,map_path,context_path,name)
                        if command=='bind':output=json.dumps(value)+'\n'
                        elif command=='runtime':
                            if not value['covered']:status=3
                            else:output=('true' if value['aggregate'] else 'false')+'\n'
                        elif command=='history':
                            row=retained_row_for_binding(value)
                            if row is None:status=3
                            else:output=row+'\n'
                        else:output=manual_delta_for_binding(value)
                    response=dict(request,status=status,output=output)
                    encoded=json.dumps(response).encode()
                    require(len(encoded)<=64*1024*1024,'router_worker_response_limit')
                    connection.sendall(encoded)
                if command=='stop':return
        finally:
            pending.unlink(missing_ok=True)
            ready.unlink(missing_ok=True)
            path.unlink(missing_ok=True)


def router_worker_request(socket_path, session, command, name):
    request=dict(session=session,nonce=secrets.token_hex(32),command=command,path=name)
    encoded=json.dumps(request).encode();require(len(encoded)<=65536,'router_worker_request_limit')
    with socket.socket(socket.AF_UNIX,socket.SOCK_STREAM) as connection:
        connection.set_inheritable(False);connection.settimeout(300)
        connection.connect(socket_path);connection.sendall(encoded);connection.shutdown(socket.SHUT_WR)
        response=_router_socket_read(connection,64*1024*1024)
    require(isinstance(response,dict) and set(response)==set(request)|{'status','output'}
            and all(response[k]==v for k,v in request.items()),'router_worker_response_identity')
    require(type(response['status']) is int and response['status'] in (0,3)
            and isinstance(response['output'],str),'router_worker_response_shape')
    require(response['status']!=3 or command in ('runtime','history') and response['output']=='',
            'router_worker_response_status')
    if command=='stop':require(response['status']==0 and response['output']=='','router_worker_stop_ack')
    return response['status'],response['output']


def policy_context(repo, map_path, pairs, require_runtime=True):
    require(len(pairs)%2==0,'policy_context_name_value_pairs');values=dict(zip(pairs[::2],pairs[1::2]))
    mandatory={'ROOT','TMP_DIR','base_sha','accepted_game_checkpoint','game_checkpoint_enabled','native_p3_active','r23_allowlist_enabled','r22_runtime_delta'}
    if not require_runtime:mandatory.remove('r22_runtime_delta')
    require(mandatory<=set(values) and values['ROOT']==str(Path(repo).resolve()),'finite_actual_policy_context')
    allowed=lambda n:n in ('ROOT','TMP_DIR','base_sha','r22_runtime_delta') or n.startswith(('accepted_','game_checkpoint_','native_p3_','r23_'))
    require(all(allowed(n) for n in values),'no_whole_environment_authority_snapshot')
    files={str(map_path):file_binding(map_path)}
    # Existing authority inputs only, not ephemeral per-path delta scratch files.
    for name,value in values.items():
        if name not in ('ROOT','TMP_DIR') and value and Path(value).is_file():files[value]=file_binding(value)
    temporary=Path(values['TMP_DIR'])
    for name in ('game-checkpoint-tree','be255-checkpoint-tree','f13-native-p3-history.tsv','f13-native-p3-approval.json',
            'r23-fixed-delta','r23-allowed-paths','changed-files'):
        files[str(temporary/name)]=file_binding(temporary/name)
    for suffix in ('.game-snapshot-approvals','.game-snapshot-alias-approvals','.native-p3-history','.role-conditional-history','.space-audit-history'):
        files[str(map_path)+suffix]=file_binding(str(map_path)+suffix)
    return dict(values=values,files=files,event={k:os.environ.get(k,'') for k in
        ('GITHUB_EVENT_NAME','GITHUB_REF','GITHUB_SHA','VOICE_CI_PUSH_BEFORE_SHA','GITHUB_BASE_REF','GITHUB_HEAD_REF')},
        source_context=router_source_context(repo))


def seal_router(repo, collection_path, collection_sha, map_path, context_path):
    collection=load_collection(repo,collection_path,collection_sha,map_path)
    # A hash is transport, not authority: reconstruct the successful source
    # collector's complete ledger once before sealing the later policy phase.
    require(collection==finalize_collection(repo,collection['collection_trace'],map_path),'independently_reconstructed_collection_ledger')
    context=json.loads(Path(context_path).read_bytes())
    require(context==policy_context(repo,map_path,sum(([k,v] for k,v in context['values'].items()),[])),'actual_original_policy_context')
    require(context['values']['r22_runtime_delta']==classify([n for n,v in collection['required_qualification'].items() if v['aggregate']]),'global_runtime_consumes_full_retained_history')
    values=context['values'];source=(ROOT/'scripts/ci/voice-db-runtime-provisioning-contract_test.sh').read_text()
    fixed=dict(re.findall(r"(?m)^(accepted_game_checkpoint|accepted_r23_base|game_checkpoint_snapshot_base|game_checkpoint_snapshot_head|game_checkpoint_snapshot_record)='([0-9a-f]{40})'$",source))
    require(set(fixed)<=set(values) and all(values[k]==v for k,v in fixed.items()),'original_fixed_checkpoint_context_not_caller_selected')
    events=[json.loads(v) for v in Path(collection['collection_trace']).read_bytes().splitlines()]
    frame=next(e['payload'] for e in events if e['kind']=='frame_enter')
    require(values['base_sha']==frame['base'],'actual_collection_policy_base')
    ancestor=lambda a,b:subprocess.run(['git','--no-replace-objects','-C',str(repo),'merge-base','--is-ancestor',a,b],capture_output=True).returncode==0
    require(values['game_checkpoint_enabled']==('true' if ancestor(fixed['accepted_game_checkpoint'],context['source_context']['target']) else 'false'),'original_Game_enabled_derivation')
    manifest=subprocess.run(['git','--no-replace-objects','-C',str(repo),'cat-file','-e',values['base_sha']+':protos/voice/r23_contract_manifest.json'],capture_output=True).returncode==0
    require(values['r23_allowlist_enabled']==('true' if ancestor(fixed['accepted_r23_base'],values['base_sha']) and not manifest else 'false'),'original_R23_window_derivation')
    observed=[e['payload'] for e in events if e['kind']=='native_rebind_gate']
    if observed:
        require(values['native_p3_active']=='true','observed_native_obligations_cannot_be_disabled')
        binding_path=values['native_p3_binding'];binding_raw=Path(binding_path).read_bytes()
        require(all(e['binding']==raw_binding(binding_raw) and e['context']=={k:context['event'].get(k,'') for k in e['context']} for e in observed),'same_actual_native_context_reaches_router')
        spec=importlib.util.spec_from_file_location('f54_router_native',ROOT/('scripts/ci/f13_native_p3_authority.py' if (ROOT/'scripts/ci/f13_native_p3_authority.py').is_file() else 'scripts/ci/f13_native_p3_source.py'))
        native=importlib.util.module_from_spec(spec);spec.loader.exec_module(native)
        binding=json.loads(binding_raw);event=context['event']
        actual=native.qualify(repo,binding['base'],binding['source'],binding['target'],event['GITHUB_EVENT_NAME'],event['GITHUB_REF'],event['GITHUB_SHA'],event['VOICE_CI_PUSH_BEFORE_SHA'],event['GITHUB_BASE_REF'])
        require(actual==binding,'router_native_binding_independently_qualified')
        proof=native.audit_map(repo,binding,map_path,Path(values['TMP_DIR'])/'f13-native-p3-history.tsv')
        require(proof==json.loads((Path(values['TMP_DIR'])/'f13-native-p3-approval.json').read_bytes()),'original_complete_native_map_audit_before_router')
    return dict(type='F54_ORIGINAL_ROUTER_SEAL',collection_path=str(collection_path),collection_sha256=collection_sha,
        complete_ledger_sha256=sha(canonical(collection)),context=context,map_path=str(map_path),admission=False)


def router_binding(repo, seal_path, seal_sha, map_path, context_path, name):
    raw=Path(seal_path).read_bytes();require(sha(raw)==seal_sha,'router_seal_hash')
    seal=json.loads(raw);require(seal['type']=='F54_ORIGINAL_ROUTER_SEAL' and seal['admission'] is False,'router_seal_type')
    collection=load_collection(repo,seal['collection_path'],seal['collection_sha256'],map_path)
    require(sha(canonical(collection))==seal['complete_ledger_sha256'],'router_complete_ledger_identity')
    context=json.loads(Path(context_path).read_bytes())
    require(context==seal['context'] and context==policy_context(repo,map_path,sum(([k,v] for k,v in context['values'].items()),[])),'router_context_survives_fixtures_and_consumption')
    item=collection['required_qualification'].get(name)
    changed=(Path(context['values']['TMP_DIR'])/'changed-files').read_text().splitlines()
    require(name in changed or item is not None,'no_future_or_foreign_router_path_authority')
    return dict(path=name,manual_result_operations=collection.get('manual_result_operations',[]),seal_sha256=seal_sha,collection_prefix=collection['collection_prefix'],complete_ledger_sha256=seal['complete_ledger_sha256'],
        map_binding=collection['map_binding'],context_sha256=sha(canonical(context)),obligations=item,
        aggregate=item['aggregate'] if item else None,covered=item is not None,
        scope='changed' if name in changed else 'retained_history',
        terminal_dispositions=[v for v in collection['terminal_dispositions'] if v['path']==name],admission=False)


def append_router(path, kind, receipt):
    p=Path(path);previous=last_event(p);e=dict(schema='F54-ORIGINAL-ROUTER-OBSERVATION-v1',
        sequence=previous['sequence']+1 if previous else 1,previous=previous['digest'] if previous else '0'*64,
        kind=kind,payload=receipt)
    e['digest']=sha(canonical(e))
    with p.open('a',encoding='utf-8',newline='\n') as stream:stream.write(json.dumps(e,sort_keys=True)+'\n')


def original_router_body():
    source=(ROOT/'scripts/ci/voice-db-runtime-provisioning-contract_test.sh').read_text()
    start=source.index('while IFS= read -r file; do\n  f13_f54_router_begin')
    end=source.index('done <"${f13_original_path_list}"',start)
    return source[source.index('\n',start)+1:end]


@lru_cache(maxsize=2)
def original_policy_functions(source):
    # This fixed shell contains Python heredocs with standalone closing braces.
    # A regex's first brace is not necessarily the Bash function terminator.
    prefixes=('f13_','native_p3_','game_checkpoint_','voice_room_grants_','be255_','r23_','t31_',
        'space_media_','identity_publisher_','voice_role_status_','account_voice_admission_')
    functions=[];covered=0
    for match in re.finditer(r'(?m)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{\n',source):
        if match.start()<covered:continue
        symbol=match.group(1)
        if not (symbol.startswith(prefixes) or symbol in ('fail','changed_content','forbidden_activation_content')):continue
        for end in re.finditer(r'(?m)^\}\n',source[match.end():]):
            stop=match.end()+end.end();candidate=source[match.start():stop]
            if subprocess.run(['bash','-n'],input=candidate.encode(),capture_output=True).returncode==0:
                functions.append(candidate);covered=stop;break
        else:raise Rejected('actual_original_policy_function_boundary:'+symbol)
    return '\n'.join(functions)


def execute_original_router(context, binding):
    """Later read-only qualification recheck, never policy before mutation.

    Reexecute the exact frozen loop body and real predicates, with the same
    complete map/context and verified aggregate. Hooks only capture its branch.
    No fake accepting native/Game predicate or selected owner map is supplied.
    """
    source=(ROOT/'scripts/ci/voice-db-runtime-provisioning-contract_test.sh').read_text()
    functions=original_policy_functions(source)
    runtime=(ROOT/'scripts/ci/voice-r22-runtime-scope.sh').read_text()
    functions+='\n'+runtime
    with tempfile.TemporaryDirectory(prefix='f54-router-recheck-') as folder:
        capture=Path(folder)/'branch.jsonl'
        # Variable names were checked by policy_context; values remain argv,
        # never shell code. Exact ordinary policy bodies above are unchanged.
        script='set -euo pipefail\n'+functions+'\n'
        script+='while (( $# )); do printf -v "$1" "%s" "$2"; shift 2; done\n'
        script+='f13_f54_collection_file=\'\'\nfailures=0\n'
        script+='f13_f54_router_begin() { :; }\n'
        script+='f13_f54_router_gate() { python3 -c \'import json,sys;open(sys.argv[1],"a").write(json.dumps(sys.argv[2:])+"\\n")\' "$F54_CAPTURE" "$@"; }\n'
        if binding['covered']:
            script+='f13_runtime_for_path() { printf "%s\\n" "$F54_AGGREGATE"; }\n'
        if any(binding['path'] in value['receipt']['spec']['paths'] for value in binding.get('manual_result_operations',[])):
            delta=Path(folder)/'verified-manual-result-delta';delta.write_text(manual_delta_for_binding(binding))
            script+='f13_delta_for_path() { printf "%s\\n" '+shlex.quote(str(delta))+'; }\n'
        historical=retained_row_for_binding(binding)
        if historical is not None:
            # Recheck the same already verified data projection; predicates
            # and their ordering remain the original router's actual bodies.
            script+='f13_retained_history_row() { printf "%s\\n" '+shlex.quote(historical)+'; }\n'
        script+='for file in "$F54_PATH"; do\n'+original_router_body()+'done\n'
        pairs=sum(([k,v] for k,v in context['values'].items()),[])
        environment=dict(os.environ,**context['event'],F54_CAPTURE=str(capture),F54_PATH=binding['path'],
            F54_AGGREGATE='true' if binding['aggregate'] else 'false')
        executable=Path(folder)/'original-router.sh';executable.write_text(script)
        p=subprocess.run(['bash',str(executable),*pairs],capture_output=True,env=environment)
        require(p.returncode==0 and capture.is_file(),'actual_original_router_recheck_execution:'+p.stderr.decode()[-1200:])
        events=[json.loads(x) for x in capture.read_bytes().splitlines()]
        require(len(events)==1,'one_actual_original_router_branch')
        gate,status,*arguments=events[0]
        return dict(gate=gate,status=int(status),arguments=arguments,
            argument_files={x:file_binding(x) for x in arguments if Path(x).is_file()},
            stdout=raw_binding(p.stdout),stderr=raw_binding(p.stderr))


def validate_router(repo, seal_path, seal_sha, map_path, context_path, sidecar_path, sidecar_sha):
    raw=Path(sidecar_path).read_bytes();require(sha(raw)==sidecar_sha,'actual_router_sidecar_hash')
    seal=json.loads(Path(seal_path).read_bytes())
    require(seal==seal_router(repo,seal['collection_path'],seal['collection_sha256'],map_path,context_path),'complete_router_seal_reconstructed_not_hash_only')
    require(sha(Path(seal_path).read_bytes())==seal_sha,'actual_router_seal_raw_hash')
    previous='0'*64;pending=None;seen=set();receipts=[]
    for sequence,e in enumerate([json.loads(v) for v in raw.splitlines()],1):
        value=dict(e);digest=value.pop('digest')
        require(e['schema']=='F54-ORIGINAL-ROUTER-OBSERVATION-v1' and e['sequence']==sequence and e['previous']==previous and sha(canonical(value))==digest,'router_append_only_chain');previous=digest
        if e['kind']=='before':
            require(pending is None,'router_prior_path_has_actual_disposition')
            v=e['payload'];name=v['path'];require(name not in seen,'router_no_reordered_duplicate_path')
            expected=router_binding(repo,seal_path,seal_sha,map_path,context_path,name)
            require(v==expected,'router_consumes_full_provenance_before_early_continue')
            pending=v;seen.add(name)
        elif e['kind']=='gate':
            v=e['payload'];require(pending is not None and v['binding']==pending,'real_router_gate_bound_to_same_ledger')
            actual=execute_original_router(seal['context'],pending)
            require(all(v[k]==actual[k] for k in ('gate','status','arguments','argument_files')),'actual_original_ordered_policy_branch_and_arguments')
            receipts.append(dict(path=pending['path'],aggregate=pending['aggregate'],gate=actual['gate'],status=actual['status'],
                source_context=seal['context']['source_context'],binding=sha(canonical(pending))))
            pending=None
        else:raise Rejected('router_observation_kind')
    require(pending is None,'router_final_path_disposition_required')
    changed=Path(seal['context']['values']['TMP_DIR'])/'changed-files'
    expected=changed.read_text().splitlines()
    collection=load_collection(repo,seal['collection_path'],seal['collection_sha256'],map_path)
    missing=set(collection['required_qualification'])-set(expected)
    require([v['path'] for v in receipts]==expected+sorted(missing),'all_changed_and_retained_history_obligations_actually_qualified')
    # Extra receipts are explicitly history qualifications through the same
    # original policy, not manufactured endpoint changes or outer owner rows.
    retained={name:collection['required_qualification'][name] for name in sorted(missing)}
    require(all(v['retained_evidence'] or v['span_evidence'] for v in retained.values()),'reverted_paths_keep_real_history_dispositions')
    return dict(type='F54_ORIGINAL_ROUTER_RECHECK',collection_prefix=collection['collection_prefix'],
        sidecar_sha256=sidecar_sha,receipts=receipts,retained_history_obligations=retained,
        failures=[v for v in receipts if v['status']!=0],qualification='HOLD',admission=False,runtime_pass=False)



# Exact historical merge-authored residual; this is not current runtime authority.
DC0_MERGE = 'dc0acdefec7e5c413988adf71f86d12bff4017bc'
DC0_PATH = 'src/backend/chat/internal/gisprincipal/method_union_test.go'
DC0_SOURCE_UNIT_SHA = 'ef290952b4e7cd66321a8019485e7a8ed70233a82a33d82166e24da3ae8f950d'
DC0_SOURCE_UNIT_RAW = __import__('zlib').decompress(__import__('base64').b64decode(
    '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'))


def dc0_residual_source(repo):
    require(sha(DC0_SOURCE_UNIT_RAW)==DC0_SOURCE_UNIT_SHA,'dc0_source_unit_raw')
    unit=json.loads(DC0_SOURCE_UNIT_RAW)
    parents=git(repo,'rev-list','--parents','-n','1',DC0_MERGE).decode().split()
    require(parents==[DC0_MERGE,*unit['ordered_parents']],'dc0_ordered_parents')
    require(git(repo,'merge-base','--all',*parents[1:]).decode().splitlines()==unit['bases'],'dc0_actual_bases')
    for version in unit['versions']:
        require(git(repo,'rev-parse',version['commit']+'^{tree}').decode().strip()==version['tree'],'dc0_complete_tree_identity')
    for edge in unit['deltas']:
        before,after=edge['before'],edge['after']
        require(git(repo,'diff','--name-only',before,after).decode().splitlines()==edge['paths'],'dc0_complete_edge_paths')
        require(file_digest(git(repo,'diff','--raw','--no-abbrev','--no-renames',before,after))==edge['raw'],'dc0_complete_edge_raw')
        require(file_digest(git(repo,'diff','--binary','--no-ext-diff','--no-renames','--abbrev=9',before,after))==edge['patch'],'dc0_complete_edge_patch')
    paths=sorted(set(unit['deltas'][2]['paths'])-set(unit['deltas'][1]['paths']))
    require(paths==unit['residual']==[DC0_PATH],'dc0_exact_singleton_residual')
    for ref in [*unit['bases'],*unit['ordered_parents']]:
        require(tree_entry(repo,ref,DC0_PATH)=='','dc0_absent_from_both_parents_and_base')
    entry=entry_binding(repo,DC0_MERGE,DC0_PATH)
    expected=unit['versions'][-1]['entries'][DC0_PATH]
    require(entry['tree_record']==raw_binding(expected['tree_record']) and entry['blob']==expected['content'],'dc0_exact_residual_entry')
    return dict(type='F54_EXACT_DC0_MERGE_RESIDUAL',source_unit_sha256=DC0_SOURCE_UNIT_SHA,
        merge=DC0_MERGE,ordered_parents=unit['ordered_parents'],bases=unit['bases'],paths=paths,entry=entry,admission=False)


def master_frame_base(entry):
    if entry['type']=='F54_EXPLICIT_MASTER_PRODUCTION_HISTORY_ENTRY_v1':
        return entry['collection_base']
    return entry['base']


def validate_trace(repo, path, expected_sha, complete=False, checkpoint=None, range_checkpoint=None, I_checkpoint=None, deletion_checkpoint=None, manual_checkpoint=False, manual_end_checkpoint=None, manual_append_checkpoint=None, J_history_checkpoint=None, J_checkpoint=None, sealed_prefix=None, exact_J_checkpoint=None, finite_J_checkpoint=False, dc0_checkpoint=None, dc0_context=None, r23_132_checkpoint=False):
    global _TRACE_USED
    _TRACE_USED = True
    raw=Path(path).read_bytes()
    if sealed_prefix is not None:
        require(isinstance(sealed_prefix,dict) and set(sealed_prefix)=={'bytes','sha256'},'trace_sealed_prefix_shape')
        require(type(sealed_prefix['bytes']) is int and 0<sealed_prefix['bytes']<=len(raw)
            and sealed_prefix['sha256']==expected_sha,'trace_sealed_prefix_exact_binding')
        raw=raw[:sealed_prefix['bytes']]
    require(sha(raw)==expected_sha,'trace_raw_hash')
    events=[json.loads(v) for v in raw.splitlines()];require(events,'nonempty_producer_trace')
    previous='0'*64;frames={};stack=[];operations={};attempts={};classified={};pending={};cumulative={};child_flags={};child_rows={};frame_rows={};carry={};failures=[];observed=set()
    expected_operations={};seen_operations={};visited={};committed={};maps={};children={};obligations={};native_bindings={}
    M_capabilities={};M_compositions={};M_ready={};M_consumed={};master_entry=None;master_wrapper=None
    capabilities_132={};compositions_132={}
    J_prior_spans={};J_ready={};J_consumed={}
    J_capabilities={};J_compositions={};F_compositions={};range_branches={};frame_results={};I_ready={};I_consumed={}
    terminal={};final_gates={};manual_specs={};manual_gates={};manual_associations={};manual_closed={}
    dc0_specs={};dc0_closed={}
    def map_binding(frame):
        raw=''.join('\t'.join(parts)+'\n' for parts in maps[frame]).encode()
        return dict(bytes=len(raw),sha256=sha(raw))
    def map_before(frame, value):
        require(value['map_before']==map_binding(frame),'conserved_map_before')
        if 'prior_rows' in value:
            require([row(x) for x in value['prior_rows']]==[x for x in maps[frame] if x[0]==value['path']],'conserved_prior_rows')
    def completed_132_child(operation):
        cap=capabilities_132[operation];unit=cap['unit']
        require(children.get(operation)==[(unit['history_boundary'],unit['parents'][1],'true','false')],
                'exact132_complete_original_child_scope')
        matches=[name for name,data in frames.items() if data['parent_operation']==operation]
        require(len(matches)==1 and frame_results.get(matches[0],{}).get('exit')==0,
                'exact132_successful_child_before_import')
        child=matches[0]
        require(frame_results[child]['final_map']==map_binding(child),'exact132_complete_child_map')
        return child
    def r23_132_state(operation, name, prefix):
        frame=operation.split('/')[0];key=(frame,name)
        item=require_pending_operation(pending,key,operation)
        require(operation in capabilities_132 and stack and stack[-1]==frame and not failures
            and not item['removed'] and item['after'] is None and 'r23_132_owner_permission' not in item,
            'r23_132_live_pending_before_mutation')
        child=completed_132_child(operation)
        require((operation,name) in child_rows and (child,name) in obligations,'r23_132_complete_child_obligations')
        return dict(frame=frame,operation=operation,path=name,frame_context=frames[frame],
            capability=capabilities_132[operation],trace_prefix=prefix,map_before=map_binding(frame),
            prior_rows=['\t'.join(x) for x in maps[frame] if x[0]==name],incoming='\t'.join(item['incoming']),
            child_row='\t'.join(child_rows[(operation,name)]),child_frame=child,
            child_result=frame_results[child],child_map=map_binding(child),
            child_obligations=obligations[(child,name)],child_aggregate=cumulative[(child,name)],
            obligations=obligations[key],aggregate=cumulative[key])
    def finished(operation, allow_manual_pending=False):
        require(operation in operations,'complete_operation_begin')
        value=operations[operation]
        if value['classification']=='dc0_residual':
            require(operation in classified and visited.get(operation,[])==[DC0_PATH]
                and committed.get(operation,[])==[DC0_PATH],'dc0_complete_residual_operation')
            return
        if value['classification']=='manual_result':
            require(operation in classified and visited.get(operation,[])==value['paths'], 'manual_F_full33_actual_gates_and_classification')
            require(committed.get(operation,[])==F_MANUAL_NEW_PATHS, 'manual_F_exact_ten_real_commits')
            require(set(manual_associations.get(operation,{}))==set(value['paths'])-set(F_MANUAL_NEW_PATHS), 'manual_F_every_other_residual_disposition')
            return
        if value['classification']=='sync':
            require(operation in sync_gates,'actual_sync_gate_receipt')
            return
        require(operation in classified,'complete_operation_classification')
        require(visited.get(operation,[])==value['paths'],'complete_ordered_path_visits')
        require(committed.get(operation,[])==value['paths'],'complete_ordered_path_commits')
        if frames[operation.split('/')[0]]['partition_source_history']=='true' and value['classification']=='merge':
            require(children.get(operation)==[(value['boundary'],value['source_head'],'true','false')],'complete_real_child_frame')
        if value['commit']==DC0_MERGE and master_entry is not None and master_entry.get('type') in ('F54_EXPLICIT_MASTER_PRODUCTION_ENTRY_v1','F54_EXPLICIT_MASTER_PRODUCTION_HISTORY_ENTRY_v1'):
            manual=operation.rsplit('/',1)[0]+'/manual-result'
            require(manual in dc0_closed or allow_manual_pending,'dc0_residual_required_before_next_operation')
        if value['commit']==F_CORRECTED_TUPLE['merge']:
            manual=operation.rsplit('/',1)[0]+'/manual-result'
            if operation.split('/')[0] in range_branches:
                require(manual in operations or allow_manual_pending, 'manual_F_cannot_omit_whole_operation_before_H')
            if manual in operations:
                finished(manual);require(manual in manual_closed, 'manual_F_complete_before_next_spine_operation')
        if operation in M_capabilities:
            require(children.get(operation)==[(O,P2,'true','true')],'complete_exact_M_history_child')
            require(operation in M_ready and M_consumed.get(operation,[])==value['paths'],'complete_M_ledger_consumption_before_mutation')
        if operation in J_capabilities:
            require(children.get(operation)==[(J3071_O,J3071_P2,'true','true')],'complete_fixed3071_real_history_child')
            require(operation in J_ready and J_consumed.get(operation,[])==value['paths'],'fixed3071_complete_J_ledger_consumed_before_mutation')
        if operation in capabilities_132:
            completed_132_child(operation)
        range_children=[name for name,data in frames.items() if data['parent_operation']==operation and name in range_branches]
        if range_children:
            require(operation in I_ready and I_consumed.get(operation,[])==value['paths'],'fixed3071_I_complete_ledger_consumed_before_collapse')
    def dc0_state(phase, gate_context=None):
        require(phase in ('begin','append','end') and master_entry is not None
            and master_entry.get('type') in ('F54_EXPLICIT_MASTER_PRODUCTION_ENTRY_v1','F54_EXPLICIT_MASTER_PRODUCTION_HISTORY_ENTRY_v1')
            and stack and not failures,'dc0_actual_production_checkpoint')
        identity=stack[-1];context=frames[identity];original=identity+'/'+DC0_MERGE+'/2'
        require([context[k] for k in ('base','target','allow_direct_followup','partition_source_history')]
            ==['698d4f064f032ea74abf3ede2017100b26c8ee69','53f75f74523735ac594e21582bea659f567a7288','true','false'],
            'dc0_exact_historical_frame')
        require(seen_operations[identity] and seen_operations[identity][-1]==(DC0_MERGE,2),
            'dc0_immediately_after_original_parent_operation')
        parent=context['parent_operation']
        require(parent in operations and operations[parent]['commit']=='e6757e8ac26032ac4bad7f4356e91fffbf0725ba'
            and operations[parent]['source_head']==context['target'],'dc0_original_recursive_source_parent')
        finished(original,allow_manual_pending=True)
        source=dc0_residual_source(repo)
        actual=(''.join('\t'.join(x)+'\n' for x in maps[identity])).encode()
        manual=identity+'/'+DC0_MERGE+'/manual-result'
        if phase!='begin':gate_context=dc0_specs.get(manual,{}).get('gate_context')
        require(isinstance(gate_context,dict) and gate_context['source_context']==router_source_context(repo)
            and all(gate_context['event'].get(k,'')==value for k,value in master_entry['context'].items()),
            'dc0_actual_gate_source_context')
        validate_manual_authority(gate_context,manual_authority_expected(repo,gate_context))
        native=json.loads(master_entry['native_binding']['raw'])
        require(DC0_PATH not in native['paths'],'dc0_no_native_path_exemption')
        game_paths=gate_context['values'].get('game_checkpoint_aggregate_paths','')
        require(not game_paths or DC0_PATH not in gate_context['snapshots'][game_paths]['raw'].splitlines(),
            'dc0_no_game_aggregate_path_exemption')
        if phase=='begin':
            require(not pending and manual not in operations and not any(x[0]==DC0_PATH for x in maps[identity]),
                'dc0_new_unowned_residual_only')
            return dict(type='F54_DC0_RESIDUAL_SPEC',source=source,frame=identity,operation=manual,
                original_operation=original,original_binding_sha256=sha(canonical(operations[original])),
                original_classification=classified[original],map_before=file_digest(actual),
                source_context=router_source_context(repo),gate_context=gate_context,admission=False)
        require(manual in dc0_specs and manual not in dc0_closed,'dc0_open_residual_operation')
        require(dc0_specs[manual]['source']==source and dc0_specs[manual]['source_context']==router_source_context(repo),
            'dc0_fresh_source_before_mutation')
        if phase=='append':
            require(len(pending)==1 and (identity,DC0_PATH) in pending,'dc0_one_pending_residual')
            item=pending[(identity,DC0_PATH)]
            require(item['op']==manual and item['prior'] is None and not item['removed'] and item['after'] is None,
                'dc0_append_only_no_prior_owner')
            return dict(operation=manual,map_before=file_digest(actual),admission=False)
        require(not pending,'dc0_no_pending_at_end');finished(manual)
        return dict(spec=dc0_specs[manual],classification=classified[manual],map_after=file_digest(actual),
            row=next('\t'.join(x) for x in maps[identity] if x[0]==DC0_PATH),admission=False)
    def ledger_state():
        return {f:dict(map_binding=map_binding(f),rows=['\t'.join(x) for x in maps[f]],paths={
            name:dict(aggregate=cumulative[(f,name)],evidence=obligations[(f,name)])
            for owner,name in obligations if owner==f},terminal_dispositions=terminal.get(f,{}),
            manual_result_operations={op:v for op,v in manual_closed.items() if op.split('/')[0]==f}) for f in frames}
    def deletion_state(identity,prefix):
        require(stack and stack[-1]==identity and not pending and identity in final_gates,
            'reviewed_F_deletion_live_final_guard_not_old_frame')
        require(identity in range_branches,'reviewed_F_deletion_actual_qualified_nine_node_range')
        gate=final_gates[identity];name=F_DELETION_PATH
        require(gate['kind']=='reviewed_F_deletion_guard' and gate['payload']['path']==name
            and gate['operation'] is None,'reviewed_F_deletion_exact_final_path_phase')
        candidates=[op for op,v in operations.items() if op.startswith(identity+'/') and v['commit']==F_CORRECTED_TUPLE['merge'] and v['classification']!='manual_result']
        require(len(candidates)==1,'reviewed_F_deletion_one_real_F_operation');operation=candidates[0]
        finished(operation)
        require(seen_operations[identity]==expected_operations[identity] and not failures,
            'reviewed_F_deletion_all_spine_operations_completed_without_failure')
        for commit,index in seen_operations[identity]:finished(identity+'/'+commit+'/'+str(index))
        prior=[raw_binding('\t'.join(x)) for x in maps[identity] if x[0]==name]
        return dict(frame_context=frames[identity],F_operation=operation,operation_binding=operations[operation],
            classification=classified[operation],composition_receipts=list(F_compositions.get(operation,{}).values()),
            prior_rows=prior,path_evidence=obligations.get((identity,name),[]),aggregate=cumulative.get((identity,name)),
            map_binding=map_binding(identity),prefix=prefix,final_guard_digest=gate['digest'],
            original_guard=gate['payload'],
            range_branch_sha256=sha(canonical(range_branches[identity])),
            frame_ancestry=[dict(frame=f,context=frames[f]) for f in stack],source_context=router_source_context(repo))
    def M_checkpoint_state(operation,prefix_events):
        require(operation in M_capabilities and operation in classified,'typed_classified_M_checkpoint')
        frame=operation.split('/')[0];value=operations[operation]
        require(stack and stack[-1]==frame and seen_operations[frame][-1]==(M,2),'exact_live_M_checkpoint')
        if len(stack)>1:
            require(master_entry is not None and frames[stack[0]]['base']==master_frame_base(master_entry) and frames[stack[0]]['target']==master_entry['source'], 'M_real_master_ancestor_context')
            for child in stack[1:]:
                parent_op=frames[child]['parent_operation'];attempt=attempts[parent_op]
                require(frames[child]['parent']==parent_op.split('/')[0] and frames[child]['target']==attempt['source_head'], 'M_actual_recursive_ancestor_operation')
        else:require(master_entry is None,'master_entry_cannot_relabel_unit_trace')
        require(visited.get(operation,[])==value['paths'] and not committed.get(operation) and not pending,'M_checkpoint_before_any_row')
        require(children.get(operation)==[(O,P2,'true','true')],'complete_M_child_before_rows')
        for commit,index in seen_operations[frame][:-1]:finished(frame+'/'+commit+'/'+str(index))
        child=[name for name,data in frames.items() if data['parent_operation']==operation]
        require(len(child)==1,'one_exact_real_M_history_child')
        return dict(type='F54_CLASSIFIED_M_PREFIX_CHECKPOINT',operation=operation,frame=frame,
            capability_sha256=sha(canonical(M_capabilities[operation])),operation_binding=value,
            classification=classified[operation],paths=value['paths'],ledger=ledger_state(),child_frame=child[0],
            coverage=dag_dispositions(repo,prefix_events),frame_ancestry=[dict(frame=name,context=frames[name]) for name in stack],M_rows_pending=True,admission=False)
    def range_checkpoint_state(identity):
        r=J3071_RANGE
        require(stack and stack[-1]==identity and not seen_operations[identity] and not maps[identity] and not pending and not failures,'fixed3071_exact_empty_range_engine_boundary')
        value=frames[identity]
        require([value[k] for k in ('base','target','allow_direct_followup','partition_source_history')]==[r['base'],r['target'],'true','false'],'fixed3071_exact_nested_range_frame')
        parent=value['parent_operation'];require(parent in operations,'fixed3071_real_I_before_range')
        integration=operations[parent]
        require([integration[k] for k in ('commit','parent_index','first_parent','boundary','source_head','candidates')]==[r['integration'],2,r['first'],r['base'],r['target'],[r['base']]],'fixed3071_actual_I_source_slot')
        main=frames[value['parent']]
        require([main[k] for k in ('base','target','allow_direct_followup','partition_source_history')]==[J3071_O,J3071_P2,'true','true'],'fixed3071_real_required_main_child')
        owner=main['parent_operation'];require(owner in J_capabilities and owner in operations,'fixed3071_range_belongs_to_actual_J')
        require(master_entry is not None and len(stack)>=3,'fixed3071_range_original_master_ancestor_context')
        return dict(type='F54_FIXED3071_PENDING_RANGE_CHECKPOINT',frame=identity,integration_operation=parent,
            integration_binding=integration,J_operation=owner,capability_sha256=sha(canonical(J_capabilities[owner])),
            frame_ancestry=[dict(frame=name,context=frames[name]) for name in stack],
            retained_maps={name:map_binding(name) for name in maps},admission=False)
    def J_prefix_state(operation, classified_required=False):
        require(operation in J_capabilities and operation in operations,'fixed3071_J_actual_boundary_operation')
        frame=operation.split('/')[0];value=operations[operation]
        require(stack and stack[-1]==frame and seen_operations[frame][-1]==(J3071,2),'fixed3071_J_live_original_frame')
        require([value[k] for k in ('commit','first_parent','boundary','source_head','candidates')]==[J3071,J3071_P1,J3071_O,J3071_P2,J3071_BASES], 'fixed3071_J_exact_original_tuple')
        require(not committed.get(operation) and not pending and not failures,'fixed3071_J_before_any_mutation')
        require((operation in classified)==classified_required,'fixed3071_J_exact_classifier_phase')
        if classified_required:require(visited.get(operation,[])==value['paths'],'fixed3071_J_all_original_pregates_before_rows')
        else:require(not visited.get(operation),'fixed3071_J_history_before_original_pregates')
        require(master_entry is not None and len(stack)>=2,'fixed3071_J_original_master_not_relabelled_unit')
        require(children.get(operation)==[(J3071_O,J3071_P2,'true','true')],'fixed3071_J_genuine_complete_source_child')
        child=[f for f,v in frames.items() if v['parent_operation']==operation]
        require(len(child)==1 and frame_results.get(child[0],{}).get('exit')==0,'fixed3071_J_successful_child_not_boolean')
        boundary=[e for e in events if e['kind']=='boundary_3071_observation' and e['operation']==operation]
        require(len(boundary)==1 and boundary[0]['payload']['prefix']==raw_binding((''.join('\t'.join(x)+'\n' for x in maps[frame])).encode()), 'fixed3071_J_original_outer_prefix_unmodified')
        prior={}
        for commit,index in seen_operations[frame][:-1]:
            identity=frame+'/'+commit+'/'+str(index);finished(identity);prior[identity]=operations[identity]
        return dict(frame=frame,frame_context=frames[frame],operation=operation,operation_binding=value,
            capability_sha256=sha(canonical(J_capabilities[operation])),ledger=ledger_state(),child_frame=child[0],
            frame_ancestry=[dict(frame=f,context=frames[f]) for f in stack],prior_operations=prior,
            prior_classifications={k:classified[k] for k in prior if k in classified},
            prior_committed_rows={k:[e['payload']['row'] for e in events if e['kind']=='row_committed' and e['operation']==k] for k in prior},admission=False)
    def J_checkpoint_state(operation, prefix_events):
        state=J_prefix_state(operation,True)
        require(operation in J_prior_spans,'fixed3071_J_requires_actual_prior_span_not_inventory')
        state.update(classification=classified[operation],prior_span=J_prior_spans[operation],M_ready=M_ready,I_ready=I_ready,
            coverage=J_history_coverage(repo,prefix_events,M_ready,range_branches,J_prior_spans[operation]),
            composition=dict(whole=manual_3071_composition(repo),consumed_path_receipts=J_compositions.get(operation,{})))
        return state
    def I_checkpoint_state(operation):
        r=J3071_RANGE;require(operation in operations and operation in classified,'fixed3071_I_actual_classified_checkpoint')
        frame=operation.split('/')[0];value=operations[operation]
        require(stack and stack[-1]==frame and seen_operations[frame][-1]==(r['integration'],2),'fixed3071_I_live_original_operation')
        require([value[k] for k in ('commit','first_parent','boundary','source_head','candidates')]==[r['integration'],r['first'],r['base'],r['target'],[r['base']]],'fixed3071_I_keeps_original_parents_and_source')
        owner=frames[frame]['parent_operation'];require(owner in J_capabilities,'fixed3071_I_actual_J_child_only')
        require(visited.get(operation,[])==value['paths'] and not committed.get(operation) and not pending and not failures,'fixed3071_I_all_original_pregates_before_rows')
        require(children.get(operation)==[(r['base'],r['target'],'true','false')],'fixed3071_I_exact_required_range_child')
        candidates=[name for name,data in frames.items() if data['parent_operation']==operation]
        require(len(candidates)==1 and candidates[0] in range_branches,'fixed3071_I_one_typed_same_state_range')
        child=candidates[0];require(child in frame_results and frame_results[child]['exit']==0,'fixed3071_I_actual_range_engine_completed')
        branch=range_branches[child]
        spine=[dict(node=node,operation=child+'/'+node+'/'+str(index),disposition='actual_same_state_spine',
            begin=next(e['digest'] for e in events if e['kind']=='operation_begin' and e['operation']==child+'/'+node+'/'+str(index))) for node,index in seen_operations[child]]
        side=[dict(node=node,operation=ref['operation'],disposition='actual_supplemental_side_operation',
            trace_sha256=branch['side_trace_sha256'],begin=ref['begin_digest']) for node,ref in zip(r['side_nodes'],branch['side_operations'])]
        coverage=spine+side
        require(len(coverage)==9 and len({x['node'] for x in coverage})==9 and {x['node'] for x in coverage}==set(branch['origin']['witness']['nodes']),'fixed3071_I_complete_nine_actual_operation_dispositions')
        require(M_ready,'fixed3071_I_retains_actual_existing_M_span_evidence')
        return dict(type='F54_FIXED3071_CLASSIFIED_I_CHECKPOINT',operation=operation,frame=frame,operation_binding=value,
            classification=classified[operation],capability_sha256=sha(canonical(J_capabilities[owner])),
            frame_ancestry=[dict(frame=name,context=frames[name]) for name in stack],ledger=ledger_state(),
            range_branch=branch,coverage=coverage,M_ready=M_ready,admission=False)
    def manual_end_state(operation):
        require(stack and stack[-1]==operation.split('/')[0] and not pending and not failures, 'manual_F_before_end_live_state')
        finished(operation);frame=operation.split('/')[0]
        return dict(spec=manual_specs[operation],classification=classified[operation],gates=manual_gates[operation],
            dispositions={name:dict(type=manual_specs[operation]['dispositions'][name]['type'],
                evidence=obligations[(frame,name)][-1]) for name in manual_specs[operation]['paths']},map_after=map_binding(frame),admission=False)
    def manual_checkpoint_state():
        require(stack and not pending and not failures, 'manual_F_live_completed_parent_boundary')
        frame=stack[-1];parent=frame+'/'+F_CORRECTED_TUPLE['merge']+'/2'
        require(frame in range_branches and seen_operations[frame][-1]==(F_CORRECTED_TUPLE['merge'],2), 'manual_F_exact_range_after_real_F_parent_loop')
        require(parent in operations and parent in classified and parent.rsplit('/',1)[0]+'/manual-result' not in operations, 'manual_F_one_actual_occurrence')
        finished(parent,allow_manual_pending=True)
        return dict(frame_context=frames[frame],map_before=map_binding(frame))
    sync_gates=set()
    for sequence,e in enumerate(events,1):
        require(set(e)=={'schema','sequence','previous','kind','frame','operation','payload','digest'} and e['schema']==SCHEMA,'producer_event_schema')
        value=dict(e);digest=value.pop('digest')
        require(e['sequence']==sequence and e['previous']==previous and sha(canonical(value))==digest,'append_only_chain');previous=digest
        kind=e['kind'];v=e['payload'];frame=e['frame'];op=e['operation']
        if kind=='master_entry':
            require(sequence==1 and not frames and frame is None and op is None and master_entry is None,'one_explicit_master_origin_before_frames')
            capability=json.loads(v['capability']['raw'])
            require(v['capability']==raw_binding(canonical(capability)) and capability==checked_capability_value(repo,capability),'master_complete_source_capability')
            require(v==master_context(repo,capability,v['native_binding']['raw'].encode(),v['context']),'independent_actual_master_origin')
            master_entry=v;continue
        if kind=='master_wrapper_result':
            require(master_entry is not None and master_wrapper is None and not stack and not failures and frame is None and op is None,'original_wrapper_result_after_successful_outer_frame')
            require(v['type']=='F54_ACTUAL_MASTER_WRAPPER_RESULT' and v['entry_sha256']==sha(canonical(master_entry)) and v['admission'] is False,'actual_wrapper_same_entry')
            root=next(iter(frames));require(v['source_map']==v['output']==map_binding(root),'original_wrapper_full_root_map_parity')
            require(Path(v['source_map_path']).read_bytes()==Path(v['output_path']).read_bytes(),'actual_wrapper_output_copy_not_selected_map')
            master_wrapper=v;continue
        if kind=='J_prior_span_link':
            require(sequence==1 and not frames and frame is None and op is None,'fixed3071_prior_separate_real_frame')
            verify_J_prior_origin(repo,v);continue
        if kind=='range_side_link':
            require(sequence==1 and not frames and frame is None and op is None and v['admission'] is False,'separate_fixed3071_side_link_before_real_execution')
            require(v['type']=='F54_FIXED3071_RANGE_AUTHOR_ORIGIN' and v['source_context']==router_source_context(repo),'fixed3071_side_actual_source_context')
            r=J3071_RANGE
            require(v['witness']==fixed3071_range_structure(repo,r['base'],r['target'],'true','false') and v['ruling_sha256']==J3071_RANGE_RULING_SHA,'fixed3071_side_exact_nine_node_witness')
            continue
        if kind=='span_link':
            require(sequence==1 and not frames and frame is None and op is None and v['admission'] is False,'separate_span_link_before_execution')
            require(v['supplemental_classifier']==classifier_binding(),'supplemental_collector_context')
            continue
        if kind=='frame_enter':
            identity=v['id'];require(frame==identity and identity not in frames and v['parent']==(stack[-1] if stack else None),'frame_parent_and_order')
            require((not v['parent'] and v['parent_operation'] is None) or v['parent_operation'] in attempts,'actual_parent_operation_reference')
            actual=git(repo,'rev-list','--first-parent','--reverse',v['base']+'..'+v['target']).decode().splitlines()
            require(v['first_parent_operations']==actual,'actual_frame_operations');frames[identity]=v;stack.append(identity)
            if len(frames)==1 and master_entry is not None:
                require([v[k] for k in ('base','target','allow_direct_followup','partition_source_history')]==[master_frame_base(master_entry),master_entry['source'],'true','true'],'genuine_original_master_source_frame')
            maps[identity]=[];seen_operations[identity]=[];expected_operations[identity]=[]
            for commit in actual:
                parents=git(repo,'rev-list','--parents','-n','1',commit).decode().split()[1:]
                expected_operations[identity].extend((commit,index) for index in ([1] if len(parents)==1 else range(2,len(parents)+1)))
            if v['parent_operation']:
                children.setdefault(v['parent_operation'],[]).append((v['base'],v['target'],v['allow_direct_followup'],v['partition_source_history']))
            continue
        require(stack and frame==stack[-1],'current_live_frame')
        if kind=='range_3071_branch':
            require(frame not in range_branches and op is None,'fixed3071_range_branch_before_same_state_operations')
            state=range_checkpoint_state(frame);capability=J_capabilities[state['J_operation']]
            prefix=b''.join(raw.splitlines(keepends=True)[:sequence-1])
            origin=range_3071_origin_value(repo,capability,state,path,dict(bytes=len(prefix),sha256=sha(prefix)))
            origin['capability_path']=v['origin']['capability_path']
            require(verified_3071_capabilities(repo,origin['capability_path'])==capability,'fixed3071_range_same_actual_capability_file')
            require(v['origin']==origin,'fixed3071_actual_range_origin_not_foreign_frame_or_old_ledger')
            require(v==verify_3071_range_side(repo,origin,v['side_trace'],v['side_map']),'fixed3071_actual_side_receipt_before_persistent_engine')
            range_branches[frame]=v;continue
        if kind=='frame_exit':
            require(v['final_map']==map_binding(frame),'conserved_final_map')
            r=J3071_RANGE;current=frames[frame]
            if v['exit']==0 and current['target']==F_CORRECTED_TUPLE['correction'] and any(x[0]==F_DELETION_PATH for x in maps[frame]):
                require(F_DELETION_PATH in terminal.get(frame,{}),
                    'reviewed_F_deletion_success_requires_actual_terminal_disposition')
            if v['exit']==0 and [current[k] for k in ('base','target')]==[r['base'],r['target']]:
                require(frame in range_branches,'fixed3071_nonfirstparent_success_requires_actual_typed_branch')
                require([commit for commit,index in seen_operations[frame]]==[r['direct'],r['merge'],r['target']],'fixed3071_persistent_three_operation_spine')
            seen=seen_operations[frame]
            require(seen==expected_operations[frame][:len(seen)],'ordered_actual_operation_prefix')
            if v['exit']==0:require(seen==expected_operations[frame],'complete_actual_operations')
            for commit,index in (seen if v['exit']==0 else seen[:-1]):finished(frame+'/'+commit+'/'+str(index))
            require(v['exit']!=0 or not any(key[0]==frame for key in pending),'row_without_commit')
            if v['exit']!=0: failures.append(dict(frame=frame,category=v['category']))
            frame_results[frame]=v
            parent_operation=frames[frame]['parent_operation']
            if parent_operation and v['exit']==0:
                for (owner,name),flag in cumulative.items():
                    if owner==frame:child_flags[(parent_operation,name)]=child_flags.get((parent_operation,name),False) or flag
                if frames[parent_operation.split('/')[0]]['partition_source_history']=='true':
                    for (owner,name),parts in frame_rows.items():
                        if owner==frame:child_rows[(parent_operation,name)]=parts
            pending={key:value for key,value in pending.items() if key[0]!=frame}
            stack.pop();continue
        if kind=='operation_attempt':
            require(v['id']==op and op not in attempts and v['commit'] in frames[frame]['first_parent_operations'],'unique_actual_attempt')
            parents=git(repo,'rev-list','--parents','-n','1',v['commit']).decode().split()[1:]
            require(v['ordered_parents']==parents and v['first_parent']==parents[0],'actual_attempt_parents')
            index=v['parent_index'];require(type(index) is int and 1<=index<=len(parents),'actual_attempt_index')
            require(op==frame+'/'+v['commit']+'/'+str(index),'actual_attempt_frame_identity')
            expected_class='direct' if len(parents)==1 else 'sync' if len(parents)==2 and parents[1]==frames[frame]['base'] else 'merge'
            require(v['classification']==expected_class,'actual_attempt_classification')
            require(v['source_head']==(v['commit'] if len(parents)==1 else parents[index-1]),'actual_attempt_source')
            require(v['candidates']==git(repo,'merge-base','--all',v['first_parent'],v['source_head']).decode().splitlines(),'actual_attempt_bases')
            seen=seen_operations[frame]
            if seen:finished(frame+'/'+seen[-1][0]+'/'+str(seen[-1][1]))
            require(len(seen)<len(expected_operations[frame]) and (v['commit'],index)==expected_operations[frame][len(seen)],'ordered_actual_operation_prefix')
            seen.append((v['commit'],index));attempts[op]=v;continue
        if kind=='history_132_capability':
            require(op in attempts and op not in operations and op not in capabilities_132,
                    'exact132_capability_after_unique_actual_attempt')
            require(master_entry is not None and master_entry.get('type') in ('F54_EXPLICIT_MASTER_PRODUCTION_ENTRY_v1','F54_EXPLICIT_MASTER_PRODUCTION_HISTORY_ENTRY_v1'),
                    'exact132_actual_production_master_entry')
            actual=frames[frame];attempt=attempts[op]
            require(frame==next(iter(frames)) and [actual[k] for k in ('base','target','allow_direct_followup','partition_source_history')]
                ==[master_frame_base(master_entry),master_entry['source'],'true','true'],'exact132_actual_outer_frame')
            expected=boundary_132_capability(repo,attempt['commit'],attempt['first_parent'],attempt['source_head'],actual['base'],actual['target'])
            require(v==expected and op==frame+'/'+MERGE_132+'/2','exact132_independent_complete_capability')
            capabilities_132[op]=v;continue
        if kind=='history_3071_capability':
            require(op in attempts and op not in operations and op not in J_capabilities,'fixed3071_capability_after_actual_attempt')
            binding=v['capability'];require(binding==raw_binding(binding['raw']) and v['admission'] is False,'fixed3071_capability_raw_binding')
            capability=json.loads(binding['raw'])
            require(capability==checked_3071_capability_value(repo,capability),'fixed3071_actual_complete_structural_capability')
            actual_frame=frames[frame];scope=capability['caller']
            require(all(actual_frame[k]==scope[k] for k in ('base','target','allow_direct_followup','partition_source_history')),'fixed3071_actual_original_false_partition_context')
            require(op==frame+'/'+J3071+'/2' and v['receipt']==dict(capability['tuple'],capability_sha256=sha(canonical(capability))),'fixed3071_exact_boundary_receipt')
            require(master_entry is not None,'fixed3071_only_original_master_author_entry')
            J_capabilities[op]=capability;continue
        if kind=='history_capability':
            require(op in attempts and op not in operations and op not in M_capabilities,'history_capability_after_actual_attempt')
            binding=v['capability'];require(binding==raw_binding(binding['raw']) and v['admission'] is False,'history_capability_raw_binding')
            capability=json.loads(binding['raw'])
            require(capability==checked_capability_value(repo,capability),'actual_complete_history_capability')
            scope=capability['collector_scope'];actual_frame=frames[frame]
            require([actual_frame[k] for k in ('base','target','allow_direct_followup','partition_source_history')]==[scope[k] for k in ('base','target','allow_direct_followup','partition_source_history')],'exact_capability_outer_context')
            require(op==frame+'/'+M+'/2' and v['receipt']==capability['history_boundary'],'exact_typed_M_boundary_receipt')
            M_capabilities[op]=capability;continue
        if kind=='dc0_residual_begin':
            expected=dc0_state('begin',v['spec']['gate_context'])
            require(v['spec']==expected and op==expected['operation']==v['id']
                and v['map_before']==map_binding(frame),'dc0_actual_source_bound_begin')
            source=expected['source'];first=source['ordered_parents'][0]
            dc0_specs[op]=expected
            operations[op]=dict(id=op,commit=DC0_MERGE,first_parent=first,source_head=DC0_MERGE,
                boundary=first,candidates=source['bases'],classification='dc0_residual',paths=[DC0_PATH],
                delta=raw_binding(DC0_PATH+'\n'),entries={DC0_PATH:dict(source=source['entry'],result=source['entry'])})
            continue
        if kind=='dc0_residual_end':
            expected=dc0_state('end')
            require(op==expected['spec']['operation'] and v['receipt']==expected
                and v['map_after']==map_binding(frame),'dc0_exact_closed_residual')
            dc0_closed[op]=expected;continue
        if kind=='manual_F_begin':
            state=manual_checkpoint_state();spec=v['spec']
            actual=(''.join('\t'.join(x)+'\n' for x in maps[frame])).encode()
            require(spec==manual_F_spec(repo,spec['capability'],state['frame_context'],actual,spec['gate_context'])
                and op==frame+'/'+F_CORRECTED_TUPLE['merge']+'/manual-result' and op==v['id']
                and v['map_before']==map_binding(frame), 'manual_F_actual_independent_result_operation')
            require(master_entry is not None and spec['gate_context']['values'].get('native_p3_active')=='true', 'manual_F_same_actual_native_context_not_disabled')
            binding_path=spec['gate_context']['values']['native_p3_binding']
            require(spec['gate_context']['snapshots'][binding_path]==master_entry['native_binding'], 'manual_F_original_native_binding_not_swapped')
            manual_specs[op]=spec
            operations[op]=dict(id=op,commit=spec['merge'],first_parent=F_CORRECTED_TUPLE['first'],
                source_head=spec['merge'],boundary=F_CORRECTED_TUPLE['first'],candidates=[F_CORRECTED_TUPLE['first']],
                classification='manual_result',paths=spec['paths'],delta=spec['delta'],entries=spec['dispositions'])
            continue
        if kind=='manual_F_gate':
            require(op in manual_specs and op not in classified and v['path'] in operations[op]['paths'], 'manual_F_gate_before_raw_classifier')
            map_before(frame,v);name=v['path']
            require(name not in manual_gates.setdefault(op,{}) and v['original_composition_status'] in (0,1,2)
                and v['qualified_composition']=='REVIEWED_CORRECTED_F_WHOLE'
                and v['native_owner_status']==v['game_owner_status']==0 and v['classifier']==classifier_binding()
                and v['eligibility'] in ('unapproved','game_aggregate','game_snapshot','checkpoint_blob'), 'manual_F_real_original_gate_outcomes')
            expected=replay_manual_F_gate(str(repo),canonical(manual_specs[op]).decode(),name)
            require(all(v[k]==value for k,value in expected.items()), 'manual_F_independent_actual_gates_and_eligibility')
            manual_gates[op][name]=v;visited.setdefault(op,[]).append(name);continue
        if kind=='manual_F_association':
            require(op in manual_specs and op in classified and v['path'] not in F_MANUAL_NEW_PATHS, 'manual_F_classified_association_not_new_row')
            map_before(frame,v);name=v['path'];spec=manual_specs[op]
            require(name in spec['paths'] and v['disposition']==spec['dispositions'][name]['type']
                and name not in manual_associations.setdefault(op,{}), 'manual_F_exact_disjoint_retained_association')
            prior=[raw_binding('\t'.join(x)) for x in maps[frame] if x[0]==name]
            require(prior==spec['dispositions'][name]['prior_rows'], 'manual_F_prior_rows_byte_exact_no_rowdrop')
            evidence=dict(operation=op,manual_result_reference=manual_result_reference(op,spec),classification=classified[op],prior_rows=prior,
                disposition=v['disposition'],gate=manual_gates[op][name])
            obligations.setdefault((frame,name),[]).append(evidence)
            cumulative[(frame,name)]=cumulative.get((frame,name),False) or classified[op]['trigger']=='true'
            manual_associations[op][name]=dict(evidence=evidence,event_digest=e['digest']);continue
        if kind=='manual_F_end':
            require(op in manual_specs and op not in manual_closed, 'manual_F_one_complete_occurrence')
            finished(op)
            expected=dict(spec=manual_specs[op],classification=classified[op],gates=manual_gates[op],
                dispositions={name:dict(type=manual_specs[op]['dispositions'][name]['type'],
                    evidence=obligations[(frame,name)][-1]) for name in manual_specs[op]['paths']},
                map_after=map_binding(frame),admission=False)
            require(v['receipt']==expected and v['map_after']==map_binding(frame), 'manual_F_full_disposition_partition_and_actual_map')
            manual_closed[op]=expected;continue
        if kind=='operation_begin':
            require(op==v['id'] and v['id'] not in operations and v['commit'] in frames[frame]['first_parent_operations'],'actual_unique_operation')
            require(op in attempts and all(v[k]==attempts[op][k] for k in ('commit','ordered_parents','parent_index','first_parent','source_head')),'actual_attempt_before_begin')
            parents=git(repo,'rev-list','--parents','-n','1',v['commit']).decode().split()[1:]
            require(v['ordered_parents']==parents and v['first_parent']==parents[0],'actual_ordered_parents')
            index=v['parent_index'];require(type(index) is int and 1<=index<=len(parents),'parent_index')
            require((len(parents)==1 and index==1 and v['source_head']==v['commit']) or (len(parents)>1 and index>=2 and v['source_head']==parents[index-1]),'actual_source_parent')
            require(v['classification']==attempts[op]['classification'],'actual_attempt_classification')
            if v['classification']=='direct':
                require(v['candidates']==attempts[op]['candidates'],'actual_attempt_candidates')
                require(v['boundary']==v['first_parent'],'actual_direct_source_boundary')
            elif v['classification']=='sync':
                # The real sync collector contributes no base-side history.
                # Its empty boundary is the frame base. The preceding attempt
                # retains the independently checked, potentially different bases.
                require(v['boundary']==v['source_head']==frames[frame]['base'] and v['candidates']==[frames[frame]['base']],'actual_sync_source_boundary')
            else:
                require(v['candidates']==attempts[op]['candidates'],'actual_attempt_candidates')
                # Multibase admission is a distinct, unfinished capability.
                # A caller-selected scalar must never erase operation paths.
                if op in M_capabilities:
                    require([v['commit'],v['first_parent'],v['source_head'],v['boundary'],v['candidates']]==[M,P1,P2,O,BASES],'exact_history_boundary_is_not_best_base')
                elif op in J_capabilities:
                    require([v['commit'],v['first_parent'],v['source_head'],v['boundary'],v['candidates']]==[J3071,J3071_P1,J3071_P2,J3071_O,J3071_BASES],'fixed3071_history_boundary_not_selected_bestbase')
                elif op in capabilities_132:
                    unit=capabilities_132[op]['unit']
                    require([v['commit'],v['first_parent'],v['source_head'],v['boundary'],v['candidates']]==
                        [unit['merge'],*unit['parents'],unit['history_boundary'],unit['best_bases']],
                        'exact132_boundary_not_selected_bestbase')
                else:require(v['candidates']==[v['boundary']],'actual_single_best_source_boundary')
            require(v['delta']==raw_binding(git(repo,'diff','--name-only',v['boundary'],v['source_head'])),'actual_operation_delta')
            for name,entry in v['entries'].items():
                require(entry==dict(base=entry_binding(repo,v['boundary'],name),source=entry_binding(repo,v['source_head'],name),result=entry_binding(repo,v['commit'],name)),'actual_full_entries')
            require(v['paths']==v['delta']['raw'].splitlines() and set(v['entries'])==set(v['paths']),'complete_delta_entries')
            operations[v['id']]=v;observed.add(v['commit']);continue
        if kind=='boundary_3071_observation':
            require(op in J_capabilities and v['history_boundary']==J3071_O and v['admission'] is False,'fixed3071_boundary_observation_exact_operation')
            require(v['prefix']==raw_binding((''.join('\t'.join(x)+'\n' for x in maps[frame])).encode()),'fixed3071_preserved_actual_outer_prefix')
            continue
        if kind in ('boundary_observation','composition_observation','scalar_seam_observation'):
            require(v['admission'] is False,'no_production_capability')
            if kind=='scalar_seam_observation':require(v['status']==1,'scalar_only_still_rejects')
            continue
        if kind=='composition_132_capability':
            require(op in capabilities_132 and op in operations and v['path'] in operations[op]['paths'],
                    'exact132_composition_actual_operation')
            completed_132_child(op)
            require(visited.get(op,[]) and visited[op][-1]==v['path'] and op not in classified
                and v['path'] not in compositions_132.setdefault(op,{}),'exact132_composition_once_before_classification')
            actual=operations[op];cap=capabilities_132[op]
            expected=composition_132_capability(repo,actual['commit'],actual['boundary'],actual['first_parent'],
                actual['source_head'],v['path'],tree_entry(repo,actual['commit'],v['path']).rstrip('\0'),
                cap['collector_scope']['base'],cap['collector_scope']['target'])
            require(v==expected and v['capability_sha256']==sha(canonical(cap)),
                    'exact132_independent_whole_composition')
            compositions_132[op][v['path']]=v;continue
        if kind=='corrected_F_composition':
            require(op in operations and operations[op]['commit']==F_CORRECTED_TUPLE['merge']
                and v['path'] in operations[op]['paths'],'corrected_F_actual_operation_path')
            actual=operations[op]
            expected=corrected_F_receipt(repo,v['capability'],actual['commit'],actual['boundary'],actual['first_parent'],
                actual['source_head'],v['path'],tree_entry(repo,actual['commit'],v['path']).rstrip('\0'))
            require(v==expected,'corrected_F_complete_whole_and_required_current_repair')
            F_compositions.setdefault(op,{})[v['path']]=v;continue
        if kind=='composition_3071_capability':
            require(op in J_capabilities and v['path'] in operations[op]['paths'],'fixed3071_composition_actual_path')
            expected=manual_3071_composition(repo)
            expected=dict(expected,capability_sha256=sha(canonical(J_capabilities[op])),path=v['path'],
                result_entry=raw_binding(tree_entry(repo,J3071,v['path']).rstrip('\0')),
                path_composition='REVIEWED_DOCUMENT_RESOLUTION' if v['path']==J3071_DOC else 'DETERMINISTIC_UNCONFLICTED_ENTRY_IN_REVIEWED_WHOLE')
            require(v==expected,'fixed3071_actual_reviewed_manual_whole_not_clean_alias')
            J_compositions.setdefault(op,{})[v['path']]=v;continue
        if kind=='composition_capability':
            require(op in M_capabilities and v['path'] in operations[op]['paths'],'exact_M_composition_path')
            expected=composition(repo,O,P1,P2,v['path'],tree_entry(repo,M,v['path']).rstrip('\0'))
            expected=dict(expected,type=M_capabilities[op]['whole_parent_composition']['type'],capability_sha256=sha(canonical(M_capabilities[op])))
            require(v==expected,'actual_distinct_whole_parent_composition_receipt')
            if v['path'] in M_compositions.setdefault(op,{}):
                require(M_compositions[op][v['path']]==v and (frame,v['path']) in pending,'repeat_composition_only_for_actual_pending_transition')
            M_compositions[op][v['path']]=v;continue
        if kind=='failure': failures.append(dict(frame=frame,category=v['category']));continue
        if kind=='final_gate_before':
            map_before(frame,v);final_gates[frame]=e;continue
        if kind=='reviewed_F_deletion_guard':
            require(op is None and sequence>1 and events[sequence-2]['kind']=='final_gate_before'
                and events[sequence-2]['frame']==frame and events[sequence-2]['payload']['path']==F_DELETION_PATH
                and v['path']==F_DELETION_PATH,'reviewed_F_deletion_original_guard_after_exact_final_visit')
            map_before(frame,v)
            require(v['target']==frames[frame]['target'] and v['target_entry']==raw_binding(tree_entry(repo,v['target'],v['path']).rstrip('\0'))
                and v['stored_entry']!=v['target_entry'],'reviewed_F_deletion_actual_checked_mismatch_not_missing_object')
            final_gates[frame]=e;continue
        if kind=='reviewed_F_deletion':
            require(op is None and v['admission'] is False and F_DELETION_PATH not in terminal.get(frame,{}),
                'reviewed_F_deletion_one_terminal_obligation_not_owner_event')
            require(events[sequence-2]['kind']=='reviewed_F_deletion_guard' and events[sequence-2]['frame']==frame,
                'reviewed_F_deletion_immediately_consumes_original_final_guard')
            prefix=b''.join(raw.splitlines(keepends=True)[:sequence-1]);receipt=v['receipt']
            state=deletion_state(frame,file_digest(prefix))
            expected=reviewed_F_deletion_value(repo,receipt['capability'],state,receipt['ruling_path'])
            require(receipt==expected and v['receipt_sha256']==sha(canonical(expected))
                and v['map_after']==map_binding(frame)==receipt['map_before']==receipt['map_after'],
                'reviewed_F_deletion_independent_receipt_and_byte_identical_map')
            terminal.setdefault(frame,{})[F_DELETION_PATH]=receipt;continue
        require(op in operations,'referenced_real_operation')
        if kind=='sync_gate_passed':
            require(operations[op]['classification']=='sync' and v['gate']=='f13_validate_sync_merge_resolution' and v['classifier']==classifier_binding(),'actual_sync_gate_identity')
            sync_gates.add(op);continue
        if kind=='classification' and op in dc0_specs:
            require(op not in classified and visited.get(op,[])==[DC0_PATH]
                and v['unapproved_delta']==raw_binding(DC0_PATH+'\n')
                and v['classifier']==classifier_binding() and v['trigger']==classify([DC0_PATH])
                and v.get('exit')==(0 if v['trigger']=='true' else 1),'dc0_original_classifier_no_exemption')
            classified[op]=v;continue
        if kind=='classification' and op in manual_specs:
            require(op not in classified and visited.get(op,[])==manual_specs[op]['paths'], 'manual_F_all33_gates_before_classifier')
            names=[n for n in manual_specs[op]['paths'] if manual_gates[op][n]['eligibility']=='unapproved']
            require(v['unapproved_delta']==raw_binding(('\n'.join(names)+'\n').encode() if names else b'')
                and v['classifier']==classifier_binding() and v['trigger']==classify(names)
                and v.get('exit')==(0 if v['trigger']=='true' else 1), 'manual_F_actual_full_operation_raw_classifier')
            classified[op]=v;continue
        if kind=='classification':
            if operations[op]['commit']==F_CORRECTED_TUPLE['merge']:
                require(F_compositions.get(op),'corrected_F_whole_consumed_before_classification')
            require(op not in classified and v['classifier']==classifier_binding(),'original_classifier_identity')
            require(v['trigger']==classify(v['unapproved_delta']['raw'].splitlines()),'actual_raw_classifier_flag')
            require(v['unapproved_delta']==raw_binding(v['unapproved_delta']['raw']),'actual_delta_bytes')
            classified[op]=v
            if op in M_capabilities:
                inherited_native={name for b in native_bindings.values() for name in b['paths']}
                required=[name for name in guarded_names(operations[op]['paths']) if name in inherited_native or operations[op]['entries'][name]['source']!=operations[op]['entries'][name]['result']]
                require(set(required)<=set(M_compositions.get(op,{})),'all_actual_M_composition_seams_consumed')
            continue
        if kind=='J_prior_history_ready':
            require(op in J_capabilities and op not in J_prior_spans and op not in classified,'fixed3071_prior_before_actual_J_gates')
            state=J_prefix_state(op);require(v['origin']['main_trace']==str(path),'fixed3071_prior_same_actual_main_trace')
            prefix=b''.join(raw.splitlines(keepends=True)[:sequence-1])
            require(v['origin']==J_prior_origin_value(repo,J_capabilities[op],state,path,file_digest(prefix)), 'fixed3071_prior_actual_unchanged_parent_prefix')
            require(v==verify_J_prior_span(repo,v['origin'],v['trace'],v['map_path']),'fixed3071_prior_reconstructed_real_source_evidence')
            J_prior_spans[op]=v;continue
        if kind=='range_J_ledger_ready':
            require(op in J_capabilities and op not in J_ready,'fixed3071_J_one_actual_ledger_checkpoint')
            binding=v['ledger'];require(binding==raw_binding(binding['raw']) and v['admission'] is False,'fixed3071_J_complete_ledger_raw_binding')
            state=J_checkpoint_state(op,events[:sequence-1]);prefix=b''.join(raw.splitlines(keepends=True)[:sequence-1])
            ready=json.loads(binding['raw'])
            require(canonical(ready)==canonical(build_3071_J_ready(state,file_digest(prefix))),'fixed3071_J_independent_complete_owner_content_OR_ledger')
            J_ready[op]=ready;continue
        if kind=='range_I_ledger_ready':
            require(op not in I_ready,'fixed3071_I_one_actual_ledger_checkpoint')
            binding=v['ledger'];require(binding==raw_binding(binding['raw']) and v['admission'] is False,'fixed3071_I_complete_ledger_raw_binding')
            ready=json.loads(binding['raw']);state=I_checkpoint_state(op)
            prefix=b''.join(raw.splitlines(keepends=True)[:sequence-1])
            require(canonical(ready)==canonical(build_3071_I_ready(state,dict(bytes=len(prefix),sha256=sha(prefix)))),'fixed3071_I_reconstructed_complete_owner_content_OR_ledger')
            I_ready[op]=ready;continue
        if kind=='M_ledger_ready':
            require(op in M_capabilities and op not in M_ready,'one_exact_M_ledger_checkpoint')
            binding=v['ledger'];require(binding==raw_binding(binding['raw']),'complete_M_ledger_raw_binding')
            ready=json.loads(binding['raw']);prefix_events=events[:sequence-1]
            prefix_raw=b''.join(raw.splitlines(keepends=True)[:sequence-1])
            state=M_checkpoint_state(op,prefix_events)
            expected=build_M_ready(repo,state,prefix_events,dict(bytes=len(prefix_raw),sha256=sha(prefix_raw)),
                ready['original_trace'],ready['span_trace'],ready['span_trace_sha256'],ready['original_native'],ready['current_native'])
            require(ready==expected,'independently_recomputed_complete_M_ledger')
            M_ready[op]=ready;continue
        if kind in ('path_gate_before','sync_gate_before'):
            map_before(frame,v)
            if kind=='path_gate_before':visited.setdefault(op,[]).append(v['path'])
            continue
        if kind=='disposition':
            require(v['operation_reference']==op and v['reason'],'actual_disposition_reference');map_before(frame,v)
            if v['reason']=='verified_original_sync_reconciliation':
                require(op in sync_gates,'sync_receipt_before_reconciliation')
                if sum(x[0]==v['path'] for x in maps[frame])==1:maps[frame]=[x for x in maps[frame] if x[0]!=v['path']]
            continue
        require(op in classified,'classification_before_row')
        key=(frame,v['path'])
        if kind=='qualification_pending':
            require(op in M_capabilities and key in pending and v['admission'] is False,'explicit_M_consumer_pending_without_mutation')
            require(v['map_before']==map_binding(frame),'pending_M_map_is_unchanged');continue
        if kind=='exact_unit_supersession':
            item=require_pending_operation(pending,key,op)
            require(op in M_ready and 'exact_unit_supersession' not in item and not item['removed'],'exact_supersession_before_mutation')
            item=pending[key];receipt=v['receipt'];prefix_raw=b''.join(raw.splitlines(keepends=True)[:sequence-1])
            expected=supersession_evidence(repo,M_capabilities[op],M_ready[op],events[:sequence-1],v['path'],
                ''.join('\t'.join(x)+'\n' for x in maps[frame]).encode(),'\t'.join(item['incoming']),M_compositions[op].get(v['path']))
            expected=dict(expected,trace_prefix=dict(bytes=len(prefix_raw),sha256=sha(prefix_raw)))
            require(receipt==expected and v['admission'] is False,'independent_exact_supersession_actual_history_binding')
            item['exact_unit_supersession']=receipt;continue
        if kind=='r23_132_owner_permission':
            item=require_pending_operation(pending,key,op)
            prefix_raw=b''.join(raw.splitlines(keepends=True)[:sequence-1])
            state=r23_132_state(op,v['path'],file_digest(prefix_raw))
            require(events[sequence-2]['kind']=='row_before' and events[sequence-2]['operation']==op
                and events[sequence-2]['payload']['path']==v['path'],'r23_132_immediately_after_pending_row')
            expected=r23_132_owner_evidence(repo,state)
            require(v['receipt']==expected and v['admission'] is False,'r23_132_independent_permission')
            item['r23_132_owner_permission']=expected;continue
        if kind=='finite_J_owner_permission':
            item=require_pending_operation(pending,key,op)
            require(op in J_ready and 'finite_J_owner_permission' not in item and not item['removed']
                    and 'conservation' not in item and not failures,'finite_J_permission_before_mutation')
            prefix_raw=b''.join(raw.splitlines(keepends=True)[:sequence-1])
            expected=finite_J_owner_evidence(repo,J_capabilities[op],J_ready[op],events[:sequence-1],
                file_digest(prefix_raw),''.join('\t'.join(x)+'\n' for x in maps[frame]).encode())
            require(v['receipt']==expected and v['admission'] is False,'finite_J_independent_permission_reconstruction')
            item['finite_J_owner_permission']=expected;continue
        if kind=='exact_J_owner_permission':
            item=require_pending_operation(pending,key,op)
            require(op in J_ready and 'exact_J_owner_permission' not in item and not item['removed']
                and 'conservation' not in item and not failures, 'exact_J_permission_before_guard_and_mutation')
            prefix_raw=b''.join(raw.splitlines(keepends=True)[:sequence-1])
            expected=exact_J_owner_evidence(repo,J_capabilities[op],J_ready[op],events[:sequence-1],
                file_digest(prefix_raw),''.join('\t'.join(x)+'\n' for x in maps[frame]).encode())
            require(v['receipt']==expected and v['admission'] is False,'exact_J_independently_reconstructed_permission')
            item['exact_J_owner_permission']=expected;continue
        if kind in ('complete_ledger_conserved','range_I_ledger_conserved','range_J_ledger_conserved'):
            ready_table={'complete_ledger_conserved':M_ready,'range_I_ledger_conserved':I_ready,'range_J_ledger_conserved':J_ready}[kind]
            consumed_table={'complete_ledger_conserved':M_consumed,'range_I_ledger_conserved':I_consumed,'range_J_ledger_conserved':J_consumed}[kind]
            require(op in ready_table and key in pending and pending[key]['op']==op,'complete_typed_ledger_before_real_row_mutation')
            receipt=v['receipt'];item=pending[key];parts=row(receipt['after_gate_row'])
            require(receipt['path']==v['path'] and receipt['incoming']==raw_binding('\t'.join(item['incoming'])),'actual_M_consumer_incoming')
            require(all(parts[i]==item['incoming'][i] for i in (0,2,3,6,7)) and parts[4:6]==item.get('rebind',item['incoming'][4:6]),'actual_M_consumer_after_gate_identity')
            require(parts[1]==item['incoming'][1] or (parts[1]=='true' and cumulative[key]),'actual_M_consumer_raw_flag')
            require(receipt['ledger_sha256']==sha(canonical(ready_table[op])) and receipt['prior_rows']==[raw_binding('\t'.join(x)) for x in maps[frame] if x[0]==v['path']],'complete_typed_consumer_prior_ledger')
            require(receipt['obligations']==ready_table[op]['required_qualification'][v['path']] and receipt['admission'] is False and v['admission'] is False,'complete_typed_consumer_evidence_not_sidecar_bit')
            require(receipt['map_before']==map_binding(frame) and receipt['transition'] in ('append','replace'),'M_transition_actual_premap')
            aggregate=cumulative[key] or ready_table[op]['required_qualification'][v['path']]['aggregate']
            require(type(receipt['aggregate']) is bool and receipt['aggregate']==aggregate,'M_consumer_must_keep_full_aggregate_or')
            require(v['policy_result'] is None and receipt['policy_result'] is None,'conservation_is_not_policy_admission')
            if kind=='range_J_ledger_conserved' and '\t'.join(item['incoming'])==EXACT_J_AUDIT_INCOMING and 'rebind' not in item:
                require('exact_J_owner_permission' in item,'exact_J_permission_required_before_conservation')
            if kind=='range_J_ledger_conserved' and v['path'] in finite_J_paths():
                require('finite_J_owner_permission' in item and 'rebind' not in item,
                        'finite_J_permission_required_before_conservation')
            validate_exact_conservation(item,op,receipt)
            consumed_table.setdefault(op,[]).append(v['path']);item['conservation']=receipt
            continue
        if kind=='native_rebind_gate':
            require(key in pending and pending[key]['op']==op and not pending[key]['removed'] and 'rebind' not in pending[key],'native_gate_before_exact_mutation')
            item=pending[key];prior=item['prior'];incoming=item['incoming'];operation=operations[op]
            require(prior is not None and v['gate']=='native_p3_duplicate_owner_qualified' and v['status']==0 and v['classifier']==classifier_binding(),'actual_native_restoration_gate')
            nested=child_rows.get((op,v['path']),incoming)
            require([v['prior_owner'],v['prior_source'],v['current_owner'],v['current_source'],v['integration']]==[prior[2],prior[5],nested[2],incoming[5],operation['commit']],'actual_native_gate_owner_source_arguments')
            require(v['outcome'] in ('clean','guarded_conflict','direct') and (v['outcome']=='direct')==(operation['classification']=='direct'),'actual_native_gate_outcome')
            binding=v['binding'];require(binding==raw_binding(binding['raw']),'native_binding_raw_hash')
            context=v['context'];cache=sha(canonical(dict(binding=binding,context=context)))
            if cache not in native_bindings:
                spec=importlib.util.spec_from_file_location('f54_native_rebind',ROOT/('scripts/ci/f13_native_p3_authority.py' if (ROOT/'scripts/ci/f13_native_p3_authority.py').is_file() else 'scripts/ci/f13_native_p3_source.py'))
                native=importlib.util.module_from_spec(spec);spec.loader.exec_module(native)
                b=json.loads(binding['raw'])
                require(b['target']==git(repo,'rev-parse','HEAD').decode().strip(),'native_receipt_actual_current_target')
                actual=native.qualify(repo,b['base'],b['source'],b['target'],context['GITHUB_EVENT_NAME'],context['GITHUB_REF'],context['GITHUB_SHA'],context['VOICE_CI_PUSH_BEFORE_SHA'],context['GITHUB_BASE_REF'])
                require(actual==b,'native_receipt_independently_qualified_binding');native_bindings[cache]=actual
            b=native_bindings[cache]
            require(v['path'] in b['paths'],'native_receipt_exact_bound_path')
            for candidate in (v['prior_owner'],v['prior_source'],v['current_owner'],v['current_source'],v['integration']):
                require(subprocess.run(['git','--no-replace-objects','-C',str(repo),'merge-base','--is-ancestor',candidate,b['snapshot']],capture_output=True).returncode==0,'native_receipt_source_ancestry')
            require(tree_entry(repo,v['integration'],v['path'])==tree_entry(repo,b['snapshot'],v['path']),'native_receipt_actual_H_entry_restoration')
            item['rebind']=[operation['first_parent'],operation['commit']]
        elif kind=='row_before':
            if op in capabilities_132:
                completed_132_child(op)
                native_paths=json.loads(master_entry['native_binding']['raw'])['paths']
                # Match the original collector's checkpoint-composition gate;
                # Native documentation still follows ordinary row conservation.
                if v['path'] in native_paths and guarded_names([v['path']]):
                    require(v['path'] in compositions_132.get(op,{}),'exact132_native_checkpoint_composition_required')
            require(op not in J_capabilities or op in J_ready,'fixed3071_unqualified_ledger_cannot_begin_mutation')
            map_before(frame,v)
            prior=row(v['prior']);incoming=row(v['incoming'])
            require(key not in pending and incoming[0]==v['path'] and incoming[2]==operations[op]['commit'],'actual_incoming_operation')
            operation=operations[op]
            if op in dc0_specs:
                require(v['path']==DC0_PATH and not prior and not v['prior_rows'],'dc0_no_existing_owner')
            if op in manual_specs:
                require(v['path'] in F_MANUAL_NEW_PATHS and not prior and not v['prior_rows'], 'manual_F_append_only_genuinely_unowned_ten')
            require(incoming[3]==operation['first_parent'] and incoming[7]==','.join(operation['candidates']),'actual_incoming_parent_and_bases')
            require(incoming[6]==(prior[6] if prior else operation['first_parent']),'actual_pre_qualification_content_base')
            if (op,v['path']) in child_rows:
                parts=child_rows[(op,v['path'])]
                require([incoming[i] for i in (1,4,5)]==[parts[i] for i in (1,4,5)],'actual_nested_source_row_reference')
            else:
                require(incoming[4:6]==[operation['boundary'],operation['source_head']],'actual_incoming_source_boundary')
                require(incoming[1]==('true' if carry.get(op,False) or classified[op]['trigger']=='true' else 'false'),'actual_path_trigger_carry')
            pending[key]=dict(prior=prior,incoming=incoming,op=op,after=None,removed=False)
            previous_rows=[row(value) for value in v['prior_rows']]
            require((previous_rows[-1] if previous_rows else None)==prior,'actual_prior_row_order')
            cumulative[key]=cumulative.get(key,False) or any(p[1]=='true' for p in previous_rows) or incoming[1]=='true' or classified[op]['trigger']=='true' or child_flags.get((op,v['path']),False)
            if op in dc0_specs:
                cumulative[key]|=dc0_specs[op]['original_classification']['trigger']=='true'
            if op in I_ready:cumulative[key]|=I_ready[op]['required_qualification'][v['path']]['aggregate']
            if op in J_ready:cumulative[key]|=J_ready[op]['required_qualification'][v['path']]['aggregate']
            obligations.setdefault(key,[]).append(dict(operation=op,classification=classified[op],prior_rows=v['prior_rows'],incoming=v['incoming'],child_aggregate=child_flags.get((op,v['path']),False),
                **(dict(dc0_residual_reference=dict(operation=op,source_unit_sha256=DC0_SOURCE_UNIT_SHA,
                    spec_sha256=sha(canonical(dc0_specs[op])),original_classification=dc0_specs[op]['original_classification'])) if op in dc0_specs else {}),
                **(dict(manual_result_reference=manual_result_reference(op,manual_specs[op]),gate=manual_gates[op][v['path']]) if op in manual_specs else {})))
        elif kind in ('row_after','row_remove'):
            require(key in pending and pending[key]['op']==op,'pre_mutation_prior_reference')
            parts=row(v['row']);require(parts[6]==pending[key]['incoming'][6],'original_content_base')
            incoming=pending[key]['incoming']
            require(all(parts[i]==incoming[i] for i in (0,2,3,6,7)) and parts[4:6]==pending[key].get('rebind',incoming[4:6]),'actual_reduction_row_identity')
            if op in M_capabilities:require(v['path'] in M_consumed.get(op,[]),'M_mutation_requires_consumed_complete_ledger')
            if op in J_capabilities:require(v['path'] in J_consumed.get(op,[]),'fixed3071_J_mutation_requires_consumed_complete_ledger')
            if any(data['parent_operation']==op and name in range_branches for name,data in frames.items()):
                require(op in I_ready and v['path'] in I_consumed.get(op,[]),'fixed3071_I_mutation_requires_consumed_full_range_ledger')
            if op in capabilities_132 and v['path'] in [r['path'] for r in json.loads(R23_132_SOURCE_UNIT_RAW)['rows']]:
                require('r23_132_owner_permission' in pending[key], 'r23_132_permission_before_mutation')
            if kind=='row_remove':
                require(pending[key]['prior'] is not None and not pending[key]['removed'],'actual_single_reduction')
                pending[key]['removed']=True;maps[frame]=[x for x in maps[frame] if x[0]!=v['path']]
            else:
                if not pending[key]['removed']:
                    # An append keeps all prior flags/owners in the map. Its
                    # original raw false flag must not be normalized to true.
                    require(parts==pending[key]['incoming'],'original_raw_append_row')
                else:
                    require(parts[1]==pending[key]['incoming'][1] or (parts[1]=='true' and cumulative[key]),'no_fabricated_reduction_flag')
                pending[key]['after']=parts;carry[op]=parts[1]=='true'
        elif kind=='row_committed':
            require_pending_operation(pending,key,op)
            require(pending[key]['after']==row(v['row']),'actual_committed_row')
            frame_rows[key]=row(v['row'])
            maps[frame].append(row(v['row']))
            require(v['map_after']==map_binding(frame) and [row(x) for x in v['retained_rows']]==[x for x in maps[frame] if x[0]==v['path']],'conserved_committed_map')
            if op in M_capabilities or op in I_ready or op in J_ready:
                receipt=pending[key]['conservation']
                require(receipt['expected_post_map']==map_binding(frame) and (receipt['transition']=='replace')==pending[key]['removed'],'M_conserved_exact_actual_post_transition')
            committed.setdefault(op,[]).append(v['path'])
            pending.pop(key)
        else:raise Rejected('unknown_trace_kind')
    manual_append_value=None
    if manual_append_checkpoint:
        op,name=manual_append_checkpoint;frame=op.split('/')[0];key=(frame,name)
        require(stack[-1]==frame and op in manual_specs and key in pending and not failures
            and pending[key]['after'] is None and pending[key]['prior'] is None, 'manual_F_actual_preappend_state')
        actual=(''.join('\t'.join(x)+'\n' for x in maps[frame])).encode()
        # Whole unit/context is rechecked before each actual append. Prior rows
        # are independently conserved by the fold; no append may erase them.
        checked_corrected_F_capability(repo,manual_specs[op]['capability'])
        manual_append_value=dict(map_before=file_digest(actual),operation=op,path=name,admission=False)
    manual_end_value=manual_end_state(manual_end_checkpoint) if manual_end_checkpoint else None
    manual_state=manual_checkpoint_state() if manual_checkpoint else None
    range_state=range_checkpoint_state(range_checkpoint) if range_checkpoint else None
    I_state=I_checkpoint_state(I_checkpoint) if I_checkpoint else None
    J_history_state=J_prefix_state(J_history_checkpoint) if J_history_checkpoint else None
    J_state=J_checkpoint_state(J_checkpoint,events) if J_checkpoint else None
    deletion_state_value=deletion_state(deletion_checkpoint,file_digest(raw)) if deletion_checkpoint else None
    exact_J_permission=None
    if exact_J_checkpoint is not None:
        require(exact_J_checkpoint in ('pending','permitted') and not failures and len(pending)==1,
                'exact_J_one_live_pending_checkpoint')
        (frame,name),item=next(iter(pending.items()));op=item['op']
        require(stack and stack[-1]==frame and op in J_ready and not item['removed'] and 'conservation' not in item,
                'exact_J_checkpoint_before_mutation')
        if exact_J_checkpoint=='pending':
            require('exact_J_owner_permission' not in item,'exact_J_no_replayed_permission')
            exact_J_permission=exact_J_owner_evidence(repo,J_capabilities[op],J_ready[op],events,
                file_digest(raw),''.join('\t'.join(x)+'\n' for x in maps[frame]).encode())
        else:
            require(events[-1]['kind']=='exact_J_owner_permission' and 'exact_J_owner_permission' in item,
                    'exact_J_consumer_exact_permission_position')
            exact_J_permission=item['exact_J_owner_permission']
    finite_state=None
    if finite_J_checkpoint:
        require(finite_J_checkpoint is True and not failures,'finite_J_live_checkpoint')
        names=finite_J_paths();last=events[-1];op=last['operation'];frame=last['frame']
        require(op in J_ready and stack and stack[-1]==frame,'finite_J_checkpoint_actual_frame')
        actual=(''.join('\t'.join(x)+'\n' for x in maps[frame])).encode()
        if last['kind']=='row_committed' and last['payload']['path']==names[-1]:
            require(not pending,'finite_J_finish_no_pending')
            phase='finish';index=len(names);permission=None
        else:
            require(len(pending)==1,'finite_J_one_pending_checkpoint')
            (pending_frame,name),item=next(iter(pending.items()))
            require(pending_frame==frame and item['op']==op and name in names and not item['removed']
                    and item['after'] is None and 'conservation' not in item,'finite_J_checkpoint_before_mutation')
            index=names.index(name);permission=item.get('finite_J_owner_permission')
            phase='consume' if permission is not None else 'permission'
            require(last['kind']==('finite_J_owner_permission' if permission is not None else 'row_before'),
                    'finite_J_checkpoint_exact_phase')
        require([n for n in committed.get(op,[]) if n in names]==names[:index],
                'finite_J_checkpoint_exact_committed_order')
        finite_state=dict(ready=J_ready[op],capability=J_capabilities[op],map=actual.decode(),
                          phase=phase,index=index,permission=permission)
    r23_permission=None
    if r23_132_checkpoint:
        require(r23_132_checkpoint is True and len(pending)==1 and events[-1]['kind']=='row_before',
                'r23_132_one_pending_checkpoint')
        (frame,name),item=next(iter(pending.items()))
        r23_permission=r23_132_owner_evidence(repo,r23_132_state(item['op'],name,file_digest(raw)))
    dc0_value=dc0_state(dc0_checkpoint,dc0_context) if dc0_checkpoint else None
    checkpoint_state=M_checkpoint_state(checkpoint,events) if checkpoint else r23_permission or dc0_value or range_state or I_state or J_history_state or J_state or deletion_state_value or manual_state or manual_end_value or manual_append_value or exact_J_permission or finite_state
    require(frames,'nonempty_producer_frames')
    require(not stack or checkpoint_state is not None,'closed_frames')
    require(checkpoint_state is not None or master_entry is None or failures or master_wrapper is not None,'successful_master_collection_requires_actual_wrapper_result')
    coverage=dag_dispositions(repo,events) if complete else []
    accepted={'visited_real_operation','shared_real_operation','exact_single_commit_source_of_real_operation'}
    require(not complete or (all(v['disposition'] in accepted for node in coverage for v in node['operations']) and not failures),'complete_33_real_producer_coverage_and_gates')
    ledger=ledger_state()
    return dict(schema=SCHEMA,trace_sha256=expected_sha,frames=len(frames),operations=len(operations),failures=failures,
        ledger=ledger,coverage=coverage,M_checkpoint=checkpoint_state if checkpoint else None,M_ready=M_ready,M_consumed=M_consumed,
        root_frame=next(iter(frames)),master_entry=master_entry,master_wrapper=master_wrapper,range_checkpoint=range_state,range_branches=range_branches,
        I_checkpoint=I_state,I_ready=I_ready,I_consumed=I_consumed,deletion_checkpoint=deletion_state_value,
        J_history_checkpoint=J_history_state,J_checkpoint=J_state,J_ready=J_ready,J_consumed=J_consumed,J_prior_spans=J_prior_spans,
        terminal_dispositions=terminal_dispositions(ledger),manual_checkpoint=manual_state,manual_end_checkpoint=manual_end_value,manual_append_checkpoint=manual_append_value,manual_result_operations=manual_operations(ledger),
        exact_J_permission=exact_J_permission,**(dict(finite_J_checkpoint=finite_state) if finite_J_checkpoint else {}),
        r23_132_permission=r23_permission,dc0_checkpoint=dc0_value,dc0_residual_operations=dc0_closed,
        observed_qualification='COMPLETE' if complete else 'OBSERVED_PREFIX_ONLY',admission=False)


def consume_aggregate(verified, frame, name, actual_map, actual_trigger):
    """Consume the verified full ledger, never a last-row flag or sidecar bit.

    CLI callers must validate the exact hashed trace in this process first.
    This is prototype qualification evidence, not production admission.
    """
    ledger=verified['ledger'][frame];required=ledger['paths'][name]
    require(ledger['map_binding']==dict(bytes=len(actual_map),sha256=sha(actual_map)),'consumed_complete_map_binding')
    require(actual_map.decode().splitlines()==ledger['rows'],'consumed_exact_ordered_rows')
    require(actual_trigger in ('true','false') and (actual_trigger=='true')==required['aggregate'],'qualification_must_consume_aggregate_or')
    require(required['evidence'],'consumed_owner_content_obligations')
    return dict(trace_sha256=verified['trace_sha256'],frame=frame,path=name,
        aggregate=required['aggregate'],evidence=required['evidence'],map_binding=ledger['map_binding'],admission=False)


def main():
    try:
        command,*args=sys.argv[1:]
        if command=='append':
            path,kind,frame,op,repo,*values=args
            identity=append(path,kind,frame,op,repo,values)
            if identity:print(identity)
            return 0
        if command=='132-owner-permission':
            repo,trace,name,map_path,incoming=args
            raw=Path(trace).read_bytes()
            verified=validate_trace(repo,trace,sha(raw),r23_132_checkpoint=True)
            receipt=verified['r23_132_permission']
            require(receipt['path']==name and receipt['incoming']==incoming and receipt['map_before']==file_binding(map_path),
                    'r23_132_actual_consumer_inputs')
            require(Path(trace).read_bytes()==raw,'r23_132_trace_unchanged_before_permission')
            print(json.dumps(receipt,sort_keys=True));return 0
        if command=='boundary':print(json.dumps(exact_boundary(*args)));return 0
        if command=='composition':print(json.dumps(composition(*args)));return 0
        if command=='unit':print(json.dumps(load_unit(*args)));return 0
        if command=='capabilities':print(json.dumps(prepare_capabilities(*args)));return 0
        if command=='current-author-capabilities':print(json.dumps(prepare_author_capabilities(*args)));return 0
        if command=='production-capabilities':print(json.dumps(prepare_production_capabilities(*args)));return 0
        if command=='132-boundary':print(json.dumps(boundary_132_capability(*args)));return 0
        if command=='132-composition':print(json.dumps(composition_132_capability(*args)));return 0
        if command=='cap-boundary':print(json.dumps(boundary_capability(*args)));return 0
        if command=='cap-entry':print(json.dumps(entry_capability(*args)));return 0
        if command=='cap-composition':print(json.dumps(composition_capability(*args)));return 0
        if command=='corrected-F-capability':print(json.dumps(prepare_corrected_F_capability(*args)));return 0
        if command=='corrected-F-production-capability':print(json.dumps(prepare_corrected_F_production_capability(*args)));return 0
        if command=='corrected-F-composition':print(json.dumps(corrected_F_composition(*args)));return 0
        if command=='manual-F-context':
            repo,map_path,*pairs=args;print(json.dumps(snapshot_manual_F_context(repo,map_path,pairs)));return 0
        if command=='manual-F-append-check':
            repo,trace,operation,name,map_path=args;raw=Path(trace).read_bytes()
            value=validate_trace(repo,trace,sha(raw),manual_append_checkpoint=(operation,name))['manual_append_checkpoint']
            require(value['map_before']==file_digest(Path(map_path).read_bytes()), 'manual_F_preappend_same_map');return 0
        if command=='dc0-residual-checkpoint':
            repo,trace,map_path,phase,*context=args;raw=Path(trace).read_bytes()
            require(len(context)==(1 if phase=='begin' else 0),'dc0_checkpoint_context_arguments')
            value=validate_trace(repo,trace,sha(raw),dc0_checkpoint=phase,
                dc0_context=json.loads(Path(context[0]).read_bytes()) if context else None)['dc0_checkpoint']
            require(value['map_after' if phase=='end' else 'map_before']==file_digest(Path(map_path).read_bytes()),
                'dc0_actual_live_map')
            print(json.dumps(value));return 0
        if command=='manual-F-spec':print(json.dumps(prepare_manual_F(*args)));return 0
        if command=='manual-F-end':print(json.dumps(prepare_manual_F_end(*args)));return 0
        if command=='router-manual-delta':
            repo,path,digest,map_path,context_path,name=args
            print(manual_delta_for_binding(router_binding(repo,path,digest,map_path,context_path,name)),end='');return 0
        if command=='reviewed-F-deletion':print(json.dumps(prepare_reviewed_F_deletion(*args)));return 0
        if command=='3071-capabilities':print(json.dumps(prepare_3071_capabilities(*args)));return 0
        if command=='3071-current-author-capabilities':print(json.dumps(prepare_3071_author_capabilities(*args)));return 0
        if command=='3071-production-capabilities':print(json.dumps(prepare_3071_production_capabilities(*args)));return 0
        if command=='3071-boundary':print(json.dumps(boundary_3071_capability(*args)));return 0
        if command=='3071-composition':print(json.dumps(composition_3071_capability(*args)));return 0
        if command=='3071-range-origin':
            repo,capability,trace=args;value=prepare_3071_range_origin(repo,capability,trace);value['capability_path']=capability
            print(json.dumps(value));return 0
        if command=='3071-range-branch':print(json.dumps(prepare_3071_range_branch(*args)));return 0
        if command=='3071-I-ledger':print(json.dumps(prepare_3071_I_ledger(*args)));return 0
        if command=='3071-J-prior-origin':print(json.dumps(prepare_J_prior_origin(*args)));return 0
        if command=='3071-J-prior-receipt':print(json.dumps(prepare_J_prior_span(*args)));return 0
        if command=='3071-J-ledger':print(json.dumps(prepare_3071_J_ledger(*args)));return 0
        if command=='3071-J-owner-permission':
            repo,capability,ledger,digest,trace,name,map_path,incoming=args
            print(json.dumps(qualify_exact_J_owner(repo,capability,ledger,digest,trace,name,Path(map_path).read_bytes(),incoming)));return 0
        if command=='3071-J-consume':
            ready,digest,trace,name,map_path,incoming,transition=args
            print(json.dumps(consume_3071_J_path(ready,digest,trace,name,Path(map_path).read_bytes(),incoming,transition)));return 0
        if command=='3071-I-consume':
            ready,digest,trace,name,map_path,incoming,transition=args
            print(json.dumps(consume_3071_I_path(ready,digest,trace,name,Path(map_path).read_bytes(),incoming,transition)));return 0
        if command=='master-entry':print(json.dumps(prepare_master_entry(*args)));return 0
        if command=='master-result':print(json.dumps(master_wrapper_result(*args)));return 0
        if command=='M-ledger':print(json.dumps(prepare_M_ledger(*args)));return 0
        if command=='M-supersession':print(json.dumps(qualify_supersession(*args)));return 0
        if command=='M-consume':
            ready,digest,trace,name,map_path,incoming,transition=args
            print(json.dumps(consume_M_path(ready,digest,trace,name,Path(map_path).read_bytes(),incoming,transition)));return 0
        if command=='collection-seal':print(json.dumps(finalize_collection(*args)));return 0
        if command=='policy-context':
            repo,map_path,*pairs=args;print(json.dumps(policy_context(repo,map_path,pairs)));return 0
        if command=='router-seal':print(json.dumps(seal_router(*args)));return 0
        if command=='finite-J-path':return 0 if args[0] in finite_J_paths() else 1
        if command=='finite-J-worker':finite_J_worker(*args);return 0
        if command=='finite-J-request':
            socket_path,session,action,name,incoming=args
            require(action in ('permission','consume','finish'),'finite_J_client_command')
            output=finite_J_worker_request(socket_path,session,action,name)
            if action=='finish':require(name==incoming==output=='','finite_J_finish_ack')
            else:
                value=json.loads(output)
                require(value['incoming']==raw_binding(incoming),'finite_J_client_exact_incoming')
                if action=='consume':require(value['transition']=='replace' and value['after_gate_row']==raw_binding(incoming),
                                            'finite_J_client_exact_mutation')
            print(output,end='');return 0
        if command=='router-worker':router_worker(*args);return 0
        if command=='router-request':
            status,output=router_worker_request(*args);print(output,end='');return status
        if command=='router-validate':print(json.dumps(validate_router(*args)));return 0
        if command=='router-bind':print(json.dumps(router_binding(*args)));return 0
        if command=='router-append':
            path,kind,receipt,*arguments=args;value=json.loads(Path(receipt).read_bytes())
            if kind=='gate':value=dict(binding=value,gate=arguments[0],status=int(arguments[1]),arguments=arguments[2:],
                argument_files={x:file_binding(x) for x in arguments[2:] if Path(x).is_file()})
            append_router(path,kind,value);return 0
        if command=='router-runtime':
            repo,path,digest,map_path,context_path,name=args;value=router_binding(repo,path,digest,map_path,context_path,name)
            if not value['covered']:return 3
            print('true' if value['aggregate'] else 'false');return 0
        if command=='router-history-row':
            repo,path,digest,map_path,context_path,name=args
            value=retained_row_for_binding(router_binding(repo,path,digest,map_path,context_path,name))
            if value is None:return 3
            print(value);return 0
        if command=='collection-runtime-paths':
            repo,path,digest,map_path=args;value=load_collection(repo,path,digest,map_path)
            print('\n'.join(name for name,item in value['required_qualification'].items() if item['aggregate']));return 0
        if command=='collection-retained-paths':
            repo,path,digest,map_path,changed_path=args;value=load_collection(repo,path,digest,map_path)
            print('\n'.join(sorted(set(value['required_qualification'])-set(Path(changed_path).read_text().splitlines()))));return 0
        if command=='span-origin':print(json.dumps(span_origin(*args)));return 0
        if command=='span-author-origin':print(json.dumps(span_author_origin(*args)));return 0
        if command=='span-validate':print(json.dumps(validate_aggregate_span(*args)));return 0
        if command=='unit-observe':print(json.dumps(observe_qualified_unit(*args)));return 0
        if command=='coverage':
            repo,path,digest=args;raw=Path(path).read_bytes();require(sha(raw)==digest,'coverage_trace_raw_hash')
            events=[json.loads(v) for v in raw.splitlines()]
            validate_trace(repo,path,digest)
            print(json.dumps(dict(trace_sha256=digest,dispositions=dag_dispositions(repo,events),admission=False)));return 0
        if command=='validate':
            repo,path,digest,*flags=args;print(json.dumps(validate_trace(repo,path,digest,bool(flags))));return 0
        if command=='consume':
            repo,path,digest,frame,name,map_path,trigger=args
            print(json.dumps(consume_aggregate(validate_trace(repo,path,digest),frame,name,Path(map_path).read_bytes(),trigger)));return 0
        raise Rejected('prototype_command')
    except (Rejected,ValueError,KeyError,TypeError,OSError) as exc:
        print('F54 observation rejected: '+str(exc),file=sys.stderr);return 1


if __name__=='__main__':sys.exit(main())
