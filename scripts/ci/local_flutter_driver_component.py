"""Execute the selected host Flutter device-driver component on Windows.

This local component is not a hosted Linux result or a whole-preflight pass.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil

import flutter_test_events
import local_flutter_execution as flutter_execution
import local_proto_execution as bounded
import local_source_snapshot as snapshot

SUITE = 'src/frontend/integration_test/device_driver_smoke_test.dart'
COMMANDS = ('pub-get', 'sqlite-prefetch', 'device-driver')


def _windows():
    return os.name == 'nt'


def test_summary(raw, suite):
    summary = flutter_test_events.summarize(flutter_test_events.read_events(raw), [suite], reject_skips=True)
    if summary['passed'] < 1 or summary['skipped']:
        raise ValueError('device-driver tests are empty or skipped')
    return summary


def _read(path):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > 16*1024*1024:
        raise ValueError('unsafe or oversized driver evidence')
    raw = path.read_bytes()
    if len(raw) > 16*1024*1024: raise ValueError('oversized driver evidence')
    return raw


def verify_component(output, source, base, report):
    if (not isinstance(report, dict) or report.get('schema') != 'voice-local-flutter-driver-component-v1' or
            report.get('status') != 'complete' or report.get('scope') != 'flutter-device-driver' or
            report.get('source') != source or report.get('base') != base or report.get('platform') != 'windows' or
            report.get('flutter_version') != '3.41.7'):
        raise ValueError('driver component identity differs')
    commands = report.get('commands')
    if (not isinstance(commands, list) or any(not isinstance(row, dict) for row in commands) or
            [row.get('name') for row in commands] != list(COMMANDS) or
            any(type(row.get('exit_code')) is not int or row['exit_code'] != 0 for row in commands)):
        raise ValueError('driver command evidence differs')
    output = Path(output)
    for row in commands:
        for suffix in ('stdout', 'stderr'):
            raw = _read(output/(row['name']+'.'+suffix))
            if hashlib.sha256(raw).hexdigest() != row.get(suffix+'_sha256'):
                raise ValueError('driver log digest differs')
    suite = str(output/'source'/SUITE)
    if test_summary(_read(output/'device-driver.stdout'), suite) != report.get('tests'):
        raise ValueError('driver event evidence differs')


def run(root, base, head, head_ref, output, flutter):
    import affected_check_plan
    import local_preflight
    root, output = Path(root).resolve(), Path(output).resolve()
    if not _windows() or output.exists() or output == root or root in output.parents:
        raise ValueError('driver component requires Windows and new external output')
    executable = shutil.which(str(flutter))
    if not executable or not Path(executable).is_absolute(): raise ValueError('Flutter executable unavailable')
    executable = Path(executable).resolve(strict=True)
    initial_hash = bounded._sha256(executable)
    local_preflight._handoff_refs(root, base, head, head_ref)
    plan = affected_check_plan.build_local_plan(root, base, head, head_ref)
    source = plan['source']
    if ('flutter-device-driver' not in plan['checks'] or source.get('head') != head or
            source.get('changed_paths') or source.get('raw_mismatch_paths')):
        raise ValueError('driver requires selected clean committed source')
    output.mkdir(parents=True)
    checkout = snapshot.materialize(root, source, output/'source')
    frontend = checkout/'src/frontend'
    env = {k:v for k,v in os.environ.items() if not k.upper().startswith(('GIT_', 'VOICE_', 'PYTHON', 'FLUTTER_', 'DART_', 'PUB_'))
           and not k.upper().endswith('_CAPTURE_DIR')}
    cache, temp = output/'pub-cache', output/'temp'
    cache.mkdir(); temp.mkdir()
    env.update(PUB_CACHE=str(cache), TEMP=str(temp), TMP=str(temp), TMPDIR=str(temp),
               FLUTTER_ROOT=str(executable.parent.parent),
               PYTHONDONTWRITEBYTECODE='1', PATH=str(executable.parent)+os.pathsep+env.get('PATH',''))
    version = bounded._execute([str(executable),'--version','--machine'], cwd=frontend, env=env, timeout=60)
    version_info = json.loads(version.stdout) if version.returncode == 0 else {}
    if (version_info.get('frameworkVersion') != '3.41.7' or
            Path(version_info.get('flutterRoot', '')).resolve() != executable.parent.parent):
        raise ValueError('Flutter version differs')
    commands = [
        ('pub-get', [str(executable),'pub','get','--enforce-lockfile']),
        ('sqlite-prefetch', ['powershell','-NoProfile','-ExecutionPolicy','Bypass','-File',
                             str(checkout/'scripts/ci/flutter-windows-prefetch-sqlite3.ps1'),'-RootDir',str(checkout)]),
        ('device-driver', [str(executable),'test','--no-pub','--reporter','json',
                           'integration_test/device_driver_smoke_test.dart','-d','flutter-tester',
                           '--dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080']),
    ]
    rows=[]; cache_hash=None
    for name, command in commands:
        result = bounded._execute(command, cwd=frontend, env=env, timeout=1800)
        row={'name':name,'exit_code':result.returncode}
        for suffix, raw in (('stdout',result.stdout),('stderr',result.stderr)):
            (output/(name+'.'+suffix)).write_bytes(raw)
            row[suffix+'_sha256']=hashlib.sha256(raw).hexdigest()
        rows.append(row)
        if result.returncode: raise ValueError(name+' failed')
        snapshot.verify_materialized(root,source,checkout,allowed_extra=flutter_execution.FLUTTER_OUTPUTS)
        if name == 'sqlite-prefetch': cache_hash=flutter_execution.directory_digest(cache)
        if name == 'device-driver' and flutter_execution.directory_digest(cache) != cache_hash:
            raise ValueError('dependency cache changed during driver test')
    tests=test_summary(_read(output/'device-driver.stdout'),str(output/'source'/SUITE))
    if (snapshot.capture(root,head) != source or bounded._sha256(executable) != initial_hash or
            affected_check_plan.build_local_plan(root,base,head,head_ref) != plan):
        raise ValueError('driver source, selection or executable changed')
    local_preflight._handoff_refs(root,base,head,head_ref)
    report={'schema':'voice-local-flutter-driver-component-v1','status':'complete','scope':'flutter-device-driver',
            'source':source,'base':base,'head_ref':head_ref,'platform':'windows','flutter_version':'3.41.7',
            'flutter_entrypoint_sha256':initial_hash,'pub_cache_sha256':cache_hash,'commands':rows,'tests':tests,
            'limitations':['Windows host tester differs from hosted Linux',
                           'Flutter SDK and external package publisher provenance is not authenticated',
                           'fetched refs are not a live remote observation']}
    verify_component(output,source,base,report)
    (output/'flutter-driver-component.json').write_text(json.dumps(report,indent=2,sort_keys=True)+'\n',encoding='utf-8')
    return report
