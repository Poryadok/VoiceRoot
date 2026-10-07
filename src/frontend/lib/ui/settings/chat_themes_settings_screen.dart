import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../settings/chat_theme_preference.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';

class ChatThemesSettingsScreen extends ConsumerStatefulWidget {
  const ChatThemesSettingsScreen({super.key, this.chatId});

  static const Key screenKey = Key('chat_themes_settings_screen');
  static const Key chatPickerKey = Key('chat_themes_chat_picker');
  static const Key applyKey = Key('chat_themes_apply');
  static const Key resetKey = Key('chat_themes_reset');

  static Key themeKey(ChatTheme theme) => Key('chat_theme_${theme.name}');

  final String? chatId;

  @override
  ConsumerState<ChatThemesSettingsScreen> createState() =>
      _ChatThemesSettingsScreenState();
}

class _ChatThemesSettingsScreenState
    extends ConsumerState<ChatThemesSettingsScreen> {
  String? _targetChatId;
  String? _ownerProfileId;
  ChatTheme? _draft;
  bool _actionFailed = false;
  int _contextGeneration = 0;

  @override
  void initState() {
    super.initState();
    _ownerProfileId = ref.read(authControllerProvider).activeProfileId;
    _targetChatId = widget.chatId ?? ref.read(selectedChatIdProvider);
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final colors = VoiceColors.of(context);
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;
    ref.listen(authControllerProvider, (previous, next) {
      if (previous?.activeProfileId != next.activeProfileId &&
          _ownerProfileId != next.activeProfileId) {
        setState(() {
          _ownerProfileId = next.activeProfileId;
          _targetChatId = null;
          _draft = null;
          _actionFailed = false;
          _contextGeneration++;
        });
      }
    });
    if (widget.chatId == null) {
      ref.listen(selectedChatIdProvider, (previous, next) {
        if (previous == next) return;
        setState(() {
          _targetChatId = next;
          _draft = null;
          _actionFailed = false;
          _contextGeneration++;
        });
      });
    }

    final targetChatId = activeProfileId == _ownerProfileId
        ? _targetChatId
        : null;
    final subscription = ref.watch(subscriptionProvider);
    final preferences = ref.watch(chatThemePreferenceProvider);
    final isPlus = subscription.valueOrNull?.isPremium == true;
    final saved = targetChatId == null
        ? null
        : preferences.valueOrNull?[targetChatId];
    final selected = _draft ?? saved ?? ChatTheme.ocean;
    final canChange = isPlus && preferences.hasValue && targetChatId != null;

    return Scaffold(
      key: ChatThemesSettingsScreen.screenKey,
      backgroundColor: colors.canvas,
      appBar: AppBar(
        backgroundColor: colors.surface,
        leading: IconButton(
          key: const Key('chat_themes_back'),
          tooltip: MaterialLocalizations.of(context).backButtonTooltip,
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.of(context).maybePop(),
        ),
        title: Text(l10n.settingsChatThemes),
      ),
      body: SafeArea(
        child: LayoutBuilder(
          builder: (context, constraints) => SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: Align(
              alignment: Alignment.topCenter,
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 720),
                child: targetChatId == null
                    ? _ChatThemeChatPicker(
                        key: ChatThemesSettingsScreen.chatPickerKey,
                        onSelected: (chatId) => setState(() {
                          _targetChatId = chatId;
                          _draft = null;
                          _actionFailed = false;
                        }),
                      )
                    : _themeContent(
                        context,
                        l10n: l10n,
                        targetChatId: targetChatId,
                        saved: saved,
                        selected: selected,
                        subscription: subscription,
                        preferences: preferences,
                        canChange: canChange,
                        columns: constraints.maxWidth < 540 ? 2 : 4,
                      ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _themeContent(
    BuildContext context, {
    required AppLocalizations l10n,
    required String targetChatId,
    required ChatTheme? saved,
    required ChatTheme selected,
    required AsyncValue<dynamic> subscription,
    required AsyncValue<Map<String, ChatTheme>> preferences,
    required bool canChange,
    required int columns,
  }) {
    final isPlus =
        ref.watch(subscriptionProvider).valueOrNull?.isPremium == true;
    final preview = isPlus ? selected : ChatTheme.ocean;
    final subscriptionFailure = subscription.hasError;
    final preferenceFailure = preferences.hasError;
    final hasDraft = _draft != null;
    final canApply = canChange && hasDraft && _draft != saved;
    final canReset = preferences.hasValue && saved != null;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (subscriptionFailure) ...[
          _InlineFailure(
            message: l10n.subscriptionLoadError,
            actionLabel: l10n.subscriptionRetry,
            onRetry: () => ref.invalidate(subscriptionProvider),
          ),
          const SizedBox(height: 12),
        ],
        if (preferenceFailure) ...[
          _InlineFailure(
            message: l10n.themeLoadError,
            actionLabel: l10n.commonRetry,
            onRetry: () => ref.invalidate(chatThemePreferenceProvider),
          ),
          const SizedBox(height: 12),
        ],
        if (!isPlus) ...[
          Text(
            l10n.chatThemesPlusLabel,
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: 4),
          Text(l10n.chatThemesPlusNotice),
          if (saved != null) ...[
            const SizedBox(height: 8),
            Text(l10n.chatThemesSelected),
          ],
          const SizedBox(height: 16),
        ],
        if (preferences.isLoading && !preferences.hasValue)
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 24),
            child: Center(child: CircularProgressIndicator()),
          )
        else ...[
          GridView.count(
            key: const Key('chat_themes_catalog'),
            crossAxisCount: columns,
            mainAxisExtent: 150,
            mainAxisSpacing: 12,
            crossAxisSpacing: 12,
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            children: [
              for (final theme in ChatTheme.values)
                _ChatThemeOption(
                  key: ChatThemesSettingsScreen.themeKey(theme),
                  theme: theme,
                  selected: theme == selected,
                  enabled: canChange,
                  onSelected: () => setState(() {
                    _draft = theme;
                    _actionFailed = false;
                  }),
                ),
            ],
          ),
          const SizedBox(height: 20),
          _ChatThemeSample(theme: preview),
          if (_actionFailed) ...[
            const SizedBox(height: 12),
            Text(l10n.commonActionFailed),
            Align(
              alignment: Alignment.centerLeft,
              child: TextButton(
                onPressed: canChange ? () => _apply(targetChatId) : null,
                child: Text(l10n.commonRetry),
              ),
            ),
          ],
          const SizedBox(height: 20),
          Wrap(
            spacing: 12,
            runSpacing: 8,
            children: [
              FilledButton(
                key: ChatThemesSettingsScreen.applyKey,
                onPressed: canApply ? () => _apply(targetChatId) : null,
                child: Text(l10n.chatThemesApply(_themeName(selected))),
              ),
              OutlinedButton(
                key: ChatThemesSettingsScreen.resetKey,
                onPressed: canReset ? () => _reset(targetChatId) : null,
                child: Text(l10n.chatThemesReset),
              ),
            ],
          ),
        ],
      ],
    );
  }

  Future<void> _apply(String chatId) async {
    final generation = _contextGeneration;
    final profileId = _ownerProfileId;
    final theme = _draft;
    if (theme == null ||
        !mounted ||
        ref.read(authControllerProvider).activeProfileId != profileId ||
        ref.read(subscriptionProvider).valueOrNull?.isPremium != true) {
      return;
    }
    try {
      await ref
          .read(chatThemePreferenceProvider.notifier)
          .setForChat(chatId, theme);
      if (!mounted || generation != _contextGeneration) return;
      setState(() => _actionFailed = false);
    } catch (_) {
      if (!mounted || generation != _contextGeneration) return;
      setState(() => _actionFailed = true);
    }
  }

  Future<void> _reset(String chatId) async {
    final generation = _contextGeneration;
    try {
      await ref.read(chatThemePreferenceProvider.notifier).resetForChat(chatId);
      if (!mounted || generation != _contextGeneration) return;
      setState(() {
        _draft = null;
        _actionFailed = false;
      });
    } catch (_) {
      if (!mounted || generation != _contextGeneration) return;
      setState(() => _actionFailed = true);
    }
  }
}

class _ChatThemeChatPicker extends ConsumerWidget {
  const _ChatThemeChatPicker({super.key, required this.onSelected});

  final ValueChanged<String> onSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final chats = ref.watch(chatListControllerProvider);
    final colors = VoiceColors.of(context);
    if (chats.isLoading && chats.items.isEmpty) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: 24),
        child: Center(child: CircularProgressIndicator()),
      );
    }
    if (chats.errorMessage != null && chats.items.isEmpty) {
      return _InlineFailure(
        message: l10n.chatListLoadError,
        actionLabel: l10n.commonRetry,
        onRetry: () =>
            ref.read(chatListControllerProvider.notifier).loadInitial(),
      );
    }
    if (chats.items.isEmpty) return Text(l10n.chatListEmpty);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.chatRoomSelectPrompt,
          style: Theme.of(context).textTheme.titleMedium,
        ),
        const SizedBox(height: 8),
        for (final item in chats.items)
          ListTile(
            key: ValueKey('chat_themes_choose_${item.chatId}'),
            title: Text(
              item.chat.name?.trim().isNotEmpty == true
                  ? item.chat.name!.trim()
                  : item.dmPeerDisplayName?.trim().isNotEmpty == true
                  ? item.dmPeerDisplayName!.trim()
                  : l10n.chatRoomTitle(item.chatId),
            ),
            trailing: Icon(Icons.chevron_right, color: colors.textSecondary),
            onTap: () => onSelected(item.chatId),
          ),
        if (chats.errorMessage != null) ...[
          const SizedBox(height: 8),
          Text(l10n.chatListLoadError),
          Align(
            alignment: Alignment.centerLeft,
            child: TextButton(
              onPressed: () =>
                  ref.read(chatListControllerProvider.notifier).loadMore(),
              child: Text(l10n.commonRetry),
            ),
          ),
        ] else if (chats.hasMore) ...[
          const SizedBox(height: 8),
          TextButton(
            onPressed: chats.isLoadingMore
                ? null
                : () =>
                      ref.read(chatListControllerProvider.notifier).loadMore(),
            child: Text(
              chats.isLoadingMore ? l10n.commonLoading : l10n.chatListLoadMore,
            ),
          ),
        ],
      ],
    );
  }
}

class _ChatThemeOption extends StatelessWidget {
  const _ChatThemeOption({
    super.key,
    required this.theme,
    required this.selected,
    required this.enabled,
    required this.onSelected,
  });

  final ChatTheme theme;
  final bool selected;
  final bool enabled;
  final VoidCallback onSelected;

  @override
  Widget build(BuildContext context) {
    final colors = VoiceColors.of(context);
    final title = _themeName(theme);
    final scheme = Theme.of(context).colorScheme;
    return Semantics(
      label: title,
      button: true,
      enabled: enabled,
      selected: selected,
      container: true,
      excludeSemantics: true,
      child: Material(
        clipBehavior: Clip.antiAlias,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(12),
          side: BorderSide(
            color: selected ? scheme.primary : colors.borderDefault,
            width: selected ? 2 : 1,
          ),
        ),
        child: FocusableActionDetector(
          enabled: enabled,
          actions: {
            ActivateIntent: CallbackAction<ActivateIntent>(
              onInvoke: (_) {
                onSelected();
                return null;
              },
            ),
          },
          shortcuts: {
            SingleActivator(LogicalKeyboardKey.enter): ActivateIntent(),
            SingleActivator(LogicalKeyboardKey.space): ActivateIntent(),
          },
          child: InkWell(
            onTap: enabled ? onSelected : null,
            child: Padding(
              padding: const EdgeInsets.all(10),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Expanded(
                    child: _ChatThemeSample(theme: theme, compact: true),
                  ),
                  const SizedBox(height: 8),
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      if (selected) ...[
                        const SizedBox(width: 4),
                        Icon(Icons.check, size: 16, color: scheme.primary),
                      ],
                    ],
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _ChatThemeSample extends StatelessWidget {
  const _ChatThemeSample({required this.theme, this.compact = false});

  final ChatTheme theme;
  final bool compact;

  @override
  Widget build(BuildContext context) {
    final palette = ChatThemePalette.forTheme(theme);
    final colors = VoiceColors.of(context);
    return Container(
      key: compact ? null : const Key('chat_themes_conversation_preview'),
      width: double.infinity,
      padding: EdgeInsets.all(compact ? 8 : 16),
      decoration: BoxDecoration(
        color: colors.canvas,
        borderRadius: BorderRadius.circular(10),
      ),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Align(
            alignment: Alignment.centerLeft,
            child: _SampleBubble(
              text: 'A message',
              color: palette.incoming,
              textColor: palette.text,
              compact: compact,
            ),
          ),
          SizedBox(height: compact ? 6 : 10),
          Align(
            alignment: Alignment.centerRight,
            child: _SampleBubble(
              text: 'Your message',
              color: palette.outgoing,
              textColor: palette.text,
              compact: compact,
            ),
          ),
        ],
      ),
    );
  }
}

class _SampleBubble extends StatelessWidget {
  const _SampleBubble({
    required this.text,
    required this.color,
    required this.textColor,
    required this.compact,
  });

  final String text;
  final Color color;
  final Color textColor;
  final bool compact;

  @override
  Widget build(BuildContext context) => Container(
    padding: EdgeInsets.symmetric(horizontal: compact ? 8 : 12, vertical: 6),
    decoration: BoxDecoration(
      color: color,
      borderRadius: BorderRadius.circular(12),
    ),
    child: Text(
      text,
      maxLines: 1,
      overflow: TextOverflow.ellipsis,
      style: TextStyle(color: textColor, fontSize: compact ? 11 : 14),
    ),
  );
}

class _InlineFailure extends StatelessWidget {
  const _InlineFailure({
    required this.message,
    required this.actionLabel,
    required this.onRetry,
  });

  final String message;
  final String actionLabel;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) => Row(
    children: [
      Expanded(child: Text(message)),
      TextButton(onPressed: onRetry, child: Text(actionLabel)),
    ],
  );
}

String _themeName(ChatTheme theme) => switch (theme) {
  ChatTheme.ocean => 'Ocean',
  ChatTheme.violet => 'Violet',
  ChatTheme.sunset => 'Sunset',
  ChatTheme.midnight => 'Midnight',
};
