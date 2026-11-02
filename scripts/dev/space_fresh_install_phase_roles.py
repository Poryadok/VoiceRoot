"""Read source-known Space role facts without exposing credentials or granting authority."""
import copy

import full_cohort_inventory_phase as inventory
import full_cohort_schema_metadata as metadata


class RoleFactsError(RuntimeError):
    pass


def _facts(observed):
    workloads = observed['workloads']
    if set(workloads) != set(metadata.CONTAINERS): raise ValueError()
    routes = []
    for name in sorted(set(metadata.OWNERS) | {'gateway'}):
        entries = workloads[name]
        if not isinstance(entries, list) or not entries: raise ValueError()
        for entry in entries:
            if entry.get('status') != 'observed': raise ValueError()
            result = entry['result']; binding = result['binding']
            if binding.get('state') != 'resolved': raise ValueError()
            routes.append((name, binding))
            secondary = result.get('secondary', [])
            if len(secondary) != len(metadata.SECONDARY.get(name, {})): raise ValueError()
            for item in secondary:
                if item.get('owner_consistency') != 'matched': raise ValueError()
                routes.append((name, item['binding']))
    space = [binding for name, binding in routes if name == 'space' and binding.get('database') == 'space_db']
    if not space or any(any(binding.get(k) != space[0].get(k) for k in ('role', 'host', 'database', 'references')) for binding in space): raise ValueError()
    role = space[0].get('role')
    if not isinstance(role, str) or not metadata.SAFE.fullmatch(role): raise ValueError()
    consumers = sorted({name for name, binding in routes if binding.get('role') == role})
    return space[0], consumers


def observe(phase, token):
    """Reobserve the protected pre-cut inventory and fixed Space Secret key.

    The result identifies source-known workload consumers only; it is not a
    complete proof of external clients, future state, or authority to change a credential.
    """
    try:
        if type(phase) is not inventory.InventoryPhase: raise ValueError()
        metadata._fence(phase._guard)
        try: receipt = phase.verify_current(token)
        except BaseException: raise metadata.GuardLost('inventory reobservation unavailable') from None
        binding, consumers = _facts(receipt['database_observations'])
        reader = metadata.kube_key_reader(metadata.linux_private_bytes)
        def current():
            metadata._fence(phase._guard)
            value = reader('secret', 'voice-app-secrets', 'SPACE_DATABASE_URL')
            metadata._fence(phase._guard)
            if not value or not isinstance(value['value'], str): raise ValueError()
            host, database, role = metadata._url(value['value'], 'voice-dev')
            reference = {'kind': 'secret', 'uid': value['uid'], 'resource_version': value['resource_version']}
            if (host != binding['host'] or database != 'space_db' or role != binding['role'] or
                    reference not in binding['references']): raise ValueError()
            return reference
        reference = current()
        if current() != reference: raise ValueError()
        try: final = phase.verify_current(token)
        except BaseException: raise metadata.GuardLost('inventory reobservation unavailable') from None
        if final != receipt: raise ValueError()
        metadata._fence(phase._guard)
        return {'role': binding['role'], 'database': 'space_db', 'consumers': consumers,
                'reference': copy.deepcopy(reference), 'scope': 'source-known-workload-routes-only'}
    except metadata.GuardLost:
        raise
    except BaseException:
        raise RoleFactsError('Space role facts unavailable') from None
