package voice.backend.auth.grpc;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.GetVoiceSessionEpochFloorRequest;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.ManagedChannel;
import io.grpc.Metadata;
import io.grpc.Server;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.stub.MetadataUtils;
import java.nio.charset.StandardCharsets;
import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.Signature;
import java.security.interfaces.RSAPublicKey;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Base64;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.principal.AuthPrincipalServices;
import voice.backend.auth.principal.AuthPrincipalVerifier;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

class AuthSessionFloorGrpcTest {
  private static final String RPC = AuthPrincipalServerInterceptor.VOICE_SESSION_FLOOR_RPC;
  private static final ObjectMapper JSON = new ObjectMapper();

  @Test void returnsMaximumOfDurableAccountEpochAndPersistedFloor() throws Exception {
    UUID accountId = UUID.randomUUID();
    AccountRepository accounts = mock(AccountRepository.class);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    when(accounts.findById(accountId.toString())).thenReturn(java.util.Optional.of(account(accountId, 4)));
    when(floors.requireFloor(accountId)).thenReturn(6L);

    var response = invoke(accountId, accounts, floors);

    assertEquals(6L, response.getSessionEpochFloor());
  }

  @Test void unavailableAndNonpositiveAuthorityFailClosed() throws Exception {
    UUID accountId = UUID.randomUUID();
    AccountRepository accounts = mock(AccountRepository.class);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    when(accounts.findById(accountId.toString())).thenReturn(java.util.Optional.of(account(accountId, 4)));
    when(floors.requireFloor(accountId)).thenThrow(new IllegalStateException("storage unavailable"));
    var unavailable = assertThrows(StatusRuntimeException.class, () -> invoke(accountId, accounts, floors));
    assertEquals(Status.Code.UNAVAILABLE, unavailable.getStatus().getCode());

    when(floors.requireFloor(accountId)).thenReturn(0L);
    unavailable = assertThrows(StatusRuntimeException.class, () -> invoke(accountId, accounts, floors));
    assertEquals(Status.Code.UNAVAILABLE, unavailable.getStatus().getCode());
  }

  @Test void absentSourcesAndUnknownAccountFailClosed() throws Exception {
    UUID accountId = UUID.randomUUID();
    var absent = assertThrows(StatusRuntimeException.class, () -> invoke(accountId, null, null));
    assertEquals(Status.Code.UNAVAILABLE, absent.getStatus().getCode());

    AccountRepository accounts = mock(AccountRepository.class);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    when(accounts.findById(accountId.toString())).thenReturn(java.util.Optional.empty());
    var missing = assertThrows(StatusRuntimeException.class, () -> invoke(accountId, accounts, floors));
    assertEquals(Status.Code.NOT_FOUND, missing.getStatus().getCode());
  }

  private static Account account(UUID id, long epoch) {
    return new Account(id, null, null, null, "regular", "active", null, false,
        epoch, Instant.parse("2026-01-01T00:00:00Z"), null);
  }

  private static app.voice.auth.v1.GetVoiceSessionEpochFloorResponse invoke(
      UUID accountId, AccountRepository accounts, SessionEpochFloorStore floors) throws Exception {
    KeyPairGenerator generator = KeyPairGenerator.getInstance("RSA");
    generator.initialize(2048);
    KeyPair keyPair = generator.generateKeyPair();
    var clock = Clock.systemUTC();
    Set<String> replay = new HashSet<>();
    var verifier = new AuthPrincipalVerifier(
        (issuer, kid) -> "current".equals(kid) && "voice".equals(issuer)
            ? (RSAPublicKey) keyPair.getPublic() : null,
        account -> { throw new AssertionError("service principal must not query user epoch here"); },
        (issuer, jti, expires) -> {
          if (!replay.add(issuer + ":" + jti)) throw new IllegalArgumentException("replayed request");
        }, clock);
    var service = new AuthGrpcService(null, null);
    if (accounts != null && floors != null) service.setVoiceSessionFloorSources(accounts, floors);
    var protectedService = AuthPrincipalServices.sessionFloorService(
        service.bindService(), new AuthPrincipalServerInterceptor(verifier));
    Server server = NettyServerBuilder.forPort(0).addService(protectedService).build().start();
    ManagedChannel channel = NettyChannelBuilder.forAddress("localhost", server.getPort()).usePlaintext().build();
    try {
      var request = GetVoiceSessionEpochFloorRequest.newBuilder().setAccountId(accountId.toString()).build();
      String requestId = UUID.randomUUID().toString();
      String token = token(keyPair, request, requestId, clock.instant());
      Metadata headers = new Metadata();
      headers.put(Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER), "Bearer " + token);
      headers.put(Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER), requestId);
      var signed = io.grpc.ClientInterceptors.intercept(channel, MetadataUtils.newAttachHeadersInterceptor(headers));
      return AuthServiceGrpc.newBlockingStub(signed)
          .withDeadlineAfter(5, TimeUnit.SECONDS)
          .getVoiceSessionEpochFloor(request);
    } finally {
      channel.shutdownNow().awaitTermination(5, TimeUnit.SECONDS);
      server.shutdownNow().awaitTermination(5, TimeUnit.SECONDS);
    }
  }

  private static String token(KeyPair keyPair, GetVoiceSessionEpochFloorRequest request,
      String requestId, Instant now) throws Exception {
    String header = encode(JSON.writeValueAsBytes(Map.of("alg", "RS256", "kid", "current")));
    String claims = encode(JSON.writeValueAsBytes(Map.of(
        "iss", "voice", "sub", "service:voice", "principal_type", "service", "aud", "auth",
        "rpc", RPC, "request_id", requestId,
        "request_hash", AuthPrincipalServerInterceptor.requestHash(request),
        "jti", UUID.randomUUID().toString(), "iat", now.getEpochSecond(),
        "nbf", now.getEpochSecond(), "exp", now.plusSeconds(20).getEpochSecond())));
    String unsigned = header + "." + claims;
    Signature signature = Signature.getInstance("SHA256withRSA");
    signature.initSign(keyPair.getPrivate());
    signature.update(unsigned.getBytes(StandardCharsets.US_ASCII));
    return unsigned + "." + encode(signature.sign());
  }

  private static String encode(byte[] value) {
    return Base64.getUrlEncoder().withoutPadding().encodeToString(value);
  }
}
