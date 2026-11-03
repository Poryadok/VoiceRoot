"""Post-restart service identity and dynamic port mapping checks."""
import json
import unittest

import space_profile50_port as port


class PortTests(unittest.TestCase):
    def test_restart_uses_current_mapping_not_previous_port(self):
        container = 'a' * 64
        row = {'Id': container, 'State': {'Running': True}, 'NetworkSettings': {'Ports': {'5432/tcp': [
            {'HostIp': '0.0.0.0', 'HostPort': '32769'}, {'HostIp': '::', 'HostPort': '32769'}]}}}
        self.assertEqual(port.resolve(container, json.dumps([row])), '32769')
        self.assertNotEqual(port.resolve(container, json.dumps([row])), '32768')
        for mutation in ('foreign', 'stopped', 'ambiguous', 'missing'):
            changed = json.loads(json.dumps(row))
            if mutation == 'foreign': changed['Id'] = 'b' * 64
            if mutation == 'stopped': changed['State']['Running'] = False
            if mutation == 'ambiguous': changed['NetworkSettings']['Ports']['5432/tcp'][1]['HostPort'] = '32770'
            if mutation == 'missing': changed['NetworkSettings']['Ports']['5432/tcp'] = None
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                port.resolve(container, json.dumps([changed]))


if __name__ == '__main__':
    unittest.main()
