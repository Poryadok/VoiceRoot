import copy
import json
from pathlib import Path
import unittest
from unittest.mock import patch

import canonical_application_package as package
import full_cohort_package_phase as admission
import media_backup_phase_test as fixture
import media_operation_journal as journal
import canonical_application_package_test as package_fixture
import full_cohort_oci_test as oci_fixture
import update_images


class PackagePhaseTest(unittest.TestCase):
    def setUp(self):
        self.base = fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.base.setUp(); self.addCleanup(self.base.doCleanups)
        self.images = {name:package.REGISTRY+'/'+name+'@sha256:'+'b'*64 for name in package.IMAGE_NAMES}
        mapped = {'voice-'+name+'/'+container:self.images[name] for name,container in update_images.SERVICES.items()}
        self.base.context['images'] = mapped
        self.base.fence.context['images'] = copy.deepcopy(mapped)
        self.base.ledger.row['images'] = copy.deepcopy(mapped)
        self.base.card['candidate']['images'] = copy.deepcopy(mapped)
        self.base.lock['data']['resource_context'] = json.dumps(self.base.context,sort_keys=True,separators=(',',':'))
        intent = {'resource_context':copy.deepcopy(self.base.context),'namespace_uid':'ns',
                  'full_cohort_package':{'run_id':123,'run_attempt':1,
                    'nodes':[{'name':'node-1','uid':'node-uid','hostname':'node-1','architecture':'amd64'}]}}
        root = Path(self.base.temp.name)/self.base.context['operation_id']; root.mkdir(mode=0o700)
        self.root = root
        self.journal = journal.Journal(root/'package-journal',intent); self.addCleanup(self.journal.close)
        self.client = fixture.Client(self.base.lock); self.client.journal = self.journal
        call = self.client.call
        def node_call(*args):
            if args[1] == 'node':
                return {'metadata':{'name':'node-1','uid':'node-uid','labels':{'kubernetes.io/os':'linux',
                    'kubernetes.io/arch':'amd64','kubernetes.io/hostname':'node-1'}},
                    'spec':{},'status':{'nodeInfo':{'architecture':'amd64','operatingSystem':'linux'},
                    'conditions':[{'type':'Ready','status':'True'}]}}
            return call(*args)
        self.client.call = node_call
        self.base.ledger.client = self.client
        self.kind = patch.object(admission.frontend_runtime_executor,'KubectlTransport',fixture.Client)
        self.kind.start(); self.addCleanup(self.kind.stop)
        self.head = patch.object(package.source,'_head',return_value={})
        self.head.start(); self.addCleanup(self.head.stop)
        self.phase = admission.PackagePhase(self.journal,self.base.fence,self.client,self.base.lock,'ns')
        self.receipt = {'schema':'voice-canonical-application-package-v1','source_sha':self.base.context['commit'],
            'verifier_sha':self.base.context['commit'],'run_id':123,'run_attempt':1,
            'request':{'schema':'voice-full-application-build-request-v1','cycle':1,'request_id':'c'*32},
            'request_blob_oid':'d'*40,'lock_artifact_id':1,'lock_sha256':'e'*64,'images':self.images,
            'runtime_qualified':False,'deployment_performed':False}
        self.platforms = {k:'sha256:'+'f'*64 for k in mapped}

    def capture(self, token, sha, run, attempt, output):
        output.write_text(json.dumps(self.receipt),encoding='utf-8'); output.chmod(0o600)
        return copy.deepcopy(self.receipt)

    def test_real_journal_and_ledger_one_shot(self):
        with patch.object(admission,'ROOT',self.root.parent), patch.object(package,'capture',side_effect=self.capture), \
             patch.object(admission.oci,'resolve',return_value=self.platforms), patch.object(admission.source,'Registry'):
            result = self.phase.run('fixture-token','fixture-actor')
            self.assertEqual(result['platform_digests'],self.platforms)
            self.assertEqual(self.base.ledger.row['full_cohort_package']['status'],'done')
            self.assertEqual(self.phase.verify_current('fixture-token'),result)
            (self.root/'canonical-application-package.json').write_text('{}',encoding='utf-8')
            with self.assertRaises(admission.PackageError): self.phase.verify_current('fixture-token')
            with self.assertRaises(admission.PackageError): self.phase.run('fixture-token','fixture-actor')

    def test_lost_capture_or_wrong_cycle_never_succeeds(self):
        self.receipt['request']['cycle'] = 2
        with patch.object(admission,'ROOT',self.root.parent), patch.object(package,'capture',side_effect=self.capture), \
             patch.object(admission.oci,'resolve') as resolver, patch.object(admission.source,'Registry'):
            with self.assertRaises(admission.PackageError): self.phase.run('fixture-token','fixture-actor')
            resolver.assert_not_called()
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_image_drift_is_uncertain(self):
        self.receipt['images']['auth'] = package.REGISTRY+'/auth@sha256:'+'0'*64
        with patch.object(admission,'ROOT',self.root.parent), patch.object(package,'capture',side_effect=self.capture), \
             patch.object(admission.oci,'resolve') as resolver, patch.object(admission.source,'Registry'):
            with self.assertRaises(admission.PackageError): self.phase.run('fixture-token','fixture-actor')
            resolver.assert_not_called()
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_lost_reservation_response_is_uncertain_and_no_capture(self):
        write = self.base.ledger.write
        def lost(revision,row):
            write(revision,row)
            if 'full_cohort_package' in row: raise OSError('lost response private sentinel')
        with patch.object(admission,'ROOT',self.root.parent), patch.object(package,'capture') as capture, \
             patch.object(self.base.ledger,'write',side_effect=lost):
            with self.assertRaises(admission.PackageError) as error: self.phase.run('fixture-token','fixture-actor')
            self.assertNotIn('sentinel',str(error.exception)); capture.assert_not_called()
        self.assertTrue(self.phase.poisoned)

    def test_node_drift_fails_before_capture(self):
        call = self.client.call
        def drift(*args):
            value = call(*args)
            if args[1] == 'node': value['metadata']['uid'] = 'foreign'
            return value
        self.client.call = drift
        with patch.object(admission,'ROOT',self.root.parent), patch.object(package,'capture') as capture:
            with self.assertRaises(admission.PackageError): self.phase.run('fixture-token','fixture-actor')
            capture.assert_not_called()

    def test_moved_master_is_not_silently_equated(self):
        self.receipt['source_equivalence'] = {'current_master_sha_after':'f'*40}
        with patch.object(admission,'ROOT',self.root.parent), patch.object(package,'capture',side_effect=self.capture), \
             patch.object(admission.oci,'resolve') as resolver, patch.object(admission.source,'Registry'):
            with self.assertRaises(admission.PackageError): self.phase.run('fixture-token','fixture-actor')
            resolver.assert_not_called()
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_authority_lost_after_terminal_guard_never_returns_receipt(self):
        guard = self.phase._guard
        def drift():
            guard()
            row = self.base.ledger.row
            if row.get('full_cohort_package',{}).get('status') == 'done':
                row['status'] = 'foreign'
        with patch.object(admission,'ROOT',self.root.parent), patch.object(package,'capture',side_effect=self.capture), \
             patch.object(admission.oci,'resolve',return_value=self.platforms), patch.object(admission.source,'Registry'), \
             patch.object(self.phase,'_guard',side_effect=drift):
            with self.assertRaises(admission.PackageError): self.phase.run('fixture-token','fixture-actor')
        self.assertTrue(self.phase.poisoned)

    def test_verify_detects_in_place_receipt_mutation(self):
        with patch.object(admission,'ROOT',self.root.parent), patch.object(package,'capture',side_effect=self.capture), \
             patch.object(admission.oci,'resolve',return_value=self.platforms), patch.object(admission.source,'Registry'):
            self.phase.run('fixture-token','fixture-actor')
            original = self.phase._nodes; calls = 0
            def drift():
                nonlocal calls
                original(); calls += 1
                if calls == 2:
                    self.journal.state['steps'][admission.STEP]['receipt']['platform_digests']['voice-auth/auth'] = 'sha256:'+'0'*64
            with patch.object(self.phase,'_nodes',side_effect=drift):
                with self.assertRaises(admission.PackageError): self.phase.verify_current('fixture-token')

    def test_master_moved_after_package_does_not_invalidate_pinned_receipt(self):
        with patch.object(admission,'ROOT',self.root.parent), patch.object(package,'capture',side_effect=self.capture), \
             patch.object(admission.oci,'resolve',return_value=self.platforms), patch.object(admission.source,'Registry'):
            result=self.phase.run('fixture-token','fixture-actor')
            with patch.object(package.source,'_head',side_effect=ValueError('private sentinel')):
                self.assertEqual(self.phase.verify_current('fixture-token'),result)

    def test_actual_capture_and_oci_composition_with_fake_remote(self):
        registry_fixture = oci_fixture.OCITest(); registry_fixture.setUp()
        images = registry_fixture.images
        mapped = {'voice-'+name+'/'+container:images[name] for name,container in update_images.SERVICES.items()}
        self.base.context['images'] = mapped; self.base.fence.context['images'] = copy.deepcopy(mapped)
        self.base.ledger.row['images'] = copy.deepcopy(mapped); self.base.card['candidate']['images'] = copy.deepcopy(mapped)
        self.base.lock['data']['resource_context'] = json.dumps(self.base.context,sort_keys=True,separators=(',',':'))
        self.journal.intent['resource_context']['images'] = copy.deepcopy(mapped)
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.phase = admission.PackagePhase(self.journal,self.base.fence,self.client,self.base.lock,'ns')
        fixture_run = package_fixture.CanonicalPackageTests(); fixture_run.setUp()
        run = dict(fixture_run.run, id=123); jobs = [dict(item,run_id=123) for item in fixture_run.jobs]
        request = json.dumps({'schema':'voice-full-application-build-request-v1','cycle':1,'request_id':'c'*32}).encode()
        def api(url,headers,deadline):
            if '/actions/runs/' in url: return run
            if '/git/commits/' in url: return {'sha':fixture_run.sha,'parents':[{'sha':'b'*40}]}
            raise AssertionError('unexpected API')
        def source_capture(token,run_id,sha,folder):
            marker = folder / package.REQUEST; marker.parent.mkdir(parents=True); marker.write_bytes(request)
            return {'run_attempt':1,'source_sha':sha,'images':images,'lock_artifact_id':7,'lock_sha256':'2'*64}
        with patch.object(admission,'ROOT',self.root.parent), \
             patch.object(package.common,'command',return_value=(fixture_run.sha).encode()), \
             patch.object(package.common,'source_snapshot',return_value={'source':fixture_run.sha}), \
             patch.object(package.source,'_get',side_effect=api), patch.object(package.source,'_pages',return_value=jobs), \
             patch.object(package,'tree_blob',return_value=None), \
             patch.object(package.source,'capture_equivalent_master_source',side_effect=source_capture), \
             patch.object(package.source,'_head',return_value={}), patch.object(package.source,'_ci',return_value=(run,{})), \
             patch.object(package.source,'_resolve_image',side_effect=lambda name,sha,deadline:images[name]), \
             patch.object(admission.source,'Registry',return_value=oci_fixture.Registry(registry_fixture.objects)):
            receipt = self.phase.run('fixture-token','fixture-actor')
        self.assertEqual(receipt['platform_digests'], {key:registry_fixture.child for key in mapped})
        self.assertEqual(self.base.ledger.row['full_cohort_package']['status'],'done')


if __name__ == '__main__': unittest.main()
