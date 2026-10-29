import copy
import hashlib
import json
import tempfile
import unittest
import sys
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import guard
import resource_claim
import resource_claim_test
import staging_prerequisites as prereq
from root_main import save


class PrivateCustodyTests(unittest.TestCase):
    def test_initial_capture_requires_real_session_and_exclusive_operation(self):
        import bridge_root
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            root=Path(folder);installed=root/'installed'
            for path in (installed/'source-authority',installed/'prerequisites',installed/'code'/'helper',
                         installed/'sources'/('a'*12)/'scripts'/'staging'/'helper'):
                path.mkdir(parents=True,mode=0o700)
            raw=b'fixture';name='helper/fixture.py';wanted=hashlib.sha256(raw).hexdigest()
            for path in (installed/'code'/name,installed/'sources'/('a'*12)/'scripts'/'staging'/name):
                path.write_bytes(raw);path.chmod(0o600)
            binding={name:wanted};save(installed/'code'/'capture-manifest.json',binding)
            context=resource_claim_test.context();card=resource_claim_test.card(context)
            session=resource_claim.Session(context,'a'*12,prereq.digest(binding),lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal());session.admit(start=True)
            approved={'source_sha':context['commit'],'source_files':{'scripts/staging/'+name:wanted}}
            execution={'fixture':'execution'}
            receipt={'schema':'voice-staging-source-capture-v1','operation':session.operation,'context':context,
                     'helper_sha256':session.helper,'approved_sha256':prereq.digest(approved),'execution_sha256':prereq.digest(execution)}
            save(installed/'source-authority'/('a'*12+'.json'),receipt)
            actions=bridge_root.Actions(installed/'code');actions.tt_session=session
            with patch.object(guard,'ROOT',root),patch.object(prereq,'REQUIRED_HELPERS',frozenset((name,))),patch.object(prereq,'BINARY_PINS',{}):
                with self.assertRaises(prereq.Blocked):prereq._capture_authority(actions,session,approved,execution)
                with resource_claim.command_scope(session.check):
                    self.assertEqual(prereq._capture_authority(actions,session,approved,execution),prereq.digest(receipt))
                    with self.assertRaises(FileExistsError):prereq._capture_authority(actions,session,approved,execution)

    def test_effect_order_is_closed_and_allows_preserved_only_slots(self):
        plan={'children':{slot:{'effects':[]} for slot in prereq.SLOTS}}
        plan['children']['p3']['effects']=[{'id':prereq.EFFECT_IDS['p3'][0]},
                                           {'id':prereq.EFFECT_IDS['p3'][-1]}]
        self.assertEqual(prereq._effect_order(plan),[('p3',prereq.EFFECT_IDS['p3'][0]),
                                                     ('p3',prereq.EFFECT_IDS['p3'][-1])])
        for ids in ([prereq.EFFECT_IDS['p3'][0]]*2,
                    [prereq.EFFECT_IDS['p3'][-1],prereq.EFFECT_IDS['p3'][0]],
                    ['Secret/../../foreign']):
            plan['children']['p3']['effects']=[{'id':ident} for ident in ids]
            with self.assertRaises(prereq.Blocked):prereq._effect_order(plan)

    def test_root_source_and_exact_payload_are_bound_to_session(self):
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            root=Path(folder);installed=root/'installed';(installed/'source-authority').mkdir(parents=True,mode=0o700)
            base=installed/'prerequisites'/('a'*12);payloads=base/'payloads'/'gis_material';payloads.mkdir(parents=True,mode=0o700)
            context=resource_claim_test.context();card=resource_claim_test.card(context)
            session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal())
            session.admit(start=True)
            source={'schema':'voice-staging-source-capture-v1','operation':session.operation,
                'context':context,'helper_sha256':session.helper,'approved_sha256':'c'*64,'execution_sha256':'d'*64}
            save(installed/'source-authority'/('a'*12+'.json'),source)
            raw=b'{"fixture":true}';payload=payloads/(hashlib.sha256(b'gis-signer').hexdigest()+'.json');payload.write_bytes(raw);payload.chmod(0o600)
            effect={'id':'gis-signer','payload_sha256':hashlib.sha256(raw).hexdigest()}
            plan={'schema':'voice-staging-prerequisites-plan-v1','operation':session.operation,'context':context,
                'helper_sha256':session.helper,'source_capture_sha256':prereq.digest(source),
                'approved_sha256':'c'*64,'execution_sha256':'d'*64,'namespace_uid':'namespace',
                'marker_sha256':'e'*64,'baseline_sha256':'f'*64,'actual_before':card['observed']['images'],
                'policy_before_sha256':'1'*64,'children':{'release_policy':{},'p3':{},'gis_material':{'effects':[effect]}},
                'auxiliary_images':{},'renderer_authority_sha256':None}
            with patch.object(guard,'ROOT',root):
                binding=session.bind_prerequisites(plan,plan['actual_before'])
                state={'schema':'voice-staging-prerequisites-checkpoint-v1','plan':plan,'initial_resource_fence':binding,
                    'status':'RUNNING','steps':{},'receipt':None,'preflight_sha256':None}
                save(base/'checkpoint.json',state)
                self.assertEqual(prereq.load_payload(session,'gis_material','gis-signer'),raw)
                payload.write_bytes(b'{"fixture":false}')
                with self.assertRaisesRegex(prereq.Blocked,'payload_changed'):
                    prereq.load_payload(session,'gis_material','gis-signer')
                with self.assertRaises(prereq.Blocked):prereq.load_payload(session,'foreign','gis-signer')
                source['approved_sha256']='0'*64;save(installed/'source-authority'/('a'*12+'.json'),source)
                with self.assertRaisesRegex(prereq.Blocked,'plan_invalid'):prereq.require_child(session,'gis_material')

    def test_source_authority_loader_requires_exact_protected_capture(self):
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            root=Path(folder);installed=root/'installed';(installed/'source-authority').mkdir(parents=True,mode=0o700)
            base=installed/'prerequisites'/('a'*12);base.mkdir(parents=True,mode=0o700)
            workspace=installed/'sources'/('a'*12);workspace.mkdir(parents=True,mode=0o700)
            code=installed/'code';(code/'nats-rollout-preservation').mkdir(parents=True,mode=0o700)
            source_file=workspace/'scripts'/'staging'/'nats-rollout-preservation'/'fixture.py'
            source_file.parent.mkdir(parents=True,mode=0o700)
            source_file.write_bytes(b'fixture');source_file.chmod(0o600)
            helper=code/'nats-rollout-preservation'/'fixture.py';helper.write_bytes(b'fixture');helper.chmod(0o600)
            name='nats-rollout-preservation/fixture.py';manifest={name:hashlib.sha256(b'fixture').hexdigest()}
            save(code/'capture-manifest.json',manifest)
            code_stat=code.stat();save(base/'code-custody.json',{'root':str(code),'manifest':manifest,
                'directory':{'dev':code_stat.st_dev,'ino':code_stat.st_ino}})
            context=resource_claim_test.context();card=resource_claim_test.card(context)
            session=resource_claim.Session(context,'a'*12,prereq.digest(manifest),lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal());session.admit(start=True)
            approved={'source_sha':context['commit'],'source_files':{'scripts/staging/'+name:manifest[name]},'images':{'fixture':'g'}}
            execution={'fixture':'execution'}
            source={'schema':'voice-staging-source-capture-v1','operation':session.operation,'context':context,
                    'helper_sha256':session.helper,'approved_sha256':prereq.digest(approved),'execution_sha256':prereq.digest(execution)}
            save(installed/'source-authority'/('a'*12+'.json'),source);save(base/'approved.json',approved);save(base/'execution.json',execution)
            plan={'schema':'voice-staging-prerequisites-plan-v1','operation':session.operation,'context':context,'helper_sha256':session.helper,
                'source_capture_sha256':prereq.digest(source),'approved_sha256':prereq.digest(approved),'execution_sha256':prereq.digest(execution),
                'namespace_uid':'namespace','marker_sha256':'e'*64,'baseline_sha256':'f'*64,'actual_before':card['observed']['images'],
                'policy_before_sha256':'1'*64,'children':{slot:{'effects':[{'id':slot.replace('_','-')+'-effect'}]} for slot in prereq.SLOTS},
                'auxiliary_images':{},'renderer_authority_sha256':None}
            with patch.object(guard,'ROOT',root),patch.object(prereq,'REQUIRED_HELPERS',frozenset((name,))),patch.object(prereq,'BINARY_PINS',{}):
                binding=session.bind_prerequisites(plan,plan['actual_before'])
                save(base/'checkpoint.json',{'schema':'voice-staging-prerequisites-checkpoint-v1','plan':plan,'initial_resource_fence':binding,
                    'status':'RUNNING','steps':{},'receipt':None,'preflight_sha256':None})
                value=prereq.load_authority(session)
                self.assertEqual(value,{'approved':approved,'execution':execution,'source':str(workspace)})
                source_file.write_bytes(b'drift')
                with self.assertRaises(prereq.Blocked):prereq.load_authority(session)
                source_file.write_bytes(b'fixture')
                helper.write_bytes(b'drift')
                with self.assertRaises(prereq.Blocked):prereq.load_authority(session)
                helper.write_bytes(b'fixture')
                approved['source_sha']='0'*40;save(base/'approved.json',approved)
                with self.assertRaises(prereq.Blocked):prereq.load_authority(session)

    def test_fixed_preflight_and_effect_order_use_real_session_and_durable_intent(self):
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            root=Path(folder);installed=root/'installed';(installed/'source-authority').mkdir(parents=True,mode=0o700)
            base=installed/'prerequisites'/('a'*12);base.mkdir(parents=True,mode=0o700)
            context=resource_claim_test.context();card=resource_claim_test.card(context)
            session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal());session.admit(start=True)
            source={'schema':'voice-staging-source-capture-v1','operation':session.operation,'context':context,
                    'helper_sha256':session.helper,'approved_sha256':'c'*64,'execution_sha256':'d'*64}
            save(installed/'source-authority'/('a'*12+'.json'),source)
            plan={'schema':'voice-staging-prerequisites-plan-v1','operation':session.operation,'context':context,'helper_sha256':session.helper,
                'source_capture_sha256':prereq.digest(source),'approved_sha256':'c'*64,'execution_sha256':'d'*64,
                'namespace_uid':'namespace','marker_sha256':'e'*64,'baseline_sha256':'f'*64,'actual_before':card['observed']['images'],
                'policy_before_sha256':'1'*64,'children':{slot:{'effects':[{'id':prereq.EFFECT_IDS[slot][0]}]} for slot in prereq.SLOTS},
                'auxiliary_images':{},'renderer_authority_sha256':None}
            calls=[]
            def child(slot):
                return SimpleNamespace(preflight_payloads=lambda session:calls.append(('preflight',slot)),
                    verify_effect=lambda session,effect,obs:(calls.append(('verify',slot,effect)) or copy.deepcopy(obs)))
            modules={name:child(slot) for slot,name in prereq.CHILD_MODULES.items()}
            with patch.object(guard,'ROOT',root),patch.dict(sys.modules,modules):
                binding=session.bind_prerequisites(plan,plan['actual_before'])
                state={'schema':'voice-staging-prerequisites-checkpoint-v1','plan':plan,'initial_resource_fence':binding,
                    'status':'RUNNING','steps':{},'receipt':None,'preflight_sha256':None};save(base/'checkpoint.json',state)
                with self.assertRaises(prereq.Blocked):prereq.record_pending(session,'release_policy','create-policy-configmap')
                prereq.preflight_all(session)
                self.assertEqual(calls,[('preflight',s) for s in prereq.SLOTS])
                with self.assertRaises(prereq.Blocked):prereq.record_pending(session,'p3','Secret/voice-realtime-principal-signing')
                prereq.record_pending(session,'release_policy','create-policy-configmap')
                with self.assertRaises(prereq.Blocked):prereq.record_pending(session,'release_policy','create-policy-configmap')
                observed={'uid':'fixture'};prereq.record_done(session,'release_policy','create-policy-configmap',observed)
                state=json.loads((base/'checkpoint.json').read_bytes())
                self.assertEqual(state['steps']['release_policy']['create-policy-configmap'],{'status':'DONE','observation':observed})
                self.assertEqual(calls[-1],('verify','release_policy','create-policy-configmap'))
                modules['staging_release_policy_prepare'].verify_effect=lambda *_: (_ for _ in ()).throw(ValueError('fixture-drift'))
                with self.assertRaisesRegex(ValueError,'fixture-drift'):prereq.record_pending(session,'p3','Secret/voice-realtime-principal-signing')
                modules['staging_release_policy_prepare'].verify_effect=lambda session,effect,obs:copy.deepcopy(obs)
                prereq.record_pending(session,'p3','Secret/voice-realtime-principal-signing')


if __name__=='__main__':unittest.main()
