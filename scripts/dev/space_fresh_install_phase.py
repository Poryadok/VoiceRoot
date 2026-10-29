"""DEV Space replacement catalog checks. Lifecycle implementation is pending."""
import copy


class SpaceFreshError(RuntimeError):
    pass


def _check_catalog(original, actual, state, old_oid, new_oid):
    """Check the exact catalog transition without attributing its authority."""
    keys = ('all_databases', 'databases', 'excluded', 'properties')
    if (type(old_oid) is not int or not 0 < old_oid < 2**32 or
            any(not isinstance(original.get(k), dict) or not isinstance(actual.get(k), dict) for k in keys) or
            original['all_databases'].get('space_db') != old_oid or
            original['databases'].get('space_db') != old_oid or 'space_db' in original['excluded']):
        raise ValueError()
    if state == 'drop_pending':
        if new_oid is not None or any(original[k] != actual[k] for k in keys): raise ValueError()
        return
    if state in ('drop_done', 'job_pending', 'job_owned'):
        if new_oid is not None or any('space_db' in actual[k] for k in keys): raise ValueError()
    elif state in ('created', 'done'):
        if (type(new_oid) is not int or not 0 < new_oid < 2**32 or new_oid == old_oid or
                new_oid in (v for k, v in original['all_databases'].items() if k != 'space_db') or
                actual['all_databases'].get('space_db') != new_oid or actual['databases'].get('space_db') != new_oid or
                'space_db' in actual['excluded'] or 'space_db' not in actual['properties']): raise ValueError()
    else:
        raise ValueError()
    for key in keys:
        before, after = copy.deepcopy(original[key]), copy.deepcopy(actual[key])
        before.pop('space_db', None); after.pop('space_db', None)
        if before != after: raise ValueError()
