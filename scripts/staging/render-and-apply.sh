#!/usr/bin/env bash
# Every ordinary version change uses the root-established paused transaction.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export VOICE_IMAGE_REGISTRY="${VOICE_IMAGE_REGISTRY:-ghcr.io/voiceroot/voiceroot}"
export VOICE_IMAGE_TAG="${VOICE_IMAGE_TAG:?VOICE_IMAGE_TAG required}"
export VOICE_K8S_NAMESPACE="${VOICE_K8S_NAMESPACE:-voice-staging}"
export DEPLOY_MODE="${DEPLOY_MODE:-full}"
export VOICE_NATS_TARGET_SOURCE="${ROOT}"
if [[ -z "${VOICE_NATS_PRESERVATION_RECEIPT:-}" ]]; then
  echo 'ERROR: version-bound NATS preservation receipt is required' >&2
  exit 1
fi
# Read-only target/ACL/identity checks precede the single-use marker claim.
# The root-issued receipt and live root journal must carry current TT authority.
# Captured runner admission is repeated before marker claim and subprocess spawn.
# No legacy ingress/migration/bootstrap/replica-one path can bypass this entry.
python3 -I -S - <<'PY'
import hashlib, json, os, pathlib, re, runpy, stat, sys
try:
    root=pathlib.Path('/var/lib/voice-nats-preservation')
    receipt=pathlib.Path(os.environ['VOICE_NATS_PRESERVATION_RECEIPT'])
    if (receipt.parent.parent!=root or receipt.name!='apply-authorization.json'
        or not re.fullmatch('rollout-[a-f0-9]{12}',receipt.parent.name)):
        raise ValueError('receipt path')
    code=receipt.parent/'code'
    for directory in (*code.parents[::-1],code):
        info=directory.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid!=0 or info.st_mode&0o022:raise ValueError('code custody')
    def read(path,limit):
        fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
        try:
            info=os.fstat(fd)
            if (not stat.S_ISREG(info.st_mode) or info.st_uid!=0 or info.st_mode&0o022
                or info.st_nlink!=1 or not 1<=info.st_size<=limit):raise ValueError('code member')
            raw=os.read(fd,info.st_size+1)
            if len(raw)!=info.st_size:raise ValueError('code changed')
            return raw
        finally:os.close(fd)
    def pairs(values):
        row={}
        for key,value in values:
            if key in row:raise ValueError('duplicate member')
            row[key]=value
        return row
    manifest=json.loads(read(code/'capture-manifest.json',65536),object_pairs_hook=pairs)
    entry='nats-rollout-preservation/runner.py'
    if (not isinstance(manifest,dict) or not (1<=len(manifest)<=86 or
        len(manifest)==94 and hashlib.sha256(json.dumps(sorted(manifest),separators=(',',':')).encode()).hexdigest()
        == '13397edd8fae921ff480cd366fbcb63439fe00e55e3f8bfe9c00ea0c9790fe20' or
        len(manifest)==99 and hashlib.sha256(json.dumps(sorted(manifest),separators=(',',':')).encode()).hexdigest()
        == '9e08bb7df3c0dcdb9f4bf6287895eff08a4451062ca41199b79ca131da7465e4' or
        len(manifest)==100 and hashlib.sha256(json.dumps(sorted(manifest),separators=(',',':')).encode()).hexdigest()
        == '6d3ce2a5893bbb4d340a3a97daddb2e024f8ebfdb358db6641b0258e621bb002')
        or entry not in manifest or 'nats-rollout-preservation/resource_claim.py' not in manifest):raise ValueError('old capture')
    for relative,wanted in manifest.items():
        part=pathlib.Path(relative)
        if part.is_absolute() or '..' in part.parts or not isinstance(wanted,str) or not re.fullmatch('[a-f0-9]{64}',wanted):
            raise ValueError('manifest member')
        path=code/part
        for directory in (path.parent,*path.parent.parents):
            info=directory.lstat()
            if not stat.S_ISDIR(info.st_mode) or info.st_uid!=0 or info.st_mode&0o022:raise ValueError('member parent')
            if directory==code:break
        limit=(32 if relative=='nats-rollout-preservation/story-restored8-probe' else 16)<<20
        if hashlib.sha256(read(path,limit)).hexdigest()!=wanted:raise ValueError('member changed')
except Exception:
    raise SystemExit('STAGING_RESOURCE_CAPTURE_REFUSED') from None
sys.argv=[str(code/entry)]
runpy.run_path(str(code/entry),run_name='__main__')
PY
echo 'Staging target applied paused. Root preservation verification/resume is required.'
