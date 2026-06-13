// This is a generated file - do not edit.
//
// Generated from voice/calls/v1/calls.proto.

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

import 'calls.pb.dart' as $0;

export 'calls.pb.dart';

/// Voice / LiveKit orchestration. HTTP: /api/v1/voice/**.
/// Package voice.calls.v1 avoids path stutter voice/voice/v1; service name matches docs.
@$pb.GrpcServiceName('voice.calls.v1.VoiceService')
class VoiceServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  VoiceServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.StartCallResponse> startCall(
    $0.StartCallRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$startCall, request, options: options);
  }

  $grpc.ResponseFuture<$0.AcceptCallResponse> acceptCall(
    $0.AcceptCallRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$acceptCall, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeclineCallResponse> declineCall(
    $0.DeclineCallRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$declineCall, request, options: options);
  }

  $grpc.ResponseFuture<$0.JoinCallResponse> joinCall(
    $0.JoinCallRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$joinCall, request, options: options);
  }

  $grpc.ResponseFuture<$0.LeaveCallResponse> leaveCall(
    $0.LeaveCallRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$leaveCall, request, options: options);
  }

  $grpc.ResponseFuture<$0.EndCallResponse> endCall(
    $0.EndCallRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$endCall, request, options: options);
  }

  $grpc.ResponseFuture<$0.JoinVoiceRoomResponse> joinVoiceRoom(
    $0.JoinVoiceRoomRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$joinVoiceRoom, request, options: options);
  }

  $grpc.ResponseFuture<$0.LeaveVoiceRoomResponse> leaveVoiceRoom(
    $0.LeaveVoiceRoomRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$leaveVoiceRoom, request, options: options);
  }

  $grpc.ResponseFuture<$0.MoveToVoiceRoomResponse> moveToVoiceRoom(
    $0.MoveToVoiceRoomRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$moveToVoiceRoom, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetJoinTokenResponse> getJoinToken(
    $0.GetJoinTokenRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getJoinToken, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdateVoiceStateResponse> updateVoiceState(
    $0.UpdateVoiceStateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateVoiceState, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetVoiceStatesResponse> getVoiceStates(
    $0.GetVoiceStatesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getVoiceStates, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetActiveCallResponse> getActiveCall(
    $0.GetActiveCallRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getActiveCall, request, options: options);
  }

  $grpc.ResponseFuture<$0.StartScreenShareResponse> startScreenShare(
    $0.StartScreenShareRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$startScreenShare, request, options: options);
  }

  $grpc.ResponseFuture<$0.StopScreenShareResponse> stopScreenShare(
    $0.StopScreenShareRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$stopScreenShare, request, options: options);
  }

  $grpc.ResponseFuture<$0.SetCommanderModeResponse> setCommanderMode(
    $0.SetCommanderModeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setCommanderMode, request, options: options);
  }

  $grpc.ResponseFuture<$0.RaiseHandResponse> raiseHand(
    $0.RaiseHandRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$raiseHand, request, options: options);
  }

  $grpc.ResponseFuture<$0.LowerHandResponse> lowerHand(
    $0.LowerHandRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$lowerHand, request, options: options);
  }

  // method descriptors

  static final _$startCall =
      $grpc.ClientMethod<$0.StartCallRequest, $0.StartCallResponse>(
          '/voice.calls.v1.VoiceService/StartCall',
          ($0.StartCallRequest value) => value.writeToBuffer(),
          $0.StartCallResponse.fromBuffer);
  static final _$acceptCall =
      $grpc.ClientMethod<$0.AcceptCallRequest, $0.AcceptCallResponse>(
          '/voice.calls.v1.VoiceService/AcceptCall',
          ($0.AcceptCallRequest value) => value.writeToBuffer(),
          $0.AcceptCallResponse.fromBuffer);
  static final _$declineCall =
      $grpc.ClientMethod<$0.DeclineCallRequest, $0.DeclineCallResponse>(
          '/voice.calls.v1.VoiceService/DeclineCall',
          ($0.DeclineCallRequest value) => value.writeToBuffer(),
          $0.DeclineCallResponse.fromBuffer);
  static final _$joinCall =
      $grpc.ClientMethod<$0.JoinCallRequest, $0.JoinCallResponse>(
          '/voice.calls.v1.VoiceService/JoinCall',
          ($0.JoinCallRequest value) => value.writeToBuffer(),
          $0.JoinCallResponse.fromBuffer);
  static final _$leaveCall =
      $grpc.ClientMethod<$0.LeaveCallRequest, $0.LeaveCallResponse>(
          '/voice.calls.v1.VoiceService/LeaveCall',
          ($0.LeaveCallRequest value) => value.writeToBuffer(),
          $0.LeaveCallResponse.fromBuffer);
  static final _$endCall =
      $grpc.ClientMethod<$0.EndCallRequest, $0.EndCallResponse>(
          '/voice.calls.v1.VoiceService/EndCall',
          ($0.EndCallRequest value) => value.writeToBuffer(),
          $0.EndCallResponse.fromBuffer);
  static final _$joinVoiceRoom =
      $grpc.ClientMethod<$0.JoinVoiceRoomRequest, $0.JoinVoiceRoomResponse>(
          '/voice.calls.v1.VoiceService/JoinVoiceRoom',
          ($0.JoinVoiceRoomRequest value) => value.writeToBuffer(),
          $0.JoinVoiceRoomResponse.fromBuffer);
  static final _$leaveVoiceRoom =
      $grpc.ClientMethod<$0.LeaveVoiceRoomRequest, $0.LeaveVoiceRoomResponse>(
          '/voice.calls.v1.VoiceService/LeaveVoiceRoom',
          ($0.LeaveVoiceRoomRequest value) => value.writeToBuffer(),
          $0.LeaveVoiceRoomResponse.fromBuffer);
  static final _$moveToVoiceRoom =
      $grpc.ClientMethod<$0.MoveToVoiceRoomRequest, $0.MoveToVoiceRoomResponse>(
          '/voice.calls.v1.VoiceService/MoveToVoiceRoom',
          ($0.MoveToVoiceRoomRequest value) => value.writeToBuffer(),
          $0.MoveToVoiceRoomResponse.fromBuffer);
  static final _$getJoinToken =
      $grpc.ClientMethod<$0.GetJoinTokenRequest, $0.GetJoinTokenResponse>(
          '/voice.calls.v1.VoiceService/GetJoinToken',
          ($0.GetJoinTokenRequest value) => value.writeToBuffer(),
          $0.GetJoinTokenResponse.fromBuffer);
  static final _$updateVoiceState = $grpc.ClientMethod<
          $0.UpdateVoiceStateRequest, $0.UpdateVoiceStateResponse>(
      '/voice.calls.v1.VoiceService/UpdateVoiceState',
      ($0.UpdateVoiceStateRequest value) => value.writeToBuffer(),
      $0.UpdateVoiceStateResponse.fromBuffer);
  static final _$getVoiceStates =
      $grpc.ClientMethod<$0.GetVoiceStatesRequest, $0.GetVoiceStatesResponse>(
          '/voice.calls.v1.VoiceService/GetVoiceStates',
          ($0.GetVoiceStatesRequest value) => value.writeToBuffer(),
          $0.GetVoiceStatesResponse.fromBuffer);
  static final _$getActiveCall =
      $grpc.ClientMethod<$0.GetActiveCallRequest, $0.GetActiveCallResponse>(
          '/voice.calls.v1.VoiceService/GetActiveCall',
          ($0.GetActiveCallRequest value) => value.writeToBuffer(),
          $0.GetActiveCallResponse.fromBuffer);
  static final _$startScreenShare = $grpc.ClientMethod<
          $0.StartScreenShareRequest, $0.StartScreenShareResponse>(
      '/voice.calls.v1.VoiceService/StartScreenShare',
      ($0.StartScreenShareRequest value) => value.writeToBuffer(),
      $0.StartScreenShareResponse.fromBuffer);
  static final _$stopScreenShare =
      $grpc.ClientMethod<$0.StopScreenShareRequest, $0.StopScreenShareResponse>(
          '/voice.calls.v1.VoiceService/StopScreenShare',
          ($0.StopScreenShareRequest value) => value.writeToBuffer(),
          $0.StopScreenShareResponse.fromBuffer);
  static final _$setCommanderMode = $grpc.ClientMethod<
          $0.SetCommanderModeRequest, $0.SetCommanderModeResponse>(
      '/voice.calls.v1.VoiceService/SetCommanderMode',
      ($0.SetCommanderModeRequest value) => value.writeToBuffer(),
      $0.SetCommanderModeResponse.fromBuffer);
  static final _$raiseHand =
      $grpc.ClientMethod<$0.RaiseHandRequest, $0.RaiseHandResponse>(
          '/voice.calls.v1.VoiceService/RaiseHand',
          ($0.RaiseHandRequest value) => value.writeToBuffer(),
          $0.RaiseHandResponse.fromBuffer);
  static final _$lowerHand =
      $grpc.ClientMethod<$0.LowerHandRequest, $0.LowerHandResponse>(
          '/voice.calls.v1.VoiceService/LowerHand',
          ($0.LowerHandRequest value) => value.writeToBuffer(),
          $0.LowerHandResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.calls.v1.VoiceService')
abstract class VoiceServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.calls.v1.VoiceService';

  VoiceServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.StartCallRequest, $0.StartCallResponse>(
        'StartCall',
        startCall_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.StartCallRequest.fromBuffer(value),
        ($0.StartCallResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AcceptCallRequest, $0.AcceptCallResponse>(
        'AcceptCall',
        acceptCall_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.AcceptCallRequest.fromBuffer(value),
        ($0.AcceptCallResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.DeclineCallRequest, $0.DeclineCallResponse>(
            'DeclineCall',
            declineCall_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.DeclineCallRequest.fromBuffer(value),
            ($0.DeclineCallResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.JoinCallRequest, $0.JoinCallResponse>(
        'JoinCall',
        joinCall_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.JoinCallRequest.fromBuffer(value),
        ($0.JoinCallResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.LeaveCallRequest, $0.LeaveCallResponse>(
        'LeaveCall',
        leaveCall_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.LeaveCallRequest.fromBuffer(value),
        ($0.LeaveCallResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.EndCallRequest, $0.EndCallResponse>(
        'EndCall',
        endCall_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.EndCallRequest.fromBuffer(value),
        ($0.EndCallResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.JoinVoiceRoomRequest, $0.JoinVoiceRoomResponse>(
            'JoinVoiceRoom',
            joinVoiceRoom_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.JoinVoiceRoomRequest.fromBuffer(value),
            ($0.JoinVoiceRoomResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.LeaveVoiceRoomRequest,
            $0.LeaveVoiceRoomResponse>(
        'LeaveVoiceRoom',
        leaveVoiceRoom_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.LeaveVoiceRoomRequest.fromBuffer(value),
        ($0.LeaveVoiceRoomResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.MoveToVoiceRoomRequest,
            $0.MoveToVoiceRoomResponse>(
        'MoveToVoiceRoom',
        moveToVoiceRoom_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.MoveToVoiceRoomRequest.fromBuffer(value),
        ($0.MoveToVoiceRoomResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetJoinTokenRequest, $0.GetJoinTokenResponse>(
            'GetJoinToken',
            getJoinToken_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetJoinTokenRequest.fromBuffer(value),
            ($0.GetJoinTokenResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpdateVoiceStateRequest,
            $0.UpdateVoiceStateResponse>(
        'UpdateVoiceState',
        updateVoiceState_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UpdateVoiceStateRequest.fromBuffer(value),
        ($0.UpdateVoiceStateResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetVoiceStatesRequest,
            $0.GetVoiceStatesResponse>(
        'GetVoiceStates',
        getVoiceStates_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetVoiceStatesRequest.fromBuffer(value),
        ($0.GetVoiceStatesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetActiveCallRequest, $0.GetActiveCallResponse>(
            'GetActiveCall',
            getActiveCall_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetActiveCallRequest.fromBuffer(value),
            ($0.GetActiveCallResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.StartScreenShareRequest,
            $0.StartScreenShareResponse>(
        'StartScreenShare',
        startScreenShare_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.StartScreenShareRequest.fromBuffer(value),
        ($0.StartScreenShareResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.StopScreenShareRequest,
            $0.StopScreenShareResponse>(
        'StopScreenShare',
        stopScreenShare_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.StopScreenShareRequest.fromBuffer(value),
        ($0.StopScreenShareResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SetCommanderModeRequest,
            $0.SetCommanderModeResponse>(
        'SetCommanderMode',
        setCommanderMode_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SetCommanderModeRequest.fromBuffer(value),
        ($0.SetCommanderModeResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RaiseHandRequest, $0.RaiseHandResponse>(
        'RaiseHand',
        raiseHand_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.RaiseHandRequest.fromBuffer(value),
        ($0.RaiseHandResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.LowerHandRequest, $0.LowerHandResponse>(
        'LowerHand',
        lowerHand_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.LowerHandRequest.fromBuffer(value),
        ($0.LowerHandResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.StartCallResponse> startCall_Pre($grpc.ServiceCall $call,
      $async.Future<$0.StartCallRequest> $request) async {
    return startCall($call, await $request);
  }

  $async.Future<$0.StartCallResponse> startCall(
      $grpc.ServiceCall call, $0.StartCallRequest request);

  $async.Future<$0.AcceptCallResponse> acceptCall_Pre($grpc.ServiceCall $call,
      $async.Future<$0.AcceptCallRequest> $request) async {
    return acceptCall($call, await $request);
  }

  $async.Future<$0.AcceptCallResponse> acceptCall(
      $grpc.ServiceCall call, $0.AcceptCallRequest request);

  $async.Future<$0.DeclineCallResponse> declineCall_Pre($grpc.ServiceCall $call,
      $async.Future<$0.DeclineCallRequest> $request) async {
    return declineCall($call, await $request);
  }

  $async.Future<$0.DeclineCallResponse> declineCall(
      $grpc.ServiceCall call, $0.DeclineCallRequest request);

  $async.Future<$0.JoinCallResponse> joinCall_Pre($grpc.ServiceCall $call,
      $async.Future<$0.JoinCallRequest> $request) async {
    return joinCall($call, await $request);
  }

  $async.Future<$0.JoinCallResponse> joinCall(
      $grpc.ServiceCall call, $0.JoinCallRequest request);

  $async.Future<$0.LeaveCallResponse> leaveCall_Pre($grpc.ServiceCall $call,
      $async.Future<$0.LeaveCallRequest> $request) async {
    return leaveCall($call, await $request);
  }

  $async.Future<$0.LeaveCallResponse> leaveCall(
      $grpc.ServiceCall call, $0.LeaveCallRequest request);

  $async.Future<$0.EndCallResponse> endCall_Pre($grpc.ServiceCall $call,
      $async.Future<$0.EndCallRequest> $request) async {
    return endCall($call, await $request);
  }

  $async.Future<$0.EndCallResponse> endCall(
      $grpc.ServiceCall call, $0.EndCallRequest request);

  $async.Future<$0.JoinVoiceRoomResponse> joinVoiceRoom_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.JoinVoiceRoomRequest> $request) async {
    return joinVoiceRoom($call, await $request);
  }

  $async.Future<$0.JoinVoiceRoomResponse> joinVoiceRoom(
      $grpc.ServiceCall call, $0.JoinVoiceRoomRequest request);

  $async.Future<$0.LeaveVoiceRoomResponse> leaveVoiceRoom_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.LeaveVoiceRoomRequest> $request) async {
    return leaveVoiceRoom($call, await $request);
  }

  $async.Future<$0.LeaveVoiceRoomResponse> leaveVoiceRoom(
      $grpc.ServiceCall call, $0.LeaveVoiceRoomRequest request);

  $async.Future<$0.MoveToVoiceRoomResponse> moveToVoiceRoom_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.MoveToVoiceRoomRequest> $request) async {
    return moveToVoiceRoom($call, await $request);
  }

  $async.Future<$0.MoveToVoiceRoomResponse> moveToVoiceRoom(
      $grpc.ServiceCall call, $0.MoveToVoiceRoomRequest request);

  $async.Future<$0.GetJoinTokenResponse> getJoinToken_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetJoinTokenRequest> $request) async {
    return getJoinToken($call, await $request);
  }

  $async.Future<$0.GetJoinTokenResponse> getJoinToken(
      $grpc.ServiceCall call, $0.GetJoinTokenRequest request);

  $async.Future<$0.UpdateVoiceStateResponse> updateVoiceState_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UpdateVoiceStateRequest> $request) async {
    return updateVoiceState($call, await $request);
  }

  $async.Future<$0.UpdateVoiceStateResponse> updateVoiceState(
      $grpc.ServiceCall call, $0.UpdateVoiceStateRequest request);

  $async.Future<$0.GetVoiceStatesResponse> getVoiceStates_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetVoiceStatesRequest> $request) async {
    return getVoiceStates($call, await $request);
  }

  $async.Future<$0.GetVoiceStatesResponse> getVoiceStates(
      $grpc.ServiceCall call, $0.GetVoiceStatesRequest request);

  $async.Future<$0.GetActiveCallResponse> getActiveCall_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetActiveCallRequest> $request) async {
    return getActiveCall($call, await $request);
  }

  $async.Future<$0.GetActiveCallResponse> getActiveCall(
      $grpc.ServiceCall call, $0.GetActiveCallRequest request);

  $async.Future<$0.StartScreenShareResponse> startScreenShare_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.StartScreenShareRequest> $request) async {
    return startScreenShare($call, await $request);
  }

  $async.Future<$0.StartScreenShareResponse> startScreenShare(
      $grpc.ServiceCall call, $0.StartScreenShareRequest request);

  $async.Future<$0.StopScreenShareResponse> stopScreenShare_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.StopScreenShareRequest> $request) async {
    return stopScreenShare($call, await $request);
  }

  $async.Future<$0.StopScreenShareResponse> stopScreenShare(
      $grpc.ServiceCall call, $0.StopScreenShareRequest request);

  $async.Future<$0.SetCommanderModeResponse> setCommanderMode_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetCommanderModeRequest> $request) async {
    return setCommanderMode($call, await $request);
  }

  $async.Future<$0.SetCommanderModeResponse> setCommanderMode(
      $grpc.ServiceCall call, $0.SetCommanderModeRequest request);

  $async.Future<$0.RaiseHandResponse> raiseHand_Pre($grpc.ServiceCall $call,
      $async.Future<$0.RaiseHandRequest> $request) async {
    return raiseHand($call, await $request);
  }

  $async.Future<$0.RaiseHandResponse> raiseHand(
      $grpc.ServiceCall call, $0.RaiseHandRequest request);

  $async.Future<$0.LowerHandResponse> lowerHand_Pre($grpc.ServiceCall $call,
      $async.Future<$0.LowerHandRequest> $request) async {
    return lowerHand($call, await $request);
  }

  $async.Future<$0.LowerHandResponse> lowerHand(
      $grpc.ServiceCall call, $0.LowerHandRequest request);
}
