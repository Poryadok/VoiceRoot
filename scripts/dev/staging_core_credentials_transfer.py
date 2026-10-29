"""One-shot private transfer of the user-authorized account file to STAGING.

This is preparation only. An unknown SSH outcome requires reconciliation;
the destination is exclusive-create and is never overwritten or removed here.
"""
import os
import shlex
import subprocess
import sys
from pathlib import Path

from staging_core_credentials import decode

SOURCE = r'C:\Users\Sergey\.codex\tmp\voice-staging-core-accounts\accounts.json'

_ACL = r'''$ErrorActionPreference='Stop'
$p='C:\Users\Sergey\.codex\tmp\voice-staging-core-accounts\accounts.json'
$a=[System.IO.File]::GetAccessControl($p)
$me=[System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$owner=$a.GetOwner([System.Security.Principal.SecurityIdentifier]).Value
$seen=@{}
foreach($e in $a.GetAccessRules($true,$true,[System.Security.Principal.SecurityIdentifier])) {
  $s=$e.IdentityReference.Translate([System.Security.Principal.SecurityIdentifier]).Value
  if (($s -ne $me -and $s -ne 'S-1-5-18') -or $e.AccessControlType -ne 'Allow') { exit 2 }
  $seen[$s]=$true
}
if (($owner -ne $me -and $owner -ne 'S-1-5-32-544') -or -not $seen.ContainsKey($me) -or -not $seen.ContainsKey('S-1-5-18')) { exit 2 }
'''

_REMOTE = r'''import os,stat,sys
root=os.open('/home/pmd',os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
try:
 s=os.fstat(root)
 if s.st_uid!=os.geteuid() or s.st_mode&0o022: raise RuntimeError()
 try: os.mkdir('voice-staging-core-accounts',0o700,dir_fd=root)
 except FileExistsError: pass
 d=os.open('voice-staging-core-accounts',os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=root)
 try:
  s=os.fstat(d)
  if s.st_uid!=os.geteuid() or stat.S_IMODE(s.st_mode)!=0o700: raise RuntimeError()
  data=sys.stdin.buffer.read(65537)
  if not 1<=len(data)<=65536: raise RuntimeError()
  f=os.open('accounts.json',os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600,dir_fd=d)
  with os.fdopen(f,'wb') as out:
   out.write(data);out.flush();os.fsync(out.fileno())
  os.fsync(d);os.fsync(root)
 finally: os.close(d)
finally: os.close(root)
'''


def _read_authorized_source():
    # Keep every path component open without delete sharing, and the file open
    # without write/delete sharing, across both ACL checks and the entire read.
    import ctypes
    from ctypes import wintypes
    import msvcrt
    class Info(ctypes.Structure):
        _fields_ = [('attributes', wintypes.DWORD), ('created', wintypes.FILETIME),
                    ('accessed', wintypes.FILETIME), ('modified', wintypes.FILETIME),
                    ('volume', wintypes.DWORD), ('size_high', wintypes.DWORD), ('size_low', wintypes.DWORD),
                    ('links', wintypes.DWORD), ('index_high', wintypes.DWORD), ('index_low', wintypes.DWORD)]
    kernel = ctypes.WinDLL('kernel32', use_last_error=True)
    kernel.CreateFileW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD, wintypes.LPVOID,
                                   wintypes.DWORD, wintypes.DWORD, wintypes.HANDLE]
    kernel.CreateFileW.restype = wintypes.HANDLE
    kernel.GetFileInformationByHandle.argtypes = [wintypes.HANDLE, ctypes.POINTER(Info)]
    kernel.GetFileInformationByHandle.restype = wintypes.BOOL
    kernel.CloseHandle.argtypes = [wintypes.HANDLE]
    kernel.GetCurrentProcess.restype = wintypes.HANDLE
    invalid = ctypes.c_void_p(-1).value
    handles = []
    def acquire(path, directory):
        # FILE_READ_ATTRIBUTES (directory) or GENERIC_READ (file); no delete
        # sharing, and the file also denies write sharing.
        handle = kernel.CreateFileW(str(path), 0x80 if directory else 0x80000000,
                                    3 if directory else 1, None, 3,
                                    0x00200000 | (0x02000000 if directory else 0), None)
        if handle == invalid:
            raise ValueError()
        handles.append(handle)
        info = Info()
        if not kernel.GetFileInformationByHandle(handle, ctypes.byref(info)):
            raise ValueError()
        if info.attributes & 0x400 or bool(info.attributes & 0x10) != directory or (not directory and info.links != 1):
            raise ValueError()
        return handle, info
    def check_acl():
        subprocess.run(['powershell.exe', '-NoProfile', '-NonInteractive', '-Command', _ACL],
                       check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=15)
    try:
        path = Path(SOURCE)
        for parent in reversed(path.parents):
            acquire(parent, True)
        handle, first = acquire(path, False)
        check_acl()
        # Duplicate the file handle for Python I/O while retaining the locked
        # original handle until the second pathname/ACL check has completed.
        process = kernel.GetCurrentProcess()
        duplicate = wintypes.HANDLE()
        kernel.DuplicateHandle.argtypes = [wintypes.HANDLE, wintypes.HANDLE, wintypes.HANDLE,
                                           ctypes.POINTER(wintypes.HANDLE), wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
        if not kernel.DuplicateHandle(process, handle, process, ctypes.byref(duplicate), 0, False, 2):
            raise ValueError()
        try:
            fd = msvcrt.open_osfhandle(duplicate.value, os.O_RDONLY | os.O_BINARY)
        except BaseException:
            kernel.CloseHandle(duplicate)
            raise
        with os.fdopen(fd, 'rb') as stream:
            raw = stream.read(65537)
        _, second = acquire(path, False)
        if ((first.volume, first.index_high, first.index_low, first.size_high, first.size_low) !=
                (second.volume, second.index_high, second.index_low, second.size_high, second.size_low) or
                len(raw) != (first.size_high << 32) + first.size_low):
            raise ValueError()
        check_acl()
        return raw
    finally:
        for handle in reversed(handles): kernel.CloseHandle(handle)


def transfer():
    if sys.platform != 'win32':
        raise RuntimeError('private transfer requires authorized Windows operator')
    try:
        raw = _read_authorized_source()
        decode(raw)
        config = subprocess.run(['ssh', '-G', 'voice-staging'], check=True, stdout=subprocess.PIPE,
                                stderr=subprocess.DEVNULL, timeout=15).stdout.decode('utf-8')
        rows = dict(line.split(' ', 1) for line in config.splitlines() if ' ' in line)
        if rows.get('hostname') != '192.168.0.109' or rows.get('user') != 'pmd' or rows.get('port') != '22':
            raise ValueError()
        command = 'python3 -c ' + shlex.quote(_REMOTE)
        subprocess.run(['ssh', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes',
                        '-o', 'HostName=192.168.0.109', '-o', 'User=pmd', '-o', 'Port=22',
                        '-o', 'ClearAllForwardings=yes', '-o', 'ProxyCommand=none', '-o', 'ProxyJump=none',
                        'voice-staging', command],
                       input=raw, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=30)
    except BaseException:
        raise RuntimeError('private transfer failed or outcome uncertain; reconcile without retry') from None


if __name__ == '__main__':
    transfer()
