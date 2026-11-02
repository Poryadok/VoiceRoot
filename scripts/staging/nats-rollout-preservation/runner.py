"""Captured node runner; target checkout is data, never privileged deploy code."""
import hashlib
import os
import re
from pathlib import Path
import sys
sys.path.insert(0,str(Path(__file__).resolve().parent))
import guard
import apply
from compiler import compatible_bootstrap,decode_yaml
from controller import Blocked
TT_FENCE_VERSION=1


def authenticated_bootstrap(root,contract,target):
    """Derive script pins only from YAML already bound by root authorization."""
    scripts={}
    for row in contract['scripts']:
        part=row['part'];relative='deploy/templates/nats-'+part+'-bootstrap.yaml'
        path=root/relative
        if path.is_symlink() or not path.is_file() or not 1<=path.stat().st_size<=131072:
            raise Blocked('rollout_bootstrap_source_invalid')
        raw=path.read_bytes()
        if hashlib.sha256(raw).hexdigest()!=target['source_hashes'].get(relative):
            raise Blocked('rollout_bootstrap_source_invalid')
        text=raw.decode('utf-8').replace('__NAMESPACE__',guard.NS).replace('__K_NAMESPACE__',guard.NS)
        decoded=decode_yaml(re.split(r'^---\s*$',text,flags=re.MULTILINE)[0].encode())
        script=decoded.get('data',{}).get('bootstrap.sh')
        if (decoded.get('kind')!='ConfigMap' or decoded.get('metadata',{}).get('name')!='voice-nats-'+part+'-bootstrap'
                or not isinstance(script,str)):
            raise Blocked('rollout_bootstrap_source_invalid')
        scripts[part]=hashlib.sha256(script.encode()).hexdigest()
    return scripts


def preflight(*,installation=None):
    receipt=guard.protected_receipt(os.environ['VOICE_NATS_PRESERVATION_RECEIPT'])
    root=guard.source_root()
    guard.validate(receipt,os.environ['VOICE_IMAGE_REGISTRY'],os.environ['VOICE_IMAGE_TAG'],os.environ['DEPLOY_MODE'],
                   [s for s in os.environ.get('CHANGED_SERVICES','').split(',') if s],root,installation=installation)
    if Path(os.environ['VOICE_NATS_PRESERVATION_RECEIPT']).parent.name!='rollout-'+receipt['operation']:
        raise Blocked('rollout_operation_path_changed')
    frontend_only=guard.frontend_image_only(receipt['target'])
    acl=hashlib.sha256((root/'deploy/nats/acl-intent.yaml').read_bytes()).hexdigest()
    if not frontend_only and (os.environ.get('VOICE_NATS_ACL_PROOF_SHA')!=acl or os.environ.get('VOICE_NATS_ACL_PROOF_GENERATION')!=receipt['marker']['generation']):
        raise Blocked('rollout_acl_proof_missing')
    guard.anchor(receipt,'rollout-prepared',receipt['marker']['resourceVersion'])
    import json
    known=json.loads((Path(__file__).resolve().parents[1]/'nats-known-baseline/deployed-contract.json').read_bytes())
    if not frontend_only:compatible_bootstrap(root,known,decode_yaml,authenticated_bootstrap(root,known,receipt['target']))
    return receipt


def _main(args,*,installation=None):
    if args not in ([],['--preflight']):raise Blocked('rollout_runner_arguments_invalid')
    receipt=preflight(installation=installation)
    if args:
        print('NATS_ROLLOUT_PREFLIGHT=PASS');return
    rv=guard.claim(receipt)
    os.environ['VOICE_NATS_ROLLOUT_CLAIM_RV']=rv
    print('NATS_ROLLOUT_CLAIM_RV='+rv,flush=True)
    if os.environ.get('GITHUB_OUTPUT'):
        with open(os.environ['GITHUB_OUTPUT'],'a') as output:output.write('claim_rv='+rv+'\n')
    apply.main()


def main(args):
    import resource_claim
    from root_cli import code_binding
    receipt=guard.protected_receipt(os.environ['VOICE_NATS_PRESERVATION_RECEIPT'])
    # Install before preflight and marker claim, not merely before kubectl apply.
    with resource_claim.runner_scope(receipt,guard.ROOT,code_binding(Path(__file__).resolve().parents[1])) as session:
        return _main(args,installation=session.installation)


if __name__=='__main__':
    try:main(sys.argv[1:])
    except Exception:
        print('NATS_ROLLOUT_RUNNER=BLOCKED',file=sys.stderr);sys.exit(1)
