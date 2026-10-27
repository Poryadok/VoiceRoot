import 'gateway_http.dart';
import '../gen/voice/space/v1/space.pb.dart' as pb;

/// Safe nonmember projection from the Space-owned catalog, never GetSpace.
class SpaceCatalogCard {
  const SpaceCatalogCard({
    required this.id,
    required this.name,
    this.description = '',
    this.iconUrl,
    this.memberCount = 0,
    this.isVerified = false,
    this.visibility = '',
  });
  final String id;
  final String name;
  final String description;
  final String? iconUrl;
  final int memberCount;
  final bool isVerified;
  final String visibility;
}

class SpaceCatalogPage {
  const SpaceCatalogPage({required this.spaces, this.nextCursor = ''});
  final List<SpaceCatalogCard> spaces;
  final String nextCursor;
}

class SpaceCatalogFailure implements Exception {
  const SpaceCatalogFailure();
}

class SpaceCatalogClient {
  SpaceCatalogClient(this.gateway);
  final GatewayHttpClient gateway;

  Future<SpaceCatalogPage> list({
    required String authorization,
    String query = '',
    String cursor = '',
  }) async {
    final result = await gateway.getProto(
      gateway.replace(
        path: '/api/v1/spaces/catalog',
        queryParameters: {
          if (query.trim().isNotEmpty) 'q': query.trim(),
          if (cursor.isNotEmpty) 'cursor': cursor,
          'page_size': '50',
        },
      ),
      authorization: authorization,
      createEmpty: pb.SearchPublicSpacesResponse.create,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => SpaceCatalogPage(
        spaces: [
          for (final s in data.spaceList.spaces)
            SpaceCatalogCard(
              id: s.id,
              name: s.name,
              description: s.description,
              iconUrl: s.hasIconUrl() ? s.iconUrl : null,
              memberCount: s.memberCount,
              isVerified: s.isVerified,
              visibility: s.visibility,
            ),
        ],
        nextCursor: data.spaceList.nextCursor,
      ),
      GatewayHttpFailure() => throw const SpaceCatalogFailure(),
    };
  }

  /// Empty-body public admission entrypoint. Visibility is only an affordance;
  /// Space rechecks guest, ban, limit and entry-policy requirements.
  Future<void> join({
    required String authorization,
    required String spaceId,
  }) async {
    final result = await gateway.postProto(
      uri: gateway.resolve(
        '/api/v1/spaces/${Uri.encodeComponent(spaceId)}/join',
      ),
      authorization: authorization,
      createEmpty: pb.JoinSpaceResponse.create,
    );
    switch (result) {
      case GatewayHttpOk(:final data):
        if (data.spaceMembership.spaceId != spaceId) {
          throw const SpaceCatalogFailure();
        }
      case GatewayHttpFailure():
        throw const SpaceCatalogFailure();
    }
  }
}
