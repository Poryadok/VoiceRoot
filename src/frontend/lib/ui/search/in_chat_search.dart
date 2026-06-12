import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/search_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/search_providers.dart';
import '../../theme/voice_colors.dart';

class InChatSearch extends ConsumerStatefulWidget {
  const InChatSearch({super.key, required this.chatId});

  static const Key panelKey = Key('in_chat_search_panel');
  static const Key searchFieldKey = Key('in_chat_search_field');
  static const Key prevHitKey = Key('in_chat_search_prev');
  static const Key nextHitKey = Key('in_chat_search_next');
  static const Key highlightKey = Key('in_chat_search_highlight');

  static Key activeHitKey(String messageId) =>
      Key('in_chat_search_active_$messageId');

  final String chatId;

  @override
  ConsumerState<InChatSearch> createState() => _InChatSearchState();
}

class _InChatSearchState extends ConsumerState<InChatSearch> {
  final _controller = TextEditingController();
  SearchQueryDebouncer? _debouncer;
  List<SearchHit> _hits = const [];
  int _activeIndex = 0;

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
    final client = ref.read(voiceSearchClientProvider);
    final result = await client.searchInChat(
      authorization: authorization,
      chatId: widget.chatId,
      query: query,
    );
    if (!mounted) return;
    if (result is SearchApiOk<InChatSearchData>) {
      setState(() {
        _hits = result.data.hits;
        _activeIndex = 0;
      });
    } else {
      setState(() {
        _hits = const [];
        _activeIndex = 0;
      });
    }
  }

  void _step(int delta) {
    if (_hits.isEmpty) return;
    setState(() {
      _activeIndex = (_activeIndex + delta) % _hits.length;
      if (_activeIndex < 0) _activeIndex += _hits.length;
    });
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final active = _hits.isEmpty ? null : _hits[_activeIndex];

    return Column(
      key: InChatSearch.panelKey,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  key: InChatSearch.searchFieldKey,
                  controller: _controller,
                  decoration: InputDecoration(
                    hintText: l10n.inChatSearchHint,
                    prefixIcon: const Icon(Icons.search),
                    isDense: true,
                  ),
                  onChanged: _debouncer?.schedule,
                ),
              ),
              IconButton(
                key: InChatSearch.prevHitKey,
                tooltip: l10n.inChatSearchPrevious,
                onPressed: _hits.isEmpty ? null : () => _step(-1),
                icon: const Icon(Icons.keyboard_arrow_up),
              ),
              IconButton(
                key: InChatSearch.nextHitKey,
                tooltip: l10n.inChatSearchNext,
                onPressed: _hits.isEmpty ? null : () => _step(1),
                icon: const Icon(Icons.keyboard_arrow_down),
              ),
            ],
          ),
        ),
        Expanded(
          child: ListView.builder(
            itemCount: _hits.length,
            itemBuilder: (context, index) {
              final hit = _hits[index];
              final isActive = active?.messageId == hit.messageId;
              return ListTile(
                key: isActive
                    ? InChatSearch.activeHitKey(hit.messageId)
                    : null,
                title: _HighlightedSnippet(
                  snippet: hit.snippet,
                  highlightKey:
                      isActive ? InChatSearch.highlightKey : null,
                ),
                subtitle: Text(
                  l10n.inChatSearchResultScore(hit.score.toStringAsFixed(1)),
                  style: TextStyle(color: voice.textSecondary),
                ),
              );
            },
          ),
        ),
      ],
    );
  }
}

class _HighlightedSnippet extends StatelessWidget {
  const _HighlightedSnippet({
    required this.snippet,
    this.highlightKey,
  });

  final String snippet;
  final Key? highlightKey;

  @override
  Widget build(BuildContext context) {
    final markMatch = RegExp(r'<mark>(.*?)</mark>', caseSensitive: false);
    final bMatch = RegExp(r'<b>(.*?)</b>', caseSensitive: false);
    final match = markMatch.firstMatch(snippet) ?? bMatch.firstMatch(snippet);
    if (match == null) {
      return Text(snippet.replaceAll(RegExp(r'</?b>|</?mark>'), ''));
    }
    final before =
        snippet.substring(0, match.start).replaceAll(RegExp(r'</?b>|</?mark>'), '');
    final highlighted = match.group(1) ?? '';
    final after =
        snippet.substring(match.end).replaceAll(RegExp(r'</?b>|</?mark>'), '');
    return Text.rich(
      TextSpan(
        style: DefaultTextStyle.of(context).style,
        children: [
          TextSpan(text: before),
          WidgetSpan(
            child: Text(
              highlighted,
              key: highlightKey,
              style: const TextStyle(fontWeight: FontWeight.bold),
            ),
          ),
          TextSpan(text: after),
        ],
      ),
    );
  }
}
