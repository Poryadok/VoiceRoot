"""Check that the historical fixture runs from its exact candidate checkout."""
import argparse
from pathlib import Path

from go_integration_matrix import verify_source_checkout

ROOT = Path(__file__).resolve().parents[2]
parser = argparse.ArgumentParser()
parser.add_argument('--candidate', required=True)
args = parser.parse_args()
verify_source_checkout(ROOT, args.candidate, [
    ROOT / 'scripts/ci/space_large34_required.json',
    ROOT / 'scripts/ci/space_large34_proof.sh',
    ROOT / 'scripts/ci/space_large34_source.py',
    ROOT / 'scripts/ci/space_large34_manifest.py',
])
