"""Opt-in fixed Compose fixture bridge; never satisfy a Compose check.

The caller must preserve exclusive daemon, socket, evidence and source custody
and authenticate helper, image and tool identities. The fixture is independent
of Voice Compose stacks. It is an incomplete diagnostic, not a preflight pass.
"""
import hashlib
import json
from pathlib import Path
import re
import sys

import local_compose_audit
import local_compose_private
import local_process_supervision as supervision

CHECKS = frozenset({'compose-e2e', 'compose-config'})
INPUT_KEYS = {'helper', 'helper_sha256', 'postgres_archive', 'postgres_archive_sha256', 'postgres_image_id'}


def validate_inputs(inputs):
    if (not isinstance(inputs, dict) or set(inputs) != INPUT_KEYS or
            any(not isinstance(inputs[key], str) or not inputs[key] for key in INPUT_KEYS) or
            any(not Path(inputs[key]).is_absolute() for key in ('helper', 'postgres_archive')) or
            any(not re.fullmatch(r'[0-9a-f]{64}', inputs[key]) for key in ('helper_sha256', 'postgres_archive_sha256')) or
            not re.fullmatch(r'sha256:[0-9a-f]{64}', inputs['postgres_image_id'])):
        raise ValueError('complete explicit Compose fixture inputs and exact digests are required')


def validate_diagnostic(directory, inputs):
    directory = Path(directory).resolve()
    diagnostic = json.loads((directory / 'diagnostic.json').read_text(encoding='utf-8'))
    owner = json.loads((directory / 'owner.json').read_text(encoding='utf-8'))
    if not isinstance(diagnostic, dict) or not isinstance(owner, dict):
        raise ValueError('invalid Compose fixture evidence')
    project, slot = diagnostic.get('project'), diagnostic.get('slot')
    if (diagnostic.get('schema') != 'voice-local-compose-private-v1' or diagnostic.get('status') != 'incomplete' or
            not isinstance(project, str) or not re.fullmatch(r'voiceci-[0-9a-f]{32}', project) or
            type(slot) is not int or slot not in (0, 1) or owner.get('run_id') != project or owner.get('slot') != slot or
            diagnostic.get('archive_sha256') != inputs['postgres_archive_sha256'] or
            diagnostic.get('postgres_image_id') != inputs['postgres_image_id'] or
            not isinstance(diagnostic.get('client_image_id'), str) or
            not re.fullmatch(r'sha256:[0-9a-f]{64}', diagnostic['client_image_id'])):
        raise ValueError('Compose fixture identity or status mismatch')
    run = directory / 'run'
    scratch = run / 'tmp' / 'ci-local' / project
    journal_path = scratch / 'ownership.json'
    if owner.get('directory') != str(scratch) or diagnostic.get('ownership_journal') != str(journal_path):
        raise ValueError('Compose fixture ownership path mismatch')
    journal = json.loads(journal_path.read_text(encoding='utf-8'))
    resources = journal.get('resources') if isinstance(journal, dict) else None
    if (not isinstance(resources, list) or len(resources) != 7 or
            any(not isinstance(item, dict) or item.get('state') != 'removed' for item in resources) or
            sorted(item.get('kind') for item in resources) != ['container', 'container', 'network',
                                                               'volume', 'volume', 'volume', 'volume'] or
            journal.get('run_id') != project):
        raise ValueError('Compose fixture cleanup evidence differs')
    raw = (directory / 'compose-model.json').read_bytes()
    if len(raw) > 65536 or b'$' in raw or hashlib.sha256(raw).hexdigest() != diagnostic.get('model_sha256'):
        raise ValueError('Compose model digest or content differs')
    model = json.loads(raw, object_pairs_hook=local_compose_private._unique_object)
    if (model != local_compose_private.model(project) or local_compose_audit.audit(model, project, run, scratch) or
            (directory / 'compose.stdout').read_bytes() != local_compose_private.MARKER):
        raise ValueError('Compose fixed fixture evidence differs')
    return diagnostic


def run(plan, inputs, output):
    if inputs is None:
        return None
    validate_inputs(inputs)
    checks = plan.get('checks') if isinstance(plan, dict) else None
    if not isinstance(checks, dict) or any(not isinstance(check, str) for check in checks):
        raise ValueError('invalid affected check plan')
    selected = sorted(CHECKS.intersection(checks))
    if not selected:
        raise ValueError('Compose fixture requires a planner-selected Compose check')
    output = Path(output).resolve()
    output.mkdir(parents=True, exist_ok=False)
    command = [sys.executable, '-B', str(Path(__file__).resolve()), '--worker', str(output / 'fixture'),
               *(inputs[key] for key in ('helper', 'helper_sha256', 'postgres_archive',
                                         'postgres_archive_sha256', 'postgres_image_id'))]
    with (output / 'worker.stdout').open('xb') as stdout, (output / 'worker.stderr').open('xb') as stderr:
        result = supervision.run_diagnostic(command, timeout=1800, stdout=stdout, stderr=stderr)
    if result.returncode != 0:
        raise ValueError('supervised Compose fixture failed; preserve its ownership evidence')
    validate_diagnostic(output / 'fixture', inputs)
    report = {'schema': 'voice-local-compose-bridge-v1', 'status': 'incomplete',
              'selected_checks': selected, 'fixture_report': 'fixture/diagnostic.json',
              'limitations': ['fixed no-network fixture only', 'does not execute Voice Compose checks',
                              'helper, image and Compose tool provenance unverified',
                              'exclusive daemon, socket and evidence custody assumed',
                              'journal is trusted producer evidence, not independent Docker or slot observation',
                              'concurrency and crash acceptance unverified']}
    (output / 'bridge.json').write_text(json.dumps(report, indent=2, sort_keys=True) + '\n', encoding='utf-8')
    return report


if __name__ == '__main__':
    if len(sys.argv) != 8 or sys.argv[1] != '--worker':
        raise SystemExit('internal worker invocation required')
    local_compose_private.run(*sys.argv[2:])
