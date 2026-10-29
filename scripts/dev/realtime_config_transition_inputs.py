"""Recheck concrete P3 creation and migration phases under the same fence."""
import copy
import hashlib
from pathlib import Path

import realtime_migration_job
import realtime_p3_create_only as creation
import realtime_p3_create_only_packet as packet
import realtime_p3_create_only_crypto as crypto
import realtime_p3_create_only_transport as transport


class InputError(RuntimeError): pass


def observe(cut,session,migration,source_root):
    """Reconstruct real producer evidence; this does not admit a config patch."""
    try:
        if (type(session) is not creation.Session or type(migration) is not realtime_migration_job.Migration
                or Path(source_root) != migration.root or session.namespace_uid != cut.namespace_uid
                or migration.namespace != cut.namespace_uid
                or session.fence.context != cut.fence.context or migration.fence.context != cut.fence.context
                or session.lock != cut.lock or migration.lock != cut.lock): raise ValueError()
        cut._guard(); session._guard(); migration._guard()
        before, rev = cut.fence.ledger.read()
        for phase in (session,migration):
            row, version = phase.fence.ledger.read()
            if row != before or version != rev: raise ValueError()
        handoff = session.handoff
        if (not session.completed or not isinstance(handoff,dict) or handoff.get('status') != 'done'
                or before.get('realtime_p3_handoff') != handoff or session.journal.state['steps'].get('realtime-p3-handoff') != handoff): raise ValueError()
        receipt = handoff.get('receipt',{})
        if (receipt.get('operation_id') != cut.fence.context['operation_id']
                or receipt.get('candidate_source_sha') != cut.fence.context['commit']
                or receipt.get('namespace_uid') != cut.namespace_uid or receipt.get('scope') != 'p3-create-only'
                or receipt.get('objects') != session.effects or len(session.effects) != len(packet.MEMBERS)): raise ValueError()
        session._check_roots(); roots_snapshot=copy.deepcopy(session.roots)
        public_sources={name:{'uid':session.roots[name,key]['uid'],
            'resourceVersion':session.roots[name,key]['resourceVersion'],
            'der_sha256':hashlib.sha256(crypto.der(session.roots[name,key]['certificate'])).hexdigest()}
            for name,key in creation.PUBLIC}
        if receipt.get('public_sources')!=public_sources: raise ValueError()
        def read_objects():
            identities = {}
            for kind,name in sorted(packet.MEMBERS):
                entry = session.effects.get(kind.lower()+'-'+name)
                if (not isinstance(entry,dict) or entry.get('status') != 'done' or entry.get('kind') != kind
                        or entry.get('name') != name or entry.get('operation_id') != cut.fence.context['operation_id']
                        or entry.get('keys') != sorted(packet.MEMBERS[kind,name])): raise ValueError()
                session._guard(); actual = transport.get(kind,name); session._guard()
                meta = actual.get('metadata',{}) if isinstance(actual,dict) else {}
                if (actual.get('kind') != kind or actual.get('apiVersion') != 'v1' or meta.get('name') != name
                        or meta.get('namespace') != 'voice-dev' or meta.get('deletionTimestamp')
                        or any(meta.get(k) != entry.get(k) or not isinstance(meta.get(k),str) or not meta[k]
                               for k in ('uid','resourceVersion'))): raise ValueError()
                identities[name] = {'kind':kind,'uid':meta['uid'],'resourceVersion':meta['resourceVersion']}
            return identities
        identities = read_objects()
        proof = realtime_migration_job.verify_writer_successor(migration,source_root)
        if (proof['job']['state'] != 'done' or migration.invocation_status != 'done'
                or not proof['record']['terminal_sha256']): raise ValueError()
        session._check_roots()
        if session.roots!=roots_snapshot: raise ValueError()
        if read_objects() != identities: raise ValueError()
        session._guard(); migration._guard(); cut._guard()
        after, end = cut.fence.ledger.read()
        if after != before or end != rev: raise ValueError()
        return {'objects':copy.deepcopy(identities),'public_sources':copy.deepcopy(public_sources),
                'migration':copy.deepcopy(proof),
                'scope':'p3-create-and-migration-only'}
    except Exception:
        raise InputError('P3 creation or migration evidence unavailable') from None
