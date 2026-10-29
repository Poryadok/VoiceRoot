"""Synthetic guarded records; real crypto/TT/Kube are covered separately."""
import copy
from types import SimpleNamespace
import unittest
from unittest import mock

import dev_peer_issuer_metadata as metadata
import space_creator_runtime_profile_delegated as delegated
import space_creator_runtime_profile_fixed9_successor as successor
import space_creator_runtime_profile_target_trust as target
import space_role_peer_create as role
import writer_cut


class Fixed9Test(unittest.TestCase):
    def test_exact_three_profile_lineage_and_unrelated_drift(self):
        row={}; rev=('uid','1'); token='token'; package=object(); lock={'uid':'lock'}
        context={'operation_id':'media-c1-'+'a'*32,'commit':'b'*40}; ca={'uid':'ca','resourceVersion':'1','der_sha256':'d'*64}
        trust=target.TargetTrustPhase.__new__(target.TargetTrustPhase); trust.package=package; trust.lock=lock; trust.namespace_uid='ns'
        trust.fence=SimpleNamespace(context=context); trust._guard=mock.Mock(); trust._local=mock.Mock(); trust._ledger=mock.Mock(return_value=(row,rev))
        phases=[]; snapshot={name:{'state':'absent'} for name in metadata.MEMBERS}
        for cls,profile in ((delegated.CreatorReceiver,'creator'),(delegated.SpaceReceiver,'space'),(role.Session,'role')):
            phase=cls.__new__(cls); phase.package=package; phase.lock=lock; phase.namespace_uid='ns'; phase.token=token
            phase.fence=SimpleNamespace(context=context,ledger=SimpleNamespace(read=mock.Mock(return_value=(row,rev))))
            phase.completed=True; phase.initial={'profile':profile,'ca':copy.deepcopy(ca),'creator_secrets':copy.deepcopy(snapshot)}
            phase.preflight=SimpleNamespace(profile=profile); phase.effects={}
            for name in metadata.PROFILES[profile]:
                uid='uid-'+name; phase.effects[name]={'status':'done','uid':uid,'resourceVersion':'1'}
                snapshot[name]={'state':'present','uid':uid,'resourceVersion':'1','keys':sorted(metadata.MEMBERS[name])}
            jwks={name:{'keys':[]} for name in metadata.PROFILES[profile] if name.endswith('-principal-signing')}
            phase.proof={'schema':'voice-dev-peer-receiver-v1','profile':profile,'operation_id':context['operation_id'],'source_sha':context['commit'],'namespace_uid':'ns',
                         'preflight_sha256':writer_cut.digest(phase.initial),'ca':copy.deepcopy(ca),
                         'objects':{name:{'uid':snapshot[name]['uid'],'resourceVersion':'1'} for name in metadata.PROFILES[profile]},
                         'public_jwks':jwks,'scope':'fixed-peer-secrets-no-runtime-readiness'}
            phase.record={'status':'done','receipt':copy.deepcopy(phase.proof)}; phase._guard=mock.Mock(return_value=(row,rev)); phase._local=mock.Mock()
            if profile!='role':
                phase.step='step-'+profile; phase.ledger='ledger-'+profile; phase.effects_key='effects-'+profile; phase._crypto=mock.Mock(return_value=jwks)
                step,ledger,effects=phase.step,phase.ledger,phase.effects_key
            else:step,ledger,effects=role.STEP,role.LEDGER,role.EFFECTS
            phase.journal=SimpleNamespace(state={'steps':{step:copy.deepcopy(phase.record)}})
            row[ledger]=copy.deepcopy(phase.record); row[effects]=copy.deepcopy(phase.effects); phases.append(phase)
        creator,space,last=phases; trust.preflight=creator.preflight
        creator.preflight._ca=mock.Mock(return_value=ca); creator.preflight._creator=mock.Mock(return_value=copy.deepcopy(snapshot))
        trust.journal=SimpleNamespace(state={'steps':{target.STEP:{'receipt':{'preflight_receipt_sha256':writer_cut.digest(creator.initial)}}}})
        with mock.patch.object(role,'verify_completed',return_value=last.proof):
            self.assertEqual(successor.verify(trust,token,*phases),creator.initial)
            creator.preflight._creator.return_value=copy.deepcopy(snapshot); creator.preflight._creator.return_value[next(iter(metadata.MEMBERS))]['resourceVersion']='drift'
            with self.assertRaises(target.metadata.GuardLost):successor.verify(trust,token,*phases)
            creator.preflight._creator.return_value=copy.deepcopy(snapshot); space.initial['creator_secrets'][next(iter(metadata.PROFILES['creator']))]['uid']='tampered'
            with self.assertRaises(target.metadata.GuardLost):successor.verify(trust,token,*phases)


if __name__=='__main__': unittest.main()
