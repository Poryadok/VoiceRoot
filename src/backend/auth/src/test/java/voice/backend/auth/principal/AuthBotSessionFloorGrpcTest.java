package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import app.voice.auth.v1.*;
import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.*;
import io.grpc.stub.MetadataUtils;
import java.security.interfaces.RSAPublicKey;
import java.time.*;
import java.util.*;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;
import org.springframework.mock.env.MockEnvironment;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.repository.*;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

class AuthBotSessionFloorGrpcTest {
  static final String RPC = "/voice.auth.v1.AuthService/GetBotSessionEpochFloor";
  static final String PEER = "urn:voice:test:bot-floor";
  static final UUID ACCOUNT = UUID.fromString("00000000-0000-0000-0000-000000000271");
  static final GetBotSessionEpochFloorRequest REQUEST = GetBotSessionEpochFloorRequest.newBuilder()
      .setAccountId(ACCOUNT.toString()).build();
  static final String INTEROP = "VOICE_AUTH_BOT_FLOOR_INTEROP_";
  record InteropOptions(Clock clock, String clockMode, java.security.KeyPair signer, String kid,
      byte[] jwks, UUID accountId, boolean suppliedSigner) {}

  static Fixture portableFixture(Map<String,String> environment) throws Exception {
    return new Fixture(PEER, "bot", true, true, null, portableOptions(environment));
  }

  static InteropOptions portableOptions(Map<String,String> environment) {
    try {
      String mode = environment.getOrDefault(INTEROP + "CLOCK_MODE", "fixed");
      if (!Set.of("fixed", "system").contains(mode)) throw new IllegalArgumentException();
      Clock clock = mode.equals("system") ? Clock.systemUTC() : Clock.fixed(AuthPrincipalVerifierTest.NOW, ZoneOffset.UTC);
      String rawAccount = environment.getOrDefault(INTEROP + "ACCOUNT_ID", ACCOUNT.toString());
      UUID account = UUID.fromString(rawAccount);
      if (!account.toString().equals(rawAccount) || account.equals(new UUID(0, 0))) throw new IllegalArgumentException();
      var names = List.of("SIGNER_KEY_FILE", "SIGNER_KEY_ID", "JWKS_FILE");
      long present = names.stream().filter(n -> environment.containsKey(INTEROP + n)).count();
      if (present != 0 && present != names.size()) throw new IllegalArgumentException();
      var signer = AuthPrincipalVerifierTest.KEY;
      String kid = "current";
      byte[] jwks = AuthPrincipalJwksResolverTest.set(
          AuthPrincipalJwksResolverTest.jwk(kid, (RSAPublicKey) signer.getPublic()),
          AuthPrincipalJwksResolverTest.jwk("next", AuthPrincipalJwksResolverTest.NEXT));
      if (present != 0) {
        for (String name : names) if (environment.get(INTEROP + name).isBlank()) throw new IllegalArgumentException();
        kid = environment.get(INTEROP + "SIGNER_KEY_ID");
        if (!kid.matches("[A-Za-z0-9][A-Za-z0-9._-]{0,127}")) throw new IllegalArgumentException();
        var keyPath = java.nio.file.Path.of(environment.get(INTEROP + "SIGNER_KEY_FILE"));
        var jwksPath = java.nio.file.Path.of(environment.get(INTEROP + "JWKS_FILE"));
        if (java.nio.file.Files.size(keyPath) > 32768 || java.nio.file.Files.size(jwksPath) > 65536)
          throw new IllegalArgumentException();
        String pem = java.nio.file.Files.readString(keyPath, java.nio.charset.StandardCharsets.US_ASCII).strip();
        String begin = "-----BEGIN PRIVATE KEY-----", end = "-----END PRIVATE KEY-----";
        if (!pem.startsWith(begin) || !pem.endsWith(end)) throw new IllegalArgumentException();
        byte[] der = Base64.getDecoder().decode(pem.substring(begin.length(), pem.length() - end.length()).replaceAll("\\s", ""));
        var factory = java.security.KeyFactory.getInstance("RSA");
        var privateKey = (java.security.interfaces.RSAPrivateCrtKey) factory.generatePrivate(new java.security.spec.PKCS8EncodedKeySpec(der));
        var publicKey = (RSAPublicKey) factory.generatePublic(new java.security.spec.RSAPublicKeySpec(
            privateKey.getModulus(), privateKey.getPublicExponent()));
        signer = new java.security.KeyPair(publicKey, privateKey);
        jwks = java.nio.file.Files.readAllBytes(jwksPath);
        byte[] approvedSnapshot = jwks;
        var approved = new AuthPrincipalJwksResolver(issuer -> approvedSnapshot, clock,
            Duration.ofSeconds(5), Duration.ofSeconds(10), Duration.ofSeconds(1)).resolveForBotFloor("bot", kid);
        if (!Arrays.equals(publicKey.getEncoded(), approved.getEncoded())) throw new IllegalArgumentException();
      }
      return new InteropOptions(clock, mode, signer, kid, jwks, account, present != 0);
    } catch (Exception invalid) {
      // This is a test-input error, never disclose private-key bytes or parser diagnostics.
      throw new IllegalArgumentException("invalid portable fixture inputs");
    }
  }

  static String privatePem(java.security.KeyPair signer) {
    return "-----BEGIN PRIVATE KEY-----\n" + Base64.getMimeEncoder(64, new byte[]{10})
        .encodeToString(signer.getPrivate().getEncoded()) + "\n-----END PRIVATE KEY-----\n";
  }

  @Test void portableSystemClockAcceptsARealCurrentSignedRequest() throws Exception {
    try (var f = portableFixture(Map.of("VOICE_AUTH_BOT_FLOOR_INTEROP_CLOCK_MODE", "system"))) {
      var claims = f.claims(REQUEST);
      Instant now = Instant.now();
      claims.put("iat", now.getEpochSecond()); claims.put("nbf", now.getEpochSecond());
      claims.put("exp", now.plusSeconds(20).getEpochSecond());
      assertEquals(9, f.call(REQUEST, claims).getSessionEpochFloor());
    }
  }

  @Test void portableDefaultsKeepTheOriginalClockKeyAccountAndServingBehavior() throws Exception {
    var defaults = portableOptions(Map.of());
    assertEquals("fixed", defaults.clockMode()); assertEquals(AuthPrincipalVerifierTest.NOW, defaults.clock().instant());
    assertEquals(ACCOUNT, defaults.accountId()); assertEquals("current", defaults.kid());
    assertFalse(defaults.suppliedSigner()); assertEquals(AuthPrincipalVerifierTest.KEY.getPublic(), defaults.signer().getPublic());
    try (var f = portableFixture(Map.of())) {
      assertEquals(AuthPrincipalVerifierTest.NOW.getEpochSecond(), f.claims(REQUEST).get("iat"));
      assertEquals(9, f.call(REQUEST, f.claims(REQUEST)).getSessionEpochFloor());
    }
  }

  static Map<String,String> suppliedInputs(java.nio.file.Path directory, java.security.KeyPair signer, String kid, UUID account) throws Exception {
    var key = directory.resolve("caller-owned-key.pem");
    var jwks = directory.resolve("approved-current-next.json");
    java.nio.file.Files.writeString(key, privatePem(signer));
    java.nio.file.Files.write(jwks, AuthPrincipalJwksResolverTest.set(
        AuthPrincipalJwksResolverTest.jwk(kid, (RSAPublicKey) signer.getPublic()),
        AuthPrincipalJwksResolverTest.jwk("next", AuthPrincipalJwksResolverTest.NEXT)));
    return new HashMap<>(Map.of(INTEROP + "SIGNER_KEY_FILE", key.toString(), INTEROP + "SIGNER_KEY_ID", kid,
        INTEROP + "JWKS_FILE", jwks.toString(), INTEROP + "ACCOUNT_ID", account.toString(), INTEROP + "CLOCK_MODE", "system"));
  }

  @Test void portableSuppliedSignerSnapshotAndAccountServeTogether(@org.junit.jupiter.api.io.TempDir java.nio.file.Path directory) throws Exception {
    var signer = AuthPrincipalVerifierTest.key(); UUID account = UUID.randomUUID();
    var inputs = suppliedInputs(directory, signer, "role-fixture-current", account);
    byte[] snapshot = java.nio.file.Files.readAllBytes(directory.resolve("approved-current-next.json"));
    assertArrayEquals(snapshot, portableOptions(inputs).jwks(), "approved complete snapshot must remain byte-for-byte unchanged");
    try (var f = portableFixture(inputs)) {
      var request = REQUEST.toBuilder().setAccountId(account.toString()).build();
      assertEquals("role-fixture-current", f.signingKid);
      assertEquals(signer.getPublic(), portableOptions(inputs).signer().getPublic());
      assertEquals(9, f.call(request, f.claims(request)).getSessionEpochFloor());
      verify(f.accounts).findById(account.toString()); verify(f.floors).requireFloor(account);
      verifyNoMoreInteractions(f.accounts, f.floors);
    }
  }

  @Test void portableInvalidInputsFailBeforeAnyListenerIsCreated(@org.junit.jupiter.api.io.TempDir java.nio.file.Path directory) throws Exception {
    var signer = AuthPrincipalVerifierTest.key();
    var valid = suppliedInputs(directory, signer, "shared-current", ACCOUNT);
    var triple = List.of("SIGNER_KEY_FILE", "SIGNER_KEY_ID", "JWKS_FILE");
    for (int mask = 1; mask < 7; mask++) {
      Map<String,String> partial = new HashMap<>();
      for (int bit = 0; bit < triple.size(); bit++) if ((mask & (1 << bit)) != 0)
        partial.put(INTEROP + triple.get(bit), valid.get(INTEROP + triple.get(bit)));
      assertThrows(IllegalArgumentException.class, () -> portableOptions(partial));
    }
    for (String name : triple) {
      var blank = new HashMap<>(valid); blank.put(INTEROP + name, "");
      assertThrows(IllegalArgumentException.class, () -> portableOptions(blank));
    }
    for (String mode : List.of("", "SYSTEM", "current", "arbitrary"))
      assertThrows(IllegalArgumentException.class, () -> portableOptions(Map.of(INTEROP + "CLOCK_MODE", mode)));
    for (String id : List.of("", "not-a-uuid", "00000000-0000-0000-0000-000000000000", "8A78BD68-75E9-4F21-9387-3BDC2F6116AB"))
      assertThrows(IllegalArgumentException.class, () -> portableOptions(Map.of(INTEROP + "ACCOUNT_ID", id)));
    var unknownKid = new HashMap<>(valid); unknownKid.put(INTEROP + "SIGNER_KEY_ID", "absent");
    assertThrows(IllegalArgumentException.class, () -> portableOptions(unknownKid));
    java.nio.file.Files.writeString(directory.resolve("caller-owned-key.pem"), privatePem(AuthPrincipalVerifierTest.key()));
    assertThrows(IllegalArgumentException.class, () -> portableOptions(valid), "different private key cannot borrow the approved kid");
    java.nio.file.Files.writeString(directory.resolve("caller-owned-key.pem"), "not a PKCS8 key");
    assertThrows(IllegalArgumentException.class, () -> portableOptions(valid));
    java.nio.file.Files.writeString(directory.resolve("caller-owned-key.pem"), privatePem(signer));
    java.nio.file.Files.writeString(directory.resolve("approved-current-next.json"), "{}");
    assertThrows(IllegalArgumentException.class, () -> portableOptions(valid));
  }

  @Test void portableMainActuallyServesCurrentClockSharedSignerAndSelectedAccount(@org.junit.jupiter.api.io.TempDir java.nio.file.Path directory) throws Exception {
    var signer = AuthPrincipalVerifierTest.key(); UUID account = UUID.randomUUID();
    var inputs = suppliedInputs(directory, signer, "shared-current", account);
    var output = directory.resolve("server-output"); java.nio.file.Files.createDirectories(output);
    var processBuilder = new ProcessBuilder(java.nio.file.Path.of(System.getProperty("java.home"), "bin", "java").toString(),
        "-cp", System.getProperty("java.class.path"), AuthBotSessionFloorGrpcTest.class.getName(), output.toString(), "30");
    processBuilder.environment().putAll(inputs);
    processBuilder.redirectOutput(output.resolve("stdout.log").toFile()); processBuilder.redirectError(output.resolve("stderr.log").toFile());
    Process process = processBuilder.start(); ManagedChannel channel = null;
    try {
      var readyPath = output.resolve("ready.json"); long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(20);
      while (!java.nio.file.Files.exists(readyPath) && process.isAlive() && System.nanoTime() < deadline) Thread.sleep(20);
      assertTrue(java.nio.file.Files.exists(readyPath), "portable child fixture must become ready");
      var ready = AuthPrincipalVerifierTest.JSON.readTree(java.nio.file.Files.readString(readyPath));
      assertEquals("system", ready.get("clock_mode").asText()); assertEquals("supplied", ready.get("signer_mode").asText());
      assertEquals(account.toString(), ready.get("account_id").asText()); assertEquals("shared-current", ready.get("kid").asText());
      assertTrue(Duration.between(Instant.parse(ready.get("clock").asText()), Instant.now()).abs().compareTo(Duration.ofSeconds(10)) < 0);
      assertArrayEquals(java.nio.file.Files.readAllBytes(directory.resolve("approved-current-next.json")),
          java.nio.file.Files.readAllBytes(output.resolve("bot-jwks.json")));
      channel = NettyChannelBuilder.forAddress("localhost", ready.get("port").asInt()).sslContext(
          GrpcSslContexts.forClient().trustManager(output.resolve("server-cert.pem").toFile())
              .keyManager(output.resolve("bot-cert.pem").toFile(), output.resolve("bot-key.pem").toFile()).build()).build();
      var request = REQUEST.toBuilder().setAccountId(account.toString()).build();
      var claims = AuthPrincipalVerifierTest.claims();
      claims.put("iss", "bot"); claims.put("sub", "service:bot"); claims.put("principal_type", "service");
      claims.put("rpc", RPC); claims.put("request_hash", AuthPrincipalServerInterceptor.requestHash(request));
      claims.remove("account_id"); claims.remove("profile_id"); claims.remove("session_epoch");
      Instant now = Instant.now(); claims.put("iat", now.getEpochSecond()); claims.put("nbf", now.getEpochSecond()); claims.put("exp", now.plusSeconds(20).getEpochSecond());
      Metadata headers = new Metadata(); headers.put(Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER),
          "Bearer " + AuthPrincipalVerifierTest.token(claims, Map.of("alg", "RS256", "kid", "shared-current"), signer.getPrivate()));
      headers.put(Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER), "request-1");
      var response = AuthServiceGrpc.newBlockingStub(ClientInterceptors.intercept(channel, MetadataUtils.newAttachHeadersInterceptor(headers)))
          .withDeadlineAfter(5, TimeUnit.SECONDS).getBotSessionEpochFloor(request);
      assertEquals(9, response.getSessionEpochFloor());
    } finally {
      if (channel != null) channel.shutdownNow().awaitTermination(5, TimeUnit.SECONDS);
      process.destroy(); if (!process.waitFor(5, TimeUnit.SECONDS)) process.destroyForcibly().waitFor(5, TimeUnit.SECONDS);
    }
  }

  /** Bounded private-network Go-to-Java interop fixture; every key and account here is test-only. */
  public static void main(String[] args) throws Exception {
    if (args.length != 2) throw new IllegalArgumentException("output directory and lifetime seconds required");
    long seconds = Long.parseLong(args[1]);
    if (seconds < 1 || seconds > 600) throw new IllegalArgumentException("fixture lifetime must be 1..600 seconds");
    var options = portableOptions(System.getenv());
    var out = java.nio.file.Path.of(args[0]).toAbsolutePath();
    java.nio.file.Files.createDirectories(out);
    var control = out.resolve("retained-floor.txt");
    java.nio.file.Files.writeString(control, "9");
    try (var f = new Fixture(PEER, "bot", true, true, null, options)) {
      when(f.floors.requireFloor(options.accountId())).thenAnswer(invocation ->
          Long.parseLong(java.nio.file.Files.readString(control).strip()));
      for (String name : List.of("server-cert.pem", "bot-cert.pem", "bot-key.pem"))
        java.nio.file.Files.copy(resource(name).toPath(), out.resolve(name), java.nio.file.StandardCopyOption.REPLACE_EXISTING);
      java.nio.file.Files.writeString(out.resolve("bot-signing-key.pem"), privatePem(options.signer()));
      java.nio.file.Files.write(out.resolve("bot-jwks.json"), options.jwks());
      String ready = AuthPrincipalVerifierTest.JSON.writeValueAsString(Map.ofEntries(
          Map.entry("port", f.server.getPort()), Map.entry("account_id", options.accountId().toString()),
          Map.entry("floor", 9), Map.entry("durable_epoch", 7), Map.entry("peer_uri", PEER),
          Map.entry("clock", options.clock().instant().toString()), Map.entry("clock_mode", options.clockMode()),
          Map.entry("kid", options.kid()), Map.entry("signer_mode", options.suppliedSigner() ? "supplied" : "generated"),
          Map.entry("rpc", RPC), Map.entry("tls_server_name", "localhost"),
          Map.entry("test_signing_key", out.resolve("bot-signing-key.pem").toString()),
          Map.entry("test_jwks", out.resolve("bot-jwks.json").toString()),
          Map.entry("retained_floor_control", control.toString())));
      java.nio.file.Files.writeString(out.resolve("ready.pending"), ready);
      java.nio.file.Files.move(out.resolve("ready.pending"), out.resolve("ready.json"),
          java.nio.file.StandardCopyOption.ATOMIC_MOVE, java.nio.file.StandardCopyOption.REPLACE_EXISTING);
      System.out.println(ready);
      new java.util.concurrent.CountDownLatch(1).await(seconds, TimeUnit.SECONDS);
    }
  }

  @Test void actualBotMutualTlsAndSignedBodyReadPositiveMaximumWithoutWrites() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      assertEquals(9, f.call(REQUEST, f.claims(REQUEST)).getSessionEpochFloor());
      when(f.floors.requireFloor(ACCOUNT)).thenReturn(4L);
      assertEquals(7, f.call(REQUEST, f.claims(REQUEST)).getSessionEpochFloor());
      verify(f.accounts, times(2)).findById(ACCOUNT.toString());
      verify(f.floors, times(2)).requireFloor(ACCOUNT);
      verifyNoMoreInteractions(f.accounts, f.floors);
    }
  }

  @Test void sameCaForeignPeerAndMissingConfigurationNeverReadOwners() throws Exception {
    for (String peer : Arrays.asList(null, "", "not a URI", "urn:voice:test:foreign", "urn:voice:*", "/relative",
        "urn:voice:test:bot-floor#fragment", "https://user@example.test/bot", "https://example.test/a/../bot")) {
      try (var f = new Fixture(peer, "bot", true)) {
        assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
        verifyNoInteractions(f.accounts, f.floors);
      }
    }
    try (var f = new Fixture(PEER, "server", true)) {
      assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void plaintextEvenWithBotConfigNeverReadsOwners() throws Exception {
    try (var f = new Fixture(PEER, "bot", false)) {
      assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void rejectsMalformedAccountsAndUnknownFieldsBeforeOwnerReads() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      for (String id : List.of("", "not-an-account", "00000000-0000-0000-0000-000000000000",
          "8A78BD68-75E9-4F21-9387-3BDC2F6116AB")) {
        var request = REQUEST.toBuilder().setAccountId(id).build();
        assertEquals(Status.Code.INVALID_ARGUMENT, assertThrows(StatusRuntimeException.class,
            () -> f.call(request, f.claims(request))).getStatus().getCode());
      }
      var request = REQUEST.toBuilder().setUnknownFields(com.google.protobuf.UnknownFieldSet.newBuilder()
          .addField(99, com.google.protobuf.UnknownFieldSet.Field.newBuilder().addVarint(1).build()).build()).build();
      assertEquals(Status.Code.INVALID_ARGUMENT, assertThrows(StatusRuntimeException.class,
          () -> f.call(request, f.claims(request))).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void wrongBindingsMixedIdentityTemporalViolationsAndReplayNeverReadOwners() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      for (String field : List.of("sub", "principal_type", "aud", "rpc", "request_id", "request_hash", "jti")) {
        var c = f.claims(REQUEST); c.put(field, "wrong");
        if (field.equals("jti")) c.remove(field);
        assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, c), field).getStatus().getCode());
      }
      for (String field : List.of("account_id", "profile_id", "session_epoch", "account_type", "bot_id", "bot_actor_id",
          "delegation", "actor_profile_id", "installation_id", "bot_scope", "arbitrary_extension"))
          for (Object value : Arrays.asList(null, "", "untrusted")) {
        var c = f.claims(REQUEST); c.put(field, value);
        assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, c), field).getStatus().getCode());
      }
      for (var change : List.of(Map.entry("exp", AuthPrincipalVerifierTest.NOW.getEpochSecond()),
          Map.entry("exp", AuthPrincipalVerifierTest.NOW.plusSeconds(31).getEpochSecond()),
          Map.entry("iat", AuthPrincipalVerifierTest.NOW.plusSeconds(6).getEpochSecond()),
          Map.entry("nbf", AuthPrincipalVerifierTest.NOW.plusSeconds(6).getEpochSecond()))) {
        var c = f.claims(REQUEST); c.put(change.getKey(), change.getValue());
        assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, c)).getStatus().getCode());
      }
      verifyNoInteractions(f.accounts, f.floors);
      var valid = f.claims(REQUEST);
      assertEquals(9, f.call(REQUEST, valid).getSessionEpochFloor());
      clearInvocations(f.accounts, f.floors);
      assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, valid)).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void readErrorsAndMissingOrDeletedAccountsNeverFabricateFloor() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      when(f.accounts.findById(ACCOUNT.toString())).thenReturn(Optional.empty());
      assertEquals(Status.Code.NOT_FOUND, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      for (Account deleted : List.of(account(7, "deleted", null), account(7, "active", Instant.now()))) {
        when(f.accounts.findById(ACCOUNT.toString())).thenReturn(Optional.of(deleted));
        assertEquals(Status.Code.NOT_FOUND, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      }
      verifyNoInteractions(f.floors);
      when(f.accounts.findById(ACCOUNT.toString())).thenReturn(Optional.of(account(7, "active", null)));
      for (long invalid : new long[]{0, -1}) {
        when(f.floors.requireFloor(ACCOUNT)).thenReturn(invalid);
        assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      }
      when(f.floors.requireFloor(ACCOUNT)).thenThrow(new IllegalStateException("floor outage"));
      assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      when(f.accounts.findById(ACCOUNT.toString())).thenThrow(new IllegalStateException("database outage"));
      assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
    }
  }

  @Test void wrongServicesAndDuplicateOrRawMetadataCannotReadOwners() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      var delegated = AuthPrincipalVerifierTest.claims();
      delegated.put("rpc", RPC); delegated.put("request_hash", AuthPrincipalServerInterceptor.requestHash(REQUEST));
      assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, delegated)).getStatus().getCode());
      for (String issuer : List.of("role", "space", "voice", "gateway")) {
        var c = f.claims(REQUEST); c.put("iss", issuer); c.put("sub", "service:" + issuer);
        assertEquals(Status.Code.PERMISSION_DENIED, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, c)).getStatus().getCode());
      }
      for (String header : List.of("authorization", "x-request-id", "x-account-id", "x-profile-id", "x-voice-bot-id")) {
        var extra = new Metadata();
        extra.put(Metadata.Key.of(header, Metadata.ASCII_STRING_MARSHALLER), "untrusted");
        assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, f.claims(REQUEST), extra)).getStatus().getCode());
      }
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void dependencyOutagesAreUnavailableAndDoNotReadOwnerSources() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      f.keyOutage = true;
      assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      f.keyOutage = false; f.replayOutage = true;
      assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void freshAuthoritativeMissingKidIsInvalidBeforeAnyOwnerReads() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      f.signingKid = "not-approved";
      for (int attempt = 0; attempt < 2; attempt++) {
        assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      }
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void exactGeneratedGoRequestHashVectorIsPreserved() {
    // Independently executed shared Go principal.RequestHash, BE170 consumer247.
    assertEquals("0a2430303030303030302d303030302d303030302d303030302d303030303030303030323731",
        java.util.HexFormat.of().formatHex(REQUEST.toByteArray()));
    assertEquals("sha256:5e3a01dc6a24fdf4ed5b82af5e85daa41e759a7e0ec243852110fa8468d8b692",
        AuthPrincipalServerInterceptor.requestHash(REQUEST));
  }

  @Test void botCannotBorrowAnyOldProtectedMethod() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      var verifier = new AuthPrincipalVerifierTest().verifier;
      for (String rpc : List.of(AuthPrincipalServerInterceptor.ISSUE_RPC, AuthPrincipalServerInterceptor.CONSUME_RPC,
          AuthPrincipalServerInterceptor.LOOKUP_RPC, AuthPrincipalServerInterceptor.VOICE_SESSION_FLOOR_RPC,
          AuthPrincipalServerInterceptor.ROLE_SESSION_FLOOR_RPC, AuthPrincipalServerInterceptor.SPACE_SESSION_FLOOR_RPC,
          AuthPrincipalServerInterceptor.SPACE_ADMISSION_RPC, AuthPrincipalServerInterceptor.SPACE_DELETE_CONSUME_RPC,
          AuthPrincipalServerInterceptor.SPACE_DELETE_LOOKUP_RPC, AuthPrincipalServerInterceptor.SPACE_DELETE_ACK_RPC)) {
        var c = f.claims(REQUEST); c.put("rpc", rpc);
        assertEquals(Status.Code.PERMISSION_DENIED, assertThrows(StatusRuntimeException.class,
            () -> verifier.verify(AuthPrincipalVerifierTest.token(c), rpc, "request-1",
                AuthPrincipalServerInterceptor.requestHash(REQUEST)), rpc).getStatus().getCode());
      }
      var old = f.claims(REQUEST);
      old.put("iss", "voice"); old.put("sub", "service:voice");
      old.put("rpc", AuthPrincipalServerInterceptor.VOICE_SESSION_FLOOR_RPC); old.put("actor_profile_id", null);
      assertEquals("voice", verifier.verify(AuthPrincipalVerifierTest.token(old),
          AuthPrincipalServerInterceptor.VOICE_SESSION_FLOOR_RPC, "request-1",
          AuthPrincipalServerInterceptor.requestHash(REQUEST)).issuer(), "closed claim shape is Bot-method only");
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void missingClientCertificateAndDisabledVerifierCannotReachBotHandler() throws Exception {
    try (var f = new Fixture(PEER, null, true)) {
      assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
    try (var f = new Fixture(PEER, "bot", true, false)) {
      assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void ordinaryAndProofDefinitionsExcludeBotMethod() {
    var service = new AuthGrpcService(null, null).bindService();
    for (var definition : List.of(AuthPrincipalServices.legacyService(service),
        AuthPrincipalServices.proofService(service, new AuthPrincipalServerInterceptor(null)))) {
      assertFalse(definition.getMethods().stream().anyMatch(m ->
          m.getMethodDescriptor().getFullMethodName().equals(RPC.substring(1))));
    }
  }

  static class Fixture implements AutoCloseable {
    final AccountRepository accounts = mock(AccountRepository.class);
    final SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    final Set<String> replay = new HashSet<>();
    volatile boolean keyOutage, replayOutage;
    String signingKid = "current";
    java.security.PrivateKey signingKey = AuthPrincipalVerifierTest.KEY.getPrivate();
    final Clock clock;
    final UUID accountId;
    final Server server;
    final ManagedChannel channel;
    Fixture(String peer, String certificate, boolean tls) throws Exception {
      this(peer, certificate, tls, true);
    }
    Fixture(String peer, String certificate, boolean tls, boolean enabled) throws Exception {
      this(peer, certificate, tls, enabled, null);
    }
    Fixture(String peer, String certificate, boolean tls, boolean enabled, AuthPrincipalJwksResolver suppliedResolver) throws Exception {
      this(peer, certificate, tls, enabled, suppliedResolver, portableOptions(Map.of()));
    }
    Fixture(String peer, String certificate, boolean tls, boolean enabled, AuthPrincipalJwksResolver suppliedResolver,
        InteropOptions options) throws Exception {
      clock = options.clock(); accountId = options.accountId();
      signingKey = options.signer().getPrivate(); signingKid = options.kid();
      when(accounts.findById(accountId.toString())).thenReturn(Optional.of(account(accountId, 7, "active", null)));
      when(floors.requireFloor(accountId)).thenReturn(9L);
      var resolver = suppliedResolver != null ? suppliedResolver : new AuthPrincipalJwksResolver(issuer -> options.jwks(),
          clock, Duration.ofSeconds(5), Duration.ofSeconds(10), Duration.ofSeconds(1));
      var verifier = new AuthPrincipalVerifier((issuer, kid) -> {
            if (keyOutage) throw new PrincipalDependencyUnavailableException();
            return resolver.resolve(issuer, kid);
          },
          id -> { throw new AssertionError("service proof must not read delegated epoch"); },
          (issuer, jti, expires) -> {
            if (replayOutage) throw new PrincipalDependencyUnavailableException();
            if (!replay.add(issuer + ":" + jti)) throw new IllegalArgumentException();
          },
          clock,
          (issuer, kid) -> {
            if (keyOutage) throw new PrincipalDependencyUnavailableException();
            return resolver.resolveForBotFloor(issuer, kid);
          });
      var interceptor = new AuthPrincipalServerInterceptor(enabled ? verifier : null, () -> {}, peer);
      var service = new AuthGrpcService(null, null);
      service.setVoiceSessionFloorSources(accounts, floors);
      var builder = NettyServerBuilder.forPort(0);
      if (tls) {
        var env = new MockEnvironment()
            .withProperty("AUTH_GRPC_TLS_CERT_FILE", resource("server-cert.pem").toString())
            .withProperty("AUTH_GRPC_TLS_KEY_FILE", resource("server-key.pem").toString())
            .withProperty("AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE", resource("server-cert.pem").toString());
        env.setActiveProfiles("production");
        AuthPrincipalTransport.configureSessionFloor(builder, env, true);
      }
      server = builder.addService(AuthPrincipalServices.sessionFloorService(service.bindService(), interceptor)).build().start();
      var client = NettyChannelBuilder.forAddress("localhost", server.getPort());
      if (tls) {
        var ssl = GrpcSslContexts.forClient().trustManager(resource("server-cert.pem"));
        if (certificate != null) ssl.keyManager(resource(certificate + "-cert.pem"), resource(certificate + "-key.pem"));
        client.sslContext(ssl.build());
      }
      else client.usePlaintext();
      channel = client.build();
      clearInvocations(accounts, floors);
    }
    Map<String,Object> claims(com.google.protobuf.MessageLite request) {
      var c = AuthPrincipalVerifierTest.claims();
      c.put("iss", "bot"); c.put("sub", "service:bot"); c.put("principal_type", "service");
      c.put("rpc", RPC); c.put("request_hash", AuthPrincipalServerInterceptor.requestHash(request));
      c.remove("account_id"); c.remove("profile_id"); c.remove("session_epoch");
      Instant now = clock.instant();
      c.put("iat", now.getEpochSecond()); c.put("nbf", now.getEpochSecond()); c.put("exp", now.plusSeconds(30).getEpochSecond());
      return c;
    }
    GetBotSessionEpochFloorResponse call(GetBotSessionEpochFloorRequest request, Map<String,Object> claims) {
      return call(request, claims, new Metadata());
    }
    GetBotSessionEpochFloorResponse call(GetBotSessionEpochFloorRequest request, Map<String,Object> claims, Metadata extra) {
      return AuthServiceGrpc.newBlockingStub(signed(claims, extra)).withDeadlineAfter(5, TimeUnit.SECONDS).getBotSessionEpochFloor(request);
    }
    long callSpace(Map<String,Object> claims) {
      return AuthServiceGrpc.newBlockingStub(signed(claims, new Metadata())).withDeadlineAfter(5, TimeUnit.SECONDS)
          .getSpaceSessionEpochFloor(GetSpaceSessionEpochFloorRequest.newBuilder().setAccountId(accountId.toString()).build())
          .getSessionEpochFloor();
    }
    Channel signed(Map<String,Object> claims, Metadata extra) {
      Metadata headers = new Metadata();
      headers.put(Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER),
          "Bearer " + AuthPrincipalVerifierTest.token(claims, Map.of("alg", "RS256", "kid", signingKid),
              signingKey));
      headers.put(Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER), "request-1");
      headers.merge(extra);
      return ClientInterceptors.intercept(channel, MetadataUtils.newAttachHeadersInterceptor(headers));
    }
    @Override public void close() throws Exception {
      channel.shutdownNow(); server.shutdownNow();
      channel.awaitTermination(5, TimeUnit.SECONDS); server.awaitTermination(5, TimeUnit.SECONDS);
    }
  }
  static Account account(long epoch, String status, Instant deleted) {
    return account(ACCOUNT, epoch, status, deleted);
  }
  static Account account(UUID id, long epoch, String status, Instant deleted) {
    return new Account(id, null, null, null, "regular", status, null, false, epoch,
        Instant.parse("2026-01-01T00:00:00Z"), deleted);
  }
  static java.io.File resource(String name) throws Exception { return AuthPrincipalServicesTest.resource(name); }
}
