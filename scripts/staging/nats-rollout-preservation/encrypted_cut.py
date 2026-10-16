"""Authenticated OpenSSL CMS backup; recovery keys never leave root custody."""
import hashlib
import json
import os
import re
from pathlib import Path
import stat
import subprocess
import tarfile
import tempfile


class CryptoError(RuntimeError):
    pass


def directory(path):
    path = Path(path)
    for parent in (*path.parents[::-1], path):
        row = parent.lstat()
        if not stat.S_ISDIR(row.st_mode) or row.st_uid != 0 or row.st_mode & 0o022:
            raise CryptoError('backup_directory_custody_invalid')
    return path


def regular(path, private=False):
    path = Path(path)
    directory(path.parent)
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    row = os.fstat(fd)
    if (not stat.S_ISREG(row.st_mode) or row.st_uid != 0 or row.st_nlink != 1 or
            row.st_mode & (0o077 if private else 0o022)):
        os.close(fd)
        raise CryptoError('backup_file_custody_invalid')
    return fd, row


def openssl(args):
    try:
        subprocess.run(['/usr/bin/openssl', *args], check=True, timeout=600,
            stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            env={'PATH': '/usr/bin:/bin', 'HOME': '/nonexistent', 'LC_ALL': 'C'})
    except (OSError, subprocess.SubprocessError):
        raise CryptoError('backup_openssl_failed') from None


def initialize_recovery_key(key_directory):
    key_directory = directory(key_directory)
    private = key_directory / 'recovery-key.pem'
    certificate = key_directory / 'recovery-cert.pem'
    if private.exists() or certificate.exists():
        for path in (private, certificate):
            fd, _ = regular(path, private=True)
            os.close(fd)
        openssl(['pkey', '-in', str(private), '-check', '-noout'])
        openssl(['x509', '-in', str(certificate), '-noout'])
        scratch=Path(tempfile.mkdtemp(prefix='key-check-',dir=key_directory))
        try:
            openssl(['pkey','-in',str(private),'-pubout','-out',str(scratch/'private.pub')])
            openssl(['x509','-in',str(certificate),'-pubkey','-noout','-out',str(scratch/'certificate.pub')])
            if (scratch/'private.pub').read_bytes()!=(scratch/'certificate.pub').read_bytes():raise CryptoError('backup_recovery_key_certificate_mismatch')
        finally:
            for path in scratch.iterdir():path.unlink()
            scratch.rmdir()
        return
    # Unique private workspace: a failed generation never becomes an installed
    # recovery key. Existing keys are checked and retained, never replaced.
    scratch = Path(tempfile.mkdtemp(prefix='key-', dir=key_directory))
    saved_umask = os.umask(0o077)
    try:
        openssl(['req', '-x509', '-newkey', 'rsa:3072', '-nodes', '-sha256',
            '-days', '36500', '-subj', '/CN=Voice rollout backup recovery',
            '-keyout', str(scratch / 'key.pem'), '-out', str(scratch / 'cert.pem')])
        for source, target in ((scratch / 'key.pem', private), (scratch / 'cert.pem', certificate)):
            os.link(source, target)
            target.chmod(0o600)
        for path in (private, certificate):
            with path.open('rb') as stream:
                os.fsync(stream.fileno())
        fd = os.open(key_directory, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(fd)
        finally:
            os.close(fd)
    finally:
        os.umask(saved_umask)
        for path in scratch.iterdir():
            path.unlink()
        scratch.rmdir()


def space_members(base, receipt):
    """Only the root-produced restored dump may join the fixed cut payload."""
    if receipt is None:return (), {}
    import space_migration
    backup=receipt['backup']
    wanted=dict(backup,offnode_verified=True)
    space_migration.require_backup(receipt['requirement'],wanted)
    if receipt.get('exporter_closed') is not True:
        raise CryptoError('backup_space_exporter_open')
    hashes={}
    for name in ('space-before.dump','space-before-manifest.json'):
        fd,row=regular(Path(base)/name,private=True)
        digest=hashlib.sha256();count=0;raw=bytearray()
        with os.fdopen(fd,'rb') as stream:
            while chunk:=stream.read(1<<20):
                count+=len(chunk);digest.update(chunk)
                if name.endswith('.json'):
                    if count>1<<20:raise CryptoError('backup_space_manifest_large')
                    raw.extend(chunk)
        if count!=row.st_size:raise CryptoError('backup_space_file_changed')
        hashes[name]=digest.hexdigest()
        if name.endswith('.dump'):
            if count!=backup['dump_bytes'] or hashes[name]!=backup['dump_sha256']:
                raise CryptoError('backup_space_dump_changed')
        elif json.loads(raw)!=receipt:raise CryptoError('backup_space_manifest_changed')
    return ('space-before.dump','space-before-manifest.json'),hashes


def authorize_space(base,state):
    """Derive off-node Space evidence only from the verified enclosing cipher."""
    receipt=state.get('space_backup')
    if receipt is None:return None
    target=state.get('target',{})
    if (receipt.get('operation')!=state['operation'] or receipt.get('source_sha')!=target.get('tag')
        or receipt.get('migration_plan_sha256')!=target.get('migration_sha256')):
        raise CryptoError('backup_space_source_binding_changed')
    _,hashes=space_members(base,receipt)
    binding=state.get('cipher_binding',{});custody=state.get('custody',{})
    keys=('operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes')
    if (state.get('cipher_space_members')!=hashes or custody.get('verified') is not True
        or custody.get('schema')!='voice-nats-custody-v1'
        or custody.get('destination')!='github:Poryadok/VoiceRoot'
        or any(custody.get(k)!=binding.get(k) or k not in binding for k in keys)
        or binding.get('operation')!=state['operation']):
        raise CryptoError('backup_space_offnode_unverified')
    import space_migration
    return space_migration.require_backup(receipt['requirement'],dict(receipt['backup'],offnode_verified=True))


def postseal_members(base, receipt):
    """Export only the full CLOSED copy and its ROOT-bound private observation."""
    if receipt is None:return (),{}
    from preserved_upload import digest
    try:
        if (receipt['schema']!='voice-expired-finish-current-observation-v1'
            or receipt['postseal']['schema']!='voice-root-postseal-current-proof-v1'):
            raise CryptoError('backup_postseal_shape_invalid')
        expected={'current-copy-closed.tar':receipt['native_disposition']['closed_archive_sha256'],
            'current-copy-observed.json':receipt['postseal']['copy_observation_sha256']}
        hashes={};observation=None
        for name in ('rollout-before-manifest.json',*expected):
            fd,row=regular(Path(base)/name,private=True)
            sha=hashlib.sha256();raw=bytearray();count=0
            with os.fdopen(fd,'rb') as stream:
                while chunk:=stream.read(1<<20):
                    count+=len(chunk);sha.update(chunk)
                    if name.endswith('.json'):
                        if count>128<<20:raise CryptoError('backup_postseal_json_large')
                        raw.extend(chunk)
                final=os.fstat(stream.fileno())
                fields=('st_dev','st_ino','st_mode','st_uid','st_gid','st_nlink','st_size','st_mtime_ns','st_ctime_ns')
                if (any(getattr(final,key)!=getattr(row,key) for key in fields)
                    or count!=row.st_size):raise CryptoError('backup_postseal_file_changed')
            hashes[name]=sha.hexdigest()
            if name.endswith('.json'):
                value=json.loads(raw)
                if name=='current-copy-observed.json':observation=value
                if (name=='rollout-before-manifest.json' and value!=receipt
                    or name in expected and digest(value)!=expected[name]):
                    raise CryptoError('backup_postseal_observation_changed')
            elif hashes[name]!=expected[name]:raise CryptoError('backup_postseal_archive_changed')
        ledger=observation['private_record_ledger']
        if ledger['storage']!='ROOT_IMMUTABLE_RECORD_FILES' or type(ledger['streams']) is not dict:
            raise CryptoError('backup_postseal_record_index_invalid')
        records=[];seen=set()
        for stream,entry in ledger['streams'].items():
            if not isinstance(stream,str) or not re.fullmatch('[A-Za-z0-9_-]{1,255}',stream):
                raise CryptoError('backup_postseal_record_index_invalid')
            for sequence,reference in entry['records'].items():
                if (type(reference) is not dict or set(reference)!={'file','sha256','bytes','subject','seq','time'}
                    or not re.fullmatch(r'[a-f0-9]{64}\.json',reference['file'])
                    or not re.fullmatch('[a-f0-9]{64}',reference['sha256'])
                    or type(reference['bytes']) is not int or not 0<reference['bytes']<=16<<20
                    or type(reference['seq']) is not int or reference['seq']<=0
                    or str(reference['seq'])!=sequence or reference['file'] in seen):
                    raise CryptoError('backup_postseal_record_index_invalid')
                seen.add(reference['file']);name='current-record-files/'+reference['file']
                fd,row=regular(Path(base)/name,private=True)
                with os.fdopen(fd,'rb') as file:raw=file.read((16<<20)+1)
                if len(raw)!=row.st_size or len(raw)!=reference['bytes'] or hashlib.sha256(raw).hexdigest()!=reference['sha256']:
                    raise CryptoError('backup_postseal_record_changed')
                saved=json.loads(raw)
                if (set(saved)!={'stream','record'} or saved['stream']!=stream
                    or any(saved['record'].get(key)!=reference[key] for key in ('seq','subject','time'))):
                    raise CryptoError('backup_postseal_record_changed')
                records.append(name);hashes[name]=reference['sha256']
        return (*expected,*records),hashes
    except (KeyError,TypeError,ValueError):
        raise CryptoError('backup_postseal_shape_invalid') from None


def encrypt_cut(base, key_directory, space=None, *, postseal=None):
    base, key_directory = directory(base), directory(key_directory)
    extra,space_hashes=space_members(base,space)
    current,current_hashes=postseal_members(base,postseal)
    for path in (key_directory / 'recovery-key.pem', key_directory / 'recovery-cert.pem'):
        fd, _ = regular(path, private=True)
        os.close(fd)
    target = base / 'rollout-backup.cms'
    # O_EXCL reservation binds this attempt to one immutable ciphertext.
    fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    os.close(fd)
    scratch = Path(tempfile.mkdtemp(prefix='encrypt-', dir=base))
    try:
        payload = scratch / 'payload.tar'
        with tarfile.open(payload, 'w') as bundle:
            payload.chmod(0o600)
            for name in ('rollout-before.tar', 'rollout-before-manifest.json', 'copy-checkpoint.json',*extra,*current):
                fd, row = regular(base / name)
                with os.fdopen(fd, 'rb') as stream:
                    member = tarfile.TarInfo(name)
                    member.size = row.st_size
                    member.mode = 0o600
                    bundle.addfile(member, stream)
        ciphertext = scratch / 'cipher.cms'
        openssl(['cms', '-encrypt', '-binary', '-stream', '-aes-256-gcm',
            '-outform', 'DER', '-in', str(payload), '-out', str(ciphertext),
            '-recip', str(key_directory / 'recovery-cert.pem')])
        ciphertext.chmod(0o600)
        with ciphertext.open('rb') as source, target.open('wb') as output:
            import shutil
            shutil.copyfileobj(source, output, length=1 << 20)
            output.flush()
            os.fsync(output.fileno())
        sha = hashlib.sha256()
        with target.open('rb') as stream:
            while chunk := stream.read(1 << 20):
                sha.update(chunk)
        result={'cipher_sha256': sha.hexdigest(), 'cipher_bytes': target.stat().st_size}
        if current_hashes:
            if postseal_members(base,postseal)[1]!=current_hashes:
                raise CryptoError('backup_postseal_payload_changed')
            result['postseal_members']=current_hashes
        if space_hashes:result['space_members']=space_hashes
        return result
    except BaseException:
        target.unlink()
        raise
    finally:
        for path in scratch.iterdir():
            path.unlink()
        scratch.rmdir()


def decrypt_verify(base,key_directory,binding,readback,*,expected_members=None):
    """Authenticated actual-operation restore bound to identical remote bytes.

    Caller first independently streams the complete remote ciphertext and
    verifies equality to this immutable local ciphertext. Every plaintext
    member must match its root-owned source; no archive path is extracted.
    The readback callback verifies/restores only the private recovered archive.
    """
    base,key_directory=directory(base),directory(key_directory)
    if not callable(readback):raise CryptoError('backup_restore_reader_invalid')
    required={'rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json'}
    expected_members=required if expected_members is None else set(expected_members)
    if not required<=expected_members:raise CryptoError('backup_restore_member_inventory_invalid')
    hashes={};cipher=base/'rollout-backup.cms'
    fd,row=regular(cipher,private=True);sha=hashlib.sha256();count=0
    with os.fdopen(fd,'rb') as stream:
        while chunk:=stream.read(1<<20):count+=len(chunk);sha.update(chunk)
    if count!=binding.get('cipher_bytes') or sha.hexdigest()!=binding.get('cipher_sha256'):
        raise CryptoError('backup_restore_cipher_changed')
    for name in ('recovery-key.pem','recovery-cert.pem'):
        fd,_=regular(key_directory/name,private=True);os.close(fd)
    scratch=Path(tempfile.mkdtemp(prefix='decrypt-',dir=base));scratch.chmod(0o700)
    try:
        payload=scratch/'authenticated-payload.tar'
        # Successful GCM authentication precedes ANY trust of decrypted bytes.
        openssl(['cms','-decrypt','-binary','-inform','DER','-in',str(cipher),
            '-recip',str(key_directory/'recovery-cert.pem'),
            '-inkey',str(key_directory/'recovery-key.pem'),'-out',str(payload)])
        payload.chmod(0o600);seen=set();recovered=scratch/'rollout-before.tar'
        with tarfile.open(payload,'r|') as archive:
            for member in archive:
                name=member.name
                if (name in seen or len(seen)>=262144 or not member.isreg()
                    or name.startswith('/') or any(p in ('','.','..') for p in name.split('/'))
                    or not (name in ('rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json',
                        'space-before.dump','space-before-manifest.json','current-copy-closed.tar','current-copy-observed.json')
                        or re.fullmatch(r'current-record-files/[a-f0-9]{64}\.json',name))):
                    raise CryptoError('backup_restore_member_invalid')
                seen.add(name);fd,expected=regular(base/name,private=True)
                if member.size!=expected.st_size:os.close(fd);raise CryptoError('backup_restore_member_changed')
                expected_sha=hashlib.sha256();actual_sha=hashlib.sha256();n=0
                output=recovered.open('xb') if name=='rollout-before.tar' else None
                if output is not None:recovered.chmod(0o600)
                try:
                    with os.fdopen(fd,'rb') as original,archive.extractfile(member) as restored:
                        while chunk:=restored.read(1<<20):
                            n+=len(chunk);actual_sha.update(chunk)
                            if output is not None:output.write(chunk)
                        while chunk:=original.read(1<<20):expected_sha.update(chunk)
                        final=os.fstat(original.fileno())
                    if (n!=expected.st_size or actual_sha.digest()!=expected_sha.digest()
                        or (expected.st_dev,expected.st_ino,expected.st_size,expected.st_mtime_ns,expected.st_ctime_ns)
                        !=(final.st_dev,final.st_ino,final.st_size,final.st_mtime_ns,final.st_ctime_ns)):
                        raise CryptoError('backup_restore_member_changed')
                    hashes[name]=actual_sha.hexdigest()
                finally:
                    if output is not None:output.close()
        if seen!=expected_members:
            raise CryptoError('backup_restore_member_missing')
        proof=readback(recovered)
        # Cipher inode/content is not permitted to change during decryption.
        fd,current=regular(cipher,private=True);sha=hashlib.sha256()
        with os.fdopen(fd,'rb') as source:
            while chunk:=source.read(1<<20):sha.update(chunk)
        if (sha.hexdigest()!=binding['cipher_sha256'] or current.st_size!=binding['cipher_bytes']
            or (row.st_dev,row.st_ino,row.st_mtime_ns,row.st_ctime_ns)
                !=(current.st_dev,current.st_ino,current.st_mtime_ns,current.st_ctime_ns)):
            raise CryptoError('backup_restore_cipher_changed')
        return {'schema':'voice-nats-decrypted-restore-v1','cipher_sha256':binding['cipher_sha256'],
            'cipher_bytes':binding['cipher_bytes'],'decrypted_members_verified':True,
            'members':hashes,'readback':proof,'remote_equivalence':'FULL_CIPHERTEXT_SHA256_AND_SIZE'}
    finally:
        import shutil
        shutil.rmtree(scratch)
