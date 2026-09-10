package voice.backend.auth.principal;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.Mockito.*;

import app.voice.auth.v1.*;
import io.grpc.*;
import io.grpc.testing.StreamRecorder;
import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.stub.MetadataUtils;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.dao.DataAccessResourceFailureException;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.ownershipproof.*;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.OtpService;

class AuthOwnershipReceiptAdapterTest {
  final OwnershipTransferProofService proofs = mock(OwnershipTransferProofService.class);
  final ProofBinding binding = new ProofBinding(UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), 7);
  final String digest = "0123456789abcdef".repeat(4);
  final OwnershipTransferProofService.Receipt receipt = new OwnershipTransferProofService.Receipt(
      UUID.randomUUID(), binding, Instant.parse("2026-09-10T09:00:00.123456Z"), List.of("password", "backup_code"));
  AuthGrpcService grpc;

  @BeforeEach void configure() {
    grpc = new AuthGrpcService(mock(AuthService.class), mock(OtpService.class));
    grpc.configureOwnershipProofService(proofs);
    when(proofs.lookup(binding, digest)).thenReturn(receipt);
  }

  GetOwnershipTransferReceiptRequest request() {
    return GetOwnershipTransferReceiptRequest.newBuilder().setAccountId(binding.accountId().toString())
        .setProfileId(binding.profileId().toString()).setSpaceId(binding.spaceId().toString())
        .setNewOwnerProfileId(binding.newOwnerProfileId().toString()).setOperationId(binding.operationId().toString())
        .setSessionEpoch(binding.sessionEpoch()).setProofDigest(digest).build();
  }
  VerifiedPrincipal space() { return new VerifiedPrincipal("service", "space", null, null, 0); }
  StreamRecorder<GetOwnershipTransferReceiptResponse> lookup(VerifiedPrincipal principal, GetOwnershipTransferReceiptRequest request) {
    var recorder = StreamRecorder.<GetOwnershipTransferReceiptResponse>create();
    Context.current().withValue(VerifiedPrincipal.CONTEXT, principal).run(() -> grpc.getOwnershipTransferReceipt(request, recorder));
    return recorder;
  }
  void denied(StreamRecorder<?> result, Status.Code code) {
    assertThat(result.getValues()).isEmpty();
    assertThat(result.getError()).isNotNull();
    assertThat(Status.fromThrowable(result.getError()).getCode()).isEqualTo(code);
  }
  void exact(GetOwnershipTransferReceiptResponse response) {
    assertThat(response.getReceiptId()).isEqualTo(receipt.receiptId().toString());
    assertThat(response.getAccountId()).isEqualTo(binding.accountId().toString());
    assertThat(response.getProfileId()).isEqualTo(binding.profileId().toString());
    assertThat(response.getSpaceId()).isEqualTo(binding.spaceId().toString());
    assertThat(response.getNewOwnerProfileId()).isEqualTo(binding.newOwnerProfileId().toString());
    assertThat(response.getOperationId()).isEqualTo(binding.operationId().toString());
    assertThat(response.getSessionEpoch()).isEqualTo(7);
    assertThat(response.getConsumedAt().getSeconds()).isEqualTo(receipt.consumedAt().getEpochSecond());
    assertThat(response.getConsumedAt().getNanos()).isEqualTo(123456000);
    assertThat(response.getVerifiedFactorsList()).containsExactly("password", "backup_code");
    assertThat(response.getDescriptorForType().getFields()).extracting(com.google.protobuf.Descriptors.FieldDescriptor::getName)
        .containsExactly("receipt_id", "account_id", "profile_id", "space_id", "new_owner_profile_id", "operation_id", "session_epoch", "consumed_at", "verified_factors");
    assertThat(response.toString()).doesNotContain(digest);
  }

  @Test void spaceRecoversExactHistoricalReceiptUsingSevenOriginalFieldsOnly() {
    var result = lookup(space(), request());
    assertThat(result.getError()).isNull();
    assertThat(result.getValues()).hasSize(1);
    exact(result.getValues().getFirst());
    verify(proofs).lookup(binding, digest);
    verifyNoMoreInteractions(proofs);
    assertThat(request().getDescriptorForType().getFields()).extracting(com.google.protobuf.Descriptors.FieldDescriptor::getName)
        .containsExactly("account_id", "profile_id", "space_id", "new_owner_profile_id", "operation_id", "session_epoch", "proof_digest");
  }

  @Test void absentContextAndVerifiedWrongCallersNeverReachDomain() {
    denied(lookup(null, request()), Status.Code.UNAUTHENTICATED);
    for (var principal : List.of(new VerifiedPrincipal("delegated_user", "gateway", binding.accountId(), binding.profileId(), 7),
        new VerifiedPrincipal("service", "gateway", null, null, 0), new VerifiedPrincipal("service", "role", null, null, 0),
        new VerifiedPrincipal("delegated_user", "space", binding.accountId(), binding.profileId(), 7))) {
      denied(lookup(principal, request()), Status.Code.PERMISSION_DENIED);
    }
    verifyNoInteractions(proofs);
  }

  @Test void everyMalformedBindingAndDigestIsInvalidArgumentBeforeDomain() {
    for (String bad : List.of("", "not-a-uuid", "1-1-1-1-1")) {
      for (var changed : List.of(request().toBuilder().setAccountId(bad), request().toBuilder().setProfileId(bad),
          request().toBuilder().setSpaceId(bad), request().toBuilder().setNewOwnerProfileId(bad), request().toBuilder().setOperationId(bad))) {
        denied(lookup(space(), changed.build()), Status.Code.INVALID_ARGUMENT);
      }
    }
    for (long epoch : new long[]{0, -1}) denied(lookup(space(), request().toBuilder().setSessionEpoch(epoch).build()), Status.Code.INVALID_ARGUMENT);
    for (String bad : List.of("", "a".repeat(63), "a".repeat(65), digest.toUpperCase(Locale.ROOT), "g".repeat(64), " " + digest, digest + " ", "sha256:" + digest)) {
      denied(lookup(space(), request().toBuilder().setProofDigest(bad).build()), Status.Code.INVALID_ARGUMENT);
    }
    verifyNoInteractions(proofs);
  }

  @Test void missingUnconsumedAndMismatchShareCoarseDenialAndStorageFailureIsUnavailable() {
    when(proofs.lookup(any(), anyString())).thenThrow(new ProofDeniedException());
    var results = List.of(lookup(space(), request()),
        lookup(space(), request().toBuilder().setOperationId(UUID.randomUUID().toString()).build()),
        lookup(space(), request().toBuilder().setProofDigest("f".repeat(64)).build()));
    for (var result : results) denied(result, Status.Code.PERMISSION_DENIED);
    assertThat(results).extracting(result -> Status.fromThrowable(result.getError()).getDescription()).containsOnly(
        Status.fromThrowable(results.getFirst().getError()).getDescription());
    doThrow(new DataAccessResourceFailureException("private-storage-" + digest)).when(proofs).lookup(any(), anyString());
    var failure = lookup(space(), request());
    denied(failure, Status.Code.UNAVAILABLE);
    assertThat(failure.getError().getMessage()).doesNotContain(digest, "private-storage");
  }

  @Test void unconfiguredServiceIsUnavailable() {
    grpc = new AuthGrpcService(mock(AuthService.class), mock(OtpService.class));
    denied(lookup(space(), request()), Status.Code.UNAVAILABLE);
    verifyNoInteractions(proofs);
  }

  @Test void realPrivateTlsLookupAcceptsFreshSpaceCredentialsAndRejectsRpcHashAudienceAndReplay() throws Exception {
    var fixture = new AuthPrincipalVerifierTest();
    Server server = NettyServerBuilder.forPort(0)
        .useTransportSecurity(AuthPrincipalServicesTest.resource("server-cert.pem"), AuthPrincipalServicesTest.resource("server-key.pem"))
        .addService(AuthPrincipalServices.proofService(grpc.bindService(), new AuthPrincipalServerInterceptor(fixture.verifier))).build().start();
    ManagedChannel channel = NettyChannelBuilder.forAddress("localhost", server.getPort())
        .sslContext(GrpcSslContexts.forClient().trustManager(AuthPrincipalServicesTest.resource("server-cert.pem")).build()).build();
    try {
      var claims = signedClaims();
      exact(networkLookup(channel, claims, request()));
      var replay = org.junit.jupiter.api.Assertions.assertThrows(StatusRuntimeException.class, () -> networkLookup(channel, claims, request()));
      assertThat(replay.getStatus().getCode()).isEqualTo(Status.Code.UNAUTHENTICATED);
      exact(networkLookup(channel, signedClaims(), request()));
      for (String field : List.of("rpc", "aud", "request_hash")) {
        var invalid = signedClaims(); invalid.put(field, "wrong");
        var failure = org.junit.jupiter.api.Assertions.assertThrows(StatusRuntimeException.class, () -> networkLookup(channel, invalid, request()));
        assertThat(failure.getStatus().getCode()).isEqualTo(Status.Code.UNAUTHENTICATED);
      }
      var wrongBody = org.junit.jupiter.api.Assertions.assertThrows(StatusRuntimeException.class, () -> networkLookup(channel,
          signedClaims(), request().toBuilder().setProofDigest("f".repeat(64)).build()));
      assertThat(wrongBody.getStatus().getCode()).isEqualTo(Status.Code.UNAUTHENTICATED);
      verify(proofs, times(2)).lookup(binding, digest);
      verifyNoMoreInteractions(proofs);
    } finally { AuthPrincipalServicesTest.stop(channel, server); }
  }

  Map<String,Object> signedClaims() {
    var claims = AuthPrincipalVerifierTest.claims();
    claims.put("iss", "space"); claims.put("sub", "service:space"); claims.put("principal_type", "service");
    claims.remove("account_id"); claims.remove("profile_id"); claims.remove("session_epoch");
    claims.put("rpc", AuthPrincipalServerInterceptor.LOOKUP_RPC);
    claims.put("request_hash", AuthPrincipalServerInterceptor.requestHash(request()));
    return claims;
  }
  GetOwnershipTransferReceiptResponse networkLookup(ManagedChannel channel, Map<String,Object> claims, GetOwnershipTransferReceiptRequest request) {
    Metadata headers = new Metadata();
    headers.put(AuthPrincipalServerInterceptorTest.AUTH, "Bearer " + AuthPrincipalVerifierTest.token(claims));
    headers.put(AuthPrincipalServerInterceptorTest.REQUEST_ID, "request-1");
    return AuthServiceGrpc.newBlockingStub(channel).withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers))
        .withDeadlineAfter(5, TimeUnit.SECONDS).getOwnershipTransferReceipt(request);
  }
}
