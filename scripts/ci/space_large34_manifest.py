"""Publish bounded fixture metadata only after the controller verifies cleanup."""
import hashlib
import json
import os
from pathlib import Path

directory = Path(os.environ['RUNNER_TEMP'])
def digest(name):
    return hashlib.sha256((directory / name).read_bytes()).hexdigest()

data = {
    'schema': 'voice-space-large34-proof-v1',
    'candidate': os.environ['CANDIDATE_SHA'],
    'run': os.environ['GITHUB_RUN_ID'],
    'attempt': os.environ['GITHUB_RUN_ATTEMPT'],
    'test': 'TestAccountBanFixtureLargeCohort34',
    'event_sha256': digest('space-large34-events.jsonl'),
    'receipt_sha256': digest('space-large34-receipt.json'),
    'database': 'space_large34',
    'owned_oid': int(os.environ['SPACE_LARGE34_OWNED_OID']),
    'cleanup': 'removed',
}
with (directory / 'space-large34-proof.json').open('x', encoding='utf-8') as output:
    json.dump(data, output, sort_keys=True)
    output.write('\n')
