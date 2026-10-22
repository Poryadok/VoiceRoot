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

import 'space_account_capture.pb.dart' as $0;
import 'user.pb.dart' as $1;

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

  /// Space-only protected capture. Not registered on the ordinary User listener.
  $grpc.ResponseFuture<$0.CaptureSpaceMemberAccountBindingsResponse>
      captureSpaceMemberAccountBindings(
    $0.CaptureSpaceMemberAccountBindingsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$captureSpaceMemberAccountBindings, request,
        options: options);
  }

  /// S2S bootstrap from Auth: ensure one primary profile per account before JWT is issued.
  /// Target: called only from trusted Auth; not exposed via public Gateway REST in v1 DM scope.
  $grpc.ResponseFuture<$1.EnsurePrimaryProfileResponse> ensurePrimaryProfile(
    $1.EnsurePrimaryProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$ensurePrimaryProfile, request, options: options);
  }

  /// S2S internal: profile ids for account-level social actions (block cascade, …).
  $grpc.ResponseFuture<$1.ListProfileIDsForAccountResponse>
      listProfileIDsForAccount(
    $1.ListProfileIDsForAccountRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listProfileIDsForAccount, request,
        options: options);
  }

  /// S2S internal: resolve the owning account for Messaging DM lifecycle checks.
  /// This returns the owner even when the profile is soft-deleted and is not a public visibility lookup.
  $grpc.ResponseFuture<$1.ResolveAccountIDForProfileResponse>
      resolveAccountIDForProfile(
    $1.ResolveAccountIDForProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveAccountIDForProfile, request,
        options: options);
  }

  /// S2S internal: title-only projection for DM peer profile IDs from authorized Chat memberships.
  $grpc.ResponseFuture<$1.GetDMPeerDisplayNamesResponse> getDMPeerDisplayNames(
    $1.GetDMPeerDisplayNamesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getDMPeerDisplayNames, request, options: options);
  }

  /// S2S internal: resolve existing, non-deleted primary profiles for Auth-owned account ids.
  /// Missing accounts and accounts without a usable primary profile are omitted; this never provisions profiles.
  $grpc.ResponseFuture<$1.ResolvePrimaryProfileIDsResponse>
      resolvePrimaryProfileIDs(
    $1.ResolvePrimaryProfileIDsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolvePrimaryProfileIDs, request,
        options: options);
  }

  /// S2S internal: after Auth converts a guest account, clear the guest marker on its User-owned profiles.
  /// Idempotent: an already-regular or unknown account succeeds unchanged.
  $grpc.ResponseFuture<$1.MarkAccountRegularResponse> markAccountRegular(
    $1.MarkAccountRegularRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$markAccountRegular, request, options: options);
  }

  /// Auth-only protected listener: exact read-only SDK target profile snapshot.
  $grpc.ResponseFuture<$1.GetSdkProfileEligibilityResponse>
      getSdkProfileEligibility(
    $1.GetSdkProfileEligibilityRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSdkProfileEligibility, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:auth
  /// Dedicated admission listener only; no SDK consent or target conversion authority.
  $grpc.ResponseFuture<$1.GetSpaceAdmissionProfileEligibilityResponse>
      getSpaceAdmissionProfileEligibility(
    $1.GetSpaceAdmissionProfileEligibilityRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSpaceAdmissionProfileEligibility, request,
        options: options);
  }

  /// Auth-only protected listener: immutable historical SDK actor tombstone receipt.
  $grpc.ResponseFuture<$1.RecordSdkAuthorTombstoneResponse>
      recordSdkAuthorTombstone(
    $1.RecordSdkAuthorTombstoneRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$recordSdkAuthorTombstone, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.GetProfileResponse> getProfile(
    $1.GetProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getProfile, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetProfilesResponse> getProfiles(
    $1.GetProfilesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getProfiles, request, options: options);
  }

  $grpc.ResponseFuture<$1.UpdateProfileResponse> updateProfile(
    $1.UpdateProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateProfile, request, options: options);
  }

  $grpc.ResponseFuture<$1.CreateProfileResponse> createProfile(
    $1.CreateProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createProfile, request, options: options);
  }

  $grpc.ResponseFuture<$1.DeleteProfileResponse> deleteProfile(
    $1.DeleteProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteProfile, request, options: options);
  }

  $grpc.ResponseFuture<$1.SwitchProfileResponse> switchProfile(
    $1.SwitchProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$switchProfile, request, options: options);
  }

  $grpc.ResponseFuture<$1.ListMyProfilesResponse> listMyProfiles(
    $1.ListMyProfilesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listMyProfiles, request, options: options);
  }

  /// Discover profiles by query (username / display_name) for friend requests and DM addressing.
  /// Friend/DM profile discovery — user_db; privacy + blocks enforced. Global search: docs/features/search.md.
  $grpc.ResponseFuture<$1.SearchProfilesResponse> searchProfiles(
    $1.SearchProfilesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$searchProfiles, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetPrivacySettingsResponse> getPrivacySettings(
    $1.GetPrivacySettingsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getPrivacySettings, request, options: options);
  }

  $grpc.ResponseFuture<$1.UpdatePrivacySettingsResponse> updatePrivacySettings(
    $1.UpdatePrivacySettingsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updatePrivacySettings, request, options: options);
  }

  $grpc.ResponseFuture<$1.UpdatePresenceResponse> updatePresence(
    $1.UpdatePresenceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updatePresence, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetPresenceResponse> getPresence(
    $1.GetPresenceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getPresence, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetBulkPresenceResponse> getBulkPresence(
    $1.GetBulkPresenceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getBulkPresence, request, options: options);
  }

  /// Internal Notification routing signal. This method is exposed only on the
  /// dedicated authenticated Notification principal listener.
  $grpc.ResponseFuture<$1.GetNotificationRoutingPresenceResponse>
      getNotificationRoutingPresence(
    $1.GetNotificationRoutingPresenceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getNotificationRoutingPresence, request,
        options: options);
  }

  /// Internal Messaging decision for an owned scheduled dispatch.
  /// Exposed only on the dedicated authenticated Messaging principal listener.
  $grpc.ResponseFuture<$1.GetScheduledMessageDispatchPresenceResponse>
      getScheduledMessageDispatchPresence(
    $1.GetScheduledMessageDispatchPresenceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getScheduledMessageDispatchPresence, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.GetSettingsResponse> getSettings(
    $1.GetSettingsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSettings, request, options: options);
  }

  $grpc.ResponseFuture<$1.UpdateSettingsResponse> updateSettings(
    $1.UpdateSettingsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateSettings, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetOnboardingStateResponse> getOnboardingState(
    $1.GetOnboardingStateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getOnboardingState, request, options: options);
  }

  $grpc.ResponseFuture<$1.CompleteOnboardingStepResponse>
      completeOnboardingStep(
    $1.CompleteOnboardingStepRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$completeOnboardingStep, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.GetVerificationStatusResponse> getVerificationStatus(
    $1.GetVerificationStatusRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getVerificationStatus, request, options: options);
  }

  /// S2S from Auth after OAuth identity checks (verification.md).
  $grpc.ResponseFuture<$1.SetVerificationResponse> setVerification(
    $1.SetVerificationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setVerification, request, options: options);
  }

  $grpc.ResponseFuture<$1.ClearVerificationResponse> clearVerification(
    $1.ClearVerificationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$clearVerification, request, options: options);
  }

  /// S2S from Auth. Applies one monotonic Twitch/YouTube source state and resolves
  /// the profile's effective visible verification without exposing source rows publicly.
  $grpc.ResponseFuture<$1.ApplyVerificationSourceStateResponse>
      applyVerificationSourceState(
    $1.ApplyVerificationSourceStateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$applyVerificationSourceState, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.StartOrganizationVerificationResponse>
      startOrganizationVerification(
    $1.StartOrganizationVerificationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$startOrganizationVerification, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.CheckOrganizationVerificationResponse>
      checkOrganizationVerification(
    $1.CheckOrganizationVerificationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$checkOrganizationVerification, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.ApplyDowngradeProfilesResponse>
      applyDowngradeProfiles(
    $1.ApplyDowngradeProfilesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$applyDowngradeProfiles, request,
        options: options);
  }

  /// Presigned PUT to Cloudflare R2 for static profile avatar (docs/features/user-profile.md).
  $grpc.ResponseFuture<$1.CreateAvatarPresignedUploadResponse>
      createAvatarPresignedUpload(
    $1.CreateAvatarPresignedUploadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createAvatarPresignedUpload, request,
        options: options);
  }

  /// Search-only protected listener. These are intentionally unimplemented on
  /// ordinary User gRPC and require the request-bound service:search principal.
  $grpc.ResponseFuture<$1.BeginSearchProfileSnapshotResponse>
      beginSearchProfileSnapshot(
    $1.BeginSearchProfileSnapshotRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$beginSearchProfileSnapshot, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.ListSearchProfileSnapshotResponse>
      listSearchProfileSnapshot(
    $1.ListSearchProfileSnapshotRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listSearchProfileSnapshot, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.ListSearchProfileJournalResponse>
      listSearchProfileJournal(
    $1.ListSearchProfileJournalRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listSearchProfileJournal, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.GetSearchProfileCheckpointResponse>
      getSearchProfileCheckpoint(
    $1.GetSearchProfileCheckpointRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSearchProfileCheckpoint, request,
        options: options);
  }

  // method descriptors

  static final _$captureSpaceMemberAccountBindings = $grpc.ClientMethod<
          $0.CaptureSpaceMemberAccountBindingsRequest,
          $0.CaptureSpaceMemberAccountBindingsResponse>(
      '/voice.user.v1.UserService/CaptureSpaceMemberAccountBindings',
      ($0.CaptureSpaceMemberAccountBindingsRequest value) =>
          value.writeToBuffer(),
      $0.CaptureSpaceMemberAccountBindingsResponse.fromBuffer);
  static final _$ensurePrimaryProfile = $grpc.ClientMethod<
          $1.EnsurePrimaryProfileRequest, $1.EnsurePrimaryProfileResponse>(
      '/voice.user.v1.UserService/EnsurePrimaryProfile',
      ($1.EnsurePrimaryProfileRequest value) => value.writeToBuffer(),
      $1.EnsurePrimaryProfileResponse.fromBuffer);
  static final _$listProfileIDsForAccount = $grpc.ClientMethod<
          $1.ListProfileIDsForAccountRequest,
          $1.ListProfileIDsForAccountResponse>(
      '/voice.user.v1.UserService/ListProfileIDsForAccount',
      ($1.ListProfileIDsForAccountRequest value) => value.writeToBuffer(),
      $1.ListProfileIDsForAccountResponse.fromBuffer);
  static final _$resolveAccountIDForProfile = $grpc.ClientMethod<
          $1.ResolveAccountIDForProfileRequest,
          $1.ResolveAccountIDForProfileResponse>(
      '/voice.user.v1.UserService/ResolveAccountIDForProfile',
      ($1.ResolveAccountIDForProfileRequest value) => value.writeToBuffer(),
      $1.ResolveAccountIDForProfileResponse.fromBuffer);
  static final _$getDMPeerDisplayNames = $grpc.ClientMethod<
          $1.GetDMPeerDisplayNamesRequest, $1.GetDMPeerDisplayNamesResponse>(
      '/voice.user.v1.UserService/GetDMPeerDisplayNames',
      ($1.GetDMPeerDisplayNamesRequest value) => value.writeToBuffer(),
      $1.GetDMPeerDisplayNamesResponse.fromBuffer);
  static final _$resolvePrimaryProfileIDs = $grpc.ClientMethod<
          $1.ResolvePrimaryProfileIDsRequest,
          $1.ResolvePrimaryProfileIDsResponse>(
      '/voice.user.v1.UserService/ResolvePrimaryProfileIDs',
      ($1.ResolvePrimaryProfileIDsRequest value) => value.writeToBuffer(),
      $1.ResolvePrimaryProfileIDsResponse.fromBuffer);
  static final _$markAccountRegular = $grpc.ClientMethod<
          $1.MarkAccountRegularRequest, $1.MarkAccountRegularResponse>(
      '/voice.user.v1.UserService/MarkAccountRegular',
      ($1.MarkAccountRegularRequest value) => value.writeToBuffer(),
      $1.MarkAccountRegularResponse.fromBuffer);
  static final _$getSdkProfileEligibility = $grpc.ClientMethod<
          $1.GetSdkProfileEligibilityRequest,
          $1.GetSdkProfileEligibilityResponse>(
      '/voice.user.v1.UserService/GetSdkProfileEligibility',
      ($1.GetSdkProfileEligibilityRequest value) => value.writeToBuffer(),
      $1.GetSdkProfileEligibilityResponse.fromBuffer);
  static final _$getSpaceAdmissionProfileEligibility = $grpc.ClientMethod<
          $1.GetSpaceAdmissionProfileEligibilityRequest,
          $1.GetSpaceAdmissionProfileEligibilityResponse>(
      '/voice.user.v1.UserService/GetSpaceAdmissionProfileEligibility',
      ($1.GetSpaceAdmissionProfileEligibilityRequest value) =>
          value.writeToBuffer(),
      $1.GetSpaceAdmissionProfileEligibilityResponse.fromBuffer);
  static final _$recordSdkAuthorTombstone = $grpc.ClientMethod<
          $1.RecordSdkAuthorTombstoneRequest,
          $1.RecordSdkAuthorTombstoneResponse>(
      '/voice.user.v1.UserService/RecordSdkAuthorTombstone',
      ($1.RecordSdkAuthorTombstoneRequest value) => value.writeToBuffer(),
      $1.RecordSdkAuthorTombstoneResponse.fromBuffer);
  static final _$getProfile =
      $grpc.ClientMethod<$1.GetProfileRequest, $1.GetProfileResponse>(
          '/voice.user.v1.UserService/GetProfile',
          ($1.GetProfileRequest value) => value.writeToBuffer(),
          $1.GetProfileResponse.fromBuffer);
  static final _$getProfiles =
      $grpc.ClientMethod<$1.GetProfilesRequest, $1.GetProfilesResponse>(
          '/voice.user.v1.UserService/GetProfiles',
          ($1.GetProfilesRequest value) => value.writeToBuffer(),
          $1.GetProfilesResponse.fromBuffer);
  static final _$updateProfile =
      $grpc.ClientMethod<$1.UpdateProfileRequest, $1.UpdateProfileResponse>(
          '/voice.user.v1.UserService/UpdateProfile',
          ($1.UpdateProfileRequest value) => value.writeToBuffer(),
          $1.UpdateProfileResponse.fromBuffer);
  static final _$createProfile =
      $grpc.ClientMethod<$1.CreateProfileRequest, $1.CreateProfileResponse>(
          '/voice.user.v1.UserService/CreateProfile',
          ($1.CreateProfileRequest value) => value.writeToBuffer(),
          $1.CreateProfileResponse.fromBuffer);
  static final _$deleteProfile =
      $grpc.ClientMethod<$1.DeleteProfileRequest, $1.DeleteProfileResponse>(
          '/voice.user.v1.UserService/DeleteProfile',
          ($1.DeleteProfileRequest value) => value.writeToBuffer(),
          $1.DeleteProfileResponse.fromBuffer);
  static final _$switchProfile =
      $grpc.ClientMethod<$1.SwitchProfileRequest, $1.SwitchProfileResponse>(
          '/voice.user.v1.UserService/SwitchProfile',
          ($1.SwitchProfileRequest value) => value.writeToBuffer(),
          $1.SwitchProfileResponse.fromBuffer);
  static final _$listMyProfiles =
      $grpc.ClientMethod<$1.ListMyProfilesRequest, $1.ListMyProfilesResponse>(
          '/voice.user.v1.UserService/ListMyProfiles',
          ($1.ListMyProfilesRequest value) => value.writeToBuffer(),
          $1.ListMyProfilesResponse.fromBuffer);
  static final _$searchProfiles =
      $grpc.ClientMethod<$1.SearchProfilesRequest, $1.SearchProfilesResponse>(
          '/voice.user.v1.UserService/SearchProfiles',
          ($1.SearchProfilesRequest value) => value.writeToBuffer(),
          $1.SearchProfilesResponse.fromBuffer);
  static final _$getPrivacySettings = $grpc.ClientMethod<
          $1.GetPrivacySettingsRequest, $1.GetPrivacySettingsResponse>(
      '/voice.user.v1.UserService/GetPrivacySettings',
      ($1.GetPrivacySettingsRequest value) => value.writeToBuffer(),
      $1.GetPrivacySettingsResponse.fromBuffer);
  static final _$updatePrivacySettings = $grpc.ClientMethod<
          $1.UpdatePrivacySettingsRequest, $1.UpdatePrivacySettingsResponse>(
      '/voice.user.v1.UserService/UpdatePrivacySettings',
      ($1.UpdatePrivacySettingsRequest value) => value.writeToBuffer(),
      $1.UpdatePrivacySettingsResponse.fromBuffer);
  static final _$updatePresence =
      $grpc.ClientMethod<$1.UpdatePresenceRequest, $1.UpdatePresenceResponse>(
          '/voice.user.v1.UserService/UpdatePresence',
          ($1.UpdatePresenceRequest value) => value.writeToBuffer(),
          $1.UpdatePresenceResponse.fromBuffer);
  static final _$getPresence =
      $grpc.ClientMethod<$1.GetPresenceRequest, $1.GetPresenceResponse>(
          '/voice.user.v1.UserService/GetPresence',
          ($1.GetPresenceRequest value) => value.writeToBuffer(),
          $1.GetPresenceResponse.fromBuffer);
  static final _$getBulkPresence =
      $grpc.ClientMethod<$1.GetBulkPresenceRequest, $1.GetBulkPresenceResponse>(
          '/voice.user.v1.UserService/GetBulkPresence',
          ($1.GetBulkPresenceRequest value) => value.writeToBuffer(),
          $1.GetBulkPresenceResponse.fromBuffer);
  static final _$getNotificationRoutingPresence = $grpc.ClientMethod<
          $1.GetNotificationRoutingPresenceRequest,
          $1.GetNotificationRoutingPresenceResponse>(
      '/voice.user.v1.UserService/GetNotificationRoutingPresence',
      ($1.GetNotificationRoutingPresenceRequest value) => value.writeToBuffer(),
      $1.GetNotificationRoutingPresenceResponse.fromBuffer);
  static final _$getScheduledMessageDispatchPresence = $grpc.ClientMethod<
          $1.GetScheduledMessageDispatchPresenceRequest,
          $1.GetScheduledMessageDispatchPresenceResponse>(
      '/voice.user.v1.UserService/GetScheduledMessageDispatchPresence',
      ($1.GetScheduledMessageDispatchPresenceRequest value) =>
          value.writeToBuffer(),
      $1.GetScheduledMessageDispatchPresenceResponse.fromBuffer);
  static final _$getSettings =
      $grpc.ClientMethod<$1.GetSettingsRequest, $1.GetSettingsResponse>(
          '/voice.user.v1.UserService/GetSettings',
          ($1.GetSettingsRequest value) => value.writeToBuffer(),
          $1.GetSettingsResponse.fromBuffer);
  static final _$updateSettings =
      $grpc.ClientMethod<$1.UpdateSettingsRequest, $1.UpdateSettingsResponse>(
          '/voice.user.v1.UserService/UpdateSettings',
          ($1.UpdateSettingsRequest value) => value.writeToBuffer(),
          $1.UpdateSettingsResponse.fromBuffer);
  static final _$getOnboardingState = $grpc.ClientMethod<
          $1.GetOnboardingStateRequest, $1.GetOnboardingStateResponse>(
      '/voice.user.v1.UserService/GetOnboardingState',
      ($1.GetOnboardingStateRequest value) => value.writeToBuffer(),
      $1.GetOnboardingStateResponse.fromBuffer);
  static final _$completeOnboardingStep = $grpc.ClientMethod<
          $1.CompleteOnboardingStepRequest, $1.CompleteOnboardingStepResponse>(
      '/voice.user.v1.UserService/CompleteOnboardingStep',
      ($1.CompleteOnboardingStepRequest value) => value.writeToBuffer(),
      $1.CompleteOnboardingStepResponse.fromBuffer);
  static final _$getVerificationStatus = $grpc.ClientMethod<
          $1.GetVerificationStatusRequest, $1.GetVerificationStatusResponse>(
      '/voice.user.v1.UserService/GetVerificationStatus',
      ($1.GetVerificationStatusRequest value) => value.writeToBuffer(),
      $1.GetVerificationStatusResponse.fromBuffer);
  static final _$setVerification =
      $grpc.ClientMethod<$1.SetVerificationRequest, $1.SetVerificationResponse>(
          '/voice.user.v1.UserService/SetVerification',
          ($1.SetVerificationRequest value) => value.writeToBuffer(),
          $1.SetVerificationResponse.fromBuffer);
  static final _$clearVerification = $grpc.ClientMethod<
          $1.ClearVerificationRequest, $1.ClearVerificationResponse>(
      '/voice.user.v1.UserService/ClearVerification',
      ($1.ClearVerificationRequest value) => value.writeToBuffer(),
      $1.ClearVerificationResponse.fromBuffer);
  static final _$applyVerificationSourceState = $grpc.ClientMethod<
          $1.ApplyVerificationSourceStateRequest,
          $1.ApplyVerificationSourceStateResponse>(
      '/voice.user.v1.UserService/ApplyVerificationSourceState',
      ($1.ApplyVerificationSourceStateRequest value) => value.writeToBuffer(),
      $1.ApplyVerificationSourceStateResponse.fromBuffer);
  static final _$startOrganizationVerification = $grpc.ClientMethod<
          $1.StartOrganizationVerificationRequest,
          $1.StartOrganizationVerificationResponse>(
      '/voice.user.v1.UserService/StartOrganizationVerification',
      ($1.StartOrganizationVerificationRequest value) => value.writeToBuffer(),
      $1.StartOrganizationVerificationResponse.fromBuffer);
  static final _$checkOrganizationVerification = $grpc.ClientMethod<
          $1.CheckOrganizationVerificationRequest,
          $1.CheckOrganizationVerificationResponse>(
      '/voice.user.v1.UserService/CheckOrganizationVerification',
      ($1.CheckOrganizationVerificationRequest value) => value.writeToBuffer(),
      $1.CheckOrganizationVerificationResponse.fromBuffer);
  static final _$applyDowngradeProfiles = $grpc.ClientMethod<
          $1.ApplyDowngradeProfilesRequest, $1.ApplyDowngradeProfilesResponse>(
      '/voice.user.v1.UserService/ApplyDowngradeProfiles',
      ($1.ApplyDowngradeProfilesRequest value) => value.writeToBuffer(),
      $1.ApplyDowngradeProfilesResponse.fromBuffer);
  static final _$createAvatarPresignedUpload = $grpc.ClientMethod<
          $1.CreateAvatarPresignedUploadRequest,
          $1.CreateAvatarPresignedUploadResponse>(
      '/voice.user.v1.UserService/CreateAvatarPresignedUpload',
      ($1.CreateAvatarPresignedUploadRequest value) => value.writeToBuffer(),
      $1.CreateAvatarPresignedUploadResponse.fromBuffer);
  static final _$beginSearchProfileSnapshot = $grpc.ClientMethod<
          $1.BeginSearchProfileSnapshotRequest,
          $1.BeginSearchProfileSnapshotResponse>(
      '/voice.user.v1.UserService/BeginSearchProfileSnapshot',
      ($1.BeginSearchProfileSnapshotRequest value) => value.writeToBuffer(),
      $1.BeginSearchProfileSnapshotResponse.fromBuffer);
  static final _$listSearchProfileSnapshot = $grpc.ClientMethod<
          $1.ListSearchProfileSnapshotRequest,
          $1.ListSearchProfileSnapshotResponse>(
      '/voice.user.v1.UserService/ListSearchProfileSnapshot',
      ($1.ListSearchProfileSnapshotRequest value) => value.writeToBuffer(),
      $1.ListSearchProfileSnapshotResponse.fromBuffer);
  static final _$listSearchProfileJournal = $grpc.ClientMethod<
          $1.ListSearchProfileJournalRequest,
          $1.ListSearchProfileJournalResponse>(
      '/voice.user.v1.UserService/ListSearchProfileJournal',
      ($1.ListSearchProfileJournalRequest value) => value.writeToBuffer(),
      $1.ListSearchProfileJournalResponse.fromBuffer);
  static final _$getSearchProfileCheckpoint = $grpc.ClientMethod<
          $1.GetSearchProfileCheckpointRequest,
          $1.GetSearchProfileCheckpointResponse>(
      '/voice.user.v1.UserService/GetSearchProfileCheckpoint',
      ($1.GetSearchProfileCheckpointRequest value) => value.writeToBuffer(),
      $1.GetSearchProfileCheckpointResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.user.v1.UserService')
abstract class UserServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.user.v1.UserService';

  UserServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.CaptureSpaceMemberAccountBindingsRequest,
            $0.CaptureSpaceMemberAccountBindingsResponse>(
        'CaptureSpaceMemberAccountBindings',
        captureSpaceMemberAccountBindings_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CaptureSpaceMemberAccountBindingsRequest.fromBuffer(value),
        ($0.CaptureSpaceMemberAccountBindingsResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.EnsurePrimaryProfileRequest,
            $1.EnsurePrimaryProfileResponse>(
        'EnsurePrimaryProfile',
        ensurePrimaryProfile_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.EnsurePrimaryProfileRequest.fromBuffer(value),
        ($1.EnsurePrimaryProfileResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ListProfileIDsForAccountRequest,
            $1.ListProfileIDsForAccountResponse>(
        'ListProfileIDsForAccount',
        listProfileIDsForAccount_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ListProfileIDsForAccountRequest.fromBuffer(value),
        ($1.ListProfileIDsForAccountResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ResolveAccountIDForProfileRequest,
            $1.ResolveAccountIDForProfileResponse>(
        'ResolveAccountIDForProfile',
        resolveAccountIDForProfile_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ResolveAccountIDForProfileRequest.fromBuffer(value),
        ($1.ResolveAccountIDForProfileResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetDMPeerDisplayNamesRequest,
            $1.GetDMPeerDisplayNamesResponse>(
        'GetDMPeerDisplayNames',
        getDMPeerDisplayNames_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetDMPeerDisplayNamesRequest.fromBuffer(value),
        ($1.GetDMPeerDisplayNamesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ResolvePrimaryProfileIDsRequest,
            $1.ResolvePrimaryProfileIDsResponse>(
        'ResolvePrimaryProfileIDs',
        resolvePrimaryProfileIDs_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ResolvePrimaryProfileIDsRequest.fromBuffer(value),
        ($1.ResolvePrimaryProfileIDsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.MarkAccountRegularRequest,
            $1.MarkAccountRegularResponse>(
        'MarkAccountRegular',
        markAccountRegular_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.MarkAccountRegularRequest.fromBuffer(value),
        ($1.MarkAccountRegularResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetSdkProfileEligibilityRequest,
            $1.GetSdkProfileEligibilityResponse>(
        'GetSdkProfileEligibility',
        getSdkProfileEligibility_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetSdkProfileEligibilityRequest.fromBuffer(value),
        ($1.GetSdkProfileEligibilityResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<
            $1.GetSpaceAdmissionProfileEligibilityRequest,
            $1.GetSpaceAdmissionProfileEligibilityResponse>(
        'GetSpaceAdmissionProfileEligibility',
        getSpaceAdmissionProfileEligibility_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetSpaceAdmissionProfileEligibilityRequest.fromBuffer(value),
        ($1.GetSpaceAdmissionProfileEligibilityResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.RecordSdkAuthorTombstoneRequest,
            $1.RecordSdkAuthorTombstoneResponse>(
        'RecordSdkAuthorTombstone',
        recordSdkAuthorTombstone_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.RecordSdkAuthorTombstoneRequest.fromBuffer(value),
        ($1.RecordSdkAuthorTombstoneResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetProfileRequest, $1.GetProfileResponse>(
        'GetProfile',
        getProfile_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.GetProfileRequest.fromBuffer(value),
        ($1.GetProfileResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.GetProfilesRequest, $1.GetProfilesResponse>(
            'GetProfiles',
            getProfiles_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.GetProfilesRequest.fromBuffer(value),
            ($1.GetProfilesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.UpdateProfileRequest, $1.UpdateProfileResponse>(
            'UpdateProfile',
            updateProfile_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.UpdateProfileRequest.fromBuffer(value),
            ($1.UpdateProfileResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.CreateProfileRequest, $1.CreateProfileResponse>(
            'CreateProfile',
            createProfile_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.CreateProfileRequest.fromBuffer(value),
            ($1.CreateProfileResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.DeleteProfileRequest, $1.DeleteProfileResponse>(
            'DeleteProfile',
            deleteProfile_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.DeleteProfileRequest.fromBuffer(value),
            ($1.DeleteProfileResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.SwitchProfileRequest, $1.SwitchProfileResponse>(
            'SwitchProfile',
            switchProfile_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.SwitchProfileRequest.fromBuffer(value),
            ($1.SwitchProfileResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ListMyProfilesRequest,
            $1.ListMyProfilesResponse>(
        'ListMyProfiles',
        listMyProfiles_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ListMyProfilesRequest.fromBuffer(value),
        ($1.ListMyProfilesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.SearchProfilesRequest,
            $1.SearchProfilesResponse>(
        'SearchProfiles',
        searchProfiles_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.SearchProfilesRequest.fromBuffer(value),
        ($1.SearchProfilesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetPrivacySettingsRequest,
            $1.GetPrivacySettingsResponse>(
        'GetPrivacySettings',
        getPrivacySettings_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetPrivacySettingsRequest.fromBuffer(value),
        ($1.GetPrivacySettingsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.UpdatePrivacySettingsRequest,
            $1.UpdatePrivacySettingsResponse>(
        'UpdatePrivacySettings',
        updatePrivacySettings_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.UpdatePrivacySettingsRequest.fromBuffer(value),
        ($1.UpdatePrivacySettingsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.UpdatePresenceRequest,
            $1.UpdatePresenceResponse>(
        'UpdatePresence',
        updatePresence_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.UpdatePresenceRequest.fromBuffer(value),
        ($1.UpdatePresenceResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.GetPresenceRequest, $1.GetPresenceResponse>(
            'GetPresence',
            getPresence_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.GetPresenceRequest.fromBuffer(value),
            ($1.GetPresenceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetBulkPresenceRequest,
            $1.GetBulkPresenceResponse>(
        'GetBulkPresence',
        getBulkPresence_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetBulkPresenceRequest.fromBuffer(value),
        ($1.GetBulkPresenceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetNotificationRoutingPresenceRequest,
            $1.GetNotificationRoutingPresenceResponse>(
        'GetNotificationRoutingPresence',
        getNotificationRoutingPresence_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetNotificationRoutingPresenceRequest.fromBuffer(value),
        ($1.GetNotificationRoutingPresenceResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<
            $1.GetScheduledMessageDispatchPresenceRequest,
            $1.GetScheduledMessageDispatchPresenceResponse>(
        'GetScheduledMessageDispatchPresence',
        getScheduledMessageDispatchPresence_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetScheduledMessageDispatchPresenceRequest.fromBuffer(value),
        ($1.GetScheduledMessageDispatchPresenceResponse value) =>
            value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.GetSettingsRequest, $1.GetSettingsResponse>(
            'GetSettings',
            getSettings_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.GetSettingsRequest.fromBuffer(value),
            ($1.GetSettingsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.UpdateSettingsRequest,
            $1.UpdateSettingsResponse>(
        'UpdateSettings',
        updateSettings_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.UpdateSettingsRequest.fromBuffer(value),
        ($1.UpdateSettingsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetOnboardingStateRequest,
            $1.GetOnboardingStateResponse>(
        'GetOnboardingState',
        getOnboardingState_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetOnboardingStateRequest.fromBuffer(value),
        ($1.GetOnboardingStateResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.CompleteOnboardingStepRequest,
            $1.CompleteOnboardingStepResponse>(
        'CompleteOnboardingStep',
        completeOnboardingStep_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.CompleteOnboardingStepRequest.fromBuffer(value),
        ($1.CompleteOnboardingStepResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetVerificationStatusRequest,
            $1.GetVerificationStatusResponse>(
        'GetVerificationStatus',
        getVerificationStatus_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetVerificationStatusRequest.fromBuffer(value),
        ($1.GetVerificationStatusResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.SetVerificationRequest,
            $1.SetVerificationResponse>(
        'SetVerification',
        setVerification_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.SetVerificationRequest.fromBuffer(value),
        ($1.SetVerificationResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ClearVerificationRequest,
            $1.ClearVerificationResponse>(
        'ClearVerification',
        clearVerification_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ClearVerificationRequest.fromBuffer(value),
        ($1.ClearVerificationResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ApplyVerificationSourceStateRequest,
            $1.ApplyVerificationSourceStateResponse>(
        'ApplyVerificationSourceState',
        applyVerificationSourceState_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ApplyVerificationSourceStateRequest.fromBuffer(value),
        ($1.ApplyVerificationSourceStateResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.StartOrganizationVerificationRequest,
            $1.StartOrganizationVerificationResponse>(
        'StartOrganizationVerification',
        startOrganizationVerification_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.StartOrganizationVerificationRequest.fromBuffer(value),
        ($1.StartOrganizationVerificationResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.CheckOrganizationVerificationRequest,
            $1.CheckOrganizationVerificationResponse>(
        'CheckOrganizationVerification',
        checkOrganizationVerification_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.CheckOrganizationVerificationRequest.fromBuffer(value),
        ($1.CheckOrganizationVerificationResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ApplyDowngradeProfilesRequest,
            $1.ApplyDowngradeProfilesResponse>(
        'ApplyDowngradeProfiles',
        applyDowngradeProfiles_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ApplyDowngradeProfilesRequest.fromBuffer(value),
        ($1.ApplyDowngradeProfilesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.CreateAvatarPresignedUploadRequest,
            $1.CreateAvatarPresignedUploadResponse>(
        'CreateAvatarPresignedUpload',
        createAvatarPresignedUpload_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.CreateAvatarPresignedUploadRequest.fromBuffer(value),
        ($1.CreateAvatarPresignedUploadResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.BeginSearchProfileSnapshotRequest,
            $1.BeginSearchProfileSnapshotResponse>(
        'BeginSearchProfileSnapshot',
        beginSearchProfileSnapshot_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.BeginSearchProfileSnapshotRequest.fromBuffer(value),
        ($1.BeginSearchProfileSnapshotResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ListSearchProfileSnapshotRequest,
            $1.ListSearchProfileSnapshotResponse>(
        'ListSearchProfileSnapshot',
        listSearchProfileSnapshot_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ListSearchProfileSnapshotRequest.fromBuffer(value),
        ($1.ListSearchProfileSnapshotResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ListSearchProfileJournalRequest,
            $1.ListSearchProfileJournalResponse>(
        'ListSearchProfileJournal',
        listSearchProfileJournal_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ListSearchProfileJournalRequest.fromBuffer(value),
        ($1.ListSearchProfileJournalResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetSearchProfileCheckpointRequest,
            $1.GetSearchProfileCheckpointResponse>(
        'GetSearchProfileCheckpoint',
        getSearchProfileCheckpoint_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetSearchProfileCheckpointRequest.fromBuffer(value),
        ($1.GetSearchProfileCheckpointResponse value) =>
            value.writeToBuffer()));
  }

  $async.Future<$0.CaptureSpaceMemberAccountBindingsResponse>
      captureSpaceMemberAccountBindings_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$0.CaptureSpaceMemberAccountBindingsRequest>
              $request) async {
    return captureSpaceMemberAccountBindings($call, await $request);
  }

  $async.Future<$0.CaptureSpaceMemberAccountBindingsResponse>
      captureSpaceMemberAccountBindings($grpc.ServiceCall call,
          $0.CaptureSpaceMemberAccountBindingsRequest request);

  $async.Future<$1.EnsurePrimaryProfileResponse> ensurePrimaryProfile_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.EnsurePrimaryProfileRequest> $request) async {
    return ensurePrimaryProfile($call, await $request);
  }

  $async.Future<$1.EnsurePrimaryProfileResponse> ensurePrimaryProfile(
      $grpc.ServiceCall call, $1.EnsurePrimaryProfileRequest request);

  $async.Future<$1.ListProfileIDsForAccountResponse>
      listProfileIDsForAccount_Pre($grpc.ServiceCall $call,
          $async.Future<$1.ListProfileIDsForAccountRequest> $request) async {
    return listProfileIDsForAccount($call, await $request);
  }

  $async.Future<$1.ListProfileIDsForAccountResponse> listProfileIDsForAccount(
      $grpc.ServiceCall call, $1.ListProfileIDsForAccountRequest request);

  $async.Future<$1.ResolveAccountIDForProfileResponse>
      resolveAccountIDForProfile_Pre($grpc.ServiceCall $call,
          $async.Future<$1.ResolveAccountIDForProfileRequest> $request) async {
    return resolveAccountIDForProfile($call, await $request);
  }

  $async.Future<$1.ResolveAccountIDForProfileResponse>
      resolveAccountIDForProfile(
          $grpc.ServiceCall call, $1.ResolveAccountIDForProfileRequest request);

  $async.Future<$1.GetDMPeerDisplayNamesResponse> getDMPeerDisplayNames_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetDMPeerDisplayNamesRequest> $request) async {
    return getDMPeerDisplayNames($call, await $request);
  }

  $async.Future<$1.GetDMPeerDisplayNamesResponse> getDMPeerDisplayNames(
      $grpc.ServiceCall call, $1.GetDMPeerDisplayNamesRequest request);

  $async.Future<$1.ResolvePrimaryProfileIDsResponse>
      resolvePrimaryProfileIDs_Pre($grpc.ServiceCall $call,
          $async.Future<$1.ResolvePrimaryProfileIDsRequest> $request) async {
    return resolvePrimaryProfileIDs($call, await $request);
  }

  $async.Future<$1.ResolvePrimaryProfileIDsResponse> resolvePrimaryProfileIDs(
      $grpc.ServiceCall call, $1.ResolvePrimaryProfileIDsRequest request);

  $async.Future<$1.MarkAccountRegularResponse> markAccountRegular_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.MarkAccountRegularRequest> $request) async {
    return markAccountRegular($call, await $request);
  }

  $async.Future<$1.MarkAccountRegularResponse> markAccountRegular(
      $grpc.ServiceCall call, $1.MarkAccountRegularRequest request);

  $async.Future<$1.GetSdkProfileEligibilityResponse>
      getSdkProfileEligibility_Pre($grpc.ServiceCall $call,
          $async.Future<$1.GetSdkProfileEligibilityRequest> $request) async {
    return getSdkProfileEligibility($call, await $request);
  }

  $async.Future<$1.GetSdkProfileEligibilityResponse> getSdkProfileEligibility(
      $grpc.ServiceCall call, $1.GetSdkProfileEligibilityRequest request);

  $async.Future<$1.GetSpaceAdmissionProfileEligibilityResponse>
      getSpaceAdmissionProfileEligibility_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$1.GetSpaceAdmissionProfileEligibilityRequest>
              $request) async {
    return getSpaceAdmissionProfileEligibility($call, await $request);
  }

  $async.Future<$1.GetSpaceAdmissionProfileEligibilityResponse>
      getSpaceAdmissionProfileEligibility($grpc.ServiceCall call,
          $1.GetSpaceAdmissionProfileEligibilityRequest request);

  $async.Future<$1.RecordSdkAuthorTombstoneResponse>
      recordSdkAuthorTombstone_Pre($grpc.ServiceCall $call,
          $async.Future<$1.RecordSdkAuthorTombstoneRequest> $request) async {
    return recordSdkAuthorTombstone($call, await $request);
  }

  $async.Future<$1.RecordSdkAuthorTombstoneResponse> recordSdkAuthorTombstone(
      $grpc.ServiceCall call, $1.RecordSdkAuthorTombstoneRequest request);

  $async.Future<$1.GetProfileResponse> getProfile_Pre($grpc.ServiceCall $call,
      $async.Future<$1.GetProfileRequest> $request) async {
    return getProfile($call, await $request);
  }

  $async.Future<$1.GetProfileResponse> getProfile(
      $grpc.ServiceCall call, $1.GetProfileRequest request);

  $async.Future<$1.GetProfilesResponse> getProfiles_Pre($grpc.ServiceCall $call,
      $async.Future<$1.GetProfilesRequest> $request) async {
    return getProfiles($call, await $request);
  }

  $async.Future<$1.GetProfilesResponse> getProfiles(
      $grpc.ServiceCall call, $1.GetProfilesRequest request);

  $async.Future<$1.UpdateProfileResponse> updateProfile_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UpdateProfileRequest> $request) async {
    return updateProfile($call, await $request);
  }

  $async.Future<$1.UpdateProfileResponse> updateProfile(
      $grpc.ServiceCall call, $1.UpdateProfileRequest request);

  $async.Future<$1.CreateProfileResponse> createProfile_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.CreateProfileRequest> $request) async {
    return createProfile($call, await $request);
  }

  $async.Future<$1.CreateProfileResponse> createProfile(
      $grpc.ServiceCall call, $1.CreateProfileRequest request);

  $async.Future<$1.DeleteProfileResponse> deleteProfile_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.DeleteProfileRequest> $request) async {
    return deleteProfile($call, await $request);
  }

  $async.Future<$1.DeleteProfileResponse> deleteProfile(
      $grpc.ServiceCall call, $1.DeleteProfileRequest request);

  $async.Future<$1.SwitchProfileResponse> switchProfile_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.SwitchProfileRequest> $request) async {
    return switchProfile($call, await $request);
  }

  $async.Future<$1.SwitchProfileResponse> switchProfile(
      $grpc.ServiceCall call, $1.SwitchProfileRequest request);

  $async.Future<$1.ListMyProfilesResponse> listMyProfiles_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ListMyProfilesRequest> $request) async {
    return listMyProfiles($call, await $request);
  }

  $async.Future<$1.ListMyProfilesResponse> listMyProfiles(
      $grpc.ServiceCall call, $1.ListMyProfilesRequest request);

  $async.Future<$1.SearchProfilesResponse> searchProfiles_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.SearchProfilesRequest> $request) async {
    return searchProfiles($call, await $request);
  }

  $async.Future<$1.SearchProfilesResponse> searchProfiles(
      $grpc.ServiceCall call, $1.SearchProfilesRequest request);

  $async.Future<$1.GetPrivacySettingsResponse> getPrivacySettings_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetPrivacySettingsRequest> $request) async {
    return getPrivacySettings($call, await $request);
  }

  $async.Future<$1.GetPrivacySettingsResponse> getPrivacySettings(
      $grpc.ServiceCall call, $1.GetPrivacySettingsRequest request);

  $async.Future<$1.UpdatePrivacySettingsResponse> updatePrivacySettings_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UpdatePrivacySettingsRequest> $request) async {
    return updatePrivacySettings($call, await $request);
  }

  $async.Future<$1.UpdatePrivacySettingsResponse> updatePrivacySettings(
      $grpc.ServiceCall call, $1.UpdatePrivacySettingsRequest request);

  $async.Future<$1.UpdatePresenceResponse> updatePresence_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UpdatePresenceRequest> $request) async {
    return updatePresence($call, await $request);
  }

  $async.Future<$1.UpdatePresenceResponse> updatePresence(
      $grpc.ServiceCall call, $1.UpdatePresenceRequest request);

  $async.Future<$1.GetPresenceResponse> getPresence_Pre($grpc.ServiceCall $call,
      $async.Future<$1.GetPresenceRequest> $request) async {
    return getPresence($call, await $request);
  }

  $async.Future<$1.GetPresenceResponse> getPresence(
      $grpc.ServiceCall call, $1.GetPresenceRequest request);

  $async.Future<$1.GetBulkPresenceResponse> getBulkPresence_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetBulkPresenceRequest> $request) async {
    return getBulkPresence($call, await $request);
  }

  $async.Future<$1.GetBulkPresenceResponse> getBulkPresence(
      $grpc.ServiceCall call, $1.GetBulkPresenceRequest request);

  $async.Future<$1.GetNotificationRoutingPresenceResponse>
      getNotificationRoutingPresence_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$1.GetNotificationRoutingPresenceRequest>
              $request) async {
    return getNotificationRoutingPresence($call, await $request);
  }

  $async.Future<$1.GetNotificationRoutingPresenceResponse>
      getNotificationRoutingPresence($grpc.ServiceCall call,
          $1.GetNotificationRoutingPresenceRequest request);

  $async.Future<$1.GetScheduledMessageDispatchPresenceResponse>
      getScheduledMessageDispatchPresence_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$1.GetScheduledMessageDispatchPresenceRequest>
              $request) async {
    return getScheduledMessageDispatchPresence($call, await $request);
  }

  $async.Future<$1.GetScheduledMessageDispatchPresenceResponse>
      getScheduledMessageDispatchPresence($grpc.ServiceCall call,
          $1.GetScheduledMessageDispatchPresenceRequest request);

  $async.Future<$1.GetSettingsResponse> getSettings_Pre($grpc.ServiceCall $call,
      $async.Future<$1.GetSettingsRequest> $request) async {
    return getSettings($call, await $request);
  }

  $async.Future<$1.GetSettingsResponse> getSettings(
      $grpc.ServiceCall call, $1.GetSettingsRequest request);

  $async.Future<$1.UpdateSettingsResponse> updateSettings_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UpdateSettingsRequest> $request) async {
    return updateSettings($call, await $request);
  }

  $async.Future<$1.UpdateSettingsResponse> updateSettings(
      $grpc.ServiceCall call, $1.UpdateSettingsRequest request);

  $async.Future<$1.GetOnboardingStateResponse> getOnboardingState_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetOnboardingStateRequest> $request) async {
    return getOnboardingState($call, await $request);
  }

  $async.Future<$1.GetOnboardingStateResponse> getOnboardingState(
      $grpc.ServiceCall call, $1.GetOnboardingStateRequest request);

  $async.Future<$1.CompleteOnboardingStepResponse> completeOnboardingStep_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.CompleteOnboardingStepRequest> $request) async {
    return completeOnboardingStep($call, await $request);
  }

  $async.Future<$1.CompleteOnboardingStepResponse> completeOnboardingStep(
      $grpc.ServiceCall call, $1.CompleteOnboardingStepRequest request);

  $async.Future<$1.GetVerificationStatusResponse> getVerificationStatus_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetVerificationStatusRequest> $request) async {
    return getVerificationStatus($call, await $request);
  }

  $async.Future<$1.GetVerificationStatusResponse> getVerificationStatus(
      $grpc.ServiceCall call, $1.GetVerificationStatusRequest request);

  $async.Future<$1.SetVerificationResponse> setVerification_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.SetVerificationRequest> $request) async {
    return setVerification($call, await $request);
  }

  $async.Future<$1.SetVerificationResponse> setVerification(
      $grpc.ServiceCall call, $1.SetVerificationRequest request);

  $async.Future<$1.ClearVerificationResponse> clearVerification_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ClearVerificationRequest> $request) async {
    return clearVerification($call, await $request);
  }

  $async.Future<$1.ClearVerificationResponse> clearVerification(
      $grpc.ServiceCall call, $1.ClearVerificationRequest request);

  $async.Future<$1.ApplyVerificationSourceStateResponse>
      applyVerificationSourceState_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$1.ApplyVerificationSourceStateRequest>
              $request) async {
    return applyVerificationSourceState($call, await $request);
  }

  $async.Future<$1.ApplyVerificationSourceStateResponse>
      applyVerificationSourceState($grpc.ServiceCall call,
          $1.ApplyVerificationSourceStateRequest request);

  $async.Future<$1.StartOrganizationVerificationResponse>
      startOrganizationVerification_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$1.StartOrganizationVerificationRequest>
              $request) async {
    return startOrganizationVerification($call, await $request);
  }

  $async.Future<$1.StartOrganizationVerificationResponse>
      startOrganizationVerification($grpc.ServiceCall call,
          $1.StartOrganizationVerificationRequest request);

  $async.Future<$1.CheckOrganizationVerificationResponse>
      checkOrganizationVerification_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$1.CheckOrganizationVerificationRequest>
              $request) async {
    return checkOrganizationVerification($call, await $request);
  }

  $async.Future<$1.CheckOrganizationVerificationResponse>
      checkOrganizationVerification($grpc.ServiceCall call,
          $1.CheckOrganizationVerificationRequest request);

  $async.Future<$1.ApplyDowngradeProfilesResponse> applyDowngradeProfiles_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ApplyDowngradeProfilesRequest> $request) async {
    return applyDowngradeProfiles($call, await $request);
  }

  $async.Future<$1.ApplyDowngradeProfilesResponse> applyDowngradeProfiles(
      $grpc.ServiceCall call, $1.ApplyDowngradeProfilesRequest request);

  $async.Future<$1.CreateAvatarPresignedUploadResponse>
      createAvatarPresignedUpload_Pre($grpc.ServiceCall $call,
          $async.Future<$1.CreateAvatarPresignedUploadRequest> $request) async {
    return createAvatarPresignedUpload($call, await $request);
  }

  $async.Future<$1.CreateAvatarPresignedUploadResponse>
      createAvatarPresignedUpload($grpc.ServiceCall call,
          $1.CreateAvatarPresignedUploadRequest request);

  $async.Future<$1.BeginSearchProfileSnapshotResponse>
      beginSearchProfileSnapshot_Pre($grpc.ServiceCall $call,
          $async.Future<$1.BeginSearchProfileSnapshotRequest> $request) async {
    return beginSearchProfileSnapshot($call, await $request);
  }

  $async.Future<$1.BeginSearchProfileSnapshotResponse>
      beginSearchProfileSnapshot(
          $grpc.ServiceCall call, $1.BeginSearchProfileSnapshotRequest request);

  $async.Future<$1.ListSearchProfileSnapshotResponse>
      listSearchProfileSnapshot_Pre($grpc.ServiceCall $call,
          $async.Future<$1.ListSearchProfileSnapshotRequest> $request) async {
    return listSearchProfileSnapshot($call, await $request);
  }

  $async.Future<$1.ListSearchProfileSnapshotResponse> listSearchProfileSnapshot(
      $grpc.ServiceCall call, $1.ListSearchProfileSnapshotRequest request);

  $async.Future<$1.ListSearchProfileJournalResponse>
      listSearchProfileJournal_Pre($grpc.ServiceCall $call,
          $async.Future<$1.ListSearchProfileJournalRequest> $request) async {
    return listSearchProfileJournal($call, await $request);
  }

  $async.Future<$1.ListSearchProfileJournalResponse> listSearchProfileJournal(
      $grpc.ServiceCall call, $1.ListSearchProfileJournalRequest request);

  $async.Future<$1.GetSearchProfileCheckpointResponse>
      getSearchProfileCheckpoint_Pre($grpc.ServiceCall $call,
          $async.Future<$1.GetSearchProfileCheckpointRequest> $request) async {
    return getSearchProfileCheckpoint($call, await $request);
  }

  $async.Future<$1.GetSearchProfileCheckpointResponse>
      getSearchProfileCheckpoint(
          $grpc.ServiceCall call, $1.GetSearchProfileCheckpointRequest request);
}
