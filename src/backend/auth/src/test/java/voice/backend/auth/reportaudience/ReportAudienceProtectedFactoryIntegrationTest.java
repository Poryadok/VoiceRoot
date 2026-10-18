package voice.backend.auth.reportaudience;

import static org.assertj.core.api.Assertions.*;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sun.net.httpserver.*;
import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.*;
import io.grpc.stub.MetadataUtils;
import java.net.*;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.security.*;
import java.security.cert.*;
import java.security.interfaces.RSAPublicKey;
import java.security.spec.PKCS8EncodedKeySpec;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;
import javax.net.ssl.*;
import javax.sql.DataSource;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.beans.factory.support.StaticListableBeanFactory;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.*;
import org.springframework.mock.env.MockEnvironment;
import org.testcontainers.containers.*;
import org.testcontainers.utility.DockerImageName;
import app.voice.auth.v1.*;

import voice.backend.auth.principal.AuthPrincipalServerInterceptor;

class ReportAudienceProtectedFactoryIntegrationTest {
  @TempDir Path files;
  @Test void actualOwnerFactoryRequiresMtlsAndFreshUserPrincipalAndStopsInMaintenance()throws Exception {
    Path certificate=fixture("server-cert.pem"),key=fixture("server-key.pem"),wrongCA=fixture("wrong-ca-cert.pem");
    var signing=ReportAudienceVerifierTest.pair();var next=ReportAudienceVerifierTest.pair();
    byte[] jwks=new ObjectMapper().writeValueAsBytes(Map.of("keys",List.of(jwk("current",signing),jwk("next",next))));
    var status=new AtomicInteger(503);var executor=Executors.newVirtualThreadPerTaskExecutor();
    HttpsServer https=HttpsServer.create(new InetSocketAddress("127.0.0.1",0),0);
    https.setHttpsConfigurator(new HttpsConfigurator(tls(certificate,key)));https.setExecutor(executor);
    https.createContext("/jwks",exchange->{exchange.sendResponseHeaders(status.get(),jwks.length);try(var output=exchange.getResponseBody()){output.write(jwks);}});https.start();
    ReportAudienceGrpcServer server=null;var channels=new ArrayList<ManagedChannel>();
    try(var postgres=new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine")).withDatabaseName("source").withUsername("fixture").withPassword("fixture").withReuse(false);
        var redis=new GenericContainer<>(DockerImageName.parse("redis:7.4-alpine")).withExposedPorts(6379).withReuse(false)) {
      postgres.start();redis.start();
      var jdbcSource=new DriverManagerDataSource(postgres.getJdbcUrl(),postgres.getUsername(),postgres.getPassword());
      Flyway.configure().dataSource(jdbcSource).locations("classpath:db/migration").load().migrate();
      var jdbc=new JdbcTemplate(jdbcSource);var account=UUID.randomUUID();
      jdbc.update("INSERT INTO accounts(id,password_hash,type,status) VALUES(?,'private fixture','regular','active')",account);
      var databaseCalls=new AtomicInteger();
      DataSource counting=new DelegatingDataSource(jdbcSource){@Override public java.sql.Connection getConnection()throws java.sql.SQLException{databaseCalls.incrementAndGet();return super.getConnection();}};
      var env=new MockEnvironment().withProperty("AUTH_REPORT_AUDIENCE_ENABLED","true")
          .withProperty("AUTH_REPORT_AUDIENCE_GRPC_LISTEN","127.0.0.1:0")
          .withProperty("AUTH_REPORT_AUDIENCE_TLS_CERT_FILE",certificate.toString()).withProperty("AUTH_REPORT_AUDIENCE_TLS_KEY_FILE",key.toString())
          .withProperty("AUTH_REPORT_AUDIENCE_CLIENT_CA_FILE",certificate.toString()).withProperty("AUTH_REPORT_AUDIENCE_JWKS_CA_FILE",certificate.toString())
          .withProperty("AUTH_REPORT_AUDIENCE_REPLAY_REDIS_ADDR",redis.getHost()+":"+redis.getMappedPort(6379))
          .withProperty("S2S_JWKS_URLS_JSON","{\"user\":\"https://localhost:"+https.getAddress().getPort()+"/jwks\"}");
      var beans=new StaticListableBeanFactory(Map.of("owningDataSource",counting));
      server=new ReportAudienceGrpcServer(env,beans.getBeanProvider(DataSource.class));
      server.start();assertThat(server.isRunning()).isTrue();int before=databaseCalls.get();
      var clientTls=GrpcSslContexts.forClient().trustManager(certificate.toFile()).keyManager(certificate.toFile(),key.toFile()).build();
      var channel=NettyChannelBuilder.forAddress("localhost",server.port()).sslContext(clientTls).build();channels.add(channel);
      var request=ClassifyReportAudienceAccountsRequest.newBuilder().setEvaluationId(UUID.randomUUID().toString()).addAccountIds(account.toString()).build();
      var credential=signed(signing,request);
      assertThatThrownBy(()->call(channel,credential,request)).isInstanceOf(StatusRuntimeException.class).satisfies(ex->assertThat(((StatusRuntimeException)ex).getStatus().getCode()).isEqualTo(Status.Code.UNAUTHENTICATED));
      assertThat(databaseCalls).hasValue(before);
      status.set(200);
      // The failed JWKS attempt did not consume this credential. Wait only for
      // the known unknown-kid retry cooldown, not a background publisher.
      org.awaitility.Awaitility.await().atMost(java.time.Duration.ofSeconds(8)).pollInterval(java.time.Duration.ofMillis(200)).ignoreException(StatusRuntimeException.class).untilAsserted(()->{
        var response=call(channel,credential,request);assertThat(response.getAccountsCount()).isEqualTo(1);assertThat(response.getAccounts(0).getState()).isEqualTo(ReportAudienceAccountState.REPORT_AUDIENCE_ACCOUNT_STATE_LIVE);
      });
      int read=databaseCalls.get();
      assertThatThrownBy(()->AuthServiceGrpc.newBlockingStub(channel).withDeadlineAfter(3,TimeUnit.SECONDS).register(RegisterRequest.getDefaultInstance()))
          .isInstanceOf(StatusRuntimeException.class).satisfies(ex->assertThat(((StatusRuntimeException)ex).getStatus().getCode()).isEqualTo(Status.Code.UNIMPLEMENTED));
      assertThat(databaseCalls).as("dedicated listener exposes classification only").hasValue(read);
      assertThatThrownBy(()->call(channel,credential,request)).isInstanceOf(StatusRuntimeException.class);assertThat(databaseCalls).hasValue(read);
      var noCertificate=NettyChannelBuilder.forAddress("localhost",server.port()).sslContext(GrpcSslContexts.forClient().trustManager(certificate.toFile()).build()).build();channels.add(noCertificate);
      var plaintext=NettyChannelBuilder.forAddress("localhost",server.port()).usePlaintext().build();channels.add(plaintext);
      var wrongHost=NettyChannelBuilder.forAddress("localhost",server.port()).sslContext(clientTls).overrideAuthority("wrong.test").build();channels.add(wrongHost);
      var wrongTrust=NettyChannelBuilder.forAddress("localhost",server.port()).sslContext(GrpcSslContexts.forClient().trustManager(wrongCA.toFile()).keyManager(certificate.toFile(),key.toFile()).build()).build();channels.add(wrongTrust);
      for(var rejected:List.of(noCertificate,plaintext,wrongHost,wrongTrust)) assertThatThrownBy(()->call(rejected,signed(signing,request),request)).isInstanceOf(StatusRuntimeException.class);
      assertThat(databaseCalls).hasValue(read);
      var tampered=request.toBuilder().clearAccountIds().addAccountIds(UUID.randomUUID().toString()).build();
      assertThatThrownBy(()->call(channel,signed(signing,request),tampered)).isInstanceOf(StatusRuntimeException.class);assertThat(databaseCalls).hasValue(read);
      jdbc.update("UPDATE accounts SET status='deleted',deleted_at=clock_timestamp() WHERE id=?",account);
      var deleted=call(channel,signed(signing,request),request);
      assertThat(deleted.getAccounts(0).getState()).isEqualTo(ReportAudienceAccountState.REPORT_AUDIENCE_ACCOUNT_STATE_DELETED);
      int maintenance=databaseCalls.get();redis.stop();
      assertThatThrownBy(()->call(channel,signed(signing,request),request)).isInstanceOf(StatusRuntimeException.class).satisfies(ex->assertThat(((StatusRuntimeException)ex).getStatus().getCode()).isEqualTo(Status.Code.UNAUTHENTICATED));
      assertThat(databaseCalls).as("unavailable replay must prevent database entry").hasValue(maintenance);
    } finally {
      if(server!=null)server.stop();for(var channel:channels)channel.shutdownNow().awaitTermination(5,TimeUnit.SECONDS);
      https.stop(0);executor.close();
    }
  }
  private record Credential(String token,String requestId){}
  private static Credential signed(KeyPair key,ClassifyReportAudienceAccountsRequest request)throws Exception {
    var claims=ReportAudienceVerifierTest.claims(Instant.now());String id=UUID.randomUUID().toString();claims.put("request_id",id);claims.put("request_hash",AuthPrincipalServerInterceptor.requestHash(request));return new Credential(ReportAudienceVerifierTest.token(key,claims),id);
  }
  private static ClassifyReportAudienceAccountsResponse call(ManagedChannel channel,Credential credential,ClassifyReportAudienceAccountsRequest request) {
    Metadata headers=new Metadata();headers.put(Metadata.Key.of("authorization",Metadata.ASCII_STRING_MARSHALLER),"Bearer "+credential.token());headers.put(Metadata.Key.of("x-request-id",Metadata.ASCII_STRING_MARSHALLER),credential.requestId());
    return AuthServiceGrpc.newBlockingStub(channel).withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers)).withDeadlineAfter(3,TimeUnit.SECONDS).classifyReportAudienceAccounts(request);
  }
  private static Map<String,String> jwk(String kid,KeyPair pair) {
    var key=(RSAPublicKey)pair.getPublic();var encoder=Base64.getUrlEncoder().withoutPadding();return Map.of("kid",kid,"kty","RSA","use","sig","alg","RS256","n",encoder.encodeToString(key.getModulus().toByteArray()),"e",encoder.encodeToString(key.getPublicExponent().toByteArray()));
  }
  private Path fixture(String name)throws Exception {Path target=files.resolve(name);try(var input=getClass().getResourceAsStream("/principal-tls/"+name)){Files.copy(input,target);}return target;}
  private static SSLContext tls(Path certificate,Path privateKey)throws Exception {
    java.security.cert.Certificate cert;try(var input=Files.newInputStream(certificate)){cert=CertificateFactory.getInstance("X.509").generateCertificate(input);}
    String pem=Files.readString(privateKey,StandardCharsets.US_ASCII);byte[] der=Base64.getMimeDecoder().decode(pem.replace("-----BEGIN PRIVATE KEY-----","").replace("-----END PRIVATE KEY-----",""));
    var key=KeyFactory.getInstance("RSA").generatePrivate(new PKCS8EncodedKeySpec(der));char[] password="public-test-only".toCharArray();
    var keys=KeyStore.getInstance("PKCS12");keys.load(null,password);keys.setKeyEntry("fixture",key,password,new java.security.cert.Certificate[]{cert});
    var manager=KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());manager.init(keys,password);
    var trust=KeyStore.getInstance("PKCS12");trust.load(null,password);trust.setCertificateEntry("fixture",cert);var trusted=TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());trusted.init(trust);
    var context=SSLContext.getInstance("TLS");context.init(manager.getKeyManagers(),trusted.getTrustManagers(),null);return context;
  }
}
