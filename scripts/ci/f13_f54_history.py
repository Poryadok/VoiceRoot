"""Finite F54 prototype observations. No approved production R/K exists here.

Append real producer events; validate their exact bytes and conservation. Raw
Git inventories are unit bindings/dispositions, never manufactured owner rows.
Neither history-boundary nor composition observation grants admission.
"""
import argparse
import base64
import hashlib
import json
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
SCHEMA = 'F54-PRODUCER-OBSERVATION-v1'
ROOT = Path(__file__).resolve().parents[2]
PRODUCTION_AUTHORITY = False


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
        parent, start, target, allow, partition, output = values
        identity = 'F' + str(sequence)
        payload = dict(id=identity, parent=parent or None, repo=str(Path(repo).resolve()), base=start, target=target,
            allow_direct_followup=allow, partition_source_history=partition, output=output,
            first_parent_operations=git(repo,'rev-list','--first-parent','--reverse',start+'..'+target).decode().splitlines(),
            initial_map=file_binding(output))
    elif kind == 'frame_exit':
        code, output, category = values
        payload = dict(exit=int(code), final_map=file_binding(output), category=Path(category).read_text().strip() if category and Path(category).is_file() else None)
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
    else: raise Rejected('unknown_event_kind')
    e=dict(schema=SCHEMA,sequence=sequence,previous=previous['digest'] if previous else '0'*64,
        kind=kind,frame=identity if kind=='frame_enter' else frame or None,
        operation=identity if kind=='operation_begin' else operation or None,payload=payload)
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


def validate_trace(repo, path, expected_sha, complete=False):
    raw=Path(path).read_bytes();require(sha(raw)==expected_sha,'trace_raw_hash')
    events=[json.loads(v) for v in raw.splitlines()];require(events,'nonempty_producer_trace')
    previous='0'*64;frames={};stack=[];operations={};classified={};pending={};cumulative={};failures=[];observed=set()
    for sequence,e in enumerate(events,1):
        require(set(e)=={'schema','sequence','previous','kind','frame','operation','payload','digest'} and e['schema']==SCHEMA,'producer_event_schema')
        value=dict(e);digest=value.pop('digest')
        require(e['sequence']==sequence and e['previous']==previous and sha(canonical(value))==digest,'append_only_chain');previous=digest
        kind=e['kind'];v=e['payload'];frame=e['frame'];op=e['operation']
        if kind=='frame_enter':
            identity=v['id'];require(identity not in frames and v['parent']==(stack[-1] if stack else None),'frame_parent_and_order')
            actual=git(repo,'rev-list','--first-parent','--reverse',v['base']+'..'+v['target']).decode().splitlines()
            require(v['first_parent_operations']==actual,'actual_frame_operations');frames[identity]=v;stack.append(identity)
            continue
        require(stack and frame==stack[-1],'current_live_frame')
        if kind=='frame_exit':
            require(v['exit']!=0 or not any(key[0]==frame for key in pending),'row_without_commit')
            if v['exit']!=0: failures.append(dict(frame=frame,category=v['category']))
            pending={key:value for key,value in pending.items() if key[0]!=frame}
            stack.pop();continue
        if kind=='operation_begin':
            require(op==v['id'] and v['id'] not in operations and v['commit'] in frames[frame]['first_parent_operations'],'actual_unique_operation')
            parents=git(repo,'rev-list','--parents','-n','1',v['commit']).decode().split()[1:]
            require(v['ordered_parents']==parents and v['first_parent']==parents[0],'actual_ordered_parents')
            index=v['parent_index'];require(type(index) is int and 1<=index<=len(parents),'parent_index')
            require((len(parents)==1 and index==1 and v['source_head']==v['commit']) or (len(parents)>1 and index>=2 and v['source_head']==parents[index-1]),'actual_source_parent')
            require(v['delta']==raw_binding(git(repo,'diff','--name-only',v['boundary'],v['source_head'])),'actual_operation_delta')
            for name,entry in v['entries'].items():
                require(entry==dict(base=entry_binding(repo,v['boundary'],name),source=entry_binding(repo,v['source_head'],name),result=entry_binding(repo,v['commit'],name)),'actual_full_entries')
            require(v['paths']==v['delta']['raw'].splitlines() and set(v['entries'])==set(v['paths']),'complete_delta_entries')
            operations[v['id']]=v;observed.add(v['commit']);continue
        if kind in ('boundary_observation','composition_observation'):require(v['admission'] is False,'no_production_capability');continue
        if kind=='failure': failures.append(dict(frame=frame,category=v['category']));continue
        if kind=='final_gate_before': continue
        require(op in operations,'referenced_real_operation')
        if kind=='classification':
            require(op not in classified and v['classifier']==classifier_binding(),'original_classifier_identity')
            require(v['trigger']==classify(v['unapproved_delta']['raw'].splitlines()),'actual_raw_classifier_flag')
            require(v['unapproved_delta']==raw_binding(v['unapproved_delta']['raw']),'actual_delta_bytes')
            classified[op]=v;continue
        if kind in ('path_gate_before','sync_gate_before'):continue
        if kind=='disposition':require(v['operation_reference']==op and v['reason'],'actual_disposition_reference');continue
        require(op in classified,'classification_before_row')
        key=(frame,v['path'])
        if kind=='row_before':
            prior=row(v['prior']);incoming=row(v['incoming'])
            require(key not in pending and incoming[0]==v['path'] and incoming[2]==operations[op]['commit'],'actual_incoming_operation')
            pending[key]=dict(prior=prior,incoming=incoming,op=op,after=None)
            previous_rows=[row(value) for value in v['prior_rows']]
            require((previous_rows[-1] if previous_rows else None)==prior,'actual_prior_row_order')
            cumulative[key]=cumulative.get(key,False) or any(p[1]=='true' for p in previous_rows) or incoming[1]=='true' or classified[op]['trigger']=='true'
        elif kind in ('row_after','row_remove'):
            require(key in pending and pending[key]['op']==op,'pre_mutation_prior_reference')
            parts=row(v['row']);require(parts[6]==pending[key]['incoming'][6],'original_content_base')
            require(parts[1]==('true' if cumulative[key] else 'false'),'cumulative_operation_or')
            if kind=='row_after':pending[key]['after']=parts
        elif kind=='row_committed':
            require(key in pending and pending[key]['after']==row(v['row']),'actual_committed_row')
            pending.pop(key)
        else:raise Rejected('unknown_trace_kind')
    require(not stack,'closed_frames')
    dag=set(git(repo,'rev-list',M,'^'+O).decode().splitlines()) if complete else set()
    coverage=[]
    for node in sorted(dag):
        refs=[key for key,v in operations.items() if v['commit']==node]
        bases=[key for key,v in frames.items() if subprocess.run(['git','-C',str(repo),'merge-base','--is-ancestor',node,v['base']],capture_output=True).returncode==0]
        coverage.append(dict(node=node,disposition='visited_operation' if refs else 'observed_frame_base' if bases else 'UNREACHED',references=refs or bases))
    require(not complete or (len(dag)==33 and all(v['disposition']!='UNREACHED' for v in coverage) and not failures),'complete_33_real_producer_coverage_and_gates')
    return dict(schema=SCHEMA,trace_sha256=expected_sha,frames=len(frames),operations=len(operations),failures=failures,
        coverage=coverage,observed_qualification='COMPLETE' if complete else 'OBSERVED_PREFIX_ONLY',admission=False)


def main():
    try:
        command,*args=sys.argv[1:]
        if command=='append':
            path,kind,frame,op,repo,*values=args;print(append(path,kind,frame,op,repo,values));return 0
        if command=='boundary':print(json.dumps(exact_boundary(*args)));return 0
        if command=='composition':print(json.dumps(composition(*args)));return 0
        if command=='validate':
            repo,path,digest,*flags=args;print(json.dumps(validate_trace(repo,path,digest,bool(flags))));return 0
        raise Rejected('prototype_command')
    except (Rejected,ValueError,KeyError,TypeError,OSError) as exc:
        print('F54 observation rejected: '+str(exc),file=sys.stderr);return 1


if __name__=='__main__':sys.exit(main())
