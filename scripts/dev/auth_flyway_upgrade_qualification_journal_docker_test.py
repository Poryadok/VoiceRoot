"""Disposable real Docker lifecycle through the actual shared journal/fence.

This checks resource custody only; it does not execute Auth or qualify a backup.
Requires the pinned local Linux images and the selected test Docker daemon.
"""
import unittest

from auth_flyway_upgrade_qualification_docker import OwnedDocker
import auth_flyway_upgrade_qualification_journal_test as fixtures
import auth_flyway_upgrade_qualification_journal as adapter
from auth_user_adoption_runtime import QUALIFIED_TOOLS


class DockerJournalIntegration(unittest.TestCase):
    def test_partial_verified_resource_cleanup_retains_uncertain(self):
        fixture=fixtures.CloneJournalTest('test_full_fenced_resource_lifecycle_redacts_private_receipt')
        fixture.setUp(); self.addCleanup(fixture.doCleanups)
        journal=fixture.create(); docker=OwnedDocker(journal)
        docker.create(QUALIFIED_TOOLS['postgres'], ['--network','none'], ['--version'])
        docker.cleanup()
        self.assertEqual(docker.records[0]['state'],'removed')
        with self.assertRaises(adapter.CloneJournalError): journal.uncertain()
        self.assertEqual(fixture.base.base.ledger.row['status'],'uncertain')

    def test_actual_owned_containers_and_full_cleanup(self):
        fixture = fixtures.CloneJournalTest('test_full_fenced_resource_lifecycle_redacts_private_receipt')
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        journal = fixture.create()
        docker = OwnedDocker(journal)
        docker.create(QUALIFIED_TOOLS['postgres'], ['--network','none'], ['--version'])
        docker.create(QUALIFIED_TOOLS['flyway'], ['--network','none'], ['-v'])
        journal.stage('flyway-migrate','pending')
        docker.create(QUALIFIED_TOOLS['flyway'], ['--network','none'], ['-v'])
        journal.stage('flyway-migrate','done')
        docker.create(QUALIFIED_TOOLS['flyway'], ['--network','none'], ['-v'])
        docker.cleanup()
        resources = docker.records
        self.assertTrue(all(r['state']=='removed' for r in resources))
        journal.state['flyway_clone']['status']='complete'
        journal.state['flyway_clone']['receipt']=fixture.receipt(resources)
        journal.save(); journal.guard()
        self.assertEqual(fixture.base.base.ledger.row['cohort_auth_flyway']['status'],'done')


if __name__ == '__main__': unittest.main()
