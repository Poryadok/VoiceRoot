"""Successor draft cannot become authority through incomplete/local metadata."""
import copy
import json
from pathlib import Path
import unittest
from unittest.mock import patch

import f13_native_p3_source as legacy
import f13_native_p3_successor as successor

ROOT = Path(__file__).resolve().parents[2]


class SuccessorRecordTest(unittest.TestCase):
    def setUp(self):
        self.previous = json.loads((ROOT / legacy.RECORD_PATH).read_text())
        self.record = json.loads((ROOT / successor.RECORD_PATH).read_text())
        self.closure = json.loads((ROOT / 'docs/testing/native-p3-source-v2-closure.json').read_text())

    def validate(self):
        return successor.validate_structure(self.record, self.previous, self.closure)

    def test_complete_draft_is_inspectable_but_never_permission(self):
        self.record['review_status'] = 'OPEN'
        self.validate()
        with self.assertRaisesRegex(legacy.Rejected, 'successor_review_open'):
            successor.require_accepted(self.record)
        with patch.object(successor, 'PINNED_RECORD_COMMIT', None), self.assertRaisesRegex(legacy.Rejected, 'successor_carrier_not_pinned'):
            successor.load_record(ROOT)

    def test_local_pass_strings_cannot_create_carrier(self):
        self.record['review_status'] = 'ACCEPT'
        for row in self.record['delta_review']:
            row['status'] = 'ACCEPT'
        for proof in self.record['evidence_matrix'].values():
            proof.update(result='PASS', review=[])
        self.validate()
        with patch.object(successor, 'PINNED_RECORD_COMMIT', None), self.assertRaisesRegex(legacy.Rejected, 'successor_carrier_not_pinned'):
            successor.load_record(ROOT)

    def test_unknown_lineage_is_rejected(self):
        self.record['predecessor']['record'] = 'f' * 40
        with self.assertRaisesRegex(legacy.Rejected, 'successor_lineage'):
            self.validate()

    def test_dropped_old_context_is_rejected(self):
        name = self.previous['context'][0]['path']
        self.record['context'] = [r for r in self.record['context'] if r['path'] != name]
        with self.assertRaisesRegex(legacy.Rejected, 'successor_context_dropped'):
            self.validate()

    def test_dropped_old_projection_is_rejected(self):
        name = self.previous['projection'][0]['path']
        self.record['projection'] = [r for r in self.record['projection'] if r['path'] != name]
        with self.assertRaisesRegex(legacy.Rejected, 'successor_projection_dropped'):
            self.validate()

    def test_unreviewed_changed_input_is_rejected(self):
        self.record['delta_review'].pop()
        with self.assertRaisesRegex(legacy.Rejected, 'successor_delta_review_incomplete'):
            self.validate()

    def test_context_cannot_be_silently_promoted(self):
        row = self.record['context'].pop(0)
        self.record['projection'].append(dict(path=row['path'], before=None, after=row['entry']))
        self.record['projection'].sort(key=lambda r: r['path'])
        with self.assertRaisesRegex(legacy.Rejected, 'successor_promoted_context'):
            self.validate()

    def test_compiled_dependency_outside_record_is_rejected(self):
        self.record['compiled_closure'].append('unreviewed/new.go')
        self.record['compiled_closure'].sort()
        with self.assertRaisesRegex(legacy.Rejected, 'successor_compiled_closure'):
            self.validate()

    def test_compiled_dependency_omission_is_rejected(self):
        self.record['compiled_closure'].pop()
        with self.assertRaisesRegex(legacy.Rejected, 'successor_compiled_closure_omitted'):
            self.validate()

    def test_unbound_evidence_reference_is_rejected(self):
        self.record['evidence_matrix']['A3']['current'] = ['invented-restart-proof']
        with self.assertRaisesRegex(legacy.Rejected, 'successor_unknown_evidence_reference'):
            self.validate()

    def test_truthy_non_reference_evidence_is_rejected(self):
        self.record['evidence_matrix']['A3']['current'] = True
        with self.assertRaisesRegex(legacy.Rejected, 'successor_case_evidence_A3'):
            self.validate()

    def test_non_generated_behavior_cannot_hide_in_context(self):
        name = 'src/backend/voice/internal/grpcsvc/ordinary_control.go'
        row = next(r for r in self.record['projection'] if r['path'] == name)
        self.record['projection'].remove(row)
        self.record['context'].append(dict(path=name, entry=row['after']))
        self.record['context'].sort(key=lambda r: r['path'])
        self.record['promoted_context'] = []
        next(r for r in self.record['delta_review'] if r['path'] == name)['kind'] = 'context'
        with self.assertRaisesRegex(legacy.Rejected, 'successor_voice_change_hidden_in_context'):
            self.validate()

    def test_missing_literal_clause_is_rejected(self):
        del self.record['evidence_matrix']['B3']
        with self.assertRaisesRegex(legacy.Rejected, 'successor_matrix'):
            self.validate()

    def test_broadened_authority_is_rejected(self):
        self.record['excluded_authority'] = []
        with self.assertRaisesRegex(legacy.Rejected, 'successor_excluded_authority'):
            self.validate()

    def test_mixed_versions_are_rejected(self):
        self.record['version'] = self.previous['version']
        with self.assertRaisesRegex(legacy.Rejected, 'successor_version'):
            self.validate()


if __name__ == '__main__':
    unittest.main()
