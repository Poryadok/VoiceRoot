import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/bot_scopes.dart';
import '../../backend/bots_client.dart';
import '../../backend/space_permissions.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/bot_providers.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_state_panel.dart';

/// Install / uninstall bots in a space — docs/features/bots.md.
class SpaceBotsSheet extends ConsumerStatefulWidget {
  const SpaceBotsSheet({super.key, required this.spaceId});

  final String spaceId;

  static Future<void> show(BuildContext context, {required String spaceId}) {
    return showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (_) => SpaceBotsSheet(spaceId: spaceId),
    );
  }

  @override
  ConsumerState<SpaceBotsSheet> createState() => _SpaceBotsSheetState();
}

class _SpaceBotsSheetState extends ConsumerState<SpaceBotsSheet> {
  String? _selectedBotId;
  final _selectedChatIds = <String>{};
  bool _busy = false;
  bool _privilegedAcknowledged = false;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final canManage = ref
            .watch(
              spacePermissionProvider((
                spaceId: widget.spaceId,
                permission: SpacePermissions.spaceManageBots,
                chatId: null,
                voiceRoomId: null,
              )),
            )
            .valueOrNull ??
        false;

    if (!canManage) {
      return SafeArea(
        child: VoiceStatePanel(
          title: l10n.spaceBotsTitle,
          message: l10n.chatRoomError('SPACE_MANAGE_BOTS required'),
          icon: Icons.lock_outlined,
        ),
      );
    }

    final installedAsync = ref.watch(installedBotsProvider(widget.spaceId));
    final discoverAsync = ref.watch(discoverableBotsProvider);
    final treeAsync = ref.watch(spaceTreeProvider(widget.spaceId));

    return SafeArea(
      child: Padding(
        padding: EdgeInsets.only(
          bottom: MediaQuery.viewInsetsOf(context).bottom,
        ),
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                l10n.spaceBotsTitle,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: 16),
              Text(
                l10n.spaceBotsInstall,
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 8),
              discoverAsync.when(
                loading: () => const LinearProgressIndicator(),
                error: (e, _) => Text(l10n.chatRoomError('$e')),
                data: (bots) {
                  if (bots.isEmpty) {
                    return Text(l10n.chatBotsEmpty);
                  }
                  return DropdownButtonFormField<String>(
                    key: const Key('space_bots_install_picker'),
                    value: _selectedBotId,
                    decoration: InputDecoration(
                      labelText: l10n.spaceBotsInstall,
                    ),
                    items: [
                      for (final bot in bots)
                        DropdownMenuItem(
                          value: bot.id,
                          child: Text(bot.name),
                        ),
                    ],
                    onChanged: _busy
                        ? null
                        : (v) => setState(() {
                            _selectedBotId = v;
                            _selectedChatIds.clear();
                            _privilegedAcknowledged = false;
                          }),
                  );
                },
              ),
              if (_selectedBotId != null) ...[
                const SizedBox(height: 12),
                _ScopeList(
                  botId: _selectedBotId!,
                  onPrivilegedDetected: (hasPrivileged) {
                    if (!hasPrivileged && _privilegedAcknowledged) {
                      setState(() => _privilegedAcknowledged = false);
                    }
                  },
                ),
                _PrivilegedAckCheckbox(
                  botId: _selectedBotId!,
                  acknowledged: _privilegedAcknowledged,
                  onChanged: _busy
                      ? null
                      : (v) => setState(() => _privilegedAcknowledged = v ?? false),
                ),
                const SizedBox(height: 12),
                Text(
                  l10n.spaceBotsSelectChats,
                  style: Theme.of(context).textTheme.titleSmall,
                ),
                treeAsync.when(
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
                            key: Key('space_bot_chat_${node.linkedChatId}'),
                            title: Text(node.displayName),
                            value: _selectedChatIds.contains(node.linkedChatId),
                            onChanged: _busy
                                ? null
                                : (checked) {
                                    setState(() {
                                      if (checked == true) {
                                        _selectedChatIds.add(node.linkedChatId!);
                                      } else {
                                        _selectedChatIds.remove(node.linkedChatId);
                                      }
                                    });
                                  },
                          ),
                      ],
                    );
                  },
                ),
                const SizedBox(height: 12),
                FilledButton(
                  key: const Key('space_bots_install_confirm'),
                  style: FilledButton.styleFrom(
                    backgroundColor: voice.profileAccent,
                  ),
                  onPressed: _busy ||
                          _selectedBotId == null ||
                          !_canInstall(ref)
                      ? null
                      : () => _install(context),
                  child: Text(l10n.spaceBotsInstallConfirm),
                ),
              ],
              const Divider(height: 32),
              Text(
                l10n.chatBotsSectionTitle,
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 8),
              installedAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (e, _) => Text(l10n.chatRoomError('$e')),
                data: (installed) {
                  if (installed.isEmpty) {
                    return Text(l10n.chatBotsEmpty);
                  }
                  return Column(
                    children: [
                      for (final entry in installed)
                        _InstalledBotTile(
                          entry: entry,
                          busy: _busy,
                          onUninstall: () => _uninstall(context, entry.bot.id),
                        ),
                    ],
                  );
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  bool _canInstall(WidgetRef ref) {
    final botId = _selectedBotId;
    if (botId == null) return false;
    final bots = ref.read(discoverableBotsProvider).valueOrNull;
    if (bots == null) return true;
    VoiceBotSummary? bot;
    for (final candidate in bots) {
      if (candidate.id == botId) {
        bot = candidate;
        break;
      }
    }
    if (bot == null) return true;
    final scopes = BotScopeLabels.parseScopesJson(bot.scopesJson);
    final needsAck = scopes.any(
      (s) => BotScopeLabels.privilegedScopes.contains(s),
    );
    return !needsAck || _privilegedAcknowledged;
  }

  Future<void> _install(BuildContext context) async {
    final l10n = AppLocalizations.of(context)!;
    final auth = ref.read(authorizationHeaderProvider);
    final botId = _selectedBotId;
    if (auth == null || botId == null) return;

    final tree = await ref.read(spaceTreeProvider(widget.spaceId).future);
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
      botId: botId,
      spaceId: widget.spaceId,
      allowedChats: allowedChats,
      acknowledgePrivilegedScopes: _privilegedAcknowledged,
    );
    if (!mounted) return;
    setState(() => _busy = false);

    switch (result) {
      case BotsApiOk():
        ref.invalidate(installedBotsProvider(widget.spaceId));
        ref.invalidate(discoverableBotsProvider);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.spaceBotsInstallSuccess)),
        );
        setState(() {
          _selectedBotId = null;
          _selectedChatIds.clear();
        });
      case BotsApiFailure(:final message):
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.chatRoomError(message))),
        );
    }
  }

  Future<void> _uninstall(BuildContext context, String botId) async {
    final l10n = AppLocalizations.of(context)!;
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;

    setState(() => _busy = true);
    final result = await ref.read(voiceBotsClientProvider).uninstallBotFromSpace(
      authorization: auth,
      botId: botId,
      spaceId: widget.spaceId,
    );
    if (!mounted) return;
    setState(() => _busy = false);

    switch (result) {
      case BotsApiOk():
        ref.invalidate(installedBotsProvider(widget.spaceId));
        ref.invalidate(discoverableBotsProvider);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.spaceBotsUninstallSuccess)),
        );
      case BotsApiFailure(:final message):
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.chatRoomError(message))),
        );
    }
  }
}

class _InstalledBotTile extends StatelessWidget {
  const _InstalledBotTile({
    required this.entry,
    required this.busy,
    required this.onUninstall,
  });

  final InstalledBotInfo entry;
  final bool busy;
  final VoidCallback onUninstall;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final offline = !entry.online;
    final statusTooltip = offline ? l10n.botUnavailableTooltip : l10n.botOnlineStatus;

    return Tooltip(
      message: statusTooltip,
      child: ListTile(
        key: Key('installed_bot_${entry.bot.id}'),
        enabled: !busy,
        leading: offline
            ? Icon(
                Icons.circle,
                key: Key('space_bots_offline_indicator_${entry.bot.id}'),
                size: 10,
                color: voice.textDisabled,
              )
            : Icon(
                Icons.circle,
                key: Key('space_bots_online_indicator_${entry.bot.id}'),
                size: 10,
                color: voice.profileAccent,
              ),
        title: Text(
          entry.bot.name,
          style: offline ? TextStyle(color: voice.textDisabled) : null,
        ),
        subtitle: Text(
          '${entry.allowedChatIds.length} chats',
          style: offline ? TextStyle(color: voice.textDisabled) : null,
        ),
        trailing: TextButton(
          onPressed: busy ? null : onUninstall,
          child: Text(l10n.spaceBotsUninstall),
        ),
      ),
    );
  }
}

class _ScopeList extends ConsumerWidget {
  const _ScopeList({
    required this.botId,
    this.onPrivilegedDetected,
  });

  final String botId;
  final void Function(bool hasPrivileged)? onPrivilegedDetected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final auth = ref.watch(authorizationHeaderProvider);
    if (auth == null) return const SizedBox.shrink();

    return FutureBuilder<BotsApiResult<VoiceBotSummary>>(
      future: ref.read(voiceBotsClientProvider).getBot(
        authorization: auth,
        botId: botId,
      ),
      builder: (context, snapshot) {
        final result = snapshot.data;
        if (result is! BotsApiOk<VoiceBotSummary>) {
          return const SizedBox.shrink();
        }
        final scopes = BotScopeLabels.parseScopesJson(result.data.scopesJson);
        final hasPrivileged = scopes.any(
          (s) => BotScopeLabels.privilegedScopes.contains(s),
        );
        WidgetsBinding.instance.addPostFrameCallback((_) {
          onPrivilegedDetected?.call(hasPrivileged);
        });
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            for (final scope in scopes)
              Text('• ${BotScopeLabels.labelFor(scope)}'),
            if (hasPrivileged) ...[
              const SizedBox(height: 8),
              Text(
                l10n.spaceBotsScopeWarning,
                style: TextStyle(
                  color: Theme.of(context).colorScheme.error,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ],
        );
      },
    );
  }
}

class _PrivilegedAckCheckbox extends ConsumerWidget {
  const _PrivilegedAckCheckbox({
    required this.botId,
    required this.acknowledged,
    required this.onChanged,
  });

  final String botId;
  final bool acknowledged;
  final ValueChanged<bool?>? onChanged;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final auth = ref.watch(authorizationHeaderProvider);
    if (auth == null) return const SizedBox.shrink();

    return FutureBuilder<BotsApiResult<VoiceBotSummary>>(
      future: ref.read(voiceBotsClientProvider).getBot(
        authorization: auth,
        botId: botId,
      ),
      builder: (context, snapshot) {
        final result = snapshot.data;
        if (result is! BotsApiOk<VoiceBotSummary>) {
          return const SizedBox.shrink();
        }
        final scopes = BotScopeLabels.parseScopesJson(result.data.scopesJson);
        final needsAck = scopes.any(
          (s) => BotScopeLabels.privilegedScopes.contains(s),
        );
        if (!needsAck) return const SizedBox.shrink();
        return CheckboxListTile(
          key: const Key('space_bots_privileged_ack_checkbox'),
          value: acknowledged,
          onChanged: onChanged,
          title: Text(l10n.spaceBotsPrivilegedAck),
          subtitle: Text(l10n.spaceBotsScopeWarning),
          controlAffinity: ListTileControlAffinity.leading,
        );
      },
    );
  }
}
