import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:http/testing.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/friends_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/matchmaking_providers.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/contact_profile_providers.dart';
import 'package:voice_frontend/state/dm_permission_provider.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/backend/stories_client.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/stories_providers.dart';
import 'package:voice_frontend/ui/core/voice_bottom_sheet.dart';
import 'package:voice_frontend/ui/social/profile_detail_sheet.dart';
import 'package:voice_frontend/ui/report/report_sheet.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';

import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';
import 'support/gateway_test_client.dart';
import 'support/voice_test_theme.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  setUp(() => SharedPreferences.setMockInitialValues({}));

  for (final locale in [const Locale('en'), const Locale('ru')]) {
    testWidgets('profile header title and copy link remain usable in ${locale.languageCode} H/V', (tester) async {
      final semantics = tester.ensureSemantics();
      final client = MockClient((request) async => throw StateError('Unexpected $request'));
      final container = _container(client);
      final theme = await _captureTheme(tester);
      final copied = <String>[];
      final messenger = TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
      messenger.setMockMethodCallHandler(SystemChannels.platform, (call) async {
        if (call.method == 'Clipboard.setData') {
          copied.add((call.arguments as Map<Object?, Object?>)['text'] as String);
        }
        return null;
      });
      addTearDown(() => messenger.setMockMethodCallHandler(SystemChannels.platform, null));
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(() => tester.binding.setSurfaceSize(null));
      try {
        for (final variant in [
          (size: const Size(1280, 800), scale: 1.0),
          (size: const Size(390, 844), scale: 1.0),
          (size: const Size(280, 844), scale: 1.0),
          (size: const Size(390, 844), scale: 2.0),
        ]) {
          tester.view.physicalSize = variant.size;
          tester.view.devicePixelRatio = 1;
          await tester.binding.setSurfaceSize(variant.size);
          await tester.pumpWidget(_app(container, theme: theme, locale: locale, textScale: variant.scale, plainModal: true));
          await tester.tap(find.byKey(const Key('open_contact_profile')));
          await tester.pumpAndSettle();
          final header = find.byKey(const Key('profile_header_controls'));
          final title = find.byKey(const Key('profile_header_title'));
          final l10n = AppLocalizations.of(tester.element(header))!;
          expect(find.descendant(of: title, matching: find.text(locale.languageCode == 'ru' ? 'Профиль' : 'Profile')), findsOneWidget);
          expect(tester.getSemantics(title).flagsCollection.isHeader, isTrue);
          final paragraph = tester.renderObject<RenderParagraph>(find.descendant(of: title, matching: find.byType(Text)));
          expect(paragraph.didExceedMaxLines, isFalse);
          final share = find.descendant(of: header, matching: find.byTooltip(l10n.shareLinkAction));
          expect(share.hitTestable(), findsOneWidget);
          expect(find.byTooltip(l10n.shareLinkAction), findsOneWidget);
          final titleRect = tester.getRect(title);
          final shareRect = tester.getRect(share);
          final dismissRect = tester.getRect(find.byKey(const Key('profile_dismiss')));
          expect(titleRect.right, lessThanOrEqualTo(shareRect.left));
          if (variant.size.width > variant.size.height) {
            expect(shareRect.right, lessThanOrEqualTo(dismissRect.left));
          } else {
            expect(dismissRect.right, lessThanOrEqualTo(titleRect.left));
          }
          expect(tester.getRect(header).bottom, lessThanOrEqualTo(tester.getRect(find.byKey(ProfileDetailSheet.avatarKey)).top));
          expect(titleRect.left, greaterThanOrEqualTo(0));
          expect(shareRect.right, lessThanOrEqualTo(variant.size.width));
          if (variant.scale == 1 && variant.size.width != 280) {
            final orientation = variant.size.width > variant.size.height ? 'h' : 'v';
            await _capture(tester, 'contact-header-title-${locale.languageCode}-$orientation.png');
          }
          final previousCopies = copied.length;
          await tester.tap(share);
          await tester.pumpAndSettle();
          expect(copied.length, previousCopies + 1);
          expect(copied.last, 'https://voice.gg/u/alex');
          expect(find.text(l10n.shareLinkCopied), findsOneWidget);
          expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
          expect(tester.takeException(), isNull);
          await tester.pumpWidget(const SizedBox.shrink());
          await tester.pumpAndSettle();
        }
      } finally {
        semantics.dispose();
        container.dispose();
        client.close();
      }
    });
  }

  for (final size in [const Size(1280, 800), const Size(390, 844)]) {
    testWidgets('profile More reuses Block confirmation and exact Report target at ${size.width}', (tester) async {
      var requests = 0;
      final client = MockClient((request) async { requests++; throw StateError('Unexpected $request'); });
      final container = _container(client);
      final theme = await _captureTheme(tester);
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(() => tester.binding.setSurfaceSize(null));
      try {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        await tester.pumpWidget(_app(container, theme: theme, plainModal: true));
        await tester.tap(find.byKey(const Key('open_contact_profile')));
        await tester.pumpAndSettle();
        final more = find.byKey(const Key('profile_more'));
        final l10n = AppLocalizations.of(tester.element(more))!;
        await tester.tap(more);
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('profile_more_block')).hitTestable(), findsOneWidget);
        expect(find.byKey(const Key('profile_more_report')).hitTestable(), findsOneWidget);
        final orientation = size.width > size.height ? 'h' : 'v';
        await _capture(tester, 'contact-header-menu-$orientation.png');
        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('profile_more_block')), findsNothing);
        expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
        await tester.tap(more);
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('profile_more_block')));
        await tester.pumpAndSettle();
        expect(find.text(l10n.profileBlockConfirmTitle), findsOneWidget);
        await tester.tap(find.text(l10n.commonCancel));
        await tester.pumpAndSettle();
        expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
        expect(requests, 0);
        await tester.tap(more);
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const Key('profile_more_report')));
        await tester.pumpAndSettle();
        expect(find.byKey(ProfileDetailSheet.sheetKey), findsNothing);
        final report = tester.widget<ReportSheet>(find.byType(ReportSheet));
        expect(report.target, isA<ReportUserTarget>());
        expect(report.target.targetId, 'p-target');
        expect(report.target.targetType, 'user');
        expect(requests, 0);
        expect(tester.takeException(), isNull);
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pumpAndSettle();
      } finally {
        container.dispose();
        client.close();
      }
    });
  }

  testWidgets('header actions follow profile visibility during authorization reload', (tester) async {
    final pending = Completer<http.Response>();
    final authorizations = <String?>[];
    final client = MockClient((request) async {
      expect(request.url.path, '/api/v1/users/profiles/p-target');
      authorizations.add(request.headers['authorization']);
      if (authorizations.length > 1) return pending.future;
      return _json({'profile': {
        'id': 'p-target', 'account_id': 'a-target', 'username': 'alex',
        'discriminator': '0001', 'display_name': 'Alex',
      }});
    });
    final container = _container(client, realProfile: true);
    try {
      await _open(tester, container);
      final title = find.byKey(const Key('profile_header_title'));
      final l10n = AppLocalizations.of(tester.element(title))!;
      expect(find.byTooltip(l10n.shareLinkAction), findsOneWidget);
      expect(find.byKey(const Key('profile_more')), findsOneWidget);
      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'other-access', refreshToken: 'other-refresh',
          accountId: 'acc-other', activeProfileId: 'other-profile',
          expiresInSeconds: 900,
        ),
      );
      await tester.pump();
      await tester.pump();
      expect(title, findsOneWidget);
      expect(find.byTooltip(l10n.shareLinkAction), findsNothing);
      expect(find.byKey(const Key('profile_more')), findsNothing);
      pending.complete(_json({'code': 5, 'message': 'not found'}, status: 404));
      await tester.pumpAndSettle();
      expect(find.byTooltip(l10n.shareLinkAction), findsNothing);
      expect(find.byKey(const Key('profile_more')), findsNothing);
      expect(find.text(l10n.socialProfileUnavailable), findsOneWidget);
      expect(authorizations, ['Bearer test-access', 'Bearer other-access']);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
    } finally {
      if (!pending.isCompleted) pending.complete(_json({'profile': {}}));
      container.dispose();
      client.close();
    }
  });

  for (final target in ['guest', 'self', 'no-username']) {
    testWidgets('header preserves copy-link gate for $target', (tester) async {
      final client = MockClient((request) async => throw StateError('Unexpected $request'));
      final container = _container(client, overrides: [
        if (target == 'no-username')
          profileProvider.overrideWith((ref, id) async => const VoiceProfile(
            id: 'p-target', accountId: 'a-target', username: '',
            discriminator: '0001', displayName: 'Alex',
          )),
      ]);
      if (target != 'no-username') {
        container.read(authControllerProvider.notifier).state = AuthState(
          isGuest: target == 'guest',
          session: AuthSession(
            accessToken: 'test-access', refreshToken: 'test-refresh',
            accountId: 'acc-test', activeProfileId: target == 'self' ? 'p-target' : 'prof-test',
            expiresInSeconds: 900, accountType: target == 'guest' ? 'guest' : 'regular',
          ),
        );
      }
      try {
        await _open(tester, container);
        final title = find.byKey(const Key('profile_header_title'));
        final l10n = AppLocalizations.of(tester.element(title))!;
        expect(find.byTooltip(l10n.shareLinkAction), findsNothing);
        expect(find.byKey(const Key('profile_more')), target == 'self' ? findsNothing : findsOneWidget);
        expect(tester.takeException(), isNull);
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pumpAndSettle();
      } finally {
        container.dispose();
        client.close();
      }
    });
  }

  for (final favorite in [false, true]) {
    testWidgets('friend Favorites ${favorite ? 'on' : 'off'} stays with communication actions in H/V', (
      tester,
    ) async {
      final client = MockClient(
        (request) async => throw StateError('Unexpected $request'),
      );
      final container = _container(client, overrides: [
        profileProvider.overrideWith((ref, id) async => const VoiceProfile(
          id: 'p-target', accountId: 'a-target', username: 'alex',
          discriminator: '0001', displayName: 'Alex', bio: 'Co-op and design.',
          verificationType: 'personal', bannerUrl: _bannerUrl,
        )),
        isFriendProvider.overrideWith((ref, id) => true),
        profileContactMembershipProvider.overrideWith((ref, id) async => false),
        presenceProvider.overrideWith((ref, id) => VoicePresence(
          profileId: id, status: 'online', customStatus: 'Ready for co-op',
        )),
        activeStoryAuthorIdsProvider.overrideWith((ref) => {'p-target'}),
        profileStoriesProvider.overrideWith((ref, id) async => const [
          StoryData(id: 'story-one', authorProfileId: 'p-target', type: 'text', textContent: 'Hello'),
        ]),
        profileHighlightsProvider.overrideWith((ref, id) async => const [
          HighlightData(id: 'co-op', name: 'Co-op', profileId: 'p-target', storyIds: ['story-one'], visibility: 'friends'),
        ]),
        contactCommonSpacesProvider.overrideWith((ref, id) async => [_commonSpace]),
        favoritesListProvider.overrideWith((ref) async => FavoritesListData(
          favorites: favorite ? ['p-target'] : [],
        )),
      ]);
      final theme = await _captureTheme(tester);
      final imageClient = _ImageClient(await _avatarBytes(tester), expectedUrl: _bannerUrl);
      final previousImageClient = debugNetworkImageHttpClientProvider;
      debugNetworkImageHttpClientProvider = () => imageClient;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(() => tester.binding.setSurfaceSize(null));
      try {
        for (final size in [const Size(1280, 800), const Size(390, 844)]) {
          tester.view.physicalSize = size;
          tester.view.devicePixelRatio = 1;
          await tester.binding.setSurfaceSize(size);
          await tester.pumpWidget(_app(container, theme: theme, plainModal: true));
          await tester.runAsync(() => precacheImage(
            const NetworkImage(_bannerUrl),
            tester.element(find.byKey(const Key('open_contact_profile'))),
          ));
          await tester.tap(find.byKey(const Key('open_contact_profile')));
          await tester.pumpAndSettle();
          final toggle = find.byKey(const Key('profile_favorite_toggle'));
          final call = find.byKey(ProfileDetailSheet.callKey);
          final l10n = AppLocalizations.of(tester.element(toggle))!;
          expect(find.byTooltip(favorite ? l10n.socialRemoveFavorite : l10n.socialAddFavorite), findsOneWidget);
          final toggleRect = tester.getRect(toggle);
          final callRect = tester.getRect(call);
          final highlightsRect = tester.getRect(find.byKey(const Key('highlights_section')));
          final commonRect = tester.getRect(find.byKey(const Key('profile_common_spaces')));
          expect(toggleRect.bottom, lessThanOrEqualTo(highlightsRect.top));
          expect(highlightsRect.bottom, lessThanOrEqualTo(commonRect.top));
          if ((toggleRect.top - callRect.top).abs() < 1) {
            expect(callRect.right, lessThan(toggleRect.left));
          } else {
            expect(callRect.bottom, lessThanOrEqualTo(toggleRect.top));
          }
          expect(toggleRect.left, greaterThanOrEqualTo(0));
          expect(toggleRect.right, lessThanOrEqualTo(size.width));
          expect(find.text('Ready for co-op'), findsOneWidget);
          expect(find.byKey(const Key('profile_not_in_contacts_warning')), findsNothing);
          expect(tester.takeException(), isNull);
          final orientation = size.width > size.height ? 'h' : 'v';
          await _capture(tester, 'contact-favorite-${favorite ? 'on' : 'off'}-$orientation.png');
          await tester.ensureVisible(find.byKey(const Key('profile_report')));
          await tester.pumpAndSettle();
          expect(find.byKey(const Key('profile_report')).hitTestable(), findsOneWidget);
          await tester.pumpWidget(const SizedBox.shrink());
          await tester.pumpAndSettle();
        }
      } finally {
        debugNetworkImageHttpClientProvider = previousImageClient;
        PaintingBinding.instance.imageCache.clear();
        PaintingBinding.instance.imageCache.clearLiveImages();
        container.dispose();
        client.close();
      }
    });
  }

  for (final state in ['loading', 'error']) {
    testWidgets('friend Favorites $state recovery stays in the action band', (
      tester,
    ) async {
      final pending = Completer<FavoritesListData>();
      var reads = 0;
      final client = MockClient((request) async => throw StateError('Unexpected $request'));
      final container = _container(client, overrides: [
        isFriendProvider.overrideWith((ref, id) => true),
        favoritesListProvider.overrideWith((ref) async {
          reads++;
          if (state == 'loading') return pending.future;
          if (reads == 1) throw StateError('favorite-private-diagnostic');
          return const FavoritesListData(favorites: []);
        }),
      ]);
      try {
        await tester.pumpWidget(_app(container, plainModal: true));
        await tester.tap(find.byKey(const Key('open_contact_profile')));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));
        if (state == 'error') {
          await tester.pumpAndSettle();
        }
        final band = find.byKey(const Key('profile_communication_actions'));
        if (state == 'loading') {
          expect(find.descendant(of: band, matching: find.byType(CircularProgressIndicator)), findsOneWidget);
          pending.complete(const FavoritesListData(favorites: []));
        } else {
          final l10n = AppLocalizations.of(tester.element(band))!;
          final retry = find.descendant(of: band, matching: find.text(l10n.commonRetry));
          expect(retry, findsOneWidget);
          expect(find.textContaining('favorite-private-diagnostic'), findsNothing);
          await tester.ensureVisible(retry);
          await tester.tap(retry);
        }
        await tester.pumpAndSettle();
        expect(find.descendant(of: band, matching: find.byKey(const Key('profile_favorite_toggle'))), findsOneWidget);
        expect(reads, state == 'loading' ? 1 : 2);
        expect(tester.takeException(), isNull);
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pumpAndSettle();
      } finally {
        if (!pending.isCompleted) {
          pending.complete(const FavoritesListData(favorites: []));
        }
        container.dispose();
        client.close();
      }
    });
  }

  for (final variant in [
    (name: 'h', size: const Size(1280, 800), scale: 1.0, locale: 'en'),
    (name: 'v', size: const Size(390, 844), scale: 1.0, locale: 'en'),
    (name: 'narrow', size: const Size(280, 844), scale: 1.0, locale: 'en'),
    (name: 'large-text', size: const Size(390, 844), scale: 2.0, locale: 'ru'),
  ]) {
    testWidgets('communication band precedes profile collections at ${variant.name}', (
      tester,
    ) async {
      final client = MockClient(
        (request) async => throw StateError('Unexpected $request'),
      );
      final populatedHighlights = variant.scale == 1;
      final container = _container(client, overrides: [
        profileProvider.overrideWith((ref, id) async => const VoiceProfile(
          id: 'p-target', accountId: 'a-target', username: 'alex',
          discriminator: '0001', displayName: 'Alex', bio: 'Co-op and design.',
          verificationType: 'personal',
        )),
        contactCommonSpacesProvider.overrideWith((ref, id) async => [_commonSpace]),
        profileHighlightsProvider.overrideWith((ref, id) async => [
          if (populatedHighlights)
            const HighlightData(id: 'co-op', name: 'Co-op', profileId: 'p-target'),
        ]),
      ]);
      final theme = await _captureTheme(tester);
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(() => tester.binding.setSurfaceSize(null));
      try {
        tester.view.physicalSize = variant.size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(variant.size);
        await tester.pumpWidget(_app(
          container, theme: theme, locale: Locale(variant.locale),
          textScale: variant.scale, plainModal: true,
        ));
        await tester.tap(find.byKey(const Key('open_contact_profile')));
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
        final message = find.byKey(ProfileDetailSheet.messageKey);
        final call = find.byKey(ProfileDetailSheet.callKey);
        await tester.ensureVisible(message);
        await tester.pumpAndSettle();
        final messageRect = tester.getRect(message);
        final callRect = tester.getRect(call);
        final bioRect = tester.getRect(find.text('Co-op and design.'));
        final commonRect = tester.getRect(
          find.byKey(const Key('profile_common_spaces')),
        );
        expect(bioRect.bottom, lessThanOrEqualTo(messageRect.top));
        expect(messageRect.bottom, lessThanOrEqualTo(commonRect.top));
        expect(callRect.bottom, lessThanOrEqualTo(commonRect.top));
        if (populatedHighlights) {
          final highlightsRect = tester.getRect(
            find.byKey(const Key('highlights_section')),
          );
          expect(messageRect.bottom, lessThanOrEqualTo(highlightsRect.top));
          expect(callRect.bottom, lessThanOrEqualTo(highlightsRect.top));
          expect(highlightsRect.bottom, lessThanOrEqualTo(commonRect.top));
        }
        // The reference permits wrapping on V when localized controls need it.
        final sharesRow = (messageRect.top - callRect.top).abs() < 1;
        if (variant.name == 'h' || (variant.name == 'v' && sharesRow)) {
          expect(messageRect.top, closeTo(callRect.top, 1));
          expect(messageRect.right, lessThan(callRect.left));
        } else {
          expect(messageRect.bottom, lessThanOrEqualTo(callRect.top));
        }
        for (final rect in [messageRect, callRect]) {
          expect(rect.left, greaterThanOrEqualTo(0));
          expect(rect.right, lessThanOrEqualTo(variant.size.width));
        }
        await _capture(tester, 'contact-communication-${variant.name}.png');
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pumpAndSettle();
      } finally {
        container.dispose();
        client.close();
      }
    });
  }

  for (final state in ['loading', 'denied', 'error']) {
    testWidgets('communication band preserves $state DM permission state', (
      tester,
    ) async {
      final pending = Completer<bool>();
      var reads = 0;
      final client = MockClient(
        (request) async => throw StateError('Unexpected $request'),
      );
      final container = _container(client, overrides: [
        dmPermissionProvider.overrideWith((ref, request) async {
          reads++;
          if (state == 'loading') return pending.future;
          if (state == 'error' && reads == 1) throw StateError('private-diagnostic');
          return state != 'denied';
        }),
        contactCommonSpacesProvider.overrideWith((ref, id) async => [_commonSpace]),
      ]);
      try {
        await _open(tester, container);
        expect(find.byKey(ProfileDetailSheet.messageKey), findsNothing);
        expect(find.byKey(ProfileDetailSheet.callKey), findsOneWidget);
        expect(find.textContaining('private-diagnostic'), findsNothing);
        if (state == 'error') {
          final l10n = AppLocalizations.of(
            tester.element(find.byKey(ProfileDetailSheet.callKey)),
          )!;
          await tester.ensureVisible(find.text(l10n.commonRetry));
          await tester.tap(find.text(l10n.commonRetry));
          await tester.pumpAndSettle();
          expect(find.byKey(ProfileDetailSheet.messageKey), findsOneWidget);
          expect(reads, 2);
        }
        expect(tester.takeException(), isNull);
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pumpAndSettle();
      } finally {
        if (!pending.isCompleted) pending.complete(false);
        container.dispose();
        client.close();
      }
    });
  }

  for (final locale in [const Locale('en'), const Locale('ru')]) {
    for (final size in [const Size(1280, 800), const Size(390, 844)]) {
      testWidgets('visible profile dismissal is localized and keyboard accessible '
          'in ${locale.languageCode} at ${size.width}', (tester) async {
        final semantics = tester.ensureSemantics();
        final client = MockClient(
          (request) async => throw StateError('Unexpected $request'),
        );
        final container = _container(client);
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        addTearDown(() => tester.binding.setSurfaceSize(null));
        try {
          tester.view.physicalSize = size;
          tester.view.devicePixelRatio = 1;
          await tester.binding.setSurfaceSize(size);
          await tester.pumpWidget(
            _app(container, locale: locale, plainModal: true),
          );
          final triggerFocus = Focus.of(tester.element(find.text('Open profile')));
          triggerFocus.requestFocus();
          await tester.pump();
          await tester.sendKeyEvent(LogicalKeyboardKey.enter);
          await tester.pumpAndSettle();
          final dismiss = find.byKey(const Key('profile_dismiss'));
          expect(dismiss, findsOneWidget);
          final narrow = size.width < 600;
          expect(
            narrow ? find.byType(BackButton) : find.byType(CloseButton),
            findsOneWidget,
          );
          final material = MaterialLocalizations.of(tester.element(dismiss));
          final label = narrow
              ? material.backButtonTooltip
              : material.closeButtonTooltip;
          expect(tester.getSemantics(dismiss).label, contains(label));
          expect(tester.getSemantics(dismiss).flagsCollection.isButton, isTrue);
          final rect = tester.getRect(dismiss);
          expect(rect.left, greaterThanOrEqualTo(0));
          expect(rect.right, lessThanOrEqualTo(size.width));
          await tester.sendKeyEvent(LogicalKeyboardKey.tab);
          await tester.pump();
          await tester.sendKeyEvent(LogicalKeyboardKey.enter);
          await tester.pumpAndSettle();
          expect(find.byKey(ProfileDetailSheet.sheetKey), findsNothing);
          expect(triggerFocus.hasFocus, isTrue);
          expect(tester.takeException(), isNull);
          await tester.pumpWidget(const SizedBox.shrink());
          await tester.pumpAndSettle();
        } finally {
          semantics.dispose();
          container.dispose();
          client.close();
        }
      });
    }
  }

  for (final state in ['loading', 'error', 'unavailable']) {
    testWidgets('profile $state keeps a visible dismissal control in H/V', (
      tester,
    ) async {
      final theme = await _captureTheme(tester);
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(() => tester.binding.setSurfaceSize(null));
      for (final size in [const Size(1280, 800), const Size(390, 844)]) {
        final pending = Completer<http.Response>();
        final methods = <String>[];
        final client = MockClient((request) async {
          methods.add(request.method);
          expect(request.url.path, '/api/v1/users/profiles/p-target');
          expect(request.headers['authorization'], 'Bearer test-access');
          if (state == 'loading') return pending.future;
          return _json(
            {'code': state == 'error' ? 13 : 5, 'message': 'private-diagnostic'},
            status: state == 'error' ? 500 : 404,
          );
        });
        final container = _container(client, realProfile: true);
        try {
          tester.view.physicalSize = size;
          tester.view.devicePixelRatio = 1;
          await tester.binding.setSurfaceSize(size);
          await tester.pumpWidget(
            _app(container, theme: theme, plainModal: true),
          );
          await tester.tap(find.byKey(const Key('open_contact_profile')));
          if (state == 'loading') {
            await tester.pump();
            await tester.pump(const Duration(milliseconds: 350));
          } else {
            await tester.pumpAndSettle();
          }
          final dismiss = find.byKey(const Key('profile_dismiss'));
          expect(dismiss.hitTestable(), findsOneWidget);
          final title = find.byKey(const Key('profile_header_title'));
          expect(find.descendant(of: title, matching: find.text('Profile')), findsOneWidget);
          expect(find.byKey(const Key('profile_more')), findsNothing);
          final l10n = AppLocalizations.of(tester.element(title))!;
          expect(find.byTooltip(l10n.shareLinkAction), findsNothing);
          final icon = find.descendant(of: dismiss, matching: find.byType(Icon));
          final iconContext = tester.element(icon);
          final foreground = tester.widget<Icon>(icon).color ??
              IconTheme.of(iconContext).color!;
          final background = Theme.of(iconContext).colorScheme.surface;
          final foregroundLuminance = foreground.computeLuminance();
          final backgroundLuminance = background.computeLuminance();
          final lighter = foregroundLuminance > backgroundLuminance
              ? foregroundLuminance
              : backgroundLuminance;
          final darker = foregroundLuminance < backgroundLuminance
              ? foregroundLuminance
              : backgroundLuminance;
          expect((lighter + 0.05) / (darker + 0.05), greaterThanOrEqualTo(3));
          expect(find.byKey(ProfileDetailSheet.messageKey), findsNothing);
          expect(find.textContaining('private-diagnostic'), findsNothing);
          final orientation = size.width > size.height ? 'h' : 'v';
          await _capture(tester, 'contact-dismiss-$state-$orientation.png');
          await tester.tap(dismiss);
          await tester.pumpAndSettle();
          expect(find.byKey(ProfileDetailSheet.sheetKey), findsNothing);
          expect(methods, ['GET']);
          expect(tester.takeException(), isNull);
          if (!pending.isCompleted) {
            pending.complete(_json({'profile': {}}));
          }
          await tester.pumpWidget(const SizedBox.shrink());
          await tester.pumpAndSettle();
        } finally {
          if (!pending.isCompleted) {
            pending.complete(_json({'profile': {}}));
          }
          container.dispose();
          client.close();
        }
      }
    });
  }

  for (final plainModal in [true, false]) {
    testWidgets('long profile content preserves dismissal in '
        '${plainModal ? 'plain' : 'shared'} modal', (tester) async {
      final client = MockClient(
        (request) async => throw StateError('Unexpected $request'),
      );
      final container = _container(
        client,
        overrides: [
          profileProvider.overrideWith(
            (ref, id) async => VoiceProfile(
              id: 'p-target',
              accountId: 'a-target',
              username: 'alex',
              discriminator: '0001',
              displayName: 'Alex',
              bio: List.filled(40, 'About Alex. ').join(),
            ),
          ),
        ],
      );
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(() => tester.binding.setSurfaceSize(null));
      try {
        tester.view.physicalSize = const Size(390, 400);
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(const Size(390, 400));
        await tester.pumpWidget(_app(container, plainModal: plainModal));
        await tester.tap(find.byKey(const Key('open_contact_profile')));
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
        await tester.ensureVisible(find.byKey(ProfileDetailSheet.blockKey));
        await tester.pumpAndSettle();
        final dismiss = find.byKey(const Key('profile_dismiss'));
        if (!plainModal) {
          await tester.ensureVisible(dismiss);
          await tester.pumpAndSettle();
        }
        expect(dismiss.hitTestable(), findsOneWidget);
        await tester.tap(dismiss);
        await tester.pumpAndSettle();
        expect(find.byKey(ProfileDetailSheet.sheetKey), findsNothing);
        expect(tester.takeException(), isNull);
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pumpAndSettle();
      } finally {
        container.dispose();
        client.close();
      }
    });
  }

  for (final locale in [const Locale('en'), const Locale('ru')]) {
    testWidgets(
      'no-story avatar semantics, keyboard and Close focus in ${locale.languageCode}',
      (tester) async {
        final semantics = tester.ensureSemantics();
        try {
          final unexpectedRequests = <String>[];
          final client = MockClient((request) async {
            unexpectedRequests.add('${request.method} ${request.url}');
            throw StateError('Unexpected HTTP request');
          });
          final container = _container(client);
          addTearDown(container.dispose);
          addTearDown(client.close);
          await tester.pumpWidget(_app(container, locale: locale));
          await tester.tap(find.byKey(const Key('open_contact_profile')));
          await tester.pumpAndSettle();

          final avatar = find.byKey(ProfileDetailSheet.avatarKey);
          final label = AppLocalizations.of(
            tester.element(avatar),
          )!.profileOpenAvatar;
          final node = tester.getSemantics(avatar);
          expect(node.label, contains(label));
          expect(node.flagsCollection.isButton, isTrue);
          expect(
            node.getSemanticsData().hasAction(ui.SemanticsAction.tap),
            isTrue,
          );
          final focus = Focus.of(
            tester.element(
              find.descendant(of: avatar, matching: find.byType(CircleAvatar)),
            ),
          );
          focus.requestFocus();
          await tester.pump();
          expect(focus.hasFocus, isTrue);
          await tester.sendKeyEvent(LogicalKeyboardKey.enter);
          await tester.pumpAndSettle();
          expect(
            find.byKey(const Key('profile_avatar_fullscreen')),
            findsOneWidget,
          );
          expect(
            find.byKey(const Key('profile_avatar_missing')),
            findsOneWidget,
          );
          await tester.tap(find.byKey(const Key('profile_avatar_close')));
          await tester.pumpAndSettle();
          expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
          expect(focus.hasFocus, isTrue);
          await tester.pumpWidget(const SizedBox.shrink());
          await tester.pumpAndSettle();
          expect(unexpectedRequests, isEmpty);
        } finally {
          semantics.dispose();
        }
      },
    );
  }

  testWidgets(
    'Call creates the target DM once and starts an actual audio call',
    (tester) async {
      final dm = Completer<http.Response>();
      final requests = <http.Request>[];
      final client = MockClient((request) async {
        requests.add(request);
        expect(request.headers['authorization'], 'Bearer test-access');
        if (request.url.path == '/api/v1/chats/dm') return dm.future;
        if (request.url.path == '/api/v1/voice/calls') {
          return _json({
            'call_session': {
              'room_id': 'room-contact',
              'livekit_room_name': 'voice-dm-room-contact',
              'linked_chat': {'id': 'dm-contact'},
              'initiator_profile_id': 'prof-test',
              'callee_profile_id': 'p-target',
              'media_kind': 'CALL_MEDIA_KIND_AUDIO',
              'status': 'CALL_STATUS_RINGING',
            },
          });
        }
        throw StateError('Unexpected $request');
      });
      final container = _container(client);
      addTearDown(container.dispose);
      addTearDown(client.close);
      await _open(tester, container);
      final button = find.byKey(ProfileDetailSheet.callKey);
      await tester.ensureVisible(button);
      await tester.tap(button);
      await tester.pump();
      expect(tester.widget<OutlinedButton>(button).onPressed, isNull);
      await tester.tap(button);
      expect(requests, hasLength(1));
      expect(jsonDecode(requests.single.body), {
        'other_profile_id': 'p-target',
      });
      dm.complete(_dmResponse());
      await tester.pumpAndSettle();
      expect(requests, hasLength(2));
      expect(requests.last.method, 'POST');
      expect(jsonDecode(requests.last.body), {
        'linked_chat': {'id': 'dm-contact'},
        'callee_profile_id': 'p-target',
        'media_kind': 'CALL_MEDIA_KIND_AUDIO',
      });
      expect(container.read(callControllerProvider).phase, CallPhase.outgoing);
      expect(
        container.read(callControllerProvider).session?.chatId,
        'dm-contact',
      );
      expect(find.byKey(ProfileDetailSheet.sheetKey), findsNothing);
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
    },
  );

  testWidgets(
    'privacy rejection creating DM keeps profile open and never calls Voice',
    (tester) async {
      final paths = <String>[];
      final client = MockClient((request) async {
        paths.add(request.url.path);
        return _json({'code': 7, 'message': 'permission denied'}, status: 403);
      });
      final container = _container(client);
      addTearDown(container.dispose);
      addTearDown(client.close);
      await _open(tester, container);
      await tester.ensureVisible(find.byKey(ProfileDetailSheet.callKey));
      await tester.tap(find.byKey(ProfileDetailSheet.callKey));
      await tester.pumpAndSettle();
      expect(paths, ['/api/v1/chats/dm']);
      expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
      expect(find.byType(SnackBar), findsOneWidget);
      expect(
        tester
            .widget<OutlinedButton>(find.byKey(ProfileDetailSheet.callKey))
            .onPressed,
        isNotNull,
      );
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
    },
  );

  testWidgets('profile switch while DM is pending never starts the old call', (
    tester,
  ) async {
    final dm = Completer<http.Response>();
    final paths = <String>[];
    final client = MockClient((request) {
      paths.add(request.url.path);
      return dm.future;
    });
    final container = _container(client);
    addTearDown(container.dispose);
    addTearDown(client.close);
    await _open(tester, container);
    await tester.ensureVisible(find.byKey(ProfileDetailSheet.callKey));
    await tester.tap(find.byKey(ProfileDetailSheet.callKey));
    await tester.pump();
    container.read(authControllerProvider.notifier).state = const AuthState(
      session: AuthSession(
        accessToken: 'other-access',
        refreshToken: 'other-refresh',
        accountId: 'acc-test',
        activeProfileId: 'other-profile',
        expiresInSeconds: 900,
      ),
    );
    await tester.pump();
    dm.complete(_dmResponse());
    await tester.pumpAndSettle();
    expect(paths, ['/api/v1/chats/dm']);
    expect(container.read(selectedChatIdProvider), isNull);
    expect(container.read(callControllerProvider).phase, CallPhase.idle);
    expect(find.byType(SnackBar), findsNothing);
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pumpAndSettle();
  });

  testWidgets(
    'switch flag alone fences pending DM selection and call startup',
    (tester) async {
      final dm = Completer<http.Response>();
      final paths = <String>[];
      final client = MockClient((request) {
        paths.add(request.url.path);
        return dm.future;
      });
      final container = _container(client);
      addTearDown(container.dispose);
      addTearDown(client.close);
      container.read(selectedChatIdProvider.notifier).state = 'previous-chat';
      await _open(tester, container);
      await tester.ensureVisible(find.byKey(ProfileDetailSheet.callKey));
      await tester.tap(find.byKey(ProfileDetailSheet.callKey));
      await tester.pump();
      container.read(profileSwitchInProgressProvider.notifier).state = true;
      await tester.pump();
      dm.complete(_dmResponse());
      await tester.pumpAndSettle();
      expect(paths, ['/api/v1/chats/dm']);
      expect(container.read(selectedChatIdProvider), 'previous-chat');
      expect(container.read(dmPeerProfileByChatIdProvider), isEmpty);
      expect(
        (container.read(voiceChatsClientProvider) as _HttpDmChatsClient)
            .inboxReads,
        0,
      );
      expect(container.read(callControllerProvider).phase, CallPhase.idle);
      expect(find.byType(SnackBar), findsNothing);
      expect(
        tester
            .widget<OutlinedButton>(find.byKey(ProfileDetailSheet.callKey))
            .onPressed,
        isNull,
      );
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
    },
  );

  for (final selected in [null, 'common']) {
    testWidgets('common-space action opens space idempotently from $selected', (
      tester,
    ) async {
      final client = MockClient(
        (request) async => throw StateError('Unexpected $request'),
      );
      final container = _container(
        client,
        overrides: [
          contactCommonSpacesProvider.overrideWith(
            (ref, id) async => const [_commonSpace],
          ),
        ],
      );
      addTearDown(container.dispose);
      addTearDown(client.close);
      container.read(selectedSpaceIdProvider.notifier).state = selected;
      await _open(tester, container);
      final row = find.byKey(const ValueKey('profile_common_space_common'));
      await tester.ensureVisible(row);
      await tester.tap(row);
      await tester.pumpAndSettle();
      expect(container.read(selectedSpaceIdProvider), 'common');
      expect(find.byKey(ProfileDetailSheet.sheetKey), findsNothing);
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
    });
  }

  testWidgets(
    'failed common-space scan offers retry and only shows successful results',
    (tester) async {
      var attempts = 0;
      final client = MockClient(
        (request) async => throw StateError('Unexpected $request'),
      );
      final container = _container(
        client,
        overrides: [
          contactCommonSpacesProvider.overrideWith((ref, id) async {
            attempts++;
            if (attempts == 1) throw StateError('failed page');
            return const [_commonSpace];
          }),
        ],
      );
      addTearDown(container.dispose);
      addTearDown(client.close);
      await _open(tester, container);
      final error = find.byKey(const Key('profile_common_spaces_error'));
      expect(error, findsOneWidget);
      expect(find.byKey(const Key('profile_common_spaces')), findsNothing);
      final retry = find.descendant(
        of: error,
        matching: find.text(
          AppLocalizations.of(tester.element(error))!.commonRetry,
        ),
      );
      await tester.ensureVisible(retry);
      await tester.tap(retry);
      await tester.pumpAndSettle();
      expect(attempts, 2);
      expect(error, findsNothing);
      expect(find.byKey(const Key('profile_common_spaces')), findsOneWidget);
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
    },
  );

  testWidgets('active story avatar on banner keeps existing viewer route and ids', (
    tester,
  ) async {
    final client = MockClient(
      (request) async => throw StateError('Unexpected $request'),
    );
    final container = _container(
      client,
      overrides: [
        profileProvider.overrideWith(
          (ref, id) async => const VoiceProfile(
            id: 'p-target',
            accountId: 'a-target',
            username: 'alex',
            discriminator: '0001',
            displayName: 'Alex',
            bannerUrl: _bannerUrl,
          ),
        ),
        activeStoryAuthorIdsProvider.overrideWith((ref) => const {'p-target'}),
        profileStoriesProvider.overrideWith(
          (ref, id) async => const [
            StoryData(
              id: 'story-one',
              authorProfileId: 'p-target',
              type: 'text',
              textContent: 'Hello',
            ),
          ],
        ),
      ],
    );
    final imageClient = _ImageClient(
      await _avatarBytes(tester),
      expectedUrl: _bannerUrl,
    );
    final previousImageClient = debugNetworkImageHttpClientProvider;
    debugNetworkImageHttpClientProvider = () => imageClient;
    Uri? destination;
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => Scaffold(
            body: TextButton(
              onPressed: () => showVoiceBottomSheet<void>(
                context: context,
                child: const ProfileDetailSheet(profileId: 'p-target'),
              ),
              child: const Text('Open profile'),
            ),
          ),
        ),
        GoRoute(
          path: '/stories/viewer',
          builder: (context, state) {
            destination = state.uri;
            return const Scaffold(body: Text('Actual story route'));
          },
        ),
      ],
    );
    try {
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp.router(
            routerConfig: router,
            theme: voiceTestTheme(),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
          ),
        ),
      );
      await tester.tap(find.text('Open profile'));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(ProfileDetailSheet.avatarKey));
      await tester.pumpAndSettle();
      expect(destination?.queryParameters, {
        'ids': 'story-one',
        'index': '0',
        'profileId': 'p-target',
      });
      expect(find.text('Actual story route'), findsOneWidget);
      expect(find.byKey(const Key('profile_avatar_fullscreen')), findsNothing);
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
    } finally {
      debugNetworkImageHttpClientProvider = previousImageClient;
      PaintingBinding.instance.imageCache.clear();
      PaintingBinding.instance.imageCache.clearLiveImages();
      router.dispose();
      container.dispose();
      client.close();
    }
  });

  for (final accountType in ['guest', 'regular']) {
    testWidgets(
      'Call is disabled for guests and absent for self ($accountType)',
      (tester) async {
        final client = MockClient(
          (request) async => throw StateError('Unexpected $request'),
        );
        final container = _container(client);
        addTearDown(container.dispose);
        addTearDown(client.close);
        container.read(authControllerProvider.notifier).state = AuthState(
          isGuest: accountType == 'guest',
          session: AuthSession(
            accessToken: 'test-access',
            refreshToken: 'test-refresh',
            accountId: 'acc-test',
            activeProfileId: accountType == 'guest' ? 'prof-test' : 'p-target',
            expiresInSeconds: 900,
            accountType: accountType,
          ),
        );
        await _open(tester, container);
        expect(
          container.read(authControllerProvider).isGuest,
          accountType == 'guest',
        );
        final call = find.byKey(ProfileDetailSheet.callKey);
        if (accountType == 'guest') {
          expect(tester.widget<OutlinedButton>(call).onPressed, isNull);
        } else {
          expect(call, findsNothing);
        }
        expect(find.byKey(const Key('profile_common_spaces')), findsNothing);
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pumpAndSettle();
      },
    );
  }

  testWidgets(
    'fullscreen profile refresh hides prior avatar during changed authorization',
    (tester) async {
      final pending = Completer<http.Response>();
      final authorizations = <String?>[];
      final client = MockClient((request) async {
        expect(request.url.path, '/api/v1/users/profiles/p-target');
        authorizations.add(request.headers['authorization']);
        return authorizations.length == 1
            ? _json({
                'profile': {
                  'id': 'p-target',
                  'account_id': 'a-target',
                  'username': 'alex',
                  'discriminator': '0001',
                  'display_name': 'Alex',
                  'is_primary': true,
                },
              })
            : pending.future;
      });
      final container = _container(client, realProfile: true);
      addTearDown(container.dispose);
      addTearDown(client.close);
      await _open(tester, container);
      await tester.tap(find.byKey(ProfileDetailSheet.avatarKey));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('profile_avatar_missing')), findsOneWidget);
      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'other-access',
          refreshToken: 'other-refresh',
          accountId: 'acc-other',
          activeProfileId: 'other-profile',
          expiresInSeconds: 900,
        ),
      );
      await tester.pump();
      expect(find.byKey(const Key('profile_avatar_missing')), findsNothing);
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      pending.complete(_json({'code': 5, 'message': 'not found'}, status: 404));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('profile_avatar_missing')), findsNothing);
      expect(find.text('User unavailable'), findsOneWidget);
      expect(authorizations, ['Bearer test-access', 'Bearer other-access']);
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
    },
  );

  testWidgets('profile and decoded fullscreen avatar stay usable in H/V', (
    tester,
  ) async {
    final unexpected = <String>[];
    final apiRequests = <String>[];
    final client = MockClient((request) async {
      expect(request.headers['authorization'], 'Bearer test-access');
      apiRequests.add(request.url.path);
      switch (request.url.path) {
        case '/api/v1/users/profiles/p-target':
          return _json({
            'profile': {
              'id': 'p-target',
              'account_id': 'a-target',
              'username': 'alex',
              'discriminator': '0001',
              'display_name': 'Alex',
              'is_primary': true,
              'avatar_url': _avatarUrl,
            },
          });
        case '/api/v1/spaces':
          expect(request.url.queryParameters, {'page_size': '100'});
          return _json({
            'space_list': {
              'spaces': [
                {
                  'id': 'common',
                  'name': 'Night Owls',
                  'visibility': 'private',
                  'owner_profile_id': 'prof-test',
                },
              ],
              'next_cursor': '',
            },
          });
        case '/api/v1/spaces/common/members':
          expect(request.url.queryParameters, {'page_size': '100'});
          return _json({
            'space_member_list': {
              'members': [
                {
                  'profile_id': 'p-target',
                  'role_names': ['Member'],
                  'joined_at': '2026-10-14T00:00:00Z',
                },
              ],
              'next_cursor': '',
            },
          });
        default:
          unexpected.add('${request.method} ${request.url}');
          throw StateError('Unexpected HTTP request');
      }
    });
    final container = _container(
      client,
      realCommonSpaces: true,
      realProfile: true,
    );
    final imageClient = _ImageClient(await _avatarBytes(tester));
    final previousImageClient = debugNetworkImageHttpClientProvider;
    final previousShadows = debugDisableShadows;
    final theme = await _captureTheme(tester);
    debugNetworkImageHttpClientProvider = () => imageClient;
    debugDisableShadows = false;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(() => tester.binding.setSurfaceSize(null));
    try {
      for (final size in [const Size(1280, 800), const Size(390, 844)]) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        await tester.pumpWidget(_app(container, theme: theme));
        // Decode the actual network ImageProvider before capture, with a bounded
        // public image fixture. This does not replace the production renderer.
        await tester.runAsync(
          () => precacheImage(
            const NetworkImage(_avatarUrl),
            tester.element(find.byKey(const Key('open_contact_profile'))),
          ),
        );
        await tester.tap(find.byKey(const Key('open_contact_profile')));
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('profile_common_spaces')), findsOneWidget);
        expect(
          find.byKey(const Key('profile_common_spaces_error')),
          findsNothing,
        );
        expect(
          tester
              .widget<OutlinedButton>(find.byKey(ProfileDetailSheet.callKey))
              .onPressed,
          isNotNull,
        );
        for (final key in [
          ProfileDetailSheet.avatarKey,
          ProfileDetailSheet.callKey,
          const ValueKey('profile_common_space_common'),
        ]) {
          final rect = tester.getRect(find.byKey(key));
          expect(rect.left, greaterThanOrEqualTo(0));
          expect(rect.right, lessThanOrEqualTo(size.width));
          expect(rect.bottom, lessThanOrEqualTo(size.height));
        }
        final orientation = size.width > size.height ? 'h' : 'v';
        await _capture(tester, 'contact-profile-$orientation.png');
        await tester.tap(find.byKey(ProfileDetailSheet.avatarKey));
        await tester.pumpAndSettle();
        expect(
          find.byKey(const Key('profile_avatar_fullscreen')),
          findsOneWidget,
        );
        expect(
          find.byKey(const Key('profile_avatar_load_error')),
          findsNothing,
        );
        final image = find.descendant(
          of: find.byKey(const Key('profile_avatar_fullscreen')),
          matching: find.byType(RawImage),
        );
        expect(tester.widget<RawImage>(image).image?.width, 256);
        expect(tester.widget<RawImage>(image).fit, BoxFit.contain);
        expect(
          tester.getRect(find.byKey(const Key('profile_avatar_fullscreen'))),
          Offset.zero & size,
        );
        await _capture(tester, 'contact-avatar-$orientation.png');
        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();
        expect(
          find.byKey(const Key('profile_avatar_fullscreen')),
          findsNothing,
        );
        expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();
        expect(find.byKey(ProfileDetailSheet.sheetKey), findsNothing);
      }
      expect(
        apiRequests,
        containsAll([
          '/api/v1/users/profiles/p-target',
          '/api/v1/spaces',
          '/api/v1/spaces/common/members',
        ]),
      );
      expect(unexpected, isEmpty);
      expect(
        imageClient.requests.every((uri) => uri.toString() == _avatarUrl),
        isTrue,
      );
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
    } finally {
      debugDisableShadows = previousShadows;
      debugNetworkImageHttpClientProvider = previousImageClient;
      PaintingBinding.instance.imageCache.clear();
      PaintingBinding.instance.imageCache.clearLiveImages();
      container.dispose();
      client.close();
    }
  });

  testWidgets(
    'failed avatar has an explicit retry and recovers the decoded image',
    (tester) async {
      final client = MockClient(
        (request) async => throw StateError('Unexpected $request'),
      );
      final container = _container(
        client,
        overrides: [
          profileProvider.overrideWith(
            (ref, id) async => const VoiceProfile(
              id: 'p-target',
              accountId: 'a-target',
              username: 'alex',
              discriminator: '0001',
              displayName: 'Alex',
              isPrimary: true,
              avatarUrl: _avatarUrl,
            ),
          ),
        ],
      );
      final imageClient = _ImageClient(await _avatarBytes(tester))..fail = true;
      final previous = debugNetworkImageHttpClientProvider;
      debugNetworkImageHttpClientProvider = () => imageClient;
      try {
        await _open(tester, container);
        await tester.tap(find.byKey(ProfileDetailSheet.avatarKey));
        await tester.pumpAndSettle();
        final error = find.byKey(const Key('profile_avatar_load_error'));
        expect(error, findsOneWidget);
        imageClient.fail = false;
        await tester.tap(
          find.descendant(
            of: error,
            matching: find.text(
              AppLocalizations.of(tester.element(error))!.commonRetry,
            ),
          ),
        );
        await tester.pump();
        await tester.runAsync(
          () => precacheImage(
            const NetworkImage(_avatarUrl),
            tester.element(find.byKey(const Key('profile_avatar_fullscreen'))),
          ),
        );
        await tester.pumpAndSettle();
        expect(error, findsNothing);
        final image = find.byType(RawImage);
        expect(tester.widget<RawImage>(image).image?.height, 256);
        expect(imageClient.requests.length, greaterThanOrEqualTo(2));
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pumpAndSettle();
      } finally {
        debugNetworkImageHttpClientProvider = previous;
        PaintingBinding.instance.imageCache.clear();
        PaintingBinding.instance.imageCache.clearLiveImages();
        container.dispose();
        client.close();
      }
    },
  );

  for (final state in ['populated', 'absent', 'failed']) {
    testWidgets('public $state banner keeps profile and avatar usable in H/V', (
      tester,
    ) async {
      final apiRequests = <String>[];
      final client = MockClient((request) async {
        apiRequests.add(request.url.path);
        expect(request.headers['authorization'], 'Bearer test-access');
        expect(request.url.path, '/api/v1/users/profiles/p-target');
        return _json({
          'profile': {
            'id': 'p-target',
            'account_id': 'a-target',
            'username': 'alex',
            'discriminator': '0001',
            'display_name': 'Alex',
            if (state != 'absent') 'banner_url': _bannerUrl,
          },
        });
      });
      final container = _container(client, realProfile: true);
      final imageClient = _ImageClient(
        await _avatarBytes(tester),
        expectedUrl: _bannerUrl,
      )..fail = state == 'failed';
      final previousImageClient = debugNetworkImageHttpClientProvider;
      final theme = await _captureTheme(tester);
      debugNetworkImageHttpClientProvider = () => imageClient;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(() => tester.binding.setSurfaceSize(null));
      try {
        for (final size in [const Size(1280, 800), const Size(390, 844)]) {
          tester.view.physicalSize = size;
          tester.view.devicePixelRatio = 1;
          await tester.binding.setSurfaceSize(size);
          await tester.pumpWidget(_app(container, theme: theme));
          if (state == 'populated') {
            await tester.runAsync(
              () => precacheImage(
                const NetworkImage(_bannerUrl),
                tester.element(find.byKey(const Key('open_contact_profile'))),
              ),
            );
          }
          await tester.tap(find.byKey(const Key('open_contact_profile')));
          await tester.pumpAndSettle();
          final banner = find.byKey(const Key('profile_banner'));
          final avatar = find.byKey(ProfileDetailSheet.avatarKey);
          if (state == 'absent') {
            expect(banner, findsNothing);
            expect(imageClient.requests, isEmpty);
          } else {
            expect(banner, findsOneWidget);
            final bannerRect = tester.getRect(banner);
            final avatarRect = tester.getRect(avatar);
            expect(bannerRect.height, size.width > size.height ? 96 : 112);
            expect(avatarRect.size, const Size(80, 80));
            expect(avatarRect.top, lessThan(bannerRect.bottom));
            expect(avatarRect.bottom - bannerRect.bottom, closeTo(32, 0.01));
            expect(avatarRect.left - bannerRect.left, closeTo(20, 0.01));
            final image = find.descendant(
              of: banner,
              matching: find.byType(RawImage),
            );
            if (state == 'populated') {
              expect(tester.widget<RawImage>(image).image?.width, 256);
              expect(tester.widget<RawImage>(image).fit, BoxFit.cover);
            } else {
              expect(
                find.byKey(const Key('profile_banner_unavailable')),
                findsOneWidget,
              );
            }
            expect(bannerRect.left, greaterThanOrEqualTo(0));
            expect(bannerRect.right, lessThanOrEqualTo(size.width));
          }
          expect(find.text('Alex'), findsOneWidget);
          expect(
            tester.widget<OutlinedButton>(
              find.byKey(ProfileDetailSheet.messageKey),
            ).onPressed,
            isNotNull,
          );
          expect(
            tester.widget<OutlinedButton>(
              find.byKey(ProfileDetailSheet.callKey),
            ).onPressed,
            isNotNull,
          );
          final orientation = size.width > size.height ? 'h' : 'v';
          await _capture(tester, 'contact-banner-$state-$orientation.png');
          await tester.tap(avatar);
          await tester.pumpAndSettle();
          expect(
            find.byKey(const Key('profile_avatar_missing')),
            findsOneWidget,
          );
          await tester.sendKeyEvent(LogicalKeyboardKey.escape);
          await tester.pumpAndSettle();
          expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
          await tester.sendKeyEvent(LogicalKeyboardKey.escape);
          await tester.pumpAndSettle();
          expect(tester.takeException(), isNull);
        }
        expect(apiRequests, isNotEmpty);
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pumpAndSettle();
      } finally {
        debugNetworkImageHttpClientProvider = previousImageClient;
        PaintingBinding.instance.imageCache.clear();
        PaintingBinding.instance.imageCache.clearLiveImages();
        container.dispose();
        client.close();
      }
    });
  }
}

ProviderContainer _container(
  http.Client client, {
  List<Override> overrides = const [],
  bool realCommonSpaces = false,
  bool realProfile = false,
}) => ProviderContainer(
  overrides: [
    ...voiceAppTestOverrides(client: client),
    voiceChatsClientProvider.overrideWithValue(_HttpDmChatsClient(client)),
    presenceProvider.overrideWith(
      (ref, id) => VoicePresence(profileId: id, status: 'online'),
    ),
    if (!realProfile) profileProvider.overrideWith((ref, id) async => _profile),
    playerProfileProvider.overrideWith(
      (ref, id) async => const PlayerProfileData(entries: []),
    ),
    gameCatalogProvider.overrideWith(
      (ref) async => const GameListData(games: []),
    ),
    activeStoryAuthorIdsProvider.overrideWith((ref) => const {}),
    profileStoriesProvider.overrideWith((ref, id) async => const []),
    profileHighlightsProvider.overrideWith((ref, id) async => const []),
    friendRequestsProvider.overrideWith(
      (ref) async => const FriendRequestsData(incoming: [], outgoing: []),
    ),
    isFriendProvider.overrideWith((ref, id) => false),
    profileContactMembershipProvider.overrideWith((ref, id) async => true),
    dmPermissionProvider.overrideWith((ref, request) async => true),
    if (!realCommonSpaces)
      contactCommonSpacesProvider.overrideWith((ref, id) async => const []),
    profileBoundRealtimeEventProvider.overrideWith(
      (ref) => const Stream.empty(),
    ),
    ...overrides,
  ],
);

class _HttpDmChatsClient extends FakeVoiceChatsClient {
  _HttpDmChatsClient(http.Client client)
    : _http = VoiceChatsClient(gateway: gatewayHttpForTest(client));
  final VoiceChatsClient _http;
  int inboxReads = 0;

  @override
  Future<ChatsApiResult<ChatListData>> listChats({
    required String authorization,
    String? cursor,
    int? pageSize,
    String? inbox,
    String? folderId,
  }) {
    inboxReads++;
    return super.listChats(
      authorization: authorization,
      cursor: cursor,
      pageSize: pageSize,
      inbox: inbox,
      folderId: folderId,
    );
  }

  @override
  Future<ChatsApiResult<VoiceChat>> createDm({
    required String authorization,
    required String otherProfileId,
  }) => _http.createDm(
    authorization: authorization,
    otherProfileId: otherProfileId,
  );
}

Future<void> _open(WidgetTester tester, ProviderContainer container) async {
  await tester.pumpWidget(_app(container));
  await tester.tap(find.byKey(const Key('open_contact_profile')));
  await tester.pumpAndSettle();
}

const _commonSpace = VoiceSpace(
  id: 'common',
  name: 'Night Owls',
  visibility: 'private',
  ownerProfileId: 'prof-test',
);

http.Response _json(Map<String, Object?> body, {int status = 200}) =>
    utf8JsonResponse(jsonEncode(body), status: status);
http.Response _dmResponse() => _json({
  'chat': {
    'id': 'dm-contact',
    'type': 'CHAT_TYPE_DM',
    'creator_profile_id': 'prof-test',
  },
});

const _profile = VoiceProfile(
  id: 'p-target',
  accountId: 'a-target',
  username: 'alex',
  discriminator: '0001',
  displayName: 'Alex',
  isPrimary: true,
);

Widget _app(
  ProviderContainer container, {
  ThemeData? theme,
  Locale? locale,
  bool plainModal = false,
  double textScale = 1,
}) =>
    RepaintBoundary(
      key: const Key('contact_profile_capture_root'),
      child: UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: theme ?? voiceTestTheme(),
          locale: locale,
          debugShowCheckedModeBanner: false,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(context).copyWith(
              textScaler: TextScaler.linear(textScale),
            ),
            child: child!,
          ),
          home: Builder(
            builder: (context) => Scaffold(
              body: TextButton(
                key: const Key('open_contact_profile'),
                onPressed: () {
                  const sheet = ProfileDetailSheet(profileId: 'p-target');
                  if (plainModal) {
                    showModalBottomSheet<void>(
                      context: context,
                      isScrollControlled: true,
                      builder: (_) => sheet,
                    );
                  } else {
                    showVoiceBottomSheet<void>(context: context, child: sheet);
                  }
                },
                child: const Text('Open profile'),
              ),
            ),
          ),
        ),
      ),
    );

const _avatarUrl = 'https://public-images.test/alex.png';
const _bannerUrl = 'https://public-images.test/alex-banner.png';
const _captureDirectory = 'VOICE_CONTACT_PROFILE_CAPTURE_DIR';
var _captureFontsLoaded = false;

Future<Uint8List> _avatarBytes(WidgetTester tester) async {
  final bytes = await tester.runAsync(() async {
    final recorder = ui.PictureRecorder();
    final canvas = Canvas(recorder);
    canvas.drawRect(
      const Rect.fromLTWH(0, 0, 256, 256),
      Paint()..color = const Color(0xff173e56),
    );
    canvas.drawCircle(
      const Offset(128, 96),
      48,
      Paint()..color = const Color(0xfff2bf88),
    );
    canvas.drawOval(
      const Rect.fromLTWH(40, 154, 176, 160),
      Paint()..color = const Color(0xff3bc5ce),
    );
    final picture = recorder.endRecording();
    final image = await picture.toImage(256, 256);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      if (data == null) throw StateError('Avatar fixture encoding failed');
      return data.buffer.asUint8List();
    } finally {
      image.dispose();
      picture.dispose();
    }
  });
  if (bytes == null) throw StateError('Avatar fixture rendering failed');
  return bytes;
}

class _ImageClient implements HttpClient {
  _ImageClient(this.bytes, {this.expectedUrl = _avatarUrl});
  final Uint8List bytes;
  final String expectedUrl;
  final requests = <Uri>[];
  bool fail = false;

  @override
  Future<HttpClientRequest> getUrl(Uri url) async {
    expect(url.toString(), expectedUrl);
    requests.add(url);
    return _ImageRequest(this);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) =>
      throw UnsupportedError('$invocation');
}

class _ImageRequest implements HttpClientRequest {
  _ImageRequest(this.client);
  final _ImageClient client;

  @override
  Future<HttpClientResponse> close() async =>
      _ImageResponse(client.bytes, client.fail ? 404 : 200);

  @override
  dynamic noSuchMethod(Invocation invocation) =>
      throw UnsupportedError('$invocation');
}

class _ImageResponse extends Stream<List<int>> implements HttpClientResponse {
  _ImageResponse(this.bytes, this.statusCode);
  final Uint8List bytes;

  @override
  final int statusCode;

  @override
  int get contentLength => bytes.length;

  @override
  HttpClientResponseCompressionState get compressionState =>
      HttpClientResponseCompressionState.notCompressed;

  @override
  StreamSubscription<List<int>> listen(
    void Function(List<int>)? onData, {
    Function? onError,
    void Function()? onDone,
    bool? cancelOnError,
  }) => Stream<List<int>>.value(bytes).listen(
    onData,
    onError: onError,
    onDone: onDone,
    cancelOnError: cancelOnError,
  );

  @override
  dynamic noSuchMethod(Invocation invocation) =>
      throw UnsupportedError('$invocation');
}

Future<ThemeData> _captureTheme(WidgetTester tester) async {
  if (Platform.environment[_captureDirectory]?.isNotEmpty != true) {
    return voiceTestTheme();
  }
  if (!_captureFontsLoaded) {
    final loaded = await tester.runAsync(() async {
      final noto = FontLoader(VoiceTheme.fontFamily)
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
      await noto.load();
      final flutterRoot = Platform.environment['FLUTTER_ROOT'];
      if (flutterRoot == null || flutterRoot.isEmpty) {
        throw StateError('FLUTTER_ROOT is required to load Material Icons');
      }
      final bytes = await File(
        '$flutterRoot/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
      ).readAsBytes();
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future.value(ByteData.sublistView(bytes)))).load();
      return true;
    });
    if (loaded != true) throw StateError('Production font loading failed');
    _captureFontsLoaded = true;
  }
  final theme = await tester.runAsync(() async {
    final catalog = await VoiceTokenCatalog.load();
    return VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );
  });
  if (theme == null) throw StateError('Production tokens did not load');
  return theme;
}

Future<void> _capture(WidgetTester tester, String name) async {
  final directory = Platform.environment[_captureDirectory];
  if (directory == null || directory.isEmpty) return;
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(const Key('contact_profile_capture_root')),
  );
  final written = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
      if (bytes == null) throw StateError('PNG encoding returned null');
      final output = File('$directory/$name');
      await output.parent.create(recursive: true);
      await output.writeAsBytes(bytes.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (written != true) throw StateError('ContactProfile capture failed');
}
