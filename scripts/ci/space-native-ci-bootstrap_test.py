#!/usr/bin/env python3
"""Custody failures must reject BEFORE migrations or installer invocation."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location('bootstrap', Path(__file__).with_name('space-native-ci-bootstrap.py'))
bootstrap = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bootstrap)


class CustodyTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.state = Path(self.temp.name)/'state.json'
        self.env = Path(self.temp.name)/'compose.env'
        self.env.write_text('PHASE0_FIXTURE_DIR=/fixture\n')
        self.args = argparse.Namespace(state=str(self.state), env_file=str(self.env), compose=['docker', 'compose'])
        self.project = 'voice-ci-native-test'
        self.volume = self.project+'_voice_pgdata'
        self.value = {'name': self.project, 'volumes': {'voice_pgdata': {'name': self.volume}},
                      'services': {'postgres': {'volumes': [{'type':'volume','source':'voice_pgdata'}]},
                                   'compose-db-init': {'environment': {'PGUSER':'owner','PGPASSWORD':'private'}},
                                   'space': {'environment': {'DATABASE_URL':'old'}}}}

    def test_existing_database_is_not_adopted_or_mutated(self):
        def execute(command, **unused):
            if command[-3:] == ['config', '--format', 'json']: return json.dumps(self.value)
            if command[1] == 'info': return 'engine'
            if command[1:3] == ['volume','ls']: return self.volume
            if command[1] == 'ps': return ''
            self.fail('existing database must not trigger a mutation')
        with mock.patch.object(bootstrap, 'execute', side_effect=execute), mock.patch.dict(os.environ, {}, clear=True):
            with self.assertRaisesRegex(RuntimeError, 'provenance is unknown'):
                bootstrap.prepare(self.args)
        self.assertFalse(self.state.exists())
        self.assertEqual(self.env.read_text(), 'PHASE0_FIXTURE_DIR=/fixture\n')

    def test_foreign_or_external_volume_is_refused(self):
        for external, name in [(True,self.volume),(False,'another-project-data')]:
            self.value['volumes']['voice_pgdata'] = {'name':name,'external':external}
            with mock.patch.object(bootstrap,'execute',return_value=json.dumps(self.value)):
                with self.assertRaisesRegex(RuntimeError,'foreign'):
                    bootstrap.config_for(self.args.compose)

    def test_install_rejects_changed_custody_source_and_serving_identity(self):
        password='a'*64
        state={'engine':'engine','project':self.project,'volume':self.volume,
               'volume_created_at':'created','nonce':'n'*32,'app_password':password,
               'migrations':{'45.sql':'pinned'},'install_attempted':False}
        self.value['services']['space']['environment']['DATABASE_URL']=bootstrap.application_url(password)
        for change in ['engine','nonce','creation','source','serving owner','repeat']:
            with self.subTest(change=change):
                selected=dict(state)
                selected['install_attempted']=change=='repeat'
                self.state.write_text(json.dumps(selected))
                volume={'CreatedAt':'other' if change=='creation' else 'created',
                        'Labels':{bootstrap.LABEL:'other' if change=='nonce' else state['nonce']}}
                value=json.loads(json.dumps(self.value))
                if change=='serving owner': value['services']['space']['environment']['DATABASE_URL']='postgres://owner@postgres/space_db'
                commands=[]
                def execute(command, **unused):
                    commands.append(command)
                    if command[-3:]==['config','--format','json']: return json.dumps(value)
                    if command[1:3]==['volume','inspect']: return json.dumps([volume])
                    if command[1]=='info': return 'other' if change=='engine' else 'engine'
                    self.fail('invalid custody must not reach migration or installer')
                hashes={'45.sql':'changed' if change=='source' else 'pinned'}
                with mock.patch.object(bootstrap,'execute',side_effect=execute), mock.patch.object(bootstrap,'migration_hashes',return_value=hashes):
                    with self.assertRaisesRegex(RuntimeError,'custody/source mismatch'):
                        bootstrap.install(self.args)
                self.assertFalse(any('up' in c or 'run' in c or 'build' in c for c in commands))


if __name__ == '__main__':
    unittest.main()
