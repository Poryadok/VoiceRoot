"""Temporary Git bridge fixtures; synthetic events are never hosted evidence."""
import subprocess
import tempfile
import unittest
from pathlib import Path

import f13_current_source as current


class BaseBridgeRealGitTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='f13-base-bridge-')
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name)
        self.git('init', '-q', '-b', 'master')
        self.git('config', 'user.name', 'Fixture')
        self.git('config', 'user.email', 'fixture@example.invalid')
        self.write('unit/source', 'old')
        self.write('carrier/verifier', 'old')
        self.write('carrier/test', 'old test')
        self.write('archive/historical', 'archive')
        self.write('evidence/old', 'old review')
        self.write('src/backend/voice/source.go', 'voice')
        self.write('outside/config', 'old config')
        self.h = self.commit('reviewed source')
        self.git('checkout', '-qb', 'advanced-base')
        self.write('outside/runtime', 'one')
        self.b1 = self.commit('unrelated advance')
        self.write('outside/runtime', 'two')
        self.b = self.commit('second unrelated advance')
        self.git('checkout', '-qb', 'carrier', self.h)
        self.write('proof/record', 'reviewed record')
        self.r = self.commit('record')
        self.write('carrier/verifier', 'executed carrier')
        self.k = self.commit('carrier')
        self.git('merge', '--no-ff', '-qm', 'incorporate actual base', self.b)
        self.m = self.git('rev-parse', 'HEAD')
        self.s = self.m
        self.git('checkout', '-qb', 'event', self.b)
        self.git('merge', '--no-ff', '-qm', 'synthetic event', self.s)
        self.t = self.git('rev-parse', 'HEAD')

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args],
                                       stderr=subprocess.PIPE).decode().strip()

    def write(self, path, value):
        target = self.repo / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(value, encoding='utf-8')

    def commit(self, message):
        self.git('add', '.')
        self.git('commit', '-qm', message)
        return self.git('rev-parse', 'HEAD')

    def event(self, base=None, source=None, target=None, before=None):
        base, source, target = base or self.b, source or self.s, target or self.t
        return current.bridge_event(self.repo, base, source, target, 'push', 'refs/heads/master',
                                    target, base if before is None else before, '', self.h, self.r, self.h)

    def history(self, value):
        tree = current.release.documentation.tree_entries(self.repo, self.h)
        directories = current.release.documentation.immediate_directory_entries(tree)
        unit = {'entries': [{'path': 'unit/source'}], 'directory_inventories': [
            {'directory': 'unit', 'entries': directories['unit']}]}
        return current.bridge_history(self.repo, value, {'role': unit}, {'evidence/old'},
                                      {'carrier/verifier', 'carrier/test'}, 'archive/historical')

    def fixture_event(self, base, source, branch):
        self.git('checkout', '-qb', branch, base)
        self.git('merge', '--no-ff', '-qm', 'synthetic event', source)
        return self.event(base, source, self.git('rev-parse', 'HEAD'))

    def test_complete_history_and_overlay(self):
        self.history(self.event())

    def test_base_unit_artifact_and_carrier_restorations_rejected(self):
        for index, path in enumerate(('unit/source', 'evidence/old', 'carrier/verifier')):
            with self.subTest(path=path):
                self.git('checkout', '-qb', 'bad-base-' + str(index), self.h)
                old = (self.repo / path).read_text(encoding='utf-8')
                self.write(path, 'intermediate bad')
                self.commit('change protected input')
                self.write(path, old)
                bad_base = self.commit('restore protected input')
                self.git('checkout', '-qb', 'bad-carrier-' + str(index), self.k)
                self.git('merge', '--no-ff', '-qm', 'incorporate base', bad_base)
                bad_source = self.git('rev-parse', 'HEAD')
                value = self.fixture_event(bad_base, bad_source, 'bad-event-' + str(index))
                with self.assertRaisesRegex(ValueError, 'bridge_base_'):
                    self.history(value)

    def test_source_unit_and_proof_restorations_rejected(self):
        for index, path in enumerate(('unit/source', 'proof/record')):
            with self.subTest(path=path):
                self.git('checkout', '-qb', 'bad-source-' + str(index), self.m)
                old = (self.repo / path).read_text(encoding='utf-8')
                self.write(path, 'intermediate bad')
                self.commit('change protected input')
                self.write(path, old)
                bad_source = self.commit('restore protected input')
                value = self.fixture_event(self.b, bad_source, 'bad-source-event-' + str(index))
                with self.assertRaisesRegex(ValueError, 'bridge_after_carrier_'):
                    self.history(value)

    def test_source_directory_and_voice_restorations_rejected(self):
        self.git('checkout', '-qb', 'bad-directory', self.m)
        self.write('unit/extra', 'new')
        self.commit('add unlisted unit input')
        self.git('rm', '-q', 'unit/extra')
        source = self.commit('remove unlisted unit input')
        value = self.fixture_event(self.b, source, 'directory-event')
        with self.assertRaisesRegex(ValueError, 'bridge_after_carrier_directory'):
            self.history(value)
        self.git('checkout', '-qb', 'bad-voice', self.m)
        self.write('src/backend/voice/source.go', 'intermediate bad')
        self.commit('change voice')
        self.write('src/backend/voice/source.go', 'voice')
        source = self.commit('restore voice')
        value = self.fixture_event(self.b, source, 'voice-event')
        with self.assertRaisesRegex(ValueError, 'bridge_after_carrier_voice'):
            self.history(value)

    def test_initial_overlay_rejects_extra_net_change(self):
        self.git('checkout', '-qb', 'extra-source', self.m)
        self.write('outside/extra', 'extra change')
        source = self.commit('extra source change')
        value = self.fixture_event(self.b, source, 'extra-event')
        with self.assertRaisesRegex(ValueError, 'bridge_initial_overlay'):
            self.history(value)

    def test_unqualified_base_cannot_precreate_record_proof(self):
        self.git('checkout', '-qb', 'base-proof-collision', self.h)
        self.write('proof/record', 'reviewed record')
        bad_base = self.commit('unqualified duplicate proof')
        self.git('checkout', '-qb', 'source-proof-collision', self.k)
        self.git('merge', '--no-ff', '-qm', 'incorporate colliding base', bad_base)
        bad_source = self.git('rev-parse', 'HEAD')
        value = self.fixture_event(bad_base, bad_source, 'event-proof-collision')
        with self.assertRaisesRegex(ValueError, 'bridge_base_proof_collision'):
            self.history(value)

    def test_r_descendant_sibling_without_carrier_is_rejected(self):
        self.git('checkout', '-qb', 'r-sibling', self.r)
        self.git('commit', '--allow-empty', '-qm', 'unqualified sibling')
        sibling = self.git('rev-parse', 'HEAD')
        self.git('checkout', '-qb', 'source-with-sibling', self.m)
        self.git('merge', '--no-ff', '-qm', 'merge sibling', sibling)
        source = self.git('rev-parse', 'HEAD')
        value = self.fixture_event(self.b, source, 'event-with-sibling')
        with self.assertRaisesRegex(ValueError, 'bridge_unqualified_side_history'):
            self.history(value)

    def test_actual_advanced_base_and_later_continuation(self):
        value = self.event()
        self.assertEqual(value, dict(type='PRESERVED-UNIT-BASE-BRIDGE-v1', reviewed_source=self.h,
            record=self.r, initial_base=self.b, carrier=self.k, incorporation=self.m,
            initial_source=self.s, first_merge=self.t, base=self.b, source=self.s, target=self.t))
        self.history(value)
        self.write('outside/later', 'later base')
        later_base = self.commit('later unrelated base')
        self.git('checkout', '-qb', 'later-source')
        self.write('outside/feature', 'later source')
        later_source = self.commit('later source')
        self.git('checkout', '-qb', 'later-event', later_base)
        self.git('merge', '--no-ff', '-qm', 'later synthetic event', later_source)
        later_target = self.git('rev-parse', 'HEAD')
        later = self.event(later_base, later_source, later_target)
        self.assertEqual((later['initial_base'], later['carrier'], later['initial_source'], later['first_merge']),
                         (self.b, self.k, self.s, self.t))
        self.assertEqual((later['base'], later['source'], later['target']),
                         (later_base, later_source, later_target))
        self.history(later)

    def test_carrier_may_merge_prior_carrier_only_branch_before_base(self):
        self.git('checkout', '-qb', 'prior-fixture', self.h)
        self.write('outside/config', 'temporary outside unit')
        self.commit('outside-unit change owned by producer')
        self.write('outside/config', 'old config')
        self.commit('restore outside-unit change')
        self.write('carrier/test', 'prior CI fixture change')
        prior = self.commit('prior carrier-only change')
        self.git('checkout', '-qb', 'carrier-with-prior', self.k)
        self.git('merge', '--no-ff', '-qm', 'merge prior carrier', prior)
        merged_carrier = self.git('rev-parse', 'HEAD')
        self.git('merge', '--no-ff', '-qm', 'incorporate base', self.b)
        source = self.git('rev-parse', 'HEAD')
        value = self.fixture_event(self.b, source, 'event-with-prior')
        self.assertEqual(value['carrier'], merged_carrier)
        self.history(value)

    def test_precarrier_unit_and_proof_restorations_rejected(self):
        for index, path in enumerate(('unit/source', 'proof/record')):
            with self.subTest(path=path):
                self.git('checkout', '-qb', 'precarrier-drift-' + str(index), self.r)
                old = (self.repo / path).read_text(encoding='utf-8')
                self.write(path, 'intermediate bad')
                self.commit('change protected input before carrier')
                self.write(path, old)
                self.commit('restore protected input before carrier')
                self.write('carrier/verifier', 'executed carrier')
                self.commit('final carrier')
                self.git('merge', '--no-ff', '-qm', 'incorporate base', self.b)
                source = self.git('rev-parse', 'HEAD')
                value = self.fixture_event(self.b, source, 'precarrier-event-' + str(index))
                with self.assertRaisesRegex(ValueError, 'bridge_precarrier_'):
                    self.history(value)

    def test_later_base_unit_restoration_rejected(self):
        self.git('checkout', '-qb', 'later-base-drift', self.t)
        self.write('unit/source', 'intermediate bad')
        self.commit('change later base unit')
        self.write('unit/source', 'old')
        later_base = self.commit('restore later base unit')
        self.git('checkout', '-qb', 'later-source-drift')
        self.write('outside/later-feature', 'feature')
        later_source = self.commit('later feature')
        value = self.fixture_event(later_base, later_source, 'later-event-drift')
        with self.assertRaisesRegex(ValueError, 'bridge_after_carrier_entry'):
            self.history(value)

    def later_pre_record_side_event(self, index, protected_path=None):
        self.git('checkout', '-qb', 'pre-record-side-' + str(index), self.h)
        if protected_path:
            old = (self.repo / protected_path).read_text(encoding='utf-8')
            self.write(protected_path, 'intermediate side-branch change')
            self.commit('change protected input')
            self.write(protected_path, old)
            self.commit('restore protected input')
        self.write('outside/side-' + str(index), 'independent unrelated advance')
        side = self.commit('unrelated side branch')
        self.git('checkout', '-qb', 'later-merge-' + str(index), self.t)
        self.git('merge', '--no-ff', '-qm', 'merge later side branch', side)
        later_base = self.git('rev-parse', 'HEAD')
        self.git('checkout', '-qb', 'later-feature-' + str(index))
        self.write('outside/feature-' + str(index), 'feature')
        later_source = self.commit('later feature')
        return self.fixture_event(later_base, later_source, 'later-event-side-' + str(index))

    def test_later_base_merges_unrelated_pre_record_side_branch(self):
        self.history(self.later_pre_record_side_event('clean'))

    def test_later_pre_record_base_unit_and_artifact_restorations_rejected(self):
        for index, path in enumerate(('unit/source', 'evidence/old')):
            with self.subTest(path=path):
                value = self.later_pre_record_side_event(index, path)
                with self.assertRaisesRegex(ValueError, 'bridge_base_entry'):
                    self.history(value)

    def test_wrong_before_and_wrong_head_are_rejected(self):
        with self.assertRaisesRegex(ValueError, 'bridge_master_event'):
            self.event(before='0' * 40)
        with self.assertRaisesRegex(ValueError, 'bridge_actual_head'):
            self.event(target=self.s)

    def test_wrong_parent_order_and_result_tree_are_rejected(self):
        tree = self.git('rev-parse', self.s + '^{tree}')
        wrong_order = self.git('commit-tree', tree, '-p', self.s, '-p', self.b, '-m', 'wrong order')
        self.git('checkout', '--detach', '-q', wrong_order)
        with self.assertRaisesRegex(ValueError, 'bridge_ordered_parents'):
            self.event(self.b, self.s, wrong_order)
        bad_tree = self.git('rev-parse', self.b + '^{tree}')
        wrong_result = self.git('commit-tree', bad_tree, '-p', self.b, '-p', self.s, '-m', 'wrong tree')
        self.git('checkout', '--detach', '-q', wrong_result)
        with self.assertRaisesRegex(ValueError, 'bridge_result_tree'):
            self.event(self.b, self.s, wrong_result)

    def test_record_must_be_on_source_first_parent_line(self):
        self.git('checkout', '-qb', 'wrong-source', self.b)
        self.git('merge', '--no-ff', '-qm', 'wrong parent order', self.k)
        wrong_source = self.git('rev-parse', 'HEAD')
        self.git('checkout', '-qb', 'wrong-event', self.b)
        self.git('merge', '--no-ff', '-qm', 'synthetic event', wrong_source)
        wrong_target = self.git('rev-parse', 'HEAD')
        with self.assertRaisesRegex(ValueError, 'bridge_record_first_parent'):
            self.event(self.b, wrong_source, wrong_target)


if __name__ == '__main__':
    unittest.main()
