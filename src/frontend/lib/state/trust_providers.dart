import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/moderation_client.dart';
import '../backend/user_privacy_client.dart';
import 'auth_providers.dart';

final voiceModerationClientProvider = Provider<VoiceModerationClient>((ref) {
  return VoiceModerationClient(gateway: ref.watch(gatewayHttpClientProvider));
});

final voiceUserPrivacyClientProvider = Provider<VoiceUserPrivacyClient>((ref) {
  return VoiceUserPrivacyClient(gateway: ref.watch(gatewayHttpClientProvider));
});
