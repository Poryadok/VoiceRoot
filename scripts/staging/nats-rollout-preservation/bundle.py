"""Build the reviewable offline human-root launcher and exact code archive."""
import hashlib
import io
import json
from pathlib import Path
import sys
import tarfile

KNOWN=('root_main.py','controller.py','commands.py','docker_runtime.py','stage_runtime.py','scenario.py','deployed-contract.json')
ROLLOUT=('apply.py','bridge.py','bridge_client.py','bridge_root.py','compiler.py','encrypted_cut.py','errors.py','github_custody.py','guard.py','installer.py','migrations.py','native_store.py','normalize.py','nonnats_plan.py','nonnats_runtime.py','source_authority.py','source_plan.py','rollout_census.py','preserve.py','root_cli.py','runner.py','runtime_stage.py','transaction.py','workflow_entry.py')

def build(kernel,output):
    source=Path(__file__).resolve().parents[1];kernel=Path(kernel);output=Path(output)
    raw=kernel.read_bytes()
    if not raw.startswith(b'\x7fELF') or len(raw)>32<<20:raise ValueError('Linux kernel required')
    contents={}
    for directory,names in (('nats-known-baseline',KNOWN),('nats-rollout-preservation',ROLLOUT)):
        for name in names:
            path=source/directory/name
            if path.is_symlink():raise ValueError('source symlink')
            contents[directory+'/'+name]=path.read_bytes()
    contents['nats-known-baseline/kernel']=raw
    contents['configure-kubectl-ci.sh']=(source/'configure-kubectl-ci.sh').read_bytes()
    contents['mail-only-patch.py']=(source/'mail-only-patch.py').read_bytes()
    contents['capture-manifest.json']=json.dumps({n:hashlib.sha256(v).hexdigest() for n,v in contents.items()},sort_keys=True).encode()
    result=io.BytesIO()
    with tarfile.open(fileobj=result,mode='w',format=tarfile.USTAR_FORMAT) as archive:
        for name,data in sorted(contents.items()):
            member=tarfile.TarInfo(name);member.size=len(data);member.mode=0o400;archive.addfile(member,io.BytesIO(data))
    bundle=result.getvalue();sha=hashlib.sha256(bundle).hexdigest()
    output.mkdir(parents=True,exist_ok=True)
    (output/'rollout-bundle.tar').write_bytes(bundle)
    template=(source/'nats-rollout-preservation/root-launch.template.sh').read_text()
    launcher=template.replace('__BUNDLE_SHA256__',sha).replace('__BUNDLE_BYTES__',str(len(bundle))).replace('__BUNDLE_FILES__',repr(sorted(contents)))
    (output/'rollout-root.sh').write_text(launcher,newline='\n')
    print('NATS_ROLLOUT_BUNDLE_SHA256='+sha)
    return sha

if __name__=='__main__':
    if len(sys.argv)!=3:raise SystemExit('usage: bundle.py LINUX_KERNEL OUTPUT_DIR')
    build(*sys.argv[1:])
