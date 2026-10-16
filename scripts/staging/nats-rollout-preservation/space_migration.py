"""Conditional Space000016 evidence; root producer supplies observed data only."""
import copy
import re

class SpaceError(ValueError):pass

def _fail():raise SpaceError('space_migration_evidence_rejected')

def observed(row):
    fields={'database','version','dirty','allow_guests_true','column_default','column_type'}
    if not isinstance(row,dict) or set(row)!=fields or row['database']!='space_db':_fail()
    if type(row['version']) is not int or not 0<=row['version']<2**31 or row['dirty'] is not False:_fail()
    if type(row['allow_guests_true']) is not int or not 0<=row['allow_guests_true']<2**63:_fail()
    if row['column_type']!='boolean' or row['column_default'] not in ('true','false'):_fail()
    return copy.deepcopy(row)

def requirement(row,target_version):
    before=observed(row)
    if type(target_version) is not int or not before['version']<=target_version<2**31:_fail()
    crossing=before['version']<16<=target_version
    # Later explicit Space opt-ins are valid. Do not backfill them again merely
    # because a later release observes an already hardened schema.
    if before['version']>=16 and before['column_default']!='false':_fail()
    return {'schema':'voice-space-migration-v1','before':before,
        'target_version':target_version,'backup_required':crossing}

def require_backup(state,backup):
    if state!=requirement(state['before'],state['target_version']):_fail()
    if not state['backup_required']:return None
    fields={'schema','before','snapshot','dump_sha256','dump_bytes','restored','offnode_verified'}
    if not isinstance(backup,dict) or set(backup)!=fields or backup['schema']!='voice-space-backup-v1':_fail()
    if backup['before']!=state['before'] or backup['restored'] is not True or backup['offnode_verified'] is not True:_fail()
    if not isinstance(backup['snapshot'],str) or not re.fullmatch(r'[0-9A-F]{8}-[0-9A-F]{8}-[0-9]{1,10}',backup['snapshot']):_fail()
    if not isinstance(backup['dump_sha256'],str) or not re.fullmatch(r'[a-f0-9]{64}',backup['dump_sha256']):_fail()
    if type(backup['dump_bytes']) is not int or not 0<backup['dump_bytes']<=64<<30:_fail()
    return copy.deepcopy(backup)

def verify_after(state,row):
    if state!=requirement(state['before'],state['target_version']):_fail()
    after=observed(row)
    if after['version']!=state['target_version']:_fail()
    if state['backup_required'] and (after['allow_guests_true']!=0 or after['column_default']!='false'):_fail()
    if after['version']>=16 and after['column_default']!='false':_fail()
    return after
