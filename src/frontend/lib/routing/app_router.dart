import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'stories_routes.dart';

final rootNavigatorKey = GlobalKey<NavigatorState>();

typedef VoiceShellBuilder = Widget Function(BuildContext context, GoRouterState state);

abstract final class VoiceAppRoutes {
  static const home = '/';
  static const invitePrefix = '/invite/';
}

/// Builds the authenticated app [GoRouter] (home shell + story overlays + deep links).
GoRouter createVoiceGoRouter({
  required VoiceShellBuilder shellBuilder,
  List<NavigatorObserver>? observers,
  void Function(GoRouterState state)? onDeepLinkPath,
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
      GoRoute(
        path: '/invite/:code',
        builder: (context, state) {
          onDeepLinkPath?.call(state);
          return shellBuilder(context, state);
        },
      ),
      GoRoute(
        path: '/s/:spaceId',
        builder: (context, state) {
          onDeepLinkPath?.call(state);
          return shellBuilder(context, state);
        },
      ),
      GoRoute(
        path: '/s/:spaceId/c/:chatId',
        builder: (context, state) {
          onDeepLinkPath?.call(state);
          return shellBuilder(context, state);
        },
      ),
      GoRoute(
        path: '/ch/:chatId',
        builder: (context, state) {
          onDeepLinkPath?.call(state);
          return shellBuilder(context, state);
        },
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
