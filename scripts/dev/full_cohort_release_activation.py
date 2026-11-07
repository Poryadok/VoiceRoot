"""Source-owned post-backup Go migration consumer.

This slice does not start the remaining release activation phases.  Its caller
must establish their concrete prerequisites before permitting post-backup work.
"""

import full_cohort_backup_metadata as metadata
import full_cohort_backup_phase as backup_phase
import full_cohort_go_migration as go
import full_cohort_supervisor as supervisor_module


class ActivationError(RuntimeError):
    pass


def _run_go(backup, token):
    """Run the qualified C1 Go migrations in the protected restore order."""
    if type(backup) is not backup_phase.BackupPhase:
        raise ActivationError('actual backup phase required')
    supervisor, cut, captured = backup.supervisor, backup.cut, backup.metadata
    if (type(supervisor) is not supervisor_module.Supervisor or
            not isinstance(token, str) or not token or
            not supervisor.context.get('operation_id', '').startswith('media-c1-') or
            not callable(getattr(supervisor_module.Supervisor, '_open_go', None))):
        raise ActivationError('C1 Go consumer authority unavailable')
    backup_phase.verify_completed(backup, token)
    observed = metadata.verify_completed(captured, token)
    databases, plans = observed['databases'], observed['metadata']['migrations']
    if (not isinstance(databases, list) or databases != sorted(set(databases)) or
            not isinstance(plans, dict) or set(plans) != set(databases) or
            cut.migration_history):
        raise ActivationError('Go migration order unavailable')
    names = [db for db in databases if db in go.source_plan.DATABASES and
             plans[db]['from'] < plans[db]['to']]
    for database in names:
        go.qualified_plan(captured, backup.cohort, cut, token, database)
        journal, fence, client = supervisor_module.Supervisor._open_go(supervisor, database)
        secret = go.SecretSnapshot(journal, fence, client, supervisor.lock,
                                   supervisor.namespace_uid, captured, backup.cohort,
                                   cut, token, database)
        secret.create()
        migration = go.Migration(secret)
        migration.run()
        migration.cleanup()
        go.verify_completed(migration)
    return tuple(names)
