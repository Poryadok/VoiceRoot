from pathlib import Path
import unittest

import full_cohort_schema_metadata_space_catalog as subject


class CatalogTest(unittest.TestCase):
    def test_source_pinned_read_only_schema_locked_check(self):
        root = Path(__file__).resolve().parents[2]
        sql = subject.compile_sql(root, {'files': subject.FILES})
        for part in ('REPEATABLE READ READ ONLY', 'LOCK TABLE', "version=25 AND NOT dirty",
                     "t.tgenabled='O'", "p.provolatile='v'", "pg_catalog.sha256(pg_catalog.convert_to(p.prosrc",
                     'count(DISTINCT e.ordinal)=5'):
            self.assertIn(part, sql)
        with self.assertRaises(subject.Hold):
            subject.compile_sql(root, {'files': {}})


if __name__ == '__main__':
    unittest.main()
