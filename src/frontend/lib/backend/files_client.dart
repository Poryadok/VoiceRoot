import 'dart:convert';
import 'dart:typed_data';

import 'package:crypto/crypto.dart' as crypto;
import 'package:http/http.dart' as http;

import 'gateway_config.dart';

sealed class FilesApiResult<T> {
  const FilesApiResult();
}

final class FilesApiOk<T> extends FilesApiResult<T> {
  const FilesApiOk(this.data);
  final T data;
}

final class FilesApiFailure extends FilesApiResult<Never> {
  const FilesApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class FileUploadTicket {
  const FileUploadTicket({
    required this.fileId,
    required this.presignedPutUrl,
    required this.r2Key,
  });

  final String fileId;
  final Uri presignedPutUrl;
  final String r2Key;

  factory FileUploadTicket.fromJson(Map<String, dynamic> json) {
    final upload = json['upload_response'] as Map<String, dynamic>? ?? {};
    return FileUploadTicket(
      fileId: upload['file_id'] as String? ?? '',
      presignedPutUrl: Uri.parse(upload['presigned_put_url'] as String? ?? ''),
      r2Key: upload['r2_key'] as String? ?? '',
    );
  }
}

class FileMetadataData {
  const FileMetadataData({
    required this.fileId,
    required this.fileType,
    required this.status,
    required this.originalName,
    this.url,
    this.previewUrl,
    this.sizeBytes,
  });

  final String fileId;
  final String fileType;
  final String status;
  final String originalName;
  final String? url;
  final String? previewUrl;
  final int? sizeBytes;

  factory FileMetadataData.fromJson(Map<String, dynamic> json) {
    final meta = json['file_metadata'] as Map<String, dynamic>? ?? json;
    return FileMetadataData(
      fileId: meta['id'] as String? ?? '',
      fileType: meta['file_type'] as String? ?? 'other',
      status: meta['status'] as String? ?? '',
      originalName: meta['original_name'] as String? ?? '',
      url: meta['converted_r2_key'] as String? ?? meta['r2_key'] as String?,
      previewUrl: meta['thumbnail_r2_key'] as String?,
      sizeBytes: (meta['size_bytes'] as num?)?.toInt(),
    );
  }
}

class VoiceFilesClient {
  VoiceFilesClient({
    required http.Client httpClient,
    required GatewayConfig config,
  }) : _http = httpClient,
       _config = config;

  final http.Client _http;
  final GatewayConfig _config;

  Future<FilesApiResult<FileUploadTicket>> requestUpload({
    required String authorization,
    required String originalName,
    required String mimeType,
    required int sizeBytes,
    String? chatId,
  }) async {
    if (!_config.hasBaseUrl) {
      return const FilesApiFailure(message: 'missing base URL');
    }
    final body = <String, dynamic>{
      'original_name': originalName,
      'mime_type': mimeType,
      'size_bytes': sizeBytes,
    };
    if (chatId != null && chatId.isNotEmpty) {
      body['context_chat'] = {'id': chatId, 'type': 'CHAT_TYPE_DM'};
    }
    final uri = Uri.parse(_config.baseUrl).resolve('/api/v1/files/upload');
    return _postJson(uri, authorization, body, FileUploadTicket.fromJson);
  }

  Future<FilesApiResult<FileMetadataData>> confirmUpload({
    required String authorization,
    required String fileId,
    required Uint8List bytes,
  }) async {
    if (!_config.hasBaseUrl) {
      return const FilesApiFailure(message: 'missing base URL');
    }
    final uri = Uri.parse(
      _config.baseUrl,
    ).resolve('/api/v1/files/$fileId/confirm');
    return _postJson(uri, authorization, {
      'sha256_hash': sha256Hex(bytes),
    }, FileMetadataData.fromJson);
  }

  Future<FilesApiResult<void>> putBytes({
    required Uri uploadUrl,
    required Uint8List bytes,
    required String mimeType,
  }) async {
    try {
      final res = await _http.put(
        uploadUrl,
        headers: {
          'Content-Type': mimeType,
          'Content-Length': '${bytes.length}',
        },
        body: bytes,
      );
      if (res.statusCode >= 200 && res.statusCode < 300) {
        return const FilesApiOk(null);
      }
      return FilesApiFailure(
        message: 'HTTP ${res.statusCode}',
        statusCode: res.statusCode,
      );
    } catch (e) {
      return FilesApiFailure(message: '$e');
    }
  }

  static String sha256Hex(Uint8List bytes) {
    return crypto.sha256.convert(bytes).toString();
  }

  Future<FilesApiResult<T>> _postJson<T>(
    Uri uri,
    String authorization,
    Map<String, dynamic> body,
    T Function(Map<String, dynamic>) parse,
  ) async {
    try {
      final res = await _http.post(
        uri,
        headers: {
          'Authorization': authorization,
          'Content-Type': 'application/json',
        },
        body: jsonEncode(body),
      );
      if (res.statusCode == 200) {
        return FilesApiOk(parse(jsonDecode(res.body) as Map<String, dynamic>));
      }
      return FilesApiFailure(
        message: _failureMessage(res),
        errorCode: _errorCode(res),
        statusCode: res.statusCode,
      );
    } catch (e) {
      return FilesApiFailure(message: '$e');
    }
  }

  static String _failureMessage(http.Response res) {
    final code = _errorCode(res);
    if (code != null) return code;
    return 'HTTP ${res.statusCode}';
  }

  static String? _errorCode(http.Response res) {
    try {
      final decoded = jsonDecode(res.body);
      if (decoded is Map<String, dynamic>) {
        final err = decoded['error_code'] ?? decoded['error'];
        if (err is String && err.isNotEmpty) return err;
      }
    } catch (_) {}
    return null;
  }
}
