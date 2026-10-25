import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/chats_client.dart';
import '../../backend/gifs_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../theme/voice_emoji_style.dart';
import 'composer_panels.dart';

enum _PickerTab { emoji, stickers, gifs }

final _catalogIdentityProvider = Provider((ref) {
  ref.watch(authControllerProvider);
  final controller = ref.read(authControllerProvider.notifier);
  return (
    request: controller.gatewayRequestIdentity,
    install: controller.sessionInstallGeneration,
  );
});

/// Real catalog browsing. Media submission awaits server catalog admission;
/// preview URLs and File IDs alone must never enable a message send.
class ComposerMediaPickerBody extends ConsumerStatefulWidget {
  const ComposerMediaPickerBody({
    super.key,
    required this.onEmojiSelected,
    this.recentEmoji = const [],
  });
  final ValueChanged<String> onEmojiSelected;
  final List<String> recentEmoji;

  @override
  ConsumerState<ComposerMediaPickerBody> createState() =>
      _ComposerMediaPickerBodyState();
}

class _ComposerMediaPickerBodyState
    extends ConsumerState<ComposerMediaPickerBody> {
  _PickerTab _tab = _PickerTab.emoji;
  final _search = TextEditingController();
  final _searchFocus = FocusNode();
  Timer? _debounce;
  int _generation = 0;
  bool _loading = false;
  bool _failed = false;
  bool _unavailable = false;
  List<StickerPack> _packs = const [];
  String? _packId;
  List<GifCatalogItem> _gifs = const [];
  String _cursor = '';

  @override
  void dispose() {
    _generation++;
    _debounce?.cancel();
    _search.dispose();
    _searchFocus.dispose();
    super.dispose();
  }

  void _selectTab(_PickerTab tab) {
    if (_tab == tab) return;
    _debounce?.cancel();
    _generation++;
    _search.clear();
    setState(() {
      _tab = tab;
      _failed = false;
      _loading = false;
      _packs = const [];
      _packId = null;
      _gifs = const [];
      _cursor = '';
    });
    if (tab != _PickerTab.emoji) {
      unawaited(_load());
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted && _tab == tab) _searchFocus.requestFocus();
      });
    }
  }

  Future<void> _load({bool more = false}) async {
    _debounce?.cancel();
    final generation = ++_generation;
    final auth = ref.read(authorizationHeaderProvider);
    final identity = ref
        .read(authControllerProvider.notifier)
        .gatewayRequestIdentity;
    final query = _search.text.trim();
    final tab = _tab;
    setState(() {
      _loading = true;
      _failed = false;
      _unavailable = false;
    });
    if (auth == null || identity == null) {
      setState(() {
        _loading = false;
        _failed = true;
      });
      return;
    }
    final gateway = ref.read(gatewayHttpClientProvider);
    bool current() =>
        mounted &&
        generation == _generation &&
        _tab == tab &&
        ref.read(authControllerProvider.notifier).gatewayRequestIdentity ==
            identity;
    try {
      if (tab == _PickerTab.stickers) {
        final result = await VoiceStickerPacksClient(
          gateway: gateway,
        ).listInstalled(authorization: auth);
        if (!current()) return;
        setState(() {
          _loading = false;
          switch (result) {
            case ChatsApiOk<StickerPackListData>(:final data):
              _packs = data.packs
                  .where((pack) => !pack.isPremium)
                  .toList(growable: false);
              _packId = _packs.firstOrNull?.id;
            case ChatsApiFailure():
              _failed = true;
          }
        });
      } else if (tab == _PickerTab.gifs) {
        final client = VoiceGifsClient(gateway: gateway);
        final cursor = more ? _cursor : '';
        final result = query.isEmpty
            ? await client.trending(authorization: auth, cursor: cursor)
            : await client.search(
                authorization: auth,
                query: query,
                cursor: cursor,
              );
        if (!current()) return;
        setState(() {
          _loading = false;
          switch (result) {
            case ChatsApiOk<GifCatalogPage>(:final data):
              final items = more ? [..._gifs, ...data.items] : data.items;
              _gifs = {
                for (final item in items)
                  (item.provider, item.providerId): item,
              }.values.toList(growable: false);
              _cursor = data.nextCursor;
            case ChatsApiFailure(:final statusCode):
              _failed = true;
              _unavailable = statusCode == 503;
          }
        });
      }
    } catch (_) {
      if (!current()) return;
      setState(() {
        _loading = false;
        _failed = true;
      });
    }
  }

  void _queryChanged(String value) {
    _debounce?.cancel();
    if (_tab == _PickerTab.stickers) {
      setState(() {});
      return;
    }
    _generation++;
    setState(() {
      _gifs = const [];
      _cursor = '';
      _failed = false;
      _loading = true;
    });
    _debounce = Timer(
      const Duration(milliseconds: 300),
      () => unawaited(_load()),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    ref.listen(_catalogIdentityProvider, (previous, next) {
      if (previous == next) return;
      _generation++;
      _debounce?.cancel();
      if (previous?.request?.profileId != next.request?.profileId ||
          previous?.request?.accountId != next.request?.accountId) {
        _search.clear();
      }
      setState(() {
        _packs = const [];
        _gifs = const [];
        _cursor = '';
        _loading = false;
        _failed = true;
        _unavailable = false;
      });
    });
    final labels = [
      l10n.composerEmojiPanelTitle,
      l10n.composerStickerTab,
      l10n.composerGifTab,
    ];
    return Material(
      key: const Key('composer_media_picker'),
      borderRadius: BorderRadius.circular(16),
      clipBehavior: Clip.antiAlias,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              for (final tab in _PickerTab.values)
                Expanded(
                  child: Semantics(
                    selected: _tab == tab,
                    child: TextButton(
                      autofocus: tab == _PickerTab.emoji,
                      style: TextButton.styleFrom(
                        foregroundColor: _tab == tab
                            ? Theme.of(context).colorScheme.primary
                            : Theme.of(context).colorScheme.onSurfaceVariant,
                        textStyle: Theme.of(context).textTheme.labelLarge
                            ?.copyWith(
                              fontWeight: _tab == tab
                                  ? FontWeight.w600
                                  : FontWeight.w400,
                            ),
                      ),
                      onPressed: () => _selectTab(tab),
                      child: Text(labels[tab.index]),
                    ),
                  ),
                ),
            ],
          ),
          if (_tab == _PickerTab.emoji)
            Flexible(
              child: ComposerEmojiPanelBody(
                title: l10n.composerEmojiPanelTitle,
                showTitle: false,
                recentEmoji: widget.recentEmoji,
                onSelected: widget.onEmojiSelected,
              ),
            )
          else ...[
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              child: TextField(
                key: const Key('composer_media_search'),
                controller: _search,
                focusNode: _searchFocus,
                autofocus: true,
                onChanged: _queryChanged,
                decoration: InputDecoration(
                  labelText: _tab == _PickerTab.gifs
                      ? l10n.composerGifSearch
                      : l10n.composerStickerSearch,
                  prefixIcon: const Icon(Icons.search),
                ),
              ),
            ),
            Padding(
              padding: const EdgeInsets.all(8),
              child: Text(
                l10n.composerMediaSendUnavailable,
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ),
            if (_failed)
              Semantics(
                liveRegion: true,
                child: Row(
                  children: [
                    Expanded(
                      child: Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 12),
                        child: Text(
                          _unavailable
                              ? l10n.composerGifUnavailable
                              : l10n.composerCatalogError,
                        ),
                      ),
                    ),
                    TextButton(
                      onPressed: _loading
                          ? null
                          : () => _load(more: _gifs.isNotEmpty),
                      child: Text(l10n.composerCatalogRetry),
                    ),
                  ],
                ),
              ),
            if (_loading) const LinearProgressIndicator(minHeight: 2),
            Flexible(
              child: _tab == _PickerTab.stickers
                  ? _stickerBody(l10n)
                  : _gifBody(l10n),
            ),
          ],
        ],
      ),
    );
  }

  Widget _stickerBody(AppLocalizations l10n) {
    final query = _search.text.trim().toLowerCase();
    final packs = _packs
        .where(
          (pack) =>
              query.isEmpty ||
              pack.title.toLowerCase().contains(query) ||
              pack.stickers.any((item) => item.emoji.contains(query)),
        )
        .toList();
    final selected =
        packs.where((pack) => pack.id == _packId).firstOrNull ??
        packs.firstOrNull;
    if (selected == null) return _empty(l10n.composerStickerEmpty);
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(
          height: 48,
          child: ListView(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 12),
            children: [
              for (final pack in packs)
                Padding(
                  padding: const EdgeInsets.only(right: 8),
                  child: ChoiceChip(
                    label: Text(pack.title),
                    selected: pack.id == selected.id,
                    onSelected: (_) => setState(() => _packId = pack.id),
                  ),
                ),
            ],
          ),
        ),
        Flexible(
          child: selected.stickers.isEmpty
              ? _empty(l10n.composerStickerEmpty)
              : GridView.count(
                  crossAxisCount: 4,
                  shrinkWrap: true,
                  padding: const EdgeInsets.all(12),
                  children: [
                    for (final item in selected.stickers)
                      Tooltip(
                        message: l10n.composerMediaSendUnavailable,
                        child: IconButton(
                          key: Key('sticker_result_${item.id}'),
                          onPressed: null,
                          icon: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              const Icon(Icons.image_not_supported_outlined),
                              if (item.emoji.isNotEmpty)
                                Text(
                                  item.emoji,
                                  style: VoiceEmojiStyle.textStyle(
                                    fontSize: 14,
                                  ),
                                ),
                            ],
                          ),
                        ),
                      ),
                  ],
                ),
        ),
      ],
    );
  }

  Widget _gifBody(AppLocalizations l10n) {
    if (_gifs.isEmpty && _cursor.isEmpty) return _empty(l10n.composerGifEmpty);
    return ListView(
      padding: const EdgeInsets.all(12),
      children: [
        GridView.count(
          crossAxisCount: 2,
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          children: [
            for (final item in _gifs)
              Tooltip(
                message: l10n.composerMediaSendUnavailable,
                child: IconButton(
                  key: Key('gif_result_${item.provider}_${item.providerId}'),
                  onPressed: null,
                  icon: _preview(item.previewUrl),
                ),
              ),
          ],
        ),
        if (_cursor.isNotEmpty)
          TextButton(
            onPressed: _loading ? null : () => _load(more: true),
            child: Text(l10n.composerCatalogMore),
          ),
      ],
    );
  }

  Widget _preview(String source) {
    final uri = Uri.tryParse(source);
    if (uri == null || uri.scheme != 'https' || uri.host.isEmpty) {
      return const Icon(Icons.gif_box_outlined, size: 48);
    }
    return Image.network(
      source,
      fit: BoxFit.contain,
      excludeFromSemantics: true,
      errorBuilder: (_, _, _) => const Icon(Icons.gif_box_outlined, size: 48),
    );
  }

  Widget _empty(String label) => Center(
    child: Padding(
      padding: const EdgeInsets.all(12),
      child: Text(_loading || _failed ? '' : label),
    ),
  );
}
