"""One root-authenticated, sealed P3 packet handoff over Unix seqpacket."""
import array
import fcntl
import json
import os
import re
import socket
import stat
import struct

FRAME = b'P3-PACKET\n'
MAX_PACKET = 2 * 1024 * 1024
REQUIRED_SEALS = fcntl.F_SEAL_WRITE | fcntl.F_SEAL_GROW | fcntl.F_SEAL_SHRINK | fcntl.F_SEAL_SEAL


class PacketError(RuntimeError): pass


def _unique(pairs):
    result = {}
    for key,value in pairs:
        if key in result: raise ValueError()
        result[key] = value
    return result


def receive(listener, operation_id, source_sha):
    """Consume one connection; returned packet is private in-process only."""
    fds = []
    try:
        if (not isinstance(operation_id,str) or not re.fullmatch(r'media-c[12]-[0-9a-f]{32}',operation_id)
                or not isinstance(source_sha,str) or not re.fullmatch(r'[0-9a-f]{40}',source_sha)
                or listener.family != socket.AF_UNIX or listener.type & socket.SOCK_SEQPACKET != socket.SOCK_SEQPACKET): raise ValueError()
        listener.settimeout(300)
        connection, _ = listener.accept()
        with connection:
            connection.settimeout(30)
            _, uid, _ = struct.unpack('3i',connection.getsockopt(socket.SOL_SOCKET,socket.SO_PEERCRED,struct.calcsize('3i')))
            if uid != 0: raise ValueError()
            data, ancillary, flags, _ = connection.recvmsg(len(FRAME)+1,socket.CMSG_SPACE(8*array.array('i').itemsize),socket.MSG_CMSG_CLOEXEC)
            invalid = False
            for level,typ,payload in ancillary:
                if level != socket.SOL_SOCKET or typ != socket.SCM_RIGHTS:
                    invalid = True; continue
                numbers = array.array('i')
                if len(payload) % numbers.itemsize: invalid = True
                numbers.frombytes(payload[:len(payload)-(len(payload)%numbers.itemsize)]); fds.extend(numbers)
            if invalid or data != FRAME or flags & (socket.MSG_TRUNC|socket.MSG_CTRUNC) or len(fds) != 1 or len(ancillary) != 1: raise ValueError()
        fd = fds[0]; info = os.fstat(fd)
        if (not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_nlink != 0
                or stat.S_IMODE(info.st_mode) != 0o600 or not 0 < info.st_size <= MAX_PACKET
                or fcntl.fcntl(fd,fcntl.F_GET_SEALS) & REQUIRED_SEALS != REQUIRED_SEALS): raise ValueError()
        raw = bytearray(); offset = 0
        while offset < info.st_size:
            chunk = os.pread(fd,min(65536,info.st_size-offset),offset)
            if not chunk: raise ValueError()
            raw.extend(chunk); offset += len(chunk)
        after = os.fstat(fd)
        if (info.st_dev,info.st_ino,info.st_size,info.st_mtime_ns,info.st_ctime_ns) != (
                after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns,after.st_ctime_ns): raise ValueError()
        value = json.loads(raw,object_pairs_hook=_unique)
        if (not isinstance(value,dict) or set(value) != {'operation_id','source_sha','issuer_receipt','objects'}
                or value['operation_id'] != operation_id or value['source_sha'] != source_sha): raise ValueError()
        return value
    except BaseException:
        raise PacketError('P3 root packet handoff unavailable') from None
    finally:
        for fd in fds:
            try: os.close(fd)
            except OSError: pass
