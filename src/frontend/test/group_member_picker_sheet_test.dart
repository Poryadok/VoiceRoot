import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/create_group_friends_provider.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/ui/chat/group_member_picker_sheet.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'filters complete friends, excludes current members and returns only on submit',
    (tester) async {
      addTearDown(() => tester.binding.setSurfaceSize(null));
      await tester.binding.setSurfaceSize(const Size(400, 800));
      Future<List<String>> friends(Ref ref) async => const [
        'existing-member',
        'profile-alice',
        'profile-bob',
      ];
      VoiceProfile? profile(String id) => VoiceProfile(
        id: id,
        accountId: 'account-$id',
        username: id == 'profile-alice' ? 'alice' : 'bob',
        discriminator: '0001',
        displayName: id == 'profile-alice' ? 'Alice Smith' : 'Bob Jones',
        isPrimary: false,
        verificationType: 'none',
      );

      late Future<List<String>?> result;
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            createGroupFriendsProvider.overrideWith(friends),
            profileProvider(
              'profile-alice',
            ).overrideWith((_) async => profile('profile-alice')),
            profileProvider(
              'profile-bob',
            ).overrideWith((_) async => profile('profile-bob')),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(
              body: Builder(
                builder: (context) => TextButton(
                  onPressed: () {
                    result = GroupMemberPickerSheet.show(
                      context,
                      existingProfileIds: const {'existing-member'},
                      remainingCapacity: 3,
                    );
                  },
                  child: const Text('Open picker'),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('Open picker'));
      await tester.pumpAndSettle();

      expect(
        find.byKey(GroupMemberPickerSheet.memberKey('existing-member')),
        findsNothing,
      );
      expect(
        find.byKey(GroupMemberPickerSheet.memberKey('profile-alice')),
        findsOneWidget,
      );
      await tester.enterText(
        find.byKey(GroupMemberPickerSheet.searchFieldKey),
        'alice',
      );
      await tester.pumpAndSettle();
      expect(
        find.byKey(GroupMemberPickerSheet.memberKey('profile-bob')),
        findsNothing,
      );
      await tester.tap(
        find.byKey(GroupMemberPickerSheet.memberKey('profile-alice')),
      );
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<CheckboxListTile>(
              find.byKey(GroupMemberPickerSheet.memberKey('profile-alice')),
            )
            .value,
        isTrue,
      );
      await tester.tap(find.byKey(GroupMemberPickerSheet.submitKey));
      await tester.pumpAndSettle();

      expect(await result, ['profile-alice']);
    },
  );

  testWidgets('cancel returns no selection', (tester) async {
    Future<List<String>> friends(Ref ref) async => const ['profile-alice'];
    late Future<List<String>?> result;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [createGroupFriendsProvider.overrideWith(friends)],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: Builder(
              builder: (context) => TextButton(
                onPressed: () {
                  result = GroupMemberPickerSheet.show(
                    context,
                    existingProfileIds: const {},
                    remainingCapacity: 5,
                  );
                },
                child: const Text('Open picker'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('Open picker'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();

    expect(await result, isNull);
  });

  testWidgets('keyboard can select a friend and Escape restores the trigger', (
    tester,
  ) async {
    final triggerFocus = FocusNode(debugLabel: 'picker trigger');
    addTearDown(triggerFocus.dispose);
    final semantics = tester.ensureSemantics();
    Future<List<String>> friends(Ref ref) async => const ['profile-alice'];
    late Future<List<String>?> result;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          createGroupFriendsProvider.overrideWith(friends),
          profileProvider('profile-alice').overrideWith(
            (_) async => const VoiceProfile(
              id: 'profile-alice',
              accountId: 'account-alice',
              username: 'alice',
              discriminator: '0001',
              displayName: 'Alice',
            ),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: Builder(
              builder: (context) => TextButton(
                focusNode: triggerFocus,
                onPressed: () {
                  result = GroupMemberPickerSheet.show(
                    context,
                    existingProfileIds: const {},
                    remainingCapacity: 2,
                  );
                },
                child: const Text('Open picker'),
              ),
            ),
          ),
        ),
      ),
    );
    triggerFocus.requestFocus();
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();

    final editable = find.byType(EditableText);
    expect(editable, findsOneWidget);
    expect(Focus.of(tester.element(editable)).hasFocus, isTrue);
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.space);
    await tester.pump();
    final memberSemantics = tester.getSemantics(
      find.byKey(GroupMemberPickerSheet.memberKey('profile-alice')),
    );
    expect(memberSemantics.label, contains('Alice'));
    expect(
      tester
          .widget<FilledButton>(find.byKey(GroupMemberPickerSheet.submitKey))
          .onPressed,
      isNotNull,
    );

    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    expect(await result, isNull);
    expect(find.byType(GroupMemberPickerSheet), findsNothing);
    expect(triggerFocus.hasFocus, isTrue);
    semantics.dispose();
  });
}
