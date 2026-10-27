import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/routing/deep_link_controller.dart';
import 'package:voice_frontend/routing/deep_link_listener.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/deep_link_navigation.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  for (final phase in [
    'before callback',
    'during resolve',
    'during initial route',
  ]) {
    final duringResolve = phase != 'before callback';
    final initialRoute = phase == 'during initial route';
    testWidgets(
      'newer deep link is delivered without another dependency change '
      '($phase)',
      (tester) async {
        const voipChannel = MethodChannel('voice/voip');
        tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
          voipChannel,
          (_) async => null,
        );
        addTearDown(
          () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
            voipChannel,
            null,
          ),
        );
        tester.binding.platformDispatcher.defaultRouteNameTestValue =
            initialRoute ? '/u/older' : '/';
        addTearDown(
          tester.binding.platformDispatcher.clearDefaultRouteNameTestValue,
        );
        final older = parseDeepLinkUrl(
          initialRoute
              ? 'https://voice.gg/u/older?'
              : 'https://voice.gg/u/older',
        );
        final newer = parseDeepLinkUrl('https://voice.gg/dm/newer');
        final firstResolve = Completer<http.Response>();
        final requested = <String>[];
        final applied = <DeepLinkTarget>[];
        final container = ProviderContainer(
          overrides: [
            ...guestShellTestOverrides(
              client: MockClient((request) async {
                if (request.url.path == '/api/v1/links/resolve') {
                  requested.add(request.url.queryParameters['url']!);
                  if (duringResolve && requested.length == 1) {
                    return firstResolve.future;
                  }
                }
                return http.Response('unavailable', 503);
              }),
            ),
            deepLinkBaseUriProvider.overrideWithValue(
              Uri.parse('https://voice.gg/'),
            ),
            deepLinkNavigatorProvider.overrideWith(
              (ref) => _RecordingNavigator(
                ref,
                applied: applied,
                release: Future.value(),
              ),
            ),
          ],
        );
        addTearDown(container.dispose);
        await container.read(voiceMaterialThemeProvider.future);
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: const VoiceApp(locale: Locale('en')),
          ),
        );
        await tester.pumpAndSettle();
        final shell = tester.state<State>(
          find.byWidgetPredicate(
            (widget) => widget.runtimeType.toString() == '_AuthenticatedShell',
          ),
        );
        final links = container.read(deepLinkControllerProvider.notifier);
        if (!initialRoute) {
          await links.onIncomingLink(older);
          // Invoke the real lifecycle callback before pumping the queued frame.
          // ignore: invalid_use_of_protected_member
          shell.didChangeDependencies();
        }
        if (duringResolve) {
          await tester.pumpAndSettle();
          expect(requested, [older.rawUrl]);
          expect(applied, isEmpty);
        }
        await links.onIncomingLink(newer);
        if (duringResolve) {
          firstResolve.complete(http.Response('unavailable', 503));
        } else {
          // Drive the actual lifecycle twice between frames: the older target's
          // post-frame callback is queued, then a newer delivery arrives.
          // ignore: invalid_use_of_protected_member
          shell.didChangeDependencies();
        }
        await tester.pumpAndSettle();

        expect(
          requested,
          duringResolve ? [older.rawUrl, newer.rawUrl] : [newer.rawUrl],
        );
        expect(applied, duringResolve ? [older, newer] : [newer]);
        expect(container.read(deepLinkControllerProvider).resolved, isNull);
        await tester.pumpWidget(const SizedBox.shrink());
      },
    );
  }

  for (final path in ['/u/vanya', '/dm/profile-2', '/invite/join-me']) {
    for (final overlap in [false, true]) {
      for (final retained in [true, false]) {
        testWidgets('initial $path applies once with delayed listener '
            '(overlap: $overlap, retained: $retained)', (tester) async {
          const voipChannel = MethodChannel('voice/voip');
          tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
            voipChannel,
            (_) async => null,
          );
          addTearDown(
            () => tester.binding.defaultBinaryMessenger
                .setMockMethodCallHandler(voipChannel, null),
          );
          tester.binding.platformDispatcher.defaultRouteNameTestValue = path;
          addTearDown(
            tester.binding.platformDispatcher.clearDefaultRouteNameTestValue,
          );
          final uri = Uri.parse('https://voice.gg$path');
          final target = parseInitialWebDeepLinkTarget(uri)!;
          final delivered = retained
              ? target
              : parseDeepLinkUrl(uri.replace(query: '').toString());
          final listenerReady = Completer<void>();
          final listenerRelease = Completer<void>();
          final applyRelease = Completer<void>();
          var resolves = 0;
          final applied = <DeepLinkTarget>[];
          final container = ProviderContainer(
            overrides: [
              ...voiceAppTestOverrides(
                client: MockClient((request) async {
                  if (request.url.path == '/api/v1/links/resolve') resolves++;
                  return http.Response('unavailable', 503);
                }),
              ),
              deepLinkBaseUriProvider.overrideWithValue(uri),
              onboardingControllerProvider.overrideWith(
                TestCompletedOnboardingController.new,
              ),
              deepLinkListenerProvider.overrideWith(
                (ref) => _DelayedListener(
                  ref,
                  started: listenerReady,
                  release: listenerRelease,
                  target: retained ? target : parseDeepLinkUrl('voice:/$path'),
                ),
              ),
              deepLinkNavigatorProvider.overrideWith(
                (ref) => _RecordingNavigator(
                  ref,
                  applied: applied,
                  release: overlap ? applyRelease.future : Future.value(),
                ),
              ),
            ],
          );
          addTearDown(container.dispose);
          final auth = container.read(authControllerProvider.notifier);
          final authenticated = auth.state;
          auth.state = const AuthState();
          final links = container.read(deepLinkControllerProvider.notifier);
          if (retained) {
            await links.onIncomingLink(target);
            expect(container.read(deepLinkControllerProvider).pending, target);
          }
          auth.state = authenticated;
          await links.flushPendingAfterAuth();
          await container.read(voiceMaterialThemeProvider.future);

          await tester.pumpWidget(
            UncontrolledProviderScope(
              container: container,
              child: const VoiceApp(locale: Locale('en')),
            ),
          );
          await tester.pumpAndSettle();
          expect(listenerReady.isCompleted, isTrue);
          expect(resolves, retained ? 1 : 0);
          expect(applied, retained ? [target] : isEmpty);

          // Releasing platform initial-link discovery must not replay the URL
          // already delivered from the auth gate, even after apply completed.
          listenerRelease.complete();
          await tester.pumpAndSettle();
          expect(resolves, 1);
          expect(applied, [delivered]);
          expect(container.read(deepLinkControllerProvider).resolved, isNull);

          if (overlap) applyRelease.complete();
          await tester.pumpAndSettle();

          // A later intentional delivery of the same URL remains navigable.
          await links.onIncomingLink(target);
          await tester.pumpWidget(
            UncontrolledProviderScope(
              container: container,
              child: const VoiceApp(locale: Locale('ru')),
            ),
          );
          await tester.pumpAndSettle();
          expect(resolves, 2);
          expect(applied, [delivered, target]);
          await tester.pumpWidget(const SizedBox.shrink());
        });
      }
    }
  }
}

class _DelayedListener extends DeepLinkListener {
  _DelayedListener(
    this.ref, {
    required this.started,
    required this.release,
    required this.target,
  }) : super(ref);

  final Ref ref;
  final Completer<void> started;
  final Completer<void> release;
  final DeepLinkTarget target;

  @override
  Future<void> start() async {
    started.complete();
    await release.future;
    await ref.read(deepLinkControllerProvider.notifier).onIncomingLink(target);
  }
}

class _RecordingNavigator extends DeepLinkNavigator {
  _RecordingNavigator(
    super.ref, {
    required this.applied,
    required this.release,
  });

  final List<DeepLinkTarget> applied;
  final Future<void> release;

  @override
  Future<void> apply(DeepLinkTarget target) async {
    applied.add(target);
    await release;
  }
}
