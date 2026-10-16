"""Root-only conditional Space16 transaction producer and migration gates."""
import copy
from pathlib import Path
import re
import shutil
from commands import capture as command
from encrypted_cut import directory
import space_authority
import space_migration
from space_backup import Postgres,Snapshot
from space_restore import restore
from controller import Blocked

def target(plan):
    rows=[row for row in plan if row['database']=='space']
    if not rows:return None
    if len(rows)!=1:raise Blocked('space_target_ambiguous')
    return max(int(name.split('_',1)[0]) for name in rows[0]['files'] if name.endswith('.up.sql'))

def capacity(kube,base):
    claim=kube.get('persistentvolumeclaim','pgdata-voice-postgres-0')
    match=re.fullmatch(r'([1-9][0-9]*)(Ki|Mi|Gi|Ti)?',claim['status']['capacity']['storage'])
    if not match:raise Blocked('space_backup_capacity_invalid')
    value=int(match[1])*{None:1,'Ki':1<<10,'Mi':1<<20,'Gi':1<<30,'Ti':1<<40}[match[2]]
    if value>64<<30 or shutil.disk_usage(base).free<2*value+(1<<30):
        raise Blocked('space_backup_capacity_insufficient')
    return value

def combined_capacity(base,native,binding):
    """Reserve native+DB+CMS/readback and actual isolated restore capacity."""
    if binding is None or not binding['requirement']['backup_required']:return None
    db=binding['max_dump_bytes'];native_max=native['pvc_capacity_bytes']
    overhead=native['max_files']*1024+(512<<20)
    cipher_max=native_max+db+overhead
    if cipher_max>64<<30:raise Blocked('space_combined_cipher_capacity_exceeded')
    required=native['required_free_bytes']+4*db+2*cipher_max
    if shutil.disk_usage(base).free<required:raise Blocked('space_combined_backup_disk_insufficient')
    raw=command(['/usr/bin/docker','info','--format','{{.DockerRootDir}}'],limit=4096,timeout=15).decode().strip()
    if not raw.startswith('/') or '\n' in raw or ',' in raw:raise Blocked('space_restore_disk_untrusted')
    restore_root=directory(Path(raw))
    if shutil.disk_usage(restore_root).free<2*db+(1<<30):raise Blocked('space_restore_disk_insufficient')
    return {'cipher_max_bytes':cipher_max,'required_free_bytes':required,
        'postgres_capacity_bytes':db,'native_capacity_bytes':native_max,'restore_root':str(restore_root)}

def preflight(kube,base,plan):
    version=target(plan)
    if version is None:return None
    authority=space_authority.capture(kube)
    with Snapshot(Postgres()) as snapshot:
        requirement=space_migration.requirement(snapshot.before,version)
    space_authority.revalidate(kube,authority)
    result={'authority':authority,'requirement':requirement}
    if requirement['backup_required']:result['max_dump_bytes']=capacity(kube,base)
    return result

def capture(kube,base,binding,operation,verify_fence,save):
    if binding is None:return None
    verify_fence();space_authority.revalidate(kube,binding['authority'])
    with Snapshot(Postgres()) as snapshot:
        # Counts can legitimately advance before the owned write fence; bind
        # the closed snapshot count instead of the earlier observation.
        expected=binding['requirement']['before']
        if {k:v for k,v in snapshot.before.items() if k!='allow_guests_true'}!={k:v for k,v in expected.items() if k!='allow_guests_true'}:
            raise Blocked('space_schema_changed_before_cut')
        requirement=space_migration.requirement(snapshot.before,binding['requirement']['target_version'])
        if not requirement['backup_required']:
            return {'authority':binding['authority'],'requirement':requirement}
        limit=capacity(kube,base)
        if limit!=binding['max_dump_bytes']:raise Blocked('space_backup_capacity_changed')
        backup=snapshot.dump(Path(base)/'space-before.dump',limit)
    # Closing the exporter releases relation locks before restore/migration.
    verify_fence();space_authority.revalidate(kube,binding['authority'])
    proof=restore(Path(base)/'space-before.dump',backup,binding['authority']['image'],operation)
    if proof.get('restored') is not True:raise Blocked('space_restore_unverified')
    backup=dict(backup,restored=True)
    receipt={'authority':copy.deepcopy(binding['authority']),'requirement':requirement,
        'backup':backup,'exporter_closed':True,'restore':proof}
    receipt.update(operation=operation,source_sha=binding['source_sha'],migration_plan_sha256=binding['migration_plan_sha256'])
    save(Path(base)/'space-before-manifest.json',receipt)
    verify_fence();space_authority.revalidate(kube,binding['authority'])
    return receipt

def before_migrate(kube,binding,offnode):
    if binding is None:return
    space_authority.revalidate(kube,binding['authority'])
    space_migration.require_backup(binding['requirement'],offnode)
    with Snapshot(Postgres()) as snapshot:
        if snapshot.before!=binding['requirement']['before']:
            raise Blocked('space_closed_snapshot_changed_before_migration')

def after_migrate(kube,binding):
    if binding is None:return None
    space_authority.revalidate(kube,binding['authority'])
    with Snapshot(Postgres()) as snapshot:
        after=space_migration.verify_after(binding['requirement'],snapshot.before)
    space_authority.revalidate(kube,binding['authority'])
    return after
