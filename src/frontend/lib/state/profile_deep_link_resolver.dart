import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/users_client.dart';
import '../routing/deep_link_parser.dart';
import 'auth_providers.dart';
import 'social_providers.dart';

Future<String?> resolveProfileIdForUsername(
  WidgetRef ref,
  String username,
) async {
  final auth = ref.read(authControllerProvider);
  if (!auth.isAuthenticated || auth.session == null) return null;
  final client = ref.read(voiceUsersClientProvider);
  final result = await client.searchProfiles(
    authorization: 'Bearer ${auth.session!.accessToken}',
    query: username,
    pageSize: 8,
  );
  if (result case UsersApiOk(:final data)) {
    for (final profile in data.profiles) {
      if (profile.username.toLowerCase() == username.toLowerCase()) {
        return profile.id;
      }
    }
    return data.profiles.firstOrNull?.id;
  }
  return null;
}

Future<String?> resolveProfileIdFromDeepLinkText(WidgetRef ref, String raw) async {
  try {
    final target = parseDeepLinkUrl(raw.trim());
    if (target.kind != DeepLinkKind.profile) return null;
    final username = target.username?.trim();
    if (username == null || username.isEmpty) return null;
    return resolveProfileIdForUsername(ref, username);
  } on DeepLinkParseException {
    return null;
  }
}
