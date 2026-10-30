"""Hold a host-shared heavy slot through exact owned-fixture cleanup.

This applies only to resources created through OwnedRun. It does not yet
supervise existing Testcontainers or Compose processes.
"""
from pathlib import Path
from local_docker_owned import OwnedRun, OwnershipError
from local_heavy_slots import acquire, host_slot_directory, recover_bound


def _reconcile_execution(run):
    uncertain = [item for item in run.state['resources']
                 if item['state'] in ('creating', 'starting', 'executing', 'stopping', 'removing')]
    if len(uncertain) > 1 or (uncertain and uncertain[0]['state'] not in ('creating', 'executing', 'stopping')):
        raise OwnershipError('uncertain resource requires separate reconciliation')
    if uncertain and uncertain[0]['state'] == 'creating':
        run.reconcile_creation(uncertain[0]['name'])
    elif uncertain:
        run.cancel_execution(uncertain[0]['name'])


class ScheduledRun:
    @classmethod
    def start_host(cls, checkout, engine, timeout, capacity=2):
        """Start using the OS-wide slot directory, with no per-worktree fallback."""
        return cls.start(checkout, host_slot_directory(), engine, timeout, capacity=capacity)

    @classmethod
    def recover_host(cls, directory, engine, slot, capacity=2):
        """Recover an abandoned run bound to the OS-wide slot directory."""
        return cls.recover(directory, host_slot_directory(), engine, slot, capacity=capacity)

    @classmethod
    def start(cls, checkout, slot_directory, engine, timeout, capacity=2):
        lease = acquire(slot_directory, timeout, capacity=capacity)
        run = None
        try:
            run = OwnedRun.create(checkout, engine)
            lease.bind_owner(run.run_id)
        except BaseException:
            # A partially initialized journal must not silently free capacity.
            lease.abandon()
            if run is not None:
                run.close()
            raise
        return cls(run, lease)

    @classmethod
    def recover(cls, directory, slot_directory, engine, slot, capacity=2):
        """Clean only a journal-bound abandoned owned run, then release its slot."""
        directory = Path(directory)
        lease = recover_bound(slot_directory, slot, directory.name, capacity=capacity)
        run = None
        try:
            run = OwnedRun.load(directory, engine)
            _reconcile_execution(run)
            run.cleanup()
            run._check_daemon()
            if any(engine.exists(item['kind'], item['id']) for item in run.state['resources']):
                raise OwnershipError('owned resource remains after recovery')
            run._check_daemon()
            lease.close()
        except BaseException:
            lease.abandon()
            raise
        finally:
            if run is not None:
                run.close()

    def __init__(self, run, lease):
        self.run = run
        self._lease = lease
        self.slot = lease.slot

    def close(self):
        if self._lease is None:
            return
        lease, self._lease = self._lease, None
        try:
            _reconcile_execution(self.run)
            self.run.cleanup()
        except BaseException:
            lease.abandon()
            raise
        else:
            lease.close()
        finally:
            self.run.close()

    def abandon(self):
        if self._lease is not None:
            lease, self._lease = self._lease, None
            lease.abandon()
            self.run.close()

    def __enter__(self):
        return self.run

    def __exit__(self, *_):
        self.close()
