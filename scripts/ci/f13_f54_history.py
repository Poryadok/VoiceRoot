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
import subprocess
import sys

M = 'f54bcadab4aa19969281d7d35d76ebad91553d4d'
P1 = '0a0024900b5bb3ba2f6fee391615f0f4fa0864e3'
P2 = 'f15d718b2d20837ae5503dc049134b66592b3675'
O = '4d943fac27fd247d136d84ae145440e54485d3ab'
BASES = ['f3feccef198b444699e66c960b84dc9c77ba4bc0', '516e6d9d92f477bbf09b2c66ef2476b6956a9b15']
TREE = '4c24a19991d52acb2507590d4c95ff19c983d359'
SOURCE = 'd38b699d3d3d641fe2415ed7f841dfe1f7360bc1'
SCHEMA = 'F54-PRODUCER-OBSERVATION-v2'
ROOT = Path(__file__).resolve().parents[2]
PRODUCTION_AUTHORITY = False
PROTOTYPE_BASE = '6d1f60f955a79e0378a3532750d209e3a11c8f23'
SPAN_AGGREGATE = '8afe8e90285aa59c19ac011ded7260d2f191fa59'
SPAN_BASE = '2445e81de07681cfd5f1491e4c793aaac747f571'
SPAN_HEAD = '23551116ac5e741d3ec4ff7b412b7c9e248539ff'
SPAN_NODES = ['4f5b30c27cb288b249ef5e517c79c1d766f4e6e1',SPAN_HEAD]
ORIGINAL_TRACE_SHA = 'e2d4184b796496e5d98448c146636b779024a6315f65bc055262c1e029a18c25'
ORIGINAL_PRODUCER_SHA = 'b09dfe5eee5cca0d760c833b8fe32f7547563558f83d235d4fa27bd899ddd7d8'
ORIGINAL_NATIVE_SHA = '0f6b40dce786a964dd8616e12cdbf51c9c9fb2c4b255e147c9df142a53d3c294'
UNIT_HASHES = {
    'corrected-contract-v3-PROPOSAL.json':'62a00947187fa58b2815b13877a2c15a94ecb588c8d5fc06ca46aceb1cc24dde',
    'corrected-history-composition-contract-v3-PROPOSED.md':'04ab09c4094a35e256c4cb6642f54cd24c22f58b166ad013e29f7470953ed943',
    'rudeus410-scalar-adjudication.md':'a4ae57d62556172567844319c00475b25cf61f9276a5580794af17293daa7ffc',
    'contract-PROPOSAL.json':'f8785bc3cc33dfb2ef68f0f3160d9e8862b1e82ce78fdea7ca65b409414edb33',
    'proposal.json':'fb1d3de95269e4e1bd860aa14b6d59d46dec85c87e5783dc3872c2807e39206a'}


class Rejected(ValueError): pass


def require(value, reason):
    if not value: raise Rejected(reason)


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
    elif kind == 'classification':
        trigger, delta = values
        payload = dict(trigger=trigger, unapproved_delta=raw_binding(Path(delta).read_bytes()), classifier=classifier_binding())
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
    elif kind == 'span_link':
        require(previous is None and not frame and not operation,'separate_supplemental_trace')
        payload=json.loads(values[0]);require(payload['admission'] is False,'span_not_admission')
    else: raise Rejected('unknown_event_kind')
    e=dict(schema=SCHEMA,sequence=sequence,previous=previous['digest'] if previous else '0'*64,
        kind=kind,frame=identity if kind=='frame_enter' else frame or None,
        operation=identity if kind in ('operation_attempt','operation_begin') else operation or None,payload=payload)
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


def span_origin(repo, original_path, original_native_path, current_native_path):
    """Fixed old observation is provenance, never an admission record.

    Only this exact missing two-node span is permitted. Current native pins
    must be identical to the observed H4 context; a newer H5 is separate work.
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
    require({k:v for k,v in old.items() if k not in ('source','target')}=={k:v for k,v in current.items() if k not in ('source','target')},'unchanged_inherited_native_pins')
    target=git(repo,'rev-parse','HEAD').decode().strip();target_parents=git(repo,'rev-list','--parents','-n','1',target).decode().split()[1:]
    require(current['target']==target and target_parents==[current['base'],current['source']],'actual_supplemental_current_target')
    require(git(repo,'rev-parse',target+'^{tree}').strip()==git(repo,'rev-parse',current['source']+'^{tree}').strip(),'supplemental_source_result_parity')
    for name in ('f13_f54_history.py','voice-db-runtime-provisioning-contract_test.sh','voice-r22-runtime-scope.sh'):
        path='scripts/ci/'+name
        require((ROOT/path).read_bytes()==git(repo,'show',target+':'+path),'actual_supplemental_observer_source_bytes')
    allowed={'scripts/ci/f13_f54_history.py','scripts/ci/f13_f54_history_test.py','scripts/ci/voice-db-runtime-provisioning-contract_test.sh','.agent/execplans/native-p3-master-source.md'}
    require(set(git(repo,'diff','--name-only',PROTOTYPE_BASE,current['source']).decode().splitlines())<=allowed,'no_foreign_context_or_pin_drift')
    retained=[e for e in events if e['kind']=='row_committed' and e['operation']==reference]
    require([e['payload']['path'] for e in retained]==value['paths'],'original_aggregate_all_committed_paths')
    return dict(type='F54_FIXED_AGGREGATE_SPAN_OBSERVATION',original_trace_sha256=ORIGINAL_TRACE_SHA,
        original_operation=operation,original_frame=frame,original_classification=classified,
        original_committed=retained,original_native_sha256=ORIGINAL_NATIVE_SHA,current_native=current,
        current_native_sha256=sha(current_raw),supplemental_classifier=classifier_binding(),
        observer_sha256=sha((ROOT/'scripts/ci/f13_f54_history.py').read_bytes()),
        base=SPAN_BASE,head=SPAN_HEAD,allow_direct_followup='false',partition_source_history='true',admission=False)


def validate_aggregate_span(repo, original_path, span_path, span_sha, original_native_path, current_native_path):
    origin=span_origin(repo,original_path,original_native_path,current_native_path)
    raw=Path(span_path).read_bytes();require(sha(raw)==span_sha,'supplemental_trace_raw_hash')
    events=[json.loads(v) for v in raw.splitlines()]
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


def validate_trace(repo, path, expected_sha, complete=False):
    raw=Path(path).read_bytes();require(sha(raw)==expected_sha,'trace_raw_hash')
    events=[json.loads(v) for v in raw.splitlines()];require(events,'nonempty_producer_trace')
    previous='0'*64;frames={};stack=[];operations={};attempts={};classified={};pending={};cumulative={};child_flags={};child_rows={};frame_rows={};carry={};failures=[];observed=set()
    expected_operations={};seen_operations={};visited={};committed={};maps={};children={};obligations={};native_bindings={}
    def map_binding(frame):
        raw=''.join('\t'.join(parts)+'\n' for parts in maps[frame]).encode()
        return dict(bytes=len(raw),sha256=sha(raw))
    def map_before(frame, value):
        require(value['map_before']==map_binding(frame),'conserved_map_before')
        if 'prior_rows' in value:
            require([row(x) for x in value['prior_rows']]==[x for x in maps[frame] if x[0]==value['path']],'conserved_prior_rows')
    def finished(operation):
        require(operation in operations,'complete_operation_begin')
        value=operations[operation]
        if value['classification']=='sync':
            require(operation in sync_gates,'actual_sync_gate_receipt')
            return
        require(operation in classified,'complete_operation_classification')
        require(visited.get(operation,[])==value['paths'],'complete_ordered_path_visits')
        require(committed.get(operation,[])==value['paths'],'complete_ordered_path_commits')
        if frames[operation.split('/')[0]]['partition_source_history']=='true' and value['classification']=='merge':
            require(children.get(operation)==[(value['boundary'],value['source_head'],'true','false')],'complete_real_child_frame')
    sync_gates=set()
    for sequence,e in enumerate(events,1):
        require(set(e)=={'schema','sequence','previous','kind','frame','operation','payload','digest'} and e['schema']==SCHEMA,'producer_event_schema')
        value=dict(e);digest=value.pop('digest')
        require(e['sequence']==sequence and e['previous']==previous and sha(canonical(value))==digest,'append_only_chain');previous=digest
        kind=e['kind'];v=e['payload'];frame=e['frame'];op=e['operation']
        if kind=='span_link':
            require(sequence==1 and not frames and frame is None and op is None and v['admission'] is False,'separate_span_link_before_execution')
            require(v['supplemental_classifier']==classifier_binding(),'supplemental_collector_context')
            continue
        if kind=='frame_enter':
            identity=v['id'];require(frame==identity and identity not in frames and v['parent']==(stack[-1] if stack else None),'frame_parent_and_order')
            require((not v['parent'] and v['parent_operation'] is None) or v['parent_operation'] in attempts,'actual_parent_operation_reference')
            actual=git(repo,'rev-list','--first-parent','--reverse',v['base']+'..'+v['target']).decode().splitlines()
            require(v['first_parent_operations']==actual,'actual_frame_operations');frames[identity]=v;stack.append(identity)
            maps[identity]=[];seen_operations[identity]=[];expected_operations[identity]=[]
            for commit in actual:
                parents=git(repo,'rev-list','--parents','-n','1',commit).decode().split()[1:]
                expected_operations[identity].extend((commit,index) for index in ([1] if len(parents)==1 else range(2,len(parents)+1)))
            if v['parent_operation']:
                children.setdefault(v['parent_operation'],[]).append((v['base'],v['target'],v['allow_direct_followup'],v['partition_source_history']))
            continue
        require(stack and frame==stack[-1],'current_live_frame')
        if kind=='frame_exit':
            require(v['final_map']==map_binding(frame),'conserved_final_map')
            seen=seen_operations[frame]
            require(seen==expected_operations[frame][:len(seen)],'ordered_actual_operation_prefix')
            if v['exit']==0:require(seen==expected_operations[frame],'complete_actual_operations')
            for commit,index in (seen if v['exit']==0 else seen[:-1]):finished(frame+'/'+commit+'/'+str(index))
            require(v['exit']!=0 or not any(key[0]==frame for key in pending),'row_without_commit')
            if v['exit']!=0: failures.append(dict(frame=frame,category=v['category']))
            parent_operation=frames[frame]['parent_operation']
            if parent_operation and v['exit']==0:
                for (owner,name),flag in cumulative.items():
                    if owner==frame:child_flags[(parent_operation,name)]=child_flags.get((parent_operation,name),False) or flag
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
        if kind=='operation_begin':
            require(op==v['id'] and v['id'] not in operations and v['commit'] in frames[frame]['first_parent_operations'],'actual_unique_operation')
            require(op in attempts and all(v[k]==attempts[op][k] for k in ('commit','ordered_parents','parent_index','first_parent','source_head')),'actual_attempt_before_begin')
            parents=git(repo,'rev-list','--parents','-n','1',v['commit']).decode().split()[1:]
            require(v['ordered_parents']==parents and v['first_parent']==parents[0],'actual_ordered_parents')
            index=v['parent_index'];require(type(index) is int and 1<=index<=len(parents),'parent_index')
            require((len(parents)==1 and index==1 and v['source_head']==v['commit']) or (len(parents)>1 and index>=2 and v['source_head']==parents[index-1]),'actual_source_parent')
            require(v['classification']==attempts[op]['classification'] and v['candidates']==attempts[op]['candidates'],'actual_attempt_classification_and_candidates')
            if v['classification']=='direct':
                require(v['boundary']==v['first_parent'],'actual_direct_source_boundary')
            elif v['classification']=='sync':
                require(v['boundary']==v['source_head']==frames[frame]['base'],'actual_sync_source_boundary')
            else:
                # Multibase admission is a distinct, unfinished capability.
                # A caller-selected scalar must never erase operation paths.
                require(v['candidates']==[v['boundary']],'actual_single_best_source_boundary')
            require(v['delta']==raw_binding(git(repo,'diff','--name-only',v['boundary'],v['source_head'])),'actual_operation_delta')
            for name,entry in v['entries'].items():
                require(entry==dict(base=entry_binding(repo,v['boundary'],name),source=entry_binding(repo,v['source_head'],name),result=entry_binding(repo,v['commit'],name)),'actual_full_entries')
            require(v['paths']==v['delta']['raw'].splitlines() and set(v['entries'])==set(v['paths']),'complete_delta_entries')
            operations[v['id']]=v;observed.add(v['commit']);continue
        if kind in ('boundary_observation','composition_observation','scalar_seam_observation'):
            require(v['admission'] is False,'no_production_capability')
            if kind=='scalar_seam_observation':require(v['status']==1,'scalar_only_still_rejects')
            continue
        if kind=='failure': failures.append(dict(frame=frame,category=v['category']));continue
        if kind=='final_gate_before':map_before(frame,v);continue
        require(op in operations,'referenced_real_operation')
        if kind=='sync_gate_passed':
            require(operations[op]['classification']=='sync' and v['gate']=='f13_validate_sync_merge_resolution' and v['classifier']==classifier_binding(),'actual_sync_gate_identity')
            sync_gates.add(op);continue
        if kind=='classification':
            require(op not in classified and v['classifier']==classifier_binding(),'original_classifier_identity')
            require(v['trigger']==classify(v['unapproved_delta']['raw'].splitlines()),'actual_raw_classifier_flag')
            require(v['unapproved_delta']==raw_binding(v['unapproved_delta']['raw']),'actual_delta_bytes')
            classified[op]=v;continue
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
                spec=importlib.util.spec_from_file_location('f54_native_rebind',ROOT/'scripts/ci/f13_native_p3_source.py')
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
            map_before(frame,v)
            prior=row(v['prior']);incoming=row(v['incoming'])
            require(key not in pending and incoming[0]==v['path'] and incoming[2]==operations[op]['commit'],'actual_incoming_operation')
            operation=operations[op]
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
            obligations.setdefault(key,[]).append(dict(operation=op,classification=classified[op],prior_rows=v['prior_rows'],incoming=v['incoming'],child_aggregate=child_flags.get((op,v['path']),False)))
        elif kind in ('row_after','row_remove'):
            require(key in pending and pending[key]['op']==op,'pre_mutation_prior_reference')
            parts=row(v['row']);require(parts[6]==pending[key]['incoming'][6],'original_content_base')
            incoming=pending[key]['incoming']
            require(all(parts[i]==incoming[i] for i in (0,2,3,6,7)) and parts[4:6]==pending[key].get('rebind',incoming[4:6]),'actual_reduction_row_identity')
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
            require(key in pending and pending[key]['after']==row(v['row']),'actual_committed_row')
            frame_rows[key]=row(v['row'])
            maps[frame].append(row(v['row']))
            require(v['map_after']==map_binding(frame) and [row(x) for x in v['retained_rows']]==[x for x in maps[frame] if x[0]==v['path']],'conserved_committed_map')
            committed.setdefault(op,[]).append(v['path'])
            pending.pop(key)
        else:raise Rejected('unknown_trace_kind')
    require(not stack,'closed_frames')
    coverage=dag_dispositions(repo,events) if complete else []
    accepted={'visited_real_operation','shared_real_operation','exact_single_commit_source_of_real_operation'}
    require(not complete or (all(v['disposition'] in accepted for node in coverage for v in node['operations']) and not failures),'complete_33_real_producer_coverage_and_gates')
    ledger={f:dict(map_binding=map_binding(f),rows=['\t'.join(x) for x in maps[f]],paths={
        name:dict(aggregate=cumulative[(f,name)],evidence=obligations[(f,name)])
        for owner,name in obligations if owner==f}) for f in frames}
    return dict(schema=SCHEMA,trace_sha256=expected_sha,frames=len(frames),operations=len(operations),failures=failures,
        ledger=ledger,coverage=coverage,observed_qualification='COMPLETE' if complete else 'OBSERVED_PREFIX_ONLY',admission=False)


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
        if command=='span-origin':print(json.dumps(span_origin(*args)));return 0
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
