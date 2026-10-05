"""Captured node runner; target checkout is data, never privileged deploy code."""
import hashlib
import os
from pathlib import Path
import sys
sys.path.insert(0,str(Path(__file__).resolve().parent))
import guard
import apply
from compiler import compatible_bootstrap,decode_yaml
from controller import Blocked


def preflight():
    receipt=guard.protected_receipt(os.environ['VOICE_NATS_PRESERVATION_RECEIPT'])
    root=guard.source_root()
    guard.validate(receipt,os.environ['VOICE_IMAGE_REGISTRY'],os.environ['VOICE_IMAGE_TAG'],os.environ['DEPLOY_MODE'],
                   [s for s in os.environ.get('CHANGED_SERVICES','').split(',') if s],root)
    if Path(os.environ['VOICE_NATS_PRESERVATION_RECEIPT']).parent.name!='rollout-'+receipt['operation']:
        raise Blocked('rollout_operation_path_changed')
    frontend_only=guard.frontend_image_only(receipt['target'])
    acl=hashlib.sha256((root/'deploy/nats/acl-intent.yaml').read_bytes()).hexdigest()
    if not frontend_only and (os.environ.get('VOICE_NATS_ACL_PROOF_SHA')!=acl or os.environ.get('VOICE_NATS_ACL_PROOF_GENERATION')!=receipt['marker']['generation']):
        raise Blocked('rollout_acl_proof_missing')
    guard.anchor(receipt,'rollout-prepared',receipt['marker']['resourceVersion'])
    import json
    known=json.loads((Path(__file__).resolve().parents[1]/'nats-known-baseline/deployed-contract.json').read_bytes())
    if not frontend_only:compatible_bootstrap(root,known,decode_yaml)
    return receipt


def main(args):
    if args not in ([],['--preflight']):raise Blocked('rollout_runner_arguments_invalid')
    receipt=preflight()
    if args:
        print('NATS_ROLLOUT_PREFLIGHT=PASS');return
    rv=guard.claim(receipt)
    os.environ['VOICE_NATS_ROLLOUT_CLAIM_RV']=rv
    print('NATS_ROLLOUT_CLAIM_RV='+rv,flush=True)
    if os.environ.get('GITHUB_OUTPUT'):
        with open(os.environ['GITHUB_OUTPUT'],'a') as output:output.write('claim_rv='+rv+'\n')
    apply.main()


if __name__=='__main__':
    try:main(sys.argv[1:])
    except Exception:
        print('NATS_ROLLOUT_RUNNER=BLOCKED',file=sys.stderr);sys.exit(1)
