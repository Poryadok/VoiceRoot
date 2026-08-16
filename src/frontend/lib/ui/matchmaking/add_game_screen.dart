import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/matchmaking_providers.dart';
import '../api_error_messages.dart';

/// Wizard to submit a user game-catalog request (docs/features/game-catalog.md §AddGame).
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

class _AddGameScreenState extends ConsumerState<AddGameScreen> {
  final _nameController = TextEditingController();
  final _modeController = TextEditingController();
  final _slotsController = TextEditingController(text: '10');
  bool _submitting = false;
  String? _error;
  List<CatalogGame> _similar = const [];

  @override
  void dispose() {
    _nameController.dispose();
    _modeController.dispose();
    _slotsController.dispose();
    super.dispose();
  }

  Future<void> _onNameChanged(String value) async {
    final query = value.trim();
    if (query.length < 2) {
      setState(() => _similar = const []);
      return;
    }
    final auth = ref.read(authControllerProvider);
    final token = auth.session?.accessToken;
    if (token == null || token.isEmpty) return;
    final client = ref.read(voiceMatchmakingClientProvider);
    final result = await client.searchGames(
      authorization: 'Bearer $token',
      query: query,
    );
    if (!mounted) return;
    if (result case MatchmakingApiOk(:final data)) {
      setState(() => _similar = data.games.take(5).toList());
    }
  }

  Future<void> _submit() async {
    final l10n = AppLocalizations.of(context)!;
    final name = _nameController.text.trim();
    final mode = _modeController.text.trim();
    final slots = int.tryParse(_slotsController.text.trim()) ?? 0;
    if (name.isEmpty || mode.isEmpty || slots < 1) {
      setState(() => _error = l10n.addGameSubmitError);
      return;
    }
    final auth = ref.read(authControllerProvider);
    final token = auth.session?.accessToken;
    if (token == null || token.isEmpty) {
      setState(() => _error = l10n.addGameSubmitError);
      return;
    }

    final config = {
      'regions': ['eu'],
      'modes': [
        {
          'name': mode,
          'slots': slots,
          'party_size_min': 1,
          'party_size_max': slots < 5 ? slots : 5,
          'roles_required': false,
          'rank_required': false,
          'roles': <Map<String, dynamic>>[],
          'ranks': <Map<String, dynamic>>[],
        },
      ],
    };

    setState(() {
      _submitting = true;
      _error = null;
    });
    final client = ref.read(voiceMatchmakingClientProvider);
    final result = await client.submitGameRequest(
      authorization: 'Bearer $token',
      name: name,
      configJson: jsonEncode(config),
    );
    if (!mounted) return;
    setState(() => _submitting = false);
    switch (result) {
      case MatchmakingApiOk():
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.addGameSubmitted)),
        );
        Navigator.of(context).pop(true);
      case MatchmakingApiFailure(:final message):
        setState(() => _error = gameCatalogErrorMessage(l10n, Exception(message)));
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return Scaffold(
      key: AddGameScreen.screenKey,
      appBar: AppBar(title: Text(l10n.addGameTitle)),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          TextField(
            key: AddGameScreen.nameFieldKey,
            controller: _nameController,
            decoration: InputDecoration(labelText: l10n.addGameNameLabel),
            onChanged: (v) => _onNameChanged(v),
          ),
          if (_similar.isNotEmpty) ...[
            const SizedBox(height: 8),
            Text(l10n.addGameSimilarHint, style: Theme.of(context).textTheme.labelLarge),
            for (final g in _similar)
              ListTile(
                dense: true,
                title: Text(g.name),
                leading: const Icon(Icons.sports_esports_outlined, size: 20),
              ),
          ],
          const SizedBox(height: 12),
          TextField(
            key: AddGameScreen.modeFieldKey,
            controller: _modeController,
            decoration: InputDecoration(labelText: l10n.addGameModeLabel),
          ),
          const SizedBox(height: 12),
          TextField(
            key: AddGameScreen.slotsFieldKey,
            controller: _slotsController,
            keyboardType: TextInputType.number,
            decoration: InputDecoration(labelText: l10n.addGameModeSlotsLabel),
          ),
          if (_error != null) ...[
            const SizedBox(height: 12),
            Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
          ],
          const SizedBox(height: 24),
          FilledButton(
            key: AddGameScreen.submitKey,
            onPressed: _submitting ? null : _submit,
            child: Text(l10n.addGameSubmit),
          ),
        ],
      ),
    );
  }
}
