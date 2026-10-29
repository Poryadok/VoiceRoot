"""Bounded parsing of original native installer output; not an authority check."""
import json
import unittest

import space_fresh_install_phase_events as events


class CreatedEventTest(unittest.TestCase):
    def setUp(self):
        self.expected = {'source_revision': 'a' * 40, 'source_sha256': 'b' * 64,
                         'executable_sha256': 'c' * 64, 'migrator_sha256': 'd' * 64,
                         'inventory_sha256': 'e' * 64, 'database_system_id': '123',
                         'installer_role': 'voice'}
        self.receipt = {k: v for k, v in self.expected.items() if k not in ('database_system_id', 'installer_role')}
        self.receipt.update(run_id='c7c38bbc-a805-4fea-8a71-b7024e40d871', created=True,
                            creation={'version': 0, 'database_system_id': '123', 'database_name': 'space_db',
                                      'database_oid': 42, 'installer_role': 'voice', 'epoch': '', 'seal_sha256': ''},
                            trusted44=None, transition45=None, transition50=None, trusted50=None,
                            transition51=None, final=None)

    def raw(self):
        return (json.dumps({'event': 'created', 'receipt': self.receipt}) + '\n').encode()

    def test_created(self):
        self.assertEqual(events.created(self.raw(), self.expected, old_oid=11),
                         {'run_id': self.receipt['run_id'], 'new_oid': 42, 'role': 'voice'})

    def test_rejects_wrong_source_target_and_ambiguous_output(self):
        for mutate in ('source', 'oid', 'role', 'duplicate', 'oversize', 'partial'):
            with self.subTest(mutate=mutate):
                raw = self.raw()
                if mutate == 'source': raw = raw.replace(b'a' * 40, b'f' * 40)
                if mutate == 'oid': raw = raw.replace(b'"database_oid": 42', b'"database_oid": 11')
                if mutate == 'role': raw = raw.replace(b'"installer_role": "voice"', b'"installer_role": "other"')
                if mutate == 'duplicate': raw += raw
                if mutate == 'oversize': raw += b'x' * (1024 * 1024)
                if mutate == 'partial': raw = raw[:-1]
                with self.assertRaises(events.EventError): events.created(raw, self.expected, old_oid=11)

    def test_rejects_noninteger_version(self):
        for value in (False, 0.0):
            with self.subTest(value=value):
                self.receipt['creation']['version'] = value
                with self.assertRaises(events.EventError): events.created(self.raw(), self.expected, old_oid=11)


if __name__ == '__main__': unittest.main()
