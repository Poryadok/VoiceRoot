# Reviewed public code only; manual operator root shell on pmdebook.
# Replace REVIEWED_LOCATOR_SHA256 after independent review.
/usr/bin/env -i PATH=/usr/sbin:/usr/bin:/sbin:/bin LANG=C.UTF-8 /usr/bin/python3.13 -I -S - <<'PY'
import hashlib
import os
from pathlib import Path
import stat
import tempfile

locator_pin = "REVIEWED_LOCATOR_SHA256"
if os.geteuid() != 0 or any(len(pin) != 64 or any(c not in "0123456789abcdef" for c in pin) for pin in (locator_pin,)):
    raise SystemExit("ROOT_AND_REVIEWED_PINS_REQUIRED")
for path in (Path("/"), Path("/root"), Path("/usr"), Path("/usr/bin"), Path("/usr/bin/python3.13")):
    s = path.lstat()
    if stat.S_ISLNK(s.st_mode) or s.st_uid != 0 or s.st_mode & 0o022:
        raise SystemExit("UNTRUSTED_BOOTSTRAP_PATH")
os.umask(0o077)
private = Path(tempfile.mkdtemp(prefix="voice-historical-locator-", dir="/root"))
for name, pin in (("locator.py", locator_pin),):
    source = "/home/pmd/voice-nats-preservation/20261003/" + name
    src = os.open(source, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC)
    try:
        s = os.fstat(src)
        if not stat.S_ISREG(s.st_mode) or not 0 < s.st_size <= 1048576:
            raise SystemExit("PUBLIC_CODE_TYPE_SIZE")
        data = bytearray()
        while chunk := os.read(src, min(65536, 1048577 - len(data))):
            data.extend(chunk)
            if len(data) > 1048576:
                raise SystemExit("PUBLIC_CODE_SIZE_BOUND")
    finally:
        os.close(src)
    if hashlib.sha256(data).hexdigest() != pin:
        raise SystemExit("CAPTURED_CODE_SHA_MISMATCH_NO_EXECUTION")
    dst = os.open(private / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        view = memoryview(data)
        while view:
            view = view[os.write(dst, view):]
        os.fsync(dst)
    finally:
        os.close(dst)
    verify = os.open(private / name, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
    try:
        copied = bytearray()
        while chunk := os.read(verify, 65536):
            copied.extend(chunk)
        if hashlib.sha256(copied).hexdigest() != pin:
            raise SystemExit("PRIVATE_CODE_SHA_MISMATCH_NO_EXECUTION")
    finally:
        os.close(verify)
os.execve("/usr/bin/python3.13", ["python3.13", "-I", "-S", str(private / "locator.py")],
          {"PATH": "/usr/sbin:/usr/bin:/sbin:/bin", "LANG": "C.UTF-8"})
PY
