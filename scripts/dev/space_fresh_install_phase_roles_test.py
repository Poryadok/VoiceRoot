import copy
import unittest
from unittest import mock

import full_cohort_schema_metadata as metadata
import space_fresh_install_phase_roles as roles


class FactsTest(unittest.TestCase):
    def fixture(self):
        workloads = {name: [] for name in metadata.CONTAINERS}
        for name in set(metadata.OWNERS) | {'gateway'}:
            db = metadata.OWNERS.get(name, 'gateway_db')
            binding = {'state':'resolved', 'database':db, 'role':name, 'host':'voice-postgres', 'references':[]}
            secondary = [{'binding':{'state':'resolved','database':target,'role':name + '_secondary'},
                          'owner_consistency':'matched'} for target in metadata.SECONDARY.get(name, {}).values()]
            workloads[name] = [{'status':'observed','result':{'binding':binding,'secondary':secondary}}]
        return {'workloads':workloads}

    def test_source_known_role_sharing_is_reported(self):
        facts = self.fixture()
        binding, consumers = roles._facts(facts)
        self.assertEqual((binding['role'], consumers), ('space', ['space']))
        facts['workloads']['chat'][0]['result']['secondary'][0]['binding']['role'] = 'space'
        self.assertEqual(roles._facts(facts)[1], ['chat', 'space'])

    def test_unresolved_or_incomplete_potential_consumers_hold(self):
        base = self.fixture()
        for change in ('missing', 'hold', 'secondary', 'mismatch'):
            facts = copy.deepcopy(base)
            if change == 'missing': del facts['workloads']['chat']
            if change == 'hold': facts['workloads']['gateway'][0] = {'status':'hold'}
            if change == 'secondary': facts['workloads']['chat'][0]['result']['secondary'] = []
            if change == 'mismatch': facts['workloads']['chat'][0]['result']['secondary'][0]['owner_consistency'] = 'hold'
            with self.subTest(change=change), self.assertRaises((ValueError, KeyError)):
                roles._facts(facts)

    def test_guard_loss_and_secret_drift_fail_closed(self):
        observed = self.fixture()
        binding = observed['workloads']['space'][0]['result']['binding']
        binding['references'] = [{'kind':'secret','uid':'u','resource_version':'1'}]
        phase = roles.inventory.InventoryPhase.__new__(roles.inventory.InventoryPhase)
        receipt = {'database_observations': observed}
        key = {'uid':'u','resource_version':'1','value':'postgres://space:private@voice-postgres/space_db?sslmode=disable'}
        with mock.patch.object(roles.inventory.InventoryPhase, '_guard'), \
             mock.patch.object(roles.inventory.InventoryPhase, 'verify_current', return_value=receipt), \
             mock.patch.object(roles.metadata, 'kube_key_reader', return_value=mock.Mock(side_effect=[key, key | {'resource_version':'2'}])):
            with self.assertRaises(roles.RoleFactsError): roles.observe(phase, 'token')
        with mock.patch.object(roles.inventory.InventoryPhase, '_guard', side_effect=RuntimeError('lost')):
            with self.assertRaises(metadata.GuardLost): roles.observe(phase, 'token')


if __name__ == '__main__': unittest.main()
