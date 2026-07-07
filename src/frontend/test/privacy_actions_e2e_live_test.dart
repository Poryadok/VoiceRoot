import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/files_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/backend/user_privacy_client.dart';
import 'package:voice_frontend/backend/voice_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'phase 11 privacy actions: calls, invites, attachments blocked for strangers',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final targetEmail = qaUniqueEmail('p11act-target');
      final targetAuth = ctx.authClient();
      final targetRegister = await targetAuth.register(
        email: targetEmail,
        password: qaPassword,
      );
      expect(targetRegister, isA<AuthSessionOk>());
      final target = (targetRegister as AuthSessionOk).session;

      final stranger = await ctx.registerUser('p11act-stranger');
      final groupOwner = await ctx.registerUser('p11act-gowner');
      final groupMember = await ctx.registerUser('p11act-gmember');
      final groupFiller = await ctx.registerUser('p11act-gfill');
      final spaceOwner = await ctx.registerUser('p11act-sowner');

      final privacy = VoiceUserPrivacyClient(gateway: ctx.gatewayHttp());
      final privacyUpdate = await privacy.updatePrivacy(
        authorization: target.authorizationHeader,
        settings: VoicePrivacySettings(
          profileId: target.activeProfileId,
          preset: 'personal',
          showOnline: VoicePrivacyAudience.friendsOnly,
          showGameStatus: VoicePrivacyAudience.friendsOnly,
          showMmRating: VoicePrivacyAudience.friendsOnly,
          showPhone: VoicePrivacyAudience.nobody,
          showStories: VoicePrivacyAudience.friendsOnly,
          allowDm: VoicePrivacyAudience.everyoneWithGuests,
          allowFriendRequests: VoicePrivacyAudience.everyoneWithGuests,
          allowGuestDm: false,
          allowPhoneSearch: VoicePrivacyAudience.friendsOnly,
          allowCalls: VoicePrivacyAudience.friendsOnly,
          allowChatSpaceInvites: VoicePrivacyAudience.friendsOnly,
          allowFiles: VoicePrivacyAudience.friendsOnly,
          allowVoiceMessages: VoicePrivacyAudience.friendsOnly,
        ),
      );
      expect(privacyUpdate, isA<UserPrivacyApiOk<VoicePrivacySettings>>());

      for (final user in [groupOwner, groupMember, groupFiller, spaceOwner]) {
        final current = await privacy.getPrivacy(
          authorization: user.authorizationHeader,
        );
        expect(current, isA<UserPrivacyApiOk<VoicePrivacySettings>>());
        final setupPrivacy = await privacy.updatePrivacy(
          authorization: user.authorizationHeader,
          settings: (current as UserPrivacyApiOk<VoicePrivacySettings>).data
              .copyWith(
            allowChatSpaceInvites: VoicePrivacyAudience.everyoneWithGuests,
          ),
        );
        expect(setupPrivacy, isA<UserPrivacyApiOk<VoicePrivacySettings>>());
      }

      final chats = ctx.chatsClient();
      final dm = await chats.createDm(
        authorization: stranger.authorizationHeader,
        otherProfileId: target.activeProfileId,
      );
      expect(dm, isA<ChatsApiOk<VoiceChat>>());
      final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

      final voice = VoiceCallsClient(gateway: ctx.gatewayHttp());
      final callAttempt = await voice.startCall(
        authorization: stranger.authorizationHeader,
        chatId: chatId,
        calleeProfileId: target.activeProfileId,
      );
      expect(callAttempt, isA<VoiceApiFailure>());

      final group = await chats.createGroup(
        authorization: groupOwner.authorizationHeader,
        name: 'Privacy actions QA',
      );
      expect(group, isA<ChatsApiOk<VoiceChat>>());
      final groupId = (group as ChatsApiOk<VoiceChat>).data.id;

      final seedMembers = await chats.addGroupMembers(
        authorization: groupOwner.authorizationHeader,
        chatId: groupId,
        profileIds: [
          groupMember.activeProfileId,
          groupFiller.activeProfileId,
        ],
      );
      expect(seedMembers, isA<ChatsApiOk<void>>());

      final inviteAttempt = await chats.addGroupMembers(
        authorization: groupMember.authorizationHeader,
        chatId: groupId,
        profileIds: [target.activeProfileId],
      );
      expect(inviteAttempt, isA<ChatsApiFailure>());

      final spaces = ctx.spacesClient();
      final space = await spaces.createSpace(
        authorization: spaceOwner.authorizationHeader,
        name: 'Privacy actions QA',
        description: 'phase 11 live',
      );
      expect(space, isA<SpacesApiOk<VoiceSpace>>());
      final spaceId = (space as SpacesApiOk<VoiceSpace>).data.id;

      final invite = await spaces.createInvite(
        authorization: spaceOwner.authorizationHeader,
        spaceId: spaceId,
      );
      expect(invite, isA<SpacesApiOk<SpaceInvite>>());
      final inviteCode = (invite as SpacesApiOk<SpaceInvite>).data.code;

      final joinAttempt = await spaces.joinByInvite(
        authorization: target.authorizationHeader,
        code: inviteCode,
      );
      expect(joinAttempt, isA<SpacesApiFailure>());

      if (await ctx.probeFileStorageAvailable(stranger)) {
        final bytes = Uint8List.fromList([0x68, 0x69]);
        final files = ctx.filesClient();
        final ticketResult = await files.requestUpload(
          authorization: stranger.authorizationHeader,
          originalName: 'privacy-blocked.txt',
          mimeType: 'text/plain',
          sizeBytes: bytes.length,
          chatId: chatId,
          chatType: 'CHAT_TYPE_DM',
        );
        expect(ticketResult, isA<FilesApiOk<FileUploadTicket>>());
        final ticket = (ticketResult as FilesApiOk<FileUploadTicket>).data;

        final put = await files.putBytes(
          uploadUrl: ticket.presignedPutUrl,
          bytes: bytes,
          mimeType: 'text/plain',
        );
        expect(put, isA<FilesApiOk<void>>());

        final confirm = await files.confirmUpload(
          authorization: stranger.authorizationHeader,
          fileId: ticket.fileId,
          bytes: bytes,
        );
        expect(confirm, isA<FilesApiOk<FileMetadataData>>());
        final fileType =
            (confirm as FilesApiOk<FileMetadataData>).data.fileType;

        final fileSend = await ctx.messagesClient().sendMessage(
          authorization: stranger.authorizationHeader,
          chatId: chatId,
          content: '',
          clientMessageId: qaClientMessageId(),
          attachments: MessageAttachment.listFromWire(jsonEncode([
            {
              'file_id': ticket.fileId,
              'type': fileType.trim().isEmpty ? 'file' : fileType.trim(),
            },
          ])),
        );
        expect(fileSend, isA<MessagesApiFailure>());

        final voiceBytes = Uint8List.fromList([0x4f, 0x67, 0x67]);
        final voiceTicketResult = await files.requestUpload(
          authorization: stranger.authorizationHeader,
          originalName: 'privacy-blocked.ogg',
          mimeType: 'audio/ogg',
          sizeBytes: voiceBytes.length,
          chatId: chatId,
          chatType: 'CHAT_TYPE_DM',
        );
        expect(voiceTicketResult, isA<FilesApiOk<FileUploadTicket>>());
        final voiceTicket =
            (voiceTicketResult as FilesApiOk<FileUploadTicket>).data;
        final voicePut = await files.putBytes(
          uploadUrl: voiceTicket.presignedPutUrl,
          bytes: voiceBytes,
          mimeType: 'audio/ogg',
        );
        expect(voicePut, isA<FilesApiOk<void>>());
        final voiceConfirm = await files.confirmUpload(
          authorization: stranger.authorizationHeader,
          fileId: voiceTicket.fileId,
          bytes: voiceBytes,
        );
        expect(voiceConfirm, isA<FilesApiOk<FileMetadataData>>());

        final voiceSend = await ctx.messagesClient().sendMessage(
          authorization: stranger.authorizationHeader,
          chatId: chatId,
          content: '',
          clientMessageId: qaClientMessageId(),
          attachments: MessageAttachment.listFromWire(jsonEncode([
            {'file_id': voiceTicket.fileId, 'type': 'voice_message'},
          ])),
        );
        expect(voiceSend, isA<MessagesApiFailure>());
      }
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
