"""Flutter JSON reports must show complete, non-skipped test execution."""
import json
import unittest

import flutter_test_events as report


def stream():
    return [
        {'type': 'start'},
        {'type': 'suite', 'suite': {'id': 1, 'path': '/repo/test/music_test.dart'}},
        {'type': 'allSuites', 'count': 1},
        {'type': 'testStart', 'test': {'id': 2, 'suiteID': 1, 'name': 'loading', 'metadata': {'skip': False}}},
        {'type': 'testDone', 'testID': 2, 'result': 'success', 'skipped': False, 'hidden': True},
        {'type': 'testStart', 'test': {'id': 3, 'suiteID': 1, 'name': 'plays music', 'metadata': {'skip': False}}},
        {'type': 'testDone', 'testID': 3, 'result': 'success', 'skipped': False, 'hidden': False},
        {'type': 'done', 'success': True},
    ]


class FlutterEventTests(unittest.TestCase):
    def test_complete_report_records_named_cases_and_hidden_load(self):
        result = report.summarize(stream(), ['/repo/test/music_test.dart'], reject_skips=True)
        self.assertEqual(result['tests'], 1)
        self.assertEqual(result['hidden'], 1)
        self.assertEqual(result['skipped'], [])
        self.assertEqual(result['passed'], 1)

    def test_missing_duplicate_failed_or_unexpected_events_are_rejected(self):
        mutations = []
        rows = stream(); rows.pop(6); mutations.append(rows)
        rows = stream(); rows.insert(7, dict(rows[6])); mutations.append(rows)
        rows = stream(); rows[6]['result'] = 'failure'; mutations.append(rows)
        rows = stream(); rows[6]['skipped'] = True; mutations.append(rows)
        rows = stream(); rows[-1]['success'] = False; mutations.append(rows)
        rows = stream(); rows[3]['test']['suiteID'] = 99; mutations.append(rows)
        rows = stream(); rows.insert(-1, {'type': 'future-unrecognized'}); mutations.append(rows)
        rows = stream(); rows[-1]['type'] = 'print'; mutations.append(rows)
        for rows in mutations:
            with self.subTest(rows=rows), self.assertRaises(ValueError):
                report.summarize(rows, ['/repo/test/music_test.dart'], reject_skips=True)
        with self.assertRaises(ValueError):
            report.summarize(stream(), ['/repo/test/other_test.dart'], reject_skips=True)
        with self.assertRaises(ValueError):
            report.summarize([None], ['/repo/test/music_test.dart'])

    def test_skips_are_recorded_when_explicitly_allowed(self):
        rows = stream()
        rows[6]['skipped'] = True
        result = report.summarize(rows, ['/repo/test/music_test.dart'], reject_skips=False)
        self.assertEqual(result['passed'], 0)
        self.assertEqual(result['skipped'][0]['name'], 'plays music')

    def test_every_expected_suite_must_have_a_visible_test(self):
        rows = stream()
        rows.insert(2, {'type': 'suite', 'suite': {'id': 4, 'path': '/repo/test/empty_test.dart'}})
        rows[3]['count'] = 2
        rows.insert(-1, {'type': 'testStart', 'test': {'id': 5, 'suiteID': 4, 'name': 'loading', 'metadata': {'skip': False}}})
        rows.insert(-1, {'type': 'testDone', 'testID': 5, 'result': 'success', 'skipped': False, 'hidden': True})
        with self.assertRaises(ValueError):
            report.summarize(rows, ['/repo/test/music_test.dart', '/repo/test/empty_test.dart'])

    def test_json_lines_must_be_complete_and_unicode_decodable(self):
        raw = ('\n'.join(json.dumps(row) for row in stream()) + '\n').encode()
        self.assertEqual(len(report.read_events(raw)), len(stream()))
        self.assertEqual(len(report.read_events(raw.decode().encode('utf-16'))), len(stream()))
        with self.assertRaises(ValueError):
            report.read_events(raw[:-1])
        with self.assertRaises(ValueError):
            report.read_events(b'not-json\n')
        with self.assertRaises(ValueError):
            report.read_events(b'{"type":"start","type":"done"}\n')


if __name__ == '__main__':
    unittest.main()
