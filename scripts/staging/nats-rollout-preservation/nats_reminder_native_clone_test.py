"""Opt-in populated V7 native transition on a disposable signed NATS realm.

Actual pinned server, RootJournal/Session, source compiler, actor, native decoder,
closed archive, migration and post-census run unchanged. Only the external TT
card and physical PVC fence are fixture identities. This is not live admission,
off-node custody, or an end-to-end Stage workflow qualification.
"""
import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import tempfile
import unittest
from unittest.mock import patch

import bootstrap_auth
import guard
import nats_contract_plan as contract
import nats_migration
import preserve
import resource_claim
import root_cli
from controller import Blocked
from docker_runtime import DockerRuntime, BOX_IMAGE, NATS_IMAGE
from nats_contract_actor import Actor
from native_store import archive_closed_store, verify_archive
from resource_claim_test import context, card
from rollout_census import census, semantic
from scenario import ready
from staging_prerequisites import BINARY_PINS


def durable(path, value):
    raw=json.dumps(value,sort_keys=True,separators=(',',':')).encode()
    temporary=path.with_suffix('.new')
    fd=os.open(temporary,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(fd,'wb') as output:
        output.write(raw);output.flush();os.fsync(output.fileno())
    os.replace(temporary,path)
    fd=os.open(path.parent,os.O_RDONLY|os.O_DIRECTORY)
    try:os.fsync(fd)
    finally:os.close(fd)


@unittest.skipUnless(os.environ.get('VOICE_COHORT_AUTH_INPUTS') and os.environ.get('VOICE_COHORT_NATIVE_ROOT'),
                     'owned signed inputs and daemon-shared test volume required')
class ReminderNativeCloneTests(unittest.TestCase):
    def test_populated_native_transition_and_lost_create_ack(self):
        source=Path(__file__).resolve().parents[3]
        code=source/'scripts/staging';binding=root_cli.code_binding(code)
        self.assertEqual(binding['nats-known-baseline/kernel'],BINARY_PINS['nats-known-baseline/kernel'])
        fixture=Path(os.environ['VOICE_COHORT_AUTH_INPUTS'])
        config=(fixture/'server.conf').read_text()
        value=lambda name:json.loads(re.search(r'^'+name+r': (.+)$',config,re.M)[1])
        system=value('system_account');resolver=value('resolver_preload')
        account=next(name for name in resolver if name!=system)
        tokens={'operator.jwt':value('operator'),'account.jwt':resolver[account],
                'system-account.jwt':resolver[system],'account.public':account,'system-account.public':system}
        operator={'data':{key:base64.b64encode(raw.encode()).decode() for key,raw in tokens.items()}}
        with tempfile.TemporaryDirectory(dir=os.environ['VOICE_COHORT_NATIVE_ROOT']) as directory:
            base=Path(directory);root=base/'authority';root.mkdir(mode=0o700)
            (root/'installed').mkdir(mode=0o700)
            inputs=base/'inputs';inputs.mkdir(mode=0o750);os.chown(inputs,0,65532)
            for name in ('bootstrap.creds','social.creds'):
                target=inputs/name;target.write_bytes((fixture/'inputs'/name).read_bytes())
                target.chmod(0o440);os.chown(target,0,65532)
            (inputs/'account.public').write_text(account);(inputs/'account.public').chmod(0o440)
            (base/'server.conf').write_text(config);(base/'server.conf').chmod(0o440);os.chown(base/'server.conf',0,65532)
            c=context();authority=card(c)
            session=resource_claim.Session(c,'7a7b7c7d7e7f',resource_claim.digest(binding),
                lambda:copy.deepcopy(authority),resource_claim.RootJournal(root))
            session.admit(start=True)
            with patch.object(guard,'ROOT',root),resource_claim.command_scope(session.check),\
                 patch.object(bootstrap_auth,'ACCOUNT_SHA',hashlib.sha256(account.encode()).hexdigest()):
                auth=bootstrap_auth.prove((inputs/'bootstrap.creds').read_bytes(),operator,session.check,
                                          version='voice-bootstrap-auth-v6')
                normalized={(row['stream'],row['durable']):row['config'] for row in auth['normalized_consumers']}
                runtime=DockerRuntime(base,'seed'+session.operation)
                store=base/'original';store.mkdir(mode=0o700);os.chown(store,65532,65532)
                broker=None
                try:
                    broker=runtime.start_broker('original',store);ready(runtime,broker)
                    actor=Actor(runtime,broker,runtime.owned[broker]['id'],store,session.check,
                                plan={'schema':contract.SCHEMA,'migration_id':contract.MIGRATION_ID})
                    subjects={'chat_events':sorted(contract.CHAT_SUBJECTS_17),
                        'role_events':sorted(contract.ROLE_SUBJECTS_10),'voice_events':sorted(contract.VOICE_SUBJECTS_12),
                        'user_events':sorted(contract.USER_SUBJECTS_14),'file_events':['file.processed'],
                        'social_events':['social.friend_request','social.friend_accepted','social.friend_removed',
                                         'social.user_blocked','social.contacts_synced']}
                    for name,values in subjects.items():
                        actor._request('$JS.API.STREAM.CREATE.'+name,{'name':name,'subjects':values,
                            'storage':'file','retention':'limits','max_age':604800000000000,
                            'duplicate_window':86400000000000 if name=='social_events' else 120000000000})
                    for stream,declaration in contract.DECLARATIONS:
                        actor.mutate('$JS.API.CONSUMER.CREATE.'+stream+'.'+declaration['durable_name'],
                            {'stream_name':stream,'action':'create','config':copy.deepcopy(declaration)})
                    # Fixture setup only: signed business publisher writes three
                    # real messages after the old explicit-ACK durable exists.
                    for number in range(3):
                        name=runtime.create('seed-'+str(number),BOX_IMAGE,[(inputs,'/inputs',False)],
                            ['/bin/sh','-ec','exec nats --server nats://127.0.0.1:4222 --creds /inputs/social.creds '
                             'pub social.friend_removed "$1"','fixture','retained-'+str(number)],broker)
                        runtime.run(['start',name]);self.assertEqual(runtime.run(['wait',name]),'0')
                    live=census(runtime,broker,account)
                    self.assertEqual(sum(row['state']['messages'] for row in live['streams']),3)
                    social=next(row for row in live['consumers'] if row['name']=='rt_realtime1_friend_removed')
                    self.assertEqual(social['num_pending'],3)
                    streams={row['name']:{'config':actor.info_config(row['name'])} for row in live['streams']}
                    consumers={(stream,item['durable_name']):{'config':actor.info_config(stream+'/'+item['durable_name'])}
                               for stream,item in contract.DECLARATIONS}
                    plan=contract.compile_plan(source,streams,consumers,normalized,version=contract.V7_SCHEMA,
                                               normalized_reminder_stream=auth['normalized_reminder_stream'])
                    self.assertEqual([row['api'] for row in plan['actions']],
                        ['$JS.API.STREAM.CREATE.'+contract.REMINDER_STREAM,
                         '$JS.API.CONSUMER.CREATE.'+contract.REMINDER_STREAM+'.rt_realtime1_subscription_reminder'])
                    session.bind_target(c['commit'],c['images'],authority['observed']['images'],contract.digest(plan))
                    runtime.stop(broker)
                    def fence():
                        session.check()
                        self.assertFalse(runtime.inspect(broker)['State']['Running'])
                    cut_runtime=DockerRuntime(base,session.operation)
                    cut=preserve.capture_cut(cut_runtime,store,fence,live_catalog=live)
                    cut['manifest_sha256']=contract.digest(cut['manifest'])
                    archive=base/'rollout-before.tar';archive_sha=hashlib.sha256(archive.read_bytes()).hexdigest()
                    self.assertTrue(cut['closed_durable_states'])
                    old_files={row['path']:row for row in cut['manifest']['files']
                               if '/msgs/' in row['path'] or '/obs/' in row['path'] and row['path'].endswith('/o.dat')}
                    self.assertTrue(any('/obs/' in path for path in old_files))
                    results=[]
                    for mode in ('positive','lost-create-ack'):
                        target=base/mode;cut_runtime.restore(archive,target,cut['manifest'])
                        state={'operation':session.operation,'events':[],'custody':{'verified':True,'scope':'local-owned-fixture'},
                            'bootstrap_enrollment':{'auth_proof':auth},'nats_contract':copy.deepcopy(plan),'cut':copy.deepcopy(cut)}
                        state['nats_contract_binding']={'plan_sha256':contract.digest(plan),'server_image':NATS_IMAGE,
                            'enrollment_sha256':contract.digest(state['bootstrap_enrollment']),'sources':plan['sources']}
                        class Stage:
                            final_path=target;final_claim={};final_pv={}
                            def verify_final_storage(self):fence()
                            def selected_store_descriptor(self):fence();return None
                        def journal(event):
                            session.check();state['events'].append(copy.deepcopy(event));durable(base/(mode+'.json'),state)
                        if mode=='positive':
                            result=nats_migration.apply_contract(base,state,Stage(),journal,mutation_guard=session.check)
                            self.assertTrue(result['verified']);self.assertEqual(result['old_consumer_state_files_verified'],4)
                            after=result['cut'];contract.verify_census(plan,cut['census'],after['census'])
                            current={row['path']:row for row in after['manifest']['files']}
                            self.assertEqual({path:current[path] for path in old_files},old_files)
                            reminder=next(row for row in after['census']['streams'] if row['name']==contract.REMINDER_STREAM)
                            self.assertEqual(reminder['state']['messages'],0);self.assertEqual(reminder['state']['consumer_count'],1)
                            self.assertEqual(len(after['census']['streams']),len(cut['census']['streams'])+1)
                            self.assertEqual(len(after['census']['consumers']),len(cut['census']['consumers'])+1)
                            results.append({'mode':mode,'proof':result['proof'],'old_native_files':len(old_files)})
                        else:
                            real=Actor.mutate;calls=[]
                            def lose_reply(actor,api,body):
                                result=real(actor,api,body)
                                if api=='$JS.API.STREAM.CREATE.'+contract.REMINDER_STREAM:
                                    calls.append(api);raise RuntimeError('fixture_create_reply_lost')
                                return result
                            with patch.object(Actor,'mutate',lose_reply):
                                with self.assertRaisesRegex(RuntimeError,'fixture_create_reply_lost'):
                                    nats_migration.apply_contract(base,state,Stage(),journal,mutation_guard=session.check)
                            self.assertEqual(calls,['$JS.API.STREAM.CREATE.'+contract.REMINDER_STREAM])
                            with self.assertRaises((Blocked,contract.ContractError)):
                                nats_migration.apply_contract(base,state,Stage(),journal,mutation_guard=session.check)
                            issued=[row for row in state['events'] if row['kind']=='nats_contract_mutation_issued']
                            self.assertEqual(len(issued),1)
                            self.assertFalse(any(row['kind']=='nats_contract_applied' for row in state['events']))
                            results.append({'mode':mode,'issued':len(issued),'replayed':False,'adopted':False})
                        self.assertEqual(hashlib.sha256(archive.read_bytes()).hexdigest(),archive_sha)
                        self.assertTrue(verify_archive(archive,cut['manifest']))
                        cut_runtime.no_operation_containers()
                    print(json.dumps({'schema':'voice-reminder-native-fixture-v1','server_image':NATS_IMAGE,
                        'source_hashes':plan['sources'],'old_messages':3,'old_consumers':4,'results':results,
                        'original_archive_unchanged':True,'cleanup_verified':True,'live_admission':False},sort_keys=True),flush=True)
                finally:
                    for name in reversed(list(runtime.owned)):
                        row=runtime.inspect(name)
                        if row['State']['Running']:runtime.stop(name)
                        runtime.run(['rm',row['Id']])
                        self.assertFalse(runtime.run(['ps','-a','--filter','id='+row['Id'],'--format','{{.ID}}']))


if __name__=='__main__':unittest.main()
