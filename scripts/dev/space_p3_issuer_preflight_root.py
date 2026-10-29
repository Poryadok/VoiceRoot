"""Fixed human-root read-only Space media-CA custody check and handoff."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import socket
import stat
import struct

import realtime_p3_missing_issuer as existing
import realtime_p3_root_export as root_export
import reset_credentials as reset
import space_p3_issuer as issuer
import space_p3_issuer_preflight_metadata_root as metadata

SOCKET_ROOT=Path('/home/pmd/voice-space-p3/preflight')


def _socket(suffix):
    uid=root_export._pmd_uid()
    for directory in (SOCKET_ROOT.parent,SOCKET_ROOT,SOCKET_ROOT/suffix):
        info=directory.lstat()
        if (not stat.S_ISDIR(info.st_mode) or info.st_uid!=uid
                or stat.S_IMODE(info.st_mode)!=0o700 or directory.resolve()!=directory): raise ValueError()
    path=SOCKET_ROOT/suffix/'custody.sock'; info=path.lstat()
    if not stat.S_ISSOCK(info.st_mode) or info.st_uid!=uid or info.st_mode & 0o077: raise ValueError()
    return path


def _check_ca(ca):
    existing._check_ancestors(issuer.ARCHIVE); info=issuer.ARCHIVE.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid!=0 or info.st_mode & 0o077: raise ValueError()
    cert_fd,cert_identity=issuer._open('media-ca.crt',False)
    try:
        key_fd,key_identity=issuer._open('media-ca.key',True)
        try:
            cert=Path('/proc')/str(os.getpid())/'fd'/str(cert_fd)
            key=Path('/proc')/str(os.getpid())/'fd'/str(key_fd)
            der=reset._openssl('x509','-in',cert,'-outform','DER')
            if (hashlib.sha256(der).hexdigest()!=ca['der_sha256']
                    or reset._openssl('x509','-in',cert,'-pubkey','-noout')!=reset._openssl('pkey','-in',key,'-pubout')
                    or b'CA:TRUE' not in reset._openssl('x509','-in',cert,'-noout','-ext','basicConstraints')
                    or not existing._unchanged(cert_fd,cert_identity) or not existing._unchanged(key_fd,key_identity)): raise ValueError()
            if (not existing._unchanged(cert_fd,cert_identity) or not existing._unchanged(key_fd,key_identity)
                    or (issuer.ARCHIVE/'media-ca.crt').lstat().st_ino!=cert_identity[1]
                    or (issuer.ARCHIVE/'media-ca.key').lstat().st_ino!=key_identity[1]): raise ValueError()
        finally: os.close(key_fd)
    finally: os.close(cert_fd)


def check(operation_id,pid,start_ticks):
    """Check fixed TT, Kubernetes and root custody against the live phase request."""
    if (os.geteuid()!=0 or not re.fullmatch(r'media-c[12]-[a-f0-9]{32}',operation_id)
            or type(pid) is not int or pid<=0 or type(start_ticks) is not int or start_ticks<=0): raise ValueError()
    with socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET) as channel:
        channel.settimeout(120); channel.connect(str(_socket(operation_id[-32:])))
        peer,uid,_=struct.unpack('3i',channel.getsockopt(socket.SOL_SOCKET,socket.SO_PEERCRED,struct.calcsize('3i')))
        if uid!=root_export._pmd_uid() or peer!=pid or root_export._start_ticks(peer)!=start_ticks: raise ValueError()
        raw,ancillary,flags,_=channel.recvmsg(65537,1)
        if ancillary or flags & (socket.MSG_TRUNC|socket.MSG_CTRUNC) or len(raw)>65536: raise ValueError()
        request=json.loads(raw,object_pairs_hook=_unique)
        if set(request)!={'context','namespace_uid','nonce','ca','lock'}: raise ValueError()
        context=request['context']; ca=request['ca']; namespace_uid=request['namespace_uid']; nonce=request['nonce']
        metadata.resource_fencing.validate_context(context)
        if (context['operation_id']!=operation_id or not re.fullmatch(r'[a-f0-9]{32}',nonce)
                or set(ca)!={'uid','resourceVersion','der_sha256'} or not re.fullmatch(r'[a-f0-9]{64}',ca['der_sha256'])): raise ValueError()
        _check_ca(ca); observations=metadata.observe(namespace_uid,context,request['lock'],ca); _check_ca(ca)
        canonical=lambda value:hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()
        response={'schema':'voice-space-media-ca-custody-v1','operation_id':operation_id,'source_sha':context['commit'],
                  'namespace_uid':namespace_uid,'nonce':nonce,'context_sha256':canonical(context),
                  'lock':{k:request['lock']['metadata'][k] for k in ('uid','resourceVersion')},'ca':ca,
                  'creator_secrets':observations,'scope':'root-custody-available-not-issued'}
        payload=json.dumps(response,sort_keys=True,separators=(',',':')).encode('ascii')
        if len(payload)>4096 or channel.send(payload)!=len(payload): raise ValueError()


def _unique(pairs):
    result={}
    for key,value in pairs:
        if key in result: raise ValueError()
        result[key]=value
    return result


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--operation-id',required=True)
    parser.add_argument('--consumer-pid',required=True,type=int); parser.add_argument('--consumer-start-ticks',required=True,type=int)
    args=parser.parse_args()
    check(args.operation_id,args.consumer_pid,args.consumer_start_ticks)


if __name__=='__main__':
    try: main()
    except Exception: raise SystemExit('Space custody preflight unavailable') from None
