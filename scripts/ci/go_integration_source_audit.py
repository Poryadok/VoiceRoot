"""Require an explicit disposition for Go *_integration_test.go entrypoints.

Unresolved exceptions are inventory gaps, not execution evidence. A subprocess
helper is classified only under its specifically mapped, required parent proof.
This audit is limited to that filename convention and does not certify other
test files.
"""
from pathlib import Path
import re


TEST = re.compile(r'(?m)^func\s+(Test\w+)\s*\(\s*\w+\s+\*testing\.T\s*\)')
DEDICATED = {
    ('gateway', 'voice/backend/gateway', 'TestComposeSpaceLeaveDiagnosticPreservesStrictStatus'):
        'backend-go-integration-pr',
    ('role', 'voice/backend/role', 'TestMemberAdmissionPositiveProtectedReceipt'):
        'role-space-receipt-integration',
    ('role', 'voice/backend/role', 'TestMemberAdmissionRole25ProtectedReceipt'):
        'role-space-receipt-integration',
    ('realtime', 'voice/backend/realtime', 'TestActualHubFanoutFreezeWaitsForEveryRecipientAndRejectsCapturedLateScope'):
        'realtime-lifecycle-integration',
    ('realtime', 'voice/backend/realtime', 'TestProtectedRealtimeActualPGReceiptsRequireMTLSExactPrincipalAndReplay'):
        'realtime-lifecycle-integration',
    ('chat', 'voice/backend/chat/internal/store', 'TestRealtimeNativeLifecycleReadsOnlyActualChatAndFailsClosedWithoutSchema'):
        'chat-lifecycle-integration',
    ('chat', 'voice/backend/chat', 'TestChatAuthActualJavaInteroperability'):
        'chat-auth-java-integration',
    ('analytics', 'voice/backend/analytics/internal/store', 'TestClickHouseInsertBatch'):
        'analytics-clickhouse-integration',
    ('role', 'voice/backend/role', 'TestBootstrapActualRole20PG'):
        'role-migration-integration',
    ('space', 'voice/backend/space/internal/nativeinstaller', 'TestMaintenanceWindowWithRealAutovacuum'):
        'space-autovacuum-integration',
    ('space', 'voice/backend/space/internal/nativeinstaller', 'TestProductionFreshChainAndRetainedVerify'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/nativeinstaller', 'TestCommittedPhaseFailureNeverAutoResumes'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanCatalogRejectsSemanticDrift'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanCannotSucceedWithoutCompleteOwnerCapture'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNative51CreatorBornCurrentSchema'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNative51ProofDiagnosticEnums'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNativeCatalog50LiteralProjectionAndDrift'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNative51RawCompositeSessionStability'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNative51ClassifierRejectsUnclassifiedMissingDatum'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanFixtureTrusted45Transition'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanFixtureTrusted50Transition'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanFixtureOver100Cohort34'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanOver100Cohort50'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNativeObserver50DefinitionAndPreimage'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNativeCatalogConcurrentRowDrift'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNativeObserverNoopTupleAndFrame'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNativeObserverCutPredicateAlgebra'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNativeObserverPrivateClosureAndLock'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNativeObserverOtherProfileCommunityGap'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNativeObserverNoopLeaseGapTuple'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanFixtureConcurrencyEnrollment50'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanFixtureRosterEnrollment50'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanIntentConcurrency50'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanLockWaitExhaustsOriginalBudget50'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanLateCaptureCannotCommitOrSplice50'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanGIS50'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanGISExpiry50'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanNative50'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanLedgerGuards50'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestAccountBanFixtureClockExpiry'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNative50CreatorBornCurrentSchema'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNative51AtomicMigrationFailureRollsBackBodySealAndHistory'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNative51FrameworkUnlockRetainsTransactionFences'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNative51StaleExpectedParentCannotCommit'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNative51FixtureCommitTrusted50'):
        'space-native-installer-integration',
    ('space', 'voice/backend/space/internal/store', 'TestNative51FreezeRequiresBoundProofAndPreservesMembership'):
        'space-native-installer-integration',
}

# This entrypoint is deliberately invoked in child processes by the required
# parent: mode 204 succeeds and mode 503 fails as part of the parent's oracle.
SUBPROCESS_HELPERS = {
    ('gateway', 'voice/backend/gateway', 'TestComposeSpaceLeaveDiagnosticProbe'):
        ('gateway', 'voice/backend/gateway', 'TestComposeSpaceLeaveDiagnosticPreservesStrictStatus'),
}


def audit(root, inventory, exceptions):
    root = Path(root)
    if not isinstance(inventory, dict) or not isinstance(exceptions, list):
        raise ValueError('invalid source inventory')
    included = set()
    for service, cases in inventory.items():
        if not isinstance(service, str) or not re.fullmatch(r'[a-z][a-z0-9]*', service):
            raise ValueError('invalid service')
        if not isinstance(cases, list) or not cases:
            raise ValueError('empty service inventory')
        for case in cases:
            if not isinstance(case, list) or len(case) != 2 or not all(isinstance(x, str) for x in case):
                raise ValueError('invalid inventory case')
            package, name = case
            prefix = 'voice/backend/' + service
            if not (package == prefix or package.startswith(prefix + '/')) or not re.fullmatch(r'Test\w+(?:/[\w.-]+)*', name):
                raise ValueError('inventory case outside service')
            key = (service, package, name)
            if key in included:
                raise ValueError('duplicate inventory case')
            included.add(key)
    omitted = {}
    dispositions = {}
    for item in exceptions:
        if not isinstance(item, dict) or set(item) != {'service', 'package', 'test', 'source', 'status', 'reason'}:
            raise ValueError('invalid source exception')
        if (item['status'] not in ('dedicated', 'subprocess', 'unresolved') or
                not isinstance(item['reason'], str) or not item['reason'].strip() or
                not all(isinstance(item[k], str) for k in ('service', 'package', 'test', 'source'))):
            raise ValueError('invalid source exception disposition')
        key = (item['service'], item['package'], item['test'])
        if key in omitted or key in included or item['service'] not in inventory:
            raise ValueError('duplicate or conflicting source exception')
        if item['status'] == 'dedicated' and DEDICATED.get(key) != item['reason']:
            raise ValueError('dedicated source exception lacks a reviewed proof mapping')
        if item['status'] == 'subprocess':
            parent = SUBPROCESS_HELPERS.get(key)
            if parent is None or item['reason'] != parent[2]:
                raise ValueError('subprocess source exception lacks a reviewed parent mapping')
        omitted[key] = item['source']
        dispositions[key] = item
    for key, item in dispositions.items():
        if item['status'] == 'subprocess':
            parent = SUBPROCESS_HELPERS[key]
            proof = dispositions.get(parent)
            if (not proof or proof['status'] != 'dedicated' or
                    proof['reason'] != DEDICATED.get(parent) or proof['source'] != item['source']):
                raise ValueError('subprocess parent lacks its dedicated proof')
    seen = {}
    for service in inventory:
        service_dir = root / 'src/backend' / service
        if not service_dir.is_dir():
            raise ValueError('service source missing: ' + service)
        for path in service_dir.rglob('*_integration_test.go'):
            package = 'voice/backend/' + service
            relative_dir = path.parent.relative_to(service_dir).as_posix()
            if relative_dir != '.':
                package += '/' + relative_dir
            for name in TEST.findall(path.read_text(encoding='utf-8')):
                key = (service, package, name)
                if key in seen:
                    raise ValueError('duplicate source test: ' + package + '/' + name)
                seen[key] = path.relative_to(root).as_posix()
    for key, source in seen.items():
        if key not in included and omitted.get(key) != source:
            raise ValueError('unclassified integration source: ' + source + '/' + key[2])
    for key, source in omitted.items():
        if seen.get(key) != source:
            raise ValueError('stale integration source exception: ' + source + '/' + key[2])
    return {'source_cases': len(seen), 'unresolved': sum(item['status'] == 'unresolved' for item in exceptions)}
