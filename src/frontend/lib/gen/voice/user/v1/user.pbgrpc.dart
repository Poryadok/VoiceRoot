// This is a generated file - do not edit.
//
// Generated from voice/user/v1/user.proto.

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

import 'user.pb.dart' as $0;

export 'user.pb.dart';

/// User Service — profiles, privacy, presence. HTTP: /api/v1/users/**.
@$pb.GrpcServiceName('voice.user.v1.UserService')
class UserServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  UserServiceClient(super.channel, {super.options, super.interceptors});

  /// S2S bootstrap from Auth: ensure one primary profile per account before JWT is issued.
  /// Target: called only from trusted Auth; not exposed via public Gateway REST in v1 DM scope.
  $grpc.ResponseFuture<$0.EnsurePrimaryProfileResponse> ensurePrimaryProfile(
    $0.EnsurePrimaryProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$ensurePrimaryProfile, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetProfileResponse> getProfile(
    $0.GetProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getProfile, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetProfilesResponse> getProfiles(
    $0.GetProfilesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getProfiles, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdateProfileResponse> updateProfile(
    $0.UpdateProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateProfile, request, options: options);
  }

  $grpc.ResponseFuture<$0.CreateProfileResponse> createProfile(
    $0.CreateProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createProfile, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteProfileResponse> deleteProfile(
    $0.DeleteProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteProfile, request, options: options);
  }

  $grpc.ResponseFuture<$0.SwitchProfileResponse> switchProfile(
    $0.SwitchProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$switchProfile, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListMyProfilesResponse> listMyProfiles(
    $0.ListMyProfilesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listMyProfiles, request, options: options);
  }

  /// Discover profiles by query (username / display_name) for friend requests and DM addressing.
  /// Friend/DM profile discovery — user_db; privacy + blocks enforced. Global search: docs/features/search.md.
  $grpc.ResponseFuture<$0.SearchProfilesResponse> searchProfiles(
    $0.SearchProfilesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$searchProfiles, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetPrivacySettingsResponse> getPrivacySettings(
    $0.GetPrivacySettingsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getPrivacySettings, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdatePrivacySettingsResponse> updatePrivacySettings(
    $0.UpdatePrivacySettingsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updatePrivacySettings, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdatePresenceResponse> updatePresence(
    $0.UpdatePresenceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updatePresence, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetPresenceResponse> getPresence(
    $0.GetPresenceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getPresence, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetBulkPresenceResponse> getBulkPresence(
    $0.GetBulkPresenceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getBulkPresence, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetSettingsResponse> getSettings(
    $0.GetSettingsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSettings, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdateSettingsResponse> updateSettings(
    $0.UpdateSettingsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateSettings, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetOnboardingStateResponse> getOnboardingState(
    $0.GetOnboardingStateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getOnboardingState, request, options: options);
  }

  $grpc.ResponseFuture<$0.CompleteOnboardingStepResponse>
      completeOnboardingStep(
    $0.CompleteOnboardingStepRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$completeOnboardingStep, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.GetVerificationStatusResponse> getVerificationStatus(
    $0.GetVerificationStatusRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getVerificationStatus, request, options: options);
  }

  /// S2S from Auth after OAuth identity checks (verification.md).
  $grpc.ResponseFuture<$0.SetVerificationResponse> setVerification(
    $0.SetVerificationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setVerification, request, options: options);
  }

  $grpc.ResponseFuture<$0.ClearVerificationResponse> clearVerification(
    $0.ClearVerificationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$clearVerification, request, options: options);
  }

  $grpc.ResponseFuture<$0.StartOrganizationVerificationResponse>
      startOrganizationVerification(
    $0.StartOrganizationVerificationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$startOrganizationVerification, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.CheckOrganizationVerificationResponse>
      checkOrganizationVerification(
    $0.CheckOrganizationVerificationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$checkOrganizationVerification, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.ApplyDowngradeProfilesResponse>
      applyDowngradeProfiles(
    $0.ApplyDowngradeProfilesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$applyDowngradeProfiles, request,
        options: options);
  }

  /// Presigned PUT to Cloudflare R2 for static profile avatar (docs/features/user-profile.md).
  $grpc.ResponseFuture<$0.CreateAvatarPresignedUploadResponse>
      createAvatarPresignedUpload(
    $0.CreateAvatarPresignedUploadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createAvatarPresignedUpload, request,
        options: options);
  }

  // method descriptors

  static final _$ensurePrimaryProfile = $grpc.ClientMethod<
          $0.EnsurePrimaryProfileRequest, $0.EnsurePrimaryProfileResponse>(
      '/voice.user.v1.UserService/EnsurePrimaryProfile',
      ($0.EnsurePrimaryProfileRequest value) => value.writeToBuffer(),
      $0.EnsurePrimaryProfileResponse.fromBuffer);
  static final _$getProfile =
      $grpc.ClientMethod<$0.GetProfileRequest, $0.GetProfileResponse>(
          '/voice.user.v1.UserService/GetProfile',
          ($0.GetProfileRequest value) => value.writeToBuffer(),
          $0.GetProfileResponse.fromBuffer);
  static final _$getProfiles =
      $grpc.ClientMethod<$0.GetProfilesRequest, $0.GetProfilesResponse>(
          '/voice.user.v1.UserService/GetProfiles',
          ($0.GetProfilesRequest value) => value.writeToBuffer(),
          $0.GetProfilesResponse.fromBuffer);
  static final _$updateProfile =
      $grpc.ClientMethod<$0.UpdateProfileRequest, $0.UpdateProfileResponse>(
          '/voice.user.v1.UserService/UpdateProfile',
          ($0.UpdateProfileRequest value) => value.writeToBuffer(),
          $0.UpdateProfileResponse.fromBuffer);
  static final _$createProfile =
      $grpc.ClientMethod<$0.CreateProfileRequest, $0.CreateProfileResponse>(
          '/voice.user.v1.UserService/CreateProfile',
          ($0.CreateProfileRequest value) => value.writeToBuffer(),
          $0.CreateProfileResponse.fromBuffer);
  static final _$deleteProfile =
      $grpc.ClientMethod<$0.DeleteProfileRequest, $0.DeleteProfileResponse>(
          '/voice.user.v1.UserService/DeleteProfile',
          ($0.DeleteProfileRequest value) => value.writeToBuffer(),
          $0.DeleteProfileResponse.fromBuffer);
  static final _$switchProfile =
      $grpc.ClientMethod<$0.SwitchProfileRequest, $0.SwitchProfileResponse>(
          '/voice.user.v1.UserService/SwitchProfile',
          ($0.SwitchProfileRequest value) => value.writeToBuffer(),
          $0.SwitchProfileResponse.fromBuffer);
  static final _$listMyProfiles =
      $grpc.ClientMethod<$0.ListMyProfilesRequest, $0.ListMyProfilesResponse>(
          '/voice.user.v1.UserService/ListMyProfiles',
          ($0.ListMyProfilesRequest value) => value.writeToBuffer(),
          $0.ListMyProfilesResponse.fromBuffer);
  static final _$searchProfiles =
      $grpc.ClientMethod<$0.SearchProfilesRequest, $0.SearchProfilesResponse>(
          '/voice.user.v1.UserService/SearchProfiles',
          ($0.SearchProfilesRequest value) => value.writeToBuffer(),
          $0.SearchProfilesResponse.fromBuffer);
  static final _$getPrivacySettings = $grpc.ClientMethod<
          $0.GetPrivacySettingsRequest, $0.GetPrivacySettingsResponse>(
      '/voice.user.v1.UserService/GetPrivacySettings',
      ($0.GetPrivacySettingsRequest value) => value.writeToBuffer(),
      $0.GetPrivacySettingsResponse.fromBuffer);
  static final _$updatePrivacySettings = $grpc.ClientMethod<
          $0.UpdatePrivacySettingsRequest, $0.UpdatePrivacySettingsResponse>(
      '/voice.user.v1.UserService/UpdatePrivacySettings',
      ($0.UpdatePrivacySettingsRequest value) => value.writeToBuffer(),
      $0.UpdatePrivacySettingsResponse.fromBuffer);
  static final _$updatePresence =
      $grpc.ClientMethod<$0.UpdatePresenceRequest, $0.UpdatePresenceResponse>(
          '/voice.user.v1.UserService/UpdatePresence',
          ($0.UpdatePresenceRequest value) => value.writeToBuffer(),
          $0.UpdatePresenceResponse.fromBuffer);
  static final _$getPresence =
      $grpc.ClientMethod<$0.GetPresenceRequest, $0.GetPresenceResponse>(
          '/voice.user.v1.UserService/GetPresence',
          ($0.GetPresenceRequest value) => value.writeToBuffer(),
          $0.GetPresenceResponse.fromBuffer);
  static final _$getBulkPresence =
      $grpc.ClientMethod<$0.GetBulkPresenceRequest, $0.GetBulkPresenceResponse>(
          '/voice.user.v1.UserService/GetBulkPresence',
          ($0.GetBulkPresenceRequest value) => value.writeToBuffer(),
          $0.GetBulkPresenceResponse.fromBuffer);
  static final _$getSettings =
      $grpc.ClientMethod<$0.GetSettingsRequest, $0.GetSettingsResponse>(
          '/voice.user.v1.UserService/GetSettings',
          ($0.GetSettingsRequest value) => value.writeToBuffer(),
          $0.GetSettingsResponse.fromBuffer);
  static final _$updateSettings =
      $grpc.ClientMethod<$0.UpdateSettingsRequest, $0.UpdateSettingsResponse>(
          '/voice.user.v1.UserService/UpdateSettings',
          ($0.UpdateSettingsRequest value) => value.writeToBuffer(),
          $0.UpdateSettingsResponse.fromBuffer);
  static final _$getOnboardingState = $grpc.ClientMethod<
          $0.GetOnboardingStateRequest, $0.GetOnboardingStateResponse>(
      '/voice.user.v1.UserService/GetOnboardingState',
      ($0.GetOnboardingStateRequest value) => value.writeToBuffer(),
      $0.GetOnboardingStateResponse.fromBuffer);
  static final _$completeOnboardingStep = $grpc.ClientMethod<
          $0.CompleteOnboardingStepRequest, $0.CompleteOnboardingStepResponse>(
      '/voice.user.v1.UserService/CompleteOnboardingStep',
      ($0.CompleteOnboardingStepRequest value) => value.writeToBuffer(),
      $0.CompleteOnboardingStepResponse.fromBuffer);
  static final _$getVerificationStatus = $grpc.ClientMethod<
          $0.GetVerificationStatusRequest, $0.GetVerificationStatusResponse>(
      '/voice.user.v1.UserService/GetVerificationStatus',
      ($0.GetVerificationStatusRequest value) => value.writeToBuffer(),
      $0.GetVerificationStatusResponse.fromBuffer);
  static final _$setVerification =
      $grpc.ClientMethod<$0.SetVerificationRequest, $0.SetVerificationResponse>(
          '/voice.user.v1.UserService/SetVerification',
          ($0.SetVerificationRequest value) => value.writeToBuffer(),
          $0.SetVerificationResponse.fromBuffer);
  static final _$clearVerification = $grpc.ClientMethod<
          $0.ClearVerificationRequest, $0.ClearVerificationResponse>(
      '/voice.user.v1.UserService/ClearVerification',
      ($0.ClearVerificationRequest value) => value.writeToBuffer(),
      $0.ClearVerificationResponse.fromBuffer);
  static final _$startOrganizationVerification = $grpc.ClientMethod<
          $0.StartOrganizationVerificationRequest,
          $0.StartOrganizationVerificationResponse>(
      '/voice.user.v1.UserService/StartOrganizationVerification',
      ($0.StartOrganizationVerificationRequest value) => value.writeToBuffer(),
      $0.StartOrganizationVerificationResponse.fromBuffer);
  static final _$checkOrganizationVerification = $grpc.ClientMethod<
          $0.CheckOrganizationVerificationRequest,
          $0.CheckOrganizationVerificationResponse>(
      '/voice.user.v1.UserService/CheckOrganizationVerification',
      ($0.CheckOrganizationVerificationRequest value) => value.writeToBuffer(),
      $0.CheckOrganizationVerificationResponse.fromBuffer);
  static final _$applyDowngradeProfiles = $grpc.ClientMethod<
          $0.ApplyDowngradeProfilesRequest, $0.ApplyDowngradeProfilesResponse>(
      '/voice.user.v1.UserService/ApplyDowngradeProfiles',
      ($0.ApplyDowngradeProfilesRequest value) => value.writeToBuffer(),
      $0.ApplyDowngradeProfilesResponse.fromBuffer);
  static final _$createAvatarPresignedUpload = $grpc.ClientMethod<
          $0.CreateAvatarPresignedUploadRequest,
          $0.CreateAvatarPresignedUploadResponse>(
      '/voice.user.v1.UserService/CreateAvatarPresignedUpload',
      ($0.CreateAvatarPresignedUploadRequest value) => value.writeToBuffer(),
      $0.CreateAvatarPresignedUploadResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.user.v1.UserService')
abstract class UserServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.user.v1.UserService';

  UserServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.EnsurePrimaryProfileRequest,
            $0.EnsurePrimaryProfileResponse>(
        'EnsurePrimaryProfile',
        ensurePrimaryProfile_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.EnsurePrimaryProfileRequest.fromBuffer(value),
        ($0.EnsurePrimaryProfileResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetProfileRequest, $0.GetProfileResponse>(
        'GetProfile',
        getProfile_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetProfileRequest.fromBuffer(value),
        ($0.GetProfileResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetProfilesRequest, $0.GetProfilesResponse>(
            'GetProfiles',
            getProfiles_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetProfilesRequest.fromBuffer(value),
            ($0.GetProfilesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.UpdateProfileRequest, $0.UpdateProfileResponse>(
            'UpdateProfile',
            updateProfile_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.UpdateProfileRequest.fromBuffer(value),
            ($0.UpdateProfileResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.CreateProfileRequest, $0.CreateProfileResponse>(
            'CreateProfile',
            createProfile_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.CreateProfileRequest.fromBuffer(value),
            ($0.CreateProfileResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.DeleteProfileRequest, $0.DeleteProfileResponse>(
            'DeleteProfile',
            deleteProfile_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.DeleteProfileRequest.fromBuffer(value),
            ($0.DeleteProfileResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SwitchProfileRequest, $0.SwitchProfileResponse>(
            'SwitchProfile',
            switchProfile_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SwitchProfileRequest.fromBuffer(value),
            ($0.SwitchProfileResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListMyProfilesRequest,
            $0.ListMyProfilesResponse>(
        'ListMyProfiles',
        listMyProfiles_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListMyProfilesRequest.fromBuffer(value),
        ($0.ListMyProfilesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SearchProfilesRequest,
            $0.SearchProfilesResponse>(
        'SearchProfiles',
        searchProfiles_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SearchProfilesRequest.fromBuffer(value),
        ($0.SearchProfilesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetPrivacySettingsRequest,
            $0.GetPrivacySettingsResponse>(
        'GetPrivacySettings',
        getPrivacySettings_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetPrivacySettingsRequest.fromBuffer(value),
        ($0.GetPrivacySettingsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpdatePrivacySettingsRequest,
            $0.UpdatePrivacySettingsResponse>(
        'UpdatePrivacySettings',
        updatePrivacySettings_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UpdatePrivacySettingsRequest.fromBuffer(value),
        ($0.UpdatePrivacySettingsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpdatePresenceRequest,
            $0.UpdatePresenceResponse>(
        'UpdatePresence',
        updatePresence_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UpdatePresenceRequest.fromBuffer(value),
        ($0.UpdatePresenceResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetPresenceRequest, $0.GetPresenceResponse>(
            'GetPresence',
            getPresence_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetPresenceRequest.fromBuffer(value),
            ($0.GetPresenceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetBulkPresenceRequest,
            $0.GetBulkPresenceResponse>(
        'GetBulkPresence',
        getBulkPresence_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetBulkPresenceRequest.fromBuffer(value),
        ($0.GetBulkPresenceResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetSettingsRequest, $0.GetSettingsResponse>(
            'GetSettings',
            getSettings_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetSettingsRequest.fromBuffer(value),
            ($0.GetSettingsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpdateSettingsRequest,
            $0.UpdateSettingsResponse>(
        'UpdateSettings',
        updateSettings_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UpdateSettingsRequest.fromBuffer(value),
        ($0.UpdateSettingsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetOnboardingStateRequest,
            $0.GetOnboardingStateResponse>(
        'GetOnboardingState',
        getOnboardingState_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetOnboardingStateRequest.fromBuffer(value),
        ($0.GetOnboardingStateResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CompleteOnboardingStepRequest,
            $0.CompleteOnboardingStepResponse>(
        'CompleteOnboardingStep',
        completeOnboardingStep_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CompleteOnboardingStepRequest.fromBuffer(value),
        ($0.CompleteOnboardingStepResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetVerificationStatusRequest,
            $0.GetVerificationStatusResponse>(
        'GetVerificationStatus',
        getVerificationStatus_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetVerificationStatusRequest.fromBuffer(value),
        ($0.GetVerificationStatusResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SetVerificationRequest,
            $0.SetVerificationResponse>(
        'SetVerification',
        setVerification_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SetVerificationRequest.fromBuffer(value),
        ($0.SetVerificationResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ClearVerificationRequest,
            $0.ClearVerificationResponse>(
        'ClearVerification',
        clearVerification_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ClearVerificationRequest.fromBuffer(value),
        ($0.ClearVerificationResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.StartOrganizationVerificationRequest,
            $0.StartOrganizationVerificationResponse>(
        'StartOrganizationVerification',
        startOrganizationVerification_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.StartOrganizationVerificationRequest.fromBuffer(value),
        ($0.StartOrganizationVerificationResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CheckOrganizationVerificationRequest,
            $0.CheckOrganizationVerificationResponse>(
        'CheckOrganizationVerification',
        checkOrganizationVerification_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CheckOrganizationVerificationRequest.fromBuffer(value),
        ($0.CheckOrganizationVerificationResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ApplyDowngradeProfilesRequest,
            $0.ApplyDowngradeProfilesResponse>(
        'ApplyDowngradeProfiles',
        applyDowngradeProfiles_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ApplyDowngradeProfilesRequest.fromBuffer(value),
        ($0.ApplyDowngradeProfilesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CreateAvatarPresignedUploadRequest,
            $0.CreateAvatarPresignedUploadResponse>(
        'CreateAvatarPresignedUpload',
        createAvatarPresignedUpload_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CreateAvatarPresignedUploadRequest.fromBuffer(value),
        ($0.CreateAvatarPresignedUploadResponse value) =>
            value.writeToBuffer()));
  }

  $async.Future<$0.EnsurePrimaryProfileResponse> ensurePrimaryProfile_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.EnsurePrimaryProfileRequest> $request) async {
    return ensurePrimaryProfile($call, await $request);
  }

  $async.Future<$0.EnsurePrimaryProfileResponse> ensurePrimaryProfile(
      $grpc.ServiceCall call, $0.EnsurePrimaryProfileRequest request);

  $async.Future<$0.GetProfileResponse> getProfile_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetProfileRequest> $request) async {
    return getProfile($call, await $request);
  }

  $async.Future<$0.GetProfileResponse> getProfile(
      $grpc.ServiceCall call, $0.GetProfileRequest request);

  $async.Future<$0.GetProfilesResponse> getProfiles_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetProfilesRequest> $request) async {
    return getProfiles($call, await $request);
  }

  $async.Future<$0.GetProfilesResponse> getProfiles(
      $grpc.ServiceCall call, $0.GetProfilesRequest request);

  $async.Future<$0.UpdateProfileResponse> updateProfile_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UpdateProfileRequest> $request) async {
    return updateProfile($call, await $request);
  }

  $async.Future<$0.UpdateProfileResponse> updateProfile(
      $grpc.ServiceCall call, $0.UpdateProfileRequest request);

  $async.Future<$0.CreateProfileResponse> createProfile_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CreateProfileRequest> $request) async {
    return createProfile($call, await $request);
  }

  $async.Future<$0.CreateProfileResponse> createProfile(
      $grpc.ServiceCall call, $0.CreateProfileRequest request);

  $async.Future<$0.DeleteProfileResponse> deleteProfile_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DeleteProfileRequest> $request) async {
    return deleteProfile($call, await $request);
  }

  $async.Future<$0.DeleteProfileResponse> deleteProfile(
      $grpc.ServiceCall call, $0.DeleteProfileRequest request);

  $async.Future<$0.SwitchProfileResponse> switchProfile_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SwitchProfileRequest> $request) async {
    return switchProfile($call, await $request);
  }

  $async.Future<$0.SwitchProfileResponse> switchProfile(
      $grpc.ServiceCall call, $0.SwitchProfileRequest request);

  $async.Future<$0.ListMyProfilesResponse> listMyProfiles_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListMyProfilesRequest> $request) async {
    return listMyProfiles($call, await $request);
  }

  $async.Future<$0.ListMyProfilesResponse> listMyProfiles(
      $grpc.ServiceCall call, $0.ListMyProfilesRequest request);

  $async.Future<$0.SearchProfilesResponse> searchProfiles_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SearchProfilesRequest> $request) async {
    return searchProfiles($call, await $request);
  }

  $async.Future<$0.SearchProfilesResponse> searchProfiles(
      $grpc.ServiceCall call, $0.SearchProfilesRequest request);

  $async.Future<$0.GetPrivacySettingsResponse> getPrivacySettings_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetPrivacySettingsRequest> $request) async {
    return getPrivacySettings($call, await $request);
  }

  $async.Future<$0.GetPrivacySettingsResponse> getPrivacySettings(
      $grpc.ServiceCall call, $0.GetPrivacySettingsRequest request);

  $async.Future<$0.UpdatePrivacySettingsResponse> updatePrivacySettings_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UpdatePrivacySettingsRequest> $request) async {
    return updatePrivacySettings($call, await $request);
  }

  $async.Future<$0.UpdatePrivacySettingsResponse> updatePrivacySettings(
      $grpc.ServiceCall call, $0.UpdatePrivacySettingsRequest request);

  $async.Future<$0.UpdatePresenceResponse> updatePresence_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UpdatePresenceRequest> $request) async {
    return updatePresence($call, await $request);
  }

  $async.Future<$0.UpdatePresenceResponse> updatePresence(
      $grpc.ServiceCall call, $0.UpdatePresenceRequest request);

  $async.Future<$0.GetPresenceResponse> getPresence_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetPresenceRequest> $request) async {
    return getPresence($call, await $request);
  }

  $async.Future<$0.GetPresenceResponse> getPresence(
      $grpc.ServiceCall call, $0.GetPresenceRequest request);

  $async.Future<$0.GetBulkPresenceResponse> getBulkPresence_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetBulkPresenceRequest> $request) async {
    return getBulkPresence($call, await $request);
  }

  $async.Future<$0.GetBulkPresenceResponse> getBulkPresence(
      $grpc.ServiceCall call, $0.GetBulkPresenceRequest request);

  $async.Future<$0.GetSettingsResponse> getSettings_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetSettingsRequest> $request) async {
    return getSettings($call, await $request);
  }

  $async.Future<$0.GetSettingsResponse> getSettings(
      $grpc.ServiceCall call, $0.GetSettingsRequest request);

  $async.Future<$0.UpdateSettingsResponse> updateSettings_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UpdateSettingsRequest> $request) async {
    return updateSettings($call, await $request);
  }

  $async.Future<$0.UpdateSettingsResponse> updateSettings(
      $grpc.ServiceCall call, $0.UpdateSettingsRequest request);

  $async.Future<$0.GetOnboardingStateResponse> getOnboardingState_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetOnboardingStateRequest> $request) async {
    return getOnboardingState($call, await $request);
  }

  $async.Future<$0.GetOnboardingStateResponse> getOnboardingState(
      $grpc.ServiceCall call, $0.GetOnboardingStateRequest request);

  $async.Future<$0.CompleteOnboardingStepResponse> completeOnboardingStep_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CompleteOnboardingStepRequest> $request) async {
    return completeOnboardingStep($call, await $request);
  }

  $async.Future<$0.CompleteOnboardingStepResponse> completeOnboardingStep(
      $grpc.ServiceCall call, $0.CompleteOnboardingStepRequest request);

  $async.Future<$0.GetVerificationStatusResponse> getVerificationStatus_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetVerificationStatusRequest> $request) async {
    return getVerificationStatus($call, await $request);
  }

  $async.Future<$0.GetVerificationStatusResponse> getVerificationStatus(
      $grpc.ServiceCall call, $0.GetVerificationStatusRequest request);

  $async.Future<$0.SetVerificationResponse> setVerification_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetVerificationRequest> $request) async {
    return setVerification($call, await $request);
  }

  $async.Future<$0.SetVerificationResponse> setVerification(
      $grpc.ServiceCall call, $0.SetVerificationRequest request);

  $async.Future<$0.ClearVerificationResponse> clearVerification_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ClearVerificationRequest> $request) async {
    return clearVerification($call, await $request);
  }

  $async.Future<$0.ClearVerificationResponse> clearVerification(
      $grpc.ServiceCall call, $0.ClearVerificationRequest request);

  $async.Future<$0.StartOrganizationVerificationResponse>
      startOrganizationVerification_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$0.StartOrganizationVerificationRequest>
              $request) async {
    return startOrganizationVerification($call, await $request);
  }

  $async.Future<$0.StartOrganizationVerificationResponse>
      startOrganizationVerification($grpc.ServiceCall call,
          $0.StartOrganizationVerificationRequest request);

  $async.Future<$0.CheckOrganizationVerificationResponse>
      checkOrganizationVerification_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$0.CheckOrganizationVerificationRequest>
              $request) async {
    return checkOrganizationVerification($call, await $request);
  }

  $async.Future<$0.CheckOrganizationVerificationResponse>
      checkOrganizationVerification($grpc.ServiceCall call,
          $0.CheckOrganizationVerificationRequest request);

  $async.Future<$0.ApplyDowngradeProfilesResponse> applyDowngradeProfiles_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ApplyDowngradeProfilesRequest> $request) async {
    return applyDowngradeProfiles($call, await $request);
  }

  $async.Future<$0.ApplyDowngradeProfilesResponse> applyDowngradeProfiles(
      $grpc.ServiceCall call, $0.ApplyDowngradeProfilesRequest request);

  $async.Future<$0.CreateAvatarPresignedUploadResponse>
      createAvatarPresignedUpload_Pre($grpc.ServiceCall $call,
          $async.Future<$0.CreateAvatarPresignedUploadRequest> $request) async {
    return createAvatarPresignedUpload($call, await $request);
  }

  $async.Future<$0.CreateAvatarPresignedUploadResponse>
      createAvatarPresignedUpload($grpc.ServiceCall call,
          $0.CreateAvatarPresignedUploadRequest request);
}
