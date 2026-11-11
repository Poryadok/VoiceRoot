import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/search_client.dart';
import '../../backend/gateway_http.dart';
import '../../backend/users_client.dart';
import '../../backend/chats_client.dart';
import '../../backend/spaces_client.dart';
import '../../l10n/app_localizations.dart';
import '../../routing/deep_link_parser.dart';
import '../../state/auth_providers.dart';
import '../../state/deep_link_navigation.dart';
import '../../state/search_providers.dart';
import '../../state/shell_providers.dart';
import '../../state/social_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_avatar.dart';
import '../core/voice_skeleton.dart';
import '../api_error_messages.dart';

typedef _SearchSession = ({AuthController controller, GatewayRequestIdentity identity, int installation});

class GlobalSearchPanel extends ConsumerStatefulWidget {
  const GlobalSearchPanel({super.key, this.compact = false, this.onClose, this.initialQuery = ''});
  final VoidCallback? onClose;
  final String initialQuery;

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
  final _focusNode = FocusNode();
  SearchQueryDebouncer? _debouncer;
  GlobalSearchData? _results;
  final Map<String, VoiceProfile> _profiles = {};
  final Map<String, VoiceSpace> _spaces = {};
  final Map<String, ChatListItem> _chats = {};
  bool _loading = false;
  bool _loadingMore = false;
  bool _openingContact = false;
  bool _surfaceOpen = false;
  _SearchSession? _sessionContext;
  int _contactGeneration = 0;
  String? _errorMessage;
  int _queryGeneration = 0;

  @override
  void initState() {
    super.initState();
    _sessionContext = _captureSession();
    _debouncer = SearchQueryDebouncer(onQuery: _runSearch);
    _controller.text = widget.initialQuery;
    if (widget.initialQuery.trim().isNotEmpty) _debouncer!.schedule(widget.initialQuery);
  }

  @override
  void dispose() {
    _debouncer?.dispose();
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  void _onQueryChanged(String query) {
    if (widget.compact && query.isNotEmpty) {
      _openSurface();
      return;
    }
    _queryGeneration++;
    _contactGeneration++;
    _debouncer?.schedule(query);
    setState(() {
      _results = null;
      _profiles.clear();
      _spaces.clear();
      _chats.clear();
      _loading = false;
      _loadingMore = false;
      _openingContact = false;
      _errorMessage = null;
    });
  }

  Future<void> _runSearch(String query, {bool loadMore = false}) async {
    if (query != _controller.text || query.trim().isEmpty) return;
    if (loadMore && (_loadingMore || _results?.nextCursor.isNotEmpty != true)) return;
    final generation = _queryGeneration;
    final session = _captureSession();
    if (session == null) return;
    final authorization = ref
        .read(authControllerProvider)
        .session
        ?.authorizationHeader;
    if (authorization == null) return;
    final previous = loadMore ? _results : null;
    setState(() {
      _loading = !loadMore;
      _loadingMore = loadMore;
      _errorMessage = null;
    });
    final client = ref.read(voiceSearchClientProvider);
    final result = await client.searchGlobal(
      authorization: authorization,
      query: query,
      cursor: previous?.nextCursor ?? '',
      pageSize: 20,
    );
    if (!_isCurrent(generation, session)) return;
    if (result is SearchApiOk<GlobalSearchData>) {
      final profiles = <String, VoiceProfile>{};
      final usersClient = ref.read(voiceUsersClientProvider);
      for (final profileId in result.data.profileIds) {
        if (_profiles.containsKey(profileId)) continue;
        final profileResult = await usersClient.getProfile(
          authorization: authorization,
          profileId: profileId,
        );
        if (!_isCurrent(generation, session)) return;
        if (profileResult is UsersApiOk<VoiceProfile>) {
          profiles[profileId] = profileResult.data;
        }
      }
      setState(() {
        final page = result.data;
        _results = GlobalSearchData(
          messages: {for (final hit in [...?previous?.messages, ...page.messages]) '${hit.chatId}/${hit.messageId}': hit}.values.toList(),
          profileIds: {...?previous?.profileIds, ...page.profileIds}.toList(),
          matchedChatIds: {...?previous?.matchedChatIds, ...page.matchedChatIds}.toList(),
          spaceIds: {...?previous?.spaceIds, ...page.spaceIds}.toList(),
          nextCursor: page.nextCursor == previous?.nextCursor ? '' : page.nextCursor,
        );
        _profiles
          ..addAll(profiles);
        _loading = false;
        _loadingMore = false;
        _errorMessage = null;
      });
      _hydrateMetadata(result.data, authorization, generation, session);
    } else if (result is SearchApiErr) {
      setState(() {
        _results = previous;
        _loading = false;
        _loadingMore = false;
        _errorMessage = searchErrorMessage(
          AppLocalizations.of(context)!,
          result.error.message,
          statusCode: result.error.statusCode,
        );
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
        _errorMessage = null;
      });
    }
  }

  _SearchSession? _captureSession() {
    final controller = ref.read(authControllerProvider.notifier);
    final identity = controller.gatewayRequestIdentity;
    if (identity == null || ref.read(profileSwitchInProgressProvider)) return null;
    return (controller: controller, identity: identity, installation: controller.sessionInstallGeneration);
  }

  bool _isCurrent(int generation, _SearchSession session) => mounted && generation == _queryGeneration && _captureSession() == session;

  void _syncIdentity() {
    final current = _captureSession();
    if (current == _sessionContext) return;
    _sessionContext = current;
    _clear();
  }

  Future<void> _hydrateMetadata(GlobalSearchData data, String authorization, int generation, _SearchSession session) async {
    // Existing API owners supply these fields; never infer profile lifecycle or
    // invent message sender/date from SearchHit, which contains neither.
    final chatIds = {...data.matchedChatIds, ...data.messages.map((hit) => hit.chatId).where((id) => id.isNotEmpty)};
    if (chatIds.isNotEmpty || data.profileIds.isNotEmpty) {
      String? cursor;
      final seen = <String>{};
      do {
        final result = await ref.read(voiceChatsClientProvider).listChats(authorization: authorization, cursor: cursor, pageSize: 20);
        if (!_isCurrent(generation, session)) return;
        if (result is! ChatsApiOk<ChatListData>) break;
        setState(() { for (final item in result.data.items) { _chats[item.chatId] = item; } });
        cursor = result.data.nextCursor;
      } while (cursor != null && cursor.isNotEmpty && seen.add(cursor) &&
          (chatIds.any((id) => !_chats.containsKey(id)) || data.profileIds.any((id) => !_chats.values.any((item) => item.dmPeerProfileId == id))));
    }
    for (final id in data.spaceIds) {
      final result = await ref.read(voiceSpacesClientProvider).getSpace(authorization: authorization, spaceId: id);
      if (!_isCurrent(generation, session)) return;
      if (result is SpacesApiOk<VoiceSpace>) setState(() => _spaces[id] = result.data);
    }
  }

  void _clear() {
    _controller.clear();
    _onQueryChanged('');
    _focusNode.requestFocus();
  }

  Future<void> _openContact(String id) async {
    if (_openingContact) return;
    final session = _captureSession();
    if (session == null) return;
    final generation = _queryGeneration;
    final operation = ++_contactGeneration;
    bool isCurrent() => mounted && operation == _contactGeneration && _isCurrent(generation, session);
    if (!isCurrent()) return;
    setState(() => _openingContact = true);
    final error = await ref.read(chatActionsProvider).openDmWithProfile(id, isCurrent: isCurrent);
    if (!isCurrent()) return;
    setState(() => _openingContact = false);
    if (error == null) {
      widget.onClose?.call();
    } else {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(searchErrorMessage(AppLocalizations.of(context)!, error))));
    }
  }

  Future<void> _openSurface() async {
    if (_surfaceOpen) return;
    _surfaceOpen = true;
    _focusNode.unfocus();
    try {
    await showGeneralDialog<void>(
      context: context,
      barrierDismissible: true,
      barrierLabel: MaterialLocalizations.of(context).modalBarrierDismissLabel,
      pageBuilder: (context, _, __) => Align(
        alignment: Alignment.centerRight,
        child: SizedBox(
          width: MediaQuery.sizeOf(context).width < 600 ? double.infinity : 320,
          height: double.infinity,
          child: Material(color: VoiceColors.of(context).canvas,
            child: SafeArea(child: GlobalSearchPanel(initialQuery: _controller.text, onClose: () => Navigator.of(context).pop())),
          ),
        ),
      ),
    );
    } finally {
      _surfaceOpen = false;
      if (mounted) _clear();
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    ref.listen(globalSearchFocusRequestProvider, (prev, next) {
      if (next != (prev ?? 0)) {
        if (widget.compact) { _openSurface(); } else { _focusNode.requestFocus(); }
      }
    });
    ref.listen(authControllerProvider, (_, __) => _syncIdentity());
    ref.listen(profileSwitchInProgressProvider, (_, __) => _syncIdentity());

    final showResults = _results != null;

    final resultsList = ListView(
      padding: EdgeInsets.zero,
      shrinkWrap: widget.compact,
      physics: widget.compact
          ? const ClampingScrollPhysics()
          : const AlwaysScrollableScrollPhysics(),
      children: [
        _SectionHeader(
          key: GlobalSearchPanel.contactsSectionKey,
          title: l10n.globalSearchContactsAndChats,
        ),
        if (_results == null)
          Text(
            l10n.globalSearchStartHint,
            style: TextStyle(color: voice.textSecondary),
          )
        else if (_results!.profileIds.isEmpty && _results!.matchedChatIds.isEmpty)
          Text(
            l10n.globalSearchEmptyContacts,
            style: TextStyle(color: voice.textSecondary),
          )
        else
          for (final profileId in _results!.profileIds)
            ListTile(
              leading: VoiceAvatar(
                imageUrl: _profiles[profileId]?.avatarUrl,
                label: _profiles[profileId]?.displayName ?? l10n.socialProfileUnavailable,
                radius: 20,
              ),
              title: Text(_profiles[profileId]?.displayName ?? l10n.socialProfileUnavailable, maxLines: 1, overflow: TextOverflow.ellipsis),
              subtitle: _chats.values.where((item) => item.dmPeerProfileId == profileId).firstOrNull?.lastMessagePreview == null ? null : Text(_chats.values.where((item) => item.dmPeerProfileId == profileId).first.lastMessagePreview!, maxLines: 1, overflow: TextOverflow.ellipsis),
              onTap: _openingContact || !_profiles.containsKey(profileId) ? null : () => _openContact(profileId),
            ),
        for (final chatId in _results?.matchedChatIds ?? <String>[])
          ListTile(
            key: ValueKey('global_search_chat_$chatId'),
            leading: VoiceAvatar(imageUrl: _chats[chatId]?.chat.avatarUrl, label: _chats[chatId]?.chat.name ?? l10n.chatRoomTitle(chatId), radius: 20),
            title: Text(_chats[chatId]?.dmPeerDisplayName ?? _chats[chatId]?.chat.name ?? l10n.chatRoomTitle(chatId), maxLines: 1, overflow: TextOverflow.ellipsis),
            subtitle: _chats[chatId]?.lastMessagePreview == null ? null : Text(_chats[chatId]!.lastMessagePreview!, maxLines: 1, overflow: TextOverflow.ellipsis),
            onTap: () { ref.read(shellNavigationProvider).selectChatFromHome(chatId); widget.onClose?.call(); },
          ),
        const SizedBox(height: 12),
        _SectionHeader(
          key: GlobalSearchPanel.spacesSectionKey,
          title: l10n.globalSearchSpaces,
        ),
        if (_results != null && _results!.spaceIds.isNotEmpty)
          for (final spaceId in _results!.spaceIds)
            ListTile(
              leading: VoiceAvatar(imageUrl: _spaces[spaceId]?.iconUrl, label: _spaces[spaceId]?.name ?? spaceId, radius: 20),
              title: Text(_spaces[spaceId]?.name ?? spaceId, maxLines: 1, overflow: TextOverflow.ellipsis),
              subtitle: _spaces[spaceId] == null ? null : Text(l10n.spaceCatalogMembers(_spaces[spaceId]!.memberCount)),
              onTap: () { ref.read(shellNavigationProvider).selectSpace(spaceId); widget.onClose?.call(); },
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
              leading: const Icon(Icons.forum_outlined),
              title: Text(_chats[hit.chatId]?.chat.name ?? l10n.chatRoomTitle(hit.chatId), maxLines: 1, overflow: TextOverflow.ellipsis),
              subtitle: _SnippetText(messageId: hit.messageId, snippet: hit.snippet),
              onTap: hit.chatId.isEmpty || hit.messageId.isEmpty
                  ? null
                  : () { ref
                        .read(deepLinkNavigatorProvider)
                        .apply(
                          DeepLinkTarget(
                            kind: DeepLinkKind.chatMessage,
                            chatId: hit.chatId,
                            messageId: hit.messageId,
                            rawUrl: '',
                          ),
                        ); widget.onClose?.call(); },
            )
        else if (_results != null)
          Text(
            l10n.globalSearchEmptyMessages,
            style: TextStyle(color: voice.textSecondary),
          ),
        if (_results?.nextCursor.isNotEmpty == true)
          Padding(padding: const EdgeInsets.all(16), child: OutlinedButton(
            key: const Key('global_search_load_more'),
            onPressed: _loadingMore ? null : () => _runSearch(_controller.text, loadMore: true),
            child: _loadingMore ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2)) : Text(l10n.globalSearchLoadMore),
          )),
      ],
    );

    return ListTileTheme(data: const ListTileThemeData(minTileHeight: 64, contentPadding: EdgeInsets.symmetric(horizontal: 16), horizontalTitleGap: 12), child: Column(
      key: GlobalSearchPanel.panelKey,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (!widget.compact && widget.onClose != null)
          SizedBox(height: 56, child: Row(children: [
            IconButton(tooltip: l10n.globalSearchBack, onPressed: widget.onClose, icon: const Icon(Icons.arrow_back)),
            Expanded(child: Text(l10n.globalSearchTitle, style: Theme.of(context).textTheme.titleMedium)),
          ])),
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
          child: TextField(
            key: GlobalSearchPanel.searchFieldKey,
            controller: _controller,
            focusNode: _focusNode,
            autofocus: widget.onClose != null,
            onTap: widget.compact ? _openSurface : null,
            decoration: InputDecoration(
              hintText: l10n.globalSearchHint,
              prefixIcon: const Icon(Icons.search),
              suffixIcon: IconButton(key: const Key('global_search_clear'), tooltip: l10n.globalSearchClear, onPressed: _clear, icon: const Icon(Icons.close)),
              filled: true,
              fillColor: voice.canvas,
              border: OutlineInputBorder(borderRadius: BorderRadius.circular(999), borderSide: BorderSide(color: voice.borderDefault)),
              isDense: true,
            ),
            onChanged: _onQueryChanged,
          ),
        ),
        if (_loading)
          const Padding(
            padding: EdgeInsets.symmetric(horizontal: 12),
            child: VoiceListSkeleton(rowCount: 3),
          ),
        if (_errorMessage != null)
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 0, 12, 8),
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [Text(
              key: const Key('global_search_error'),
              _errorMessage!,
              style: TextStyle(color: voice.error),
            ), TextButton(key: const Key('global_search_retry'), onPressed: _loading || _loadingMore ? null : () => _runSearch(_controller.text, loadMore: _results != null), child: Text(l10n.globalSearchRetry))]),
          ),
        if (!showResults && !widget.compact)
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 0, 12, 8),
            child: Text(
              l10n.globalSearchStartHint,
              style: TextStyle(color: voice.textSecondary, fontSize: 12),
            ),
          ),
        if (showResults)
          widget.compact
              ? ConstrainedBox(
                  constraints: const BoxConstraints(maxHeight: 260),
                  child: resultsList,
                )
              : Expanded(child: NotificationListener<ScrollNotification>(
                  onNotification: (notification) { if (notification.metrics.extentAfter < 80 && _errorMessage == null && !_loadingMore && _results?.nextCursor.isNotEmpty == true) _runSearch(_controller.text, loadMore: true); return false; },
                  child: resultsList)),
      ],
    ));
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({super.key, required this.title});

  final String title;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Semantics(header: true, child: Text(title.toUpperCase(), style: TextStyle(fontSize: 11, letterSpacing: .6, color: VoiceColors.of(context).textSecondary, fontWeight: FontWeight.w600))),
    );
  }
}

class _SnippetText extends StatelessWidget {
  const _SnippetText({required this.messageId, required this.snippet});
  final String messageId;

  final String snippet;

  @override
  Widget build(BuildContext context) {
    final matches = RegExp(r'<(b|mark)>(.*?)</\1>', dotAll: true).allMatches(snippet);
    if (matches.isEmpty) return Text(snippet, key: ValueKey('global_search_snippet_$messageId'));
    final spans = <TextSpan>[];
    var offset = 0;
    for (final match in matches) {
      spans.add(TextSpan(text: snippet.substring(offset, match.start)));
      spans.add(TextSpan(text: match.group(2), style: TextStyle(backgroundColor: VoiceColors.of(context).warning.withValues(alpha: .24), fontWeight: FontWeight.w600)));
      offset = match.end;
    }
    spans.add(TextSpan(text: snippet.substring(offset)));
    return Text.rich(TextSpan(children: spans), key: ValueKey('global_search_snippet_$messageId'));
  }
}
