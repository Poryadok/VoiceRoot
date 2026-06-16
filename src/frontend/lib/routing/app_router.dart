import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'stories_routes.dart';

final rootNavigatorKey = GlobalKey<NavigatorState>();

typedef VoiceShellBuilder = Widget Function(BuildContext context, GoRouterState state);

/// Builds the authenticated app [GoRouter] (home shell + story overlays).
GoRouter createVoiceGoRouter({
  required VoiceShellBuilder shellBuilder,
  List<NavigatorObserver>? observers,
}) {
  return GoRouter(
    navigatorKey: rootNavigatorKey,
    initialLocation: VoiceAppRoutes.home,
    observers: observers,
    routes: [
      GoRoute(
        path: VoiceAppRoutes.home,
        builder: shellBuilder,
      ),
      ...StoriesRoutes.routes(rootKey: rootNavigatorKey),
    ],
  );
}

final voiceGoRouterProvider = Provider<GoRouter>((ref) {
  throw UnimplementedError(
    'voiceGoRouterProvider must be overridden in VoiceApp',
  );
});
