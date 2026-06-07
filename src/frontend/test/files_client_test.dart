import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/files_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

import 'support/gateway_test_client.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  test('requestUpload sends chat-scoped file upload request', () async {
    final client = VoiceFilesClient(
      gateway: gatewayHttpForTest(
        MockClient((req) async {
          expect(req.method, 'POST');
          expect(req.url.path, '/api/v1/files/upload');
          final body = jsonDecode(req.body) as Map<String, dynamic>;
          expect(body['original_name'], 'cat.png');
          expect(body['mime_type'], 'image/png');
          expect(body['size_bytes'], '3');
          expect(body['context_chat'], {
            'id': 'chat-1',
            'type': 'CHAT_TYPE_DM',
          });
          return http.Response(
            jsonEncode({
              'upload_response': {
                'file_id': 'file-1',
                'presigned_put_url': 'https://r2.example/upload',
                'r2_key': 'attachments/file-1/cat.png',
              },
            }),
            200,
          );
        }),
        config: config,
      ),
    );

    final result = await client.requestUpload(
      authorization: auth,
      originalName: 'cat.png',
      mimeType: 'image/png',
      sizeBytes: 3,
      chatId: 'chat-1',
    );

    expect(result, isA<FilesApiOk<FileUploadTicket>>());
    expect((result as FilesApiOk<FileUploadTicket>).data.fileId, 'file-1');
  });

  test('putBytes uploads bytes to presigned URL', () async {
    final bytes = Uint8List.fromList([1, 2, 3]);
    final client = VoiceFilesClient(
      gateway: gatewayHttpForTest(
        MockClient((req) async {
          expect(req.method, 'PUT');
          expect(req.url.toString(), 'https://r2.example/upload');
          expect(req.headers['Content-Type'], 'image/png');
          expect(req.headers['Content-Length'], '3');
          expect(req.bodyBytes, bytes);
          return http.Response('', 200);
        }),
        config: config,
      ),
    );

    final result = await client.putBytes(
      uploadUrl: Uri.parse('https://r2.example/upload'),
      bytes: bytes,
      mimeType: 'image/png',
    );

    expect(result, isA<FilesApiOk<void>>());
  });

  test('confirmUpload posts SHA-256 and parses metadata', () async {
    final bytes = Uint8List.fromList([1, 2, 3]);
    final client = VoiceFilesClient(
      gateway: gatewayHttpForTest(
        MockClient((req) async {
          expect(req.method, 'POST');
          expect(req.url.path, '/api/v1/files/file-1/confirm');
          final body = jsonDecode(req.body) as Map<String, dynamic>;
          expect(
            body['sha256_hash'],
            '039058c6f2c0cb492c533b0a4d14ef77cc0f78abccced5287d84a1a2011cfb81',
          );
          return http.Response(
            jsonEncode({
              'file_metadata': {
                'id': 'file-1',
                'status': 'ready',
                'file_type': 'image',
                'original_name': 'cat.png',
                'thumbnail_r2_key': 'processed/file-1/thumb.webp',
                'converted_r2_key': 'processed/file-1/full.webp',
                'r2_key': 'attachments/file-1/cat.png',
                'size_bytes': 3,
              },
            }),
            200,
          );
        }),
        config: config,
      ),
    );

    final result = await client.confirmUpload(
      authorization: auth,
      fileId: 'file-1',
      bytes: bytes,
    );

    expect(result, isA<FilesApiOk<FileMetadataData>>());
    final metadata = (result as FilesApiOk<FileMetadataData>).data;
    expect(metadata.previewUrl, isNull);
    expect(metadata.url, isNull);
    expect(metadata.fileId, 'file-1');
  });

  test('getFileUrl returns presigned_get_url', () async {
    final client = VoiceFilesClient(
      gateway: gatewayHttpForTest(
        MockClient((req) async {
          expect(req.method, 'GET');
          expect(req.url.path, '/api/v1/files/file-1/url');
          return http.Response(
            jsonEncode({
              'presigned_get_url': 'https://r2.example/get/file-1',
            }),
            200,
          );
        }),
        config: config,
      ),
    );

    final result = await client.getFileUrl(
      authorization: auth,
      fileId: 'file-1',
    );

    expect(result, isA<FilesApiOk<String>>());
    expect(
      (result as FilesApiOk<String>).data,
      'https://r2.example/get/file-1',
    );
  });
}
