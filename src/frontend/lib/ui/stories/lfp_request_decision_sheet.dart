import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/matchmaking_client.dart';
import '../../state/auth_providers.dart';
import '../../state/matchmaking_providers.dart';
import '../../theme/voice_colors.dart';

/// Author Accept/Decline overlay for LFP JOIN/INVITE (matchmaking.md Social Discovery).
class LfpRequestDecisionSheet extends ConsumerWidget {
  const LfpRequestDecisionSheet({
    super.key,
    required this.storyId,
    required this.responderProfileId,
    required this.responseType,
  });

  static const Key acceptKey = Key('lfp_request_accept');
  static const Key declineKey = Key('lfp_request_decline');

  final String storyId;
  final String responderProfileId;
  final String responseType;

  static Future<DecideLfpRequestData?> show(
    BuildContext context, {
    required String storyId,
    required String responderProfileId,
    required String responseType,
  }) {
    return showModalBottomSheet<DecideLfpRequestData>(
      context: context,
      builder: (_) => LfpRequestDecisionSheet(
        storyId: storyId,
        responderProfileId: responderProfileId,
        responseType: responseType,
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final voice = VoiceColors.of(context);
    final isJoin = responseType.toUpperCase() == 'JOIN';
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            isJoin ? 'LFP join request' : 'LFP party invite',
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  color: voice.textPrimary,
                ),
          ),
          const SizedBox(height: 16),
          FilledButton(
            key: acceptKey,
            onPressed: () => _decide(context, ref, 'ACCEPT'),
            child: const Text('Accept'),
          ),
          const SizedBox(height: 8),
          OutlinedButton(
            key: declineKey,
            onPressed: () => _decide(context, ref, 'DECLINE'),
            child: const Text('Decline'),
          ),
        ],
      ),
    );
  }

  Future<void> _decide(BuildContext context, WidgetRef ref, String decision) async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final result = await ref.read(voiceMatchmakingClientProvider).decideLfpRequest(
          authorization: auth,
          storyId: storyId,
          responderProfileId: responderProfileId,
          responseType: responseType,
          decision: decision,
        );
    if (!context.mounted) return;
    switch (result) {
      case MatchmakingApiOk(:final data):
        Navigator.of(context).pop(data);
      case MatchmakingApiFailure(:final message):
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
    }
  }
}
