// This is a generated file - do not edit.
//
// Generated from voice/file/v1/file.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:grpc/service_api.dart' as $grpc;
import 'package:protobuf/protobuf.dart' as $pb;

import 'file.pb.dart' as $0;

export 'file.pb.dart';

/// Uploads, R2, scanning. HTTP: /api/v1/files/**.
@$pb.GrpcServiceName('voice.file.v1.FileService')
class FileServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  FileServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.RequestUploadResponse> requestUpload(
    $0.RequestUploadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$requestUpload, request, options: options);
  }

  $grpc.ResponseFuture<$0.ConfirmUploadResponse> confirmUpload(
    $0.ConfirmUploadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$confirmUpload, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetFileURLResponse> getFileURL(
    $0.GetFileURLRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getFileURL, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetFileMetadataResponse> getFileMetadata(
    $0.GetFileMetadataRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getFileMetadata, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetBulkMetadataResponse> getBulkMetadata(
    $0.GetBulkMetadataRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getBulkMetadata, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteFileResponse> deleteFile(
    $0.DeleteFileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteFile, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListFilesResponse> listFiles(
    $0.ListFilesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listFiles, request, options: options);
  }

  $grpc.ResponseFuture<$0.CheckQuotaResponse> checkQuota(
    $0.CheckQuotaRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$checkQuota, request, options: options);
  }

  // method descriptors

  static final _$requestUpload =
      $grpc.ClientMethod<$0.RequestUploadRequest, $0.RequestUploadResponse>(
          '/voice.file.v1.FileService/RequestUpload',
          ($0.RequestUploadRequest value) => value.writeToBuffer(),
          $0.RequestUploadResponse.fromBuffer);
  static final _$confirmUpload =
      $grpc.ClientMethod<$0.ConfirmUploadRequest, $0.ConfirmUploadResponse>(
          '/voice.file.v1.FileService/ConfirmUpload',
          ($0.ConfirmUploadRequest value) => value.writeToBuffer(),
          $0.ConfirmUploadResponse.fromBuffer);
  static final _$getFileURL =
      $grpc.ClientMethod<$0.GetFileURLRequest, $0.GetFileURLResponse>(
          '/voice.file.v1.FileService/GetFileURL',
          ($0.GetFileURLRequest value) => value.writeToBuffer(),
          $0.GetFileURLResponse.fromBuffer);
  static final _$getFileMetadata =
      $grpc.ClientMethod<$0.GetFileMetadataRequest, $0.GetFileMetadataResponse>(
          '/voice.file.v1.FileService/GetFileMetadata',
          ($0.GetFileMetadataRequest value) => value.writeToBuffer(),
          $0.GetFileMetadataResponse.fromBuffer);
  static final _$getBulkMetadata =
      $grpc.ClientMethod<$0.GetBulkMetadataRequest, $0.GetBulkMetadataResponse>(
          '/voice.file.v1.FileService/GetBulkMetadata',
          ($0.GetBulkMetadataRequest value) => value.writeToBuffer(),
          $0.GetBulkMetadataResponse.fromBuffer);
  static final _$deleteFile =
      $grpc.ClientMethod<$0.DeleteFileRequest, $0.DeleteFileResponse>(
          '/voice.file.v1.FileService/DeleteFile',
          ($0.DeleteFileRequest value) => value.writeToBuffer(),
          $0.DeleteFileResponse.fromBuffer);
  static final _$listFiles =
      $grpc.ClientMethod<$0.ListFilesRequest, $0.ListFilesResponse>(
          '/voice.file.v1.FileService/ListFiles',
          ($0.ListFilesRequest value) => value.writeToBuffer(),
          $0.ListFilesResponse.fromBuffer);
  static final _$checkQuota =
      $grpc.ClientMethod<$0.CheckQuotaRequest, $0.CheckQuotaResponse>(
          '/voice.file.v1.FileService/CheckQuota',
          ($0.CheckQuotaRequest value) => value.writeToBuffer(),
          $0.CheckQuotaResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.file.v1.FileService')
abstract class FileServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.file.v1.FileService';

  FileServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.RequestUploadRequest, $0.RequestUploadResponse>(
            'RequestUpload',
            requestUpload_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.RequestUploadRequest.fromBuffer(value),
            ($0.RequestUploadResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ConfirmUploadRequest, $0.ConfirmUploadResponse>(
            'ConfirmUpload',
            confirmUpload_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ConfirmUploadRequest.fromBuffer(value),
            ($0.ConfirmUploadResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetFileURLRequest, $0.GetFileURLResponse>(
        'GetFileURL',
        getFileURL_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetFileURLRequest.fromBuffer(value),
        ($0.GetFileURLResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetFileMetadataRequest,
            $0.GetFileMetadataResponse>(
        'GetFileMetadata',
        getFileMetadata_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetFileMetadataRequest.fromBuffer(value),
        ($0.GetFileMetadataResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetBulkMetadataRequest,
            $0.GetBulkMetadataResponse>(
        'GetBulkMetadata',
        getBulkMetadata_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetBulkMetadataRequest.fromBuffer(value),
        ($0.GetBulkMetadataResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeleteFileRequest, $0.DeleteFileResponse>(
        'DeleteFile',
        deleteFile_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.DeleteFileRequest.fromBuffer(value),
        ($0.DeleteFileResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListFilesRequest, $0.ListFilesResponse>(
        'ListFiles',
        listFiles_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ListFilesRequest.fromBuffer(value),
        ($0.ListFilesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CheckQuotaRequest, $0.CheckQuotaResponse>(
        'CheckQuota',
        checkQuota_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.CheckQuotaRequest.fromBuffer(value),
        ($0.CheckQuotaResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.RequestUploadResponse> requestUpload_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RequestUploadRequest> $request) async {
    return requestUpload($call, await $request);
  }

  $async.Future<$0.RequestUploadResponse> requestUpload(
      $grpc.ServiceCall call, $0.RequestUploadRequest request);

  $async.Future<$0.ConfirmUploadResponse> confirmUpload_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ConfirmUploadRequest> $request) async {
    return confirmUpload($call, await $request);
  }

  $async.Future<$0.ConfirmUploadResponse> confirmUpload(
      $grpc.ServiceCall call, $0.ConfirmUploadRequest request);

  $async.Future<$0.GetFileURLResponse> getFileURL_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetFileURLRequest> $request) async {
    return getFileURL($call, await $request);
  }

  $async.Future<$0.GetFileURLResponse> getFileURL(
      $grpc.ServiceCall call, $0.GetFileURLRequest request);

  $async.Future<$0.GetFileMetadataResponse> getFileMetadata_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetFileMetadataRequest> $request) async {
    return getFileMetadata($call, await $request);
  }

  $async.Future<$0.GetFileMetadataResponse> getFileMetadata(
      $grpc.ServiceCall call, $0.GetFileMetadataRequest request);

  $async.Future<$0.GetBulkMetadataResponse> getBulkMetadata_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetBulkMetadataRequest> $request) async {
    return getBulkMetadata($call, await $request);
  }

  $async.Future<$0.GetBulkMetadataResponse> getBulkMetadata(
      $grpc.ServiceCall call, $0.GetBulkMetadataRequest request);

  $async.Future<$0.DeleteFileResponse> deleteFile_Pre($grpc.ServiceCall $call,
      $async.Future<$0.DeleteFileRequest> $request) async {
    return deleteFile($call, await $request);
  }

  $async.Future<$0.DeleteFileResponse> deleteFile(
      $grpc.ServiceCall call, $0.DeleteFileRequest request);

  $async.Future<$0.ListFilesResponse> listFiles_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListFilesRequest> $request) async {
    return listFiles($call, await $request);
  }

  $async.Future<$0.ListFilesResponse> listFiles(
      $grpc.ServiceCall call, $0.ListFilesRequest request);

  $async.Future<$0.CheckQuotaResponse> checkQuota_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CheckQuotaRequest> $request) async {
    return checkQuota($call, await $request);
  }

  $async.Future<$0.CheckQuotaResponse> checkQuota(
      $grpc.ServiceCall call, $0.CheckQuotaRequest request);
}
