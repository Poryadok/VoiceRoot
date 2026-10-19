"""Actual producer events, never Git-node rows or record-supplied flags."""
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import shlex
import subprocess
import tempfile
import unittest

from f13_guarded_merge_test import definitions, git

ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / 'scripts/ci/f13_f54_history.py'
SHELL = ROOT / 'scripts/ci/voice-db-runtime-provisioning-contract_test.sh'
RUNTIME = 'src/backend/voice/main.go'
PROTO = 'protos/voice/fixture/v1/fixture.proto'


def module():
    spec = importlib.util.spec_from_file_location('f54', HELPER)
    value = importlib.util.module_from_spec(spec); spec.loader.exec_module(value)
    return value


class ProducerTrace(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(); self.addCleanup(self.temp.cleanup)
        self.folder = Path(self.temp.name); self.repo = self.folder / 'repo'; self.repo.mkdir()
        git(self.repo, 'init', '-q', '-b', 'lane')
        git(self.repo, 'config', 'user.name', 'Finite producer fixture')
        git(self.repo, 'config', 'user.email', 'fixture@example.invalid')
        self.write(RUNTIME, 'package main\n'); self.write(PROTO, 'base\n')
        for name in ('f13_f54_history.py','voice-db-runtime-provisioning-contract_test.sh','voice-r22-runtime-scope.sh'):
            self.write('scripts/ci/'+name,(ROOT/'scripts/ci'/name).read_text())
        self.base = self.commit('base')

    def write(self, path, text):
        p = self.repo / path; p.parent.mkdir(parents=True, exist_ok=True); p.write_text(text)

    def commit(self, message):
        git(self.repo, 'add', '.'); git(self.repo, 'commit', '-qm', message)
        return git(self.repo, 'rev-parse', 'HEAD')

    def collect(self, target=None, cross_scope=False, partition=True, allow=True, origin=None):
        out = self.folder / ('out' + str(len(list(self.folder.iterdir())))); out.mkdir()
        for name in ('game-checkpoint-tree', 'be255-checkpoint-tree'): (out / name).write_text('')
        extra_names = {'f13_cross_scope_path_allowed'}
        extra = '\n'.join(m.group(0) for m in re.finditer(
            r'(?ms)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{\n.*?^\}\n', SHELL.read_text())
            if m.group(1).startswith('f13_f54_') or m.group(1) in extra_names)
        values = dict(ROOT=str(self.repo), TMP_DIR=str(out), GITHUB_EVENT_NAME='pull_request',
            GITHUB_REF='refs/pull/875/merge', native_p3_active='false', game_checkpoint_aggregate_active='false',
            game_checkpoint_snapshot_base='', game_checkpoint_snapshot_head='', game_checkpoint_snapshot_record='',
            game_checkpoint_aggregate_head='', game_checkpoint_aggregate_paths='',
            f13_f54_trace_file=str(out / 'trace.jsonl'), f13_f54_observer=str(HELPER))
        program = 'set -eu\n' + definitions() + '\n' + extra + '\n'
        program += '\n'.join(k + '=' + shlex.quote(v) for k, v in values.items()) + '\n'
        if origin is not None:program+='f13_f54_trace_event span_link "$ROOT" '+shlex.quote(json.dumps(origin))+'\n'
        program += f'f13_collect_develop_contributions "$ROOT" {self.base} {target or git(self.repo,"rev-parse","HEAD")} "$TMP_DIR/map" "$TMP_DIR/category" {str(allow).lower()} {str(partition).lower()}\n'
        if cross_scope: program += 'f13_cross_scope_path_allowed ' + shlex.quote(PROTO) + ' true\n'
        p = subprocess.run(['bash', '-c', program], capture_output=True, text=True)
        return p, out

    def hidden_runtime(self):
        self.write(RUNTIME, 'package main\n// actual runtime change\n'); self.write(PROTO, 'incoming\n')
        self.first = self.commit('runtime and proto actual operation')
        self.write(RUNTIME, 'package main\n'); self.head = self.commit('runtime reverted')

    def test_runtime_revert_has_real_pre_mutation_events_and_cross_scope_still_fails(self):
        self.hidden_runtime(); p, out = self.collect(cross_scope=True)
        self.assertEqual(p.returncode, 1, p.stderr)
        self.assertTrue((out / 'trace.jsonl').exists(), 'missing actual producer trace, not a raw Git inventory')
        events = [json.loads(v) for v in (out / 'trace.jsonl').read_text().splitlines()]
        self.assertEqual([e['payload']['trigger'] for e in events if e['kind'] == 'classification'], ['true', 'true'])
        before = [e for e in events if e['kind'] == 'row_before' and e['payload']['path'] == RUNTIME]
        self.assertEqual(len(before), 2)
        self.assertIsNone(before[0]['payload']['prior'])
        self.assertEqual(before[1]['payload']['prior']['raw'].split('\t')[2], self.first)
        self.assertEqual(module().validate_trace(self.repo, out / 'trace.jsonl', hashlib.sha256((out/'trace.jsonl').read_bytes()).hexdigest())['admission'], False)
        net = git(self.repo, 'diff', '--name-only', self.base, self.head).splitlines()
        self.assertEqual(net, [PROTO])

    def test_passive_observer_does_not_add_stdout_to_original_producer(self):
        self.hidden_runtime();p,_=self.collect()
        self.assertEqual(p.returncode,0,p.stderr)
        self.assertEqual(p.stdout,'','observer must not change original emitted prefix')

    def test_raw_false_append_preserves_and_consumes_prior_true_ledger(self):
        name='.agent/execplans/append-fixture.md'
        self.write(name,'first\n');self.write(RUNTIME,'package main\n// true contribution\n')
        first=self.commit('actual true operation and nonguarded document')
        self.write(name,'second\n');self.commit('actual false document append')
        p,out=self.collect();self.assertEqual(p.returncode,0,p.stderr)
        raw=(out/'trace.jsonl').read_bytes();m=module()
        result=m.validate_trace(self.repo,out/'trace.jsonl',hashlib.sha256(raw).hexdigest())
        rows=[v.split('\t') for v in (out/'map').read_text().splitlines() if v.startswith(name+'\t')]
        self.assertEqual([v[1] for v in rows],['true','false'])
        self.assertEqual(rows[0][2],first)
        self.assertTrue(m.consume_aggregate(result,'F1',name,(out/'map').read_bytes(),'true')['aggregate'])
        with self.assertRaises(m.Rejected):m.consume_aggregate(result,'F1',name,(out/'map').read_bytes(),'false')
        lost='\t'.join(rows[-1])+'\n'
        with self.assertRaises(m.Rejected):m.consume_aggregate(result,'F1',name,lost.encode(),'false')
        with self.assertRaises(m.Rejected):m.consume_aggregate(result,'F1',name,(out/'map').read_bytes()+('\t'.join(rows[0])+'\n').encode(),'true')
        events=[json.loads(v) for v in raw.splitlines()]
        changed=json.loads(json.dumps(events));e=next(e for e in changed if e['kind']=='row_after' and e['payload']['path']==name and e['payload']['row']['raw'].split('\t')[1]=='false')
        parts=e['payload']['row']['raw'].split('\t');parts[1]='true';e['payload']['row']=m.raw_binding('\t'.join(parts))
        path=out/'normalized-raw-false.jsonl';m.write_events(path,changed)
        with self.assertRaises(m.Rejected):m.validate_trace(self.repo,path,hashlib.sha256(path.read_bytes()).hexdigest())

    def forged_trace(self, m, out, events, name):
        # Replay forged map bindings too: the negative must reach semantic
        # identity checks rather than fail on a stale transport checksum.
        maps={}
        for e in events:
            kind=e['kind'];v=e['payload'];frame=e['frame']
            if kind=='frame_enter':maps[frame]=[]
            if frame not in maps:continue
            def binding():return m.raw_binding(''.join(x+'\n' for x in maps[frame]))
            if 'map_before' in v:
                v['map_before']={k:binding()[k] for k in ('bytes','sha256')}
                if 'prior_rows' in v:
                    prior=[m.raw_binding(x) for x in maps[frame] if x.split('\t')[0]==v['path']]
                    v['prior_rows']=prior;v['prior']=prior[-1] if prior else None
            if kind=='row_remove':maps[frame]=[x for x in maps[frame] if x.split('\t')[0]!=v['path']]
            if kind=='row_committed':
                maps[frame].append(v['row']['raw'])
                v['retained_rows']=[m.raw_binding(x) for x in maps[frame] if x.split('\t')[0]==v['path']]
                v['map_after']={k:binding()[k] for k in ('bytes','sha256')}
            if kind=='frame_exit':v['final_map']={k:binding()[k] for k in ('bytes','sha256')}
        path=out/name;m.write_events(path,events)
        return path,hashlib.sha256(path.read_bytes()).hexdigest()

    def test_direct_boundary_head_cannot_erase_actual_operation_paths(self):
        self.write('one.md','actual source\n');self.commit('one actual source operation')
        p,out=self.collect();self.assertEqual(p.returncode,0,p.stderr)
        m=module();events=[json.loads(v) for v in (out/'trace.jsonl').read_bytes().splitlines()]
        operation=next(e for e in events if e['kind']=='operation_begin')
        operation['payload'].update(boundary=operation['payload']['source_head'],delta=m.raw_binding(''),paths=[],entries={})
        events=[e for e in events if not e['payload'].get('path')]
        next(e for e in events if e['kind']=='classification')['payload']['unapproved_delta']=m.raw_binding('')
        path,digest=self.forged_trace(m,out,events,'boundary-head-empty.jsonl')
        with self.assertRaisesRegex(m.Rejected,'actual_direct_source_boundary'):
            m.validate_trace(self.repo,path,digest)

    def test_reduction_cannot_forge_owner_path_parent_source_or_candidates(self):
        self.hidden_runtime();p,out=self.collect();self.assertEqual(p.returncode,0,p.stderr)
        m=module();original=[json.loads(v) for v in (out/'trace.jsonl').read_bytes().splitlines()]
        target=next(e for e in original if e['kind']=='row_remove')
        for index in (0,2,3,4,5,7):
            with self.subTest(field=index):
                events=json.loads(json.dumps(original))
                for e in events:
                    if e['operation']==target['operation'] and e['payload'].get('path')==target['payload']['path'] and e['kind'] in ('row_remove','row_after','row_committed'):
                        parts=e['payload']['row']['raw'].split('\t');parts[index]='forged.md' if index==0 else '0'*40
                        e['payload']['row']=m.raw_binding('\t'.join(parts))
                path,digest=self.forged_trace(m,out,events,'reduction-field-'+str(index)+'.jsonl')
                with self.assertRaisesRegex(m.Rejected,'actual_reduction_row_identity'):
                    m.validate_trace(self.repo,path,digest)

    def test_actual_attempt_frame_classification_and_candidates_are_bound(self):
        self.write('one.md','actual source\n');self.commit('one actual source operation')
        p,out=self.collect();self.assertEqual(p.returncode,0,p.stderr)
        m=module();original=[json.loads(v) for v in (out/'trace.jsonl').read_bytes().splitlines()]
        for change in ('candidates','begin_classification','attempt_classification','attempt_frame','frame_identity'):
            with self.subTest(change=change):
                events=json.loads(json.dumps(original))
                begin=next(e for e in events if e['kind']=='operation_begin')
                attempt=next(e for e in events if e['kind']=='operation_attempt')
                if change=='candidates':begin['payload']['candidates']=['0'*40]
                elif change=='begin_classification':begin['payload']['classification']='merge'
                elif change=='attempt_classification':attempt['payload']['classification']='merge'
                elif change=='attempt_frame':
                    attempt['operation']='wrong/'+attempt['payload']['commit']+'/1'
                    attempt['payload']['id']=attempt['operation']
                else:next(e for e in events if e['kind']=='frame_enter')['frame']='wrong'
                path,digest=self.forged_trace(m,out,events,change+'.jsonl')
                with self.assertRaises(m.Rejected):m.validate_trace(self.repo,path,digest)

    def test_sync_boundary_retains_actual_attempt_bases_without_aliasing(self):
        common=self.base
        self.write('lane.md','lane\n');lane=self.commit('independent lane')
        git(self.repo,'checkout','-qb','master-side',common)
        self.write('master.md','master\n');self.base=self.commit('later actual frame base')
        git(self.repo,'checkout','-q',lane)
        git(self.repo,'merge','--no-ff','-qm','actual base synchronization',self.base)
        p,out=self.collect();self.assertEqual(p.returncode,0,p.stderr)
        m=module();events=[json.loads(v) for v in (out/'trace.jsonl').read_bytes().splitlines()]
        attempt=next(e for e in events if e['kind']=='operation_attempt' and e['payload']['classification']=='sync')
        begin=next(e for e in events if e['kind']=='operation_begin' and e['payload']['classification']=='sync')
        self.assertEqual(attempt['payload']['candidates'],[common])
        self.assertEqual(begin['payload']['candidates'],[self.base])
        self.assertNotEqual(common,self.base)
        path=out/'trace.jsonl';m.validate_trace(self.repo,path,hashlib.sha256(path.read_bytes()).hexdigest())
        begin['payload']['boundary']=common
        path,digest=self.forged_trace(m,out,events,'forged-sync-base.jsonl')
        with self.assertRaisesRegex(m.Rejected,'actual_sync_source_boundary'):m.validate_trace(self.repo,path,digest)

    def bound_span_fixture(self, hidden_runtime=False):
        self.write('README','first lane\n');first_lane=self.commit('first lane')
        git(self.repo,'checkout','-qb','span',self.base)
        if hidden_runtime:self.hidden_runtime()
        else:
            self.write('span-doc.md','first\n');self.write(PROTO,'actual source\n');self.first=self.commit('first actual source')
            self.write('span-doc.md','second\n');self.head=self.commit('second actual source')
        source=self.head
        git(self.repo,'checkout','-q',first_lane);git(self.repo,'merge','--no-ff','-qm','original aggregate',source)
        aggregate=git(self.repo,'rev-parse','HEAD')
        p,original=self.collect(partition=False);self.assertEqual(p.returncode,0,p.stderr)
        original_raw=(original/'trace.jsonl').read_bytes();m=module()
        # Exact private fixture constants only; no production pin/record flag.
        m.SPAN_BASE,m.SPAN_HEAD,m.SPAN_AGGREGATE,m.SPAN_NODES=self.base,source,aggregate,[self.first,self.head]
        m.ORIGINAL_TRACE_SHA=hashlib.sha256(original_raw).hexdigest();m.ORIGINAL_PRODUCER_SHA=m.classifier_binding()['producer_sha256'];m.PROTOTYPE_BASE=aggregate
        old=dict(base=first_lane,consumer='MASTER-SOURCE-v1',version='NATIVE-P3-SOURCE-v1',snapshot=self.base,source=aggregate,target=aggregate,paths=[PROTO,RUNTIME])
        old_path=self.folder/'original-native.json';old_path.write_text(json.dumps(old));m.ORIGINAL_NATIVE_SHA=hashlib.sha256(old_path.read_bytes()).hexdigest()
        target=git(self.repo,'commit-tree',aggregate+'^{tree}','-p',first_lane,'-p',aggregate,'-m','private target')
        git(self.repo,'checkout','-q',target);current_path=self.folder/'current-native.json';current_path.write_text(json.dumps(dict(old,target=target)))
        binding=m.span_origin(self.repo,original/'trace.jsonl',old_path,current_path)
        return m,original,original_raw,aggregate,source,old_path,current_path,binding

    def test_genuine_supplemental_span_keeps_original_aggregate_owner(self):
        m,original,original_raw,aggregate,source,old_path,current_path,binding=self.bound_span_fixture()
        p,span=self.collect(target=source,allow=False,origin=binding);self.assertEqual(p.returncode,0,p.stderr)
        digest=hashlib.sha256((span/'trace.jsonl').read_bytes()).hexdigest()
        result=m.validate_aggregate_span(self.repo,original/'trace.jsonl',span/'trace.jsonl',digest,old_path,current_path)
        self.assertEqual([v['node'] for v in result['dispositions']],m.SPAN_NODES)
        self.assertTrue(all(v['disposition']=='aggregate_span_qualified' for v in result['dispositions']))
        self.assertFalse(m.consume_span_aggregate(result,PROTO,'false')['aggregate'])
        with self.assertRaises(m.Rejected):m.consume_span_aggregate(result,PROTO,'true')
        original_owner=next(v.split('\t')[2] for v in (original/'map').read_text().splitlines() if v.startswith(PROTO+'\t'))
        self.assertEqual(original_owner,aggregate)
        self.assertEqual((original/'trace.jsonl').read_bytes(),original_raw)
        events=[json.loads(v) for v in (span/'trace.jsonl').read_text().splitlines()]
        for change in ('net_delta_only','missing_link','wrong_hash','wrong_context','missing_second','forged_failure','wrong_partition'):
            with self.subTest(change=change):
                altered=json.loads(json.dumps(events))
                if change=='net_delta_only':next(e for e in altered if e['kind']=='classification')['payload']['unapproved_delta']=m.raw_binding('')
                elif change=='missing_link':altered=[e for e in altered if e['kind']!='span_link']
                elif change=='wrong_hash':altered[0]['payload']['original_trace_sha256']='0'*64
                elif change=='wrong_context':altered[0]['payload']['current_native']['snapshot']='0'*40
                elif change=='missing_second':altered=[e for e in altered if e['operation'] and self.head not in e['operation'] or e['operation'] is None]
                elif change=='forged_failure':next(e for e in altered if e['kind']=='frame_exit')['payload']['exit']=1
                else:next(e for e in altered if e['kind']=='frame_enter')['payload']['partition_source_history']='false'
                path=span/(change+'.jsonl');m.write_events(path,altered)
                with self.assertRaises((m.Rejected,KeyError)):
                    m.validate_aggregate_span(self.repo,original/'trace.jsonl',path,hashlib.sha256(path.read_bytes()).hexdigest(),old_path,current_path)

    def test_hidden_runtime_revert_span_retains_true_and_real_gate_failure(self):
        m,original,original_raw,aggregate,source,old_path,current_path,binding=self.bound_span_fixture(True)
        p,span=self.collect(target=source,allow=False,origin=binding)
        self.assertEqual(p.returncode,1,p.stderr)
        self.assertEqual((span/'category').read_text().strip(),'duplicate_guarded_owner')
        raw=(span/'trace.jsonl').read_bytes();digest=hashlib.sha256(raw).hexdigest()
        prefix=m.validate_trace(self.repo,span/'trace.jsonl',digest)
        frame=next(iter(prefix['ledger']))
        self.assertTrue(prefix['ledger'][frame]['paths'][PROTO]['aggregate'])
        with self.assertRaises(m.Rejected):m.consume_aggregate(prefix,frame,PROTO,(span/'map').read_bytes(),'false')
        with self.assertRaises(m.Rejected):m.validate_aggregate_span(self.repo,original/'trace.jsonl',span/'trace.jsonl',digest,old_path,current_path)
        altered=[e for e in [json.loads(v) for v in raw.splitlines()] if e['kind']!='failure']
        next(e for e in altered if e['kind']=='frame_exit')['payload'].update(exit=0,category=None)
        path=span/'failure-relabeled-success.jsonl';m.write_events(path,altered)
        with self.assertRaises(m.Rejected):m.validate_aggregate_span(self.repo,original/'trace.jsonl',path,hashlib.sha256(path.read_bytes()).hexdigest(),old_path,current_path)
        self.assertEqual((original/'trace.jsonl').read_bytes(),original_raw)

    def test_rehashed_drop_reorder_forgery_and_raw_dag_do_not_substitute_trace(self):
        self.hidden_runtime(); p, out = self.collect(); self.assertEqual(p.returncode, 0, p.stderr)
        m = module(); original = [json.loads(v) for v in (out/'trace.jsonl').read_text().splitlines()]
        for change in ('whole_operation', 'whole_path', 'drop_classification', 'reorder', 'duplicate', 'flag', 'content_base', 'incoming_source', 'parent_index', 'raw_dag'):
            with self.subTest(change=change):
                events = json.loads(json.dumps(original))
                if change == 'whole_operation':
                    removed=next(e['operation'] for e in events if e['kind']=='operation_begin')
                    events=[e for e in events if e['operation']!=removed]
                elif change == 'whole_path':
                    events=[e for e in events if e['payload'].get('path')!=PROTO]
                elif change == 'drop_classification': events = [e for e in events if e['kind'] != 'classification']
                elif change == 'reorder': events[1], events[2] = events[2], events[1]
                elif change == 'duplicate': events.insert(2, events[1])
                elif change == 'flag': next(e for e in events if e['kind']=='classification')['payload']['trigger']='false'
                elif change == 'content_base':
                    e = next(e for e in events if e['kind']=='row_after'); parts=e['payload']['row']['raw'].split('\t');parts[6]=self.head
                    e['payload']['row'] = m.raw_binding('\t'.join(parts))
                elif change == 'incoming_source':
                    e=next(e for e in events if e['kind']=='row_before');parts=e['payload']['incoming']['raw'].split('\t');parts[5]='0'*40
                    e['payload']['incoming']=m.raw_binding('\t'.join(parts))
                elif change == 'parent_index': next(e for e in events if e['kind']=='operation_begin')['payload']['parent_index']=2
                else: events=[dict(commit=self.first),dict(commit=self.head)]
                path = out / (change + '.jsonl'); m.write_events(path, events)
                with self.assertRaises((m.Rejected, KeyError, TypeError)):
                    m.validate_trace(self.repo,path,hashlib.sha256(path.read_bytes()).hexdigest())
        with self.assertRaises(m.Rejected): m.validate_trace(self.repo,out/'trace.jsonl','0'*64)

    def test_side_frame_runtime_history_retains_parent_operation_and_true_proto_flag(self):
        self.write('README','first lane\n');first=self.commit('first parent lane')
        git(self.repo,'checkout','-qb','side',self.base);self.hidden_runtime()
        source=self.head;git(self.repo,'checkout','-q',first)
        git(self.repo,'merge','--no-ff','-qm','real side history merge',source)
        p,out=self.collect(cross_scope=True);self.assertEqual(p.returncode,1,p.stderr)
        events=[json.loads(v) for v in (out/'trace.jsonl').read_text().splitlines()]
        child=next(e for e in events if e['kind']=='frame_enter' and e['payload']['parent'])
        parent=next(e for e in events if e['kind']=='operation_begin' and e['frame']==child['payload']['parent'] and e['payload']['commit']==git(self.repo,'rev-parse','HEAD'))
        self.assertEqual(child['payload']['parent_operation'],parent['payload']['id'])
        final=next(v.split('\t') for v in (out/'map').read_text().splitlines() if v.startswith(PROTO+'\t'))
        self.assertEqual(final[1],'true')
        original_trigger=next(e for e in events if e['kind']=='classification' and e['operation']==parent['payload']['id'])
        self.assertEqual(original_trigger['payload']['trigger'],'false')
        m=module();self.assertFalse(m.validate_trace(self.repo,out/'trace.jsonl',hashlib.sha256((out/'trace.jsonl').read_bytes()).hexdigest())['admission'])
        child_id=child['payload']['id']
        dropped=[e for e in events if e['frame']!=child_id]
        path=out/'whole-child-dropped.jsonl';m.write_events(path,dropped)
        with self.assertRaises(m.Rejected):
            m.validate_trace(self.repo,path,hashlib.sha256(path.read_bytes()).hexdigest())

    def test_same_bytes_independent_owners_remain_a_duplicate_failure(self):
        self.write(PROTO, 'same final\n');self.write(RUNTIME,'package main\n// earlier true operation\n')
        first = self.commit('first real owner and actual runtime taint')
        git(self.repo,'checkout','-qb','other',self.base)
        self.write(PROTO,'same final\n'); other=self.commit('independent same bytes owner')
        git(self.repo,'checkout','-q',first);git(self.repo,'merge','--no-ff','-qm','actual shared bytes merge',other)
        p,out=self.collect();self.assertEqual(p.returncode,1,p.stderr)
        self.assertEqual((out/'category').read_text().strip(),'duplicate_guarded_owner')
        events=[json.loads(v) for v in (out/'trace.jsonl').read_text().splitlines()]
        self.assertTrue(any(e['kind']=='failure' and e['payload']['category']=='duplicate_guarded_owner' for e in events))
        self.assertTrue(any(e['kind']=='row_before' and e['payload']['prior'] and
            e['payload']['prior']['raw'].split('\t')[1:3]==['true',first] and
            e['payload']['incoming']['raw'].split('\t')[1]=='false' for e in events))

    def test_scalar_only_crisscross_still_fails_single_best_seam(self):
        self.write(RUNTIME,'package main\n// A\n'); a=self.commit('A runtime')
        git(self.repo,'checkout','-qb','B',self.base);self.write(PROTO,'B\n');b=self.commit('B proto')
        git(self.repo,'checkout','-q',a);git(self.repo,'merge','--no-ff','-qm','AB',b);p1=git(self.repo,'rev-parse','HEAD')
        git(self.repo,'checkout','-q',b);git(self.repo,'merge','--no-ff','-qm','BA',a);p2=git(self.repo,'rev-parse','HEAD')
        self.assertEqual(set(git(self.repo,'merge-base','--all',p1,p2).splitlines()),{a,b})
        out=self.folder/'scalar';out.mkdir()
        command='set -eu\n'+definitions()+f'\nTMP_DIR={shlex.quote(str(out))}\nf13_checkpoint_merge_tree_entry_matches {shlex.quote(str(self.repo))} {self.base} {p1} {p2} {PROTO} unused\n'
        p=subprocess.run(['bash','-c',command],capture_output=True,text=True)
        self.assertEqual(p.returncode,1,p.stderr)

    def test_fixed_private_composition_observation_is_distinct_and_cannot_admit(self):
        self.write(RUNTIME,'package main\n// A\n');a=self.commit('A runtime')
        git(self.repo,'checkout','-qb','B',self.base);self.write(PROTO,'B\n');b=self.commit('B proto')
        git(self.repo,'checkout','-q',a);git(self.repo,'merge','--no-ff','-qm','AB',b);p1=git(self.repo,'rev-parse','HEAD')
        git(self.repo,'checkout','-q',b);git(self.repo,'merge','--no-ff','-qm','BA',a);p2=git(self.repo,'rev-parse','HEAD')
        git(self.repo,'checkout','-q',p1);git(self.repo,'merge','--no-ff','-qm','actual composed M',p2);merge=git(self.repo,'rev-parse','HEAD')
        m=module()
        # In-memory fixture-only constants; no production file, ROOT pin or
        # caller-selected record is edited or accepted.
        m.M,m.P1,m.P2,m.O,m.BASES,m.TREE=merge,p1,p2,self.base,git(self.repo,'merge-base','--all',p1,p2).splitlines(),git(self.repo,'rev-parse',merge+'^{tree}')
        history=m.exact_boundary(self.repo,merge,p1,p2)
        raw=git(self.repo,'ls-tree',merge,'--',PROTO)
        composition=m.composition(self.repo,self.base,p1,p2,PROTO,raw)
        self.assertNotEqual(history['type'],composition['type'])
        self.assertFalse(history['admission']);self.assertFalse(composition['admission']);self.assertFalse(m.PRODUCTION_AUTHORITY)
        for args in ((merge,p2,p1),(p1,p1,p2)):
            with self.assertRaises(m.Rejected):m.exact_boundary(self.repo,*args)
        with self.assertRaises(m.Rejected):m.composition(self.repo,a,p1,p2,PROTO,raw)
        with self.assertRaises(m.Rejected):m.composition(self.repo,self.base,p1,p2,PROTO,raw.replace('100644','100755'))
        m.BASES.reverse()
        with self.assertRaises(m.Rejected):m.exact_boundary(self.repo,merge,p1,p2)
        unit=self.folder/'unit';unit.mkdir();(unit/'corrected-contract-v3-PROPOSAL.json').write_text('{}')
        with self.assertRaises(m.Rejected):m.load_unit(self.repo,unit)


if __name__ == '__main__': unittest.main()
