package voice.backend.auth.principal;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.Mockito.*;

import app.voice.auth.v1.*;
import io.grpc.Context;
import io.grpc.Status;
import io.grpc.testing.StreamRecorder;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.dao.DataAccessResourceFailureException;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.ownershipproof.OwnershipTransferProofService;
import voice.backend.auth.ownershipproof.ProofBinding;
import voice.backend.auth.ownershipproof.ProofDeniedException;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.OtpService;

/** Adapter contract; principal verification itself belongs to the interceptor tests. */
class AuthOwnershipProofAdapterTest {
  private final AuthService auth = mock(AuthService.class);
  private final OwnershipTransferProofService proofs = mock(OwnershipTransferProofService.class);
  private final UUID account = UUID.randomUUID();
  private final UUID profile = UUID.randomUUID();
  private final ProofBinding binding = new ProofBinding(account, profile, UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), 7);
  private final Instant now = Instant.parse("2026-09-10T10:00:00.123456789Z");
  private AuthGrpcService grpc;

  @BeforeEach void configure() {
    grpc = new AuthGrpcService(auth, mock(OtpService.class));
    grpc.configureOwnershipProofService(proofs);
  }

  private VerifiedPrincipal gateway() { return new VerifiedPrincipal("delegated_user", "gateway", account, profile, 7); }
  private VerifiedPrincipal space() { return new VerifiedPrincipal("service", "space", null, null, 0); }

  private IssueOwnershipTransferProofRequest issueRequest() {
    return IssueOwnershipTransferProofRequest.newBuilder().setSpaceId(binding.spaceId().toString())
        .setNewOwnerProfileId(binding.newOwnerProfileId().toString()).setOperationId(binding.operationId().toString())
        .setPassword("password").setTotpCode("123456").setBackupCode("BACKUP").build();
  }

  private ConsumeOwnershipTransferProofRequest consumeRequest() {
    return ConsumeOwnershipTransferProofRequest.newBuilder().setAccountId(account.toString()).setProfileId(profile.toString())
        .setSpaceId(binding.spaceId().toString()).setNewOwnerProfileId(binding.newOwnerProfileId().toString())
        .setOperationId(binding.operationId().toString()).setSessionEpoch(7).setProof("opaque-proof").build();
  }

  private StreamRecorder<IssueOwnershipTransferProofResponse> issue(VerifiedPrincipal principal, IssueOwnershipTransferProofRequest request) {
    var result = StreamRecorder.<IssueOwnershipTransferProofResponse>create();
    Context.current().withValue(VerifiedPrincipal.CONTEXT, principal).run(() -> grpc.issueOwnershipTransferProof(request, result));
    return result;
  }

  private StreamRecorder<ConsumeOwnershipTransferProofResponse> consume(VerifiedPrincipal principal, ConsumeOwnershipTransferProofRequest request) {
    var result = StreamRecorder.<ConsumeOwnershipTransferProofResponse>create();
    Context.current().withValue(VerifiedPrincipal.CONTEXT, principal).run(() -> grpc.consumeOwnershipTransferProof(request, result));
    return result;
  }

  private static void fails(StreamRecorder<?> recorder, Status.Code code) {
    assertThat(recorder.getValues()).isEmpty();
    assertThat(recorder.getError()).isNotNull();
    assertThat(Status.fromThrowable(recorder.getError()).getCode()).isEqualTo(code);
  }

  @Test void rememberedLegacyLoginTokenNeverSubstitutesForVerifiedPrincipal() {
    when(auth.login(any())).thenReturn(new voice.backend.auth.service.AuthSession(
        "legacy-token", "refresh", 300, account.toString(), profile.toString(), "regular"));
    var login = StreamRecorder.<LoginResponse>create();
    grpc.login(LoginRequest.newBuilder().setEmail("user@example.com").setPassword("password").build(), login);
    assertThat(login.getError()).isNull();
    assertThat(login.getValues()).hasSize(1);
    fails(issue(null, issueRequest()), Status.Code.UNAUTHENTICATED);
    fails(consume(null, consumeRequest()), Status.Code.UNAUTHENTICATED);
    verifyNoInteractions(proofs);
  }

  @Test void gatewayDelegatedIssueBindsVerifiedActorAndReturnsExactExpiry() {
    when(proofs.issue(binding, "password", "123456", "BACKUP"))
        .thenReturn(new OwnershipTransferProofService.IssuedProof("opaque-proof", now));
    var result = issue(gateway(), issueRequest());
    assertThat(result.getError()).isNull();
    assertThat(result.getValues()).hasSize(1);
    var response = result.getValues().getFirst();
    assertThat(response.getProof()).isEqualTo("opaque-proof");
    assertThat(response.getExpiresAt().getSeconds()).isEqualTo(now.getEpochSecond());
    assertThat(response.getExpiresAt().getNanos()).isEqualTo(now.getNano());
    verify(proofs).issue(binding, "password", "123456", "BACKUP");
    verifyNoInteractions(auth);
  }

  @Test void wrongIssueKindOrIssuerCannotReachDomain() {
    for (var principal : List.of(space(), new VerifiedPrincipal("service", "gateway", account, profile, 7),
        new VerifiedPrincipal("delegated_user", "space", account, profile, 7))) {
      fails(issue(principal, issueRequest()), Status.Code.PERMISSION_DENIED);
    }
    verifyNoInteractions(proofs);
  }

  @Test void wrongConsumeKindOrIssuerCannotReachDomain() {
    for (var principal : List.of(gateway(), new VerifiedPrincipal("service", "gateway", null, null, 0),
        new VerifiedPrincipal("delegated_user", "space", account, profile, 7))) {
      fails(consume(principal, consumeRequest()), Status.Code.PERMISSION_DENIED);
    }
    verifyNoInteractions(proofs);
  }

  @Test void malformedOrMissingIssueUuidIsInvalidArgument() {
    for (String bad : List.of("", "not-a-uuid", "1-1-1-1-1")) {
      fails(issue(gateway(), issueRequest().toBuilder().setSpaceId(bad).build()), Status.Code.INVALID_ARGUMENT);
      fails(issue(gateway(), issueRequest().toBuilder().setNewOwnerProfileId(bad).build()), Status.Code.INVALID_ARGUMENT);
      fails(issue(gateway(), issueRequest().toBuilder().setOperationId(bad).build()), Status.Code.INVALID_ARGUMENT);
    }
    verifyNoInteractions(proofs);
  }

  @Test void malformedOrMissingConsumeUuidAndNonpositiveEpochAreInvalidArgument() {
    for (String bad : List.of("", "not-a-uuid", "1-1-1-1-1")) {
      for (var request : List.of(consumeRequest().toBuilder().setAccountId(bad).build(),
          consumeRequest().toBuilder().setProfileId(bad).build(), consumeRequest().toBuilder().setSpaceId(bad).build(),
          consumeRequest().toBuilder().setNewOwnerProfileId(bad).build(), consumeRequest().toBuilder().setOperationId(bad).build())) {
        fails(consume(space(), request), Status.Code.INVALID_ARGUMENT);
      }
    }
    for (long bad : new long[]{0, -1}) {
      fails(consume(space(), consumeRequest().toBuilder().setSessionEpoch(bad).build()), Status.Code.INVALID_ARGUMENT);
    }
    verifyNoInteractions(proofs);
  }

  @Test void spaceConsumeForwardsEveryBindingAndReturnsCompleteDurableReceipt() {
    UUID receiptId = UUID.randomUUID();
    when(proofs.consume(binding, "opaque-proof")).thenReturn(new OwnershipTransferProofService.Receipt(
        receiptId, binding, now, List.of("password", "totp")));
    var result = consume(space(), consumeRequest());
    assertThat(result.getError()).isNull();
    assertThat(result.getValues()).hasSize(1);
    var response = result.getValues().getFirst();
    assertThat(response.getReceiptId()).isEqualTo(receiptId.toString());
    assertThat(response.getAccountId()).isEqualTo(account.toString());
    assertThat(response.getProfileId()).isEqualTo(profile.toString());
    assertThat(response.getSpaceId()).isEqualTo(binding.spaceId().toString());
    assertThat(response.getNewOwnerProfileId()).isEqualTo(binding.newOwnerProfileId().toString());
    assertThat(response.getOperationId()).isEqualTo(binding.operationId().toString());
    assertThat(response.getSessionEpoch()).isEqualTo(7);
    assertThat(response.getConsumedAt().getSeconds()).isEqualTo(now.getEpochSecond());
    assertThat(response.getConsumedAt().getNanos()).isEqualTo(now.getNano());
    assertThat(response.getVerifiedFactorsList()).containsExactly("password", "totp");
    verify(proofs).consume(binding, "opaque-proof");
    verifyNoInteractions(auth);
  }

  @Test void domainDenialAndStorageFailureMapToCoarsePublicStatuses() {
    when(proofs.issue(any(), anyString(), anyString(), anyString())).thenThrow(new ProofDeniedException());
    when(proofs.consume(any(), anyString())).thenThrow(new ProofDeniedException());
    fails(issue(gateway(), issueRequest()), Status.Code.PERMISSION_DENIED);
    fails(consume(space(), consumeRequest()), Status.Code.PERMISSION_DENIED);
    reset(proofs);
    when(proofs.issue(any(), anyString(), anyString(), anyString()))
        .thenThrow(new DataAccessResourceFailureException("private connection detail"));
    when(proofs.consume(any(), anyString())).thenThrow(new DataAccessResourceFailureException("private connection detail"));
    var issued = issue(gateway(), issueRequest());
    var consumed = consume(space(), consumeRequest());
    fails(issued, Status.Code.UNAVAILABLE);
    fails(consumed, Status.Code.UNAVAILABLE);
    assertThat(issued.getError().getMessage()).doesNotContain("private connection detail");
    assertThat(consumed.getError().getMessage()).doesNotContain("private connection detail");
  }

  @Test void missingProofServiceFailsClosedForBothRpcs() {
    grpc = new AuthGrpcService(auth, mock(OtpService.class));
    fails(issue(gateway(), issueRequest()), Status.Code.UNAVAILABLE);
    fails(consume(space(), consumeRequest()), Status.Code.UNAVAILABLE);
    verifyNoInteractions(proofs);
  }
}
