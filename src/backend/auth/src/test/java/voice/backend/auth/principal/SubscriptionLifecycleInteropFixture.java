package voice.backend.auth.principal;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Base64;
import java.util.Map;

/** Standalone actual Auth receiver for the Go caller test; loopback disposable fixtures only. */
public final class SubscriptionLifecycleInteropFixture {
  public static void main(String[] args) throws Exception {
    Path directory=Path.of(System.getenv("VOICE_AUTH_SUBSCRIPTION_FIXTURE_DIR")).toAbsolutePath();
    if(!Files.isDirectory(directory))throw new IllegalArgumentException("fixture directory required");
    Path signer=directory.resolve("subscription-public-test-signer.pem");
    Files.writeString(signer,"-----BEGIN PRIVATE KEY-----\n"+Base64.getMimeEncoder(64,new byte[]{10}).encodeToString(AuthPrincipalVerifierTest.KEY.getPrivate().getEncoded())+"\n-----END PRIVATE KEY-----\n",StandardCharsets.US_ASCII);
    SubscriptionLifecycleNativeGrpcTest.postgres.start();
    SubscriptionLifecycleNativeGrpcTest.cache.start();
    try {
      SubscriptionLifecycleNativeGrpcTest.setup();
      try(var fixture=new SubscriptionLifecycleNativeGrpcTest.Fixture("/jwks")) {
        fixture.prewarmCredentialCode();
        fixture.warm();
        var config=new java.util.LinkedHashMap<String,String>(Map.of("address","localhost:"+fixture.owned.server.getPort(),"server_name","localhost",
            "ca_file",SubscriptionLifecycleNativeGrpcTest.resource("server-cert.pem").toString(),
            "certificate_file",SubscriptionLifecycleNativeGrpcTest.resource("subscription-lifecycle-cert.pem").toString(),
            "key_file",SubscriptionLifecycleNativeGrpcTest.resource("subscription-lifecycle-key.pem").toString(),
            "client_uri",SubscriptionLifecycleNativeGrpcTest.PEER,"signing_key_file",signer.toString(),"signing_key_id","current", "account_id",fixture.account.toString()));
        config.put("postgres_container",SubscriptionLifecycleNativeGrpcTest.postgres.getContainerId());
        config.put("redis_container",SubscriptionLifecycleNativeGrpcTest.cache.getContainerId());
        System.out.println("VOICE_AUTH_SUBSCRIPTION_FIXTURE="+AuthPrincipalVerifierTest.JSON.writeValueAsString(config));System.out.flush();
        var input=new java.io.BufferedReader(new java.io.InputStreamReader(System.in,StandardCharsets.UTF_8));
        String command;
        while((command=input.readLine())!=null&&!command.equals("STOP")) {
          if(command.equals("RESTORE")) {
            boolean restored=new voice.backend.auth.repository.JdbcAccountRepository(new org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate(SubscriptionLifecycleNativeGrpcTest.database)).restoreDeleted(fixture.account);
            System.out.println("VOICE_AUTH_SUBSCRIPTION_RESTORED="+restored);System.out.flush();
          } else throw new IllegalArgumentException("unknown fixture command");
        }
      }
    } finally {
      SubscriptionLifecycleNativeGrpcTest.closeRedis();
      SubscriptionLifecycleNativeGrpcTest.cache.stop();
      SubscriptionLifecycleNativeGrpcTest.postgres.stop();
    }
  }
}
