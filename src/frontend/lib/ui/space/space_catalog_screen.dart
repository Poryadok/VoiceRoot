import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../backend/space_catalog_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/space_catalog_providers.dart';
import '../../state/space_providers.dart';
import '../../state/shell_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_avatar.dart';
import 'join_space_invite_sheet.dart';

class SpaceCatalogScreen extends ConsumerStatefulWidget {
  const SpaceCatalogScreen({super.key});
  static const searchKey = Key('space_catalog_search');
  static const moreKey = Key('space_catalog_more');
  static const previewKey = Key('space_catalog_preview');
  static Key joinKey(String id) => ValueKey('space_catalog_join_$id');
  static Key cardKey(String id) => ValueKey('space_catalog_card_$id');

  static Future<void> show(BuildContext context) => Navigator.of(
    context,
  ).push<void>(MaterialPageRoute(builder: (_) => const SpaceCatalogScreen()));

  @override
  ConsumerState<SpaceCatalogScreen> createState() => _SpaceCatalogScreenState();
}

class _SpaceCatalogScreenState extends ConsumerState<SpaceCatalogScreen> {
  final _search = TextEditingController();
  Timer? _debounce;
  String _query = '';
  final _cursors = <String>[''];
  bool _previewOpen = false;
  final _joining = <String>{};
  final _joined = <String>{};
  String? _joinError;
  int _actorGeneration = 0;

  Future<void> _join(SpaceCatalogCard card) async {
    final auth = ref.read(authorizationHeaderProvider);
    final generation = _actorGeneration;
    if (auth == null || _joining.contains(card.id)) return;
    setState(() {
      _joining.add(card.id);
      _joinError = null;
    });
    try {
      await ref
          .read(spaceCatalogClientProvider)
          .join(authorization: auth, spaceId: card.id);
      if (!mounted ||
          generation != _actorGeneration ||
          ref.read(authorizationHeaderProvider) != auth) {
        return;
      }
      ref.invalidate(mySpacesProvider);
      ref.invalidate(catalogMembershipsProvider);
      setState(() => _joined.add(card.id));
      _openSpace(card.id);
    } catch (_) {
      if (!mounted ||
          generation != _actorGeneration ||
          ref.read(authorizationHeaderProvider) != auth) {
        return;
      }
      // An uncertain response may still have admitted the viewer. Reconcile
      // membership before allowing a second user-initiated attempt.
      ref.invalidate(mySpacesProvider);
      ref.invalidate(catalogMembershipsProvider);
      setState(
        () => _joinError = AppLocalizations.of(context)!.spaceCatalogJoinError,
      );
    } finally {
      if (mounted && generation == _actorGeneration) {
        setState(() => _joining.remove(card.id));
      }
    }
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _search.dispose();
    super.dispose();
  }

  void _filter(String text) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 300), () {
      if (!mounted || _query == text.trim()) return;
      setState(() {
        _query = text.trim();
        _cursors
          ..clear()
          ..add('');
      });
    });
  }

  void _openSpace(String id) {
    // Catalog Open expresses an idempotent destination; the rail's selection
    // action intentionally toggles an already selected Space off.
    if (ref.read(selectedSpaceIdProvider) != id) {
      ref.read(shellNavigationProvider).selectSpace(id);
    }
    Navigator.of(context).maybePop();
  }

  void _preview(SpaceCatalogCard card) {
    _previewOpen = true;
    showDialog<void>(
      context: context,
      builder: (context) => Dialog(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 440),
          child: SingleChildScrollView(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                key: SpaceCatalogScreen.previewKey,
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Align(
                    alignment: Alignment.centerRight,
                    child: IconButton(
                      tooltip: AppLocalizations.of(context)!.spaceCatalogClose,
                      onPressed: () => Navigator.pop(context),
                      icon: const Icon(Icons.close),
                    ),
                  ),
                  _identity(card),
                  const SizedBox(height: 12),
                  Text(card.description),
                ],
              ),
            ),
          ),
        ),
      ),
    ).whenComplete(() => _previewOpen = false);
  }

  Widget _identity(SpaceCatalogCard card) {
    final l10n = AppLocalizations.of(context)!;
    return Row(
      children: [
        ExcludeSemantics(
          child: VoiceAvatar(label: card.name, imageUrl: card.iconUrl),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      card.name,
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                  ),
                  if (card.isVerified)
                    Tooltip(
                      message: l10n.spaceCatalogVerified,
                      child: Semantics(
                        label: l10n.spaceCatalogVerified,
                        child: Icon(
                          Icons.verified,
                          size: 18,
                          color: VoiceColors.of(context).info,
                        ),
                      ),
                    ),
                ],
              ),
              Text(
                l10n.spaceCatalogMembers(card.memberCount),
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _card(SpaceCatalogCard card, Set<String>? memberships) {
    final colors = VoiceColors.of(context);
    final l10n = AppLocalizations.of(context)!;
    return Container(
      key: SpaceCatalogScreen.cardKey(card.id),
      decoration: BoxDecoration(
        color: colors.canvas,
        border: Border.all(color: colors.borderDefault),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          InkWell(
            borderRadius: BorderRadius.circular(6),
            onTap: () => _preview(card),
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _identity(card),
                  const SizedBox(height: 8),
                  Text(
                    card.description,
                    maxLines: 3,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
          ),
          if ((memberships?.contains(card.id) ?? false) ||
              _joined.contains(card.id))
            Padding(
              padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
              child: FilledButton(
                onPressed: () => _openSpace(card.id),
                child: Text(l10n.spaceOpenAction),
              ),
            ),
          if (memberships != null &&
              !memberships.contains(card.id) &&
              !_joined.contains(card.id) &&
              (card.visibility == 'public' || card.visibility == 'invite_only'))
            Padding(
              padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
              child: FilledButton(
                key: SpaceCatalogScreen.joinKey(card.id),
                onPressed: _joining.contains(card.id)
                    ? null
                    : () => card.visibility == 'public'
                          ? _join(card)
                          : JoinSpaceInviteSheet.show(context),
                child: _joining.contains(card.id)
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : Text(
                        card.visibility == 'public'
                            ? l10n.spaceInviteJoinSubmit
                            : l10n.spaceCatalogInviteRequired,
                      ),
              ),
            ),
        ],
      ),
    );
  }

  Widget _error(String label, VoidCallback retry) => Padding(
    padding: const EdgeInsets.symmetric(vertical: 12),
    child: Column(
      children: [
        Semantics(liveRegion: true, child: Text(label)),
        TextButton(
          onPressed: retry,
          child: Text(AppLocalizations.of(context)!.commonRetry),
        ),
      ],
    ),
  );

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final memberships = ref.watch(catalogMembershipsProvider);
    ref.listen(authorizationHeaderProvider, (previous, next) {
      if (previous != next) {
        _actorGeneration++;
        if (_previewOpen) Navigator.of(context).pop();
        _joined.clear();
        _joining.clear();
        _joinError = null;
        setState(
          () => _cursors
            ..clear()
            ..add(''),
        );
      }
    });
    return Scaffold(
      backgroundColor: VoiceColors.of(context).canvas,
      appBar: AppBar(
        title: Text(l10n.spaceCatalogTitle),
        toolbarHeight: 56,
        backgroundColor: VoiceColors.of(context).canvas,
        surfaceTintColor: Colors.transparent,
        shape: Border(
          bottom: BorderSide(color: VoiceColors.of(context).borderDefault),
        ),
      ),
      body: FocusTraversalGroup(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            TextField(
              key: SpaceCatalogScreen.searchKey,
              controller: _search,
              onChanged: _filter,
              onSubmitted: (text) {
                _debounce?.cancel();
                setState(() {
                  _query = text.trim();
                  _cursors
                    ..clear()
                    ..add('');
                });
              },
              decoration: InputDecoration(
                labelText: l10n.spaceCatalogSearch,
                prefixIcon: const Icon(Icons.search),
                suffixIcon: IconButton(
                  tooltip: l10n.spaceCatalogClearSearch,
                  onPressed: () {
                    _search.clear();
                    _filter('');
                  },
                  icon: const Icon(Icons.clear),
                ),
              ),
            ),
            const SizedBox(height: 12),
            if (_joinError != null)
              Semantics(
                liveRegion: true,
                child: Text(
                  _joinError!,
                  style: TextStyle(color: VoiceColors.of(context).error),
                ),
              ),
            if (memberships.hasError)
              _error(
                l10n.spaceCatalogMembershipError,
                () => ref.invalidate(catalogMembershipsProvider),
              ),
            for (final cursor in _cursors)
              Consumer(
                builder: (context, ref, child) {
                  final request = (query: _query, cursor: cursor);
                  final page = ref.watch(spaceCatalogProvider(request));
                  return page.when(
                    skipLoadingOnRefresh: false,
                    skipLoadingOnReload: false,
                    loading: () => const Padding(
                      padding: EdgeInsets.all(24),
                      child: Center(child: CircularProgressIndicator()),
                    ),
                    error: (_, _) => _error(
                      l10n.spaceCatalogLoadError,
                      () => ref.invalidate(spaceCatalogProvider(request)),
                    ),
                    data: (data) => Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        if (data.spaces.isEmpty && cursor.isEmpty)
                          Padding(
                            padding: const EdgeInsets.all(24),
                            child: Text(
                              l10n.spaceCatalogEmpty,
                              textAlign: TextAlign.center,
                            ),
                          ),
                        LayoutBuilder(
                          builder: (context, constraints) {
                            final width = constraints.maxWidth >= 700
                                ? (constraints.maxWidth - 12) / 2
                                : constraints.maxWidth;
                            return Wrap(
                              spacing: 12,
                              runSpacing: 12,
                              children: [
                                for (final card in data.spaces)
                                  SizedBox(
                                    width: width,
                                    child: _card(
                                      card,
                                      memberships.isLoading ||
                                              memberships.hasError
                                          ? null
                                          : memberships.valueOrNull,
                                    ),
                                  ),
                              ],
                            );
                          },
                        ),
                        if (data.nextCursor.isNotEmpty &&
                            !_cursors.contains(data.nextCursor))
                          Align(
                            alignment: Alignment.center,
                            child: TextButton(
                              key: SpaceCatalogScreen.moreKey,
                              onPressed: () =>
                                  setState(() => _cursors.add(data.nextCursor)),
                              child: Text(l10n.spaceCatalogMore),
                            ),
                          ),
                        const SizedBox(height: 12),
                      ],
                    ),
                  );
                },
              ),
          ],
        ),
      ),
    );
  }
}
