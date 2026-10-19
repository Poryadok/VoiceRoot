package voice.backend.auth.sessionepoch;

import voice.backend.auth.admission.AdmissionObservation;
import voice.backend.auth.admission.JdbcAdmissionAccountReader;

import static org.junit.jupiter.api.Assertions.*;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Assumptions;

class AdmissionNativeAuthorityTest {
  @Test void realDatabaseAndRedisMaxFloorAndPostRemoteRevocation() {
    String url=System.getenv("AUTH_ADMISSION_TEST_JDBC_URL");Assumptions.assumeTrue(url!=null,"explicit isolated native fixture required");
    var ds=new org.springframework.jdbc.datasource.DriverManagerDataSource(url,"test","test");
    org.flywaydb.core.Flyway.configure().dataSource(ds).load().migrate();
    var jdbc=new org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate(ds);
    var repo=new voice.backend.auth.repository.JdbcAccountRepository(jdbc);
    var account=repo.create("admission-"+UUID.randomUUID()+"@example.test",null,"hash","guest");
    var redis=new org.springframework.data.redis.connection.lettuce.LettuceConnectionFactory(System.getenv("AUTH_ADMISSION_TEST_REDIS_HOST"),6379);
    redis.afterPropertiesSet();
    try {
      var template=new org.springframework.data.redis.core.StringRedisTemplate(redis);template.afterPropertiesSet();
      var floors=new voice.backend.auth.sessionepoch.RedisSessionEpochFloorStore(new voice.backend.auth.sessionepoch.StringRedisSessionEpochCommands(template),java.time.Duration.ofSeconds(2));
      floors.recordAtLeast(account.id(),8);
      var reader=new JdbcAdmissionAccountReader(jdbc,floors);
      UUID profile=UUID.randomUUID();
      var request=new AdmissionObservation.Request(AdmissionObservation.Phase.SUBMIT,AdmissionObservation.Role.APPLICANT,account.id(),profile,UUID.randomUUID(),"ab".repeat(32),"sha256:"+"cd".repeat(32),8L);
      var service=new AdmissionObservation(reader,r->new AdmissionObservation.Profile(account.id(),profile,r.requestHash(),3,false,false));
      assertEquals(8,service.observe(request).floor());
      repo.advanceSessionEpochAtLeast(account.id(),9);
      assertThrows(io.grpc.StatusRuntimeException.class,()->service.observe(request));
      var delayed=new AdmissionObservation.Request(AdmissionObservation.Phase.FIRST_PUBLISH,AdmissionObservation.Role.APPLICANT,account.id(),profile,request.space(),request.binding(),request.requestHash(),null);
      assertEquals(9,service.observe(delayed).floor());
      var revoke=new AdmissionObservation(reader,r->{
        assertFalse(org.springframework.transaction.support.TransactionSynchronizationManager.isActualTransactionActive());
        repo.setStatus(account.id(),"suspended");
        return new AdmissionObservation.Profile(account.id(),profile,r.requestHash(),3,false,false);
      });
      assertEquals(io.grpc.Status.Code.PERMISSION_DENIED,assertThrows(io.grpc.StatusRuntimeException.class,()->revoke.observe(delayed)).getStatus().getCode());
      repo.setStatus(account.id(),"active");template.opsForValue().set(floors.keyFor(account.id()),"corrupt");
      assertEquals(io.grpc.Status.Code.UNAVAILABLE,assertThrows(io.grpc.StatusRuntimeException.class,()->service.observe(delayed)).getStatus().getCode());
    } finally {redis.destroy();}
  }
}
