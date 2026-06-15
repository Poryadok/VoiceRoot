import 'dart:typed_data';

import 'package:crypto/crypto.dart' as crypto;

import '../gen/voice/file/v1/file.pb.dart' as file_pb;
import 'api_result.dart';
import 'gateway_http.dart';
import 'presigned_upload.dart';
import 'proto_mappers.dart';

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
    this.isE2e = false,
    this.expiresAt,
  });

  final String fileId;
  final String fileType;
  final String status;
  final String originalName;
  final String? url;
  final String? previewUrl;
  final int? sizeBytes;
  final bool isE2e;
  final DateTime? expiresAt;
}

class VoiceFilesClient {
  VoiceFilesClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<FilesApiResult<FileUploadTicket>> requestUpload({
    required String authorization,
    required String originalName,
    required String mimeType,
    required int sizeBytes,
    String? chatId,
    String? chatType,
    bool isE2e = false,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/files/upload'),
      authorization: authorization,
      body: requestUploadToProto(
        originalName: originalName,
        mimeType: mimeType,
        sizeBytes: sizeBytes,
        chatId: chatId,
        chatType: chatTypeFromWire(chatType),
        isE2e: isE2e,
      ),
      createEmpty: file_pb.RequestUploadResponse.create,
    );
    return _map(
      result,
      (data) => fileUploadTicketFromProto(
        data.hasUploadResponse()
            ? data.uploadResponse
            : file_pb.UploadResponse(),
      ),
    );
  }

  Future<FilesApiResult<FileMetadataData>> confirmUpload({
    required String authorization,
    required String fileId,
    required Uint8List bytes,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/files/$fileId/confirm'),
      authorization: authorization,
      body: confirmUploadRequestToProto(
        fileId: fileId,
        sha256Hash: sha256Hex(bytes),
      ),
      createEmpty: file_pb.ConfirmUploadResponse.create,
    );
    return _map(
      result,
      (data) => fileMetadataFromProto(
        data.hasFileMetadata() ? data.fileMetadata : file_pb.FileMetadata(),
      ),
    );
  }

  Future<FilesApiResult<Uint8List>> fetchFileBytes({
    required String authorization,
    required String fileId,
  }) async {
    final urlResult = await getFileUrl(
      authorization: authorization,
      fileId: fileId,
    );
    return switch (urlResult) {
      FilesApiOk(:final data) => _downloadPresigned(Uri.parse(data)),
      FilesApiFailure(:final message, :final errorCode, :final statusCode) =>
        FilesApiFailure(
          message: message,
          errorCode: errorCode,
          statusCode: statusCode,
        ),
    };
  }

  Future<FilesApiResult<Uint8List>> _downloadPresigned(Uri uri) async {
    final result = await _gateway.getBytes(uri: uri);
    return switch (result) {
      GatewayHttpOk(:final data) => FilesApiOk(data),
      GatewayHttpFailure(:final error) => FilesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  Future<FilesApiResult<String>> getFileUrl({
    required String authorization,
    required String fileId,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/files/$fileId/url'),
      authorization: authorization,
      createEmpty: file_pb.GetFileURLResponse.create,
    );
    return _map(result, (data) => data.presignedGetUrl);
  }

  Future<FilesApiResult<void>> putBytes({
    required Uri uploadUrl,
    required Uint8List bytes,
    required String mimeType,
  }) async {
    final result = await putPresigned(
      gateway: _gateway,
      uploadUrl: uploadUrl,
      requiredHeaders: {
        'Content-Type': mimeType,
        'Content-Length': '${bytes.length}',
      },
      bytes: bytes,
    );
    return _mapEmpty(result);
  }

  static String sha256Hex(Uint8List bytes) {
    return crypto.sha256.convert(bytes).toString();
  }

  FilesApiResult<T> _map<T>(
    GatewayHttpResult<dynamic> result,
    T Function(dynamic data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => FilesApiOk(parse(data)),
      GatewayHttpFailure(:final error) => FilesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  FilesApiResult<void> _mapEmpty(GatewayHttpResult<dynamic> result) {
    return switch (result) {
      GatewayHttpOk() => const FilesApiOk(null),
      GatewayHttpFailure(:final error) => FilesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}
