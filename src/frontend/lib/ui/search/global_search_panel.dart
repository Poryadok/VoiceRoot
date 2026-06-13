import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/search_client.dart';
import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/search_providers.dart';
import '../../state/shell_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_avatar.dart';

class GlobalSearchPanel extends ConsumerStatefulWidget {
  const GlobalSearchPanel({super.key, this.compact = false});

  /// When true, only the search field is shown until the user runs a query.
  final bool compact;

  static const Key panelKey = Key('global_search_panel');
  static const Key searchFieldKey = Key('global_search_field');
  static const Key contactsSectionKey = Key('global_search_contacts');
  static const Key spacesSectionKey = Key('global_search_spaces');
  static const Key messagesSectionKey = Key('global_search_messages');

  @override
  ConsumerState<GlobalSearchPanel> createState() => _GlobalSearchPanelState();
}

class _GlobalSearchPanelState extends ConsumerState<GlobalSearchPanel> {
  final _controller = TextEditingController();
  SearchQueryDebouncer? _debouncer;
  GlobalSearchData? _results;
  final Map<String, VoiceProfile> _profiles = {};
  bool _loading = false;

  @override
  void initState() {
    super.initState();
    _debouncer = SearchQueryDebouncer(onQuery: _runSearch);
  }

  @override
  void dispose() {
    _debouncer?.dispose();
    _controller.dispose();
    super.dispose();
  }

  Future<void> _runSearch(String query) async {
    final authorization =
        ref.read(authControllerProvider).session?.authorizationHeader;
    if (authorization == null) return;
    setState(() => _loading = true);
    final client = ref.read(voiceSearchClientProvider);
    final result = await client.searchGlobal(
      authorization: authorization,
      query: query,
    );
    if (!mounted) return;
    if (result is SearchApiOk<GlobalSearchData>) {
      final profiles = <String, VoiceProfile>{};
      final usersClient = ref.read(voiceUsersClientProvider);
      for (final profileId in result.data.profileIds.take(5)) {
        final profileResult = await usersClient.getProfile(
          authorization: authorization,
          profileId: profileId,
        );
        if (profileResult is UsersApiOk<VoiceProfile>) {
          profiles[profileId] = profileResult.data;
        }
      }
      setState(() {
        _results = result.data;
        _profiles
          ..clear()
          ..addAll(profiles);
        _loading = false;
      });
    } else {
      setState(() {
        _results = const GlobalSearchData(
          messages: [],
          profileIds: [],
          matchedChatIds: [],
          spaceIds: [],
        );
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    final showResults = _loading || _results != null;

    return Column(
      key: GlobalSearchPanel.panelKey,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(12, 12, 12, 8),
          child: TextField(
            key: GlobalSearchPanel.searchFieldKey,
            controller: _controller,
            decoration: InputDecoration(
              hintText: l10n.globalSearchHint,
              prefixIcon: const Icon(Icons.search),
              isDense: true,
            ),
            onChanged: _debouncer?.schedule,
          ),
        ),
        if (_loading)
          const LinearProgressIndicator(minHeight: 2),
        if (!showResults && !widget.compact)
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 0, 12, 8),
            child: Text(
              l10n.globalSearchStartHint,
              style: TextStyle(color: voice.textSecondary, fontSize: 12),
            ),
          ),
        if (showResults)
          Expanded(
            child: ListView(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            children: [
              _SectionHeader(
                key: GlobalSearchPanel.contactsSectionKey,
                title: l10n.globalSearchContacts,
              ),
              if (_results == null)
                Text(
                  l10n.globalSearchStartHint,
                  style: TextStyle(color: voice.textSecondary),
                )
              else if (_results!.profileIds.isEmpty)
                Text(
                  l10n.globalSearchEmptyContacts,
                  style: TextStyle(color: voice.textSecondary),
                )
              else
                for (final profileId in _results!.profileIds)
                  ListTile(
                    leading: VoiceAvatar(
                      imageUrl: _profiles[profileId]?.avatarUrl,
                      label: _profiles[profileId]?.displayName ?? profileId,
                      radius: 18,
                    ),
                    title: Text(
                      _profiles[profileId]?.displayName ?? profileId,
                    ),
                  ),
              const SizedBox(height: 12),
              _SectionHeader(
                key: GlobalSearchPanel.spacesSectionKey,
                title: l10n.globalSearchSpaces,
              ),
              if (_results != null && _results!.spaceIds.isNotEmpty)
                for (final spaceId in _results!.spaceIds)
                  ListTile(
                    leading: const Icon(Icons.public),
                    title: Text(spaceId),
                    onTap: () =>
                        ref.read(shellNavigationProvider).selectSpace(spaceId),
                  )
              else if (_results != null)
                Text(
                  l10n.globalSearchEmptySpaces,
                  style: TextStyle(color: voice.textSecondary),
                ),
              const SizedBox(height: 12),
              _SectionHeader(
                key: GlobalSearchPanel.messagesSectionKey,
                title: l10n.globalSearchMessages,
              ),
              if (_results != null && _results!.messages.isNotEmpty)
                for (final hit in _results!.messages)
                  ListTile(
                    title: _SnippetText(snippet: hit.snippet),
                    subtitle: Text(hit.messageId),
                    onTap: () {
                      final chatId = hit.chatId.isNotEmpty
                          ? hit.chatId
                          : (_results!.matchedChatIds.isNotEmpty
                              ? _results!.matchedChatIds.first
                              : null);
                      if (chatId != null) {
                        ref.read(selectedChatIdProvider.notifier).state =
                            chatId;
                      }
                    },
                  )
              else if (_results != null)
                Text(
                  l10n.globalSearchEmptyMessages,
                  style: TextStyle(color: voice.textSecondary),
                ),
            ],
            ),
          ),
      ],
    );
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({super.key, required this.title});

  final String title;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Text(
        title,
        style: Theme.of(context).textTheme.titleSmall,
      ),
    );
  }
}

class _SnippetText extends StatelessWidget {
  const _SnippetText({required this.snippet});

  final String snippet;

  @override
  Widget build(BuildContext context) {
    final plain = snippet
        .replaceAll(RegExp(r'</?b>'), '')
        .replaceAll(RegExp(r'</?mark>'), '');
    return Text(plain);
  }
}
