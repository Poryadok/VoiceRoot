"""Explicit local Linux Docker integration; disposable JWTs only.

No staging resources, credentials, or Kubernetes endpoint are accepted here.
The test volume is prepared by TestDockerFixtureInputs, under a fixed prefix.
"""
import json
import os
import re
from pathlib import Path
import shutil
import sys
import time
import uuid

from controller import Blocked, file_sha, verify_archive
from docker_runtime import DockerRuntime, BOX_IMAGE


from scenario import fixture as run_fixture, staging_baseline


if __name__ == '__main__':
    prepared = Path('/var/lib/docker/volumes/voice-known-test-20261004/_data')
    operation='test'+uuid.uuid4().hex[:12]
    base=prepared/operation; base.mkdir(mode=0o700)
    (base/'inputs').mkdir(mode=0o750); os.chown(base/'inputs',0,65532)
    for role in ('bootstrap','social','realtime'):
        path=base/'inputs'/(role+'.creds')
        shutil.copyfile(prepared/'inputs'/(role+'.creds'),path)
        path.chmod(0o440); os.chown(path,0,65532)
    for name in ('server.conf','kernel','bootstrap-realtime.sh','bootstrap-notification.sh','bootstrap-analytics-chat.sh','bootstrap-search.sh'):
        path=base/name; shutil.copyfile(prepared/name,path)
        path.chmod(0o555 if name=='kernel' else 0o440); os.chown(path,0,65532)
    runtime = DockerRuntime(base,operation)
    try:
        fixture_receipt=run_fixture(runtime)
        store=base/'final-clean-test-store'; store.mkdir(mode=0o700)
        baseline_receipt=staging_baseline(runtime,store)
        assert fixture_receipt['archive_sha256']!=baseline_receipt['archive_sha256']
        print(json.dumps({'fixture':fixture_receipt,'zero_baseline':baseline_receipt},sort_keys=True))
    except Blocked:
        for name in runtime.owned:
            if 'kernel' in name:
                logs=runtime.run(['logs',name])
                print(json.dumps({'kernel_error_codes':re.findall(r'(?m)^[a-z_]{1,80}$',logs)}))
            if 'bootstrap' in name:
                row=runtime.inspect(name)
                logs=runtime.run(['logs',name])
                errors=[re.sub(r'[A-Za-z0-9_.=-]{32,}','[REDACTED]',line)[:300] for line in logs.splitlines()[:8]]
                print(json.dumps({'test_container':name,'exit_code':row['State']['ExitCode'],
                    'test_error':errors[:2],
                    'test_error_tail':[re.sub(r'[A-Za-z0-9_.=-]{32,}','[REDACTED]',line)[:300] for line in logs.splitlines()[-2:]],
                    'authorization_violation':'authorization violation' in logs.lower(),
                    'permission_violation':'permissions violation' in logs.lower(),
                    'stream_not_found':'stream not found' in logs.lower(),
                    'connection_refused':'connection refused' in logs.lower(),
                    'missing_jq':'jq: not found' in logs.lower(),
                    'permission_denied':'permission denied' in logs.lower(),
                    'not_found':'not found' in logs.lower(),
                    'no_responders':'no responders' in logs.lower(),
                    'timeout':'timeout' in logs.lower(),
                    'nats_error':'nats: error:' in logs.lower(),
                    'invalid':'invalid' in logs.lower(),
                    'incompatible':'incompatible' in logs.lower()}))
        raise
    finally:
        for name in reversed(runtime.owned):
            runtime.inspect(name)
            runtime.run(['rm','-f',name])
