import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/routing/deep_link_controller.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/deep_link_navigation.dart';
import 'package:voice_frontend/state/space_providers.dart';

void main() {
  test('pending invite link survives auth gate until login', () async {
    final container = ProviderContainer(
      overrides: [
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
      ],
    );
    addTearDown(container.dispose);

    final controller = container.read(deepLinkControllerProvider.notifier);
    const pending = DeepLinkTarget(
      kind: DeepLinkKind.invite,
      inviteCode: 'join-me',
      rawUrl: 'https://voice.gg/invite/join-me',
    );

    controller.onIncomingLink(pending);
    expect(container.read(deepLinkControllerProvider).pending, pending);
    expect(container.read(deepLinkControllerProvider).resolved, isNull);

    final auth = container.read(authControllerProvider.notifier);
    auth.state = const AuthState(
      session: AuthSession(
        accessToken: 'access',
        refreshToken: 'refresh',
        accountId: 'acc-1',
        activeProfileId: 'prof-1',
        expiresInSeconds: 900,
      ),
    );

    await controller.flushPendingAfterAuth();
    final state = container.read(deepLinkControllerProvider);
    expect(state.pending, isNull);
    expect(state.resolved?.kind, DeepLinkKind.invite);
    expect(state.resolved?.inviteCode, 'join-me');
  });

  test('web invite survives auth and guest nickname gate until join', () async {
    final inviteActions = _RecordingInviteActions();
    final container = ProviderContainer(
      overrides: [
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        spaceInviteActionsProvider.overrideWithValue(inviteActions),
      ],
    );
    addTearDown(container.dispose);

    final target = parseInitialWebInviteTarget(
      Uri.parse('https://app.comrade.click/invite/join-me'),
    );
    expect(target?.inviteCode, 'join-me');

    final links = container.read(deepLinkControllerProvider.notifier);
    await links.onIncomingLink(target!);
    expect(container.read(deepLinkControllerProvider).pending, target);

    final auth = container.read(authControllerProvider.notifier);
    auth.state = const AuthState(
      session: AuthSession(
        accessToken: 'access',
        refreshToken: 'refresh',
        accountId: 'acc-1',
        activeProfileId: 'prof-1',
        expiresInSeconds: 900,
        accountType: 'guest',
      ),
      isGuest: true,
      needsGuestNickname: true,
    );
    await links.flushPendingAfterAuth();

    final afterAuth = container.read(deepLinkControllerProvider).resolved;
    expect(afterAuth, target);
    expect(container.read(authControllerProvider).needsGuestNickname, isTrue);

    // The target stays queued while VoiceAppBootstrap shows GuestNicknameScreen.
    auth.state = auth.state.copyWith(needsGuestNickname: false);
    expect(container.read(deepLinkControllerProvider).resolved, target);

    await container.read(deepLinkNavigatorProvider).apply(afterAuth!);
    expect(inviteActions.joinedCode, 'join-me');
    expect(container.read(selectedSpaceIdProvider), 'joined-space');
  });

  test('incoming link while authenticated resolves immediately', () async {
    final container = ProviderContainer(
      overrides: [
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        authControllerProvider.overrideWith((ref) {
          final c = AuthController(
            authClient: ref.watch(voiceAuthClientProvider),
            storage: ref.watch(authSessionStorageProvider),
            guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
          );
          c.state = const AuthState(
            session: AuthSession(
              accessToken: 'access',
              refreshToken: 'refresh',
              accountId: 'acc-1',
              activeProfileId: 'prof-1',
              expiresInSeconds: 900,
            ),
          );
          return c;
        }),
      ],
    );
    addTearDown(container.dispose);

    final controller = container.read(deepLinkControllerProvider.notifier);
    const target = DeepLinkTarget(
      kind: DeepLinkKind.space,
      spaceId: '550e8400-e29b-41d4-a716-446655440000',
      rawUrl: 'voice://s/550e8400-e29b-41d4-a716-446655440000',
    );

    await controller.onIncomingLink(target);
    final state = container.read(deepLinkControllerProvider);
    expect(state.pending, isNull);
    expect(state.resolved, target);
  });
}

class _RecordingInviteActions extends SpaceInviteActions {
  _RecordingInviteActions() : super(_UnwiredRef());

  String? joinedCode;

  @override
  Future<({String? error, String? spaceId})> joinByInvite({
    required String code,
  }) async {
    joinedCode = code;
    return (error: null, spaceId: 'joined-space');
  }
}

class _UnwiredRef implements Ref {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}
