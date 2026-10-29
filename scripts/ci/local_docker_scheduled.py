"""Hold a host-shared heavy slot through exact owned-fixture cleanup.

This applies only to resources created through OwnedRun. It does not yet
supervise existing Testcontainers or Compose processes.
"""
from local_docker_owned import OwnedRun
from local_heavy_slots import acquire


class ScheduledRun:
    @classmethod
    def start(cls, checkout, slot_directory, engine, timeout):
        lease = acquire(slot_directory, timeout)
        try:
            run = OwnedRun.create(checkout, engine)
        except BaseException:
            # A partially initialized journal must not silently free capacity.
            lease.abandon()
            raise
        return cls(run, lease)

    def __init__(self, run, lease):
        self.run = run
        self._lease = lease

    def close(self):
        if self._lease is None:
            return
        lease, self._lease = self._lease, None
        try:
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
