import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/friends_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/backend/user_privacy_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test('guest restrictions: DM, friends, space denied via gateway', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final guestResult = await ctx.authClient().registerGuest(password: qaPassword);
    expect(guestResult, isA<AuthSessionOk>());
    final guest = (guestResult as AuthSessionOk).session;

    final regular = await ctx.registerUser('guest-restrict-regular');

    final dm = await ctx.chatsClient().createDm(
      authorization: guest.authorizationHeader,
      otherProfileId: regular.activeProfileId,
    );
    expect(dm, isA<ChatsApiFailure>());
    expect((dm as ChatsApiFailure).statusCode, 403);

    final invite = await ctx.friendsClient().sendFriendInvitation(
      authorization: guest.authorizationHeader,
      targetProfileId: regular.activeProfileId,
    );
    expect(invite, isA<FriendsApiFailure>());
    expect((invite as FriendsApiFailure).statusCode, 403);

    final space = await ctx.spacesClient().createSpace(
      authorization: guest.authorizationHeader,
      name: 'guest-space',
      description: 'denied',
    );
    expect(space, isA<SpacesApiFailure>());
    expect((space as SpacesApiFailure).statusCode, 403);
  }, skip: runLiveIntegration
      ? null
      : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true');

  test('guest dm reply: blocked when allow_guest_dm false, allowed when true', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final guestResult = await ctx.authClient().registerGuest(password: qaPassword);
    expect(guestResult, isA<AuthSessionOk>());
    final guest = (guestResult as AuthSessionOk).session;

    final regular = await ctx.registerUser('guest-dm-regular');
    final privacy = VoiceUserPrivacyClient(gateway: ctx.gatewayHttp());
    final currentPrivacy = await privacy.getPrivacy(
      authorization: regular.authorizationHeader,
    );
    expect(currentPrivacy, isA<UserPrivacyApiOk<VoicePrivacySettings>>());
    final base = (currentPrivacy as UserPrivacyApiOk<VoicePrivacySettings>).data;

    final setupPrivacy = await privacy.updatePrivacy(
      authorization: regular.authorizationHeader,
      settings: base.copyWith(
        allowDm: VoicePrivacyAudience.everyoneWithGuests,
        allowGuestDm: false,
      ),
    );
    expect(setupPrivacy, isA<UserPrivacyApiOk<VoicePrivacySettings>>());

    final chats = ctx.chatsClient();
    final dm = await chats.createDm(
      authorization: regular.authorizationHeader,
      otherProfileId: guest.activeProfileId,
    );
    expect(dm, isA<ChatsApiOk<VoiceChat>>());
    final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

    final messages = ctx.messagesClient();
    final blockedSend = await messages.sendMessage(
      authorization: guest.authorizationHeader,
      chatId: chatId,
      content: 'guest reply should fail',
      clientMessageId: qaClientMessageId(),
    );
    expect(blockedSend, isA<MessagesApiFailure>());
    expect((blockedSend as MessagesApiFailure).statusCode, 403);

    final allowPrivacy = await privacy.updatePrivacy(
      authorization: regular.authorizationHeader,
      settings: base.copyWith(
        allowDm: VoicePrivacyAudience.everyoneWithGuests,
        allowGuestDm: true,
      ),
    );
    expect(allowPrivacy, isA<UserPrivacyApiOk<VoicePrivacySettings>>());

    final allowedSend = await messages.sendMessage(
      authorization: guest.authorizationHeader,
      chatId: chatId,
      content: 'guest reply allowed',
      clientMessageId: qaClientMessageId(),
    );
    expect(allowedSend, isA<MessagesApiOk<VoiceMessage>>());
  }, skip: runLiveIntegration
      ? null
      : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true');
}
