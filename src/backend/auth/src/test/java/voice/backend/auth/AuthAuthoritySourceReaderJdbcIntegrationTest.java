package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.List;
import java.util.UUID;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.authoritysource.AuthAuthorityReader;

class AuthAuthoritySourceReaderJdbcIntegrationTest {
  private static UUID fixture(int value) {
    return UUID.fromString(String.format("10000000-0000-4000-8000-%012d",value));
  }

  @Test
  void actualOwningDatabaseMatchesSharedJavaGoGoldenForEveryGroup() throws Exception {
    try(var postgres=database()) {
      postgres.start();
      var source=new DriverManagerDataSource(postgres.getJdbcUrl(),postgres.getUsername(),postgres.getPassword());
      Flyway.configure().dataSource(source).locations("classpath:db/migration").load().migrate();
      var jdbc=new JdbcTemplate(source);
      UUID a=fixture(1),sdk=fixture(2),missing=fixture(3),actor=fixture(4),app=fixture(5),env=fixture(6),device=fixture(7),key=fixture(8),request=fixture(9),binding=fixture(10),profile=fixture(11),conversion=fixture(12),grant=fixture(13);
      jdbc.update("INSERT INTO accounts(id,password_hash,type,status) VALUES(?,'private-password','regular','active')",a);
      jdbc.update("INSERT INTO sdk_identities(account_id,actor_id,application_id,environment_id,issuer,provider_subject,created_at) VALUES(?,?,?,?,'fixture','private-subject',now())",sdk,actor,app,env);
      jdbc.update("INSERT INTO sdk_devices(device_id,account_id,thumbprint,public_jwk) VALUES(?,?,'private-thumbprint','private-jwk')",device,sdk);
      jdbc.update("INSERT INTO sdk_device_keys(key_id,device_id,application_id,environment_id,public_jwk,key_thumbprint,generation,not_before,not_after,status) VALUES(?,?,?,?,'private-jwk','private-thumbprint',1,'2026-01-01 UTC','2027-01-01 UTC','active')",key,device,app,env);
      jdbc.update("INSERT INTO sdk_sessions(token_hash,account_id,device_id,ownership_generation,game_subject,expires_at) VALUES(repeat('0',64),?,?,1,'private-game-subject','2027-01-01 UTC')",sdk,device);
      jdbc.update("""
        INSERT INTO sdk_authorizations(request_id,source_account_id,device_id,source_session_hash,source_generation,application_id,environment_id,idempotency_key,request_hash,redirect_uri,code_challenge,client_state,scopes,policy_revision,display_name,expires_at,target_account_id,target_profile_id,target_epoch,profile_revision,game_binding_intent,game_binding_id,game_binding_status,game_binding_authority_revision,game_binding_challenge_id,game_binding_operation_id,game_binding_challenge_nonce,game_binding_consent_revision)
        VALUES(?,?,?,repeat('0',64),1,?,?,?,repeat('1',64),'https://fixture.test',repeat('a',43),'private-state','game.chat.send',3,'private-display','2027-01-01 UTC',?,?,1,4,true,?,'active',2,?,?,repeat('a',43),2)
        """,request,sdk,device,app,env,fixture(14),a,profile,binding,fixture(15),fixture(16));
      jdbc.update("INSERT INTO sdk_linked_sessions(token_hash,request_id,consent_revision,expires_at) VALUES(repeat('2',64),?,2,'2027-01-01 UTC')",request);
      jdbc.update("""
        INSERT INTO sdk_conversion_operations(operation_id,source_account_id,device_id,public_jwk,application_id,environment_id,source_generation,binding_id,mode,state,revision,target_account_id,target_profile_id,target_epoch,profile_revision,idempotency_key,request_hash,created_at)
        VALUES(?,?,?,'private-jwk',?,?,1,?,'existing','prepared',1,?,?,1,4,?,repeat('3',64),now())
        """,conversion,sdk,device,app,env,binding,a,profile,fixture(17));
      jdbc.update("""
        INSERT INTO sdk_game_message_grants(grant_id,authorization_request_id,application_id,environment_id,target_account_id,target_profile_id,target_epoch,binding_id,consent_revision,scopes,policy_revision,profile_revision,status,authority_revision,created_at,updated_at)
        VALUES(?,?,?,?,?,?,1,?,2,'game.chat.send',3,4,'active',2,now(),now())
        """,grant,request,app,env,a,profile,binding);
      try(var reader=new AuthAuthorityReader(source)) {
        var snapshot=reader.snapshot(List.of(a.toString(),sdk.toString(),missing.toString()));
        byte[] golden=java.nio.file.Files.readAllBytes(java.nio.file.Path.of("../pkg/authoritysource/testdata/auth-state-v1.json"));
        assertThat(snapshot.canonicalState()).as("owning JDBC projection matches the shared publisher codec contract").isEqualTo(golden);
        assertThat(snapshot.validUntilUnixMillis()).isEqualTo(1798761600000L);
        jdbc.update("UPDATE sdk_game_message_grants SET status='revoking',authority_revision=authority_revision+1 WHERE grant_id=?",grant);
        var revoked=reader.snapshot(List.of(a.toString(),sdk.toString(),missing.toString()));
        assertThat(revoked.revision()).isGreaterThan(snapshot.revision());
        assertThat(new ObjectMapper().readTree(revoked.canonicalState()).get("sdk_message_grants").get(0).get("status").asText()).isEqualTo("revoking");
        jdbc.update("UPDATE sdk_game_message_grants SET environment_id=? WHERE grant_id=?",app,grant);
        assertThatThrownBy(()->reader.snapshot(List.of(a.toString(),sdk.toString(),missing.toString()))).isInstanceOf(IllegalStateException.class);
      }
    }
  }

  private static PostgreSQLContainer<?> database() {
    return new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
        .withDatabaseName("auth_source").withUsername("fixture").withPassword("fixture")
        .withReuse(false).withLabel("voice.task", "auth-source-reader-20261002");
  }

  @Test
  void exactAccountFactsIncludeStandaloneSdkAndNoCredentialMaterial() throws Exception {
    try(var postgres=database()) {
      postgres.start();
      var source=new DriverManagerDataSource(postgres.getJdbcUrl(),postgres.getUsername(),postgres.getPassword());
      Flyway.configure().dataSource(source).locations("classpath:db/migration").load().migrate();
      var jdbc=new JdbcTemplate(source);
      var account=UUID.randomUUID();var sdk=UUID.randomUUID();var missing=UUID.randomUUID();var control=UUID.randomUUID();
      jdbc.update("INSERT INTO accounts(id,password_hash,type,status) VALUES(?,'private-password','regular','active'),(?,'control-password','regular','active')",account,control);
      jdbc.update("INSERT INTO sdk_identities(account_id,actor_id,application_id,environment_id,issuer,provider_subject,created_at) VALUES(?,?,?,?,'fixture','private-provider-subject',now())",sdk,UUID.randomUUID(),UUID.randomUUID(),UUID.randomUUID());
      var ids=List.of(account.toString(),sdk.toString(),missing.toString()).stream().sorted().toList();
      var reader=new AuthAuthorityReader(source);
      reader.checkSchema();
      var first=reader.snapshot(ids);
      var json=new ObjectMapper().readTree(first.canonicalState());
      assertThat(json.get("accounts").size()).isEqualTo(3);
      assertThat(json.get("sdk_identities").size()).isEqualTo(3);
      assertThat(json.get("accounts").findValues("exists").stream().filter(n->n.booleanValue()).count()).isEqualTo(1);
      assertThat(json.get("sdk_identities").findValues("exists").stream().filter(n->n.booleanValue()).count()).isEqualTo(1);
      String raw=new String(first.canonicalState(),java.nio.charset.StandardCharsets.UTF_8);
      assertThat(raw).doesNotContain("private-password","private-provider-subject",control.toString(),"password_hash","public_jwk","token_hash","approval_jti");
      assertThat(reader.revision(ids)).isEqualTo(first.revision());
      jdbc.update("UPDATE accounts SET status='suspended' WHERE id=?",account);
      var changed=reader.snapshot(ids);
      assertThat(changed.revision()).isGreaterThan(first.revision());
      assertThat(new ObjectMapper().readTree(changed.canonicalState()).get("accounts").findValues("status").stream().anyMatch(n->n.asText().equals("suspended"))).isTrue();
      assertThatThrownBy(()->reader.snapshot(List.of(account.toString(),account.toString()))).isInstanceOf(IllegalArgumentException.class);
    }
  }

  @Test
  void expiredRawSdkLeaseKeepsBytesAndClockStableAndStopsRenewal() throws Exception {
    try(var postgres=database()) {
      postgres.start();
      var source=new DriverManagerDataSource(postgres.getJdbcUrl(),postgres.getUsername(),postgres.getPassword());
      Flyway.configure().dataSource(source).locations("classpath:db/migration").load().migrate();
      var jdbc=new JdbcTemplate(source);
      var sdk=UUID.randomUUID();var actor=UUID.randomUUID();var app=UUID.randomUUID();var environment=UUID.randomUUID();var device=UUID.randomUUID();
      jdbc.update("INSERT INTO sdk_identities(account_id,actor_id,application_id,environment_id,issuer,provider_subject,created_at) VALUES(?,?,?,?,'fixture','private-subject',now())",sdk,actor,app,environment);
      jdbc.update("INSERT INTO sdk_devices(device_id,account_id,thumbprint,public_jwk) VALUES(?,?,'private-thumbprint','private-jwk')",device,sdk);
      jdbc.update("INSERT INTO sdk_device_keys(key_id,device_id,application_id,environment_id,public_jwk,key_thumbprint,generation,not_before,not_after,status) VALUES(?,?,?,?,'private-jwk','private-thumbprint',1,now()-interval '1 minute',now()+interval '1 minute','active')",UUID.randomUUID(),device,app,environment);
      jdbc.update("INSERT INTO sdk_sessions(token_hash,account_id,device_id,ownership_generation,game_subject,expires_at) VALUES(repeat('0',64),?,?,1,'private-game-subject',clock_timestamp()+interval '900 milliseconds')",sdk,device);
      var reader=new AuthAuthorityReader(source);
      var first=reader.snapshot(List.of(sdk.toString()));
      assertThat(first.validUntilUnixMillis()).isGreaterThan(System.currentTimeMillis());
      org.awaitility.Awaitility.await().atMost(java.time.Duration.ofSeconds(3)).until(()->Boolean.TRUE.equals(jdbc.queryForObject("SELECT bool_and(expires_at<clock_timestamp()) FROM sdk_sessions",Boolean.class)));
      var expired=reader.snapshot(List.of(sdk.toString()));
      assertThat(expired.revision()).isEqualTo(first.revision());
      assertThat(expired.canonicalState()).isEqualTo(first.canonicalState());
      // A later key boundary may remain; it cannot extend the saved session.
      assertThat(new String(expired.canonicalState(),java.nio.charset.StandardCharsets.UTF_8)).doesNotContain("private-jwk","private-thumbprint","private-game-subject","token_hash");
      assertThat(new ObjectMapper().readTree(expired.canonicalState()).get("sdk_sessions").get(0).get("until_unix_millis").longValue()).isLessThan(System.currentTimeMillis());
    }
  }

  @Test
  void sourceCatalogDriftAndFailedLoaderHistoryFailClosed() throws Exception {
    try(var postgres=database()) {
      postgres.start();
      var source=new DriverManagerDataSource(postgres.getJdbcUrl(),postgres.getUsername(),postgres.getPassword());
      Flyway.configure().dataSource(source).locations("classpath:db/migration").load().migrate();
      var jdbc=new JdbcTemplate(source);var reader=new AuthAuthorityReader(source);
      reader.checkSchema();
      jdbc.execute("ALTER TABLE sdk_sessions DISABLE TRIGGER auth_source_sdk_sessions");
      assertThatThrownBy(()->reader.revision(List.of())).isInstanceOf(IllegalStateException.class);
      jdbc.execute("ALTER TABLE sdk_sessions ENABLE TRIGGER auth_source_sdk_sessions");
      String body=jdbc.queryForObject("SELECT prosrc FROM pg_proc WHERE oid='public.auth_authority_changed()'::regprocedure",String.class);
      jdbc.execute("CREATE OR REPLACE FUNCTION public.auth_authority_changed() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$");
      assertThatThrownBy(reader::checkSchema).isInstanceOf(IllegalStateException.class);
      jdbc.execute("CREATE OR REPLACE FUNCTION public.auth_authority_changed() RETURNS trigger LANGUAGE plpgsql AS $$"+body+"$$");
      jdbc.execute("UPDATE flyway_schema_history SET success=false WHERE version='26'");
      assertThatThrownBy(reader::checkSchema).isInstanceOf(IllegalStateException.class);
    }
  }
}
