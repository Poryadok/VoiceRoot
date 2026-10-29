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
            raw=b'{"fixture":true}';(payloads/'gis-signer.json').write_bytes(raw);(payloads/'gis-signer.json').chmod(0o600)
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
                (payloads/'gis-signer.json').write_bytes(b'{"fixture":false}')
                with self.assertRaisesRegex(prereq.Blocked,'payload_changed'):
                    prereq.load_payload(session,'gis_material','gis-signer')
                with self.assertRaises(prereq.Blocked):prereq.load_payload(session,'foreign','gis-signer')
                source['approved_sha256']='0'*64;save(installed/'source-authority'/('a'*12+'.json'),source)
                with self.assertRaisesRegex(prereq.Blocked,'plan_invalid'):prereq.require_child(session,'gis_material')

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
                'policy_before_sha256':'1'*64,'children':{slot:{'effects':[{'id':slot.replace('_','-')+'-effect'}]} for slot in prereq.SLOTS},
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
                with self.assertRaises(prereq.Blocked):prereq.record_pending(session,'release_policy','release-policy-effect')
                prereq.preflight_all(session)
                self.assertEqual(calls,[('preflight',s) for s in prereq.SLOTS])
                with self.assertRaises(prereq.Blocked):prereq.record_pending(session,'p3','p3-effect')
                prereq.record_pending(session,'release_policy','release-policy-effect')
                with self.assertRaises(prereq.Blocked):prereq.record_pending(session,'release_policy','release-policy-effect')
                observed={'uid':'fixture'};prereq.record_done(session,'release_policy','release-policy-effect',observed)
                state=json.loads((base/'checkpoint.json').read_bytes())
                self.assertEqual(state['steps']['release_policy']['release-policy-effect'],{'status':'DONE','observation':observed})
                self.assertEqual(calls[-1],('verify','release_policy','release-policy-effect'))
                prereq.record_pending(session,'p3','p3-effect')


if __name__=='__main__':unittest.main()
