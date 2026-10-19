"""Actual producer events, never Git-node rows or record-supplied flags."""
import hashlib
import ast
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
    def test_original_full_router_keeps_auth_exception_and_rejects_forbidden_revert_control(self):
        self.write('scripts/ci/t31-r22-scope-allowlist.txt',(ROOT/'scripts/ci/t31-r22-scope-allowlist.txt').read_text())
        self.base=self.commit('private original policy context')
        auth='protos/voice/auth/v1/auth.proto'
        self.write(auth,'private auth contract\n');self.hidden_runtime()
        result,out=self.collect();self.assertEqual(result.returncode,0,result.stderr)
        (out/'f13-contributions').write_bytes((out/'map').read_bytes())
        m=module()
        context=dict(values=dict(ROOT=str(self.repo),TMP_DIR=str(out),base_sha=self.base,
            accepted_game_checkpoint='',game_checkpoint_enabled='false',game_checkpoint_aggregate_map='',
            native_p3_active='false',r23_allowlist_enabled='false',r22_runtime_delta='true'),event={})
        allowed=m.execute_original_router(context,dict(path=auth,aggregate=True,covered=True))
        self.assertEqual((allowed['gate'],allowed['status']),('space_media_path_allowed',0))
        self.assertEqual(m.original_scope_exit(auth,'true'),1)
        # This proto is outside every applicable earlier original exception.
        # The real operation contained runtime, subsequently reverted to base.
        forbidden=m.execute_original_router(context,dict(path=PROTO,aggregate=True,covered=True))
        self.assertEqual((forbidden['gate'],forbidden['status']),('original_remaining_path_policy',1))

    def test_M_consumer_uses_prior_true_and_original_gate_without_flag_rewrite(self):
        # Private transport unit test only. Actual constructor/trace validation
        # is separate; these hand-built inputs never claim observed M rows.
        m=module();op='F1/'+m.M+'/2';name=PROTO
        prior='\t'.join([name,'true','prior-owner','first','base','source','content','best'])
        incoming='\t'.join([name,'false',m.M,m.P1,m.O,m.P2,'content',','.join(m.BASES)])
        prefix=b'private-constructor-prefix\n';trace=self.folder/'consumer-trace'
        # The bound constructor prefix is opaque here; the observed caller
        # supplies a real JSONL prefix in the actual producer proof.
        prefix=b'{"kind":"private-fixture-prefix"}\n'
        before=dict(kind='row_before',operation=op,sequence=2,payload=dict(path=name,
            incoming=m.raw_binding(incoming),prior_rows=[m.raw_binding(prior)],map_before=dict(bytes=len((prior+'\n').encode()),sha256=m.sha((prior+'\n').encode()))))
        trace.write_bytes(prefix+json.dumps(before).encode()+b'\n')
        evidence=dict(aggregate=True,retained_evidence=[dict(frame='F1',evidence=[dict(prior_rows=[m.raw_binding(prior)])])],span_evidence=None,M_classification=None)
        ready=dict(type='F54_ACTUAL_M_COMPLETE_RETAINED_LEDGER',operation=op,admission=False,
            prefix=dict(bytes=len(prefix),sha256=m.sha(prefix)),operation_binding=dict(paths=[name]),
            outer_rows=[prior],required_qualification={name:evidence})
        ledger=self.folder/'consumer-ledger';ledger.write_bytes(m.canonical(ready));digest=m.sha(ledger.read_bytes())
        receipt=m.consume_M_path(ledger,digest,trace,name,(prior+'\n').encode(),incoming)
        self.assertTrue(receipt['aggregate']);self.assertEqual(receipt['after_gate_row']['raw'],incoming)
        self.assertEqual(m.original_scope_exit(name,'true'),1)
        self.assertEqual(m.original_scope_exit(name,'false'),0)
        # Prior true must not disappear just because the current raw row is false.
        for map_bytes,path,after,expected_hash in ((b'',name,incoming,digest),
                ((prior+'\n').encode(),name+'-future',incoming,digest),
                ((prior+'\n').encode(),name,incoming.replace(m.M,'forged-owner'),'0'*64),
                ((prior+'\n').encode(),name,incoming.replace(m.P2,m.M),digest)):
            with self.assertRaises(m.Rejected):m.consume_M_path(ledger,expected_hash,trace,path,map_bytes,after)
        self.assertEqual(receipt['obligations'],evidence)

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

    def exact_M_fixture(self):
        # Entirely private, internally re-pinned complete source unit. These
        # constants/files never leave this temporary ROOT or authorize prod.
        carrier_shell='scripts/ci/voice-db-runtime-provisioning-contract_test.sh'
        names=sorted(['protos/voice/fixture/v1/cap'+str(i)+'.proto' for i in range(15)]+
                     ['docs/fixture/cap'+str(i)+'.md' for i in range(25)]+[carrier_shell])
        original_shell=(self.repo/carrier_shell).read_text()
        def content(name,side):
            if name==carrier_shell:return original_shell+'\n# fixture '+('A' if side=='A' else 'left')+'\n# fixture stable\n# fixture '+('B' if side=='B' else 'right')+'\n'
            return ('A' if side=='A' else 'left')+'\nstable\n'+('B' if side=='B' else 'right')+'\n'
        for name in names:self.write(name,content(name,'base'))
        common=self.commit('complete41 source base')
        for name in names:self.write(name,content(name,'A'))
        self.commit('A actual source')
        for i in range(14):git(self.repo,'commit','--allow-empty','-qm','A actual empty operation '+str(i))
        a=git(self.repo,'rev-parse','HEAD')
        git(self.repo,'checkout','-qb','B-cap',common)
        for name in names:self.write(name,content(name,'B'))
        self.commit('B actual source')
        for i in range(14):git(self.repo,'commit','--allow-empty','-qm','B actual empty operation '+str(i))
        b=git(self.repo,'rev-parse','HEAD')
        git(self.repo,'checkout','-q',a);git(self.repo,'merge','--no-ff','-qm','AB actual',b);p1=git(self.repo,'rev-parse','HEAD')
        git(self.repo,'checkout','-q',b);git(self.repo,'merge','--no-ff','-qm','BA actual',a);p2=git(self.repo,'rev-parse','HEAD')
        git(self.repo,'checkout','-q',p1);git(self.repo,'merge','--no-ff','-qm','M actual composition',p2);merge=git(self.repo,'rev-parse','HEAD')
        m=module();m.M,m.P1,m.P2,m.O,m.BASES,m.TREE=merge,p1,p2,common,git(self.repo,'merge-base','--all',p1,p2).splitlines(),git(self.repo,'rev-parse',merge+'^{tree}')
        m.SOURCE=m.PROTOTYPE_BASE=merge;m.ROOT=self.repo
        revisions=dict(zip(('older_common_base','best_base_a','best_base_b','first_parent','source_head','merge_result','native_H53','qualified_H4','current_consumer'),(common,*m.BASES,p1,p2,merge,merge,merge,merge)))
        entries=[]
        for name in names:
            versions={}
            for label,revision in revisions.items():
                mode,kind,rest=git(self.repo,'ls-tree',revision,'--',name).split(maxsplit=2)
                versions[label]=dict(mode=mode,type=kind,oid=rest.split('\t')[0])
            entries.append(dict(path=name,versions=versions))
        folder=self.folder/'complete-unit';folder.mkdir();artifacts=[]
        for i in range(177):
            name='fixture-fact-'+str(i)+'.json';raw=json.dumps(dict(fixture=True,index=i,merge=merge)).encode();(folder/name).write_bytes(raw)
            artifacts.append(dict(name=name,bytes=len(raw),sha256=hashlib.sha256(raw).hexdigest()))
        packet=dict(older_common_base=common,current_consumer=merge,revisions=revisions,entries=entries,all_base_diff_paths=names,traversal_base=common,recursive_target=merge,
                    guarded_base_difference_paths=[n for n in names if n.startswith('protos/')])
        contract=dict(merge=merge,ordered_parents=[p1,p2],complete_best_bases=m.BASES,artifacts=artifacts,
                      complete_candidate_difference_paths=names,guarded_difference_paths=packet['guarded_base_difference_paths'])
        for name in m.UNIT_HASHES:
            raw=json.dumps(contract if name=='contract-PROPOSAL.json' else packet if name=='proposal.json' else dict(private_fixture=True)).encode()
            (folder/name).write_bytes(raw)
        m.UNIT_HASHES={name:hashlib.sha256((folder/name).read_bytes()).hexdigest() for name in m.UNIT_HASHES}
        return m,folder,names

    def test_two_M_capabilities_require_complete_source_unit_and_actual_composition(self):
        m,folder,names=self.exact_M_fixture()
        cap=m.prepare_capabilities(self.repo,folder)
        self.assertNotEqual(cap['history_boundary']['type'],cap['whole_parent_composition']['type'])
        self.assertFalse(cap['admission']);self.assertFalse(m.PRODUCTION_AUTHORITY)
        self.assertEqual(len(cap['complete_dag']),33)
        path=self.folder/'capabilities.json';path.write_text(json.dumps(cap))
        self.assertEqual(m.boundary_capability(self.repo,path,m.M,m.P1,m.P2)['history_boundary'],m.O)
        result=git(self.repo,'ls-tree',m.M,'--',names[0])
        self.assertEqual(m.composition_capability(self.repo,path,m.M,m.O,m.P1,m.P2,names[0],result)['result_tree'],m.TREE)
        for args in ((m.M,m.P2,m.P1),(m.P1,m.P1,m.P2)):
            with self.assertRaises(m.Rejected):m.boundary_capability(self.repo,path,*args)
        for name,value in ((names[0],result.replace('100644','100755')),('future.proto','')):
            with self.assertRaises(m.Rejected):m.composition_capability(self.repo,path,m.M,m.O,m.P1,m.P2,name,value)
        for change in ('parent','bases','tree','unit','dag'):
            altered=json.loads(json.dumps(cap))
            if change=='parent':altered['history_boundary']['parents'].reverse()
            elif change=='bases':altered['history_boundary']['best_bases'].reverse()
            elif change=='tree':altered['whole_parent_composition']['result_tree']='0'*40
            elif change=='unit':altered['source_facts']['artifacts']=176
            else:altered['complete_dag'].pop()
            path.write_text(json.dumps(altered))
            with self.assertRaises(m.Rejected):m.boundary_capability(self.repo,path,m.M,m.P1,m.P2)
        path.write_text(json.dumps(cap));(folder/'fixture-fact-176.json').unlink()
        with self.assertRaises(m.Rejected):m.boundary_capability(self.repo,path,m.M,m.P1,m.P2)

    def test_real_two_seams_consume_separate_complete_capabilities_only(self):
        m,folder,names=self.exact_M_fixture()
        helper=self.repo/'scripts/ci/f13_f54_history.py';raw=helper.read_text();lines=raw.splitlines(keepends=True)
        # Literal replacements in a temporary private ROOT only. Actual source
        # hashes, complete packet and carrier commit are recomputed afterwards.
        bindings={name:getattr(m,name) for name in ('M','P1','P2','O','BASES','TREE','SOURCE','PROTOTYPE_BASE','UNIT_HASHES')}
        changes=[n for n in ast.parse(raw).body if isinstance(n,ast.Assign) and len(n.targets)==1 and isinstance(n.targets[0],ast.Name) and n.targets[0].id in bindings]
        for n in sorted(changes,key=lambda n:n.lineno,reverse=True):
            name=n.targets[0].id;lines[n.lineno-1:n.end_lineno]=[name+' = '+repr(bindings[name])+'\n']
        helper.write_text(''.join(lines));self.commit('private immutable capability carrier')
        cap=self.folder/'two-capabilities.json';cap.write_text(json.dumps(m.prepare_capabilities(self.repo,folder)))
        out=self.folder/'seam';out.mkdir();boundary=out/'boundary'
        result=git(self.repo,'ls-tree',m.M,'--',names[0])
        setup='set -eu\n'+definitions()+'\nTMP_DIR='+shlex.quote(str(out))+'\n'
        resolver='f13_resolve_source_base '+ ' '.join(map(shlex.quote,map(str,(self.repo,m.P1,m.P2,boundary,m.M,cap,helper))))+'\n'
        composition='f13_checkpoint_merge_tree_entry_matches '+' '.join(map(shlex.quote,map(str,(self.repo,m.O,m.P1,m.P2,names[0],result,m.M,cap,helper))))+'\n'
        p=subprocess.run(['bash','-c',setup+resolver+composition],capture_output=True,text=True)
        self.assertEqual(p.returncode,0,p.stderr)
        self.assertEqual(boundary.read_text().strip(),m.O)
        self.assertEqual(Path(str(boundary)+'.candidates').read_text().splitlines(),m.BASES)
        # Removing either separately typed capability restores the ordinary
        # scalar gate rejection; a whole-tree match cannot supply a boundary.
        for command in (
            'f13_resolve_source_base '+' '.join(map(shlex.quote,map(str,(self.repo,m.P1,m.P2,boundary)))),
            'f13_checkpoint_merge_tree_entry_matches '+' '.join(map(shlex.quote,map(str,(self.repo,m.O,m.P1,m.P2,names[0],result))))):
            p=subprocess.run(['bash','-c',setup+command],capture_output=True,text=True)
            self.assertEqual(p.returncode,1,p.stderr)

    def test_historical_shell_fact_and_current_carrier_are_separately_bound(self):
        m,folder,names=self.exact_M_fixture()
        shell='scripts/ci/voice-db-runtime-provisioning-contract_test.sh'
        self.write(shell,(self.repo/shell).read_text()+'\n# new exact verifier carrier\n')
        target=self.commit('new current verifier implementation only')
        cap=m.prepare_capabilities(self.repo,folder)
        change=cap['carrier_context_transition']
        self.assertEqual([v['path'] for v in change],[shell])
        self.assertEqual(change[0]['historical_source'],m.SOURCE)
        self.assertEqual(change[0]['current_carrier'],target)
        self.assertNotEqual(change[0]['historical_entry'],change[0]['current_entry'])
        self.assertEqual(cap['source_facts']['source_context'],m.SOURCE)
        self.write(names[0],'unreviewed product source\n');self.commit('product context drift')
        with self.assertRaises(m.Rejected):m.prepare_capabilities(self.repo,folder)

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
