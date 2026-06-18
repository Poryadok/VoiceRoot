import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/moderation_client.dart';
import 'package:voice_frontend/backend/user_privacy_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'phase 11 trust: privacy DM block, report accepted, 2FA login gate',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final stranger = await ctx.registerUser('p11-stranger');
      final targetEmail = qaUniqueEmail('p11-target');
      final targetAuth = ctx.authClient();
      final targetRegister = await targetAuth.register(
        email: targetEmail,
        password: qaPassword,
      );
      expect(targetRegister, isA<AuthSessionOk>());
      final target = (targetRegister as AuthSessionOk).session;

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
          allowDm: VoicePrivacyAudience.friendsOnly,
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

      final chats = ctx.chatsClient();
      final dmAttempt = await chats.createDm(
        authorization: stranger.authorizationHeader,
        otherProfileId: target.activeProfileId,
      );
      expect(dmAttempt, isA<ChatsApiFailure>());

      final moderation = VoiceModerationClient(gateway: ctx.gatewayHttp());
      final report = await moderation.createReport(
        authorization: stranger.authorizationHeader,
        targetType: 'user',
        targetId: target.activeProfileId,
        category: 'mm_toxic',
      );
      expect(report, isA<ModerationApiOk<ReportSubmission>>());
      expect(
        (report as ModerationApiOk<ReportSubmission>).data.reportId,
        isNotEmpty,
      );

      final auth = targetAuth;
      final enroll = await auth.enable2FA(
        session: target,
        password: qaPassword,
      );
      expect(enroll, isA<Enable2FAOk>());
      final enrollment = (enroll as Enable2FAOk).enrollment;
      expect(enrollment.totpUri, contains('otpauth://'));
      expect(enrollment.backupCodes.length, greaterThanOrEqualTo(8));

      final verified = await auth.verify2FA(
        session: target,
        totpCode: '000000',
      );
      expect(verified, isA<AuthSessionOk>());

      final loginNoTotp = await auth.login(
        email: targetEmail,
        password: qaPassword,
      );
      expect(loginNoTotp, isA<AuthSessionFailure>());
      expect(
        (loginNoTotp as AuthSessionFailure).errorCode,
        'totp_required',
      );

      final loginBackup = await auth.login(
        email: targetEmail,
        password: qaPassword,
        totpCode: enrollment.backupCodes.first,
      );
      expect(loginBackup, isA<AuthSessionOk>());
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
