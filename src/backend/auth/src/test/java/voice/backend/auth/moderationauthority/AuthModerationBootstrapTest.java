package voice.backend.auth.moderationauthority;

import static org.mockito.Mockito.*;
import static org.junit.jupiter.api.Assertions.*;
import java.nio.file.*;
import java.nio.file.attribute.PosixFilePermissions;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.mock.env.MockEnvironment;

class AuthModerationBootstrapTest {
  @TempDir Path directory;
  @Test void terminalDeadlineWorkerCannotPurgeBeforeOwnAuthority() {
    var store=mock(AuthModerationStore.class);
    new AuthModerationConfiguration.TerminalDeadlineWorker(store).purge();
    verifyNoInteractions(store);
  }
  @Test void protectedContextRejectsUnsafeOrAmbiguousCarrier() throws Exception {
    UUID store=UUID.randomUUID(),generation=UUID.randomUUID(),deployment=UUID.randomUUID();
    String valid=body("LIVE_CURRENT",store,generation,deployment,null);
    Path file=write(valid);assertEquals(store,AuthModerationStartContext.read(file).storeIncarnation());
    for(String invalid:new String[]{valid.replace("\"protocol_version\":1","\"protocol_version\":1,\"protocol_version\":1"),
        valid.replace("\"protocol_version\":1","\"protocol_version\":4294967297"),valid.replace("{","{\"ready\":true,"),
        valid+" {}",body("DECLARED_RESTORE",store,generation,deployment,null),
        body("LIVE_CURRENT",store,generation,deployment,UUID.randomUUID())," ".repeat(16385)})
      assertThrows(IllegalArgumentException.class,()->AuthModerationStartContext.read(write(invalid)));
    Files.setPosixFilePermissions(file,PosixFilePermissions.fromString("rw-------"));
    assertThrows(IllegalArgumentException.class,()->AuthModerationStartContext.read(file));
    Files.setPosixFilePermissions(file,PosixFilePermissions.fromString("r-x------"));
    assertThrows(IllegalArgumentException.class,()->AuthModerationStartContext.read(file));
    Files.setPosixFilePermissions(file,PosixFilePermissions.fromString("r--------"));
    Path link=directory.resolve("context-link");Files.createSymbolicLink(link,file);
    assertThrows(IllegalArgumentException.class,()->AuthModerationStartContext.read(link));
    Path shared=Files.createDirectory(directory.resolve("writable-ancestor"));
    Files.setPosixFilePermissions(shared,PosixFilePermissions.fromString("rwxrwxrwx"));
    Path privateChild=Files.createDirectory(shared.resolve("private"));Files.setPosixFilePermissions(privateChild,PosixFilePermissions.fromString("rwx------"));
    Path replaced=privateChild.resolve("context.json");Files.writeString(replaced,valid);Files.setPosixFilePermissions(replaced,PosixFilePermissions.fromString("r--------"));
    assertThrows(IllegalArgumentException.class,()->AuthModerationStartContext.read(replaced));
    assertThrows(IllegalArgumentException.class,()->AuthModerationStartContext.read(Path.of("relative.json")));
  }
  @Test void actualLocalInventoryEstablishesP1OnlyAfterP0AndBoundContext() throws Exception {
    try(var f=AuthLifecycleDeliveryIntegrationTest.fixture()) {
      UUID account=f.account();Path file=context(f,"LIVE_CURRENT",null);var context=AuthModerationStartContext.read(file);
      var boot=new AuthModerationBootAuthority();var manager=new DataSourceTransactionManager(f.source);var named=new NamedParameterJdbcTemplate(f.source);
      var store=new AuthModerationConfiguration().authModerationStore(named,manager,boot);
      assertThrows(io.grpc.StatusRuntimeException.class,()->store.resolve(account,null));
      var settings=new MockEnvironment().withProperty("AUTH_AUTOMOD_START_CONTEXT_FILE",file.toString())
          .withProperty("AUTH_AUTOMOD_ENVIRONMENT","test").withProperty("AUTH_AUTOMOD_DEPLOYMENT_GENERATION",context.deploymentGeneration().toString());
      try(var runtime=new AuthModerationStartupRuntime(named,manager,boot,settings,null)) {
        runtime.verify();assertThrows(io.grpc.StatusRuntimeException.class,boot::acquire);
        runtime.localListenersStarted();assertEquals("AUTHORITY_P1",runtime.state());
        Runnable captured=boot.acquire();captured.run();assertEquals(account.toString(),store.resolve(account,null).getSubject().getAccountId());
        var workerStore=mock(AuthModerationStore.class);new AuthModerationConfiguration.TerminalDeadlineWorker(workerStore,boot).purge();verify(workerStore).purgeDue();
        Path changed=write(body("LIVE_CURRENT",context.storeIncarnation(),context.authorityGeneration(),UUID.randomUUID(),null));
        Files.setPosixFilePermissions(file,PosixFilePermissions.fromString("rw-------"));Files.writeString(file,Files.readString(changed));Files.setPosixFilePermissions(file,PosixFilePermissions.fromString("r--------"));
        runtime.verify();assertThrows(io.grpc.StatusRuntimeException.class,captured::run);
        assertEquals("AUTHORITY_RECOVERY",runtime.state());
      }
    }
  }
  @Test void validOldRestoreContextOrDifferentStoreCannotOpenAuthority() throws Exception {
    try(var f=AuthLifecycleDeliveryIntegrationTest.fixture()) {
      f.account();var named=new NamedParameterJdbcTemplate(f.source);var verifier=new AuthModerationStartupVerifier(named,new DataSourceTransactionManager(f.source));
      var restored=AuthModerationStartContext.read(context(f,"DECLARED_RESTORE",UUID.randomUUID()));
      assertThrows(IllegalStateException.class,()->verifier.verifyLive(restored,UUID.randomUUID()));
      var valid=AuthModerationStartContext.read(context(f,"LIVE_CURRENT",null));
      var wrong=new AuthModerationStartContext(valid.mode(),valid.environment(),valid.deploymentGeneration(),UUID.randomUUID(),valid.authorityGeneration(),null,valid.sha256());
      assertThrows(IllegalStateException.class,()->verifier.verifyLive(wrong,UUID.randomUUID()));
      UUID incarnation=f.jdbc.queryForObject("SELECT incarnation FROM moderation_startup_identity",UUID.class);
      assertThrows(org.springframework.dao.DataAccessException.class,()->f.jdbc.update("UPDATE moderation_startup_identity SET incarnation=?",UUID.randomUUID()));
      assertThrows(org.springframework.dao.DataAccessException.class,()->f.jdbc.update("DELETE FROM moderation_startup_identity"));
      assertEquals(incarnation,f.jdbc.queryForObject("SELECT incarnation FROM moderation_startup_identity",UUID.class));
    }
  }
  @Test void changedNativeFunctionOrDisabledTriggerIsNotIntactLiveAuthority() throws Exception {
    for(boolean function:new boolean[]{false,true})try(var f=AuthLifecycleDeliveryIntegrationTest.fixture()) {
      f.account();var verifier=new AuthModerationStartupVerifier(new NamedParameterJdbcTemplate(f.source),new DataSourceTransactionManager(f.source));
      var valid=AuthModerationStartContext.read(context(f,"LIVE_CURRENT",null));
      if(function)f.jdbc.execute("CREATE OR REPLACE FUNCTION moderation_startup_identity_guard() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NEW; END'");
      else f.jdbc.execute("ALTER TABLE accounts DISABLE TRIGGER auth_moderation_capture_lifecycle");
      assertThrows(IllegalStateException.class,()->verifier.verifyLive(valid,UUID.randomUUID()));
    }
  }
  @Test void staleProjectionOrInventedTerminalHighWaterIsNotIntactLiveAuthority() throws Exception {
    for(boolean projection:new boolean[]{false,true})try(var f=AuthLifecycleDeliveryIntegrationTest.fixture()) {
      UUID account=f.account();var verifier=new AuthModerationStartupVerifier(new NamedParameterJdbcTemplate(f.source),new DataSourceTransactionManager(f.source));
      var valid=AuthModerationStartContext.read(context(f,"LIVE_CURRENT",null));
      if(projection){f.jdbc.execute("ALTER TABLE accounts DISABLE TRIGGER auth_moderation_capture_lifecycle");f.jdbc.update("UPDATE accounts SET deleted_at=clock_timestamp() WHERE id=?",account);f.jdbc.execute("ALTER TABLE accounts ENABLE TRIGGER auth_moderation_capture_lifecycle");}
      else f.jdbc.update("UPDATE moderation_authority_generation SET terminal_high_water=1");
      assertThrows(IllegalStateException.class,()->verifier.verifyLive(valid,UUID.randomUUID()));
    }
  }
  @Test void generationClosureRollsBackTerminalLedgerAndMappingBeforeCommit() throws Exception {
    try(var f=AuthLifecycleDeliveryIntegrationTest.fixture()) {
      UUID account=UUID.randomUUID();f.jdbc.update("INSERT INTO accounts(id,password_hash,type,status,deleted_at) VALUES (?,'fixture','regular','deleted',clock_timestamp()-interval '31 days')",account);
      var named=new NamedParameterJdbcTemplate(f.source);var manager=new DataSourceTransactionManager(f.source);var boot=new AuthModerationBootAuthority();
      boot.install(new AuthModerationStartupVerifier(named,manager).verifyLive(AuthModerationStartContext.read(context(f,"LIVE_CURRENT",null)),boot.bootId()));
      var store=new AuthModerationStore(named,manager,()->{Runnable check=boot.acquire();int[] calls={0};return ()->{check.run();if(++calls[0]==3){boot.close();check.run();}};});
      assertThrows(io.grpc.StatusRuntimeException.class,store::purgeDue);
      assertEquals(0,f.jdbc.queryForObject("SELECT count(*) FROM moderation_terminal_subjects",Integer.class));
      assertEquals(1,f.jdbc.queryForObject("SELECT count(*) FROM accounts WHERE id=? AND moderation_subject_id IS NOT NULL",Integer.class,account));
      assertEquals(2,f.jdbc.queryForObject("SELECT count(*) FROM moderation_lifecycle_delivery",Integer.class));
    }
  }
  Path context(AuthLifecycleDeliveryIntegrationTest.Fixture f,String mode,UUID operation)throws Exception {
    return write(body(mode,f.jdbc.queryForObject("SELECT incarnation FROM moderation_startup_identity",UUID.class),
        f.jdbc.queryForObject("SELECT generation FROM moderation_authority_generation",UUID.class),UUID.randomUUID(),operation));
  }
  Path write(String body)throws Exception {Path file=directory.resolve(UUID.randomUUID()+".json");Files.writeString(file,body);Files.setPosixFilePermissions(file,PosixFilePermissions.fromString("r--------"));return file;}
  String body(String mode,UUID store,UUID generation,UUID deployment,UUID operation) {
    return "{\"protocol_version\":1,\"mode\":\""+mode+"\",\"environment\":\"test\",\"deployment_generation\":\""+deployment+
        "\",\"store_incarnation\":\""+store+"\",\"authority_generation\":\""+generation+"\""+(operation==null?"":",\"operation_id\":\""+operation+"\"")+"}";
  }
}
