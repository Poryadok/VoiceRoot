import '../gen/voice/chat/v1/chat.pb.dart' as chat_pb;
import 'api_result.dart';
import 'chats_client.dart';
import 'gateway_http.dart';

/// Catalog identity and preview metadata do not authorize sending a message.
class GifCatalogItem {
  const GifCatalogItem({
    required this.provider,
    required this.providerId,
    required this.fileId,
    required this.previewUrl,
    required this.width,
    required this.height,
    required this.durationSeconds,
  });

  final String provider;
  final String providerId;
  final String fileId;
  final String previewUrl;
  final int width;
  final int height;
  final double durationSeconds;

  factory GifCatalogItem.fromProto(chat_pb.GifResult item) => GifCatalogItem(
    provider: item.provider,
    providerId: item.providerId,
    fileId: item.fileId,
    previewUrl: item.previewUrl,
    width: item.width,
    height: item.height,
    durationSeconds: item.durationSeconds,
  );
}

class GifCatalogPage {
  const GifCatalogPage({required this.items, required this.nextCursor});
  final List<GifCatalogItem> items;
  final String nextCursor;
}

class VoiceGifsClient {
  const VoiceGifsClient({required GatewayHttpClient gateway})
    : _gateway = gateway;
  final GatewayHttpClient _gateway;

  Future<ChatsApiResult<GifCatalogPage>> search({
    required String authorization,
    required String query,
    String cursor = '',
    int limit = 20,
  }) async {
    final result = await _gateway.getProto(
      _gateway
          .resolve('/api/v1/gifs/search')
          .replace(
            queryParameters: {
              'q': query,
              'limit': '$limit',
              if (cursor.isNotEmpty) 'cursor': cursor,
            },
          ),
      authorization: authorization,
      createEmpty: chat_pb.SearchGifsResponse.create,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => ChatsApiOk(
        GifCatalogPage(
          items: data.items
              .map(GifCatalogItem.fromProto)
              .toList(growable: false),
          nextCursor: data.nextCursor,
        ),
      ),
      GatewayHttpFailure(:final error) => ChatsApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  Future<ChatsApiResult<GifCatalogPage>> trending({
    required String authorization,
    String cursor = '',
    int limit = 20,
  }) async {
    final result = await _gateway.getProto(
      _gateway
          .resolve('/api/v1/gifs/trending')
          .replace(
            queryParameters: {
              'limit': '$limit',
              if (cursor.isNotEmpty) 'cursor': cursor,
            },
          ),
      authorization: authorization,
      createEmpty: chat_pb.GetTrendingGifsResponse.create,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => ChatsApiOk(
        GifCatalogPage(
          items: data.items
              .map(GifCatalogItem.fromProto)
              .toList(growable: false),
          nextCursor: data.nextCursor,
        ),
      ),
      GatewayHttpFailure(:final error) => ChatsApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}
