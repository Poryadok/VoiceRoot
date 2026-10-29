"""Parse original native installer creation output, without conferring authority."""
import json
import re
import uuid


class EventError(RuntimeError):
    pass


def _unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result: raise ValueError()
        result[key] = value
    return result


def created(raw, expected, *, old_oid):
    """Extract creation identity; caller must bind expected to qualified source and Job.

    Output must be complete JSON lines from the uniquely owned Pod. This parser
    does not authenticate the Pod, establish fence authority, or verify final 51.
    """
    try:
        source_keys = ('source_revision', 'source_sha256', 'executable_sha256',
                       'migrator_sha256', 'inventory_sha256')
        if (not isinstance(raw, bytes) or not raw.endswith(b'\n') or len(raw) > 1024 * 1024 or
                type(old_oid) is not int or not 0 < old_oid < 2**32 or
                type(expected) is not dict or set(expected) != set(source_keys) | {'database_system_id', 'installer_role'} or
                not re.fullmatch(r'[0-9a-f]{40}', expected['source_revision']) or
                any(not re.fullmatch(r'[0-9a-f]{64}', expected[k]) for k in source_keys[1:]) or
                not isinstance(expected['database_system_id'], str) or not expected['database_system_id'].isdecimal() or
                not isinstance(expected['installer_role'], str) or not expected['installer_role']):
            raise ValueError()
        records = [json.loads(line, object_pairs_hook=_unique) for line in raw.splitlines()]
        if not records or any(type(r) is not dict or set(r) != {'event', 'receipt'} for r in records): raise ValueError()
        matching = [r['receipt'] for r in records if r['event'] == 'created']
        if len(matching) != 1 or records[0]['event'] != 'created': raise ValueError()
        receipt = matching[0]
        keys = set(source_keys) | {'run_id', 'created', 'creation', 'trusted44', 'transition45',
                                   'transition50', 'trusted50', 'transition51', 'final'}
        if type(receipt) is not dict or set(receipt) != keys or receipt['created'] is not True or any(receipt[k] != expected[k] for k in source_keys): raise ValueError()
        run_id = receipt['run_id']
        if not isinstance(run_id, str) or str(uuid.UUID(run_id)) != run_id: raise ValueError()
        facts = receipt['creation']
        if (type(facts) is not dict or set(facts) != {'version','database_system_id','database_name','database_oid','installer_role','epoch','seal_sha256'} or
                facts['database_name'] != 'space_db' or facts['database_system_id'] != expected['database_system_id'] or
                facts['installer_role'] != expected['installer_role'] or type(facts['database_oid']) is not int or
                not 0 < facts['database_oid'] < 2**32 or facts['database_oid'] == old_oid or
                type(facts['version']) is not int or facts['version'] != 0 or
                facts['epoch'] != '' or facts['seal_sha256'] != '' or
                any(receipt[k] is not None for k in ('trusted44','transition45','transition50','trusted50','transition51','final'))):
            raise ValueError()
        return {'run_id': run_id, 'new_oid': facts['database_oid'], 'role': facts['installer_role']}
    except (ValueError, TypeError, KeyError, UnicodeError, RecursionError):
        raise EventError('native installer creation evidence invalid') from None


def completed(raw, expected, *, old_oid):
    """Check the native linked completion record; caller authenticates Job/log custody."""
    try:
        identity = created(raw, expected, old_oid=old_oid)
        records = [json.loads(line, object_pairs_hook=_unique) for line in raw.splitlines()]
        if records[-1]['event'] != 'complete' or sum(r['event'] == 'complete' for r in records) != 1: raise ValueError()
        receipt = records[-1]['receipt']
        if (not isinstance(receipt, dict) or receipt.get('run_id') != identity['run_id'] or receipt.get('created') is not True or
                receipt.get('creation') != records[0]['receipt']['creation'] or
                any(receipt.get(k) != expected[k] for k in ('source_revision','source_sha256','executable_sha256','migrator_sha256','inventory_sha256'))):
            raise ValueError()
        creation, first, middle, final = (receipt[k] for k in ('creation','trusted44','trusted50','final'))
        transitions = [receipt[k] for k in ('transition45','transition50','transition51')]
        base = {'database_name':'space_db', 'database_oid':identity['new_oid'],
                'database_system_id':expected['database_system_id'], 'installer_role':identity['role']}
        for fact, version in ((first,44),(middle,50),(final,51)):
            if (not isinstance(fact,dict) or type(fact.get('database_oid')) is not int or any(fact.get(k)!=v for k,v in base.items()) or
                    type(fact.get('version')) is not int or fact['version']!=version or
                    fact.get('epoch')!='94db7e08-e958-44fa-94b2-988410ab0044' or
                    not re.fullmatch(r'[0-9a-f]{64}',fact.get('seal_sha256',''))): raise ValueError()
        if (not isinstance(creation,dict) or type(creation.get('database_oid')) is not int or
                type(creation.get('version')) is not int or any(creation.get(k)!=v for k,v in base.items())): raise ValueError()
        seals = [first['seal_sha256'], middle['seal_sha256'], final['seal_sha256']]
        for i, transition in enumerate(transitions):
            if (not isinstance(transition,dict) or type(transition.get('database_oid')) is not int or any(transition.get(k)!=v for k,v in base.items()) or
                    transition.get('epoch')!=first['epoch'] or transition.get('changed') is not True or
                    not re.fullmatch(r'[0-9a-f]{64}',transition.get('migration_sha256','')) or
                    not re.fullmatch(r'[0-9a-f]{64}',transition.get('old_seal_sha256','')) or
                    not re.fullmatch(r'[0-9a-f]{64}',transition.get('new_seal_sha256',''))): raise ValueError()
            if i == 0 and transition['old_seal_sha256'] != seals[0]: raise ValueError()
            if i == 1 and (transition['old_seal_sha256'] != transitions[0]['new_seal_sha256'] or transition['new_seal_sha256'] != seals[1]): raise ValueError()
            if i == 2 and (transition['old_seal_sha256'] != seals[1] or transition['new_seal_sha256'] != seals[2] or
                    transition.get('profile') != 'native_catalog_rowjson_51' or
                    transition.get('migration_engine') != 'github.com/golang-migrate/migrate/v4@v4.18.1' or
                    transition.get('migration_sha256') != '9271fe4135719a204a57e6b0c868f81038d4f320b03ada228f4c5b9fca40da47'): raise ValueError()
        return identity
    except (EventError, ValueError, TypeError, KeyError, UnicodeError, RecursionError):
        raise EventError('native installer completion evidence invalid') from None
