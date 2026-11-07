import tempfile
import os
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path

import auth_candidate_classpath as candidate


class CandidateClasspathTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.test = self.root / 'src/backend/auth/target/test-classes'
        self.classes = self.root / 'src/backend/auth/target/classes'
        self.test.mkdir(parents=True)
        self.classes.mkdir(parents=True)
        self.jar = self.root / 'dependency.jar'
        self.jar.write_bytes(b'fixture')
        self.report = self.root / 'report.xml'

    def write_report(self, cp, **values):
        root = ET.Element('testsuite', {'name': candidate.TEST, 'tests': '4', 'errors': '0',
                                       'failures': '0', 'skipped': '0', **values})
        props = ET.SubElement(root, 'properties')
        ET.SubElement(props, 'property', {'name': 'java.class.path', 'value': cp})
        for i in range(4):
            ET.SubElement(root, 'testcase', {'classname': candidate.TEST, 'name': 'case' + str(i)})
        ET.ElementTree(root).write(self.report, encoding='utf-8')

    def test_valid_report_removes_only_trailing_empty_entry(self):
        cp = os.pathsep.join(map(str, (self.test, self.classes, self.jar)))
        self.write_report(cp + os.pathsep)
        self.assertEqual(candidate.extract(self.report, self.root), cp)

    def test_skips_failures_and_foreign_or_missing_classpath_fail(self):
        cp = os.pathsep.join(map(str, (self.test, self.classes, self.jar)))
        for values in ({'skipped': '1'}, {'errors': '1'}, {'tests': '0'}, {'name': 'foreign'}):
            with self.subTest(values=values):
                self.write_report(cp, **values)
                with self.assertRaises(ValueError):
                    candidate.extract(self.report, self.root)
        for bad in (os.pathsep.join(map(str, (self.classes, self.test, self.jar))),
                    str(self.test) + os.pathsep * 2 + str(self.jar), cp + os.pathsep + str(self.root / 'missing.jar')):
            with self.subTest(cp=bad):
                self.write_report(bad)
                with self.assertRaises((ValueError, OSError)):
                    candidate.extract(self.report, self.root)

    def test_report_cannot_hide_missing_or_skipped_testcases(self):
        cp = os.pathsep.join(map(str, (self.test, self.classes, self.jar)))
        self.write_report(cp)
        tree = ET.parse(self.report)
        first = tree.getroot().find('testcase')
        ET.SubElement(first, 'skipped')
        tree.write(self.report, encoding='utf-8')
        with self.assertRaises(ValueError):
            candidate.extract(self.report, self.root)
        self.write_report(cp)
        tree = ET.parse(self.report)
        tree.getroot().remove(tree.getroot().find('testcase'))
        tree.write(self.report, encoding='utf-8')
        with self.assertRaises(ValueError):
            candidate.extract(self.report, self.root)


if __name__ == '__main__':
    unittest.main()
