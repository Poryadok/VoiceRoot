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


def check(operation_id,source_sha,namespace_uid,nonce,ca_uid,ca_rv,ca_der_sha256,pid,start_ticks):
    """Send only public metadata after validating the existing root keypair."""
    if (os.geteuid()!=0 or not re.fullmatch(r'media-c[12]-[a-f0-9]{32}',operation_id)
            or not re.fullmatch(r'[a-f0-9]{40}',source_sha) or not re.fullmatch(r'[a-f0-9]{32}',nonce)
            or not re.fullmatch(r'[a-f0-9]{64}',ca_der_sha256)
            or any(not isinstance(x,str) or not x or len(x)>256 or '\n' in x for x in (namespace_uid,ca_uid,ca_rv))
            or type(pid) is not int or pid<=0 or type(start_ticks) is not int or start_ticks<=0): raise ValueError()
    existing._check_ancestors(issuer.ARCHIVE); info=issuer.ARCHIVE.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid!=0 or info.st_mode & 0o077: raise ValueError()
    cert_fd,cert_identity=issuer._open('media-ca.crt',False)
    try:
        key_fd,key_identity=issuer._open('media-ca.key',True)
        try:
            cert=Path('/proc')/str(os.getpid())/'fd'/str(cert_fd)
            key=Path('/proc')/str(os.getpid())/'fd'/str(key_fd)
            der=reset._openssl('x509','-in',cert,'-outform','DER')
            if (hashlib.sha256(der).hexdigest()!=ca_der_sha256
                    or reset._openssl('x509','-in',cert,'-pubkey','-noout')!=reset._openssl('pkey','-in',key,'-pubout')
                    or b'CA:TRUE' not in reset._openssl('x509','-in',cert,'-noout','-ext','basicConstraints')
                    or not existing._unchanged(cert_fd,cert_identity) or not existing._unchanged(key_fd,key_identity)): raise ValueError()
            response={'schema':'voice-space-media-ca-custody-v1','operation_id':operation_id,'source_sha':source_sha,
                      'namespace_uid':namespace_uid,'nonce':nonce,'ca':{'uid':ca_uid,'resourceVersion':ca_rv,
                      'der_sha256':ca_der_sha256},'creator_secrets':metadata.observe(namespace_uid),
                      'scope':'root-custody-available-not-issued'}
            payload=json.dumps(response,sort_keys=True,separators=(',',':')).encode('ascii')
            if len(payload)>4096: raise ValueError()
            with socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET) as channel:
                channel.settimeout(10); channel.connect(str(_socket(operation_id[-32:])))
                peer,uid,_=struct.unpack('3i',channel.getsockopt(socket.SOL_SOCKET,socket.SO_PEERCRED,struct.calcsize('3i')))
                if uid!=root_export._pmd_uid() or peer!=pid or root_export._start_ticks(peer)!=start_ticks: raise ValueError()
                if (not existing._unchanged(cert_fd,cert_identity) or not existing._unchanged(key_fd,key_identity)
                        or (issuer.ARCHIVE/'media-ca.crt').lstat().st_ino!=cert_identity[1]
                        or (issuer.ARCHIVE/'media-ca.key').lstat().st_ino!=key_identity[1]): raise ValueError()
                if channel.send(payload)!=len(payload): raise ValueError()
        finally: os.close(key_fd)
    finally: os.close(cert_fd)


def main():
    parser=argparse.ArgumentParser()
    for name in ('operation-id','source-sha','namespace-uid','nonce','ca-uid','ca-rv','ca-der-sha256'): parser.add_argument('--'+name,required=True)
    parser.add_argument('--consumer-pid',required=True,type=int); parser.add_argument('--consumer-start-ticks',required=True,type=int)
    args=parser.parse_args()
    check(args.operation_id,args.source_sha,args.namespace_uid,args.nonce,args.ca_uid,args.ca_rv,args.ca_der_sha256,
          args.consumer_pid,args.consumer_start_ticks)


if __name__=='__main__':
    try: main()
    except Exception: raise SystemExit('Space custody preflight unavailable') from None
