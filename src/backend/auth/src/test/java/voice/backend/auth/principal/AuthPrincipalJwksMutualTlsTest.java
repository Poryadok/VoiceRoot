package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;

import com.sun.net.httpserver.HttpsConfigurator;
import com.sun.net.httpserver.HttpsParameters;
import com.sun.net.httpserver.HttpsServer;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyFactory;
import java.security.KeyStore;
import java.security.cert.CertificateFactory;
import java.security.spec.PKCS8EncodedKeySpec;
import java.util.Base64;
import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.mock.env.MockEnvironment;

class AuthPrincipalJwksMutualTlsTest {
  @TempDir Path files;

  @Test void bootstrapFetchPresentsWorkloadKeypairWithoutJwtOrPublicReadiness() throws Exception {
    Path certificate=fixture("server-cert.pem"), key=fixture("server-key.pem");
    var server=HttpsServer.create(new InetSocketAddress("127.0.0.1",0),0);
    var tls=fixtureTls(certificate,key,true);
    server.setHttpsConfigurator(new HttpsConfigurator(tls) {
      @Override public void configure(HttpsParameters parameters) {
        var policy=tls.getDefaultSSLParameters();policy.setNeedClientAuth(true);parameters.setSSLParameters(policy);
      }
    });
    server.createContext("/jwks",exchange->{
      assertNull(exchange.getRequestHeaders().getFirst("authorization"));
      byte[] body="{\"keys\":[]}".getBytes(java.nio.charset.StandardCharsets.UTF_8);
      exchange.sendResponseHeaders(200,body.length);
      try(var output=exchange.getResponseBody()){output.write(body);}
    });
    server.start();
    try {
      var environment=new MockEnvironment().withProperty("S2S_JWKS_CA_FILE",certificate.toString())
          .withProperty("S2S_JWKS_CLIENT_CERT_FILE",certificate.toString())
          .withProperty("S2S_JWKS_CLIENT_KEY_FILE",key.toString());
      URI endpoint=URI.create("https://localhost:"+server.getAddress().getPort()+"/jwks");
      try(var client=AuthPrincipalConfiguration.httpClient(environment)) {
        assertEquals("HTTPS",client.sslParameters().getEndpointIdentificationAlgorithm());
        assertArrayEquals("{\"keys\":[]}".getBytes(java.nio.charset.StandardCharsets.UTF_8),AuthPrincipalConfiguration.fetch(client,endpoint));
      }
      try(var anonymous=java.net.http.HttpClient.newBuilder().sslContext(fixtureTls(certificate,key,false)).build()) {
        assertThrows(IllegalArgumentException.class,()->AuthPrincipalConfiguration.fetch(anonymous,endpoint));
      }
      environment.setProperty("S2S_JWKS_CA_FILE",fixture("wrong-ca-cert.pem").toString());
      try(var wrongTrust=AuthPrincipalConfiguration.httpClient(environment)) {
        assertThrows(IllegalArgumentException.class,()->AuthPrincipalConfiguration.fetch(wrongTrust,endpoint));
      }
    } finally {server.stop(0);}
  }

  @Test void partialOrMissingWorkloadMaterialNeverDowngradesToAnonymous() {
    for(String property:new String[]{"S2S_JWKS_CA_FILE","S2S_JWKS_CLIENT_CERT_FILE","S2S_JWKS_CLIENT_KEY_FILE"}) {
      var partial=new MockEnvironment().withProperty(property,"missing.pem");
      assertThrows(IllegalArgumentException.class,()->AuthPrincipalConfiguration.httpClient(partial));
      assertThrows(IllegalArgumentException.class,()->AuthPrincipalConfiguration.fromEnvironment(partial,null,null));
    }
    var environment=new MockEnvironment().withProperty("S2S_JWKS_URLS_JSON",
        "{\"gateway\":\"https://gateway.invalid/jwks\",\"space\":\"https://space.invalid/jwks\",\"moderation\":\"https://moderation.invalid/jwks\"}");
    assertThrows(IllegalArgumentException.class,()->AuthPrincipalConfiguration.fromEnvironment(environment,
        org.mockito.Mockito.mock(voice.backend.auth.sessionepoch.SessionEpochFloorStore.class),
        org.mockito.Mockito.mock(org.springframework.data.redis.core.StringRedisTemplate.class)));
  }

  private Path fixture(String name)throws Exception {
    Path target=files.resolve(name);
    try(var input=getClass().getResourceAsStream("/principal-tls/"+name)){Files.copy(input,target);}
    return target;
  }
  private static SSLContext fixtureTls(Path certificate,Path privateKey,boolean presentCertificate)throws Exception {
    java.security.cert.Certificate cert;
    try(var input=Files.newInputStream(certificate)){cert=CertificateFactory.getInstance("X.509").generateCertificate(input);}
    String pem=Files.readString(privateKey);byte[] der=Base64.getMimeDecoder().decode(pem.replace("-----BEGIN PRIVATE KEY-----","").replace("-----END PRIVATE KEY-----",""));
    var key=KeyFactory.getInstance("RSA").generatePrivate(new PKCS8EncodedKeySpec(der));char[] password="fixture-only".toCharArray();
    var keys=KeyStore.getInstance("PKCS12");keys.load(null,password);keys.setKeyEntry("fixture",key,password,new java.security.cert.Certificate[]{cert});
    var manager=KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());manager.init(keys,password);
    var trust=KeyStore.getInstance("PKCS12");trust.load(null,password);trust.setCertificateEntry("fixture",cert);
    var trusted=TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());trusted.init(trust);
    var context=SSLContext.getInstance("TLS");context.init(presentCertificate?manager.getKeyManagers():null,trusted.getTrustManagers(),null);return context;
  }
}
