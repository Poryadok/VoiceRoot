"""Count admission tests retain their slot files for inspection."""
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import local_heavy_slots as slots
import local_go_full_integration as integration
import local_preflight


class CountAdmissionTests(unittest.TestCase):
    def test_parent_rejects_worker_mode_or_saved_report_mismatch(self):
        report = {'status':'incomplete', 'source':{'head':'a'*40}, 'admission_mode':'count'}
        self.assertTrue(integration._worker_admission_matches(report, dict(report), report['source'], 'count'))
        self.assertFalse(integration._worker_admission_matches(report, dict(report), report['source'], 'resource-claim'))
        self.assertFalse(integration._worker_admission_matches(report, {**report, 'admission_mode':'resource-claim'}, report['source'], 'count'))

    def test_admission_evidence_rejects_mismatch_and_missing_reservation_state(self):
        for mode, released in (('resource-claim', True), ('count', None)):
            report = {'admission_mode':mode, 'cleanup_evidence':{'capacity_reservation_released':released}}
            local_preflight._validate_go_admission_evidence(report, mode)
            with self.assertRaises(ValueError):
                local_preflight._validate_go_admission_evidence(report, 'count' if mode == 'resource-claim' else 'resource-claim')
            with self.assertRaises(ValueError):
                local_preflight._validate_go_admission_evidence({'admission_mode':mode, 'cleanup_evidence':{}}, mode)
            with self.assertRaises(ValueError):
                local_preflight._validate_go_admission_evidence(
                    {'admission_mode':mode, 'cleanup_evidence':{'capacity_reservation_released':not released}}, mode)

    def test_go_admission_mode_is_explicit_and_preserves_default(self):
        claim = {'nano_cpus':1,'memory_bytes':1,'pids_limit':1}
        budget = {'nano_cpus':2,'memory_bytes':2,'pids_limit':2}
        self.assertEqual(integration._admission_options('resource-claim', claim, budget),
                         {'resource_claim':claim, 'capacity_budget':budget})
        self.assertEqual(integration._admission_options('count', claim, budget), {})
        with self.assertRaises(ValueError):
            integration._admission_options('other', claim, budget)

    def test_go_supervisor_forwards_selected_mode_to_shared_scheduler(self):
        root = Path.cwd().resolve()
        output = root.parent / 'never-created-count-test'
        claim = {'nano_cpus':1, 'memory_bytes':1, 'pids_limit':1}
        budget = {'nano_cpus':2, 'memory_bytes':2, 'pids_limit':2}
        limits = {'host_job':{'cpu_rate':1, 'job_memory_bytes':1, 'active_process_limit':2},
                  'capacity_budget':budget}
        plan = {'source':{'head':'a'*40}}
        with mock.patch.object(integration, '_validate_resource_limits', return_value=limits), \
                mock.patch.object(integration.heavy_slots, 'measured_host_capacity', return_value=budget), \
                mock.patch.object(integration.heavy_slots, '_validate_capacity_budget', return_value=budget), \
                mock.patch.object(integration, '_resource_claim', return_value=claim), \
                mock.patch.object(integration, '_prepare_output_root', return_value=output), \
                mock.patch.object(Path, 'mkdir'), mock.patch.object(integration, 'DockerCLI'), \
                mock.patch.object(integration.proof_scheduler, '_session_identity', return_value={}), \
                mock.patch.object(integration.ScheduledRun, 'start_host', side_effect=RuntimeError('stop')) as start:
            for mode in ('resource-claim', 'count'):
                with self.assertRaisesRegex(RuntimeError, 'stop'):
                    integration.run_supervised(plan, 'chat', root, output, 'helper', 'a'*64, {},
                        base='b'*40, head_ref='fixture', resource_limits=limits, admission_mode=mode)
                kwargs = start.call_args.kwargs
                self.assertEqual('resource_claim' in kwargs, mode == 'resource-claim')
                self.assertEqual('capacity_budget' in kwargs, mode == 'resource-claim')
                self.assertIs(kwargs['resource_limits'], limits)

    def test_two_held_slots_block_third_and_mixed_mode(self):
        directory = Path(tempfile.mkdtemp(prefix='voice-count-admission-retained-')).resolve()
        print('Retained slot fixture:', directory, flush=True)
        owners = ['voiceci-' + char * 32 for char in 'abc']
        claim = {'nano_cpus':1, 'memory_bytes':1, 'pids_limit':1}
        budget = {'nano_cpus':2, 'memory_bytes':2, 'pids_limit':2}
        first = slots.acquire(directory, 0, owner_id=owners[0])
        second = None
        try:
            with mock.patch.object(slots, 'measured_host_capacity', return_value=budget):
                with self.assertRaises(slots.SlotAcquireTimeout):
                    slots.acquire(directory, 0, owner_id=owners[2], resource_claim=claim, capacity_budget=budget)
            second = slots.acquire(directory, 0, owner_id=owners[1])
            self.assertNotEqual(first.slot, second.slot)
            with self.assertRaises(slots.SlotAcquireTimeout):
                slots.acquire(directory, 0, owner_id=owners[2])
            with mock.patch.object(slots, 'measured_host_capacity', return_value=budget):
                with self.assertRaises(slots.SlotAcquireTimeout):
                    slots.acquire(directory, 0, owner_id=owners[2], resource_claim=claim, capacity_budget=budget)
            first.close(cleanup_verified=True)
            replacement = slots.acquire(directory, 0, owner_id=owners[2])
            replacement.close(cleanup_verified=True)
        finally:
            first.close(cleanup_verified=True)
            if second is not None:
                second.close(cleanup_verified=True)

    def test_resource_reservation_blocks_count_mode(self):
        directory = Path(tempfile.mkdtemp(prefix='voice-count-admission-retained-')).resolve()
        print('Retained slot fixture:', directory, flush=True)
        claim = {'nano_cpus':1, 'memory_bytes':1, 'pids_limit':1}
        budget = {'nano_cpus':2, 'memory_bytes':2, 'pids_limit':2}
        with mock.patch.object(slots, 'measured_host_capacity', return_value=budget):
            first = slots.acquire(directory, 0, owner_id='voiceci-'+'a'*32,
                                  resource_claim=claim, capacity_budget=budget)
            try:
                with self.assertRaises(slots.SlotAcquireTimeout):
                    slots.acquire(directory, 0, owner_id='voiceci-'+'b'*32)
            finally:
                first.close(cleanup_verified=True)


if __name__ == '__main__':
    unittest.main()
