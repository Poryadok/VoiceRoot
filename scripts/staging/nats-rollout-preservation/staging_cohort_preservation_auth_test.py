"""Actual isolated signed actor/bootstrap auth, with synthetic source/Kube/TT.

Fixture inputs are generated only by TestDockerFixtureInputs in a separately
owned test volume. No deployed signing key, credential, namespace or PVC.
"""
import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch

import staging_cohort_preservation_test as fixtures
import staging_cohort_preservation as owner
import staging_cohort_preservation_auth as auth
import actor_auth
import bootstrap_auth
import guard
import resource_claim
import commands
from docker_runtime import NATS_IMAGE


class ScratchBoundaryTests(unittest.TestCase):
    def test_foreign_command_and_lost_terminal_reply_never_replay(self):
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            before={'metadata':{'name':'voice-user','uid':'user'},'spec':{'replicas':1}}
            preflight={'schema':'voice-staging-cohort-preflight-v1','operation':'a'*12,
                'services':[],'actor':None,'nats':{},'contract':{},'enrollment':{}}
            with patch.object(guard,'ROOT',root),patch.object(owner,'_source'),\
                patch.object(auth.ScratchRuntime,'_inputs',return_value={'controlled_crypto_fixture':'a'*64}):
                session,_=fixtures.CutEffectTests().fixture(root,before,preflight=preflight)
                state=owner._read(owner._base(session)/'checkpoint.json');state['status']='PAUSED';owner.save(owner._base(session)/'checkpoint.json',state)
                base=root/'service-actor-auth-fixture';base.mkdir(mode=0o750);os.chown(base,0,65532)
                inputs=base/'inputs';inputs.mkdir(mode=0o750);os.chown(inputs,0,65532)
                with resource_claim.command_scope(session.check):
                    runtime=auth.ScratchRuntime(session,base,'actor')
                    calls=[]
                    def lost(argv,**kwargs):
                        commands.authority_guard(argv);calls.append(argv)
                        raise RuntimeError('terminal_reply_lost')
                    with patch.object(commands,'capture',side_effect=lost):
                        with self.assertRaisesRegex(Exception,'cohort_auth_command_invalid'):
                            runtime._run(['exec','foreign','/bin/sh'])
                        self.assertEqual(calls,[])
                        with self.assertRaisesRegex(RuntimeError,'terminal_reply_lost'):
                            runtime._run(['image','inspect',NATS_IMAGE])
                        with self.assertRaisesRegex(Exception,'cohort_.*reconciliation_required'):
                            runtime._run(['image','inspect',NATS_IMAGE])
                        self.assertEqual(len(calls),1)
                        self.assertIsNone(session._cohort_auth_permit)
                        with self.assertRaisesRegex(Exception,'cohort_auth_cleanup_unqualified'):
                            runtime.finish_auth()
                    intents=[owner._read(path) for path in (owner._base(session)/'effects').iterdir()]
                    self.assertEqual([row['status'] for row in intents],['PENDING'])


@unittest.skipUnless(os.environ.get('VOICE_COHORT_AUTH_INPUTS'),'isolated generated auth fixture required')
class ActualAuthTests(unittest.TestCase):
    def test_actual_actor_and_bootstrap_scratch_with_root_session(self):
        inputs=Path(os.environ['VOICE_COHORT_AUTH_INPUTS']);text=(inputs/'server.conf').read_text()
        value=lambda name:json.loads(re.search(r'^'+name+r': (.+)$',text,re.M)[1])
        operator_jwt=value('operator');system=value('system_account');resolver=value('resolver_preload')
        account=next(name for name in resolver if name!=system)
        def secret(name,data):
            return {'apiVersion':'v1','kind':'Secret','metadata':{'name':name,'namespace':'voice-staging','uid':name,'resourceVersion':'1'},
                'type':'Opaque','data':{key:base64.b64encode(raw).decode() for key,raw in data.items()}}
        operator=secret('voice-nats-operator-fixture',{key:raw.encode() for key,raw in {
            'operator.jwt':operator_jwt,'account.jwt':resolver[account],'system-account.jwt':resolver[system],
            'account.public':account,'system-account.public':system}.items()})
        actor=(inputs/'inputs'/'social.creds').read_bytes();bootstrap=(inputs/'inputs'/'bootstrap.creds').read_bytes()
        services=secret('voice-nats-service-credentials-fixture',{'social.creds':actor})
        bootstrap_secret=secret('voice-nats-bootstrap-fixture',{'bootstrap.creds':bootstrap})
        objects={row['metadata']['name']:row for row in (operator,services,bootstrap_secret)}
        preflight={'schema':'voice-staging-cohort-preflight-v1','operation':'a'*12,'services':['social'],
            'actor':{'roles':['social'],'binding':{'operator_body_sha256':owner.digest(operator),
                'services_body_sha256':owner.digest(services),'services':{'name':services['metadata']['name']}}},
            'nats':{},'contract':{},'enrollment':{'binding':{'generation':'fixture','operator_secret':{
                key:operator['metadata'][key] for key in ('name','uid','resourceVersion')}},'secret':{
                key:bootstrap_secret['metadata'][key] for key in ('name','uid','resourceVersion')}}}
        with tempfile.TemporaryDirectory(dir=os.environ['VOICE_COHORT_NATIVE_ROOT']) as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            before={'metadata':{'name':'voice-user','namespace':'voice-staging','uid':'user','resourceVersion':'1','generation':1},'spec':{'replicas':1}}
            with patch.object(guard,'ROOT',root),patch.object(owner,'_source'),\
                patch.object(auth.Kube,'get',side_effect=lambda kind,name:copy.deepcopy(objects[name])):
                session,_=fixtures.CutEffectTests().fixture(root,before,preflight=preflight)
                state=owner._read(owner._base(session)/'checkpoint.json');state['status']='PAUSED';owner.save(owner._base(session)/'checkpoint.json',state)
                binary=Path(__file__).with_name('bootstrap-renewer')
                self.assertEqual(hashlib.sha256(binary.read_bytes()).hexdigest(),owner.private.BINARY_PINS['nats-rollout-preservation/bootstrap-renewer'])
                with resource_claim.command_scope(session.check):
                    actor_proof=actor_auth.prove(actor,operator,binary,session.check)
                    self.assertTrue(actor_proof['server_authentication_verified'])
                    with patch.object(bootstrap_auth,'ACCOUNT_SHA',hashlib.sha256(account.encode()).hexdigest()):
                        bootstrap_proof=bootstrap_auth.prove(bootstrap,operator,session.check)
                    self.assertTrue(bootstrap_proof['verified']);owner._no_pending(session)
                    attempts=[owner._read(path) for path in (owner._base(session)/'auth').iterdir()]
                    self.assertEqual(len(attempts),2)
                    self.assertTrue(all(row['status']=='COMPLETE' and row['resources'] and
                        all(value['status']=='REMOVED' for value in row['resources'].values()) for row in attempts))


if __name__=='__main__':unittest.main()
