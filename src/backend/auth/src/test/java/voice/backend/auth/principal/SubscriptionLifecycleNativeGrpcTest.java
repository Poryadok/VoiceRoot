package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import app.voice.auth.v1.*;
import com.sun.net.httpserver.HttpsConfigurator;
import com.sun.net.httpserver.HttpsServer;
import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.*;
import io.grpc.stub.MetadataUtils;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.file.Path;
import java.security.interfaces.RSAPublicKey;
import java.time.Instant;
import java.time.Clock;
import java.time.Duration;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.*;
import org.springframework.data.redis.connection.lettuce.LettuceConnectionFactory;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.transaction.support.TransactionTemplate;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.repository.JdbcAccountRepository;
import voice.backend.auth.roleidentityauthority.*;

/** Actual mTLS + HTTPS/JWKS + Redis replay + Auth SQL source. SQL setup invokes real capture triggers. */
@Testcontainers
class SubscriptionLifecycleNativeGrpcTest {
  static final String RPC=SubscriptionLifecyclePrincipalTest.RPC;
  static final String PEER="urn:voice:test:subscription-lifecycle";
  @Container static final PostgreSQLContainer<?> postgres=new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
      .withDatabaseName("subscription_owner_transport").withUsername("voice").withPassword("voice");
  @Container static final GenericContainer<?> cache=new GenericContainer<>(DockerImageName.parse("redis:7-alpine")).withExposedPorts(6379);
  static DriverManagerDataSource database;
  static LettuceConnectionFactory connections;
  static StringRedisTemplate redis;
  @BeforeAll static void setup() {
    database=new DriverManagerDataSource(postgres.getJdbcUrl(),postgres.getUsername(),postgres.getPassword());
    Flyway.configure().dataSource(database).locations("classpath:db/migration").load().migrate();
    var jdbc=new JdbcTemplate(database);
    jdbc.execute("CREATE ROLE subscription_owner LOGIN PASSWORD 'fixture-only'");
    jdbc.execute("GRANT USAGE ON SCHEMA public TO subscription_owner");
    jdbc.execute("GRANT SELECT,UPDATE ON accounts,account_deletion_operations,account_subscription_lifecycle_heads,account_subscription_lifecycle_outbox TO subscription_owner");
    jdbc.execute("GRANT SELECT ON flyway_schema_history TO subscription_owner");
    connections=new LettuceConnectionFactory(cache.getHost(),cache.getMappedPort(6379)); connections.afterPropertiesSet(); connections.start();
    redis=new StringRedisTemplate(connections); redis.afterPropertiesSet(); redis.hasKey("fixture-only-warmup");
  }
  @AfterAll static void closeRedis() { if(connections!=null) connections.destroy(); }

  @Test void actualFactoryReturnsFullCurrentInputAndRejectsReplayAndChangedRequest() throws Exception {
    try(var f=new Fixture("/jwks")) {
      f.warm();
      var request=f.request(); var claims=f.claims(request);
      var observed=f.call(f.channel,request,claims,new Metadata());
      assertEquals(1,observed.getProtocolVersion()); assertEquals(f.account.toString(),observed.getAccountId());
      assertEquals("deleted",observed.getAccountStatus()); assertTrue(observed.hasDeletedAt());
      assertEquals(1,observed.getHead().getSourceRevision()); assertEquals(observed.getHead(),observed.getFacts(0));
      assertEquals(f.cycle.toString(),observed.getHead().getDeletionCycleId());
      assertEquals(SubscriptionAccountCaptureCoverage.SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_CAPTURED,observed.getCaptureCoverage());
      assertTrue(f.requests.get()>0,"real HTTPS JWKS handler must have supplied the signing keys");
      String digest=java.util.HexFormat.of().formatHex(java.security.MessageDigest.getInstance("SHA-256").digest(((String)claims.get("jti")).getBytes(java.nio.charset.StandardCharsets.UTF_8)));
      assertEquals("1",redis.opsForValue().get("auth:principal:replay:subscription:"+digest));
      assertEquals(Status.Code.UNAUTHENTICATED,assertThrows(StatusRuntimeException.class,()->f.call(f.channel,request,claims,new Metadata())).getStatus().getCode());
      var changed=request.toBuilder().setAccountId(UUID.randomUUID().toString()).build();
      var fresh=f.claims(request);
      assertEquals(Status.Code.UNAUTHENTICATED,assertThrows(StatusRuntimeException.class,()->f.call(f.channel,changed,fresh,new Metadata())).getStatus().getCode());
      var accounts=new JdbcAccountRepository(new NamedParameterJdbcTemplate(database));
      assertTrue(accounts.restoreDeleted(f.account));
      var restored=f.call(f.channel,request,f.claims(request),new Metadata());
      assertEquals("active",restored.getAccountStatus()); assertFalse(restored.hasDeletedAt()); assertEquals(2,restored.getFactsCount());
      assertEquals(SubscriptionAccountLifecycleKind.SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_RESTORE_REQUESTED,restored.getHead().getKind());
      assertEquals(f.cycle.toString(),restored.getHead().getDeletionCycleId());
    }
  }

  @Test void lowerMissingUnknownFieldsAndRawHeadersNeverBecomeSuccessfulObservation() throws Exception {
    try(var f=new Fixture("/jwks")) {
      f.warm();
      for(var request:List.of(f.request().toBuilder().setMinimumLifecycleRevision(2).build(),f.request().toBuilder().setAccountId(UUID.randomUUID().toString()).build()))
        assertEquals(Status.Code.UNAVAILABLE,assertThrows(StatusRuntimeException.class,()->f.call(f.channel,request,f.claims(request),new Metadata())).getStatus().getCode());
      var unknown=f.request().toBuilder().setUnknownFields(com.google.protobuf.UnknownFieldSet.newBuilder().addField(99,com.google.protobuf.UnknownFieldSet.Field.newBuilder().addVarint(1).build()).build()).build();
      assertEquals(Status.Code.INVALID_ARGUMENT,assertThrows(StatusRuntimeException.class,()->f.call(f.channel,unknown,f.claims(unknown),new Metadata())).getStatus().getCode());
      var raw=new Metadata(); raw.put(Metadata.Key.of("x-profile-id",Metadata.ASCII_STRING_MARSHALLER),UUID.randomUUID().toString());
      assertEquals(Status.Code.UNAUTHENTICATED,assertThrows(StatusRuntimeException.class,()->f.call(f.channel,f.request(),f.claims(f.request()),raw)).getStatus().getCode());
    }
  }

  @Test void trustedWrongPeerAndMissingClientCertificateFailBeforeJwks() throws Exception {
    try(var f=new Fixture("/jwks");var wrong=new ClosingChannel(f.channel("bot"));var missing=new ClosingChannel(f.channel(null))) {
      f.signingKid="unapproved-denied-peer-probe";
      assertEquals(Status.Code.UNAUTHENTICATED,assertThrows(StatusRuntimeException.class,()->f.call(wrong.channel,f.request(),f.claims(f.request()),new Metadata())).getStatus().getCode());
      assertThrows(StatusRuntimeException.class,()->f.call(missing.channel,f.request(),f.claims(f.request()),new Metadata()));
      assertEquals(f.startupRequests,f.requests.get(),"denied peers cannot trigger request-time key refresh");
    }
  }

  @Test void ordinaryListenerRejectsTheNewMethodEvenIfAccidentallyRegistered() throws Exception {
    try(var f=new Fixture("/jwks")) {
      var server=NettyServerBuilder.forPort(0).addService(ServerInterceptors.intercept(new AuthSubscriptionLifecycleGrpcAdapter(f.source),new AuthPrincipalServerInterceptor(null))).build().start();
      try(var plain=new ClosingChannel(NettyChannelBuilder.forAddress("localhost",server.getPort()).usePlaintext().build())) {
        assertEquals(Status.Code.PERMISSION_DENIED,assertThrows(StatusRuntimeException.class,()->f.call(plain.channel,f.request(),f.claims(f.request()),new Metadata())).getStatus().getCode());
        assertEquals(f.startupRequests,f.requests.get());
      } finally {server.shutdownNow();server.awaitTermination(5,TimeUnit.SECONDS);}
    }
  }

  @Test void serverBudgetExpiresDuringRealSqlLockWithoutLateSuccessfulRead() throws Exception {
    try(var f=new Fixture("/jwks");var lock=database.getConnection()) {
      f.warm();
      lock.setAutoCommit(false);
      try(var statement=lock.prepareStatement("SELECT id FROM accounts WHERE id=? FOR UPDATE")) {statement.setObject(1,f.account);statement.executeQuery().close();}
      int pid;try(var statement=lock.createStatement();var row=statement.executeQuery("SELECT pg_backend_pid()")){row.next();pid=row.getInt(1);}
      var executor=Executors.newSingleThreadExecutor();
      try {
        var response=executor.submit(()->f.call(f.channel,f.request(),f.claims(f.request()),new Metadata()));
        boolean blocked=false;long end=System.nanoTime()+TimeUnit.MILLISECONDS.toNanos(350);
        while(!blocked&&System.nanoTime()<end){blocked=Boolean.TRUE.equals(new JdbcTemplate(database).queryForObject("SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE usename='subscription_owner' AND ?=ANY(pg_blocking_pids(pid)))",Boolean.class,pid));if(!blocked)Thread.sleep(5);}
        assertTrue(blocked,"actual authenticated owner handler must have reached the blocked SQL read");
        var failure=assertThrows(ExecutionException.class,()->response.get(2,TimeUnit.SECONDS));
        assertEquals(Status.Code.DEADLINE_EXCEEDED,Status.fromThrowable(failure.getCause()).getCode());
      } finally {lock.rollback();executor.shutdownNow();}
    }
  }

  @Test void authenticationIsAlsoInsideServerBudget() throws Exception {
    try(var f=new Fixture("/slow")) {
      f.signingKid="unapproved-refresh-probe";
      long start=System.nanoTime();
      assertEquals(Status.Code.DEADLINE_EXCEEDED,assertThrows(StatusRuntimeException.class,()->f.call(f.channel,f.request(),f.claims(f.request()),new Metadata())).getStatus().getCode());
      assertTrue(System.nanoTime()-start<TimeUnit.SECONDS.toNanos(2));
      assertEquals(2,f.requests.get(),"startup preflight plus the bounded unknown-key refresh");
    }
  }

  static Path resource(String name) throws Exception {return AuthPrincipalServicesTest.resource(name).toPath();}
  static final class ClosingChannel implements AutoCloseable {
    final ManagedChannel channel;ClosingChannel(ManagedChannel channel){this.channel=channel;}
    @Override public void close() throws Exception{channel.shutdownNow();channel.awaitTermination(5,TimeUnit.SECONDS);}
  }
  static final class Fixture implements AutoCloseable {
    String signingKid="current";
    final UUID account,cycle=UUID.randomUUID();
    final AtomicInteger requests=new AtomicInteger();
    final ExecutorService httpsThreads=Executors.newCachedThreadPool(Thread.ofPlatform().daemon().factory());
    final HttpsServer https;
    final AuthSubscriptionLifecycleOwnerStore source;
    final AuthSubscriptionLifecycleGrpcServer.Owned owned;
    final int startupRequests;
    final ManagedChannel channel;
    final URI jwksEndpoint;
    Fixture(String endpoint) throws Exception {
      var jdbc=new JdbcTemplate(database);
      account=new JdbcAccountRepository(new NamedParameterJdbcTemplate(database)).create(UUID.randomUUID()+"@fixture.invalid",null,"fixture-only","regular").id();
      new TransactionTemplate(new DataSourceTransactionManager(database)).executeWithoutResult(tx->{
        jdbc.update("UPDATE accounts SET status='deleted',deleted_at=clock_timestamp(),session_epoch=session_epoch+1 WHERE id=?",account);
        jdbc.update("INSERT INTO account_deletion_operations(operation_id,account_id,session_epoch,restore_token_hash,state,next_attempt_at,created_at,updated_at) SELECT ?,id,session_epoch,'fixture-only','PENDING_FLOOR',clock_timestamp(),clock_timestamp(),clock_timestamp() FROM accounts WHERE id=?",cycle,account);
      });
      source=new AuthSubscriptionLifecycleOwnerStore(new DriverManagerDataSource(postgres.getJdbcUrl(),"subscription_owner","fixture-only"),new AuthIdentityDatabasePolicy(postgres.getUsername(),AuthIdentityDatabasePolicy.Loader.FLYWAY));
      assertEquals(1,source.observe(account,1).facts().size());
      byte[] jwks=AuthPrincipalJwksResolverTest.set(AuthPrincipalJwksResolverTest.jwk("current",(RSAPublicKey)AuthPrincipalVerifierTest.KEY.getPublic()),AuthPrincipalJwksResolverTest.jwk("next",AuthPrincipalJwksResolverTest.NEXT));
      https=HttpsServer.create(new InetSocketAddress("127.0.0.1",0),0);https.setHttpsConfigurator(new HttpsConfigurator(AuthPrincipalJwksHttpTest.fixtureContext()));https.setExecutor(httpsThreads);
      for(String path:List.of("/jwks","/slow"))https.createContext(path,exchange->{requests.incrementAndGet();if(path.equals("/slow")){try{Thread.sleep(1000);}catch(InterruptedException e){Thread.currentThread().interrupt();}}
        try{exchange.sendResponseHeaders(200,jwks.length);try(var output=exchange.getResponseBody()){output.write(jwks);}}finally{exchange.close();}});
      https.start();
      var config=new SubscriptionLifecycleTransportConfig(new InetSocketAddress("127.0.0.1",0),resource("server-cert.pem"),resource("server-key.pem"),resource("server-cert.pem"),PEER,
          URI.create("https://localhost:"+https.getAddress().getPort()+endpoint),resource("server-cert.pem"));
      jwksEndpoint=config.jwks();
      owned=AuthSubscriptionLifecycleGrpcServer.build(config,source,redis);startupRequests=requests.get();owned.server.start();channel=channel("subscription-lifecycle");
      long end=System.nanoTime()+TimeUnit.SECONDS.toNanos(5);
      while(channel.getState(true)!=ConnectivityState.READY&&System.nanoTime()<end)Thread.sleep(5);
    }
    ManagedChannel channel(String certificate) throws Exception {
      var tls=GrpcSslContexts.forClient().trustManager(resource("server-cert.pem").toFile());
      if(certificate!=null)tls.keyManager(resource(certificate+"-cert.pem").toFile(),resource(certificate+"-key.pem").toFile());
      return NettyChannelBuilder.forAddress("localhost",owned.server.getPort()).sslContext(tls.build()).build();
    }
    ObserveSubscriptionAccountRequest request(){return ObserveSubscriptionAccountRequest.newBuilder().setAccountId(account.toString()).setMinimumLifecycleRevision(1).build();}
    void prewarmCredentialCode() throws Exception {
      var clock=Clock.systemUTC();
      try(var keys=IdentityAuthorityJwks.subscriptionLifecycle(jwksEndpoint,resource("server-cert.pem"),clock,Duration.ofSeconds(30),Duration.ofMinutes(2),Duration.ofSeconds(5));
          var replay=new RedisPrincipalReplayGuard(redis,clock)) {
        replay.requireAvailable();
        var request=request();var claims=claims(request);
        AuthPrincipalVerifier.subscriptionLifecycle(keys,replay,clock).verify(AuthPrincipalVerifierTest.token(claims),RPC,(String)claims.get("request_id"),AuthPrincipalServerInterceptor.requestHash(request));
      }
    }
    // Explicit test setup only: a cold JVM/HTTPS request may legitimately use its
    // entire 500ms budget. A distinct fresh request establishes the warm serving
    // fixture; the slow-authentication case independently verifies cold denial.
    void warm(){
      try{call(channel,request(),claims(request()),new Metadata());}
      catch(StatusRuntimeException cold){if(cold.getStatus().getCode()!=Status.Code.DEADLINE_EXCEEDED)throw cold;}
      assertEquals(account.toString(),call(channel,request(),claims(request()),new Metadata()).getAccountId());
    }
    Map<String,Object> claims(ObserveSubscriptionAccountRequest request){
      var claims=SubscriptionLifecyclePrincipalTest.claims();var now=Instant.now();
      claims.put("request_id",UUID.randomUUID().toString());claims.put("request_hash",AuthPrincipalServerInterceptor.requestHash(request));
      claims.put("iat",now.getEpochSecond());claims.put("nbf",now.getEpochSecond());claims.put("exp",now.plusSeconds(30).getEpochSecond());return claims;
    }
    ObserveSubscriptionAccountResponse call(ManagedChannel target,ObserveSubscriptionAccountRequest request,Map<String,Object> claims,Metadata extra){
      var metadata=new Metadata();metadata.put(Metadata.Key.of("authorization",Metadata.ASCII_STRING_MARSHALLER),"Bearer "+AuthPrincipalVerifierTest.token(claims,Map.of("alg","RS256","kid",signingKid),AuthPrincipalVerifierTest.KEY.getPrivate()));
      metadata.put(Metadata.Key.of("x-request-id",Metadata.ASCII_STRING_MARSHALLER),(String)claims.get("request_id"));metadata.merge(extra);
      return AuthSubscriptionLifecycleServiceGrpc.newBlockingStub(ClientInterceptors.intercept(target,MetadataUtils.newAttachHeadersInterceptor(metadata)))
          .withDeadlineAfter(2,TimeUnit.SECONDS).observeSubscriptionAccount(request);
    }
    @Override public void close() throws Exception{channel.shutdownNow();owned.close();https.stop(0);httpsThreads.shutdownNow();channel.awaitTermination(5,TimeUnit.SECONDS);owned.server.awaitTermination(5,TimeUnit.SECONDS);}
  }
}
