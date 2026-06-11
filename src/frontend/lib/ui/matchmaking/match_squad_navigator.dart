import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/matchmaking_client.dart';
import '../../state/matchmaking_providers.dart';
import 'match_squad_screen.dart';

/// Pushes [MatchSquadScreen] when [activeSquadMatchProvider] is set.
class MatchSquadNavigator extends ConsumerStatefulWidget {
  const MatchSquadNavigator({super.key, required this.child});

  final Widget child;

  @override
  ConsumerState<MatchSquadNavigator> createState() => _MatchSquadNavigatorState();
}

class _MatchSquadNavigatorState extends ConsumerState<MatchSquadNavigator> {
  @override
  Widget build(BuildContext context) {
    ref.listen<MatchData?>(activeSquadMatchProvider, (prev, next) {
      if (next == null) return;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        ref.read(activeSquadMatchProvider.notifier).state = null;
        Navigator.of(context).push(
          MaterialPageRoute<void>(
            builder: (_) => MatchSquadScreen(match: next),
          ),
        );
      });
    });
    return widget.child;
  }
}
