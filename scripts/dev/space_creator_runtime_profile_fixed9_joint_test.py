"""Three constructed receiver phases and real crypto; TT and Kubernetes are fixtures."""
import copy
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import dev_peer_issuer_metadata as metadata
import frontend_runtime_executor as transport
import full_cohort_package_phase as package
import media_operation_journal as journals
import resource_fencing
import reset_credentials as reset
import space_creator_runtime_profile_delegated as delegated
import space_creator_runtime_profile_delegated_test as fixture
import space_creator_runtime_profile_fixed9_successor as successor
import space_creator_runtime_profile_target_trust as target
import space_role_peer_create as role
import writer_cut


class ThreeProfileTest(unittest.TestCase):
    def test_three_constructed_preserve_phases_and_fixed9_drift(self):
        case=fixture.PhaseTest('test_unknown_issue_never_submits_kubernetes_write'); case.profile='creator'
        self.addCleanup(lambda:self.assertTrue(case.doCleanups())); case.setUp()
        material=reset.ResetMaterial(case.root); ca=(case.root/'fixture-ca.key',case.root/'fixture-ca.crt')
        for name in sorted(set(metadata.MEMBERS)-set(case.desired)):
            if name.endswith('-principal-signing'): material.signer(name,active=True)
            else: material.tls(name,ca,*delegated.packet.TLS[name])
        for obj in material.items:case.desired[obj['metadata']['name']]=copy.deepcopy(obj)
        for name,obj in case.desired.items():
            obj=copy.deepcopy(obj); obj['metadata'].update(uid='uid-'+name,resourceVersion='1'); case.actual[name]=obj
            case.identities[name]={'state':'present','uid':'uid-'+name,'resourceVersion':'1','keys':sorted(metadata.MEMBERS[name])}
        case.package.journal.state['steps'][package.STEP]={'status':'done','receipt':{'fixture':True}}
        def set_initial(pre,profile):
            initial={'profile':profile,'ca':copy.deepcopy(case.ca),'creator_secrets':copy.deepcopy(case.identities)}
            pre.record={'status':'done','ca':copy.deepcopy(case.ca),'creator_secrets':copy.deepcopy(case.identities),
                        'receipt_sha256':writer_cut.digest(initial)}
            pre.journal.state['steps'][pre.step]=copy.deepcopy(pre.record)|{'receipt':copy.deepcopy(initial)}
            case.base.ledger.row[pre.ledger]=copy.deepcopy(pre.record)
            return initial
        set_initial(case.preflight,'creator')
        client_type=type(case.base.client); backing=case.base.ledger
        class Ledger:
            def __init__(self,client):self.client=client
            def read(self):return backing.read()
            def write(self,revision,row):return backing.write(revision,row)
        def attach(name,intent):
            journal=journals.Journal(case.root/name,intent); self.addCleanup(journal.close)
            client=client_type(case.base.lock,journal); client.workloads=case.base.client.workloads
            fence=resource_fencing.Fence(case.base.context,lambda:copy.deepcopy(case.base.card),Ledger(client))
            return journal,client,fence
        base={'resource_context':copy.deepcopy(case.base.context),'namespace_uid':'ns'}
        def fresh(profile):
            intent=base|{'dev_peer_issuer_preflight':{'source_root':str(case.root),'source_sha':case.base.context['commit'],'profile':profile}}
            j,c,f=attach('pre-'+profile,intent)
            pre=delegated.preflight.PeerIssuerPreflightPhase(j,f,c,case.base.lock,'ns',case.package,case.base.cut,str(case.root),'token',profile)
            pre._ca=Mock(side_effect=lambda:copy.deepcopy(case.ca)); pre._creator=Mock(side_effect=lambda:copy.deepcopy(case.identities))
            set_initial(pre,profile); return pre
        with patch.object(role.canonical,'ROOT',case.root),patch.object(role.source,'verify'), \
             patch.object(role,'_get',side_effect=lambda name:copy.deepcopy(case.actual.get(name))):
            creator=case.phase; creator.run()
            pre=fresh('space'); intent=base|{'peer_receiver':{'profile':'space','source_sha':case.base.context['commit']}}
            j,c,f=attach('space-receiver',intent)
            space=delegated.SpaceReceiver(j,f,c,case.base.lock,'ns',case.package,case.base.cut,pre,case.root,'token'); space.run()
            pre=fresh('role'); j,c,f=attach('role-receiver',base)
            role_phase=role.Session(j,f,c,case.base.lock,'ns',case.package,pre,case.root,'token')
            self.assertEqual(role_phase.prepare()['status'],'done')
            trust=target.TargetTrustPhase.__new__(target.TargetTrustPhase)
            trust.package=case.package; trust.lock=case.base.lock; trust.namespace_uid='ns'; trust.fence=creator.fence
            trust.preflight=creator.preflight; trust._guard=Mock(); trust._local=Mock(); trust._ledger=backing.read
            trust.journal=SimpleNamespace(state={'steps':{target.STEP:{'receipt':{'preflight_receipt_sha256':writer_cut.digest(creator.initial)}}}})
            self.assertEqual(successor.verify(trust,'token',creator,space,role_phase),creator.initial)
            case.identities['voice-space-principal-signing']['resourceVersion']='drift'
            with self.assertRaises(target.metadata.GuardLost):successor.verify(trust,'token',creator,space,role_phase)
        self.assertEqual(case.writes,[])
