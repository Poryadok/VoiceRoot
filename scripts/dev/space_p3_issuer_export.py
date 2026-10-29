"""One-shot root-to-operator sealed Space packet handoff; no Kube access."""
import array
import argparse
import fcntl
import json
import os
from pathlib import Path
import re
import socket
import struct
import sys

import realtime_p3_root_export as existing
import space_p3_issuer_packet as packet

ROOT = Path('/home/voice-dev/issuance')
SOCKET_ROOT = Path('/home/pmd/voice-space-p3/operations')


class ExportError(RuntimeError): pass


def _socket_path(suffix):
    uid = existing._pmd_uid()
    for directory in (SOCKET_ROOT.parent,SOCKET_ROOT,SOCKET_ROOT/suffix):
        info = directory.lstat()
        if (not __import__('stat').S_ISDIR(info.st_mode) or info.st_uid != uid
            or __import__('stat').S_IMODE(info.st_mode) != 0o700 or directory.resolve() != directory): raise ValueError()
    path = SOCKET_ROOT/suffix/'packet.sock'; info = path.lstat()
    if not __import__('stat').S_ISSOCK(info.st_mode) or info.st_uid != uid or info.st_mode & 0o077: raise ValueError()
    return path


def export(operation_id,source_sha,consumer_pid,consumer_start_ticks):
    try:
        if (os.geteuid()!=0 or type(consumer_pid) is not int or consumer_pid<=0
            or type(consumer_start_ticks) is not int or consumer_start_ticks<=0
            or not re.fullmatch(r'media-c[12]-[a-f0-9]{32}',operation_id)
            or not re.fullmatch(r'[a-f0-9]{40}',source_sha)): raise ValueError()
        suffix=operation_id[-32:]; existing._check_root_custody(ROOT)
        directory=ROOT/('space-p3-'+suffix); existing._check_root_custody(directory)
        if ((directory/'handoff-started').exists() or (directory/'handoff-started').is_symlink()
            or existing._read(ROOT/'space-p3-issuance.once',128)!=(directory.name+'\n').encode()
            or existing._read(directory/'complete',1024)!=b'Offline issuance complete; runtime not qualified.\n'): raise ValueError()
        receipt=json.loads(existing._read(directory/'receipt.json',65536))
        objects=json.loads(existing._read(directory/'objects.json',262144))
        envelope={'operation_id':operation_id,'source_sha':source_sha,'issuer_receipt':receipt,'objects':objects}
        media=existing._read(directory/'media-public.crt',65536)
        packet.validate(envelope,operation_id,source_sha,media)
        raw=json.dumps(envelope,separators=(',',':')).encode()
        if len(raw)>524288: raise ValueError()
        fd=os.memfd_create('voice-space-p3',os.MFD_CLOEXEC|os.MFD_ALLOW_SEALING)
        try:
            os.fchmod(fd,0o600); remaining=memoryview(raw)
            while remaining:
                written=os.write(fd,remaining)
                if written<=0: raise ValueError()
                remaining=remaining[written:]
            os.lseek(fd,0,os.SEEK_SET); fcntl.fcntl(fd,fcntl.F_ADD_SEALS,existing.REQUIRED_SEALS)
            if fcntl.fcntl(fd,fcntl.F_GET_SEALS)&existing.REQUIRED_SEALS != existing.REQUIRED_SEALS: raise ValueError()
            with socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET) as channel:
                channel.settimeout(10); channel.connect(str(_socket_path(suffix)))
                pid,uid,_=struct.unpack('3i',channel.getsockopt(socket.SOL_SOCKET,socket.SO_PEERCRED,struct.calcsize('3i')))
                if uid!=existing._pmd_uid() or pid!=consumer_pid or existing._start_ticks(pid)!=consumer_start_ticks: raise ValueError()
                marker=os.open(directory/'handoff-started',os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
                try: os.write(marker,b'Potentially sent; reconcile before any further handoff.\n'); os.fsync(marker)
                finally: os.close(marker)
                dfd=os.open(directory,os.O_RDONLY|os.O_DIRECTORY)
                try: os.fsync(dfd)
                finally: os.close(dfd)
                frame=b'P3-PACKET\n'
                if channel.sendmsg([frame],[(socket.SOL_SOCKET,socket.SCM_RIGHTS,array.array('i',[fd]))])!=len(frame): raise ValueError()
        finally: os.close(fd)
        return {'status':'send_attempted','receiver_verified':False,'admission_qualified':False}
    except Exception:
        raise ExportError('Space handoff failed or uncertain; retain packet and reconcile') from None


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--operation-id',required=True); parser.add_argument('--source-sha',required=True)
    parser.add_argument('--consumer-pid',required=True,type=int); parser.add_argument('--consumer-start-ticks',required=True,type=int)
    args=parser.parse_args()
    print(json.dumps(export(args.operation_id,args.source_sha,args.consumer_pid,args.consumer_start_ticks),sort_keys=True))


if __name__ == '__main__':
    try: main()
    except Exception:
        print('Space handoff failed or uncertain; retain packet and reconcile',file=sys.stderr)
        sys.exit(1)
