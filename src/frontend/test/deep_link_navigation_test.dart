import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/state/deep_link_navigation.dart';

void main() {
  test('shareUrlForChat builds space message link', () {
    expect(
      shareUrlForChat(chatId: 'chat-1', spaceId: 'space-1', messageId: 'msg-1'),
      'https://voice.gg/s/space-1/c/chat-1/m/msg-1',
    );
  });

  test('parseDeepLinkUrl profile and dm kinds', () {
    final profile = parseDeepLinkUrl('https://voice.gg/u/alice');
    expect(profile.kind, DeepLinkKind.profile);
    expect(profile.username, 'alice');

    final dm = parseDeepLinkUrl('https://voice.gg/dm/user-1');
    expect(dm.kind, DeepLinkKind.dm);
    expect(dm.userId, 'user-1');
  });

  test('joining a space invite opens the joined space', () async {
    final container = ProviderContainer(
      overrides: [
        spaceInviteActionsProvider.overrideWithValue(_JoinedInviteActions()),
      ],
    );
    addTearDown(container.dispose);

    await container
        .read(deepLinkNavigatorProvider)
        .apply(
          const DeepLinkTarget(
            kind: DeepLinkKind.invite,
            rawUrl: 'https://voice.gg/invite/join-me',
            inviteCode: 'join-me',
          ),
        );

    expect(container.read(selectedSpaceIdProvider), 'joined-space');
  });
}

class _JoinedInviteActions extends SpaceInviteActions {
  _JoinedInviteActions() : super(_UnwiredRef());

  @override
  Future<({String? error, String? spaceId})> joinByInvite({
    required String code,
  }) async => (error: null, spaceId: 'joined-space');
}

class _UnwiredRef implements Ref {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}
