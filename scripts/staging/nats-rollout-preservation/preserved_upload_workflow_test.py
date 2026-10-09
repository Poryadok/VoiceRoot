"""Actual workflow routing and historical target data remain separate from helper code."""
import ast
import copy
import datetime as dt
import hashlib
import os
from pathlib import Path
import tempfile
import textwrap
import unittest
from unittest.mock import patch
import guard
import guard_test
import workflow_entry


class WorkflowTests(unittest.TestCase):
    def route(self, action, **changes):
        source=(Path(__file__).resolve().parents[3]/'.github/workflows/staging-deploy.yml').read_text()
        rows=source.splitlines()
        start=next(i for i,row in enumerate(rows) if row.strip()=="if action=='--bridge-prepare':")
        end=next(i for i,row in enumerate(rows[start:],start) if row.strip().startswith('sys.argv='))
        body=textwrap.dedent('\n'.join(rows[start:end]))
        namespace={'os':os,'action':action,'op':'3340764a7d24','bridge_action':True}
        env={'GITHUB_SHA':'f'*40,'GITHUB_RUN_ID':'37910000000','VOICE_IMAGE_TAG':'f'*40,
             'DEPLOY_MODE':'images-only','CHANGED_SERVICES':'story','ROLLOUT_ARTIFACT_ID':'123'}
        env.update(changes)
        with patch.dict(os.environ,env,clear=True):exec(compile(ast.parse(body),'<exact-workflow-route>','exec'),namespace)
        return namespace['arguments']

    def test_exact_workflow_fresh_upload_and_ordinary_authorize_are_distinct(self):
        self.assertEqual(self.route('--bridge-prepare',ROLLOUT_CIPHER_UPLOAD_OPERATION='3340764a7d24'),
                         ['resume-cipher-upload','3340764a7d24','37910000000'])
        self.assertEqual(self.route('--bridge-authorize',ROLLOUT_UPLOAD_NONCE='1'*64),
                         ['authorize-preserved-upload','3340764a7d24','123','1'*64,'37910000000'])
        self.assertEqual(self.route('--bridge-authorize'),['authorize','3340764a7d24','123'])
        for change in ({'ROLLOUT_COLD_BACKUP_OPERATION':'3340764a7d24'},
                       {'ROLLOUT_ROLLBACK_OPERATION':'abcdef123456'}, {'VOICE_IMAGE_TAG':'a'*40},
                       {'DEPLOY_MODE':'full'}, {'CHANGED_SERVICES':'story,user'},
                       {'ROLLOUT_CIPHER_UPLOAD_OPERATION':'abcdef123456'}):
            with self.subTest(change=change),self.assertRaises(SystemExit):
                self.route('--bridge-prepare',**({'ROLLOUT_CIPHER_UPLOAD_OPERATION':'3340764a7d24'}|change))

    def test_exact_workflow_historical_data_is_acquired_before_fresh_upload_window(self):
        source=(Path(__file__).resolve().parents[3]/'.github/workflows/staging-deploy.yml').read_text()
        first=source.index('      - name: Download exact staging source archive')
        data=source.index('      - name: Acquire preserved historical target as data')
        prepare=source.index('      - name: Prepare cold backup')
        self.assertLess(first,data);self.assertLess(data,prepare)
        block=source[data:prepare]
        self.assertIn('STAGING_SOURCE_SHA: 73ca52699ddf6a9182e3407d5bbdedbc29c06dee',block)
        self.assertIn('run: *download_staging_source',block)
        self.assertNotIn('python',block)

    def test_actual_workflow_entry_consumes_historical_data_with_real_guard_hash_veto(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);data=root/'historical';data.mkdir()
            path=data/'scripts/target.py';path.parent.mkdir();path.write_bytes(b'captured historical input\n')
            receipt=guard_test.receipt()|{'schema':'nats-rollout-preservation-v1',
                'backup':{'archive_sha256':'a'*64,'manifest_sha256':'b'*64,'census_sha256':'c'*64,'off_node_verified':True},
                'expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(minutes=5)).isoformat(),
                'target':{'registry':'ghcr.io/poryadok/voiceroot','tag':'73ca52699ddf6a9182e3407d5bbdedbc29c06dee',
                    'mode':'images-only','changed_services':['story'],
                    'source_hashes':{'scripts/target.py':hashlib.sha256(path.read_bytes()).hexdigest()},
                    'template_hashes':{'voice-story':'d'*64},'input_template_hashes':{'voice-story':'e'*64},
                    'images':{'voice-story/story':'ghcr.io/poryadok/voiceroot/story@sha256:'+'f'*64},
                    'manifest_sha256':'a'*64,'migration_sha256':'b'*64,'nonnats_sha256':'c'*64}}
            entry=root/('rollout-'+receipt['operation'])/'code/nats-rollout-preservation/workflow_entry.py'
            entry.parent.mkdir(parents=True)
            def runner(args):
                self.assertEqual(args,['--preflight'])
                return guard.validate(receipt,'ghcr.io/poryadok/voiceroot',receipt['target']['tag'],'images-only',['story'],guard.source_root())
            env={'VOICE_NATS_ROLLOUT_OPERATION':receipt['operation'],'GITHUB_WORKSPACE':str(data)}
            with patch.dict(os.environ,env,clear=True),patch.object(workflow_entry,'__file__',str(entry)),\
                 patch.object(workflow_entry,'code_binding',return_value={}),patch.object(workflow_entry.runner,'main',side_effect=runner) as called:
                workflow_entry.main(['--preflight']);self.assertEqual(called.call_count,1)
                for mutation in ('hash','symlink','missing','oversize'):
                    with self.subTest(mutation=mutation):
                        path.unlink()
                        if mutation=='hash':path.write_bytes(b'current helper bytes')
                        elif mutation=='symlink':path.symlink_to(entry)
                        elif mutation=='oversize':path.write_bytes(b'x'*((2<<20)+1))
                        with self.assertRaises(guard.Blocked):workflow_entry.main(['--preflight'])
                        if path.exists() or path.is_symlink():path.unlink()
                        path.write_bytes(b'captured historical input\n')


if __name__=='__main__':unittest.main()
