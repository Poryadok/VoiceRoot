package voice.backend.auth.principal;

import static org.assertj.core.api.Assertions.assertThat;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.reset;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

import app.voice.auth.v1.AcknowledgeSpaceDeletionProofReceiptRequest;
import app.voice.auth.v1.AcknowledgeSpaceDeletionProofReceiptResponse;
import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.ConsumeSpaceDeletionProofRequest;
import app.voice.auth.v1.ConsumeSpaceDeletionProofResponse;
import app.voice.auth.v1.GetSpaceDeletionProofReceiptRequest;
import app.voice.auth.v1.GetSpaceDeletionProofReceiptResponse;
import app.voice.auth.v1.IssueSpaceDeletionProofRequest;
import app.voice.auth.v1.LoginRequest;
import app.voice.auth.v1.SpaceDeletionProofReceipt;
import app.voice.auth.v1.VerifiedFactor;
import com.google.protobuf.ByteString;
import com.google.protobuf.CodedOutputStream;
import com.google.protobuf.Message;
import com.google.protobuf.UnknownFieldSet;
import io.grpc.Context;
import io.grpc.ManagedChannel;
import io.grpc.Metadata;
import io.grpc.Server;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.stub.MetadataUtils;
import io.grpc.testing.StreamRecorder;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.dao.DataAccessResourceFailureException;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.grpc.AuthorizationServerInterceptor;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AuthSession;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.OtpService;
import voice.backend.auth.spacedeletionproof.DeletionProofAcknowledgement;
import voice.backend.auth.spacedeletionproof.DeletionProofActor;
import voice.backend.auth.spacedeletionproof.DeletionProofBinding;
import voice.backend.auth.spacedeletionproof.DeletionProofDeniedException;
import voice.backend.auth.spacedeletionproof.DeletionProofLookup;
import voice.backend.auth.spacedeletionproof.SpaceDeletionProofService;

/** Wire behavior and listener authority for the four R23 Auth deletion-proof RPCs. */
class AuthSpaceDeletionProofAdapterTest {
  private final UUID accountId = UUID.randomUUID();
  private final UUID profileId = UUID.randomUUID();
  private final UUID spaceId = UUID.randomUUID();
  private final UUID operationId = UUID.randomUUID();
  private final UUID receiptId = UUID.randomUUID();
  private final byte[] nameDigest = digest("Exact Space");
  private final byte[] proofDigest = digest("opaque-proof");
  private final byte[] bindingDigest = digest("binding");
  private final Instant now = Instant.parse("2026-09-12T08:00:00.123456Z");
  private final SpaceDeletionProofService proofs = mock(SpaceDeletionProofService.class);
  private final AuthService auth = mock(AuthService.class);
  private AuthGrpcService grpc;
  private DeletionProofBinding binding;
  private SpaceDeletionProofReceipt receipt;
  private Server ordinaryServer;
  private ManagedChannel ordinaryChannel;

  @BeforeEach
  void configure() throws Exception {
    binding = new DeletionProofBinding(accountId, profileId, 7, spaceId, operationId);
    receipt = SpaceDeletionProofReceipt.newBuilder()
        .setProtocolVersion(1).setReceiptId(receiptId.toString())
        .setOperationId(operationId.toString()).setBindingSha256(ByteString.copyFrom(bindingDigest))
        .setConfirmationNameSha256(ByteString.copyFrom(nameDigest)).setConsumedAt(timestamp(now))
        .addVerifiedFactors(VerifiedFactor.VERIFIED_FACTOR_PASSWORD)
        .addVerifiedFactors(VerifiedFactor.VERIFIED_FACTOR_TOTP).build();
    grpc = new AuthGrpcService(auth, mock(OtpService.class));
    grpc.configureSpaceDeletionProofService(proofs);
    when(auth.spaceDeletionProofActor("Bearer user-access"))
        .thenReturn(new DeletionProofActor(accountId, profileId, 7));
    when(proofs.issue(binding, "Exact Space", "password", "123456", ""))
        .thenReturn(new SpaceDeletionProofService.IssuedProof("opaque-proof", now.plusSeconds(300)));
    when(proofs.consume(binding, "Exact Space", "opaque-proof")).thenReturn(receipt);
    when(proofs.lookup(any(DeletionProofLookup.class))).thenReturn(receipt);
    when(proofs.acknowledge(any(DeletionProofAcknowledgement.class))).thenReturn(now.plusSeconds(10));

    String name = InProcessServerBuilder.generateName();
    ordinaryServer = InProcessServerBuilder.forName(name).directExecutor()
        .intercept(new AuthorizationServerInterceptor())
        .addService(AuthPrincipalServices.legacyService(grpc.bindService())).build().start();
    ordinaryChannel = InProcessChannelBuilder.forName(name).directExecutor().build();
  }

  @AfterEach
  void stopOrdinaryListener() throws Exception {
    ordinaryChannel.shutdownNow();
    ordinaryServer.shutdownNow();
    ordinaryChannel.awaitTermination(5, TimeUnit.SECONDS);
    ordinaryServer.awaitTermination(5, TimeUnit.SECONDS);
  }

  @Test
  void issueIsAnOrdinaryPublicRpcUsingTheUserAccessCredential() {
    var response = ordinary().issueSpaceDeletionProof(issueRequest());
    assertThat(response.getProof()).isEqualTo("opaque-proof");
    assertThat(response.getExpiresAt()).isEqualTo(timestamp(now.plusSeconds(300)));
    verify(auth).spaceDeletionProofActor("Bearer user-access");
    verify(proofs).issue(binding, "Exact Space", "password", "123456", "");
  }

  @Test
  void issueWithoutCurrentAuthorizationNeverReusesAnotherActorsRememberedToken() {
    UUID priorAccountId = UUID.randomUUID();
    UUID priorProfileId = UUID.randomUUID();
    when(auth.login(any())).thenReturn(new AuthSession(
        "prior-actor-access", "refresh", 900,
        priorAccountId.toString(), priorProfileId.toString(), "regular"));
    when(auth.spaceDeletionProofActor("Bearer prior-actor-access"))
        .thenReturn(new DeletionProofActor(priorAccountId, priorProfileId, 11));

    var login = ordinary(null).login(LoginRequest.newBuilder()
        .setEmail("prior@example.com").setPassword("password").build());
    assertThat(login.getSession().getAccessToken()).isEqualTo("prior-actor-access");

    assertStatus(
        () -> ordinary(null).issueSpaceDeletionProof(issueRequest()),
        Status.Code.UNAUTHENTICATED);
    verify(auth, never()).spaceDeletionProofActor("Bearer prior-actor-access");
    verifyNoInteractions(proofs);
  }

  @Test
  void ordinaryListenerOmitsProtectedMethodsAndPrivateListenerOmitsIssue() throws Exception {
    assertUnimplemented(() -> ordinary().consumeSpaceDeletionProof(consumeRequest()));
    assertUnimplemented(() -> ordinary().getSpaceDeletionProofReceipt(lookupRequest()));
    assertUnimplemented(() -> ordinary().acknowledgeSpaceDeletionProofReceipt(
        acknowledgeRequest(domainHash(receipt))));
    var fixture = new AuthPrincipalVerifierTest();
    Server server = tlsServer(fixture);
    ManagedChannel channel = privateChannel(server);
    try {
      assertUnimplemented(() -> AuthServiceGrpc.newBlockingStub(channel)
          .withDeadlineAfter(5, TimeUnit.SECONDS).issueSpaceDeletionProof(issueRequest()));
    } finally {
      AuthPrincipalServicesTest.stop(channel, server);
    }
    verifyNoInteractions(proofs);
  }

  @Test
  void consumeLookupAndAcknowledgeAcceptValidPrivateSpaceCredentials() throws Exception {
    var fixture = new AuthPrincipalVerifierTest();
    Server server = tlsServer(fixture);
    ManagedChannel channel = privateChannel(server);
    try {
      var consume = consumeRequest();
      assertThat(network(channel, signedSpace(AuthPrincipalServerInterceptor.SPACE_DELETE_CONSUME_RPC, consume))
          .consumeSpaceDeletionProof(consume).getReceipt()).isEqualTo(receipt);
      var lookup = lookupRequest();
      assertThat(network(channel, signedSpace(AuthPrincipalServerInterceptor.SPACE_DELETE_LOOKUP_RPC, lookup))
          .getSpaceDeletionProofReceipt(lookup).getReceipt()).isEqualTo(receipt);
      var acknowledge = acknowledgeRequest(domainHash(receipt));
      assertThat(network(channel, signedSpace(AuthPrincipalServerInterceptor.SPACE_DELETE_ACK_RPC, acknowledge))
          .acknowledgeSpaceDeletionProofReceipt(acknowledge).getAcknowledgedAt())
          .isEqualTo(timestamp(now.plusSeconds(10)));
    } finally {
      AuthPrincipalServicesTest.stop(channel, server);
    }
  }

  @Test
  void protectedAdaptersReturnOnlyCanonicalReceiptFields() {
    var consumed = consume(space(), consumeRequest());
    assertThat(consumed.getError()).isNull();
    assertThat(consumed.getValues()).singleElement().satisfies(response -> {
      assertThat(response.getReceipt()).isEqualTo(receipt);
      assertThat(response.getDescriptorForType().getFields())
          .extracting(com.google.protobuf.Descriptors.FieldDescriptor::getName).containsExactly("receipt");
      assertThat(response.toString()).doesNotContain("opaque-proof", "Exact Space");
    });
    var lookedUp = lookup(space(), lookupRequest());
    assertThat(lookedUp.getError()).isNull();
    assertThat(lookedUp.getValues()).singleElement()
        .extracting(GetSpaceDeletionProofReceiptResponse::getReceipt).isEqualTo(receipt);
    verify(proofs).lookup(new DeletionProofLookup(
        accountId, profileId, 7, spaceId, operationId, nameDigest, proofDigest));
    byte[] receiptDigest = domainHash(receipt);
    var acknowledged = acknowledge(space(), acknowledgeRequest(receiptDigest));
    assertThat(acknowledged.getError()).isNull();
    assertThat(acknowledged.getValues()).singleElement().satisfies(response ->
        assertThat(response.getAcknowledgedAt()).isEqualTo(timestamp(now.plusSeconds(10))));
    verify(proofs).acknowledge(new DeletionProofAcknowledgement(
        receiptId, spaceId, operationId, receiptDigest));
  }

  @Test
  void authorityRequestsRejectUnknownFieldsBeforeTheDomain() {
    UnknownFieldSet unknown = UnknownFieldSet.newBuilder()
        .addField(999, UnknownFieldSet.Field.newBuilder().addVarint(1).build()).build();
    assertStatus(() -> ordinary().issueSpaceDeletionProof(
        issueRequest().toBuilder().setUnknownFields(unknown).build()), Status.Code.INVALID_ARGUMENT);
    denied(consume(space(), consumeRequest().toBuilder().setUnknownFields(unknown).build()),
        Status.Code.INVALID_ARGUMENT);
    denied(lookup(space(), lookupRequest().toBuilder().setUnknownFields(unknown).build()),
        Status.Code.INVALID_ARGUMENT);
    denied(acknowledge(space(), acknowledgeRequest(domainHash(receipt)).toBuilder()
        .setUnknownFields(unknown).build()), Status.Code.INVALID_ARGUMENT);
    verifyNoInteractions(proofs);
  }

  @Test
  void missingUserCredentialAndWrongProtectedCallersNeverReachDomain() {
    when(auth.spaceDeletionProofActor(any())).thenThrow(new AuthException("invalid_token"));
    assertStatus(() -> ordinary(null).issueSpaceDeletionProof(issueRequest()), Status.Code.UNAUTHENTICATED);
    denied(consume(null, consumeRequest()), Status.Code.UNAUTHENTICATED);
    denied(lookup(null, lookupRequest()), Status.Code.UNAUTHENTICATED);
    denied(acknowledge(null, acknowledgeRequest(domainHash(receipt))), Status.Code.UNAUTHENTICATED);
    for (VerifiedPrincipal wrong : List.of(
        new VerifiedPrincipal("delegated_user", "gateway", accountId, profileId, 7),
        new VerifiedPrincipal("service", "gateway", null, null, 0),
        new VerifiedPrincipal("service", "role", null, null, 0),
        new VerifiedPrincipal("delegated_user", "space", accountId, profileId, 7))) {
      denied(consume(wrong, consumeRequest()), Status.Code.PERMISSION_DENIED);
      denied(lookup(wrong, lookupRequest()), Status.Code.PERMISSION_DENIED);
      denied(acknowledge(wrong, acknowledgeRequest(domainHash(receipt))), Status.Code.PERMISSION_DENIED);
    }
    verifyNoInteractions(proofs);
  }

  @Test
  void malformedCanonicalIdsEpochVersionAndDigestsAreInvalidBeforeDomain() {
    for (String bad : List.of("", "not-a-uuid", operationId.toString().toUpperCase())) {
      assertStatus(() -> ordinary().issueSpaceDeletionProof(
          issueRequest().toBuilder().setSpaceId(bad).build()), Status.Code.INVALID_ARGUMENT);
      assertStatus(() -> ordinary().issueSpaceDeletionProof(
          issueRequest().toBuilder().setOperationId(bad).build()), Status.Code.INVALID_ARGUMENT);
      denied(consume(space(), consumeRequest().toBuilder().setAccountId(bad).build()), Status.Code.INVALID_ARGUMENT);
      denied(consume(space(), consumeRequest().toBuilder().setProfileId(bad).build()), Status.Code.INVALID_ARGUMENT);
      denied(consume(space(), consumeRequest().toBuilder().setSpaceId(bad).build()), Status.Code.INVALID_ARGUMENT);
      denied(consume(space(), consumeRequest().toBuilder().setOperationId(bad).build()), Status.Code.INVALID_ARGUMENT);
    }
    for (long epoch : new long[] {0, -1}) {
      denied(consume(space(), consumeRequest().toBuilder().setSessionEpoch(epoch).build()), Status.Code.INVALID_ARGUMENT);
      denied(lookup(space(), lookupRequest().toBuilder().setSessionEpoch(epoch).build()), Status.Code.INVALID_ARGUMENT);
    }
    denied(consume(space(), consumeRequest().toBuilder().setProtocolVersion(0).build()), Status.Code.INVALID_ARGUMENT);
    denied(lookup(space(), lookupRequest().toBuilder().setProtocolVersion(2).build()), Status.Code.INVALID_ARGUMENT);
    denied(lookup(space(), lookupRequest().toBuilder()
        .setProofDigestSha256(ByteString.copyFrom(new byte[31])).build()), Status.Code.INVALID_ARGUMENT);
    denied(lookup(space(), lookupRequest().toBuilder()
        .setConfirmationNameSha256(ByteString.copyFrom(new byte[33])).build()), Status.Code.INVALID_ARGUMENT);
    denied(acknowledge(space(), acknowledgeRequest(new byte[31])), Status.Code.INVALID_ARGUMENT);
    verifyNoInteractions(proofs);
  }

  @Test
  void domainDenialIsCoarseAndStorageDetailsAreRedacted() {
    when(proofs.issue(any(), anyString(), anyString(), anyString(), anyString()))
        .thenThrow(new DeletionProofDeniedException());
    when(proofs.consume(any(), anyString(), anyString())).thenThrow(new DeletionProofDeniedException());
    when(proofs.lookup(any())).thenThrow(new DeletionProofDeniedException());
    when(proofs.acknowledge(any())).thenThrow(new DeletionProofDeniedException());
    assertStatus(() -> ordinary().issueSpaceDeletionProof(issueRequest()), Status.Code.PERMISSION_DENIED);
    List<StreamRecorder<?>> deniedResults = List.of(
        consume(space(), consumeRequest()), lookup(space(), lookupRequest()),
        acknowledge(space(), acknowledgeRequest(domainHash(receipt))));
    deniedResults.forEach(result -> denied(result, Status.Code.PERMISSION_DENIED));
    assertThat(deniedResults)
        .extracting(result -> Status.fromThrowable(result.getError()).getDescription())
        .containsOnly("space deletion proof denied");
    reset(proofs);
    when(proofs.lookup(any())).thenThrow(new DataAccessResourceFailureException(
        "private-proof-row opaque-proof"));
    var unavailable = lookup(space(), lookupRequest());
    denied(unavailable, Status.Code.UNAVAILABLE);
    assertThat(unavailable.getError().getMessage()).doesNotContain("private-proof-row", "opaque-proof");
  }

  @Test
  void privateCredentialRejectsWrongHashAudienceRpcAndReplay() throws Exception {
    var fixture = new AuthPrincipalVerifierTest();
    Server server = tlsServer(fixture);
    ManagedChannel channel = privateChannel(server);
    try {
      var claims = signedSpace(AuthPrincipalServerInterceptor.SPACE_DELETE_LOOKUP_RPC, lookupRequest());
      assertThat(network(channel, claims).getSpaceDeletionProofReceipt(lookupRequest()).getReceipt())
          .isEqualTo(receipt);
      assertStatus(() -> network(channel, claims).getSpaceDeletionProofReceipt(lookupRequest()),
          Status.Code.UNAUTHENTICATED);
      for (String field : List.of("request_hash", "aud", "rpc")) {
        var invalid = signedSpace(AuthPrincipalServerInterceptor.SPACE_DELETE_LOOKUP_RPC, lookupRequest());
        invalid.put(field, "wrong");
        assertStatus(() -> network(channel, invalid).getSpaceDeletionProofReceipt(lookupRequest()),
            Status.Code.UNAUTHENTICATED);
      }
    } finally {
      AuthPrincipalServicesTest.stop(channel, server);
    }
  }

  private IssueSpaceDeletionProofRequest issueRequest() {
    return IssueSpaceDeletionProofRequest.newBuilder()
        .setSpaceId(spaceId.toString()).setConfirmationName("Exact Space")
        .setOperationId(operationId.toString()).setPassword("password").setTotpCode("123456").build();
  }

  private ConsumeSpaceDeletionProofRequest consumeRequest() {
    return ConsumeSpaceDeletionProofRequest.newBuilder().setProtocolVersion(1)
        .setAccountId(accountId.toString()).setProfileId(profileId.toString()).setSessionEpoch(7)
        .setSpaceId(spaceId.toString()).setOperationId(operationId.toString())
        .setConfirmationName("Exact Space").setProof("opaque-proof").build();
  }

  private GetSpaceDeletionProofReceiptRequest lookupRequest() {
    return GetSpaceDeletionProofReceiptRequest.newBuilder().setProtocolVersion(1)
        .setAccountId(accountId.toString()).setProfileId(profileId.toString()).setSessionEpoch(7)
        .setSpaceId(spaceId.toString()).setOperationId(operationId.toString())
        .setConfirmationNameSha256(ByteString.copyFrom(nameDigest))
        .setProofDigestSha256(ByteString.copyFrom(proofDigest)).build();
  }

  private AcknowledgeSpaceDeletionProofReceiptRequest acknowledgeRequest(byte[] receiptDigest) {
    return AcknowledgeSpaceDeletionProofReceiptRequest.newBuilder().setProtocolVersion(1)
        .setReceiptId(receiptId.toString()).setSpaceId(spaceId.toString())
        .setOperationId(operationId.toString()).setReceiptSha256(ByteString.copyFrom(receiptDigest)).build();
  }

  private VerifiedPrincipal space() {
    return new VerifiedPrincipal("service", "space", null, null, 0);
  }

  private AuthServiceGrpc.AuthServiceBlockingStub ordinary() {
    return ordinary("Bearer user-access");
  }

  private AuthServiceGrpc.AuthServiceBlockingStub ordinary(String authorization) {
    Metadata headers = new Metadata();
    if (authorization != null) {
      headers.put(Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER), authorization);
    }
    return AuthServiceGrpc.newBlockingStub(ordinaryChannel)
        .withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers))
        .withDeadlineAfter(5, TimeUnit.SECONDS);
  }

  private StreamRecorder<ConsumeSpaceDeletionProofResponse> consume(
      VerifiedPrincipal principal, ConsumeSpaceDeletionProofRequest request) {
    var result = StreamRecorder.<ConsumeSpaceDeletionProofResponse>create();
    withPrincipal(principal, () -> grpc.consumeSpaceDeletionProof(request, result));
    return result;
  }

  private StreamRecorder<GetSpaceDeletionProofReceiptResponse> lookup(
      VerifiedPrincipal principal, GetSpaceDeletionProofReceiptRequest request) {
    var result = StreamRecorder.<GetSpaceDeletionProofReceiptResponse>create();
    withPrincipal(principal, () -> grpc.getSpaceDeletionProofReceipt(request, result));
    return result;
  }

  private StreamRecorder<AcknowledgeSpaceDeletionProofReceiptResponse> acknowledge(
      VerifiedPrincipal principal, AcknowledgeSpaceDeletionProofReceiptRequest request) {
    var result = StreamRecorder.<AcknowledgeSpaceDeletionProofReceiptResponse>create();
    withPrincipal(principal, () -> grpc.acknowledgeSpaceDeletionProofReceipt(request, result));
    return result;
  }

  private static void withPrincipal(VerifiedPrincipal principal, Runnable call) {
    if (principal == null) call.run();
    else Context.current().withValue(VerifiedPrincipal.CONTEXT, principal).run(call);
  }

  private static void denied(StreamRecorder<?> result, Status.Code code) {
    assertThat(result.getValues()).isEmpty();
    assertThat(result.getError()).isNotNull();
    assertThat(Status.fromThrowable(result.getError()).getCode()).isEqualTo(code);
  }

  private static void assertStatus(Runnable call, Status.Code code) {
    StatusRuntimeException error = assertThrows(StatusRuntimeException.class, call::run);
    assertThat(error.getStatus().getCode()).isEqualTo(code);
  }

  private static void assertUnimplemented(Runnable call) {
    assertStatus(call, Status.Code.UNIMPLEMENTED);
  }

  private Server tlsServer(AuthPrincipalVerifierTest fixture) throws Exception {
    return NettyServerBuilder.forPort(0)
        .useTransportSecurity(
            AuthPrincipalServicesTest.resource("server-cert.pem"),
            AuthPrincipalServicesTest.resource("server-key.pem"))
        .addService(AuthPrincipalServices.proofService(
            grpc.bindService(), new AuthPrincipalServerInterceptor(fixture.verifier)))
        .build().start();
  }

  private Map<String, Object> signedSpace(String rpc, Message request) {
    var claims = AuthPrincipalVerifierTest.claims();
    claims.put("iss", "space");
    claims.put("sub", "service:space");
    claims.put("principal_type", "service");
    claims.remove("account_id");
    claims.remove("profile_id");
    claims.remove("session_epoch");
    claims.put("rpc", rpc);
    claims.put("request_hash", AuthPrincipalServerInterceptor.requestHash(request));
    return claims;
  }

  private AuthServiceGrpc.AuthServiceBlockingStub network(
      ManagedChannel channel, Map<String, Object> claims) {
    Metadata headers = new Metadata();
    headers.put(AuthPrincipalServerInterceptorTest.AUTH,
        "Bearer " + AuthPrincipalVerifierTest.token(claims));
    headers.put(AuthPrincipalServerInterceptorTest.REQUEST_ID, "request-1");
    return AuthServiceGrpc.newBlockingStub(channel)
        .withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers))
        .withDeadlineAfter(5, TimeUnit.SECONDS);
  }

  private static ManagedChannel privateChannel(Server server) throws Exception {
    return NettyChannelBuilder.forAddress("localhost", server.getPort())
        .sslContext(GrpcSslContexts.forClient()
            .trustManager(AuthPrincipalServicesTest.resource("server-cert.pem")).build()).build();
  }

  private static com.google.protobuf.Timestamp timestamp(Instant instant) {
    return com.google.protobuf.Timestamp.newBuilder()
        .setSeconds(instant.getEpochSecond()).setNanos(instant.getNano()).build();
  }

  private static byte[] digest(String value) {
    try {
      return MessageDigest.getInstance("SHA-256").digest(value.getBytes(StandardCharsets.UTF_8));
    } catch (Exception impossible) {
      throw new AssertionError(impossible);
    }
  }

  private static byte[] domainHash(Message message) {
    try {
      byte[] bytes = new byte[message.getSerializedSize()];
      CodedOutputStream output = CodedOutputStream.newInstance(bytes);
      output.useDeterministicSerialization();
      message.writeTo(output);
      output.checkNoSpaceLeft();
      byte[] name = message.getDescriptorForType().getFullName().getBytes(StandardCharsets.UTF_8);
      MessageDigest sha = MessageDigest.getInstance("SHA-256");
      sha.update(name);
      sha.update((byte) 0);
      return sha.digest(bytes);
    } catch (Exception impossible) {
      throw new AssertionError(impossible);
    }
  }
}
