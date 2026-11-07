"""Hold a host-shared heavy slot through exact owned-fixture cleanup.

This applies only to resources created through OwnedRun. It does not intercept
arbitrary Testcontainers or Compose activity.
"""
from pathlib import Path

import local_ci_proofs_scheduler as proof_scheduler
import local_process_supervision as supervision
from local_docker_owned import OwnedRun, OwnershipError
from local_heavy_slots import (acquire, active_resource_reservation, host_slot_directory,
                               recover_bound, release_resource_reservation,
                               resource_reservation_slot)


def _reconcile_execution(run):
    uncertain = [item for item in run.state['resources']
                 if item['state'] in ('creating', 'starting', 'executing', 'stopping', 'removing')]
    if len(uncertain) > 1 or (uncertain and uncertain[0]['state'] not in ('creating', 'executing', 'stopping')):
        raise OwnershipError('uncertain resource requires separate reconciliation')
    if uncertain and uncertain[0]['state'] == 'creating':
        run.reconcile_creation(uncertain[0]['name'])
    elif uncertain:
        run.cancel_execution(uncertain[0]['name'])


def _require_absent_job(host_job):
    expected = host_job['session_identity']
    before = proof_scheduler._session_identity()
    if before != expected:
        raise OwnershipError('host session identity changed before Job recovery')
    names = host_job['job_names']
    if len(names) != 1:
        raise OwnershipError('host Job intent is ambiguous')
    try:
        exists = supervision.named_job_exists(names[0])
    except BaseException:
        raise OwnershipError('named host Job absence is uncertain') from None
    after = proof_scheduler._session_identity()
    if after != before:
        raise OwnershipError('host session identity changed during Job recovery')
    if exists is not False:
        raise OwnershipError('named host Job is still present')
    return before


class ScheduledRun:
    @classmethod
    def start_host(cls, checkout, engine, timeout, capacity=2, **kwargs):
        """Start using the OS-wide slot directory, with no per-worktree fallback."""
        return cls.start(checkout, host_slot_directory(), engine, timeout,
                         capacity=capacity, **kwargs)

    @classmethod
    def recover_host(cls, directory, engine, slot=None, capacity=2, *, slot_directory=None):
        """Recover only an exact named-Job owner after session and daemon checks."""
        directory = Path(directory)
        slot_directory = Path(slot_directory) if slot_directory is not None else host_slot_directory()
        run = OwnedRun.load(directory, engine)
        lease = None
        try:
            host_job = run.state.get('host_job')
            if run.state.get('version') == 1:
                # Keep the legacy exact-owner recovery contract. It has no named
                # Job evidence and therefore makes no bounded-job guarantee.
                run.close()
                run = None
                return cls.recover(directory, slot_directory, engine, slot, capacity)
            if run.state.get('version') != 2 or not isinstance(host_job, dict):
                raise OwnershipError('host owner journal version is unsupported')
            if slot is None:
                slot = host_job['slot']
                if slot is None:
                    slot = resource_reservation_slot(slot_directory, run.run_id)
            if type(slot) is not int or not 0 <= slot < capacity:
                raise OwnershipError('host owner slot is not durably identified')
            if host_job['slot'] is not None and host_job['slot'] != slot:
                raise OwnershipError('host owner slot differs from durable intent')
            _require_absent_job(host_job)
            lease = recover_bound(slot_directory, slot, run.run_id, capacity=capacity)
            # The slot lock closes the probe/cleanup race with another owner.
            # Revalidate durable intent and the external Job after acquiring it.
            if run.state.get('host_job') != host_job:
                raise OwnershipError('host owner intent changed while acquiring recovery lock')
            _require_absent_job(host_job)
            run._check_daemon()
            _reconcile_execution(run)
            run.cleanup()
            run._check_daemon()
            if any(engine.exists(item['kind'], item['id']) for item in run.state['resources'] if item['id']):
                raise OwnershipError('owned resource remains after recovery')
            run._check_daemon()
            if proof_scheduler._session_identity() != host_job['session_identity']:
                raise OwnershipError('host session identity changed during cleanup')
            if active_resource_reservation(slot_directory, run.run_id) is not None:
                release_resource_reservation(slot_directory, run.run_id, cleanup_verified=True)
            lease.close(cleanup_verified=True)
        except BaseException:
            if lease is not None:
                lease.abandon()
            raise
        finally:
            if run is not None:
                run.close()

    @classmethod
    def start(cls, checkout, slot_directory, engine, timeout, capacity=2, *,
              resource_claim=None, capacity_budget=None, host_job=None, resource_limits=None,
              run_id=None):
        """Persist owner intent before atomically admitting and binding its slot."""
        if host_job is None and (resource_limits is not None or resource_claim is not None or capacity_budget is not None):
            raise OwnershipError('bounded admission requires a durable host Job intent')
        if host_job is None and run_id is None:
            run = OwnedRun.create(checkout, engine)
        else:
            run = OwnedRun.create(checkout, engine, run_id=run_id, host_job=host_job,
                                  resource_limits=resource_limits)
        lease = None
        try:
            if resource_claim is None and capacity_budget is None:
                lease = acquire(slot_directory, timeout, capacity=capacity, owner_id=run.run_id)
            else:
                lease = acquire(slot_directory, timeout, capacity=capacity, owner_id=run.run_id,
                                resource_claim=resource_claim, capacity_budget=capacity_budget)
            if host_job is not None:
                run.state['host_job']['slot'] = lease.slot
                run._save()
            return cls(run, lease, engine, slot_directory)
        except BaseException:
            if lease is not None:
                lease.abandon()
            run.close()
            raise

    @classmethod
    def attach_worker(cls, directory, engine, slot, owner_id, resource_limits):
        """Open only the parent-prepared owner journal inside the supervised worker."""
        run = OwnedRun.load(directory, engine)
        try:
            host_job = run.state.get('host_job')
            if (run.state.get('version') != 2 or run.run_id != owner_id or
                    not isinstance(host_job, dict) or host_job['slot'] != slot or
                    run.state.get('resource_limits') != resource_limits):
                raise OwnershipError('worker owner journal differs from bounded request')
            attached = cls(run, None, engine, None)
            attached.slot = slot
            return attached
        except BaseException:
            run.close()
            raise

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
            if any(engine.exists(item['kind'], item['id']) for item in run.state['resources'] if item['id']):
                raise OwnershipError('owned resource remains after recovery')
            run._check_daemon()
            lease.close(cleanup_verified=True)
        except BaseException:
            lease.abandon()
            raise
        finally:
            if run is not None:
                run.close()

    def __init__(self, run, lease, engine, slot_directory):
        self.run = run
        self._lease = lease
        self._engine = engine
        self._slot_directory = slot_directory
        self._directory = run.directory if run is not None else None
        self.slot = lease.slot if lease is not None else None
        self._worker_attached = lease is None and run is not None and run.state.get('version') == 2

    def release_owner_handle(self):
        """Let the named worker acquire the same durable journal lock; keep slot held."""
        if self.run is not None:
            self._directory = self.run.directory
            self.run.close()
            self.run = None

    def close(self):
        if self.run is None and self._lease is None:
            return
        run = self.run
        if run is None:
            run = OwnedRun.load(self._directory, self._engine)
        self.run = None
        lease, self._lease = self._lease, None
        try:
            if self._worker_attached:
                # The child is still inside its named Job, so it cannot prove
                # Job absence or release the parent's reservation/slot. The
                # parent performs final cleanup after the Job terminates.
                return
            host_job = run.state.get('host_job') if run.state.get('version') == 2 else None
            if host_job is not None:
                _require_absent_job(host_job)
                run._check_daemon()
            _reconcile_execution(run)
            run.cleanup()
            run._check_daemon()
            if host_job is not None:
                if proof_scheduler._session_identity() != host_job['session_identity']:
                    raise OwnershipError('host session identity changed during cleanup')
                _require_absent_job(host_job)
            if any(self._engine.exists(item['kind'], item['id']) for item in run.state['resources'] if item['id']):
                raise OwnershipError('owned resource remains after cleanup')
            run._check_daemon()
        except BaseException:
            if lease is not None:
                lease.abandon()
            raise
        else:
            release_evidence = None
            if lease is not None:
                release_evidence = lease.close(cleanup_verified=True)
        finally:
            run.close()
        if lease is None:
            return None
        return {'daemon_id': run.state['daemon'], 'owned_resources_remaining': 0,
                'slot_release': release_evidence}

    def abandon(self):
        if self.run is not None:
            self._directory = self.run.directory
            self.run.close()
            self.run = None
        if self._lease is not None:
            lease, self._lease = self._lease, None
            lease.abandon()

    def __enter__(self):
        return self.run

    def __exit__(self, *_):
        self.close()
