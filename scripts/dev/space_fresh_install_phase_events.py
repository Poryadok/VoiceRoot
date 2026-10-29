"""Parse original native installer creation output, without conferring authority."""
import json
import re
import uuid


class EventError(RuntimeError):
    pass


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
        def no_duplicates(pairs):
            result = {}
            for key, value in pairs:
                if key in result: raise ValueError()
                result[key] = value
            return result
        records = [json.loads(line, object_pairs_hook=no_duplicates) for line in raw.splitlines()]
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
