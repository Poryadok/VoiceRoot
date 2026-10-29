import copy
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import commands
import gis_existing_update
import gis_first_install
import normalize
import resource_claim
import resource_claim_test
import staging_cohort_preservation as cohort
import staging_prerequisites as prereq


class PreservedGISNormalizeTest(unittest.TestCase):
    def test_unselected_pair_uses_original_ready_spec_while_live_is_paused(self):
        uid='11111111-1111-4111-8111-111111111111';service_uid='22222222-2222-4222-8222-222222222222'
        meta={'name':'voice-gameintegration','namespace':'voice-staging','uid':uid}
        original={'apiVersion':'apps/v1','kind':'Deployment','metadata':meta,'spec':{'replicas':1}}
        paused=copy.deepcopy(original);paused['spec']['replicas']=0
        service={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-gameintegration','namespace':'voice-staging','uid':service_uid},'spec':{}}
        stage=SimpleNamespace(expected={'source_claim':'claim'},snapshots={'voice-gameintegration':paused},original_snapshots={'voice-gameintegration':original})
        kube=SimpleNamespace(get=lambda kind,name:copy.deepcopy(service))
        binding={'service_uid':service_uid,'service':gis_first_install.semantic(service)};selected=[]
        target={'images':{},'template_hashes':{}}
        with patch.object(normalize,'_verify_gis_normalization',return_value=True):
            normalize.normalize_target(kube,stage,[],target,gis_binding=binding,gis_target_sink=lambda dep,svc,chosen:selected.append((dep,chosen)))
        self.assertEqual(selected[0][0]['spec']['replicas'],1);self.assertFalse(selected[0][1])

    def test_paused_pair_requires_protected_captured_stage(self):
        context=resource_claim_test.context();card=resource_claim_test.card(context);journal=resource_claim_test.MemoryJournal()
        session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),journal)
        session.admit(start=True);journal.row.update(phase='cohort-preservation',target_sha256='c'*64,cohort_preservation_sha256='c'*64)
        original={'metadata':{'uid':'gis','generation':1},'spec':{'replicas':1}}
        paused=copy.deepcopy(original);paused['metadata']['generation']=2;paused['spec']['replicas']=0
        stage=SimpleNamespace(snapshots={'voice-gameintegration':paused},original_snapshots={'voice-gameintegration':original},
                              marker={'metadata':{'name':'marker'},'data':{}})
        binding={'fixture':'qualified'};kube=object()
        with patch.object(commands,'authority_guard',session.check),patch.object(cohort,'capture_final_stage',return_value=stage), \
             patch.object(prereq,'load_gis_authority',return_value=binding),patch.object(gis_existing_update,'verify_phase') as verify:
            normalize._verify_gis_normalization(kube,stage,binding)
            verify.assert_called_once_with(binding,kube,'paused-old')
            altered=copy.deepcopy(stage);altered.snapshots['voice-gameintegration']['spec']['replicas']=1
            with self.assertRaisesRegex(normalize.Blocked,'gis_existing_normalization_binding_changed'):
                normalize._verify_gis_normalization(kube,altered,binding)


if __name__=='__main__':unittest.main()
