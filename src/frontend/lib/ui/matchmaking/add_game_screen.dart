import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/matchmaking_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';
import '../api_error_messages.dart';

/// User catalog requests remain pending until staff moderation.
class AddGameScreen extends ConsumerStatefulWidget {
  const AddGameScreen({super.key});
  static const Key screenKey = Key('add_game_screen');
  static const Key nameFieldKey = Key('add_game_name');
  static const Key modeFieldKey = Key('add_game_mode');
  static const Key slotsFieldKey = Key('add_game_slots');
  static const Key submitKey = Key('add_game_submit');

  @override
  ConsumerState<AddGameScreen> createState() => _AddGameScreenState();
}

class _ModeDraft {
  _ModeDraft(this.id);
  final int id;
  final name = TextEditingController();
  final slots = TextEditingController(text: '10');
  bool get valid =>
      name.text.trim().isNotEmpty && (int.tryParse(slots.text.trim()) ?? 0) > 0;
  Map<String, dynamic> toJson() {
    final count = int.parse(slots.text.trim());
    return {
      'name': name.text.trim(),
      'slots': count,
      'party_size_min': 1,
      'party_size_max': count < 5 ? count : 5,
      'roles_required': false,
      'rank_required': false,
      'roles': <Map<String, dynamic>>[],
      'ranks': <Map<String, dynamic>>[],
    };
  }

  void dispose() {
    name.dispose();
    slots.dispose();
  }
}

class _AddGameScreenState extends ConsumerState<AddGameScreen> {
  final _nameController = TextEditingController();
  final _modes = <_ModeDraft>[_ModeDraft(0)];
  int _nextMode = 1;
  int _searchGeneration = 0;
  bool _submitting = false;
  bool _searching = false;
  bool _searched = false;
  bool _searchFailed = false;
  String? _error;
  List<CatalogGame> _similar = const [];
  bool get _valid =>
      _nameController.text.trim().isNotEmpty &&
      _modes.isNotEmpty &&
      _modes.every((mode) => mode.valid) &&
      _modes.map((mode) => mode.name.text.trim()).toSet().length ==
          _modes.length;

  @override
  void dispose() {
    _nameController.dispose();
    for (final mode in _modes) {
      mode.dispose();
    }
    super.dispose();
  }

  Future<void> _onNameChanged(String value) async {
    final generation = ++_searchGeneration;
    final query = value.trim();
    final session = ref.read(authControllerProvider).session;
    setState(() {
      _similar = const [];
      _searched = false;
      _searchFailed = false;
      _searching = query.length >= 2 && session != null;
    });
    if (!_searching) return;
    final result = await ref
        .read(voiceMatchmakingClientProvider)
        .searchGames(
          authorization: 'Bearer ${session!.accessToken}',
          query: query,
        );
    if (!mounted ||
        generation != _searchGeneration ||
        ref.read(authControllerProvider).session != session) {
      return;
    }
    setState(() {
      _searching = false;
      _searched = true;
      switch (result) {
        case MatchmakingApiOk(:final data):
          _similar = data.games.take(5).toList();
        case MatchmakingApiFailure():
          _searchFailed = true;
      }
    });
  }

  Future<void> _submit() async {
    if (_submitting || !_valid) return;
    final l10n = AppLocalizations.of(context)!;
    final session = ref.read(authControllerProvider).session;
    if (session == null || session.accessToken.isEmpty) return;
    setState(() {
      _submitting = true;
      _error = null;
    });
    final result = await ref
        .read(voiceMatchmakingClientProvider)
        .submitGameRequest(
          authorization: 'Bearer ${session.accessToken}',
          name: _nameController.text.trim(),
          configJson: jsonEncode({
            'regions': ['eu'],
            'modes': _modes.map((mode) => mode.toJson()).toList(),
          }),
        );
    if (!mounted) return;
    setState(() => _submitting = false);
    if (ref.read(authControllerProvider).session != session) {
      setState(() => _error = l10n.commonActionFailed);
      return;
    }
    switch (result) {
      case MatchmakingApiOk():
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(l10n.addGameSubmitted)));
        Navigator.of(context).pop(true);
      case MatchmakingApiFailure(:final message):
        setState(
          () => _error = gameCatalogErrorMessage(l10n, Exception(message)),
        );
    }
  }

  Widget _card(Widget child) {
    final metrics = context.voiceMetrics;
    final colors = VoiceColors.of(context);
    return Container(
      padding: metrics.inset('16', fallback: 16),
      decoration: BoxDecoration(
        color: colors.surface,
        border: Border.all(color: colors.borderDefault),
        borderRadius: BorderRadius.circular(metrics.corner('lg', fallback: 8)),
      ),
      child: child,
    );
  }

  Widget _modeCard(_ModeDraft mode, AppLocalizations l10n) => _card(
    Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            const Icon(Icons.sports_esports_outlined),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                l10n.addGameModeLabel,
                style: Theme.of(context).textTheme.titleMedium,
              ),
            ),
            IconButton(
              key: Key('add_game_remove_mode_${mode.id}'),
              tooltip: l10n.addGameRemoveMode,
              onPressed: _submitting
                  ? null
                  : () {
                      setState(() => _modes.remove(mode));
                      WidgetsBinding.instance.addPostFrameCallback(
                        (_) => mode.dispose(),
                      );
                    },
              icon: const Icon(Icons.close),
            ),
          ],
        ),
        const SizedBox(height: 8),
        _columns(
          TextField(
            key: mode.id == 0
                ? AddGameScreen.modeFieldKey
                : Key('add_game_mode_${mode.id}'),
            controller: mode.name,
            enabled: !_submitting,
            textInputAction: TextInputAction.next,
            onChanged: (_) => setState(() {}),
            decoration: InputDecoration(
              labelText: l10n.addGameModeLabel,
              errorText:
                  mode.name.text.trim().isNotEmpty &&
                      _modes
                              .where(
                                (other) =>
                                    other.name.text.trim() ==
                                    mode.name.text.trim(),
                              )
                              .length >
                          1
                  ? l10n.addGameModeDuplicate
                  : null,
            ),
          ),
          TextField(
            key: mode.id == 0
                ? AddGameScreen.slotsFieldKey
                : Key('add_game_slots_${mode.id}'),
            controller: mode.slots,
            enabled: !_submitting,
            keyboardType: TextInputType.number,
            onChanged: (_) => setState(() {}),
            decoration: InputDecoration(
              labelText: l10n.addGameModeSlotsLabel,
              errorText: (int.tryParse(mode.slots.text.trim()) ?? 0) <= 0
                  ? l10n.addGameSlotsInvalid
                  : null,
            ),
          ),
        ),
        const SizedBox(height: 12),
        Text(
          l10n.addGameCriteriaUnavailable,
          style: TextStyle(color: VoiceColors.of(context).textSecondary),
        ),
        const SizedBox(height: 8),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            OutlinedButton.icon(
              onPressed: null,
              icon: const Icon(Icons.numbers),
              label: Text(l10n.addGameNumericCriterion),
            ),
            OutlinedButton.icon(
              onPressed: null,
              icon: const Icon(Icons.list),
              label: Text(l10n.addGameEnumCriterion),
            ),
          ],
        ),
      ],
    ),
  );

  Widget _columns(Widget left, Widget right) => LayoutBuilder(
    builder: (context, constraints) {
      if (constraints.maxWidth < 560) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [left, const SizedBox(height: 12), right],
        );
      }
      return Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(child: left),
          const SizedBox(width: 12),
          Expanded(child: right),
        ],
      );
    },
  );

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final session = ref.watch(authControllerProvider).session;
    final metrics = context.voiceMetrics;
    return PopScope(
      canPop: !_submitting,
      child: Scaffold(
        key: AddGameScreen.screenKey,
        backgroundColor: VoiceColors.of(context).canvas,
        appBar: AppBar(
          automaticallyImplyLeading: false,
          title: Text(l10n.addGameTitle),
          actions: [
            IconButton(
              tooltip: MaterialLocalizations.of(context).closeButtonTooltip,
              onPressed: _submitting
                  ? null
                  : () => Navigator.of(context).maybePop(),
              icon: const Icon(Icons.close),
            ),
          ],
        ),
        body: FocusTraversalGroup(
          child: ListView(
            padding: metrics.inset('16', fallback: 16),
            children: [
              _columns(
                TextField(
                  key: AddGameScreen.nameFieldKey,
                  controller: _nameController,
                  enabled: !_submitting,
                  textInputAction: TextInputAction.next,
                  decoration: InputDecoration(labelText: l10n.addGameNameLabel),
                  onChanged: _onNameChanged,
                ),
                Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(
                      l10n.addGameIcon,
                      style: Theme.of(context).textTheme.labelLarge,
                    ),
                    OutlinedButton.icon(
                      onPressed: null,
                      icon: const Icon(Icons.image_outlined),
                      label: Text(l10n.addGameUploadIcon),
                    ),
                    Text(
                      l10n.addGameIconUnavailable,
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ],
                ),
              ),
              if (_searching || _searched) ...[
                const SizedBox(height: 12),
                _card(
                  Semantics(
                    liveRegion: true,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Text(
                          l10n.addGameSimilarHint,
                          style: Theme.of(context).textTheme.titleSmall,
                        ),
                        if (_searching)
                          LinearProgressIndicator(
                            semanticsLabel: l10n.addGameSimilarHint,
                          )
                        else if (_searchFailed) ...[
                          Text(l10n.gameCatalogLoadError),
                          Align(
                            alignment: AlignmentDirectional.centerStart,
                            child: TextButton(
                              onPressed: _submitting
                                  ? null
                                  : () => _onNameChanged(_nameController.text),
                              child: Text(l10n.commonRetry),
                            ),
                          ),
                        ] else if (_similar.isEmpty)
                          Text(l10n.addGameSearchEmpty)
                        else
                          for (final game in _similar)
                            ListTile(
                              dense: true,
                              contentPadding: EdgeInsets.zero,
                              leading: const Icon(
                                Icons.sports_esports_outlined,
                              ),
                              title: Text(game.name),
                            ),
                      ],
                    ),
                  ),
                ),
              ],
              for (final mode in _modes) ...[
                const SizedBox(height: 12),
                KeyedSubtree(
                  key: ValueKey(mode.id),
                  child: _modeCard(mode, l10n),
                ),
              ],
              const SizedBox(height: 12),
              Align(
                alignment: AlignmentDirectional.centerStart,
                child: OutlinedButton.icon(
                  key: const Key('add_game_add_mode'),
                  onPressed: _submitting
                      ? null
                      : () =>
                            setState(() => _modes.add(_ModeDraft(_nextMode++))),
                  icon: const Icon(Icons.add),
                  label: Text(l10n.addGameAddMode),
                ),
              ),
              const SizedBox(height: 12),
              Text(
                l10n.addGameModerationHint,
                style: TextStyle(color: VoiceColors.of(context).textSecondary),
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Semantics(
                  liveRegion: true,
                  child: Text(
                    _error!,
                    style: TextStyle(
                      color: Theme.of(context).colorScheme.error,
                    ),
                  ),
                ),
              ],
              const SizedBox(height: 16),
              FilledButton(
                key: AddGameScreen.submitKey,
                onPressed:
                    _submitting ||
                        !_valid ||
                        session == null ||
                        session.accessToken.isEmpty
                    ? null
                    : _submit,
                child: _submitting
                    ? SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          semanticsLabel: l10n.addGameSubmit,
                        ),
                      )
                    : Text(l10n.addGameSubmit),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
