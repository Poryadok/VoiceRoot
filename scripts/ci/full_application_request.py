"""Validate a source-owned request; a request is not deployment evidence."""
import json
import os
import re
import subprocess

REQUEST_PATH = '.github/ci/full-application-build-request.json'
SCHEMA = 'voice-full-application-build-request-v1'


def parse(raw):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError('duplicate key')
            result[key] = value
        return result
    if not isinstance(raw, bytes) or len(raw) > 4096:
        raise ValueError('request size')
    try:
        value = json.loads(raw, object_pairs_hook=unique)
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ValueError('request JSON') from exc
    if (type(value) is not dict or set(value) != {'schema', 'cycle', 'request_id'}
            or value['schema'] != SCHEMA or type(value['cycle']) is not int
            or value['cycle'] not in (1, 2) or type(value['request_id']) is not str
            or not re.fullmatch('[a-f0-9]{32}', value['request_id'])):
        raise ValueError('request schema')
    return value


def validate_transition(previous, current):
    new = parse(current)
    if previous is None:
        if new['cycle'] != 1:
            raise ValueError('initial cycle')
    else:
        old = parse(previous)
        if new['request_id'] == old['request_id'] or new['cycle'] < old['cycle']:
            raise ValueError('request transition')
    return new


def git(*args):
    return subprocess.check_output(['git', *args], stderr=subprocess.DEVNULL)


def blob(revision):
    entry = git('ls-tree', revision, '--', REQUEST_PATH)
    if not entry:
        return None
    if not re.fullmatch(rb'100644 blob [a-f0-9]{40}\t' + re.escape(REQUEST_PATH.encode()) + rb'\n', entry):
        raise ValueError('request tree entry')
    return git('show', revision + ':' + REQUEST_PATH)


def main():
    sha = os.environ['GITHUB_SHA']
    if not re.fullmatch('[a-f0-9]{40}', sha) or git('rev-parse', 'HEAD').decode().strip() != sha:
        raise ValueError('request commit')
    git('rev-parse', '--verify', sha + '^1')
    before, after = blob(sha + '^1'), blob(sha)
    if after is None or before == after:
        raise ValueError('request not changed in source commit')
    validate_transition(before, after)
    with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as out:
        out.write('requested=true\n')


if __name__ == '__main__':
    main()
