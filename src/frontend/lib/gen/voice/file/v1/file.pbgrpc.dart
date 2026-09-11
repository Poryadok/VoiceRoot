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

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$0.GetFileURLResponse> getFileURL(
    $0.GetFileURLRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getFileURL, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$0.GetFileMetadataResponse> getFileMetadata(
    $0.GetFileMetadataRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getFileMetadata, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
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

  /// @voice.security=protected;callers=service:messaging,service:chat,service:story,service:user
  $grpc.ResponseFuture<$0.IssueFileAccessCapabilityResponse>
      issueFileAccessCapability(
    $0.IssueFileAccessCapabilityRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$issueFileAccessCapability, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:space
  $grpc.ResponseFuture<$0.PrepareSpaceDeletionReferenceManifestResponse>
      prepareSpaceDeletionReferenceManifest(
    $0.PrepareSpaceDeletionReferenceManifestRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$prepareSpaceDeletionReferenceManifest, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:space,service:chat,service:messaging
  $grpc.ResponseFuture<$0.RegisterSpaceDeletionReferenceChunkResponse>
      registerSpaceDeletionReferenceChunk(
    $0.RegisterSpaceDeletionReferenceChunkRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$registerSpaceDeletionReferenceChunk, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:space,service:chat,service:messaging
  $grpc.ResponseFuture<$0.ReleaseSpaceDeletionProducerReferencesResponse>
      releaseSpaceDeletionProducerReferences(
    $0.ReleaseSpaceDeletionProducerReferencesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$releaseSpaceDeletionProducerReferences, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:space
  $grpc.ResponseFuture<$0.ApplySpaceLifecycleFenceResponse>
      applySpaceLifecycleFence(
    $0.ApplySpaceLifecycleFenceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$applySpaceLifecycleFence, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:space
  $grpc.ResponseFuture<$0.PurgeSpaceResponse> purgeSpace(
    $0.PurgeSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$purgeSpace, request, options: options);
  }

  /// @voice.security=protected;callers=service:space,service:chat,service:messaging
  $grpc.ResponseFuture<$0.AcquireFileReferencesResponse> acquireFileReferences(
    $0.AcquireFileReferencesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$acquireFileReferences, request, options: options);
  }

  /// @voice.security=protected;callers=service:space,service:chat,service:messaging
  $grpc.ResponseFuture<$0.ReleaseFileReferencesResponse> releaseFileReferences(
    $0.ReleaseFileReferencesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$releaseFileReferences, request, options: options);
  }

  /// @voice.security=protected;callers=service:space
  $grpc.ResponseFuture<$0.GetSpacePurgeReceiptResponse> getSpacePurgeReceipt(
    $0.GetSpacePurgeReceiptRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSpacePurgeReceipt, request, options: options);
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
  static final _$issueFileAccessCapability = $grpc.ClientMethod<
          $0.IssueFileAccessCapabilityRequest,
          $0.IssueFileAccessCapabilityResponse>(
      '/voice.file.v1.FileService/IssueFileAccessCapability',
      ($0.IssueFileAccessCapabilityRequest value) => value.writeToBuffer(),
      $0.IssueFileAccessCapabilityResponse.fromBuffer);
  static final _$prepareSpaceDeletionReferenceManifest = $grpc.ClientMethod<
          $0.PrepareSpaceDeletionReferenceManifestRequest,
          $0.PrepareSpaceDeletionReferenceManifestResponse>(
      '/voice.file.v1.FileService/PrepareSpaceDeletionReferenceManifest',
      ($0.PrepareSpaceDeletionReferenceManifestRequest value) =>
          value.writeToBuffer(),
      $0.PrepareSpaceDeletionReferenceManifestResponse.fromBuffer);
  static final _$registerSpaceDeletionReferenceChunk = $grpc.ClientMethod<
          $0.RegisterSpaceDeletionReferenceChunkRequest,
          $0.RegisterSpaceDeletionReferenceChunkResponse>(
      '/voice.file.v1.FileService/RegisterSpaceDeletionReferenceChunk',
      ($0.RegisterSpaceDeletionReferenceChunkRequest value) =>
          value.writeToBuffer(),
      $0.RegisterSpaceDeletionReferenceChunkResponse.fromBuffer);
  static final _$releaseSpaceDeletionProducerReferences = $grpc.ClientMethod<
          $0.ReleaseSpaceDeletionProducerReferencesRequest,
          $0.ReleaseSpaceDeletionProducerReferencesResponse>(
      '/voice.file.v1.FileService/ReleaseSpaceDeletionProducerReferences',
      ($0.ReleaseSpaceDeletionProducerReferencesRequest value) =>
          value.writeToBuffer(),
      $0.ReleaseSpaceDeletionProducerReferencesResponse.fromBuffer);
  static final _$applySpaceLifecycleFence = $grpc.ClientMethod<
          $0.ApplySpaceLifecycleFenceRequest,
          $0.ApplySpaceLifecycleFenceResponse>(
      '/voice.file.v1.FileService/ApplySpaceLifecycleFence',
      ($0.ApplySpaceLifecycleFenceRequest value) => value.writeToBuffer(),
      $0.ApplySpaceLifecycleFenceResponse.fromBuffer);
  static final _$purgeSpace =
      $grpc.ClientMethod<$0.PurgeSpaceRequest, $0.PurgeSpaceResponse>(
          '/voice.file.v1.FileService/PurgeSpace',
          ($0.PurgeSpaceRequest value) => value.writeToBuffer(),
          $0.PurgeSpaceResponse.fromBuffer);
  static final _$acquireFileReferences = $grpc.ClientMethod<
          $0.AcquireFileReferencesRequest, $0.AcquireFileReferencesResponse>(
      '/voice.file.v1.FileService/AcquireFileReferences',
      ($0.AcquireFileReferencesRequest value) => value.writeToBuffer(),
      $0.AcquireFileReferencesResponse.fromBuffer);
  static final _$releaseFileReferences = $grpc.ClientMethod<
          $0.ReleaseFileReferencesRequest, $0.ReleaseFileReferencesResponse>(
      '/voice.file.v1.FileService/ReleaseFileReferences',
      ($0.ReleaseFileReferencesRequest value) => value.writeToBuffer(),
      $0.ReleaseFileReferencesResponse.fromBuffer);
  static final _$getSpacePurgeReceipt = $grpc.ClientMethod<
          $0.GetSpacePurgeReceiptRequest, $0.GetSpacePurgeReceiptResponse>(
      '/voice.file.v1.FileService/GetSpacePurgeReceipt',
      ($0.GetSpacePurgeReceiptRequest value) => value.writeToBuffer(),
      $0.GetSpacePurgeReceiptResponse.fromBuffer);
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
    $addMethod($grpc.ServiceMethod<$0.IssueFileAccessCapabilityRequest,
            $0.IssueFileAccessCapabilityResponse>(
        'IssueFileAccessCapability',
        issueFileAccessCapability_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.IssueFileAccessCapabilityRequest.fromBuffer(value),
        ($0.IssueFileAccessCapabilityResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<
            $0.PrepareSpaceDeletionReferenceManifestRequest,
            $0.PrepareSpaceDeletionReferenceManifestResponse>(
        'PrepareSpaceDeletionReferenceManifest',
        prepareSpaceDeletionReferenceManifest_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.PrepareSpaceDeletionReferenceManifestRequest.fromBuffer(value),
        ($0.PrepareSpaceDeletionReferenceManifestResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<
            $0.RegisterSpaceDeletionReferenceChunkRequest,
            $0.RegisterSpaceDeletionReferenceChunkResponse>(
        'RegisterSpaceDeletionReferenceChunk',
        registerSpaceDeletionReferenceChunk_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RegisterSpaceDeletionReferenceChunkRequest.fromBuffer(value),
        ($0.RegisterSpaceDeletionReferenceChunkResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<
            $0.ReleaseSpaceDeletionProducerReferencesRequest,
            $0.ReleaseSpaceDeletionProducerReferencesResponse>(
        'ReleaseSpaceDeletionProducerReferences',
        releaseSpaceDeletionProducerReferences_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ReleaseSpaceDeletionProducerReferencesRequest.fromBuffer(value),
        ($0.ReleaseSpaceDeletionProducerReferencesResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ApplySpaceLifecycleFenceRequest,
            $0.ApplySpaceLifecycleFenceResponse>(
        'ApplySpaceLifecycleFence',
        applySpaceLifecycleFence_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ApplySpaceLifecycleFenceRequest.fromBuffer(value),
        ($0.ApplySpaceLifecycleFenceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.PurgeSpaceRequest, $0.PurgeSpaceResponse>(
        'PurgeSpace',
        purgeSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.PurgeSpaceRequest.fromBuffer(value),
        ($0.PurgeSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AcquireFileReferencesRequest,
            $0.AcquireFileReferencesResponse>(
        'AcquireFileReferences',
        acquireFileReferences_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.AcquireFileReferencesRequest.fromBuffer(value),
        ($0.AcquireFileReferencesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ReleaseFileReferencesRequest,
            $0.ReleaseFileReferencesResponse>(
        'ReleaseFileReferences',
        releaseFileReferences_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ReleaseFileReferencesRequest.fromBuffer(value),
        ($0.ReleaseFileReferencesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetSpacePurgeReceiptRequest,
            $0.GetSpacePurgeReceiptResponse>(
        'GetSpacePurgeReceipt',
        getSpacePurgeReceipt_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetSpacePurgeReceiptRequest.fromBuffer(value),
        ($0.GetSpacePurgeReceiptResponse value) => value.writeToBuffer()));
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

  $async.Future<$0.IssueFileAccessCapabilityResponse>
      issueFileAccessCapability_Pre($grpc.ServiceCall $call,
          $async.Future<$0.IssueFileAccessCapabilityRequest> $request) async {
    return issueFileAccessCapability($call, await $request);
  }

  $async.Future<$0.IssueFileAccessCapabilityResponse> issueFileAccessCapability(
      $grpc.ServiceCall call, $0.IssueFileAccessCapabilityRequest request);

  $async.Future<$0.PrepareSpaceDeletionReferenceManifestResponse>
      prepareSpaceDeletionReferenceManifest_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$0.PrepareSpaceDeletionReferenceManifestRequest>
              $request) async {
    return prepareSpaceDeletionReferenceManifest($call, await $request);
  }

  $async.Future<$0.PrepareSpaceDeletionReferenceManifestResponse>
      prepareSpaceDeletionReferenceManifest($grpc.ServiceCall call,
          $0.PrepareSpaceDeletionReferenceManifestRequest request);

  $async.Future<$0.RegisterSpaceDeletionReferenceChunkResponse>
      registerSpaceDeletionReferenceChunk_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$0.RegisterSpaceDeletionReferenceChunkRequest>
              $request) async {
    return registerSpaceDeletionReferenceChunk($call, await $request);
  }

  $async.Future<$0.RegisterSpaceDeletionReferenceChunkResponse>
      registerSpaceDeletionReferenceChunk($grpc.ServiceCall call,
          $0.RegisterSpaceDeletionReferenceChunkRequest request);

  $async.Future<$0.ReleaseSpaceDeletionProducerReferencesResponse>
      releaseSpaceDeletionProducerReferences_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$0.ReleaseSpaceDeletionProducerReferencesRequest>
              $request) async {
    return releaseSpaceDeletionProducerReferences($call, await $request);
  }

  $async.Future<$0.ReleaseSpaceDeletionProducerReferencesResponse>
      releaseSpaceDeletionProducerReferences($grpc.ServiceCall call,
          $0.ReleaseSpaceDeletionProducerReferencesRequest request);

  $async.Future<$0.ApplySpaceLifecycleFenceResponse>
      applySpaceLifecycleFence_Pre($grpc.ServiceCall $call,
          $async.Future<$0.ApplySpaceLifecycleFenceRequest> $request) async {
    return applySpaceLifecycleFence($call, await $request);
  }

  $async.Future<$0.ApplySpaceLifecycleFenceResponse> applySpaceLifecycleFence(
      $grpc.ServiceCall call, $0.ApplySpaceLifecycleFenceRequest request);

  $async.Future<$0.PurgeSpaceResponse> purgeSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$0.PurgeSpaceRequest> $request) async {
    return purgeSpace($call, await $request);
  }

  $async.Future<$0.PurgeSpaceResponse> purgeSpace(
      $grpc.ServiceCall call, $0.PurgeSpaceRequest request);

  $async.Future<$0.AcquireFileReferencesResponse> acquireFileReferences_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.AcquireFileReferencesRequest> $request) async {
    return acquireFileReferences($call, await $request);
  }

  $async.Future<$0.AcquireFileReferencesResponse> acquireFileReferences(
      $grpc.ServiceCall call, $0.AcquireFileReferencesRequest request);

  $async.Future<$0.ReleaseFileReferencesResponse> releaseFileReferences_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ReleaseFileReferencesRequest> $request) async {
    return releaseFileReferences($call, await $request);
  }

  $async.Future<$0.ReleaseFileReferencesResponse> releaseFileReferences(
      $grpc.ServiceCall call, $0.ReleaseFileReferencesRequest request);

  $async.Future<$0.GetSpacePurgeReceiptResponse> getSpacePurgeReceipt_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetSpacePurgeReceiptRequest> $request) async {
    return getSpacePurgeReceipt($call, await $request);
  }

  $async.Future<$0.GetSpacePurgeReceiptResponse> getSpacePurgeReceipt(
      $grpc.ServiceCall call, $0.GetSpacePurgeReceiptRequest request);
}
