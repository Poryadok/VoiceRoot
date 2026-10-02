# Paste this reviewed command into the operator's root shell on pmdebook.
# Replace REVIEWED_INSPECTOR_SHA256 with the independently reviewed inspector.py SHA.
# Do not execute this template from the pmd-owned upload directory as root.
/usr/bin/env -i PATH=/usr/sbin:/usr/bin:/sbin:/bin LANG=C.UTF-8 /usr/bin/python3.13 -I -S - <<'PY'
import hashlib
import os
from pathlib import Path
import stat
import tempfile

pin = "REVIEWED_INSPECTOR_SHA256"
source = "/home/pmd/voice-nats-preservation/20261002/inspector.py"
if os.geteuid() != 0 or len(pin) != 64 or any(c not in "0123456789abcdef" for c in pin):
    raise SystemExit("ROOT_AND_REVIEWED_PIN_REQUIRED")
for path in (Path("/"), Path("/root"), Path("/usr"), Path("/usr/bin"), Path("/usr/bin/python3.13")):
    s = path.lstat()
    if stat.S_ISLNK(s.st_mode) or s.st_uid != 0 or s.st_mode & 0o022:
        raise SystemExit("UNTRUSTED_BOOTSTRAP_PATH")
os.umask(0o077)
private = Path(tempfile.mkdtemp(prefix="voice-pvc-inspector-", dir="/root"))
target = private / "inspector.py"
src = os.open(source, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
try:
    if not stat.S_ISREG(os.fstat(src).st_mode):
        raise SystemExit("PUBLIC_SOURCE_NOT_REGULAR")
    dst = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        while chunk := os.read(src, 1024 * 1024):
            view = memoryview(chunk)
            while view:
                view = view[os.write(dst, view):]
        os.fsync(dst)
    finally:
        os.close(dst)
finally:
    os.close(src)
# Hash the completed private copy, never a path check followed by public execution.
fd = os.open(target, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
try:
    h = hashlib.sha256()
    while chunk := os.read(fd, 1024 * 1024):
        h.update(chunk)
    if h.hexdigest() != pin:
        raise SystemExit("COPIED_SCRIPT_SHA_MISMATCH_NO_EXECUTION")
finally:
    os.close(fd)
os.execve("/usr/bin/python3.13", ["python3.13", "-I", "-S", str(target)],
          {"PATH": "/usr/sbin:/usr/bin:/sbin:/bin", "LANG": "C.UTF-8"})
PY
