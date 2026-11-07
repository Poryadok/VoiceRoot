import copy
import unittest

import space_fresh_install_phase as phase


class CatalogTest(unittest.TestCase):
    def setUp(self):
        self.original = {'all_databases':{'postgres':1,'space_db':2}, 'databases':{'postgres':1,'space_db':2},
                         'excluded':{}, 'properties':{'postgres':{'owner':'voice'},'space_db':{'owner':'voice'}}}

    def test_exact_old_absent_and_new_states(self):
        phase._check_catalog(self.original, self.original, 'drop_pending', 2, None)
        absent = copy.deepcopy(self.original)
        for key in ('all_databases','databases','properties'): del absent[key]['space_db']
        for state in ('drop_done','job_pending','job_owned'):
            phase._check_catalog(self.original, absent, state, 2, None)
        new = copy.deepcopy(absent)
        new['all_databases']['space_db'] = new['databases']['space_db'] = 9
        new['properties']['space_db'] = {'owner':'voice'}
        for state in ('created','done'): phase._check_catalog(self.original, new, state, 2, 9)

    def test_uncertain_or_other_database_changes_are_rejected(self):
        absent = copy.deepcopy(self.original)
        for key in ('all_databases','databases','properties'): del absent[key]['space_db']
        with self.assertRaises(ValueError): phase._check_catalog(self.original, absent, 'drop_pending', 2, None)
        other = copy.deepcopy(absent); other['all_databases']['postgres'] = 3
        with self.assertRaises(ValueError): phase._check_catalog(self.original, other, 'drop_done', 2, None)
        with self.assertRaises(ValueError): phase._check_catalog(self.original, absent, 'created', 2, 9)
        with self.assertRaises(ValueError): phase._check_catalog(self.original, absent, 'done', 2, 2)


if __name__ == '__main__': unittest.main()
