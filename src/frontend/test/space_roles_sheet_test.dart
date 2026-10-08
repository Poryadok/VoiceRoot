import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:voice_frontend/backend/api_errors.dart';
import 'package:voice_frontend/backend/roles_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/space/space_role_editor_sheet.dart';
import 'package:voice_frontend/ui/space/space_roles_sheet.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _actionDiagnostic = 'private role service diagnostic';

class _FailingRoleActions extends SpaceRoleActions {
  _FailingRoleActions(super.ref);

  @override
  Future<String?> createRole({
    required String spaceId,
    required String name,
    int permissionsMask = 0,
    int position = 1,
  }) async => _actionDiagnostic;

  @override
  Future<String?> deleteRole({
    required String spaceId,
    required String roleId,
  }) async => _actionDiagnostic;

  @override
  Future<String?> setDefaultJoinRole({
    required String spaceId,
    required String roleId,
  }) async => _actionDiagnostic;
}

void main() {
  Future<void> pumpSheet(
    WidgetTester tester, {
    required Future<List<SpaceRole>> Function(Ref ref) roles,
    Future<SpaceRole?> Function(Ref ref)? defaultJoinRole,
  }) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceRolesProvider('space-1').overrideWith(roles),
          defaultJoinRoleProvider(
            'space-1',
          ).overrideWith(defaultJoinRole ?? (ref) async => null),
          spacePermissionProvider.overrideWith((ref, query) async => false),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SpaceRolesSheet(spaceId: 'space-1')),
        ),
      ),
    );
  }

  testWidgets('SpaceRolesSheet uses the canonical skeleton while roles load', (
    tester,
  ) async {
    final rolesCompleter = Completer<List<SpaceRole>>();
    await pumpSheet(tester, roles: (_) => rolesCompleter.future);
    await tester.pump();

    expect(find.byType(VoiceListSkeleton), findsOneWidget);
    expect(find.byType(CircularProgressIndicator), findsNothing);
  });

  testWidgets('SpaceRolesSheet localizes backend unavailable errors', (
    tester,
  ) async {
    await pumpSheet(
      tester,
      roles: (_) async => throw const BackendUnavailableException(),
    );
    await tester.pumpAndSettle();

    expect(
      find.text(
        'Social and chat features are unavailable. '
        'Start the full API stack (docker compose --profile app).',
      ),
      findsOneWidget,
    );
    expect(find.text('Exception: backend unavailable'), findsNothing);
  });

  testWidgets('SpaceRolesSheet hides raw generic role errors', (tester) async {
    await pumpSheet(
      tester,
      roles: (_) async => throw Exception('sensitive backend payload'),
    );
    await tester.pumpAndSettle();

    expect(find.text('Could not load roles'), findsNWidgets(2));
    expect(find.text('Exception: sensitive backend payload'), findsNothing);
  });

  testWidgets('default join role failure is safe and retryable', (
    tester,
  ) async {
    var attempts = 0;
    await pumpSheet(
      tester,
      roles: (_) async => const [
        SpaceRole(id: 'r1', spaceId: 'space-1', name: 'Raid Leader'),
      ],
      defaultJoinRole: (_) async {
        attempts++;
        if (attempts == 1) {
          throw Exception('private default role diagnostic');
        }
        return const SpaceRole(id: 'r2', spaceId: 'space-1', name: 'Member');
      },
    );
    await tester.pumpAndSettle();

    expect(attempts, 1);
    expect(find.text('Could not load roles'), findsOneWidget);
    expect(find.text('private default role diagnostic'), findsNothing);
    expect(find.text('Raid Leader'), findsOneWidget);
    expect(find.byKey(const Key('retry_default_join_role')), findsOneWidget);

    await tester.tap(find.byKey(const Key('retry_default_join_role')));
    await tester.pumpAndSettle();

    expect(attempts, 2);
    expect(find.textContaining('Member'), findsOneWidget);
    expect(find.text('Raid Leader'), findsOneWidget);
  });

  testWidgets('SpaceRolesSheet lists roles and create button when allowed', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceRolesProvider('space-1').overrideWith(
            (ref) async => [
              const SpaceRole(
                id: 'r1',
                spaceId: 'space-1',
                name: 'Owner',
                position: 4,
                managed: true,
              ),
              const SpaceRole(
                id: 'r2',
                spaceId: 'space-1',
                name: 'Raid Leader',
                position: 2,
              ),
            ],
          ),
          defaultJoinRoleProvider('space-1').overrideWith(
            (ref) async =>
                const SpaceRole(id: 'r3', spaceId: 'space-1', name: 'Member'),
          ),
          spacePermissionProvider.overrideWith((ref, query) async => true),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SpaceRolesSheet(spaceId: 'space-1')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SpaceRolesSheet.sheetKey), findsOneWidget);
    expect(find.text('Raid Leader'), findsOneWidget);
    expect(find.byKey(const Key('create_space_role')), findsOneWidget);
  });

  Future<void> pumpActionSheet(
    WidgetTester tester, {
    bool editor = false,
  }) async {
    bindLargeTestViewport(tester);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceRolesProvider('space-1').overrideWith(
            (ref) async => const [
              SpaceRole(id: 'r1', spaceId: 'space-1', name: 'Raid Leader'),
            ],
          ),
          defaultJoinRoleProvider('space-1').overrideWith((ref) async => null),
          spacePermissionProvider.overrideWith((ref, query) async => true),
          spaceRoleActionsProvider.overrideWith(_FailingRoleActions.new),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: editor
                ? const SpaceRoleEditorSheet(spaceId: 'space-1')
                : const SpaceRolesSheet(spaceId: 'space-1'),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('role creation hides raw action failure', (tester) async {
    await pumpActionSheet(tester, editor: true);
    await tester.enterText(find.byType(TextField).first, 'New role');
    await tester.tap(find.text('Save'));
    await tester.pumpAndSettle();

    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text(_actionDiagnostic), findsNothing);
  });

  testWidgets('role deletion and default action hide raw failures', (
    tester,
  ) async {
    await pumpActionSheet(tester);
    final l10n = await AppLocalizations.delegate.load(const Locale('en'));
    final roleTile = find.byKey(const Key('space_role_r1'));

    await tester.tap(
      find.descendant(
        of: roleTile,
        matching: find.byType(PopupMenuButton<String>),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10n.spaceSetDefaultJoinRole));
    await tester.pumpAndSettle();
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text(_actionDiagnostic), findsNothing);

    await tester.tap(
      find.descendant(
        of: roleTile,
        matching: find.byType(PopupMenuButton<String>),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10n.commonDelete));
    await tester.pumpAndSettle();
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text(_actionDiagnostic), findsNothing);
  });
}
