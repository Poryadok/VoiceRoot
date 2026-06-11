import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/matchmaking_providers.dart';
import '../../theme/voice_colors.dart';

/// Solo queue search: criteria form, searching state, cancel.
class QueueSearchScreen extends ConsumerStatefulWidget {
  const QueueSearchScreen({
    super.key,
    required this.game,
    required this.mode,
  });

  static const Key screenKey = Key('queue_search_screen');
  static const Key startButtonKey = Key('queue_search_start');
  static const Key cancelButtonKey = Key('queue_search_cancel');
  static const Key searchingStateKey = Key('queue_search_searching');

  final CatalogGame game;
  final GameMode mode;

  @override
  ConsumerState<QueueSearchScreen> createState() => _QueueSearchScreenState();
}

class _QueueSearchScreenState extends ConsumerState<QueueSearchScreen> {
  String? _region;
  String? _role;
  String? _rank;
  String? _soughtMin;
  String? _soughtMax;
  SearchSessionData? _session;
  var _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    final regions = widget.game.config.regions;
    if (regions.isNotEmpty) _region = regions.first;
    if (widget.mode.roles.isNotEmpty) _role = widget.mode.roles.first.name;
    if (widget.mode.ranks.isNotEmpty) {
      _rank = widget.mode.ranks.first.name;
      _soughtMin = widget.mode.ranks.first.name;
      _soughtMax = widget.mode.ranks.last.name;
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final searching = _session != null && _session!.status == 'searching';

    return Scaffold(
      key: QueueSearchScreen.screenKey,
      appBar: AppBar(title: Text(l10n.queueSearchTitle)),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Text(widget.game.name, style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 4),
          Text(widget.mode.name, style: Theme.of(context).textTheme.bodyMedium),
          const SizedBox(height: 16),
          if (searching) ...[
            Card(
              key: QueueSearchScreen.searchingStateKey,
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(l10n.queueSearchSearching),
                    const SizedBox(height: 12),
                    FilledButton(
                      key: QueueSearchScreen.cancelButtonKey,
                      onPressed: _busy ? null : _cancelSearch,
                      child: Text(l10n.queueSearchCancel),
                    ),
                  ],
                ),
              ),
            ),
          ] else ...[
            _dropdown(
              label: l10n.queueSearchRegion,
              value: _region,
              items: widget.game.config.regions,
              onChanged: (v) => setState(() => _region = v),
            ),
            if (widget.mode.rolesRequired && widget.mode.roles.isNotEmpty)
              _dropdown(
                label: l10n.queueSearchRole,
                value: _role,
                items: widget.mode.roles.map((r) => r.name).toList(),
                onChanged: (v) => setState(() => _role = v),
              ),
            if (widget.mode.rankRequired && widget.mode.ranks.isNotEmpty) ...[
              _dropdown(
                label: l10n.queueSearchRank,
                value: _rank,
                items: widget.mode.ranks.map((r) => r.name).toList(),
                onChanged: (v) => setState(() => _rank = v),
              ),
              _dropdown(
                label: l10n.queueSearchSoughtRankMin,
                value: _soughtMin,
                items: widget.mode.ranks.map((r) => r.name).toList(),
                onChanged: (v) => setState(() => _soughtMin = v),
              ),
              _dropdown(
                label: l10n.queueSearchSoughtRankMax,
                value: _soughtMax,
                items: widget.mode.ranks.map((r) => r.name).toList(),
                onChanged: (v) => setState(() => _soughtMax = v),
              ),
            ],
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(_error!, style: TextStyle(color: voice.error)),
            ],
            const SizedBox(height: 16),
            FilledButton(
              key: QueueSearchScreen.startButtonKey,
              onPressed: _busy ? null : _startSearch,
              child: Text(l10n.queueSearchStart),
            ),
          ],
        ],
      ),
    );
  }

  Widget _dropdown({
    required String label,
    required String? value,
    required List<String> items,
    required ValueChanged<String?> onChanged,
  }) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: DropdownButtonFormField<String>(
        initialValue: value,
        decoration: InputDecoration(labelText: label),
        items: [
          for (final item in items)
            DropdownMenuItem(value: item, child: Text(item)),
        ],
        onChanged: onChanged,
      ),
    );
  }

  Map<String, dynamic> _buildCriteria() {
    final criteria = <String, dynamic>{
      'region': _region,
      'self': <String, dynamic>{},
    };
    if (_role != null && _role!.isNotEmpty) {
      (criteria['self'] as Map<String, dynamic>)['role'] = _role;
    }
    if (_rank != null && _rank!.isNotEmpty) {
      (criteria['self'] as Map<String, dynamic>)['rank'] = _rank;
    }
    if (_soughtMin != null && _soughtMax != null) {
      criteria['sought'] = {
        'rank_min': _soughtMin,
        'rank_max': _soughtMax,
      };
    }
    return criteria;
  }

  Future<void> _startSearch() async {
    final l10n = AppLocalizations.of(context)!;
    final token = ref.read(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    final client = ref.read(voiceMatchmakingClientProvider);
    final result = await client.startSearch(
      authorization: 'Bearer $token',
      gameId: widget.game.id,
      mode: widget.mode.name,
      criteria: _buildCriteria(),
    );
    if (!mounted) return;
    switch (result) {
      case MatchmakingApiOk(:final data):
        setState(() {
          _session = data;
          _busy = false;
        });
        ref.read(activeSearchSessionProvider.notifier).state = data;
      case MatchmakingApiFailure():
        setState(() {
          _busy = false;
          _error = l10n.queueSearchStartError;
        });
    }
  }

  Future<void> _cancelSearch() async {
    final l10n = AppLocalizations.of(context)!;
    final session = _session;
    if (session == null) return;
    final token = ref.read(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return;
    setState(() => _busy = true);
    final client = ref.read(voiceMatchmakingClientProvider);
    final result = await client.cancelSearch(
      authorization: 'Bearer $token',
      sessionId: session.id,
    );
    if (!mounted) return;
    switch (result) {
      case MatchmakingApiOk():
        setState(() {
          _session = null;
          _busy = false;
        });
        ref.read(activeSearchSessionProvider.notifier).state = null;
      case MatchmakingApiFailure():
        setState(() {
          _busy = false;
          _error = l10n.queueSearchCancelError;
        });
    }
  }
}
