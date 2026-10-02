package voice.backend.auth.authoritysource;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.Mockito.*;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.atomic.AtomicInteger;
import javax.sql.DataSource;
import org.junit.jupiter.api.Test;

class AuthSourceReadBudgetTest {
  @Test void owningPoolCheckoutIsBoundedAndCancelledWithoutAConnectionLeak()throws Exception {
    var dataSource=mock(DataSource.class);var waits=new CountDownLatch(1);var interrupted=new AtomicInteger();
    when(dataSource.getConnection()).thenAnswer(call->{try {waits.await();}catch(InterruptedException cancelled){interrupted.incrementAndGet();throw new java.sql.SQLException("cancelled fixture");}throw new java.sql.SQLException("unavailable fixture");});
    try(var reader=new AuthAuthorityReader(dataSource)) {
      org.junit.jupiter.api.Assertions.assertTimeoutPreemptively(java.time.Duration.ofMillis(1500),()->assertThatThrownBy(()->reader.snapshot(List.of())).isInstanceOf(IllegalStateException.class));
      org.awaitility.Awaitility.await().atMost(java.time.Duration.ofSeconds(1)).until(()->interrupted.get()==1);
    } finally {waits.countDown();}
  }
}
