import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../backend/space_catalog_client.dart';
import '../backend/spaces_client.dart';
import 'auth_providers.dart';
import 'space_providers.dart';

final spaceCatalogClientProvider = Provider(
  (ref) => SpaceCatalogClient(ref.watch(gatewayHttpClientProvider)),
);

final spaceCatalogProvider = FutureProvider.autoDispose
    .family<SpaceCatalogPage, ({String query, String cursor})>((
      ref,
      request,
    ) async {
      final auth = ref.watch(authorizationHeaderProvider);
      if (auth == null) throw const SpaceCatalogFailure();
      return ref
          .watch(spaceCatalogClientProvider)
          .list(
            authorization: auth,
            query: request.query,
            cursor: request.cursor,
          );
    });

/// Read every membership page before choosing Join/Open. A first page alone
/// cannot establish that the viewer is not a member of a catalog card.
final catalogMembershipsProvider = FutureProvider.autoDispose<Set<String>>((
  ref,
) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) throw const SpaceCatalogFailure();
  final client = ref.watch(voiceSpacesClientProvider);
  final firstPage = await ref.watch(mySpacesProvider.future);
  final ids = firstPage.spaces.map((s) => s.id).toSet();
  final seen = <String>{};
  String? cursor = firstPage.nextCursor;
  while (cursor != null && cursor.isNotEmpty) {
    if (!seen.add(cursor)) throw const SpaceCatalogFailure();
    final result = await client.listMySpaces(
      authorization: auth,
      cursor: cursor,
    );
    switch (result) {
      case SpacesApiOk(:final data):
        ids.addAll(data.spaces.map((s) => s.id));
        cursor = data.nextCursor;
      case SpacesApiFailure():
        throw const SpaceCatalogFailure();
    }
  }
  return ids;
});
