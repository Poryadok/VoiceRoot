"""Fixed Creator identities from the existing media CA; no cluster authority.

The private generation primitive must only be invoked by the source-owned root
guard after it establishes the exact protected missing subset. It is not an
operator entry point or a substitute for that guard.
"""
import hashlib
import argparse
import json
import os
from pathlib import Path
import re
import socket
import struct
import sys
import tempfile

import credentials
import realtime_p3_missing_issuer as existing
import reset_credentials as reset
import space_p3_issuer as ca_custody
import space_creator_runtime_profile_issuer_packet as packet
import space_creator_runtime_profile_issuer_export as exporter

class IssuerError(RuntimeError): pass

def _unique(pairs):
    result={}
    for key,value in pairs:
        if key in result: raise ValueError()
        result[key]=value
    return result

def _exchange(channel,operation_id,source_sha):
    from space_p3_issuer_preflight_metadata_root import MEMBERS
    if channel.send(b'PREFLIGHT-REQUEST\n')!=len(b'PREFLIGHT-REQUEST\n'): raise ValueError()
    raw,ancillary,flags,_=channel.recvmsg(65537,1)
    if ancillary or flags&(socket.MSG_TRUNC|socket.MSG_CTRUNC) or not 0<len(raw)<=65536: raise ValueError()
    request=json.loads(raw,object_pairs_hook=_unique)
    if channel.recv(64)!=b'PREFLIGHT-READY\n': raise ValueError()
    if (set(request)!={'context','namespace_uid','lock','ca','creator_secrets','preflight_sha256'} or
        request['context'].get('operation_id')!=operation_id or request['context'].get('commit')!=source_sha or
        not re.fullmatch(r'[a-f0-9]{64}',request['preflight_sha256']) or
        set(request['creator_secrets'])!=set(MEMBERS) or set(MEMBERS)!=set(packet.FAMILIES)):
        raise ValueError()
    for name,item in request['creator_secrets'].items():
        if item=={'state':'absent'}: continue
        if (not isinstance(item,dict) or set(item)!={'state','uid','resourceVersion','keys'} or item['state']!='present' or
            not all(isinstance(item[k],str) and item[k] for k in ('uid','resourceVersion')) or
            item['keys']!=sorted(MEMBERS[name])): raise ValueError()
    return request

def _guard(request):
    import space_p3_issuer_preflight_metadata_root as metadata
    import space_p3_issuer_preflight_root as preflight
    preflight._check_ca(request['ca'])
    if metadata.observe(request['namespace_uid'],request['context'],request['lock'],request['ca'])!=request['creator_secrets']:
        raise ValueError()
    preflight._check_ca(request['ca'])

def _clean_env():
    return (not set(os.environ)-{'PATH','LANG','LC_ALL','LC_CTYPE'} and os.environ.get('PATH')=='/usr/bin:/bin' and
            os.environ.get('LANG')=='C' and os.environ.get('LC_ALL')=='C')

def stage(operation_id,source_sha,consumer_pid,consumer_start_ticks):
    """One guarded issuance; all failed or uncertain attempts require reconciliation."""
    try:
        if (os.geteuid()!=0 or not _clean_env() or not re.fullmatch(r'media-c1-[a-f0-9]{32}',operation_id) or
            not re.fullmatch(r'[a-f0-9]{40}',source_sha) or type(consumer_pid) is not int or consumer_pid<=0 or
            type(consumer_start_ticks) is not int or consumer_start_ticks<=0): raise ValueError()
        exporter.existing._check_root_custody(exporter.ROOT); suffix=operation_id[-32:]
        with socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET) as channel:
            channel.settimeout(120); channel.connect(str(exporter._socket_path(suffix)))
            pid,uid,_=struct.unpack('3i',channel.getsockopt(socket.SOL_SOCKET,socket.SO_PEERCRED,struct.calcsize('3i')))
            if uid!=exporter.existing._pmd_uid() or pid!=consumer_pid or exporter.existing._start_ticks(pid)!=consumer_start_ticks: raise ValueError()
            first=_exchange(channel,operation_id,source_sha); _guard(first)
            missing=sorted(name for name,item in first['creator_secrets'].items() if item=={'state':'absent'})
            if not missing: raise ValueError()
            destination=exporter.ROOT/('space-creator-'+suffix)
            reservation=exporter.ROOT/'space-creator-issuance.once'
            fd=os.open(reservation,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
            try: os.write(fd,('space-creator-'+suffix+'\n').encode('ascii')); os.fsync(fd)
            finally: os.close(fd)
            parent_fd=os.open(exporter.ROOT,os.O_RDONLY|os.O_DIRECTORY)
            try: os.fsync(parent_fd)
            finally: os.close(parent_fd)
            destination.mkdir(mode=0o700); credentials.write_private(destination/'pending',b'Generation begun; reconcile before retry.\n')
            existing._sync_file(destination/'pending')
            dfd=os.open(destination,os.O_RDONLY|os.O_DIRECTORY)
            try: os.fsync(dfd)
            finally: os.close(dfd)
            cert_fd,cert_identity=ca_custody._open('media-ca.crt',False)
            try:
                media=os.read(cert_fd,65537)
                if not existing._unchanged(cert_fd,cert_identity) or len(media)!=cert_identity[2]: raise ValueError()
            finally: os.close(cert_fd)
            _guard(first)
            old_umask=os.umask(0o077); old_tempdir=tempfile.tempdir; tempfile.tempdir=str(destination)
            try:
                receipt=_prepare(destination,media,missing,operation_id,source_sha,first['namespace_uid'],first['preflight_sha256'])
            finally:
                tempfile.tempdir=old_tempdir; os.umask(old_umask)
            second=_exchange(channel,operation_id,source_sha)
            if second!=first: raise ValueError()
            _guard(second)
            credentials.write_private(destination/'receipt.json',json.dumps(receipt,sort_keys=True,separators=(',',':'))+'\n')
            for path in destination.iterdir(): existing._sync_file(path)
            credentials.write_private(destination/'complete',b'Creator profile issuance complete; runtime not qualified.\n')
            existing._sync_file(destination/'complete')
            for path in (destination,exporter.ROOT):
                fd=os.open(path,os.O_RDONLY|os.O_DIRECTORY)
                try: os.fsync(fd)
                finally: os.close(fd)
            return {'status':'staged','object_names':missing,'runtime_qualified':False,'cluster_applied':False}
    except Exception: raise IssuerError('Creator profile issuance unavailable; reconcile if attempted') from None

def _prepare(destination,media,missing,operation_id,source_sha,namespace_uid,preflight_sha256):
    """Generate a fixed subset after an external source-owned guard has admitted it."""
    try:
        if (not _clean_env() or
            not isinstance(missing,list) or not missing or missing!=sorted(set(missing)) or set(missing)-set(packet.FAMILIES) or
            not re.fullmatch(r'media-c1-[a-f0-9]{32}',operation_id) or not re.fullmatch(r'[a-f0-9]{40}',source_sha) or
            not isinstance(namespace_uid,str) or not namespace_uid or not re.fullmatch(r'[a-f0-9]{64}',preflight_sha256) or
            not isinstance(media,bytes)): raise ValueError()
        destination=Path(destination); ca_custody._check_ancestors(ca_custody.ARCHIVE)
        info=ca_custody.ARCHIVE.lstat()
        if (not __import__('stat').S_ISDIR(info.st_mode) or info.st_uid!=0 or info.st_mode&0o077 or
            ca_custody.ARCHIVE.resolve()!=ca_custody.ARCHIVE): raise ValueError()
        cert_fd,cert_id=ca_custody._open('media-ca.crt',False)
        try:
            key_fd,key_id=ca_custody._open('media-ca.key',True)
            try:
                cert=Path('/proc')/str(os.getpid())/'fd'/str(cert_fd); key=Path('/proc')/str(os.getpid())/'fd'/str(key_fd)
                public=destination/'media-public.crt'; credentials.write_private(public,media)
                cert_der=reset._openssl('x509','-in',cert,'-outform','DER')
                if (cert_der!=reset._openssl('x509','-in',public,'-outform','DER') or
                    reset._openssl('x509','-in',cert,'-pubkey','-noout')!=reset._openssl('pkey','-in',key,'-pubout') or
                    b'CA:TRUE' not in reset._openssl('x509','-in',cert,'-noout','-ext','basicConstraints')): raise ValueError()
                material=reset.ResetMaterial(destination)
                for name in missing:
                    spec=packet.FAMILIES[name]
                    if spec is None: material.signer(name,active=True)
                    else: material.tls(name,(key,cert),*spec)
                signer=next((item for item in material.items if item['metadata']['name']=='voice-gateway-principal-signing'),None)
                receipt={'schema':'voice-space-creator-profile-issuer-v1','status':'staged','namespace':'voice-dev',
                         'object_names':missing,'media_ca_der_sha256':hashlib.sha256(cert_der).hexdigest(),
                         'public_jwks':reset.signer_jwks(signer) if signer else None,
                         'runtime_qualified':False,'cluster_applied':False}
                envelope={'schema':'voice-space-creator-profile-packet-v1','operation_id':operation_id,'source_sha':source_sha,
                          'namespace_uid':namespace_uid,'preflight_sha256':preflight_sha256,'issuer_receipt':receipt,
                          'objects':{'apiVersion':'v1','kind':'List','items':material.items}}
                packet.validate(envelope,operation_id,source_sha,namespace_uid,preflight_sha256,missing,media)
                if not existing._unchanged(cert_fd,cert_id) or not existing._unchanged(key_fd,key_id): raise ValueError()
                ca_custody._check_ancestors(ca_custody.ARCHIVE)
                def identity(info): return (info.st_dev,info.st_ino,info.st_size,info.st_mtime_ns,info.st_ctime_ns)
                if (identity(os.stat(ca_custody.ARCHIVE/'media-ca.crt',follow_symlinks=False))!=cert_id or
                    identity(os.stat(ca_custody.ARCHIVE/'media-ca.key',follow_symlinks=False))!=key_id): raise ValueError()
                credentials.write_private(destination/'objects.json',json.dumps(envelope,sort_keys=True,separators=(',',':'))+'\n')
                return receipt
            finally: os.close(key_fd)
        finally: os.close(cert_fd)
    except Exception: raise IssuerError('Creator profile generation refused') from None

def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--operation-id',required=True); parser.add_argument('--source-sha',required=True)
    parser.add_argument('--consumer-pid',required=True,type=int); parser.add_argument('--consumer-start-ticks',required=True,type=int)
    args=parser.parse_args()
    print(json.dumps(stage(args.operation_id,args.source_sha,args.consumer_pid,args.consumer_start_ticks),sort_keys=True))

if __name__=='__main__':
    try: main()
    except Exception:
        print('Creator profile issuance unavailable; reconcile if attempted',file=sys.stderr)
        sys.exit(1)
