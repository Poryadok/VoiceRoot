"""Actual producer events, never Git-node rows or record-supplied flags."""
import hashlib
import ast
import importlib.util
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import socket
import sys
import tempfile
import threading
import time
import unittest
from contextlib import contextmanager, ExitStack
import copy
from unittest.mock import patch

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


class R23Owner132Contract(unittest.TestCase):
    def test_exact_shell_wiring_uses_real_append_and_rejects_failed_authority(self):
        # Only the authority CLI is mocked here. The literal shell selectors,
        # CLI argument transport and actual append envelope are exercised.
        # Independent permission/fold semantics are covered by owner_fixture.
        functions=dict(re.findall(r'(?ms)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{\n(.*?)^\}\n',SHELL.read_text(encoding='utf8')))
        selected=''.join(name+'() {\n'+functions[name]+'}\n' for name in
            ('f13_f54_trace_event','f13_f54_r23_132_owner_permission'))
        with tempfile.TemporaryDirectory() as temporary:
            folder=Path(temporary);wrapper=folder/'observer.py';trace=folder/'trace'
            wrapper.write_text('import json,os,subprocess,sys\n'
                'if sys.argv[1]=="132-owner-permission":\n'
                ' if os.environ.get("DENY")=="1":sys.exit(37)\n'
                ' print(json.dumps({"fixture_authority":False,"argv":sys.argv[2:]}))\n'
                'else:sys.exit(subprocess.call([sys.executable,'+repr(str(HELPER))+']+sys.argv[1:]))\n')
            script=folder/'run.sh'
            script.write_text('set -eu\n'+selected+'\nf13_f54_r23_132_owner_permission "$REPO" "$TEST_PATH" "$MAP" "literal incoming"\n')
            path='protos/voice/r23_contract_manifest.json';op='F2/132437c43f6f0625fae8954542f8d9c2f6aa3037/2'
            env=dict(os.environ,TMP_DIR=str(folder),REPO=str(ROOT),TEST_PATH=path,MAP=str(folder/'map'),
                f13_f54_observer=str(wrapper),f13_f54_trace_file=str(trace),f13_f54_frame='F2',f13_f54_operation=op)
            result=subprocess.run(['bash',str(script)],env=env,capture_output=True,text=True)
            self.assertEqual(result.returncode,0,result.stderr)
            event=json.loads(trace.read_text());self.assertEqual(event['operation'],op)
            self.assertEqual(event['kind'],'r23_132_owner_permission')
            self.assertEqual(event['payload']['path'],path)
            self.assertEqual(event['payload']['receipt']['argv'],[str(ROOT),str(trace),path,str(folder/'map'),'literal incoming'])
            for change in ({'DENY':'1'},{'TEST_PATH':'protos/voice/other.json'},{'f13_f54_operation':'F2/other/2'}):
                trace.unlink(missing_ok=True)
                result=subprocess.run(['bash',str(script)],env=dict(env,**change),capture_output=True,text=True)
                self.assertNotEqual(result.returncode,0)
                self.assertFalse(trace.exists())

    def test_exact_source_unit_reconstructs_all_history_and_rows(self):
        m=module();u=m.r23_132_source_unit(ROOT)
        self.assertEqual(len(u['rows']),2)
        self.assertEqual(len(u['versions']),12)
        for key in ('versions','rows','ancestry'):
            forged=copy.deepcopy(u);forged[key]=forged[key][:-1]
            raw=json.dumps(forged,sort_keys=True,indent=2).encode()
            with self.subTest(key=key),patch.object(m,'R23_132_SOURCE_UNIT_RAW',raw),self.assertRaises(m.Rejected):
                m.r23_132_source_unit(ROOT)


    @contextmanager
    def owner_fixture(self):
        """Real append/fold/permission, synthetic graph and explicit authority seams.

        The prior whole operation is runtime-tainted while both raw R23 rows
        remain false. The exact historical source unit has separate Git tests.
        """
        m=module()
        with tempfile.TemporaryDirectory() as temporary,ExitStack() as patches:
            folder=Path(temporary);repo=folder/'repo';repo.mkdir()
            git(repo,'init','-q');git(repo,'config','user.name','F13 fixture');git(repo,'config','user.email','f13@example.invalid')
            def commit(values):
                for name,text in values.items():
                    p=repo/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text)
                git(repo,'add','.');git(repo,'commit','-qm','fixture');return git(repo,'rev-parse','HEAD')
            base=commit({'docs/base.md':'base\n'})
            left=commit({'docs/left.md':'left\n'})
            git(repo,'checkout','-q','--detach',base)
            names=['protos/voice/r23_contract_manifest.json','protos/voice/r23_contract_test/r23_contract_test.go']
            origin=commit({name:'fixture\n' for name in names})
            right=commit({RUNTIME:'package main\n'})
            tree=git(repo,'merge-tree','--write-tree',left,right).splitlines()[0]
            prior=git(repo,'commit-tree',tree,'-p',left,'-p',right,'-m','prior')
            source=git(repo,'commit-tree',tree,'-p',right,'-p',left,'-m','source')
            merge=git(repo,'commit-tree',tree,'-p',prior,'-p',source,'-m','result')
            bases=git(repo,'merge-base','--all',prior,source).splitlines()
            unit=dict(type='F54_EXACT132_SOURCE_UNIT',source_unit_sha256='fixture',merge=merge,
                parents=[prior,source],best_bases=bases,history_boundary=base,source_history=[],
                automatic_tree=tree,admission=False,path_authority=False)
            production=dict(base=base,source=merge,admission=False,runtime_pass=False,current_or_future_path_authority=False)
            original_git=m.git
            def fixture_git(repo,*args):
                if args==('rev-list','--first-parent','--reverse',base+'..'+merge):return (prior+'\n'+merge+'\n').encode()
                return original_git(repo,*args)
            patches.enter_context(patch.object(m,'git',side_effect=fixture_git))
            patches.enter_context(patch.object(m,'MERGE_132',merge))
            patches.enter_context(patch.object(m,'source_132_unit',return_value=unit))
            patches.enter_context(patch.object(m,'production_applicability',return_value=production))
            authority={'fixture':'omitted-outer-authority-prefix'}
            entry=dict(type='F54_EXPLICIT_MASTER_PRODUCTION_ENTRY_v1',base=base,source=merge,
                capability=m.raw_binding(m.canonical(authority)),native_binding=m.raw_binding('{"paths":[]}'),context={})
            patches.enter_context(patch.object(m,'checked_capability_value',return_value=authority))
            patches.enter_context(patch.object(m,'master_context',return_value=entry))
            trace=folder/'trace';mapping=folder/'map';mapping.write_bytes(b'')
            delta=folder/'delta';category=folder/'category';category.write_text('')
            ep=folder/'entry';ep.write_text(json.dumps(entry));m.append(trace,'master_entry','','',repo,[str(ep)])
            root=m.append(trace,'frame_enter','','',repo,['',base,merge,'true','true',str(mapping),''])
            expected_rows=[];receipt_paths=[]
            owner_unit=dict(rows=expected_rows,parents=[prior,source],prior_owner=prior)
            patches.enter_context(patch.object(m,'r23_132_source_unit',return_value=owner_unit))
            patches.enter_context(patch.object(m,'R23_132_SOURCE_UNIT_RAW',json.dumps({'rows':[{'path':n} for n in names]}).encode()))
            for outer,first,head in ((prior,left,right),(merge,prior,source)):
                op=m.append(trace,'operation_attempt',root,'',repo,[outer,'2',first,head,'merge'])
                if outer==merge:
                    cap=m.boundary_132_capability(repo,merge,prior,source,base,merge)
                    m.append(trace,'history_132_capability',root,op,repo,[json.dumps(cap)])
                paths=git(repo,'diff','--name-only',base,head).splitlines();delta.write_text('\n'.join(paths)+'\n')
                candidate_bases=bases if outer==merge else [base]
                m.append(trace,'operation_begin',root,op,repo,[outer,'2',first,base,head,','.join(candidate_bases),'false',str(delta)])
                childmap=folder/('child-'+outer);childmap.write_bytes(b'')
                child=m.append(trace,'frame_enter',root,'',repo,[root,base,head,'true','false',str(childmap),op])
                for owner in git(repo,'rev-list','--first-parent','--reverse',base+'..'+head).splitlines():
                    parents=git(repo,'rev-list','--parents','-n','1',owner).split()[1:]
                    direct=len(parents)==1;parent=parents[0];nested=owner if direct else parents[1]
                    boundary=parent if direct else base;index='1' if direct else '2'
                    childop=m.append(trace,'operation_attempt',child,'',repo,[owner,index,parent,nested,'direct' if direct else 'merge'])
                    changed=git(repo,'diff','--name-only',boundary,nested).splitlines();delta.write_text('\n'.join(changed)+'\n')
                    m.append(trace,'operation_begin',child,childop,repo,[owner,index,parent,boundary,nested,boundary,'true' if direct else 'false',str(delta)])
                    for name in changed:m.append(trace,'path_gate_before',child,childop,repo,[name,str(childmap)])
                    trigger=m.classify(changed);m.append(trace,'classification',child,childop,repo,[trigger,str(delta)])
                    for name in changed:
                        row='\t'.join([name,trigger,owner,parent,boundary,nested,parent,boundary])
                        m.append(trace,'row_before',child,childop,repo,[name,str(childmap),row])
                        m.append(trace,'row_after',child,childop,repo,[name,row,'original_owner_and_row_assignment'])
                        with childmap.open('a') as stream:stream.write(row+'\n')
                        m.append(trace,'row_committed',child,childop,repo,[name,str(childmap)])
                m.append(trace,'frame_exit',child,'',repo,['0',str(childmap),str(category)])
                delta.write_text('\n'.join(paths)+'\n')
                for name in paths:m.append(trace,'path_gate_before',root,op,repo,[name,str(mapping)])
                m.append(trace,'classification',root,op,repo,[m.classify(paths),str(delta)])
                nested_rows={r.split('\t')[0]:r for r in childmap.read_text().splitlines()}
                for name in paths:
                    previous=[r for r in mapping.read_text().splitlines() if r.split('\t')[0]==name]
                    childrow=nested_rows[name];parts=childrow.split('\t');content=previous[-1].split('\t')[6] if previous else first
                    row='\t'.join([name,parts[1],outer,first,parts[4],parts[5],content,','.join(candidate_bases)])
                    m.append(trace,'row_before',root,op,repo,[name,str(mapping),row])
                    if outer==merge and name in names:
                        expected_rows.append(dict(path=name,prior=previous[-1],incoming=row,child_row=childrow))
                        result=m.validate_trace(repo,trace,m.sha(trace.read_bytes()),r23_132_checkpoint=True)
                        receipt=result['r23_132_permission'];rp=folder/('receipt-'+str(len(receipt_paths)));rp.write_text(json.dumps(receipt));receipt_paths.append(rp)
                        m.append(trace,'r23_132_owner_permission',root,op,repo,[name,str(rp)])
                    if previous:
                        m.append(trace,'row_remove',root,op,repo,[name,row,'original_guarded_map_replacement'])
                        mapping.write_text(''.join(r+'\n' for r in mapping.read_text().splitlines() if r.split('\t')[0]!=name))
                    m.append(trace,'row_after',root,op,repo,[name,row,'original_owner_and_row_assignment'])
                    with mapping.open('a') as stream:stream.write(row+'\n')
                    m.append(trace,'row_committed',root,op,repo,[name,str(mapping)])
            category.write_text('synthetic_fixture_stop')
            m.append(trace,'failure',root,op,repo,['synthetic_fixture_stop',str(mapping)])
            m.append(trace,'frame_exit',root,'',repo,['1',str(mapping),str(category)])
            yield dict(m=m,repo=repo,trace=trace,map=mapping,root=root,paths=names,receipts=receipt_paths)

    def test_actual_pending_permission_and_replacement_keep_raw_false_and_true_ledger(self):
        with self.owner_fixture() as f:
            m=f['m'];result=m.validate_trace(f['repo'],f['trace'],m.sha(f['trace'].read_bytes()))
            for name in f['paths']:
                self.assertTrue(result['ledger'][f['root']]['paths'][name]['aggregate'])
                row=next(r for r in f['map'].read_text().splitlines() if r.split('\t')[0]==name)
                self.assertEqual(row.split('\t')[1],'false')
            for p in f['receipts']:
                receipt=json.loads(p.read_text());self.assertTrue(receipt['aggregate']);self.assertFalse(receipt['runtime_pass'])

    def test_permission_replay_phase_taint_and_evidence_mutations_reject(self):
        with self.owner_fixture() as f:
            m=f['m'];original=[json.loads(v) for v in f['trace'].read_text().splitlines()]
            for case in ('missing','duplicate','late','aggregate','writer','child-map','raw-flag','path','frame'):
                events=copy.deepcopy(original);index=next(i for i,e in enumerate(events) if e['kind']=='r23_132_owner_permission')
                permission=events[index];receipt=permission['payload']['receipt']
                if case=='missing':events.pop(index)
                elif case=='duplicate':events.insert(index,copy.deepcopy(permission))
                elif case=='late':events[index],events[index+1]=events[index+1],events[index]
                elif case=='aggregate':receipt['aggregate']=False
                elif case=='writer':receipt['obligations'].pop(0)
                elif case=='child-map':receipt['child_map']['sha256']='0'*64
                elif case=='raw-flag':receipt['incoming']=receipt['incoming'].replace('\tfalse\t','\ttrue\t',1)
                elif case=='path':permission['payload']['path']=f['paths'][1]
                else:permission['frame']='F999'
                previous='0'*64
                for n,e in enumerate(events,1):
                    e.update(sequence=n,previous=previous);e.pop('digest',None);e['digest']=m.sha(m.canonical(e));previous=e['digest']
                f['trace'].write_text(''.join(json.dumps(e,sort_keys=True)+'\n' for e in events))
                with self.subTest(case=case),self.assertRaises(m.Rejected):m.validate_trace(f['repo'],f['trace'],m.sha(f['trace'].read_bytes()))


class Exact132HistoryContract(unittest.TestCase):
    def test_clean_composition_is_exact_and_not_owner_permission(self):
        m=module();unit=m.source_132_unit(ROOT)
        base='288e6a4fb963b3d5775f885e14ecfda1b603a4ff'
        target='ecde2e7b29d93dd01b233cd3128b373137aa6715'
        production=dict(base=base,source=target,admission=False,runtime_pass=False,current_or_future_path_authority=False)
        path='src/backend/voice/internal/principaljwks/publisher.go'
        entry=m.git(ROOT,'ls-tree',unit['merge'],'--',path).decode().strip()
        args=[ROOT,unit['merge'],unit['history_boundary'],*unit['parents'],path,entry,base,target]
        with patch.object(m,'production_applicability',return_value=production):
            value=m.composition_132_capability(*args)
            self.assertFalse(value['admission']);self.assertFalse(value['owner_permission'])
            self.assertEqual(value['automatic_tree'],unit['automatic_tree'])
            self.assertEqual(value['best_bases'],unit['best_bases'])
            self.assertEqual(value['result_entry'],entry)
            for index in (1,2,3,4,5,6,7,8):
                changed=list(args);changed[index]='invalid'
                with self.subTest(field=index),self.assertRaises(m.Rejected):m.composition_132_capability(*changed)

    @contextmanager
    def fold_fixture(self, composition=False, runtime_path=RUNTIME):
        """Private real Git criss-cross; source-unit/outer authority seams mocked.

        The literal historical unit has separate real-Git tests. Here actual
        append/fold validates two completed child operations and outer imports;
        an omitted outer prefix is explicit fixture scope, not producer proof.
        """
        m=module()
        with tempfile.TemporaryDirectory() as temporary,ExitStack() as patches:
            folder=Path(temporary);repo=folder/'repo';repo.mkdir()
            git(repo,'init','-q');git(repo,'config','user.name','F13 fixture')
            git(repo,'config','user.email','f13@example.invalid')
            def commit(path,text):
                file=repo/path;file.parent.mkdir(parents=True,exist_ok=True);file.write_text(text)
                git(repo,'add','.');git(repo,'commit','-qm','fixture');return git(repo,'rev-parse','HEAD')
            base=commit('docs/base.md','base\n')
            left=commit(runtime_path,'package main\n')
            git(repo,'checkout','-q','--detach',base)
            right=commit('docs/right.md','right\n')
            tree=git(repo,'merge-tree','--write-tree',left,right).splitlines()[0]
            first=git(repo,'commit-tree',tree,'-p',left,'-p',right,'-m','first')
            source=git(repo,'commit-tree',tree,'-p',right,'-p',left,'-m','source')
            merge=git(repo,'commit-tree',tree,'-p',first,'-p',source,'-m','result')
            bases=git(repo,'merge-base','--all',first,source).splitlines()
            unit=dict(type='F54_EXACT132_SOURCE_UNIT',source_unit_sha256='fixture',merge=merge,
                parents=[first,source],best_bases=bases,history_boundary=base,
                source_history=git(repo,'rev-list','--reverse','--topo-order','--parents',base+'..'+source).splitlines(),
                automatic_tree=tree,admission=False,path_authority=False)
            production=dict(base=base,source=merge,admission=False,runtime_pass=False,current_or_future_path_authority=False)
            original_git=m.git
            def fixture_git(repo,*args):
                if args==('rev-list','--first-parent','--reverse',base+'..'+merge):return (merge+'\n').encode()
                return original_git(repo,*args)
            patches.enter_context(patch.object(m,'git',side_effect=fixture_git))
            patches.enter_context(patch.object(m,'MERGE_132',merge))
            patches.enter_context(patch.object(m,'source_132_unit',return_value=unit))
            patches.enter_context(patch.object(m,'production_applicability',return_value=production))
            capability={'fixture':'outer-authority-only'}
            entry=dict(type='F54_EXPLICIT_MASTER_PRODUCTION_ENTRY_v1',base=base,source=merge,
                capability=m.raw_binding(m.canonical(capability)),native_binding=m.raw_binding(json.dumps({'paths':[runtime_path,'docs/right.md'] if composition else []})),context={})
            patches.enter_context(patch.object(m,'checked_capability_value',return_value=capability))
            patches.enter_context(patch.object(m,'master_context',return_value=entry))
            trace=folder/'trace';mapping=folder/'map';childmap=folder/'child-map'
            mapping.write_bytes(b'');childmap.write_bytes(b'');delta=folder/'delta';category=folder/'category';category.write_text('')
            entry_path=folder/'entry';entry_path.write_text(json.dumps(entry))
            m.append(trace,'master_entry','','',repo,[str(entry_path)])
            root=m.append(trace,'frame_enter','F2','',repo,['',base,merge,'true','true',str(mapping),''])
            op=m.append(trace,'operation_attempt',root,'',repo,[merge,'2',first,source,'merge'])
            receipt=m.boundary_132_capability(repo,merge,first,source,base,merge)
            m.append(trace,'history_132_capability',root,op,repo,[json.dumps(receipt)])
            paths=git(repo,'diff','--name-only',base,source).splitlines();delta.write_text('\n'.join(paths)+'\n')
            m.append(trace,'operation_begin',root,op,repo,[merge,'2',first,base,source,','.join(bases),'false',str(delta)])
            child=m.append(trace,'frame_enter','F5','',repo,[root,base,source,'true','false',str(childmap),op])
            for owner,parent,head,boundary,index in ((right,base,right,base,'1'),(source,right,left,base,'2')):
                childop=m.append(trace,'operation_attempt',child,'',repo,[owner,index,parent,head,'direct' if index=='1' else 'merge'])
                names=git(repo,'diff','--name-only',boundary,head).splitlines();delta.write_text('\n'.join(names)+'\n')
                m.append(trace,'operation_begin',child,childop,repo,[owner,index,parent,boundary,head,boundary,'true' if index=='1' else 'false',str(delta)])
                for name in names:m.append(trace,'path_gate_before',child,childop,repo,[name,str(childmap)])
                trigger=m.classify(names);m.append(trace,'classification',child,childop,repo,[trigger,str(delta)])
                for name in names:
                    row='\t'.join([name,trigger,owner,parent,boundary,head,parent,boundary])
                    m.append(trace,'row_before',child,childop,repo,[name,str(childmap),row])
                    m.append(trace,'row_after',child,childop,repo,[name,row,'original_owner_and_row_assignment'])
                    with childmap.open('a') as stream:stream.write(row+'\n')
                    m.append(trace,'row_committed',child,childop,repo,[name,str(childmap)])
            m.append(trace,'frame_exit',child,'',repo,['0',str(childmap),str(category)])
            delta.write_text('\n'.join(paths)+'\n')
            for name in paths:
                m.append(trace,'path_gate_before',root,op,repo,[name,str(mapping)])
                if composition and name==runtime_path:
                    receipt=m.composition_132_capability(repo,merge,base,first,source,name,
                        m.tree_entry(repo,merge,name).rstrip('\0'),base,merge)
                    m.append(trace,'composition_132_capability',root,op,repo,[json.dumps(receipt)])
            m.append(trace,'classification',root,op,repo,[m.classify(paths),str(delta)])
            rows={line.split('\t')[0]:line.split('\t') for line in childmap.read_text().splitlines()}
            for name in paths:
                nested=rows[name];row='\t'.join([name,nested[1],merge,first,nested[4],nested[5],first,','.join(bases)])
                m.append(trace,'row_before',root,op,repo,[name,str(mapping),row])
                m.append(trace,'row_after',root,op,repo,[name,row,'original_owner_and_row_assignment'])
                with mapping.open('a') as stream:stream.write(row+'\n')
                m.append(trace,'row_committed',root,op,repo,[name,str(mapping)])
            # Close only the synthetic outer scope as failed: this fixture tests
            # successful child/import conservation, not a production wrapper.
            category.write_text('synthetic_fixture_stop')
            m.append(trace,'failure',root,op,repo,['synthetic_fixture_stop',str(mapping)])
            m.append(trace,'frame_exit',root,'',repo,['1',str(mapping),str(category)])
            yield dict(m=m,repo=repo,trace=trace,root=root,child=child,operation=op,paths=paths)

    def test_composition_receipt_is_independently_folded_and_required(self):
        with self.fold_fixture(composition=True) as f:
            m=f['m'];original=[json.loads(line) for line in f['trace'].read_text().splitlines()]
            m.validate_trace(f['repo'],f['trace'],m.sha(f['trace'].read_bytes()))
            for case in ('missing','duplicate','changed-result'):
                events=copy.deepcopy(original)
                index=next(i for i,e in enumerate(events) if e['kind']=='composition_132_capability')
                if case=='missing':events.pop(index)
                elif case=='duplicate':events.insert(index,copy.deepcopy(events[index]))
                else:events[index]['payload']['result_entry']='forged'
                previous='0'*64
                for number,event in enumerate(events,1):
                    event['sequence']=number;event['previous']=previous;event.pop('digest',None)
                    event['digest']=m.sha(m.canonical(event));previous=event['digest']
                f['trace'].write_text(''.join(json.dumps(e,sort_keys=True)+'\n' for e in events))
                with self.subTest(case=case),self.assertRaises(m.Rejected):
                    m.validate_trace(f['repo'],f['trace'],m.sha(f['trace'].read_bytes()))

    def test_native_documentation_keeps_ordinary_checks_and_guarded_receipts(self):
        # Native documentation is not a guarded merge-composition path.
        paths=('src/backend/voice/internal/principaljwks/publisher.go',
               'src/backend/voice/internal/principaljwks/publisher_test.go',
               'src/backend/voice/jwks_recovery_test.go',
               'src/backend/voice/jwks_startup_order_test.go',
               'src/backend/voice/main.go')
        for name in paths:
            with self.subTest(path=name),self.fold_fixture(composition=True,runtime_path=name) as f:
                m=f['m'];original=[json.loads(line) for line in f['trace'].read_text().splitlines()]
                self.assertEqual(m.guarded_names([name,'docs/right.md']),[name])
                result=m.validate_trace(f['repo'],f['trace'],m.sha(f['trace'].read_bytes()))
                self.assertIn('docs/right.md',result['ledger'][f['root']]['paths'])
                for case in ('missing-guarded-receipt','documentation-map-tamper'):
                    events=copy.deepcopy(original)
                    if case=='missing-guarded-receipt':
                        events=[e for e in events if e['kind']!='composition_132_capability']
                    else:
                        event=next(e for e in events if e['kind']=='row_before' and
                            e['frame']==f['root'] and e['payload']['path']=='docs/right.md')
                        event['payload']['map_before']['sha256']='0'*64
                    previous='0'*64
                    for number,event in enumerate(events,1):
                        event['sequence']=number;event['previous']=previous;event.pop('digest',None)
                        event['digest']=m.sha(m.canonical(event));previous=event['digest']
                    f['trace'].write_text(''.join(json.dumps(e,sort_keys=True)+'\n' for e in events))
                    with self.subTest(case=case),self.assertRaises(m.Rejected) as error:
                        m.validate_trace(f['repo'],f['trace'],m.sha(f['trace'].read_bytes()))
                    if case=='missing-guarded-receipt':
                        self.assertEqual(str(error.exception),'exact132_native_checkpoint_composition_required')

    def test_actual_append_fold_requires_child_and_preserves_aggregate(self):
        # The unchanged child_flags OR is isolated by the real-fold oracle
        # Dc0ResidualContract.test_synthetic_original_taint_survives_false_residual_and_parent.
        with self.fold_fixture() as f:
            m=f['m'];result=m.validate_trace(f['repo'],f['trace'],m.sha(f['trace'].read_bytes()))
            for name in f['paths']:
                self.assertTrue(result['ledger'][f['root']]['paths'][name]['aggregate'])
            self.assertFalse(result['ledger'][f['child']]['paths']['docs/right.md']['aggregate'])

    def test_capability_child_completion_map_and_taint_mutations_reject(self):
        with self.fold_fixture() as f:
            m=f['m'];original=[json.loads(line) for line in f['trace'].read_text().splitlines()]
            for case in ('missing-capability','duplicate-capability','wrong-scope','child-failure','child-map','classifier-tamper'):
                events=copy.deepcopy(original)
                if case=='missing-capability':events=[e for e in events if e['kind']!='history_132_capability']
                elif case=='duplicate-capability':
                    index=next(i for i,e in enumerate(events) if e['kind']=='history_132_capability');events.insert(index,copy.deepcopy(events[index]))
                elif case=='wrong-scope':next(e for e in events if e['kind']=='history_132_capability')['payload']['collector_scope']['target']='0'*40
                elif case=='child-failure':next(e for e in events if e['kind']=='frame_exit' and e['frame']==f['child'])['payload']['exit']=1
                elif case=='child-map':next(e for e in events if e['kind']=='frame_exit' and e['frame']==f['child'])['payload']['final_map']['sha256']='0'*64
                else:next(e for e in events if e['kind']=='classification' and e['frame']==f['root'])['payload']['trigger']='false'
                previous='0'*64
                for index,event in enumerate(events,1):
                    event['sequence']=index;event['previous']=previous;event.pop('digest',None)
                    event['digest']=m.sha(m.canonical(event));previous=event['digest']
                f['trace'].write_text(''.join(json.dumps(e,sort_keys=True)+'\n' for e in events))
                with self.subTest(case=case),self.assertRaises(m.Rejected):
                    m.validate_trace(f['repo'],f['trace'],m.sha(f['trace'].read_bytes()))

    def test_boundary_requires_exact_tuple_and_fresh_actual_production_scope(self):
        # Source reconstruction is real; this test explicitly isolates the
        # production qualifier seam. It is not an actual-event authority proof.
        m=module();unit=m.source_132_unit(ROOT)
        base='288e6a4fb963b3d5775f885e14ecfda1b603a4ff'
        target='ecde2e7b29d93dd01b233cd3128b373137aa6715'
        production=dict(base=base,source=target,admission=False,runtime_pass=False,
            current_or_future_path_authority=False)
        args=(ROOT,unit['merge'],*unit['parents'],base,target)
        with patch.object(m,'production_applicability',return_value=production) as check:
            value=m.boundary_132_capability(*args)
            self.assertEqual(value['unit'],unit)
            self.assertEqual(value['history_boundary'],unit['history_boundary'])
            check.assert_called_once_with(ROOT)
            for index in range(1,6):
                changed=list(args);changed[index]='0'*40
                with self.subTest(field=index),self.assertRaises(m.Rejected):
                    m.boundary_132_capability(*changed)
        with patch.object(m,'production_applicability',side_effect=m.Rejected('fresh-event-denied')):
            with self.assertRaisesRegex(m.Rejected,'^fresh-event-denied$'):
                m.boundary_132_capability(*args)

    def test_complete_clean_source_unit_keeps_bases_distinct_from_boundary(self):
        m=module();value=m.source_132_unit(ROOT)
        self.assertEqual(value['merge'],'132437c43f6f0625fae8954542f8d9c2f6aa3037')
        self.assertEqual(value['history_boundary'],'304b49dcf5b771efae5a7d4ddaa28a3d0b9ddb5d')
        self.assertEqual(len(value['best_bases']),2)
        self.assertNotIn(value['history_boundary'],value['best_bases'])
        self.assertEqual(len(value['source_history']),14)
        self.assertEqual(value['automatic_tree'],'182c39b2a05b637ae2a1d5b18016a658bfde2a81')
        self.assertFalse(value['admission'])
        self.assertFalse(value['path_authority'])

    def test_source_unit_rejects_changed_bases_history_and_result(self):
        m=module();real=m.git
        for kind in ('bases','history','result'):
            with self.subTest(kind=kind):
                def changed(repo,*args):
                    raw=real(repo,*args)
                    if kind=='bases' and args[:2]==('merge-base','--all') and len(raw.splitlines())==2:
                        return b'\n'.join(reversed(raw.splitlines()))+b'\n'
                    if kind=='history' and args[:3]==('rev-list','--reverse','--topo-order'):
                        return b'\n'.join(raw.splitlines()[:-1])+b'\n'
                    if kind=='result' and args==('rev-parse','132437c43f6f0625fae8954542f8d9c2f6aa3037^{tree}'):
                        return b'0'*40+b'\n'
                    return raw
                with patch.object(m,'git',side_effect=changed),self.assertRaises(m.Rejected):
                    m.source_132_unit(ROOT)


class Dc0ResidualContract(unittest.TestCase):
    @contextmanager
    def fold_fixture(self, import_residual=False, synthetic_parent_taint=False):
        """Real append/fold; outer production authority/range is explicitly mocked.

        Historical dc0 source objects, parent operation and row conservation are
        real. This compact transport fixture is not full producer evidence.
        """
        m=module();original_git=m.git
        base='698d4f064f032ea74abf3ede2017100b26c8ee69'
        target='53f75f74523735ac594e21582bea659f567a7288'
        outer='e6757e8ac26032ac4bad7f4356e91fffbf0725ba'
        eventbase='288e6a4fb963b3d5775f885e14ecfda1b603a4ff'
        unit=m.dc0_residual_source(ROOT);first,source=unit['ordered_parents'];boundary=unit['bases'][0]
        def fixture_git(repo,*args):
            if args==('rev-list','--first-parent','--reverse',eventbase+'..'+outer):return (outer+'\n').encode()
            if args==('rev-list','--first-parent','--reverse',base+'..'+target):return (m.DC0_MERGE+'\n').encode()
            if args==('diff','--name-only',base,target):return (m.DC0_PATH+'\n').encode() if import_residual else b''
            return original_git(repo,*args)
        with tempfile.TemporaryDirectory() as temporary,ExitStack() as patches:
            folder=Path(temporary);trace=folder/'trace';mapping=folder/'map';mapping.write_bytes(b'')
            outermap=folder/'outer-map';outermap.write_bytes(b'');delta=folder/'delta'
            capability={'fixture':'outer-authority-only'}
            entry=dict(type='F54_EXPLICIT_MASTER_PRODUCTION_ENTRY_v1',base=eventbase,source=outer,
                capability=m.raw_binding(m.canonical(capability)),native_binding=m.raw_binding('{"paths":[]}'),context={})
            context=dict(source_context={'fixture':True},event={},values={},snapshots={},files={})
            patches.enter_context(patch.object(m,'git',side_effect=fixture_git))
            patches.enter_context(patch.object(m,'checked_capability_value',return_value=capability))
            patches.enter_context(patch.object(m,'master_context',return_value=entry))
            patches.enter_context(patch.object(m,'router_source_context',return_value={'fixture':True}))
            patches.enter_context(patch.object(m,'manual_authority_expected',return_value=dict(values={},files={})))
            if synthetic_parent_taint:
                # Explicit OR control, not a claim that historical dc0 triggers
                # Voice runtime scope (its actual classifier result is false).
                original_classify=m.classify
                patches.enter_context(patch.object(m,'classify',side_effect=lambda names:
                    'true' if len(names)>1 else original_classify(names)))
            entry_path=folder/'entry';entry_path.write_text(json.dumps(entry))
            m.append(trace,'master_entry','','',ROOT,[str(entry_path)])
            root=m.append(trace,'frame_enter','F2','',ROOT,['',eventbase,outer,'true','true',str(outermap),''])
            rootop=m.append(trace,'operation_attempt',root,'',ROOT,[outer,'2',base,target,'merge'])
            delta.write_bytes((m.DC0_PATH+'\n').encode() if import_residual else b'')
            m.append(trace,'operation_begin',root,rootop,ROOT,[outer,'2',base,base,target,base,'false',str(delta)])
            frame=m.append(trace,'frame_enter','F5','',ROOT,[root,base,target,'true','false',str(mapping),rootop])
            op=m.append(trace,'operation_attempt',frame,'',ROOT,[m.DC0_MERGE,'2',first,source,'merge'])
            paths=original_git(ROOT,'diff','--name-only',boundary,source).decode().splitlines()
            delta.write_text('\n'.join(paths)+'\n')
            m.append(trace,'operation_begin',frame,op,ROOT,[m.DC0_MERGE,'2',first,boundary,source,boundary,'false',str(delta)])
            for name in paths:m.append(trace,'path_gate_before',frame,op,ROOT,[name,str(mapping)])
            trigger=m.classify(paths)
            m.append(trace,'classification',frame,op,ROOT,[trigger,str(delta)])
            for name in paths:
                row='\t'.join([name,trigger,m.DC0_MERGE,first,boundary,source,first,boundary])
                m.append(trace,'row_before',frame,op,ROOT,[name,str(mapping),row])
                m.append(trace,'row_after',frame,op,ROOT,[name,row,'original_owner_and_row_assignment'])
                with mapping.open('ab') as out:out.write((row+'\n').encode())
                m.append(trace,'row_committed',frame,op,ROOT,[name,str(mapping)])
            def check(phase,ctx=None):
                return m.validate_trace(ROOT,trace,m.sha(trace.read_bytes()),dc0_checkpoint=phase,dc0_context=ctx)['dc0_checkpoint']
            def begin():
                spec=check('begin',context);specpath=folder/'spec';specpath.write_text(json.dumps(spec))
                manual=m.append(trace,'dc0_residual_begin',frame,op,ROOT,[str(specpath),str(mapping)])
                m.append(trace,'path_gate_before',frame,manual,ROOT,[m.DC0_PATH,str(mapping)])
                delta.write_text(m.DC0_PATH+'\n');flag=m.classify([m.DC0_PATH])
                m.append(trace,'classification',frame,manual,ROOT,[flag,str(delta),'0' if flag=='true' else '1'])
                row='\t'.join([m.DC0_PATH,flag,m.DC0_MERGE,first,first,m.DC0_MERGE,first,boundary])
                m.append(trace,'row_before',frame,manual,ROOT,[m.DC0_PATH,str(mapping),row])
                return manual,row,spec
            def close_prefix(manual):
                # Explicit synthetic failure closure only makes a partial fold
                # inspectable; it is never successful producer evidence.
                category=folder/'category';category.write_text('synthetic_fixture_stop')
                m.append(trace,'failure',frame,manual,ROOT,['synthetic_fixture_stop',str(mapping)])
                m.append(trace,'frame_exit',frame,'',ROOT,['1',str(mapping),str(category)])
                m.append(trace,'failure',root,rootop,ROOT,['synthetic_fixture_stop',str(outermap)])
                m.append(trace,'frame_exit',root,'',ROOT,['1',str(outermap),str(category)])
                return m.validate_trace(ROOT,trace,m.sha(trace.read_bytes()))
            def finish(manual,row):
                m.append(trace,'row_after',frame,manual,ROOT,[m.DC0_PATH,row,'actual_dc0_result_parent_assignment'])
                with mapping.open('ab') as out:out.write((row+'\n').encode())
                m.append(trace,'row_committed',frame,manual,ROOT,[m.DC0_PATH,str(mapping)])
                receipt=check('end');receipt_path=folder/'receipt';receipt_path.write_text(json.dumps(receipt))
                m.append(trace,'dc0_residual_end',frame,manual,ROOT,[str(receipt_path),str(mapping)])
                return receipt
            yield dict(m=m,trace=trace,mapping=mapping,frame=frame,original=op,context=context,
                folder=folder,check=check,begin=begin,close=close_prefix,finish=finish,
                root=root,rootop=rootop,outermap=outermap,delta=delta,entry=entry)

    def test_successful_child_row_is_imported_into_exact_e675_parent(self):
        self.check_successful_import(False)

    def test_synthetic_original_taint_survives_false_residual_and_parent(self):
        self.check_successful_import(True)

    def check_successful_import(self, synthetic_parent_taint):
        with self.fold_fixture(import_residual=True,synthetic_parent_taint=synthetic_parent_taint) as f:
            m=f['m'];manual,row,spec=f['begin']();f['finish'](manual,row)
            category=f['folder']/'category';category.write_text('')
            m.append(f['trace'],'frame_exit',f['frame'],'',ROOT,['0',str(f['mapping']),str(category)])
            m.append(f['trace'],'path_gate_before',f['root'],f['rootop'],ROOT,[m.DC0_PATH,str(f['outermap'])])
            f['delta'].write_text(m.DC0_PATH+'\n')
            m.append(f['trace'],'classification',f['root'],f['rootop'],ROOT,[m.classify([m.DC0_PATH]),str(f['delta'])])
            parts=row.split('\t');parent=m.git(ROOT,'rev-list','--parents','-n','1','e6757e8ac26032ac4bad7f4356e91fffbf0725ba').decode().split()[1]
            incoming='\t'.join([m.DC0_PATH,parts[1],'e6757e8ac26032ac4bad7f4356e91fffbf0725ba',parent,
                parts[4],parts[5],parent,'698d4f064f032ea74abf3ede2017100b26c8ee69'])
            for kind,args in (('row_before',[m.DC0_PATH,str(f['outermap']),incoming]),
                              ('row_after',[m.DC0_PATH,incoming,'original_owner_and_row_assignment'])):
                m.append(f['trace'],kind,f['root'],f['rootop'],ROOT,args)
            f['outermap'].write_text(incoming+'\n')
            m.append(f['trace'],'row_committed',f['root'],f['rootop'],ROOT,[m.DC0_PATH,str(f['outermap'])])
            # Root failure is explicitly synthetic; child success/import is real.
            category.write_text('synthetic_fixture_stop')
            m.append(f['trace'],'failure',f['root'],f['rootop'],ROOT,['synthetic_fixture_stop',str(f['outermap'])])
            m.append(f['trace'],'frame_exit',f['root'],'',ROOT,['1',str(f['outermap']),str(category)])
            folded=m.validate_trace(ROOT,f['trace'],m.sha(f['trace'].read_bytes()))
            expected=spec['original_classification']['trigger']=='true' or m.classify([m.DC0_PATH])=='true'
            self.assertEqual(expected,synthetic_parent_taint)
            self.assertEqual(folded['ledger'][f['frame']]['paths'][m.DC0_PATH]['aggregate'],expected)
            self.assertEqual(folded['ledger'][f['root']]['paths'][m.DC0_PATH]['aggregate'],expected)
            self.assertEqual(folded['ledger'][f['root']]['rows'],[incoming])

    def test_context_membership_and_event_order_cannot_bypass_residual_guards(self):
        for change in ('context','native','game','parent','frame','injection','missing_end','reordered_end','owner'):
            with self.subTest(change=change),self.fold_fixture() as f:
                m=f['m'];trace=f['trace'];events=[json.loads(x) for x in trace.read_bytes().splitlines()]
                if change in ('context','native','game','parent','frame','owner'):
                    context=copy.deepcopy(f['context'])
                    if change=='context':context['source_context']={'changed':True}
                    elif change=='native':
                        events[0]['payload']['native_binding']=m.raw_binding(json.dumps({'paths':[m.DC0_PATH]}))
                        f['entry']['native_binding']=events[0]['payload']['native_binding']
                    elif change=='game':
                        context['values']['game_checkpoint_aggregate_paths']='fixture-game'
                        context['snapshots']['fixture-game']=m.raw_binding(m.DC0_PATH+'\n')
                        context['files']['fixture-game']=m.file_digest((m.DC0_PATH+'\n').encode())
                    elif change=='parent':
                        next(e for e in events if e['kind']=='frame_enter' and e['frame']==f['frame'])['payload']['parent_operation']='foreign/operation'
                    elif change=='frame':
                        next(e for e in events if e['kind']=='frame_enter' and e['frame']==f['frame'])['payload']['allow_direct_followup']='false'
                    else:
                        # Existing owner's path cannot be relabeled into this residual.
                        event=next(e for e in events if e['kind']=='row_committed')
                        event['payload']['path']=m.DC0_PATH
                    m.write_events(trace,events)
                    if change=='game':
                        with patch.object(m,'manual_authority_expected',return_value=dict(values=context['values'],files={})),self.assertRaisesRegex(m.Rejected,'dc0_no_game_aggregate_path_exemption'):
                            f['check']('begin',context)
                    elif change=='native':
                        with self.assertRaisesRegex(m.Rejected,'dc0_no_native_path_exemption'):f['check']('begin',context)
                    else:
                        with self.assertRaises((m.Rejected,KeyError)):f['check']('begin',context)
                else:
                    manual,row,spec=f['begin']()
                    if change=='injection':
                        events=[json.loads(x) for x in trace.read_bytes().splitlines()]
                        m.write_events(trace,[e for e in events if e['kind']!='dc0_residual_begin'])
                        with self.assertRaises((m.Rejected,KeyError)):f['check']('append')
                    else:
                        f['finish'](manual,row)
                        events=[json.loads(x) for x in trace.read_bytes().splitlines()]
                        end=events.pop()
                        if change=='reordered_end':events.insert(-1,end)
                        m.write_events(trace,events)
                        category=f['folder']/'category';category.write_text('')
                        m.append(trace,'frame_exit',f['frame'],'',ROOT,['0',str(f['mapping']),str(category)])
                        with self.assertRaises(m.Rejected):m.validate_trace(ROOT,trace,m.sha(trace.read_bytes()))

    def test_actual_append_fold_conserves_residual_and_original_taint(self):
        with self.fold_fixture() as f:
            m=f['m'];manual,row,spec=f['begin']();f['check']('append')
            m.append(f['trace'],'row_after',f['frame'],manual,ROOT,[m.DC0_PATH,row,'actual_dc0_result_parent_assignment'])
            with f['mapping'].open('ab') as out:out.write((row+'\n').encode())
            m.append(f['trace'],'row_committed',f['frame'],manual,ROOT,[m.DC0_PATH,str(f['mapping'])])
            receipt=f['check']('end');receipt_path=f['folder']/'receipt';receipt_path.write_text(json.dumps(receipt))
            m.append(f['trace'],'dc0_residual_end',f['frame'],manual,ROOT,[str(receipt_path),str(f['mapping'])])
            self.assertEqual(receipt['row'],row)
            self.assertEqual(receipt['spec']['original_classification'],spec['original_classification'])
            # Replaying a second begin or finish cannot mint a second owner.
            with self.assertRaises(m.Rejected):f['check']('begin',f['context'])
            with self.assertRaises(m.Rejected):f['check']('end')
            folded=f['close'](manual)
            self.assertEqual(folded['dc0_residual_operations'][manual],receipt)
            self.assertEqual(folded['ledger'][f['frame']]['paths'][m.DC0_PATH]['aggregate'],
                spec['original_classification']['trigger']=='true' or receipt['classification']['trigger']=='true')

    def test_real_shell_block_append_and_independent_fold(self):
        with self.fold_fixture() as f:
            m=f['m'];folder=f['folder'];spec=f['check']('begin',f['context'])
            spec_path=folder/'fixture-spec';spec_path.write_text(json.dumps(spec))
            context_path=folder/'fixture-context';context_path.write_text(json.dumps(f['context']))
            # Only checkpoint CLI authority is a fixture seam. Actual shell,
            # classifier, append CLI and independent final fold are unchanged.
            checkpoint=folder/'checkpoint.py'
            checkpoint.write_text('import json,sys,hashlib\nfrom pathlib import Path\n'
                'phase,trace,mapping,specpath=sys.argv[1:]\n'
                'spec=json.loads(Path(specpath).read_bytes())\n'
                'if phase=="begin": print(json.dumps(spec))\n'
                'elif phase=="end":\n'
                ' events=[json.loads(x) for x in Path(trace).read_bytes().splitlines()]\n'
                ' classification=next(e["payload"] for e in events if e["kind"]=="classification" and e["operation"]==spec["operation"])\n'
                ' raw=Path(mapping).read_bytes();name=spec["source"]["paths"][0]\n'
                ' row=next(x for x in raw.decode().splitlines() if x.split("\\t")[0]==name)\n'
                ' print(json.dumps(dict(spec=spec,classification=classification,map_after=dict(bytes=len(raw),sha256=hashlib.sha256(raw).hexdigest()),row=row,admission=False)))\n')
            source=SHELL.read_text();start=source.index('    if [[ "${commit}" == dc0acdef')
            block=source[start:source.index('    # Exact F authored merge-result',start)]
            variables=dict(ROOT=str(ROOT),TMP_DIR=str(folder),f13_f54_observer=str(HELPER),
                f13_f54_trace_file=str(f['trace']),f13_f54_frame=f['frame'],f13_f54_operation=f['original'],
                capability_command='production-capabilities',repo=str(ROOT),output=str(f['mapping']),
                base='698d4f064f032ea74abf3ede2017100b26c8ee69',target='53f75f74523735ac594e21582bea659f567a7288',
                commit=m.DC0_MERGE,FIXTURE_CHECKPOINT=str(checkpoint),FIXTURE_SPEC=str(spec_path),FIXTURE_CONTEXT=str(context_path))
            program='set -euo pipefail\n'+definitions()+'\n'
            program+=re.search(r'(?ms)^f13_f54_trace_event\(\) \{\n.*?^\}\n',source).group(0)+'\n'
            program+='\n'.join(k+'='+shlex.quote(v) for k,v in variables.items())+'\n'
            program+='source "$ROOT/scripts/ci/voice-r22-runtime-scope.sh"\n'
            program+='''python3() {
  if [[ "$2" == dc0-residual-checkpoint ]]; then
    command python3 "$FIXTURE_CHECKPOINT" "$6" "$4" "$5" "$FIXTURE_SPEC"
  else command python3 "$@"; fi
}
f13_f54_manual_gate_context() { cp "$FIXTURE_CONTEXT" "$1"; }
fixture_run() {
  local path source_entry
  local -A owners=() owner_triggers=() owner_content_bases=() owner_source_heads=() owner_source_entries=() guarded_entries=() guarded_entry_commits=()
'''+block+'\n}\nfixture_run\n'
            executable=folder/'actual-dc0.sh';executable.write_text(program)
            result=subprocess.run(['bash',str(executable)],capture_output=True)
            self.assertEqual(result.returncode,0,result.stderr.decode())
            folded=f['close'](spec['operation'])
            self.assertIn(spec['operation'],folded['dc0_residual_operations'])

    def test_fold_rejects_missing_parent_completion_and_forged_classifier(self):
        with self.fold_fixture() as f:
            m=f['m'];original=f['trace'].read_bytes();events=[json.loads(x) for x in original.splitlines()]
            m.write_events(f['trace'],events[:-1])
            with self.assertRaisesRegex(m.Rejected,'complete_ordered_path_commits'):f['check']('begin',f['context'])
            f['trace'].write_bytes(original);manual,row,spec=f['begin']()
            events=[json.loads(x) for x in f['trace'].read_bytes().splitlines()]
            classified=next(e for e in events if e['kind']=='classification' and e['operation']==manual)
            classified['payload']['trigger']='true' if classified['payload']['trigger']=='false' else 'false'
            m.write_events(f['trace'],events)
            with self.assertRaisesRegex(m.Rejected,'dc0_original_classifier_no_exemption'):f['check']('append')

    def test_exact_historical_residual_is_independently_reconstructed(self):
        m=module();value=m.dc0_residual_source(ROOT)
        self.assertEqual(value['merge'],'dc0acdefec7e5c413988adf71f86d12bff4017bc')
        self.assertEqual(value['paths'],['src/backend/chat/internal/gisprincipal/method_union_test.go'])
        self.assertEqual(value['entry']['blob']['sha256'],'2a2c785a1faf086f14c4702fc062dbaf5698ccfe8c590a4217f07ad5ec64ab7d')
        self.assertFalse(value['admission'])

    def test_parent_tree_blob_and_residual_set_changes_are_rejected(self):
        m=module();original=m.git
        mutations=(('rev-list',),('rev-parse',),('diff','--name-only'),('ls-tree',))
        for prefix in mutations:
            def changed(repo,*args):
                raw=original(repo,*args)
                if args[:len(prefix)]==prefix:
                    return raw+b'forged\n'
                return raw
            with self.subTest(prefix=prefix),patch.object(m,'git',side_effect=changed),self.assertRaises(m.Rejected):
                m.dc0_residual_source(ROOT)

    def test_raw_source_unit_cannot_be_rewritten(self):
        m=module()
        with patch.object(m,'DC0_SOURCE_UNIT_RAW',m.DC0_SOURCE_UNIT_RAW+b' '),self.assertRaisesRegex(m.Rejected,'dc0_source_unit_raw'):
            m.dc0_residual_source(ROOT)

    def test_actual_append_binds_manual_identity_not_original_parent_operation(self):
        m=module()
        with tempfile.TemporaryDirectory() as folder:
            folder=Path(folder);trace=folder/'trace';spec=folder/'spec';mapping=folder/'map'
            spec.write_text('{}');mapping.write_bytes(b'')
            original='F5/'+m.DC0_MERGE+'/2'
            identity=m.append(trace,'dc0_residual_begin','F5',original,ROOT,[str(spec),str(mapping)])
            event=json.loads(trace.read_bytes())
            self.assertEqual(identity,'F5/'+m.DC0_MERGE+'/manual-result')
            self.assertEqual(event['operation'],identity)
            self.assertEqual(event['payload']['id'],identity)


class Manual3071RenderingContract(unittest.TestCase):
    def test_frozen_patch_is_independent_of_default_abbreviation(self):
        m=module()
        for width in ('7','10','40'):
            with self.subTest(core_abbrev=width),patch.dict(os.environ,{
                'GIT_CONFIG_COUNT':'1','GIT_CONFIG_KEY_0':'core.abbrev','GIT_CONFIG_VALUE_0':width}):
                result=m.manual_3071_composition(ROOT)
                self.assertEqual(result['raw_difference']['sha256'],'9f7f1689717879d2877428fae2834a6f69385c118a9d533bbc0b8ed2a8fa0920')
                self.assertEqual(result['patch']['bytes'],1238)
                self.assertEqual(result['patch']['sha256'],'78ebeaf659af962865b25a662045825e2bf9c06549fca4d64e6fc762e42358ea')

    def test_expanded_git_prefix_is_rejected_not_truncated(self):
        m=module();original=m.git
        # Fresh clones do not retain unreachable automatic merge trees. Build
        # and verify the exact original composition before reading its patch.
        m.manual_3071_composition(ROOT)
        # Model Git returning a longer unique prefix using actual ten-digit Git
        # output. This is a transport oracle, not a manufactured SHA collision.
        expanded=original(ROOT,'diff','--binary','--abbrev=10',m.J3071_AUTO_TREE,m.J3071)
        self.assertIn(b'index ffb31b64dc..64b197ec4e ',expanded)
        def returned(repo,*args):
            return expanded if args[:2]==('diff','--binary') else original(repo,*args)
        with patch.object(m,'git',side_effect=returned),self.assertRaisesRegex(m.Rejected,'fixed3071_exact_whole_tree_manual_resolution_diff'):
            m.manual_3071_composition(ROOT)


class FiniteJHistoricalContract(unittest.TestCase):
    """Finite source and transition controls; actual full-prefix replay is separate."""
    def test_finite_permission_is_distinct_required_and_consumed_exactly(self):
        m=module(); permission={'type':'finite-control','path':'historical-only'}
        item=dict(op='F5/J/2',finite_J_owner_permission=permission)
        m.validate_exact_conservation(item,item['op'],dict(finite_J_owner_permission=permission))
        for receipt in ({},dict(exact_J_owner_permission=permission),
                        dict(finite_J_owner_permission=None),dict(finite_J_owner_permission={'changed':True})):
            with self.subTest(receipt=receipt),self.assertRaises(m.Rejected):
                m.validate_exact_conservation(item,item['op'],receipt)
        with self.assertRaises(m.Rejected):
            m.validate_exact_conservation(dict(op=item['op']),item['op'],dict(finite_J_owner_permission=permission))

    def test_source_unit_preserves_ten_literal_rows_and_no_current_equivalence(self):
        m=module(); unit=m.finite_J_source_unit(ROOT)
        self.assertEqual(len(unit['cases']),10)
        self.assertNotIn(m.EXACT_J_AUDIT_PATH,[x['path'] for x in unit['cases']])
        self.assertFalse(unit['admission']);self.assertFalse(unit['runtime_pass'])
        for case in unit['cases']:
            self.assertEqual(case['prior'].split('\t')[1],'false')
            self.assertEqual(case['incoming'].split('\t')[1],'true')
        self.assertEqual(sum(len(x['versions']) for x in unit['cases']),114)
        self.assertEqual(sum(len(x['edges']) for x in unit['cases']),168)

    def test_source_unit_mutations_cannot_become_permission(self):
        m=module()
        for old,new in ((b'100644 blob',b'100755 blob'),(b'2445e81',b'1445e81'),(b'false',b'true')):
            with self.subTest(change=new),patch.object(m,'FINITE_J_SOURCE_UNIT_RAW',m.FINITE_J_SOURCE_UNIT_RAW.replace(old,new,1)):
                with self.assertRaisesRegex(m.Rejected,'finite_J_frozen_source_unit'):
                    m.finite_J_source_unit(ROOT)

    def test_rehashed_source_inventory_still_reconstructs_actual_git(self):
        m=module()
        for change,reason in (('version','complete_source_inventory'),('edge','complete_source_inventory'),
                              ('parent','ordered_parents'),('bytes','historical_complete_bytes'),
                              ('tree','historical_tree_entry')):
            with self.subTest(change=change):
                unit=json.loads(m.FINITE_J_SOURCE_UNIT_RAW);case=unit['cases'][0]
                if change=='version':case['versions'].pop(next(iter(case['versions'])))
                elif change=='edge':case['edges'].pop(next(iter(case['edges'])))
                elif change=='parent':unit['ordered_parents'].reverse()
                else:
                    version=next(v for v in case['versions'].values() if v['content'])
                    if change=='bytes':version['content']['sha256']='0'*64
                    else:version['tree_record']=version['tree_record'].replace('100644','100755')
                raw=m.canonical(unit)
                with patch.object(m,'FINITE_J_SOURCE_UNIT_RAW',raw),patch.object(m,'FINITE_J_SOURCE_UNIT_SHA',m.sha(raw)):
                    with self.assertRaisesRegex(m.Rejected,'finite_J_'+reason):m.finite_J_source_unit(ROOT)

    @contextmanager
    def session_fixture(self):
        """FSM-only fixture: bootstrap authority is mocked, never claimed as proof."""
        m=module();unit=json.loads(m.FINITE_J_SOURCE_UNIT_RAW);cases=unit['cases']
        op='F5/'+m.J3071+'/2';cap=dict(type='F54_FIXED3071_PRODUCTION_CAPABILITIES_v1',fixture=True)
        ready=dict(frame='F5',operation=op,operation_binding=dict(paths=[c['path'] for c in cases]),
            capability_sha256=m.sha(m.canonical(cap)),composition=dict(whole={'fixture':True}),
            required_qualification={c['path']:dict(aggregate=True,retained_evidence=[{'prior':'kept'}],
                inherited_qualification=[{'aggregate':True}],prior_span_evidence=[],J_classification={'trigger':'true'}) for c in cases})
        rows=[c['prior'] for c in cases];events=[]
        with tempfile.TemporaryDirectory() as folder,ExitStack() as patches:
            folder=Path(folder);trace=folder/'trace';map_path=folder/'map';ready_path=folder/'ready';cap_path=folder/'cap'
            def map_bytes():return ('\n'.join(rows)+'\n').encode()
            def append(kind,payload,operation=op):
                event=dict(schema=m.SCHEMA,sequence=len(events)+1,previous=events[-1]['digest'] if events else '0'*64,
                    kind=kind,frame='F5',operation=operation,payload=payload)
                event['digest']=m.sha(m.canonical(event));events.append(event)
                with trace.open('ab') as stream:stream.write(m.canonical(event)+b'\n')
                return event
            def pending(index):
                case=cases[index]
                return append('row_before',dict(path=case['path'],prior=m.raw_binding(case['prior']),
                    prior_rows=[m.raw_binding(case['prior'])],incoming=m.raw_binding(case['incoming']),
                    map_before=m.file_digest(map_bytes()),phase='before_qualification'))
            prior_op='F5/'+cases[0]['prior'].split('\t')[2]+'/2'
            append('operation_begin',{},prior_op);append('classification',{'trigger':'false'},prior_op)
            for case in cases:
                append('row_after',dict(path=case['path'],row=m.raw_binding(case['prior'])),prior_op)
                append('row_committed',dict(path=case['path'],row=m.raw_binding(case['prior'])),prior_op)
            pending(0);map_path.write_bytes(map_bytes());ready_path.write_bytes(m.canonical(ready));cap_path.write_bytes(m.canonical(cap))
            initial=dict(ready=ready,capability=cap,map=map_bytes().decode(),phase='permission',index=0,permission=None)
            bootstrap=patches.enter_context(patch.object(m,'validate_trace',return_value={'finite_J_checkpoint':initial}))
            patches.enter_context(patch.object(m,'verified_3071_capabilities',return_value=cap))
            patches.enter_context(patch.object(m,'checked_3071_capability_value',return_value=cap))
            patches.enter_context(patch.object(m,'finite_J_source_unit',return_value=unit))
            patches.enter_context(patch.object(m,'manual_3071_composition',return_value={'fixture':True}))
            state=m.FiniteJSession(ROOT,cap_path,ready_path,m.sha(ready_path.read_bytes()),trace,map_path)
            def complete(index):
                case=cases[index];name=case['path'];incoming=m.raw_binding(case['incoming'])
                permission=json.loads(state.request('permission',name))
                append('finite_J_owner_permission',dict(path=name,receipt=permission,admission=False))
                conserved=json.loads(state.request('consume',name))
                append('range_J_ledger_conserved',dict(path=name,receipt=conserved,policy_result=None,admission=False))
                append('row_remove',dict(path=name,row=incoming,reason='original_guarded_map_replacement',phase='before_mutation'))
                rows[:]=[r for r in rows if r.split('\t')[0]!=name]
                append('row_after',dict(path=name,row=incoming,reason='original_owner_and_row_assignment',phase='before_mutation'))
                rows.append(case['incoming']);map_path.write_bytes(map_bytes())
                append('row_committed',dict(path=name,row=incoming,retained_rows=[incoming],map_after=m.file_digest(map_bytes())))
                return permission,conserved
            yield dict(m=m,state=state,cases=cases,ready=ready,cap=cap,unit=unit,trace=trace,map=map_path,
                ready_path=ready_path,cap_path=cap_path,events=events,rows=rows,append=append,pending=pending,
                complete=complete,bootstrap=bootstrap)

    def test_process_local_unit_consumes_all_ten_only_after_actual_commits(self):
        with self.session_fixture() as f:
            for index,case in enumerate(f['cases']):
                if index:f['pending'](index)
                permission=f['state'].request('permission',case['path'])
                self.assertEqual(permission,f['state'].request('permission',case['path']), 'exact pending retry')
                self.assertEqual(f['rows'][index if index==0 else 0].split('\t')[1],'false')
                p,c=f['complete'](index)
                self.assertEqual(c['finite_J_owner_permission'],p)
                self.assertEqual(c['obligations'],f['ready']['required_qualification'][case['path']])
            self.assertEqual(f['state'].request('finish',''),'')
            self.assertEqual(f['bootstrap'].call_count,1)
            with self.assertRaisesRegex(f['m'].Rejected,'finished_session'):f['state'].request('finish','')

    def test_no_future_phase_partial_finish_or_intervening_event_is_accepted(self):
        for kind in ('future','consume_early','finish_early','unexpected','duplicate_permission','remove_before_conservation'):
            with self.subTest(kind=kind),self.session_fixture() as f:
                m=f['m'];state=f['state'];name=f['cases'][0]['path']
                if kind in ('duplicate_permission','remove_before_conservation'):
                    p=json.loads(state.request('permission',name));payload=dict(path=name,receipt=p,admission=False)
                    f['append']('finite_J_owner_permission',payload)
                    f['append']('finite_J_owner_permission' if kind=='duplicate_permission' else 'row_remove',payload)
                elif kind=='unexpected':f['append']('classification',dict(path=name,trigger='true'))
                with self.assertRaises(m.Rejected):
                    state.request('finish' if kind=='finish_early' else 'consume' if kind=='consume_early' else 'permission',
                        '' if kind=='finish_early' else f['cases'][1]['path'] if kind=='future' else name)

    def test_prefix_ready_map_capability_and_event_context_are_fresh(self):
        for kind in ('prefix','ready','map','capability','event'):
            with self.subTest(kind=kind),self.session_fixture() as f:
                m=f['m'];state=f['state'];name=f['cases'][0]['path']
                if kind in ('prefix','ready','map'):
                    path=f['trace'] if kind=='prefix' else f['ready_path'] if kind=='ready' else f['map']
                    stat=path.stat();raw=path.read_bytes();path.write_bytes(bytes([raw[0]^1])+raw[1:])
                    os.utime(path,ns=(stat.st_atime_ns,stat.st_mtime_ns))
                if kind=='capability':
                    with patch.object(m,'verified_3071_capabilities',return_value=dict(f['cap'],changed=True)),self.assertRaises(m.Rejected):
                        state.request('permission',name)
                elif kind=='event':
                    with patch.dict(os.environ,{'GITHUB_SHA':'changed'}),self.assertRaises(m.Rejected):state.request('permission',name)
                else:
                    with self.assertRaises(m.Rejected):state.request('permission',name)

    def test_lost_obligations_or_changed_permission_cannot_authorize_removal(self):
        for kind in ('aggregate','evidence','permission','map','row'):
            with self.subTest(kind=kind),self.session_fixture() as f:
                m=f['m'];name=f['cases'][0]['path'];p=json.loads(f['state'].request('permission',name))
                f['append']('finite_J_owner_permission',dict(path=name,receipt=p,admission=False))
                c=json.loads(f['state'].request('consume',name))
                if kind=='aggregate':c['aggregate']=False
                elif kind=='evidence':c['obligations']['retained_evidence']=[]
                elif kind=='permission':c['finite_J_owner_permission']['prior_operation']='independent-same-blob-owner'
                elif kind=='map':c['expected_post_map']['sha256']='0'*64
                else:c['after_gate_row']=m.raw_binding(c['after_gate_row']['raw'].replace('\ttrue\t','\tfalse\t',1))
                f['append']('range_J_ledger_conserved',dict(path=name,receipt=c,policy_result=None,admission=False))
                with self.assertRaisesRegex(m.Rejected,'finite_J_tail_complete_conservation'):
                    f['state'].request('consume',name)

    def test_each_literal_prior_and_incoming_field_is_required(self):
        for field in ('prior','incoming'):
            for index in range(8):
                with self.subTest(field=field,index=index),self.session_fixture() as f:
                    m=f['m'];events=copy.deepcopy(f['events']);before=events[-1]
                    parts=before['payload'][field]['raw'].split('\t');parts[index]+='-changed'
                    before['payload'][field]=m.raw_binding('\t'.join(parts))
                    if field=='prior':before['payload']['prior_rows']=[before['payload']['prior']]
                    with self.assertRaisesRegex(m.Rejected,'finite_J_complete_literal_rows'):
                        m.finite_J_owner_evidence(ROOT,f['cap'],f['ready'],events,f['state'].prefix,f['map'].read_bytes())

    def test_worker_restart_always_reconstructs_and_never_accepts_disk_state(self):
        with self.session_fixture() as f:
            m=f['m'];f['state'].request('permission',f['cases'][0]['path'])
            m.FiniteJSession(ROOT,f['cap_path'],f['ready_path'],m.sha(f['ready_path'].read_bytes()),f['trace'],f['map'])
            self.assertEqual(f['bootstrap'].call_count,2)
            with patch.object(m,'validate_trace',side_effect=m.Rejected('changed-prefix')),self.assertRaisesRegex(m.Rejected,'changed-prefix'):
                m.FiniteJSession(ROOT,f['cap_path'],f['ready_path'],m.sha(f['ready_path'].read_bytes()),f['trace'],f['map'])


class FiniteJTransportContract(unittest.TestCase):
    def test_extracted_shell_drives_real_cli_socket_and_owned_cleanup(self):
        # Only authority State is mocked; real shell functions, CLI framing,
        # socket server/client, explicit finish and EXIT cleanup execute.
        wanted={'f13_cleanup','f13_f54_finite_J_owner_permission','f13_f54_consume_J_before_mutation','f13_f54_finite_J_finish'}
        functions={match.group(1):match.group(0) for match in re.finditer(
            r'(?ms)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{\n.*?^\}\n',SHELL.read_text()) if match.group(1) in wanted}
        self.assertEqual(set(functions),wanted)
        for mode in ('success','failed_consume','worker_death'):
            with self.subTest(mode=mode),tempfile.TemporaryDirectory() as directory:
                folder=Path(directory);temp=folder/'private';temp.mkdir(mode=0o700)
                wrapper=folder/'observer.py';calls=folder/'calls';events=folder/'events';pid=folder/'pid'
                wrapper.write_text('''import importlib.util,json,os,sys
from pathlib import Path
spec=importlib.util.spec_from_file_location('candidate',sys.argv.pop(1))
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
class State:
    def __init__(self,*args):self.event_context=m.finite_J_event_context()
    def request(self,command,name):
        with open(os.environ['CALLS'],'a') as stream:stream.write(command+'\\n')
        if command=='consume' and os.environ['MODE']=='failed_consume':raise m.Rejected('fixture-consumer-denied')
        if command=='finish':return ''
        incoming=m.raw_binding(os.environ['INCOMING'])
        return json.dumps(dict(incoming=incoming,transition='replace',after_gate_row=incoming))+'\\n'
m.FiniteJSession=State
sys.exit(m.main())
''')
                # The shell's observer takes argv directly; pin helper in wrapper.
                wrapper.write_text(wrapper.read_text().replace("sys.argv.pop(1)",repr(str(HELPER))))
                m=module();case=json.loads(m.FINITE_J_SOURCE_UNIT_RAW)['cases'][-1]
                env=dict(os.environ,CALLS=str(calls),MODE=mode,INCOMING=case['incoming'])
                values=dict(TMP_DIR=str(temp),f13_f54_observer=str(wrapper),f13_f54_frame='F5',
                    f13_f54_operation='F5/'+m.J3071+'/2',f13_f54_J_operation='F5/'+m.J3071+'/2',
                    f13_f54_3071_capability_file='fixture',f13_f54_J_ledger='fixture',f13_f54_J_ledger_sha='fixture',
                    f13_f54_trace_file='fixture',name=case['path'],incoming=case['incoming'])
                program='set -eu\n'+'\n'.join(functions.values())+'\n'
                program+='\n'.join(k+'='+shlex.quote(v) for k,v in values.items())+'\ntrap f13_cleanup EXIT\n'
                program+='f13_f54_trace_event() { printf "%s\\n" "$1" >>'+shlex.quote(str(events))+'; }\n'
                program+='f13_f54_finite_J_owner_permission unused "$name" unused "$incoming"\n'
                program+='printf "%s" "$f13_finite_J_worker_pid" >'+shlex.quote(str(pid))+'\n'
                if mode=='worker_death':program+='kill "$f13_finite_J_worker_pid"; wait "$f13_finite_J_worker_pid" || true\n'
                program+='f13_f54_consume_J_before_mutation unused "$name" unused "$incoming" replace || exit 37\n'
                program+='f13_f54_finite_J_finish "$name"\n[[ -z "$f13_finite_J_worker_pid" ]]\n'
                script=folder/'driver.sh';script.write_text(program)
                result=subprocess.run(['bash',str(script)],env=env,capture_output=True,text=True,timeout=20)
                self.assertEqual(result.returncode,0 if mode=='success' else 37,result.stderr)
                self.assertFalse(temp.exists())
                self.assertEqual(events.read_text().splitlines(),['finite_J_owner_permission']+(['range_J_ledger_conserved'] if mode=='success' else []))
                self.assertEqual(calls.read_text().splitlines(),['permission','consume','finish'] if mode=='success' else
                                 ['permission','consume'] if mode=='failed_consume' else ['permission'])
                with self.assertRaises(ProcessLookupError):os.kill(int(pid.read_text()),0)

    def test_client_rejects_changed_identity_malformed_frames_and_dead_worker(self):
        m=module()
        for change in ('nonce','session','command','path','context','extra','status','output','eof','trailing'):
            with self.subTest(change=change),tempfile.TemporaryDirectory() as folder:
                path=str(Path(folder)/'s')
                with socket.socket(socket.AF_UNIX,socket.SOCK_STREAM) as server:
                    server.bind(path);server.listen(1)
                    def reply():
                        connection,_=server.accept()
                        with connection:
                            request=m._router_socket_read(connection,65536)
                            response=dict(request,status=0,output='')
                            if change in request:response[change]='wrong'
                            elif change=='extra':response['extra']=True
                            elif change=='status':response['status']=False
                            elif change=='output':response['output']={}
                            if change!='eof':connection.sendall(json.dumps(response).encode()+(b'{}' if change=='trailing' else b''))
                    thread=threading.Thread(target=reply);thread.start()
                    with self.assertRaises((m.Rejected,ValueError)):
                        m.finite_J_worker_request(path,'a'*64,'permission','some/path')
                    thread.join(timeout=5);self.assertFalse(thread.is_alive())
        with tempfile.TemporaryDirectory() as folder,self.assertRaises(OSError):
            m.finite_J_worker_request(str(Path(folder)/'missing'),'a'*64,'permission','path')

    def test_worker_publishes_ready_atomically_and_requires_explicit_finish(self):
        # Transport-only oracle. The session's authority reconstruction has its own tests.
        m=module();entered=threading.Event();release=threading.Event();errors=[];calls=[]
        class State:
            event_context=m.finite_J_event_context()
            def request(self,command,name):
                calls.append((command,name));return '' if command=='finish' else 'receipt\n'
        with tempfile.TemporaryDirectory() as folder:
            os.chmod(folder,0o700);path=str(Path(folder)/'s');session='a'*64;link=os.link
            def paused_link(source,target):
                self.assertEqual(Path(source).read_text(),session+'\n')
                self.assertFalse(Path(target).exists());entered.set()
                if not release.wait(5):raise AssertionError('publication release missing')
                link(source,target)
            def serve():
                try:m.finite_J_worker(path,session,*(['unused']*6))
                except Exception as error:errors.append(error)
            with patch.object(m,'FiniteJSession',return_value=State()) as bootstrap,patch.object(m.os,'link',paused_link):
                thread=threading.Thread(target=serve);thread.start()
                try:
                    self.assertTrue(entered.wait(5));self.assertFalse(Path(path+'.ready').exists());release.set()
                    for _ in range(200):
                        if Path(path+'.ready').exists():break
                        time.sleep(.01)
                    self.assertEqual(Path(path+'.ready').read_text(),session+'\n')
                    self.assertEqual(Path(path).stat().st_mode & 0o777,0o600)
                    self.assertEqual(m.finite_J_worker_request(path,session,'permission','p'),'receipt\n')
                    self.assertTrue(thread.is_alive())
                    self.assertEqual(m.finite_J_worker_request(path,session,'finish',''),'')
                finally:release.set();thread.join(timeout=5)
                bootstrap.assert_called_once()
            self.assertFalse(thread.is_alive());self.assertEqual(errors,[])
            self.assertEqual(calls,[('permission','p'),('finish','')])
            self.assertFalse(Path(path).exists());self.assertFalse(Path(path+'.ready').exists())


class ReadyDigestHoistContract(unittest.TestCase):
    def fixtures(self,m):
        def ready(kind):
            return dict(type='F54_FIXED3071_ACTUAL_'+kind+'_COMPLETE_LEDGER' if kind=='J' else 'F54_FIXED3071_ACTUAL_I_RANGE_LEDGER',
                        operation=kind,admission=False,required_qualification={
                            'a':dict(aggregate=True,evidence=['a']),
                            'b':dict(aggregate=False,evidence=['b'])})
        inherited=ready('I');other=ready('J')
        ledger=dict(rows=[],paths={},map_binding=dict(bytes=0,sha256=m.sha(b'')))
        state=dict(operation_binding=dict(commit=m.J3071,paths=[]),operation='F5/'+m.J3071+'/2',
                   classification=dict(trigger='false'),ledger={'F5':ledger},frame='F5',
                   M_ready={'M':other},I_ready={'I':inherited},prior_span=dict(ledger=ledger,
                   trace_sha256='trace',actual_operations=[]),capability_sha256='cap',frame_ancestry=[],coverage=[],composition={})
        return state,inherited,other

    def test_each_ready_digest_once_output_bound_and_inputs_unchanged(self):
        m=module();state,inherited,other=self.fixtures(m);before=m.canonical(state)
        original=m.canonical;counts={id(inherited):0,id(other):0}
        def counted(value):
            if id(value) in counts:counts[id(value)]+=1
            return original(value)
        expected={id(value):m.sha(original(value)) for value in (inherited,other)}
        with patch.object(m,'canonical',side_effect=counted):
            built=m.build_3071_J_ready(state,{})
        self.assertEqual(list(counts.values()),[1,1]);self.assertEqual(original(state),before)
        for obligation in built['required_qualification'].values():
            self.assertEqual({x['ledger_sha256'] for x in obligation['inherited_qualification']},set(expected.values()))
        counts=dict.fromkeys(counts,0);required={'kept':dict(aggregate=False)};saved=original(required)
        with patch.object(m,'canonical',side_effect=counted):
            collected=m.include_typed_collection_required(required,{'I':inherited},{'J':other})
        self.assertEqual(list(counts.values()),[1,1]);self.assertEqual(original(state),before)
        self.assertEqual(original(required),saved)
        for name in ('a','b'):
            self.assertEqual({x['ledger_sha256'] for x in collected[name]['retained_evidence']},set(expected.values()))
        inherited['required_qualification']['a']['evidence'].append('changed')
        changed=m.build_3071_J_ready(state,{})
        self.assertNotEqual(original(built),original(changed))
        self.assertEqual(changed['required_qualification']['a']['inherited_qualification'][1]['ledger_sha256'],m.sha(original(inherited)))
        changed_collection=m.include_typed_collection_required(required,{'I':inherited},{'J':other})
        self.assertEqual(changed_collection['a']['retained_evidence'][0]['ledger_sha256'],m.sha(original(inherited)))

    def test_empty_and_invalid_ledgers_do_not_serialize_before_old_checks(self):
        m=module();state,inherited,other=self.fixtures(m)
        for ready in (inherited,other):ready['required_qualification']={}
        with patch.object(m,'canonical',side_effect=AssertionError('unexpected ready serialization')):
            m.build_3071_J_ready(state,{})
            m.include_typed_collection_required({}, {'I':inherited},{'J':other})
        inherited['required_qualification']={'a':dict(aggregate=1)}
        with patch.object(m,'canonical',side_effect=AssertionError('unexpected ready serialization')):
            with self.assertRaisesRegex(Exception,'fixed3071_J_inherited_boolean'):m.build_3071_J_ready(state,{})
            with self.assertRaisesRegex(Exception,'fixed3071_collection_typed_boolean'):
                m.include_typed_collection_required({}, {'I':inherited},{'J':other})


class CollectionTraceIndexContract(unittest.TestCase):
    def test_ready_marker_is_invisible_until_complete_atomic_publication(self):
        m=module();entered=threading.Event();release=threading.Event();errors=[]
        with tempfile.TemporaryDirectory() as folder:
            path=str(Path(folder)/'s');session='a'*64;link=os.link
            def paused_link(source,target):
                self.assertEqual(Path(source).read_text(),session+'\n')
                self.assertFalse(Path(target).exists());entered.set()
                if not release.wait(5):raise AssertionError('publication release missing')
                link(source,target)
            def serve():
                try:m.router_worker(path,session,'unused','unused','unused','unused','unused')
                except Exception as error:errors.append(error)
            with patch.object(m.os,'link',paused_link):
                thread=threading.Thread(target=serve);thread.start()
                try:
                    self.assertTrue(entered.wait(5));self.assertFalse(Path(path+'.ready').exists())
                    release.set()
                    for _ in range(200):
                        if Path(path+'.ready').exists():break
                        time.sleep(.01)
                    self.assertEqual(Path(path+'.ready').read_text(),session+'\n')
                    self.assertEqual(m.router_worker_request(path,session,'stop',''),(0,''))
                finally:release.set();thread.join(timeout=5)
            self.assertFalse(thread.is_alive());self.assertEqual(errors,[])
            self.assertFalse(Path(path).exists());self.assertFalse(Path(path+'.ready').exists())

    def test_exact_raw_bytes_and_fresh_decoded_results(self):
        m=module()
        events=[dict(kind=kind,payload={'value':'a'}) for kind in
                ('row_before','manual_F_end','reviewed_F_deletion','range_I_ledger_ready','range_J_ledger_ready')]
        raw=b'\n'.join(json.dumps(e).encode() for e in events)+b'\n'
        expected=events[1:]
        self.assertEqual(m.collection_trace_events(raw),expected)
        mutated=m.collection_trace_events(raw);mutated[0]['payload']['value']='poison'
        self.assertEqual(m.collection_trace_events(raw),expected)
        changed=raw.replace(b'"a"',b'"b"')
        self.assertEqual(len(changed),len(raw))
        self.assertNotEqual(m.collection_trace_events(changed),expected)
        self.assertEqual(m._collection_trace_index.cache_info().currsize,1)
        self.assertEqual(m.collection_trace_events(raw),expected)

    def test_irrelevant_malformed_event_is_not_skipped(self):
        m=module()
        for raw in (b'{"kind":"row_before"}\n{bad}\n',b'{"payload":{}}\n'):
            with self.subTest(raw=raw), self.assertRaises((ValueError,KeyError)):
                m.collection_trace_events(raw)

    def test_cached_prefix_does_not_replace_fresh_hash_validation(self):
        m=module();raw=b'{"kind":"row_before","payload":"a"}\n'
        prefix=dict(bytes=len(raw),sha256=m.sha(raw))
        m.collection_trace_events(raw)
        for validate in (m.manual_operations_in_prefix,m.terminal_dispositions_in_prefix):
            with self.subTest(validate=validate.__name__),self.assertRaises(m.Rejected):
                validate({},raw.replace(b'"a"',b'"b"'),prefix)

    def test_worker_client_rejects_wrong_identity_shape_eof_and_trailing_frames(self):
        m=module()
        for change in ('nonce','session','command','path','extra','status','eof','trailing'):
            with self.subTest(change=change),tempfile.TemporaryDirectory() as folder:
                path=str(Path(folder)/'s')
                with socket.socket(socket.AF_UNIX,socket.SOCK_STREAM) as server:
                    server.bind(path);server.listen(1)
                    def reply():
                        connection,_=server.accept()
                        with connection:
                            request=m._router_socket_read(connection,65536)
                            response=dict(request,status=0,output='')
                            if change in request:response[change]='wrong'
                            elif change=='extra':response['extra']=True
                            elif change=='status':response['status']=False
                            if change!='eof':connection.sendall(json.dumps(response).encode()+(b'{}' if change=='trailing' else b''))
                    thread=threading.Thread(target=reply);thread.start()
                    with self.assertRaises((m.Rejected,ValueError)):
                        m.router_worker_request(path,'a'*64,'bind','some/path')
                    thread.join(timeout=5);self.assertFalse(thread.is_alive())
        with tempfile.TemporaryDirectory() as folder,self.assertRaises(OSError):
            m.router_worker_request(str(Path(folder)/'missing'),'a'*64,'bind','path')


class ReplayConservationContract(unittest.TestCase):
    """Transition controls; full historical replay remains a separate gate."""
    def setUp(self):
        self.m = module()
        self.op = 'F5/first/2'
        self.key = ('F5', PROTO)
        self.exact = dict(operation=self.op, path=PROTO, evidence='qualified-private-control')

    def test_same_transition_requires_exact_receipt_presence_and_value(self):
        item = dict(op=self.op, exact_unit_supersession=self.exact)
        self.m.validate_exact_conservation(item, self.op, dict(exact_unit_supersession=self.exact))
        for receipt in ({}, dict(exact_unit_supersession=None), dict(exact_unit_supersession={}),
                        dict(exact_unit_supersession=dict(self.exact, operation='F5/other/2'))):
            with self.subTest(receipt=receipt), self.assertRaises(self.m.Rejected):
                self.m.validate_exact_conservation(item, self.op, receipt)

    def test_later_operation_rejects_any_unrequested_exact_receipt_field(self):
        later = dict(op='F5/later/2')
        self.m.validate_exact_conservation(later, later['op'], {})
        for stale in (self.exact, None, {}, False):
            with self.subTest(stale=stale), self.assertRaises(self.m.Rejected):
                self.m.validate_exact_conservation(later, later['op'], dict(exact_unit_supersession=stale))

    def test_pending_operation_ownership_is_required_for_registration_and_commit(self):
        pending = {self.key: dict(op=self.op)}
        self.assertIs(self.m.require_pending_operation(pending, self.key, self.op), pending[self.key])
        for key, operation in ((self.key, 'F5/other/2'), (('F6', PROTO), self.op)):
            with self.assertRaises(self.m.Rejected):
                self.m.require_pending_operation(pending, key, operation)
        self.assertEqual(pending, {self.key: dict(op=self.op)})

    def test_exact_J_permission_is_distinct_and_consumed_without_M_alias(self):
        item = dict(op=self.op, exact_J_owner_permission=self.exact)
        self.m.validate_exact_conservation(item, self.op, dict(exact_J_owner_permission=self.exact))
        for receipt in ({}, dict(exact_unit_supersession=self.exact),
                        dict(exact_J_owner_permission=None),
                        dict(exact_J_owner_permission=dict(self.exact, operation='other'))):
            with self.subTest(receipt=receipt), self.assertRaises(self.m.Rejected):
                self.m.validate_exact_conservation(item, self.op, receipt)
        with self.assertRaises(self.m.Rejected):
            self.m.validate_exact_conservation(dict(op=self.op), self.op,
                                               dict(exact_J_owner_permission=self.exact))

    def test_exact_J_source_unit_is_frozen_inside_four_file_carrier(self):
        self.assertEqual(self.m.EXACT_J_SOURCE_UNIT_RAW,
                         (ROOT/'scripts/ci/testdata/f13-exact-j-547/source-unit.json').read_bytes())
        for old, new in ((b'100644 blob', b'100755 blob'), (b'100644 blob', b'100644 tree'),
                         (b'd24e08d221c42661e8b1431defe61d742b23e8d9', b'0'*40)):
            with self.subTest(change=new), patch.object(self.m, 'EXACT_J_SOURCE_UNIT_RAW',
                    self.m.EXACT_J_SOURCE_UNIT_RAW.replace(old, new, 1)):
                with self.assertRaisesRegex(self.m.Rejected, 'exact_J_frozen_full_source_unit'):
                    self.m.exact_J_audit_source_unit(ROOT)

    def test_exact_J_rejects_every_changed_raw_row_field_without_mutation(self):
        import copy
        import zipfile
        with zipfile.ZipFile(ROOT/'scripts/ci/testdata/f13-exact-j-547/historical-inputs.zip') as archive:
            raw = archive.read('objects/08259a181aca2a185a3310aa2f745f663fc6a0baddc458418136b02da02e4fe8')
            actual_map = archive.read('objects/f088e093d271ea39b5394be0dd217d1d17f99782b8f7349d39be5eac0dde9e45')
        before = json.loads(raw.splitlines()[10644])
        ready = dict(operation=before['operation'], frame=before['frame'])
        cap = dict(type='F54_FIXED3071_CURRENT_AUTHOR_CAPABILITIES')
        frozen = self.m.sha(actual_map)
        # These are refusal-only controls. No fabricated ready record is used
        # for a positive grant; actual-prefix positive is the separate harness.
        for field in ('prior', 'incoming'):
            for index in range(8):
                changed = copy.deepcopy(before)
                parts = changed['payload'][field]['raw'].split('\t')
                parts[index] += '-changed'
                changed['payload'][field] = self.m.raw_binding('\t'.join(parts))
                if field == 'prior':
                    changed['payload']['prior_rows'] = [changed['payload']['prior']]
                with self.subTest(field=field, index=index), self.assertRaisesRegex(self.m.Rejected, 'exact_J_complete_fixed_raw_rows'):
                    self.m.exact_J_owner_evidence(ROOT, cap, ready, [changed], self.m.file_digest(raw), actual_map)
                self.assertEqual(self.m.sha(actual_map), frozen)
        for operation in ('F5/'+self.m.M+'/2', 'F5/'+self.m.J3071+'/1', 'F6/third/2'):
            changed = copy.deepcopy(before)
            changed['operation'] = operation
            with self.subTest(operation=operation), self.assertRaises(self.m.Rejected):
                self.m.exact_J_owner_evidence(ROOT, cap, ready, [changed], self.m.file_digest(raw), actual_map)

    def test_frozen_trace_supersession_is_consumed_before_unrelated_J_transition(self):
        import zipfile
        with zipfile.ZipFile(ROOT/'scripts/ci/testdata/f13-exact-j-547/historical-inputs.zip') as archive:
            raw = archive.read('objects/08259a181aca2a185a3310aa2f745f663fc6a0baddc458418136b02da02e4fe8')
        self.assertEqual(hashlib.sha256(raw).hexdigest(), '08259a181aca2a185a3310aa2f745f663fc6a0baddc458418136b02da02e4fe8')
        events = [json.loads(line) for line in raw.splitlines()]
        exact, conserved, committed, later = [events[n-1] for n in (6665, 6666, 6669, 10293)]
        self.assertEqual(exact['operation'], conserved['operation'])
        self.assertEqual(committed['operation'], exact['operation'])
        self.assertNotEqual(later['operation'], exact['operation'])
        self.assertEqual(exact['payload']['path'], later['payload']['path'])
        self.assertEqual(conserved['payload']['receipt']['exact_unit_supersession'], exact['payload']['receipt'])
        first = dict(op=exact['operation'], exact_unit_supersession=exact['payload']['receipt'])
        self.m.validate_exact_conservation(first, exact['operation'], conserved['payload']['receipt'])
        # Isolate the real receipt boundary, without claiming an admitted frame.
        next_item = dict(op=later['operation'])
        self.m.validate_exact_conservation(next_item, later['operation'], later['payload']['receipt'])
        stale = dict(later['payload']['receipt'], exact_unit_supersession=exact['payload']['receipt'])
        with self.assertRaises(self.m.Rejected):
            self.m.validate_exact_conservation(next_item, later['operation'], stale)

    def test_successor_binding_checks_git_disk_hashes_and_binds_once(self):
        with tempfile.TemporaryDirectory() as folder:
            repo = Path(folder).resolve()
            git(repo, 'init', '-q')
            git(repo, 'config', 'user.name', 'Private fixture')
            git(repo, 'config', 'user.email', 'private@example.invalid')
            for name in self.m.AUTHOR_SOURCE_PATHS:
                path = repo/name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(('private historical carrier '+name+'\n').encode())
            git(repo, 'add', '.')
            git(repo, 'commit', '-qm', 'private historical carrier')
            source = git(repo, 'rev-parse', 'HEAD')
            hashes = {name:self.m.sha((repo/name).read_bytes()) for name in self.m.AUTHOR_SOURCE_PATHS}
            original = module()
            original_root = original.ROOT
            original_globals = dict(vars(original))
            for change in ('missing', 'hash', 'source', 'disk'):
                successor = module()
                wrong = dict(hashes)
                ref = source
                if change == 'missing': wrong.pop(next(iter(wrong)))
                if change == 'hash': wrong[next(iter(wrong))] = '0'*64
                if change == 'source': ref = 'not-a-full-commit'
                path = repo/next(iter(hashes))
                before = path.read_bytes()
                if change == 'disk': path.write_bytes(before+b'changed')
                try:
                    with self.subTest(change=change), self.assertRaises(successor.Rejected):
                        successor.bind_verified_replay_root(repo, ref, wrong)
                    self.assertIsNone(successor._REPLAY_ROOT_BINDING)
                finally:
                    path.write_bytes(before)
            successor = module()
            identity = successor.bind_verified_replay_root(repo, source, hashes)
            self.assertEqual(successor.ROOT, repo)
            self.assertEqual(identity['historical_source'], source)
            self.assertEqual(identity['executable_sha256'], self.m.sha(HELPER.read_bytes()))
            self.assertEqual(identity['historical_carriers'], tuple(sorted(hashes.items())))
            with self.assertRaises(TypeError): identity['historical_source'] = 'changed'
            with self.assertRaises(successor.Rejected): successor.bind_verified_replay_root(repo, source, hashes)
            self.assertEqual(original.ROOT, original_root)
            self.assertEqual(vars(original), original_globals)
            used = module()
            trace = repo/'empty-trace'
            trace.write_bytes(b'')
            try: used.validate_trace(repo, trace, used.sha(b''))
            except used.Rejected: pass
            with self.assertRaises(used.Rejected): used.bind_verified_replay_root(repo, source, hashes)


class ProducerTrace(unittest.TestCase):
    def current_author_J_fixture(self, native_only=False):
        # Complete private historical DAG plus genuine old/current native
        # qualification. No production tuple, authority or raw row is invented.
        from f13_native_p3_source_test import NativeSourceTest, native
        import copy
        m,folder,names=self.exact_M_fixture()
        seed=NativeSourceTest();seed.setUp();self.addCleanup(seed.doCleanups)
        f=NativeSourceTest();f.repo=self.repo;f.base=self.base
        self.write(RUNTIME,'old dormant fixture\n');f.head=self.commit('old native snapshot')
        f.record_data=copy.deepcopy(seed.record_data)
        f.record_data.update(base=f.base,source=f.head,projection=[dict(path=RUNTIME,
            before=f.entry(f.base,RUNTIME),after=f.entry(f.head,RUNTIME))],
            context=[dict(path=PROTO,entry=f.entry(f.head,PROTO))])
        f.pin();old_carrier=f.carrier;old_snapshot=f.head
        old_target=git(self.repo,'commit-tree',old_carrier+'^{tree}','-p',f.base,'-p',old_carrier,'-m','old private master')
        old=f.qualify(target=old_target,event='pull_request',ref='refs/pull/875/merge',base_ref='master')
        self.write('docs/private-current-master.md','current event parent\n');event_base=self.commit('current event base')
        extra='src/backend/voice/pb/voice/calls/go.mod'
        self.write(extra,'module privatefixture\n');self.write(RUNTIME,'current dormant fixture\n')
        f.head=self.commit('current native snapshot')
        f.record_data.update(source=f.head,projection=[dict(path=n,before=f.entry(f.base,n),after=f.entry(f.head,n)) for n in (RUNTIME,extra)])
        f.pin()
        text=Path(native.__file__).read_text()
        for key,value in f.pins.items():text=re.sub(r'(?m)^'+key+r' = .*$',key+' = '+repr(value),text)
        self.write(native.VERIFIER_PATH,text)
        for n in ('f13_f54_history.py','voice-r22-runtime-scope.sh'):self.write('scripts/ci/'+n,(ROOT/'scripts/ci'/n).read_text())
        reference=self.commit('fixed current AUTHOR reference')
        target=git(self.repo,'commit-tree',reference+'^{tree}','-p',event_base,'-p',reference,'-m','current private master')
        git(self.repo,'checkout','-q',target)
        context=dict(GITHUB_EVENT_NAME='pull_request',GITHUB_REF='refs/pull/875/merge',GITHUB_SHA=target,
            VOICE_CI_PUSH_BEFORE_SHA=event_base,GITHUB_BASE_REF='master')
        spec=importlib.util.spec_from_file_location('private_current_J_native',self.repo/native.VERIFIER_PATH)
        authority=importlib.util.module_from_spec(spec);spec.loader.exec_module(authority)
        binding=authority.qualify(self.repo,event_base,reference,target,'pull_request',context['GITHUB_REF'],target,event_base,'master')
        bound=self.folder/'current-J-native.json';bound.write_bytes(m.canonical(binding))
        m.AUTHOR_REFERENCE=reference;m.AUTHOR_REFERENCE_TREE=git(self.repo,'rev-parse',reference+'^{tree}')
        m.AUTHOR_REFERENCE_DELTA_SHA=m.sha(m.git(self.repo,'diff','--raw','--no-abbrev','--no-renames',m.PROTOTYPE_BASE,reference))
        m.AUTHOR_EVENT_BASE=event_base;m.AUTHOR_NATIVE_CONTENT_BASE=f.base
        m.AUTHOR_HISTORICAL_SCOPE=dict(base=m.O,target=m.M,allow_direct_followup='true',partition_source_history='false')
        m.J3071,m.J3071_P1,m.J3071_P2=m.M,m.P1,m.P2
        m.J3071_O,m.J3071_BASES,m.J3071_TREE=m.O,m.BASES,m.TREE
        # Isolate the second mismatch without skipping a guard: in that private
        # control the genuine current carrier has no post-prototype tree delta.
        m.J3071_PROTOTYPE_BASE=reference if native_only else old_carrier
        rule=b'Private complete J contract; no production authority'
        (folder/'rudeus410-3071-boundary-decision.md').write_bytes(rule);m.J3071_RULING_SHA=m.sha(rule)
        nodes=git(self.repo,'rev-list','--topo-order','--reverse',m.M,'^'+m.O).splitlines()
        sources=git(self.repo,'rev-list','--topo-order','--reverse',m.P2,'^'+m.O).splitlines()
        edges=[];touched=set()
        for node in nodes:
            parents=git(self.repo,'rev-list','--parents','-n','1',node).split()[1:];sides=[]
            for parent in parents:
                paths=git(self.repo,'diff','--name-only','--no-renames',parent,node).splitlines();touched.update(paths)
                sides.append(dict(parent=parent,paths=paths,raw_diff=m.raw_binding(m.git(self.repo,'diff','--raw','--no-abbrev','--no-renames','-z',parent,node)),
                    entries={n:dict(before=m.tree_entry(self.repo,parent,n),after=m.tree_entry(self.repo,node,n)) for n in paths}))
            edges.append(dict(node=node,ordered_parents=parents,parent_edges=sides,source_side=node in sources))
        refs=dict(O=m.O,bestA=m.BASES[0],bestB=m.BASES[1],P1=m.P1,P2=m.P2,J=m.M,H53=m.M,H4=old_snapshot)
        sets={n:git(self.repo,'diff','--name-only',a,b).splitlines() for n,a,b in
            [('candidate_difference',*m.BASES),('history_to_source',m.O,m.P2),('first_to_result',m.P1,m.M),('source_to_result',m.P2,m.M),('older_to_result',m.O,m.M)]}
        packet=dict(version='F54-FIXED3071-STRUCTURAL-FACTS-v1',admission=False,merge=m.M,parents=[m.P1,m.P2],best_bases=m.BASES,
            older_history_boundary=m.O,committed_tree=m.TREE,ruling_sha256=m.J3071_RULING_SHA,nodes=edges,source_nodes=sources,
            actual_source_first_parent_operations=git(self.repo,'rev-list','--first-parent','--reverse',m.O+'..'+m.P2).splitlines(),
            all_internal_touched_paths=sorted(touched),all_internal_versions={n:{k:m.tree_entry(self.repo,v,n) for k,v in refs.items()} for n in sorted(touched)},
            original_caller=dict(m.AUTHOR_HISTORICAL_SCOPE),original_native_binding=m.raw_binding(m.canonical(old)),
            source_qualification=dict(status='UNRESOLVED'),actual_history='UNOBSERVED',actual_rows='UNOBSERVED',Git_only_path_sets=sets)
        raw=m.canonical(packet);(folder/'structural-facts-v1.json').write_bytes(raw);m.J3071_PACKET_SHA=m.sha(raw)
        self.assertEqual(m.validate_3071_packet(self.repo,folder),packet)
        return m,folder,bound,context,old,binding

    def test_current_author_J_binds_current_carrier_without_changing_historical_packet(self):
        m,folder,bound,context,old,binding=self.current_author_J_fixture()
        prepare=getattr(m,'prepare_3071_author_capabilities',m.prepare_3071_capabilities)
        with patch.dict('os.environ',context):cap=prepare(self.repo,folder,bound)
        self.assertEqual(cap['type'],'F54_FIXED3071_CURRENT_AUTHOR_CAPABILITIES')
        self.assertEqual(cap['historical_native_binding'],m.raw_binding(m.canonical(old)))
        self.assertEqual(cap['native_binding'],m.raw_binding(m.canonical(binding)))
        self.assertEqual(cap['author_applicability']['reference'],m.AUTHOR_REFERENCE)
        self.assertFalse(cap['admission']);self.assertEqual(cap['history'],'UNOBSERVED')
        # The old constructor is still a refusal, not an implicit new route.
        with patch.dict('os.environ',context),self.assertRaisesRegex(m.Rejected,'fixed3071_current_carrier_only_four_owned_paths'):
            m.prepare_3071_capabilities(self.repo,folder,bound)
        import copy
        path=self.folder/'private-J-current-certificate.json';path.write_bytes(m.canonical(cap))
        with patch.dict('os.environ',context):self.assertEqual(m.verified_3071_capabilities(self.repo,path),cap)
        for error in ('reference','record','snapshot','classification','path','mode','oid','frame','history','runtime','type'):
            forged=copy.deepcopy(cap)
            if error=='reference':forged['author_applicability']['reference']=m.J3071
            elif error=='record':forged['native_applicability']['current']['record']=old['record']
            elif error=='snapshot':forged['native_applicability']['current']['snapshot']=old['snapshot']
            elif error=='classification':forged['native_applicability']['current']['classification']='ACTIVE'
            elif error=='path':forged['native_applicability']['path_entries'].pop()
            elif error=='mode':forged['native_applicability']['path_entries'][0]['current_entry']['mode']='100755'
            elif error=='oid':forged['native_applicability']['path_entries'][0]['current_entry']['oid']='0'*40
            elif error=='frame':forged['caller']['base']=binding['base']
            elif error=='history':
                altered=dict(old);altered['content_base']=old['snapshot'];forged['historical_native_binding']=m.raw_binding(m.canonical(altered))
            elif error=='runtime':forged['native_applicability']['runtime_pass']=True
            else:forged['type']='F54_FIXED3071_STRUCTURAL_CAPABILITIES_PROTOTYPE'
            path.write_bytes(m.canonical(forged))
            with self.subTest(certificate=error),patch.dict('os.environ',context),self.assertRaises(m.Rejected):
                m.verified_3071_capabilities(self.repo,path)

    def test_current_author_J_keeps_old_native_facts_and_qualifies_current_authority(self):
        m,folder,bound,context,old,binding=self.current_author_J_fixture(native_only=True)
        prepare=getattr(m,'prepare_3071_author_capabilities',m.prepare_3071_capabilities)
        with patch.dict('os.environ',context):cap=prepare(self.repo,folder,bound)
        self.assertNotEqual(old['record'],binding['record']);self.assertNotEqual(old['base'],binding['base'])
        self.assertEqual(cap['native_applicability']['historical']['record'],old['record'])
        self.assertEqual(cap['native_applicability']['current']['record'],binding['record'])
        self.assertEqual(cap['native_applicability']['added_paths'],sorted(set(binding['paths'])-set(old['paths'])))
        with patch.dict('os.environ',context),self.assertRaisesRegex(m.Rejected,'fixed3071_no_borrowed_new_native_authority'):
            m.prepare_3071_capabilities(self.repo,folder,bound)
        import copy
        for error in ('base','content_base','snapshot','record','paths','classification','source'):
            forged=copy.deepcopy(binding)
            if error=='paths':forged['paths']=old['paths']
            elif error=='classification':forged[error]='ACTIVE'
            elif error=='source':forged[error]=old['source']
            else:forged[error]=old[error] if old[error]!=binding[error] else old['snapshot']
            bound.write_bytes(m.canonical(forged))
            with self.subTest(native=error),patch.dict('os.environ',context),self.assertRaisesRegex(m.Rejected,'fixed3071_(native_|actual_native_context)'):
                prepare(self.repo,folder,bound)
        bound.write_bytes(m.canonical(binding))
        packet_path=folder/'structural-facts-v1.json';packet=json.loads(packet_path.read_bytes())
        for error in ('hash','snapshot','paths'):
            forged=copy.deepcopy(packet);historic=copy.deepcopy(old)
            if error=='hash':historic['record_sha256']='0'*64
            elif error=='snapshot':historic['snapshot']=binding['snapshot']
            else:historic['paths']=[]
            forged['original_native_binding']=m.raw_binding(m.canonical(historic))
            raw=m.canonical(forged);packet_path.write_bytes(raw);m.J3071_PACKET_SHA=m.sha(raw)
            expected='fixed3071_all_versioned_path_entries' if error=='snapshot' else 'current_author_J_'
            with self.subTest(historical=error),patch.dict('os.environ',context),self.assertRaisesRegex(m.Rejected,expected):
                prepare(self.repo,folder,bound)

    def test_current_author_capability_keeps_fixed_reference_and_historical_facts(self):
        # Complete private source unit; no production record/permission is made.
        m,folder,names=self.exact_M_fixture();historical=m.M
        git(self.repo,'checkout','-qb','private-current-master',m.O)
        self.write('docs/private-master-context.md','actual event parent\n')
        event_base=self.commit('private current event base')
        git(self.repo,'checkout','-q',historical)
        changed=next(n for n in names if n.startswith('protos/'))
        self.write(changed,'separately fixed current carrier product input\n')
        self.commit('private accepted integration input')
        git(self.repo,'merge','--no-ff','-qm','current integration',event_base)
        reference=git(self.repo,'rev-parse','HEAD')
        m.AUTHOR_REFERENCE=reference;m.AUTHOR_EVENT_BASE=event_base;m.AUTHOR_NATIVE_CONTENT_BASE=self.base
        m.AUTHOR_REFERENCE_TREE=git(self.repo,'rev-parse',reference+'^{tree}')
        m.AUTHOR_REFERENCE_DELTA_SHA=m.sha(m.git(self.repo,'diff','--raw','--no-abbrev','--no-renames',m.PROTOTYPE_BASE,reference))
        m.AUTHOR_HISTORICAL_SCOPE=dict(base=m.O,target=historical,allow_direct_followup='true',partition_source_history='false')
        helper='scripts/ci/f13_f54_history.py'
        # Re-pin the complete private factual unit in the executable too, so
        # the real CLI/shell seams below read the same immutable authority.
        source=(self.repo/helper).read_text();lines=source.splitlines(keepends=True)
        keys=('M','P1','P2','O','BASES','TREE','SOURCE','PROTOTYPE_BASE','UNIT_HASHES',
            'AUTHOR_REFERENCE','AUTHOR_EVENT_BASE','AUTHOR_NATIVE_CONTENT_BASE','AUTHOR_REFERENCE_TREE',
            'AUTHOR_REFERENCE_DELTA_SHA','AUTHOR_HISTORICAL_SCOPE')
        assignments=[n for n in ast.parse(source).body if isinstance(n,ast.Assign) and len(n.targets)==1
            and isinstance(n.targets[0],ast.Name) and n.targets[0].id in keys]
        for n in sorted(assignments,key=lambda n:n.lineno,reverse=True):
            key=n.targets[0].id;lines[n.lineno-1:n.end_lineno]=[key+' = '+repr(getattr(m,key))+'\n']
        self.write(helper,''.join(lines)+'\n# reviewed private AUTHOR successor\n')
        successor=self.commit('only author successor code')
        # The fallback exercises the unchanged D6 guard for the semantic RED;
        # a missing API alone cannot be the regression failure.
        prepare=getattr(m,'prepare_author_capabilities',m.prepare_capabilities)
        cap=prepare(self.repo,folder)
        self.assertEqual(cap['type'],'F54_FIXED_CURRENT_AUTHOR_CAPABILITIES')
        self.assertFalse(cap['admission']);self.assertFalse(m.PRODUCTION_AUTHORITY)
        self.assertEqual(cap['source_facts']['source_context'],historical)
        self.assertEqual(cap['collector_scope'],m.AUTHOR_HISTORICAL_SCOPE)
        certificate=cap['author_applicability']
        self.assertEqual(certificate['reference'],reference)
        self.assertEqual(certificate['current_carrier'],successor)
        self.assertEqual(certificate['native_content_baseline'],self.base)
        self.assertEqual(certificate['event_base'],event_base)
        self.assertEqual(certificate['own_delta_paths'],[helper])
        self.assertIn(changed,[x['path'] for x in certificate['reference_delta']])
        self.assertEqual(cap['complete_41_entries'],__import__('json').loads((folder/'proposal.json').read_bytes())['entries'])
        path=self.folder/'private-current-author-cap.json';path.write_bytes(m.canonical(cap))
        cli=subprocess.run(['python3',str(self.repo/helper),'current-author-capabilities',str(self.repo),str(folder)],capture_output=True,text=True)
        self.assertEqual(cli.returncode,0,cli.stderr);self.assertEqual(json.loads(cli.stdout),cap)
        out=self.folder/'current-author-two-seams';out.mkdir();boundary_path=out/'boundary'
        result=git(self.repo,'ls-tree',m.M,'--',changed)
        setup='set -eu\n'+definitions()+'\nTMP_DIR='+shlex.quote(str(out))+'\n'
        def seam_program(scope):
            resolver='f13_resolve_source_base '+' '.join(map(shlex.quote,map(str,(self.repo,m.P1,m.P2,boundary_path,m.M,path,self.repo/helper,'',*scope))))
            composition='f13_checkpoint_merge_tree_entry_matches '+' '.join(map(shlex.quote,map(str,(self.repo,m.O,m.P1,m.P2,changed,result,m.M,path,self.repo/helper,'','',*scope))))
            return setup+resolver+'\n'+composition+'\n'
        actual_seams=subprocess.run(['bash','-c',seam_program((m.O,historical))],capture_output=True,text=True)
        self.assertEqual(actual_seams.returncode,0,actual_seams.stderr)
        self.assertEqual(boundary_path.read_text().strip(),m.O)
        foreign_seams=subprocess.run(['bash','-c',seam_program((event_base,historical))],capture_output=True,text=True)
        self.assertEqual(foreign_seams.returncode,1,foreign_seams.stderr)
        boundary=m.boundary_capability(self.repo,path,m.M,m.P1,m.P2,m.O,historical)
        self.assertEqual(boundary['history_boundary'],m.O)
        for wrong in ((m.O,reference),(reference,historical),('0'*40,historical)):
            with self.subTest(frame=wrong),self.assertRaises(m.Rejected):
                m.boundary_capability(self.repo,path,m.M,m.P1,m.P2,*wrong)
        import copy
        for error in ('event-base','native-base','reference','reference-entry','own-path','historical-base','owner','OR'):
            forged=copy.deepcopy(cap)
            if error=='event-base':forged['author_applicability']['event_base']=m.O
            elif error=='native-base':forged['author_applicability']['native_content_baseline']=event_base
            elif error=='reference':forged['author_applicability']['reference']=historical
            elif error=='reference-entry':forged['author_applicability']['reference_delta'][0]['current_entry']=''
            elif error=='own-path':forged['author_applicability']['own_delta_paths'].append('foreign.go')
            elif error=='historical-base':forged['collector_scope']['base']=event_base
            elif error=='owner':forged['complete_dag'][0]['ordered_parents']=[]
            else:forged['author_applicability']['admission']=True
            path.write_bytes(m.canonical(forged))
            with self.subTest(error=error),self.assertRaises(m.Rejected):m.verified_capabilities(self.repo,path)
        path.write_bytes(m.canonical(cap))
        self.write('docs/unauthorized-after-reference.md','outside own four files\n')
        self.commit('unauthorized current carrier drift')
        with self.assertRaises(m.Rejected):prepare(self.repo,folder)

    def test_current_author_master_keeps_event_history_and_native_bases_distinct(self):
        # Real native qualification; only the historical capability transport is
        # a private constructor fixture. It is not complete M/history evidence.
        from f13_native_p3_source_test import NativeSourceTest, native
        fixture=NativeSourceTest();fixture.setUp();self.addCleanup(fixture.doCleanups)
        content_base=fixture.base;historical_base=fixture.head;historical_target=fixture.carrier
        fixture.git('checkout','-qb','private-event-base',historical_base)
        fixture.write('docs/private-event-base.md','actual master base\n');fixture.commit('actual event base')
        event_base=fixture.git('rev-parse','HEAD')
        fixture.git('checkout','-q',historical_target);fixture.git('merge','--no-ff','-qm','current integration',event_base)
        source=Path(native.__file__).read_text()
        for key,value in fixture.pins.items():source=re.sub(r'(?m)^'+key+r' = .*$',key+' = '+repr(value),source)
        fixture.write(native.VERIFIER_PATH,source)
        for n in ('f13_f54_history.py','voice-r22-runtime-scope.sh'):fixture.write('scripts/ci/'+n,(ROOT/'scripts/ci'/n).read_text())
        fixture.commit('private exact current integration reference');reference=fixture.git('rev-parse','HEAD')
        target=fixture.git('commit-tree',reference+'^{tree}','-p',event_base,'-p',reference,'-m','actual private master target')
        fixture.git('checkout','-q',target)
        spec=importlib.util.spec_from_file_location('private_current_author_native',fixture.repo/native.VERIFIER_PATH)
        authority=importlib.util.module_from_spec(spec);spec.loader.exec_module(authority)
        event=dict(GITHUB_EVENT_NAME='pull_request',GITHUB_REF='refs/pull/875/merge',GITHUB_SHA=target,
            VOICE_CI_PUSH_BEFORE_SHA=event_base,GITHUB_BASE_REF='master')
        binding=authority.qualify(fixture.repo,event_base,reference,target,'pull_request',event['GITHUB_REF'],target,event_base,'master')
        m=module();m.ROOT=fixture.repo;m.PROTOTYPE_BASE=historical_base
        m.AUTHOR_REFERENCE=reference;m.AUTHOR_EVENT_BASE=event_base;m.AUTHOR_NATIVE_CONTENT_BASE=content_base
        m.AUTHOR_REFERENCE_TREE=fixture.git('rev-parse',reference+'^{tree}')
        m.AUTHOR_REFERENCE_DELTA_SHA=m.sha(m.git(fixture.repo,'diff','--raw','--no-abbrev','--no-renames',historical_base,reference))
        m.AUTHOR_HISTORICAL_SCOPE=dict(base=historical_base,target=historical_target,allow_direct_followup='true',partition_source_history='false')
        certificate=m.current_author_applicability(fixture.repo) if hasattr(m,'current_author_applicability') else {}
        cap=dict(type='F54_FIXED_CURRENT_AUTHOR_CAPABILITIES',collector_scope=m.AUTHOR_HISTORICAL_SCOPE,
            author_applicability=certificate,prepared_at_target=target,admission=False)
        result=m.master_context(fixture.repo,cap,m.canonical(binding),event)
        self.assertEqual(result['base'],event_base);self.assertNotEqual(result['base'],historical_base)
        self.assertEqual(result['author_base_roles']['native_content_baseline'],content_base)
        self.assertEqual(result['author_base_roles']['historical_collector'],m.AUTHOR_HISTORICAL_SCOPE)
        self.assertFalse(result['admission'])
        import copy
        for error in ('history','native','event','source','carrier','map-context'):
            wrong=copy.deepcopy(cap);bad=copy.deepcopy(binding)
            if error=='history':wrong['collector_scope']['base']=event_base
            elif error=='native':wrong['author_applicability']['native_content_baseline']=historical_base
            elif error=='event':wrong['author_applicability']['event_base']=historical_base
            elif error=='source':bad['source']=historical_target
            elif error=='carrier':wrong['author_applicability']['current_carrier']=reference
            else:wrong['author_applicability']['reference_tree']='0'*40
            with self.subTest(error=error),self.assertRaises((m.Rejected,authority.Rejected)):
                m.master_context(fixture.repo,wrong,m.canonical(bad),event)

    def test_J_consumer_accepts_only_independently_qualified_native_normalization(self):
        # Private Git authority and an actual original shell predicate call.
        # The transport below is a fixture, never appended to production trace.
        import copy
        from f13_native_p3_source_test import NativeSourceTest, native
        fixture=NativeSourceTest();fixture.setUp();self.addCleanup(fixture.doCleanups)
        old=fixture.head
        fixture.git('checkout','-q',old);fixture.write('private-first-parent.md','first parent\n')
        fixture.commit('private first parent');first=fixture.git('rev-parse','HEAD')
        integration=fixture.git('commit-tree',first+'^{tree}','-p',first,'-p',old,'-m','private J integration')
        fixture.git('checkout','-q',integration)
        fixture.head=integration;fixture.record_data['source']=integration;fixture.pin()
        # Independent qualifier reads these committed private constants itself.
        text=Path(native.__file__).read_text()
        for key,value in fixture.pins.items():
            text=re.sub(r'(?m)^'+key+r' = .*$',key+' = '+repr(value),text)
        fixture.write(native.VERIFIER_PATH,text)
        fixture.write('scripts/ci/voice-r22-runtime-scope.sh',(ROOT/'scripts/ci/voice-r22-runtime-scope.sh').read_text())
        fixture.commit('private pinned authority carrier');fixture.carrier=fixture.git('rev-parse','HEAD')
        spec=importlib.util.spec_from_file_location('private_J_native',fixture.repo/native.VERIFIER_PATH)
        authority=importlib.util.module_from_spec(spec);spec.loader.exec_module(authority)
        target=fixture.git('commit-tree',fixture.carrier+'^{tree}','-p',fixture.base,'-p',fixture.carrier,'-m','private master T')
        fixture.git('checkout','-q',target)
        context=dict(GITHUB_EVENT_NAME='pull_request',GITHUB_REF='refs/pull/875/merge',
            GITHUB_SHA=target,VOICE_CI_PUSH_BEFORE_SHA=fixture.base,GITHUB_BASE_REF='master')
        binding=authority.qualify(fixture.repo,fixture.base,fixture.carrier,target,
            context['GITHUB_EVENT_NAME'],context['GITHUB_REF'],target,fixture.base,'master')
        m=module();m.ROOT=fixture.repo;m.J3071=integration;m.J3071_P1=first;m.J3071_P2=old;m.J3071_O=fixture.base
        name=RUNTIME;frame='private-J';op=frame+'/'+integration+'/2'
        prior='\t'.join([name,'true',old,fixture.base,fixture.base,old,fixture.base,fixture.base])
        incoming='\t'.join([name,'false',integration,first,fixture.base,old,fixture.base,fixture.base])
        after=incoming.split('\t');after[4:6]=[first,integration];after='\t'.join(after)
        other='\t'.join([PROTO,'false',old,fixture.base,fixture.base,old,fixture.base,fixture.base])
        actual=(prior+'\n'+other+'\n').encode();empty=m.file_digest(b'')
        operation=dict(commit=integration,first_parent=first,source_head=old,boundary=fixture.base,
            candidates=[fixture.base],paths=[name],classification='merge')
        state=dict(frame=frame,operation=op,operation_binding=operation,classification=dict(trigger='false'),
            capability_sha256='private-Git-fixture',frame_ancestry=[],coverage=[],M_ready={},I_ready={},composition={},
            ledger={frame:dict(map_binding=m.file_digest(actual),rows=[prior,other],paths={name:dict(aggregate=True,evidence=[m.raw_binding(prior)]),
                PROTO:dict(aggregate=False,evidence=[m.raw_binding(other)])})},
            prior_span=dict(ledger=dict(paths={},rows=[],map_binding=empty),trace_sha256=m.sha(b''),actual_operations=[]))
        before=dict(kind='row_before',frame=frame,operation=op,sequence=2,
            payload=dict(path=name,map_before=m.file_digest(actual),prior_rows=[m.raw_binding(prior)],incoming=m.raw_binding(incoming)))
        paths=self.folder/'private-native-paths';paths.write_text('\n'.join(binding['paths'])+'\n')
        bound=self.folder/'private-native-binding';bound.write_bytes(m.canonical(binding))
        functions=definitions()+'\n'+'\n'.join(x.group(0) for x in re.finditer(
            r'(?ms)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{\n.*?^\}\n',SHELL.read_text())
            if x.group(1) in ('native_p3_path_bound','native_p3_duplicate_owner_qualified'))
        values=dict(ROOT=str(fixture.repo),native_p3_active='true',native_p3_snapshot=integration,
            native_p3_paths=str(paths),native_p3_binding=str(bound),native_p3_paths_hash=fixture.git('hash-object',str(paths)),
            native_p3_binding_hash=fixture.git('hash-object',str(bound)))
        program=functions+'\n'+'\n'.join(k+'='+shlex.quote(v) for k,v in values.items())+'\n'
        program+='native_p3_duplicate_owner_qualified '+ ' '.join(map(shlex.quote,[str(fixture.repo),name,old,old,integration,old,integration,'clean']))+'\n'
        status=subprocess.run(['bash','-c',program],capture_output=True)
        self.assertEqual(status.returncode,0,status.stderr)
        gate=dict(kind='native_rebind_gate',frame=frame,operation=op,sequence=3,payload=dict(path=name,
            gate='native_p3_duplicate_owner_qualified',status=status.returncode,outcome='clean',prior_owner=old,prior_source=old,
            current_owner=integration,current_source=old,integration=integration,binding=m.raw_binding(bound.read_bytes()),
            classifier=m.classifier_binding(),context=context))
        # Only master transport is constructed; all native authority/Git/gate
        # checks below run genuinely under this explicitly private ROOT.
        def encode(event,sequence,previous):
            event=copy.deepcopy(event);event.update(schema=m.SCHEMA,sequence=sequence,previous=previous)
            event.pop('digest',None);event['digest']=m.sha(m.canonical(event))
            return m.canonical(event)+b'\n',event['digest']
        prefix,first_digest=encode(dict(kind='master_entry',frame=None,operation=None,
            payload=dict(native_binding=m.raw_binding(bound.read_bytes()),context=context)),1,'0'*64)
        ready=m.build_3071_J_ready(state,m.file_digest(prefix))
        ledger=self.folder/'private-J-native-ledger';ledger.write_bytes(m.canonical(ready))
        trace=self.folder/'private-J-native-trace'
        def consume(event=gate,final=after,current=actual,value=None,pending=before,
                    genesis='0'*64,malformed_genesis=None):
            current_prefix,root_digest=encode(dict(kind='master_entry',frame=None,operation=None,
                payload=dict(native_binding=m.raw_binding(bound.read_bytes()),context=context)),1,genesis)
            if malformed_genesis:
                first=json.loads(current_prefix)
                if malformed_genesis=='missing':
                    first.pop('previous')
                    body=dict(first);body.pop('digest',None);first['digest']=m.sha(m.canonical(body))
                elif malformed_genesis=='tampered':first['digest']='f'*64
                root_digest=first['digest']
                current_prefix=m.canonical(first)+b'\n'
            if value is None:value=m.build_3071_J_ready(state,m.file_digest(current_prefix))
            self.assertEqual(value['prefix'],m.file_digest(current_prefix))
            ledger.write_bytes(m.canonical(value))
            pending_raw,digest=encode(pending,2,root_digest)
            gate_raw=encode(event,3,digest)[0] if event else b''
            trace.write_bytes(current_prefix+pending_raw+gate_raw)
            downstream=[json.loads(item) for item in trace.read_bytes().splitlines()]
            for index,item in enumerate(downstream[1:],1):
                body=dict(item);item_digest=body.pop('digest')
                self.assertEqual(item['previous'],downstream[index-1]['digest'])
                self.assertEqual(m.sha(m.canonical(body)),item_digest)
            return m.consume_3071_J_path(ledger,m.sha(ledger.read_bytes()),trace,name,current,final,'replace')
        result=consume()
        self.assertTrue(result['aggregate']);self.assertEqual(result['prior_rows'],[m.raw_binding(prior)])
        self.assertEqual(result['incoming'],m.raw_binding(incoming));self.assertEqual(result['after_gate_row'],m.raw_binding(after))
        self.assertEqual(result['expected_post_map'],m.file_digest((other+'\n'+after+'\n').encode()))
        for label,genesis,malformed in (('null',None,None),('nonzero','f'*64,None),
                                        ('missing',None,'missing'),('tampered-chain','0'*64,'tampered')):
            with self.subTest(genesis=label),self.assertRaisesRegex(m.Rejected,'fixed3071_J_native_append_only_trace'):
                consume(genesis=genesis,malformed_genesis=malformed)
        for error in ('missing','failed','status2','foreign-frame','foreign-operation','foreign-path','binding-rehashed',
                      'foreign-prior','foreign-source','foreign-owner','content','map','map-rehashed','map-reorder-rehashed',
                      'after-source','after-base','after-owner','after-parent','after-candidates','context','classifier','drop-prior-true'):
            event=copy.deepcopy(gate);final=after;current=actual;value=copy.deepcopy(ready);pending=copy.deepcopy(before)
            if error=='missing':event=None
            elif error in ('failed','status2'):event['payload']['status']=1 if error=='failed' else 2
            elif error=='foreign-frame':event['frame']='foreign'
            elif error=='foreign-operation':event['operation']='foreign/'+integration+'/2'
            elif error=='foreign-path':event['payload']['path']=PROTO
            elif error=='binding-rehashed':
                forged=copy.deepcopy(binding);forged['paths']=[];event['payload']['binding']=m.raw_binding(m.canonical(forged))
            elif error=='foreign-prior':event['payload']['prior_owner']=fixture.base
            elif error=='foreign-source':event['payload']['current_source']=fixture.base
            elif error=='foreign-owner':event['payload']['current_owner']=old
            elif error=='content':
                parts=final.split('\t');parts[6]=old;final='\t'.join(parts)
            elif error=='map':current=actual+actual
            elif error in ('map-rehashed','map-reorder-rehashed'):
                current=(prior+'\n').encode() if error=='map-rehashed' else (other+'\n'+prior+'\n').encode()
                pending['payload']['map_before']=m.file_digest(current)
            elif error.startswith('after-'):
                index={'after-source':5,'after-base':4,'after-owner':2,'after-parent':3,'after-candidates':7}[error]
                parts=final.split('\t');parts[index]=old if error=='after-candidates' else fixture.base;final='\t'.join(parts)
            elif error=='context':event['payload']['context']['GITHUB_REF']='refs/pull/876/merge'
            elif error=='classifier':event['payload']['classifier']['sha256']='0'*64
            else:value['outer_rows']=[]
            with self.subTest(error=error),self.assertRaises((m.Rejected,authority.Rejected)):
                consume(event,final,current,value,pending)

    def test_manual_result_full33_spec_preserves_real_parent_rows_and_disjoint_edges(self):
        # Explicit value-constructor fixture, not an executed producer ledger.
        # Immutable Git establishes the full residual and content identities.
        import copy
        m=module();new=m.F_MANUAL_NEW_PATHS;retired=m.F_DELETION_PATH
        covered=['docs/private-manual-covered-'+str(i).zfill(2)+'.md' for i in range(22)]
        names=sorted(new+covered+[retired])
        for name in names:self.write(name,'base\n'+'stable\n'*20+'base end\n')
        base=self.commit('private residual C')
        for name in covered:self.write(name,'first\n'+'stable\n'*20+'base end\n')
        first=self.commit('private residual D')
        git(self.repo,'checkout','-qb','private-residual-A',base)
        for name in covered:self.write(name,'base\n'+'stable\n'*20+'source end\n')
        source=self.commit('private residual A')
        auto=git(self.repo,'merge-tree','--write-tree',first,source).splitlines()[0]
        git(self.repo,'checkout','-q',first)
        merge=subprocess.run(['git','-C',str(self.repo),'merge','--no-ff','--no-commit',source],capture_output=True)
        self.assertEqual(merge.returncode,0)
        for name in names:
            if name==retired:(self.repo/name).unlink()
            else:self.write(name,(self.repo/name).read_text()+'authored manual result\n')
        result=self.commit('private entire authored residual')
        self.assertEqual(git(self.repo,'diff','--name-only',auto,result).splitlines(),names)
        m.F_CORRECTED_TUPLE=dict(merge=result,first=first,source=source,base=base,automatic_tree=auto,correction=result,result_tree=git(self.repo,'rev-parse',result+'^{tree}'))
        m.J3071_RANGE=dict(m.J3071_RANGE,base=base)
        rows=['\t'.join([n,'true' if n==retired else 'false',first,base,base,first,base,base]) for n in covered+[retired]]
        m.F_DELETION_ROW_SHA=m.sha(rows[-1].encode());actual=('\n'.join(rows)+'\n').encode()
        cap=dict(unit_path='private-value-fixture',whole=dict(complete_manual_paths=names),source_context=dict(private_fixture=True),context={})
        values=dict(re.findall(r"(?m)^(accepted_[a-z0-9_]+|game_checkpoint_snapshot_[a-z_]+)='([0-9a-f]{40})'$",SHELL.read_text()),
            ROOT=str(self.repo.resolve()),TMP_DIR=str(self.folder),game_checkpoint_enabled='false',game_checkpoint_aggregate_active='false',
            game_checkpoint_aggregate_paths='',game_checkpoint_aggregate_base='',game_checkpoint_aggregate_head='',native_p3_active='false')
        context=dict(source_context=cap['source_context'],values=values,event={},files={},snapshots={})
        # Isolate the authority-derivation boundary from the real-Git residual
        # constructor fixture. Production must derive this expectation itself;
        # a supplied snapshot/hash is not the independent expectation.
        native_paths=str(self.folder/'native-paths');game_tree=str(self.folder/'game-checkpoint-tree')
        paths_raw=('\n'.join(new)+'\n').encode();tree_raw=m.git(self.repo,'ls-tree','-r',first)
        def blob_hash(raw):return hashlib.sha1(b'blob '+str(len(raw)).encode()+b'\0'+raw).hexdigest()
        context['values'].update(native_p3_paths=native_paths,native_p3_paths_hash=blob_hash(paths_raw))
        for name,raw in ((native_paths,paths_raw),(game_tree,tree_raw)):
            context['files'][name]=m.file_digest(raw);context['snapshots'][name]=m.raw_binding(raw)
        authority=dict(values=copy.deepcopy(context['values']),files=copy.deepcopy(context['snapshots']))
        frame=dict(id='private-same-state',base=base,target=result,allow_direct_followup='true',partition_source_history='false')
        with patch.object(m,'prepare_corrected_F_capability',return_value=cap),\
                patch.object(m,'manual_authority_expected',return_value=authority,create=True):
            spec=m.manual_F_spec(self.repo,cap,frame,actual,context)
            self.assertEqual(spec['paths'],names);self.assertEqual(spec['result_parent_edge']['base'],first)
            self.assertEqual(spec['original_merge_bases'],[base]);self.assertNotEqual(first,base)
            for name in new:self.assertEqual(spec['dispositions'][name]['type'],'NEW_RESULT_PARENT_ROW')
            self.assertEqual(spec['dispositions'][retired]['prior_rows'],[m.raw_binding(rows[-1])])
            for forged in ('native-empty-with-updatedhash','Game-resultblob-with-updatedhash'):
                changed=copy.deepcopy(context)
                name=native_paths if forged.startswith('native') else game_tree
                raw=b'' if forged.startswith('native') else m.git(self.repo,'ls-tree','-r',result)
                changed['snapshots'][name]=m.raw_binding(raw);changed['files'][name]=m.file_digest(raw)
                if forged.startswith('native'):changed['values']['native_p3_paths_hash']=blob_hash(raw)
                with self.subTest(authority_forgery=forged),self.assertRaises(m.Rejected):
                    m.manual_F_spec(self.repo,cap,frame,actual,changed)
            # Re-execute the real original predicates with private Git inputs;
            # no predicate is replaced by a fake accepting implementation.
            for name in (new[0],covered[0]):
                gate=m.replay_manual_F_gate(str(self.repo),m.canonical(spec).decode(),name)
                self.assertEqual(gate,dict(original_composition_status=1,native_owner_status=0,game_owner_status=0,eligibility='unapproved'))
            for bad in ('prior-owner','drop-covered','raw-deletion-false','context','flags','full33'):
                changed=actual;fr=copy.deepcopy(frame);cx=copy.deepcopy(context)
                if bad=='prior-owner':changed+=(new[0]+'\tfalse\t'+ '\t'.join([first]*6)+'\n').encode()
                elif bad=='drop-covered':changed=('\n'.join(rows[1:])+'\n').encode()
                elif bad=='raw-deletion-false':changed=actual.replace(rows[-1].encode(),rows[-1].replace('\ttrue\t','\tfalse\t').encode())
                elif bad=='context':cx['values']['ROOT']='/foreign'
                elif bad=='flags':fr['partition_source_history']='true'
                else:cap['whole']['complete_manual_paths']=names[:-1]
                with self.subTest(bad=bad),self.assertRaises(m.Rejected):m.manual_F_spec(self.repo,cap,fr,changed,cx)
                cap['whole']['complete_manual_paths']=names

    def test_manual_result_sealed_full_delta_and_complete_prefix_are_required(self):
        import copy
        m=module();names=sorted(m.F_MANUAL_NEW_PATHS+['docs/covered-'+str(i) for i in range(23)])
        spec=dict(type='EXACT_F_MANUAL_RESULT_v1',admission=False,paths=names,delta=m.raw_binding('\n'.join(names)+'\n'))
        receipt=dict(spec=spec,classification=dict(trigger='false'),dispositions={n:{} for n in names})
        binding=dict(path=names[-1],manual_result_operations=[dict(frame='F1',operation='F1/F/manual-result',receipt=receipt)])
        self.assertEqual(m.manual_delta_for_binding(binding),spec['delta']['raw'])
        for bad in ('ten-only','omit-disposition','wrong-delta','duplicate-occurrence','missing-operation'):
            value=copy.deepcopy(binding)
            if bad=='ten-only':value['manual_result_operations'][0]['receipt']['spec']['paths']=m.F_MANUAL_NEW_PATHS
            elif bad=='omit-disposition':value['manual_result_operations'][0]['receipt']['dispositions'].pop(names[0])
            elif bad=='wrong-delta':value['manual_result_operations'][0]['receipt']['spec']['delta']=m.raw_binding('parent/import.go\n')
            elif bad=='duplicate-occurrence':value['manual_result_operations']*=2
            else:value['manual_result_operations']=[]
            with self.subTest(bad=bad),self.assertRaises(m.Rejected):m.manual_delta_for_binding(value)
        ledger={'F1':dict(manual_result_operations={'F1/F/manual-result':receipt})}
        trace=(json.dumps(dict(frame='F1',operation='F1/F/manual-result',kind='manual_F_end',payload=dict(receipt=receipt)))+'\n').encode()
        prefix=m.file_digest(trace)
        self.assertEqual(m.manual_operations_in_prefix(ledger,trace,prefix),binding['manual_result_operations'])
        with self.assertRaises(m.Rejected):m.manual_operations_in_prefix({},trace,prefix)
        with self.assertRaises(m.Rejected):m.manual_operations_in_prefix(ledger,b'',m.file_digest(b''))

    def test_manual_result_delta_cannot_use_unsealed_parent_import_context(self):
        # Real private Git: D->F contains an imported path which AUTO->F does
        # not. A typed result row alone must not authorize that broader delta.
        imported='docs/private-parent-import.md'
        self.write(RUNTIME,'base\n');self.write(imported,'base\n');base=self.commit('manual context base')
        self.write(RUNTIME,'manual result\n');self.write(imported,'parent import\n');result=self.commit('private result edge')
        raw='\t'.join([RUNTIME,'false',result,base,base,result,base,base])
        output=self.folder/'typed-map';output.write_text(raw+'\n')
        function=re.search(r'(?ms)^f13_delta_for_path\(\) \{\n.*?^\}\n',SHELL.read_text()).group(0)
        function=function.replace('fa1528a7c232a644218edf49811e2bb2e373105f',result)
        program='set -eu\nTMP_DIR='+shlex.quote(str(self.folder))+'\n'+function+'''
f13_f54_manual_delta_for_path() { return 2; }
f13_retained_history_row() { return 1; }
'''
        p=subprocess.run(['bash','-c',program+'f13_delta_for_path '+shlex.quote(str(self.repo))+' '+
            shlex.quote(str(output))+' '+shlex.quote(RUNTIME)],capture_output=True,text=True)
        self.assertNotEqual(p.returncode,0,'Unsealed typed row incorrectly accepted whole D->F parent-import delta')

    def test_corrected_historical_manual_unit_requires_complete_source_and_current_repair(self):
        # Private real-Git specification at the failing per-entry seam. A
        # corrected target is not a claim that its historical source was safe.
        # No production record, runtime result or manufactured owner row exists.
        retired='src/backend/voice/internal/store/space_lifecycle_test.go'
        names=sorted([RUNTIME,retired]+['src/backend/voice/internal/spacemedia/private_admission'+str(i)+'.go' for i in range(2)]+
                     ['docs/private-f-manual/source'+str(i)+'.md' for i in range(29)])
        # Keep explicit conflict names independent of lexicographic positions.
        conflicts=[n for n in names if 'private_admission' in n]
        service='src/backend/voice/internal/spacelifecycle/service.go'
        effects='src/backend/voice/internal/spacelifecycle/effects.go'
        regression='src/backend/voice/internal/spacelifecycle/recovery_publication_test.go'
        repair_paths=[RUNTIME,service,effects,regression]
        self.write(service,'historical mutable publication\n');self.write(effects,'effect interface bindings\n')
        def text(name,left='base-left',right='base-right',manual=False):
            middle='\n'.join('stable '+str(i) for i in range(20))
            return left+'\n'+middle+'\n'+right+'\n'+('reviewed manual contribution\n' if manual else '')
        for name in names:
            if name!=retired:self.write(name,text(name))
        base=self.commit('private full33 boundary')
        for name in names:self.write(name,text(name,left='first contribution'))
        for name in conflicts:self.write(name,text(name,left='first contribution',right='first conflict obligation'))
        first=self.commit('private first contributions')
        git(self.repo,'checkout','-qb','corrected-source',base)
        for name in names:
            if name!=retired:self.write(name,text(name,right='incoming contribution'))
        source=self.commit('private incoming contributions')
        git(self.repo,'checkout','-q',first)
        merge=subprocess.run(['git','-C',str(self.repo),'merge','--no-ff','--no-commit',source],capture_output=True,text=True)
        self.assertEqual(merge.returncode,1,merge.stderr)
        for name in names:
            if name==retired:
                (self.repo/name).unlink();continue
            raw=text(name,left='first contribution',right='incoming contribution',manual=True)
            if name in conflicts:raw+='first conflict obligation\n'
            if name==RUNTIME:raw+='KNOWN_STARTUP_PUBLICATION_DEFECT\n'
            self.write(name,raw)
        historical=self.commit('private reviewed manual33 with known historical defect')
        mt=subprocess.run(['git','-C',str(self.repo),'merge-tree','--write-tree','--name-only','-z','--no-messages',first,source],capture_output=True)
        self.assertEqual(mt.returncode,1,mt.stderr)
        parts=mt.stdout.split(b'\0');automatic=parts[0].decode()
        self.assertEqual(sorted(x.decode() for x in parts[1:-1]),conflicts)
        self.assertEqual(git(self.repo,'diff','--name-only',automatic,historical).splitlines(),names)
        self.write(RUNTIME,(self.repo/RUNTIME).read_text().replace('KNOWN_STARTUP_PUBLICATION_DEFECT','CURRENT_REVIEWED_STARTUP_REPAIR'))
        self.write(service,'current immutable snapshot\n');self.write(regression,'current publication regression\n')
        correction=self.commit('private required current correction')
        for name in names:
            if name==retired:
                self.assertEqual(git(self.repo,'ls-tree',correction,'--',name),'')
                self.assertEqual(git(self.repo,'ls-tree',historical,'--',name),'')
                self.assertIn('first contribution',git(self.repo,'show',first+':'+name))
                continue
            else:current=git(self.repo,'show',correction+':'+name)
            self.assertIn('first contribution',current);self.assertIn('incoming contribution',current)
            self.assertIn('reviewed manual contribution',current)
        self.assertNotIn('KNOWN_STARTUP_PUBLICATION_DEFECT',git(self.repo,'show',correction+':'+RUNTIME))
        # This private accepted-source/current-correction contract is supplied
        # separately from native/M/J authority. The later implementation must
        # validate its whole unit and negatives, not merely select this path.
        m=module();m.ROOT=self.repo
        whole_entries={n:{k:(value+'\0' if value else '') for k,rev in
            [('base',base),('first',first),('source',source),('automatic',automatic),('historical',historical)]
            for value in [git(self.repo,'ls-tree',rev,'--',n)]} for n in names}
        # Convert stripped newline records to the canonical single NUL record.
        current_bindings={n:(raw+'\0' if raw else '') for n in sorted(set(names+repair_paths))
            for raw in [git(self.repo,'ls-tree',correction,'--',n)]}
        unit=dict(type='F13_FIXED_HISTORICAL_COMPOSITION_WITH_REQUIRED_CURRENT_CORRECTION',
            merge=historical,parents=[first,source],best_bases=[base],automatic_tree=automatic,
            result_tree=git(self.repo,'rev-parse',historical+'^{tree}'),conflicts=conflicts,manual_paths=names,
            entries=whole_entries,required_current_correction=correction,qualified_current=correction,
            current_bindings=current_bindings,repair_bindings={n:dict(correction=current_bindings[n],candidate=current_bindings[n]) for n in repair_paths},
            historical_gap='KNOWN_STARTUP_PUBLICATION_DEFECT',source_condition='Retained contributions and mandatory current repair',
            evidence={str(i):hashlib.sha256(('private evidence '+str(i)).encode()).hexdigest() for i in range(4)},
            producer_only=True,runtime_pass=False,activation_authority=False,current_or_future_path_authority=False)
        unit_file=self.folder/'private-source-unit.json';unit_file.write_text(json.dumps(unit))
        m.F_CORRECTED_UNIT_SHA=m.sha(unit_file.read_bytes());m.F_CORRECTED_REPAIR_PATHS=repair_paths
        m.F_CORRECTED_TUPLE=dict(merge=historical,first=first,source=source,base=base,automatic_tree=automatic,
            result_tree=unit['result_tree'],correction=correction,current=correction,source_ancestor=correction)
        raw='\t'.join([retired,'true',first,base,base,first,base,base])
        m.F_DELETION_ROW_SHA=m.sha(raw.encode());m.F_DELETION_ENTRY=git(self.repo,'ls-tree',first,'--',retired)+'\0'
        ruling=self.folder/'private-reviewed-deletion.txt';ruling.write_text('private fixture review, not production authority')
        m.F_DELETION_RULING_SHA=m.sha(ruling.read_bytes());m.J3071_RANGE=dict(m.J3071_RANGE,base=base)
        helper=self.repo/'scripts/ci/f13_f54_history.py'
        lines=HELPER.read_text().splitlines(keepends=True);bindings={n:getattr(m,n) for n in
            ('F_CORRECTED_UNIT_SHA','F_CORRECTED_TUPLE','F_CORRECTED_REPAIR_PATHS',
             'F_DELETION_ROW_SHA','F_DELETION_ENTRY','F_DELETION_RULING_SHA','J3071_RANGE')}
        for node in sorted([n for n in ast.parse(''.join(lines)).body if isinstance(n,ast.Assign)
                and isinstance(n.targets[0],ast.Name) and n.targets[0].id in bindings],key=lambda n:n.lineno,reverse=True):
            name=node.targets[0].id;lines[node.lineno-1:node.end_lineno]=[name+' = '+repr(bindings[name])+'\n']
        helper.write_text(''.join(lines))
        shell=self.repo/'scripts/ci/voice-db-runtime-provisioning-contract_test.sh'
        shell.write_text(SHELL.read_text().replace('fa1528a7c232a644218edf49811e2bb2e373105f',historical))
        target=self.commit('private immutable corrected-history carrier')
        context=dict(GITHUB_EVENT_NAME='pull_request',GITHUB_REF='refs/pull/875/merge',GITHUB_SHA=target,GITHUB_BASE_REF='master')
        capability=self.folder/'private-corrected-history-capability.json'
        with patch.dict(m.os.environ,context):capability.write_text(json.dumps(m.prepare_corrected_F_capability(self.repo,unit_file)))
        result_entry=git(self.repo,'ls-tree',historical,'--',RUNTIME)
        args=[self.repo,base,first,source,RUNTIME,result_entry,historical,'',helper,'',capability]
        program='set -eu\n'+definitions().replace('fa1528a7c232a644218edf49811e2bb2e373105f',historical)+'\nTMP_DIR='+shlex.quote(str(self.folder))+'\n'
        with patch.dict(m.os.environ,context):
            result=subprocess.run(['bash','-c',program+'f13_checkpoint_merge_tree_entry_matches '+
                ' '.join(shlex.quote(str(value)) for value in args)+'\n'],capture_output=True,text=True)
            self.assertEqual(result.returncode,0,'A complete manual source with mandatory retained current repair is rejected: '+result.stderr)
            proof=m.corrected_F_composition(self.repo,capability,historical,base,first,source,RUNTIME,result_entry)
            self.assertEqual(proof['merge_tree_exit'],1);self.assertEqual(proof['complete_manual_paths'],names)
            self.assertIn('KNOWN_STARTUP_PUBLICATION_DEFECT',proof['historical_gap'])
            self.assertFalse(proof['runtime_pass']);self.assertTrue(proof['retain_runtime_and_content_taint'])
            self._private_F_case=dict(module=m,capability=json.loads(capability.read_bytes()),proof=proof,
                target=target,context=context,raw=raw,retired=retired,ruling=ruling)
            original=unit_file.read_bytes()
            import copy
            for error in ('partial','parent','base','tree','auto','mode','entry','current','forged-absence',
                    'missing-repair','repair-absence','correction','conflict','runtime','activation','future','gap'):
                altered=copy.deepcopy(unit)
                if error=='partial':altered['manual_paths'].pop()
                elif error=='parent':altered['parents'].reverse()
                elif error=='base':altered['best_bases']=[first]
                elif error=='tree':altered['result_tree']='0'*40
                elif error=='auto':altered['automatic_tree']='0'*40
                elif error=='mode':altered['entries'][RUNTIME]['historical']=altered['entries'][RUNTIME]['historical'].replace('100644','100755')
                elif error=='entry':altered['entries'][names[0]]['first']=altered['entries'][names[0]]['source']
                elif error=='current':altered['current_bindings'].pop(names[0])
                elif error=='forged-absence':altered['current_bindings'][RUNTIME]=''
                elif error=='missing-repair':altered['repair_bindings'].pop(service)
                elif error=='repair-absence':altered['current_bindings'][service]=''
                elif error=='correction':altered['required_current_correction']=historical
                elif error=='conflict':altered['conflicts'].pop()
                elif error=='gap':altered['historical_gap']=''
                else:altered[dict(runtime='runtime_pass',activation='activation_authority',future='current_or_future_path_authority')[error]]=True
                unit_file.write_text(json.dumps(altered));m.F_CORRECTED_UNIT_SHA=m.sha(unit_file.read_bytes())
                with self.subTest(error=error),self.assertRaises(m.Rejected):m.corrected_F_whole(self.repo,unit_file,target)
            unit_file.write_bytes(original);m.F_CORRECTED_UNIT_SHA=m.sha(original)
            for bad_args in ((historical,first,source,RUNTIME,result_entry),(base,source,first,RUNTIME,result_entry),
                (base,first,source,RUNTIME+'.future',result_entry),(base,first,source,RUNTIME,result_entry.replace('100644','100755'))):
                with self.assertRaises(m.Rejected):m.corrected_F_composition(self.repo,capability,historical,*bad_args)
            with patch.dict(m.os.environ,{'GITHUB_BASE_REF':'develop'}),self.assertRaises(m.Rejected):
                m.corrected_F_composition(self.repo,capability,historical,base,first,source,RUNTIME,result_entry)
            self.write(RUNTIME,'regressed current startup binding\n');regressed=self.commit('private current regression')
            with self.assertRaises(m.Rejected):m.corrected_F_whole(self.repo,unit_file,regressed)
            with self.assertRaises(m.Rejected):m.corrected_F_whole(self.repo,unit_file,historical)

    def test_reviewed_deletion_constructor_requires_real_F33_transition_and_repair(self):
        # Real private Git graph/full33 source/current repair. This exercises
        # the value constructor, not an invented original-engine trace.
        import copy
        self.test_corrected_historical_manual_unit_requires_complete_source_and_current_repair()
        case=self._private_F_case;m=case['module'];f=m.F_CORRECTED_TUPLE
        git(self.repo,'checkout','-q',case['target'])
        raw=case['raw'];binding=m.file_digest((raw+'\n').encode());frame='private-F-frame'
        state=dict(frame_context=dict(id=frame,base=m.J3071_RANGE['base'],target=f['correction'],
            allow_direct_followup='true',partition_source_history='false'),F_operation=frame+'/'+f['merge']+'/2',
            operation_binding=dict(commit=f['merge'],first_parent=f['first'],boundary=f['base'],source_head=f['source']),
            classification=dict(trigger='true'),composition_receipts=[case['proof']],prior_rows=[m.raw_binding(raw)],
            aggregate=True,path_evidence=[dict(private_value_fixture_only=True,row=m.raw_binding(raw))],
            map_binding=binding,source_context=case['capability']['source_context'],range_branch_sha256='1'*64)
        state['original_guard']=dict(gate='f13_guarded_entry_matches_final_target',original_status=1,
            path=case['retired'],target=f['correction'],stored_entry=m.raw_binding(m.F_DELETION_ENTRY.rstrip('\0')),
            target_entry=m.raw_binding(''),guarded_owner=f['first'],source_head=f['first'],
            prior_rows=state['prior_rows'],map_before=binding)
        with patch.dict(m.os.environ,case['context']):
            value=m.reviewed_F_deletion_value(self.repo,case['capability'],state,case['ruling'])
            self.assertEqual(value['expected_terminal_state'],'ABSENT');self.assertTrue(value['aggregate'])
            self.assertEqual(value['map_before'],value['map_after']);self.assertEqual(value['state']['prior_rows'],[m.raw_binding(raw)])
            for error in ('target','base','mode','owner','rawfalse','source','content','context','no-composition',
                    'no-history','false-aggregate','wrong-operation','missing-range',
                    'guard-entry','guard-target','guard-owner','guard-source','guard-status'):
                changed=copy.deepcopy(state)
                if error in ('target','base'):changed['frame_context'][error]=f['first']
                elif error in ('owner','rawfalse','source','content'):
                    parts=raw.split('\t');index=dict(owner=2,rawfalse=1,source=5,content=6)[error]
                    parts[index]='false' if error=='rawfalse' else f['merge'];changed['prior_rows']=[m.raw_binding('\t'.join(parts))]
                elif error=='mode':
                    saved=m.F_DELETION_ENTRY;m.F_DELETION_ENTRY=saved.replace('100644','100755')
                elif error=='context':changed['source_context']={}
                elif error=='no-composition':changed['composition_receipts']=[]
                elif error=='no-history':changed['path_evidence']=[]
                elif error=='false-aggregate':changed['aggregate']=False
                elif error=='wrong-operation':changed['F_operation']=frame+'/'+f['first']+'/1'
                elif error=='missing-range':changed['range_branch_sha256']=''
                elif error=='guard-entry':changed['original_guard']['stored_entry']=m.raw_binding('')
                elif error=='guard-target':changed['original_guard']['target_entry']=m.raw_binding(m.F_DELETION_ENTRY.rstrip('\0'))
                elif error in ('guard-owner','guard-source'):
                    changed['original_guard']['guarded_owner' if error=='guard-owner' else 'source_head']=f['merge']
                else:changed['original_guard']['original_status']=0
                with self.subTest(constructor=error),self.assertRaises(m.Rejected):
                    m.reviewed_F_deletion_value(self.repo,case['capability'],changed,case['ruling'])
                if error=='mode':m.F_DELETION_ENTRY=saved
            with self.assertRaises(m.Rejected):m.tree_entry(self.repo,'0'*40,case['retired'])
        ordinary=re.search(r'(?ms)^f13_guarded_entry_matches_final_target\(\) \{\n.*?^\}\n',SHELL.read_text()).group(0)
        result=subprocess.run(['bash','-c',ordinary+'\nf13_guarded_entry_matches_final_target '+
            shlex.quote(m.F_DELETION_ENTRY.rstrip('\0'))+" ''\n"],capture_output=True)
        self.assertEqual(result.returncode,1,'Default PRESENT guard must still reject without the typed disposition')

    def test_reviewed_deletion_preserves_raw_history_and_consumes_typed_state(self):
        # Fold-only finite representation test, not a synthetic source or
        # original-engine qualification. Real F/repair/gate proof is separate.
        import copy
        m=module();f=m.F_CORRECTED_TUPLE;name=m.F_DELETION_PATH;frame='private-retained-frame'
        raw='\t'.join([name,'true',f['first'],f['base'],f['base'],f['first'],f['base'],f['base']])
        self.assertEqual(m.sha(raw.encode()),m.F_DELETION_ROW_SHA)
        binding=m.file_digest((raw+'\n').encode())
        receipt=dict(type='F54_REVIEWED_F_DELETION_v1',path=name,expected_terminal_state='ABSENT',
            aggregate=True,map_before=binding,map_after=binding,
            state=dict(prior_rows=[m.raw_binding(raw)],frame_context=dict(id=frame)))
        ledger={frame:dict(rows=[raw],map_binding=binding,paths={name:dict(aggregate=True,
            evidence=[dict(private_fold_only=True,row=m.raw_binding(raw))])},terminal_dispositions={name:receipt})}
        event=dict(kind='reviewed_F_deletion',frame=frame,operation=None,payload=dict(receipt=receipt,
            receipt_sha256=m.sha(m.canonical(receipt)),map_after=binding))
        trace=m.canonical(event)+b'\n';prefix=m.file_digest(trace)
        expected=m.terminal_dispositions_in_prefix(ledger,trace,prefix)
        self.assertEqual(len(expected),1);self.assertEqual(ledger[frame]['rows'],[raw])
        self.assertTrue(ledger[frame]['paths'][name]['aggregate'])
        bound=dict(path=name,covered=True,aggregate=True,obligations=dict(aggregate=True,
            retained_evidence=[dict(private_fold_only=True)]),terminal_dispositions=expected)
        self.assertEqual(m.retained_row_for_binding(bound),raw)
        for error in ('false-OR','wrong-path','filtered-history','forged-owner'):
            altered=copy.deepcopy(bound)
            if error=='false-OR':altered['aggregate']=False
            elif error=='wrong-path':altered['path']=name+'.future'
            elif error=='filtered-history':altered['obligations']['retained_evidence']=[]
            else:
                item=altered['terminal_dispositions'][0]
                item['receipt']['state']['prior_rows'][0]=m.raw_binding(raw.replace(f['first'],f['merge']))
                item['receipt_sha256']=m.sha(m.canonical(item['receipt']))
            with self.subTest(projection=error),self.assertRaises(m.Rejected):m.retained_row_for_binding(altered)
        for error in ('row-drop','false','owner','source','content','synthetic-F','missing-history',
                'missing-disposition','map-mutation','frame','terminal-present','other-path'):
            altered=copy.deepcopy(ledger);value=altered[frame];r=value['terminal_dispositions'][name]
            if error=='row-drop':value['rows']=[]
            elif error in ('false','owner','source','content','synthetic-F'):
                parts=value['rows'][0].split('\t')
                index=dict(false=1,owner=2,source=5,content=6,**{'synthetic-F':2})[error]
                parts[index]='false' if error=='false' else f['merge'];value['rows']=['\t'.join(parts)]
            elif error=='missing-history':value['paths'][name]['evidence']=[]
            elif error=='missing-disposition':value['terminal_dispositions']={}
            elif error=='map-mutation':r['map_after']=m.file_digest(b'changed')
            elif error=='frame':r['state']['frame_context']['id']='foreign-frame'
            elif error=='terminal-present':r['expected_terminal_state']='PRESENT'
            else:r['path']=name+'.future';value['terminal_dispositions']={name+'.future':r}
            with self.subTest(error=error),self.assertRaises(m.Rejected):
                m.terminal_dispositions_in_prefix(altered,trace,prefix)
        # Rehashing transport cannot manufacture or remove the typed state.
        for modified in (b'',m.canonical(dict(event,operation='fake-F-row'))+b'\n'):
            with self.assertRaises(m.Rejected):m.terminal_dispositions_in_prefix(ledger,modified,m.file_digest(modified))

    def test_large_range_receipts_use_exact_file_transport_not_argv(self):
        # Transport only: the collector stub grants no range/source authority.
        # The real caller used to expand this complete origin into one argv.
        m=module();origin=self.folder/'large-origin.json';trace=self.folder/'transport.jsonl'
        payload=dict(type='PRIVATE_TRANSPORT_ONLY',admission=False,private_transport_only=True,retained_maps={'raw-owner-evidence':'x'*262144})
        origin.write_bytes(m.canonical(payload))
        names={'f13_f54_trace_event','f13_f54_fixed3071_range_side'}
        functions='\n'.join(match.group(0) for match in re.finditer(
            r'(?ms)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{\n.*?^\}\n',SHELL.read_text()) if match.group(1) in names)
        program='set -eu\n'+functions+'\nf13_collect_develop_contributions() { return 0; }\n'
        program+='f13_f54_observer='+shlex.quote(str(HELPER))+'\nTMP_DIR='+shlex.quote(str(self.folder))+'\n'
        program+='f13_f54_fixed3071_range_side '+ ' '.join(shlex.quote(str(value)) for value in (self.repo,origin,trace,self.folder/'unused-map'))+'\n'
        result=subprocess.run(['bash','-c',program],capture_output=True,text=True)
        self.assertEqual(result.returncode,0,result.stderr)
        first=json.loads(trace.read_bytes());self.assertEqual(first['payload'],payload)
        for receipt in (origin,self.folder/'missing.json',self.folder/'malformed.json'):
            if receipt.name=='malformed.json':receipt.write_text('{incomplete')
            before=trace.read_bytes()
            command=['python3',str(HELPER),'append',str(trace),'range_3071_branch','private-frame','private-operation',str(self.repo),str(receipt)]
            result=subprocess.run(command,capture_output=True,text=True)
            if receipt==origin:
                self.assertEqual(result.returncode,0,result.stderr)
                second=json.loads(trace.read_bytes().splitlines()[-1]);self.assertEqual(second['payload'],payload)
                self.assertEqual(second['previous'],first['digest'])
            else:
                self.assertNotEqual(result.returncode,0)
                self.assertEqual(trace.read_bytes(),before)
        # These are transport events, not real engine observations.
        with self.assertRaises(m.Rejected):m.validate_trace(self.repo,trace,m.sha(trace.read_bytes()))

    def test_J_transition_cannot_borrow_M_ledger_transport(self):
        # Private transport forgery, not an observed J classification or row.
        m=module();name=PROTO;op='private/'+m.J3071+'/2'
        incoming='\t'.join((name,'false',m.J3071,m.J3071_P1,m.J3071_O,m.J3071_P2,m.J3071_P1,','.join(m.J3071_BASES)))
        empty=dict(bytes=0,sha256=m.sha(b''))
        before=dict(kind='row_before',operation=op,sequence=1,payload=dict(path=name,map_before=empty,prior_rows=[],incoming=m.raw_binding(incoming)))
        trace=self.folder/'private-J-as-M-trace';trace.write_text(json.dumps(before)+'\n')
        value=dict(type='F54_ACTUAL_M_COMPLETE_RETAINED_LEDGER',admission=False,operation=op,
            prefix=empty,operation_binding=dict(commit=m.J3071,paths=[name]),outer_rows=[],
            required_qualification={name:dict(aggregate=True,retained_evidence=[dict(private_fixture=True)])})
        ledger=self.folder/'private-J-as-M-ledger';ledger.write_bytes(m.canonical(value))
        with self.assertRaises(m.Rejected):
            m.consume_M_path(ledger,m.sha(ledger.read_bytes()),trace,name,b'',incoming,'append')

    def test_J_prior_prefix_preserves_original_identity_not_a_temporary_alias(self):
        # Isolated prefix-call seam, with an explicit private authority/frame
        # boundary. Actual fixed-J authority is exercised by the frozen probe,
        # not manufactured by this fixture.
        m=module();trace=self.folder/'original-main.jsonl';prefix=b'private sealed collection\n'
        trace.write_bytes(prefix+b'later events are outside this seal\n')
        bound=m.file_digest(prefix);cap=dict(type='F54_FIXED3071_STRUCTURAL_CAPABILITIES_PROTOTYPE',directory='private-unit',native_path='private-native')
        operation='private/J/2';state=dict(private_original_range=True)
        origin=dict(capability=cap,main_trace=str(trace),prefix=bound,J_operation=operation)
        expected=dict(origin);calls=[]
        m.prepare_3071_capabilities=lambda repo,directory,native:cap
        def checkpoint(repo,path,digest,**kwargs):
            calls.append(str(path));m.require(str(path)==str(trace),'private_original_range_identity')
            m.require(kwargs.get('sealed_prefix')==bound,'private_same_sealed_collection_not_old_ledger')
            m.require(kwargs.get('J_history_checkpoint')==operation,'private_same_original_frame')
            raw=Path(path).read_bytes()[:kwargs['sealed_prefix']['bytes']]
            m.require(m.sha(raw)==digest==bound['sha256'],'private_original_prefix_hash')
            return dict(J_history_checkpoint=state)
        m.validate_trace=checkpoint
        m.J_prior_origin_value=lambda repo,authority,value,path,binding:dict(expected,main_trace=str(path),prefix=binding,capability=authority)
        self.assertEqual(m.verify_J_prior_origin(self.repo,origin),origin)
        self.assertEqual(calls,[str(trace)])
        foreign=self.folder/'foreign-same-bytes.jsonl';foreign.write_bytes(trace.read_bytes())
        for error in ('foreign-identical-file','old-ledger-rehashed','foreign-frame','changed-source-prefix'):
            value=json.loads(json.dumps(origin));original=trace.read_bytes()
            if error=='foreign-identical-file':value['main_trace']=str(foreign)
            elif error=='old-ledger-rehashed':value['prefix']=m.file_digest(prefix[:4])
            elif error=='foreign-frame':value['J_operation']='private/foreign/2'
            else:trace.write_bytes(b'X'+original[1:])
            with self.subTest(error=error),self.assertRaises(m.Rejected):m.verify_J_prior_origin(self.repo,value)
            trace.write_bytes(original)

    def test_trace_prefix_is_exactly_sealed_and_old_unfinished_ledger_rejects(self):
        m=module();self.hidden_runtime();run,out=self.collect(partition=False)
        self.assertEqual(run.returncode,0,run.stderr)
        trace=out/'trace.jsonl';prefix=trace.read_bytes();bound=m.file_digest(prefix)
        trace.write_bytes(prefix+b'{"later":"not part of the collection seal"}\n')
        value=m.validate_trace(self.repo,trace,bound['sha256'],sealed_prefix=bound)
        self.assertFalse(value['failures'])
        self.assertEqual(value['ledger'][value['root_frame']]['map_binding'],m.file_digest((out/'map').read_bytes()))
        with self.assertRaisesRegex(m.Rejected,'trace_raw_hash'):m.validate_trace(self.repo,trace,bound['sha256'])
        for error in ('zero-length','boolean-length','oversized-length','different-hash','old-unfinished-rehashed'):
            binding=dict(bound);digest=bound['sha256']
            if error=='zero-length':binding['bytes']=0
            elif error=='boolean-length':binding['bytes']=True
            elif error=='oversized-length':binding['bytes']=len(trace.read_bytes())+1
            elif error=='different-hash':binding['sha256']='0'*64
            else:
                earlier=b''.join(prefix.splitlines(keepends=True)[:-1]);binding=m.file_digest(earlier);digest=binding['sha256']
            with self.subTest(error=error),self.assertRaises(m.Rejected):
                m.validate_trace(self.repo,trace,digest,sealed_prefix=binding)

    def test_private_J_prior_span_requires_all_five_real_operations_and_full_map(self):
        # Actual temporary Git/collector evidence under an explicit private
        # authority boundary. This does not qualify the fixed production J.
        m=module();self.hidden_runtime();nodes=[self.first,self.head]
        for index in range(3):
            self.write(PROTO,'private supplemental '+str(index)+'\n')
            nodes.append(self.commit('private supplemental direct '+str(index)))
        origin=dict(type='PRIVATE_J_PRIOR_AUTHORITY_FIXTURE',admission=False)
        run,out=self.collect(target=nodes[-1],partition=False,origin=origin,origin_kind='J_prior_span_link')
        self.assertEqual(run.returncode,0,run.stderr)
        m.J3071_PRIOR_SPAN=dict(base=self.base,target=nodes[-1],nodes=nodes)
        # Only authority is mocked; operation/parent/delta/classifier/map and
        # append-only evidence validation below still executes normally.
        m.verify_J_prior_origin=lambda repo,value:self.assertEqual(value,origin)
        trace=out/'trace.jsonl';raw=trace.read_bytes();events=[json.loads(x) for x in raw.splitlines()]
        value=m.verify_J_prior_span(self.repo,origin,trace,out/'map')
        self.assertEqual(len(value['actual_operations']),5)
        self.assertTrue(value['ledger']['paths'][RUNTIME]['aggregate'])
        self.assertEqual(len(value['classifications']),5)
        for error in ('drop-whole-operation','classification-flag','frame-flags','node-order','map-bytes','missing-root-exit'):
            changed=json.loads(json.dumps(events));map_path=out/'map';original_nodes=list(m.J3071_PRIOR_SPAN['nodes'])
            if error=='drop-whole-operation':
                operation=next(e['operation'] for e in changed if e['kind']=='operation_begin')
                changed=[e for e in changed if e['operation']!=operation]
            elif error=='classification-flag':
                event=next(e for e in changed if e['kind']=='classification')
                event['payload']['trigger']='false' if event['payload']['trigger']=='true' else 'true'
            elif error=='frame-flags':
                next(e for e in changed if e['kind']=='frame_enter')['payload']['partition_source_history']='true'
            elif error=='node-order':m.J3071_PRIOR_SPAN['nodes']=list(reversed(original_nodes))
            elif error=='map-bytes':
                map_path=out/'forged-map';map_path.write_bytes((out/'map').read_bytes()+b'forged\n')
            else:changed=[e for e in changed if e['kind']!='frame_exit']
            forged=out/('forged-'+error+'.jsonl');m.write_events(forged,changed)
            expected='closed_frames' if error=='missing-root-exit' else '.'
            with self.subTest(error=error),self.assertRaisesRegex(m.Rejected,expected):
                m.verify_J_prior_span(self.repo,origin,forged,map_path)
            m.J3071_PRIOR_SPAN['nodes']=original_nodes

    def test_private_J_consumer_retains_prior_span_true_without_rewriting_append(self):
        # Transport/conservation fixture only: its real temporary source traces
        # do not qualify the fixed production J boundary or the five-node span.
        m=module();self.hidden_runtime();middle=self.head
        self.write(PROTO,'J private final false\n');tail=self.commit('private J transport')
        side_run,side=self.collect(target=tail,partition=False)
        self.assertEqual(side_run.returncode,0,side_run.stderr)
        side_raw=(side/'trace.jsonl').read_bytes();side_value=m.validate_trace(self.repo,side/'trace.jsonl',m.sha(side_raw))
        side_ledger=side_value['ledger'][side_value['root_frame']]
        old_base=self.base;self.base=middle
        main_run,main=self.collect(target=tail,partition=False);self.base=old_base
        self.assertEqual(main_run.returncode,0,main_run.stderr)
        events=[json.loads(x) for x in (main/'trace.jsonl').read_bytes().splitlines()]
        before=next(e for e in events if e['kind']=='row_before' and e['payload']['path']==PROTO)
        classification=next(e['payload'] for e in events if e['kind']=='classification')
        binding=next(e['payload'] for e in events if e['kind']=='operation_begin')
        incoming=before['payload']['incoming']['raw'];self.assertEqual(incoming.split('\t')[1],'false')
        # Only this private transport identity is synthetic; raw source rows,
        # classifier and prior-span evidence above come from temporary traces.
        before['operation']='private/'+tail+'/2'
        prefix=b''.join((main/'trace.jsonl').read_bytes().splitlines(keepends=True)[:before['sequence']-1])
        trace=self.folder/'J-private-trace';trace.write_bytes(prefix+json.dumps(before,sort_keys=True).encode()+b'\n')
        empty=dict(bytes=0,sha256=m.sha(b''));frame=before['frame'];m.J3071=tail
        span=dict(ledger=side_ledger,trace_sha256=m.sha(side_raw),actual_operations=[e['operation'] for e in map(json.loads,side_raw.splitlines()) if e['kind']=='operation_begin'])
        state=dict(frame=frame,operation=before['operation'],operation_binding=binding,classification=classification,
            capability_sha256='private-fixture-only',frame_ancestry=[],ledger={frame:dict(map_binding=empty,rows=[],paths={})},
            coverage=[],prior_span=span,M_ready={},I_ready={},composition={})
        ready=m.build_3071_J_ready(state,dict(bytes=len(prefix),sha256=m.sha(prefix)))
        self.assertTrue(ready['required_qualification'][PROTO]['aggregate'])
        self.assertEqual(ready['required_qualification'][PROTO]['prior_span_evidence'][0]['evidence'],side_ledger['paths'][PROTO]['evidence'])
        # A reverted runtime path outside J's final delta must reach collection
        # with its prior true and actual owner evidence, not just net paths.
        self.assertNotIn(RUNTIME,binding['paths'])
        required=m.include_typed_collection_required({}, {}, {ready['operation']:ready})
        self.assertTrue(required[RUNTIME]['aggregate'])
        self.assertEqual(required[RUNTIME]['retained_evidence'][0]['evidence'],ready['required_qualification'][RUNTIME])
        forged=json.loads(json.dumps(ready));forged['required_qualification'][RUNTIME]['aggregate']=1
        with self.assertRaises(m.Rejected):m.include_typed_collection_required({}, {}, {forged['operation']:forged})
        ledger=self.folder/'J-private-ledger';ledger.write_bytes(m.canonical(ready));digest=m.sha(ledger.read_bytes())
        receipt=m.consume_3071_J_path(ledger,digest,trace,PROTO,b'',incoming,'append')
        self.assertTrue(receipt['aggregate']);self.assertEqual(receipt['after_gate_row']['raw'],incoming)
        for error in ('drop-true','drop-owner-evidence','integer-aggregate','normalize-append','wrong-source','wrong-owner','wrong-map','M-type'):
            value=json.loads(json.dumps(ready));after=incoming;actual=b''
            if error=='drop-true':value['required_qualification'][PROTO]['aggregate']=False
            elif error=='drop-owner-evidence':value['required_qualification'][PROTO]['prior_span_evidence']=[]
            elif error=='integer-aggregate':value['required_qualification'][PROTO]['aggregate']=1
            elif error=='normalize-append':
                parts=incoming.split('\t');parts[1]='true';after='\t'.join(parts)
            elif error in ('wrong-source','wrong-owner'):
                parts=incoming.split('\t');parts[5 if error=='wrong-source' else 2]=middle;after='\t'.join(parts)
            elif error=='wrong-map':actual=b'foreign map\n'
            else:value['type']='F54_ACTUAL_M_COMPLETE_RETAINED_LEDGER'
            ledger.write_bytes(m.canonical(value))
            with self.subTest(error=error),self.assertRaises(m.Rejected):
                m.consume_3071_J_path(ledger,m.sha(ledger.read_bytes()),trace,PROTO,actual,after,'append')

    def test_private_production_J_permission_roundtrip_before_mutation(self):
        # Private transport fixture only: real temporary collector maps and
        # history below are not the fixed production J source qualification.
        # The independent capability boundary is tested with real Git in
        # f13_production_source_test; this isolates its distinct receipt at the
        # actual J pre-mutation consumer and conservation validator.
        m=module();m.ROOT=self.repo;self.hidden_runtime();middle=self.head
        prior_run,prior=self.collect(target=middle,partition=False)
        self.assertEqual(prior_run.returncode,0,prior_run.stderr)
        prior_raw=(prior/'trace.jsonl').read_bytes()
        prior_state=m.validate_trace(self.repo,prior/'trace.jsonl',m.sha(prior_raw))
        prior_ledger=prior_state['ledger'][prior_state['root_frame']]
        actual=(prior/'map').read_bytes()
        self.write(PROTO,'production receipt private transport\n');tail=self.commit('private pending replacement')
        run,out=self.collect(target=tail,partition=False);self.assertEqual(run.returncode,0,run.stderr)
        raw=(out/'trace.jsonl').read_bytes();events=[json.loads(x) for x in raw.splitlines()]
        before=next(e for e in reversed(events) if e['kind']=='row_before' and e['payload']['path']==PROTO)
        original_op=before['operation']
        binding=next(e['payload'] for e in events if e['kind']=='operation_begin' and e['operation']==original_op)
        classification=next(e['payload'] for e in events if e['kind']=='classification' and e['operation']==original_op)
        self.assertEqual(before['payload']['map_before'],m.file_digest(actual))
        incoming=before['payload']['incoming']['raw'];m.J3071=tail;m.EXACT_J_AUDIT_INCOMING=incoming
        before['operation']='private/'+tail+'/2'
        prefix=b''.join(raw.splitlines(keepends=True)[:before['sequence']-1])
        trace=self.folder/'production-J-private-trace';trace.write_bytes(prefix+json.dumps(before,sort_keys=True).encode()+b'\n')
        span=dict(ledger=prior_ledger,trace_sha256=m.sha(prior_raw),actual_operations=[e['operation'] for e in map(json.loads,prior_raw.splitlines()) if e['kind']=='operation_begin'])
        state=dict(frame=before['frame'],operation=before['operation'],operation_binding=binding,classification=classification,
            capability_sha256='PRIVATE_PRODUCTION_TRANSPORT_ONLY',frame_ancestry=[],ledger={before['frame']:prior_ledger},
            coverage=[],prior_span=span,M_ready={},I_ready={},composition={})
        ready=m.build_3071_J_ready(state,m.file_digest(prefix))
        ledger=self.folder/'production-J-private-ledger';ledger.write_bytes(m.canonical(ready));digest=m.sha(ledger.read_bytes())
        permission=dict(type='F54_EXACT_J_AUDIT_OWNER_TRANSITION_PRODUCTION_v1',ledger_sha256=digest,
                        map_before=m.file_digest(actual),operation=before['operation'])
        calls=[]
        def independently_qualified_transport(repo,path,trace_sha,**options):
            self.assertEqual(options,dict(exact_J_checkpoint='permitted'))
            self.assertEqual(repo,self.repo);self.assertEqual(Path(path),trace)
            self.assertEqual(trace_sha,m.sha(trace.read_bytes()));calls.append(path)
            return dict(exact_J_permission=permission)
        m.validate_trace=independently_qualified_transport
        receipt=m.consume_3071_J_path(ledger,digest,trace,PROTO,actual,incoming,'replace')
        self.assertEqual(calls,[trace]);self.assertEqual(receipt['exact_J_owner_permission'],permission)
        self.assertEqual((prior/'map').read_bytes(),actual)
        pending=dict(op=before['operation'],exact_J_owner_permission=permission)
        m.validate_exact_conservation(pending,before['operation'],receipt)
        changed=dict(receipt,exact_J_owner_permission=dict(permission,type='F54_EXACT_J_AUDIT_OWNER_TRANSITION_AUTHOR_v1'))
        with self.assertRaises(m.Rejected):m.validate_exact_conservation(pending,before['operation'],changed)
        permission['ledger_sha256']='0'*64
        with self.assertRaisesRegex(m.Rejected,'exact_J_consumer_current_full_binding'):
            m.consume_3071_J_path(ledger,digest,trace,PROTO,actual,incoming,'replace')
        self.assertEqual((prior/'map').read_bytes(),actual)

    def test_generic_or_synthetic_stop_is_not_production_completion(self):
        m=module();self.hidden_runtime();run,out=self.collect(partition=False)
        self.assertEqual(run.returncode,0,run.stderr)
        # A genuinely complete ordinary trace is not an M/J/F production run.
        with self.assertRaisesRegex(m.Rejected,'successful_exact_M_collection_required'):
            m.finalize_collection(self.repo,out/'trace.jsonl',out/'map')
        events=[json.loads(x) for x in (out/'trace.jsonl').read_bytes().splitlines()]
        events[-1]['payload']['exit']=1
        stopped=self.folder/'synthetic-stopped-trace';m.write_events(stopped,events)
        with self.assertRaises(m.Rejected):m.finalize_collection(self.repo,stopped,out/'map')

    def test_private_I_consumer_keeps_actual_side_true_and_raw_false_main_row(self):
        # Private transport/oracles only. These are real temporary collector
        # traces, not production I/J observations or an exact-nine qualification.
        self.hidden_runtime();middle=self.head
        self.write(PROTO,'later proto only\n');tail=self.commit('private false final source')
        m=module();side_run,side=self.collect(target=tail,partition=False)
        self.assertEqual(side_run.returncode,0,side_run.stderr)
        side_raw=(side/'trace.jsonl').read_bytes();side_state=m.validate_trace(self.repo,side/'trace.jsonl',m.sha(side_raw))
        side_ledger=side_state['ledger'][side_state['root_frame']]
        original_base=self.base;self.base=middle
        main_run,main=self.collect(target=tail,partition=False);self.base=original_base
        self.assertEqual(main_run.returncode,0,main_run.stderr)
        events=[json.loads(x) for x in (main/'trace.jsonl').read_bytes().splitlines()]
        before=next(e for e in events if e['kind']=='row_before' and e['payload']['path']==PROTO)
        classification=next(e['payload'] for e in events if e['kind']=='classification')
        binding=next(e['payload'] for e in events if e['kind']=='operation_begin')
        incoming=before['payload']['incoming']['raw']
        self.assertEqual(incoming.split('\t')[1],'false');self.assertTrue(side_ledger['paths'][PROTO]['aggregate'])
        prefix=b''.join((main/'trace.jsonl').read_bytes().splitlines(keepends=True)[:before['sequence']-1])
        trace=self.folder/'I-private-pending-trace';trace.write_bytes(prefix+json.dumps(before,sort_keys=True).encode()+b'\n')
        branch=dict(type='F54_FIXED3071_RANGE_ENGINE_OBSERVATION',side_ledger=side_ledger,side_trace_sha256=m.sha(side_raw),
            side_operations=[dict(operation=e['operation'],begin_digest=e['digest']) for e in [json.loads(x) for x in side_raw.splitlines()] if e['kind']=='operation_begin'])
        empty=dict(bytes=0,sha256=m.sha(b''));frame=before['frame']
        state=dict(frame=frame,operation=before['operation'],operation_binding=binding,classification=classification,
            capability_sha256='private-fixture-only',frame_ancestry=[],ledger={frame:dict(map_binding=empty,rows=[],paths={})},
            range_branch=branch,coverage=[],M_ready={})
        ready=m.build_3071_I_ready(state,dict(bytes=len(prefix),sha256=m.sha(prefix)))
        self.assertTrue(ready['required_qualification'][PROTO]['aggregate'])
        self.assertEqual(ready['required_qualification'][PROTO]['range_evidence'][0]['evidence'],side_ledger['paths'][PROTO]['evidence'])
        m.J3071_RANGE=dict(m.J3071_RANGE,integration=tail,first=middle)
        ledger=self.folder/'I-private-ledger';ledger.write_bytes(m.canonical(ready));digest=m.sha(ledger.read_bytes())
        receipt=m.consume_3071_I_path(ledger,digest,trace,PROTO,b'',incoming,'append')
        self.assertTrue(receipt['aggregate']);self.assertEqual(receipt['after_gate_row']['raw'],incoming)
        self.assertEqual(receipt['expected_post_map'],dict(bytes=len((incoming+'\n').encode()),sha256=m.sha((incoming+'\n').encode())))
        for error in ('drop-side-true','integer-aggregate','drop-owner-evidence','normalize-raw-false','wrong-map','wrong-source'):
            changed=json.loads(json.dumps(ready));after=incoming;actual=b''
            if error=='drop-side-true':changed['required_qualification'][PROTO]['aggregate']=False
            elif error=='integer-aggregate':changed['required_qualification'][PROTO]['aggregate']=1
            elif error=='drop-owner-evidence':changed['required_qualification'][PROTO]['range_evidence']=[]
            elif error=='normalize-raw-false':
                parts=incoming.split('\t');parts[1]='true';after='\t'.join(parts)
            elif error=='wrong-map':actual=b'foreign map\n'
            else:
                parts=incoming.split('\t');parts[5]=middle;after='\t'.join(parts)
            ledger.write_bytes(m.canonical(changed))
            with self.assertRaises(m.Rejected):m.consume_3071_I_path(ledger,m.sha(ledger.read_bytes()),trace,PROTO,actual,after,'append')

    def test_fixed_nine_node_range_witness_keeps_default_guard_and_real_F_boundary(self):
        m=module();common=self.base
        self.write('range/direct.txt','direct\n');direct=self.commit('private direct spine')
        git(self.repo,'checkout','-qb','range-side',common)
        self.write('range/base.txt','boundary\n');base=self.commit('private B')
        side=[]
        for index in range(6):
            self.write('range/side'+str(index)+'.txt',str(index)+'\n');side.append(self.commit('private side '+str(index)))
        git(self.repo,'checkout','-q',direct);git(self.repo,'merge','--no-ff','-qm','private F',side[-1]);merge=git(self.repo,'rev-parse','HEAD')
        self.write('range/head.txt','head\n');target=self.commit('private H')
        integration=git(self.repo,'commit-tree',target+'^{tree}','-p',base,'-p',target,'-m','private I')
        m.J3071_RANGE=dict(integration=integration,first=base,base=base,target=target,baseline=common,side=side[-1],direct=direct,merge=merge,side_nodes=side)
        witness=m.fixed3071_range_structure(self.repo,base,target,'true','false')
        self.assertEqual(witness['spine'],[direct,merge,target]);self.assertEqual(witness['side_nodes'],side)
        self.assertEqual(witness['original_guard']['status'],1);self.assertFalse(witness['admission'])
        self.assertEqual(witness['history'],'UNOBSERVED')
        original=self.base;self.base=base
        rejected,out=self.collect(target=target,partition=False)
        self.assertEqual(rejected.returncode,1,rejected.stderr)
        self.assertEqual((out/'category').read_text().strip(),'first_parent_range')
        passed,observed=self.collect(target=side[-1],partition=False)
        self.assertEqual(passed.returncode,0,passed.stderr)
        self.assertEqual(sum(json.loads(x)['kind']=='operation_begin' for x in (observed/'trace.jsonl').read_text().splitlines()),6)
        with self.assertRaisesRegex(m.Rejected,'supplemental_same_actual_origin'):
            m.verify_3071_range_side(self.repo,{},observed/'trace.jsonl',observed/'map')
        self.base=original
        for allow,partition in (('false','false'),('true','true')):
            with self.assertRaises(m.Rejected):m.fixed3071_range_structure(self.repo,base,target,allow,partition)
        with self.assertRaises(m.Rejected):m.fixed3071_range_structure(self.repo,common,target,'true','false')
        m.J3071_RANGE=dict(m.J3071_RANGE,side_nodes=side[:-1])
        with self.assertRaises(m.Rejected):m.fixed3071_range_structure(self.repo,base,target,'true','false')

    def test_private_reviewed3071_equation_keeps_both_parent_obligations(self):
        m=module();old=b'# Base\n\nrest\n';offset=8
        first=b'## P1\nDo not reset identity.\n\n';second=b'## P2\nRequire independent acceptance.\n\n'
        contents=dict(O=old,bestA=old,bestB=old,P1=old[:offset]+first+old[offset:],
            P2=old[:offset]+second+old[offset:],J=old[:offset]+first+second+old[offset:],AUTO=b'<<<<<<< private conflict\n')
        # Fixture-only review pins. No tuple/hash override is available to the
        # production CLI; these bytes never become actual historical evidence.
        m.J3071_DOC_HASHES={k:m.sha(v) for k,v in contents.items()}
        m.J3071_INSERTIONS=[(offset,len(first),m.sha(first)),(offset,len(second),m.sha(second))]
        self.assertEqual(m.verify_3071_document_equation(contents)['result_sha256'],m.sha(contents['J']))
        for wrong in (contents['P1'],contents['P2'],old[:offset]+second+first+old[offset:],
                contents['J'].replace(b'Do not reset',b'Reset'),contents['J']+b'>>>>>>> marker\n'):
            altered=dict(contents,J=wrong)
            with self.assertRaises(m.Rejected):m.verify_3071_document_equation(altered)
        # Even with a privately re-pinned output hash, the source-preservation
        # equation itself must reject a plausible-looking dropped obligation.
        altered=dict(contents,J=contents['P1']);m.J3071_DOC_HASHES['J']=m.sha(altered['J'])
        with self.assertRaisesRegex(m.Rejected,'reviewed_additive_resolution'):m.verify_3071_document_equation(altered)

    def test_fixed3071_structural_boundary_is_distinct_and_precedes_history_and_composition(self):
        m,folder,names=self.exact_M_fixture()
        # Private exact source fixture only: no production tuple, pin or file
        # is changed. A boundary must not require its not-yet-observed child.
        m.J3071,m.J3071_P1,m.J3071_P2=m.M,m.P1,m.P2
        m.J3071_O,m.J3071_BASES,m.J3071_TREE=m.O,m.BASES,m.TREE
        result=m.exact_3071_boundary(self.repo,m.J3071,m.J3071_P1,m.J3071_P2)
        self.assertEqual(result['type'],'F54_FIXED3071_STRUCTURAL_BOUNDARY_PROTOTYPE')
        self.assertEqual(result['history_boundary'],m.O)
        self.assertFalse(result['admission'])
        self.assertEqual(result['history'],'UNOBSERVED')
        self.assertEqual(result['composition'],'UNQUALIFIED')
        for merge,first,source in ((m.J3071,m.J3071_P2,m.J3071_P1),
                (m.J3071_P1,m.J3071_P1,m.J3071_P2)):
            with self.assertRaises(m.Rejected):m.exact_3071_boundary(self.repo,merge,first,source)
        m.J3071_BASES=list(reversed(m.J3071_BASES))
        with self.assertRaises(m.Rejected):m.exact_3071_boundary(self.repo,m.J3071,m.J3071_P1,m.J3071_P2)

    def test_fixed3071_inventory_checks_every_internal_parent_and_version_not_only_net(self):
        m,folder,names=self.exact_M_fixture()
        m.J3071,m.J3071_P1,m.J3071_P2=m.M,m.P1,m.P2
        m.J3071_O,m.J3071_BASES,m.J3071_TREE=m.O,m.BASES,m.TREE
        rule=b'Private structural fixture contract, not production authority'
        (folder/'rudeus410-3071-boundary-decision.md').write_bytes(rule);m.J3071_RULING_SHA=m.sha(rule)
        nodes=git(self.repo,'rev-list','--topo-order','--reverse',m.M,'^'+m.O).splitlines()
        sources=git(self.repo,'rev-list','--topo-order','--reverse',m.P2,'^'+m.O).splitlines()
        edges=[];touched=set()
        for node in nodes:
            parents=git(self.repo,'rev-list','--parents','-n','1',node).split()[1:];sides=[]
            for parent in parents:
                paths=git(self.repo,'diff','--name-only','--no-renames',parent,node).splitlines();touched.update(paths)
                sides.append(dict(parent=parent,paths=paths,raw_diff=m.raw_binding(m.git(self.repo,'diff','--raw','--no-abbrev','--no-renames','-z',parent,node)),
                    entries={name:dict(before=m.tree_entry(self.repo,parent,name),after=m.tree_entry(self.repo,node,name)) for name in paths}))
            edges.append(dict(node=node,ordered_parents=parents,parent_edges=sides,source_side=node in sources))
        refs=dict(O=m.O,bestA=m.BASES[0],bestB=m.BASES[1],P1=m.P1,P2=m.P2,J=m.M,H53=m.M,H4=m.M)
        sets={name:git(self.repo,'diff','--name-only',a,b).splitlines() for name,a,b in
            [('candidate_difference',*m.BASES),('history_to_source',m.O,m.P2),('first_to_result',m.P1,m.M),
             ('source_to_result',m.P2,m.M),('older_to_result',m.O,m.M)]}
        packet=dict(version='F54-FIXED3071-STRUCTURAL-FACTS-v1',admission=False,merge=m.M,parents=[m.P1,m.P2],
            best_bases=m.BASES,older_history_boundary=m.O,committed_tree=m.TREE,ruling_sha256=m.J3071_RULING_SHA,
            nodes=edges,source_nodes=sources,actual_source_first_parent_operations=git(self.repo,'rev-list','--first-parent','--reverse',m.O+'..'+m.P2).splitlines(),
            all_internal_touched_paths=sorted(touched),all_internal_versions={name:{label:m.tree_entry(self.repo,ref,name) for label,ref in refs.items()} for name in sorted(touched)},
            original_caller=dict(target=m.M),original_native_binding=m.raw_binding(m.canonical(dict(snapshot=m.M))),
            source_qualification=dict(status='UNRESOLVED'),actual_history='UNOBSERVED',actual_rows='UNOBSERVED',Git_only_path_sets=sets)
        path=folder/'structural-facts-v1.json'
        def freeze(value):
            raw=m.canonical(value);path.write_bytes(raw);m.J3071_PACKET_SHA=m.sha(raw)
        freeze(packet);self.assertEqual(m.validate_3071_packet(self.repo,folder),packet)
        import copy
        for error in ('missing-node','missing-internal-path','wrong-mode-entry','net-only-classification'):
            altered=copy.deepcopy(packet)
            if error=='missing-node':altered['nodes'].pop(0)
            elif error=='missing-internal-path':next(e for n in altered['nodes'] for e in n['parent_edges'] if e['paths'])['paths'].pop()
            elif error=='wrong-mode-entry':
                name=next(iter(altered['all_internal_versions']));altered['all_internal_versions'][name]['P1']=altered['all_internal_versions'][name]['P1'].replace('100644','100755')
            else:altered['source_qualification']['status']='MATCHED_EXISTING_QUALIFICATION'
            # Re-pin only this private fixture, so refusal must check actual
            # Git/provenance semantics rather than merely a changed file hash.
            freeze(altered)
            with self.assertRaises(m.Rejected):m.validate_3071_packet(self.repo,folder)

    def test_private_master_entry_binds_actual_B_S_T_and_original_wrapper_copy(self):
        from f13_native_p3_source_test import NativeSourceTest, native
        fixture=NativeSourceTest();fixture.setUp();self.addCleanup(fixture.doCleanups)
        # Private immutable native record and its ordinary qualifier; these
        # in-memory/disk fixture pins never authorize the real repository.
        source=Path(native.__file__).read_text()
        for key,value in fixture.pins.items():
            source=re.sub(r'(?m)^'+key+r' = .*$',key+' = '+repr(value),source)
        fixture.write(native.VERIFIER_PATH,source)
        fixture.commit('private exact native verifier pins');head=fixture.git('rev-parse','HEAD')
        target=fixture.git('commit-tree',head+'^{tree}','-p',fixture.base,'-p',head,'-m','private master two-parent carrier')
        fixture.git('checkout','-q',target)
        spec=importlib.util.spec_from_file_location('private_master_native',fixture.repo/native.VERIFIER_PATH)
        checker=importlib.util.module_from_spec(spec);spec.loader.exec_module(checker)
        binding=checker.qualify(fixture.repo,fixture.base,head,target,'pull_request','refs/pull/875/merge',target,fixture.base,'master')
        m=module();m.ROOT=fixture.repo
        capability=dict(collector_scope=dict(base=fixture.base,target=fixture.head))
        context=dict(GITHUB_EVENT_NAME='pull_request',GITHUB_REF='refs/pull/875/merge',GITHUB_SHA=target,
            VOICE_CI_PUSH_BEFORE_SHA=fixture.base,GITHUB_BASE_REF='master')
        entry=m.master_context(fixture.repo,capability,m.canonical(binding),context)
        self.assertEqual((entry['base'],entry['source'],entry['target']),(fixture.base,head,target))
        self.assertFalse(entry['admission'])
        for change in ('target','before','base'):
            wrong=dict(context);cap=json.loads(json.dumps(capability))
            if change=='target':wrong['GITHUB_SHA']=head
            elif change=='before':wrong.update(GITHUB_EVENT_NAME='push',GITHUB_REF='refs/heads/master',VOICE_CI_PUSH_BEFORE_SHA=head)
            else:cap['collector_scope']['base']=head
            with self.assertRaises((m.Rejected,native.Rejected,ValueError)):
                m.master_context(fixture.repo,cap,m.canonical(binding),wrong)
        source_map=self.folder/'source-map';output=self.folder/'wrapper-map';record=self.folder/'master-entry.json'
        source_map.write_bytes(b'actual full original rows\n');output.write_bytes(source_map.read_bytes());record.write_bytes(m.canonical(entry))
        result=m.master_wrapper_result(record,source_map,output)
        self.assertEqual(result['source_map'],result['output'])
        output.write_bytes(b'selected clean map\n')
        with self.assertRaises(m.Rejected):m.master_wrapper_result(record,source_map,output)

    def test_private_native_early_branch_consumes_full_transport_and_rejects_stale_map_context(self):
        from f13_native_p3_source_test import NativeSourceTest, native
        fixture=NativeSourceTest();fixture.setUp();self.addCleanup(fixture.doCleanups)
        for name in ('f13_f54_history.py','voice-r22-runtime-scope.sh'):
            fixture.write('scripts/ci/'+name,(ROOT/'scripts/ci'/name).read_text())
        fixture.commit('private observer inputs');fixture.carrier=fixture.git('rev-parse','HEAD')
        binding=fixture.qualify()
        self.repo=fixture.repo;self.base=fixture.base
        result,out=self.collect(target=fixture.carrier)
        self.assertEqual(result.returncode,0,result.stderr)
        m=module();trace=out/'trace.jsonl';trace_raw=trace.read_bytes()
        verified=m.validate_trace(self.repo,trace,m.sha(trace_raw))
        actual_map=out/'f13-contributions';actual_map.write_bytes((out/'map').read_bytes())
        paths=out/'native-paths';paths.write_text('\n'.join(binding['paths'])+'\n')
        native_binding=out/'native-binding.json';native_binding.write_bytes(m.canonical(binding))
        # Exact original producer rows, not invented eight-field Git-node rows.
        history=out/'f13-native-p3-history.tsv'
        history.write_text(''.join(x+'\n' for x in actual_map.read_text().splitlines() if x.split('\t')[0] in binding['paths']))
        with patch.multiple(native,**fixture.pins):
            proof=native.audit_map(self.repo,binding,actual_map,history)
        (out/'f13-native-p3-approval.json').write_bytes(m.canonical(proof))
        context_values=dict(ROOT=str(self.repo),TMP_DIR=str(out),base_sha=self.base,
            accepted_game_checkpoint='',game_checkpoint_enabled='false',game_checkpoint_aggregate_map='',
            native_p3_active='true',native_p3_paths=str(paths),native_p3_binding=str(native_binding),
            native_p3_paths_hash=fixture.git('hash-object',str(paths)),native_p3_binding_hash=fixture.git('hash-object',str(native_binding)),
            r23_allowlist_enabled='false',r22_runtime_delta='true')
        name='src/backend/voice/main.go';(out/'changed-files').write_text(name+'\n')
        context=m.policy_context(self.repo,actual_map,sum(([k,v] for k,v in context_values.items()),[]))
        context_file=out/'context.json';context_file.write_bytes(m.canonical(context))
        root=verified['ledger']['F1']
        # Isolate the transport binding with genuine generic producer evidence.
        # This is NOT an M collection: the real constructor must reject it.
        collection=dict(type='F54_SEALED_SUCCESSFUL_COLLECTION',admission=False,
            collection_trace=str(trace),collection_prefix=dict(bytes=len(trace_raw),sha256=m.sha(trace_raw)),
            source_context=m.router_source_context(self.repo),map_path=str(actual_map),map_binding=root['map_binding'],outer_rows=root['rows'],
            required_qualification=root['paths'],ledger=verified['ledger'],terminal_dispositions=[])
        collection_file=out/'private-transport.json';collection_file.write_bytes(m.canonical(collection))
        with self.assertRaises(m.Rejected):m.seal_router(self.repo,collection_file,m.sha(collection_file.read_bytes()),actual_map,context_file)
        seal=dict(type='F54_ORIGINAL_ROUTER_SEAL',collection_path=str(collection_file),collection_sha256=m.sha(collection_file.read_bytes()),
            complete_ledger_sha256=m.sha(m.canonical(collection)),context=context,map_path=str(actual_map),admission=False)
        seal_file=out/'private-seal.json';seal_file.write_bytes(m.canonical(seal));seal_sha=m.sha(seal_file.read_bytes())
        bound=m.router_binding(self.repo,seal_file,seal_sha,actual_map,context_file,name)
        self.assertTrue(bound['aggregate']);self.assertTrue(bound['obligations']['evidence'])
        # Exercise the real private worker against this same transport fixture.
        # It remains deliberately ineligible for successful M collection sealing.
        with tempfile.TemporaryDirectory(prefix='f54-ipc-') as ipc:
            socket_path=str(Path(ipc)/'s');session='a'*64
            worker=[sys.executable,str(HELPER),'router-worker',socket_path,session,str(self.repo),
                    str(seal_file),seal_sha,str(actual_map),str(context_file)]
            client=[sys.executable,str(HELPER),'router-request',socket_path,session]
            script='\n'.join([shlex.join(worker)+' &', 'child=$!',
                'trap \'kill "$child" 2>/dev/null || true; wait "$child" 2>/dev/null || true\' EXIT',
                'for i in {1..100}; do [[ ! -f '+shlex.quote(socket_path+'.ready')+' ]] || break; sleep .02; done',
                'value="$('+shlex.join(client+['runtime',name])+')"', '[[ "$value" == true ]]',
                shlex.join(client+['stop','']), 'wait "$child"', 'trap - EXIT'])
            result=subprocess.run(['bash','-euc',script],capture_output=True,text=True)
            self.assertEqual(result.returncode,0,result.stderr)
            self.assertFalse(Path(socket_path).exists())
            for artifact in (trace,actual_map,context_file,collection_file,seal_file):
                with self.subTest(worker_tamper=artifact.name):
                    original=artifact.read_bytes();stat=artifact.stat()
                    proc=subprocess.Popen(worker,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
                    try:
                        for _ in range(200):
                            if Path(socket_path+'.ready').exists() or proc.poll() is not None:break
                            time.sleep(.01)
                        status,output=m.router_worker_request(socket_path,session,'bind',name)
                        self.assertEqual(status,0);self.assertEqual(json.loads(output),bound)
                        # Same length and restored mtime must still invalidate.
                        changed=bytearray(original);changed[len(changed)//2]^=1
                        artifact.write_bytes(changed);os.utime(artifact,ns=(stat.st_atime_ns,stat.st_mtime_ns))
                        with self.assertRaises((m.Rejected,ValueError,OSError)):
                            m.router_worker_request(socket_path,session,'bind',name)
                        proc.communicate(timeout=10);self.assertNotEqual(proc.returncode,0)
                    finally:
                        artifact.write_bytes(original)
                        if proc.poll() is None:proc.kill()
                        proc.communicate(timeout=10)
            # An abandoned client is a failed invocation, never stale success.
            proc=subprocess.Popen(worker,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
            try:
                for _ in range(200):
                    if Path(socket_path+'.ready').exists() or proc.poll() is not None:break
                    time.sleep(.01)
                with socket.socket(socket.AF_UNIX,socket.SOCK_STREAM) as abandoned:
                    abandoned.connect(socket_path)
                proc.communicate(timeout=10);self.assertNotEqual(proc.returncode,0)
            finally:
                if proc.poll() is None:proc.kill()
                proc.communicate(timeout=10)
            for mutation in ('event','source','dead-worker'):
                with self.subTest(worker_freshness=mutation):
                    original_context=context_file.read_bytes();head=git(self.repo,'rev-parse','HEAD')
                    proc=subprocess.Popen(worker,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
                    try:
                        for _ in range(200):
                            if Path(socket_path+'.ready').exists() or proc.poll() is not None:break
                            time.sleep(.01)
                        self.assertEqual(m.router_worker_request(socket_path,session,'bind',name)[0],0)
                        if mutation=='event':
                            changed=json.loads(original_context);changed['event']['GITHUB_SHA']='f'*40
                            context_file.write_bytes(m.canonical(changed))
                        elif mutation=='source':
                            self.write('future-source.txt','future source changes the tree\n')
                            self.commit('private future source must invalidate worker')
                        else:
                            proc.kill();proc.communicate(timeout=10)
                        with self.assertRaises((m.Rejected,ValueError,OSError)):
                            m.router_worker_request(socket_path,session,'bind',name)
                        proc.communicate(timeout=10);self.assertNotEqual(proc.returncode,0)
                    finally:
                        context_file.write_bytes(original_context)
                        if mutation=='source':git(self.repo,'checkout','-q',head)
                        if proc.poll() is None:proc.kill()
                        proc.communicate(timeout=10)
                        # SIGKILL cannot perform the owned socket cleanup.
                        Path(socket_path).unlink(missing_ok=True)
                        Path(socket_path+'.ready').unlink(missing_ok=True)
        actual=m.execute_original_router(context,bound)
        self.assertEqual((actual['gate'],actual['status']),('native_p3_path_allowed_for_provenance',0))
        # The unchanged original native predicate alone still accepts its row;
        # the top seal must catch missing evidence before that early continue.
        original_collection=collection_file.read_bytes();collection['required_qualification'][name]['evidence']=[]
        collection_file.write_bytes(m.canonical(collection))
        with self.assertRaises(m.Rejected):m.router_binding(self.repo,seal_file,seal_sha,actual_map,context_file,name)
        collection_file.write_bytes(original_collection)
        original_map=actual_map.read_bytes();actual_map.write_bytes(original_map+b'forged\tfalse\towner\tparent\tbase\thead\tcontent\tbest\n')
        with self.assertRaises(m.Rejected):m.router_binding(self.repo,seal_file,seal_sha,actual_map,context_file,name)
        actual_map.write_bytes(original_map)
        changed_context=json.loads(context_file.read_bytes());changed_context['values']['r23_allowlist_enabled']='true'
        context_file.write_bytes(m.canonical(changed_context))
        with self.assertRaises(m.Rejected):m.router_binding(self.repo,seal_file,seal_sha,actual_map,context_file,name)

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
        # A qualified replacement must consume the same pending transition;
        # a true sidecar bit or receipt from another operation is insufficient.
        exact=dict(operation=op,path=name,ledger_sha256=m.sha(m.canonical(ready)),
            incoming=m.raw_binding(incoming),prior_rows=[m.raw_binding(prior)],
            map_before=before['payload']['map_before'],aggregate=True)
        original_trace=trace.read_bytes()
        event=dict(kind='exact_unit_supersession',operation=op,payload=dict(path=name,receipt=exact))
        trace.write_bytes(original_trace+json.dumps(event).encode()+b'\n')
        replacement=m.consume_M_path(ledger,digest,trace,name,(prior+'\n').encode(),incoming,'replace')
        self.assertEqual(replacement['exact_unit_supersession'],exact)
        self.assertTrue(replacement['aggregate']);self.assertEqual(replacement['after_gate_row']['raw'],incoming)
        for field,value in (('operation','F999/'+m.M+'/2'),('path',name+'.future'),
                ('ledger_sha256','0'*64),('aggregate',False),('prior_rows',[])):
            altered=dict(exact);altered[field]=value
            event['payload']['receipt']=altered;trace.write_bytes(original_trace+json.dumps(event).encode()+b'\n')
            with self.assertRaises(m.Rejected):m.consume_M_path(ledger,digest,trace,name,(prior+'\n').encode(),incoming,'replace')

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

    def collect(self, target=None, cross_scope=False, partition=True, allow=True, origin=None, origin_kind='span_link'):
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
        if origin is not None:
            if origin_kind=='J_prior_span_link':
                origin_path=out/'fixture-origin.json';origin_path.write_text(json.dumps(origin))
                program+='f13_f54_trace_event J_prior_span_link "$ROOT" '+shlex.quote(str(origin_path))+'\n'
            else:
                self.assertEqual(origin_kind,'span_link')
                program+='f13_f54_trace_event span_link "$ROOT" '+shlex.quote(json.dumps(origin))+'\n'
        program += f'f13_collect_develop_contributions "$ROOT" {self.base} {target or git(self.repo,"rev-parse","HEAD")} "$TMP_DIR/map" "$TMP_DIR/category" {str(allow).lower()} {str(partition).lower()}\n'
        if cross_scope: program += 'f13_cross_scope_path_allowed ' + shlex.quote(PROTO) + ' true\n'
        # A complete inherited origin plus the real extracted shell can exceed
        # Linux's per-argument limit; preserve the program verbatim as a file.
        script=out/'collect.sh';script.write_bytes(program.encode())
        p = subprocess.run(['bash', str(script)], capture_output=True, text=True)
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

    def current_author_span_fixture(self):
        # Reuse the genuine complete-M/current native fixture, then create a
        # distinct real aggregate with exactly two internal direct operations.
        m,unit,_,context,old,_=self.current_author_J_fixture()
        from f13_native_p3_source_test import native
        base=git(self.repo,'rev-parse','HEAD')
        self.write('docs/span-first.md','first lane\n');first_lane=self.commit('span first lane')
        git(self.repo,'checkout','-qb','current-author-span',base)
        name='protos/voice/span/v1/span.proto'
        self.write(name,'first actual source\n');first=self.commit('span first actual node')
        self.write('docs/span-side.md','second actual source\n');head=self.commit('span second actual node')
        git(self.repo,'checkout','-q',first_lane);git(self.repo,'merge','--no-ff','-qm','span actual aggregate',head)
        aggregate=git(self.repo,'rev-parse','HEAD')
        self.base=base
        p,original=self.collect(partition=False);self.assertEqual(p.returncode,0,p.stderr)
        original_raw=(original/'trace.jsonl').read_bytes()
        m.SPAN_BASE,m.SPAN_HEAD,m.SPAN_AGGREGATE,m.SPAN_NODES=base,head,aggregate,[first,head]
        m.ORIGINAL_TRACE_SHA=m.sha(original_raw);m.ORIGINAL_PRODUCER_SHA=m.classifier_binding()['producer_sha256']
        old_path=self.folder/'span-historical-native.json';old_path.write_bytes(m.canonical(old));m.ORIGINAL_NATIVE_SHA=m.sha(old_path.read_bytes())
        m.AUTHOR_REFERENCE=aggregate;m.AUTHOR_REFERENCE_TREE=git(self.repo,'rev-parse',aggregate+'^{tree}')
        m.AUTHOR_REFERENCE_DELTA_SHA=m.sha(m.git(self.repo,'diff','--raw','--no-abbrev','--no-renames',m.PROTOTYPE_BASE,aggregate))
        target=git(self.repo,'commit-tree',aggregate+'^{tree}','-p',m.AUTHOR_EVENT_BASE,'-p',aggregate,'-m','actual current span target')
        git(self.repo,'checkout','-q',target);context['GITHUB_SHA']=target
        spec=importlib.util.spec_from_file_location('private_current_span_native',self.repo/native.VERIFIER_PATH)
        authority=importlib.util.module_from_spec(spec);spec.loader.exec_module(authority)
        current=authority.qualify(self.repo,m.AUTHOR_EVENT_BASE,aggregate,target,'pull_request',context['GITHUB_REF'],target,m.AUTHOR_EVENT_BASE,'master')
        current_path=self.folder/'span-current-native.json';current_path.write_bytes(m.canonical(current))
        capability=self.folder/'span-author-capability.json'
        capability.write_bytes(m.canonical(m.prepare_author_capabilities(self.repo,unit)))
        return m,original,old_path,current_path,capability,context,name,old,current

    def test_current_author_span_separates_three_bases_and_reconstructs_link(self):
        m,original,old_path,current_path,capability,context,name,old,current=self.current_author_span_fixture()
        self.assertEqual(len({m.AUTHOR_HISTORICAL_SCOPE['base'],m.AUTHOR_NATIVE_CONTENT_BASE,m.AUTHOR_EVENT_BASE}),3)
        original_raw=(original/'trace.jsonl').read_bytes();old_raw=old_path.read_bytes()
        with patch.dict('os.environ',context):
            # Before the adapter exists, exercise the actual old refusing
            # predicate, not a missing-symbol/signature setup failure.
            origin=(m.span_author_origin(self.repo,original/'trace.jsonl',old_path,current_path,capability)
                if hasattr(m,'span_author_origin') else m.span_origin(self.repo,original/'trace.jsonl',old_path,current_path))
            self.assertEqual(origin['type'],'F54_FIXED_CURRENT_AUTHOR_AGGREGATE_SPAN_OBSERVATION')
            p,span=self.collect(target=m.SPAN_HEAD,allow=False,origin=origin);self.assertEqual(p.returncode,0,p.stderr)
            result=m.validate_aggregate_span(self.repo,original/'trace.jsonl',span/'trace.jsonl',m.sha((span/'trace.jsonl').read_bytes()),old_path,current_path)
        self.assertEqual([x['node'] for x in result['dispositions']],m.SPAN_NODES)
        self.assertFalse(m.consume_span_aggregate(result,name,'false')['aggregate'])
        self.assertEqual((original/'trace.jsonl').read_bytes(),original_raw);self.assertEqual(old_path.read_bytes(),old_raw)
        self.assertNotEqual(old['record'],current['record']);self.assertNotEqual(old['base'],current['base'])
        # No implicit adaptation of legacy callers when bindings differ.
        with self.assertRaisesRegex(m.Rejected,'unchanged_inherited_native_pins'):
            m.span_origin(self.repo,original/'trace.jsonl',old_path,current_path)

    def test_current_author_span_rejects_rehashed_authority_and_real_tree_entry_forgery(self):
        m,original,old_path,current_path,capability,context,name,old,current=self.current_author_span_fixture()
        saved=capability.read_bytes()
        with patch.dict('os.environ',context):
            origin=m.span_author_origin(self.repo,original/'trace.jsonl',old_path,current_path,capability)
            p,span=self.collect(target=m.SPAN_HEAD,allow=False,origin=origin);self.assertEqual(p.returncode,0,p.stderr)
            events=[json.loads(x) for x in (span/'trace.jsonl').read_bytes().splitlines()]
            for change in ('reference','carrier','historical_base','native_base','event_base','path','mode','oid','runtime','type'):
                altered=json.loads(saved)
                app=altered['author_applicability']
                if change=='reference':app['reference']=m.SPAN_BASE
                elif change=='carrier':app['current_carrier']=m.SPAN_AGGREGATE
                elif change=='historical_base':app['historical_collector']['base']=m.AUTHOR_EVENT_BASE
                elif change=='native_base':app['native_content_baseline']=m.AUTHOR_EVENT_BASE
                elif change=='event_base':app['event_base']=m.AUTHOR_NATIVE_CONTENT_BASE
                elif change=='path':app['reference_delta'].pop()
                elif change in ('mode','oid'):
                    # Mutate the actual serialized Git representation, not an
                    # unused extra key in the entry dictionary (old c54 limit).
                    item=next(x for x in app['reference_delta'] if x['current_entry'])
                    raw=item['current_entry']
                    item['current_entry']=(re.sub(r'^100644', '100755', raw) if change=='mode'
                        else re.sub(r'[0-9a-f]{40}', '0'*40, raw, count=1))
                    self.assertNotEqual(item['current_entry'],raw)
                elif change=='runtime':app['runtime_pass']=True
                else:altered['type']='F54_FIXED_COMPLETE_SOURCE_CAPABILITIES_PROTOTYPE'
                capability.write_bytes(m.canonical(altered))
                forged=json.loads(json.dumps(events))
                forged[0]['payload']['author_capability']=dict(bytes=len(capability.read_bytes()),sha256=m.sha(capability.read_bytes()))
                path,digest=self.forged_trace(m,span,forged,'author-certificate-'+change+'.jsonl')
                with self.subTest(certificate=change),self.assertRaises(m.Rejected):
                    m.validate_aggregate_span(self.repo,original/'trace.jsonl',path,digest,old_path,current_path)
                capability.write_bytes(saved)
            for change in ('base','content_base','source','target','record','record_sha256','snapshot','paths'):
                altered=dict(current)
                altered[change]=[] if change=='paths' else '0'*(64 if change=='record_sha256' else 40)
                current_path.write_bytes(m.canonical(altered))
                forged=json.loads(json.dumps(events));forged[0]['payload']['current_native']=altered
                forged[0]['payload']['current_native_sha256']=m.sha(current_path.read_bytes())
                path,digest=self.forged_trace(m,span,forged,'current-native-'+change+'.jsonl')
                with self.subTest(native=change),self.assertRaises(m.Rejected):
                    m.validate_aggregate_span(self.repo,original/'trace.jsonl',path,digest,old_path,current_path)
            current_path.write_bytes(m.canonical(current))
            for key in ('author_capability','master_entry','native_applicability'):
                forged=json.loads(json.dumps(events));forged[0]['payload'][key]['sha256']='0'*64
                path,digest=self.forged_trace(m,span,forged,'forged-reconstructed-'+key+'.jsonl')
                with self.subTest(binding=key),self.assertRaisesRegex(m.Rejected,'exact_observed_aggregate_link'):
                    m.validate_aggregate_span(self.repo,original/'trace.jsonl',path,digest,old_path,current_path)
            with patch.dict('os.environ',dict(context,GITHUB_SHA=m.SPAN_AGGREGATE)),self.assertRaises(m.Rejected):
                m.validate_aggregate_span(self.repo,original/'trace.jsonl',span/'trace.jsonl',m.sha((span/'trace.jsonl').read_bytes()),old_path,current_path)

    def test_current_author_span_keeps_original_owner_content_map_and_or_obligations(self):
        m,original,old_path,current_path,capability,context,name,old,current=self.current_author_span_fixture()
        with patch.dict('os.environ',context):
            origin=m.span_author_origin(self.repo,original/'trace.jsonl',old_path,current_path,capability)
            p,span=self.collect(target=m.SPAN_HEAD,allow=False,origin=origin);self.assertEqual(p.returncode,0,p.stderr)
            events=[json.loads(x) for x in (span/'trace.jsonl').read_bytes().splitlines()]
            for change in ('owner','source','content','rawflag','frame','missing_operation','historical_origin','missing_author_capability'):
                forged=json.loads(json.dumps(events))
                if change in ('owner','source','content','rawflag'):
                    index={'owner':2,'source':5,'content':6,'rawflag':1}[change]
                    for event in forged:
                        if event['kind'] in ('row_after','row_committed') and event['payload']['path']==name:
                            parts=event['payload']['row']['raw'].split('\t');parts[index]='true' if change=='rawflag' else '0'*40
                            event['payload']['row']=m.raw_binding('\t'.join(parts))
                elif change=='frame':next(x for x in forged if x['kind']=='frame_enter')['payload']['base']=m.AUTHOR_EVENT_BASE
                elif change=='missing_operation':forged=[x for x in forged if x.get('operation') is None or m.SPAN_NODES[0] not in x['operation']]
                elif change=='historical_origin':forged[0]['payload']['original_trace_sha256']='0'*64
                else:forged[0]['payload'].pop('author_capability_path')
                path,digest=self.forged_trace(m,span,forged,'author-ledger-'+change+'.jsonl')
                with self.subTest(ledger=change),self.assertRaises(m.Rejected):
                    m.validate_aggregate_span(self.repo,original/'trace.jsonl',path,digest,old_path,current_path)
            result=m.validate_aggregate_span(self.repo,original/'trace.jsonl',span/'trace.jsonl',m.sha((span/'trace.jsonl').read_bytes()),old_path,current_path)
            with self.assertRaisesRegex(m.Rejected,'aggregate_span_consumer_must_keep_internal_taint'):
                m.consume_span_aggregate(result,name,'true')

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
