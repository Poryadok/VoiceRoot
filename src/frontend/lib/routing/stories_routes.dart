import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../ui/stories/story_archive_screen.dart';
import '../ui/stories/story_create_screen.dart';
import '../ui/stories/story_highlights_screen.dart';
import '../ui/stories/story_viewer_screen.dart';

/// Route paths for story screens.
abstract final class VoiceAppRoutes {
  static const home = '/';
  static const storyCreate = '/stories/create';
  static const storyViewer = '/stories/viewer';
  static const storyArchive = '/stories/archive';
  static const storyHighlights = '/stories/highlights';
}

/// Navigation helpers for story flows (go_router).
abstract final class StoriesRoutes {
  static void openCreate(BuildContext context) {
    context.push(VoiceAppRoutes.storyCreate);
  }

  static void openArchive(BuildContext context) {
    context.push(VoiceAppRoutes.storyArchive);
  }

  static void openHighlights(BuildContext context, {required String profileId}) {
    context.push(
      Uri(
        path: VoiceAppRoutes.storyHighlights,
        queryParameters: {'profileId': profileId},
      ).toString(),
    );
  }

  static void openViewer(
    BuildContext context, {
    required List<String> storyIds,
    int initialIndex = 0,
    String? profileId,
  }) {
    if (storyIds.isEmpty) return;
    final params = <String, String>{
      'ids': storyIds.join(','),
      'index': '$initialIndex',
    };
    if (profileId != null && profileId.isNotEmpty) {
      params['profileId'] = profileId;
    }
    final uri = Uri(
      path: VoiceAppRoutes.storyViewer,
      queryParameters: params,
    );
    context.push(uri.toString());
  }

  static Future<void> openProfileStories(
    BuildContext context, {
    required String profileId,
    required List<String> storyIds,
  }) {
    openViewer(
      context,
      storyIds: storyIds,
      profileId: profileId,
    );
    return Future.value();
  }

  static List<GoRoute> routes({required GlobalKey<NavigatorState> rootKey}) {
    return [
      GoRoute(
        path: VoiceAppRoutes.storyCreate,
        parentNavigatorKey: rootKey,
        builder: (context, state) => const StoryCreateScreen(),
      ),
      GoRoute(
        path: VoiceAppRoutes.storyViewer,
        parentNavigatorKey: rootKey,
        builder: (context, state) {
          final idsParam = state.uri.queryParameters['ids'];
          final storyIds = idsParam == null || idsParam.isEmpty
              ? <String>[]
              : idsParam.split(',').where((id) => id.isNotEmpty).toList();
          final index =
              int.tryParse(state.uri.queryParameters['index'] ?? '0') ?? 0;
          final profileId = state.uri.queryParameters['profileId'];
          return StoryViewerScreen(
            storyIds: storyIds,
            initialIndex: index,
            profileId: profileId,
          );
        },
      ),
      GoRoute(
        path: VoiceAppRoutes.storyArchive,
        parentNavigatorKey: rootKey,
        builder: (context, state) => const StoryArchiveScreen(),
      ),
      GoRoute(
        path: VoiceAppRoutes.storyHighlights,
        parentNavigatorKey: rootKey,
        builder: (context, state) {
          final profileId = state.uri.queryParameters['profileId'] ?? '';
          return StoryHighlightsScreen(profileId: profileId);
        },
      ),
    ];
  }
}
