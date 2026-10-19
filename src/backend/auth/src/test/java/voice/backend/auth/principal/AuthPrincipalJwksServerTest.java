package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyStore;
import java.security.cert.CertificateFactory;
import java.time.Clock;
import java.time.Duration;
import java.util.Base64;
import javax.net.ssl.KeyManager;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.mock.env.MockEnvironment;
import voice.backend.auth.sdkidentity.AuthUserPrincipalIssuer;

class AuthPrincipalJwksServerTest {
  @TempDir Path keys;

  private AuthUserPrincipalIssuer issuer() throws Exception {
    for(String kid:java.util.List.of("current","next")) {
      var generator=java.security.KeyPairGenerator.getInstance("RSA");generator.initialize(2048);
      Files.writeString(keys.resolve(kid+".pem"),"-----BEGIN PRIVATE KEY-----\n"+
          Base64.getMimeEncoder(64,new byte[]{'\n'}).encodeToString(generator.generateKeyPair().getPrivate().getEncoded())+"\n-----END PRIVATE KEY-----\n");
    }
    return AuthUserPrincipalIssuer.load(keys,"current",Clock.systemUTC());
  }
  private MockEnvironment environment() throws Exception {
    var env=new MockEnvironment().withProperty("AUTH_PRINCIPAL_JWKS_PORT","0")
        .withProperty("AUTH_PRINCIPAL_JWKS_LISTEN_HOST","127.0.0.1")
        .withProperty("AUTH_GRPC_TLS_CERT_FILE",AuthPrincipalServicesTest.resource("server-cert.pem").getAbsolutePath())
        .withProperty("AUTH_GRPC_TLS_KEY_FILE",AuthPrincipalServicesTest.resource("server-key.pem").getAbsolutePath())
        .withProperty("AUTH_PRINCIPAL_JWKS_CLIENT_CA_FILE",AuthPrincipalServicesTest.resource("server-cert.pem").getAbsolutePath());
    env.setActiveProfiles("test");return env;
  }

  @Test void privatePublicationNeedsOnlyLocalIssuerAndVerifiedCertificateNotJwtOrPeerReadiness() throws Exception {
    var publisher=new AuthPrincipalJwksServer(issuer(),environment());
    var client=HttpClient.newBuilder().sslContext(AuthPrincipalJwksHttpTest.fixtureContext())
        .connectTimeout(Duration.ofSeconds(2)).build();
    try {
      publisher.start();assertTrue(publisher.isRunning());assertEquals(Integer.MIN_VALUE,publisher.getPhase());
      URI endpoint=URI.create("https://localhost:"+publisher.port()+"/internal/principals/jwks");
      var response=client.send(request(endpoint),HttpResponse.BodyHandlers.ofString());
      assertEquals(200,response.statusCode());
      var body=new ObjectMapper().readTree(response.body());assertEquals(2,body.get("keys").size());
      for(var key:body.get("keys"))for(String secret:java.util.List.of("d","p","q","dp","dq","qi","oth"))assertFalse(key.has(secret));
      assertEquals(404,client.send(request(URI.create(endpoint+"/extra")),HttpResponse.BodyHandlers.discarding()).statusCode());
      assertEquals(405,client.send(HttpRequest.newBuilder(endpoint).timeout(Duration.ofSeconds(2)).POST(HttpRequest.BodyPublishers.noBody()).build(),HttpResponse.BodyHandlers.discarding()).statusCode());
      for(String defect:java.util.List.of("anonymous","wrong-ca","hostname")) {
        var context=defect.equals("hostname")?AuthPrincipalJwksHttpTest.fixtureContext():trustOnly(defect.equals("wrong-ca")?"wrong-ca-cert.pem":"server-cert.pem");
        var denied=HttpClient.newBuilder().sslContext(context).connectTimeout(Duration.ofSeconds(2)).build();
        try {
          URI target=defect.equals("hostname")?URI.create("https://127.0.0.1:"+publisher.port()+"/internal/principals/jwks"):endpoint;
          assertThrows(IOException.class,()->denied.send(request(target),HttpResponse.BodyHandlers.discarding()));
        }finally{denied.shutdownNow();}
      }
    }finally{client.shutdownNow();publisher.stop();assertFalse(publisher.isRunning());assertEquals(-1,publisher.port());}
  }
  @Test void partialTrustFailsBeforeOpeningThePrivatePort() throws Exception {
    var env=environment().withProperty("AUTH_PRINCIPAL_JWKS_CLIENT_CA_FILE","");
    var publisher=new AuthPrincipalJwksServer(issuer(),env);
    assertThrows(IllegalArgumentException.class,publisher::start);assertFalse(publisher.isRunning());assertEquals(-1,publisher.port());
  }
  private static HttpRequest request(URI endpoint){return HttpRequest.newBuilder(endpoint).timeout(Duration.ofSeconds(2)).GET().build();}
  private static SSLContext trustOnly(String certificate) throws Exception {
    var trust=KeyStore.getInstance("PKCS12");trust.load(null,new char[0]);
    try(var input=Files.newInputStream(AuthPrincipalServicesTest.resource(certificate).toPath())){
      trust.setCertificateEntry("fixture",CertificateFactory.getInstance("X.509").generateCertificate(input));
    }
    var factory=TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());factory.init(trust);
    var context=SSLContext.getInstance("TLS");context.init(new KeyManager[0],factory.getTrustManagers(),null);return context;
  }
}
