"""Real Git event boundary tests for the current release successor."""
import subprocess
import tempfile
import unittest
import json
import hashlib
import io
import os
import sys
import re
import signal
import time
from contextlib import redirect_stdout, redirect_stderr
from pathlib import Path
from unittest.mock import patch

import f13_release_source as release


class BlobDigestBatchTest(unittest.TestCase):
    def test_binary_blobs_and_missing_or_non_blob_are_checked_each_time(self):
        with tempfile.TemporaryDirectory() as folder:
            repo = Path(folder)
            subprocess.run(['git', 'init', '-q', str(repo)], check=True)
            def obj(raw, kind='blob'):
                return subprocess.check_output(['git', '-C', str(repo), 'hash-object', '-w', '-t', kind, '--stdin'], input=raw).decode().strip()
            first, empty = obj(b'one\x00two\n'), obj(b'')
            self.assertEqual(release.blob_digests(repo, [first, empty, first]), [
                hashlib.sha256(b'one\x00two\n').hexdigest(),
                hashlib.sha256(b'').hexdigest(),
                hashlib.sha256(b'one\x00two\n').hexdigest(),
            ])
            with self.assertRaises(release.Rejected):
                release.blob_digests(repo, [first, '0' * 40])
            tree = subprocess.check_output(['git', '-C', str(repo), 'mktree'], input=b'').decode().strip()
            with self.assertRaises(release.Rejected):
                release.blob_digests(repo, [tree])
            loose = repo / '.git' / 'objects' / first[:2] / first[2:]
            loose.chmod(0o600)
            loose.unlink()
            with self.assertRaises(release.Rejected):
                release.blob_digests(repo, [first])

    def test_more_than_one_batch_and_malformed_response(self):
        with tempfile.TemporaryDirectory() as folder:
            repo = Path(folder)
            subprocess.run(['git', 'init', '-q', str(repo)], check=True)
            values = [str(i).encode() for i in range(35)]
            ids = [subprocess.check_output(['git', '-C', str(repo), 'hash-object', '-w', '--stdin'], input=value).decode().strip() for value in values]
            self.assertEqual(release.blob_digests(repo, ids), [hashlib.sha256(value).hexdigest() for value in values])
            def truncated(*args, **kwargs):
                kwargs['stdout'].write((ids[0] + ' blob 5\nabc').encode())
                return subprocess.CompletedProcess([], 0)
            with patch.object(release.subprocess, 'run', side_effect=truncated):
                with self.assertRaises(release.Rejected):
                    release.blob_digests(repo, [ids[0]])
            for payload, status in [
                ((ids[1] + ' blob 1\n0\n').encode(), 0),
                ((ids[0] + ' blob 1\n0\nextra').encode(), 0),
                (b'', 1),
            ]:
                def bad_response(*args, **kwargs):
                    kwargs['stdout'].write(payload)
                    return subprocess.CompletedProcess([], status)
                with patch.object(release.subprocess, 'run', side_effect=bad_response):
                    with self.assertRaises(release.Rejected):
                        release.blob_digests(repo, [ids[0]])


class EventBoundaryTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = Path(self.tmp.name)
        self.git('init', '-q')
        self.git('config', 'user.name', 'fixture')
        self.git('config', 'user.email', 'fixture@example.invalid')
        self.git('config', 'core.autocrlf', 'false')
        (self.repo / 'source').write_text('source\n')
        for name in ('docs/context.md','src/backend/pkg/dep.go','src/backend/role/dep.go','src/backend/voice/main.go',
                     'scripts/ci/testdata/f13-exact-j-547/historical-inputs.zip'):
            path=self.repo/name
            path.parent.mkdir(parents=True,exist_ok=True)
            path.write_text('original\n')
        verifier = self.repo / 'scripts/ci/f13_release_source.py'
        verifier.parent.mkdir(parents=True,exist_ok=True)
        verifier.write_text('old\n')
        self.git('add', '.')
        self.git('commit', '-qm', 'H')
        self.h = self.git('rev-parse', 'HEAD')
        base_patch = patch.object(release, 'COLLECTION_BASE', self.h)
        base_patch.start()
        self.addCleanup(base_patch.stop)
        (self.repo / 'record').write_text('record\n')
        self.git('add', 'record')
        self.git('commit', '-qm', 'R')
        self.r = self.git('rev-parse', 'HEAD')
        (self.repo / 'carrier').write_text('carrier\n')
        verifier.write_text('new\n')
        self.git('add', '.')
        self.git('commit', '-qm', 'K')
        self.k = self.git('rev-parse', 'HEAD')
        tree = self.git('rev-parse', self.k + '^{tree}')
        self.t = self.git('commit-tree', tree, '-p', self.h, '-p', self.k, input='event\n')

    def tearDown(self):
        self.tmp.cleanup()

    def git(self, *args, input=None):
        return subprocess.check_output(['git', '-C', str(self.repo), *args],
                                       input=input, text=True).strip()

    def event(self, base=None, source=None, target=None, before=None):
        return release.check_event(self.repo, base or self.h, source or self.k,
            target or self.t, 'push', 'refs/heads/master', target or self.t,
            before or self.h, '', self.h, self.r)

    def test_real_master_event_and_negative_parents_and_result(self):
        self.event()
        with self.assertRaises(release.Rejected):
            self.event(before=self.r)
        with self.assertRaises(release.Rejected):
            self.event(source=self.r)
        wrong_tree = self.git('rev-parse', self.h + '^{tree}')
        wrong = self.git('commit-tree', wrong_tree, '-p', self.h, '-p', self.k, input='wrong\n')
        with self.assertRaises(release.Rejected):
            self.event(target=wrong)

    def test_two_real_post_foundation_events_preserve_actual_base(self):
        base = self.t
        for name in ('.github/ci/full-application-build-request.json', 'scripts/dev/media_initial_admission.py'):
            self.git('checkout','--detach','-q',base)
            path = self.repo/name
            path.parent.mkdir(parents=True,exist_ok=True)
            path.write_text('outside-unit\n')
            self.git('add',name)
            self.git('commit','-qm','outside unit contribution')
            source = self.git('rev-parse','HEAD')
            target = self.git('commit-tree',self.git('rev-parse',source+'^{tree}'),'-p',base,'-p',source,input='event\n')
            info = release.check_event(self.repo,base,source,target,'push','refs/heads/master',
                target,base,'',self.h,self.r)
            self.assertEqual(info,dict(base=self.h,source=self.k,target=self.t))
            base = target

    def test_continuation_rejects_wrong_actual_event_and_foreign_history(self):
        self.git('checkout','--detach','-q',self.t)
        (self.repo/'marker').write_text('marker\n')
        self.git('add','marker')
        self.git('commit','-qm','marker')
        source=self.git('rev-parse','HEAD')
        target=self.git('commit-tree',self.git('rev-parse',source+'^{tree}'),'-p',self.t,'-p',source,input='event\n')
        with self.assertRaises(release.Rejected):
            release.check_event(self.repo,self.t,source,target,'push','refs/heads/master',target,self.h,'',self.h,self.r)
        with self.assertRaises(release.Rejected):
            release.check_event(self.repo,self.t,self.k,target,'push','refs/heads/master',target,self.t,'',self.h,self.r)
        detached=self.git('commit-tree',self.git('rev-parse',self.k+'^{tree}'),'-p',self.k,input='not based on F\n')
        detached_event=self.git('commit-tree',self.git('rev-parse',detached+'^{tree}'),'-p',self.t,
            '-p',detached,input='wrong source\n')
        with self.assertRaisesRegex(release.Rejected,'git_merge-base'):
            release.check_event(self.repo,self.t,detached,detached_event,'push','refs/heads/master',
                detached_event,self.t,'',self.h,self.r)
        foreign=self.git('commit-tree',self.git('rev-parse',source+'^{tree}'),'-p',self.h,input='side\n')
        merged=self.git('commit-tree',self.git('rev-parse',source+'^{tree}'),'-p',source,'-p',foreign,input='side merge\n')
        wrong=self.git('commit-tree',self.git('rev-parse',merged+'^{tree}'),'-p',self.t,'-p',merged,input='event\n')
        # A later side branch predating F must never be treated as a continuation.
        with patch.object(release,'FOUNDATION_BASE',self.h,create=True), patch.object(release,'RECORD_COMMIT',self.r):
            with self.assertRaisesRegex(release.Rejected,'git_merge-base'):
                release.check_continuation(self.repo,dict(base=self.h,source=self.k,target=self.t),merged,
                    dict(projection=[],context=[],compiled_closure=[],release_record={'closure':{'directory_inventories':[]}}))

    def test_first_foundation_must_be_exact_merge_with_record_carrier(self):
        tree=self.git('rev-parse',self.k+'^{tree}')
        nonmerge=self.git('commit-tree',tree,'-p',self.h,input='not foundation merge\n')
        source=self.git('commit-tree',tree,'-p',nonmerge,input='source\n')
        target=self.git('commit-tree',tree,'-p',nonmerge,'-p',source,input='target\n')
        with self.assertRaisesRegex(release.Rejected,'release_foundation_ordered_parents'):
            release.check_event(self.repo,nonmerge,source,target,'push','refs/heads/master',target,nonmerge,'',self.h,self.r)
        missing=self.git('commit-tree',tree,'-p',self.h,input='carrier missing R\n')
        first=self.git('commit-tree',tree,'-p',self.h,'-p',missing,input='false foundation\n')
        source=self.git('commit-tree',tree,'-p',first,input='source\n')
        target=self.git('commit-tree',tree,'-p',first,'-p',source,input='target\n')
        with self.assertRaisesRegex(release.Rejected,'git_merge-base'):
            release.check_event(self.repo,first,source,target,'push','refs/heads/master',target,first,'',self.h,self.r)

    def test_continuation_rejects_shallow_history(self):
        (self.repo/'.git/shallow').write_text(self.h+'\n')
        self.assertEqual(self.git('rev-parse','--is-shallow-repository'),'true')
        with self.assertRaisesRegex(release.Rejected,'release_shallow_history'):
            self.event()

    def test_post_foundation_snapshots_reject_drift_and_restoration(self):
        record=dict(projection=[dict(path='source')],context=[dict(path='docs/context.md')],
            compiled_closure=['src/backend/pkg/dep.go'],
            release_record={'closure':{'directory_inventories':[dict(directory='src/backend/pkg',entries={})]}})
        variants=('projection','context','compiled','projection_delete','compiled_mode','compiled_type','directory','voice','proof','carrier',
                  'proof_mode','carrier_mode','projection_restore','context_restore','compiled_restore',
                  'directory_restore','voice_restore','proof_restore','carrier_restore')
        for variant in variants:
            with self.subTest(variant=variant):
                self.git('checkout','--detach','-q',self.t)
                paths={'projection':'source','context':'docs/context.md','compiled':'src/backend/pkg/dep.go',
                    'projection_delete':'source','compiled_mode':'src/backend/pkg/dep.go','compiled_type':'src/backend/pkg/dep.go',
                    'directory':'src/backend/pkg/added.go','voice':'src/backend/voice/added.go',
                    'proof':'record','carrier':'carrier','proof_mode':'record','carrier_mode':'carrier',
                    'projection_restore':'source','context_restore':'docs/context.md',
                    'compiled_restore':'src/backend/pkg/dep.go','directory_restore':'src/backend/pkg/added.go',
                    'voice_restore':'src/backend/voice/added.go',
                    'proof_restore':'record','carrier_restore':'carrier'}
                name=paths[variant]
                path=self.repo/name
                if variant == 'projection_delete':
                    path.unlink()
                    self.git('add','-A')
                elif variant == 'compiled_type':
                    blob=self.git('hash-object','-w','--stdin',input='symlink target')
                    self.git('update-index','--cacheinfo','120000',blob,name)
                    path.unlink()
                    path.symlink_to('symlink target')
                elif variant.endswith('_mode'):
                    path.chmod(path.stat().st_mode | 0o111)
                    self.git('update-index','--chmod=+x',name)
                else:
                    path.parent.mkdir(parents=True,exist_ok=True)
                    path.write_text('changed\n')
                    self.git('add','-A')
                self.git('commit','-qm','drift')
                if variant.endswith('_restore'):
                    if name.endswith('/added.go'):
                        path.unlink()
                    else:
                        self.git('checkout',self.t,'--',name)
                    self.git('add','-A')
                    self.git('commit','-qm','restore')
                source=self.git('rev-parse','HEAD')
                with patch.multiple(release,FOUNDATION_BASE=self.h,RECORD_COMMIT=self.r,
                    CARRIER_PATHS=frozenset({'carrier','scripts/ci/f13_release_source.py'}),create=True):
                    reason=('release_continuation_directory_drift' if variant.startswith('directory') else
                            'release_continuation_voice_drift' if variant.startswith('voice') else 'release_continuation_entry_drift')
                    with self.assertRaisesRegex(release.Rejected,reason):
                        release.check_continuation(self.repo,dict(base=self.h,source=self.k,target=self.t),source,record)

    def test_carrier_context_is_bound_to_k_but_runtime_overlap_is_rejected(self):
        carrier_path='scripts/ci/f13_release_source.py'
        record=dict(projection=[dict(path='source')],context=[dict(path=carrier_path)],
            compiled_closure=['src/backend/pkg/dep.go'],
            release_record={'closure':{'directory_inventories':[dict(directory='src/backend/pkg',entries={})]}})
        foundation=dict(base=self.h,source=self.k,target=self.t)
        with patch.multiple(release,FOUNDATION_BASE=self.h,RECORD_COMMIT=self.r,
                CARRIER_PATHS=frozenset({'carrier',carrier_path})):
            release.check_continuation(self.repo,foundation,self.k,record)
        with patch.multiple(release,FOUNDATION_BASE=self.h,RECORD_COMMIT=self.r,
                CARRIER_PATHS=frozenset({'source',carrier_path})):
            with self.assertRaisesRegex(release.Rejected,'release_carrier_runtime_overlap'):
                release.check_continuation(self.repo,foundation,self.k,record)

    def test_post_foundation_side_branch_mutation_then_restore_is_rejected(self):
        self.git('checkout','--detach','-q',self.t)
        (self.repo/'source').write_text('side mutation\n')
        self.git('add','source')
        self.git('commit','-qm','side mutation')
        mutation=self.git('rev-parse','HEAD')
        self.git('checkout',self.t,'--','source')
        self.git('commit','-qm','side restore')
        restored=self.git('rev-parse','HEAD')
        self.git('checkout','--detach','-q',self.t)
        (self.repo/'marker').write_text('outside unit\n')
        self.git('add','marker')
        self.git('commit','-qm','clean main')
        main=self.git('rev-parse','HEAD')
        merged=self.git('commit-tree',self.git('rev-parse',main+'^{tree}'),'-p',main,'-p',restored,
                        input='merge side\n')
        for revision in (mutation,restored,main,merged):
            self.git('merge-base','--is-ancestor',self.t,revision)
        self.assertEqual(release.legacy.entry(self.repo,merged,'source'),
                         release.legacy.entry(self.repo,self.t,'source'))
        record=dict(projection=[dict(path='source')],context=[dict(path='docs/context.md')],
            compiled_closure=['src/backend/pkg/dep.go'],
            release_record={'closure':{'directory_inventories':[dict(directory='src/backend/pkg',entries={})]}})
        with patch.multiple(release,FOUNDATION_BASE=self.h,RECORD_COMMIT=self.r,
                CARRIER_PATHS=frozenset({'carrier','scripts/ci/f13_release_source.py'})):
            with self.assertRaisesRegex(release.Rejected,'release_continuation_entry_drift'):
                release.check_continuation(self.repo,dict(base=self.h,source=self.k,target=self.t),merged,record)

    def test_unreviewed_carrier_delta_is_rejected(self):
        with patch.multiple(release, RECORD_COMMIT=self.r, ROOT=self.repo,
                            CARRIER_PATHS=frozenset({'carrier','scripts/ci/f13_release_source.py'})):
            release._carrier(self.repo,self.k)
            (self.repo/'unreviewed').write_text('unreviewed\n')
            self.git('add','unreviewed')
            self.git('commit','-qm','unreviewed')
            with self.assertRaises(release.Rejected):
                release._carrier(self.repo,self.git('rev-parse','HEAD'))

    def test_record_commit_may_only_add_named_evidence(self):
        native = dict(delta_review=dict(path='record'),evidence=[])
        production = dict(current_inputs=dict(path='record'),evidence=[])
        with patch.multiple(release, FOUNDATION_BASE=self.h, RECORD_COMMIT=self.r,
                            RECORD_PATH='record', PRODUCTION_RECORD_PATH='record'):
            release._record_delta(self.repo,native,production)
            (self.repo/'source').write_text('tampered\n')
            self.git('add','source')
            self.git('commit','-qm','tamper')
            with patch.object(release,'RECORD_COMMIT',self.git('rev-parse','HEAD')):
                with self.assertRaises(release.Rejected):
                    release._record_delta(self.repo,native,production)

    def test_native_requires_actual_root_and_checked_out_event(self):
        fake_record = dict(base=self.h,projection=[])
        with patch.multiple(release,FOUNDATION_BASE=self.h,RECORD_COMMIT=self.r,ROOT=self.repo), \
             patch.object(release,'production_record',return_value={}), \
             patch.object(release,'load_native',return_value=fake_record), patch.object(release,'load_role'), patch.object(release,'_carrier'), \
             patch.object(release,'check_continuation'):
            args = (self.repo,self.h,self.k,self.t,'push','refs/heads/master',self.t,self.h,'')
            with self.assertRaises(release.Rejected): release.native_qualify(*args)
            self.git('checkout','--detach','-q',self.t)
            release.native_qualify(*args)
            with patch.object(release,'ROOT',self.repo/'other'):
                with self.assertRaises(release.Rejected): release.native_qualify(*args)

    def test_collection_base_must_be_on_first_parent_chain(self):
        tree = self.git('rev-parse',self.h+'^{tree}')
        side = self.git('commit-tree',tree,'-p',self.h,input='side\n')
        head = self.git('commit-tree',tree,'-p',self.h,'-p',side,input='event base\n')
        record = self.git('commit-tree',tree,'-p',head,input='record\n')
        carrier = self.git('commit-tree',tree,'-p',record,input='carrier\n')
        target = self.git('commit-tree',tree,'-p',head,'-p',carrier,input='target\n')
        with patch.object(release,'COLLECTION_BASE',side):
            with self.assertRaises(release.Rejected):
                release.check_event(self.repo,head,carrier,target,'push','refs/heads/master',
                                    target,head,'',head,record)

    def test_historical_records_and_archive_remain_immutable(self):
        paths = ('docs/testing/native-p3-source-v4.json','docs/testing/f13-production-v10-evidence/data.json',
                 'scripts/ci/testdata/f13-exact-j-547/historical-inputs.zip')
        for name in paths:
            path = self.repo/name
            path.parent.mkdir(parents=True,exist_ok=True)
            path.write_text('original\n')
        self.git('add','.')
        self.git('commit','-qm','protected baseline')
        original = self.git('rev-parse','HEAD')
        release.preserve_predecessor(self.repo,original,original,paths[-1])
        (self.repo/paths[0]).write_text('forged\n')
        (self.repo/paths[1]).unlink()
        self.git('add','-A')
        self.git('commit','-qm','forged historical proof')
        changed = self.git('rev-parse','HEAD')
        with self.assertRaises(release.Rejected):
            release.preserve_predecessor(self.repo,original,changed,paths[-1])
        self.git('checkout','--detach','-q',original)
        self.git('update-index','--chmod=+x',paths[-1])
        self.git('commit','-qm','archive mode')
        changed_mode = self.git('rev-parse','HEAD')
        # Comparing against an unchanged baseline separately isolates mode drift.
        baseline_entry = release.legacy.entry(self.repo,original,paths[-1])
        self.assertNotEqual(baseline_entry,release.legacy.entry(self.repo,changed_mode,paths[-1]))
        with self.assertRaises(release.Rejected):
            release.preserve_predecessor(self.repo,original,changed_mode,paths[-1])


    def test_role_unit_drift_and_restoration_is_rejected_at_each_commit(self):
        role_path = 'src/backend/role/dep.go'
        role = dict(required_current_source=[dict(path=role_path)], context=[],
                    closure={'directory_inventories': [dict(directory='src/backend/role', entries={})]})
        native = dict(projection=[dict(path='source')], context=[], compiled_closure=[],
            release_record={'closure': {'directory_inventories': []}})
        self.git('checkout', '--detach', '-q', self.t)
        (self.repo / role_path).write_text('changed\n')
        self.git('add', role_path)
        self.git('commit', '-qm', 'role drift')
        self.git('checkout', self.t, '--', role_path)
        self.git('commit', '-qm', 'role restore')
        source = self.git('rev-parse', 'HEAD')
        self.git('checkout', '--detach', '-q', self.t)
        (self.repo/'unrelated').write_text('outside role unit\n')
        self.git('add','unrelated'); self.git('commit','-qm','unrelated')
        main = self.git('rev-parse','HEAD')
        side_merge = self.git('commit-tree',self.git('rev-parse',main+'^{tree}'),
            '-p',main,'-p',source,input='merge restored role branch\n')
        with patch.multiple(release, FOUNDATION_BASE=self.h, RECORD_COMMIT=self.r,
                CARRIER_PATHS=frozenset({'carrier', 'scripts/ci/f13_release_source.py'})):
            for candidate in (source,side_merge):
                with self.subTest(candidate=candidate), self.assertRaisesRegex(release.Rejected, 'release_continuation_entry_drift'):
                    release.check_continuation(self.repo, dict(base=self.h, source=self.k, target=self.t),
                                               candidate, native, role)


class RoleEvidenceTest(unittest.TestCase):
    def test_review_requires_unique_named_narrow_decision(self):
        kind='ROLE-V3-INDEPENDENT-SOURCE-REVIEW-v1'
        decision=dict(type=kind,reviewer='independent reviewer',runtime_pass=False,activation_authority=False,
            current_or_future_path_authority=False,generic_duplicate_authority=False)
        block=lambda value: ('```json\n'+json.dumps(value)+'\n```\n').encode()
        self.assertEqual(release.role_review_block(block(decision),kind),decision)
        with self.assertRaises(release.Rejected): release.role_review_block(block(decision)*2,kind)
        for invalid in (dict(decision,reviewer=''),dict(decision,runtime_pass=True),
                        dict(decision,current_or_future_path_authority=True)):
            with self.assertRaises(release.Rejected): release.role_review_block(block(invalid),kind)

    def test_raw_go_stream_derives_tests_embeds_modules_and_local_directories(self):
        objects = [dict(Dir='/work/src/backend/role/pkg', GoFiles=['main.go','/root/.cache/go-build/generated'],
            TestGoFiles=['main_test.go'], EmbedFiles=['data/policy.json'],
            Module=dict(Dir='/work/src/backend/role',Replace=dict(Dir='/work/src/backend/shared'))),
            dict(Dir='/usr/local/go/src/fmt',GoFiles=['print.go'])]
        tree = {p: 'entry' for p in ('src/backend/role/pkg/main.go','src/backend/role/pkg/main_test.go',
            'src/backend/role/pkg/data/policy.json','src/backend/role/go.mod','src/backend/shared/go.mod',
            'src/backend/shared/go.sum')}
        raw = '\n'.join(json.dumps(o,indent=2) for o in objects).encode()
        paths,dirs,manifests = release.role_evidence_paths(raw,tree)
        self.assertEqual(paths,sorted(p for p in tree if '/pkg/' in p))
        self.assertEqual(dirs,['src/backend/role/pkg'])
        self.assertEqual(manifests,sorted(p for p in tree if '/pkg/' not in p))
        forged = json.dumps(dict(Dir='/work/src/backend/role/pkg',GoFiles=['../../escape.go'])).encode()
        with self.assertRaises(release.Rejected): release.role_evidence_paths(forged,tree)
        missing = json.dumps(dict(Dir='/work/src/backend/role/pkg',GoFiles=['missing.go'])).encode()
        with self.assertRaisesRegex(release.Rejected,'release_role_missing_go_input'):
            release.role_evidence_paths(missing,tree)
        absolute = json.dumps(dict(Dir='/work/src/backend/role/pkg',GoFiles=['/work/src/backend/role/pkg/main.go'])).encode()
        with self.assertRaisesRegex(release.Rejected,'release_role_absolute_go_path'):
            release.role_evidence_paths(absolute,tree)


class NativeRecordTest(unittest.TestCase):
    """Fixture pins are private; all commits, tree entries, and blob reads are real Git."""
    def fixture(self, variant='valid', integrated=False):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        repo = Path(tmp.name)
        def git(*args):
            return subprocess.check_output(['git','-C',str(repo),*args], text=True).strip()
        def write(path, data):
            target = repo/path
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(data, encoding='utf-8')
        def commit(message):
            git('add','-A')
            git('commit','-qm',message)
            return git('rev-parse','HEAD')
        git('init','-q')
        git('config','user.name','fixture')
        git('config','user.email','fixture@example.invalid')
        git('config','core.autocrlf','false')
        projection = 'src/backend/voice/main.go'
        compiled = 'src/backend/pkg/unit.go'
        old_proto = 'protos/voice/test/old.proto'
        extra_proto = 'protos/voice/test/new.proto'
        write(projection,'old\n')
        write(compiled,'old\n')
        write(old_proto,'old proto\n')
        write('scripts/ci/testdata/f13-exact-j-547/historical-inputs.zip','fixture archive\n')
        if variant == 'old_compiled_dropped': write('src/backend/pkg/previous.go','retained\n')
        predecessor = commit('predecessor')
        before = release.legacy.entry(repo,predecessor,projection)
        write(projection,'new\n')
        write(compiled,'new\n')
        write(extra_proto,'new proto\n')
        evidence_source = commit('evidence source')
        old = dict(base=predecessor, source=predecessor,
            projection=[dict(path=projection,before=before,after=release.legacy.entry(repo,predecessor,projection))],
            context=[dict(path=p,entry=release.legacy.entry(repo,predecessor,p)) for p in (old_proto,compiled)],
            compiled_closure=[compiled])
        if variant == 'old_compiled_dropped': old['compiled_closure'].append('src/backend/pkg/previous.go')
        if variant == 'compiled':
            write(compiled,'changed\n')
        elif variant == 'deletion':
            (repo/compiled).unlink()
        elif variant == 'directory':
            write('src/backend/pkg/extra.go','new file\n')
        elif variant == 'voice':
            write('src/backend/voice/extra.go','new file\n')
        elif variant == 'extra_proto':
            write(extra_proto,'changed extra proto\n')
        elif variant == 'old_proto':
            write(old_proto,'changed old proto\n')
        if variant in ('compiled','deletion','directory','voice','extra_proto','old_proto'):
            source = commit('unit drift')
        elif variant == 'mode':
            git('update-index','--chmod=+x',compiled)
            git('commit','-qm','mode drift')
            source = git('rev-parse','HEAD')
        elif variant == 'type':
            blob = subprocess.check_output(['git','-C',str(repo),'hash-object','-w','--stdin'],
                                           input='target',text=True).strip()
            git('update-index','--cacheinfo','120000',blob,compiled)
            git('commit','-qm','type drift')
            source = git('rev-parse','HEAD')
        else:
            source = evidence_source
        evidence_tree = release.documentation.tree_entries(repo,evidence_source)
        current = release.documentation.tree_entries(repo,source)
        old_tree = release.documentation.tree_entries(repo,predecessor)
        directories = ['src/backend/pkg']
        new_dirs = release.documentation.immediate_directory_entries(current)
        old_dirs = release.documentation.immediate_directory_entries(old_tree)
        relevant = {projection,compiled,old_proto,extra_proto}
        relevant |= set(old_dirs.get(directories[0],{})) | set(new_dirs.get(directories[0],{}))
        relevant |= {p for p in set(old_tree)|set(current) if p.startswith('src/backend/voice/')}
        delta = [dict(path=p,before=release._meta(old_tree,p),after=release._meta(current,p),classification='REVIEWED')
                 for p in sorted(relevant) if old_tree.get(p) != current.get(p)]
        proofs = {}
        def proof(name, value):
            path = 'docs/testing/'+name
            raw = (json.dumps(value,sort_keys=True)+'\n').encode()
            target = repo/path
            target.parent.mkdir(parents=True,exist_ok=True)
            target.write_bytes(raw)
            result = dict(path=path,sha256=hashlib.sha256(raw).hexdigest())
            proofs[path] = result
            return result
        closure = proof('compiled.json',dict(source=evidence_source,tree=git('rev-parse',evidence_source+'^{tree}'),
            command='go list -mod=readonly -deps -test -json ./...',image='golang:1.26.5-bookworm',paths=[compiled]))
        delta_proof = proof('delta.json',dict(source=source,predecessor=predecessor,entries=delta))
        review = proof('review.json',dict(source=source,status='ACCEPTED'))
        extension = proof('extension.json',dict(source=source,evidence_source=evidence_source,status='ACCEPTED',
            delta_sha256=hashlib.sha256(subprocess.check_output(['git','-C',str(repo),'diff','--raw','--no-abbrev',
                '--no-renames',evidence_source,source])).hexdigest(),review=[review['path']]))
        clauses = {f'{g}{i}':dict(status='ACCEPTED',source=evidence_source,evidence=[closure['path']],
            review=[review['path']]) for g in 'AB' for i in range(1,5)}
        if variant == 'clause': clauses['A1']['status'] = 'OPEN'
        if variant == 'missing_proof': clauses['A1']['review'] = ['docs/testing/missing.json']
        if variant == 'extension':
            extension = proof('extension.json',dict(source='forged',status='ACCEPTED'))
        record = dict(version=release.NATIVE_VERSION,status='SOURCE_APPLICABILITY_CLEAR',source=source,
            source_tree=git('rev-parse',source+'^{tree}'),predecessor=dict(version='fixture-v4',source=predecessor,
                record=predecessor,path='docs/testing/old.json',sha256='0'*64),
            projection=[dict(path=projection,before=before,after=release.legacy.entry(repo,source,projection))],
            context=[dict(path=p,entry=release.legacy.entry(repo,source,p)) for p in (extra_proto,old_proto,compiled)],
            closure=dict(projection_paths=[projection],context_paths=[extra_proto,old_proto,compiled],compiled_paths=[compiled],
                directory_inventories=[dict(directory=directories[0],entries=new_dirs.get(directories[0],{}))]),
            compiled_evidence=closure,additional_context=[extra_proto],delta_review=delta_proof,
            evidence_source=evidence_source,extension_evidence=extension,
            evidence=[proofs[p] for p in sorted(proofs)],clauses=clauses,
            runtime_pass=False,activation_authority=False,current_or_future_path_authority=False)
        if variant == 'missing_unit': record['projection'] = []
        if variant == 'missing_compiled': record['closure']['compiled_paths'] = []
        raw = (json.dumps(record,sort_keys=True)+'\n').encode()
        path = repo/release.RECORD_PATH
        path.parent.mkdir(parents=True,exist_ok=True)
        path.write_bytes(raw)
        production_digest = None
        if integrated:
            import f13_production_source as production
            inputs = proof('inputs.json',dict(source=source,reference=source,
                delta_sha256=('f'*64 if variant == 'production_inventory' else hashlib.sha256(b'').hexdigest()),entries=[]))
            production_review = proof('production-review.json',dict(status='ACCEPTED'))
            production_record = dict(version=release.PRODUCTION_VERSION,status='SOURCE_CONSUMPTION_CONTRACT_CLEAR',
                source=source,source_tree=git('rev-parse',source+'^{tree}'),
                predecessor=dict(version=production.BOOTSTRAP_VERSION,record=source,
                    path=production.BOOTSTRAP_RECORD_PATH,sha256=production.BOOTSTRAP_RECORD_SHA256),
                foundation_base=source,reuse_policy=('UNREVIEWED' if variant=='production_policy' else release.REUSE_POLICY),
                collection_base=source,current_inputs=inputs,
                native_record=dict(version=release.NATIVE_VERSION,source=source,path=release.RECORD_PATH,
                    sha256=hashlib.sha256(raw).hexdigest()),
                role_record=dict(version=release.ROLE_VERSION,source=source,path=release.ROLE_RECORD_PATH,
                    sha256=release.ROLE_SHA256),evidence=[production_review],
                review=[production_review['path']],runtime_pass=False,activation_authority=False,
                current_or_future_path_authority=False)
            production_raw = (json.dumps(production_record,sort_keys=True)+'\n').encode()
            (repo/release.PRODUCTION_RECORD_PATH).write_bytes(production_raw)
            (repo/release.ROLE_RECORD_PATH).write_text('mocked role record\n')
            production_digest = hashlib.sha256(production_raw).hexdigest()
        record_commit = commit('record')
        self.fixture_info = dict(source=source, record=record_commit, production_digest=production_digest)
        patches = [patch.multiple(release,FOUNDATION_BASE=source,EVIDENCE_SOURCE=evidence_source,
                    RECORD_COMMIT=record_commit,NATIVE_SHA256=hashlib.sha256(raw).hexdigest()),
                   patch.multiple(release.documentation,CONTEXT_VERSION='fixture-v4',CONTEXT_SOURCE=predecessor,
                    CONTEXT_RECORD_COMMIT=predecessor,CONTEXT_RECORD_PATH='docs/testing/old.json',CONTEXT_RECORD_SHA256='0'*64),
                   patch.object(release.documentation,'load_context_record',return_value=old)]
        if integrated:
            patches.append(patch.object(release,'load_role',return_value=dict(
                required_current_source=[],context=[],closure={'directory_inventories':[]},
                release_record={'closure':delta_proof,'evidence':[]})))
        for item in patches:
            item.start()
            self.addCleanup(item.stop)
        return repo

    def test_single_record_through_both_qualifiers_and_collection_cli(self):
        import f13_production_source as production
        repo = self.fixture(integrated=True)
        info = self.fixture_info
        def git(*args, input=None):
            return subprocess.check_output(['git','-C',str(repo),*args],input=input,text=True).strip()
        verifier = repo/'scripts/ci/f13_release_source.py'
        verifier.parent.mkdir(parents=True,exist_ok=True)
        verifier.write_text('fixture verifier\n')
        git('add','-A')
        git('commit','-qm','carrier')
        carrier = git('rev-parse','HEAD')
        tree = git('rev-parse',carrier+'^{tree}')
        target = git('commit-tree',tree,'-p',info['source'],'-p',carrier,input='event\n')
        git('checkout','--detach','-q',target)
        event = dict(GITHUB_EVENT_NAME='push',GITHUB_REF='refs/heads/master',GITHUB_SHA=target,
                     GITHUB_BASE_REF='',VOICE_CI_PUSH_BEFORE_SHA=info['source'])
        with patch.multiple(release,ROOT=repo,COLLECTION_BASE=info['source'],
                PRODUCTION_SHA256=info['production_digest'],CARRIER_PATHS=frozenset({'scripts/ci/f13_release_source.py'})), \
             patch.multiple(production,REFERENCE=info['source'],BOOTSTRAP_RECORD_COMMIT=info['source']), \
             patch.object(production,'load_bootstrap_fix'), \
             patch.object(production,'load_record',return_value={'source':info['source']}), \
             patch.dict(os.environ,event):
            binding = release.native_qualify(repo,info['source'],carrier,target,'push','refs/heads/master',
                                             target,info['source'],'')
            self.assertEqual(release.production_qualify(repo)['type'],release.PRODUCTION_VERSION)
            self.assertEqual(release.verified_record(repo,binding)['source'],info['source'])
            binding_path = repo/'binding.json'
            binding_path.write_text(json.dumps(binding))
            output = io.StringIO()
            with patch.object(sys,'argv',['release','collection-base',str(repo),str(binding_path)]), redirect_stdout(output):
                self.assertEqual(release.main(),0)
            self.assertEqual(output.getvalue(),info['source']+'\n')
            for binding_change, event_change in (
                ({'collection_base': info['record']}, {}),
                ({'version': 'NATIVE-P3-SOURCE-v4'}, {}),
                ({}, {'GITHUB_SHA': carrier}),
                ({}, {'VOICE_CI_PUSH_BEFORE_SHA': info['record']}),
            ):
                with self.subTest(binding=binding_change, event=event_change):
                    binding_path.write_text(json.dumps(dict(binding, **binding_change)))
                    output, errors = io.StringIO(), io.StringIO()
                    with patch.object(sys, 'argv', ['release', 'collection-base', str(repo), str(binding_path)]), \
                         patch.dict(os.environ, event_change), redirect_stdout(output), redirect_stderr(errors):
                        self.assertEqual(release.main(), 1)
                    self.assertEqual(output.getvalue(), '')
                    self.assertIn('Release source rejected:', errors.getvalue())
            base=target
            for name in ('.github/ci/full-application-build-request.json','scripts/dev/media_initial_admission.py'):
                git('checkout','--detach','-q',base)
                changed=repo/name
                changed.parent.mkdir(parents=True,exist_ok=True)
                changed.write_text('outside unit\n')
                git('add',name)
                git('commit','-qm','outside unit')
                current_source=git('rev-parse','HEAD')
                current_target=git('commit-tree',git('rev-parse',current_source+'^{tree}'),
                    '-p',base,'-p',current_source,input='continuation event\n')
                git('checkout','--detach','-q',current_target)
                with patch.dict(os.environ,dict(GITHUB_SHA=current_target,VOICE_CI_PUSH_BEFORE_SHA=base)):
                    current=release.native_qualify(repo,base,current_source,current_target,'push',
                        'refs/heads/master',current_target,base,'')
                    actual=release.production_qualify(repo)
                    self.assertEqual(current['foundation'],dict(base=info['source'],source=carrier,target=target))
                    self.assertEqual(current['qualification'],'UNCHANGED_UNIT_CONTINUATION')
                    self.assertEqual(actual['base'],base)
                    self.assertEqual(actual['foundation'],current['foundation'])
                    raw=subprocess.check_output(['git','-C',str(repo),'diff','--raw','--no-abbrev',
                                                 '--no-renames',base,current_source])
                    self.assertEqual(current['event_delta_sha256'],hashlib.sha256(raw).hexdigest())
                    self.assertEqual(actual['event_delta_sha256'],current['event_delta_sha256'])
                    self.assertFalse(actual['admission'])
                    self.assertFalse(actual['current_or_future_path_authority'])
                    self.assertEqual(release.collection_base(repo,current),info['source'])
                base=current_target

    def test_real_record_and_drift_rejections(self):
        repo = self.fixture()
        self.assertEqual(release.load_native(repo)['version'],release.NATIVE_VERSION)
        with patch.object(release,'NATIVE_SHA256','f'*64):
            with self.assertRaises(release.Rejected): release.load_native(repo)
        subprocess.check_call(['git','-C',str(repo),'commit','--allow-empty','-qm','forged record successor'])
        forged = subprocess.check_output(['git','-C',str(repo),'rev-parse','HEAD'],text=True).strip()
        with patch.object(release,'RECORD_COMMIT',forged):
            with self.assertRaises(release.Rejected): release.load_native(repo)
        (repo/'docs/testing/review.json').write_text('tampered\n')
        subprocess.check_call(['git','-C',str(repo),'add','docs/testing/review.json'])
        subprocess.check_call(['git','-C',str(repo),'commit','-qm','tampered evidence'])
        changed = subprocess.check_output(['git','-C',str(repo),'rev-parse','HEAD'],text=True).strip()
        with patch.object(release,'RECORD_COMMIT',changed):
            with self.assertRaises(release.Rejected): release.load_native(repo)

    def test_rejects_missing_closure_drift_and_unaccepted_evidence(self):
        for variant in ('missing_unit','missing_compiled','old_compiled_dropped','compiled','mode','type','deletion',
                        'directory','voice','clause','missing_proof','extension'):
            with self.subTest(variant=variant):
                repo = self.fixture(variant)
                with self.assertRaises(release.Rejected): release.load_native(repo)

    def test_context_proto_changes_are_rejected(self):
        for variant in ('extra_proto','old_proto'):
            with self.subTest(variant=variant):
                repo = self.fixture(variant)
                with self.assertRaisesRegex(release.Rejected,'release_context_proto_drift'):
                    release.load_native(repo)

    def test_production_inventory_and_pinned_record(self):
        import f13_production_source as production
        for variant in ('valid','production_inventory','production_policy'):
            with self.subTest(variant=variant):
                repo = self.fixture(variant, integrated=True)
                info = self.fixture_info
                source = info['source']
                with patch.multiple(release,COLLECTION_BASE=source,PRODUCTION_SHA256=info['production_digest']), \
                     patch.multiple(production,REFERENCE=source,BOOTSTRAP_RECORD_COMMIT=source), \
                     patch.object(production,'load_bootstrap_fix'):
                    if variant != 'valid':
                        with self.assertRaises(release.Rejected): release.production_record(repo)
                    else:
                        self.assertEqual(release.production_record(repo)['version'],release.PRODUCTION_VERSION)
                        with patch.object(release,'PRODUCTION_SHA256','f'*64):
                            with self.assertRaises(release.Rejected): release.production_record(repo)


class MapTest(unittest.TestCase):
    def test_real_owner_best_bases_taint_and_collection_base(self):
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            def git(*args):
                return subprocess.check_output(['git','-C',str(repo),*args],text=True).strip()
            git('init','-q')
            git('config','user.name','fixture')
            git('config','user.email','fixture@example.invalid')
            (repo/'unit.txt').write_text('old')
            git('add','unit.txt')
            git('commit','-qm','base')
            base = git('rev-parse','HEAD')
            (repo/'unit.txt').write_text('new')
            git('add','unit.txt')
            git('commit','-qm','owner')
            source = git('rev-parse','HEAD')
            record = dict(source=source,projection=[dict(path='unit.txt',after=release.legacy.entry(repo,source,'unit.txt'))])
            binding = dict(source=source,target=source)
            row = '\t'.join(('unit.txt','false',source,base,base,source,base,base))
            map_path, history_path = repo/'map',repo/'history'
            def audit(line, history=None):
                map_path.write_text(line+'\n')
                history_path.write_text((history or line)+'\n')
                return release.audit_map(repo,binding,map_path,history_path)
            with patch.object(release,'verified_record',return_value=record), patch.object(release,'COLLECTION_BASE',base):
                self.assertEqual(audit(row)['rows'],{'unit.txt':row})
                with self.assertRaises(release.Rejected): audit(row.replace(base,source))
                with self.assertRaises(release.Rejected): audit(row, row.replace('\tfalse\t','\ttrue\t')+'\n'+row)
                with self.assertRaises(release.Rejected): audit(row.rsplit('\t',1)[0]+'\t'+source)
                map_path.write_text('')
                history_path.write_text(row+'\n')
                with self.assertRaises(release.Rejected): release.audit_map(repo,binding,map_path,history_path)
                with patch.object(release,'COLLECTION_BASE',source):
                    with self.assertRaises(release.Rejected): audit(row)


@unittest.skipUnless(sys.platform == 'linux', 'full-history child lifecycle runs on Linux CI/Docker')
class StandaloneSuiteLifecycleTest(unittest.TestCase):
    """Execute the real shell launcher with small disposable suite programs."""
    names = ['f13_space_audit_test.py', 'f13_production_source_test.py',
             'f13_release_source_test.py', 'f13_native_p3_documentation_test.py']

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='f13-suite-lifecycle-')
        self.addCleanup(self.directory.cleanup)
        self.folder = Path(self.directory.name)
        self.repo = self.folder / 'repo'
        (self.repo / 'scripts/ci').mkdir(parents=True)
        self.inputs = self.folder / 'inputs'
        self.inputs.mkdir()
        self.source = (Path(__file__).resolve().parent /
                       'voice-db-runtime-provisioning-contract_test.sh').read_text()
        functions = {m.group(1): m.group(0) for m in re.finditer(
            r'(?ms)^([A-Za-z_][A-Za-z0-9_]*)\(\) \{\n.*?^\}\n', self.source)}
        self.functions = '\n'.join(functions[name] for name in (
            'f13_cleanup', 'f13_standalone_suites', 'f13_start_standalone_suites',
            'f13_wait_standalone_suites'))
        for index, name in enumerate(self.names):
            self.write_suite(index, f"with open({str(self.folder/'order')!r}, 'a') as out: out.write({name!r}+'\\n')\n")

    def write_suite(self, index, body):
        (self.repo / 'scripts/ci' / self.names[index]).write_text(body)

    def launch(self, producer=':', prelude=''):
        import shlex
        program = 'set -euo pipefail\n' + self.functions + '\n'
        program += 'ROOT=' + shlex.quote(str(self.repo)) + '\nTMP_DIR=' + shlex.quote(str(self.inputs)) + '\n'
        program += "trap f13_cleanup EXIT\ntrap 'exit 130' INT\ntrap 'exit 143' TERM\n"
        program += prelude + '\nf13_start_standalone_suites\n' + producer + '\n'
        program += 'f13_wait_standalone_suites\necho FULL_PASS\n'
        process = subprocess.Popen(['/bin/bash', '-c', program], stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, text=True, start_new_session=True)
        self.addCleanup(self.settle_harness, process)
        return process

    @staticmethod
    def settle_harness(process):
        if process.poll() is None:
            process.terminate()
            try:
                process.communicate(timeout=10)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.communicate()

    def result(self, process, expected):
        out, err = process.communicate(timeout=10)
        self.assertEqual(process.returncode, expected, out + err)
        self.assertEqual('FULL_PASS' in out, expected == 0, out + err)
        self.assertFalse(self.inputs.exists(), 'inputs removed only after owned children settle')
        return out, err

    def wait_path(self, path):
        deadline = time.monotonic() + 5
        while not path.exists() and time.monotonic() < deadline:
            time.sleep(0.01)
        self.assertTrue(path.exists(), str(path))

    def assert_no_descendants(self):
        for name in ('suite.pid', 'grandchild.pid'):
            path = self.folder / name
            self.wait_path(path)
            self.assertFalse(Path('/proc', path.read_text()).exists(), name + ' survived cleanup')

    def long_suite(self):
        self.write_suite(0, 'import os, pathlib, signal, subprocess, sys, time\n'
            'signal.signal(signal.SIGTERM, signal.SIG_IGN)\n'
            f'pathlib.Path({str(self.folder/"suite.pid")!r}).write_text(str(os.getpid()))\n'
            'child = subprocess.Popen([sys.executable, "-c", '
            f'{("import os,pathlib,signal,time; signal.signal(signal.SIGTERM,signal.SIG_IGN); pathlib.Path(" + repr(str(self.folder/"grandchild.pid")) + ").write_text(str(os.getpid())); time.sleep(60)")!r}])\n'
            'time.sleep(60)\n')

    def test_exact_inventory_conditions_order_and_unchanged_whole_producer(self):
        prefix = self.source[self.source.index('printf \'%s\\n\' \'== F13/G7:'):self.source.index('f13_start_standalone_suites\nbase_sha=')]
        tail = re.search(r'(?ms)^f13_standalone_suites\(\) \{\n(.*?)^\}\n', self.source).group(1)
        names = re.findall(r'python3 "\$\{ROOT\}/scripts/ci/([^" ]+)"', prefix + tail)
        self.assertEqual(names, ['f13_native_p3_source_test.py', 'f13_native_p3_source_history_test.py',
            'f13_native_p3_successor_test.py', 'f13_native_p3_successor_integration_test.py',
            'f13_guarded_merge_test.py', 'f13_reviewed_test_merge_test.py',
            'f13_historical_composition_test.py', 'f13_role_conditional_test.py',
            'f13_role_conditional_successor_test.py'] + self.names)
        for name in self.names[1:]:
            self.assertIn('if [[ -f "${ROOT}/scripts/ci/' + name + '" ]]; then', tail)
        producer = self.source[self.source.index('\nbase_sha="${VOICE_R22') + 1:self.source.rindex('f13_wait_standalone_suites\n')]
        self.assertEqual(hashlib.sha256(producer.encode()).hexdigest(),
                         'a71b4dd2fdd1775285dc19de82992b0360fcd420fdc4166acd21e188e513ee7c')
        self.assertTrue(self.source.rstrip().endswith("f13_wait_standalone_suites\necho 'Voice DB RED-F runtime/provisioning/docs contract: PASS'"))

    def test_success_requires_actual_ordered_suites_and_overlaps_producer(self):
        gate = self.folder / 'producer'
        self.write_suite(0, 'import pathlib,time\np=' + repr(str(gate)) + '\n'
            'end=time.monotonic()+4\n'
            'while not pathlib.Path(p).exists() and time.monotonic()<end: time.sleep(.01)\n'
            'assert pathlib.Path(p).exists(), "producer did not overlap"\n' +
            f'open({str(self.folder/"order")!r}, "a").write({self.names[0]!r}+"\\n")\n')
        self.result(self.launch('touch "' + str(gate) + '"'), 0)
        self.assertEqual((self.folder / 'order').read_text().splitlines(), self.names)

    def test_optional_conditions_still_control_discovery(self):
        for name in self.names[1:]:
            (self.repo / 'scripts/ci' / name).unlink()
        self.result(self.launch(), 0)
        self.assertEqual((self.folder / 'order').read_text().splitlines(), self.names[:1])

    def test_suite_failure_is_mandatory_and_stops_later_suite_commands(self):
        self.write_suite(1, 'raise SystemExit(37)\n')
        self.result(self.launch(), 37)
        self.assertEqual((self.folder / 'order').read_text().splitlines(), self.names[:1])

    def test_producer_failure_preserved_and_owned_descendants_reaped(self):
        self.long_suite()
        gate = str(self.folder / 'grandchild.pid')
        self.result(self.launch('while [[ ! -f "' + gate + '" ]]; do sleep .01; done\nexit 41'), 41)
        self.assert_no_descendants()

    def test_parent_signals_join_owned_descendants_before_input_cleanup(self):
        for received in (signal.SIGTERM, signal.SIGINT):
            with self.subTest(signal=received):
                self.inputs.mkdir(exist_ok=True)
                for name in ('suite.pid', 'grandchild.pid'):
                    (self.folder / name).unlink(missing_ok=True)
                self.long_suite()
                process = self.launch('while :; do sleep .02; done')
                self.wait_path(self.folder / 'grandchild.pid')
                process.send_signal(received)
                self.result(process, 128 + received)
                self.assert_no_descendants()

    def test_supervisor_launch_failure_cannot_print_pass(self):
        self.result(self.launch(prelude='python3() { return 73; }'), 73)

    def test_suite_shell_spawn_failure_cannot_print_pass(self):
        self.functions = self.functions.replace("['/bin/bash', '-c'", "['/missing-f13-bash', '-c'")
        self.result(self.launch(), 1)

    def test_escaped_double_fork_is_reaped_without_signalling_foreign_process(self):
        foreign = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(60)'])
        self.addCleanup(self.settle_harness, foreign)
        escaped = self.folder / 'escaped.pid'
        self.write_suite(0, 'import os,pathlib,signal,time\n'
            'if os.fork(): os._exit(0)\n'
            'os.setsid()\n'
            'if os.fork(): os._exit(0)\n'
            'signal.signal(signal.SIGTERM, signal.SIG_IGN)\n'
            f'pathlib.Path({str(escaped)!r}).write_text(str(os.getpid()))\n'
            'time.sleep(60)\n')
        self.result(self.launch('while [[ ! -f "' + str(escaped) + '" ]]; do sleep .01; done\nexit 41'), 41)
        self.assertFalse(Path('/proc', escaped.read_text()).exists())
        self.assertIsNone(foreign.poll())

    def test_signal_in_launch_capture_gap_is_deferred_and_joined(self):
        marker = self.folder / 'launch-gap'
        self.functions = self.functions.replace('  f13_standalone_pid=$!',
            '  touch "' + str(marker) + '"\n  sleep .5 || true\n  f13_standalone_pid=$!')
        self.long_suite()
        process = self.launch('exit 99')
        self.wait_path(marker)
        process.send_signal(signal.SIGTERM)
        self.result(process, 143)
        # A suite may be cancelled before it starts; any recorded children must be gone.
        for name in ('suite.pid', 'grandchild.pid'):
            path = self.folder / name
            if path.exists():
                self.assertFalse(Path('/proc', path.read_text()).exists())

    def test_repeated_signal_during_cleanup_keeps_join(self):
        self.long_suite()
        process = self.launch('while :; do sleep .02; done')
        self.wait_path(self.folder / 'grandchild.pid')
        process.send_signal(signal.SIGTERM)
        time.sleep(.05)
        process.send_signal(signal.SIGTERM)
        self.result(process, 143)
        self.assert_no_descendants()

    def test_signals_interrupt_actual_join_but_inputs_survive_until_settlement(self):
        for received in (signal.SIGTERM, signal.SIGINT):
            with self.subTest(signal=received):
                self.inputs.mkdir(exist_ok=True)
                for name in ('suite.pid', 'grandchild.pid'):
                    (self.folder / name).unlink(missing_ok=True)
                self.long_suite()
                # The producer finishes immediately; the shell is inside the
                # actual f13_wait_standalone_suites while the suite is alive.
                process = self.launch(':')
                self.wait_path(self.folder / 'grandchild.pid')
                process.send_signal(received)
                time.sleep(.05)
                self.assertTrue(self.inputs.exists(), 'inputs removed before settlement')
                self.assertTrue(Path('/proc', (self.folder / 'grandchild.pid').read_text()).exists())
                process.send_signal(received)
                self.result(process, 128 + received)
                self.assert_no_descendants()

    def test_descendant_forked_during_term_is_reaped(self):
        ready, escaped = self.folder / 'ready', self.folder / 'term-fork.pid'
        self.write_suite(0, 'import os,pathlib,signal,time\n'
            'def on_term(sig, frame):\n'
            '    if os.fork() == 0:\n'
            '        os.setsid()\n'
            '        signal.signal(signal.SIGTERM, signal.SIG_IGN)\n'
            f'        pathlib.Path({str(escaped)!r}).write_text(str(os.getpid()))\n'
            '        time.sleep(60)\n'
            'signal.signal(signal.SIGTERM, on_term)\n'
            f'pathlib.Path({str(ready)!r}).touch()\n'
            'time.sleep(60)\n')
        self.result(self.launch('while [[ ! -f "' + str(ready) + '" ]]; do sleep .01; done\nexit 41'), 41)
        self.wait_path(escaped)
        self.assertFalse(Path('/proc', escaped.read_text()).exists())


if __name__ == '__main__':
    unittest.main()
