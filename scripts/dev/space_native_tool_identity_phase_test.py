import copy
import json
from pathlib import Path
import unittest
from unittest.mock import patch

import full_cohort_package_phase as package_phase
import full_cohort_package_phase_test as package_fixture
import media_backup_phase_test as fixture
import media_operation_journal as journal
import resource_fencing
import space_native_tool_identity as tool


class ToolPhaseTest(unittest.TestCase):
    def setUp(self):
        self.pkg=package_fixture.PackagePhaseTest('test_real_journal_and_ledger_one_shot')
        self.pkg.setUp(); self.addCleanup(self.pkg.doCleanups)
        patches=[patch.object(package_phase,'ROOT',self.pkg.root.parent),
                 patch.object(package_phase.package,'capture',side_effect=self.pkg.capture),
                 patch.object(package_phase.oci,'resolve',return_value=self.pkg.platforms),
                 patch.object(package_phase.source,'Registry')]
        for p in patches: p.start(); self.addCleanup(p.stop)
        self.package_receipt=self.pkg.phase.run('token','actor')
        base=self.pkg.base
        intent={'resource_context':copy.deepcopy(base.context),'namespace_uid':'ns'}
        self.journal=journal.Journal(self.pkg.root/'tool-journal',intent); self.addCleanup(self.journal.close)
        self.client=fixture.Client(base.lock); self.client.journal=self.journal
        def transport(fallback):
            def call(*args,**kwargs):
                if len(args)>2 and args[1]=='configmap' and args[2]==resource_fencing.LEDGER:
                    if args[0]=='get':
                        row,revision=base.ledger.read()
                        return {'metadata':{'name':resource_fencing.LEDGER,'namespace':'voice-dev','uid':revision[0],'resourceVersion':revision[1]},
                                'data':{'executor':json.dumps(row,sort_keys=True,separators=(',',':'))}}
                    if args[0]=='patch':
                        body=kwargs['body']; old=base.ledger.read()[1]
                        if body[0]['value']!=old[0] or body[1]['value']!=old[1]: raise RuntimeError('CAS changed')
                        data=body[2]['value']; base.ledger.write(old,json.loads(data['executor']))
                        return {'metadata':{'uid':old[0]},'data':data}
                return fallback(*args,**kwargs)
            return call
        self.client.call=transport(self.client.call)
        self.pkg.client.call=transport(self.pkg.client.call)
        self.pkg.phase.fence.ledger=resource_fencing.KubeLedger(self.pkg.client)
        self.ledger=resource_fencing.KubeLedger(self.client)
        self.fence=resource_fencing.Fence(base.context,lambda:copy.deepcopy(base.card),self.ledger)
        self.phase=tool.SpaceNativeToolPhase(self.journal,self.fence,self.client,base.lock,'ns',self.pkg.phase,'actor')
        source=base.context['commit']; index='sha256:'+'d'*64; platform='sha256:'+'e'*64; config='sha256:'+'f'*64
        inventory={'source_sha':source,'build_source_sha256':'1'*64,'sql_inventory_sha256':'2'*64,
                   'dockerfile_sha256':'3'*64,'source_manifest_sha256':'4'*64,'sql_files':{'1.sql':'5'*64}}
        identity={'source_sha':source,'run_id':123,'run_attempt':1,
                  'tag':'ghcr.io/poryadok/voiceroot/space-native-installer:source-'+source+'-123-1'}
        publication={'schema':'voice-space-native-tool-publication-v1',**identity,'index_digest':index,'platform_digest':platform,
            'config_digest':config,'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@'+platform,
            'source_inventory':inventory,'request':self.pkg.receipt['request'],
            'content':{'executable_sha256':'6'*64,'migrator_sha256':'7'*64,'sql_inventory_sha256':'2'*64,
                       'embedded_values_observed':True,'runtime_receipt_verified':False,'help_exit_code':0,
                       'compiled_gate_probe':'native51-source-gates-reached-v1',
                       'closure':{'schema':'voice-space-native-local-closure-v1','packages':{},'files':{},'raw_sha256':'9'*64}},
            'runtime_qualified':False,'deployment_performed':False}
        self.capture={'artifact_id':42,'artifact_sha256':'8'*64,'publication':publication}

    def test_one_shot_receipt_requires_same_protected_package(self):
        with patch.object(tool,'authenticated_publication',return_value=copy.deepcopy(self.capture)):
            result=self.phase.run('token')
            self.assertEqual(result['source_sha'],self.pkg.base.context['commit'])
            self.assertEqual(tool.verify_completed(self.phase,'token'),result)
            with self.assertRaises(tool.ToolIdentityError): self.phase.run('token')
            self.journal.state['steps']['space-native-tool']['receipt']['executable_sha256']='0'*64
            with self.assertRaises(tool.ToolIdentityError): tool.verify_completed(self.phase,'token')

    def test_lost_pending_reply_is_uncertain_and_no_capture(self):
        write=self.ledger.write
        def lost(revision,row):
            write(revision,row)
            if 'space_native_tool' in row: raise OSError('private sentinel')
        with patch.object(self.ledger,'write',side_effect=lost),patch.object(tool,'authenticated_publication') as capture:
            with self.assertRaises(tool.ToolIdentityError): self.phase.run('token')
            capture.assert_not_called()
        self.assertTrue(self.phase.poisoned)
        self.assertEqual(self.pkg.base.ledger.row['status'],'uncertain')

    def test_completed_reader_detects_package_drift_during_artifact_read(self):
        with patch.object(tool,'authenticated_publication',return_value=copy.deepcopy(self.capture)):
            self.phase.run('token')
        def drift(*args):
            self.pkg.journal.state['steps']['full-cohort-package']['receipt']['run_id']=999
            return copy.deepcopy(self.capture)
        with patch.object(tool,'authenticated_publication',side_effect=drift),self.assertRaises(tool.ToolIdentityError):
            tool.verify_completed(self.phase,'token')

    def test_wrong_tool_source_is_uncertain_and_never_completed(self):
        bad=copy.deepcopy(self.capture); bad['publication']['source_inventory']['source_sha']='b'*40
        with patch.object(tool,'authenticated_publication',return_value=bad),self.assertRaises(tool.ToolIdentityError):
            self.phase.run('token')
        self.assertTrue(self.phase.poisoned)
        self.assertEqual(self.pkg.base.ledger.row['status'],'uncertain')

    def test_completed_reader_rejects_closed_own_journal(self):
        with patch.object(tool,'authenticated_publication',return_value=copy.deepcopy(self.capture)):
            self.phase.run('token'); self.journal.close()
            with self.assertRaises(tool.ToolIdentityError): tool.verify_completed(self.phase,'token')

    def test_completed_reader_rejects_package_journal_closed_during_artifact(self):
        with patch.object(tool,'authenticated_publication',return_value=copy.deepcopy(self.capture)):
            self.phase.run('token')
        def close_package(*args):
            self.pkg.journal.close(); return copy.deepcopy(self.capture)
        with patch.object(tool,'authenticated_publication',side_effect=close_package),self.assertRaises(tool.ToolIdentityError):
            tool.verify_completed(self.phase,'token')

    def test_completed_reader_rejects_own_journal_closed_during_artifact(self):
        with patch.object(tool,'authenticated_publication',return_value=copy.deepcopy(self.capture)):
            self.phase.run('token')
        def close_own(*args):
            self.journal.close(); return copy.deepcopy(self.capture)
        with patch.object(tool,'authenticated_publication',side_effect=close_own),self.assertRaises(tool.ToolIdentityError):
            tool.verify_completed(self.phase,'token')

    def test_completed_reader_rejects_closed_package_journal(self):
        with patch.object(tool,'authenticated_publication',return_value=copy.deepcopy(self.capture)):
            self.phase.run('token'); self.pkg.journal.close()
            with self.assertRaises(tool.ToolIdentityError): tool.verify_completed(self.phase,'token')

    def test_completed_reader_rejects_ledger_replacement_during_artifact(self):
        with patch.object(tool,'authenticated_publication',return_value=copy.deepcopy(self.capture)):
            self.phase.run('token')
        def replace(*args):
            self.fence.ledger=resource_fencing.KubeLedger(self.client)
            return copy.deepcopy(self.capture)
        with patch.object(tool,'authenticated_publication',side_effect=replace),self.assertRaises(tool.ToolIdentityError):
            tool.verify_completed(self.phase,'token')


if __name__=='__main__': unittest.main()
