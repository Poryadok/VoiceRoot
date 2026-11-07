"""Extract a checked candidate classpath from the actual Maven test report."""
import argparse
import os
from pathlib import Path
import sys
import xml.etree.ElementTree as ET


TEST = 'voice.backend.auth.sessionepoch.AuthChatFloorInteropFixtureTest'


def extract(report, checkout):
    root = ET.parse(report).getroot()
    if root.tag != 'testsuite' or root.get('name') != TEST:
        raise ValueError('unexpected Maven test report')
    try:
        count = int(root.attrib['tests'])
        if count < 1 or any(int(root.attrib[key]) != 0 for key in ('failures', 'errors', 'skipped')):
            raise ValueError('Maven fixture tests did not all pass')
    except (KeyError, TypeError) as exc:
        raise ValueError('incomplete Maven test report') from exc
    cases = root.findall('./testcase')
    identities = [(case.get('classname'), case.get('name')) for case in cases]
    if (len(cases) != count or len(set(identities)) != count
            or any(not name or not classname for classname, name in identities)
            or any(case.find(tag) is not None for case in cases for tag in ('skipped', 'failure', 'error'))):
        raise ValueError('Maven testcase inventory is incomplete or not passing')
    properties = [item.get('value') for item in root.findall('./properties/property')
                  if item.get('name') == 'java.class.path']
    if len(properties) != 1 or not isinstance(properties[0], str):
        raise ValueError('Maven classpath is missing or ambiguous')
    entries = properties[0].split(os.pathsep)
    if entries and entries[-1] == '':
        entries.pop()  # Surefire may append its empty current-directory entry.
    if len(entries) < 3 or any(not entry or '\n' in entry or '\r' in entry for entry in entries):
        raise ValueError('Maven classpath is incomplete')
    checkout = Path(checkout).resolve(strict=True)
    for value, relative in zip(entries[:2], ('src/backend/auth/target/test-classes', 'src/backend/auth/target/classes')):
        if Path(value).resolve(strict=True) != (checkout / relative).resolve(strict=True):
            raise ValueError('Maven classpath does not use candidate outputs')
    if any(not Path(value).is_absolute() or not Path(value).is_file() or not value.endswith('.jar') for value in entries[2:]):
        raise ValueError('Maven dependency classpath is invalid')
    return os.pathsep.join(entries)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--report', required=True)
    parser.add_argument('--checkout', required=True)
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    try:
        result = extract(args.report, args.checkout)
        with Path(args.output).open('x', encoding='utf-8') as stream:
            stream.write(result + '\n')
    except (ET.ParseError, OSError, ValueError) as exc:
        print('candidate classpath rejected: ' + str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
