# DEV media CA: root-only read-only custody check

Run this literal command on the DEV host as an interactive operator who can
authenticate to `sudo`. Review the entire command before running it. It does
not execute a script from the pmd-writable preparation directory, expose or
print the private key, create an issuance packet, or claim a rollout. OpenSSL
reads the key locally through a held file descriptor to derive its public key. It checks the
fixed archive expected by the reviewed Space issuer at commit
`a82249cd96b399424f7bc5e0e9913484db317afc`, against the public-only DEV
capture made by the operator. This is a preparation check; the active fenced
operation must recheck the CA and custody independently before issuance.

```sh
sudo /usr/bin/python3 -I -S - <<'PY'
import hashlib, os, pwd, re, stat, subprocess

archive = '/home/voice-dev/issuance/generation1'
public = '/home/pmd/voice-prepare-TKKQWcSr/p3-ca-public-4b148349b66742b783ea59a662388b2a/media-ca.pem'
expected = '9f181873e80e269b5adfe9b9991a7b6f8ffd8060fa3f38c6116675e9a877b828'
fds = []

def snapshot(fd):
    s = os.fstat(fd)
    return (s.st_dev, s.st_ino, s.st_size, s.st_mtime_ns, s.st_ctime_ns)

def open_checked(path, owner, private):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    fds.append(fd)
    s = os.fstat(fd)
    if (not stat.S_ISREG(s.st_mode) or s.st_uid != owner or s.st_nlink != 1
        or s.st_mode & (0o077 if private else 0o022) or not 0 < s.st_size <= 65536):
        raise ValueError()
    return fd, snapshot(fd)

def openssl(*args):
    result = subprocess.run(['/usr/bin/openssl', *args], stdin=subprocess.DEVNULL,
                            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                            check=True, timeout=30, env={'PATH':'/usr/bin:/bin'})
    if len(result.stdout) > 65536:
        raise ValueError()
    return result.stdout

try:
    if os.geteuid() != 0:
        raise ValueError()
    for path in ('/', '/home', '/home/voice-dev', '/home/voice-dev/issuance', archive):
        s = os.lstat(path)
        if (not stat.S_ISDIR(s.st_mode) or s.st_uid != 0 or s.st_mode & 0o022
            or os.path.realpath(path) != path):
            raise ValueError()
    if stat.S_IMODE(os.lstat(archive).st_mode) != 0o700:
        raise ValueError()
    uid = pwd.getpwnam('pmd').pw_uid
    public_fd, public_id = open_checked(public, uid, True)
    cert_fd, cert_id = open_checked(archive + '/media-ca.crt', 0, False)
    key_fd, key_id = open_checked(archive + '/media-ca.key', 0, True)
    pem = os.pread(public_fd, 65537, 0)
    if not re.fullmatch(rb'-----BEGIN CERTIFICATE-----\r?\n(?:[A-Za-z0-9+/=]+\r?\n)+-----END CERTIFICATE-----\r?\n?', pem):
        raise ValueError()
    held = lambda fd: '/proc/' + str(os.getpid()) + '/fd/' + str(fd)
    captured_der = openssl('x509', '-in', held(public_fd), '-outform', 'DER')
    archived_der = openssl('x509', '-in', held(cert_fd), '-outform', 'DER')
    if hashlib.sha256(captured_der).hexdigest() != expected:
        raise ValueError()
    cert_pub = openssl('x509', '-in', held(cert_fd), '-pubkey', '-noout')
    key_pub = openssl('pkey', '-in', held(key_fd), '-pubout')
    if any(snapshot(fd) != old for fd, old in
           ((public_fd, public_id), (cert_fd, cert_id), (key_fd, key_id))):
        raise ValueError()
    if captured_der != archived_der or cert_pub != key_pub:
        print('MEDIA_CA_CUSTODY=MISMATCH')
    else:
        print('MEDIA_CA_CUSTODY=MATCH')
except FileNotFoundError:
    print('MEDIA_CA_CUSTODY=MISSING')
except Exception:
    print('MEDIA_CA_CUSTODY=BLOCKED')
finally:
    for fd in fds:
        os.close(fd)
PY
```

Return only the `MEDIA_CA_CUSTODY=...` line. `MATCH` means the fixed
root-owned archive's key matches the captured public CA at the time of this
read-only check. Any other result requires reconciliation; it does not permit
the issuer to use a different path or replace the CA.
