"""Disposable Linux-container verifier probe, not a broker or rollout PASS.

Uses the actual pinned renewer and its real inherited private result pipe.
The caller owns container lifecycle; no credential or JWT is printed.
"""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

ROOT=Path('/workspace')
sys.path.insert(0,str(ROOT/'scripts/staging/nats-known-baseline'))
sys.path.insert(0,str(ROOT/'scripts/staging/nats-rollout-preservation'))
from bootstrap_renewal import invoke,RenewalError
import actor_permissions
import actor_root

PIN='618f1f78e650d19c4948c1d08741ed625c6f9aa706159219dbf11c785a56237c'

def run(generator,binary):
    if hashlib.sha256(binary.read_bytes()).hexdigest()!=PIN:
        raise ValueError('renewal_binary_pin_changed')
    with tempfile.TemporaryDirectory(prefix='owned-crypto-',dir='/root') as temp:
        out=Path(temp)/'private';out.mkdir(mode=0o700)
        result=subprocess.run([str(generator),str(out),str(ROOT/'deploy/nats/acl-intent.yaml')],
                              stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
        if result.returncode:raise ValueError('owned_genesis_failed')
        genesis=json.loads((out/'genesis.json').read_bytes())
        credentials=(out/'bootstrap.creds').read_text()
        sha=lambda value:hashlib.sha256(value.encode()).hexdigest()
        request={'schema':'voice-nats-existing-actor-verification-v1','credentials':credentials,
                 'account_jwt':(out/'account.jwt').read_text(),'operator_jwt':(out/'operator.jwt').read_text(),
                 'actor_sha256':sha(genesis['users']['bootstrap']),'account_sha256':sha(genesis['account'])}
        actual=json.loads(invoke(binary,request,mode='--verify-existing-actor'))
        if (actual.get('signed_chain_verified') is not True or actual.get('actor_sha256')!=request['actor_sha256']
            or actual.get('account_sha256')!=request['account_sha256']
            or actual.get('credential_sha256')!=sha(credentials)):
            raise ValueError('owned_crypto_binding_invalid')
        grant=actor_root.source_grants((ROOT/'deploy/nats/acl-intent.yaml').read_bytes())['bootstrap']
        actor_permissions.evaluate(actual['effective'],{'pub':grant['publish'],'sub':grant['subscribe']})
        lines=credentials.splitlines(keepends=True)
        indexes=[i for i,line in enumerate(lines) if line.count('.')==2]
        if len(indexes)!=1:raise ValueError('owned_credential_shape_invalid')
        index=indexes[0];parts=lines[index].split('.');sig=parts[2]
        parts[2]=('A' if sig[0]!='A' else 'B')+sig[1:];lines[index]='.'.join(parts)
        changed=dict(request,credentials=''.join(lines))
        try:invoke(binary,changed,mode='--verify-existing-actor')
        except RenewalError:pass
        else:raise ValueError('invalid_signature_accepted')
    print('owned_crypto_private_pipe=PASS complete_acl=PASS invalid_signature=REFUSED')


if __name__=='__main__':
    try:
        if len(sys.argv)!=3 or os.name!='posix':raise ValueError('linux_inputs_required')
        run(Path(sys.argv[1]),Path(sys.argv[2]))
    except Exception as error:
        print('owned_crypto_private_pipe=FAIL '+type(error).__name__,file=sys.stderr)
        sys.exit(1)
