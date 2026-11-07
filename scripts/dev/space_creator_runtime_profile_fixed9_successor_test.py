"""Synthetic guarded records; real crypto/TT/Kube are covered separately."""
import copy
import hashlib
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

import dev_peer_issuer_metadata as metadata
import full_cohort_package_phase as package_module
import space_creator_runtime_profile_delegated as delegated
import space_creator_runtime_profile_fixed9_successor as successor
import space_creator_runtime_profile_target_trust as target
import space_role_peer_create as role
import writer_cut
import dev_peer_issuer_material as issuer
import reset_credentials as reset


class Fixed9Test(unittest.TestCase):
    @unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'isolated Linux root fixture')
    def test_current_crypto_accepts_later_profile_changes_without_old_current_check(self):
        with tempfile.TemporaryDirectory(dir='/root',prefix='fixed9-crypto-') as temporary:
            root=Path(temporary); archive=root/'archive'; archive.mkdir(mode=0o700)
            key,cert=reset.ResetMaterial(root).ca('fixture')
            (archive/'media-ca.crt').write_bytes(cert.read_bytes()); (archive/'media-ca.key').write_bytes(key.read_bytes())
            (archive/'media-ca.key').chmod(0o600); media=cert.read_bytes()
            der=delegated.packet.crypto._openssl(['x509','-in','{0}','-outform','DER'],[media])
            ca={'uid':'ca','resourceVersion':'1','der_sha256':hashlib.sha256(der).hexdigest()}
            op='media-c1-'+'a'*32; source='b'*40; objects={}
            with mock.patch.object(issuer.custody,'ARCHIVE',archive):
                for profile in ('creator','space'):
                    directory=root/profile; directory.mkdir(mode=0o700)
                    issuer.generate(directory,media,profile,sorted(metadata.PROFILES[profile]),op,source,'ns','c'*64)
                    envelope=json.loads((directory/'objects.json').read_bytes())
                    for obj in envelope['objects']['items']:
                        name=obj['metadata']['name']; obj['metadata'].update(uid='uid-'+name,resourceVersion='1'); objects[name]=obj
            final={name:{'state':'present','uid':'uid-'+name,'resourceVersion':'1','keys':sorted(keys)} for name,keys in metadata.MEMBERS.items()}
            with mock.patch.object(delegated,'_get',side_effect=lambda name,profile:copy.deepcopy(objects[name])):
                for cls,profile in ((delegated.CreatorReceiver,'creator'),(delegated.SpaceReceiver,'space')):
                    phase=cls.__new__(cls); phase.initial={'profile':profile,'ca':ca,'creator_secrets':{n:{'state':'absent'} for n in metadata.MEMBERS}}
                    phase.media=media; phase.fence=SimpleNamespace(context={'operation_id':op,'commit':source}); phase.namespace_uid='ns'
                    phase.preflight=SimpleNamespace(_ca=mock.Mock(return_value=ca),_creator=mock.Mock(return_value=copy.deepcopy(final)))
                    phase._guard=mock.Mock(); phase._current=mock.Mock(side_effect=AssertionError('old snapshot must not be used'))
                    jwks=successor._current_crypto(phase,final,ca)
                    self.assertEqual(set(jwks),{n for n in metadata.PROFILES[profile] if n.endswith('-principal-signing')})
                    phase._current.assert_not_called()

    def test_exact_three_profile_lineage_and_unrelated_drift(self):
        row={}; rev=('uid','1'); token='token'; package=SimpleNamespace(journal=SimpleNamespace(state={'steps':{package_module.STEP:{'status':'done','receipt':{'safe':True}}}})); lock={'uid':'lock'}
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
        with mock.patch.object(role,'verify_completed',return_value=last.proof), mock.patch.object(successor,'_current_crypto',side_effect=lambda phase,*args:phase.proof['public_jwks']):
            self.assertEqual(successor.verify(trust,token,*phases),creator.initial)
            creator.preflight._creator.return_value=copy.deepcopy(snapshot); creator.preflight._creator.return_value[next(iter(metadata.MEMBERS))]['resourceVersion']='drift'
            with self.assertRaises(target.metadata.GuardLost):successor.verify(trust,token,*phases)
            creator.preflight._creator.return_value=copy.deepcopy(snapshot)
            space.initial=copy.deepcopy(space.initial); space.initial['creator_secrets']=copy.deepcopy(creator.initial['creator_secrets'])
            for name,effect in creator.effects.items():
                space.initial['creator_secrets'][name]={'state':'present','uid':effect['uid'],'resourceVersion':effect['resourceVersion'],'keys':sorted(metadata.MEMBERS[name])}
            def delete_package():
                package.journal.state['steps'].pop(package_module.STEP,None); return row,rev
            last.fence.ledger.read=mock.Mock(side_effect=delete_package)
            with self.assertRaises(target.metadata.GuardLost):successor.verify(trust,token,*phases)
            last.fence.ledger.read.assert_called_once()
            package.journal.state['steps'][package_module.STEP]={'status':'done','receipt':{'safe':True}}
            last.fence.ledger.read=mock.Mock(return_value=(row,rev))
            space.initial['creator_secrets'][next(iter(metadata.PROFILES['creator']))]['uid']='tampered'
            with self.assertRaises(target.metadata.GuardLost):successor.verify(trust,token,*phases)


if __name__=='__main__': unittest.main()
