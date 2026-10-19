package voice.backend.auth.principal;

import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyFactory;
import java.security.KeyStore;
import java.security.cert.Certificate;
import java.security.cert.CertificateFactory;
import java.security.spec.PKCS8EncodedKeySpec;
import java.util.Base64;
import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;
import org.springframework.core.env.Environment;

/** Certificate-only workload bootstrap; no JWT/floor/peer-domain dependency. */
final class AuthPrincipalJwksTls {
  private AuthPrincipalJwksTls() {}

  static SSLContext load(Environment environment,boolean required) {
    String certificate=environment.getProperty("S2S_JWKS_CLIENT_CERT_FILE");
    String key=environment.getProperty("S2S_JWKS_CLIENT_KEY_FILE");
    String ca=environment.getProperty("S2S_JWKS_CA_FILE");
    if(certificate==null && key==null && ca==null && !required)return null;
    return loadFiles(certificate,key,ca);
  }

  static SSLContext loadFiles(String certificate,String key,String ca) {
    try {
      Path certificatePath=boundedFile(certificate,1048576),keyPath=boundedFile(key,65536),caPath=boundedFile(ca,1048576);
      var factory=CertificateFactory.getInstance("X.509");
      Certificate[] chain;
      try(var input=Files.newInputStream(certificatePath)) {chain=factory.generateCertificates(input).toArray(Certificate[]::new);}
      if(chain.length==0)throw new IllegalArgumentException();
      String pem=Files.readString(keyPath,java.nio.charset.StandardCharsets.US_ASCII).trim();
      String begin="-----BEGIN PRIVATE KEY-----",end="-----END PRIVATE KEY-----";
      if(!pem.startsWith(begin) || !pem.endsWith(end))throw new IllegalArgumentException();
      byte[] encoded=Base64.getMimeDecoder().decode(pem.substring(begin.length(),pem.length()-end.length()));
      var privateKey=KeyFactory.getInstance(chain[0].getPublicKey().getAlgorithm()).generatePrivate(new PKCS8EncodedKeySpec(encoded));
      char[] password=new char[0];
      var keys=KeyStore.getInstance("PKCS12");keys.load(null,password);keys.setKeyEntry("workload",privateKey,password,chain);
      var keyManagers=KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());keyManagers.init(keys,password);
      var trust=KeyStore.getInstance("PKCS12");trust.load(null,password);
      try(var input=Files.newInputStream(caPath)) {
        int count=0;
        for(var anchor:factory.generateCertificates(input))trust.setCertificateEntry("authority-"+(count++),anchor);
        if(count==0)throw new IllegalArgumentException();
      }
      var trustManagers=TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());trustManagers.init(trust);
      var context=SSLContext.getInstance("TLS");context.init(keyManagers.getKeyManagers(),trustManagers.getTrustManagers(),null);
      return context;
    } catch(Exception ex) {
      throw new IllegalArgumentException("complete valid principal JWKS workload TLS files are required");
    }
  }

  private static Path boundedFile(String raw,long limit)throws Exception {
    if(raw==null || raw.isBlank())throw new IllegalArgumentException();
    Path path=Path.of(raw);
    if(!Files.isRegularFile(path) || Files.size(path)==0 || Files.size(path)>limit)throw new IllegalArgumentException();
    return path;
  }
}
