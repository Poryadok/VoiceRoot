"""One-shot root-to-verified-consumer transfer; no cluster mutation."""
import array
import argparse
import fcntl
import json
import os
from pathlib import Path
import re
import socket
import stat
import struct
import sys

import realtime_p3_root_export as existing
import space_creator_runtime_profile_issuer_packet as packet

ROOT=Path('/home/voice-dev/issuance')
SOCKET_ROOT=Path('/home/pmd/voice-space-creator/operations')
FRAME=b'SPACE-CREATOR-PROFILE\n'
ACK=b'SPACE-CREATOR-ACCEPTED\n'
ACK_TIMEOUT_SECONDS=120

class ExportError(RuntimeError): pass

def _read(path,limit):
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        before=os.fstat(fd)
        if (not stat.S_ISREG(before.st_mode) or before.st_uid!=0 or before.st_nlink!=1 or
            stat.S_IMODE(before.st_mode)!=0o600 or not 0<before.st_size<=limit): raise ValueError()
        raw=bytearray()
        while len(raw)<before.st_size:
            part=os.read(fd,min(65536,before.st_size-len(raw)))
            if not part: raise ValueError()
            raw.extend(part)
        def identity(s): return (s.st_dev,s.st_ino,s.st_size,s.st_mtime_ns,s.st_ctime_ns,s.st_mode,s.st_uid,s.st_nlink)
        if identity(before)!=identity(os.fstat(fd)) or identity(before)!=identity(os.stat(path,follow_symlinks=False)):
            raise ValueError()
        return bytes(raw)
    finally: os.close(fd)

def _unique(pairs):
    result={}
    for key,value in pairs:
        if key in result: raise ValueError()
        result[key]=value
    return result

def _socket_path(suffix):
    uid=existing._pmd_uid()
    for directory in (SOCKET_ROOT.parent,SOCKET_ROOT,SOCKET_ROOT/suffix):
        info=directory.lstat()
        if (not stat.S_ISDIR(info.st_mode) or info.st_uid!=uid or stat.S_IMODE(info.st_mode)!=0o700 or
            directory.resolve()!=directory): raise ValueError()
    path=SOCKET_ROOT/suffix/'packet.sock'; info=path.lstat()
    if not stat.S_ISSOCK(info.st_mode) or info.st_uid!=uid or info.st_mode&0o077: raise ValueError()
    return path

def export(operation_id,source_sha,namespace_uid,preflight_sha256,consumer_pid,consumer_start_ticks):
    """Attempt one durable handoff of a previously guarded private packet."""
    try:
        if (os.geteuid()!=0 or not re.fullmatch(r'media-c1-[a-f0-9]{32}',operation_id) or
            not re.fullmatch(r'[a-f0-9]{40}',source_sha) or not re.fullmatch(r'[a-f0-9]{64}',preflight_sha256) or
            not isinstance(namespace_uid,str) or not namespace_uid or type(consumer_pid) is not int or consumer_pid<=0 or
            type(consumer_start_ticks) is not int or consumer_start_ticks<=0): raise ValueError()
        suffix=operation_id[-32:]; existing._check_root_custody(ROOT)
        directory=ROOT/('space-creator-'+suffix); existing._check_root_custody(directory)
        if ((directory/'handoff-started').exists() or (directory/'handoff-started').is_symlink() or
            _read(ROOT/'space-creator-issuance.once',128)!=(directory.name+'\n').encode('ascii') or
            _read(directory/'complete',1024)!=b'Creator profile issuance complete; runtime not qualified.\n'):
            raise ValueError()
        envelope=json.loads(_read(directory/'objects.json',524288),object_pairs_hook=_unique); media=_read(directory/'media-public.crt',65536)
        missing=envelope['issuer_receipt']['object_names']
        packet.validate(envelope,operation_id,source_sha,namespace_uid,preflight_sha256,missing,media)
        raw=json.dumps(envelope,sort_keys=True,separators=(',',':'),allow_nan=False).encode()
        fd=os.memfd_create('voice-space-creator',os.MFD_CLOEXEC|os.MFD_ALLOW_SEALING)
        try:
            os.fchmod(fd,0o600); data=memoryview(raw)
            while data:
                count=os.write(fd,data)
                if count<=0: raise ValueError()
                data=data[count:]
            os.lseek(fd,0,os.SEEK_SET); fcntl.fcntl(fd,fcntl.F_ADD_SEALS,existing.REQUIRED_SEALS)
            if fcntl.fcntl(fd,fcntl.F_GET_SEALS)&existing.REQUIRED_SEALS!=existing.REQUIRED_SEALS: raise ValueError()
            with socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET) as channel:
                channel.settimeout(ACK_TIMEOUT_SECONDS); channel.connect(str(_socket_path(suffix)))
                pid,uid,_=struct.unpack('3i',channel.getsockopt(socket.SOL_SOCKET,socket.SO_PEERCRED,struct.calcsize('3i')))
                if uid!=existing._pmd_uid() or pid!=consumer_pid or existing._start_ticks(pid)!=consumer_start_ticks: raise ValueError()
                marker=os.open(directory/'handoff-started',os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
                try: os.write(marker,b'Potentially sent; reconcile before further handoff.\n'); os.fsync(marker)
                finally: os.close(marker)
                dfd=os.open(directory,os.O_RDONLY|os.O_DIRECTORY)
                try: os.fsync(dfd)
                finally: os.close(dfd)
                if channel.sendmsg([FRAME],[(socket.SOL_SOCKET,socket.SCM_RIGHTS,array.array('i',[fd]))])!=len(FRAME): raise ValueError()
                response,ancillary,flags,_=channel.recvmsg(128,0)
                if ancillary or flags&(socket.MSG_TRUNC|socket.MSG_CTRUNC) or response!=ACK: raise ValueError()
        finally: os.close(fd)
        return {'status':'send_acknowledged','receiver_verified':False,'admission_qualified':False}
    except Exception: raise ExportError('Creator profile handoff failed or uncertain; reconcile') from None

def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--operation-id',required=True); parser.add_argument('--source-sha',required=True)
    parser.add_argument('--namespace-uid',required=True); parser.add_argument('--preflight-sha256',required=True)
    parser.add_argument('--consumer-pid',required=True,type=int); parser.add_argument('--consumer-start-ticks',required=True,type=int)
    args=parser.parse_args()
    print(json.dumps(export(args.operation_id,args.source_sha,args.namespace_uid,args.preflight_sha256,
                            args.consumer_pid,args.consumer_start_ticks),sort_keys=True))

if __name__=='__main__':
    try: main()
    except Exception:
        print('Creator profile handoff failed or uncertain; reconcile',file=sys.stderr)
        sys.exit(1)
