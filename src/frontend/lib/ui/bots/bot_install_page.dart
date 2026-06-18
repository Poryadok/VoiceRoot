import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/bot_scopes.dart';
import '../../backend/bots_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/bot_providers.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_avatar.dart';
import '../core/voice_state_panel.dart';

/// Public bot install page opened from `bots/{slug}` deep links.
class BotInstallPage extends ConsumerStatefulWidget {
  const BotInstallPage({super.key, required this.slug});

  static const Key pageKey = Key('bot_install_page');

  final String slug;

  @override
  ConsumerState<BotInstallPage> createState() => _BotInstallPageState();
}

class _BotInstallPageState extends ConsumerState<BotInstallPage> {
  String? _selectedSpaceId;
  final _selectedChatIds = <String>{};
  bool _busy = false;
  bool _privilegedAcknowledged = false;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final botAsync = ref.watch(botBySlugProvider(widget.slug));

    return Scaffold(
      key: BotInstallPage.pageKey,
      appBar: AppBar(
        title: Text(l10n.botInstallTitle),
      ),
      body: botAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => VoiceStatePanel(
          title: l10n.botInstallTitle,
          message: l10n.chatRoomError('$e'),
          icon: Icons.smart_toy_outlined,
        ),
        data: (bot) => SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  VoiceAvatar(
                    label: bot.name,
                    imageUrl: bot.avatarUrl,
                    radius: 28,
                  ),
                  const SizedBox(width: 16),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          bot.name,
                          style: Theme.of(context).textTheme.headlineSmall,
                        ),
                        if (bot.slug != null && bot.slug!.isNotEmpty)
                          Text(
                            '@${bot.slug}',
                            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                              color: Theme.of(context).colorScheme.onSurfaceVariant,
                            ),
                          ),
                      ],
                    ),
                  ),
                ],
              ),
              if (bot.description.isNotEmpty) ...[
                const SizedBox(height: 16),
                Text(
                  l10n.botInstallDescriptionHeading,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 8),
                Text(bot.description),
              ],
              const SizedBox(height: 16),
              Text(
                l10n.botInstallScopesHeading,
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 8),
              for (final scope in BotScopeLabels.parseScopesJson(bot.scopesJson))
                Text('• ${BotScopeLabels.labelFor(context, scope)}'),
              const SizedBox(height: 16),
              Text(
                l10n.botInstallCommandsHeading,
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 8),
              Text(
                l10n.botInstallCommandsEmpty,
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 16),
              Text(
                l10n.botInstallWhitelistHeading,
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 8),
              _SpaceInstallSection(
                bot: bot,
                selectedSpaceId: _selectedSpaceId,
                selectedChatIds: _selectedChatIds,
                privilegedAcknowledged: _privilegedAcknowledged,
                busy: _busy,
                onSpaceChanged: (spaceId) => setState(() {
                  _selectedSpaceId = spaceId;
                  _selectedChatIds.clear();
                  _privilegedAcknowledged = false;
                }),
                onChatToggled: (chatId, checked) => setState(() {
                  if (checked) {
                    _selectedChatIds.add(chatId);
                  } else {
                    _selectedChatIds.remove(chatId);
                  }
                }),
                onPrivilegedAckChanged: (value) =>
                    setState(() => _privilegedAcknowledged = value),
              ),
              const SizedBox(height: 16),
              FilledButton(
                key: const Key('bot_install_confirm'),
                style: FilledButton.styleFrom(
                  backgroundColor: voice.profileAccent,
                ),
                onPressed: _busy || !_canInstall(bot) ? null : () => _install(context, bot),
                child: Text(l10n.botInstallConfirm),
              ),
            ],
          ),
        ),
      ),
    );
  }

  bool _canInstall(VoiceBotSummary bot) {
    if (_selectedSpaceId == null || _selectedChatIds.isEmpty) return false;
    final scopes = BotScopeLabels.parseScopesJson(bot.scopesJson);
    final needsAck = scopes.any(
      (s) => BotScopeLabels.privilegedScopes.contains(s),
    );
    return !needsAck || _privilegedAcknowledged;
  }

  Future<void> _install(BuildContext context, VoiceBotSummary bot) async {
    final l10n = AppLocalizations.of(context)!;
    final auth = ref.read(authorizationHeaderProvider);
    final spaceId = _selectedSpaceId;
    if (auth == null || spaceId == null) return;

    final tree = await ref.read(spaceTreeProvider(spaceId).future);
    final allowedChats = tree.nodes
        .where(
          (n) =>
              n.isTextChat &&
              n.linkedChatId != null &&
              _selectedChatIds.contains(n.linkedChatId),
        )
        .map(
          (n) => (
            id: n.linkedChatId!,
            type: n.chatType ?? 'CHAT_TYPE_GROUP',
          ),
        )
        .toList();

    setState(() => _busy = true);
    final result = await ref.read(voiceBotsClientProvider).installBotInSpace(
      authorization: auth,
      botId: bot.id,
      spaceId: spaceId,
      allowedChats: allowedChats,
      acknowledgePrivilegedScopes: _privilegedAcknowledged,
    );
    if (!mounted) return;
    setState(() => _busy = false);

    switch (result) {
      case BotsApiOk():
        ref.invalidate(installedBotsProvider(spaceId));
        ref.invalidate(discoverableBotsProvider);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.spaceBotsInstallSuccess)),
        );
        Navigator.of(context).pop();
      case BotsApiFailure(:final message):
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.chatRoomError(message))),
        );
    }
  }
}

class _SpaceInstallSection extends ConsumerWidget {
  const _SpaceInstallSection({
    required this.bot,
    required this.selectedSpaceId,
    required this.selectedChatIds,
    required this.privilegedAcknowledged,
    required this.busy,
    required this.onSpaceChanged,
    required this.onChatToggled,
    required this.onPrivilegedAckChanged,
  });

  final VoiceBotSummary bot;
  final String? selectedSpaceId;
  final Set<String> selectedChatIds;
  final bool privilegedAcknowledged;
  final bool busy;
  final ValueChanged<String?> onSpaceChanged;
  final void Function(String chatId, bool checked) onChatToggled;
  final ValueChanged<bool> onPrivilegedAckChanged;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final spacesAsync = ref.watch(mySpacesProvider);
    final scopes = BotScopeLabels.parseScopesJson(bot.scopesJson);
    final needsAck = scopes.any(
      (s) => BotScopeLabels.privilegedScopes.contains(s),
    );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        spacesAsync.when(
          loading: () => const LinearProgressIndicator(),
          error: (e, _) => Text(l10n.chatRoomError('$e')),
          data: (spaces) {
            if (spaces.spaces.isEmpty) {
              return Text(l10n.botInstallNoSpaces);
            }
            return DropdownButtonFormField<String>(
              key: const Key('bot_install_space_picker'),
              value: selectedSpaceId,
              decoration: InputDecoration(
                labelText: l10n.botInstallSelectSpace,
              ),
              items: [
                for (final space in spaces.spaces)
                  DropdownMenuItem(
                    value: space.id,
                    child: Text(space.name),
                  ),
              ],
              onChanged: busy ? null : onSpaceChanged,
            );
          },
        ),
        if (selectedSpaceId != null) ...[
          const SizedBox(height: 12),
          if (needsAck) ...[
            CheckboxListTile(
              value: privilegedAcknowledged,
              onChanged: busy
                  ? null
                  : (v) => onPrivilegedAckChanged(v ?? false),
              title: Text(l10n.spaceBotsPrivilegedAck),
              subtitle: Text(l10n.spaceBotsScopeWarning),
              controlAffinity: ListTileControlAffinity.leading,
            ),
          ],
          Text(
            l10n.spaceBotsSelectChats,
            style: Theme.of(context).textTheme.titleSmall,
          ),
          ref.watch(spaceTreeProvider(selectedSpaceId!)).when(
            loading: () => const LinearProgressIndicator(),
            error: (e, _) => Text(l10n.chatRoomError('$e')),
            data: (tree) {
              final textChats = tree.nodes
                  .where((n) => n.isTextChat && n.linkedChatId != null)
                  .toList();
              if (textChats.isEmpty) {
                return Text(l10n.chatBotsEmpty);
              }
              return Column(
                children: [
                  for (final node in textChats)
                    CheckboxListTile(
                      key: Key('bot_install_chat_${node.linkedChatId}'),
                      title: Text(node.displayName),
                      value: selectedChatIds.contains(node.linkedChatId),
                      onChanged: busy
                          ? null
                          : (checked) =>
                              onChatToggled(node.linkedChatId!, checked == true),
                    ),
                ],
              );
            },
          ),
        ],
      ],
    );
  }
}
