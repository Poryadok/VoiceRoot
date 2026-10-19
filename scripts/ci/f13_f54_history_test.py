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
        self.base = self.commit('base')

    def write(self, path, text):
        p = self.repo / path; p.parent.mkdir(parents=True, exist_ok=True); p.write_text(text)

    def commit(self, message):
        git(self.repo, 'add', '.'); git(self.repo, 'commit', '-qm', message)
        return git(self.repo, 'rev-parse', 'HEAD')

    def collect(self, target=None, cross_scope=False):
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
        program += f'f13_collect_develop_contributions "$ROOT" {self.base} {target or git(self.repo,"rev-parse","HEAD")} "$TMP_DIR/map" "$TMP_DIR/category" true true\n'
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

    def test_rehashed_drop_reorder_forgery_and_raw_dag_do_not_substitute_trace(self):
        self.hidden_runtime(); p, out = self.collect(); self.assertEqual(p.returncode, 0, p.stderr)
        m = module(); original = [json.loads(v) for v in (out/'trace.jsonl').read_text().splitlines()]
        for change in ('drop_classification', 'reorder', 'duplicate', 'flag', 'content_base', 'incoming_source', 'parent_index', 'raw_dag'):
            with self.subTest(change=change):
                events = json.loads(json.dumps(original))
                if change == 'drop_classification': events = [e for e in events if e['kind'] != 'classification']
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
