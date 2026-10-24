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
    elif kind in ('history_capability','history_3071_capability'):
        capability,receipt=values
        payload=dict(capability=raw_binding(Path(capability).read_bytes()),receipt=json.loads(receipt),admission=False)
    elif kind in ('composition_capability','composition_3071_capability','corrected_F_composition','range_3071_branch','J_prior_history_ready'):
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
    elif kind in ('exact_unit_supersession', 'exact_J_owner_permission'):
        name,receipt=values
        payload=dict(path=name,receipt=json.loads(Path(receipt).read_bytes()),admission=False)
    elif kind in ('master_entry','master_wrapper_result'):
        payload=json.loads(Path(values[0]).read_bytes())
    else: raise Rejected('unknown_event_kind')
    e=dict(schema=SCHEMA,sequence=sequence,previous=previous['digest'] if previous else '0'*64,
        kind=kind,frame=identity if kind=='frame_enter' else frame or None,
        operation=identity if kind in ('operation_attempt','operation_begin','manual_F_begin') else operation or None,payload=payload)
    e['digest']=sha(canonical(e))
    with p.open('a',encoding='utf-8',newline='\n') as stream: stream.write(json.dumps(e,sort_keys=True)+'\n')
    return identity


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


def current_author_J_native_applicability(repo, native, historical, current):
    """Preserve old reviewed facts; independently derive current source authority."""
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
    patch=git(repo,'diff','--binary',J3071_AUTO_TREE,J3071)
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
    if binding['type']=='F13-PRODUCTION-SOURCE-v4':
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
    return dict(type='F54_EXPLICIT_MASTER_PRODUCTION_ENTRY_v1' if production_roles is not None else
        ('F54_EXPLICIT_MASTER_CURRENT_AUTHOR_ENTRY' if author_roles is not None else 'F54_EXPLICIT_MASTER_PROTOTYPE_ENTRY'),
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
        require(entry['type']==('F54_EXPLICIT_MASTER_PRODUCTION_ENTRY_v1' if is_production else
            'F54_EXPLICIT_MASTER_CURRENT_AUTHOR_ENTRY'),'current_author_span_actual_master_entry')
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


def validate_trace(repo, path, expected_sha, complete=False, checkpoint=None, range_checkpoint=None, I_checkpoint=None, deletion_checkpoint=None, manual_checkpoint=False, manual_end_checkpoint=None, manual_append_checkpoint=None, J_history_checkpoint=None, J_checkpoint=None, sealed_prefix=None, exact_J_checkpoint=None):
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
    J_prior_spans={};J_ready={};J_consumed={}
    J_capabilities={};J_compositions={};F_compositions={};range_branches={};frame_results={};I_ready={};I_consumed={}
    terminal={};final_gates={};manual_specs={};manual_gates={};manual_associations={};manual_closed={}
    def map_binding(frame):
        raw=''.join('\t'.join(parts)+'\n' for parts in maps[frame]).encode()
        return dict(bytes=len(raw),sha256=sha(raw))
    def map_before(frame, value):
        require(value['map_before']==map_binding(frame),'conserved_map_before')
        if 'prior_rows' in value:
            require([row(x) for x in value['prior_rows']]==[x for x in maps[frame] if x[0]==value['path']],'conserved_prior_rows')
    def finished(operation, allow_manual_pending=False):
        require(operation in operations,'complete_operation_begin')
        value=operations[operation]
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
        range_children=[name for name,data in frames.items() if data['parent_operation']==operation and name in range_branches]
        if range_children:
            require(operation in I_ready and I_consumed.get(operation,[])==value['paths'],'fixed3071_I_complete_ledger_consumed_before_collapse')
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
            require(master_entry is not None and frames[stack[0]]['base']==master_entry['base'] and frames[stack[0]]['target']==master_entry['source'], 'M_real_master_ancestor_context')
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
                require([v[k] for k in ('base','target','allow_direct_followup','partition_source_history')]==[master_entry['base'],master_entry['source'],'true','true'],'genuine_original_master_source_frame')
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
            require(op not in J_capabilities or op in J_ready,'fixed3071_unqualified_ledger_cannot_begin_mutation')
            map_before(frame,v)
            prior=row(v['prior']);incoming=row(v['incoming'])
            require(key not in pending and incoming[0]==v['path'] and incoming[2]==operations[op]['commit'],'actual_incoming_operation')
            operation=operations[op]
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
            if op in I_ready:cumulative[key]|=I_ready[op]['required_qualification'][v['path']]['aggregate']
            if op in J_ready:cumulative[key]|=J_ready[op]['required_qualification'][v['path']]['aggregate']
            obligations.setdefault(key,[]).append(dict(operation=op,classification=classified[op],prior_rows=v['prior_rows'],incoming=v['incoming'],child_aggregate=child_flags.get((op,v['path']),False),
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
    checkpoint_state=M_checkpoint_state(checkpoint,events) if checkpoint else range_state or I_state or J_history_state or J_state or deletion_state_value or manual_state or manual_end_value or manual_append_value or exact_J_permission
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
        exact_J_permission=exact_J_permission,
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
        if command=='boundary':print(json.dumps(exact_boundary(*args)));return 0
        if command=='composition':print(json.dumps(composition(*args)));return 0
        if command=='unit':print(json.dumps(load_unit(*args)));return 0
        if command=='capabilities':print(json.dumps(prepare_capabilities(*args)));return 0
        if command=='current-author-capabilities':print(json.dumps(prepare_author_capabilities(*args)));return 0
        if command=='production-capabilities':print(json.dumps(prepare_production_capabilities(*args)));return 0
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
